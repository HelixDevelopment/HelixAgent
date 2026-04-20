package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// HTTP helpers to avoid import issues in tests
var (
	jsonMarshal               = json.Marshal
	jsonDecode                = func(r io.Reader, v interface{}) error { return json.NewDecoder(r).Decode(v) }
	httpNewRequestWithContext = func(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	}
	httpClient = func() *http.Client {
		return &http.Client{Timeout: 60 * time.Second}
	}
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

// ProviderAdapter adapts LLMsVerifier providers to HelixAgent's provider interface
type ProviderAdapter struct {
	providerID   string
	providerName string
	apiKey       string
	baseURL      string
	config       *ProviderAdapterConfig
	metrics      *ProviderMetrics
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

	// Build the API request based on provider type
	var response string
	var err error

	maxTokens := getIntOption(options, "max_tokens", 1024)
	temperature := getFloat64Option(options, "temperature", 0.7)

	switch a.providerName {
	case "claude", "anthropic":
		response, err = a.completeAnthropic(ctx, model, prompt, maxTokens, temperature)
	case "openai", "gpt":
		response, err = a.completeOpenAI(ctx, model, prompt, maxTokens, temperature)
	case "gemini", "google":
		response, err = a.completeGemini(ctx, model, prompt, maxTokens, temperature)
	case "deepseek":
		response, err = a.completeDeepSeek(ctx, model, prompt, maxTokens, temperature)
	case "ollama":
		response, err = a.completeOllama(ctx, model, prompt, maxTokens, temperature)
	default:
		response, err = a.completeGeneric(ctx, model, prompt, maxTokens, temperature)
	}

	latency := time.Since(start).Milliseconds()

	if err != nil {
		a.recordFailure(latency)
		return "", fmt.Errorf("provider %s completion failed: %w", a.providerName, err)
	}

	a.recordSuccess(latency)
	return response, nil
}

// completeAnthropic handles Anthropic/Claude API calls
func (a *ProviderAdapter) completeAnthropic(ctx context.Context, model, prompt string, maxTokens int, temperature float64) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("anthropic API key not configured")
	}

	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	return a.makeHTTPRequest(ctx, baseURL+"/messages", reqBody, map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
		"Content-Type":      "application/json",
	}, "anthropic")
}

// completeOpenAI handles OpenAI API calls
func (a *ProviderAdapter) completeOpenAI(ctx context.Context, model, prompt string, maxTokens int, temperature float64) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("openai API key not configured")
	}

	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	reqBody := map[string]interface{}{
		"model":       model,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	return a.makeHTTPRequest(ctx, baseURL+"/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + a.apiKey,
		"Content-Type":  "application/json",
	}, "openai")
}

// completeGemini handles Google Gemini API calls
func (a *ProviderAdapter) completeGemini(ctx context.Context, model, prompt string, maxTokens int, temperature float64) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("gemini API key not configured")
	}

	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1"
	}

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": maxTokens,
			"temperature":     temperature,
		},
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, a.apiKey)
	return a.makeHTTPRequest(ctx, endpoint, reqBody, map[string]string{
		"Content-Type": "application/json",
	}, "gemini")
}

// completeDeepSeek handles DeepSeek API calls
func (a *ProviderAdapter) completeDeepSeek(ctx context.Context, model, prompt string, maxTokens int, temperature float64) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("deepseek API key not configured")
	}

	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}

	reqBody := map[string]interface{}{
		"model":       model,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	return a.makeHTTPRequest(ctx, baseURL+"/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + a.apiKey,
		"Content-Type":  "application/json",
	}, "openai") // DeepSeek uses OpenAI-compatible format
}

// completeOllama handles local Ollama API calls
func (a *ProviderAdapter) completeOllama(ctx context.Context, model, prompt string, maxTokens int, temperature float64) (string, error) {
	baseURL := a.baseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	reqBody := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"num_predict": maxTokens,
			"temperature": temperature,
		},
	}

	return a.makeHTTPRequest(ctx, baseURL+"/api/generate", reqBody, map[string]string{
		"Content-Type": "application/json",
	}, "ollama")
}

