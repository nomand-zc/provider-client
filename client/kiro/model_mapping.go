package kiro

// modelMapping maps standard model names to Kiro-specific model identifiers
var modelMapping = map[string]string{
	"claude-haiku-4-5":           "claude-haiku-4.5",
	"claude-opus-4-5":            "claude-opus-4.5",
	"claude-opus-4-5-20251101":   "claude-opus-4.5",
	"claude-sonnet-4-5":          "CLAUDE_SONNET_4_5_20250929_V1_0",
	"claude-sonnet-4-5-20250929": "CLAUDE_SONNET_4_5_20250929_V1_0",
}

// getKiroModel returns the Kiro-specific model identifier for the given model name
func getKiroModel(modelName string) string {
	if mapped, ok := modelMapping[modelName]; ok {
		return mapped
	}
	// 默认使用 claude-sonnet-4-5
	return modelMapping["claude-sonnet-4-5"]
}
