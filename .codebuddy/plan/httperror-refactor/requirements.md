# 需求文档：provider-client HTTP 错误归一化重构

## 引言

当前 `provider-client/types/httperror/errors.go` 中的 `ClassifyHTTPError` 函数将多个渠道特有的错误识别逻辑混合在一起，且错误类型的语义划分不够准确。

存在的问题：

1. **渠道特有逻辑污染了公共包**：`MONTHLY_REQUEST_COUNT`、`TEMPORARILY_SUSPENDED` 等关键字是各渠道私有的错误标识，不应出现在公共的 `httperror` 包中
2. **错误类型过于细碎**：原有 8 个错误类型过于细化，调用方需要处理过多分支；应收敛为 5 个语义清晰的归一化类型
3. **各渠道没有统一的错误归一化入口**：应由各渠道 client 包暴露 `NewHTTPError` 方法，作为该渠道将原始 HTTP 响应转换为归一化 `HTTPError` 的唯一入口
4. **缺少原始错误信息**：归一化后的 `HTTPError` 没有保留原始的 HTTP 状态码和响应体，不便于调试和问题排查

**正确设计**：
- `HTTPError` 结构体 + `ErrorType` 枚举 → **公共，所有渠道共用**
- 公共包**不提供** `ClassifyHTTPError` 函数，各渠道通过自己的 `NewHTTPError` 完成全部归一化逻辑
- `NewHTTPError(statusCode int, body []byte) *httperror.HTTPError` → **各渠道 client 包各自实现**，作为该渠道的错误归一化唯一入口，内部完成渠道特有的 body 关键字匹配和状态码映射

---

## 归一化错误类型清单

参考 `proxy-gateway/internal/adapter` 中各渠道适配器的 `ClassifyError` 实现，梳理出所有错误场景，并映射到统一的 `ErrorType` 枚举：

### 公共错误（所有渠道共用，仅依赖状态码）

| 状态码 | 触发条件 | 归一化 ErrorType | 语义说明 |
|--------|----------|-----------------|---------|
| 400 | 任意渠道 | `ErrorTypeBadRequest` | 请求参数错误，不可重试 |
| 401 | 任意渠道 | `ErrorTypeUnauthorized` | Token 过期或无效，可通过刷新 Token 恢复 |
| 403 | 任意渠道 | `ErrorTypeForbidden` | 账号权限不足、账号无效或需要人工重新授权认证 |
| 429 | 任意渠道 | `ErrorTypeRateLimit` | 限流（含月度配额耗尽、触发限流规则等所有限流场景） |
| 其他非 2xx | 任意渠道 | `ErrorTypeServerError` | 其他非 200 的错误（含 5xx 及其他未知状态码） |

### Kiro 渠道特有错误（body 关键字匹配）

| 状态码 | body 关键字 | 归一化 ErrorType | 语义说明 |
|--------|------------|-----------------|---------|
| 402 | `MONTHLY_REQUEST_COUNT` | `ErrorTypeRateLimit` | 月度配额耗尽，归入限流类，`CooldownUntil=nil`（无法解析重置时间） |
| 403 | `TEMPORARILY_SUSPENDED` | `ErrorTypeForbidden` | 账号被 AWS 临时封禁，归入 403 权限类 |

### GeminiCLI 渠道特有错误（body 关键字匹配）

| 状态码 | body 关键字 | 归一化 ErrorType | 语义说明 |
|--------|------------|-----------------|---------|
| 403 | `VALIDATION_REQUIRED` | `ErrorTypeForbidden` | 账号需要人工重新授权认证，归入 403 权限类 |
| 429 | `reset after Xh...` | `ErrorTypeRateLimit` | 限流，`CooldownUntil` 从 body 中解析冷却时长后计算得出（当前时间 + 解析时长 + 60s 缓冲） |

### CodeBuddy 渠道

