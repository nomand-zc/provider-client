package converter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/parser"
	"github.com/nomand-zc/provider-client/queue"
)

const (
	defaultBufferSize = 1024
	defaultQueueSize  = 100
)

// ProcessEventStream 处理事件流
func ProcessEventStream(ctx context.Context, reader io.Reader) (queue.Reader[*parser.EventStreamMessage], error) {
	buf := make([]byte, defaultBufferSize)
	chainQueue := queue.NewChanQueue[*parser.EventStreamMessage](defaultQueueSize)
	var totalReadBytes int

	go func(){
		defer chainQueue.Close()
		for {
			n, err := reader.Read(buf)
			totalReadBytes += n
			if n > 0 {
				// 解析事件流
				events, parseErr := parser.DefaultRobustParser.ParseStream(buf[:n])

				if parseErr != nil {
					log.Warnf("符合规范的解析器处理失败, err: %v, read_bytes: %d",
						parseErr, n)
				}
				for _, event := range events {
					chainQueue.Write(event)
				}
			}

			if err != nil {
				if err == io.EOF {
					log.Debugf("响应流结束, total_read_bytes: %d", totalReadBytes)
				} else {
					log.Errorf("读取响应流时发生错误, err: %v, total_read_bytes: %d",
						err, totalReadBytes)
				}
				break
			}
		}
	}()
	

	// 直传模式：无需冲刷剩余文本
	return chainQueue, nil
}

// ConvertResponse 将 Kiro CodeWhisperer 响应转换为通用响应格式
func ConvertResponse(_ context.Context, resp *parser.EventStreamMessage) (
	*providers.Response, error) {
	if resp == nil {
		return nil, nil
	}

	// jsonData, _ := json.Marshal(resp)
	// log.Debugf("kiro response: %s", string(jsonData))

	messageType := resp.GetMessageType()
	eventType := resp.GetEventType()

	log.Debugf("kiro response: %s", string(resp.Payload))

	// 根据消息类型分别处理
	switch messageType {
	case parser.MessageTypes.EVENT:
		return convertEventMessage(resp, eventType)
	case parser.MessageTypes.ERROR:
		return convertErrorMessage(resp)
	case parser.MessageTypes.EXCEPTION:
		return convertExceptionMessage(resp)
	default:
		log.Warnf("未知消息类型: %s", messageType)
		return nil, nil
	}
}

// convertEventMessage 处理事件类型消息
func convertEventMessage(msg *parser.EventStreamMessage, eventType string) (*providers.Response, error) {
	switch eventType {
	case parser.EventTypes.COMPLETION:
		return convertCompletionEvent(msg)
	case parser.EventTypes.COMPLETION_CHUNK:
		return convertCompletionChunkEvent(msg)
	case parser.EventTypes.ASSISTANT_RESPONSE_EVENT:
		return convertAssistantResponseEvent(msg)
	case parser.EventTypes.TOOL_CALL_REQUEST:
		return convertToolCallRequestEvent(msg)
	case parser.EventTypes.TOOL_USE_EVENT:
		return convertToolUseEvent(msg)
	case parser.EventTypes.TOOL_CALL_ERROR:
		return convertToolCallErrorEvent(msg)
	case parser.EventTypes.SESSION_START, parser.EventTypes.SESSION_END:
		// 会话管理事件，不需要转换为 Response
		log.Debugf("跳过会话管理事件: %s", eventType)
		return nil, nil
	default:
		log.Debugf("未知事件类型: %s", eventType)
		return nil, nil
	}
}

// convertCompletionEvent 处理 completion 事件（完整响应）
func convertCompletionEvent(msg *parser.EventStreamMessage) (*providers.Response, error) {
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 completion 事件载荷失败: %w", err)
	}

	content, _ := data["content"].(string)
	finishReason, _ := data["finish_reason"].(string)

	resp := &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: false,
		Done:      true,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:    providers.RoleAssistant,
					Content: content,
				},
			},
		},
	}

	// 设置完成原因
	if finishReason != "" {
		resp.Choices[0].FinishReason = &finishReason
	}

	// 处理工具调用
	toolCalls := parseToolCalls(data)
	if len(toolCalls) > 0 {
		resp.Choices[0].Delta.ToolCalls = toolCalls
		resp.Done = false
	}

	return resp, nil
}

