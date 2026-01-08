package claude

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

	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/models"
)

const (
	ClaudeAPIURL = "https://api.anthropic.com/v1/messages"
	ClaudeModel  = "claude-3-sonnet-20240229"
)

// AuthType represents the type of authentication used
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth  AuthType = "oauth"
)

type ClaudeProvider struct {
	apiKey           string
	baseURL          string
	model            string
	httpClient       *http.Client
	retryConfig      RetryConfig
	authType         AuthType
	oauthCredReader  *oauth_credentials.OAuthCredentialReader
}

// RetryConfig defines retry behavior for API calls
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

type ClaudeRequest struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Temperature   float64         `json:"temperature,omitempty"`
	TopP          float64         `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Messages      []ClaudeMessage `json:"messages"`
	System        string          `json:"system,omitempty"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      []ClaudeContent `json:"content"`
	Model        string          `json:"model"`
	StopReason   *string         `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        ClaudeUsage     `json:"usage"`
}

type ClaudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClaudeStreamResponse struct {
	Type    string          `json:"type"`
	Message *ClaudeResponse `json:"message,omitempty"`
	Delta   *ClaudeDelta    `json:"delta,omitempty"`
	Usage   *ClaudeUsage    `json:"usage,omitempty"`
}

type ClaudeDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// DefaultRetryConfig returns sensible defaults for Claude API retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func NewClaudeProvider(apiKey, baseURL, model string) *ClaudeProvider {
	return NewClaudeProviderWithRetry(apiKey, baseURL, model, DefaultRetryConfig())
}

func NewClaudeProviderWithRetry(apiKey, baseURL, model string, retryConfig RetryConfig) *ClaudeProvider {
	if baseURL == "" {
		baseURL = ClaudeAPIURL
	}
	if model == "" {
		model = ClaudeModel
	}

	return &ClaudeProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		retryConfig: retryConfig,
		authType:    AuthTypeAPIKey,
	}
}

// NewClaudeProviderWithOAuth creates a new Claude provider using OAuth credentials from Claude Code CLI
func NewClaudeProviderWithOAuth(baseURL, model string) (*ClaudeProvider, error) {
	return NewClaudeProviderWithOAuthAndRetry(baseURL, model, DefaultRetryConfig())
}

// NewClaudeProviderWithOAuthAndRetry creates a new Claude provider using OAuth credentials with custom retry config
func NewClaudeProviderWithOAuthAndRetry(baseURL, model string, retryConfig RetryConfig) (*ClaudeProvider, error) {
	if baseURL == "" {
		baseURL = ClaudeAPIURL
	}
	if model == "" {
		model = ClaudeModel
	}

	credReader := oauth_credentials.GetGlobalReader()

	// Verify credentials are available
	if !credReader.HasValidClaudeCredentials() {
		return nil, fmt.Errorf("no valid Claude OAuth credentials available: ensure you are logged in via Claude Code CLI")
	}

	return &ClaudeProvider{
		apiKey:          "", // Will use OAuth token instead
		baseURL:         baseURL,
		model:           model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		retryConfig:     retryConfig,
		authType:        AuthTypeOAuth,
		oauthCredReader: credReader,
	}, nil
}

// NewClaudeProviderAuto creates a Claude provider, automatically choosing OAuth if enabled and available
func NewClaudeProviderAuto(apiKey, baseURL, model string) (*ClaudeProvider, error) {
	// Check if OAuth is enabled and credentials are available
	if oauth_credentials.IsClaudeOAuthEnabled() {
		credReader := oauth_credentials.GetGlobalReader()
		if credReader.HasValidClaudeCredentials() {
			return NewClaudeProviderWithOAuth(baseURL, model)
		}
	}

	// Fall back to API key authentication
	if apiKey == "" {
		return nil, fmt.Errorf("no API key provided and OAuth credentials not available")
	}
	return NewClaudeProvider(apiKey, baseURL, model), nil
}

// GetAuthType returns the authentication type being used
func (p *ClaudeProvider) GetAuthType() AuthType {
	return p.authType
}

// getAuthHeader returns the appropriate authorization header based on auth type
func (p *ClaudeProvider) getAuthHeader() (string, string, error) {
	switch p.authType {
	case AuthTypeOAuth:
		if p.oauthCredReader == nil {
			return "", "", fmt.Errorf("OAuth credential reader not initialized")
		}
		token, err := p.oauthCredReader.GetClaudeAccessToken()
		if err != nil {
			return "", "", fmt.Errorf("failed to get OAuth token: %w", err)
		}
		return "Authorization", "Bearer " + token, nil
	default:
		return "x-api-key", p.apiKey, nil
	}
}

func (p *ClaudeProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to Claude format
	claudeReq := p.convertRequest(req)

	// Make API call
	resp, err := p.makeAPICall(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("Claude API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Claude API error: %d - %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %w", err)
	}

	// Convert back to internal format
	return p.convertResponse(req, &claudeResp, startTime), nil
}

func (p *ClaudeProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to Claude format
	claudeReq := p.convertRequest(req)
	claudeReq.Stream = true

	// Make streaming API call
	resp, err := p.makeAPICall(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("Claude streaming API call failed: %w", err)
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
					ProviderID:     "claude",
					ProviderName:   "Claude",
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
			var streamResp ClaudeStreamResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue // Skip malformed JSON
			}

			// Handle different event types
			switch streamResp.Type {
			case "content_block_delta":
				if streamResp.Delta != nil && streamResp.Delta.Type == "text_delta" {
					delta := streamResp.Delta.Text
					if delta != "" {
						fullContent += delta

						// Send chunk response
						chunkResp := &models.LLMResponse{
							ID:             "claude-stream-" + req.ID,
							RequestID:      req.ID,
							ProviderID:     "claude",
							ProviderName:   "Claude",
							Content:        delta,
							Confidence:     0.9, // High confidence for Claude
							TokensUsed:     1,   // Estimated
							ResponseTime:   time.Since(startTime).Milliseconds(),
							FinishReason:   "",
							Selected:       false,
							SelectionScore: 0.0,
							CreatedAt:      time.Now(),
						}
						ch <- chunkResp
					}
				}
			case "message_stop":
				// Stream finished
				break
			}
		}

		// Send final response
		finalResp := &models.LLMResponse{
			ID:             "claude-final-" + req.ID,
			RequestID:      req.ID,
			ProviderID:     "claude",
			ProviderName:   "Claude",
			Content:        "",
			Confidence:     0.9,
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

func (p *ClaudeProvider) convertRequest(req *models.LLMRequest) ClaudeRequest {
	// Convert messages
	messages := make([]ClaudeMessage, 0, len(req.Messages))

	// Add conversation messages (Claude doesn't use system prompt in messages)
	for _, msg := range req.Messages {
		if msg.Role != "system" {
			messages = append(messages, ClaudeMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Extract system message if present
	var systemPrompt string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			break
		}
	}

	return ClaudeRequest{
		Model:         p.model,
		MaxTokens:     req.ModelParams.MaxTokens,
		Temperature:   req.ModelParams.Temperature,
		TopP:          req.ModelParams.TopP,
		StopSequences: req.ModelParams.StopSequences,
		Stream:        false,
		Messages:      messages,
		System:        systemPrompt,
	}
}

func (p *ClaudeProvider) convertResponse(req *models.LLMRequest, claudeResp *ClaudeResponse, startTime time.Time) *models.LLMResponse {
	var content string
	var finishReason string

	if len(claudeResp.Content) > 0 {
		content = claudeResp.Content[0].Text
	}

	if claudeResp.StopReason != nil {
		finishReason = *claudeResp.StopReason
	}

	// Calculate confidence based on finish reason and response quality
	confidence := p.calculateConfidence(content, finishReason)

	return &models.LLMResponse{
		ID:           claudeResp.ID,
		RequestID:    req.ID,
		ProviderID:   "claude",
		ProviderName: "Claude",
		Content:      content,
		Confidence:   confidence,
		TokensUsed:   claudeResp.Usage.OutputTokens,
		ResponseTime: time.Since(startTime).Milliseconds(),
		FinishReason: finishReason,
		Metadata: map[string]any{
			"model":        claudeResp.Model,
			"input_tokens": claudeResp.Usage.InputTokens,
			"type":         claudeResp.Type,
		},
		Selected:       false,
		SelectionScore: 0.0,
		CreatedAt:      time.Now(),
	}
}

func (p *ClaudeProvider) calculateConfidence(content, finishReason string) float64 {
	confidence := 0.9 // High base confidence for Claude

	// Adjust based on finish reason
	switch finishReason {
	case "end_turn":
		confidence += 0.05
	case "max_tokens":
		confidence -= 0.1
	case "stop_sequence":
		confidence += 0.02
	}

	// Adjust based on content length and quality
	if len(content) > 50 {
		confidence += 0.02
	}
	if len(content) > 200 {
		confidence += 0.02
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

func (p *ClaudeProvider) makeAPICall(ctx context.Context, req ClaudeRequest) (*http.Response, error) {
	return p.makeAPICallWithAuthRetry(ctx, req, true)
}

// makeAPICallWithAuthRetry performs the API call with optional 401 retry
func (p *ClaudeProvider) makeAPICallWithAuthRetry(ctx context.Context, req ClaudeRequest, allowAuthRetry bool) (*http.Response, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

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
		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")

		// Set authentication header based on auth type
		authHeaderName, authHeaderValue, authErr := p.getAuthHeader()
		if authErr != nil {
			return nil, fmt.Errorf("failed to get auth header: %w", authErr)
		}
		httpReq.Header.Set(authHeaderName, authHeaderValue)

		httpReq.Header.Set("anthropic-version", "2023-06-01")
		httpReq.Header.Set("User-Agent", "HelixAgent/1.0")

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

		// Check for auth errors (401) - retry once with a short delay
		// This handles transient auth issues (token validation delays, auth service hiccups)
		if isAuthRetryableStatus(resp.StatusCode) && allowAuthRetry {
			resp.Body.Close()
			// Short delay before auth retry (500ms with jitter)
			authRetryDelay := 500 * time.Millisecond
			p.waitWithJitter(ctx, authRetryDelay)
			// Recursive call with auth retry disabled to prevent infinite loops
			return p.makeAPICallWithAuthRetry(ctx, req, false)
		}

		// Check for retryable status codes (429, 5xx)
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

// isAuthRetryableStatus returns true for auth errors that may be transient
// (e.g., token validation delays, temporary auth service issues)
func isAuthRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized // 401
}

// waitWithJitter waits for the specified duration plus random jitter
func (p *ClaudeProvider) waitWithJitter(ctx context.Context, delay time.Duration) {
	// Add 10% jitter
	jitter := time.Duration(rand.Float64() * 0.1 * float64(delay))
	select {
	case <-ctx.Done():
	case <-time.After(delay + jitter):
	}
}

// nextDelay calculates the next delay using exponential backoff
func (p *ClaudeProvider) nextDelay(currentDelay time.Duration) time.Duration {
	nextDelay := time.Duration(float64(currentDelay) * p.retryConfig.Multiplier)
	if nextDelay > p.retryConfig.MaxDelay {
		nextDelay = p.retryConfig.MaxDelay
	}
	return nextDelay
}

// GetCapabilities returns the capabilities of the Claude provider
func (p *ClaudeProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"claude-3-sonnet-20240229",
			"claude-3-opus-20240229",
			"claude-3-haiku-20240307",
			"claude-2.1",
			"claude-2.0",
		},
		SupportedFeatures: []string{
			"text_completion",
			"chat",
			"function_calling",
			"streaming",
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
		SupportsRefactoring:     true,
		Limits: models.ModelLimits{
			MaxTokens:             200000,
			MaxInputLength:        100000,
			MaxOutputLength:       4096,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"provider":     "Anthropic",
			"model_family": "Claude",
			"api_version":  "2023-06-01",
		},
	}
}

// ValidateConfig validates the provider configuration
func (p *ClaudeProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	// For OAuth auth, we don't need API key - check OAuth credentials instead
	if p.authType == AuthTypeOAuth {
		if p.oauthCredReader == nil {
			errors = append(errors, "OAuth credential reader is required")
		} else if !p.oauthCredReader.HasValidClaudeCredentials() {
			errors = append(errors, "valid OAuth credentials are required")
		}
	} else {
		// API key auth
		if p.apiKey == "" {
			errors = append(errors, "API key is required")
		}
	}

	if p.baseURL == "" {
		errors = append(errors, "base URL is required")
	}

	if p.model == "" {
		errors = append(errors, "model is required")
	}

	return len(errors) == 0, errors
}

// HealthCheck implements health checking for the Claude provider
func (p *ClaudeProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Simple health check - try to get models list or basic endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.anthropic.com/v1/messages", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Set authentication header based on auth type
	authHeaderName, authHeaderValue, authErr := p.getAuthHeader()
	if authErr != nil {
		return fmt.Errorf("failed to get auth header: %w", authErr)
	}
	req.Header.Set(authHeaderName, authHeaderValue)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	// Claude API returns 400 for GET requests to messages endpoint (expected)
	// We just check that the API is reachable and returns a response
	if resp.StatusCode >= 500 {
		return fmt.Errorf("health check failed with server error: %d", resp.StatusCode)
	}

	return nil
}
