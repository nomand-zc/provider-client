package providers

import (
	"context"
	"fmt"
	"unicode/utf8"
)

// defaultApproxRunesPerToken is the default approximate runes per token heuristic.
const defaultApproxRunesPerToken = 4.0

type simpleTokenCounterOptions struct {
	approxRunesPerToken float64
}

// SimpleTokenCounterOption configures a SimpleTokenCounter.
type SimpleTokenCounterOption func(*simpleTokenCounterOptions)

// WithApproxRunesPerToken sets the approximate runes per token heuristic.
// This is a heuristic and may vary across languages and models.
//
// Note:
// Values <= 0 are ignored and the default value is kept.
func WithApproxRunesPerToken(v float64) SimpleTokenCounterOption {
	return func(o *simpleTokenCounterOptions) {
		if v <= 0 {
			return
		}
		o.approxRunesPerToken = v
	}
}

// TokenCounter counts tokens for messages and tools.
// The implementation is model-agnostic to keep the model package lightweight.
type TokenCounter interface {
	// CountTokens returns the estimated token count for a single message.
	CountTokens(ctx context.Context, message Message) (int, error)

	// CountTokensRange returns the estimated token count for a range of messages.
	// This is more efficient than calling CountTokens multiple times.
	CountTokensRange(ctx context.Context, messages []Message, start, end int) (int, error)
}

// SimpleTokenCounter provides a very rough token estimation based on rune length.
// Heuristic: approximately one token per several UTF-8 runes for text fields.
type SimpleTokenCounter struct {
	approxRunesPerToken float64
}

// NewSimpleTokenCounter creates a SimpleTokenCounter.
func NewSimpleTokenCounter(opts ...SimpleTokenCounterOption) *SimpleTokenCounter {
	o := simpleTokenCounterOptions{
		approxRunesPerToken: defaultApproxRunesPerToken,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&o)
	}
	return &SimpleTokenCounter{approxRunesPerToken: o.approxRunesPerToken}
}

// CountTokens estimates tokens for a single message.
func (c *SimpleTokenCounter) CountTokens(_ context.Context, message Message) (int, error) {
	total := 0

	// Count main content.
	total += utf8.RuneCountInString(message.Content)

	// Count reasoning content if present.
	if message.ReasoningContent != "" {
		total += utf8.RuneCountInString(message.ReasoningContent)
	}

	// Count text parts in multimodal content.
	for _, part := range message.ContentParts {
		if part.Text != nil {
			total += utf8.RuneCountInString(*part.Text)
		}
	}

	// Count tool calls.
	for _, toolCall := range message.ToolCalls {
		total += c.countToolCallRunes(toolCall)
	}

	runesPerToken := c.approxRunesPerToken
	if runesPerToken <= 0 {
		// Fall back to default to avoid division by zero.
		runesPerToken = defaultApproxRunesPerToken
	}
	total = int(float64(total) / runesPerToken)

	// Total should be at least 1 if message is not empty.
	if isMessageNotEmpty(message) {
		return max(total, 1), nil
	}
	return total, nil
}

// countToolCallRunes calculates the rune count for a single tool call.
// This is used for simple token estimation based on character count.
func (c *SimpleTokenCounter) countToolCallRunes(toolCall ToolCall) int {
	total := 0

	// Count runes for tool call type (e.g., "function").
	total += utf8.RuneCountInString(toolCall.Type)

	// Count runes for tool call ID.
	total += utf8.RuneCountInString(toolCall.ID)

	// Count runes for function name.
	total += utf8.RuneCountInString(toolCall.Function.Name)

	// Count runes for function description.
	total += utf8.RuneCountInString(toolCall.Function.Description)

	// Count runes for function arguments (JSON string).
	total += utf8.RuneCount(toolCall.Function.Arguments)

	return total
}

// CountTokensRange estimates tokens for a range of messages.
func (c *SimpleTokenCounter) CountTokensRange(ctx context.Context, messages []Message, start, end int) (int, error) {
	if start < 0 || end > len(messages) || start >= end {
		return 0, fmt.Errorf("invalid range: start=%d, end=%d, len=%d", start, end, len(messages))
	}

	total := 0
	for i := start; i < end; i++ {
		// Ignore error because SimpleTokenCounter's CountTokens does not return error.
		tokens, _ := c.CountTokens(ctx, messages[i])
		total += tokens
	}
	return total, nil
}

// isMessageNotEmpty checks if the message contains any content that should result in at least 1 token.
func isMessageNotEmpty(message Message) bool {
	// Check main content.
	if len(message.Content) > 0 {
		return true
	}

	// Check reasoning content.
	if len(message.ReasoningContent) > 0 {
		return true
	}

	// Check tool calls - any tool call with content should count.
	for _, toolCall := range message.ToolCalls {
		if toolCall.Type != "" || toolCall.ID != "" ||
			toolCall.Function.Name != "" || toolCall.Function.Description != "" ||
			len(toolCall.Function.Arguments) > 0 {
			return true
		}
	}
	return false
}
