package parser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nomand-zc/provider-client/providers"
)

// completionChunkParser 处理 completion_chunk 事件（流式增量）
type completionChunkParser struct{}

func init() {
	Register(&completionChunkParser{})
}

func (p *completionChunkParser) MessageType() string { return MessageTypeEvent }
func (p *completionChunkParser) EventType() string   { return EventTypeCompletionChunk }

func (p *completionChunkParser) Parse(ctx context.Context, msg *StreamMessage, opts ...OptionFunc) (*providers.Response, error) {
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 completion_chunk 事件载荷失败: %w", err)
	}

	content, _ := data["content"].(string)
	delta, _ := data["delta"].(string)
	finishReason, _ := data["finish_reason"].(string)

	// 使用 delta 作为实际的文本增量，如果没有则使用 content
	textDelta := delta
	if textDelta == "" {
		textDelta = content
	}

	resp := providers.NewResponse(ctx,
		providers.WithObject(providers.ObjectChatCompletion),
		providers.WithChoices(providers.Choice{
			Index: 0,
			Delta: providers.Message{
				Role:    providers.RoleAssistant,
				Content: textDelta,
			},
		}),
	)

	// 如果有完成原因，标记为最终响应
	if finishReason != "" {
		resp.Choices[0].FinishReason = &finishReason
		resp.Done = true
		resp.IsPartial = false
	}

	return resp, nil
}
