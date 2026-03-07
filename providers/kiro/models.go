package kiro

const (
	// kiro 模型列表
	CLAUDE_HAIKU_4_5                = "claude-haiku-4.5"
	CLAUDE_OPUS_4_5                 = "claude-opus-4.5"
	CLAUDE_SONNET_4_5               = "claude-sonnet-4.5"
	CLAUDE_SONNET_4_5_20250929_V1_0 = "CLAUDE_SONNET_4_5_20250929_V1_0"
)

// ModelList 列出所有对外公开的模型名称（包含别名）
var (
	ModelList = []string{
		CLAUDE_HAIKU_4_5,
		CLAUDE_OPUS_4_5,
		CLAUDE_SONNET_4_5,
		// 别名：带连字符的写法
		"claude-haiku-4-5",
		"claude-opus-4-5",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-5",
		"claude-sonnet-4-5-20250929",
	}

	// ModelMap 将对外模型名（包含别名）映射到内部实际使用的模型 ID
	ModelMap = map[string]string{
		CLAUDE_HAIKU_4_5:  CLAUDE_HAIKU_4_5,
		CLAUDE_OPUS_4_5:   CLAUDE_OPUS_4_5,
		CLAUDE_SONNET_4_5: CLAUDE_SONNET_4_5_20250929_V1_0,

		"claude-haiku-4-5":           CLAUDE_HAIKU_4_5,
		"claude-opus-4-5":            CLAUDE_OPUS_4_5,
		"claude-opus-4-5-20251101":   CLAUDE_OPUS_4_5,
		"claude-sonnet-4-5":          CLAUDE_SONNET_4_5_20250929_V1_0,
		"claude-sonnet-4-5-20250929": CLAUDE_SONNET_4_5_20250929_V1_0,
	}
)
