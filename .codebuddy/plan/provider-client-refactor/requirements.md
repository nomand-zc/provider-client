# 需求文档：provider-client 整体重构

## 引言

本文档基于对 `provider-client` 代码库的多维度分析（目录结构、模块划分、函数最小职责、接口设计、类型安全等），梳理出当前存在的设计与实现问题，并提出对应的重构需求。

当前代码库整体结构如下：

```
provider-client/
├── model.go              # 顶层 Model 接口
├── client/
│   ├── converter.go      # Converter 接口
│   ├── http_client.go    # HTTPClient 接口
│   └── kiro/
│       ├── kiro.go       # kiro 结构体（Model 实现）
│       ├── converter.go  # KiroConverter（Converter 实现）+ 所有 Kiro 类型定义
│       ├── options.go    # Options / Option 函数
│       └── converter_test.go
├── convetor/             # 空目录（拼写错误）
└── types/                # 公共类型
```

---

## 问题分析

### 问题 1：`convetor/` 目录拼写错误且为空目录

`convetor/` 目录存在拼写错误（应为 `converter`），且目录为空，对代码库造成噪音。

### 问题 2：`converter.go` 文件职责过重（单文件超 500 行）

`client/kiro/converter.go` 同时承担了以下职责：
- `KiroConverter` 结构体及其三个接口方法（`ConvertRequest`、`ConvertHeaders`、`ConvertResponse`）
- 流式响应解析（`ParseStreamChunk`）
- 所有 Kiro API 类型定义（`KiroRequest`、`KiroResponse`、`KiroError` 等）
- 消息格式转换（`convertMessagesToConversationState`）
- 工具转换（`convertToolsToToolsContext`）
- 认证 token 提取（`extractAuthToken`）
- 响应格式转换（`convertKiroResponseToStandard`）
- URL 构建（`buildURL`）
- 模型映射（`modelMapping`、`getKiroModel`）

这违反了单一职责原则，导致文件难以维护和测试。

### 问题 3：`options.go` 中 `defaultOptions` 与 `ConvertHeaders` 存在重复的 header 定义

`options.go` 的 `defaultOptions.headers` 中已经定义了固定 header（`Content-Type`、`Accept`、`amz-sdk-request` 等），而 `ConvertHeaders` 方法中又重新硬编码了一遍相同的 header 列表，造成两处维护同一份数据的问题。

### 问题 4：`kiro.go` 中 `GenerateContent` 与 `GenerateContentStream` 存在大量重复代码

两个方法中的以下逻辑完全重复：
- 请求验证（`validateRequest`）
- `ConvertRequest` 调用
- `ConvertHeaders` 调用及 header 设置循环
- HTTP 请求发送
- 响应状态码检查

这违反了 DRY 原则，且增加了后续维护成本。

### 问题 5：`kiro.go` 中 `converter` 字段类型为具体类型而非接口

```go
type kiro struct {
    converter  *KiroConverter  // ❌ 具体类型，无法 mock 测试
}
```

`converter` 字段使用了具体类型 `*KiroConverter`，而非 `client.Converter` 接口，导致无法在测试中替换为 mock 实现，降低了可测试性。

### 问题 6：`ParseStreamChunk` 方法不属于 `client.Converter` 接口，但挂载在 `KiroConverter` 上

`ParseStreamChunk` 是流式处理的内部方法，不在 `client.Converter` 接口中定义，但却作为 `KiroConverter` 的公开方法暴露，且被 `kiro.go` 直接调用。这使得 `kiro.go` 对 `KiroConverter` 产生了超出接口约定的隐式依赖。

### 问题 7：`generateResponseID` 使用 `time.Now().UnixNano()` 不够唯一

在高并发场景下，`time.Now().UnixNano()` 可能产生重复 ID。应使用 `crypto/rand` 或 UUID 生成唯一 ID。

### 问题 8：`kiro.go` 中流式响应的 Content-Type 判断逻辑不健壮

```go
if contentType != "text/event-stream" && contentType != "application/x-ndjson" {
```

Content-Type 可能包含 `; charset=utf-8` 等参数，直接字符串比较会导致误判，应使用 `strings.Contains` 或 `mime.ParseMediaType`。

### 问题 9：`kiro.go` 中 `checkHTTPResponse` 函数定义后未被使用

`checkHTTPResponse` 函数在 `kiro.go` 中定义，但实际代码中并未调用（状态码检查是内联的），属于死代码。

### 问题 10：`convertRequest` 中模型名称硬编码，未使用请求中的 `Model` 字段

```go
modelName := getKiroModel("claude-sonnet-4-5")  // ❌ 忽略了 req 中的模型字段
```

`types.Request` 中没有 `Model` 字段，但 `convertRequest` 始终使用默认模型，无法根据请求动态选择模型。

---

## 需求

### 需求 1：清理空目录和拼写错误

