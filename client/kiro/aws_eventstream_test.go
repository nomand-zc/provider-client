package kiro

import (
	"testing"
)

// buildKiroRawData 构建包含多个 JSON 事件的原始字节数据（模拟 Kiro 实际返回的二进制流）
// Kiro 实际返回的是二进制帧头 + JSON payload 的混合格式，
// 新实现直接扫描字符串中的 JSON 对象，因此测试数据只需包含 JSON 片段即可
func buildKiroRawData(jsonFragments ...string) []byte {
	var result []byte
	for _, frag := range jsonFragments {
		// 模拟二进制帧头（随机字节）+ JSON payload
		// 实际帧头是二进制，但解析器只扫描 JSON，所以帧头内容不影响结果
		result = append(result, []byte("\x00\x00\x00\x50\x00\x00\x00\x20\x00\x00\x00\x00")...)
		result = append(result, []byte(frag)...)
		result = append(result, []byte("\x00\x00\x00\x00")...)
	}
	return result
}

// TestParseAWSEventStream_BasicContent 测试基本内容帧解析
func TestParseAWSEventStream_BasicContent(t *testing.T) {
	// 直接构造包含 assistantResponseEvent JSON 的原始数据
	data := buildKiroRawData(
		`{"content":"Hello, "}`,
		`{"content":"World!"}`,
	)

	result, err := ParseAWSEventStream(data)
	if err != nil {
		t.Fatalf("ParseAWSEventStream 失败: %v", err)
	}

	if result.Content != "Hello, World!" {
		t.Errorf("内容不匹配，期望 'Hello, World!'，实际: %q", result.Content)
	}

	t.Logf("✅ 基本内容帧解析测试通过，内容: %s", result.Content)
}

// TestParseAWSEventStream_WithContextUsage 测试包含 contextUsagePercentage 的解析（token 用量估算）
func TestParseAWSEventStream_WithContextUsage(t *testing.T) {
	// contextUsagePercentage=1.0 表示使用了 1% 的上下文（172500 * 0.01 = 1725 tokens）
	data := buildKiroRawData(
		`{"content":"2"}`,
		`{"contextUsagePercentage":1.0}`,
	)

	result, err := ParseAWSEventStream(data)
	if err != nil {
		t.Fatalf("ParseAWSEventStream 失败: %v", err)
	}

	if result.Content != "2" {
		t.Errorf("内容不匹配，期望 '2'，实际: %q", result.Content)
	}
	if result.Usage == nil {
		t.Fatal("Usage 不应为 nil（contextUsagePercentage > 0 时应估算 token 用量）")
	}
	if result.Usage.TotalTokens <= 0 {
		t.Errorf("TotalTokens 应大于 0，实际: %d", result.Usage.TotalTokens)
	}
	if result.Usage.InputTokens < 0 {
		t.Errorf("InputTokens 不应为负数，实际: %d", result.Usage.InputTokens)
	}

	t.Logf("✅ contextUsagePercentage 解析测试通过，内容: %s, 用量: %+v", result.Content, result.Usage)
}

// TestParseAWSEventStreamChunks_StreamingChunks 测试流式块解析
func TestParseAWSEventStreamChunks_StreamingChunks(t *testing.T) {
	// 3 个内容片段 + 1 个 contextUsagePercentage
	data := buildKiroRawData(
		`{"content":"Hello"}`,
		`{"content":", "}`,
		`{"content":"World!"}`,
		`{"contextUsagePercentage":0.5}`,
	)

	chunks, err := ParseAWSEventStreamChunks(data)
	if err != nil {
		t.Fatalf("ParseAWSEventStreamChunks 失败: %v", err)
	}

	// 应有 1 个聚合内容块 + 1 个 usage 块
	// 注意：新实现将所有内容聚合为一个 chunk，而非逐帧拆分
	if len(chunks) < 1 {
		t.Fatalf("期望至少 1 个块，实际: %d", len(chunks))
	}

	// 验证内容块（第一个块包含聚合内容）
	if chunks[0].Content != "Hello, World!" {
		t.Errorf("内容块不匹配，期望 'Hello, World!'，实际: %q", chunks[0].Content)
	}

	// 验证 usage 块（最后一个块）
	lastChunk := chunks[len(chunks)-1]
	if lastChunk.Usage == nil {
		t.Fatal("最后一个块的 Usage 不应为 nil")
	}
	if lastChunk.Usage.TotalTokens <= 0 {
		t.Errorf("TotalTokens 应大于 0，实际: %d", lastChunk.Usage.TotalTokens)
	}

	t.Logf("✅ 流式块解析测试通过，共 %d 个块", len(chunks))
}

// TestParseAWSEventStream_EmptyData 测试空数据处理
func TestParseAWSEventStream_EmptyData(t *testing.T) {
	result, err := ParseAWSEventStream([]byte{})
	if err != nil {
		t.Fatalf("空数据不应返回错误: %v", err)
	}
	if result.Content != "" {
		t.Errorf("空数据的内容应为空，实际: %q", result.Content)
	}

	t.Logf("✅ 空数据处理测试通过")
}

