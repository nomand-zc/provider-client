package providers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== 辅助函数 =====

// intPtr 返回 int 指针
func intPtr(v int) *int { return &v }

// strPtr 返回 string 指针
func strPtr(v string) *string { return &v }

// makeTextChunk 创建一个包含文本内容的流式 chunk
func makeTextChunk(id, model, content string) *Response {
	return &Response{
		ID:    id,
		Model: model,
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					Role:    RoleAssistant,
					Content: content,
				},
			},
		},
		IsPartial: true,
	}
}

// makeDoneChunk 创建一个标记完成的 chunk
func makeDoneChunk(id string) *Response {
	return &Response{
		ID:   id,
		Done: true,
		Choices: []Choice{
			{
				Index:        0,
				FinishReason: strPtr("stop"),
			},
		},
	}
}

// makeToolCallChunk 创建一个包含工具调用增量的 chunk
func makeToolCallChunk(id string, toolIndex int, toolID, toolType, funcName string, args []byte) *Response {
	return &Response{
		ID: id,
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					Role: RoleAssistant,
					ToolCalls: []ToolCall{
						{
							Index: intPtr(toolIndex),
							ID:    toolID,
							Type:  toolType,
							Function: FunctionDefinitionParam{
								Name:      funcName,
								Arguments: args,
							},
						},
					},
				},
			},
		},
		IsPartial: true,
	}
}

// makeUsageChunk 创建一个包含 Usage 信息的 chunk
func makeUsageChunk(id string, prompt, completion, total int) *Response {
	return &Response{
		ID: id,
		Usage: &Usage{
			PromptTokens:     prompt,
			CompletionTokens: completion,
			TotalTokens:      total,
		},
	}
}

// ===== 测试: AddChunk 基本行为 =====

func TestAddChunk_NilChunk(t *testing.T) {
	acc := &ResponseAccumulator{}
	// nil chunk 应当返回 true 且不影响累积状态
	ok := acc.AddChunk(nil)
	require.True(t, ok)
	require.Equal(t, 0, acc.ChunkCount())
	resp := acc.Response()
	require.NotNil(t, resp)
	require.Empty(t, resp.ID)
	require.Empty(t, resp.Choices)
}

func TestAddChunk_EmptyResponse(t *testing.T) {
	acc := &ResponseAccumulator{}
	// 空 Response（无 Choices、无 Usage 等）
	ok := acc.AddChunk(&Response{})
	require.True(t, ok)
	require.Equal(t, 1, acc.ChunkCount())
}

func TestAddChunk_IDMismatch(t *testing.T) {
	acc := &ResponseAccumulator{}
	// 首个 chunk 设置 ID
	ok := acc.AddChunk(&Response{ID: "resp-1"})
	require.True(t, ok)
	// 不同 ID 的 chunk 应当返回 false
	ok = acc.AddChunk(&Response{ID: "resp-2"})
	require.False(t, ok)
	// 确认内部状态未被破坏
	resp := acc.Response()
	require.Equal(t, "resp-1", resp.ID)
}

func TestAddChunk_IDMatch(t *testing.T) {
	acc := &ResponseAccumulator{}
	ok := acc.AddChunk(&Response{ID: "resp-1"})
	require.True(t, ok)
	// 相同 ID 应当正常累积
	ok = acc.AddChunk(&Response{ID: "resp-1"})
	require.True(t, ok)
	require.Equal(t, 2, acc.ChunkCount())
}

func TestAddChunk_EmptyIDAfterSet(t *testing.T) {
	acc := &ResponseAccumulator{}
	ok := acc.AddChunk(&Response{ID: "resp-1"})
	require.True(t, ok)
	// 空 ID 的 chunk 也应当正常累积（不做 ID 检查）
	ok = acc.AddChunk(&Response{ID: ""})
	require.True(t, ok)
	resp := acc.Response()
	require.Equal(t, "resp-1", resp.ID)
}

// ===== 测试: 文本内容累积 =====

func TestAccumulate_TextContent(t *testing.T) {
	acc := &ResponseAccumulator{}

	chunks := []*Response{
		makeTextChunk("resp-1", "gpt-4", "Hello"),
		makeTextChunk("resp-1", "gpt-4", ", "),
		makeTextChunk("resp-1", "gpt-4", "world!"),
	}

	for _, chunk := range chunks {
		ok := acc.AddChunk(chunk)
		require.True(t, ok)
	}

	resp := acc.Response()
	require.Equal(t, "resp-1", resp.ID)
	require.Equal(t, "gpt-4", resp.Model)
	require.Len(t, resp.Choices, 1)
	require.Equal(t, "Hello, world!", resp.Choices[0].Message.Content)
	require.Equal(t, RoleAssistant, resp.Choices[0].Message.Role)
}

func TestAccumulate_EmptyContentChunks(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 空内容的 chunk 也应当正常累积
	chunks := []*Response{
		makeTextChunk("resp-1", "gpt-4", ""),
		makeTextChunk("resp-1", "gpt-4", "Hello"),
		makeTextChunk("resp-1", "gpt-4", ""),
	}

	for _, chunk := range chunks {
		ok := acc.AddChunk(chunk)
		require.True(t, ok)
	}

	resp := acc.Response()
	require.Equal(t, "Hello", resp.Choices[0].Message.Content)
}

// ===== 测试: ReasoningContent 累积 =====

