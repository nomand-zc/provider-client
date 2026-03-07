package providers

import (
	"time"

	"github.com/nomand-zc/provider-client/utils"
)

// responseStateEnum 表示响应累积过程中的状态枚举
type responseStateEnum int

const (
	emptyState    responseStateEnum = iota // 空状态
	contentState                           // 正在累积文本内容
	toolState                              // 正在累积工具调用
	finishedState                          // 已完成
)

// responseState 记录每个 Choice 的当前累积状态
type responseState struct {
	state responseStateEnum
	index int // 当前工具调用的索引（仅 toolState 时有效）
}

// FinishedToolCall 表示一个已完成累积的工具调用
type FinishedToolCall struct {
	ToolCall
}

// ResponseAccumulator 用于将流式 Response chunk 累积成一个完整的 Response。
// 参考 openai-go 的 ChatCompletionAccumulator 设计模式。
//
// 用法示例：
//
//	acc := &providers.ResponseAccumulator{}
//	for {
//	    chunk, err := reader.Read(ctx)
//	    if err != nil { break }
//	    acc.AddChunk(chunk)
//	}
//	resp := acc.Response()
type ResponseAccumulator struct {
	// response 是累积后的完整响应
	response Response
	// choiceStates 跟踪每个 Choice 的累积状态
	choiceStates []responseState
	// justFinished 记录刚刚完成的状态（用于事件通知）
	justFinished responseState
	// chunkCount 记录已累积的 chunk 数量
	chunkCount int
}

// AddChunk 将一个流式 Response chunk 累积到当前响应中。
// chunk 必须按顺序添加。
// 返回 false 表示 chunk 无法被成功累积（例如 ID 不匹配）。
func (acc *ResponseAccumulator) AddChunk(chunk *Response) bool {
	if chunk == nil {
		return true
	}

	acc.justFinished = responseState{}

	if !acc.accumulateDelta(chunk) {
		return false
	}

	// 只有包含 Choices 的 chunk 才可能触发完成事件
	if len(chunk.Choices) == 0 {
		return true
	}

	chunkIndex := chunk.Choices[0].Index
	acc.choiceStates = expandChoiceStates(acc.choiceStates, chunkIndex)
	acc.justFinished = acc.choiceStates[chunkIndex].update(chunk)
	return true
}

// JustFinishedContent 获取刚刚完成的文本内容。
// 当最近添加的 chunk 不再包含文本 delta 时，内容被认为"刚刚完成"。
// 如果内容刚刚完成，返回内容字符串和 true；否则返回空字符串和 false。
func (acc *ResponseAccumulator) JustFinishedContent() (content string, ok bool) {
	if acc.justFinished.state == contentState && len(acc.response.Choices) > 0 {
		return acc.response.Choices[0].Message.Content, true
	}
	return "", false
}

// JustFinishedToolCall 获取刚刚完成的工具调用。
// 当最近添加的 chunk 不再包含该工具调用的 delta，或者包含不同工具调用的 delta 时，
// 工具调用被认为"刚刚完成"。
func (acc *ResponseAccumulator) JustFinishedToolCall() (toolCall FinishedToolCall, ok bool) {
	if acc.justFinished.state == toolState && len(acc.response.Choices) > 0 {
		toolCalls := acc.response.Choices[0].Message.ToolCalls
		if acc.justFinished.index < len(toolCalls) {
			return FinishedToolCall{
				ToolCall: toolCalls[acc.justFinished.index],
			}, true
		}
	}
	return FinishedToolCall{}, false
}

// Response 返回累积后的完整 Response。
// 返回的是副本，后续的 AddChunk 调用不会影响已返回的结果。
func (acc *ResponseAccumulator) Response() *Response {
	return acc.response.Clone()
}

// ChunkCount 返回已累积的 chunk 数量
func (acc *ResponseAccumulator) ChunkCount() int {
	return acc.chunkCount
}