// convertCompletionChunkEvent 处理 completion_chunk 事件（流式增量）
func convertCompletionChunkEvent(msg *parser.EventStreamMessage) (*providers.Response, error) {
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 completion_chunk 事件载荷失败: %w", err)
	}

	content, _ := data["content"].(string)
	delta, _ := data["delta"].(string)
	finishReason, _ := data["finish_reason"].(string)

	// 使用 delta 作为实际的文本增量，如果没有则使用 content
	textDelta := delta
	if textDelta == "" {
		textDelta = content
	}

	resp := &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: true,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:    providers.RoleAssistant,
					Content: textDelta,
				},
			},
		},
	}

	// 如果有完成原因，标记为最终响应
	if finishReason != "" {
		resp.Choices[0].FinishReason = &finishReason
		resp.Done = true
		resp.IsPartial = false
	}

	return resp, nil
}

// convertAssistantResponseEvent 处理 assistantResponseEvent 事件
func convertAssistantResponseEvent(msg *parser.EventStreamMessage) (*providers.Response, error) {
	payloadStr := string(msg.Payload)

	// 检查是否是工具调用事件
	if isToolCallPayload(payloadStr) {
		return convertAssistantToolCallEvent(msg)
	}

	// 尝试解析为 JSON
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		// 非 JSON 格式，作为纯文本内容处理
		text := strings.TrimSpace(payloadStr)
		if text == "" {
			return nil, nil
		}
		return &providers.Response{
			Object:    "chat.completion.chunk",
			Created:   time.Now().Unix(),
			Timestamp: time.Now(),
			IsPartial: true,
			Choices: []providers.Choice{
				{
					Index: 0,
					Delta: providers.Message{
						Role:    providers.RoleAssistant,
						Content: text,
					},
				},
			},
		}, nil
	}

	// 提取内容
	content, _ := data["content"].(string)
	if content == "" {
		return nil, nil
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: true,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:    providers.RoleAssistant,
					Content: content,
				},
			},
		},
	}, nil
}

// convertAssistantToolCallEvent 处理 assistantResponseEvent 中的工具调用
func convertAssistantToolCallEvent(msg *parser.EventStreamMessage) (*providers.Response, error) {
	var evt struct {
		Name      string `json:"name"`
		ToolUseId string `json:"toolUseId"`
		Input     any    `json:"input"`
		Stop      bool   `json:"stop"`
	}
	if err := json.Unmarshal(msg.Payload, &evt); err != nil {
		log.Warnf("解析工具调用事件失败: %v", err)
		return nil, nil
	}

	toolCall := providers.ToolCall{
		ID:   evt.ToolUseId,
		Type: "function",
		Function: providers.FunctionDefinitionParam{
			Name:      evt.Name,
			Arguments: convertInputToArgs(evt.Input),
		},
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: !evt.Stop,
		Done:      false,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:      providers.RoleAssistant,
					ToolCalls: []providers.ToolCall{toolCall},
				},
			},
		},
	}, nil
}

// convertToolCallRequestEvent 处理标准 tool_call_request 事件
func convertToolCallRequestEvent(msg *parser.EventStreamMessage) (*providers.Response, error) {
	var data map[string]any
	if err := json.Unmarshal(msg.Payload, &data); err != nil {
		return nil, fmt.Errorf("解析 tool_call_request 事件载荷失败: %w", err)
	}

	toolCallID, _ := data["toolCallId"].(string)
	toolName, _ := data["toolName"].(string)

	// 解析 input
	args := []byte("{}")
	if inputData, ok := data["input"].(map[string]any); ok && len(inputData) > 0 {
		if argsJSON, err := json.Marshal(inputData); err == nil {
			args = argsJSON
		}
	}

	toolCall := providers.ToolCall{
		ID:   toolCallID,
		Type: "function",
		Function: providers.FunctionDefinitionParam{
			Name:      toolName,
			Arguments: args,
		},
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: false,
		Done:      false,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:      providers.RoleAssistant,
					ToolCalls: []providers.ToolCall{toolCall},
				},
			},
		},
	}, nil
}

