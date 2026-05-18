package api

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	AppKey    = "389885588s0648fa"
	Host      = "https://run-lb.tanmasports.com/"
	userAgent = "okhttp/3.12.0"
)

var upstreamHTTPClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		// The upstream WAF can reject Go's default HTTP/2 client fingerprint from
		// local desktop networks. Keep the transport on HTTP/1.1 for local builds.
		TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
	},
}

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
	StandardID          int64  `json:"standardId,omitempty"`
	SchoolID            int64  `json:"schoolId,omitempty"`
	BoyOnceTimeMin      int64  `json:"boyOnceTimeMin,omitempty"`
	BoyOnceTimeMax      int64  `json:"boyOnceTimeMax,omitempty"`
	BoyOnceDistanceMin  int64  `json:"boyOnceDistanceMin,omitempty"`
	BoyOnceDistanceMax  int64  `json:"boyOnceDistanceMax,omitempty"`
	BoyAllRunDistance   int64  `json:"boyAllRunDistance,omitempty"`
	BoyAllRunTime       int64  `json:"boyAllRunTime,omitempty"`
	GirlOnceTimeMin     int64  `json:"girlOnceTimeMin,omitempty"`
	GirlOnceTimeMax     int64  `json:"girlOnceTimeMax,omitempty"`
	GirlOnceDistanceMin int64  `json:"girlOnceDistanceMin,omitempty"`
	GirlOnceDistanceMax int64  `json:"girlOnceDistanceMax,omitempty"`
	GirlAllRunDistance  int64  `json:"girlAllRunDistance,omitempty"`
	GirlAllRunTime      int64  `json:"girlAllRunTime,omitempty"`
	FirstSemesterStart  string `json:"firstSemesterDateStart,omitempty"`
	FirstSemesterEnd    string `json:"firstSemesterDateEnd,omitempty"`
	SecondSemesterStart string `json:"secondSemesterDateStart,omitempty"`
	SecondSemesterEnd   string `json:"secondSemesterDateEnd,omitempty"`
	InstanceSemester    string `json:"instanceSemester,omitempty"`
	SemesterYear        string `json:"semesterYear"`
	BoyRunSpeed         int64  `json:"boyRunSpeed,omitempty"`
	GirlRunSpeed        int64  `json:"girlRunSpeed,omitempty"`
	BoyMaxSpeed         int64  `json:"boyMaxSpeed,omitempty"`
	BoyMinSpeed         int64  `json:"boyMinSpeed,omitempty"`
	GirlMaxSpeed        int64  `json:"girlMaxSpeed,omitempty"`
	GirlMinSpeed        int64  `json:"girlMinSpeed,omitempty"`
	EffectiveRangeType  string `json:"effectiveRangeType,omitempty"`
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

func applyCommonHeaders(req *http.Request, sign, token string) {
	req.Header.Set("sign", sign)
	req.Header.Set("appkey", AppKey)
	req.Header.Set("token", token)
	req.Header.Set("User-Agent", userAgent)
	if req.Method != http.MethodGet && req.Body != nil {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}
}

func doUpstream(req *http.Request) ([]byte, error) {
	resp, err := upstreamHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if isHTMLResponse(respBody) {
		return nil, fmt.Errorf("上游接口被风控拦截，请稍后重试或更换网络")
	}
	return respBody, nil
}

func decodeResponse[T any](respBody []byte, result *Response[T], label string) error {
	if err := json.Unmarshal(respBody, result); err != nil {
		if isHTMLResponse(respBody) {
			return fmt.Errorf("%s: 上游接口被风控拦截，请稍后重试或更换网络", label)
		}
		return fmt.Errorf("%s: JSON解析失败: %v, raw=%s", label, err, string(respBody))
	}
	return nil
}

func isHTMLResponse(body []byte) bool {
	trimmed := strings.ToLower(strings.TrimSpace(string(body)))
	compact := strings.ReplaceAll(trimmed, " ", "")
	return strings.HasPrefix(trimmed, "<html") ||
		strings.HasPrefix(trimmed, "<!doctype") ||
		strings.HasPrefix(compact, "<!doctypehtml") ||
		strings.Contains(trimmed, "your request has been blocked") ||
		strings.Contains(trimmed, "errors.aliyun.com") ||
		strings.Contains(trimmed, "block_traceid")
}

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
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return LoginResult{}, err
	}

	// 解析泛型 JSON
	var result Response[UserInfo]
	if err := decodeResponse(respBody, &result, "登录失败"); err != nil {
		return LoginResult{}, err
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
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return nil, err
	}

	var result Response[[]SchoolBound]
	if err := decodeResponse(respBody, &result, "获取围栏失败"); err != nil {
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
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return nil, err
	}

	var result Response[RunStandard]
	if err := decodeResponse(respBody, &result, "获取标准失败"); err != nil {
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
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return "", err
	}

	// 校验业务响应码
	var result Response[map[string]any]
	if err := decodeResponse(respBody, &result, "提交失败"); err != nil {
		return "", err
	}
	if result.Code != 10000 {
		return "", fmt.Errorf("提交失败: %s", result.Msg)
	}

	return string(respBody), nil
}