func TestAccumulate_ReasoningContent(t *testing.T) {
	acc := &ResponseAccumulator{}

	chunk1 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					Role:             RoleAssistant,
					ReasoningContent: "Let me think",
				},
			},
		},
	}
	chunk2 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					ReasoningContent: " about this...",
				},
			},
		},
	}
	chunk3 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					Content: "The answer is 42.",
				},
			},
		},
	}

	for _, c := range []*Response{chunk1, chunk2, chunk3} {
		require.True(t, acc.AddChunk(c))
	}

	resp := acc.Response()
	require.Equal(t, "Let me think about this...", resp.Choices[0].Message.ReasoningContent)
	require.Equal(t, "The answer is 42.", resp.Choices[0].Message.Content)
}

// ===== 测试: 工具调用累积 =====

func TestAccumulate_SingleToolCall(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 第一个 chunk: 工具调用头部
	ok := acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_123", "function", "get_weather", nil))
	require.True(t, ok)

	// 第二个 chunk: 参数部分1
	ok = acc.AddChunk(makeToolCallChunk("resp-1", 0, "", "", "", []byte(`{"city"`)))
	require.True(t, ok)

	// 第三个 chunk: 参数部分2
	ok = acc.AddChunk(makeToolCallChunk("resp-1", 0, "", "", "", []byte(`:"Beijing"}`)))
	require.True(t, ok)

	resp := acc.Response()
	require.Len(t, resp.Choices, 1)
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)

	tc := resp.Choices[0].Message.ToolCalls[0]
	require.Equal(t, "call_123", tc.ID)
	require.Equal(t, "function", tc.Type)
	require.Equal(t, "get_weather", tc.Function.Name)
	require.Equal(t, `{"city":"Beijing"}`, string(tc.Function.Arguments))
}

func TestAccumulate_MultipleToolCalls(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 第一个工具调用
	ok := acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_1", "function", "get_weather", []byte(`{"city":"BJ"}`)))
	require.True(t, ok)

	// 第二个工具调用
	ok = acc.AddChunk(makeToolCallChunk("resp-1", 1, "call_2", "function", "get_time", []byte(`{"tz":"UTC"}`)))
	require.True(t, ok)

	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.ToolCalls, 2)

	require.Equal(t, "call_1", resp.Choices[0].Message.ToolCalls[0].ID)
	require.Equal(t, "get_weather", resp.Choices[0].Message.ToolCalls[0].Function.Name)
	require.Equal(t, `{"city":"BJ"}`, string(resp.Choices[0].Message.ToolCalls[0].Function.Arguments))

	require.Equal(t, "call_2", resp.Choices[0].Message.ToolCalls[1].ID)
	require.Equal(t, "get_time", resp.Choices[0].Message.ToolCalls[1].Function.Name)
	require.Equal(t, `{"tz":"UTC"}`, string(resp.Choices[0].Message.ToolCalls[1].Function.Arguments))
}

func TestAccumulate_ToolCallWithNilIndex(t *testing.T) {
	acc := &ResponseAccumulator{}

	// Index 为 nil 时应默认使用 0
	chunk := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					ToolCalls: []ToolCall{
						{
							Index: nil, // nil Index
							ID:    "call_x",
							Type:  "function",
							Function: FunctionDefinitionParam{
								Name:      "test_func",
								Arguments: []byte(`{}`),
							},
						},
					},
				},
			},
		},
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)
	require.Equal(t, "call_x", resp.Choices[0].Message.ToolCalls[0].ID)
}

// ===== 测试: Message 中的工具调用（非 Delta 方式）=====

func TestAccumulate_MessageToolCalls(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 通过 Message 而非 Delta 传递的工具调用
	chunk := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role: RoleAssistant,
					ToolCalls: []ToolCall{
						{
							Index: intPtr(0),
							ID:    "msg_call_1",
							Type:  "function",
							Function: FunctionDefinitionParam{
								Name:      "search",
								Arguments: []byte(`{"query":"test"}`),
							},
						},
					},
				},
				// Delta 中没有工具调用
				Delta: Message{},
			},
		},
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)
	require.Equal(t, "msg_call_1", resp.Choices[0].Message.ToolCalls[0].ID)
	require.Equal(t, "search", resp.Choices[0].Message.ToolCalls[0].Function.Name)
}

func TestAccumulate_MessageToolCallsIgnoredWhenDeltaPresent(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 当 Delta 中有工具调用时，Message 中的工具调用应被忽略
	chunk := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					ToolCalls: []ToolCall{
						{
							Index: intPtr(0),
							ID:    "delta_call",
							Type:  "function",
							Function: FunctionDefinitionParam{
								Name: "delta_func",
							},
						},
					},
				},
				Message: Message{
					ToolCalls: []ToolCall{
						{
							Index: intPtr(0),
							ID:    "msg_call",
							Type:  "function",
							Function: FunctionDefinitionParam{
								Name: "msg_func",
							},
						},
					},
				},
			},
		},
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)
	// 应该是 delta 的工具调用被累积
	require.Equal(t, "delta_call", resp.Choices[0].Message.ToolCalls[0].ID)
	require.Equal(t, "delta_func", resp.Choices[0].Message.ToolCalls[0].Function.Name)
}

// ===== 测试: Usage 累积 =====

