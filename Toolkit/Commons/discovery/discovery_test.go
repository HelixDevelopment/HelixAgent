package discovery

import (
	"testing"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
)

func TestNewBaseDiscovery(t *testing.T) {
	capInferrer := &DefaultCapabilityInferrer{}
	catInferrer := &DefaultCategoryInferrer{}
	formatter := &DefaultModelFormatter{}

	discovery := NewBaseDiscovery("test-provider", capInferrer, catInferrer, formatter)

	if discovery.providerName != "test-provider" {
		t.Errorf("Expected provider name 'test-provider', got %s", discovery.providerName)
	}

	if discovery.capabilityInferrer != capInferrer {
		t.Error("Expected capability inferrer to be set")
	}

	if discovery.categoryInferrer != catInferrer {
		t.Error("Expected category inferrer to be set")
	}

	if discovery.modelFormatter != formatter {
		t.Error("Expected model formatter to be set")
	}
}

func TestBaseDiscovery_ConvertToModelInfo(t *testing.T) {
	capInferrer := &DefaultCapabilityInferrer{}
	catInferrer := &DefaultCategoryInferrer{}
	formatter := &DefaultModelFormatter{}

	discovery := NewBaseDiscovery("test-provider", capInferrer, catInferrer, formatter)

	modelInfo := discovery.ConvertToModelInfo("qwen2.5-7b", "chat")

	if modelInfo.ID != "qwen2.5-7b" {
		t.Errorf("Expected ID 'qwen2.5-7b', got %s", modelInfo.ID)
	}

	if modelInfo.Provider != "test-provider" {
		t.Errorf("Expected provider 'test-provider', got %s", modelInfo.Provider)
	}

	if !modelInfo.Capabilities.SupportsChat {
		t.Error("Expected chat capability to be true")
	}

	if modelInfo.Category != toolkit.CategoryChat {
		t.Errorf("Expected category Chat, got %v", modelInfo.Category)
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_Chat(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	caps := inferrer.InferCapabilities("qwen2.5-7b", "chat")

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true for chat model")
	}

	if caps.SupportsEmbedding {
		t.Error("Expected SupportsEmbedding to be false for chat model")
	}

	if caps.SupportsVision {
		t.Error("Expected SupportsVision to be false for chat model")
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_Embedding(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	caps := inferrer.InferCapabilities("text-embedding-ada-002", "embedding")

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false for embedding model")
	}

	if !caps.SupportsEmbedding {
		t.Error("Expected SupportsEmbedding to be true for embedding model")
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_Vision(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	caps := inferrer.InferCapabilities("qwen2-vl-7b", "multimodal")

	if !caps.SupportsVision {
		t.Error("Expected SupportsVision to be true for vision model")
	}

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true for multimodal model")
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_Audio(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	caps := inferrer.InferCapabilities("tts-model", "audio")

	if !caps.SupportsAudio {
		t.Error("Expected SupportsAudio to be true for audio model")
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_Video(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	caps := inferrer.InferCapabilities("flux-video", "video")

	if !caps.SupportsVideo {
		t.Error("Expected SupportsVideo to be true for video model")
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_FunctionCalling(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	// Test supported models
	caps := inferrer.InferCapabilities("qwen-max", "chat")
	if !caps.FunctionCalling {
		t.Error("Expected FunctionCalling to be true for qwen model")
	}

	caps = inferrer.InferCapabilities("deepseek-chat", "chat")
	if !caps.FunctionCalling {
		t.Error("Expected FunctionCalling to be true for deepseek model")
	}

	// Test unsupported model
	caps = inferrer.InferCapabilities("llama-7b", "chat")
	if caps.FunctionCalling {
		t.Error("Expected FunctionCalling to be false for llama model")
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_ContextWindow(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	tests := []struct {
		modelID  string
		expected int
	}{
		{"deepseek-r1", 131072},
		{"deepseek-v3", 131072},
		{"qwen3-8b", 32768},
		{"qwen2.5-72b", 131072},
		{"qwen2.5-7b", 32768},
		{"qwen2-vl-7b", 32768},
		{"glm-4.6", 131072},
		{"glm-4", 32768},
		{"kimi-chat", 131072},
		{"unknown-model", 4096},
	}

	for _, test := range tests {
		caps := inferrer.InferCapabilities(test.modelID, "chat")
		if caps.ContextWindow != test.expected {
			t.Errorf("Expected context window %d for %s, got %d", test.expected, test.modelID, caps.ContextWindow)
		}
	}
}

func TestDefaultCapabilityInferrer_InferCapabilities_MaxTokens(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	tests := []struct {
		modelID  string
		expected int
	}{
		{"deepseek-r1", 8192},
		{"qwen3-8b", 8192},
		{"qwen2.5-72b", 8192},
		{"qwen2.5-7b", 4096},
		{"unknown-model", 4096},
	}

	for _, test := range tests {
		caps := inferrer.InferCapabilities(test.modelID, "chat")
		if caps.MaxTokens != test.expected {
			t.Errorf("Expected max tokens %d for %s, got %d", test.expected, test.modelID, caps.MaxTokens)
		}
	}
}

func TestDefaultCategoryInferrer_InferCategory(t *testing.T) {
	inferrer := &DefaultCategoryInferrer{}

	tests := []struct {
		modelID   string
		modelType string
		expected  toolkit.ModelCategory
	}{
		{"text-embedding", "embedding", toolkit.CategoryEmbedding},
		{"rerank-model", "rerank", toolkit.CategoryRerank},
		{"qwen-vl", "multimodal", toolkit.CategoryMultimodal},
		{"flux-image", "image", toolkit.CategoryImage},
		{"video-model", "video", toolkit.CategoryMultimodal},
		{"qwen-chat", "chat", toolkit.CategoryChat},
		{"unknown", "unknown", toolkit.CategoryChat},
	}

	for _, test := range tests {
		category := inferrer.InferCategory(test.modelID, test.modelType)
		if category != test.expected {
			t.Errorf("Expected category %v for %s/%s, got %v", test.expected, test.modelID, test.modelType, category)
		}
	}
}

func TestDefaultModelFormatter_FormatModelName(t *testing.T) {
	formatter := &DefaultModelFormatter{}

	tests := []struct {
		input    string
		expected string
	}{
		{"qwen2.5-7b", "qwen2.5 7b"},
		{"provider/model-name", "provider model name"},
		{"deepseek-chat", "deepseek chat"},
		{"simple", "simple"},
	}

	for _, test := range tests {
		result := formatter.FormatModelName(test.input)
		if result != test.expected {
			t.Errorf("Expected '%s' for '%s', got '%s'", test.expected, test.input, result)
		}
	}
}

func TestDefaultModelFormatter_GetModelDescription(t *testing.T) {
	formatter := &DefaultModelFormatter{}

	tests := []struct {
		modelID  string
		expected string
	}{
		{"qwen2.5", "Qwen series models"},
		{"deepseek-chat", "DeepSeek models for advanced reasoning"},
		{"glm-4", "GLM models"},
		{"kimi-chat", "Kimi models with advanced capabilities"},
		{"unknown", "AI model"},
	}

	for _, test := range tests {
		result := formatter.GetModelDescription(test.modelID)
		if result != test.expected {
			t.Errorf("Expected '%s' for '%s', got '%s'", test.expected, test.modelID, result)
		}
	}
}

func TestDefaultCapabilityInferrer_containsAny(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	if !inferrer.containsAny("hello world", []string{"world", "test"}) {
		t.Error("Expected true for 'hello world' containing 'world'")
	}

	if inferrer.containsAny("hello world", []string{"test", "other"}) {
		t.Error("Expected false for 'hello world' not containing keywords")
	}
}

func TestDefaultCapabilityInferrer_supportsFunctionCalling(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	if !inferrer.supportsFunctionCalling("qwen-max") {
		t.Error("Expected true for qwen model")
	}

	if !inferrer.supportsFunctionCalling("deepseek-chat") {
		t.Error("Expected true for deepseek model")
	}

	if inferrer.supportsFunctionCalling("llama-7b") {
		t.Error("Expected false for llama model")
	}
}

func TestDefaultCapabilityInferrer_inferContextWindow(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	tests := []struct {
		modelID  string
		expected int
	}{
		{"deepseek-r1", 131072},
		{"deepseek-v3", 131072},
		{"qwen3-8b", 32768},
		{"qwen2.5-72b", 131072},
		{"qwen2.5-7b", 32768},
		{"qwen2-vl-7b", 32768},
		{"glm-4.6", 131072},
		{"glm-4", 32768},
		{"kimi-chat", 131072},
		{"unknown", 4096},
	}

	for _, test := range tests {
		result := inferrer.inferContextWindow(test.modelID)
		if result != test.expected {
			t.Errorf("Expected %d for %s, got %d", test.expected, test.modelID, result)
		}
	}
}

func TestDefaultCapabilityInferrer_inferMaxTokens(t *testing.T) {
	inferrer := &DefaultCapabilityInferrer{}

	tests := []struct {
		modelID  string
		expected int
	}{
		{"deepseek-r1", 8192},
		{"qwen3-8b", 8192},
		{"qwen2.5-72b", 8192},
		{"qwen2.5-7b", 4096},
		{"unknown", 4096},
	}

	for _, test := range tests {
		result := inferrer.inferMaxTokens(test.modelID)
		if result != test.expected {
			t.Errorf("Expected %d for %s, got %d", test.expected, test.modelID, result)
		}
	}
}
