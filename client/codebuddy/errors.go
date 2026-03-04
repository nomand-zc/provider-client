package codebuddy

import (
	"fmt"

	"github.com/nomand-zc/token101/provider-client/types/httperror"
)

// NewHTTPError 将 CodeBuddy 渠道的原始 HTTP 响应归一化为 HTTPError。
// 这是 CodeBuddy 渠道将原始 HTTP 响应转换为归一化 HTTPError 的唯一入口。
//
// CodeBuddy 无特殊错误语义，所有错误均走通用状态码映射处理：
//   - 400 → ErrorTypeBadRequest
//   - 401 → ErrorTypeUnauthorized
//   - 403 → ErrorTypeForbidden
//   - 429 → ErrorTypeRateLimit（CooldownUntil=nil）
//   - 其他非 2xx → ErrorTypeServerError
func NewHTTPError(statusCode int, body []byte) *httperror.HTTPError {
	bodyStr := string(body)

	httpErr := &httperror.HTTPError{
		RawStatusCode: statusCode,
		RawBody:       body,
	}

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
