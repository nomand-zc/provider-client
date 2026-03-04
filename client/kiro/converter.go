package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nomand-zc/token101/provider-client/types"
)

const (
	kiroOriginAIEditor    = "AI_EDITOR"
	kiroChatTriggerManual = "MANUAL"
)

// KiroConverter 实现 client.Converter 接口，负责 Kiro API 的请求/响应格式转换
type KiroConverter struct {
	options Options
}

// NewKiroConverter 创建新的 KiroConverter 实例
func NewKiroConverter(opts Options) *KiroConverter {
	return &KiroConverter{
		options: opts,
	}
}

// ConvertRequest 将标准请求序列化为 Kiro CodeWhisperer API 格式的 JSON 字节切片
// creds 为账号凭证字节切片，social 模式下用于注入 profileArn
func (c *KiroConverter) ConvertRequest(ctx context.Context, creds []byte, req types.Request) ([]byte, error) {
	// 将标准请求转换为 Kiro 格式
	kiroReq, err := convertRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request to Kiro format: %w", err)
	}

	// social 模式下从凭证中注入 profileArn
	if len(creds) > 0 {
		if c, err := ExtractCredentials(creds); err == nil && c.AuthMethod == AuthMethodSocial && c.ProfileArn != "" {
			kiroReq.ProfileArn = c.ProfileArn
		}
	}

	// 序列化请求体
	body, err := json.Marshal(kiroReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	return body, nil
}

// ConvertResponse 将 Kiro API 响应的 JSON 字节切片反序列化为标准响应格式
func (c *KiroConverter) ConvertResponse(ctx context.Context, body []byte) (*types.Response, error) {
	// 解析 Kiro 响应
	var kiroResp KiroResponse
	if err := json.Unmarshal(body, &kiroResp); err != nil {
		return nil, fmt.Errorf("failed to parse Kiro response: %w", err)
	}

	// 转换为标准响应格式
	return convertKiroResponseToStandard(&kiroResp), nil
}

// ConvertStreamChunk 将单行 SSE 流式数据（含 "data: " 前缀）解析为标准流式响应格式
// 返回 nil 表示该行应被跳过（空行或 [DONE] 标记）
func (c *KiroConverter) ConvertStreamChunk(ctx context.Context, line []byte) (*types.Response, error) {
	// 去除 SSE 前缀 "data: " 及首尾空白
	chunk := bytes.TrimPrefix(line, []byte("data: "))
	chunk = bytes.TrimSpace(chunk)

	// 跳过空行和结束标记
	if len(chunk) == 0 || string(chunk) == "[DONE]" {
		return nil, nil
	}

	// 复用 ConvertResponse 解析 JSON
	response, err := c.ConvertResponse(ctx, chunk)
	if err != nil {
		return nil, err
	}

	// 修正为流式响应特有字段
	response.Object = "chat.completion.chunk"
	response.IsPartial = true
	response.Done = false

	// 将 Message 移至 Delta（流式响应使用 Delta 字段）
	for i := range response.Choices {
		response.Choices[i].Delta = response.Choices[i].Message
		response.Choices[i].Message = types.Message{}
	}

	return response, nil
}

// convertRequest
// convertRequest 将 OpenAI 兼容请求转换为 Kiro CodeWhisperer conversationState 格式
func convertRequest(req types.Request) (*KiroRequest, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("no messages provided in request")
	}

	// 映射模型 ID
	modelID := getKiroModel("claude-sonnet-4-5")

	// 提取 system prompt 和普通消息
	systemPrompt := ""
	messages := make([]types.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		if msg.Role == types.RoleSystem {
			systemPrompt = extractMessageText(msg)
		} else {
			messages = append(messages, msg)
		}
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no user messages found after filtering system prompt")
	}

	// 构建工具列表（CodeWhisperer toolSpecification 格式）
	var kiroTools []ToolSpecificationWrapper
	if len(req.Tools) > 0 {
		const maxDescLen = 9216
		for name, tool := range req.Tools {
			declaration := tool.Declaration()
			if declaration == nil {
				continue
			}
			desc := declaration.Description
			if len(desc) > maxDescLen {
				desc = desc[:maxDescLen] + "..."
			}
			inputSchema := make(map[string]interface{})
			if declaration.InputSchema != nil && declaration.InputSchema.Properties != nil {
				for paramName, prop := range declaration.InputSchema.Properties {
					if prop != nil {
						inputSchema[paramName] = map[string]interface{}{
							"type":        prop.Type,
							"description": prop.Description,
						}
					}
				}
			}
			kiroTools = append(kiroTools, ToolSpecificationWrapper{
				ToolSpecification: ToolSpecification{
					Name:        name,
					Description: desc,
					InputSchema: ToolInputSchema{JSON: inputSchema},
				},
			})
		}
	}

	// 构建历史消息（除最后一条外）
	history := make([]HistoryMessage, 0)
	startIndex := 0

	// 处理 system prompt：拼接到第一条 user 消息前面
	if systemPrompt != "" && len(messages) > 0 && messages[0].Role == types.RoleUser {
		firstContent := extractMessageText(messages[0])
		history = append(history, HistoryMessage{
			UserInputMessage: &UserInputMessage{
				Content: systemPrompt + "\n\n" + firstContent,
				ModelID: modelID,
				Origin:  kiroOriginAIEditor,
			},
		})
		startIndex = 1
	}

	for i := startIndex; i < len(messages)-1; i++ {
		msg := messages[i]
		switch msg.Role {
		case types.RoleUser:
			history = append(history, buildUserHistoryMessage(msg, modelID))
		case types.RoleAssistant:
			history = append(history, buildAssistantHistoryMessage(msg))
		}
	}

	// 确保 history 以 assistantResponseMessage 结尾（Kiro API 要求）
	if len(history) > 0 {
		last := history[len(history)-1]
		if last.AssistantResponseMessage == nil {
			history = append(history, HistoryMessage{
				AssistantResponseMessage: &AssistantResponseMessage{
					Content: "Continue",
				},
			})
		}
	}

	// 构建当前消息（最后一条）
	currentMsg := messages[len(messages)-1]
	currentContent := extractMessageText(currentMsg)
	if currentContent == "" {
		currentContent = "Continue"
	}

	userInputMessage := &UserInputMessage{
		Content: currentContent,
		ModelID: modelID,
		Origin:  kiroOriginAIEditor,
	}

	// 构建 userInputMessageContext
	msgContext := &UserInputMessageContext{}
	hasContext := false

	if len(kiroTools) > 0 {
		msgContext.Tools = kiroTools
		hasContext = true
	}

	if hasContext {
		userInputMessage.UserInputMessageContext = msgContext
	}

	conversationState := &ConversationState{
		ChatTriggerType: kiroChatTriggerManual,
		ConversationID:  generateResponseID(),
		CurrentMessage: &CurrentMessage{
			UserInputMessage: userInputMessage,
		},
	}

	if len(history) > 0 {
		conversationState.History = history
	}

	return &KiroRequest{
		ConversationState: conversationState,
	}, nil
}

