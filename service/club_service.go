package service

import (
	"context"
	"fmt"

	"autorun-go/api"
)

// ClubAPI defines the minimal surface used by the club flow.
// Provide your own mock implementation for compliance/testing.
// type ClubAPI interface {
// 	Login(phone, password, appVersion, brand, deviceToken, deviceType, mobileType, sysVersion string) (string, int64, int64, error)
// 	GetClubActivityList(token string, studentId int64, date string, schoolId int64) ([]api.ClubInfo, error)
// 	JoinClubActivity(token string, studentId int64, activityId int64) (string, error)
// 	GetSignInTf(token string, studentId int64) (*api.SignInTf, error)
// 	SignInOrSignBack(token string, body api.SignInOrSignBackBody) (string, error)
// }

//NoopClubAPI 是一个安全的默认值，强制调用者注入模拟。
// type NoopClubAPI struct{}

// func (NoopClubAPI) Login(string, string, string, string, string, string, string, string) (string, int64, int64, error) {
// 	return "", 0, 0, fmt.Errorf("club api not configured: please inject a mock ClubAPI")
// }
// func (NoopClubAPI) GetClubActivityList(string, int64, string, int64) ([]api.ClubInfo, error) {
// 	return nil, fmt.Errorf("club api not configured: please inject a mock ClubAPI")
// }
// func (NoopClubAPI) JoinClubActivity(string, int64, int64) (string, error) {
// 	return "", fmt.Errorf("club api not configured: please inject a mock ClubAPI")
// }
// func (NoopClubAPI) GetSignInTf(string, int64) (*api.SignInTf, error) {
// 	return nil, fmt.Errorf("club api not configured: please inject a mock ClubAPI")
// }
// func (NoopClubAPI) SignInOrSignBack(string, api.SignInOrSignBackBody) (string, error) {
// 	return "", fmt.Errorf("club api not configured: please inject a mock ClubAPI")
// }

type ClubInput struct {
	Phone       string
	Password    string
	AppVersion  string
	Brand       string
	MobileType  string
	SysVersion  string
	DeviceToken string
	DeviceType  string

	// 安卓逻辑里的筛选字段
	Location string
	Keyword  string
}

