// Package openrouter provides model discovery for OpenRouter API.
package openrouter

import (
	"context"
	"strings"

	"github.com/superagent/toolkit/pkg/toolkit"
	"github.com/superagent/toolkit/pkg/toolkit/common/discovery"
)

// OpenRouterCapabilityInferrer implements capability inference for OpenRouter models.
type OpenRouterCapabilityInferrer struct{}

// InferCapabilities infers model capabilities from ID and type.
func (o *OpenRouterCapabilityInferrer) InferCapabilities(modelID, modelType string) toolkit.ModelCapabilities {
	capabilities := toolkit.ModelCapabilities{}

	modelLower := strings.ToLower(modelID)
	typeLower := strings.ToLower(modelType)

	// Embedding capabilities
	capabilities.SupportsEmbedding = strings.Contains(typeLower, "embedding")

	// Rerank capabilities
	capabilities.SupportsRerank = strings.Contains(typeLower, "rerank")

	// Audio capabilities
	audioKeywords := []string{"tts", "audio", "speech", "voice"}
	capabilities.SupportsAudio = o.containsAny(modelLower, audioKeywords) || o.containsAny(typeLower, audioKeywords)

	// Vision capabilities
	visionKeywords := []string{"vl", "vision", "visual", "multimodal"}
	capabilities.SupportsVision = o.containsAny(modelLower, visionKeywords) || o.containsAny(typeLower, visionKeywords)

	// Chat capabilities - OpenRouter has many models
	specializedKeywords := []string{
		"embedding", "rerank", "tts", "speech", "audio", "video",
		"t2v", "i2v", "vl", "vision", "visual", "multimodal", "image",
	}

	isSpecialized := o.containsAny(modelLower, specializedKeywords)
	chatTypeIndicators := []string{"chat", "text", "completion", "instruction", "instruct"}
	hasChatType := o.containsAny(typeLower, chatTypeIndicators)
	chatIDIndicators := []string{
		"instruct", "chat", "gpt", "claude", "llama", "mistral", "mixtral", "gemma",
		"qwen", "deepseek", "glm", "kimi", "phi", "yi",
	}
	hasChatID := o.containsAny(modelLower, chatIDIndicators)

	if hasChatType && !isSpecialized {
		capabilities.SupportsChat = true
	} else if hasChatID && !isSpecialized {
		capabilities.SupportsChat = true
	} else if typeLower == "" || typeLower == "unknown" {
		capabilities.SupportsChat = false
	} else {
		capabilities.SupportsChat = false
	}

	// Function calling support
	capabilities.FunctionCalling = o.supportsFunctionCalling(modelID)

	// Context window and max tokens
	capabilities.ContextWindow = o.inferContextWindow(modelID)
	capabilities.MaxTokens = o.inferMaxTokens(modelID)

	return capabilities
}

// containsAny checks if the string contains any of the keywords.
func (o *OpenRouterCapabilityInferrer) containsAny(str string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(str, keyword) {
			return true
		}
	}
	return false
}

// supportsFunctionCalling checks if model supports function calling.
func (o *OpenRouterCapabilityInferrer) supportsFunctionCalling(modelID string) bool {
	modelLower := strings.ToLower(modelID)

	supportedModels := []string{"gpt", "claude", "llama", "mistral", "gemma", "qwen"}
	for _, model := range supportedModels {
		if strings.Contains(modelLower, model) {
			return true
		}
	}

	return false
}

// inferContextWindow infers context window size.
func (o *OpenRouterCapabilityInferrer) inferContextWindow(modelID string) int {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "claude-3.5-sonnet") {
		return 200000
	}
	if strings.Contains(modelLower, "claude-3") {
		return 200000
	}
	if strings.Contains(modelLower, "gpt-4o") {
		return 128000
	}
	if strings.Contains(modelLower, "gpt-4") {
		return 128000
	}
	if strings.Contains(modelLower, "llama-3.1-405b") {
		return 131072
	}
	if strings.Contains(modelLower, "llama-3.1") {
		return 131072
	}

	return 4096
}

// inferMaxTokens infers maximum tokens for output.
func (o *OpenRouterCapabilityInferrer) inferMaxTokens(modelID string) int {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "claude-3.5-sonnet") {
		return 8192
	}
	if strings.Contains(modelLower, "gpt-4o") {
		return 4096
	}
	if strings.Contains(modelLower, "llama-3.1-405b") {
		return 4096
	}

	return 4096
}

// OpenRouterModelFormatter implements model formatting for OpenRouter models.
type OpenRouterModelFormatter struct{}

// FormatModelName formats model ID into human-readable name.
func (o *OpenRouterModelFormatter) FormatModelName(modelID string) string {
	parts := strings.Split(modelID, "/")
	if len(parts) > 1 {
		name := strings.ReplaceAll(parts[1], "-", " ")
		name = strings.ReplaceAll(name, "_", " ")
		return parts[0] + " " + name
	}
	return strings.ReplaceAll(modelID, "-", " ")
}

// GetModelDescription returns a description for the model.
func (o *OpenRouterModelFormatter) GetModelDescription(modelID string) string {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "anthropic") || strings.Contains(modelLower, "claude") {
		return "Anthropic Claude models via OpenRouter"
	}
	if strings.Contains(modelLower, "openai") || strings.Contains(modelLower, "gpt") {
		return "OpenAI GPT models via OpenRouter"
	}
	if strings.Contains(modelLower, "google") || strings.Contains(modelLower, "gemini") {
		return "Google Gemini models via OpenRouter"
	}
	if strings.Contains(modelLower, "meta") || strings.Contains(modelLower, "llama") {
		return "Meta Llama models via OpenRouter"
	}

	return "Model available via OpenRouter"
}

// Discovery implements the ModelDiscovery interface for OpenRouter.
type Discovery struct {
	*discovery.BaseDiscovery
	client *Client
}

// NewDiscovery creates a new OpenRouter model discovery instance.
func NewDiscovery(apiKey string) *Discovery {
	client := NewClient(apiKey)
	base := discovery.NewBaseDiscovery(
		"openrouter",
		&OpenRouterCapabilityInferrer{},
		&discovery.DefaultCategoryInferrer{},
		&OpenRouterModelFormatter{},
	)

	return &Discovery{
		BaseDiscovery: base,
		client:        client,
	}
}

// Discover discovers available models from OpenRouter.
func (d *Discovery) Discover(ctx context.Context) ([]toolkit.ModelInfo, error) {
	models, err := d.client.GetModels(ctx)
	if err != nil {
		return nil, err
	}

	var modelInfos []toolkit.ModelInfo
	for _, model := range models {
		modelInfo := d.ConvertToModelInfo(model.ID, model.Type)
		modelInfos = append(modelInfos, modelInfo)
	}

	return modelInfos, nil
}