func TestAccumulate_Usage(t *testing.T) {
	acc := &ResponseAccumulator{}

	ok := acc.AddChunk(makeUsageChunk("resp-1", 10, 20, 30))
	require.True(t, ok)

	resp := acc.Response()
	require.NotNil(t, resp.Usage)
	require.Equal(t, 10, resp.Usage.PromptTokens)
	require.Equal(t, 20, resp.Usage.CompletionTokens)
	require.Equal(t, 30, resp.Usage.TotalTokens)
}

func TestAccumulate_UsageMultipleChunks(t *testing.T) {
	acc := &ResponseAccumulator{}

	// Usage 在多个 chunk 中累加
	ok := acc.AddChunk(makeUsageChunk("resp-1", 10, 5, 15))
	require.True(t, ok)
	ok = acc.AddChunk(makeUsageChunk("resp-1", 0, 10, 10))
	require.True(t, ok)

	resp := acc.Response()
	require.NotNil(t, resp.Usage)
	require.Equal(t, 10, resp.Usage.PromptTokens)
	require.Equal(t, 15, resp.Usage.CompletionTokens)
	require.Equal(t, 25, resp.Usage.TotalTokens)
}

func TestAccumulate_UsageWithDetails(t *testing.T) {
	acc := &ResponseAccumulator{}

	chunk := &Response{
		ID: "resp-1",
		Usage: &Usage{
			PromptTokens:     50,
			CompletionTokens: 30,
			TotalTokens:      80,
			PromptTokensDetails: PromptTokensDetails{
				CachedTokens:        20,
				CacheCreationTokens: 5,
				CacheReadTokens:     15,
			},
			Credit: 0.01,
		},
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	resp := acc.Response()
	require.NotNil(t, resp.Usage)
	require.Equal(t, 20, resp.Usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 5, resp.Usage.PromptTokensDetails.CacheCreationTokens)
	require.Equal(t, 15, resp.Usage.PromptTokensDetails.CacheReadTokens)
	require.Equal(t, 0.01, resp.Usage.Credit)
}

func TestAccumulate_UsageCreditTakesLatestPositive(t *testing.T) {
	acc := &ResponseAccumulator{}

	chunk1 := &Response{
		ID:    "resp-1",
		Usage: &Usage{Credit: 0.05},
	}
	chunk2 := &Response{
		ID:    "resp-1",
		Usage: &Usage{Credit: 0.0}, // 零值不应覆盖
	}
	chunk3 := &Response{
		ID:    "resp-1",
		Usage: &Usage{Credit: 0.08},
	}

	for _, c := range []*Response{chunk1, chunk2, chunk3} {
		require.True(t, acc.AddChunk(c))
	}

	resp := acc.Response()
	require.Equal(t, 0.08, resp.Usage.Credit)
}

func TestAccumulate_NilUsage(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 没有 Usage 的 chunk 不应创建 Usage 对象
	ok := acc.AddChunk(&Response{ID: "resp-1"})
	require.True(t, ok)

	resp := acc.Response()
	require.Nil(t, resp.Usage)
}

// ===== 测试: 元数据累积 =====

func TestAccumulate_Metadata(t *testing.T) {
	acc := &ResponseAccumulator{}

	fp := "fp_abc123"
	chunk := &Response{
		ID:                "resp-1",
		Object:            ObjectChatCompletion,
		Model:             "gpt-4-turbo",
		Created:           1700000000,
		SystemFingerprint: &fp,
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	resp := acc.Response()
	require.Equal(t, ObjectChatCompletion, resp.Object)
	require.Equal(t, "gpt-4-turbo", resp.Model)
	require.Equal(t, int64(1700000000), resp.Created)
	require.NotNil(t, resp.SystemFingerprint)
	require.Equal(t, "fp_abc123", *resp.SystemFingerprint)
}

func TestAccumulate_MetadataUpdatesWithLatest(t *testing.T) {
	acc := &ResponseAccumulator{}

	ok := acc.AddChunk(&Response{ID: "resp-1", Model: "gpt-3.5"})
	require.True(t, ok)
	ok = acc.AddChunk(&Response{ID: "resp-1", Model: "gpt-4"})
	require.True(t, ok)

	resp := acc.Response()
	// 最新的非空值覆盖
	require.Equal(t, "gpt-4", resp.Model)
}

func TestAccumulate_MetadataEmptyDoesNotOverwrite(t *testing.T) {
	acc := &ResponseAccumulator{}

	ok := acc.AddChunk(&Response{ID: "resp-1", Model: "gpt-4"})
	require.True(t, ok)
	// 空字符串不应覆盖已有值
	ok = acc.AddChunk(&Response{ID: "resp-1", Model: ""})
	require.True(t, ok)

	resp := acc.Response()
	require.Equal(t, "gpt-4", resp.Model)
}

// ===== 测试: Done 和 Error 状态 =====

func TestAccumulate_DoneFlag(t *testing.T) {
	acc := &ResponseAccumulator{}

	ok := acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))
	require.True(t, ok)
	ok = acc.AddChunk(makeDoneChunk("resp-1"))
	require.True(t, ok)

	resp := acc.Response()
	require.True(t, resp.Done)
	require.False(t, resp.IsPartial)
	require.NotNil(t, resp.Choices[0].FinishReason)
	require.Equal(t, "stop", *resp.Choices[0].FinishReason)
}