// AutoClubService mirrors Android club logic but relies on an injected ClubAPI.
// It returns a Response to align with the Android flow.
func AutoClubService(ctx context.Context, input ClubInput) (api.Response[map[string]any], error) {
	_ = ctx

	

	// 1. 登录获取 Token 和 ID
	token, userID, _, err := api.Login(
		input.Phone, input.Password, input.AppVersion, input.Brand,
		input.DeviceToken, input.DeviceType, input.MobileType, input.SysVersion,
	)
	if err != nil {
		return api.Response[map[string]any]{Code: 50000, Msg: fmt.Sprintf("登录失败: %v", err)}, err
	}

	// 2. 是否已有待签到项目（安卓：有则直接返回）
	tfInfo, err := api.GetSignInTf(token, userID)
	if err != nil {
		return api.Response[map[string]any]{Code: 50000, Msg: fmt.Sprintf("获取签到信息失败: %v", err)}, err
	}
	if tfInfo == nil || isEmptySignInTf(tfInfo) {
		fmt.Println("没有可签到项目，继续后续流程")
		return api.Response[map[string]any]{Code: 10000, Msg: "没有可签到项目，继续后续流程", Response: map[string]any{}}, nil
	}
	if tfInfo != nil && (tfInfo.SignInStatus == "1" || tfInfo.SignBackStatus == "1") {
		fmt.Printf("可签到项目: activityId=%d name=%s start=%s end=%s signStatus=%s signInStatus=%s signBackStatus=%s\n",
			tfInfo.ActivityId,
			tfInfo.ActivityName,
			tfInfo.StartTime,
			tfInfo.EndTime,
			tfInfo.SignStatus,
			tfInfo.SignInStatus,
			tfInfo.SignBackStatus,
		)
	} 

	// 3. 查询未来活动（安卓：今天 + 6 天）
	// queryDate := time.Now().Add(6 * 24 * time.Hour).Format("2006-01-02")
	// activities, err := api.GetClubActivityList(token, userID, queryDate, schoolID)
	// if err != nil {
	// 	return api.Response[map[string]any]{Code: 50000, Msg: fmt.Sprintf("获取活动列表失败: %v", err)}, err
	// }

	// 4. 筛选可加入活动（未满员）
	// available := make([]api.ClubInfo, 0, len(activities))
	// for _, act := range activities {
	// 	if act.SignInStudent < act.MaxStudent {
	// 		available = append(available, act)
	// 	}
	// }
	// if len(available) == 0 {
	// 	return api.Response[map[string]any]{Code: 10000, Msg: "没有可以参加的俱乐部", Response: map[string]any{}}, nil
	// }

	// 5. 按 location + keyword 筛选
	// filtered := make([]api.ClubInfo, 0, len(available))
	// for _, act := range available {
	// 	ok := true
	// 	if input.Location != "" {
	// 		ok = ok && strings.Contains(act.ActivityName, input.Location)
	// 	}
	// 	if input.Keyword != "" {
	// 		ok = ok && strings.Contains(act.ActivityName, input.Keyword)
	// 	}
	// 	if ok {
	// 		filtered = append(filtered, act)
	// 	}
	// }
	// if len(filtered) == 0 {
	// 	return api.Response[map[string]any]{
	// 		Code: 10000,
	// 		Msg:  fmt.Sprintf("没有找到可加入的俱乐部\n你的校区：%s\n你的关键词：%s", input.Location, input.Keyword),
	// 		Response: map[string]any{},
	// 	}, nil
	// }

	// target := filtered[0]
	// _, err = api.JoinClubActivity(token, userID, target.ClubActivityID)
	// if err != nil {
	// 	return api.Response[map[string]any]{Code: 50000, Msg: fmt.Sprintf("加入活动失败: %v", err)}, err
	// }

	// 6. 签到/签退逻辑（安卓 signStatus / signInStatus / signBackStatus）
	// tfInfo, err = api.GetSignInTf(token, userID)
	// if err != nil {
	// 	return api.Response[map[string]any]{Code: 50000, Msg: fmt.Sprintf("获取签到信息失败: %v", err)}, err
	// }
	// if tfInfo == nil {
	// 	return api.Response[map[string]any]{Code: 10000, Msg: "无可签到项目", Response: map[string]any{}}, nil
	// }

	// if tfInfo.SignInStatus == "1" && tfInfo.SignBackStatus == "1" {
	// 	return api.Response[map[string]any]{Code: 10000, Msg: "已完成签到签退", Response: map[string]any{}}, nil
	// }

	signType := ""
	if tfInfo.SignStatus == "1" {
		signType = "1"
	} else if tfInfo.SignInStatus == "1" && tfInfo.SignStatus == "2" {
		signType = "2"
	} else {
		return api.Response[map[string]any]{Code: 10000, Msg: "非可签到签退状态，或没有可签到项目", Response: map[string]any{}}, nil
	}

	_, err = api.SignInOrSignBack(token, api.SignInOrSignBackBody{
		ActivityId: tfInfo.ActivityId,
		Latitude:   tfInfo.Latitude,
		Longitude:  tfInfo.Longitude,
		SignType:   signType,
		StudentId:  userID,
	})
	if err != nil {
		return api.Response[map[string]any]{Code: 50000, Msg: fmt.Sprintf("签到/签退失败: %v", err)}, err
	}

	return api.Response[map[string]any]{
		Code: 10000,
		Msg:  "ok",
		Response: map[string]any{
			"success":      true,
			"activityName": tfInfo.ActivityName,
		},
	}, nil
}

func isEmptySignInTf(tfInfo *api.SignInTf) bool {
	if tfInfo == nil {
		return true
	}
	isZeroStatus := func(v string) bool { return v == "" || v == "0" }
	return tfInfo.ActivityId == 0 &&
		tfInfo.ActivityName == "" &&
		tfInfo.StartTime == "" &&
		tfInfo.EndTime == "" &&
		tfInfo.Latitude == "" &&
		tfInfo.Longitude == "" &&
		isZeroStatus(tfInfo.SignStatus) &&
		isZeroStatus(tfInfo.SignInStatus) &&
		isZeroStatus(tfInfo.SignBackStatus)
}
