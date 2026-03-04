package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nomand-zc/token101/provider-client/log"
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
// creds 为账号凭证，支持 *Credentials 或 []byte（JSON 格式），social 模式下用于注入 profileArn
func (c *KiroConverter) ConvertRequest(ctx context.Context, creds any, req types.Request) ([]byte, error) {
	log.DebugfContext(ctx, "[kiro] ConvertRequest start, msgCount=%d toolCount=%d", len(req.Messages), len(req.Tools))

	// 将标准请求转换为 Kiro 格式
	kiroReq, err := convertRequest(req)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] ConvertRequest failed to convert to Kiro format: %v", err)
		return nil, fmt.Errorf("failed to convert request to Kiro format: %w", err)
	}

	// social 模式下从凭证中注入 profileArn
	if creds != nil {
		if extracted, err := extractKiroCredentials(creds); err == nil && extracted.AuthMethod == AuthMethodSocial && extracted.ProfileArn != "" {
			log.DebugfContext(ctx, "[kiro] ConvertRequest injecting profileArn for social mode")
			kiroReq.ProfileArn = extracted.ProfileArn
		}
	}

	// 序列化请求体
	body, err := json.Marshal(kiroReq)
	if err != nil {
		log.ErrorfContext(ctx, "[kiro] ConvertRequest failed to marshal request: %v", err)
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	log.DebugfContext(ctx, "[kiro] ConvertRequest done, bodyLen=%d", len(body))

	return body, nil
}

