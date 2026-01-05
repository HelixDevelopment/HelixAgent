package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/superagent/superagent/internal/models"
)

const (
	GeminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
	GeminiModel  = "gemini-pro"
)

type GeminiProvider struct {
	apiKey      string
	baseURL     string
	healthURL   string
	model       string
	httpClient  *http.Client
	retryConfig RetryConfig
}

// RetryConfig defines retry behavior for API calls
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

type GeminiRequest struct {
	Contents         []GeminiContent        `json:"contents"`
	GenerationConfig GeminiGenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings   []GeminiSafetySetting  `json:"safetySettings,omitempty"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type GeminiPart struct {
	Text         string            `json:"text,omitempty"`
	InlineData   *GeminiInlineData `json:"inlineData,omitempty"`
	FunctionCall map[string]any    `json:"functionCall,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GeminiGenerationConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type GeminiResponse struct {
	Candidates     []GeminiCandidate     `json:"candidates"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata  `json:"usageMetadata,omitempty"`
}

type GeminiCandidate struct {
	Content       GeminiContent        `json:"content"`
	FinishReason  string               `json:"finishReason"`
	Index         int                  `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

type GeminiPromptFeedback struct {
	BlockReason   string               `json:"blockReason"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
	Blocked     bool   `json:"blocked"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type GeminiStreamResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates,omitempty"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// DefaultRetryConfig returns sensible defaults for Gemini API retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func NewGeminiProvider(apiKey, baseURL, model string) *GeminiProvider {
	return NewGeminiProviderWithRetry(apiKey, baseURL, model, DefaultRetryConfig())
}

func NewGeminiProviderWithRetry(apiKey, baseURL, model string, retryConfig RetryConfig) *GeminiProvider {
	if baseURL == "" {
		baseURL = GeminiAPIURL
	}
	if model == "" {
		model = GeminiModel
	}

	return &GeminiProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		retryConfig: retryConfig,
	}
}

func (p *GeminiProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to Gemini format
	geminiReq := p.convertRequest(req)

	// Make API call
	resp, err := p.makeAPICall(ctx, geminiReq)
	if err != nil {
		return nil, fmt.Errorf("Gemini API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API error: %d - %s", resp.StatusCode, string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	// Convert back to internal format
	return p.convertResponse(req, &geminiResp, startTime), nil
}

func (p *GeminiProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to Gemini format
	geminiReq := p.convertRequest(req)

	// Add streaming parameter to generation config
	if geminiReq.GenerationConfig.MaxOutputTokens == 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = 2048
	}

	// Make streaming API call
	resp, err := p.makeAPICall(ctx, geminiReq)
	if err != nil {
		return nil, fmt.Errorf("Gemini streaming API call failed: %w", err)
	}

	// Create response channel
	ch := make(chan *models.LLMResponse)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		reader := bufio.NewReader(resp.Body)
		var fullContent string

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				// Send error response and exit
				errorResp := &models.LLMResponse{
					ID:             "stream-error-" + req.ID,
					RequestID:      req.ID,
					ProviderID:     "gemini",
					ProviderName:   "Gemini",
					Content:        "",
					Confidence:     0.0,
					TokensUsed:     0,
					ResponseTime:   time.Since(startTime).Milliseconds(),
					FinishReason:   "error",
					Selected:       false,
					SelectionScore: 0.0,
					CreatedAt:      time.Now(),
				}
				ch <- errorResp
				return
			}

			// Skip empty lines and "data: " prefix
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			line = bytes.TrimPrefix(line, []byte("data: "))

			// Skip "[DONE]" marker
			if bytes.Equal(line, []byte("[DONE]")) {
				break
			}

			// Parse JSON
			var streamResp GeminiStreamResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue // Skip malformed JSON
			}

			// Extract content from candidates
			if len(streamResp.Candidates) > 0 {
				candidate := streamResp.Candidates[0]
				if len(candidate.Content.Parts) > 0 {
					for _, part := range candidate.Content.Parts {
						if part.Text != "" {
							fullContent += part.Text

							// Send chunk response
							chunkResp := &models.LLMResponse{
								ID:             "gemini-stream-" + req.ID,
								RequestID:      req.ID,
								ProviderID:     "gemini",
								ProviderName:   "Gemini",
								Content:        part.Text,
								Confidence:     0.85, // High confidence for Gemini
								TokensUsed:     1,    // Estimated
								ResponseTime:   time.Since(startTime).Milliseconds(),
								FinishReason:   "",
								Selected:       false,
								SelectionScore: 0.0,
								CreatedAt:      time.Now(),
							}
							ch <- chunkResp
						}
					}
				}

				// Check if stream is finished
				if candidate.FinishReason != "" {
					break
				}
			}
		}

		// Send final response
		finalResp := &models.LLMResponse{
			ID:             "gemini-final-" + req.ID,
			RequestID:      req.ID,
			ProviderID:     "gemini",
			ProviderName:   "Gemini",
			Content:        "",
			Confidence:     0.85,
			TokensUsed:     len(fullContent) / 4, // Rough estimate
			ResponseTime:   time.Since(startTime).Milliseconds(),
			FinishReason:   "stop",
			Selected:       false,
			SelectionScore: 0.0,
			CreatedAt:      time.Now(),
		}
		ch <- finalResp
	}()

	return ch, nil
}