func TestAccumulate_Error(t *testing.T) {
	acc := &ResponseAccumulator{}

	errChunk := &Response{
		ID: "resp-1",
		Error: &ResponseError{
			Message: "rate limit exceeded",
			Type:    "rate_limit_error",
			Code:    strPtr("429"),
		},
	}

	ok := acc.AddChunk(errChunk)
	require.True(t, ok)

	resp := acc.Response()
	require.NotNil(t, resp.Error)
	require.Equal(t, "rate limit exceeded", resp.Error.Message)
	require.Equal(t, "rate_limit_error", resp.Error.Type)
	require.Equal(t, "429", *resp.Error.Code)
}

func TestAccumulate_ErrorOverwritesPrevious(t *testing.T) {
	acc := &ResponseAccumulator{}

	err1 := &Response{
		ID: "resp-1",
		Error: &ResponseError{
			Message: "error 1",
		},
	}
	err2 := &Response{
		ID: "resp-1",
		Error: &ResponseError{
			Message: "error 2",
		},
	}

	require.True(t, acc.AddChunk(err1))
	require.True(t, acc.AddChunk(err2))

	resp := acc.Response()
	require.Equal(t, "error 2", resp.Error.Message)
}

// ===== 测试: Timestamp =====

func TestAccumulate_Timestamp(t *testing.T) {
	acc := &ResponseAccumulator{}

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	chunk := &Response{
		ID:        "resp-1",
		Timestamp: ts,
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	resp := acc.Response()
	require.Equal(t, ts, resp.Timestamp)
}

func TestAccumulate_TimestampDefaultsToNow(t *testing.T) {
	acc := &ResponseAccumulator{}

	before := time.Now()
	ok := acc.AddChunk(&Response{ID: "resp-1"})
	require.True(t, ok)
	after := time.Now()

	resp := acc.Response()
	// Timestamp 应该在 before 和 after 之间
	require.False(t, resp.Timestamp.Before(before))
	require.False(t, resp.Timestamp.After(after))
}

// ===== 测试: FinishReason =====

func TestAccumulate_FinishReason(t *testing.T) {
	acc := &ResponseAccumulator{}

	ok := acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))
	require.True(t, ok)

	// FinishReason 在流式中间为 nil
	resp := acc.Response()
	require.Nil(t, resp.Choices[0].FinishReason)

	// 最后设置 FinishReason
	ok = acc.AddChunk(makeDoneChunk("resp-1"))
	require.True(t, ok)

	resp = acc.Response()
	require.NotNil(t, resp.Choices[0].FinishReason)
	require.Equal(t, "stop", *resp.Choices[0].FinishReason)
}

func TestAccumulate_FinishReasonToolCalls(t *testing.T) {
	acc := &ResponseAccumulator{}

	ok := acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_1", "function", "test", []byte(`{}`)))
	require.True(t, ok)

	fr := "tool_calls"
	ok = acc.AddChunk(&Response{
		ID:   "resp-1",
		Done: true,
		Choices: []Choice{
			{
				Index:        0,
				FinishReason: &fr,
			},
		},
	})
	require.True(t, ok)

	resp := acc.Response()
	require.NotNil(t, resp.Choices[0].FinishReason)
	require.Equal(t, "tool_calls", *resp.Choices[0].FinishReason)
}

// ===== 测试: JustFinishedContent =====

func TestJustFinishedContent_TriggerOnTransition(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 添加文本 chunk
	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))
	_, ok := acc.JustFinishedContent()
	require.False(t, ok, "内容尚未完成时不应触发")

	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", " world"))
	_, ok = acc.JustFinishedContent()
	require.False(t, ok, "仍在累积文本时不应触发")

	// 添加 Done chunk，状态从 content 转为 finished
	acc.AddChunk(makeDoneChunk("resp-1"))
	content, ok := acc.JustFinishedContent()
	require.True(t, ok, "文本完成后应触发")
	require.Equal(t, "Hello world", content)
}

func TestJustFinishedContent_OnlyTriggersOnce(t *testing.T) {
	acc := &ResponseAccumulator{}

	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))
	acc.AddChunk(makeDoneChunk("resp-1"))

	content, ok := acc.JustFinishedContent()
	require.True(t, ok)
	require.Equal(t, "Hello", content)

	// 再次添加 chunk 后，JustFinished 应重置
	acc.AddChunk(&Response{ID: "resp-1"})
	_, ok = acc.JustFinishedContent()
	require.False(t, ok, "JustFinished 事件应只触发一次")
}

func TestJustFinishedContent_TransitionToToolCall(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 先文本再工具调用
	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Let me check..."))
	acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_1", "function", "search", []byte(`{}`)))

	// 从 content -> tool 的转换应触发 JustFinishedContent
	content, ok := acc.JustFinishedContent()
	require.True(t, ok)
	require.Equal(t, "Let me check...", content)
}

// ===== 测试: JustFinishedToolCall =====

func TestJustFinishedToolCall_TriggerOnTransition(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 工具调用 chunk
	acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_1", "function", "get_weather", []byte(`{"city":"BJ"}`)))
	_, ok := acc.JustFinishedToolCall()
	require.False(t, ok, "工具调用尚未完成时不应触发")

	// 完成 chunk
	acc.AddChunk(makeDoneChunk("resp-1"))
	tc, ok := acc.JustFinishedToolCall()
	require.True(t, ok, "工具调用完成后应触发")
	require.Equal(t, "call_1", tc.ID)
	require.Equal(t, "get_weather", tc.Function.Name)
}

