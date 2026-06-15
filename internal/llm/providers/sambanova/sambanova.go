package sambanova

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"dev.helix.agent/internal/llm/discovery"
	"dev.helix.agent/internal/models"
)

const (
	SambaNovaAPIURL     = "https://api.sambanova.ai/v1/chat/completions"
	SambaNovaModel      = "Meta-Llama-3.1-8B-Instruct"
	SambaNovaModelsURL  = "https://api.sambanova.ai/v1/models"
	SambaNovaMaxContext = 131072
	SambaNovaMaxOutput  = 4096
)

type SambaNovaProvider struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	retryConfig RetryConfig
	discoverer  *discovery.Discoverer
}

type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

type SambaNovaRequest struct {
	Model       string             `json:"model"`
	Messages    []SambaNovaMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type SambaNovaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SambaNovaResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []SambaNovaChoice `json:"choices"`
	Usage   SambaNovaUsage    `json:"usage"`
}

type SambaNovaChoice struct {
	Index        int              `json:"index"`
	Message      SambaNovaMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type SambaNovaUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type SambaNovaStreamResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []SambaNovaStreamChoice `json:"choices"`
}

type SambaNovaStreamChoice struct {
	Index        int              `json:"index"`
	Delta        SambaNovaMessage `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type SambaNovaErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func NewSambaNovaProvider(apiKey, baseURL, model string) *SambaNovaProvider {
	return NewSambaNovaProviderWithRetry(apiKey, baseURL, model, DefaultRetryConfig())
}

func NewSambaNovaProviderWithRetry(apiKey, baseURL, model string, retryConfig RetryConfig) *SambaNovaProvider {
	if baseURL == "" {
		baseURL = SambaNovaAPIURL
	}
	if model == "" {
		model = SambaNovaModel
	}

	p := &SambaNovaProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		retryConfig: retryConfig,
	}

	p.discoverer = discovery.NewDiscoverer(discovery.ProviderConfig{
		ProviderName:   "sambanova",
		ModelsEndpoint: SambaNovaModelsURL,
		ModelsDevID:    "sambanova",
		APIKey:         apiKey,
		FallbackModels: []string{
			"Meta-Llama-3.1-8B-Instruct",
			"Meta-Llama-3.1-70B-Instruct",
			"Meta-Llama-3.2-1B-Instruct",
			"Meta-Llama-3.2-3B-Instruct",
		},
	})

	return p
}

func (p *SambaNovaProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	startTime := time.Now()

	sReq := p.convertRequest(req)

	resp, err := p.makeAPICall(ctx, sReq)
	if err != nil {
		return nil, fmt.Errorf("SambaNova API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp SambaNovaErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("SambaNova API error: %d - %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("SambaNova API error: %d - %s", resp.StatusCode, string(body))
	}

	var sResp SambaNovaResponse
	if err := json.Unmarshal(body, &sResp); err != nil {
		return nil, fmt.Errorf("failed to parse SambaNova response: %w", err)
	}

	if len(sResp.Choices) == 0 {
		return nil, fmt.Errorf("SambaNova API returned no choices")
	}

	return p.convertResponse(req, &sResp, startTime), nil
}

func (p *SambaNovaProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	startTime := time.Now()

	sReq := p.convertRequest(req)
	sReq.Stream = true

	resp, err := p.makeAPICall(ctx, sReq)
	if err != nil {
		return nil, fmt.Errorf("SambaNova streaming API call failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body) //nolint:errcheck
		_ = resp.Body.Close()
		return nil, fmt.Errorf("SambaNova API error: HTTP %d - %s", resp.StatusCode, string(body))
	}

	ch := make(chan *models.LLMResponse)

	go func() {
		defer func() { _ = resp.Body.Close() }()
		defer close(ch)

		reader := bufio.NewReader(resp.Body)
		var fullContent string

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				ch <- &models.LLMResponse{
					ID:           "stream-error-" + req.ID,
					RequestID:    req.ID,
					ProviderID:   "sambanova",
					ProviderName: "SambaNova",
					FinishReason: "error",
					CreatedAt:    time.Now(),
				}
				return
			}

			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			line = bytes.TrimPrefix(line, []byte("data: "))

			if bytes.Equal(line, []byte("[DONE]")) {
				break
			}

			var streamResp SambaNovaStreamResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				delta := streamResp.Choices[0].Delta.Content
				if delta != "" {
					fullContent += delta
					ch <- &models.LLMResponse{
						ID:           streamResp.ID,
						RequestID:    req.ID,
						ProviderID:   "sambanova",
						ProviderName: "SambaNova",
						Content:      delta,
						Confidence:   0.8,
						TokensUsed:   1,
						ResponseTime: time.Since(startTime).Milliseconds(),
						CreatedAt:    time.Now(),
					}
				}

				if streamResp.Choices[0].FinishReason != nil {
					break
				}
			}
		}

		ch <- &models.LLMResponse{
			ID:           "stream-final-" + req.ID,
			RequestID:    req.ID,
			ProviderID:   "sambanova",
			ProviderName: "SambaNova",
			Content:      "",
			Confidence:   0.8,
			TokensUsed:   len(fullContent) / 4,
			ResponseTime: time.Since(startTime).Milliseconds(),
			FinishReason: "stop",
			CreatedAt:    time.Now(),
		}
	}()

	return ch, nil
}

func (p *SambaNovaProvider) convertRequest(req *models.LLMRequest) SambaNovaRequest {
	messages := make([]SambaNovaMessage, 0, len(req.Messages)+1)

	if req.Prompt != "" {
		messages = append(messages, SambaNovaMessage{Role: "system", Content: req.Prompt})
	}

	for _, msg := range req.Messages {
		messages = append(messages, SambaNovaMessage{Role: msg.Role, Content: msg.Content})
	}

	maxTokens := req.ModelParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = SambaNovaMaxOutput
	} else if maxTokens > SambaNovaMaxOutput {
		maxTokens = SambaNovaMaxOutput
	}

	return SambaNovaRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   maxTokens,
		TopP:        req.ModelParams.TopP,
		Stream:      false,
	}
}

func (p *SambaNovaProvider) convertResponse(req *models.LLMRequest, sResp *SambaNovaResponse, startTime time.Time) *models.LLMResponse {
	var content, finishReason string
	if len(sResp.Choices) > 0 {
		content = sResp.Choices[0].Message.Content
		finishReason = sResp.Choices[0].FinishReason
	}

	confidence := 0.8
	if finishReason == "stop" {
		confidence += 0.1
	}
	if len(content) > 100 {
		confidence += 0.05
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return &models.LLMResponse{
		ID:           sResp.ID,
		RequestID:    req.ID,
		ProviderID:   "sambanova",
		ProviderName: "SambaNova",
		Content:      content,
		Confidence:   confidence,
		TokensUsed:   sResp.Usage.TotalTokens,
		ResponseTime: time.Since(startTime).Milliseconds(),
		FinishReason: finishReason,
		Metadata: map[string]any{
			"model":             sResp.Model,
			"prompt_tokens":     sResp.Usage.PromptTokens,
			"completion_tokens": sResp.Usage.CompletionTokens,
		},
		CreatedAt: time.Now(),
	}
}

func (p *SambaNovaProvider) makeAPICall(ctx context.Context, req SambaNovaRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	delay := p.retryConfig.InitialDelay

	for attempt := 0; attempt <= p.retryConfig.MaxRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("User-Agent", "helix_agent/1.0")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			if attempt < p.retryConfig.MaxRetries {
				jitter := time.Duration(rand.Float64() * 0.1 * float64(delay)) // #nosec G404 -- non-security random (jitter/test data/load spread); math/rand is the correct choice
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay + jitter):
				}
				delay = time.Duration(float64(delay) * p.retryConfig.Multiplier)
				if delay > p.retryConfig.MaxDelay {
					delay = p.retryConfig.MaxDelay
				}
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if attempt < p.retryConfig.MaxRetries {
				_ = resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d: retryable error", resp.StatusCode)
				jitter := time.Duration(rand.Float64() * 0.1 * float64(delay)) // #nosec G404 -- non-security random (jitter/test data/load spread); math/rand is the correct choice
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay + jitter):
				}
				delay = time.Duration(float64(delay) * p.retryConfig.Multiplier)
				if delay > p.retryConfig.MaxDelay {
					delay = p.retryConfig.MaxDelay
				}
				continue
			}
		}

		return resp, nil
	}

	return nil, fmt.Errorf("all retries failed: %w", lastErr)
}

func (p *SambaNovaProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:        p.discoverer.DiscoverModels(),
		SupportedFeatures:      []string{"text_completion", "chat", "streaming", "code_completion"},
		SupportsStreaming:      true,
		SupportsCodeCompletion: true,
		SupportsCodeAnalysis:   true,
		Limits: models.ModelLimits{
			MaxTokens:       SambaNovaMaxOutput,
			MaxInputLength:  SambaNovaMaxContext,
			MaxOutputLength: SambaNovaMaxOutput,
		},
		Metadata: map[string]string{
			"provider": "SambaNova",
			"note":     "SambaNova AI inference platform",
		},
	}
}

func (p *SambaNovaProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string
	if p.apiKey == "" {
		errors = append(errors, "API key is required")
	}
	return len(errors) == 0, errors
}

func (p *SambaNovaProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Derive the models endpoint from the configured base URL (the chat-completions
	// URL) so a custom/injected base URL is honored instead of the hardcoded
	// production constant — otherwise HealthCheck ignores p.baseURL entirely.
	modelsURL := strings.TrimSuffix(p.baseURL, "/chat/completions") + "/models"
	req, _ := http.NewRequestWithContext(ctx, "GET", modelsURL, nil) //nolint:errcheck
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}
	return nil
}
