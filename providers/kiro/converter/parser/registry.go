package parser

import (
	"fmt"

	"github.com/nomand-zc/provider-client/log"
)

// registry 全局解析器注册表
var registry = make(map[string]PayloadParser)

// buildKey 构建注册表的查找键
// 对于 messageType 级别的解析器（如 error、exception），eventType 为空字符串
func buildKey(messageType, eventType string) string {
	if eventType == "" {
		return messageType
	}
	return fmt.Sprintf("%s:%s", messageType, eventType)
}

// Register 注册解析器
// 从 PayloadParser 接口的 MessageType() 和 EventType() 方法获取注册键
func Register(p PayloadParser) {
	key := buildKey(p.MessageType(), p.EventType())
	if _, exists := registry[key]; exists {
		log.Warnf("解析器重复注册，将被覆盖: key=%s", key)
	}
	registry[key] = p
	log.Debugf("注册解析器: key=%s", key)
}

// Get 根据消息类型和事件类型获取对应的解析器
// 返回 nil 表示没有找到匹配的解析器
func Get(messageType, eventType string) PayloadParser {
	key := buildKey(messageType, eventType)
	return registry[key]
}
