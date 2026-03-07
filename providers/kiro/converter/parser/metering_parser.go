package parser

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
)

// meteringParser 处理计量事件
type meteringParser struct{}

func init() {
	Register(&meteringParser{})
}

func (p *meteringParser) MessageType() string { return MessageTypeEvent }
func (p *meteringParser) EventType() string   { return EventTypeMeteringEvent }

func (p *meteringParser) Parse(msg *StreamMessage) (*providers.Response, error) {
	var data struct {
		Unit       string  `json:"unit"`
		UnitPlural string  `json:"unitPlural"`
		Usage      float64 `json:"usage"`
	}
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 meteringEvent 载荷失败: %w", err)
	}

	log.Debugf("计量事件: unit=%s, usage=%f", data.Unit, data.Usage)

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: true,
		Usage: &providers.Usage{
			Credit: data.Usage,
		},
	}, nil
}