CodeBuddy 无特殊错误语义，所有错误均走通用状态码映射处理。

---

## 最终 ErrorType 枚举定义（5 个）

| ErrorType 常量 | 值 | 语义 | 可自动恢复 |
|---------------|-----|------|-----------|
| `ErrorTypeBadRequest` | `"bad_request"` | 请求参数错误 | ❌ 不可重试 |
| `ErrorTypeUnauthorized` | `"unauthorized"` | Token 过期/无效 | ✅ 刷新 Token 后可恢复 |
| `ErrorTypeForbidden` | `"forbidden"` | 账号权限不足、账号无效或需要人工重新授权认证 | ❌ 需人工介入 |
| `ErrorTypeRateLimit` | `"rate_limit"` | 所有限流/限额类错误（含月度配额耗尽、触发限流规则等） | ✅ 等待至 CooldownUntil 后可重试 |
| `ErrorTypeServerError` | `"server_error"` | 其他非 200 的错误（含 5xx 及其他未知状态码） | ✅ 可重试 |

> **注意**：
> - `ErrorTypeForbidden` 涵盖所有 403 场景：通用 403 权限不足、Kiro 账号临时封禁（`TEMPORARILY_SUSPENDED`）、GeminiCLI 需人工验证（`VALIDATION_REQUIRED`）。调用方统一按"账号不可用，需人工介入"处理，无需区分具体原因（具体原因可通过 `RawBody` 调试）
> - `ErrorTypeRateLimit` 通过 `CooldownUntil *time.Time` 字段表达冷却截止时间节点：
>   - `CooldownUntil == nil`：限流错误，但不知道具体的冷却时长（如通用 429、Kiro 月度配额耗尽）
>   - `CooldownUntil != nil`：限流错误，且有明确的冷却截止时间（如 GeminiCLI 429 含 `reset after Xh...`）
>   - 调用方通过 `err.CooldownUntil != nil` 判断是否有明确的冷却截止时间
> - `ErrorTypeServerError` 兜底所有非 2xx 且不属于以上 4 类的错误

---

## HTTPError 结构体字段定义

```
HTTPError
├── ErrorType     ErrorType    // 归一化后的错误类型（程序逻辑使用）
├── ErrorCode     int          // 归一化后的错误码（程序逻辑使用，与 ErrorType 对应）
├── Message       string       // 归一化后的人类可读描述（程序逻辑使用）
├── CooldownUntil *time.Time   // 限流冷却截止时间节点，仅 ErrorTypeRateLimit 时有效
│                              //   nil  = 限流错误，但不知道具体冷却时长
│                              //   非nil = 限流错误，且有明确的冷却截止时间
├── RawStatusCode int          // 原始 HTTP 状态码（调试用）
└── RawBody       []byte       // 原始响应体字节（调试用）
```

### ErrorCode 与 ErrorType 对应关系

| ErrorType | ErrorCode | 说明 |
|-----------|-----------|------|
| `ErrorTypeBadRequest` | `400` | 对应 HTTP 400 |
| `ErrorTypeUnauthorized` | `401` | 对应 HTTP 401 |
| `ErrorTypeForbidden` | `403` | 对应 HTTP 403（含所有 403 子场景） |
| `ErrorTypeRateLimit` | `429` | 对应所有限流/限额场景（含 Kiro 402 月度配额） |
| `ErrorTypeServerError` | `500` | 对应所有其他非 2xx 错误 |

- `ErrorType` + `ErrorCode` + `Message` + `CooldownUntil`：归一化后的语义字段，供程序逻辑判断和处理
- `RawStatusCode` + `RawBody`：原始信息字段，仅用于日志记录和问题排查，**程序逻辑不应依赖这两个字段**
- 调用方使用示例：
  ```go
  if err.ErrorType == httperror.ErrorTypeRateLimit {
      if err.CooldownUntil != nil {
          waitDuration := time.Until(*err.CooldownUntil)
          // 等待 waitDuration 后重试
      } else {
          // 限流但不知道冷却时长，使用默认退避策略
      }
  }
  ```

