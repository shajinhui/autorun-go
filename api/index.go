package handler

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"time"

	"autorun-go/service"
	"autorun-go/storage"
	"autorun-go/track"
	api "autorun-go/unirunapi"
)

//go:embed map.json
var embeddedMapJSON []byte

const (
	appVersion = "1.8.3"
	brand      = "Xiaomi"
	mobileType = "Mi 11"
	sysVersion = "Android 11"
	deviceType = "1"
)

type credentialsPayload struct {
	Phone        string `json:"phone"`
	Password     string `json:"password"`
	Action       string `json:"action"`
	AdminToken   string `json:"adminToken"`
	QueryDate    string `json:"queryDate"`
	ActivityID   int64  `json:"activityId"`
	StudentID    int64  `json:"studentId"`
	ForceRefresh bool   `json:"forceRefresh"`
}

type actionResponse struct {
	Code     int         `json:"code"`
	Msg      string      `json:"msg"`
	Response interface{} `json:"response"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{"service": "autorun-go", "runtime": "vercel-go"}})
		return
	}

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, actionResponse{Code: 40500, Msg: "method not allowed", Response: map[string]any{}})
		return
	}

	var payload credentialsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, actionResponse{Code: 40000, Msg: "请求体格式错误", Response: map[string]any{}})
		return
	}
	defer r.Body.Close()

	action := strings.ToLower(strings.TrimSpace(payload.Action))
	if action == "" {
		action = "run"
	}

	phone, password, err := resolveCredentials(payload, action)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, actionResponse{Code: 40100, Msg: err.Error(), Response: map[string]any{}})
		return
	}

	ctx := r.Context()
	res, status, err := handleAction(ctx, action, payload, phone, password)
	if err != nil {
		writeJSON(w, status, actionResponse{Code: 50000, Msg: err.Error(), Response: map[string]any{}})
		return
	}
	writeJSON(w, status, res)
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func resolveCredentials(payload credentialsPayload, action string) (string, string, error) {
	if payload.Phone != "" && payload.Password != "" {
		return payload.Phone, payload.Password, nil
	}

	adminToken := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
	if adminToken != "" && payload.AdminToken != "" && payload.AdminToken == adminToken {
		phone := os.Getenv("RUN_PHONE")
		password := os.Getenv("RUN_PASSWORD")
		if phone == "" || password == "" {
			return "", "", fmt.Errorf("缺少环境变量 RUN_PHONE 或 RUN_PASSWORD")
		}
		return phone, password, nil
	}

	if action == "login" || action == "run" || action == "club" {
		return "", "", fmt.Errorf("缺少手机号或密码（或管理员口令无效）")
	}
	return "", "", nil
}

func handleAction(ctx context.Context, action string, payload credentialsPayload, phone, password string) (actionResponse, int, error) {
	switch action {
	case "login":
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, true)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, fmt.Errorf("登录失败: %v", err)
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"userId":    loginInfo.UserID,
			"studentId": loginInfo.StudentID,
			"schoolId":  loginInfo.SchoolID,
			"tokenSrc":  tokenSource,
		}}, http.StatusOK, nil

	case "run":
		if phone == "" || password == "" {
			return actionResponse{}, http.StatusUnauthorized, fmt.Errorf("run 需要手机号和密码")
		}
		locations, err := loadTrackMap()
		if err != nil {
			return actionResponse{}, http.StatusInternalServerError, err
		}
		runDistance := int64(4675 + rand.IntN(500))
		runTime := 31 + rand.IntN(5)
		result, err := service.SubmitRun(ctx, locations, service.RunInput{
			Phone:       phone,
			Password:    password,
			RunDistance: runDistance,
			RunTime:     runTime,
			AppVersion:  appVersion,
			Brand:       brand,
			MobileType:  mobileType,
			SysVersion:  sysVersion,
			DeviceToken: "",
			DeviceType:  deviceType,
		})
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"rawResponse": result.RawResponse,
			"userId":      result.UserID,
			"schoolId":    result.SchoolID,
		}}, http.StatusOK, nil

	case "club":
		if phone == "" || password == "" {
			return actionResponse{}, http.StatusUnauthorized, fmt.Errorf("club 需要手机号和密码")
		}
		res, err := service.AutoClubService(ctx, service.ClubInput{
			Phone:       phone,
			Password:    password,
			AppVersion:  appVersion,
			Brand:       brand,
			MobileType:  mobileType,
			SysVersion:  sysVersion,
			DeviceToken: "",
			DeviceType:  deviceType,
		})
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: res.Code, Msg: res.Msg, Response: res.Response}, http.StatusOK, nil

	case "club_data":
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, false)
		if err != nil {
			return actionResponse{}, http.StatusUnauthorized, err
		}
		queryDate := strings.TrimSpace(payload.QueryDate)
		if queryDate == "" {
			queryDate = time.Now().Format("2006-01-02")
		}
		tfInfo, err := api.GetSignInTf(loginInfo.Token, loginInfo.StudentID)
		if err != nil {
			refreshed, refreshErr := retryWithFreshLogin(ctx, phone, password, loginInfo, payload)
			if refreshErr == nil {
				loginInfo = refreshed
				tokenSource = "relogin"
				tfInfo, err = api.GetSignInTf(loginInfo.Token, loginInfo.StudentID)
			}
		}
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		activities, err := api.GetClubActivityList(loginInfo.Token, loginInfo.StudentID, queryDate, loginInfo.SchoolID)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		joinProgress, err := api.GetClubJoinNum(loginInfo.Token, loginInfo.SchoolID, loginInfo.StudentID)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		topThree, err := api.GetSchoolActivityTopThree(loginInfo.Token)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"queryDate":     queryDate,
			"signTask":      tfInfo,
			"activities":    activities,
			"joinProgress":  joinProgress,
			"topThree":      topThree,
			"tokenSrc":      tokenSource,
		}}, http.StatusOK, nil

	case "run_info":
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, false)
		if err != nil {
			return actionResponse{}, http.StatusUnauthorized, err
		}
		runStandard, err := api.GetRunStandard(loginInfo.Token, loginInfo.SchoolID)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		runInfo, err := api.GetRunInfo(loginInfo.Token, loginInfo.UserID, runStandard.SemesterYear)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"runStandard": runStandard,
			"runInfo":     runInfo,
			"tokenSrc":    tokenSource,
		}}, http.StatusOK, nil

	case "club_join_num":
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, false)
		if err != nil {
			return actionResponse{}, http.StatusUnauthorized, err
		}
		joinProgress, err := api.GetClubJoinNum(loginInfo.Token, loginInfo.SchoolID, loginInfo.StudentID)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"joinProgress": joinProgress,
			"tokenSrc":     tokenSource,
		}}, http.StatusOK, nil

	case "club_top_three":
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, false)
		if err != nil {
			return actionResponse{}, http.StatusUnauthorized, err
		}
		topThree, err := api.GetSchoolActivityTopThree(loginInfo.Token)
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"topThree": topThree,
			"tokenSrc": tokenSource,
		}}, http.StatusOK, nil

	case "club_join":
		if payload.ActivityID <= 0 {
			return actionResponse{}, http.StatusBadRequest, fmt.Errorf("缺少 activityId")
		}
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, false)
		if err != nil {
			return actionResponse{}, http.StatusUnauthorized, err
		}
		rawResp, err := api.JoinClubActivity(loginInfo.Token, loginInfo.StudentID, payload.ActivityID)
		if err != nil {
			refreshed, refreshErr := retryWithFreshLogin(ctx, phone, password, loginInfo, payload)
			if refreshErr == nil {
				loginInfo = refreshed
				tokenSource = "relogin"
				rawResp, err = api.JoinClubActivity(loginInfo.Token, loginInfo.StudentID, payload.ActivityID)
			}
		}
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"rawResponse": rawResp,
			"tokenSrc":    tokenSource,
		}}, http.StatusOK, nil

	case "club_cancel":
		if payload.ActivityID <= 0 {
			return actionResponse{}, http.StatusBadRequest, fmt.Errorf("缺少 activityId")
		}
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, false)
		if err != nil {
			return actionResponse{}, http.StatusUnauthorized, err
		}
		rawResp, err := api.CancelClubActivity(loginInfo.Token, loginInfo.StudentID, payload.ActivityID)
		if err != nil {
			refreshed, refreshErr := retryWithFreshLogin(ctx, phone, password, loginInfo, payload)
			if refreshErr == nil {
				loginInfo = refreshed
				tokenSource = "relogin"
				rawResp, err = api.CancelClubActivity(loginInfo.Token, loginInfo.StudentID, payload.ActivityID)
			}
		}
		if err != nil {
			return actionResponse{}, http.StatusBadGateway, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"rawResponse": rawResp,
			"tokenSrc":    tokenSource,
		}}, http.StatusOK, nil

	case "session_bootstrap":
		loginInfo, tokenSource, err := loginWithCachePolicy(ctx, phone, password, payload, false)
		if err != nil {
			return actionResponse{}, http.StatusUnauthorized, err
		}
		return actionResponse{Code: 10000, Msg: "ok", Response: map[string]any{
			"userId":    loginInfo.UserID,
			"studentId": loginInfo.StudentID,
			"schoolId":  loginInfo.SchoolID,
			"tokenSrc":  tokenSource,
		}}, http.StatusOK, nil

	default:
		return actionResponse{Code: 40000, Msg: fmt.Sprintf("不支持的 action: %s", action), Response: map[string]any{}}, http.StatusBadRequest, nil
	}
}

func loginWithCachePolicy(ctx context.Context, phone, password string, payload credentialsPayload, alwaysFresh bool) (api.LoginResult, string, error) {
	if !alwaysFresh && !payload.ForceRefresh {
		store, err := storage.GetStore()
		if err == nil && store != nil && store.Enabled() {
			if payload.StudentID > 0 {
				session, source, loadErr := store.LoadByStudentID(ctx, payload.StudentID)
				if loadErr == nil && session != nil {
					return api.LoginResult{
						Token:     session.Token,
						UserID:    session.UserID,
						StudentID: session.StudentID,
						SchoolID:  session.SchoolID,
					}, source, nil
				}
			}
			if phone != "" {
				session, source, loadErr := store.LoadByPhone(ctx, phone)
				if loadErr == nil && session != nil {
					return api.LoginResult{
						Token:     session.Token,
						UserID:    session.UserID,
						StudentID: session.StudentID,
						SchoolID:  session.SchoolID,
					}, source, nil
				}
			}
		}
	}

	if phone == "" || password == "" {
		return api.LoginResult{}, "", fmt.Errorf("缺少可用登录态，且未提供手机号/密码")
	}
	loginInfo, err := api.Login(phone, password, appVersion, brand, "", deviceType, mobileType, sysVersion)
	if err != nil {
		return api.LoginResult{}, "", err
	}
	persistLogin(ctx, phone, loginInfo)
	return loginInfo, "login", nil
}

func retryWithFreshLogin(ctx context.Context, phone, password string, current api.LoginResult, payload credentialsPayload) (api.LoginResult, error) {
	if phone == "" || password == "" {
		return current, fmt.Errorf("token 已失效且无可用账号密码刷新")
	}
	loginInfo, err := api.Login(phone, password, appVersion, brand, "", deviceType, mobileType, sysVersion)
	if err != nil {
		return current, err
	}
	persistLogin(ctx, phone, loginInfo)
	return loginInfo, nil
}

func persistLogin(ctx context.Context, phone string, loginInfo api.LoginResult) {
	store, err := storage.GetStore()
	if err != nil || store == nil || !store.Enabled() {
		return
	}
	_ = store.Save(ctx, phone, storage.Session{
		Token:     loginInfo.Token,
		UserID:    loginInfo.UserID,
		StudentID: loginInfo.StudentID,
		SchoolID:  loginInfo.SchoolID,
		UpdatedAt: time.Now(),
	})
}

func loadTrackMap() ([]track.Location, error) {
	if len(embeddedMapJSON) > 0 {
		var locations []track.Location
		if err := json.Unmarshal(embeddedMapJSON, &locations); err == nil && len(locations) > 0 {
			return locations, nil
		}
	}

	candidates := []string{"api/map.json", "map.json", "./map.json", "/var/task/map.json"}
	var lastErr error
	for _, path := range candidates {
		bytes, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		var locations []track.Location
		if err := json.Unmarshal(bytes, &locations); err != nil {
			return nil, fmt.Errorf("解析 map.json 失败: %v", err)
		}
		if len(locations) == 0 {
			return nil, fmt.Errorf("map.json 为空")
		}
		return locations, nil
	}
	return nil, fmt.Errorf("读取 map.json 失败: %v", lastErr)
}
