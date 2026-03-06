package kiro

const (
	// kiro 模型列表
	CLAUDE_HAIKU_4_5                = "claude-haiku-4.5"
	CLAUDE_OPUS_4_5                 = "claude-opus-4.5"
	CLAUDE_SONNET_4_5               = "claude-sonnet-4.5"
	CLAUDE_SONNET_4_5_20250929_V1_0 = "CLAUDE_SONNET_4_5_20250929_V1_0"
)

var (
	ModelList = []string{
		CLAUDE_HAIKU_4_5,
		CLAUDE_OPUS_4_5,
		CLAUDE_SONNET_4_5,
	}

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
