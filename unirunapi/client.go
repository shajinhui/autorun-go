package api

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

const (
	AppKey    = "389885588s0648fa"
	Host      = "https://run-lb.tanmasports.com/"
	userAgent = "okhttp/3.12.0"
)

// ================= 数据结构定义 =================

type UserInfo struct {
	UserId     int64 `json:"userId"`
	StudentId  int64 `json:"studentId"`
	SchoolId   int64 `json:"schoolId"`
	OauthToken struct {
		Token string `json:"token"`
	} `json:"oauthToken"`
}

type LoginResult struct {
	Token     string
	UserID    int64
	StudentID int64
	SchoolID  int64
}

type SchoolBound struct {
	SiteBound string `json:"siteBound"`
}

type RunStandard struct {
	SemesterYear string `json:"semesterYear"`
}

type NewRecordBody struct {
	AgainRunStatus     string `json:"againRunStatus"`
	AgainRunTime       int    `json:"againRunTime"`
	AppVersions        string `json:"appVersions"`
	Brand              string `json:"brand"`
	MobileType         string `json:"mobileType"`
	SysVersions        string `json:"sysVersions"`
	TrackPoints        string `json:"trackPoints"`
	DistanceTimeStatus string `json:"distanceTimeStatus"`
	InnerSchool        string `json:"innerSchool"`
	RunDistance        int64  `json:"runDistance"`
	RunTime            int    `json:"runTime"`
	UserID             int64  `json:"userId"`
	VocalStatus        string `json:"vocalStatus"`
	YearSemester       string `json:"yearSemester"`
	RecordDate         string `json:"recordDate"`
	RealityTrackPoints string `json:"realityTrackPoints"`
}

// ================= 核心接口实现 =================

// Login 模拟登录，返回统一对象，避免多返回值错位。
func Login(phone, password, appVersion, brand, deviceToken, deviceType, mobileType, sysVersion string) (LoginResult, error) {
	hash := md5.Sum([]byte(password))
	passMD5 := fmt.Sprintf("%x", hash)

	bodyData := map[string]string{
		"appVersion":  appVersion,
		"brand":       brand,
		"deviceToken": deviceToken,
		"deviceType":  deviceType,
		"mobileType":  mobileType,
		"password":    passMD5,
		"sysVersion":  sysVersion,
		"userPhone":   phone,
	}

	bodyBytes, _ := json.Marshal(bodyData)
	sign := GenerateSign(nil, string(bodyBytes))
	token := "" // 登录时没有 token，签名里这个字段留空

	req, _ := http.NewRequest("POST", Host+"v1/auth/login/password", bytes.NewBuffer(bodyBytes))
	req.Header.Set("sign", sign)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("token", token)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return LoginResult{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// 解析泛型 JSON
	var result Response[UserInfo]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return LoginResult{}, fmt.Errorf("JSON解析失败: %v\n原始响应: %s", err, string(respBody))
	}

	if result.Code != 10000 {
		return LoginResult{}, fmt.Errorf("登录失败: %s (响应: %s)", result.Msg, string(respBody))
	}

	return LoginResult{
		Token:     result.Response.OauthToken.Token,
		UserID:    result.Response.UserId,
		StudentID: result.Response.StudentId,
		SchoolID:  result.Response.SchoolId,
	}, nil
}

// GetSchoolBound 获取学校围栏
func GetSchoolBound(token string, schoolId int64) ([]SchoolBound, error) {
	schoolIdStr := strconv.FormatInt(schoolId, 10)

	params := map[string]string{
		"schoolId": schoolIdStr,
	}
	sign := GenerateSign(params, "") // GET请求没有body

	apiURL := Host + "v1/unirun/querySchoolBound?schoolId=" + schoolIdStr
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result Response[[]SchoolBound]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Code != 10000 {
		return nil, fmt.Errorf("获取围栏失败: %s", result.Msg)
	}

	return result.Response, nil
}

