package service

import (
	"context"
	"fmt"
	"time"

	"autorun-go/track"
	api "autorun-go/unirunapi"
)

type RunInput struct {
	Phone       string
	Password    string
	RunDistance int64
	RunTime     int

	AppVersion  string
	Brand       string
	MobileType  string
	SysVersion  string
	DeviceToken string
	DeviceType  string
}

type RunResult struct {
	RawResponse string
	UserID      int64
	SchoolID    int64
}

func SubmitRun(ctx context.Context, locations []track.Location, input RunInput) (RunResult, error) {
	_ = ctx

	loginInfo, err := api.Login(
		input.Phone,
		input.Password,
		input.AppVersion,
		input.Brand,
		input.DeviceToken,
		input.DeviceType,
		input.MobileType,
		input.SysVersion,
	)
	if err != nil {
		return RunResult{}, fmt.Errorf("登录失败: %v", err)
	}

	runStandard, err := api.GetRunStandard(loginInfo.Token, loginInfo.SchoolID)
	if err != nil {
		return RunResult{}, fmt.Errorf("获取跑步标准失败: %v", err)
	}

	bounds, err := api.GetSchoolBound(loginInfo.Token, loginInfo.SchoolID)
	if err != nil {
		return RunResult{}, fmt.Errorf("获取学校围栏失败: %v", err)
	}

	realityTrackPoints := "00.000,00.000--"
	if len(bounds) > 0 {
		realityTrackPoints = bounds[0].SiteBound + "--"
	}

	trackPointsStr := track.Gen(input.RunDistance, locations)

	recordDate := time.Now().Format("2006-01-02")
	recordBody := api.NewRecordBody{
		AppVersions:        input.AppVersion,
		Brand:              input.Brand,
		MobileType:         input.MobileType,
		SysVersions:        input.SysVersion,
		RunDistance:        input.RunDistance,
		RunTime:            input.RunTime,
		UserID:             loginInfo.UserID,
		YearSemester:       runStandard.SemesterYear,
		RecordDate:         recordDate,
		RealityTrackPoints: realityTrackPoints,
		TrackPoints:        trackPointsStr,
		VocalStatus:        "1",
		InnerSchool:        "1",
		DistanceTimeStatus: "1",
		AgainRunStatus:     "0",
	}

	result, err := api.RecordNew(loginInfo.Token, recordBody)
	if err != nil {
		return RunResult{}, fmt.Errorf("提交打卡失败: %v", err)
	}

	return RunResult{
		RawResponse: result,
		UserID:      loginInfo.UserID,
		SchoolID:    loginInfo.SchoolID,
	}, nil
}
