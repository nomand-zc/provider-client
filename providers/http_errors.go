package providers

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidGrant = errors.New("invalid_grant")
)

// ErrorType 归一化的 HTTP 错误类型枚举，所有渠道共用
type ErrorType string

const (
	// ErrorTypeBadRequest 对应 HTTP 400，请求参数错误。
	// 触发条件：请求格式不正确、参数缺失或非法。
	// 可恢复性：❌ 不可重试，需修正请求参数。
	ErrorTypeBadRequest ErrorType = "bad_request"

	// ErrorTypeUnauthorized 对应 HTTP 401，Token 过期或无效。
	// 触发条件：认证 Token 失效、过期或格式错误。
	// 可恢复性：✅ 刷新 Token 后可恢复。
	ErrorTypeUnauthorized ErrorType = "unauthorized"

	// ErrorTypeForbidden 对应 HTTP 403，账号权限不足、账号无效或需要人工重新授权认证。
	// 触发条件：涵盖所有"账号不可用"场景，包括：
	//   - 通用 403 权限不足
	//   - Kiro 账号被 AWS 临时封禁（body 含 TEMPORARILY_SUSPENDED）
	//   - GeminiCLI 账号需人工重新授权认证（body 含 VALIDATION_REQUIRED）
	// 可恢复性：❌ 需人工介入，程序无法自动恢复。
	ErrorTypeForbidden ErrorType = "forbidden"

	// ErrorTypeRateLimit 对应所有限流/限额类错误，含月度配额耗尽、触发限流规则等所有"需等待后重试"场景。
	// 触发条件：
	//   - 通用 429 限流
	//   - Kiro 402 月度配额耗尽（body 含 MONTHLY_REQUEST_COUNT）
	//   - GeminiCLI 429 含冷却时长（body 含 reset after Xh...）
	// 可恢复性：✅ 等待至 CooldownUntil 后可重试；若 CooldownUntil 为 nil，使用默认退避策略。
	ErrorTypeRateLimit ErrorType = "rate_limit"

	// ErrorTypeServerError 对应所有其他非 2xx 错误，含 5xx 服务器错误及其他未知状态码。
	// 触发条件：5xx 服务器内部错误，或其他不属于以上 4 类的非 2xx 状态码。
	// 可恢复性：✅ 可重试，但需注意重试频率。
	ErrorTypeServerError ErrorType = "server_error"
)

// 归一化错误码常量，与 ErrorType 一一对应，用于 HTTPError.ErrorCode 字段，禁止使用魔数。
const (
	// ErrorCodeBadRequest 对应 ErrorTypeBadRequest
	ErrorCodeBadRequest = 400
	// ErrorCodeUnauthorized 对应 ErrorTypeUnauthorized
	ErrorCodeUnauthorized = 401
	// ErrorCodeForbidden 对应 ErrorTypeForbidden
	ErrorCodeForbidden = 403
	// ErrorCodeRateLimit 对应 ErrorTypeRateLimit（含所有限流/限额场景，如原始 402/429 均归一化为此码）
	ErrorCodeRateLimit = 429
	// ErrorCodeServerError 对应 ErrorTypeServerError
	ErrorCodeServerError = 500
)

// HTTPError 结构化的 HTTP 错误，包含归一化后的语义字段和原始 HTTP 信息。
//
// 归一化字段（ErrorType、ErrorCode、Message、CooldownUntil）供程序逻辑使用；
// 原始字段（RawStatusCode、RawBody）仅用于日志记录和问题排查，程序逻辑不应依赖这两个字段。
type HTTPError struct {
	// ErrorType 归一化后的错误类型，供程序逻辑使用
	ErrorType ErrorType

	// ErrorCode 归一化后的错误码，与 ErrorType 对应（400/401/403/429/500），供程序逻辑使用
	ErrorCode int

	// Message 归一化后的人类可读描述，供程序逻辑使用
	Message string

	// CooldownUntil 限流冷却截止时间节点，仅 ErrorTypeRateLimit 时有效。
	//   nil  = 限流错误，但不知道具体冷却时长，使用默认退避策略
	//   非nil = 限流错误，且有明确的冷却截止时间，等待至该时间后重试
	CooldownUntil *time.Time

	// RawStatusCode 原始 HTTP 状态码，仅用于调试，程序逻辑不应依赖此字段
	RawStatusCode int

	// RawBody 原始响应体字节，仅用于调试，程序逻辑不应依赖此字段
	RawBody []byte
}

// Error 实现 error 接口，返回包含归一化信息和原始状态码的可读字符串
func (e *HTTPError) Error() string {
	return fmt.Sprintf("[%s(%d)] %s (raw status: %d)", e.ErrorType, e.ErrorCode, e.Message, e.RawStatusCode)
}

// Is 判断给定的 error 是否是 *HTTPError 类型。
// 可用于 errors.As 的前置判断，也可直接调用：httperror.Is(err)
func Is(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*HTTPError)
	return ok
}

// As 将 error 转换为 *HTTPError。
// 转换成功时返回 (*HTTPError, true)，否则返回 (nil, false)。
// 示例：
//
//	if httpErr, ok := httperror.As(err); ok {
//	    // 使用 httpErr
//	}
func As(err error) (*HTTPError, bool) {
	if err == nil {
		return nil, false
	}
	e, ok := err.(*HTTPError)
	return e, ok
}

// IsBadRequestError 判断是否是 ErrorTypeBadRequest（400）错误
func IsBadRequestError(err error) bool {
	convertedErr, ok := As(err)
	return ok && convertedErr.IsBadRequestError()
}

// IsBadRequestError 判断是否是 ErrorTypeBadRequest（400）错误
func (e *HTTPError) IsBadRequestError() bool {
	return e.ErrorType == ErrorTypeBadRequest
}

// IsUnauthorizedError 判断是否是 ErrorTypeUnauthorized（401）错误
func IsUnauthorizedError(err error) bool {
	convertedErr, ok := As(err)
	return ok && convertedErr.IsUnauthorizedError()
}

// IsUnauthorizedError 判断是否是 ErrorTypeUnauthorized（401）错误
func (e *HTTPError) IsUnauthorizedError() bool {
	return e.ErrorType == ErrorTypeUnauthorized
}

// IsForbiddenError 判断是否是 ErrorTypeForbidden（403）错误
func IsForbiddenError(err error) bool {
	convertedErr, ok := As(err)
	return ok && convertedErr.IsForbiddenError()
}

// IsForbiddenError 判断是否是 ErrorTypeForbidden（403）错误
func (e *HTTPError) IsForbiddenError() bool {
	return e.ErrorType == ErrorTypeForbidden
}

// IsRateLimitError 判断是否是 ErrorTypeRateLimit（429）错误
func IsRateLimitError(err error) bool {
	convertedErr, ok := As(err)
	return ok && convertedErr.IsRateLimitError()
}

// IsRateLimitError 判断是否是 ErrorTypeRateLimit（429）错误
func (e *HTTPError) IsRateLimitError() bool {
	return e.ErrorType == ErrorTypeRateLimit
}

// IsServerError 判断是否是 ErrorTypeServerError（500）错误
func IsServerError(err error) bool {
	convertedErr, ok := As(err)
	return ok && convertedErr.IsServerError()
}

// IsServerError 判断是否是 ErrorTypeServerError（500）错误
func (e *HTTPError) IsServerError() bool {
	return e.ErrorType == ErrorTypeServerError
}