// GetRunStandard 获取跑步标准 (主要是为了拿当前学期 YearSemester)
func GetRunStandard(token string, schoolId int64) (*RunStandard, error) {
	schoolIdStr := strconv.FormatInt(schoolId, 10)

	params := map[string]string{
		"schoolId": schoolIdStr,
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/unirun/query/runStandard?schoolId=" + schoolIdStr
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result Response[RunStandard]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Code != 10000 {
		return nil, fmt.Errorf("获取标准失败: %s", result.Msg)
	}

	return &result.Response, nil
}

// RecordNew 提交跑步记录
func RecordNew(token string, body NewRecordBody) (string, error) {
	bodyBytes, _ := json.Marshal(body)
	sign := GenerateSign(nil, string(bodyBytes))

	req, _ := http.NewRequest("POST", Host+"v1/unirun/save/run/record/new", bytes.NewBuffer(bodyBytes))
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// 校验业务响应码
	var result Response[map[string]any]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("响应解析失败: %v, raw=%s", err, string(respBody))
	}
	if result.Code != 10000 {
		return "", fmt.Errorf("提交失败: %s", result.Msg)
	}

	return string(respBody), nil
}

// GetSignInTf 获取签到坐标与状态 (对应 Request.java 的 getSignInTf)
func GetSignInTf(token string, studentId int64) (*SignInTf, error) {
	studentIdStr := strconv.FormatInt(studentId, 10)
	params := map[string]string{
		"studentId": studentIdStr,
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/clubactivity/getSignInTf?studentId=" + studentIdStr
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	// req.Header.Set("User-Agent", UserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result Response[SignInTf]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.Code != 10000 {
		return nil, fmt.Errorf("获取签到信息失败: %s", result.Msg)
	}
	return &result.Response, nil
}

// SignInOrSignBack 提交签到/签退 (对应 Request.java 的 signInOrSignBack)
func SignInOrSignBack(token string, body SignInOrSignBackBody) (string, error) {
	bodyBytes, _ := json.Marshal(body)
	sign := GenerateSign(nil, string(bodyBytes)) // POST 请求，将 Body 进行签名

	req, _ := http.NewRequest("POST", Host+"v1/clubactivity/signInOrSignBack", bytes.NewBuffer(bodyBytes))
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result Response[map[string]any]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if result.Code != 10000 {
		return "", fmt.Errorf("签到/签退失败: %s", result.Msg)
	}
	return string(respBody), nil
}

// GetClubActivityList 获取活动列表 (对应 Request.java 的 getActivityList)
func GetClubActivityList(token string, studentId int64, date string, schoolId int64) ([]ClubInfo, error) {
	studentIdStr := strconv.FormatInt(studentId, 10)
	schoolIdStr := strconv.FormatInt(schoolId, 10)

	params := map[string]string{
		"queryTime": date,
		"studentId": studentIdStr,
		"schoolId":  schoolIdStr,
		"pageNo":    "1",
		"pageSize":  "15",
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/clubactivity/queryActivityList?queryTime=" + url.QueryEscape(date) +
		"&studentId=" + studentIdStr + "&schoolId=" + schoolIdStr + "&pageNo=1&pageSize=15"

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result Response[[]ClubInfo]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.Code != 10000 {
		return nil, fmt.Errorf("查询活动失败: %s", result.Msg)
	}
	return result.Response, nil
}

// JoinClubActivity 报名俱乐部
func JoinClubActivity(token string, studentId int64, activityId int64) (string, error) {
	studentIdStr := strconv.FormatInt(studentId, 10)
	activityIdStr := strconv.FormatInt(activityId, 10)

	params := map[string]string{
		"studentId":  studentIdStr,
		"activityId": activityIdStr,
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/clubactivity/joinClubActivity?studentId=" + studentIdStr + "&activityId=" + activityIdStr
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result Response[map[string]any]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if result.Code != 10000 {
		return "", fmt.Errorf("加入俱乐部失败: %s", result.Msg)
	}
	return string(respBody), nil
}

// CancelClubActivity 取消报名
func CancelClubActivity(token string, studentId int64, activityId int64) (string, error) {
	studentIdStr := strconv.FormatInt(studentId, 10)
	activityIdStr := strconv.FormatInt(activityId, 10)

	params := map[string]string{
		"studentId":  studentIdStr,
		"activityId": activityIdStr,
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/clubactivity/cancelActivity?studentId=" + studentIdStr + "&activityId=" + activityIdStr
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("sign", sign)
	req.Header.Set("token", token)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result Response[any]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if result.Code != 10000 {
		return "", fmt.Errorf("取消报名失败: %s", result.Msg)
	}
	return string(respBody), nil
}
