# Kiro Client 重构需求文档

## 引言

当前 `provider-client/client/kiro/` 目录下的实现存在架构耦合问题：`converter.go` 中的格式转换逻辑与 HTTP 请求构建逻辑混杂，且未实现 `client/converter.go` 中定义的标准 `Converter` 接口。本次重构目标是让 `kiro/converter.go` 实现 `client.Converter` 接口，使 `kiro.go` 中的 `GenerateContent` 和 `GenerateContentStream` 方法通过标准接口完成请求转换和响应解析，调用链更清晰。

## 需求

### 需求 1：KiroConverter 实现 client.Converter 接口

**用户故事：** 作为一名开发者，我希望 `kiro/converter.go` 实现标准的 `client.Converter` 接口，以便 `kiro.go` 可以通过统一接口完成请求/响应转换。

#### 验收标准

1. WHEN 定义 `KiroConverter` 结构体 THEN 系统 SHALL 实现 `client.Converter` 接口的两个方法：`ConvertRequest` 和 `ConvertResponse`
2. WHEN 调用 `ConvertRequest(ctx, req types.Request)` THEN 系统 SHALL 返回完整可用的 `*http.Request`，包含正确的 URL、认证头、请求体
3. WHEN 调用 `ConvertResponse(ctx, resp *http.Response)` THEN 系统 SHALL 解析响应体并返回标准 `*types.Response`
4. WHEN 创建 `KiroConverter` 时 THEN 系统 SHALL 通过构造函数注入 `account types.Account` 和 `Options`，以便 `ConvertRequest` 能构建完整的认证请求

### 需求 2：重构 kiro.go 的 GenerateContent 调用链

**用户故事：** 作为一名开发者，我希望 `GenerateContent` 方法通过 `converter.ConvertRequest` 获得 `*http.Request`，再调用 `httpClient.Do()`，响应通过 `converter.ConvertResponse` 转换，使调用链清晰简洁。

#### 验收标准

1. WHEN 调用 `GenerateContent` THEN 系统 SHALL 按以下顺序执行：`converter.ConvertRequest` → `httpClient.Do` → `converter.ConvertResponse`
2. WHEN 调用 `GenerateContentStream` THEN 系统 SHALL 通过 `converter.ConvertRequest` 构建流式请求，调用 `httpClient.Do`，再逐块解析流式响应
3. WHEN `kiro` 结构体初始化时 THEN 系统 SHALL 使用 `client.HTTPClient`（来自 `client/http_client.go`）而非 `kiro` 包内自定义的 `HTTPClient` 接口
4. WHEN `kiro` 结构体初始化时 THEN 系统 SHALL 将 `account` 和 `options` 注入到 `KiroConverter` 中

### 需求 3：简化 kiro/http_client.go

**用户故事：** 作为一名开发者，我希望 `kiro/http_client.go` 只保留必要的底层 HTTP 功能，将请求构建和响应解析职责移交给 `KiroConverter`。

#### 验收标准

1. WHEN 重构完成后 THEN `kiro/http_client.go` SHALL 不再包含 `buildRequest`、`sendRequest`、`parseResponse` 等与格式转换相关的方法
2. WHEN 重构完成后 THEN `kiro/http_client.go` 中的 `KiroHTTPClient` 类型 SHALL 被移除或简化，直接使用 `client.HTTPClient` 接口
3. WHEN 重构完成后 THEN `KiroResponse`、`KiroToolCall`、`KiroUsage`、`KiroError` 等响应类型 SHALL 移至 `converter.go` 中，与响应解析逻辑放在一起

### 需求 4：保持编译通过和功能完整

**用户故事：** 作为一名开发者，我希望重构后代码能正常编译，且 `GenerateContent` 和 `GenerateContentStream` 功能与重构前等价。

#### 验收标准

1. WHEN 执行 `go build ./...` THEN 系统 SHALL 编译通过，无任何错误
2. WHEN 调用 `GenerateContent` THEN 系统 SHALL 正确发送请求并返回标准响应
3. WHEN 调用 `GenerateContentStream` THEN 系统 SHALL 正确处理流式响应

## 技术约束

- `client.Converter` 接口签名不可修改：`ConvertRequest(ctx context.Context, req types.Request) (*http.Request, error)` 和 `ConvertResponse(ctx context.Context, resp *http.Response) (*types.Response, error)`
- `account types.Account` 需在 `KiroConverter` 初始化时注入，因为 `ConvertRequest` 接口不接受 `account` 参数
- 使用 `client.HTTPClient` 接口（`client/http_client.go` 中定义），不在 kiro 包内重复定义
- 保持 `kiro.go` 中 `New(opts ...Option)` 工厂函数签名不变，`account` 在调用 `GenerateContent` 时传入，需在调用时动态创建 converter
