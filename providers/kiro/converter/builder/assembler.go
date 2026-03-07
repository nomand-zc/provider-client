package builder

import (
	"github.com/google/uuid"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/builder/types"
)

// Assemble 将 BuildContext 中的数据组装为最终的 *types.Request
func Assemble(ctx *BuildContext) *types.Request {
	req := &types.Request{}
	req.ConversationState.ChatTriggerType = "MANUAL"
	req.ConversationState.ConversationId = uuid.NewString()

	// 构建 userInputMessage
	userInputMsg := types.UserInputMessage{
		Content: ctx.CurrentContent,
		ModelId: ctx.ModelId,
		Origin:  "AI_EDITOR",
	}
	if len(ctx.CurrentImages) > 0 {
		userInputMsg.Images = ctx.CurrentImages
	}

	// 构建 userInputMessageContext
	userInputMsgCtx := &types.UserInputMessageContext{}
	hasCtx := false

	if len(ctx.CurrentToolResults) > 0 {
		deduped := DeduplicateToolResults(ctx.CurrentToolResults)
		userInputMsgCtx.ToolResults = deduped
		hasCtx = true
	}
	if len(ctx.KiroTools) > 0 {
		userInputMsgCtx.Tools = ctx.KiroTools
		hasCtx = true
	}
	if hasCtx {
		userInputMsg.UserInputMessageContext = userInputMsgCtx
	}

	req.ConversationState.CurrentMessage.UserInputMessage = userInputMsg

	if len(ctx.History) > 0 {
		req.ConversationState.History = ctx.History
	}

	return req
}
