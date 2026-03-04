# 实施计划

- [ ] 1. 清理空目录并拆分 `converter.go`
   - 删除 `convetor/` 空目录（拼写错误）
   - 新建 `client/kiro/types.go`，将所有 Kiro API 类型定义（`KiroRequest`、`KiroResponse`、`KiroError` 等）从 `converter.go` 迁移至此
   - 新建 `client/kiro/model_mapping.go`，将 `modelMapping` 变量和 `getKiroModel` 函数从 `converter.go` 迁移至此
   - 确保 `converter.go` 仅保留 `KiroConverter` 结构体及其接口方法实现（`ConvertRequest`、`ConvertHeaders`、`ConvertResponse`）和内部辅助函数
   - _需求：1.1、2.1_

- [ ] 2. 消除 header 重复定义
   - 修改 `ConvertHeaders` 方法，以 `options.headers`（即 `defaultOptions.headers`）为基础构建 header map，移除方法内的硬编码 header 列表
   - 确保自定义 header 能正确合并覆盖默认 header
   - _需求：3.1、3.2_

- [ ] 3. 将 `converter` 字段改为接口类型，并内化 `ParseStreamChunk`
   - 修改 `kiro` 结构体，将 `converter` 字段类型从 `*KiroConverter` 改为 `client.Converter` 接口
   - 将 `KiroConverter.ParseStreamChunk` 公开方法重命名为包内私有函数 `parseStreamChunk`，移至 `kiro.go` 或独立文件
   - 更新 `kiro.go` 中所有调用 `ParseStreamChunk` 的地方改为调用私有函数
   - _需求：5.1、5.2、6.1、6.2_

- [ ] 4. 提取公共请求构建方法，消除重复代码
   - 在 `kiro` 结构体上新增私有方法 `buildHTTPRequest`，封装 `ConvertRequest` + `ConvertHeaders` + header 设置逻辑
   - 重构 `GenerateContent` 和 `GenerateContentStream`，均调用 `buildHTTPRequest`，移除各自重复的请求构建代码
   - _需求：4.1、4.2_

- [ ] 5. 修复健壮性问题并删除死代码
   - 修复 `generateResponseID`：改用 `crypto/rand` 或 `fmt.Sprintf` + `rand.Read` 生成唯一 ID，替换 `time.Now().UnixNano()`
   - 修复 Content-Type 判断：将 `!=` 直接比较改为 `strings.HasPrefix` 或 `mime.ParseMediaType`，正确处理带参数的媒体类型
   - 删除 `kiro.go` 中未被调用的 `checkHTTPResponse` 函数
   - _需求：7.1、8.1、8.2、9.1_

- [ ] 6. 更新测试，确保全部通过
   - 检查并更新 `converter_test.go`，适配文件拆分后的包结构变化
   - 为 `parseStreamChunk` 私有函数补充测试（如有必要，通过包内测试文件覆盖）
   - 运行全部测试，确保通过率 100%
   - _需求：2.1、5.2、6.2_
