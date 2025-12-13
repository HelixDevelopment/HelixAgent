// Package discovery provides generic model discovery framework and utilities.
package discovery

import (
	"strings"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
)

// CapabilityInferrer defines the interface for inferring model capabilities.
type CapabilityInferrer interface {
	InferCapabilities(modelID, modelType string) toolkit.ModelCapabilities
}

// CategoryInferrer defines the interface for inferring model categories.
type CategoryInferrer interface {
	InferCategory(modelID, modelType string) toolkit.ModelCategory
}

// ModelFormatter defines the interface for formatting model information.
type ModelFormatter interface {
	FormatModelName(modelID string) string
	GetModelDescription(modelID string) string
}

// BaseDiscovery provides common functionality for model discovery.
type BaseDiscovery struct {
	capabilityInferrer CapabilityInferrer
	categoryInferrer   CategoryInferrer
	modelFormatter     ModelFormatter
	providerName       string
}

// NewBaseDiscovery creates a new base discovery instance.
func NewBaseDiscovery(providerName string, capabilityInferrer CapabilityInferrer, categoryInferrer CategoryInferrer, modelFormatter ModelFormatter) *BaseDiscovery {
	return &BaseDiscovery{
		capabilityInferrer: capabilityInferrer,
		categoryInferrer:   categoryInferrer,
		modelFormatter:     modelFormatter,
		providerName:       providerName,
	}
}

// ConvertToModelInfo converts raw model info to toolkit ModelInfo.
func (d *BaseDiscovery) ConvertToModelInfo(modelID, modelType string) toolkit.ModelInfo {
	capabilities := d.capabilityInferrer.InferCapabilities(modelID, modelType)

	return toolkit.ModelInfo{
		ID:           modelID,
		Name:         d.modelFormatter.FormatModelName(modelID),
		Category:     d.categoryInferrer.InferCategory(modelID, modelType),
		Capabilities: capabilities,
		Provider:     d.providerName,
		Description:  d.modelFormatter.GetModelDescription(modelID),
	}
}

// DefaultCapabilityInferrer provides default capability inference logic.
type DefaultCapabilityInferrer struct{}

// InferCapabilities infers model capabilities from ID and type.
func (d *DefaultCapabilityInferrer) InferCapabilities(modelID, modelType string) toolkit.ModelCapabilities {
	capabilities := toolkit.ModelCapabilities{}

	modelLower := strings.ToLower(modelID)
	typeLower := strings.ToLower(modelType)

	// Embedding capabilities
	capabilities.SupportsEmbedding = strings.Contains(typeLower, "embedding")

	// Rerank capabilities
	capabilities.SupportsRerank = strings.Contains(typeLower, "rerank")

	// Audio capabilities
	audioKeywords := []string{"tts", "audio", "speech", "voice"}
	capabilities.SupportsAudio = d.containsAny(modelLower, audioKeywords) || d.containsAny(typeLower, audioKeywords)

	// Video capabilities
	videoKeywords := []string{"t2v", "video", "i2v", "flux"}
	capabilities.SupportsVideo = d.containsAny(modelLower, videoKeywords) || d.containsAny(typeLower, videoKeywords)

	// Vision capabilities
	visionKeywords := []string{"vl", "vision", "visual", "multimodal"}
	capabilities.SupportsVision = d.containsAny(modelLower, visionKeywords) || d.containsAny(typeLower, visionKeywords)

	// Chat capabilities - conservative approach
	specializedKeywords := []string{
		"embedding", "rerank", "tts", "speech", "audio", "video",
		"t2v", "i2v", "flux", "image",
	}

	isSpecialized := d.containsAny(modelLower, specializedKeywords)
	chatTypeIndicators := []string{"chat", "text", "completion", "instruction", "instruct"}
	hasChatType := d.containsAny(typeLower, chatTypeIndicators)
	chatIDIndicators := []string{
		"instruct", "chat", "qwen", "deepseek", "glm", "kimi",
		"llama", "mistral", "mixtral", "gemma", "phi", "yi",
	}
	hasChatID := d.containsAny(modelLower, chatIDIndicators)

	// Multimodal and vision models can still support chat
	isMultimodal := d.containsAny(modelLower, []string{"vl", "vision", "visual", "multimodal"}) ||
		d.containsAny(typeLower, []string{"multimodal"})

	if hasChatType && (!isSpecialized || isMultimodal) {
		capabilities.SupportsChat = true
	} else if hasChatID && (!isSpecialized || isMultimodal) {
		capabilities.SupportsChat = true
	} else if typeLower == "" || typeLower == "unknown" {
		capabilities.SupportsChat = false
	} else {
		capabilities.SupportsChat = false
	}

	// Function calling support
	capabilities.FunctionCalling = d.supportsFunctionCalling(modelID)

	// Context window and max tokens
	capabilities.ContextWindow = d.inferContextWindow(modelID)
	capabilities.MaxTokens = d.inferMaxTokens(modelID)

	return capabilities
}

