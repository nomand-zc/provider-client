package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/nomand-zc/provider-client/providers"
)

// toolCallRequestParser 处理标准 tool_call_request 事件
type toolCallRequestParser struct{}

func init() {
	Register(&toolCallRequestParser{})
}

func (p *toolCallRequestParser) MessageType() string { return MessageTypeEvent }
func (p *toolCallRequestParser) EventType() string   { return EventTypeToolCallRequest }

func (p *toolCallRequestParser) Parse(msg *eventstream.Message) (*providers.Response, error) {
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

	toolCall := providers.ToolCall{
		ID:   toolCallID,
		Type: "function",
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
