package service

import (
	"context"
	"fmt"

	api "autorun-go/unirunapi"
)

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
	loginInfo, err := api.Login(
		input.Phone, input.Password, input.AppVersion, input.Brand,
		input.DeviceToken, input.DeviceType, input.MobileType, input.SysVersion,
	)
	if err != nil {
		return api.Response[map[string]any]{Code: 50000, Msg: fmt.Sprintf("登录失败: %v", err)}, err
	}
	token := loginInfo.Token
	studentId := loginInfo.StudentID

	// 2. 是否已有待签到项目（安卓：有则直接返回）
	tfInfo, err := api.GetSignInTf(token, studentId)
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
		StudentId:  studentId,
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