// containsAny checks if the string contains any of the keywords.
func (d *DefaultCapabilityInferrer) containsAny(s string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(s, keyword) {
			return true
		}
	}
	return false
}

// supportsFunctionCalling checks if model supports function calling.
func (d *DefaultCapabilityInferrer) supportsFunctionCalling(modelID string) bool {
	modelLower := strings.ToLower(modelID)

	supportedModels := []string{"qwen", "deepseek", "glm", "kimi"}
	for _, model := range supportedModels {
		if strings.Contains(modelLower, model) {
			return true
		}
	}

	return false
}

// inferContextWindow infers context window size.
func (d *DefaultCapabilityInferrer) inferContextWindow(modelID string) int {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "deepseek") {
		if strings.Contains(modelLower, "r1") {
			return 131072
		}
		return 131072 // V3 series
	}

	if strings.Contains(modelLower, "qwen") {
		if strings.Contains(modelLower, "qwen3") {
			return 32768
		}
		if strings.Contains(modelLower, "qwen2.5") {
			if strings.Contains(modelLower, "72b") {
				return 131072
			}
			return 32768
		}
		if strings.Contains(modelLower, "qwen2-vl") {
			return 32768
		}
	}

	if strings.Contains(modelLower, "glm") {
		if strings.Contains(modelLower, "4.6") {
			return 131072
		}
		return 32768
	}

	if strings.Contains(modelLower, "kimi") {
		return 131072
	}

	return 4096
}

// inferMaxTokens infers maximum tokens for output.
func (d *DefaultCapabilityInferrer) inferMaxTokens(modelID string) int {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "deepseek-r1") {
		return 8192
	}
	if strings.Contains(modelLower, "qwen3") {
		return 8192
	}
	if strings.Contains(modelLower, "qwen2.5-72b") {
		return 8192
	}

	return 4096
}

// DefaultCategoryInferrer provides default category inference logic.
type DefaultCategoryInferrer struct{}

// InferCategory infers the model category.
func (d *DefaultCategoryInferrer) InferCategory(modelID, modelType string) toolkit.ModelCategory {
	typeLower := strings.ToLower(modelType)
	modelLower := strings.ToLower(modelID)

	if strings.Contains(typeLower, "embedding") {
		return toolkit.CategoryEmbedding
	}
	if strings.Contains(typeLower, "rerank") {
		return toolkit.CategoryRerank
	}
	if strings.Contains(modelLower, "vl") || strings.Contains(modelLower, "vision") ||
		strings.Contains(modelLower, "multimodal") {
		return toolkit.CategoryMultimodal
	}
	if strings.Contains(modelLower, "image") || strings.Contains(modelLower, "flux") {
		return toolkit.CategoryImage
	}
	if strings.Contains(modelLower, "video") || strings.Contains(modelLower, "t2v") {
		return toolkit.CategoryMultimodal
	}

	return toolkit.CategoryChat
}

// DefaultModelFormatter provides default model formatting logic.
type DefaultModelFormatter struct{}

// FormatModelName formats model ID into human-readable name.
func (d *DefaultModelFormatter) FormatModelName(modelID string) string {
	parts := strings.Split(modelID, "/")
	if len(parts) > 1 {
		name := strings.ReplaceAll(parts[1], "-", " ")
		name = strings.ReplaceAll(name, "_", " ")
		return parts[0] + " " + name
	}
	return strings.ReplaceAll(modelID, "-", " ")
}

// GetModelDescription returns a description for the model.
func (d *DefaultModelFormatter) GetModelDescription(modelID string) string {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "qwen") {
		return "Qwen series models"
	}
	if strings.Contains(modelLower, "deepseek") {
		return "DeepSeek models for advanced reasoning"
	}
	if strings.Contains(modelLower, "glm") {
		return "GLM models"
	}
	if strings.Contains(modelLower, "kimi") {
		return "Kimi models with advanced capabilities"
	}

	return "AI model"
}