// buildUserHistoryMessage 构建历史中的 user 消息
func buildUserHistoryMessage(msg types.Message, modelID string) HistoryMessage {
	return HistoryMessage{
		UserInputMessage: &UserInputMessage{
			Content: extractMessageText(msg),
			ModelID: modelID,
			Origin:  kiroOriginAIEditor,
		},
	}
}

// buildAssistantHistoryMessage 构建历史中的 assistant 消息
func buildAssistantHistoryMessage(msg types.Message) HistoryMessage {
	assistantMsg := &AssistantResponseMessage{
		Content: extractMessageText(msg),
	}

	// 处理工具调用
	if len(msg.ToolCalls) > 0 {
		toolUses := make([]ToolUse, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			var input interface{}
			if err := json.Unmarshal(tc.Function.Arguments, &input); err != nil {
				input = map[string]interface{}{"raw_arguments": string(tc.Function.Arguments)}
			}
			toolUses = append(toolUses, ToolUse{
				Input:     input,
				Name:      tc.Function.Name,
				ToolUseID: tc.ID,
			})
		}
		assistantMsg.ToolUses = toolUses
	}

	return HistoryMessage{AssistantResponseMessage: assistantMsg}
}

// extractMessageText 从消息中提取文本内容
func extractMessageText(msg types.Message) string {
	if msg.Content != "" {
		return msg.Content
	}
	if len(msg.ContentParts) > 0 {
		var parts []string
		for _, part := range msg.ContentParts {
			if part.Type == types.ContentTypeText && part.Text != nil && *part.Text != "" {
				parts = append(parts, *part.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// extractAuthToken 从凭证字节切片中提取认证 token
func extractAuthToken(creds []byte) (string, error) {
	if len(creds) == 0 {
		return "", fmt.Errorf("no credentials provided")
	}

	c, err := ExtractCredentials(creds)
	if err != nil {
		return "", fmt.Errorf("failed to parse credentials: %w", err)
	}

	if c.AccessToken == "" {
		return "", fmt.Errorf("no valid token found in credentials")
	}

	return c.AccessToken, nil
}

// convertKiroResponseToStandard 将 Kiro 响应转换为标准 OpenAI 格式
func convertKiroResponseToStandard(kiroResp *KiroResponse) *types.Response {
	response := &types.Response{
		ID:        generateResponseID(),
		Object:    "chat.completion",
		Created:   time.Now().Unix(),
		Model:     "kiro-codewhisperer",
		Timestamp: time.Now(),
		Done:      true,
		IsPartial: false,
	}

	// 处理错误响应
	if kiroResp.Error != nil {
		response.Error = &types.ResponseError{
			Code:    &kiroResp.Error.Code,
			Message: kiroResp.Error.Message,
		}
		return response
	}

	// 构建包含思考内容的响应文本
	content := kiroResp.Content
	if kiroResp.ThinkingContent != "" {
		content = fmt.Sprintf("%s\n\n[Thinking] %s", content, kiroResp.ThinkingContent)
	}

	// 创建 choice
	choice := types.Choice{
		Index: 0,
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: content,
		},
		FinishReason: nil,
	}

	// 处理工具调用（如果有）
	if len(kiroResp.ToolCalls) > 0 {
		toolCalls := make([]types.ToolCall, 0, len(kiroResp.ToolCalls))
		for _, kiroToolCall := range kiroResp.ToolCalls {
			var argsBytes []byte
			if kiroToolCall.Arguments != nil {
				argsBytes, _ = json.Marshal(kiroToolCall.Arguments)
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   kiroToolCall.ID,
				Type: "function",
				Function: types.FunctionDefinitionParam{
					Name:      kiroToolCall.Name,
					Arguments: argsBytes,
				},
			})
		}
		choice.Message.ToolCalls = toolCalls
	}

	response.Choices = []types.Choice{choice}

	// 处理 token 用量（如果有）
	if kiroResp.Usage != nil {
		response.Usage = &types.Usage{
			PromptTokens:     kiroResp.Usage.InputTokens,
			CompletionTokens: kiroResp.Usage.OutputTokens,
			TotalTokens:      kiroResp.Usage.TotalTokens,
		}
	}

	return response
}