func (p *GeminiProvider) convertRequest(req *models.LLMRequest) GeminiRequest {
	// Convert messages to Gemini content format
	contents := make([]GeminiContent, 0, len(req.Messages)+1)

	// Add system prompt as user message if present
	if req.Prompt != "" {
		contents = append(contents, GeminiContent{
			Parts: []GeminiPart{
				{Text: req.Prompt},
			},
			Role: "user",
		})
	}

	// Add conversation messages
	for _, msg := range req.Messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}

		contents = append(contents, GeminiContent{
			Parts: []GeminiPart{
				{Text: msg.Content},
			},
			Role: role,
		})
	}

	// Cap max_tokens to Gemini's limit (8192 for most models)
	maxTokens := req.ModelParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096 // Default
	} else if maxTokens > 8192 {
		maxTokens = 8192 // Safe max for Gemini models
	}

	return GeminiRequest{
		Contents: contents,
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     req.ModelParams.Temperature,
			TopP:            req.ModelParams.TopP,
			MaxOutputTokens: maxTokens,
			StopSequences:   req.ModelParams.StopSequences,
		},
		SafetySettings: []GeminiSafetySetting{
			{
				Category:  "HARM_CATEGORY_HARASSMENT",
				Threshold: "BLOCK_NONE",
			},
			{
				Category:  "HARM_CATEGORY_HATE_SPEECH",
				Threshold: "BLOCK_NONE",
			},
			{
				Category:  "HARM_CATEGORY_SEXUALLY_EXPLICIT",
				Threshold: "BLOCK_NONE",
			},
			{
				Category:  "HARM_CATEGORY_DANGEROUS_CONTENT",
				Threshold: "BLOCK_NONE",
			},
		},
	}
}

func (p *GeminiProvider) convertResponse(req *models.LLMRequest, geminiResp *GeminiResponse, startTime time.Time) *models.LLMResponse {
	var content string
	var finishReason string
	var tokensUsed int

	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		if len(candidate.Content.Parts) > 0 {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					content += part.Text
				}
			}
		}
		finishReason = candidate.FinishReason
	}

	if geminiResp.UsageMetadata != nil {
		tokensUsed = geminiResp.UsageMetadata.TotalTokenCount
	}

	// Calculate confidence based on finish reason and response quality
	confidence := p.calculateConfidence(content, finishReason)

	return &models.LLMResponse{
		ID:           "gemini-" + req.ID,
		RequestID:    req.ID,
		ProviderID:   "gemini",
		ProviderName: "Gemini",
		Content:      content,
		Confidence:   confidence,
		TokensUsed:   tokensUsed,
		ResponseTime: time.Since(startTime).Milliseconds(),
		FinishReason: finishReason,
		Metadata: map[string]any{
			"model": p.model,
		},
		Selected:       false,
		SelectionScore: 0.0,
		CreatedAt:      time.Now(),
	}
}

func (p *GeminiProvider) calculateConfidence(content, finishReason string) float64 {
	confidence := 0.85 // High base confidence for Gemini

	// Adjust based on finish reason
	switch finishReason {
	case "STOP":
		confidence += 0.1
	case "MAX_TOKENS":
		confidence -= 0.1
	case "SAFETY":
		confidence -= 0.3
	case "RECITATION":
		confidence -= 0.2
	}

	// Adjust based on content length
	if len(content) > 100 {
		confidence += 0.03
	}
	if len(content) > 500 {
		confidence += 0.03
	}

	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

func (p *GeminiProvider) makeAPICall(ctx context.Context, req GeminiRequest) (*http.Response, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL with model
	url := fmt.Sprintf(p.baseURL, p.model)

	var lastErr error
	delay := p.retryConfig.InitialDelay

	for attempt := 0; attempt <= p.retryConfig.MaxRetries; attempt++ {
		// Check context before making request
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Create HTTP request (fresh for each attempt)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-goog-api-key", p.apiKey)
		httpReq.Header.Set("User-Agent", "SuperAgent/1.0")

		// Make request
		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			// Retry on network errors
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

		return resp, nil
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

// waitWithJitter waits for the specified duration plus random jitter
func (p *GeminiProvider) waitWithJitter(ctx context.Context, delay time.Duration) {
	// Add 10% jitter
	jitter := time.Duration(rand.Float64() * 0.1 * float64(delay))
	select {
	case <-ctx.Done():
	case <-time.After(delay + jitter):
	}
}

// nextDelay calculates the next delay using exponential backoff
func (p *GeminiProvider) nextDelay(currentDelay time.Duration) time.Duration {
	nextDelay := time.Duration(float64(currentDelay) * p.retryConfig.Multiplier)
	if nextDelay > p.retryConfig.MaxDelay {
		nextDelay = p.retryConfig.MaxDelay
	}
	return nextDelay
}

// GetCapabilities returns the capabilities of the Gemini provider
func (p *GeminiProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"gemini-pro",
			"gemini-pro-vision",
			"gemini-1.5-pro",
			"gemini-1.5-flash",
		},
		SupportedFeatures: []string{
			"text_completion",
			"chat",
			"function_calling",
			"streaming",
			"vision",
		},
		SupportedRequestTypes: []string{
			"text_completion",
			"chat",
		},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsSearch:          false,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     false,
		Limits: models.ModelLimits{
			MaxTokens:             32768,
			MaxInputLength:        32768,
			MaxOutputLength:       8192,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"provider":     "Google",
			"model_family": "Gemini",
			"api_version":  "v1beta",
		},
	}
}

// ValidateConfig validates the provider configuration
func (p *GeminiProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	if p.apiKey == "" {
		errors = append(errors, "API key is required")
	}

	if p.baseURL == "" {
		errors = append(errors, "base URL is required")
	}

	if p.model == "" {
		errors = append(errors, "model is required")
	}

	return len(errors) == 0, errors
}

// HealthCheck implements health checking for the Gemini provider
func (p *GeminiProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use healthURL if set, otherwise use default
	healthURL := p.healthURL
	if healthURL == "" {
		healthURL = "https://generativelanguage.googleapis.com/v1beta/models"
	}

	// Simple health check - try to get models list
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL+"?key="+p.apiKey, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}
