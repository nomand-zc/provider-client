package parser

import (
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
)

// errorParser 处理 error 类型消息
type errorParser struct{}

func init() {
	Register(&errorParser{})
}

func (p *errorParser) MessageType() string { return MessageTypeError }
func (p *errorParser) EventType() string   { return "" }

func (p *errorParser) Parse(msg *eventstream.Message) (*providers.Response, error) {
	var errorData map[string]any
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &errorData); err != nil {
			log.Warnf("解析错误消息载荷失败: %v", err)
			errorData = map[string]any{
				"message": string(msg.Payload),
			}
		}
	}

	errorCode := ""
	errorMessage := ""
	if errorData != nil {
		if code, ok := errorData["__type"].(string); ok {
			errorCode = code
		}
		if m, ok := errorData["message"].(string); ok {
			errorMessage = m
		}
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		Done:      true,
		Error: &providers.ResponseError{
			Message: errorMessage,
			Type:    "error",
			Code:    strPtr(errorCode),
		},
	}, nil
}
