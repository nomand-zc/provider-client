# 实施计划

- [ ] 1. 更新 `client.Converter` 接口，新增 `ConvertHeaders` 方法
   - 在 `client/converter.go` 中为 `Converter` 接口新增方法签名：`ConvertHeaders(account types.Account) (map[string]string, error)`
   - 同时将 `ConvertRequest` 签名中的 `account types.Account` 参数移除，保持与接口一致
   - _需求：1.1、1.2_

- [ ] 2. 在 `KiroConverter` 中实现 `ConvertHeaders` 方法
   - 在 `client/kiro/converter.go` 中新增 `ConvertHeaders(account types.Account) (map[string]string, error)` 方法
   - 方法内部调用已有的 `extractAuthToken(account)` 提取 Bearer token
   - 返回包含所有固定 header（`Content-Type`、`Accept`、`Authorization`、`amz-sdk-request`、`x-amzn-kiro-agent-mode`、`x-amz-user-agent`、`User-Agent`）以及 `options.headers` 中自定义 header 的 map
   - _需求：2.1、2.2、2.3_

- [ ] 3. 重构 `KiroConverter.ConvertRequest`，移除 header 设置逻辑
   - 修改 `ConvertRequest` 方法签名，去掉 `account types.Account` 参数
   - 删除方法内部的 `extractAuthToken` 调用和所有 `httpReq.Header.Set(...)` 语句
   - 保留请求 body 序列化和 URL 构建逻辑（`buildURL`、`json.Marshal`、`http.NewRequestWithContext`）
   - _需求：2.4_

- [ ] 4. 更新 `kiro` 结构体的调用链
   - 在 `client/kiro/kiro.go` 的 `GenerateContent` 和 `GenerateContentStream` 方法中：
     1. 调用 `k.converter.ConvertRequest(ctx, req)` 构建不含 header 的 `*http.Request`
     2. 调用 `k.converter.ConvertHeaders(account)` 获取 header map
     3. 遍历 header map，通过 `httpReq.Header.Set(key, value)` 将所有 header 设置到请求上
     4. 若 `ConvertHeaders` 返回错误则直接返回，不发送请求
   - _需求：3.1、3.2、3.3、3.4_

- [ ] 5. 检查并更新测试用例
   - 搜索项目中所有调用 `ConvertRequest` 传入 `account` 参数的测试代码，更新为新签名
   - 为 `KiroConverter.ConvertHeaders` 新增单元测试，覆盖：正常 token 提取、`access_token` 优先于 `token`、Creds 为空时返回错误、自定义 header 合并
   - 确保所有测试通过率 100%
   - _需求：1.2、2.1、2.2、2.3_
