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

	"dev.helix.agent/internal/llm/discovery"
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
	discoverer  *discovery.Discoverer
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
	p := &SimpleOpenRouterProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		retryConfig: retryConfig,
	}
	p.discoverer = discovery.NewDiscoverer(discovery.ProviderConfig{
		ProviderName:   "openrouter",
		ModelsEndpoint: baseURL + "/models",
		ModelsDevID:    "openrouter",
		APIKey:         apiKey,
		ExtraHeaders: map[string]string{
			"HTTP-Referer": "helixagent",
		},
		FallbackModels: []string{
			// Premium models
			"anthropic/claude-3.5-sonnet",
			"openai/gpt-4o",
			"google/gemini-pro",
			"meta-llama/llama-3.1-405b-instruct",
			"mistralai/mistral-large",
			"meta-llama/llama-3.1-70b-instruct",
			"deepseek/deepseek-chat",
			// FREE models (OpenRouter Zen - no API key required, high quality)
			// Llama 4 free models
			"meta-llama/llama-4-maverick:free",
			"meta-llama/llama-4-scout:free",
			"meta-llama/llama-3.3-70b-instruct:free",
			// DeepSeek free models (excellent for coding)
			"deepseek/deepseek-chat-v3-0324:free",
			"deepseek/deepseek-r1:free",
			"deepseek/deepseek-r1-zero:free",
			"deepseek/deepseek-r1-distill-llama-70b:free",
			"deepseek/deepseek-r1-distill-qwen-32b:free",
			"deepseek/deepseek-r1-distill-qwen-14b:free",
			// Qwen free models
			"qwen/qwq-32b:free",
			"qwen/qwen2.5-vl-3b-instruct:free",
			// Google/Gemini free models
			"google/gemini-2.5-pro-exp-03-25:free",
			"google/gemini-2.0-flash-thinking-exp:free",
			"google/gemini-2.0-flash-exp:free",
			"google/gemma-3-27b-it:free",
			// NVIDIA free models
			"nvidia/llama-3.1-nemotron-ultra-253b-v1:free",
			// Microsoft free models
			"microsoft/phi-3-mini-128k-instruct:free",
			"microsoft/phi-3-medium-128k-instruct:free",
			// Other free models
			"mistralai/mistral-7b-instruct:free",
			"huggingfaceh4/zephyr-7b-beta:free",
			"openchat/openchat-7b:free",
			"nousresearch/nous-capybara-7b:free",
		},
	})
	return p
}