func TestJustFinishedToolCall_TransitionBetweenTools(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 第一个工具调用
	acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_1", "function", "func_a", []byte(`{}`)))

	// 切换到第二个工具调用 → 第一个应该"刚完成"
	acc.AddChunk(makeToolCallChunk("resp-1", 1, "call_2", "function", "func_b", []byte(`{}`)))

	tc, ok := acc.JustFinishedToolCall()
	require.True(t, ok)
	require.Equal(t, "call_1", tc.ID)
	require.Equal(t, "func_a", tc.Function.Name)
}

// ===== 测试: 多 Choice =====

func TestAccumulate_MultipleChoices(t *testing.T) {
	acc := &ResponseAccumulator{}

	// Choice 0
	chunk1 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{Index: 0, Delta: Message{Content: "AAA"}},
		},
	}
	// Choice 1
	chunk2 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{Index: 1, Delta: Message{Content: "BBB"}},
		},
	}
	// Choice 0 继续
	chunk3 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{Index: 0, Delta: Message{Content: "CCC"}},
		},
	}

	for _, c := range []*Response{chunk1, chunk2, chunk3} {
		require.True(t, acc.AddChunk(c))
	}

	resp := acc.Response()
	require.Len(t, resp.Choices, 2)
	require.Equal(t, "AAACCC", resp.Choices[0].Message.Content)
	require.Equal(t, "BBB", resp.Choices[1].Message.Content)
}

func TestAccumulate_SparseChoiceIndex(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 直接使用 Index=2（跳过0和1）
	chunk := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{Index: 2, Delta: Message{Content: "Sparse"}},
		},
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	resp := acc.Response()
	require.Len(t, resp.Choices, 3) // 0, 1, 2
	require.Equal(t, "", resp.Choices[0].Message.Content)
	require.Equal(t, "", resp.Choices[1].Message.Content)
	require.Equal(t, "Sparse", resp.Choices[2].Message.Content)
}

// ===== 测试: Clone 隔离性 =====

func TestResponse_CloneIsolation(t *testing.T) {
	acc := &ResponseAccumulator{}

	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))

	resp1 := acc.Response()

	// 继续累积
	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", " world"))

	resp2 := acc.Response()

	// resp1 不应被后续累积影响
	require.Equal(t, "Hello", resp1.Choices[0].Message.Content)
	require.Equal(t, "Hello world", resp2.Choices[0].Message.Content)
}

// ===== 测试: ChunkCount =====

func TestChunkCount(t *testing.T) {
	acc := &ResponseAccumulator{}

	require.Equal(t, 0, acc.ChunkCount())

	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "a"))
	require.Equal(t, 1, acc.ChunkCount())

	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "b"))
	require.Equal(t, 2, acc.ChunkCount())

	// nil chunk 不增加计数
	acc.AddChunk(nil)
	require.Equal(t, 2, acc.ChunkCount())

	// ID 不匹配的 chunk 不增加计数
	acc.AddChunk(&Response{ID: "different-id"})
	require.Equal(t, 2, acc.ChunkCount())
}

// ===== 测试: 完整流式模拟 =====

func TestAccumulate_FullStreamSimulation(t *testing.T) {
	acc := &ResponseAccumulator{}

	fp := "fp_xyz"
	chunks := []*Response{
		// chunk 1: 角色和模型信息
		{
			ID:                "chatcmpl-123",
			Object:            ObjectChatCompletion,
			Created:           1700000000,
			Model:             "gpt-4-turbo",
			SystemFingerprint: &fp,
			Choices: []Choice{
				{
					Index: 0,
					Delta: Message{
						Role: RoleAssistant,
					},
				},
			},
			IsPartial: true,
		},
		// chunk 2-4: 文本内容
		makeTextChunk("chatcmpl-123", "gpt-4-turbo", "The weather "),
		makeTextChunk("chatcmpl-123", "gpt-4-turbo", "in Beijing "),
		makeTextChunk("chatcmpl-123", "gpt-4-turbo", "is sunny."),
		// chunk 5: Usage 信息
		{
			ID: "chatcmpl-123",
			Usage: &Usage{
				PromptTokens:     15,
				CompletionTokens: 8,
				TotalTokens:      23,
			},
		},
		// chunk 6: 完成
		{
			ID:   "chatcmpl-123",
			Done: true,
			Choices: []Choice{
				{
					Index:        0,
					FinishReason: strPtr("stop"),
				},
			},
		},
	}

	for _, chunk := range chunks {
		ok := acc.AddChunk(chunk)
		require.True(t, ok)
	}

	resp := acc.Response()
	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "gpt-4-turbo", resp.Model)
	assert.Equal(t, int64(1700000000), resp.Created)
	assert.NotNil(t, resp.SystemFingerprint)
	assert.Equal(t, "fp_xyz", *resp.SystemFingerprint)
	assert.True(t, resp.Done)
	assert.False(t, resp.IsPartial)

	require.Len(t, resp.Choices, 1)
	assert.Equal(t, RoleAssistant, resp.Choices[0].Message.Role)
	assert.Equal(t, "The weather in Beijing is sunny.", resp.Choices[0].Message.Content)
	assert.NotNil(t, resp.Choices[0].FinishReason)
	assert.Equal(t, "stop", *resp.Choices[0].FinishReason)

	require.NotNil(t, resp.Usage)
	assert.Equal(t, 15, resp.Usage.PromptTokens)
	assert.Equal(t, 8, resp.Usage.CompletionTokens)
	assert.Equal(t, 23, resp.Usage.TotalTokens)

	assert.Equal(t, 6, acc.ChunkCount())
}

