package converter

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/parser"
)

// ConvertResponse 将 Kiro CodeWhisperer 响应转换为通用响应格式
// 通过 parser 注册器根据 messageType 和 eventType 获取对应的解析器来处理
func ConvertResponse(_ context.Context, resp *eventstream.Message) (
	*providers.Response, error) {
	if resp == nil {
		return nil, nil
	}

	jsonData, _ := json.Marshal(resp)
	log.Debugf("-----kiro response: %s, playload: %s", string(jsonData), string(resp.Payload))

	messageType := parser.GetMessageTypeFromHeaders(resp.Headers)
	eventType := parser.GetEventTypeFromHeaders(resp.Headers)

	// 优先尝试 messageType+eventType 组合查找（适用于 event 类型消息）
	p := parser.Get(messageType, eventType)
	if p == nil && eventType != "" {
		// 对于未注册的 event 子类型，记录日志并忽略
		log.Debugf("未注册的事件解析器: messageType=%s, eventType=%s", messageType, eventType)
		return nil, nil
	}

	if p == nil {
		// 对于完全未知的 messageType，记录警告
		log.Warnf("未知消息类型: %s", messageType)
		return nil, nil
	}

	return p.Parse(resp)
}
