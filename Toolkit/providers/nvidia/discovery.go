// Package nvidia provides model discovery for Nvidia API.
package nvidia

import (
	"context"
	"strings"

	"github.com/superagent/toolkit/pkg/toolkit"
	"github.com/superagent/toolkit/pkg/toolkit/common/discovery"
)

// NvidiaCapabilityInferrer implements capability inference for Nvidia models.
type NvidiaCapabilityInferrer struct{}

// InferCapabilities infers model capabilities from ID and type.
func (n *NvidiaCapabilityInferrer) InferCapabilities(modelID, modelType string) toolkit.ModelCapabilities {
	capabilities := toolkit.ModelCapabilities{}

	modelLower := strings.ToLower(modelID)
	typeLower := strings.ToLower(modelType)

	// Embedding capabilities
	capabilities.SupportsEmbedding = strings.Contains(typeLower, "embedding")

	// Rerank capabilities
	capabilities.SupportsRerank = strings.Contains(typeLower, "rerank")

	// Audio capabilities
	audioKeywords := []string{"tts", "audio", "speech", "voice"}
	capabilities.SupportsAudio = n.containsAny(modelLower, audioKeywords) || n.containsAny(typeLower, audioKeywords)

	// Vision capabilities
	visionKeywords := []string{"vl", "vision", "visual", "multimodal"}
	capabilities.SupportsVision = n.containsAny(modelLower, visionKeywords) || n.containsAny(typeLower, visionKeywords)

	// Chat capabilities - Nvidia specific models
	specializedKeywords := []string{
		"embedding", "rerank", "tts", "speech", "audio", "video",
		"t2v", "i2v", "vl", "vision", "visual", "multimodal", "image",
	}

	isSpecialized := n.containsAny(modelLower, specializedKeywords)
	chatTypeIndicators := []string{"chat", "text", "completion", "instruction", "instruct"}
	hasChatType := n.containsAny(typeLower, chatTypeIndicators)
	chatIDIndicators := []string{
		"instruct", "chat", "llama", "mistral", "mixtral", "gemma",
	}
	hasChatID := n.containsAny(modelLower, chatIDIndicators)

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
	capabilities.FunctionCalling = n.supportsFunctionCalling(modelID)

	// Context window and max tokens
	capabilities.ContextWindow = n.inferContextWindow(modelID)
	capabilities.MaxTokens = n.inferMaxTokens(modelID)

	return capabilities
}

// containsAny checks if the string contains any of the keywords.
func (n *NvidiaCapabilityInferrer) containsAny(str string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(str, keyword) {
			return true
		}
	}
	return false
}

// supportsFunctionCalling checks if model supports function calling.
func (n *NvidiaCapabilityInferrer) supportsFunctionCalling(modelID string) bool {
	modelLower := strings.ToLower(modelID)

	supportedModels := []string{"llama", "mistral", "gemma"}
	for _, model := range supportedModels {
		if strings.Contains(modelLower, model) {
			return true
		}
	}

	return false
}

// inferContextWindow infers context window size.
func (n *NvidiaCapabilityInferrer) inferContextWindow(modelID string) int {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "llama-3.1-405b") {
		return 131072
	}
	if strings.Contains(modelLower, "llama-3.1") {
		return 131072
	}
	if strings.Contains(modelLower, "llama-3") {
		return 8192
	}
	if strings.Contains(modelLower, "mistral-7b") {
		return 32768
	}

	return 4096
}

// inferMaxTokens infers maximum tokens for output.
func (n *NvidiaCapabilityInferrer) inferMaxTokens(modelID string) int {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "llama-3.1-405b") {
		return 4096
	}
	if strings.Contains(modelLower, "llama-3.1") {
		return 4096
	}

	return 4096
}

// NvidiaModelFormatter implements model formatting for Nvidia models.
type NvidiaModelFormatter struct{}

// FormatModelName formats model ID into human-readable name.
func (n *NvidiaModelFormatter) FormatModelName(modelID string) string {
	parts := strings.Split(modelID, "/")
	if len(parts) > 1 {
		name := strings.ReplaceAll(parts[1], "-", " ")
		name = strings.ReplaceAll(name, "_", " ")
		return parts[0] + " " + name
	}
	return strings.ReplaceAll(modelID, "-", " ")
}

// GetModelDescription returns a description for the model.
func (n *NvidiaModelFormatter) GetModelDescription(modelID string) string {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "llama") {
		return "Meta Llama models hosted on Nvidia"
	}
	if strings.Contains(modelLower, "mistral") {
		return "Mistral models hosted on Nvidia"
	}
	if strings.Contains(modelLower, "gemma") {
		return "Google Gemma models hosted on Nvidia"
	}

	return "Nvidia hosted model"
}

// Discovery implements the ModelDiscovery interface for Nvidia.
type Discovery struct {
	*discovery.BaseDiscovery
	client *Client
}

// NewDiscovery creates a new Nvidia model discovery instance.
func NewDiscovery(apiKey string) *Discovery {
	client := NewClient(apiKey)
	base := discovery.NewBaseDiscovery(
		"nvidia",
		&NvidiaCapabilityInferrer{},
		&discovery.DefaultCategoryInferrer{},
		&NvidiaModelFormatter{},
	)

	return &Discovery{
		BaseDiscovery: base,
		client:        client,
	}
}

// Discover discovers available models from Nvidia.
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
