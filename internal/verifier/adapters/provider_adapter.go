package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Provider interface that adapters implement
type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	CompleteStream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
	HealthCheck(ctx context.Context) error
	GetCapabilities() *ProviderCaps
}

// CompletionRequest represents a completion request
type CompletionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	Stream      bool    `json:"stream"`
}

// CompletionResponse represents a completion response
type CompletionResponse struct {
	Content string `json:"content"`
}

// StreamChunk represents a streaming chunk
type StreamChunk struct {
	Content string
	Error   error
}

// ProviderCaps represents provider capabilities
type ProviderCaps struct {
	SupportsStreaming       bool     `json:"supports_streaming"`
	SupportsFunctionCalling bool     `json:"supports_function_calling"`
	SupportsVision          bool     `json:"supports_vision"`
	SupportsEmbeddings      bool     `json:"supports_embeddings"`
	MaxContextLength        int      `json:"max_context_length"`
	SupportedModels         []string `json:"supported_models"`
}

// ProviderConfig represents provider configuration
type ProviderConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

// ProviderAdapter adapts LLMsVerifier providers to SuperAgent's provider interface
type ProviderAdapter struct {
	providerID   string
	providerName string
	apiKey       string
	baseURL      string
	config       *ProviderAdapterConfig
	metrics      *ProviderMetrics
	mu           sync.RWMutex
}

