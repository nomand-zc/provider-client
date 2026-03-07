package builder

import (
	"strings"

	"github.com/nomand-zc/provider-client/providers"
	"github.com/nomand-zc/provider-client/providers/kiro/converter/builder/types"
)

const (
	// maxDescriptionLength 工具描述最大长度
	maxDescriptionLength = 9216
)

// placeholderTool 是当无可用工具时使用的占位工具
var placeholderTool = types.Tool{
	ToolSpecification: types.ToolSpecification{
		Name:        "no_tool_available",
		Description: "This is a placeholder tool when no other tools are available. It does nothing.",
		InputSchema: types.InputSchema{Json: map[string]any{"type": "object", "properties": map[string]any{}}},
	},
}

// ToolsBuilder 负责将 providers.Tool 列表转换为 types.Tool 列表：
//   - 过滤 web_search/websearch、空名称、空描述的工具
//   - 截断超长描述
//   - 若过滤后列表为空，使用占位工具（no_tool_available）填充
//
// 结果写入 BuildContext.KiroTools
type ToolsBuilder struct{}

// Build 实现 MessageBuilder 接口
func (b *ToolsBuilder) Build(ctx *BuildContext) error {
	ctx.KiroTools = buildKiroTools(ctx.Req.Tools)
	return nil
}

// buildKiroTools 将 providers.Tool 列表转换为 types.Tool 列表
func buildKiroTools(tools []providers.Tool) []types.Tool {
	if len(tools) == 0 {
		return []types.Tool{placeholderTool}
	}

	// 过滤 web_search / websearch 及空名称
	var filtered []providers.Tool
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		name := strings.ToLower(tool.Name)
		if name == "web_search" || name == "websearch" {
			continue
		}
		filtered = append(filtered, tool)
	}

	if len(filtered) == 0 {
		return []types.Tool{placeholderTool}
	}

	// 过滤空描述 + 截断超长描述
	var kiroTools []types.Tool
	for _, tool := range filtered {
		desc := tool.Description
		if strings.TrimSpace(desc) == "" {
			continue
		}
		if len(desc) > maxDescriptionLength {
			desc = desc[:maxDescriptionLength] + "..."
		}
		kiroTools = append(kiroTools, types.Tool{
			ToolSpecification: types.ToolSpecification{
				Name:        tool.Name,
				Description: desc,
				InputSchema: types.InputSchema{Json: ConvertSchema(&tool.Parameters)},
			},
		})
	}

	if len(kiroTools) == 0 {
		return []types.Tool{placeholderTool}
	}

	return kiroTools
}