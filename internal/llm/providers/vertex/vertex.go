// Package vertex provides Google Vertex AI integration
package vertex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// Provider implements LLMProvider for Google Vertex AI
type Provider struct {
	projectID string
	location  string
	model     string
	apiKey    string
	client    *http.Client
}

// Config holds provider configuration
type Config struct {
	ProjectID string
	Location  string // e.g., "us-central1"
	Model     string // e.g., "gemini-1.5-pro"
	APIKey    string // Or use service account
}

// NewProvider creates a new Vertex AI provider
func NewProvider(config Config) *Provider {
	if config.Location == "" {
		config.Location = "us-central1"
	}
	if config.Model == "" {
		config.Model = "gemini-1.5-pro"
	}

	return &Provider{
		projectID: config.ProjectID,
		location:  config.Location,
		model:     config.Model,
		apiKey:    config.APIKey,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete implements LLMProvider
func (p *Provider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Build request body for Gemini API
	contents := make([]map[string]interface{}, len(req.Messages))
	for i, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents[i] = map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{"text": m.Content},
			},
		}
	}

	body := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     req.ModelParams.Temperature,
			"maxOutputTokens": req.ModelParams.MaxTokens,
			"topP":            req.ModelParams.TopP,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.location, p.projectID, p.location, p.model)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	var content string
	for _, part := range result.Candidates[0].Content.Parts {
		content += part.Text
	}

	return &models.LLMResponse{
		Content:      content,
		FinishReason: result.Candidates[0].FinishReason,
		TokensUsed:   result.UsageMetadata.TotalTokenCount,
		ProviderID:   "vertex",
		ProviderName: "Google Vertex AI",
	}, nil
}

// CompleteStream implements LLMProvider
func (p *Provider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *models.LLMResponse, 1)
	ch <- resp
	close(ch)
	return ch, nil
}

// HealthCheck implements LLMProvider
func (p *Provider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/models?key=%s",
		p.location, p.projectID, p.location, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetCapabilities implements LLMProvider
func (p *Provider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"gemini-1.5-pro",
			"gemini-1.5-flash",
			"gemini-1.0-pro",
			"gemini-1.0-pro-vision",
		},
		SupportedFeatures: []string{
			"chat", "streaming", "vision", "function_calling",
			"code_completion", "json_mode", "multi_modal",
		},
		SupportedRequestTypes:   []string{"chat", "completion"},
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
			MaxTokens:             1000000, // Gemini 1.5 Pro supports up to 1M tokens
			MaxInputLength:        1000000,
			MaxOutputLength:       8192,
			MaxConcurrentRequests: 100,
		},
		Metadata: map[string]string{
			"provider": "vertex",
			"type":     "google_cloud",
			"location": p.location,
		},
	}
}

// ValidateConfig implements LLMProvider
func (p *Provider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	if p.projectID == "" {
		errors = append(errors, "project_id is required")
	}

	if p.location == "" {
		errors = append(errors, "location is required")
	}

	if p.apiKey == "" {
		errors = append(errors, "api_key is required")
	}

	if p.model == "" {
		errors = append(errors, "model is required")
	}

	return len(errors) == 0, errors
}

// Ensure interface is implemented
var _ llm.LLMProvider = (*Provider)(nil)
