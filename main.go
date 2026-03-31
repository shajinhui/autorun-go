package main

import (
	"context"
	_ "embed" // 必须匿名引入 embed 包
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	api "autorun-go/unirunapi"
	// 注意：这里的 campus-run-auto 替换为你 go mod init 时使用的真实模块名
	"autorun-go/service"
	"autorun-go/track"

	"github.com/tencentyun/scf-go-lib/cloudfunction"
)

//go:embed map.json
var mapJSONData []byte // 编译时，map.json 的内容会被自动塞进这个变量

// TimerEvent 定义腾讯云定时触发器传入的事件结构
type TimerEvent struct {
	Type        string `json:"Type"`
	TriggerName string `json:"TriggerName"`
	Time        string `json:"Time"`
	Message     string `json:"Message"`
	Body        string `json:"body"`
}

var RunDistance int64 = 4675 // 目标跑步距离（米）
var RunTime = 31             // 目标跑步时间（分钟）
// 你的个人配置信息
const (
	// 伪装设备信息
	AppVersion  = "1.8.3"
	Brand       = "Xiaomi"
	MobileType  = "Mi 11"
	SysVersion  = "Android 11"
	DeviceToken = ""
	DeviceType  = "1"
)

func getCredentials() (string, string, error) {
	phone := os.Getenv("RUN_PHONE")
	password := os.Getenv("RUN_PASSWORD")
	if phone == "" || password == "" {
		return "", "", fmt.Errorf("缺少环境变量 RUN_PHONE 或 RUN_PASSWORD")
	}
	return phone, password, nil
}

type CredentialsPayload struct {
	Phone      string `json:"phone"`
	Password   string `json:"password"`
	Action     string `json:"action"`
	AdminToken string `json:"adminToken"`
	QueryDate  string `json:"queryDate"`
	ActivityID int64  `json:"activityId"`
}

type ActionResponse struct {
	Code     int         `json:"code"`
	Msg      string      `json:"msg"`
	Response interface{} `json:"response"`
}

func getPayloadFromEvent(event TimerEvent) (CredentialsPayload, bool) {
	raw := strings.TrimSpace(event.Body)
	if raw == "" {
		raw = strings.TrimSpace(event.Message)
	}
	if raw == "" {
		return CredentialsPayload{}, false
	}
	var payload CredentialsPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return CredentialsPayload{}, false
	}
	return payload, true
}

func isAdminTokenValid(token string) bool {
	adminToken := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
	if adminToken == "" {
		return false
	}
	return token != "" && token == adminToken
}

func resolveCredentials(event TimerEvent) (string, string, error) {
	payload, ok := getPayloadFromEvent(event)
	if ok && payload.Phone != "" && payload.Password != "" {
		return payload.Phone, payload.Password, nil
	}
	if ok && isAdminTokenValid(payload.AdminToken) {
		return getCredentials()
	}
	return "", "", fmt.Errorf("缺少手机号或密码（或管理员口令无效）")
}

func getActionFromEvent(event TimerEvent) string {
	raw := strings.TrimSpace(event.Body)
	if raw == "" {
		raw = strings.TrimSpace(event.Message)
	}
	if raw == "" {
		return "run"
	}
	var payload CredentialsPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "run"
	}
	if payload.Action == "" {
		return "run"
	}
	return strings.ToLower(payload.Action)
}

func getFullPayload(event TimerEvent) CredentialsPayload {
	payload, ok := getPayloadFromEvent(event)
	if !ok {
		return CredentialsPayload{}
	}
	return payload
}

