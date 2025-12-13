// Package siliconflow provides model discovery for SiliconFlow API.
package siliconflow

import (
	"context"
	"strings"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit/common/discovery"
)

// SiliconFlowCapabilityInferrer implements capability inference for SiliconFlow models.
type SiliconFlowCapabilityInferrer struct{}

// InferCapabilities infers model capabilities from ID and type.
func (s *SiliconFlowCapabilityInferrer) InferCapabilities(modelID, modelType string) toolkit.ModelCapabilities {
	capabilities := toolkit.ModelCapabilities{}

	modelLower := strings.ToLower(modelID)
	typeLower := strings.ToLower(modelType)

	// Embedding capabilities
	capabilities.SupportsEmbedding = strings.Contains(typeLower, "embedding")

	// Rerank capabilities
	capabilities.SupportsRerank = strings.Contains(typeLower, "rerank")

	// Audio capabilities
	audioKeywords := []string{"tts", "audio", "speech", "voice"}
	capabilities.SupportsAudio = s.containsAny(modelLower, audioKeywords) || s.containsAny(typeLower, audioKeywords)

	// Video capabilities
	videoKeywords := []string{"t2v", "video", "i2v", "flux"}
	capabilities.SupportsVideo = s.containsAny(modelLower, videoKeywords) || s.containsAny(typeLower, videoKeywords)

	// Vision capabilities
	visionKeywords := []string{"vl", "vision", "visual", "multimodal"}
	capabilities.SupportsVision = s.containsAny(modelLower, visionKeywords) || s.containsAny(typeLower, visionKeywords)

	// Chat capabilities - more conservative approach
	specializedKeywords := []string{
		"embedding", "rerank", "tts", "speech", "audio", "video",
		"t2v", "i2v", "flux", "vl", "vision", "visual", "multimodal", "image",
	}

	isSpecialized := s.containsAny(modelLower, specializedKeywords)
	chatTypeIndicators := []string{"chat", "text", "completion", "instruction", "instruct"}
	hasChatType := s.containsAny(typeLower, chatTypeIndicators)
	chatIDIndicators := []string{
		"instruct", "chat", "qwen", "deepseek", "glm", "kimi",
		"llama", "mistral", "mixtral", "gemma", "phi", "yi",
	}
	hasChatID := s.containsAny(modelLower, chatIDIndicators)

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
	capabilities.FunctionCalling = s.supportsFunctionCalling(modelID)

	// Context window and max tokens
	capabilities.ContextWindow = s.inferContextWindow(modelID)
	capabilities.MaxTokens = s.inferMaxTokens(modelID)

	return capabilities
}

// containsAny checks if the string contains any of the keywords.
func (s *SiliconFlowCapabilityInferrer) containsAny(str string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(str, keyword) {
			return true
		}
	}
	return false
}

// supportsFunctionCalling checks if model supports function calling.
func (s *SiliconFlowCapabilityInferrer) supportsFunctionCalling(modelID string) bool {
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
func (s *SiliconFlowCapabilityInferrer) inferContextWindow(modelID string) int {
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
func (s *SiliconFlowCapabilityInferrer) inferMaxTokens(modelID string) int {
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

// SiliconFlowModelFormatter implements model formatting for SiliconFlow models.
type SiliconFlowModelFormatter struct{}

// FormatModelName formats model ID into human-readable name.
func (s *SiliconFlowModelFormatter) FormatModelName(modelID string) string {
	parts := strings.Split(modelID, "/")
	if len(parts) > 1 {
		name := strings.ReplaceAll(parts[1], "-", " ")
		name = strings.ReplaceAll(name, "_", " ")
		return parts[0] + " " + name
	}
	return strings.ReplaceAll(modelID, "-", " ")
}

// GetModelDescription returns a description for the model.
func (s *SiliconFlowModelFormatter) GetModelDescription(modelID string) string {
	modelLower := strings.ToLower(modelID)

	if strings.Contains(modelLower, "qwen") {
		return "Qwen series models from Alibaba Cloud"
	}
	if strings.Contains(modelLower, "deepseek") {
		return "DeepSeek models for advanced reasoning"
	}
	if strings.Contains(modelLower, "glm") {
		return "GLM models from Tsinghua University"
	}
	if strings.Contains(modelLower, "kimi") {
		return "Kimi models with advanced capabilities"
	}

	return "SiliconFlow hosted model"
}

// Discovery implements the ModelDiscovery interface for SiliconFlow.
type Discovery struct {
	*discovery.BaseDiscovery
	client *Client
}

// NewDiscovery creates a new SiliconFlow model discovery instance.
func NewDiscovery(apiKey string) *Discovery {
	client := NewClient(apiKey)
	base := discovery.NewBaseDiscovery(
		"siliconflow",
		&SiliconFlowCapabilityInferrer{},
		&discovery.DefaultCategoryInferrer{},
		&SiliconFlowModelFormatter{},
	)

	return &Discovery{
		BaseDiscovery: base,
		client:        client,
	}
}

// Discover discovers available models from SiliconFlow.
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
