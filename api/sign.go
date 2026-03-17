package api

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

const (
	// AppKey    = "389885588s0648fa"
	AppSecret = "56E39A1658455588885690425C0FD16055A21676"
)

// GenerateSign 完美复刻原 Java 的 SignUtils.get
func GenerateSign(query map[string]string, body string) string {
	var sb strings.Builder

	// 1. 处理 query 参数并按字典序正序排列
	if query != nil && len(query) > 0 {
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys) // 对应 Java 的 TreeSet 自然排序

		for _, k := range keys {
			v := query[k]
			if v != "" {
				sb.WriteString(k)
				sb.WriteString(v)
			}
		}
	}

	// 2. 追加 KEY 和 SECRET
	sb.WriteString(AppKey)
	sb.WriteString(AppSecret)

	// 3. 追加 body
	if body != "" {
		sb.WriteString(body)
	}

	rawStr := sb.String()
	hasReplaced := false // 对应 Java 里的 z2

	// 4. 检查并删除导致异常的特殊字符
	charsToRemove := []string{" ", "~", "!", "(", ")", "'"}
	for _, char := range charsToRemove {
		if strings.Contains(rawStr, char) {
			rawStr = strings.ReplaceAll(rawStr, char, "")
			hasReplaced = true
		}
	}

	var finalSign string

	// 5. 根据是否发生过替换，走不同的 MD5 逻辑
	if hasReplaced {
		// URL 编码
		// 注意：Go 的 url.QueryEscape 会把 '*' 转码为 '%2A'，
		// 而 Java 的 URLEncoder.encode 默认保留 '*'。为了追求 100% 一致，手动将其替换回 '*'
		encodedStr := url.QueryEscape(rawStr)
		encodedStr = strings.ReplaceAll(encodedStr, "%2A", "*")

		// Java 的 URLEncoder 会把空格转为 '+'，但因为前一步空格已经被删了，所以无需处理空格差异

		// 计算 MD5 并转大写
		hash := md5.Sum([]byte(encodedStr))
		md5Str := strings.ToUpper(hex.EncodeToString(hash[:]))

		// 追加神仙硬编码
		finalSign = md5Str + "encodeutf8"
	} else {
		// 未发生替换，直接计算原始字符串的 MD5 并转大写
		hash := md5.Sum([]byte(rawStr))
		finalSign = strings.ToUpper(hex.EncodeToString(hash[:]))
	}

	return finalSign
}
