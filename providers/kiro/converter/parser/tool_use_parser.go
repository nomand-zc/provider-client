package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nomand-zc/provider-client/providers"
)

// toolUseParser 处理旧格式 toolUseEvent 事件
type toolUseParser struct{}

func init() {
	Register(&toolUseParser{})
}

func (p *toolUseParser) MessageType() string { return MessageTypeEvent }
func (p *toolUseParser) EventType() string   { return EventTypeToolUseEvent }

func (p *toolUseParser) Parse(ctx context.Context, msg *StreamMessage, opts ...OptionFunc) (*providers.Response, error) {
	var evt struct {
		Name      string `json:"name"`
		ToolUseId string `json:"toolUseId"`
		Input     any    `json:"input"`
		Stop      bool   `json:"stop"`
	}
	if err := json.Unmarshal(msg.Payload, &evt); err != nil {
		return nil, fmt.Errorf("解析 toolUseEvent 载荷失败: %w", err)
	}

	// 解析选项参数
	parseOpt := &ParseOption{}
	for _, opt := range opts {
		opt(parseOpt)
	}

	// 使用ParseOption中的ToolCallIndexManager获取工具调用索引
	var index int
	if parseOpt.ToolCallIndexManager != nil {
		index = parseOpt.ToolCallIndexManager.GetToolCallIndex(evt.ToolUseId)
	}

	toolCall := providers.ToolCall{
		ID:    evt.ToolUseId,
		Type:  "function",
		Index: &index, // 设置正确的索引
		Function: providers.FunctionDefinitionParam{
			Name:      evt.Name,
			Arguments: convertInputToArgs(evt.Input),
		},
	}

	return &providers.Response{
		Object:    providers.ObjectChatCompletion,
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