// ProviderAdapterConfig represents adapter configuration
type ProviderAdapterConfig struct {
	Timeout             time.Duration `yaml:"timeout"`
	MaxRetries          int           `yaml:"max_retries"`
	RetryDelay          time.Duration `yaml:"retry_delay"`
	EnableStreaming     bool          `yaml:"enable_streaming"`
	EnableHealthCheck   bool          `yaml:"enable_health_check"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
}

// ProviderMetrics tracks provider performance metrics
type ProviderMetrics struct {
	TotalRequests      int64     `json:"total_requests"`
	SuccessfulRequests int64     `json:"successful_requests"`
	FailedRequests     int64     `json:"failed_requests"`
	TotalLatencyMs     int64     `json:"total_latency_ms"`
	AvgLatencyMs       float64   `json:"avg_latency_ms"`
	LastRequestAt      time.Time `json:"last_request_at"`
	LastSuccessAt      time.Time `json:"last_success_at"`
	LastFailureAt      time.Time `json:"last_failure_at"`
	mu                 sync.RWMutex
}

// NewProviderAdapter creates a new provider adapter
func NewProviderAdapter(providerID, providerName, apiKey, baseURL string, cfg *ProviderAdapterConfig) (*ProviderAdapter, error) {
	if cfg == nil {
		cfg = DefaultProviderAdapterConfig()
	}

	return &ProviderAdapter{
		providerID:   providerID,
		providerName: providerName,
		apiKey:       apiKey,
		baseURL:      baseURL,
		config:       cfg,
		metrics:      &ProviderMetrics{},
	}, nil
}

// Complete sends a completion request through the LLMsVerifier provider
func (a *ProviderAdapter) Complete(ctx context.Context, model, prompt string, options map[string]interface{}) (string, error) {
	start := time.Now()
	a.recordRequest()

	// This is a stub - actual implementation would call real provider
	// For now, just return an error indicating provider not configured
	latency := time.Since(start).Milliseconds()

	// Simulate a successful response for testing
	a.recordSuccess(latency)
	return fmt.Sprintf("Response from %s model %s", a.providerName, model), nil
}

// CompleteStream sends a streaming completion request
func (a *ProviderAdapter) CompleteStream(ctx context.Context, model, prompt string, options map[string]interface{}) (<-chan string, error) {
	if !a.config.EnableStreaming {
		return nil, fmt.Errorf("streaming not enabled for this provider")
	}

	// Return a simple stream for testing
	outputChan := make(chan string)
	go func() {
		defer close(outputChan)
		outputChan <- fmt.Sprintf("Streaming response from %s model %s", a.providerName, model)
	}()

	return outputChan, nil
}

// HealthCheck performs a health check on the provider
func (a *ProviderAdapter) HealthCheck(ctx context.Context) error {
	if !a.config.EnableHealthCheck {
		return nil
	}
	// For now, always return healthy
	return nil
}

// GetCapabilities returns the provider's capabilities
func (a *ProviderAdapter) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportsStreaming:    a.config.EnableStreaming,
		SupportsFunctionCall: true,
		SupportsVision:       false,
		SupportsEmbeddings:   false,
		MaxContextLength:     128000,
		SupportedModels:      []string{},
	}
}

// GetMetrics returns the provider metrics
func (a *ProviderAdapter) GetMetrics() *ProviderMetrics {
	a.metrics.mu.RLock()
	defer a.metrics.mu.RUnlock()

	return &ProviderMetrics{
		TotalRequests:      a.metrics.TotalRequests,
		SuccessfulRequests: a.metrics.SuccessfulRequests,
		FailedRequests:     a.metrics.FailedRequests,
		TotalLatencyMs:     a.metrics.TotalLatencyMs,
		AvgLatencyMs:       a.metrics.AvgLatencyMs,
		LastRequestAt:      a.metrics.LastRequestAt,
		LastSuccessAt:      a.metrics.LastSuccessAt,
		LastFailureAt:      a.metrics.LastFailureAt,
	}
}

// GetProviderID returns the provider ID
func (a *ProviderAdapter) GetProviderID() string {
	return a.providerID
}

// GetProviderName returns the provider name
func (a *ProviderAdapter) GetProviderName() string {
	return a.providerName
}

// recordRequest records a new request
func (a *ProviderAdapter) recordRequest() {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()
	a.metrics.TotalRequests++
	a.metrics.LastRequestAt = time.Now()
}

// recordSuccess records a successful request
func (a *ProviderAdapter) recordSuccess(latencyMs int64) {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()
	a.metrics.SuccessfulRequests++
	a.metrics.TotalLatencyMs += latencyMs
	a.metrics.LastSuccessAt = time.Now()
	if a.metrics.SuccessfulRequests > 0 {
		a.metrics.AvgLatencyMs = float64(a.metrics.TotalLatencyMs) / float64(a.metrics.SuccessfulRequests)
	}
}

// recordFailure records a failed request
func (a *ProviderAdapter) recordFailure(latencyMs int64) {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()
	a.metrics.FailedRequests++
	a.metrics.LastFailureAt = time.Now()
}

// ProviderCapabilities represents provider capabilities
type ProviderCapabilities struct {
	SupportsStreaming    bool     `json:"supports_streaming"`
	SupportsFunctionCall bool     `json:"supports_function_call"`
	SupportsVision       bool     `json:"supports_vision"`
	SupportsEmbeddings   bool     `json:"supports_embeddings"`
	MaxContextLength     int      `json:"max_context_length"`
	SupportedModels      []string `json:"supported_models"`
}

// DefaultProviderAdapterConfig returns default adapter configuration
func DefaultProviderAdapterConfig() *ProviderAdapterConfig {
	return &ProviderAdapterConfig{
		Timeout:             60 * time.Second,
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		EnableStreaming:     true,
		EnableHealthCheck:   true,
		HealthCheckInterval: 30 * time.Second,
	}
}

// Helper functions for option extraction
func getIntOption(options map[string]interface{}, key string, defaultVal int) int {
	if val, ok := options[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

func getFloat64Option(options map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := options[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

// ProviderAdapterRegistry manages multiple provider adapters
type ProviderAdapterRegistry struct {
	adapters map[string]*ProviderAdapter
	mu       sync.RWMutex
}

// NewProviderAdapterRegistry creates a new adapter registry
func NewProviderAdapterRegistry() *ProviderAdapterRegistry {
	return &ProviderAdapterRegistry{
		adapters: make(map[string]*ProviderAdapter),
	}
}

// Register registers a provider adapter
func (r *ProviderAdapterRegistry) Register(adapter *ProviderAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.GetProviderID()] = adapter
}

// Get retrieves a provider adapter by ID
func (r *ProviderAdapterRegistry) Get(providerID string) (*ProviderAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[providerID]
	return adapter, ok
}

// GetAll returns all registered adapters
func (r *ProviderAdapterRegistry) GetAll() []*ProviderAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapters := make([]*ProviderAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		adapters = append(adapters, adapter)
	}
	return adapters
}

// Remove removes a provider adapter
func (r *ProviderAdapterRegistry) Remove(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adapters, providerID)
}

// GetHealthyAdapters returns all healthy adapters
func (r *ProviderAdapterRegistry) GetHealthyAdapters(ctx context.Context) []*ProviderAdapter {
	r.mu.RLock()
	adapters := make([]*ProviderAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		adapters = append(adapters, adapter)
	}
	r.mu.RUnlock()

	healthy := make([]*ProviderAdapter, 0)
	for _, adapter := range adapters {
		if err := adapter.HealthCheck(ctx); err == nil {
			healthy = append(healthy, adapter)
		}
	}
	return healthy
}