// convertToolUseEvent 处理旧格式 toolUseEvent 事件
func convertToolUseEvent(msg *parser.EventStreamMessage) (*providers.Response, error) {
	var evt struct {
		Name      string `json:"name"`
		ToolUseId string `json:"toolUseId"`
		Input     any    `json:"input"`
		Stop      bool   `json:"stop"`
	}
	if err := json.Unmarshal(msg.Payload, &evt); err != nil {
		return nil, fmt.Errorf("解析 toolUseEvent 载荷失败: %w", err)
	}

	toolCall := providers.ToolCall{
		ID:   evt.ToolUseId,
		Type: "function",
		Function: providers.FunctionDefinitionParam{
			Name:      evt.Name,
			Arguments: convertInputToArgs(evt.Input),
		},
	}

	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		IsPartial: !evt.Stop,
		Done:      false,
		Choices: []providers.Choice{
			{
				Index: 0,
				Delta: providers.Message{
					Role:      providers.RoleAssistant,
					ToolCalls: []providers.ToolCall{toolCall},
				},
			},
		},
	}, nil
}

// convertToolCallErrorEvent 处理工具调用错误事件
func convertToolCallErrorEvent(msg *parser.EventStreamMessage) (*providers.Response, error) {
	var errorInfo struct {
		ToolCallID string `json:"tool_call_id"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(msg.Payload, &errorInfo); err != nil {
		return nil, fmt.Errorf("解析 tool_call_error 载荷失败: %w", err)
	}

	errMsg := fmt.Sprintf("tool_call_error: %s (tool_call_id: %s)", errorInfo.Error, errorInfo.ToolCallID)
	return &providers.Response{
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Timestamp: time.Now(),
		Done:      true,
		Error: &providers.ResponseError{
			Message: errMsg,
			Type:    "tool_call_error",
		},
	}, nil
}

// convertErrorMessage 处理 error 类型消息
func convertErrorMessage(msg *parser.EventStreamMessage) (*providers.Response, error) {
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

// convertExceptionMessage 处理 exception 类型消息
func convertExceptionMessage(msg *parser.EventStreamMessage) (*providers.Response, error) {
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
			Code:    strPtr(exceptionType),
		},
	}, nil
}

// === 辅助函数 ===

// isToolCallPayload 检查载荷是否包含工具调用信息
func isToolCallPayload(payload string) bool {
	return strings.Contains(payload, "\"toolUseId\":") ||
		strings.Contains(payload, "\"tool_use_id\":") ||
		(strings.Contains(payload, "\"name\":") && strings.Contains(payload, "\"input\":"))
}

// parseToolCalls 从 completion 数据中解析工具调用列表
func parseToolCalls(data map[string]any) []providers.ToolCall {
	tcData, ok := data["tool_calls"].([]any)
	if !ok {
		return nil
	}

	var toolCalls []providers.ToolCall
	for _, tc := range tcData {
		tcMap, ok := tc.(map[string]any)
		if !ok {
			continue
		}

		toolCall := providers.ToolCall{}
		if id, ok := tcMap["id"].(string); ok {
			toolCall.ID = id
		}
		if tcType, ok := tcMap["type"].(string); ok {
			toolCall.Type = tcType
		}
		if function, ok := tcMap["function"].(map[string]any); ok {
			if name, ok := function["name"].(string); ok {
				toolCall.Function.Name = name
			}
			if args, ok := function["arguments"].(string); ok {
				toolCall.Function.Arguments = []byte(args)
			}
		}
		toolCalls = append(toolCalls, toolCall)
	}
	return toolCalls
}

// convertInputToArgs 将 any 类型的 input 转换为 JSON 字节
func convertInputToArgs(input any) []byte {
	if input == nil {
		return []byte("{}")
	}
	if str, ok := input.(string); ok {
		return []byte(str)
	}
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		log.Warnf("转换 input 为 JSON 失败: %v", err)
		return []byte("{}")
	}
	return jsonBytes
}

// strPtr 返回字符串的指针
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