---

## 需求

### 需求 1：定义公共 `ErrorType` 枚举（5 个）

**用户故事：** 作为 provider-client 的调用方，我希望归一化后的错误类型简洁明确，只有 5 个语义清晰的类型，以便调用方用最少的分支处理所有错误场景。

#### 验收标准

1. WHEN 定义 `ErrorType` 枚举 THEN 系统 SHALL 包含且仅包含以下 5 个类型：`ErrorTypeBadRequest`、`ErrorTypeUnauthorized`、`ErrorTypeForbidden`、`ErrorTypeRateLimit`、`ErrorTypeServerError`
2. WHEN 为每个 `ErrorType` 添加注释 THEN 系统 SHALL 说明其对应的原始状态码范围、触发条件和可恢复性
3. WHEN 定义 `ErrorTypeForbidden` THEN 系统 SHALL 在注释中说明其涵盖：通用 403 权限不足、账号被封禁、账号需人工重新授权认证等所有"账号不可用"场景
4. WHEN 定义 `ErrorTypeRateLimit` THEN 系统 SHALL 在注释中说明其涵盖：429 限流、月度配额耗尽等所有"需等待后重试"场景，并通过 `CooldownUntil` 字段表达冷却截止时间
5. WHEN 定义 `ErrorTypeServerError` THEN 系统 SHALL 在注释中说明其涵盖：5xx 服务器错误及其他所有非 2xx 未知错误

### 需求 2：定义 `HTTPError` 结构体

**用户故事：** 作为 provider-client 的调试人员，我希望 `HTTPError` 中同时包含归一化后的语义字段和原始 HTTP 信息，以便程序逻辑使用归一化字段，排查问题时使用原始字段。

#### 验收标准

1. WHEN 定义 `HTTPError` 结构体 THEN 系统 SHALL 包含以下字段：
   - `ErrorType ErrorType`：归一化后的错误类型，供程序逻辑使用
   - `ErrorCode int`：归一化后的错误码，与 `ErrorType` 对应（400/401/403/429/500），供程序逻辑使用
   - `Message string`：归一化后的人类可读描述，供程序逻辑使用
   - `CooldownUntil *time.Time`：限流冷却截止时间节点，仅 `ErrorTypeRateLimit` 时有效；`nil` 表示限流但不知道具体冷却时长，非 `nil` 表示有明确的冷却截止时间
   - `RawStatusCode int`：原始 HTTP 状态码，仅用于调试
   - `RawBody []byte`：原始响应体字节，仅用于调试
2. WHEN `HTTPError.Error()` 被调用 THEN 系统 SHALL 返回包含归一化信息和原始状态码的可读字符串，格式为：`"[{ErrorType}({ErrorCode})] {Message} (raw status: {RawStatusCode})"`
3. WHEN 各渠道 `NewHTTPError` 构造 `HTTPError` THEN 系统 SHALL 将传入的 `statusCode` 和 `body` 分别赋值给 `RawStatusCode` 和 `RawBody`
4. WHEN 程序逻辑需要判断错误类型 THEN 系统 SHALL 仅使用 `ErrorType` 或 `ErrorCode` 字段，**禁止**使用 `RawStatusCode` 进行业务判断

### 需求 3：公共包仅提供 `HTTPError` 结构体和 `ErrorType` 枚举，不提供 `ClassifyHTTPError` 函数

**用户故事：** 作为 provider-client 的维护者，我希望公共的 `httperror` 包只定义数据结构，不包含任何归一化逻辑，以便各渠道完全自主地实现自己的错误归一化，避免公共包被渠道特有逻辑污染。

#### 验收标准

