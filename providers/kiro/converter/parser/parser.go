package parser

import (
	"github.com/nomand-zc/provider-client/providers"
)

// PayloadParser 定义事件流消息解析器接口
// 每种消息类型+事件类型的组合对应一个 PayloadParser 实现
type PayloadParser interface {
	// MessageType 返回解析器处理的消息类型（event/error/exception）
	MessageType() string
	// EventType 返回解析器处理的事件类型（仅当 MessageType 为 event 时有意义，否则返回空字符串）
	EventType() string
	// Parse 解析事件流消息并转换为通用响应格式
	Parse(msg *StreamMessage) (*providers.Response, error)
}
