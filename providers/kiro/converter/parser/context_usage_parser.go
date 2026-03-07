package parser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nomand-zc/provider-client/providers"
)

// contextUsageParser 处理上下文使用量事件
type contextUsageParser struct{}

func init() {
	Register(&contextUsageParser{})
}

func (p *contextUsageParser) MessageType() string { return MessageTypeEvent }
func (p *contextUsageParser) EventType() string   { return EventTypeContextUsageEvent }

func (p *contextUsageParser) Parse(ctx context.Context, msg *StreamMessage, opts ...OptionFunc) (*providers.Response, error) {
	var data struct {
		ContextUsagePercentage float64 `json:"contextUsagePercentage"`
	}
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 contextUsageEvent 载荷失败: %w", err)
	}

	// 上下文使用量为信息性事件，不需要转换为用户可见的响应
	return nil, nil
}
