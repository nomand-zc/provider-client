# 实施计划：Converter 接口职责边界重构

- [ ] 1. 更新 `client/converter.go` 接口定义
   - 将 `ConvertRequest` 返回值从 `(*http.Request, error)` 改为 `([]byte, error)`
   - 将 `ConvertResponse` 入参 `resp *http.Response` 改为 `body []byte`
   - 删除 `ConvertHeaders` 方法，移除 `net/http` 导入
   - _需求：4.1_

- [ ] 2. 重构 `KiroConverter.ConvertRequest`，只负责序列化
   - 删除 `buildURL`、`http.NewRequestWithContext` 等传输层逻辑
   - 只保留：调用 `convertRequest` 将 `types.Request` 转为 `KiroRequest`，再 `json.Marshal` 返回 `[]byte`
   - 移除不再需要的 `bytes`、`net/http` 导入
   - _需求：1.1、1.2_

- [ ] 3. 重构 `KiroConverter.ConvertResponse`，只负责反序列化
   - 入参改为 `body []byte`，删除 `defer resp.Body.Close()` 和 `io.ReadAll` 逻辑
   - 只保留：`json.Unmarshal(body, &kiroResp)` 并调用 `convertKiroResponseToStandard`
   - 移除不再需要的 `io`、`net/http` 导入
   - _需求：2.1、2.3_

- [ ] 4. 删除 `KiroConverter.ConvertHeaders` 方法，将 `buildURL` 迁移到 `kiro` 结构体
   - 删除 `converter.go` 中的 `ConvertHeaders` 方法和 `buildURL` 方法
   - 在 `kiro.go` 中新增私有方法 `buildHeaders(account types.Account) (map[string]string, error)`，包含 token 提取和默认 header 设置逻辑
   - 在 `kiro.go` 中新增私有方法 `buildURL() string`，从 `k.options` 读取 region 和 url 模板
   - _需求：3.1、3.2、3.3、3.4、1.4_

- [ ] 5. 重构 `kiro.go` 中的 `buildHTTPRequest` 方法
   - 调用 `k.converter.ConvertRequest(ctx, req)` 获取 `[]byte`
   - 用 `bytes.NewReader(body)` 包装为 `io.Reader`
   - 调用 `k.buildURL()` 构建 URL
   - 调用 `http.NewRequestWithContext` 创建请求
   - 调用 `k.buildHeaders(account)` 获取 header 并设置到请求上
   - _需求：1.3、3.2_

- [ ] 6. 重构 `kiro.go` 中 `GenerateContent` 和 `GenerateContentStream` 的响应处理
   - 在调用 `k.converter.ConvertResponse` 前，先 `io.ReadAll(resp.Body)` 读取 body 并 `resp.Body.Close()`
   - 将读取到的 `[]byte` 传给 `k.converter.ConvertResponse(ctx, body)`
   - 同步更新 `GenerateContentStream` 中非流式分支的 `ConvertResponse` 调用
   - _需求：2.1、2.2_

- [ ] 7. 更新 `converter_test.go` 测试用例
   - 更新 `ConvertRequest` 相关测试：断言返回值为 `[]byte`，验证 JSON 内容正确
   - 更新 `ConvertResponse` 相关测试：入参改为 `[]byte`，删除 mock `*http.Response` 构造逻辑
   - 删除 `ConvertHeaders` 相关测试用例
   - 运行 `go test ./...` 确保所有测试通过
   - _需求：5.1、5.2、5.3_
