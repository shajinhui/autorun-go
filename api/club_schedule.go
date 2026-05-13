package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"autorun-go/storage"
	unirunapi "autorun-go/unirunapi"
)

const (
	clubScheduleFileName = "club_schedule.json"
	clubProbeLead        = 10 * time.Minute
	clubSchedulerEvery   = time.Minute
)

type clubScheduleEntry struct {
	StudentID       int64     `json:"studentId"`
	SchoolID        int64     `json:"schoolId,omitempty"`
	SessionKey      string    `json:"sessionKey,omitempty"`
	Enabled         bool      `json:"enabled"`
	LastSignInKey   string    `json:"lastSignInKey,omitempty"`
	LastSignBackKey string    `json:"lastSignBackKey,omitempty"`
	LastProbeAt     time.Time `json:"lastProbeAt,omitempty"`
	LastActionAt    time.Time `json:"lastActionAt,omitempty"`
	LastMessage     string    `json:"lastMessage,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type clubScheduleFile struct {
	Entries   []clubScheduleEntry `json:"entries"`
	UpdatedAt time.Time           `json:"updatedAt"`
}

type clubSchedulePublicState struct {
	Enabled      bool   `json:"enabled"`
	StudentID    int64  `json:"studentId,omitempty"`
	LastProbeAt  string `json:"lastProbeAt,omitempty"`
	LastActionAt string `json:"lastActionAt,omitempty"`
	LastMessage  string `json:"lastMessage,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

type clubDueProbe struct {
	ActivityID int64
	SignType   string
	Key        string
}

var (
	clubScheduleMu        sync.Mutex
	clubScheduleStartOnce sync.Once
)

func StartLocalClubScheduler() {
	clubScheduleStartOnce.Do(func() {
		go runClubScheduler(context.Background())
	})
}

func saveClubSchedule(ctx context.Context, loginInfo unirunapi.LoginResult, sessionKey string, enabled bool) (clubSchedulePublicState, error) {
	if err := ctx.Err(); err != nil {
		return clubSchedulePublicState{}, err
	}
	if loginInfo.StudentID <= 0 {
		return clubSchedulePublicState{}, fmt.Errorf("缺少 studentId，无法保存定时配置")
	}

	clubScheduleMu.Lock()
	defer clubScheduleMu.Unlock()

	file, path, err := loadClubScheduleFileLocked()
	if err != nil {
		return clubSchedulePublicState{}, err
	}

	now := time.Now()
	entryIndex := -1
	for i := range file.Entries {
		if file.Entries[i].StudentID == loginInfo.StudentID {
			entryIndex = i
			break
		}
	}
	if entryIndex < 0 {
		file.Entries = append(file.Entries, clubScheduleEntry{StudentID: loginInfo.StudentID})
		entryIndex = len(file.Entries) - 1
	}

	entry := file.Entries[entryIndex]
	entry.StudentID = loginInfo.StudentID
	entry.SchoolID = loginInfo.SchoolID
	entry.SessionKey = strings.TrimSpace(sessionKey)
	entry.Enabled = enabled
	entry.UpdatedAt = now
	if enabled {
		entry.LastMessage = "定时已开启：活动开始前 10 分钟试探签到，结束前 10 分钟试探签退"
	} else {
		entry.LastMessage = "定时已关闭"
	}
	file.Entries[entryIndex] = entry
	file.UpdatedAt = now

	if err := saveClubScheduleFileLocked(path, file); err != nil {
		return clubSchedulePublicState{}, err
	}
	return entry.public(), nil
}

func loadClubSchedulePublic(studentID int64) clubSchedulePublicState {
	entry, ok, err := loadClubScheduleEntry(studentID)
	if err != nil || !ok {
		return clubSchedulePublicState{Enabled: false, StudentID: studentID}
	}
	return entry.public()
}

func loadClubScheduleEntry(studentID int64) (clubScheduleEntry, bool, error) {
	if studentID <= 0 {
		return clubScheduleEntry{}, false, nil
	}

	clubScheduleMu.Lock()
	defer clubScheduleMu.Unlock()

	file, _, err := loadClubScheduleFileLocked()
	if err != nil {
		return clubScheduleEntry{}, false, err
	}
	for _, entry := range file.Entries {
		if entry.StudentID == studentID {
			return entry, true, nil
		}
	}
	return clubScheduleEntry{}, false, nil
}

func loadEnabledClubSchedules() ([]clubScheduleEntry, error) {
	clubScheduleMu.Lock()
	defer clubScheduleMu.Unlock()

	file, _, err := loadClubScheduleFileLocked()
	if err != nil {
		return nil, err
	}
	entries := make([]clubScheduleEntry, 0, len(file.Entries))
	for _, entry := range file.Entries {
		if entry.Enabled && entry.StudentID > 0 {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func updateClubScheduleRuntime(studentID int64, mutate func(*clubScheduleEntry)) {
	if studentID <= 0 || mutate == nil {
		return
	}

	clubScheduleMu.Lock()
	defer clubScheduleMu.Unlock()

	file, path, err := loadClubScheduleFileLocked()
	if err != nil {
		log.Printf("club schedule load failed: %v", err)
		return
	}
	for i := range file.Entries {
		if file.Entries[i].StudentID != studentID {
			continue
		}
		mutate(&file.Entries[i])
		file.Entries[i].UpdatedAt = time.Now()
		file.UpdatedAt = file.Entries[i].UpdatedAt
		if err := saveClubScheduleFileLocked(path, file); err != nil {
			log.Printf("club schedule save failed: %v", err)
		}
		return
	}
}

func loadClubScheduleFileLocked() (clubScheduleFile, string, error) {
	path, err := resolveClubSchedulePath()
	if err != nil {
		return clubScheduleFile{}, "", err
	}

	file := clubScheduleFile{Entries: []clubScheduleEntry{}}
	bytes, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return file, path, nil
	}
	if err != nil {
		return clubScheduleFile{}, path, err
	}
	if len(bytes) == 0 {
		return file, path, nil
	}
	if err := json.Unmarshal(bytes, &file); err != nil {
		return clubScheduleFile{}, path, fmt.Errorf("解析俱乐部定时配置失败: %w", err)
	}
	return file, path, nil
}

func saveClubScheduleFileLocked(path string, file clubScheduleFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("初始化俱乐部定时目录失败: %w", err)
	}
	bytes, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0600)
}

func resolveClubSchedulePath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CLUB_SCHEDULE_PATH")); configured != "" {
		return filepath.Abs(configured)
	}
	if dataDir := strings.TrimSpace(os.Getenv("AUTORUN_DATA_DIR")); dataDir != "" {
		return filepath.Abs(filepath.Join(dataDir, clubScheduleFileName))
	}
	if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
		return filepath.Join(configDir, "autorun-go", clubScheduleFileName), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("无法确定俱乐部定时配置目录: %w", err)
	}
	return filepath.Join(cwd, ".autorun", clubScheduleFileName), nil
}