// Complete implements LLM provider interface
func (p *SimpleOpenRouterProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Use provided context or create timeout context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
	}

	// Convert to OpenRouter format (OpenAI-compatible)
	type OpenRouterTool struct {
		Type     string `json:"type"`
		Function struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description,omitempty"`
			Parameters  map[string]interface{} `json:"parameters,omitempty"`
		} `json:"function"`
	}

	type OpenRouterRequest struct {
		Model       string           `json:"model"`
		Messages    []models.Message `json:"messages"`
		MaxTokens   int              `json:"max_tokens,omitempty"`
		Temperature float64          `json:"temperature,omitempty"`
		Tools       []OpenRouterTool `json:"tools,omitempty"`
		ToolChoice  interface{}      `json:"tool_choice,omitempty"`
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

	// Convert tools if present
	if len(req.Tools) > 0 {
		orReq.Tools = make([]OpenRouterTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			if tool.Type == "function" {
				orTool := OpenRouterTool{
					Type: "function",
				}
				orTool.Function.Name = tool.Function.Name
				orTool.Function.Description = tool.Function.Description
				orTool.Function.Parameters = tool.Function.Parameters
				orReq.Tools = append(orReq.Tools, orTool)
			}
		}
		if req.ToolChoice != nil {
			orReq.ToolChoice = req.ToolChoice
		}
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
		httpReq.Header.Set("X-Title", "HelixAgent")

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
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: retryable error", resp.StatusCode)
			p.waitWithJitter(ctx, delay)
			delay = p.nextDelay(delay)
			continue
		}

		// Parse response
		type OpenRouterToolCall struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		}

		var orResp struct {
			ID      interface{} `json:"id"` // Can be string or number depending on provider
			Choices []struct {
				Message struct {
					Role      string               `json:"role"`
					Content   string               `json:"content"`
					ToolCalls []OpenRouterToolCall `json:"tool_calls,omitempty"`
				} `json:"message"`
				FinishReason string `json:"finish_reason,omitempty"`
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
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode OpenRouter response: %w", err)
		}
		_ = resp.Body.Close()

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

		// Convert tool calls if present
		var toolCalls []models.ToolCall
		if len(choice.Message.ToolCalls) > 0 {
			toolCalls = make([]models.ToolCall, 0, len(choice.Message.ToolCalls))
			for _, tc := range choice.Message.ToolCalls {
				toolCalls = append(toolCalls, models.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: models.ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}

		// Determine finish reason
		finishReason := choice.FinishReason
		if finishReason == "" {
			finishReason = "stop"
		}

		response := &models.LLMResponse{
			ID:           responseID,
			RequestID:    req.ID,
			ProviderID:   "openrouter",
			ProviderName: "OpenRouter",
			Content:      choice.Message.Content,
			Confidence:   0.85, // OpenRouter doesn't provide confidence
			TokensUsed:   0,
			ResponseTime: time.Now().UnixMilli(),
			FinishReason: finishReason,
			Metadata: map[string]any{
				"model":    orResp.Model,
				"provider": "openrouter",
			},
			Selected:       false,
			SelectionScore: 0.0,
			CreatedAt:      time.Now(),
			ToolCalls:      toolCalls,
		}

		if orResp.Usage != nil {
			response.TokensUsed = orResp.Usage.TotalTokens
			// Record the REAL per-direction split, not just the total.
			// These are the keys models.LLMResponse.TokenSplit reads to
			// build the OpenAI-compatible `usage` envelope; without
			// them the wire response reports 0 for both directions even
			// though OpenRouter measured and returned them here.
			response.Metadata["prompt_tokens"] = orResp.Usage.PromptTokens
			response.Metadata["completion_tokens"] = orResp.Usage.CompletionTokens
			response.Metadata["total_tokens"] = orResp.Usage.TotalTokens
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
//
//nolint:unused
func isAuthRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized // 401
}

// jitterFor returns the randomised jitter added to a retry delay: a duration in
// the half-open range [0, 10% of delay).
//
// Extracted from waitWithJitter so the 10% bound is directly and
// deterministically unit-testable. Asserting that bound through wall-clock
// timing is unreliable: OS scheduling delay is additive and, on a CPU-quota
// limited cgroup, a single throttle period can add ~100ms — an order of
// magnitude more than the jitter being measured, which both masks real
// regressions and causes spurious failures.
func jitterFor(delay time.Duration) time.Duration {
	// Using math/rand is acceptable for non-security jitter
	return time.Duration(rand.Float64() * 0.1 * float64(delay)) // #nosec G404 - jitter doesn't require cryptographic randomness
}

// waitWithJitter waits for the specified duration plus random jitter
func (p *SimpleOpenRouterProvider) waitWithJitter(ctx context.Context, delay time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(delay + jitterFor(delay)):
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
	httpReq.Header.Set("X-Title", "HelixAgent")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("OpenRouter stream request failed: %w", err)
	}

	// Check for HTTP errors before starting stream
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck
		_ = resp.Body.Close()            //nolint:errcheck
		close(ch)
		return nil, fmt.Errorf("OpenRouter API error: HTTP %d - %s", resp.StatusCode, string(body))
	}

	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()

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

					// Skip SSE comments (OpenRouter sends
					// ": OPENROUTER PROCESSING" keep-alive)
					if len(lineStr) > 0 && lineStr[0] == ':' {
						continue
					}

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

						// Check for mid-stream errors
						if bytes.Contains(
							[]byte(jsonData), []byte(`"error"`),
						) {
							var errResp struct {
								Error struct {
									Message string `json:"message"`
									Code    int    `json:"code"`
								} `json:"error"`
							}
							if json.Unmarshal(
								[]byte(jsonData), &errResp,
							) == nil && errResp.Error.Message != "" {
								ch <- &models.LLMResponse{
									ID: fmt.Sprintf(
										"stream-error-%d", chunkIndex,
									),
									RequestID:    req.ID,
									ProviderID:   "openrouter",
									ProviderName: "OpenRouter",
									Content:      contentBuilder.String(),
									FinishReason: "error",
									CreatedAt:    time.Now(),
									Metadata: map[string]any{
										"error":      errResp.Error.Message,
										"error_code": errResp.Error.Code,
									},
								}
								return
							}
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
	req.Header.Set("X-Title", "HelixAgent")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("OpenRouter health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
		SupportedModels: p.discoverer.DiscoverModels(),
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
