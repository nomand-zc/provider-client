package kiro

import (
	"fmt"
	"strings"

	"github.com/nomand-zc/token101/provider-client/types/httperror"
)

// NewHTTPError 将 Kiro/AWS CodeWhisperer 渠道的原始 HTTP 响应归一化为 HTTPError。
// 这是 Kiro 渠道将原始 HTTP 响应转换为归一化 HTTPError 的唯一入口。
//
// 渠道特有错误识别规则（优先于通用状态码映射）：
//   - 402 + body 含 "MONTHLY_REQUEST_COUNT" → ErrorTypeRateLimit（月度配额耗尽，归入限流类）
//   - 403 + body 含 "TEMPORARILY_SUSPENDED" → ErrorTypeForbidden（账号被 AWS 临时封禁）
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
	case statusCode == 402 && strings.Contains(bodyStr, "MONTHLY_REQUEST_COUNT"):
		// 月度配额耗尽，归入限流类，无法解析重置时间
		httpErr.ErrorType = httperror.ErrorTypeRateLimit
		httpErr.ErrorCode = httperror.ErrorCodeRateLimit
		httpErr.Message = fmt.Sprintf("monthly quota exhausted: %s", bodyStr)
		httpErr.CooldownUntil = nil
		return httpErr

	case statusCode == 403 && strings.Contains(bodyStr, "TEMPORARILY_SUSPENDED"):
		// 账号被 AWS 临时封禁
		httpErr.ErrorType = httperror.ErrorTypeForbidden
		httpErr.ErrorCode = httperror.ErrorCodeForbidden
		httpErr.Message = fmt.Sprintf("account temporarily suspended: %s", bodyStr)
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

	case 429:
		httpErr.ErrorType = httperror.ErrorTypeRateLimit
		httpErr.ErrorCode = httperror.ErrorCodeRateLimit
		httpErr.Message = fmt.Sprintf("rate limited: %s", bodyStr)
		httpErr.CooldownUntil = nil

	default:
		httpErr.ErrorType = httperror.ErrorTypeServerError
		httpErr.ErrorCode = httperror.ErrorCodeServerError
		httpErr.Message = fmt.Sprintf("server error (status %d): %s", statusCode, bodyStr)
	}

	return httpErr
}
