// Package proxy provides API proxy functionality with iFlow-specific headers and HMAC signatures.
package proxy

// Model represents an AI model available through iFlow.
type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GetAvailableModels returns the list of supported models.
func GetAvailableModels() []Model {
	return []Model{
		// Text models
		{ID: "glm-4.6", Name: "GLM-4.6", Description: "智谱 GLM-4.6"},
		{ID: "glm-4.7", Name: "GLM-4.7", Description: "智谱 GLM-4.7"},
		{ID: "glm-5", Name: "GLM-5", Description: "智谱 GLM-5 (推荐)"},
		{ID: "iFlow-ROME-30BA3B", Name: "iFlow-ROME-30BA3B", Description: "iFlow ROME 30B (快速)"},
		{ID: "deepseek-v3.2-chat", Name: "DeepSeek-V3.2", Description: "DeepSeek V3.2 对话模型"},
		{ID: "qwen3-coder-plus", Name: "Qwen3-Coder-Plus", Description: "通义千问 Qwen3 Coder Plus"},
		{ID: "kimi-k2", Name: "Kimi-K2", Description: "Moonshot Kimi K2"},
		{ID: "kimi-k2-thinking", Name: "Kimi-K2-Thinking", Description: "Moonshot Kimi K2 思考模型"},
		{ID: "kimi-k2.5", Name: "Kimi-K2.5", Description: "Moonshot Kimi K2.5"},
		{ID: "kimi-k2-0905", Name: "Kimi-K2-0905", Description: "Moonshot Kimi K2 0905"},
		{ID: "minimax-m2.5", Name: "MiniMax-M2.5", Description: "MiniMax M2.5"},
		// Vision models
		{ID: "qwen-vl-max", Name: "Qwen-VL-Max", Description: "通义千问 VL Max 视觉模型"},
	}
}

// IsVisionModel checks if a model supports vision features.
func IsVisionModel(modelID string) bool {
	return modelID == "qwen-vl-max"
}

// ModelParams contains model-specific request parameters.
type ModelParams struct {
	ThinkingMode       bool              `json:"thinking_mode,omitempty"`
	Reasoning          bool              `json:"reasoning,omitempty"`
	EnableThinking     bool              `json:"enable_thinking,omitempty"`
	ChatTemplateKwargs map[string]bool   `json:"chat_template_kwargs,omitempty"`
	Thinking           map[string]string `json:"thinking,omitempty"`
}

// GetModelParams returns model-specific parameters for the given model.
// This matches the iFlow CLI's model configuration.
func GetModelParams(modelID string) ModelParams {
	params := ModelParams{}

	switch {
	case modelID == "glm-5":
		// GLM-5 special configuration
		params.ThinkingMode = true
		params.Reasoning = true
		params.EnableThinking = true
		params.ChatTemplateKwargs = map[string]bool{"enable_thinking": true}
		params.Thinking = map[string]string{"type": "enabled"}

	case modelID == "glm-4.7":
		// GLM-4.7
		params.ChatTemplateKwargs = map[string]bool{"enable_thinking": true}

	case modelID[:3] == "glm":
		// Other GLM models
		params.ChatTemplateKwargs = map[string]bool{"enable_thinking": true}

	case modelID[:8] == "deepseek":
		// DeepSeek models
		params.ThinkingMode = true
		params.Reasoning = true

	case modelID[:8] == "kimi-k2.5":
		// Kimi K2.5
		params.Thinking = map[string]string{"type": "enabled"}

	case len(modelID) > 10 && modelID[len(modelID)-8:] == "thinking":
		// Models with "thinking" suffix
		params.ThinkingMode = true

	case modelID[:5] == "mimo-":
		// mimo- models
		params.Thinking = map[string]string{"type": "enabled"}

	case contains(modelID, "claude"):
		// Claude models
		params.ChatTemplateKwargs = map[string]bool{"enable_thinking": true}

	case contains(modelID, "sonnet-"):
		// sonnet- models
		params.ChatTemplateKwargs = map[string]bool{"enable_thinking": true}

	case contains(modelID, "reasoning"):
		// Models with "reasoning"
		params.Reasoning = true

	// Qwen 4B models don't support thinking - remove params
	default:
		if contains(modelID, "4b") {
			params = ModelParams{}
		}
	}

	return params
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) >= len(substr) && s[:len(substr)] == substr
}