// HandleRequest 是云函数的入口处理逻辑
func HandleRequest(ctx context.Context, event TimerEvent) (string, error) {
	fmt.Printf("云函数被触发，时间: %s\n", event.Time)

	// 0. 加载地图数据
	fmt.Println(">>> 0. 正在解析地图图纸数据...")
	var locations []track.Location
	if err := json.Unmarshal(mapJSONData, &locations); err != nil {
		return "", fmt.Errorf("解析 map.json 失败: %v", err)
	}

	// 1. 模拟登录获取 Token
	fmt.Println(">>> 1. 开始模拟登录...")
	phone, password, err := resolveCredentials(event)
	if err != nil {
		return "", err
	}
	action := getActionFromEvent(event)
	fullPayload := getFullPayload(event)
	switch action {
	case "login":
		loginInfo, err := api.Login(
			phone,
			password,
			AppVersion,
			Brand,
			DeviceToken,
			DeviceType,
			MobileType,
			SysVersion,
		)
		if err != nil {
			return "", err
		}
		payload, err := json.Marshal(ActionResponse{
			Code: 10000,
			Msg:  "ok",
			Response: map[string]any{
				"userId":    loginInfo.UserID,
				"studentId": loginInfo.StudentID,
				"schoolId":  loginInfo.SchoolID,
			},
		})
		if err != nil {
			return "", err
		}
		return string(payload), nil
	case "run":
		result, err := service.SubmitRun(ctx, locations, service.RunInput{
			Phone:       phone,
			Password:    password,
			RunDistance: RunDistance,
			RunTime:     RunTime,
			AppVersion:  AppVersion,
			Brand:       Brand,
			MobileType:  MobileType,
			SysVersion:  SysVersion,
			DeviceToken: DeviceToken,
			DeviceType:  DeviceType,
		})
		if err != nil {
			return "", err
		}

		payload, err := json.Marshal(ActionResponse{
			Code: 10000,
			Msg:  "ok",
			Response: map[string]any{
				"rawResponse": result.RawResponse,
				"userId":      result.UserID,
				"schoolId":    result.SchoolID,
			},
		})
		if err != nil {
			return "", err
		}
		return string(payload), nil
	case "club":
		// 处理俱乐部相关操作（签到、查询活动等）
		location := ""
		keyword := ""
		res, err := service.AutoClubService(ctx, service.ClubInput{
			Phone:       phone,
			Password:    password,
			AppVersion:  AppVersion,
			Brand:       Brand,
			MobileType:  MobileType,
			SysVersion:  SysVersion,
			DeviceToken: DeviceToken,
			DeviceType:  DeviceType,
			Location:    location,
			Keyword:     keyword,
		})
		if err != nil {
			return "", err
		}
		payload, err := json.Marshal(res)
		if err != nil {
			return "", err
		}
		return string(payload), nil
	case "club_data":
		loginInfo, err := api.Login(
			phone,
			password,
			AppVersion,
			Brand,
			DeviceToken,
			DeviceType,
			MobileType,
			SysVersion,
		)
		if err != nil {
			return "", err
		}
		queryDate := strings.TrimSpace(fullPayload.QueryDate)
		if queryDate == "" {
			queryDate = time.Now().Format("2006-01-02")
		}

		tfInfo, err := api.GetSignInTf(loginInfo.Token, loginInfo.StudentID)
		if err != nil {
			return "", err
		}
		activities, err := api.GetClubActivityList(loginInfo.Token, loginInfo.StudentID, queryDate, loginInfo.SchoolID)
		if err != nil {
			return "", err
		}

		payload, err := json.Marshal(ActionResponse{
			Code: 10000,
			Msg:  "ok",
			Response: map[string]any{
				"queryDate":  queryDate,
				"signTask":   tfInfo,
				"activities": activities,
			},
		})
		if err != nil {
			return "", err
		}
		return string(payload), nil
	case "club_join":
		if fullPayload.ActivityID <= 0 {
			return "", fmt.Errorf("缺少 activityId")
		}
		loginInfo, err := api.Login(
			phone,
			password,
			AppVersion,
			Brand,
			DeviceToken,
			DeviceType,
			MobileType,
			SysVersion,
		)
		if err != nil {
			return "", err
		}
		rawResp, err := api.JoinClubActivity(loginInfo.Token, loginInfo.StudentID, fullPayload.ActivityID)
		if err != nil {
			return "", err
		}
		payload, err := json.Marshal(ActionResponse{
			Code: 10000,
			Msg:  "ok",
			Response: map[string]any{
				"rawResponse": rawResp,
			},
		})
		if err != nil {
			return "", err
		}
		return string(payload), nil
	case "club_cancel":
		if fullPayload.ActivityID <= 0 {
			return "", fmt.Errorf("缺少 activityId")
		}
		loginInfo, err := api.Login(
			phone,
			password,
			AppVersion,
			Brand,
			DeviceToken,
			DeviceType,
			MobileType,
			SysVersion,
		)
		if err != nil {
			return "", err
		}
		rawResp, err := api.CancelClubActivity(loginInfo.Token, loginInfo.StudentID, fullPayload.ActivityID)
		if err != nil {
			return "", err
		}
		payload, err := json.Marshal(ActionResponse{
			Code: 10000,
			Msg:  "ok",
			Response: map[string]any{
				"rawResponse": rawResp,
			},
		})
		if err != nil {
			return "", err
		}
		return string(payload), nil
	default:
		payload, _ := json.Marshal(ActionResponse{
			Code:     40000,
			Msg:      fmt.Sprintf("不支持的 action: %s", action),
			Response: map[string]any{},
		})
		return string(payload), nil
	}
}

func main() {
	// 生成随机的p跑步距离和时间，增加随机性，降低被风控的风险
	randDistance := int64(rand.IntN(500))
	RunDistance += randDistance

	randTime := rand.IntN(5)
	RunTime += randTime

	// 本地测试模式
	if len(os.Args) > 1 && os.Args[1] == "local" {
		ctx := context.Background()
		event := TimerEvent{
			Type: "Timer",
			Time: time.Now().Format(time.RFC3339),
			Body: `{
			"action":"club",
			"phone":"18328488404",
			"password":"Zc20060418"
		}`,
		}

		result, err := HandleRequest(ctx, event)
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("结果: %s\n", result)
		return
	}

	cloudfunction.Start(HandleRequest)
}
