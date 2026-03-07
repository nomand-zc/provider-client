package parser

import (
	"errors"

	"github.com/nomand-zc/provider-client/log"
)

// registry 全局解析器注册表
// 第一维 key 为 messageType（event/error/exception），第二维 key 为 eventType
var registry = make(map[string]map[string]PayloadParser)

// Register 注册解析器
// 从 PayloadParser 接口的 MessageType() 和 EventType() 方法获取注册键
func Register(p PayloadParser) {
	msgType := p.MessageType()
	evtType := p.EventType()

	if _, ok := registry[msgType]; !ok {
		registry[msgType] = make(map[string]PayloadParser)
	}
	if _, exists := registry[msgType][evtType]; exists {
		log.Warnf("解析器重复注册，将被覆盖: messageType=%s, eventType=%s", msgType, evtType)
	}
	registry[msgType][evtType] = p
	log.Debugf("注册解析器: messageType=%s, eventType=%s", msgType, evtType)
}

// MustRegister 注册解析器
// 从 PayloadParser 接口的 MessageType() 和 EventType() 方法获取注册键
func MustRegister(p PayloadParser) error {
	msgType := p.MessageType()
	evtType := p.EventType()

	if msgType == "" || evtType == "" {
		return errors.New("messageType 或 eventType 不能为空")
	}

	if _p := Get(msgType, evtType); _p != nil {
		return errors.New("解析器已存在")
	}

	if _, ok := registry[msgType]; !ok {
		registry[msgType] = make(map[string]PayloadParser)
	}
	if _, exists := registry[msgType][evtType]; exists {
		log.Warnf("解析器重复注册，将被覆盖: messageType=%s, eventType=%s", msgType, evtType)
	}
	registry[msgType][evtType] = p
	log.Debugf("注册解析器: messageType=%s, eventType=%s", msgType, evtType)
}

// Get 根据消息类型和事件类型获取对应的解析器
// 返回 nil 表示没有找到匹配的解析器
func Get(messageType, eventType string) PayloadParser {
	if sub, ok := registry[messageType]; ok {
		return sub[eventType]
	}
	return nil
}
