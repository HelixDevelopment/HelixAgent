// Package helixllm provides a HelixLLM provider for HelixAgent.
// It integrates the HelixLLM submodule as a first-class LLM provider,
// leveraging HelixLLM's OpenAI-compatible API and RAG capabilities.
package helixllm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

const (
	defaultEndpoint    = "https://localhost:8443"
	defaultModel       = "helixllm-default"
	defaultTimeout     = 60 * time.Second
	chatEndpoint       = "/v1/chat/completions"
	embeddingsEndpoint = "/v1/embeddings"
	modelsEndpoint     = "/v1/models"
	healthEndpoint     = "/internal/health"
)

// Provider implements the LLMProvider interface for HelixLLM
type Provider struct {
	endpoint      string
	apiKey        string
	model         string
	timeout       time.Duration
	tlsSkipVerify bool
	httpClient    *http.Client

	mu          sync.RWMutex
	initialized bool
	initErr     error
	initOnce    sync.Once
}

// Config holds configuration for the HelixLLM provider
type Config struct {
	Endpoint      string
	APIKey        string
	Model         string
	Timeout       time.Duration
	TLSSkipVerify bool
}

// NewProvider creates a new HelixLLM provider
func NewProvider(cfg Config) *Provider {
	if cfg.Endpoint == "" {
		cfg.Endpoint = getEnv("HELIX_LLM_ENDPOINT", defaultEndpoint)
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify || getEnvBool("HELIX_LLM_TLS_SKIP_VERIFY", false),
		},
	}

	return &Provider{
		endpoint:      cfg.Endpoint,
		apiKey:        cfg.APIKey,
		model:         cfg.Model,
		timeout:       cfg.Timeout,
		tlsSkipVerify: cfg.TLSSkipVerify,
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
	}
}

// NewProviderFromEnv creates a new HelixLLM provider from environment variables
func NewProviderFromEnv() *Provider {
	return NewProvider(Config{
		Endpoint:      os.Getenv("HELIX_LLM_ENDPOINT"),
		APIKey:        os.Getenv("HELIX_LLM_API_KEY"),
		Model:         os.Getenv("HELIX_LLM_MODEL"),
		TLSSkipVerify: os.Getenv("HELIX_LLM_TLS_SKIP_VERIFY") == "true",
	})
}

// Complete implements the LLMProvider interface
func (p *Provider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if err := p.initialize(); err != nil {
		return nil, err
	}

	model := req.ModelParams.Model
	if model == "" {
		model = p.model
	}

	messages := make([]Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "" {
			role = "user"
		}
		messages = append(messages, Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	chatReq := ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Stream:      false,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   req.ModelParams.MaxTokens,
		TopP:        req.ModelParams.TopP,
	}

	endpoint := p.endpoint + chatEndpoint
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	return &models.LLMResponse{
		Content:    chatResp.Choices[0].Message.Content,
		TokensUsed: chatResp.Usage.TotalTokens,
		Metadata: map[string]interface{}{
			"model":        chatResp.Model,
			"provider":     "helixllm",
			"prompt_tokens":     chatResp.Usage.PromptTokens,
			"completion_tokens": chatResp.Usage.CompletionTokens,
		},
	}, nil
}

// CompleteStream implements the LLMProvider interface for streaming
func (p *Provider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if err := p.initialize(); err != nil {
		return nil, err
	}

	model := req.ModelParams.Model
	if model == "" {
		model = p.model
	}

	messages := make([]Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "" {
			role = "user"
		}
		messages = append(messages, Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	chatReq := ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Stream:      true,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   req.ModelParams.MaxTokens,
		TopP:        req.ModelParams.TopP,
	}

	endpoint := p.endpoint + chatEndpoint
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	resultChan := make(chan *models.LLMResponse)
	go p.handleStream(resp, resultChan)

	return resultChan, nil
}

func (p *Provider) handleStream(resp *http.Response, resultChan chan<- *models.LLMResponse) {
	defer close(resultChan)
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		var streamResp ChatCompletionResponse
		if err := decoder.Decode(&streamResp); err != nil {
			if err == io.EOF {
				return
			}
			resultChan <- &models.LLMResponse{
				Metadata: map[string]interface{}{
					"error": fmt.Sprintf("stream decode error: %v", err),
				},
			}
			return
		}

		if len(streamResp.Choices) > 0 {
			resultChan <- &models.LLMResponse{
				Content:    streamResp.Choices[0].Message.Content,
				TokensUsed: streamResp.Usage.TotalTokens,
				Metadata: map[string]interface{}{
					"model": streamResp.Model,
				},
			}
		}
	}
}

// HealthCheck implements the LLMProvider interface
func (p *Provider) HealthCheck() error {
	if err := p.initialize(); err != nil {
		return err
	}

	endpoint := p.endpoint + healthEndpoint
	resp, err := p.httpClient.Get(endpoint)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetCapabilities implements the LLMProvider interface
func (p *Provider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:         []string{p.model},
		SupportedFeatures:       []string{"streaming", "embeddings", "function_calling"},
		SupportedRequestTypes:   []string{"chat", "completion"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		Limits: models.ModelLimits{
			MaxTokens:             8192,
			MaxInputLength:        4096,
			MaxOutputLength:       4096,
			MaxConcurrentRequests: 100,
		},
		Metadata: map[string]string{
			"provider_name": "helixllm",
			"default_model": p.model,
		},
		SupportsTools:          true,
		SupportsSearch:         false,
		SupportsReasoning:      true,
		SupportsCodeCompletion: true,
		SupportsCodeAnalysis:   true,
		SupportsRefactoring:    true,
	}
}

// ValidateConfig implements the LLMProvider interface
func (p *Provider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	if endpoint, ok := config["endpoint"].(string); ok && endpoint == "" {
		errors = append(errors, "endpoint cannot be empty")
	}

	return len(errors) == 0, errors
}

// initialize performs lazy initialization of the provider
func (p *Provider) initialize() error {
	p.initOnce.Do(func() {
		p.initErr = p.doInitialize()
		p.initialized = true
	})
	return p.initErr
}

func (p *Provider) doInitialize() error {
	if p.endpoint == "" {
		return fmt.Errorf("helixllm endpoint not configured")
	}
	return nil
}

// Ensure Provider implements LLMProvider interface
var _ llm.LLMProvider = (*Provider)(nil)
