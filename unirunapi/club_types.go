package api

// ClubInfo mirrors the club activity fields referenced by the service layer.
// This file only defines types; mock or real implementations can live elsewhere.
type ClubInfo struct {
	ClubActivityID   int64  `json:"clubActivityId"`
	ActivityName     string `json:"activityName"`
	SignInStudent    int64  `json:"signInStudent"`
	MaxStudent       int64  `json:"maxStudent"`
	CancelSign       string `json:"cancelSign"`
	StartTime        string `json:"startTime"`
	EndTime          string `json:"endTime"`
	AddressDetail    string `json:"addressDetail,omitempty"`
	ClubIntroduction string `json:"clubIntroduction,omitempty"`
	TeacherName      string `json:"teacherName,omitempty"`
	OptionStatus     string `json:"optionStatus,omitempty"`
	FullActivity     string `json:"fullActivity,omitempty"`
	YearSemester     int64  `json:"yearSemester,omitempty"`
	ActivityItemId   int64  `json:"activityItemId,omitempty"`
	SignStatus       string `json:"signStatus,omitempty"`
}

// SignInTf mirrors the Android response shape for club sign-in task.
type SignInTf struct {
	ActivityId        int64  `json:"activityId"`
	ActivityName      string `json:"activityName,omitempty"`
	ActivityType      string `json:"activityType,omitempty"`
	Address           string `json:"address,omitempty"`
	ContinueTime      int    `json:"continueTime,omitempty"`
	StartTime         string `json:"startTime,omitempty"`
	EndTime           string `json:"endTime,omitempty"`
	Longitude         string `json:"longitude"`
	Latitude          string `json:"latitude"`
	SignBackLimitTime int    `json:"signBackLimitTime,omitempty"`
	SignBackStatus    string `json:"signBackStatus"`
	SignInStatus      string `json:"signInStatus"`
	SignInTime        string `json:"signInTime,omitempty"`
	SignStatus        string `json:"signStatus"`
}

// SignInOrSignBackBody is the payload used for sign-in / sign-back requests.
type SignInOrSignBackBody struct {
	ActivityId int64  `json:"activityId"`
	Latitude   string `json:"latitude"`
	Longitude  string `json:"longitude"`
	SignType   string `json:"signType"`
	StudentId  int64  `json:"studentId"`
}

type RunInfo struct {
	SemesterID       int64 `json:"semesterId"`
	YearSemester     int64 `json:"yearSemester"`
	UserID           int64 `json:"userId"`
	StudentID        int64 `json:"studentId"`
	SchoolID         int64 `json:"schoolId"`
	RunCount         int64 `json:"runCount"`
	RunValidCount    int64 `json:"runValidCount"`
	RunDistance      int64 `json:"runDistance"`
	RunValidDistance int64 `json:"runValidDistance"`
	RunDay           int64 `json:"runDay"`
	RunValidDay      int64 `json:"runValidDay"`
	RunCalorie       int64 `json:"runCalorie"`
	RunValidCalorie  int64 `json:"runValidCalorie"`
	InfoStatus       string `json:"infoStatus"`
	CreateTime       string `json:"createTime"`
}

type ClubJoinNum struct {
	TotalNum    int64 `json:"totalNum"`
	JoinNum     int64 `json:"joinNum"`
	RunTotalNum int64 `json:"runTotalNum"`
	RunJoinNum  int64 `json:"runJoinNum"`
}

type ClubTopActivity struct {
	ClubActivityID   string `json:"clubActivityId"`
	ActivityItemID   string `json:"activityItemId"`
	ItemName         string `json:"itemName"`
	ActivityName     string `json:"activityName"`
	StartTime        string `json:"startTime"`
	EndTime          string `json:"endTime"`
	AddressDetail    string `json:"addressDetail"`
	MaxStudent       string `json:"maxStudent"`
	ApplyStudentCount string `json:"applyStudentCount"`
}
