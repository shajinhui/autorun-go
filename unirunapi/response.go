package api

// Response 对应 Java 的 Response<T>
// 用于解析接口统一响应结构：code / msg / response
type Response[T any] struct {
	Code     int    `json:"code"`
	Msg      string `json:"msg"`
	Response T      `json:"response"`
}
