package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/nomand-zc/provider-client/providers"
)

// completionParser 处理 completion 事件（完整响应）
type completionParser struct{}

func init() {
	Register(&completionParser{})
}

func (p *completionParser) MessageType() string { return MessageTypeEvent }
func (p *completionParser) EventType() string   { return EventTypeCompletion }

func (p *completionParser) Parse(msg *eventstream.Message) (*providers.Response, error) {
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 completion 事件载荷失败: %w", err)
	}

	content, _ := data["content"].(string)
	finishReason, _ := data["finish_reason"].(string)

	resp := &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: false,
		Done:      true,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:    providers.RoleAssistant,
					Content: content,
				},
			},
		},
	}

	// 设置完成原因
	if finishReason != "" {
		resp.Choices[0].FinishReason = &finishReason
	}

	// 处理工具调用
	toolCalls := parseToolCalls(data)
	if len(toolCalls) > 0 {
		resp.Choices[0].Delta.ToolCalls = toolCalls
		resp.Done = false
	}

	return resp, nil
}
