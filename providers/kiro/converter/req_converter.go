package converter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nomand-zc/provider-client/providers"
)

const (
	// keepImageThreshold 保留最近 N 条历史消息中的图片
	keepImageThreshold = 5
	// maxDescriptionLength 工具描述最大长度
	maxDescriptionLength = 9216
)

// ConvertRequest 将通用请求转换为 Kiro CodeWhisperer 请求格式
// 完全对齐 buildCodewhispererRequest 的实现逻辑
func ConvertRequest(ctx context.Context, req providers.Request) *Request {
	if len(req.Messages) == 0 {
		return nil
	}

	// 解析模型 ID
	modelId := req.Model

	// Step 1: 预处理消息（移除末尾 "{" 的 assistant 消息 + 合并相邻同 role 消息）
	messages := preprocessMessages(req.Messages)
	if len(messages) == 0 {
		return nil
	}

	// Step 2: 提取 system prompt，并从消息列表中移除 system 消息
	var systemPrompt string
	var nonSystemMessages []providers.Message
	for _, msg := range messages {
		if msg.Role == providers.RoleSystem {
			if msg.Content != "" {
				if systemPrompt != "" {
					systemPrompt += "\n\n" + msg.Content
				} else {
					systemPrompt = msg.Content
				}
			}
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}
	messages = nonSystemMessages
	if len(messages) == 0 {
		return nil
	}

	// Step 3: 处理 tools（过滤、截断、占位）
	kiroTools := buildKiroTools(req.Tools)

	// Step 4: 构建 history
	history := []HistoryItem{}
	startIndex := 0

	// 将 system prompt 合并到第一条 user 消息，或单独作为一条 user 消息
	if systemPrompt != "" {
		if messages[0].Role == providers.RoleUser {
			firstUserContent := getMessageText(messages[0])
			history = append(history, HistoryItem{
				UserInputMessage: &UserInputMessage{
					Content: systemPrompt + "\n\n" + firstUserContent,
					ModelId: modelId,
					Origin:  "AI_EDITOR",
				},
			})
			startIndex = 1
		} else {
			history = append(history, HistoryItem{
				UserInputMessage: &UserInputMessage{
					Content: systemPrompt,
					ModelId: modelId,
					Origin:  "AI_EDITOR",
				},
			})
		}
	}

	// Step 5: 构建历史消息（除最后一条之外的所有消息）
	totalMessages := len(messages)
	for i := startIndex; i < totalMessages-1; i++ {
		msg := messages[i]
		// 计算距末尾的距离（从后往前数，最后一条消息距离为 0）
		distanceFromEnd := (totalMessages - 1) - i
		shouldKeepImages := distanceFromEnd <= keepImageThreshold

		switch msg.Role {
		case providers.RoleUser, providers.RoleTool:
			userInputMsg := buildHistoryUserMessage(msg, modelId, shouldKeepImages)
			history = append(history, HistoryItem{UserInputMessage: &userInputMsg})
		case providers.RoleAssistant:
			assistantMsg := buildAssistantMessage(msg)
			history = append(history, HistoryItem{AssistantResponseMessage: &assistantMsg})
		}
	}

	// Step 6: 处理最后一条消息（currentMessage）
	lastMsg := messages[totalMessages-1]

	var currentContent string
	var currentToolResults []ToolResult
	var currentImages []Image

	if lastMsg.Role == providers.RoleAssistant {
		// 最后一条是 assistant 消息：将其加入 history，currentMessage 设为 "Continue"
		assistantMsg := buildAssistantMessage(lastMsg)
		history = append(history, HistoryItem{AssistantResponseMessage: &assistantMsg})
		currentContent = "Continue"
	} else {
		// 最后一条是 user 消息：确保 history 末尾是 assistantResponseMessage
		if len(history) > 0 {
			lastHistoryItem := history[len(history)-1]
			if lastHistoryItem.AssistantResponseMessage == nil {
				history = append(history, HistoryItem{
					AssistantResponseMessage: &AssistantResponseMessage{Content: "Continue"},
				})
			}
		}

		// 解析最后一条 user 消息的内容
		if len(lastMsg.ContentParts) > 0 {
			for _, part := range lastMsg.ContentParts {
				switch part.Type {
				case providers.ContentTypeText:
					if part.Text != nil {
						currentContent += *part.Text
					}
				case providers.ContentTypeImage:
					if part.Image != nil {
						img := convertImage(part.Image)
						if img != nil {
							currentImages = append(currentImages, *img)
						}
					}
				}
			}
		} else if lastMsg.Role == providers.RoleTool {
			// RoleTool 消息作为 toolResult
			currentToolResults = append(currentToolResults, ToolResult{
				ToolUseId: lastMsg.ToolID,
				Status:    "success",
				Content:   []ToolResultContent{{Text: lastMsg.Content}},
			})
		} else {
			currentContent = lastMsg.Content
		}

		// content 兜底
		if currentContent == "" {
			if len(currentToolResults) > 0 {
				currentContent = "Tool results provided."
			} else {
				currentContent = "Continue"
			}
		}
	}

	// Step 7: 构建最终请求
	kiroReq := &Request{}
	kiroReq.ConversationState.ChatTriggerType = "MANUAL"
	kiroReq.ConversationState.ConversationId = uuid.NewString()

	// 构建 userInputMessage
	userInputMsg := UserInputMessage{
		Content: currentContent,
		ModelId: modelId,
		Origin:  "AI_EDITOR",
	}
	if len(currentImages) > 0 {
		userInputMsg.Images = currentImages
	}

	// 构建 userInputMessageContext
	userInputMsgCtx := &UserInputMessageContext{}
	hasCtx := false

	if len(currentToolResults) > 0 {
		userInputMsgCtx.ToolResults = deduplicateToolResults(currentToolResults)
		hasCtx = true
	}
	if len(kiroTools) > 0 {
		userInputMsgCtx.Tools = kiroTools
		hasCtx = true
	}
	if hasCtx {
		userInputMsg.UserInputMessageContext = userInputMsgCtx
	}

	kiroReq.ConversationState.CurrentMessage.UserInputMessage = userInputMsg

	if len(history) > 0 {
		kiroReq.ConversationState.History = history
	}

	return kiroReq
}

// preprocessMessages 预处理消息列表：
// 1. 移除末尾内容为 "{" 的 assistant 消息
// 2. 合并相邻相同 role 的消息
func preprocessMessages(messages []providers.Message) []providers.Message {
	if len(messages) == 0 {
		return messages
	}

	// Step 1: 检查并移除末尾内容为 "{" 的 assistant 消息
	result := make([]providers.Message, len(messages))
	copy(result, messages)

	last := result[len(result)-1]
	if last.Role == providers.RoleAssistant {
		content := last.Content
		if content == "" && len(last.ContentParts) > 0 {
			if last.ContentParts[0].Type == providers.ContentTypeText && last.ContentParts[0].Text != nil {
				content = *last.ContentParts[0].Text
			}
		}
		if content == "{" {
			result = result[:len(result)-1]
		}
	}

	if len(result) == 0 {
		return result
	}

	// Step 2: 合并相邻相同 role 的消息
	merged := []providers.Message{result[0]}
	for i := 1; i < len(result); i++ {
		cur := result[i]
		prev := &merged[len(merged)-1]

		if cur.Role != prev.Role {
			merged = append(merged, cur)
			continue
		}

		// 相同 role，合并内容
		prevIsArray := len(prev.ContentParts) > 0
		curIsArray := len(cur.ContentParts) > 0

		if prevIsArray && curIsArray {
			// 都是 ContentParts，直接追加
			prev.ContentParts = append(prev.ContentParts, cur.ContentParts...)
		} else if !prevIsArray && !curIsArray {
			// 都是 Content 字符串，用换行连接
			if prev.Content != "" && cur.Content != "" {
				prev.Content += "\n" + cur.Content
			} else if cur.Content != "" {
				prev.Content = cur.Content
			}
		} else if prevIsArray && !curIsArray {
			// prev 是数组，cur 是字符串，追加为 text part
			if cur.Content != "" {
				text := cur.Content
				prev.ContentParts = append(prev.ContentParts, providers.ContentPart{
					Type: providers.ContentTypeText,
					Text: &text,
				})
			}
		} else {
			// prev 是字符串，cur 是数组，转换 prev 为数组格式
			var newParts []providers.ContentPart
			if prev.Content != "" {
				text := prev.Content
				newParts = append(newParts, providers.ContentPart{
					Type: providers.ContentTypeText,
					Text: &text,
				})
			}
			newParts = append(newParts, cur.ContentParts...)
			prev.Content = ""
			prev.ContentParts = newParts
		}
	}

	return merged
}

// getMessageText 获取消息的纯文本内容
func getMessageText(msg providers.Message) string {
	if msg.Content != "" {
		return msg.Content
	}
	var parts []string
	for _, part := range msg.ContentParts {
		if part.Type == providers.ContentTypeText && part.Text != nil {
			parts = append(parts, *part.Text)
		}
	}
	return strings.Join(parts, "")
}

// buildKiroTools 将 providers.Tool 列表转换为 kiro Tool 列表
// 包含过滤（web_search/websearch、空描述）、截断、占位工具逻辑
func buildKiroTools(tools []providers.Tool) []Tool {
	placeholderTool := Tool{
		ToolSpecification: ToolSpecification{
			Name:        "no_tool_available",
			Description: "This is a placeholder tool when no other tools are available. It does nothing.",
			InputSchema: InputSchema{
				Json: map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
	}

	if len(tools) == 0 {
		return []Tool{placeholderTool}
	}

	// 过滤 web_search / websearch
	var filtered []providers.Tool
	for _, tool := range tools {
		decl := tool.Declaration()
		if decl == nil {
			continue
		}
		name := strings.ToLower(decl.Name)
		if name == "web_search" || name == "websearch" {
			continue
		}
		filtered = append(filtered, tool)
	}

	if len(filtered) == 0 {
		return []Tool{placeholderTool}
	}

	// 过滤空描述 + 截断超长描述
	var kiroTools []Tool
	for _, tool := range filtered {
		decl := tool.Declaration()
		if decl == nil || decl.Name == "" {
			continue
		}
		desc := decl.Description
		if strings.TrimSpace(desc) == "" {
			continue
		}
		if len(desc) > maxDescriptionLength {
			desc = desc[:maxDescriptionLength] + "..."
		}
		kiroTools = append(kiroTools, Tool{
			ToolSpecification: ToolSpecification{
				Name:        decl.Name,
				Description: desc,
				InputSchema: InputSchema{
					Json: convertSchema(decl.InputSchema),
				},
			},
		})
	}

	if len(kiroTools) == 0 {
		return []Tool{placeholderTool}
	}

	return kiroTools
}

// buildHistoryUserMessage 构建历史中的 user 消息（支持图片保留策略）
func buildHistoryUserMessage(msg providers.Message, modelId string, shouldKeepImages bool) UserInputMessage {
	userInputMsg := UserInputMessage{
		ModelId: modelId,
		Origin:  "AI_EDITOR",
	}

	var textContent string
	var images []Image
	var toolResults []ToolResult
	imageCount := 0

	if msg.Role == providers.RoleTool {
		toolResults = append(toolResults, ToolResult{
			ToolUseId: msg.ToolID,
			Status:    "success",
			Content:   []ToolResultContent{{Text: msg.Content}},
		})
	} else if len(msg.ContentParts) > 0 {
		for _, part := range msg.ContentParts {
			switch part.Type {
			case providers.ContentTypeText:
				if part.Text != nil {
					textContent += *part.Text
				}
			case providers.ContentTypeImage:
				if part.Image != nil {
					if shouldKeepImages {
						img := convertImage(part.Image)
						if img != nil {
							images = append(images, *img)
						}
					} else {
						imageCount++
					}
				}
			}
		}
	} else {
		textContent = msg.Content
	}

	// 图片占位符
	if imageCount > 0 {
		placeholder := fmt.Sprintf("[此消息包含 %d 张图片，已在历史记录中省略]", imageCount)
		if textContent != "" {
			textContent = textContent + "\n" + placeholder
		} else {
			textContent = placeholder
		}
	}

	if len(toolResults) > 0 {
		userInputMsg.UserInputMessageContext = &UserInputMessageContext{
			ToolResults: deduplicateToolResults(toolResults),
		}
		userInputMsg.Content = ""
	} else {
		userInputMsg.Content = textContent
	}

	if len(images) > 0 {
		userInputMsg.Images = images
	}

	return userInputMsg
}

// buildAssistantMessage 将 providers.Message（assistant 角色）转换为 AssistantResponseMessage
// 支持 thinking 内容处理
func buildAssistantMessage(msg providers.Message) AssistantResponseMessage {
	assistantMsg := AssistantResponseMessage{}

	var content string
	var toolUses []ToolUse
	var thinkingText string

	// 处理 ContentParts（包含 thinking 类型）
	if len(msg.ContentParts) > 0 {
		for _, part := range msg.ContentParts {
			switch part.Type {
			case providers.ContentTypeText:
				if part.Text != nil {
					content += *part.Text
				}
			}
		}
	} else {
		content = msg.Content
	}

	// 处理 ReasoningContent（thinking）
	if msg.ReasoningContent != "" {
		thinkingText = msg.ReasoningContent
	}

	// 将 thinking 内容前置到 content
	if thinkingText != "" {
		if content != "" {
			content = fmt.Sprintf("<thinking>%s</thinking>\n\n%s", thinkingText, content)
		} else {
			content = fmt.Sprintf("<thinking>%s</thinking>", thinkingText)
		}
	}

	assistantMsg.Content = content

	// 处理工具调用
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			if tc.Type == "function" || tc.Type == "tool_use" {
				var input any
				if len(tc.Function.Arguments) > 0 {
					input = parseJSONOrString(tc.Function.Arguments)
				} else {
					input = map[string]any{}
				}
				toolUses = append(toolUses, ToolUse{
					ToolUseId: tc.ID,
					Name:      tc.Function.Name,
					Input:     input,
				})
			}
		}
	}

	if len(toolUses) > 0 {
		assistantMsg.ToolUses = toolUses
	}

	return assistantMsg
}

// deduplicateToolResults 按 toolUseId 去重（保留首次出现）
func deduplicateToolResults(toolResults []ToolResult) []ToolResult {
	seen := make(map[string]bool)
	var unique []ToolResult
	for _, tr := range toolResults {
		if !seen[tr.ToolUseId] {
			seen[tr.ToolUseId] = true
			unique = append(unique, tr)
		}
	}
	return unique
}

// convertImage 将 providers.Image 转换为 kiro Image
func convertImage(img *providers.Image) *Image {
	if img == nil {
		return nil
	}

	format := img.Format
	if format == "" {
		format = "jpeg"
	}

	var bytesStr string
	if len(img.Data) > 0 {
		bytesStr = base64.StdEncoding.EncodeToString(img.Data)
	} else if img.URL != "" {
		// URL 格式的图片暂不支持直接转换，跳过
		return nil
	}

	if bytesStr == "" {
		return nil
	}

	return &Image{
		Format: format,
		Source: ImageSource{
			Bytes: bytesStr,
		},
	}
}

// convertSchema 将 providers.Schema 转换为 map[string]any
func convertSchema(schema *providers.Schema) any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}

	result := map[string]any{}
	if schema.Type != "" {
		result["type"] = schema.Type
	}
	if schema.Description != "" {
		result["description"] = schema.Description
	}
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	if len(schema.Properties) > 0 {
		props := map[string]any{}
		for k, v := range schema.Properties {
			props[k] = convertSchema(v)
		}
		result["properties"] = props
	}
	return result
}

// parseJSONOrString 尝试将字节解析为 JSON 对象，失败则返回原始字符串
func parseJSONOrString(data []byte) any {
	if len(data) == 0 {
		return map[string]any{}
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var result any
		if err := json.Unmarshal(data, &result); err == nil {
			return result
		}
	}
	return fmt.Sprintf("%s", data)
}
