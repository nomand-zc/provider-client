package parser

import "github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"

// 消息类型常量
const (
	MessageTypeEvent     = "event"
	MessageTypeError     = "error"
	MessageTypeException = "exception"
)

// 事件类型常量
const (
	// 代码补全
	EventTypeCompletion      = "completion"
	EventTypeCompletionChunk = "completion_chunk"

	// 工具调用相关
	EventTypeToolCallRequest    = "tool_call_request"
	EventTypeToolCallResult     = "tool_call_result"
	EventTypeToolCallError      = "tool_call_error"
	EventTypeToolExecutionStart = "tool_execution_start"
	EventTypeToolExecutionEnd   = "tool_execution_end"

	// 会话管理
	EventTypeSessionStart = "session_start"
	EventTypeSessionEnd   = "session_end"

	// 统计信息
	EventTypeMeteringEvent     = "meteringEvent"
	EventTypeContextUsageEvent = "contextUsageEvent"

	// 兼容旧格式
	EventTypeAssistantResponseEvent = "assistantResponseEvent"
	EventTypeToolUseEvent           = "toolUseEvent"
)

// GetMessageTypeFromHeaders 从头部提取消息类型
func GetMessageTypeFromHeaders(headers eventstream.Headers) string {
	if v := headers.Get(":message-type"); v != nil {
		if sv, ok := v.(eventstream.StringValue); ok {
			return string(sv)
		}
	}
	return MessageTypeEvent // 默认为事件类型
}

// GetEventTypeFromHeaders 从头部提取事件类型
func GetEventTypeFromHeaders(headers eventstream.Headers) string {
	if v := headers.Get(":event-type"); v != nil {
		if sv, ok := v.(eventstream.StringValue); ok {
			return string(sv)
		}
	}
	return ""
}

// GetContentTypeFromHeaders 从头部提取内容类型
func GetContentTypeFromHeaders(headers eventstream.Headers) string {
	if v := headers.Get(":content-type"); v != nil {
		if sv, ok := v.(eventstream.StringValue); ok {
			return string(sv)
		}
	}
	return "application/json" // 默认为JSON
}