// ConvertResponse 将 Kiro API 响应的 JSON 字节切片反序列化为标准响应格式
func (c *KiroConverter) ConvertResponse(ctx context.Context, body []byte) (*types.Response, error) {
	log.DebugfContext(ctx, "[kiro] ConvertResponse bodyLen=%d", len(body))

	// 解析 Kiro 响应
	var kiroResp KiroResponse
	if err := json.Unmarshal(body, &kiroResp); err != nil {
		log.ErrorfContext(ctx, "[kiro] ConvertResponse failed to unmarshal: %v, body=%s", err, string(body))
		return nil, fmt.Errorf("failed to parse Kiro response: %w", err)
	}

	if kiroResp.Error != nil {
		log.WarnfContext(ctx, "[kiro] ConvertResponse got error response: code=%s msg=%s",
			kiroResp.Error.Code, kiroResp.Error.Message)
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

// convertRequest 将 OpenAI 兼容请求转换为 Kiro CodeWhisperer conversationState 格式
func convertRequest(req types.Request) (*KiroRequest, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("no messages provided in request")
	}

	// 映射模型 ID（优先使用 req.Model，为空则使用默认值）
	modelID := getKiroModel(req.Model)
	log.Debugf("[kiro] convertRequest modelID=%s msgCount=%d toolCount=%d", modelID, len(req.Messages), len(req.Tools))

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

	log.Debugf("[kiro] convertRequest done, historyLen=%d hasTools=%v hasSystemPrompt=%v",
		len(history), len(kiroTools) > 0, systemPrompt != "")

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

// ParseAWSEventStream 解析 AWS 事件流二进制格式（application/vnd.amazon.eventstream）
// 将所有事件帧聚合为一个 KiroResponse
//
// Kiro 实际返回的是二进制帧头 + JSON payload 的混合格式，
// 采用与 proxy-gateway/internal/adapter/kiro.go 相同的策略：
// 将原始字节转为字符串，直接扫描 JSON 对象，根据字段内容判断事件类型。
func ParseAWSEventStream(data []byte) (*KiroResponse, error) {
	log.Debugf("[kiro] ParseAWSEventStream dataLen=%d", len(data))
	parseResult := parseKiroEventStreamFull(data)
	log.Debugf("[kiro] ParseAWSEventStream parsed contentLen=%d toolUses=%d contextUsagePct=%.2f stop=%v",
		len(parseResult.Content), len(parseResult.ToolUses), parseResult.ContextUsagePct, parseResult.Stop)

	result := &KiroResponse{
		Content: parseResult.Content,
	}

	// 转换工具调用
	for _, tu := range parseResult.ToolUses {
		var args map[string]interface{}
		if tu.Input != "" {
			_ = json.Unmarshal([]byte(tu.Input), &args)
		}
		result.ToolCalls = append(result.ToolCalls, KiroToolCall{
			ID:        tu.ID,
			Name:      tu.Name,
			Arguments: args,
		})
	}

	// 转换 token 用量（基于 contextUsagePercentage 估算）
	if parseResult.ContextUsagePct > 0 {
		const kiroTotalContextTokens = 172500
		totalTokens := int(float64(kiroTotalContextTokens) * parseResult.ContextUsagePct / 100)
		outputTokens := estimateKiroTokenCount(result.Content)
		inputTokens := totalTokens - outputTokens
		if inputTokens < 0 {
			inputTokens = 0
		}
		result.Usage = &KiroUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  totalTokens,
		}
	}

	return result, nil
}

// ParseAWSEventStreamChunks 解析 AWS 事件流二进制格式，逐帧返回 KiroResponse（用于流式处理）
// 每个内容片段对应一个流式响应块
func ParseAWSEventStreamChunks(data []byte) ([]*KiroResponse, error) {
	log.Debugf("[kiro] ParseAWSEventStreamChunks dataLen=%d", len(data))
	parseResult := parseKiroEventStreamFull(data)
	log.Debugf("[kiro] ParseAWSEventStreamChunks parsed contentLen=%d toolUses=%d contextUsagePct=%.2f",
		len(parseResult.Content), len(parseResult.ToolUses), parseResult.ContextUsagePct)

	var chunks []*KiroResponse

	// 内容块：按字符逐个拆分以模拟流式输出
	// 实际上 Kiro 的每个 assistantResponseEvent 就是一个内容片段，直接作为一个 chunk
	if parseResult.Content != "" {
		chunks = append(chunks, &KiroResponse{Content: parseResult.Content})
	}

	// 工具调用块
	for _, tu := range parseResult.ToolUses {
		var args map[string]interface{}
		if tu.Input != "" {
			_ = json.Unmarshal([]byte(tu.Input), &args)
		}
		chunks = append(chunks, &KiroResponse{
			ToolCalls: []KiroToolCall{
				{
					ID:        tu.ID,
					Name:      tu.Name,
					Arguments: args,
				},
			},
		})
	}

	// usage 块（基于 contextUsagePercentage 估算）
	if parseResult.ContextUsagePct > 0 {
		const kiroTotalContextTokens = 172500
		totalTokens := int(float64(kiroTotalContextTokens) * parseResult.ContextUsagePct / 100)
		outputTokens := estimateKiroTokenCount(parseResult.Content)
		inputTokens := totalTokens - outputTokens
		if inputTokens < 0 {
			inputTokens = 0
		}
		chunks = append(chunks, &KiroResponse{
			Usage: &KiroUsage{
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				TotalTokens:  totalTokens,
			},
		})
	}

	return chunks, nil
}

// ============================================================
// 以下为 Kiro AWS 事件流核心解析逻辑
// 参考自 proxy-gateway/internal/adapter/kiro.go 的实现策略：
// 将二进制数据转为字符串，直接扫描 JSON 对象，根据字段内容判断事件类型
// ============================================================

// kiroToolUseItem Kiro 工具调用信息（内部解析用）
type kiroToolUseItem struct {
	ID    string
	Name  string
	Input string // JSON 字符串（可能是流式片段拼接）
}

// kiroParseResult Kiro 事件流解析结果（内部解析用）
type kiroParseResult struct {
	Content         string
	Stop            bool
	ContextUsagePct float64
	ToolUses        []kiroToolUseItem
}

// parseKiroEventStreamFull 从 AWS 事件流二进制数据中提取完整解析结果（含工具调用）
// 策略：将原始字节转为字符串，逐个扫描 JSON 对象，根据字段内容判断事件类型
func parseKiroEventStreamFull(data []byte) kiroParseResult {
	rawStr := string(data)
	return extractKiroJSONFragments(rawStr)
}

// extractKiroJSONFragments 从字符串中扫描并提取所有 Kiro JSON 事件片段
// Kiro 事件类型通过 JSON 字段内容区分：
//   - {"content":"..."} → assistantResponseEvent（文本内容）
//   - {"toolUseId":"...","name":"..."} → toolUseEvent（工具名称）
//   - {"toolUseId":"...","input":{...}} → toolUseEvent（工具输入）
//   - {"contextUsagePercentage":...} → contextUsageEvent（流结束 + token 用量）
func extractKiroJSONFragments(rawStr string) kiroParseResult {
	var result kiroParseResult
	var sb strings.Builder
	// toolUseMap 用于聚合同一 toolUseId 的 name 和 input
	toolUseMap := make(map[string]*kiroToolUseItem)
	toolUseOrder := make([]string, 0)
	searchStart := 0

	for searchStart < len(rawStr) {
		// 找到下一个 { 的位置
		start := strings.Index(rawStr[searchStart:], "{")
		if start == -1 {
			break
		}
		start += searchStart

		// 找到匹配的 }
		end := findKiroMatchingBrace(rawStr, start)
		if end == -1 {
			break
		}

		jsonCandidate := rawStr[start : end+1]
		var eventData map[string]any
		if err := json.Unmarshal([]byte(jsonCandidate), &eventData); err != nil {
			searchStart = start + 1
			continue
		}

		// 处理 contextUsagePercentage 事件（流结束标志 + token 用量）
		if pct, ok := eventData["contextUsagePercentage"]; ok {
			if v, ok := pct.(float64); ok {
				result.ContextUsagePct = v
			}
			result.Stop = true
			searchStart = end + 1
			continue
		}

		// 处理工具调用事件（toolUseEvent）
		// Kiro 返回两种 toolUseEvent：
		// 1. {"name":"xxx","toolUseId":"yyy"} - 工具名称和 ID
		// 2. {"input":{...},"toolUseId":"yyy"} - 工具输入（对象格式）
		// 3. {"input":"...","toolUseId":"yyy"} - 工具输入（流式 JSON 片段字符串格式）
		if toolUseID, ok := eventData["toolUseId"].(string); ok && toolUseID != "" {
			if _, exists := toolUseMap[toolUseID]; !exists {
				toolUseMap[toolUseID] = &kiroToolUseItem{ID: toolUseID}
				toolUseOrder = append(toolUseOrder, toolUseID)
			}
			tu := toolUseMap[toolUseID]
			if name, ok := eventData["name"].(string); ok && name != "" {
				tu.Name = name
			}
			if input, ok := eventData["input"]; ok {
				switch v := input.(type) {
				case string:
					// 流式 JSON 片段字符串，直接追加
					tu.Input += v
				default:
					// 对象格式，序列化为 JSON 字符串
					if inputBytes, err := json.Marshal(v); err == nil {
						tu.Input = string(inputBytes)
					}
				}
			}
			searchStart = end + 1
			continue
		}

		// 处理文本内容事件（assistantResponseEvent）
		if text, ok := eventData["content"].(string); ok {
			sb.WriteString(text)
		}

		searchStart = end + 1
	}

	result.Content = sb.String()
	// 按顺序收集工具调用
	for _, id := range toolUseOrder {
		result.ToolUses = append(result.ToolUses, *toolUseMap[id])
	}
	return result
}

// findKiroMatchingBrace 找到匹配的右花括号位置（处理嵌套和字符串转义）
func findKiroMatchingBrace(s string, start int) int {
	if start >= len(s) || s[start] != '{' {
		return -1
	}

	count := 1
	inString := false
	escape := false

	for i := start + 1; i < len(s); i++ {
		ch := s[i]

		if escape {
			escape = false
			continue
		}

		if ch == '\\' && inString {
			escape = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if ch == '{' {
				count++
			} else if ch == '}' {
				count--
				if count == 0 {
					return i
				}
			}
		}
	}

	return -1
}

// estimateKiroTokenCount 简单估算 token 数量（约 4 字符 = 1 token）
func estimateKiroTokenCount(text string) int {
	if text == "" {
		return 0
	}
	return len([]rune(text))/4 + 1
}

// convertKiroResponseToStreamChunk 将单个 KiroResponse（来自 AWS 事件流的一帧）转换为流式响应块
func convertKiroResponseToStreamChunk(kiroResp *KiroResponse) *types.Response {
	response := &types.Response{
		ID:        generateResponseID(),
		Object:    "chat.completion.chunk",
		Created:   time.Now().Unix(),
		Model:     "kiro-codewhisperer",
		Timestamp: time.Now(),
		Done:      false,
		IsPartial: true,
	}

	// 处理错误
	if kiroResp.Error != nil {
		response.Error = &types.ResponseError{
			Code:    &kiroResp.Error.Code,
			Message: kiroResp.Error.Message,
		}
		response.Done = true
		return response
	}

	// 处理 usage 帧（meteringEvent）
	if kiroResp.Usage != nil && kiroResp.Content == "" && len(kiroResp.ToolCalls) == 0 {
		response.Usage = &types.Usage{
			PromptTokens:     kiroResp.Usage.InputTokens,
			CompletionTokens: kiroResp.Usage.OutputTokens,
			TotalTokens:      kiroResp.Usage.TotalTokens,
		}
		return response
	}

	// 处理工具调用帧
	if len(kiroResp.ToolCalls) > 0 {
		toolCalls := make([]types.ToolCall, 0, len(kiroResp.ToolCalls))
		for _, tc := range kiroResp.ToolCalls {
			var argsBytes []byte
			if tc.Arguments != nil {
				argsBytes, _ = json.Marshal(tc.Arguments)
			}
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: types.FunctionDefinitionParam{
					Name:      tc.Name,
					Arguments: argsBytes,
				},
			})
		}
		response.Choices = []types.Choice{
			{
				Index: 0,
				Delta: types.Message{
					Role:      types.RoleAssistant,
					ToolCalls: toolCalls,
				},
			},
		}
		return response
	}

	// 处理内容帧（assistantResponseEvent）
	response.Choices = []types.Choice{
		{
			Index: 0,
			Delta: types.Message{
				Role:    types.RoleAssistant,
				Content: kiroResp.Content,
			},
		},
	}

	return response
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
