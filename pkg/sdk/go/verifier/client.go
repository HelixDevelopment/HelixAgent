// Package verifier provides a Go SDK for the SuperAgent LLMsVerifier API
package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the main client for interacting with the Verifier API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// ClientConfig represents client configuration
type ClientConfig struct {
	BaseURL    string
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// NewClient creates a new verifier client
func NewClient(config ClientConfig) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8081"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: config.Timeout,
		}
	}

	return &Client{
		baseURL:    config.BaseURL,
		apiKey:     config.APIKey,
		httpClient: httpClient,
	}
}

// VerificationRequest represents a verification request
type VerificationRequest struct {
	ModelID  string   `json:"model_id"`
	Provider string   `json:"provider"`
	Tests    []string `json:"tests,omitempty"`
}

// VerificationResult represents a verification result
type VerificationResult struct {
	ModelID          string          `json:"model_id"`
	Provider         string          `json:"provider"`
	Verified         bool            `json:"verified"`
	Score            float64         `json:"score"`
	OverallScore     float64         `json:"overall_score"`
	ScoreSuffix      string          `json:"score_suffix"`
	CodeVisible      bool            `json:"code_visible"`
	Tests            map[string]bool `json:"tests"`
	VerificationTime int64           `json:"verification_time_ms"`
	Message          string          `json:"message,omitempty"`
}

// VerifyModel verifies a specific model
func (c *Client) VerifyModel(ctx context.Context, req VerificationRequest) (*VerificationResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/verifier/verify", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result VerificationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// BatchVerifyRequest represents a batch verification request
type BatchVerifyRequest struct {
	Models []struct {
		ModelID  string `json:"model_id"`
		Provider string `json:"provider"`
	} `json:"models"`
}

// BatchVerifyResult represents a batch verification result
type BatchVerifyResult struct {
	Results []VerificationResult `json:"results"`
	Summary struct {
		Total    int `json:"total"`
		Verified int `json:"verified"`
		Failed   int `json:"failed"`
	} `json:"summary"`
}

// BatchVerify verifies multiple models
func (c *Client) BatchVerify(ctx context.Context, req BatchVerifyRequest) (*BatchVerifyResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/verifier/verify/batch", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BatchVerifyResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetVerificationStatus gets the verification status of a model
func (c *Client) GetVerificationStatus(ctx context.Context, modelID string) (*VerificationResult, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/verifier/status/"+modelID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result VerificationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CodeVisibilityRequest represents a code visibility test request
type CodeVisibilityRequest struct {
	ModelID  string `json:"model_id"`
	Provider string `json:"provider"`
	Language string `json:"language,omitempty"`
}

// CodeVisibilityResult represents a code visibility test result
type CodeVisibilityResult struct {
	ModelID     string  `json:"model_id"`
	Provider    string  `json:"provider"`
	CodeVisible bool    `json:"code_visible"`
	Language    string  `json:"language"`
	Prompt      string  `json:"prompt"`
	Response    string  `json:"response"`
	Confidence  float64 `json:"confidence"`
}

// TestCodeVisibility tests if a model can see injected code
func (c *Client) TestCodeVisibility(ctx context.Context, req CodeVisibilityRequest) (*CodeVisibilityResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/verifier/test/code-visibility", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CodeVisibilityResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ScoreResult represents a model score result
type ScoreResult struct {
	ModelID      string          `json:"model_id"`
	ModelName    string          `json:"model_name"`
	OverallScore float64         `json:"overall_score"`
	ScoreSuffix  string          `json:"score_suffix"`
	Components   ScoreComponents `json:"components"`
	CalculatedAt string          `json:"calculated_at"`
	DataSource   string          `json:"data_source"`
}

// ScoreComponents represents score components
type ScoreComponents struct {
	SpeedScore      float64 `json:"speed_score"`
	EfficiencyScore float64 `json:"efficiency_score"`
	CostScore       float64 `json:"cost_score"`
	CapabilityScore float64 `json:"capability_score"`
	RecencyScore    float64 `json:"recency_score"`
}

// GetModelScore gets the score for a model
func (c *Client) GetModelScore(ctx context.Context, modelID string) (*ScoreResult, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/verifier/scores/"+modelID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ScoreResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// TopModelsResult represents top models result
type TopModelsResult struct {
	Models []ModelWithScore `json:"models"`
	Total  int              `json:"total"`
}

// ModelWithScore represents a model with score
type ModelWithScore struct {
	ModelID      string  `json:"model_id"`
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	OverallScore float64 `json:"overall_score"`
	ScoreSuffix  string  `json:"score_suffix"`
	Rank         int     `json:"rank"`
}

// GetTopModels gets the top scoring models
func (c *Client) GetTopModels(ctx context.Context, limit int) (*TopModelsResult, error) {
	url := fmt.Sprintf("/api/v1/verifier/scores/top?limit=%d", limit)
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TopModelsResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ProviderHealth represents provider health status
type ProviderHealth struct {
	ProviderID    string  `json:"provider_id"`
	ProviderName  string  `json:"provider_name"`
	Healthy       bool    `json:"healthy"`
	CircuitState  string  `json:"circuit_state"`
	FailureCount  int     `json:"failure_count"`
	SuccessCount  int     `json:"success_count"`
	AvgResponseMs int64   `json:"avg_response_ms"`
	UptimePercent float64 `json:"uptime_percent"`
	LastCheckedAt string  `json:"last_checked_at"`
}

// GetProviderHealth gets the health status of a provider
func (c *Client) GetProviderHealth(ctx context.Context, providerID string) (*ProviderHealth, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/verifier/health/providers/"+providerID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ProviderHealth
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetAllProvidersHealth gets health status for all providers
func (c *Client) GetAllProvidersHealth(ctx context.Context) ([]ProviderHealth, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/verifier/health/providers", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Providers []ProviderHealth `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Providers, nil
}

// GetHealthyProviders gets a list of healthy provider IDs
func (c *Client) GetHealthyProviders(ctx context.Context) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/verifier/health/healthy", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Providers []string `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Providers, nil
}

// GetModelNameWithScore gets the model name with score suffix
func (c *Client) GetModelNameWithScore(ctx context.Context, modelID string) (string, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/verifier/scores/"+modelID+"/name", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		NameWithScore string `json:"name_with_score"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.NameWithScore, nil
}

// Health gets the overall health of the verifier service
func (c *Client) Health(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/verifier/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// doRequest performs an HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
