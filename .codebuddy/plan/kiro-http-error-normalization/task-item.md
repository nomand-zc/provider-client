# 实施计划

- [ ] 1. 新建 `client/kiro/errors.go`，定义错误类型和分类函数
   - 定义 `ErrorType` 字符串枚举常量（`ErrorTypeUnauthorized`、`ErrorTypeMonthlyQuotaExhausted`、`ErrorTypeQuotaExhausted`、`ErrorTypeForbidden`、`ErrorTypeAccountSuspended`、`ErrorTypeRateLimit`、`ErrorTypeBadRequest`、`ErrorTypeServerError`、`ErrorTypeUpstream`）
   - 定义 `HTTPError` 结构体，包含 `StatusCode`、`ErrorType`、`RawBody`、`Message`、`CooldownSeconds` 字段，并实现 `Error() string` 方法
   - 实现包内私有函数 `classifyHTTPError(statusCode int, body []byte) *HTTPError`，参考 `proxy-gateway/internal/adapter/kiro.go` 中 `KiroAdapter.ClassifyError` 的分类逻辑，覆盖 401/402/403/429/400/5xx 等所有情况，并在 429 时尝试从响应体解析冷却时长
   - _需求：1.1、1.2、1.3、1.4、2.1-2.9、4.1、4.2、4.3_

- [ ] 2. 修改 `client/kiro/kiro.go`，在 `GenerateContent` 和 `GenerateContentStream` 中使用归一化错误
   - 将 `GenerateContent` 中的错误处理从"直接关闭 body + 返回 `fmt.Errorf`"改为"读取 body → 调用 `classifyHTTPError` → 返回 `*HTTPError`"
   - 将 `GenerateContentStream` 中的错误处理做同样修改
   - 删除已无用的 `checkHTTPResponse` 函数（当前仅定义未被调用）
   - _需求：3.1、3.2、3.3、3.4_