func (entry clubScheduleEntry) public() clubSchedulePublicState {
	return clubSchedulePublicState{
		Enabled:      entry.Enabled,
		StudentID:    entry.StudentID,
		LastProbeAt:  formatScheduleTime(entry.LastProbeAt),
		LastActionAt: formatScheduleTime(entry.LastActionAt),
		LastMessage:  entry.LastMessage,
		UpdatedAt:    formatScheduleTime(entry.UpdatedAt),
	}
}

func formatScheduleTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func runClubScheduler(ctx context.Context) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		runClubScheduleTick(ctx)
	}

	ticker := time.NewTicker(clubSchedulerEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runClubScheduleTick(ctx)
		}
	}
}

func runClubScheduleTick(ctx context.Context) {
	entries, err := loadEnabledClubSchedules()
	if err != nil {
		log.Printf("club schedule load enabled failed: %v", err)
		return
	}

	now := time.Now()
	for _, entry := range entries {
		processClubScheduleEntry(ctx, entry, now)
	}
}

func processClubScheduleEntry(ctx context.Context, entry clubScheduleEntry, now time.Time) {
	session, err := loadScheduledSession(ctx, entry)
	if err != nil {
		updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
			current.LastProbeAt = now
			current.LastMessage = err.Error()
		})
		return
	}

	schoolID := session.SchoolID
	if schoolID <= 0 {
		schoolID = entry.SchoolID
	}
	if schoolID <= 0 || session.Token == "" {
		updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
			current.LastProbeAt = now
			current.LastMessage = "定时任务缺少可用登录态"
		})
		return
	}

	queryDate := now.Format("2006-01-02")
	activities, err := unirunapi.GetClubActivityList(session.Token, session.StudentID, queryDate, schoolID)
	if err != nil {
		updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
			current.LastProbeAt = now
			current.LastMessage = fmt.Sprintf("获取活动列表失败: %v", err)
		})
		return
	}

	probes := dueClubProbes(now, queryDate, activities, entry)
	if len(probes) == 0 {
		return
	}

	tfInfo, err := unirunapi.GetSignInTf(session.Token, session.StudentID)
	if err != nil {
		updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
			current.LastProbeAt = now
			current.LastMessage = fmt.Sprintf("试探签到/签退失败: %v", err)
		})
		return
	}
	if tfInfo == nil || isEmptySignInTf(tfInfo) {
		updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
			current.LastProbeAt = now
			current.LastMessage = "已试探：当前没有可签到/签退任务"
		})
		return
	}

	signType := resolveClubSignType(tfInfo)
	for _, probe := range probes {
		if probe.SignType != signType {
			continue
		}
		if probe.ActivityID > 0 && tfInfo.ActivityId > 0 && probe.ActivityID != tfInfo.ActivityId {
			continue
		}

		_, err := unirunapi.SignInOrSignBack(session.Token, unirunapi.SignInOrSignBackBody{
			ActivityId: tfInfo.ActivityId,
			Latitude:   tfInfo.Latitude,
			Longitude:  tfInfo.Longitude,
			SignType:   signType,
			StudentId:  session.StudentID,
		})
		if err != nil {
			updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
				current.LastProbeAt = now
				current.LastMessage = fmt.Sprintf("自动%s失败: %v", clubSignTypeName(signType), err)
			})
			return
		}

		updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
			if signType == "1" {
				current.LastSignInKey = probe.Key
			}
			if signType == "2" {
				current.LastSignBackKey = probe.Key
			}
			current.LastProbeAt = now
			current.LastActionAt = now
			current.LastMessage = fmt.Sprintf("自动%s成功：%s", clubSignTypeName(signType), tfInfo.ActivityName)
		})
		return
	}

	updateClubScheduleRuntime(entry.StudentID, func(current *clubScheduleEntry) {
		current.LastProbeAt = now
		current.LastMessage = "已试探：服务器暂未开放对应签到/签退"
	})
}

