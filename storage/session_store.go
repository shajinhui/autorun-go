package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultRedisTTLSeconds = 24 * 60 * 60
)

type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"userId"`
	StudentID int64     `json:"studentId"`
	SchoolID  int64     `json:"schoolId"`
	PhoneHash string    `json:"phoneHash,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Store struct {
	db              *sql.DB
	redisURL        string
	redisToken      string
	redisTTLSeconds int
	httpClient      *http.Client
}

var (
	globalStore *Store
	initOnce    sync.Once
	initErr     error
)

func GetStore() (*Store, error) {
	initOnce.Do(func() {
		globalStore, initErr = newStoreFromEnv()
	})
	return globalStore, initErr
}

func newStoreFromEnv() (*Store, error) {
	postgresURL := firstNonEmptyEnv(
		"POSTGRES_URL",
		"DATABASE_URL",
		"POSTGRES_DATABASE_URL_UNPOOLED",
		"POSTGRES_URL_NON_POOLING",
	)
	redisURL := firstNonEmptyEnv(
		"UPSTASH_REDIS_REST_URL",
		"UPSTASH_REDIS_REST_REDIS_URL",
		"UPSTASH_REDIS_REST_KV_REST_API_URL",
		"UPSTASH_REDIS_REST_KV_URL",
	)
	redisToken := firstNonEmptyEnv(
		"UPSTASH_REDIS_REST_TOKEN",
		"UPSTASH_REDIS_REST_REDIS_TOKEN",
		"UPSTASH_REDIS_REST_KV_REST_API_TOKEN",
	)

	if postgresURL == "" && (redisURL == "" || redisToken == "") {
		return &Store{}, nil
	}

	store := &Store{
		redisURL:        strings.TrimRight(redisURL, "/"),
		redisToken:      redisToken,
		redisTTLSeconds: defaultRedisTTLSeconds,
		httpClient: &http.Client{
			Timeout: 6 * time.Second,
		},
	}

	if postgresURL != "" {
		db, err := sql.Open("pgx", postgresURL)
		if err != nil {
			return nil, fmt.Errorf("打开 Postgres 失败: %w", err)
		}
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("连接 Postgres 失败: %w", err)
		}
		store.db = db
		if err := store.ensureSchema(context.Background()); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return store, nil
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func (s *Store) Enabled() bool {
	if s == nil {
		return false
	}
	return s.db != nil || (s.redisURL != "" && s.redisToken != "")
}

func (s *Store) Save(ctx context.Context, phone string, session Session) error {
	if session.StudentID <= 0 || session.Token == "" {
		return fmt.Errorf("session 参数不完整")
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = time.Now()
	}
	phoneHash := hashPhone(phone)
	if phoneHash != "" {
		session.PhoneHash = phoneHash
	}

	var errs []string
	if s.db != nil {
		if err := s.saveToDB(ctx, session); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if s.redisURL != "" && s.redisToken != "" {
		if err := s.saveToRedis(ctx, session); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("保存 session 失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *Store) LoadByStudentID(ctx context.Context, studentID int64) (*Session, string, error) {
	if studentID <= 0 {
		return nil, "", nil
	}
	if s.redisURL != "" && s.redisToken != "" {
		session, err := s.getRedisSessionByStudentID(ctx, studentID)
		if err == nil && session != nil {
			return session, "redis", nil
		}
	}
	if s.db != nil {
		session, err := s.getDBSessionByStudentID(ctx, studentID)
		if err != nil {
			return nil, "", err
		}
		if session != nil {
			if s.redisURL != "" && s.redisToken != "" {
				_ = s.saveToRedis(ctx, *session)
			}
			return session, "database", nil
		}
	}
	return nil, "", nil
}

func (s *Store) LoadByPhone(ctx context.Context, phone string) (*Session, string, error) {
	phoneHash := hashPhone(phone)
	if phoneHash == "" {
		return nil, "", nil
	}
	if s.redisURL != "" && s.redisToken != "" {
		session, err := s.getRedisSessionByPhoneHash(ctx, phoneHash)
		if err == nil && session != nil {
			return session, "redis", nil
		}
	}
	if s.db != nil {
		session, err := s.getDBSessionByPhoneHash(ctx, phoneHash)
		if err != nil {
			return nil, "", err
		}
		if session != nil {
			if s.redisURL != "" && s.redisToken != "" {
				_ = s.saveToRedis(ctx, *session)
			}
			return session, "database", nil
		}
	}
	return nil, "", nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS user_sessions (
  student_id BIGINT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  school_id BIGINT NOT NULL,
  token TEXT NOT NULL,
  phone_hash TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_phone_hash ON user_sessions(phone_hash);
`)
	if err != nil {
		return fmt.Errorf("初始化 user_sessions 表失败: %w", err)
	}
	return nil
}

func (s *Store) saveToDB(ctx context.Context, session Session) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO user_sessions (student_id, user_id, school_id, token, phone_hash, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (student_id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  school_id = EXCLUDED.school_id,
  token = EXCLUDED.token,
  phone_hash = EXCLUDED.phone_hash,
  updated_at = NOW()
`, session.StudentID, session.UserID, session.SchoolID, session.Token, nullIfEmpty(session.PhoneHash))
	if err != nil {
		return fmt.Errorf("写入 Postgres 失败: %w", err)
	}
	return nil
}

func (s *Store) getDBSessionByStudentID(ctx context.Context, studentID int64) (*Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var out Session
	var phoneHash sql.NullString
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT student_id, user_id, school_id, token, phone_hash, updated_at
FROM user_sessions
WHERE student_id = $1
`, studentID).Scan(&out.StudentID, &out.UserID, &out.SchoolID, &out.Token, &phoneHash, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 Postgres 失败: %w", err)
	}
	out.UpdatedAt = updatedAt
	if phoneHash.Valid {
		out.PhoneHash = phoneHash.String
	}
	return &out, nil
}

func (s *Store) getDBSessionByPhoneHash(ctx context.Context, phoneHash string) (*Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var out Session
	var dbPhoneHash sql.NullString
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT student_id, user_id, school_id, token, phone_hash, updated_at
FROM user_sessions
WHERE phone_hash = $1
ORDER BY updated_at DESC
LIMIT 1
`, phoneHash).Scan(&out.StudentID, &out.UserID, &out.SchoolID, &out.Token, &dbPhoneHash, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 Postgres 失败: %w", err)
	}
	out.UpdatedAt = updatedAt
	if dbPhoneHash.Valid {
		out.PhoneHash = dbPhoneHash.String
	}
	return &out, nil
}

func (s *Store) saveToRedis(ctx context.Context, session Session) error {
	valueBytes, _ := json.Marshal(session)
	value := string(valueBytes)

	studentKey := redisStudentKey(session.StudentID)
	if err := s.redisSet(ctx, studentKey, value, s.redisTTLSeconds); err != nil {
		return err
	}
	if session.PhoneHash != "" {
		phoneKey := redisPhoneKey(session.PhoneHash)
		if err := s.redisSet(ctx, phoneKey, value, s.redisTTLSeconds); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) getRedisSessionByStudentID(ctx context.Context, studentID int64) (*Session, error) {
	return s.getRedisSession(ctx, redisStudentKey(studentID))
}

func (s *Store) getRedisSessionByPhoneHash(ctx context.Context, phoneHash string) (*Session, error) {
	return s.getRedisSession(ctx, redisPhoneKey(phoneHash))
}

func (s *Store) getRedisSession(ctx context.Context, key string) (*Session, error) {
	raw, err := s.redisGet(ctx, key)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	var out Session
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析 Redis session 失败: %w", err)
	}
	return &out, nil
}

func (s *Store) redisSet(ctx context.Context, key, value string, ttlSeconds int) error {
	u := fmt.Sprintf("%s/set/%s/%s", s.redisURL, url.PathEscape(key), url.PathEscape(value))
	if ttlSeconds > 0 {
		u = fmt.Sprintf("%s?EX=%d", u, ttlSeconds)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	req.Header.Set("Authorization", "Bearer "+s.redisToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Redis SET 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Redis SET 失败: status=%d body=%s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *Store) redisGet(ctx context.Context, key string) (string, error) {
	u := fmt.Sprintf("%s/get/%s", s.redisURL, url.PathEscape(key))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+s.redisToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Redis GET 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Redis GET 失败: status=%d body=%s", resp.StatusCode, string(body))
	}
	var payload struct {
		Result *string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("Redis GET 解析失败: %w", err)
	}
	if payload.Result == nil {
		return "", nil
	}
	return *payload.Result, nil
}

func hashPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(phone))
	return hex.EncodeToString(sum[:])
}

func redisStudentKey(studentID int64) string {
	return fmt.Sprintf("session:student:%d", studentID)
}

func redisPhoneKey(phoneHash string) string {
	return "session:phone:" + phoneHash
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
