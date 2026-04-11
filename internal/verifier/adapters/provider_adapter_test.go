package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultProviderAdapterConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultProviderAdapterConfig()
	if cfg == nil {
		t.Fatal("DefaultProviderAdapterConfig returned nil")
	}

	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected Timeout 60s, got %v", cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if !cfg.EnableStreaming {
		t.Error("expected EnableStreaming to be true")
	}
	if !cfg.EnableHealthCheck {
		t.Error("expected EnableHealthCheck to be true")
	}
}

func TestNewProviderAdapter(t *testing.T) {
	t.Parallel()
	adapter, err := NewProviderAdapter("openai", "OpenAI", "sk-test", "https://api.openai.com", nil)
	if err != nil {
		t.Fatalf("NewProviderAdapter failed: %v", err)
	}
	if adapter == nil {
		t.Fatal("adapter is nil")
	}
	if adapter.GetProviderID() != "openai" {
		t.Errorf("expected provider ID 'openai', got '%s'", adapter.GetProviderID())
	}
	if adapter.GetProviderName() != "OpenAI" {
		t.Errorf("expected provider name 'OpenAI', got '%s'", adapter.GetProviderName())
	}
}

func TestNewProviderAdapter_WithConfig(t *testing.T) {
	t.Parallel()
	cfg := &ProviderAdapterConfig{
		Timeout:         30 * time.Second,
		MaxRetries:      5,
		EnableStreaming: false,
	}

	adapter, err := NewProviderAdapter("test", "Test", "key", "url", cfg)
	if err != nil {
		t.Fatalf("NewProviderAdapter failed: %v", err)
	}
	if adapter.config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", adapter.config.Timeout)
	}
	if adapter.config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", adapter.config.MaxRetries)
	}
}

func TestProviderAdapter_Complete(t *testing.T) {
	t.Parallel()
	// Create mock server that returns OpenAI-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Hello back!"}}]}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("test", "Test Provider", "key", server.URL, nil)

	response, err := adapter.Complete(context.Background(), "gpt-4", "Hello", nil)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if response == "" {
		t.Error("expected non-empty response")
	}

	// Check metrics were recorded
	metrics := adapter.GetMetrics()
	if metrics.TotalRequests != 1 {
		t.Errorf("expected 1 total request, got %d", metrics.TotalRequests)
	}
	if metrics.SuccessfulRequests != 1 {
		t.Errorf("expected 1 successful request, got %d", metrics.SuccessfulRequests)
	}
}

func TestProviderAdapter_CompleteStream(t *testing.T) {
	t.Parallel()
	adapter, _ := NewProviderAdapter("test", "Test Provider", "key", "url", nil)

	stream, err := adapter.CompleteStream(context.Background(), "gpt-4", "Hello", nil)
	if err != nil {
		t.Fatalf("CompleteStream failed: %v", err)
	}

	// Read from stream
	var chunks []string
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestProviderAdapter_CompleteStream_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &ProviderAdapterConfig{
		EnableStreaming: false,
	}

	adapter, _ := NewProviderAdapter("test", "Test Provider", "key", "url", cfg)

	_, err := adapter.CompleteStream(context.Background(), "gpt-4", "Hello", nil)
	if err == nil {
		t.Error("expected error when streaming is disabled")
	}
}

func TestProviderAdapter_HealthCheck(t *testing.T) {
	t.Parallel()
	adapter, _ := NewProviderAdapter("test", "Test Provider", "key", "url", nil)

	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestProviderAdapter_HealthCheck_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &ProviderAdapterConfig{
		EnableHealthCheck: false,
	}

	adapter, _ := NewProviderAdapter("test", "Test Provider", "key", "url", cfg)

	err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck should succeed when disabled: %v", err)
	}
}

func TestProviderAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	adapter, _ := NewProviderAdapter("test", "Test Provider", "key", "url", nil)

	caps := adapter.GetCapabilities()
	if caps == nil {
		t.Fatal("GetCapabilities returned nil")
	}
	if !caps.SupportsStreaming {
		t.Error("expected SupportsStreaming to be true")
	}
	if !caps.SupportsFunctionCall {
		t.Error("expected SupportsFunctionCall to be true")
	}
	if caps.MaxContextLength != 128000 {
		t.Errorf("expected MaxContextLength 128000, got %d", caps.MaxContextLength)
	}
}

