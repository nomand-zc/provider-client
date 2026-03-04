# 需求文档：KiroConverter 职责拆分重构

## 引言

当前 `KiroConverter` 在 `ConvertRequest` 方法中同时承担了三项职责：
1. 将标准请求（`types.Request`）转换为 Kiro API 的请求 body
2. 构建请求 URL（依赖 `options.defaultRegion`、`options.url`）
3. 设置 HTTP 请求头（依赖 `account` 中的认证 token 以及 `options.headers`）

这导致 `ConvertRequest` 需要同时接收 `account` 和 `options` 两个外部依赖，职责不单一，且与 `client.Converter` 接口定义（不含 `account` 参数）不一致。

本次重构目标：**让 Converter 只负责请求 body 的转换，header 的构建职责从 Converter 中剥离**，使各组件职责更清晰。

---

## 需求

### 需求 1：Converter 接口新增 Header 转换方法

**用户故事：** 作为开发者，我希望 `client.Converter` 接口提供独立的 Header 转换方法，以便不同渠道的 Converter 实现可以各自封装自己的认证逻辑，而不污染请求 body 转换方法。

#### 验收标准

1. WHEN 开发者查看 `client.Converter` 接口 THEN 接口 SHALL 包含 `ConvertHeaders(account types.Account) (map[string]string, error)` 方法
2. WHEN `ConvertRequest` 被调用 THEN 该方法 SHALL 不再接收 `account` 参数，签名与 `client.Converter` 接口保持一致
3. IF `ConvertHeaders` 方法被调用 THEN 系统 SHALL 返回包含所有必要 HTTP 请求头的 map，包括认证头和自定义头

### 需求 2：KiroConverter 实现 Header 转换方法

**用户故事：** 作为开发者，我希望 `KiroConverter` 实现 `ConvertHeaders` 方法，以便将 Kiro 特有的认证逻辑（从 `account.Creds` 提取 token）和固定请求头集中管理。

#### 验收标准

1. WHEN `KiroConverter.ConvertHeaders` 被调用 THEN 系统 SHALL 从 `account.Creds` 中提取 `access_token` 或 `token` 字段作为 Bearer token
2. WHEN `KiroConverter.ConvertHeaders` 被调用 THEN 系统 SHALL 返回包含以下 header 的 map：
   - `Content-Type: application/json`
   - `Accept: application/json`
   - `Authorization: Bearer <token>`
   - `amz-sdk-request: attempt=1; max=1`
   - `x-amzn-kiro-agent-mode: vibe`
   - `x-amz-user-agent: aws-sdk-js/1.0.0 KiroIDE-0.8.140`
   - `User-Agent: aws-sdk-js/1.0.0 ua/2.1 api/codewhispererruntime#1.0.0 m/E KiroIDE-0.8.140`
   - `options.headers` 中的所有自定义 header
3. WHEN `account.Creds` 为空或无法解析 THEN 系统 SHALL 返回错误
4. WHEN `KiroConverter.ConvertRequest` 被调用 THEN 该方法 SHALL 只负责构建请求 body 和 URL，不再设置任何 HTTP header

### 需求 3：kiro 结构体调用链更新

**用户故事：** 作为开发者，我希望 `kiro` 结构体的 `GenerateContent` 和 `GenerateContentStream` 方法按照新的职责划分调用 converter，以便代码流程清晰。

#### 验收标准

1. WHEN `kiro.GenerateContent` 或 `kiro.GenerateContentStream` 被调用 THEN 系统 SHALL 先调用 `converter.ConvertRequest` 构建带 body 的 `*http.Request`
2. WHEN 上一步成功后 THEN 系统 SHALL 调用 `converter.ConvertHeaders(account)` 获取 header map
3. WHEN 获取到 header map 后 THEN 系统 SHALL 将所有 header 设置到 `*http.Request` 上
4. IF `ConvertHeaders` 返回错误 THEN 系统 SHALL 将错误向上传播，不发送 HTTP 请求

### 需求 4：KiroConverter 不再持有 options 中与 header 无关的字段（可选优化）

**用户故事：** 作为开发者，我希望 `KiroConverter` 的依赖尽可能精简，以便降低组件间耦合。

#### 验收标准

1. WHEN 重构完成后 THEN `KiroConverter` SHALL 仍可通过 `options` 获取 `defaultRegion`、`url`（用于构建请求 URL）和 `headers`（用于自定义 header）
2. IF 未来需要将 URL 构建也从 Converter 中剥离 THEN 该需求作为后续优化，本次不强制要求