func TestAccumulate_FullToolCallStreamSimulation(t *testing.T) {
	acc := &ResponseAccumulator{}

	chunks := []*Response{
		// 角色设置
		{
			ID: "chatcmpl-456",
			Choices: []Choice{
				{Index: 0, Delta: Message{Role: RoleAssistant}},
			},
		},
		// 文本部分
		makeTextChunk("chatcmpl-456", "", "Let me search for that."),
		// 工具调用头部
		makeToolCallChunk("chatcmpl-456", 0, "call_abc", "function", "web_search", nil),
		// 工具调用参数分段
		makeToolCallChunk("chatcmpl-456", 0, "", "", "", []byte(`{"que`)),
		makeToolCallChunk("chatcmpl-456", 0, "", "", "", []byte(`ry":"weather"}`)),
		// 完成
		{
			ID:   "chatcmpl-456",
			Done: true,
			Choices: []Choice{
				{Index: 0, FinishReason: strPtr("tool_calls")},
			},
		},
	}

	for _, chunk := range chunks {
		require.True(t, acc.AddChunk(chunk))
	}

	resp := acc.Response()
	assert.Equal(t, "chatcmpl-456", resp.ID)
	assert.True(t, resp.Done)

	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "Let me search for that.", resp.Choices[0].Message.Content)
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)

	tc := resp.Choices[0].Message.ToolCalls[0]
	assert.Equal(t, "call_abc", tc.ID)
	assert.Equal(t, "function", tc.Type)
	assert.Equal(t, "web_search", tc.Function.Name)
	assert.Equal(t, `{"query":"weather"}`, string(tc.Function.Arguments))
	assert.Equal(t, "tool_calls", *resp.Choices[0].FinishReason)
}

// ===== 测试: expand 辅助函数 =====

func TestExpandChoices(t *testing.T) {
	tests := []struct {
		name    string
		initial []Choice
		index   int
		wantLen int
	}{
		{
			name:    "空切片扩展到索引0",
			initial: nil,
			index:   0,
			wantLen: 1,
		},
		{
			name:    "空切片扩展到索引2",
			initial: nil,
			index:   2,
			wantLen: 3,
		},
		{
			name:    "已有元素不扩展",
			initial: make([]Choice, 3),
			index:   1,
			wantLen: 3,
		},
		{
			name:    "已有元素需要扩展",
			initial: make([]Choice, 1),
			index:   3,
			wantLen: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandChoices(tt.initial, tt.index)
			require.Len(t, result, tt.wantLen)
		})
	}
}

func TestExpandToolCalls(t *testing.T) {
	tests := []struct {
		name    string
		initial []ToolCall
		index   int
		wantLen int
	}{
		{
			name:    "空切片扩展",
			initial: nil,
			index:   0,
			wantLen: 1,
		},
		{
			name:    "跳跃索引扩展",
			initial: make([]ToolCall, 1),
			index:   5,
			wantLen: 6,
		},
		{
			name:    "不需要扩展",
			initial: make([]ToolCall, 3),
			index:   2,
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandToolCalls(tt.initial, tt.index)
			require.Len(t, result, tt.wantLen)
		})
	}
}

func TestExpandChoiceStates(t *testing.T) {
	// 使用 cap 大于 len 的切片测试中间分支
	base := make([]responseState, 1, 5)
	result := expandChoiceStates(base, 3)
	require.Len(t, result, 4)
	require.Equal(t, 5, cap(result)) // 应复用原始 cap
}

// ===== 测试: Role 累积 =====

func TestAccumulate_RoleFromDelta(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 第一个 chunk 设置 role
	chunk1 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{Index: 0, Delta: Message{Role: RoleAssistant}},
		},
	}
	// 后续 chunk 不设置 role
	chunk2 := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{Index: 0, Delta: Message{Content: "Hello"}},
		},
	}

	require.True(t, acc.AddChunk(chunk1))
	require.True(t, acc.AddChunk(chunk2))

	resp := acc.Response()
	require.Equal(t, RoleAssistant, resp.Choices[0].Message.Role)
	require.Equal(t, "Hello", resp.Choices[0].Message.Content)
}

// ===== 测试: 零值 ResponseAccumulator =====

func TestResponseAccumulator_ZeroValue(t *testing.T) {
	var acc ResponseAccumulator

	require.Equal(t, 0, acc.ChunkCount())

	resp := acc.Response()
	require.NotNil(t, resp)
	require.Empty(t, resp.ID)
	require.Empty(t, resp.Choices)
	require.Nil(t, resp.Usage)

	_, ok := acc.JustFinishedContent()
	require.False(t, ok)

	_, ok = acc.JustFinishedToolCall()
	require.False(t, ok)
}

// ===== 测试: 只有 Usage 的 chunk（无 Choices）=====

func TestAccumulate_OnlyUsageChunk(t *testing.T) {
	acc := &ResponseAccumulator{}

	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))
	acc.AddChunk(makeUsageChunk("resp-1", 10, 5, 15))

	resp := acc.Response()
	require.Equal(t, "Hello", resp.Choices[0].Message.Content)
	require.NotNil(t, resp.Usage)
	require.Equal(t, 10, resp.Usage.PromptTokens)
}

// ===== 测试: 混合 Content 和 ReasoningContent =====

