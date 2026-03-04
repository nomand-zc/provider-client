# 需求文档：Converter 接口职责边界重构

## 引言

当前 `client.Converter` 接口的 `ConvertRequest` 方法返回 `*http.Request`，这意味着 converter 需要感知 HTTP 协议细节（URL 构建、HTTP 方法选择、`http.NewRequestWithContext` 调用），而这些属于传输层职责，不属于"格式转换"的范畴。

同样，`ConvertResponse` 接收 `*http.Response`，converter 需要负责读取 body、关闭 body，这也是传输层的职责。

用户提出的核心思路是：**converter 只负责数据格式转换（序列化/反序列化），完整的 HTTP 请求构造（URL、method、header）统一由 `kiro` 结构体负责**。`Converter` 接口不再包含任何与 HTTP 传输相关的方法，边界更清晰，也更易于测试。

---

## 需求

### 需求 1：ConvertRequest 只负责请求 body 序列化

**用户故事：** 作为开发者，我希望 `ConvertRequest` 只返回序列化后的请求 body（`[]byte`）和 error，以便 converter 不再感知 HTTP 协议细节（URL、method、header），单纯只负责结构转换。

#### 验收标准

1. WHEN `ConvertRequest` 被调用 THEN 接口 SHALL 返回 `([]byte, error)`，不再返回 `*http.Request` 或 `io.Reader`
2. WHEN converter 实现 `ConvertRequest` THEN 实现 SHALL 只负责将 `types.Request` 序列化为 Kiro API 格式的 JSON 字节切片，不构建 URL，不创建 `*http.Request`
3. WHEN `kiro` 结构体的 `buildHTTPRequest` 方法被调用 THEN 该方法 SHALL 负责：调用 `ConvertRequest` 获取 body 字节、用 `bytes.NewReader` 包装为 `io.Reader`、调用 `buildURL` 构建 URL、调用 `http.NewRequestWithContext` 创建请求、构建并设置 header
4. IF `buildURL` 逻辑需要访问 `options` THEN `buildURL` SHALL 作为 `kiro` 结构体的私有方法，而非 `KiroConverter` 的方法

### 需求 2：ConvertResponse 只负责响应 body 反序列化

**用户故事：** 作为开发者，我希望 `ConvertResponse` 只接收响应 body 的 `[]byte`，以便 converter 不再负责读取/关闭 HTTP body，传输层职责完全由 `kiro` 结构体承担，converter 单纯只负责结构转换。

#### 验收标准

1. WHEN `ConvertResponse` 被调用 THEN 接口 SHALL 接收 `(ctx context.Context, body []byte)` 而非 `*http.Response` 或 `io.Reader`
2. WHEN `kiro` 结构体处理响应 THEN `kiro` SHALL 负责检查 HTTP 状态码、读取并关闭 `resp.Body`，然后将读取到的 `[]byte` 传给 `ConvertResponse`
3. WHEN `ConvertResponse` 实现被调用 THEN 实现 SHALL 只负责将 `[]byte` 反序列化为 Kiro 格式响应，并转换为 `*types.Response`

### 需求 3：header 构建职责归属 kiro 结构体

**用户故事：** 作为开发者，我希望 header 构建逻辑（认证 token 提取、Content-Type 等默认 header 设置）完全由 `kiro` 结构体的方法负责，以便 `Converter` 接口不包含任何传输层相关方法。

#### 验收标准

1. WHEN `Converter` 接口被定义 THEN 接口 SHALL 不包含任何 header 相关方法（即去掉 `ConvertHeaders`）
2. WHEN `kiro` 结构体的 `buildHTTPRequest` 方法被调用 THEN 该方法 SHALL 直接从 `account` 中提取认证信息并设置到请求 header 上，无需通过 converter
3. WHEN `KiroConverter` 中原有的 `ConvertHeaders` 方法存在 THEN 该方法 SHALL 被删除，相关逻辑迁移为 `kiro` 结构体的私有方法 `buildHeaders`
4. WHEN `kiro` 结构体构建 header THEN 该逻辑 SHALL 包含：从 `account.Token` 提取 Bearer token、设置 `Content-Type: application/json`、设置 `Accept` 等必要 header

### 需求 4：更新 Converter 接口定义

**用户故事：** 作为开发者，我希望 `client.Converter` 接口定义与上述职责边界保持一致，以便所有实现方都遵循相同的约定。

#### 验收标准

1. WHEN 更新 `client/converter.go` THEN 接口 SHALL 定义为：
   ```go
   type Converter interface {
       ConvertRequest(ctx context.Context, req types.Request) ([]byte, error)
       ConvertResponse(ctx context.Context, body []byte) (*types.Response, error)
   }
   ```
2. WHEN `KiroConverter` 实现该接口 THEN 编译 SHALL 通过，所有方法签名与接口一致
3. WHEN `kiro` 结构体使用 `converter` 字段 THEN 该字段类型 SHALL 为 `client.Converter` 接口，而非具体类型

### 需求 5：更新测试用例

**用户故事：** 作为开发者，我希望测试用例与重构后的接口保持一致，以便确保重构正确性。

#### 验收标准

1. WHEN 重构完成 THEN `converter_test.go` 中涉及 `ConvertRequest`、`ConvertHeaders` 和 `ConvertResponse` 的测试 SHALL 更新为新签名
2. WHEN `ConvertHeaders` 相关测试存在 THEN 该测试 SHALL 迁移为通过 `buildHTTPRequest` 的集成测试间接覆盖，或直接删除
3. WHEN 运行 `go test ./...` THEN 所有测试 SHALL 通过