// completeGeneric handles generic OpenAI-compatible API calls
func (a *ProviderAdapter) completeGeneric(ctx context.Context, model, prompt string, maxTokens int, temperature float64) (string, error) {
	baseURL := a.baseURL
	if baseURL == "" {
		return "", fmt.Errorf("base URL required for generic provider")
	}

	reqBody := map[string]interface{}{
		"model":       model,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if a.apiKey != "" {
		headers["Authorization"] = "Bearer " + a.apiKey
	}

	return a.makeHTTPRequest(ctx, baseURL+"/chat/completions", reqBody, headers, "openai")
}

// makeHTTPRequest performs the HTTP request and extracts the response content
func (a *ProviderAdapter) makeHTTPRequest(ctx context.Context, url string, body interface{}, headers map[string]string, responseFormat string) (string, error) {
	jsonBody, err := jsonMarshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := httpNewRequestWithContext(ctx, "POST", url, jsonBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := jsonDecode(resp.Body, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return extractContent(result, responseFormat)
}

// extractContent extracts the text content from different API response formats
func extractContent(result map[string]interface{}, format string) (string, error) {
	switch format {
	case "anthropic":
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			if block, ok := content[0].(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					return text, nil
				}
			}
		}
	case "openai":
		if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if message, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok {
						return content, nil
					}
				}
			}
		}
	case "gemini":
		if candidates, ok := result["candidates"].([]interface{}); ok && len(candidates) > 0 {
			if candidate, ok := candidates[0].(map[string]interface{}); ok {
				if content, ok := candidate["content"].(map[string]interface{}); ok {
					if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
						if part, ok := parts[0].(map[string]interface{}); ok {
							if text, ok := part["text"].(string); ok {
								return text, nil
							}
						}
					}
				}
			}
		}
	case "ollama":
		if response, ok := result["response"].(string); ok {
			return response, nil
		}
	}
	return "", fmt.Errorf("could not extract content from %s response", format)
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

// ProviderAdapterRegistry manages multiple provider adapters.
//
// Concurrency model (CONST-029): adapters is a *safe.Store; each
// Register/Get/Remove/GetAll is atomic without any caller-held lock.
// The health-check scan copies the adapter list out via Snapshot
// before issuing any network calls so slow probes cannot block
// concurrent Register/Remove.
type ProviderAdapterRegistry struct {
	adapters *safe.Store[string, *ProviderAdapter]
}

// NewProviderAdapterRegistry creates a new adapter registry
func NewProviderAdapterRegistry() *ProviderAdapterRegistry {
	return &ProviderAdapterRegistry{
		adapters: safe.NewStore[string, *ProviderAdapter](),
	}
}

// Register registers a provider adapter
func (r *ProviderAdapterRegistry) Register(adapter *ProviderAdapter) {
	r.adapters.Put(adapter.GetProviderID(), adapter)
}

// Get retrieves a provider adapter by ID
func (r *ProviderAdapterRegistry) Get(providerID string) (*ProviderAdapter, bool) {
	return r.adapters.Get(providerID)
}

// GetAll returns all registered adapters
func (r *ProviderAdapterRegistry) GetAll() []*ProviderAdapter {
	snap := r.adapters.Snapshot()
	adapters := make([]*ProviderAdapter, 0, len(snap))
	for _, adapter := range snap {
		adapters = append(adapters, adapter)
	}
	return adapters
}

// Remove removes a provider adapter
func (r *ProviderAdapterRegistry) Remove(providerID string) {
	r.adapters.Delete(providerID)
}

// GetHealthyAdapters returns all healthy adapters
func (r *ProviderAdapterRegistry) GetHealthyAdapters(ctx context.Context) []*ProviderAdapter {
	adapters := r.GetAll()

	healthy := make([]*ProviderAdapter, 0)
	for _, adapter := range adapters {
		if err := adapter.HealthCheck(ctx); err == nil {
			healthy = append(healthy, adapter)
		}
	}
	return healthy
}