func TestAccumulate_InterleavedContentAndReasoning(t *testing.T) {
	acc := &ResponseAccumulator{}

	chunks := []*Response{
		{
			ID: "resp-1",
			Choices: []Choice{
				{Index: 0, Delta: Message{ReasoningContent: "Step 1: "}},
			},
		},
		{
			ID: "resp-1",
			Choices: []Choice{
				{Index: 0, Delta: Message{ReasoningContent: "analyze. "}},
			},
		},
		{
			ID: "resp-1",
			Choices: []Choice{
				{Index: 0, Delta: Message{Content: "Answer: "}},
			},
		},
		{
			ID: "resp-1",
			Choices: []Choice{
				{Index: 0, Delta: Message{Content: "42"}},
			},
		},
	}

	for _, c := range chunks {
		require.True(t, acc.AddChunk(c))
	}

	resp := acc.Response()
	require.Equal(t, "Step 1: analyze. ", resp.Choices[0].Message.ReasoningContent)
	require.Equal(t, "Answer: 42", resp.Choices[0].Message.Content)
}

// ===== 测试: 连续添加同一个工具的多个参数 chunk =====

func TestAccumulate_ToolCallArgumentsConcatenation(t *testing.T) {
	acc := &ResponseAccumulator{}

	argParts := []string{`{"`, `name`, `":"`, `test`, `","`, `value`, `":`, `123`, `}`}

	// 首个 chunk 带 ID 和函数名
	acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_1", "function", "my_func", nil))

	// 后续只带参数片段
	for _, part := range argParts {
		acc.AddChunk(makeToolCallChunk("resp-1", 0, "", "", "", []byte(part)))
	}

	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)
	require.Equal(t, `{"name":"test","value":123}`, string(resp.Choices[0].Message.ToolCalls[0].Function.Arguments))
}

// ===== 测试: SystemFingerprint 为 nil 不覆盖 =====

func TestAccumulate_SystemFingerprintNilDoesNotOverwrite(t *testing.T) {
	acc := &ResponseAccumulator{}

	fp := "fp_123"
	acc.AddChunk(&Response{
		ID:                "resp-1",
		SystemFingerprint: &fp,
	})
	// nil 不应覆盖
	acc.AddChunk(&Response{
		ID:                "resp-1",
		SystemFingerprint: nil,
	})

	resp := acc.Response()
	require.NotNil(t, resp.SystemFingerprint)
	require.Equal(t, "fp_123", *resp.SystemFingerprint)
}

// ===== 测试: Created 为 0 不覆盖 =====

func TestAccumulate_CreatedZeroDoesNotOverwrite(t *testing.T) {
	acc := &ResponseAccumulator{}

	acc.AddChunk(&Response{ID: "resp-1", Created: 1700000000})
	acc.AddChunk(&Response{ID: "resp-1", Created: 0})

	resp := acc.Response()
	require.Equal(t, int64(1700000000), resp.Created)
}

// ===== 测试: expand 函数 cap 中间分支 =====

func TestExpandChoices_CapBranch(t *testing.T) {
	// 创建 len=1, cap=5 的切片，index=3 应触发 cap 中间分支
	base := make([]Choice, 1, 5)
	base[0] = Choice{Index: 0, Message: Message{Content: "first"}}

	result := expandChoices(base, 3)
	require.Len(t, result, 4)
	require.Equal(t, 5, cap(result))
	require.Equal(t, "first", result[0].Message.Content)
}

func TestExpandToolCalls_CapBranch(t *testing.T) {
	// 创建 len=1, cap=5 的切片，index=3 应触发 cap 中间分支
	base := make([]ToolCall, 1, 5)
	base[0] = ToolCall{ID: "tc_0"}

	result := expandToolCalls(base, 3)
	require.Len(t, result, 4)
	require.Equal(t, 5, cap(result))
	require.Equal(t, "tc_0", result[0].ID)
}

// ===== 测试: update 函数 - Message 中的工具调用（非 Delta 方式）触发 toolState =====

func TestUpdate_MessageToolCallsTriggersToolState(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 通过 Message（非 Delta）传递工具调用，应在 update 中走 default -> Message 分支
	chunk := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					ToolCalls: []ToolCall{
						{
							Index: intPtr(0),
							ID:    "msg_tc",
							Type:  "function",
							Function: FunctionDefinitionParam{
								Name:      "msg_func",
								Arguments: []byte(`{"key":"val"}`),
							},
						},
					},
				},
				Delta: Message{}, // Delta 为空
			},
		},
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	// 添加 Done chunk，应从 toolState 转为 finishedState，触发 JustFinishedToolCall
	acc.AddChunk(makeDoneChunk("resp-1"))
	tc, ok := acc.JustFinishedToolCall()
	require.True(t, ok)
	require.Equal(t, "msg_tc", tc.ID)
	require.Equal(t, "msg_func", tc.Function.Name)
}

func TestUpdate_MessageToolCallsWithNilIndex(t *testing.T) {
	acc := &ResponseAccumulator{}

	// Message 工具调用 Index 为 nil
	chunk := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					ToolCalls: []ToolCall{
						{
							Index: nil, // nil index
							ID:    "msg_tc_nil",
							Type:  "function",
							Function: FunctionDefinitionParam{
								Name: "nil_index_func",
							},
						},
					},
				},
				Delta: Message{},
			},
		},
	}

	ok := acc.AddChunk(chunk)
	require.True(t, ok)

	// 验证默认 index 为 0
	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)
	require.Equal(t, "msg_tc_nil", resp.Choices[0].Message.ToolCalls[0].ID)
}

