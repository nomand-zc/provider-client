# 需求文档：Kiro Client HTTP 错误归一化处理

## 引言

当前 `provider-client/client/kiro/kiro.go` 中，HTTP 非 2xx 响应的处理方式非常粗糙：

```go
// GenerateContent 中
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    resp.Body.Close()
    return nil, fmt.Errorf("Kiro API returned error status: %s", resp.Status)
}

// GenerateContentStream 中
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    resp.Body.Close()
    return nil, fmt.Errorf("Kiro API returned error status: %s", resp.Status)
}
```

这种处理方式存在以下问题：
1. **丢失响应体信息**：直接关闭 body，无法读取错误详情（如 `MONTHLY_REQUEST_COUNT`、`TEMPORARILY_SUSPENDED` 等关键字段）
2. **错误类型不明确**：调用方无法区分限流、配额耗尽、账号封禁、认证失败等不同错误场景
3. **无法提取限流信息**：429 限流错误中可能包含冷却时长，当前实现无法提取
4. **错误信息不结构化**：返回的是普通 `error`，调用方只能通过字符串匹配判断错误类型

参考 `proxy-gateway/internal/adapter/kiro.go` 中 `ClassifyError` 的错误分类逻辑，在 `provider-client` 层实现结构化的 HTTP 错误归一化处理。

---

## 需求

### 需求 1：定义结构化的 HTTP 错误类型

**用户故事：** 作为 provider-client 的调用方，我希望 kiro client 返回结构化的错误对象，以便能够类型安全地区分不同的错误场景（限流、配额耗尽、封禁等），而无需解析错误字符串。

#### 验收标准

1. WHEN kiro client 遇到 HTTP 非 2xx 响应 THEN 系统 SHALL 返回实现了 `error` 接口的 `*HTTPError` 结构体，而非普通 `fmt.Errorf` 错误
2. WHEN 定义 `HTTPError` THEN 系统 SHALL 包含以下字段：
   - `StatusCode int`：HTTP 状态码
   - `ErrorType ErrorType`：归一化的错误类型枚举
   - `RawBody []byte`：原始响应体（用于调试）
   - `Message string`：人类可读的错误描述
   - `CooldownSeconds int64`：限流冷却时长（秒），仅限流错误时有效，0 表示未知
3. WHEN 定义 `ErrorType` 枚举 THEN 系统 SHALL 覆盖以下类型：
   - `ErrorTypeUnauthorized`：401，Token 过期或无效
   - `ErrorTypeMonthlyQuotaExhausted`：402 + body 含 `MONTHLY_REQUEST_COUNT`，月度配额耗尽
   - `ErrorTypeQuotaExhausted`：402 其他情况，通用配额不足
   - `ErrorTypeForbidden`：403 通用权限不足
   - `ErrorTypeAccountSuspended`：403 + body 含 `TEMPORARILY_SUSPENDED`，账号临时封禁
   - `ErrorTypeRateLimit`：429，限流
   - `ErrorTypeBadRequest`：400，请求格式错误
   - `ErrorTypeServerError`：5xx，服务器错误
   - `ErrorTypeUpstream`：其他非 2xx，通用上游错误
4. WHEN `HTTPError.Error()` 被调用 THEN 系统 SHALL 返回包含状态码、错误类型和消息的可读字符串

### 需求 2：实现 Kiro 特有的错误分类逻辑

**用户故事：** 作为 provider-client 的调用方，我希望 kiro client 能够识别 Kiro/AWS CodeWhisperer 特有的错误语义，以便上层系统能够针对不同错误采取正确的处理策略。

#### 验收标准

1. WHEN Kiro 返回 `401` THEN 系统 SHALL 返回 `ErrorTypeUnauthorized`（Token 过期，可通过刷新 Token 恢复）
2. WHEN Kiro 返回 `402` 且响应体包含 `MONTHLY_REQUEST_COUNT` THEN 系统 SHALL 返回 `ErrorTypeMonthlyQuotaExhausted`
3. WHEN Kiro 返回 `402` 且响应体不包含 `MONTHLY_REQUEST_COUNT` THEN 系统 SHALL 返回 `ErrorTypeQuotaExhausted`
4. WHEN Kiro 返回 `403` 且响应体包含 `TEMPORARILY_SUSPENDED` THEN 系统 SHALL 返回 `ErrorTypeAccountSuspended`
5. WHEN Kiro 返回 `403` 且响应体不包含特殊标识 THEN 系统 SHALL 返回 `ErrorTypeForbidden`
6. WHEN Kiro 返回 `429` THEN 系统 SHALL 返回 `ErrorTypeRateLimit`，并尝试从响应体中解析冷却时长填入 `CooldownSeconds`（解析失败时 `CooldownSeconds=0`）
7. WHEN Kiro 返回 `400` THEN 系统 SHALL 返回 `ErrorTypeBadRequest`
8. WHEN Kiro 返回 `5xx` THEN 系统 SHALL 返回 `ErrorTypeServerError`
9. WHEN Kiro 返回其他非 2xx 状态码 THEN 系统 SHALL 返回 `ErrorTypeUpstream`

### 需求 3：在 GenerateContent 和 GenerateContentStream 中使用归一化错误

**用户故事：** 作为 provider-client 的调用方，我希望无论是同步还是流式调用，都能收到结构化的 HTTP 错误，以便统一处理。

#### 验收标准

1. WHEN `GenerateContent` 收到非 2xx 响应 THEN 系统 SHALL 读取响应体后关闭，并返回 `*HTTPError`（而非直接关闭 body 后返回普通 error）
2. WHEN `GenerateContentStream` 收到非 2xx 响应 THEN 系统 SHALL 读取响应体后关闭，并返回 `*HTTPError`
3. WHEN 读取响应体失败 THEN 系统 SHALL 仍然返回 `*HTTPError`，`RawBody` 为空，`Message` 中包含读取失败的原因
4. WHEN 错误分类逻辑被提取为独立函数 `classifyHTTPError(statusCode int, body []byte) *HTTPError` THEN 系统 SHALL 在 `GenerateContent` 和 `GenerateContentStream` 中复用该函数

### 需求 4：错误类型文件组织

**用户故事：** 作为 provider-client 的维护者，我希望错误类型定义有清晰的文件组织，以便于维护和扩展。

#### 验收标准

1. WHEN 新增错误类型定义 THEN 系统 SHALL 将 `HTTPError`、`ErrorType` 及相关常量定义在 `client/kiro/errors.go` 新文件中
2. WHEN 定义错误类型 THEN 系统 SHALL 为所有导出类型和常量添加 Go 注释
3. WHEN 实现 `classifyHTTPError` THEN 系统 SHALL 将其作为包内私有函数放在 `errors.go` 中，供 `kiro.go` 调用
