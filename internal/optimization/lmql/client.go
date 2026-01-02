// Package lmql provides HTTP client for the LMQL service.
package lmql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the LMQL service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClientConfig holds configuration for the LMQL client.
type ClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL: "http://localhost:8014",
		Timeout: 120 * time.Second,
	}
}

// NewClient creates a new LMQL client.
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	return &Client{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// QueryRequest represents an LMQL-style query request.
type QueryRequest struct {
	Query       string                 `json:"query"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
}

// QueryResponse represents the query result.
type QueryResponse struct {
	Result               map[string]interface{} `json:"result"`
	RawOutput            string                 `json:"raw_output"`
	ConstraintsSatisfied bool                   `json:"constraints_satisfied"`
}

// Constraint represents a generation constraint.
type Constraint struct {
	Type    string `json:"type"`
	Value   string `json:"value,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// ConstrainedRequest represents a constrained generation request.
type ConstrainedRequest struct {
	Prompt      string       `json:"prompt"`
	Constraints []Constraint `json:"constraints"`
	Temperature float64      `json:"temperature,omitempty"`
}

// ConstraintResult represents a constraint check result.
type ConstraintResult struct {
	Type      string `json:"type"`
	Value     string `json:"value,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Satisfied bool   `json:"satisfied"`
}

// ConstrainedResponse represents the constrained generation result.
type ConstrainedResponse struct {
	Text               string             `json:"text"`
	ConstraintsChecked []ConstraintResult `json:"constraints_checked"`
	AllSatisfied       bool               `json:"all_satisfied"`
}

// DecodingRequest represents a custom decoding strategy request.
type DecodingRequest struct {
	Prompt      string  `json:"prompt"`
	Strategy    string  `json:"strategy"` // argmax, sample, beam
	BeamWidth   int     `json:"beam_width,omitempty"`
	NumSamples  int     `json:"num_samples,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// DecodingResponse represents the custom decoding result.
type DecodingResponse struct {
	Outputs      []string               `json:"outputs"`
	StrategyUsed string                 `json:"strategy_used"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ScoreResponse represents completion scoring results.
type ScoreResponse struct {
	Prompt  string             `json:"prompt"`
	Scores  map[string]float64 `json:"scores"`
	Ranking []int              `json:"ranking"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status       string `json:"status"`
	Version      string `json:"version"`
	LLMAvailable bool   `json:"llm_available"`
}

// Health checks the service health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// ExecuteQuery executes an LMQL-style query.
func (c *Client) ExecuteQuery(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 500
	}

	resp, err := c.doRequest(ctx, "POST", "/query", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GenerateConstrained generates text with explicit constraints.
func (c *Client) GenerateConstrained(ctx context.Context, req *ConstrainedRequest) (*ConstrainedResponse, error) {
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	resp, err := c.doRequest(ctx, "POST", "/constrained", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ConstrainedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GenerateWithMaxLength generates text with a maximum length constraint.
func (c *Client) GenerateWithMaxLength(ctx context.Context, prompt string, maxLength int) (*ConstrainedResponse, error) {
	return c.GenerateConstrained(ctx, &ConstrainedRequest{
		Prompt: prompt,
		Constraints: []Constraint{
			{Type: "max_length", Value: fmt.Sprintf("%d", maxLength)},
		},
	})
}

// GenerateContaining generates text that must contain specific content.
func (c *Client) GenerateContaining(ctx context.Context, prompt string, mustContain []string) (*ConstrainedResponse, error) {
	constraints := make([]Constraint, len(mustContain))
	for i, s := range mustContain {
		constraints[i] = Constraint{Type: "contains", Value: s}
	}
	return c.GenerateConstrained(ctx, &ConstrainedRequest{
		Prompt:      prompt,
		Constraints: constraints,
	})
}

// GenerateWithPattern generates text matching a regex pattern.
func (c *Client) GenerateWithPattern(ctx context.Context, prompt, pattern string) (*ConstrainedResponse, error) {
	return c.GenerateConstrained(ctx, &ConstrainedRequest{
		Prompt: prompt,
		Constraints: []Constraint{
			{Type: "regex", Pattern: pattern},
		},
	})
}

// Decode applies a custom decoding strategy.
func (c *Client) Decode(ctx context.Context, req *DecodingRequest) (*DecodingResponse, error) {
	if req.Strategy == "" {
		req.Strategy = "argmax"
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	resp, err := c.doRequest(ctx, "POST", "/decode", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result DecodingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// DecodeGreedy performs greedy (argmax) decoding.
func (c *Client) DecodeGreedy(ctx context.Context, prompt string) (string, error) {
	result, err := c.Decode(ctx, &DecodingRequest{
		Prompt:   prompt,
		Strategy: "argmax",
	})
	if err != nil {
		return "", err
	}
	if len(result.Outputs) > 0 {
		return result.Outputs[0], nil
	}
	return "", fmt.Errorf("no output generated")
}

// DecodeSample generates multiple samples.
func (c *Client) DecodeSample(ctx context.Context, prompt string, numSamples int, temperature float64) ([]string, error) {
	if numSamples == 0 {
		numSamples = 3
	}
	if temperature == 0 {
		temperature = 0.7
	}

	result, err := c.Decode(ctx, &DecodingRequest{
		Prompt:      prompt,
		Strategy:    "sample",
		NumSamples:  numSamples,
		Temperature: temperature,
	})
	if err != nil {
		return nil, err
	}
	return result.Outputs, nil
}

// DecodeBeam performs beam search decoding.
func (c *Client) DecodeBeam(ctx context.Context, prompt string, beamWidth int) ([]string, error) {
	if beamWidth == 0 {
		beamWidth = 3
	}

	result, err := c.Decode(ctx, &DecodingRequest{
		Prompt:    prompt,
		Strategy:  "beam",
		BeamWidth: beamWidth,
	})
	if err != nil {
		return nil, err
	}
	return result.Outputs, nil
}

// ScoreCompletions scores multiple completions for a prompt.
func (c *Client) ScoreCompletions(ctx context.Context, prompt string, completions []string) (*ScoreResponse, error) {
	type scoreReq struct {
		Prompt      string   `json:"prompt"`
		Completions []string `json:"completions"`
	}

	resp, err := c.doRequest(ctx, "POST", "/score", &scoreReq{
		Prompt:      prompt,
		Completions: completions,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// SelectBestCompletion selects the best completion from a list.
func (c *Client) SelectBestCompletion(ctx context.Context, prompt string, completions []string) (string, error) {
	result, err := c.ScoreCompletions(ctx, prompt, completions)
	if err != nil {
		return "", err
	}

	var best string
	var bestScore float64 = -1

	for completion, score := range result.Scores {
		if score > bestScore {
			bestScore = score
			best = completion
		}
	}

	return best, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

// IsAvailable checks if the service is available.
func (c *Client) IsAvailable(ctx context.Context) bool {
	health, err := c.Health(ctx)
	return err == nil && health.Status == "healthy"
}
