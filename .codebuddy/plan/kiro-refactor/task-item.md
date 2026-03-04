# Kiro Client 重构实施计划

- [ ] 1. 重构 `kiro/converter.go`，定义 `KiroConverter` 结构体并实现 `client.Converter` 接口
   - 新增 `KiroConverter` 结构体，持有 `account types.Account` 和 `options Options` 字段
   - 新增 `NewKiroConverter(account types.Account, opts Options) *KiroConverter` 构造函数
   - 将 `http_client.go` 中的 `KiroResponse`、`KiroToolCall`、`KiroUsage`、`KiroError` 类型定义迁移至 `converter.go`
   - 实现 `ConvertRequest(ctx context.Context, req types.Request) (*http.Request, error)`：调用现有 `convertRequest` 逻辑构建 `KiroRequest`，再调用现有 `buildRequest` 逻辑构建完整 `*http.Request`（含认证头、URL、请求体）
   - 实现 `ConvertResponse(ctx context.Context, resp *http.Response) (*types.Response, error)`：迁移 `http_client.go` 中的 `parseResponse` 逻辑
   - 新增 `ParseStreamChunk(data []byte) (*types.Response, error)` 方法（非接口方法），供流式处理使用
   - _需求：需求1.1、需求1.2、需求1.3、需求1.4、需求3.3_

- [ ] 2. 删除 `kiro/http_client.go`，移除冗余的 `KiroHTTPClient` 和自定义 `HTTPClient` 接口
   - 删除 `kiro/http_client.go` 整个文件（其中的响应类型已迁移至 `converter.go`，`buildRequest`/`parseResponse` 逻辑已迁移至 `KiroConverter`）
   - _需求：需求3.1、需求3.2_

- [ ] 3. 重构 `kiro/kiro.go`，使用 `KiroConverter` 和 `client.HTTPClient` 重写调用链
   - 修改 `kiro` 结构体：移除 `converter client.Converter` 字段（改为在方法内动态创建），`httpClient` 类型保持 `client.HTTPClient`
   - 修改 `New(opts ...Option)` 工厂函数：使用 `client.DefaultNewHTTPClient()` 初始化 `httpClient`，移除对 `NewKiroHTTPClient` 的调用
   - 重写 `GenerateContent`：`conv := NewKiroConverter(account, k.options)` → `conv.ConvertRequest(ctx, req)` → `k.httpClient.Do(httpReq)` → `conv.ConvertResponse(ctx, resp)`
   - 重写 `GenerateContentStream`：`conv := NewKiroConverter(account, k.options)` → `conv.ConvertRequest(ctx, req)` → `k.httpClient.Do(httpReq)` → 逐块调用 `conv.ParseStreamChunk` 解析流式响应
   - 移除 `handleStreaming`、`parseStreamResponse` 等旧方法
   - _需求：需求2.1、需求2.2、需求2.3、需求2.4_

- [ ] 4. 验证编译通过，确保重构后功能完整
   - 执行 `go build ./...` 确认无编译错误
   - 检查 `GenerateContent` 和 `GenerateContentStream` 调用链是否符合 `converter.ConvertRequest → httpClient.Do → converter.ConvertResponse` 的顺序
   - _需求：需求4.1、需求4.2、需求4.3_
