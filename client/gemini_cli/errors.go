package gemini_cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nomand-zc/token101/provider-client/types/httperror"
)

// NewHTTPError 将 GeminiCLI（Google Code Assist API）渠道的原始 HTTP 响应归一化为 HTTPError。
// 这是 GeminiCLI 渠道将原始 HTTP 响应转换为归一化 HTTPError 的唯一入口。
//
// 渠道特有错误识别规则（优先于通用状态码映射）：
//   - 403 + body 含 "VALIDATION_REQUIRED" → ErrorTypeForbidden（账号需人工重新授权认证）
//   - 429 + body 可解析冷却时长 → ErrorTypeRateLimit，CooldownUntil 设为当前时间加冷却时长
//   - 429 + body 无法解析冷却时长 → ErrorTypeRateLimit，CooldownUntil=nil
//
// 通用状态码映射：
//   - 400 → ErrorTypeBadRequest
//   - 401 → ErrorTypeUnauthorized
//   - 403 → ErrorTypeForbidden
//   - 429 → ErrorTypeRateLimit
//   - 其他非 2xx → ErrorTypeServerError
func NewHTTPError(statusCode int, body []byte) *httperror.HTTPError {
	bodyStr := string(body)

	httpErr := &httperror.HTTPError{
		RawStatusCode: statusCode,
		RawBody:       body,
	}

	// 优先匹配渠道特有的 body 关键字
	switch {
	case statusCode == 403 && strings.Contains(bodyStr, "VALIDATION_REQUIRED"):
		// 账号需人工重新授权认证
		httpErr.ErrorType = httperror.ErrorTypeForbidden
		httpErr.ErrorCode = httperror.ErrorCodeForbidden
		httpErr.Message = fmt.Sprintf("account validation required: %s", bodyStr)
		return httpErr

	case statusCode == 429:
		// 限流，尝试从响应体中解析冷却时长
		httpErr.ErrorType = httperror.ErrorTypeRateLimit
		httpErr.ErrorCode = httperror.ErrorCodeRateLimit
		duration := parseCooldownDuration(bodyStr)
		if duration > 0 {
			t := time.Now().Add(duration)
			httpErr.CooldownUntil = &t
			httpErr.Message = fmt.Sprintf("rate limited, cooldown until %s: %s", t.Format(time.RFC3339), bodyStr)
		} else {
			httpErr.CooldownUntil = nil
			httpErr.Message = fmt.Sprintf("rate limited: %s", bodyStr)
		}
		return httpErr
	}

	// 通用状态码映射
	switch statusCode {
	case 400:
		httpErr.ErrorType = httperror.ErrorTypeBadRequest
		httpErr.ErrorCode = httperror.ErrorCodeBadRequest
		httpErr.Message = fmt.Sprintf("bad request: %s", bodyStr)

	case 401:
		httpErr.ErrorType = httperror.ErrorTypeUnauthorized
		httpErr.ErrorCode = httperror.ErrorCodeUnauthorized
		httpErr.Message = fmt.Sprintf("unauthorized, token may be expired: %s", bodyStr)

	case 403:
		httpErr.ErrorType = httperror.ErrorTypeForbidden
		httpErr.ErrorCode = httperror.ErrorCodeForbidden
		httpErr.Message = fmt.Sprintf("forbidden: %s", bodyStr)

	default:
		httpErr.ErrorType = httperror.ErrorTypeServerError
		httpErr.ErrorCode = httperror.ErrorCodeServerError
		httpErr.Message = fmt.Sprintf("server error (status %d): %s", statusCode, bodyStr)
	}

	return httpErr
}

// parseCooldownDuration 从限流响应体中解析冷却时长。
// 支持以下格式：
//   - "reset after Xh Xm Xs" / "retry after Xh Xm Xs"（如 "reset after 6h9m38s"、"retry after 30m"）
//   - "retry after X seconds" / "retry-after: X"（纯秒数格式）
//
// 解析 duration 格式时会额外加 60 秒缓冲，纯秒数格式直接返回原值对应的 Duration。
// 若无法解析则返回 0。
func parseCooldownDuration(body string) time.Duration {
	// 尝试匹配 "reset after Xh Xm Xs" 或 "retry after Xh Xm Xs" 格式
	reHMS := regexp.MustCompile(`(?i)(?:reset|retry)\s+after\s+([\dhms]+)`)
	if matches := reHMS.FindStringSubmatch(body); len(matches) >= 2 {
		if d, err := time.ParseDuration(matches[1]); err == nil && d > 0 {
			return d + 60*time.Second // 额外加 60 秒缓冲
		}
	}

	// 尝试匹配 "retry after X seconds" 或 "retry-after: X" 格式
	reSeconds := regexp.MustCompile(`(?i)(?:retry.?after)[:\s]+(\d+)`)
	if matches := reSeconds.FindStringSubmatch(body); len(matches) >= 2 {
		var seconds int64
		if _, err := fmt.Sscanf(matches[1], "%d", &seconds); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	return 0
}
