package kiro

import (
	"encoding/json"
	"fmt"
	"time"
)

type Credentials struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	ProfileArn   string     `json:"profile_arn,omitempty"`
	AuthMethod   string     `json:"auth_method,omitempty"`
	Provider     string     `json:"provider,omitempty"`
	Region       string     `json:"region,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

// ExtractCredentials extracts credentials from the given byte slice
func ExtractCredentials(creds []byte) (*Credentials, error) {
	var c Credentials
	if err := json.Unmarshal(creds, &c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}
	return &c, nil
}

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
	JSON map[string]interface{} `json:"json"`
}

// ToolResult represents a tool execution result
type ToolResult struct {
	ToolUseID string `json:"toolUseId"`
	Content   string `json:"content"`
	Status    string `json:"status,omitempty"`
}

// HistoryMessage represents a message in conversation history
type HistoryMessage struct {
	UserInputMessage       *UserInputMessage       `json:"userInputMessage,omitempty"`
	AssistantResponseMessage *AssistantResponseMessage `json:"assistantResponseMessage,omitempty"`
}

// AssistantResponseMessage represents an assistant response in history
type AssistantResponseMessage struct {
	Content   string     `json:"content"`
	ToolUses  []ToolUse  `json:"toolUses,omitempty"`
}

// ToolUse represents a tool call made by the assistant
type ToolUse struct {
	Input     interface{} `json:"input"`
	Name      string      `json:"name"`
	ToolUseID string      `json:"toolUseId"`
}

// KiroResponse represents the Kiro CodeWhisperer API response format
type KiroResponse struct {
	Content         string         `json:"content"`
	ThinkingContent string         `json:"thinkingContent,omitempty"`
	ToolCalls       []KiroToolCall `json:"toolCalls,omitempty"`
	Usage           *KiroUsage     `json:"usage,omitempty"`
	Error           *KiroError     `json:"error,omitempty"`
}

// KiroToolCall represents a tool call in Kiro response
type KiroToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// KiroUsage represents token usage information in Kiro response
type KiroUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// KiroError represents an error in Kiro response
type KiroError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