1. WHEN 重构 `httperror` 包 THEN 系统 SHALL 移除 `ClassifyHTTPError` 函数，公共包只包含：`ErrorType` 枚举、`HTTPError` 结构体、`ParseCooldownDuration` 工具函数
2. WHEN 各渠道需要实现错误归一化 THEN 系统 SHALL 在各自的 `errors.go` 中自行实现完整的状态码映射和 body 关键字匹配逻辑，不依赖公共包的归一化函数

### 需求 4：各渠道 client 包实现 `NewHTTPError` 作为归一化入口

**用户故事：** 作为 provider-client 的调用方，我希望每个渠道 client 包都暴露一个统一的 `NewHTTPError` 方法，作为将原始 HTTP 响应转换为归一化 `HTTPError` 的唯一入口，以便调用方无需关心各渠道的内部差异。

#### 验收标准

1. WHEN 各渠道 client 包实现错误归一化 THEN 系统 SHALL 在各自的 `errors.go` 文件中定义导出函数 `NewHTTPError(statusCode int, body []byte) *httperror.HTTPError`
2. WHEN `NewHTTPError` 被调用 THEN 系统 SHALL 在函数内部完成：渠道特有的 body 关键字匹配 + 状态码到 `ErrorType`/`ErrorCode` 的映射 + `CooldownUntil` 的计算
3. WHEN 渠道 client 中遇到非 2xx 响应 THEN 系统 SHALL 调用本包的 `NewHTTPError` 而非直接调用 `httperror.ClassifyHTTPError`
4. WHEN 各渠道 `NewHTTPError` 构造 `HTTPError` THEN 系统 SHALL 确保 `RawStatusCode` 和 `RawBody` 始终被正确赋值

### 需求 5：Kiro client 的 `NewHTTPError` 实现

**用户故事：** 作为 provider-client 的调用方，我希望 Kiro client 的 `NewHTTPError` 能够识别 Kiro/AWS CodeWhisperer 特有的错误语义，并将其归一化到标准的 5 个错误类型上。

#### 验收标准

1. WHEN Kiro `NewHTTPError` 处理 `402` 且 body 含 `MONTHLY_REQUEST_COUNT` THEN 系统 SHALL 返回 `ErrorTypeRateLimit`（ErrorCode=429），`CooldownUntil=nil`，`Message` 中包含 "monthly quota exhausted"
2. WHEN Kiro `NewHTTPError` 处理 `403` 且 body 含 `TEMPORARILY_SUSPENDED` THEN 系统 SHALL 返回 `ErrorTypeForbidden`（ErrorCode=403），`Message` 中包含 "account temporarily suspended"
3. WHEN Kiro `NewHTTPError` 处理 `400` THEN 系统 SHALL 返回 `ErrorTypeBadRequest`（ErrorCode=400）
4. WHEN Kiro `NewHTTPError` 处理 `401` THEN 系统 SHALL 返回 `ErrorTypeUnauthorized`（ErrorCode=401）
5. WHEN Kiro `NewHTTPError` 处理 `403`（无特殊 body）THEN 系统 SHALL 返回 `ErrorTypeForbidden`（ErrorCode=403）
6. WHEN Kiro `NewHTTPError` 处理 `429` THEN 系统 SHALL 返回 `ErrorTypeRateLimit`（ErrorCode=429），`CooldownUntil=nil`
7. WHEN Kiro `NewHTTPError` 处理其他非 2xx THEN 系统 SHALL 返回 `ErrorTypeServerError`（ErrorCode=500）
8. WHEN Kiro `NewHTTPError` 构造 `HTTPError` THEN 系统 SHALL 将 `statusCode` 和 `body` 赋值给 `RawStatusCode` 和 `RawBody`

### 需求 6：GeminiCLI client 的 `NewHTTPError` 实现

**用户故事：** 作为 provider-client 的调用方，我希望 GeminiCLI client 的 `NewHTTPError` 能够识别 Google Code Assist API 特有的错误语义，并将其归一化到标准的 5 个错误类型上。

