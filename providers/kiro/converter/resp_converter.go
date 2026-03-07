package converter

import (
	"context"

	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/parser"
)

// ConvertResponse 将 Kiro CodeWhisperer 响应转换为通用响应格式
// 通过 parser 注册器根据 messageType 和 eventType 获取对应的解析器来处理
func ConvertResponse(ctx context.Context, resp *parser.StreamMessage, opts ...parser.OptionFunc) (
	*providers.Response, error) {
	if resp == nil {
		return nil, nil
	}

	messageType := resp.MessageType()
	eventType := resp.EventType()

	// 优先尝试 messageType+eventType 组合查找（适用于 event 类型消息）
	p := parser.Get(messageType, eventType)
	if p == nil {
		// 对于未注册的 event 子类型，记录日志并忽略
		log.Warnf("未注册的事件解析器: messageType=%s, eventType=%s, payload: %s",
			messageType, eventType, string(resp.Payload))
		return nil, nil
	}

	return p.Parse(ctx, resp, opts...)
}
