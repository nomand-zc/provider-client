# 实施计划

- [ ] 1. 重构公共包 `types/httperror/errors.go`
   - 将 `ErrorType` 枚举收敛为 5 个：`ErrorTypeBadRequest`、`ErrorTypeUnauthorized`、`ErrorTypeForbidden`、`ErrorTypeRateLimit`、`ErrorTypeServerError`，每个类型添加说明原始状态码、触发条件和可恢复性的注释
   - 重新定义 `HTTPError` 结构体，包含 `ErrorType`、`ErrorCode int`、`Message`、`CooldownUntil *time.Time`、`RawStatusCode`、`RawBody` 字段
   - 实现 `Error()` 方法，格式为 `"[{ErrorType}({ErrorCode})] {Message} (raw status: {RawStatusCode})"`
   - 将 `ParseCooldownSeconds` 重命名为 `ParseCooldownDuration`，返回值改为 `time.Duration`，解析失败返回 `0`
   - 移除 `ClassifyHTTPError` 函数
   - _需求：1.1-1.5、2.1-2.4、3.1、8.1_

- [ ] 2. 实现 Kiro client 的 `NewHTTPError`（新建 `client/kiro/errors.go`）
   - 实现导出函数 `NewHTTPError(statusCode int, body []byte) *httperror.HTTPError`
   - 优先匹配渠道特有 body 关键字：`402+MONTHLY_REQUEST_COUNT` → `ErrorTypeRateLimit`（ErrorCode=429，CooldownUntil=nil）；`403+TEMPORARILY_SUSPENDED` → `ErrorTypeForbidden`（ErrorCode=403）
   - 其余状态码按通用映射处理：400/401/403/429/其他非2xx → 对应 ErrorType 和 ErrorCode
   - 始终将 `statusCode` 和 `body` 赋值给 `RawStatusCode` 和 `RawBody`
   - 修改 `kiro.go` 中非 2xx 响应处理，改为调用本包 `NewHTTPError`，移除对 `httperror.ClassifyHTTPError` 的直接调用
   - _需求：4.1-4.4、5.1-5.8_

- [ ] 3. 实现 GeminiCLI client 的 `NewHTTPError`（新建 `client/gemini_cli/errors.go`）
   - 实现导出函数 `NewHTTPError(statusCode int, body []byte) *httperror.HTTPError`
   - 优先匹配渠道特有 body 关键字：`403+VALIDATION_REQUIRED` → `ErrorTypeForbidden`（ErrorCode=403）；`429` → `ErrorTypeRateLimit`（ErrorCode=429），调用 `httperror.ParseCooldownDuration` 解析冷却时长，若 `> 0` 则计算 `t := time.Now().Add(duration)` 赋值给 `CooldownUntil`，否则 `CooldownUntil=nil`
   - 其余状态码按通用映射处理
   - 始终将 `statusCode` 和 `body` 赋值给 `RawStatusCode` 和 `RawBody`
   - 修改 `gemini_cli.go` 中非 2xx 响应处理，改为调用本包 `NewHTTPError`
   - _需求：4.1-4.4、6.1-6.8、8.2-8.3_

- [ ] 4. 实现 CodeBuddy client 的 `NewHTTPError`（新建 `client/codebuddy/errors.go`）
   - 实现导出函数 `NewHTTPError(statusCode int, body []byte) *httperror.HTTPError`，按纯状态码映射处理（无特殊 body 关键字匹配）：400→`ErrorTypeBadRequest`、401→`ErrorTypeUnauthorized`、403→`ErrorTypeForbidden`、429→`ErrorTypeRateLimit`（CooldownUntil=nil）、其他→`ErrorTypeServerError`
   - 始终将 `statusCode` 和 `body` 赋值给 `RawStatusCode` 和 `RawBody`
   - 修改 `codebuddy.go` 中非 2xx 响应处理，先读取 body 再关闭，改为调用本包 `NewHTTPError`
   - _需求：4.1-4.4、7.1-7.3_

- [ ] 5. 编译验证与整体检查
   - 执行 `go build ./...` 确保全量编译通过，无未使用的 import 和编译错误
   - 确认 `httperror` 包中不再存在 `ClassifyHTTPError` 函数及旧的 `ErrorTypeMonthlyQuotaExhausted`、`ErrorTypeQuotaExhausted` 等已废弃常量
   - 确认三个渠道 client 包均不再直接调用 `httperror.ClassifyHTTPError`
   - _需求：3.1-3.2、4.3_