**用户故事：** 作为一名开发者，我希望代码库中不存在无意义的空目录和拼写错误，以便保持目录结构整洁。

#### 验收标准

1. WHEN 开发者浏览项目目录 THEN 系统 SHALL 不存在 `convetor/` 空目录
2. IF 需要公共转换工具 THEN 系统 SHALL 使用正确拼写的 `converter/` 目录

---

### 需求 2：拆分 `converter.go`，按职责分离到多个文件

**用户故事：** 作为一名开发者，我希望 `client/kiro/` 目录下的文件按职责清晰划分，以便快速定位和维护代码。

#### 验收标准

1. WHEN 开发者查看 `client/kiro/` 目录 THEN 系统 SHALL 包含以下文件：
   - `types.go`：所有 Kiro API 类型定义（`KiroRequest`、`KiroResponse` 等）
   - `converter.go`：`KiroConverter` 结构体及接口方法实现
   - `model_mapping.go`：模型映射表和 `getKiroModel` 函数
   - `kiro.go`：`kiro` 结构体及 `Model` 接口实现
   - `options.go`：`Options` 和 `Option` 函数
2. WHEN 单个文件行数超过 200 行 THEN 系统 SHALL 考虑进一步拆分

---

### 需求 3：消除 `options.go` 与 `ConvertHeaders` 中的 header 重复定义

**用户故事：** 作为一名开发者，我希望固定 header 只在一处定义，以便修改时不遗漏。

#### 验收标准

1. WHEN `ConvertHeaders` 构建 header map THEN 系统 SHALL 以 `defaultOptions.headers` 为基础，而非重新硬编码
2. IF `options.headers` 中存在自定义 header THEN 系统 SHALL 将其合并覆盖到基础 header 上

---

### 需求 4：提取 `kiro.go` 中的公共请求发送逻辑，消除重复代码

**用户故事：** 作为一名开发者，我希望 `GenerateContent` 和 `GenerateContentStream` 共享请求构建和发送逻辑，以便减少重复代码。

#### 验收标准

1. WHEN `kiro` 结构体需要发送 HTTP 请求 THEN 系统 SHALL 提供私有方法 `buildHTTPRequest` 封装请求构建（ConvertRequest + ConvertHeaders + header 设置）
2. WHEN `GenerateContent` 和 `GenerateContentStream` 发送请求 THEN 系统 SHALL 均调用 `buildHTTPRequest`，不重复实现

---

### 需求 5：将 `kiro.go` 中 `converter` 字段类型改为 `client.Converter` 接口

**用户故事：** 作为一名开发者，我希望 `kiro` 结构体依赖 `client.Converter` 接口而非具体类型，以便在测试中注入 mock 实现。

#### 验收标准

1. WHEN `kiro` 结构体定义 `converter` 字段 THEN 系统 SHALL 使用 `client.Converter` 接口类型
2. WHEN 编写 `kiro` 的单元测试 THEN 系统 SHALL 能够注入 mock converter 而无需修改生产代码

---

### 需求 6：将 `ParseStreamChunk` 内化为 `kiro.go` 的私有方法或独立函数

**用户故事：** 作为一名开发者，我希望流式解析逻辑不暴露为 `KiroConverter` 的公开方法，以便保持接口边界清晰。

#### 验收标准

1. WHEN `KiroConverter` 实现 `client.Converter` 接口 THEN 系统 SHALL 不暴露 `ParseStreamChunk` 公开方法
2. WHEN `kiro.go` 需要解析流式数据块 THEN 系统 SHALL 通过包内私有函数 `parseStreamChunk` 实现

---

### 需求 7：修复 `generateResponseID` 的唯一性问题

**用户故事：** 作为一名开发者，我希望响应 ID 在高并发下保持唯一，以便正确追踪每次响应。

#### 验收标准

1. WHEN 生成响应 ID THEN 系统 SHALL 使用 `crypto/rand` 或 UUID 库生成，而非 `time.Now().UnixNano()`

---

### 需求 8：修复流式响应 Content-Type 判断逻辑

**用户故事：** 作为一名开发者，我希望 Content-Type 判断能正确处理带参数的媒体类型，以便流式响应不被误判为普通响应。

#### 验收标准

1. WHEN 响应 Content-Type 为 `text/event-stream; charset=utf-8` THEN 系统 SHALL 正确识别为流式响应
2. WHEN 判断 Content-Type THEN 系统 SHALL 使用 `strings.HasPrefix` 或 `mime.ParseMediaType` 而非直接字符串相等比较

---

### 需求 9：删除未使用的 `checkHTTPResponse` 函数

**用户故事：** 作为一名开发者，我希望代码库中不存在死代码，以便保持代码整洁。

#### 验收标准

1. WHEN 开发者查看 `kiro.go` THEN 系统 SHALL 不包含未被调用的 `checkHTTPResponse` 函数
