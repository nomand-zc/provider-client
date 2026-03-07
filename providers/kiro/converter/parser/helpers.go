package parser

import (
	"encoding/json"
	"strings"

	"github.com/nomand-zc/provider-client/log"
	"github.com/nomand-zc/provider-client/providers"
)

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
		return []byte("")
	}
	if str, ok := input.(string); ok {
		return []byte(str)
	}
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		log.Warnf("转换 input 为 JSON 失败: %v", err)
		return []byte("")
	}
	return jsonBytes
}
