package main

import (
	"context"
	_ "embed" // 必须匿名引入 embed 包
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"

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
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Action   string `json:"action"`
}

type ActionResponse struct {
	Code     int         `json:"code"`
	Msg      string      `json:"msg"`
	Response interface{} `json:"response"`
}

func getCredentialsFromEvent(event TimerEvent) (string, string, bool) {
	raw := strings.TrimSpace(event.Body)
	if raw == "" {
		raw = strings.TrimSpace(event.Message)
	}
	if raw == "" {
		return "", "", false
	}
	var payload CredentialsPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", "", false
	}
	if payload.Phone == "" || payload.Password == "" {
		return "", "", false
	}
	return payload.Phone, payload.Password, true
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
	phone, password, ok := getCredentialsFromEvent(event)
	if !ok {
		var err error
		phone, password, err = getCredentials()
		if err != nil {
			return "", err
		}
	}
	action := getActionFromEvent(event)
	switch action {
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
	default:
		payload, _ := json.Marshal(ActionResponse{
			Code: 40000,
			Msg:  fmt.Sprintf("不支持的 action: %s", action),
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

	cloudfunction.Start(HandleRequest)
}
