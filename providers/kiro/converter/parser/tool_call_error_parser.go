package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nomand-zc/provider-client/providers"
)

// toolCallErrorParser 处理工具调用错误事件
type toolCallErrorParser struct{}

func init() {
	Register(&toolCallErrorParser{})
}

func (p *toolCallErrorParser) MessageType() string { return MessageTypeEvent }
func (p *toolCallErrorParser) EventType() string   { return EventTypeToolCallError }

func (p *toolCallErrorParser) Parse(ctx context.Context, msg *StreamMessage, opts ...OptionFunc) (*providers.Response, error) {
	var errorInfo struct {
		ToolCallID string `json:"tool_call_id"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(msg.Payload, &errorInfo); err != nil {
		return nil, fmt.Errorf("解析 tool_call_error 载荷失败: %w", err)
	}

	errMsg := fmt.Sprintf("tool_call_error: %s (tool_call_id: %s)", errorInfo.Error, errorInfo.ToolCallID)
	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		Done:      true,
		Error: &providers.ResponseError{
			Message: errMsg,
			Type:    "tool_call_error",
		},
	}, nil
}