func TestProviderAdapter_GetMetrics(t *testing.T) {
	t.Parallel()
	// Create mock server that returns OpenAI-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Hello!"}}]}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("test", "Test Provider", "key", server.URL, nil)

	// Make some requests
	_, _ = adapter.Complete(context.Background(), "model", "prompt", nil)
	_, _ = adapter.Complete(context.Background(), "model", "prompt", nil)

	metrics := adapter.GetMetrics()
	if metrics.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", metrics.TotalRequests)
	}
	if metrics.SuccessfulRequests != 2 {
		t.Errorf("expected 2 successful requests, got %d", metrics.SuccessfulRequests)
	}
	if metrics.LastRequestAt.IsZero() {
		t.Error("expected LastRequestAt to be set")
	}
}

func TestProviderAdapterConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := &ProviderAdapterConfig{
		Timeout:             time.Minute,
		MaxRetries:          5,
		RetryDelay:          10 * time.Second,
		EnableStreaming:     true,
		EnableHealthCheck:   true,
		HealthCheckInterval: 30 * time.Second,
	}

	if cfg.Timeout != time.Minute {
		t.Error("Timeout mismatch")
	}
	if cfg.MaxRetries != 5 {
		t.Error("MaxRetries mismatch")
	}
	if cfg.RetryDelay != 10*time.Second {
		t.Error("RetryDelay mismatch")
	}
}

func TestProviderMetrics_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	metrics := &ProviderMetrics{
		TotalRequests:      100,
		SuccessfulRequests: 90,
		FailedRequests:     10,
		TotalLatencyMs:     10000,
		AvgLatencyMs:       100.0,
		LastRequestAt:      now,
		LastSuccessAt:      now,
		LastFailureAt:      now,
	}

	if metrics.TotalRequests != 100 {
		t.Error("TotalRequests mismatch")
	}
	if metrics.SuccessfulRequests != 90 {
		t.Error("SuccessfulRequests mismatch")
	}
	if metrics.FailedRequests != 10 {
		t.Error("FailedRequests mismatch")
	}
}

func TestProviderCapabilities_Fields(t *testing.T) {
	t.Parallel()
	caps := &ProviderCapabilities{
		SupportsStreaming:    true,
		SupportsFunctionCall: true,
		SupportsVision:       false,
		SupportsEmbeddings:   true,
		MaxContextLength:     128000,
		SupportedModels:      []string{"gpt-4", "gpt-3.5-turbo"},
	}

	if !caps.SupportsStreaming {
		t.Error("SupportsStreaming mismatch")
	}
	if len(caps.SupportedModels) != 2 {
		t.Error("SupportedModels length mismatch")
	}
}

func TestCompletionRequest_Fields(t *testing.T) {
	t.Parallel()
	req := CompletionRequest{
		Model:       "gpt-4",
		Prompt:      "Hello",
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        0.9,
		Stream:      true,
	}

	if req.Model != "gpt-4" {
		t.Error("Model mismatch")
	}
	if req.Prompt != "Hello" {
		t.Error("Prompt mismatch")
	}
	if req.MaxTokens != 1000 {
		t.Error("MaxTokens mismatch")
	}
	if req.Temperature != 0.7 {
		t.Error("Temperature mismatch")
	}
}

func TestCompletionResponse_Fields(t *testing.T) {
	t.Parallel()
	resp := CompletionResponse{
		Content: "Hello World",
	}

	if resp.Content != "Hello World" {
		t.Error("Content mismatch")
	}
}

func TestStreamChunk_Fields(t *testing.T) {
	t.Parallel()
	chunk := StreamChunk{
		Content: "chunk content",
		Error:   nil,
	}

	if chunk.Content != "chunk content" {
		t.Error("Content mismatch")
	}
	if chunk.Error != nil {
		t.Error("Error should be nil")
	}
}

func TestProviderCaps_Fields(t *testing.T) {
	t.Parallel()
	caps := &ProviderCaps{
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		SupportsEmbeddings:      true,
		MaxContextLength:        128000,
		SupportedModels:         []string{"model1", "model2"},
	}

	if !caps.SupportsStreaming {
		t.Error("SupportsStreaming mismatch")
	}
	if caps.MaxContextLength != 128000 {
		t.Error("MaxContextLength mismatch")
	}
}

func TestProviderConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := ProviderConfig{
		APIKey:  "sk-test-key",
		BaseURL: "https://api.example.com",
	}

	if cfg.APIKey != "sk-test-key" {
		t.Error("APIKey mismatch")
	}
	if cfg.BaseURL != "https://api.example.com" {
		t.Error("BaseURL mismatch")
	}
}

func TestGetIntOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		options  map[string]interface{}
		key      string
		defVal   int
		expected int
	}{
		{"int value", map[string]interface{}{"key": 42}, "key", 0, 42},
		{"int64 value", map[string]interface{}{"key": int64(42)}, "key", 0, 42},
		{"float64 value", map[string]interface{}{"key": 42.0}, "key", 0, 42},
		{"missing key", map[string]interface{}{}, "key", 10, 10},
		{"nil options", nil, "key", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getIntOption(tt.options, tt.key, tt.defVal)
			if result != tt.expected {
				t.Errorf("getIntOption() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestGetFloat64Option(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		options  map[string]interface{}
		key      string
		defVal   float64
		expected float64
	}{
		{"float64 value", map[string]interface{}{"key": 0.7}, "key", 0, 0.7},
		{"int value", map[string]interface{}{"key": 42}, "key", 0, 42.0},
		{"int64 value", map[string]interface{}{"key": int64(42)}, "key", 0, 42.0},
		{"missing key", map[string]interface{}{}, "key", 0.5, 0.5},
		{"nil options", nil, "key", 0.5, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getFloat64Option(tt.options, tt.key, tt.defVal)
			if result != tt.expected {
				t.Errorf("getFloat64Option() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestNewProviderAdapterRegistry(t *testing.T) {
	t.Parallel()
	registry := NewProviderAdapterRegistry()
	if registry == nil {
		t.Fatal("NewProviderAdapterRegistry returned nil")
	}
	if registry.adapters == nil {
		t.Error("adapters map not initialized")
	}
}

func TestProviderAdapterRegistry_Register(t *testing.T) {
	t.Parallel()
	registry := NewProviderAdapterRegistry()
	adapter, _ := NewProviderAdapter("test", "Test", "key", "url", nil)

	registry.Register(adapter)

	retrieved, ok := registry.Get("test")
	if !ok {
		t.Error("adapter not found after registration")
	}
	if retrieved.GetProviderID() != "test" {
		t.Error("retrieved adapter ID mismatch")
	}
}

func TestProviderAdapterRegistry_Get_NotFound(t *testing.T) {
	t.Parallel()
	registry := NewProviderAdapterRegistry()

	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("expected adapter not to be found")
	}
}

func TestProviderAdapterRegistry_GetAll(t *testing.T) {
	t.Parallel()
	registry := NewProviderAdapterRegistry()

	adapter1, _ := NewProviderAdapter("test1", "Test1", "key", "url", nil)
	adapter2, _ := NewProviderAdapter("test2", "Test2", "key", "url", nil)

	registry.Register(adapter1)
	registry.Register(adapter2)

	all := registry.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 adapters, got %d", len(all))
	}
}

func TestProviderAdapterRegistry_Remove(t *testing.T) {
	t.Parallel()
	registry := NewProviderAdapterRegistry()
	adapter, _ := NewProviderAdapter("test", "Test", "key", "url", nil)

	registry.Register(adapter)
	registry.Remove("test")

	_, ok := registry.Get("test")
	if ok {
		t.Error("adapter should have been removed")
	}
}

func TestProviderAdapterRegistry_GetHealthyAdapters(t *testing.T) {
	t.Parallel()
	registry := NewProviderAdapterRegistry()
	adapter, _ := NewProviderAdapter("test", "Test", "key", "url", nil)

	registry.Register(adapter)

	healthy := registry.GetHealthyAdapters(context.Background())
	if len(healthy) != 1 {
		t.Errorf("expected 1 healthy adapter, got %d", len(healthy))
	}
}

func TestProviderAdapter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	adapter, _ := NewProviderAdapter("test", "Test", "key", "url", nil)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = adapter.Complete(context.Background(), "model", "prompt", nil)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := adapter.GetMetrics()
	if metrics.TotalRequests != 10 {
		t.Errorf("expected 10 requests, got %d", metrics.TotalRequests)
	}
}

func TestProviderAdapter_RecordFailure(t *testing.T) {
	t.Parallel()
	adapter, _ := NewProviderAdapter("test", "Test", "key", "url", nil)

	// Manually record a failure
	adapter.recordFailure(100)

	metrics := adapter.GetMetrics()
	if metrics.FailedRequests != 1 {
		t.Errorf("expected 1 failed request, got %d", metrics.FailedRequests)
	}
	if metrics.LastFailureAt.IsZero() {
		t.Error("LastFailureAt should be set")
	}
}

func TestProviderAdapterRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	registry := NewProviderAdapterRegistry()

	done := make(chan bool, 20)

	// Concurrent registrations
	for i := 0; i < 10; i++ {
		go func(id int) {
			adapter, _ := NewProviderAdapter(
				"test-"+string(rune('A'+id)),
				"Test",
				"key",
				"url",
				nil,
			)
			registry.Register(adapter)
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			registry.GetAll()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

// ============================================================================
// extractContent Tests (via Complete with different response formats)
// ============================================================================

func TestProviderAdapter_Complete_AnthropicFormat(t *testing.T) {
	t.Parallel()
	// Create mock server that returns Anthropic-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"Hello from Anthropic!"}]}`))
	}))
	defer server.Close()

	// Provider name must match switch case: "claude" or "anthropic"
	adapter, _ := NewProviderAdapter("anthropic", "anthropic", "test-key", server.URL, nil)
	response, err := adapter.Complete(context.Background(), "claude-3", "Hello", nil)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if response != "Hello from Anthropic!" {
		t.Errorf("expected 'Hello from Anthropic!', got '%s'", response)
	}
}

func TestProviderAdapter_Complete_GeminiFormat(t *testing.T) {
	t.Parallel()
	// Create mock server that returns Gemini-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"Hello from Gemini!"}]}}]}`))
	}))
	defer server.Close()

	// Provider name must match switch case: "gemini" or "google"
	adapter, _ := NewProviderAdapter("gemini", "gemini", "test-key", server.URL, nil)
	response, err := adapter.Complete(context.Background(), "gemini-pro", "Hello", nil)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if response != "Hello from Gemini!" {
		t.Errorf("expected 'Hello from Gemini!', got '%s'", response)
	}
}