func loadScheduledSession(ctx context.Context, entry clubScheduleEntry) (*storage.Session, error) {
	store, err := storage.GetStore()
	if err != nil || store == nil || !store.Enabled() {
		return nil, fmt.Errorf("本地登录态不可用，无法执行定时任务")
	}

	if strings.TrimSpace(entry.SessionKey) != "" {
		if session, _, err := store.LoadBySessionKey(ctx, entry.SessionKey); err == nil && session != nil {
			return session, nil
		}
	}
	if entry.StudentID > 0 {
		if session, _, err := store.LoadByStudentID(ctx, entry.StudentID); err == nil && session != nil {
			return session, nil
		}
	}
	return nil, fmt.Errorf("未找到可用登录态，请重新登录后开启定时")
}

func dueClubProbes(now time.Time, queryDate string, activities []unirunapi.ClubInfo, entry clubScheduleEntry) []clubDueProbe {
	probes := make([]clubDueProbe, 0, 2)
	for _, activity := range activities {
		activityID := activity.ClubActivityID
		if activityID <= 0 {
			continue
		}

		signInKey := clubProbeKey(queryDate, activityID, "1")
		if entry.LastSignInKey != signInKey {
			if startAt, ok := parseClubEventTime(queryDate, activity.StartTime, now.Location()); ok && inClubProbeWindow(now, startAt) {
				probes = append(probes, clubDueProbe{ActivityID: activityID, SignType: "1", Key: signInKey})
			}
		}

		signBackKey := clubProbeKey(queryDate, activityID, "2")
		if entry.LastSignBackKey != signBackKey {
			if endAt, ok := parseClubEventTime(queryDate, activity.EndTime, now.Location()); ok && inClubProbeWindow(now, endAt) {
				probes = append(probes, clubDueProbe{ActivityID: activityID, SignType: "2", Key: signBackKey})
			}
		}
	}
	return probes
}

func clubProbeKey(queryDate string, activityID int64, signType string) string {
	return queryDate + ":" + strconv.FormatInt(activityID, 10) + ":" + signType
}

func parseClubEventTime(queryDate, raw string, loc *time.Location) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "--:--" {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}

	fullLayouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
	}
	for _, layout := range fullLayouts {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed, true
		}
	}

	date := strings.TrimSpace(queryDate)
	if date == "" {
		date = time.Now().In(loc).Format("2006-01-02")
	}
	timeLayouts := []string{"15:04:05", "15:04"}
	for _, layout := range timeLayouts {
		if parsed, err := time.ParseInLocation("2006-01-02 "+layout, date+" "+value, loc); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func inClubProbeWindow(now, eventAt time.Time) bool {
	windowStart := eventAt.Add(-clubProbeLead)
	windowEnd := eventAt.Add(clubProbeLead) // 事件前后各 10 分钟，防止服务端延迟开放签到/签退
	return !now.Before(windowStart) && !now.After(windowEnd)
}

func clubSignTypeName(signType string) string {
	if signType == "2" {
		return "签退"
	}
	return "签到"
}