// accumulateDelta 将一个 chunk 的增量数据合并到累积的响应中。
// 返回 false 表示检测到不匹配（如 ID 不一致）。
func (acc *ResponseAccumulator) accumulateDelta(chunk *Response) bool {
	cc := &acc.response

	// ID 匹配检查：首个 chunk 设置 ID，后续 chunk 必须匹配
	if cc.ID == "" {
		cc.ID = chunk.ID
	} else if chunk.ID != "" && cc.ID != chunk.ID {
		return false
	}

	// 累积每个 Choice
	for _, delta := range chunk.Choices {
		cc.Choices = expandChoices(cc.Choices, delta.Index)
		choice := &cc.Choices[delta.Index]

		choice.Index = delta.Index

		// FinishReason 取最新的非空值
		if delta.FinishReason != nil {
			choice.FinishReason = delta.FinishReason
		}

		// 累积 Delta 中的角色信息到 Message
		if delta.Delta.Role != "" {
			choice.Message.Role = delta.Delta.Role
		}

		// 拼接文本内容
		choice.Message.Content += delta.Delta.Content

		// 拼接推理内容（如 deepseek think content）
		choice.Message.ReasoningContent += delta.Delta.ReasoningContent

		// 累积工具调用
		// 优先处理 Delta 中的工具调用
		for _, deltaTool := range delta.Delta.ToolCalls {
			toolIndex := 0
			if deltaTool.Index != nil {
				toolIndex = *deltaTool.Index
			}

			choice.Message.ToolCalls = expandToolCalls(choice.Message.ToolCalls, toolIndex)
			tool := &choice.Message.ToolCalls[toolIndex]

			if deltaTool.ID != "" {
				tool.ID = deltaTool.ID
			}
			if deltaTool.Type != "" {
				tool.Type = deltaTool.Type
			}
			tool.Index = utils.ToPtr(toolIndex)
			if deltaTool.Function.Name != "" {
				tool.Function.Name = deltaTool.Function.Name
			}
			tool.Function.Arguments = append(tool.Function.Arguments, deltaTool.Function.Arguments...)
		}

		// 只有在 Delta 中没有工具调用时，才累积 Message 中的工具调用
		if len(delta.Delta.ToolCalls) == 0 {
			for _, msgTool := range delta.Message.ToolCalls {
				toolIndex := 0
				if msgTool.Index != nil {
					toolIndex = *msgTool.Index
				}

				choice.Message.ToolCalls = expandToolCalls(choice.Message.ToolCalls, toolIndex)
				tool := &choice.Message.ToolCalls[toolIndex]

				if msgTool.ID != "" {
					tool.ID = msgTool.ID
				}
				if msgTool.Type != "" {
					tool.Type = msgTool.Type
				}
				tool.Index = utils.ToPtr(toolIndex)
				if msgTool.Function.Name != "" {
					tool.Function.Name = msgTool.Function.Name
				}
				tool.Function.Arguments = append(tool.Function.Arguments, msgTool.Function.Arguments...)
			}
		}
	}

	// 累积 Usage
	if chunk.Usage != nil {
		if cc.Usage == nil {
			cc.Usage = &Usage{}
		}
		cc.Usage.CompletionTokens += chunk.Usage.CompletionTokens
		cc.Usage.PromptTokens += chunk.Usage.PromptTokens
		cc.Usage.TotalTokens += chunk.Usage.TotalTokens
		cc.Usage.PromptTokensDetails.CachedTokens += chunk.Usage.PromptTokensDetails.CachedTokens
		cc.Usage.PromptTokensDetails.CacheCreationTokens += chunk.Usage.PromptTokensDetails.CacheCreationTokens
		cc.Usage.PromptTokensDetails.CacheReadTokens += chunk.Usage.PromptTokensDetails.CacheReadTokens
		if chunk.Usage.Credit > 0 {
			cc.Usage.Credit = chunk.Usage.Credit
		}
	}

	// 更新元数据（取最新值）
	if chunk.Model != "" {
		cc.Model = chunk.Model
	}
	if chunk.Created != 0 {
		cc.Created = chunk.Created
	}
	if chunk.SystemFingerprint != nil {
		cc.SystemFingerprint = chunk.SystemFingerprint
	}
	if chunk.Object != "" {
		cc.Object = chunk.Object
	}

	// 更新完成状态
	if chunk.Done {
		cc.Done = true
		cc.IsPartial = false
	}

	// 更新错误信息
	if chunk.Error != nil {
		cc.Error = chunk.Error
	}

	// 更新时间戳为最新 chunk 的时间
	if !chunk.Timestamp.IsZero() {
		cc.Timestamp = chunk.Timestamp
	} else {
		cc.Timestamp = time.Now()
	}

	acc.chunkCount++
	return true
}

// update 更新 Choice 的内部响应状态，并返回之前的状态（如果状态发生了变化）。
// 这确保 JustFinished 事件只触发一次。
func (prev *responseState) update(chunk *Response) (justFinished responseState) {
	if len(chunk.Choices) == 0 {
		return responseState{}
	}

	delta := chunk.Choices[0].Delta
	current := responseState{}

	switch {
	case delta.Content != "":
		current.state = contentState
	case len(delta.ToolCalls) > 0:
		current.state = toolState
		toolIndex := 0
		if delta.ToolCalls[0].Index != nil {
			toolIndex = *delta.ToolCalls[0].Index
		}
		current.index = toolIndex
	case chunk.Done:
		current.state = finishedState
	default:
		// 检查 Message 中是否有工具调用（非 Delta 方式）
		if len(chunk.Choices[0].Message.ToolCalls) > 0 {
			current.state = toolState
			toolIndex := 0
			if chunk.Choices[0].Message.ToolCalls[0].Index != nil {
				toolIndex = *chunk.Choices[0].Message.ToolCalls[0].Index
			}
			current.index = toolIndex
		} else {
			current.state = finishedState
		}
	}

	if *prev != current {
		justFinished = *prev
	}
	*prev = current

	return
}

// expandChoices 扩展 Choice 切片以适应指定的索引
func expandChoices(slice []Choice, index int) []Choice {
	if index < len(slice) {
		return slice
	}
	if index < cap(slice) {
		return slice[:index+1]
	}
	newSlice := make([]Choice, index+1)
	copy(newSlice, slice)
	return newSlice
}

// expandChoiceStates 扩展 responseState 切片以适应指定的索引
func expandChoiceStates(slice []responseState, index int) []responseState {
	if index < len(slice) {
		return slice
	}
	if index < cap(slice) {
		return slice[:index+1]
	}
	newSlice := make([]responseState, index+1)
	copy(newSlice, slice)
	return newSlice
}

// expandToolCalls 扩展 ToolCall 切片以适应指定的索引
func expandToolCalls(slice []ToolCall, index int) []ToolCall {
	if index < len(slice) {
		return slice
	}
	if index < cap(slice) {
		return slice[:index+1]
	}
	newSlice := make([]ToolCall, index+1)
	copy(newSlice, slice)
	return newSlice
}
