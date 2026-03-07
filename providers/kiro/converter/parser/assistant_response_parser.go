package parser

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
)

// assistantResponseParser 处理 assistantResponseEvent 事件
type assistantResponseParser struct{}

func init() {
	Register(&assistantResponseParser{})
}

func (p *assistantResponseParser) MessageType() string { return MessageTypeEvent }
func (p *assistantResponseParser) EventType() string   { return EventTypeAssistantResponseEvent }

func (p *assistantResponseParser) Parse(msg *StreamMessage) (*providers.Response, error) {
	payloadStr := string(msg.Payload)

	// 检查是否是工具调用事件
	if isToolCallPayload(payloadStr) {
		return p.parseToolCall(msg)
	}

	// 尝试解析为 JSON
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		// 非 JSON 格式，作为纯文本内容处理
		text := strings.TrimSpace(payloadStr)
		if text == "" {
			return nil, nil
		}
		return &providers.Response{
			Object:    "chat.completion.chunk",
			Created:   time.Now().Unix(),
			Timestamp: time.Now(),
			IsPartial: true,
			Choices: []providers.Choice{
				{
					Index: 0,
					Delta: providers.Message{
						Role:    providers.RoleAssistant,
						Content: text,
					},
				},
			},
		}, nil
	}

	// 提取内容
	content, _ := data["content"].(string)
	if content == "" {
		return nil, nil
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: true,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:    providers.RoleAssistant,
					Content: content,
				},
			},
		},
	}, nil
}

// parseToolCall 处理 assistantResponseEvent 中的工具调用
func (p *assistantResponseParser) parseToolCall(msg *StreamMessage) (*providers.Response, error) {
	var evt struct {
		Name      string `json:"name"`
		ToolUseId string `json:"toolUseId"`
		Input     any    `json:"input"`
		Stop      bool   `json:"stop"`
	}
	if err := json.Unmarshal(msg.Payload, &evt); err != nil {
		log.Warnf("解析工具调用事件失败: %v", err)
		return nil, nil
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
