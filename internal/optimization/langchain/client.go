// Package langchain provides HTTP client for the LangChain service.
package langchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the LangChain service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClientConfig holds configuration for the LangChain client.
type ClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL: "http://localhost:8011",
		Timeout: 120 * time.Second,
	}
}

// NewClient creates a new LangChain client.
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

// DecomposeRequest represents a task decomposition request.
type DecomposeRequest struct {
	Task     string `json:"task"`
	MaxSteps int    `json:"max_steps,omitempty"`
	Context  string `json:"context,omitempty"`
}

// Subtask represents a decomposed subtask.
type Subtask struct {
	ID           int      `json:"id"`
	Description  string   `json:"description"`
	Dependencies []int    `json:"dependencies"`
	Complexity   string   `json:"complexity"`
}

// DecomposeResponse represents the decomposition result.
type DecomposeResponse struct {
	Subtasks  []Subtask `json:"subtasks"`
	Reasoning string    `json:"reasoning"`
}

// ChainRequest represents a chain execution request.
type ChainRequest struct {
	ChainType   string                 `json:"chain_type"`
	Prompt      string                 `json:"prompt"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
}

// ChainStep represents a step in chain execution.
type ChainStep struct {
	Step   string `json:"step"`
	Input  string `json:"input,omitempty"`
	Output string `json:"output,omitempty"`
}

// ChainResponse represents the chain execution result.
type ChainResponse struct {
	Result string      `json:"result"`
	Steps  []ChainStep `json:"steps"`
}

// ReActRequest represents a ReAct agent request.
type ReActRequest struct {
	Goal           string   `json:"goal"`
	AvailableTools []string `json:"available_tools,omitempty"`
	MaxIterations  int      `json:"max_iterations,omitempty"`
	Context        string   `json:"context,omitempty"`
}

// ReActStep represents a reasoning step.
type ReActStep struct {
	Iteration   int    `json:"iteration"`
	Thought     string `json:"thought"`
	Action      string `json:"action"`
	ActionInput string `json:"action_input"`
	Observation string `json:"observation"`
}

// ReActResponse represents the ReAct agent result.
type ReActResponse struct {
	Answer         string      `json:"answer"`
	ReasoningTrace []ReActStep `json:"reasoning_trace"`
	ToolsUsed      []string    `json:"tools_used"`
	Iterations     int         `json:"iterations"`
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

// Decompose decomposes a task into subtasks.
func (c *Client) Decompose(ctx context.Context, req *DecomposeRequest) (*DecomposeResponse, error) {
	if req.MaxSteps == 0 {
		req.MaxSteps = 5
	}

	resp, err := c.doRequest(ctx, "POST", "/decompose", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result DecomposeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// ExecuteChain executes a LangChain chain.
func (c *Client) ExecuteChain(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	resp, err := c.doRequest(ctx, "POST", "/chain", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChainResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// RunReActAgent runs a ReAct reasoning agent.
func (c *Client) RunReActAgent(ctx context.Context, req *ReActRequest) (*ReActResponse, error) {
	if req.MaxIterations == 0 {
		req.MaxIterations = 10
	}

	resp, err := c.doRequest(ctx, "POST", "/react", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ReActResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// Summarize summarizes text using a chain.
func (c *Client) Summarize(ctx context.Context, text string, maxLength int) (string, error) {
	if maxLength == 0 {
		maxLength = 200
	}

	type summaryReq struct {
		Text      string `json:"text"`
		MaxLength int    `json:"max_length"`
	}

	resp, err := c.doRequest(ctx, "POST", "/summarize", &summaryReq{
		Text:      text,
		MaxLength: maxLength,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Summary, nil
}

// Transform transforms text according to instructions.
func (c *Client) Transform(ctx context.Context, text, transformation string) (string, error) {
	type transformReq struct {
		Text           string `json:"text"`
		Transformation string `json:"transformation"`
	}

	resp, err := c.doRequest(ctx, "POST", "/transform", &transformReq{
		Text:           text,
		Transformation: transformation,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	return result.Result, nil
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