// ===== 测试: update 函数 - 空 Choices 的 chunk 不触发事件 =====

func TestUpdate_EmptyChoicesChunk(t *testing.T) {
	acc := &ResponseAccumulator{}

	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))
	// 无 Choices 的 chunk（如纯 Usage 信息）
	acc.AddChunk(makeUsageChunk("resp-1", 10, 5, 15))

	_, ok := acc.JustFinishedContent()
	require.False(t, ok, "无 Choices 的 chunk 不应触发 JustFinished 事件")
}

// ===== 测试: JustFinishedToolCall index 超界 =====

func TestJustFinishedToolCall_IndexOutOfRange(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 手动构造一个 justFinished 中 index 超出实际 ToolCalls 长度的场景
	// 这发生在流式中途时工具数量可能还未完全展开
	acc.choiceStates = []responseState{{state: toolState, index: 99}}
	acc.justFinished = responseState{state: toolState, index: 99}
	acc.response = Response{
		Choices: []Choice{
			{
				Message: Message{
					ToolCalls: []ToolCall{
						{ID: "only_one"},
					},
				},
			},
		},
	}

	_, ok := acc.JustFinishedToolCall()
	require.False(t, ok, "index 超界时不应返回 ToolCall")
}

// ===== 测试: JustFinishedContent 无 Choices =====

func TestJustFinishedContent_NoChoices(t *testing.T) {
	acc := &ResponseAccumulator{}
	acc.justFinished = responseState{state: contentState}
	// 响应中没有 Choices
	acc.response = Response{}

	_, ok := acc.JustFinishedContent()
	require.False(t, ok, "没有 Choices 时不应返回内容")
}

// ===== 测试: JustFinishedToolCall 无 Choices =====

func TestJustFinishedToolCall_NoChoices(t *testing.T) {
	acc := &ResponseAccumulator{}
	acc.justFinished = responseState{state: toolState, index: 0}
	// 响应中没有 Choices
	acc.response = Response{}

	_, ok := acc.JustFinishedToolCall()
	require.False(t, ok, "没有 Choices 时不应返回 ToolCall")
}

// ===== 测试: 大量 chunk 压力测试 =====

func TestAccumulate_ManyChunks(t *testing.T) {
	acc := &ResponseAccumulator{}

	for i := 0; i < 1000; i++ {
		ok := acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "x"))
		require.True(t, ok)
	}

	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.Content, 1000)
	require.Equal(t, 1000, acc.ChunkCount())
}

// ===== 测试: update 函数 - 状态不变时不触发 JustFinished =====

func TestUpdate_SameStateSameIndex(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 连续两个相同工具调用的 chunk（相同 toolIndex），状态不应改变
	acc.AddChunk(makeToolCallChunk("resp-1", 0, "call_1", "function", "func_a", []byte(`{"a":`)))
	_, ok := acc.JustFinishedToolCall()
	require.False(t, ok)

	// 同样的 toolIndex=0 继续
	acc.AddChunk(makeToolCallChunk("resp-1", 0, "", "", "", []byte(`"b"}`)))
	_, ok = acc.JustFinishedToolCall()
	require.False(t, ok, "相同状态+相同索引不应触发 JustFinished")
}

// ===== 测试: update 函数 - Delta 工具调用 Index 不为 nil =====

func TestUpdate_DeltaToolCallWithNonNilIndex(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 明确设置 toolIndex=2
	acc.AddChunk(makeToolCallChunk("resp-1", 2, "call_2", "function", "func_c", []byte(`{}`)))

	resp := acc.Response()
	require.Len(t, resp.Choices[0].Message.ToolCalls, 3) // 0, 1, 2
	require.Equal(t, "call_2", resp.Choices[0].Message.ToolCalls[2].ID)
}

// ===== 测试: update 函数 - default/else finishedState 分支 =====

func TestUpdate_DefaultElseFinishedState(t *testing.T) {
	acc := &ResponseAccumulator{}

	// 先设置为文本状态
	acc.AddChunk(makeTextChunk("resp-1", "gpt-4", "Hello"))

	// 接着发一个 Choice 不为空但 Delta 为空、Done=false、Message 无 ToolCalls 的 chunk
	// 这会走 default -> else -> finishedState 分支
	chunk := &Response{
		ID: "resp-1",
		Choices: []Choice{
			{
				Index:        0,
				FinishReason: strPtr("stop"),
				Delta:        Message{}, // Delta 空
				Message:      Message{}, // Message 无 ToolCalls
			},
		},
		Done: false, // 非 Done
	}

	acc.AddChunk(chunk)

	// 从 contentState -> finishedState，应触发 JustFinishedContent
	content, ok := acc.JustFinishedContent()
	require.True(t, ok)
	require.Equal(t, "Hello", content)
}

// ===== 测试: update 函数 - 直接调用 update 覆盖 Choices 为空的防御性分支 =====

func TestUpdate_EmptyChoicesDirectCall(t *testing.T) {
	prev := &responseState{state: contentState}
	chunk := &Response{
		ID:      "resp-1",
		Choices: []Choice{}, // 空 Choices
	}

	justFinished := prev.update(chunk)

	// 空 Choices 时应返回零值 responseState
	require.Equal(t, responseState{}, justFinished)
	// prev 不应被修改
	require.Equal(t, contentState, prev.state)
}
