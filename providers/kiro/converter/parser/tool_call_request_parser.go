package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nomand-zc/provider-client/providers"
)

// toolCallRequestParser 处理标准 tool_call_request 事件
type toolCallRequestParser struct{}

func init() {
	Register(&toolCallRequestParser{})
}

func (p *toolCallRequestParser) MessageType() string { return MessageTypeEvent }
func (p *toolCallRequestParser) EventType() string   { return EventTypeToolCallRequest }

func (p *toolCallRequestParser) Parse(ctx context.Context, msg *StreamMessage, opts ...OptionFunc) (*providers.Response, error) {
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 tool_call_request 事件载荷失败: %w", err)
	}

	toolCallID, _ := data["toolCallId"].(string)
	toolName, _ := data["toolName"].(string)

	// 解析 input
	args := []byte("{}")
	if inputData, ok := data["input"].(map[string]any); ok && len(inputData) > 0 {
		if argsJSON, err := json.Marshal(inputData); err == nil {
			args = argsJSON
		}
	}

	// 解析选项参数
	parseOpt := &ParseOption{}
	for _, opt := range opts {
		opt(parseOpt)
	}

	// 使用ParseOption中的ToolCallIndexManager获取工具调用索引
	var index int
	if parseOpt.ToolCallIndexManager != nil {
		index = parseOpt.ToolCallIndexManager.GetToolCallIndex(toolCallID)
	}

	toolCall := providers.ToolCall{
		ID:    toolCallID,
		Type:  "function",
		Index: &index, // 设置正确的索引
		Function: providers.FunctionDefinitionParam{
			Name:      toolName,
			Arguments: args,
		},
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: false,
		Done:      false,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:      providers.RoleAssistant,
					ToolCalls: []providers.ToolCall{toolCall},
				},
			},
		},
	}, nil
}
