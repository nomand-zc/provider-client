package builder

import (
	"fmt"

	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/builder/types"
)

const (
	// keepImageThreshold 保留最近 N 条历史消息中的图片
	keepImageThreshold = 5
)

// HistoryBuilder 负责构建历史消息列表：
//   - 将 system prompt 合并到首条 user 消息（或单独作为一条 user 消息插入）
//   - 将除最后一条之外的所有消息转换为 types.HistoryItem
//   - 按 keepImageThreshold 策略决定是否保留历史消息中的图片
//
// 结果写入 BuildContext.History
type HistoryBuilder struct{}

// Build 实现 MessageBuilder 接口
func (b *HistoryBuilder) Build(ctx *BuildContext) error {
	messages := ctx.Messages
	modelId := ctx.ModelId
	systemPrompt := ctx.SystemPrompt

	history := []types.HistoryItem{}
	startIndex := 0

	// 将 system prompt 合并到第一条 user 消息，或单独作为一条 user 消息
	if systemPrompt != "" {
		if messages[0].Role == providers.RoleUser {
			firstUserContent := GetMessageText(messages[0])
			history = append(history, types.HistoryItem{
				UserInputMessage: &types.UserInputMessage{
					Content: systemPrompt + "\n\n" + firstUserContent,
					ModelId: modelId,
					Origin:  originAIEditor,
				},
			})
			startIndex = 1
		} else {
			history = append(history, types.HistoryItem{
				UserInputMessage: &types.UserInputMessage{
					Content: systemPrompt,
					ModelId: modelId,
					Origin:  originAIEditor,
				},
			})
		}
	}

	// 构建历史消息（除最后一条之外的所有消息）
	totalMessages := len(messages)
	for i := startIndex; i < totalMessages-1; i++ {
		msg := messages[i]
		// 计算距末尾的距离（从后往前数，最后一条消息距离为 0）
		distanceFromEnd := (totalMessages - 1) - i
		shouldKeepImages := distanceFromEnd <= keepImageThreshold

		switch msg.Role {
		case providers.RoleUser, providers.RoleTool:
			userInputMsg := BuildHistoryUserMessage(msg, modelId, shouldKeepImages)
			history = append(history, types.HistoryItem{UserInputMessage: &userInputMsg})
		case providers.RoleAssistant:
			assistantMsg := BuildAssistantMessage(msg)
			history = append(history, types.HistoryItem{AssistantResponseMessage: &assistantMsg})
		}
	}

	ctx.History = history
	return nil
}

// BuildHistoryUserMessage 构建历史中的 user 消息（支持图片保留策略）
func BuildHistoryUserMessage(msg providers.Message, modelId string, shouldKeepImages bool) types.UserInputMessage {
	userInputMsg := types.UserInputMessage{
		ModelId: modelId,
		Origin:  originAIEditor,
	}

	var textContent string
	var images []types.Image
	var toolResults []types.ToolResult
	imageCount := 0

	if msg.Role == providers.RoleTool {
		toolResults = append(toolResults, types.ToolResult{
			ToolUseId: msg.ToolID,
			Status:    "success",
			Content:   []types.ToolResultContent{{Text: msg.Content}},
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
						img := ConvertImage(part.Image)
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
		userInputMsg.UserInputMessageContext = &types.UserInputMessageContext{
			ToolResults: DeduplicateToolResults(toolResults),
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

// BuildAssistantMessage 将 providers.Message（assistant 角色）转换为 types.AssistantResponseMessage
// 支持 thinking 内容处理
func BuildAssistantMessage(msg providers.Message) types.AssistantResponseMessage {
	assistantMsg := types.AssistantResponseMessage{}

	var content string
	var toolUses []types.ToolUse
	var thinkingText string

	// 处理 ContentParts
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
					input = ParseJSONOrString(tc.Function.Arguments)
				} else {
					input = map[string]any{}
				}
				toolUses = append(toolUses, types.ToolUse{
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