// GetSignInTf 获取签到坐标与状态
func GetSignInTf(token string, studentId int64) (*SignInTf, error) {
	studentIdStr := strconv.FormatInt(studentId, 10)
	params := map[string]string{
		"studentId": studentIdStr,
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/clubactivity/getSignInTf?studentId=" + studentIdStr
	req, _ := http.NewRequest("GET", apiURL, nil)
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return nil, err
	}
	var result Response[SignInTf]
	if err := decodeResponse(respBody, &result, "获取签到信息失败"); err != nil {
		return nil, err
	}
	if result.Code != 10000 {
		return nil, fmt.Errorf("获取签到信息失败: %s", result.Msg)
	}
	return &result.Response, nil
}

// SignInOrSignBack 提交签到/签退
func SignInOrSignBack(token string, body SignInOrSignBackBody) (string, error) {
	bodyBytes, _ := json.Marshal(body)
	sign := GenerateSign(nil, string(bodyBytes)) // POST 请求，将 Body 进行签名

	req, _ := http.NewRequest("POST", Host+"v1/clubactivity/signInOrSignBack", bytes.NewBuffer(bodyBytes))
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return "", err
	}
	var result Response[map[string]any]
	if err := decodeResponse(respBody, &result, "签到/签退失败"); err != nil {
		return "", err
	}
	if result.Code != 10000 {
		return "", fmt.Errorf("签到/签退失败: %s", result.Msg)
	}
	return string(respBody), nil
}

// GetClubActivityList 获取活动列表
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
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return nil, err
	}
	var result Response[[]ClubInfo]
	if err := decodeResponse(respBody, &result, "查询活动失败"); err != nil {
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
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return "", err
	}
	var result Response[map[string]any]
	if err := decodeResponse(respBody, &result, "加入俱乐部失败"); err != nil {
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
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return "", err
	}
	var result Response[any]
	if err := decodeResponse(respBody, &result, "取消报名失败"); err != nil {
		return "", err
	}
	if result.Code != 10000 {
		return "", fmt.Errorf("取消报名失败: %s", result.Msg)
	}
	return string(respBody), nil
}

// GetRunInfo 查询当前学期跑步统计信息
func GetRunInfo(token string, userId int64, yearSemester string) (*RunInfo, error) {
	userIdStr := strconv.FormatInt(userId, 10)
	params := map[string]string{
		"userId":       userIdStr,
		"yearSemester": yearSemester,
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/unirun/query/runInfo?userId=" + userIdStr + "&yearSemester=" + url.QueryEscape(yearSemester)
	req, _ := http.NewRequest("GET", apiURL, nil)
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return nil, err
	}
	var result Response[RunInfo]
	if err := decodeResponse(respBody, &result, "查询跑步信息失败"); err != nil {
		return nil, err
	}
	if result.Code != 10000 {
		return nil, fmt.Errorf("查询跑步信息失败: %s", result.Msg)
	}
	return &result.Response, nil
}

// GetClubJoinNum 查询俱乐部参与次数与目标次数
func GetClubJoinNum(token string, schoolId int64, studentId int64) (*ClubJoinNum, error) {
	schoolIdStr := strconv.FormatInt(schoolId, 10)
	studentIdStr := strconv.FormatInt(studentId, 10)
	params := map[string]string{
		"schoolId":  schoolIdStr,
		"studentId": studentIdStr,
	}
	sign := GenerateSign(params, "")

	apiURL := Host + "v1/clubactivity/getJoinNum?schoolId=" + schoolIdStr + "&studentId=" + studentIdStr
	req, _ := http.NewRequest("GET", apiURL, nil)
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return nil, err
	}
	var result Response[ClubJoinNum]
	if err := decodeResponse(respBody, &result, "查询俱乐部参与进度失败"); err != nil {
		return nil, err
	}
	if result.Code != 10000 {
		return nil, fmt.Errorf("查询俱乐部参与进度失败: %s", result.Msg)
	}
	return &result.Response, nil
}

// GetSchoolActivityTopThree 查询学校俱乐部推荐活动（Top3）
func GetSchoolActivityTopThree(token string) ([]ClubTopActivity, error) {
	sign := GenerateSign(nil, "")

	apiURL := Host + "v1/clubactivity/querySchoolActivityTopThree"
	req, _ := http.NewRequest("GET", apiURL, nil)
	applyCommonHeaders(req, sign, token)

	respBody, err := doUpstream(req)
	if err != nil {
		return nil, err
	}
	var result Response[[]ClubTopActivity]
	if err := decodeResponse(respBody, &result, "查询俱乐部推荐活动失败"); err != nil {
		return nil, err
	}
	if result.Code != 10000 {
		return nil, fmt.Errorf("查询俱乐部推荐活动失败: %s", result.Msg)
	}
	return result.Response, nil
}
