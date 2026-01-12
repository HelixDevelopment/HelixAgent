package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"dev.helix.agent/internal/models"
)

const (
	defaultBaseURL = "https://openrouter.ai/api/v1"
)

// RetryConfig defines retry behavior for API calls
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultRetryConfig returns sensible defaults for OpenRouter API retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// SimpleOpenRouterProvider implements LLM provider interface for OpenRouter
type SimpleOpenRouterProvider struct {
	apiKey      string
	baseURL     string
	client      *http.Client
	retryConfig RetryConfig
}

// NewSimpleOpenRouterProvider creates a new OpenRouter provider
func NewSimpleOpenRouterProvider(apiKey string) *SimpleOpenRouterProvider {
	return NewSimpleOpenRouterProviderWithRetry(apiKey, defaultBaseURL, DefaultRetryConfig())
}

// NewSimpleOpenRouterProviderWithBaseURL creates a new OpenRouter provider with custom base URL
func NewSimpleOpenRouterProviderWithBaseURL(apiKey, baseURL string) *SimpleOpenRouterProvider {
	return NewSimpleOpenRouterProviderWithRetry(apiKey, baseURL, DefaultRetryConfig())
}

// NewSimpleOpenRouterProviderWithRetry creates a new OpenRouter provider with custom retry config
func NewSimpleOpenRouterProviderWithRetry(apiKey, baseURL string, retryConfig RetryConfig) *SimpleOpenRouterProvider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &SimpleOpenRouterProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		retryConfig: retryConfig,
	}
}