// TestParseAWSEventStream_UnknownFields 测试未知字段被忽略
func TestParseAWSEventStream_UnknownFields(t *testing.T) {
	// 包含未知字段的 JSON 应被忽略（不影响内容提取）
	data := buildKiroRawData(
		`{"content":"hello"}`,
		`{"someUnknownField":"someValue","anotherField":123}`,
	)

	result, err := ParseAWSEventStream(data)
	if err != nil {
		t.Fatalf("ParseAWSEventStream 失败: %v", err)
	}

	if result.Content != "hello" {
		t.Errorf("内容不匹配，期望 'hello'，实际: %q", result.Content)
	}

	t.Logf("✅ 未知字段忽略测试通过")
}

// TestParseAWSEventStream_ToolUse 测试工具调用解析
func TestParseAWSEventStream_ToolUse(t *testing.T) {
	// 模拟 Kiro 工具调用事件：先发送 name+id，再发送 input+id
	data := buildKiroRawData(
		`{"name":"get_weather","toolUseId":"tool-123"}`,
		`{"input":{"city":"Beijing"},"toolUseId":"tool-123"}`,
	)

	result, err := ParseAWSEventStream(data)
	if err != nil {
		t.Fatalf("ParseAWSEventStream 失败: %v", err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("期望 1 个工具调用，实际: %d", len(result.ToolCalls))
	}

	tc := result.ToolCalls[0]
	if tc.ID != "tool-123" {
		t.Errorf("工具调用 ID 不匹配，期望 'tool-123'，实际: %q", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("工具调用名称不匹配，期望 'get_weather'，实际: %q", tc.Name)
	}
	if tc.Arguments == nil {
		t.Fatal("工具调用参数不应为 nil")
	}
	if city, ok := tc.Arguments["city"].(string); !ok || city != "Beijing" {
		t.Errorf("工具调用参数不匹配，期望 city='Beijing'，实际: %v", tc.Arguments)
	}

	t.Logf("✅ 工具调用解析测试通过，工具: %s, 参数: %v", tc.Name, tc.Arguments)
}

// TestParseAWSEventStream_MultipleToolUses 测试多个工具调用解析
func TestParseAWSEventStream_MultipleToolUses(t *testing.T) {
	data := buildKiroRawData(
		`{"name":"tool_a","toolUseId":"id-1"}`,
		`{"input":{"param":"value1"},"toolUseId":"id-1"}`,
		`{"name":"tool_b","toolUseId":"id-2"}`,
		`{"input":{"param":"value2"},"toolUseId":"id-2"}`,
	)

	result, err := ParseAWSEventStream(data)
	if err != nil {
		t.Fatalf("ParseAWSEventStream 失败: %v", err)
	}

	if len(result.ToolCalls) != 2 {
		t.Fatalf("期望 2 个工具调用，实际: %d", len(result.ToolCalls))
	}

	if result.ToolCalls[0].Name != "tool_a" {
		t.Errorf("第一个工具名称不匹配: %q", result.ToolCalls[0].Name)
	}
	if result.ToolCalls[1].Name != "tool_b" {
		t.Errorf("第二个工具名称不匹配: %q", result.ToolCalls[1].Name)
	}

	t.Logf("✅ 多工具调用解析测试通过")
}

// TestExtractKiroJSONFragments_DirectJSON 测试直接 JSON 字符串解析（无二进制帧头）
func TestExtractKiroJSONFragments_DirectJSON(t *testing.T) {
	// 直接传入纯 JSON 字符串（模拟从二进制流中提取的字符串部分）
	rawStr := `{"content":"Hello"} some binary garbage {"content":" World"} more garbage {"contextUsagePercentage":2.5}`

	result := extractKiroJSONFragments(rawStr)

	if result.Content != "Hello World" {
		t.Errorf("内容不匹配，期望 'Hello World'，实际: %q", result.Content)
	}
	if result.ContextUsagePct != 2.5 {
		t.Errorf("ContextUsagePct 不匹配，期望 2.5，实际: %f", result.ContextUsagePct)
	}
	if !result.Stop {
		t.Error("Stop 应为 true（contextUsagePercentage 存在时）")
	}

	t.Logf("✅ 直接 JSON 字符串解析测试通过")
}

// TestFindKiroMatchingBrace 测试花括号匹配
func TestFindKiroMatchingBrace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		start    int
		expected int
	}{
		{"简单对象", `{"key":"value"}`, 0, 14},
		{"嵌套对象", `{"a":{"b":"c"}}`, 0, 14},
		{"含字符串中的括号", `{"key":"{not a brace}"}`, 0, 22},
		{"无效起始", `not a brace`, 0, -1},
		{"空字符串", ``, 0, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findKiroMatchingBrace(tt.input, tt.start)
			if result != tt.expected {
				t.Errorf("期望 %d，实际: %d", tt.expected, result)
			}
		})
	}

	t.Logf("✅ 花括号匹配测试通过")
}
