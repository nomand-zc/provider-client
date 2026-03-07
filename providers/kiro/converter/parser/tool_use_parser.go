package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/nomand-zc/provider-client/providers"
)

// toolUseParser 处理旧格式 toolUseEvent 事件
type toolUseParser struct{}

func init() {
	Register(&toolUseParser{})
}

func (p *toolUseParser) MessageType() string { return MessageTypeEvent }
func (p *toolUseParser) EventType() string   { return EventTypeToolUseEvent }

func (p *toolUseParser) Parse(msg *eventstream.Message) (*providers.Response, error) {
	var evt struct {
		Name      string `json:"name"`
		ToolUseId string `json:"toolUseId"`
		Input     any    `json:"input"`
		Stop      bool   `json:"stop"`
	}
	if err := json.Unmarshal(msg.Payload, &evt); err != nil {
		return nil, fmt.Errorf("解析 toolUseEvent 载荷失败: %w", err)
	}

	toolCall := providers.ToolCall{
		ID:   evt.ToolUseId,
		Type: "function",
		Function: providers.FunctionDefinitionParam{
			Name:      evt.Name,
			Arguments: convertInputToArgs(evt.Input),
		},
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: !evt.Stop,
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
