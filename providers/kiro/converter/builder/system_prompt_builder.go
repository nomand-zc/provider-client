package builder

import "github.com/nomand-zc/provider-client/providers"

// SystemPromptBuilder 负责从消息列表中提取 system prompt：
//   - 将所有 system 消息的内容用 "\n\n" 合并，写入 BuildContext.SystemPrompt
//   - 将非 system 消息写回 BuildContext.Messages
//   - 若过滤后消息列表为空，则设置 Done = true
type SystemPromptBuilder struct{}

// Build 实现 MessageBuilder 接口
func (b *SystemPromptBuilder) Build(ctx *BuildContext) error {
	var systemPrompt string
	var nonSystemMessages []providers.Message

	for _, msg := range ctx.Messages {
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

	ctx.SystemPrompt = systemPrompt
	ctx.Messages = nonSystemMessages

	if len(ctx.Messages) == 0 {
		ctx.Done = true
	}
	return nil
}
