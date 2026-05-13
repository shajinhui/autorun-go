package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultDataDirName = "autorun-go"
	defaultStoreName   = "sessions.json"
)

type Session struct {
	Token      string    `json:"token"`
	UserID     int64     `json:"userId"`
	StudentID  int64     `json:"studentId"`
	SchoolID   int64     `json:"schoolId"`
	SessionKey string    `json:"sessionKey,omitempty"`
	PhoneHash  string    `json:"phoneHash,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

type DebugInfo struct {
	Enabled         bool   `json:"enabled"`
	Path            string `json:"path,omitempty"`
	HasLocalFile    bool   `json:"hasLocalFile"`
	LocalStatus     string `json:"localStatus"`
	WriteReadStatus string `json:"writeReadStatus"`
	Detail          string `json:"detail,omitempty"`
}

type storeFile struct {
	Sessions  []Session `json:"sessions"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	path, err := resolveStorePath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("初始化本地 session 目录失败: %w", err)
	}
	return &Store{path: path}, nil
}

func resolveStorePath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("SESSION_STORE_PATH")); configured != "" {
		return filepath.Abs(configured)
	}
	if dataDir := strings.TrimSpace(os.Getenv("AUTORUN_DATA_DIR")); dataDir != "" {
		return filepath.Abs(filepath.Join(dataDir, defaultStoreName))
	}
	if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
		return filepath.Join(configDir, defaultDataDirName, defaultStoreName), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("无法确定本地 session 存储目录: %w", err)
	}
	return filepath.Join(cwd, ".autorun", defaultStoreName), nil
}

func (s *Store) Enabled() bool {
	return s != nil && strings.TrimSpace(s.path) != ""
}

func (s *Store) Debug(ctx context.Context) DebugInfo {
	info := DebugInfo{
		Enabled: s != nil && s.Enabled(),
	}
	if s == nil || !s.Enabled() {
		info.LocalStatus = "disabled"
		info.WriteReadStatus = "failed"
		info.Detail = "store is not configured"
		return info
	}

	info.Path = s.path
	if _, err := os.Stat(s.path); err == nil {
		info.HasLocalFile = true
	} else if os.IsNotExist(err) {
		info.HasLocalFile = false
	} else {
		info.LocalStatus = "error"
		info.WriteReadStatus = "failed"
		info.Detail = err.Error()
		return info
	}

	if err := ctxErr(ctx); err != nil {
		info.LocalStatus = "error"
		info.WriteReadStatus = "failed"
		info.Detail = err.Error()
		return info
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		info.LocalStatus = "error"
		info.WriteReadStatus = "failed"
		info.Detail = err.Error()
		return info
	}
	if err := s.save(data); err != nil {
		info.LocalStatus = "error"
		info.WriteReadStatus = "error"
		info.Detail = err.Error()
		return info
	}
	if _, err := s.load(); err != nil {
		info.LocalStatus = "error"
		info.WriteReadStatus = "error"
		info.Detail = err.Error()
		return info
	}

	info.LocalStatus = "ok"
	info.WriteReadStatus = "ok"
	if _, err := os.Stat(s.path); err == nil {
		info.HasLocalFile = true
	}
	return info
}

func (s *Store) Save(ctx context.Context, phone string, session Session) (string, error) {
	if err := ctxErr(ctx); err != nil {
		return "", err
	}
	if session.StudentID <= 0 || session.Token == "" {
		return "", fmt.Errorf("session 参数不完整")
	}
	if strings.TrimSpace(session.SessionKey) == "" {
		session.SessionKey = generateSessionKey()
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = time.Now()
	}
	if phoneHash := hashPhone(phone); phoneHash != "" {
		session.PhoneHash = phoneHash
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return "", err
	}

	replaced := false
	for i := range data.Sessions {
		if data.Sessions[i].StudentID == session.StudentID {
			data.Sessions[i] = mergeSession(data.Sessions[i], session)
			replaced = true
			break
		}
	}
	if !replaced {
		data.Sessions = append(data.Sessions, session)
	}
	data.UpdatedAt = time.Now()

	if err := s.save(data); err != nil {
		return "", err
	}
	return session.SessionKey, nil
}

func (s *Store) LoadByStudentID(ctx context.Context, studentID int64) (*Session, string, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, "", err
	}
	if studentID <= 0 {
		return nil, "", nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return nil, "", err
	}
	for _, session := range data.Sessions {
		if session.StudentID == studentID {
			out := session
			return &out, "local", nil
		}
	}
	return nil, "", nil
}

func (s *Store) LoadByPhone(ctx context.Context, phone string) (*Session, string, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, "", err
	}
	phoneHash := hashPhone(phone)
	if phoneHash == "" {
		return nil, "", nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return nil, "", err
	}
	var picked *Session
	for _, session := range data.Sessions {
		if session.PhoneHash != phoneHash {
			continue
		}
		candidate := session
		if picked == nil || candidate.UpdatedAt.After(picked.UpdatedAt) {
			picked = &candidate
		}
	}
	if picked == nil {
		return nil, "", nil
	}
	return picked, "local", nil
}

func (s *Store) LoadBySessionKey(ctx context.Context, sessionKey string) (*Session, string, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, "", err
	}
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil, "", nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.load()
	if err != nil {
		return nil, "", err
	}
	for _, session := range data.Sessions {
		if session.SessionKey == sessionKey {
			out := session
			return &out, "local", nil
		}
	}
	return nil, "", nil
}

func (s *Store) load() (storeFile, error) {
	if s == nil || !s.Enabled() {
		return storeFile{}, fmt.Errorf("本地 session 存储未启用")
	}
	bytes, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return storeFile{Sessions: []Session{}}, nil
	}
	if err != nil {
		return storeFile{}, fmt.Errorf("读取本地 session 失败: %w", err)
	}
	if len(bytes) == 0 {
		return storeFile{Sessions: []Session{}}, nil
	}
	var data storeFile
	if err := json.Unmarshal(bytes, &data); err != nil {
		return storeFile{}, fmt.Errorf("解析本地 session 失败: %w", err)
	}
	if data.Sessions == nil {
		data.Sessions = []Session{}
	}
	return data, nil
}

func (s *Store) save(data storeFile) error {
	if s == nil || !s.Enabled() {
		return fmt.Errorf("本地 session 存储未启用")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("创建本地 session 目录失败: %w", err)
	}
	if data.Sessions == nil {
		data.Sessions = []Session{}
	}
	if data.UpdatedAt.IsZero() {
		data.UpdatedAt = time.Now()
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化本地 session 失败: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".sessions-*.tmp")
	if err != nil {
		return fmt.Errorf("创建本地 session 临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(bytes); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入本地 session 临时文件失败: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("设置本地 session 文件权限失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭本地 session 临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		if removeErr := os.Remove(s.path); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("替换本地 session 文件失败: %w", removeErr)
		}
		if retryErr := os.Rename(tmpPath, s.path); retryErr != nil {
			return fmt.Errorf("保存本地 session 失败: %w", retryErr)
		}
	}
	return nil
}

func mergeSession(existing, incoming Session) Session {
	if incoming.SessionKey == "" {
		incoming.SessionKey = existing.SessionKey
	}
	if incoming.PhoneHash == "" {
		incoming.PhoneHash = existing.PhoneHash
	}
	return incoming
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func hashPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(phone))
	return hex.EncodeToString(sum[:])
}

func generateSessionKey() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