func TestProviderAdapter_Complete_OllamaFormat(t *testing.T) {
	t.Parallel()
	// Create mock server that returns Ollama-compatible response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"Hello from Ollama!"}`))
	}))
	defer server.Close()

	// Provider name must match switch case: "ollama"
	adapter, _ := NewProviderAdapter("ollama", "ollama", "test-key", server.URL, nil)
	response, err := adapter.Complete(context.Background(), "llama2", "Hello", nil)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if response != "Hello from Ollama!" {
		t.Errorf("expected 'Hello from Ollama!', got '%s'", response)
	}
}

func TestProviderAdapter_Complete_DeepSeekFormat(t *testing.T) {
	t.Parallel()
	// DeepSeek uses OpenAI-compatible format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Hello from DeepSeek!"}}]}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("deepseek", "DeepSeek", "key", server.URL, nil)
	response, err := adapter.Complete(context.Background(), "deepseek-chat", "Hello", nil)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if response != "Hello from DeepSeek!" {
		t.Errorf("expected 'Hello from DeepSeek!', got '%s'", response)
	}
}

func TestProviderAdapter_Complete_InvalidFormat(t *testing.T) {
	t.Parallel()
	// Create mock server that returns invalid response format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"invalid":"response"}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("test", "Test", "key", server.URL, nil)
	_, err := adapter.Complete(context.Background(), "model", "Hello", nil)
	if err == nil {
		t.Error("expected error for invalid response format")
	}
}

func TestProviderAdapter_Complete_EmptyChoices(t *testing.T) {
	t.Parallel()
	// Create mock server that returns empty choices array
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("openai", "OpenAI", "key", server.URL, nil)
	_, err := adapter.Complete(context.Background(), "gpt-4", "Hello", nil)
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestProviderAdapter_Complete_EmptyContent(t *testing.T) {
	t.Parallel()
	// Anthropic format with empty content array
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("anthropic", "Anthropic", "key", server.URL, nil)
	_, err := adapter.Complete(context.Background(), "claude", "Hello", nil)
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestProviderAdapter_Complete_MalformedJSON(t *testing.T) {
	t.Parallel()
	// Create mock server that returns malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("openai", "OpenAI", "key", server.URL, nil)
	_, err := adapter.Complete(context.Background(), "gpt-4", "Hello", nil)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestProviderAdapter_Complete_ServerError(t *testing.T) {
	t.Parallel()
	// Create mock server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	adapter, _ := NewProviderAdapter("openai", "OpenAI", "key", server.URL, nil)
	_, err := adapter.Complete(context.Background(), "gpt-4", "Hello", nil)
	if err == nil {
		t.Error("expected error for server error response")
	}

	// Check that failure was recorded
	metrics := adapter.GetMetrics()
	if metrics.FailedRequests != 1 {
		t.Errorf("expected 1 failed request, got %d", metrics.FailedRequests)
	}
}
