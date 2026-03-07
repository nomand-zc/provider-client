package parser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nomand-zc/provider-client/providers"
)

// meteringParser 处理计量事件
type meteringParser struct{}

func init() {
	Register(&meteringParser{})
}

func (p *meteringParser) MessageType() string { return MessageTypeEvent }
func (p *meteringParser) EventType() string   { return EventTypeMeteringEvent }

func (p *meteringParser) Parse(ctx context.Context, msg *StreamMessage, opts ...OptionFunc) (*providers.Response, error) {
	var data struct {
		Unit       string  `json:"unit"`
		UnitPlural string  `json:"unitPlural"`
		Usage      float64 `json:"usage"`
	}
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 meteringEvent 载荷失败: %w", err)
	}

	return providers.NewResponse(ctx,
		providers.WithObject(providers.ObjectChatCompletion),
		providers.WithUsage(&providers.Usage{
			Credit: data.Usage,
		}),
	), nil
}