// Complete implements LLM provider interface
func (p *SimpleOpenRouterProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Use provided context or create timeout context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
	}

	// Convert to OpenRouter format (no prompt field - convert to system message for compatibility)
	type OpenRouterRequest struct {
		Model       string           `json:"model"`
		Messages    []models.Message `json:"messages"`
		MaxTokens   int              `json:"max_tokens,omitempty"`
		Temperature float64          `json:"temperature,omitempty"`
	}

	// Cap max_tokens to reasonable limit (varies by model, using 16384 as safe max)
	maxTokens := req.ModelParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096 // Default
	} else if maxTokens > 16384 {
		maxTokens = 16384 // Safe max for most OpenRouter models
	}

	// Convert prompt to system message if provided (some providers don't support prompt field)
	messages := req.Messages
	if req.Prompt != "" {
		systemMsg := models.Message{
			Role:    "system",
			Content: req.Prompt,
		}
		messages = append([]models.Message{systemMsg}, messages...)
	}

	orReq := OpenRouterRequest{
		Model:       req.ModelParams.Model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.ModelParams.Temperature,
	}

	// Make request
	jsonData, err := json.Marshal(orReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenRouter request: %w", err)
	}

	// Retry loop with exponential backoff
	var lastErr error
	delay := p.retryConfig.InitialDelay

	for attempt := 0; attempt <= p.retryConfig.MaxRetries; attempt++ {
		// Check context before making request
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenRouter request: %w", err)
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("HTTP-Referer", "helixagent")

		// Make request
		resp, err := p.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("OpenRouter API request failed: %w", err)
			if attempt < p.retryConfig.MaxRetries {
				p.waitWithJitter(ctx, delay)
				delay = p.nextDelay(delay)
				continue
			}
			return nil, lastErr
		}

		// Check for retryable status codes
		if isRetryableStatus(resp.StatusCode) && attempt < p.retryConfig.MaxRetries {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: retryable error", resp.StatusCode)
			p.waitWithJitter(ctx, delay)
			delay = p.nextDelay(delay)
			continue
		}

		// Parse response
		var orResp struct {
			ID      interface{} `json:"id"` // Can be string or number depending on provider
			Choices []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Usage   *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage,omitempty"`
			Error *struct {
				Message string      `json:"message"`
				Type    string      `json:"type"`
				Code    interface{} `json:"code,omitempty"` // Dynamically handles int or string
			} `json:"error,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode OpenRouter response: %w", err)
		}
		resp.Body.Close()

		if orResp.Error != nil {
			return nil, fmt.Errorf("OpenRouter API error: %s", orResp.Error.Message)
		}

		// Convert to internal response format
		if len(orResp.Choices) == 0 {
			return nil, fmt.Errorf("no choices in OpenRouter response")
		}

		// Convert ID to string (some providers return number, others string)
		responseID := ""
		if orResp.ID != nil {
			responseID = fmt.Sprintf("%v", orResp.ID)
		}

		choice := orResp.Choices[0]
		response := &models.LLMResponse{
			ID:           responseID,
			RequestID:    req.ID,
			ProviderID:   "openrouter",
			ProviderName: "OpenRouter",
			Content:      choice.Message.Content,
			Confidence:   0.85, // OpenRouter doesn't provide confidence
			TokensUsed:   0,
			ResponseTime: time.Now().UnixMilli(),
			FinishReason: "stop",
			Metadata: map[string]any{
				"model":    orResp.Model,
				"provider": "openrouter",
			},
			Selected:       false,
			SelectionScore: 0.0,
			CreatedAt:      time.Now(),
		}

		if orResp.Usage != nil {
			response.TokensUsed = orResp.Usage.TotalTokens
		}

		return response, nil
	}

	return nil, fmt.Errorf("all %d retry attempts failed: %w", p.retryConfig.MaxRetries+1, lastErr)
}

// isRetryableStatus returns true for HTTP status codes that warrant a retry
func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429 - Rate limited
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// isAuthRetryableStatus returns true for auth errors that may be transient
// (e.g., token validation delays, temporary auth service issues)
func isAuthRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized // 401
}

// waitWithJitter waits for the specified duration plus random jitter
func (p *SimpleOpenRouterProvider) waitWithJitter(ctx context.Context, delay time.Duration) {
	// Add 10% jitter
	jitter := time.Duration(rand.Float64() * 0.1 * float64(delay))
	select {
	case <-ctx.Done():
	case <-time.After(delay + jitter):
	}
}

// nextDelay calculates the next delay using exponential backoff
func (p *SimpleOpenRouterProvider) nextDelay(currentDelay time.Duration) time.Duration {
	nextDelay := time.Duration(float64(currentDelay) * p.retryConfig.Multiplier)
	if nextDelay > p.retryConfig.MaxDelay {
		nextDelay = p.retryConfig.MaxDelay
	}
	return nextDelay
}

// CompleteStream implements streaming completion using Server-Sent Events
func (p *SimpleOpenRouterProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 100)

	// Create streaming request (no prompt field - convert to system message for compatibility)
	type OpenRouterStreamRequest struct {
		Model       string           `json:"model"`
		Messages    []models.Message `json:"messages"`
		MaxTokens   int              `json:"max_tokens,omitempty"`
		Temperature float64          `json:"temperature,omitempty"`
		Stream      bool             `json:"stream"`
	}

	// Cap max_tokens to reasonable limit
	maxTokens := req.ModelParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	} else if maxTokens > 16384 {
		maxTokens = 16384
	}

	// Convert prompt to system message if provided (some providers don't support prompt field)
	messages := req.Messages
	if req.Prompt != "" {
		systemMsg := models.Message{
			Role:    "system",
			Content: req.Prompt,
		}
		messages = append([]models.Message{systemMsg}, messages...)
	}

	orReq := OpenRouterStreamRequest{
		Model:       req.ModelParams.Model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.ModelParams.Temperature,
		Stream:      true,
	}

	jsonData, err := json.Marshal(orReq)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to marshal OpenRouter stream request: %w", err)
	}

	// Make HTTP request BEFORE starting goroutine to check for errors early
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to create OpenRouter stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("HTTP-Referer", "helixagent")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("OpenRouter stream request failed: %w", err)
	}

	// Check for HTTP errors before starting stream
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		close(ch)
		return nil, fmt.Errorf("OpenRouter API error: HTTP %d - %s", resp.StatusCode, string(body))
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Read SSE stream
		reader := resp.Body
		buf := make([]byte, 4096)
		var contentBuilder bytes.Buffer
		chunkIndex := 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			n, err := reader.Read(buf)
			if err != nil {
				if err.Error() != "EOF" {
					// Send final chunk with accumulated content
					if contentBuilder.Len() > 0 {
						ch <- &models.LLMResponse{
							ID:           fmt.Sprintf("stream-%d", chunkIndex),
							RequestID:    req.ID,
							ProviderID:   "openrouter",
							ProviderName: "OpenRouter",
							Content:      contentBuilder.String(),
							FinishReason: "stop",
							CreatedAt:    time.Now(),
						}
					}
				}
				return
			}

			if n > 0 {
				data := string(buf[:n])
				// Parse SSE data lines
				lines := bytes.Split([]byte(data), []byte("\n"))
				for _, line := range lines {
					lineStr := string(line)
					if len(lineStr) > 6 && lineStr[:6] == "data: " {
						jsonData := lineStr[6:]
						if jsonData == "[DONE]" {
							// Stream complete
							ch <- &models.LLMResponse{
								ID:           fmt.Sprintf("stream-%d", chunkIndex),
								RequestID:    req.ID,
								ProviderID:   "openrouter",
								ProviderName: "OpenRouter",
								Content:      contentBuilder.String(),
								FinishReason: "stop",
								CreatedAt:    time.Now(),
							}
							return
						}

						var chunk struct {
							Choices []struct {
								Delta struct {
									Content string `json:"content"`
								} `json:"delta"`
								FinishReason string `json:"finish_reason"`
							} `json:"choices"`
						}

						if err := json.Unmarshal([]byte(jsonData), &chunk); err == nil {
							if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
								content := chunk.Choices[0].Delta.Content
								contentBuilder.WriteString(content)
								chunkIndex++

								// Send chunk
								ch <- &models.LLMResponse{
									ID:           fmt.Sprintf("stream-%d", chunkIndex),
									RequestID:    req.ID,
									ProviderID:   "openrouter",
									ProviderName: "OpenRouter",
									Content:      content,
									FinishReason: "",
									CreatedAt:    time.Now(),
									Metadata: map[string]any{
										"is_chunk": true,
										"index":    chunkIndex,
									},
								}
							}
						}
					}
				}
			}
		}
	}()

	return ch, nil
}

// HealthCheck implements provider health monitoring
func (p *SimpleOpenRouterProvider) HealthCheck() error {
	if p.apiKey == "" {
		return fmt.Errorf("OpenRouter API key is required for health check")
	}

	// Create a context with timeout for health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the /models endpoint as a lightweight health check
	// This verifies both connectivity and API key validity
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("HTTP-Referer", "helixagent")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("OpenRouter health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("OpenRouter API key is invalid or expired")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenRouter health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetCapabilities returns provider capabilities
func (p *SimpleOpenRouterProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"openrouter/anthropic/claude-3.5-sonnet",
			"openrouter/openai/gpt-4o",
			"openrouter/google/gemini-pro",
			"openrouter/meta-llama/llama-3.1-405b",
			"openrouter/mistralai/mistral-large",
			"openrouter/meta-llama/llama-3.1-70b",
			"openrouter/perplexity-70b",
			"openwizard/cohere-2",
			"openwizard/palm-2-chat-bison",
			"openwizard/gemma-2-7b",
			"openwizard/gemma-1.5-pro",
			"openwizard/dbrx-instruct",
			"openwizard/dbrx-small",
			"openwizard/llava-2",
			"openwizard/code-llama-2",
			"openwizard/qwen-1.8b-chat",
			"openwizard/qwen-1.8b-code",
			"openwizard/zephyr-7b-alpha",
			"openrouter/deepseek-v2-lite",
			"openrouter/deepseek-coder-v2-lite",
			"openwizard/nous-hermes-2",
			"openwizard/nous-hermes-2-predetermined",
			"openwizard/seamless-emb",
			"openwizard/command-r",
			"openwizard/vicuna-1.3",
			"openwizard/unitree-2",
			"openwizard/vicuna-2.0",
			"openwizard/yi-34b",
			"openwizard/yi-6b",
			"openwizard/yi-34b-200k",
			"openrouter/segway-3.5b",
			"openrouter/segway-3.5b-16k",
			"openwizard/gpt-4o",
			"openwizard/gpt-4-turbo",
			"openwizard/gpt-4-32k",
			"openrouter/gpt-4-vision-preview",
			"openwizard/o1-preview",
			"openwizard/grande-3",
			"openwizard/grande-3-instruct",
			"openwizard/yi-6b",
			"openwizard/mistral-7b",
			"openwizard/mixtral-8x7b",
			"openwizard/mixtral-8x22b",
			"openwizard/pixtral-12b",
			"openwizard/starcoder2-15b",
			"openwizard/starcoder2-13b",
		},
		SupportedFeatures: []string{
			"text_completion",
			"chat",
			"multi_model_routing",
			"cost_optimization",
		},
		SupportedRequestTypes: []string{
			"text_completion",
			"chat",
		},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsSearch:          true,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     true,
		Limits: models.ModelLimits{
			MaxTokens:             200000,
			MaxInputLength:        200000,
			MaxOutputLength:       8192,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"provider":      "OpenRouter",
			"api_version":   "v1",
			"routing":       "basic",
			"multi_tenancy": "true",
		},
	}
}

// ValidateConfig validates provider configuration
func (p *SimpleOpenRouterProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if p.apiKey == "" {
		return false, []string{"api_key is required"}
	}
	return true, nil
}
