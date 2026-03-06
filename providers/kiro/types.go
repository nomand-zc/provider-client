package kiro

// KiroRequest represents the Kiro CodeWhisperer API request format
type KiroRequest struct {
	ConversationState *ConversationState `json:"conversationState"`
	ProfileArn        string             `json:"profileArn,omitempty"` // social 模式需要
}

// ConversationState represents the AWS CodeWhisperer conversation state
type ConversationState struct {
	ChatTriggerType string           `json:"chatTriggerType"`
	ConversationID  string           `json:"conversationId"`
	CurrentMessage  *CurrentMessage  `json:"currentMessage"`
	History         []HistoryMessage `json:"history,omitempty"`
}

// CurrentMessage represents the current user message
type CurrentMessage struct {
	UserInputMessage *UserInputMessage `json:"userInputMessage"`
}

// UserInputMessage represents a user input message in CodeWhisperer format
type UserInputMessage struct {
	Content                 string                   `json:"content"`
	ModelID                 string                   `json:"modelId"`
	Origin                  string                   `json:"origin"`
	UserInputMessageContext *UserInputMessageContext `json:"userInputMessageContext,omitempty"`
}

// UserInputMessageContext contains tools and tool results
type UserInputMessageContext struct {
	Tools       []ToolSpecificationWrapper `json:"tools,omitempty"`
	ToolResults []ToolResult               `json:"toolResults,omitempty"`
}

// ToolSpecificationWrapper wraps a tool specification
type ToolSpecificationWrapper struct {
	ToolSpecification ToolSpecification `json:"toolSpecification"`
}

// ToolSpecification represents a tool definition in CodeWhisperer format
type ToolSpecification struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolInputSchema wraps the JSON schema for tool input
type ToolInputSchema struct {
	JSON map[string]any `json:"json"`
}

// ToolResult represents a tool execution result
type ToolResult struct {
	ToolUseID string              `json:"toolUseId"`
	Content   []ToolResultContent `json:"content"`
	Status    string              `json:"status,omitempty"`
}

// ToolResultContent represents a single content item in a tool result
type ToolResultContent struct {
	Text string `json:"text"`
}

// HistoryMessage represents a message in conversation history
type HistoryMessage struct {
	UserInputMessage         *UserInputMessage         `json:"userInputMessage,omitempty"`
	AssistantResponseMessage *AssistantResponseMessage `json:"assistantResponseMessage,omitempty"`
}

// AssistantResponseMessage represents an assistant response in history
type AssistantResponseMessage struct {
	Content  string    `json:"content"`
	ToolUses []ToolUse `json:"toolUses,omitempty"`
}

// ToolUse represents a tool call made by the assistant
type ToolUse struct {
	Input     any    `json:"input"`
	Name      string `json:"name"`
	ToolUseID string `json:"toolUseId"`
}

// KiroResponse represents the Kiro CodeWhisperer API response format
type KiroResponse struct {
	Content         string     `json:"content"`
	ThinkingContent string     `json:"thinkingContent,omitempty"`
	ConversationId  string     `json:"conversationId,omitempty"`
	ToolCalls       []ToolCall `json:"toolUses,omitempty"`
	Usage           *Usage     `json:"usage,omitempty"`
	Error           *Error     `json:"error,omitempty"`
}

// ToolCall represents a tool call in Kiro response
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Usage represents token usage information in Kiro response
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// Error represents an error in Kiro response
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