#### 验收标准

1. WHEN GeminiCLI `NewHTTPError` 处理 `403` 且 body 含 `VALIDATION_REQUIRED` THEN 系统 SHALL 返回 `ErrorTypeForbidden`（ErrorCode=403），`Message` 中包含 "account validation required"
2. WHEN GeminiCLI `NewHTTPError` 处理 `429` 且 body 中可解析冷却时长 THEN 系统 SHALL 返回 `ErrorTypeRateLimit`（ErrorCode=429），并将 `time.Now().Add(duration)` 的指针赋值给 `CooldownUntil`
3. WHEN GeminiCLI `NewHTTPError` 处理 `429` 且 body 中无法解析冷却时长 THEN 系统 SHALL 返回 `ErrorTypeRateLimit`（ErrorCode=429），`CooldownUntil=nil`
4. WHEN GeminiCLI `NewHTTPError` 处理 `400` THEN 系统 SHALL 返回 `ErrorTypeBadRequest`（ErrorCode=400）
5. WHEN GeminiCLI `NewHTTPError` 处理 `401` THEN 系统 SHALL 返回 `ErrorTypeUnauthorized`（ErrorCode=401）
6. WHEN GeminiCLI `NewHTTPError` 处理 `403`（无特殊 body）THEN 系统 SHALL 返回 `ErrorTypeForbidden`（ErrorCode=403）
7. WHEN GeminiCLI `NewHTTPError` 处理其他非 2xx THEN 系统 SHALL 返回 `ErrorTypeServerError`（ErrorCode=500）
8. WHEN GeminiCLI `NewHTTPError` 构造 `HTTPError` THEN 系统 SHALL 将 `statusCode` 和 `body` 赋值给 `RawStatusCode` 和 `RawBody`

### 需求 7：CodeBuddy client 的 `NewHTTPError` 实现

**用户故事：** 作为 provider-client 的调用方，我希望 CodeBuddy client 也通过 `NewHTTPError` 返回结构化错误，与其他渠道保持一致的接口。

#### 验收标准

1. WHEN CodeBuddy `NewHTTPError` 被调用 THEN 系统 SHALL 按纯状态码映射处理（CodeBuddy 无特殊错误语义）：400→`ErrorTypeBadRequest`、401→`ErrorTypeUnauthorized`、403→`ErrorTypeForbidden`、429→`ErrorTypeRateLimit`（CooldownUntil=nil）、其他→`ErrorTypeServerError`
2. WHEN `codebuddy.go` 中遇到非 2xx 响应 THEN 系统 SHALL 先读取 body 再关闭，调用本包 `NewHTTPError` 返回结构化错误
3. WHEN CodeBuddy `NewHTTPError` 构造 `HTTPError` THEN 系统 SHALL 将 `statusCode` 和 `body` 赋值给 `RawStatusCode` 和 `RawBody`

### 需求 8：`ParseCooldownDuration` 保留在公共包中

**用户故事：** 作为 provider-client 的维护者，我希望冷却时长解析逻辑保留在公共包中，以便多个渠道复用。

#### 验收标准

1. WHEN 定义公共解析函数 THEN 系统 SHALL 将函数命名为 `ParseCooldownDuration`，返回值类型为 `time.Duration`，解析失败时返回 `0`
2. WHEN GeminiCLI `NewHTTPError` 需要计算 `CooldownUntil` THEN 系统 SHALL 调用 `httperror.ParseCooldownDuration` 获取 `time.Duration`，若返回值 `> 0` 则计算 `t := time.Now().Add(duration)`，将 `&t` 赋值给 `CooldownUntil`；若返回值为 `0` 则 `CooldownUntil=nil`
3. WHEN 各渠道需要解析冷却时长 THEN 系统 SHALL 调用 `httperror.ParseCooldownDuration`，而非自行实现解析逻辑
