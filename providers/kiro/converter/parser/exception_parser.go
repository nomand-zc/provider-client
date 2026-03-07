package parser

import (
	"encoding/json"
	"time"

	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/utils"
)

// exceptionParser 处理 exception 类型消息
type exceptionParser struct{}

func init() {
	Register(&exceptionParser{})
}

func (p *exceptionParser) MessageType() string { return MessageTypeException }
func (p *exceptionParser) EventType() string   { return "" }

func (p *exceptionParser) Parse(msg *StreamMessage) (*providers.Response, error) {
	var exceptionData map[string]any
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &exceptionData); err != nil {
			log.Warnf("解析异常消息载荷失败: %v", err)
			exceptionData = map[string]any{
				"message": string(msg.Payload),
			}
		}
	}

	exceptionType := ""
	exceptionMessage := ""
	if exceptionData != nil {
		if eType, ok := exceptionData["__type"].(string); ok {
			exceptionType = eType
		}
		if m, ok := exceptionData["message"].(string); ok {
			exceptionMessage = m
		}
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		Done:      true,
		Error: &providers.ResponseError{
			Message: exceptionMessage,
			Type:    "exception",
			Code:    utils.ToPtr(exceptionType),
		},
	}, nil
}
