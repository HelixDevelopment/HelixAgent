package siliconflow

import (
	"testing"
)

func TestSiliconFlowCapabilityInferrer_InferCapabilities_Embedding(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

	caps := inferrer.InferCapabilities("text-embedding-ada-002", "embedding")

	if !caps.SupportsEmbedding {
		t.Error("Expected SupportsEmbedding to be true for embedding model")
	}

	if caps.SupportsChat {
		t.Error("Expected SupportsChat to be false for embedding model")
	}
}

func TestSiliconFlowCapabilityInferrer_InferCapabilities_Rerank(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

	caps := inferrer.InferCapabilities("rerank-model", "rerank")

	if !caps.SupportsRerank {
		t.Error("Expected SupportsRerank to be true for rerank model")
	}
}

func TestSiliconFlowCapabilityInferrer_InferCapabilities_Audio(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

	caps := inferrer.InferCapabilities("tts-model", "audio")

	if !caps.SupportsAudio {
		t.Error("Expected SupportsAudio to be true for audio model")
	}
}

func TestSiliconFlowCapabilityInferrer_InferCapabilities_Video(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

	caps := inferrer.InferCapabilities("flux-video", "video")

	if !caps.SupportsVideo {
		t.Error("Expected SupportsVideo to be true for video model")
	}
}

func TestSiliconFlowCapabilityInferrer_InferCapabilities_Vision(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

	caps := inferrer.InferCapabilities("qwen2-vl-7b", "multimodal")

	if !caps.SupportsVision {
		t.Error("Expected SupportsVision to be true for vision model")
	}
}

func TestSiliconFlowCapabilityInferrer_InferCapabilities_Chat(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

	caps := inferrer.InferCapabilities("qwen2.5-7b", "chat")

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true for chat model")
	}

	if caps.SupportsEmbedding {
		t.Error("Expected SupportsEmbedding to be false for chat model")
	}
}

func TestSiliconFlowCapabilityInferrer_InferCapabilities_FunctionCalling(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

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

func TestSiliconFlowCapabilityInferrer_InferCapabilities_ContextWindow(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

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

func TestSiliconFlowCapabilityInferrer_InferCapabilities_MaxTokens(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

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

func TestSiliconFlowCapabilityInferrer_containsAny(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

	if !inferrer.containsAny("hello world", []string{"world", "test"}) {
		t.Error("Expected true for 'hello world' containing 'world'")
	}

	if inferrer.containsAny("hello world", []string{"test", "other"}) {
		t.Error("Expected false for 'hello world' not containing keywords")
	}
}

func TestSiliconFlowCapabilityInferrer_supportsFunctionCalling(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

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

func TestSiliconFlowCapabilityInferrer_inferContextWindow(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

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

func TestSiliconFlowCapabilityInferrer_inferMaxTokens(t *testing.T) {
	inferrer := &SiliconFlowCapabilityInferrer{}

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

func TestSiliconFlowModelFormatter_FormatModelName(t *testing.T) {
	formatter := &SiliconFlowModelFormatter{}

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

func TestSiliconFlowModelFormatter_GetModelDescription(t *testing.T) {
	formatter := &SiliconFlowModelFormatter{}

	tests := []struct {
		modelID  string
		expected string
	}{
		{"qwen2.5", "Qwen series models from Alibaba Cloud"},
		{"deepseek-chat", "DeepSeek models for advanced reasoning"},
		{"glm-4", "GLM models from Tsinghua University"},
		{"kimi-chat", "Kimi models with advanced capabilities"},
		{"unknown", "SiliconFlow hosted model"},
	}

	for _, test := range tests {
		result := formatter.GetModelDescription(test.modelID)
		if result != test.expected {
			t.Errorf("Expected '%s' for '%s', got '%s'", test.expected, test.modelID, result)
		}
	}
}

func TestNewDiscovery(t *testing.T) {
	discovery := NewDiscovery("test-api-key")

	if discovery == nil {
		t.Error("Expected non-nil discovery")
	}

	if discovery.BaseDiscovery == nil {
		t.Error("Expected BaseDiscovery to be initialized")
	}

	if discovery.client == nil {
		t.Error("Expected client to be initialized")
	}
}

func TestDiscovery_Discover(t *testing.T) {
	// Test the capability inferrer directly
	inferrer := &SiliconFlowCapabilityInferrer{}
	caps := inferrer.InferCapabilities("qwen2.5-7b", "chat")

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true")
	}

	// Test the formatter
	formatter := &SiliconFlowModelFormatter{}
	name := formatter.FormatModelName("qwen2.5-7b")
	if name != "qwen2.5 7b" {
		t.Errorf("Expected name 'qwen2.5 7b', got %s", name)
	}

	// Test basic discovery creation
	discovery := NewDiscovery("test-key")
	if discovery == nil {
		t.Error("Expected non-nil discovery")
	}
}
