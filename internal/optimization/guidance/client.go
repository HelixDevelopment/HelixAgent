// Package guidance provides HTTP client for the Guidance service.
package guidance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the Guidance service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClientConfig holds configuration for the Guidance client.
type ClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL: "http://localhost:8013",
		Timeout: 120 * time.Second,
	}
}

// NewClient creates a new Guidance client.
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

// GrammarRequest represents a grammar-constrained generation request.
type GrammarRequest struct {
	Prompt      string  `json:"prompt"`
	Grammar     string  `json:"grammar"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// GrammarResponse represents the grammar generation result.
type GrammarResponse struct {
	Text   string                 `json:"text"`
	Parsed map[string]interface{} `json:"parsed"`
	Valid  bool                   `json:"valid"`
}

// TemplateRequest represents a template-based generation request.
type TemplateRequest struct {
	Template    string                 `json:"template"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Constraints map[string]string      `json:"constraints,omitempty"`
}

// TemplateResponse represents the template generation result.
type TemplateResponse struct {
	FilledTemplate  string            `json:"filled_template"`
	GeneratedValues map[string]string `json:"generated_values"`
}

// SelectOption represents an option for selection.
type SelectOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// SelectRequest represents a constrained selection request.
type SelectRequest struct {
	Prompt        string   `json:"prompt"`
	Options       []string `json:"options"`
	AllowMultiple bool     `json:"allow_multiple,omitempty"`
}

// SelectResponse represents the selection result.
type SelectResponse struct {
	Selected  []string `json:"selected"`
	Reasoning string   `json:"reasoning"`
}

// RegexRequest represents a regex-constrained generation request.
type RegexRequest struct {
	Prompt      string `json:"prompt"`
	Pattern     string `json:"pattern"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
}

// RegexResponse represents the regex generation result.
type RegexResponse struct {
	Text        string   `json:"text"`
	Matches     bool     `json:"matches"`
	MatchGroups []string `json:"match_groups"`
}

// JSONSchemaRequest represents a JSON schema generation request.
type JSONSchemaRequest struct {
	Prompt string                 `json:"prompt"`
	Schema map[string]interface{} `json:"schema"`
}

// JSONSchemaResponse represents the JSON schema generation result.
type JSONSchemaResponse struct {
	JSON  map[string]interface{} `json:"json"`
	Valid bool                   `json:"valid"`
	Raw   string                 `json:"raw,omitempty"`
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

// GenerateWithGrammar generates text following a grammar specification.
func (c *Client) GenerateWithGrammar(ctx context.Context, req *GrammarRequest) (*GrammarResponse, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 500
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	resp, err := c.doRequest(ctx, "POST", "/grammar", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GrammarResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GenerateFromTemplate generates text using a template with constraints.
func (c *Client) GenerateFromTemplate(ctx context.Context, req *TemplateRequest) (*TemplateResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/template", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result TemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// Select selects from constrained options.
func (c *Client) Select(ctx context.Context, req *SelectRequest) (*SelectResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/select", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SelectResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// SelectOne selects exactly one option.
func (c *Client) SelectOne(ctx context.Context, prompt string, options []string) (string, error) {
	result, err := c.Select(ctx, &SelectRequest{
		Prompt:        prompt,
		Options:       options,
		AllowMultiple: false,
	})
	if err != nil {
		return "", err
	}
	if len(result.Selected) > 0 {
		return result.Selected[0], nil
	}
	return "", fmt.Errorf("no option selected")
}

// GenerateWithRegex generates text matching a regex pattern.
func (c *Client) GenerateWithRegex(ctx context.Context, req *RegexRequest) (*RegexResponse, error) {
	if req.MaxAttempts == 0 {
		req.MaxAttempts = 5
	}

	resp, err := c.doRequest(ctx, "POST", "/regex", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RegexResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GenerateJSON generates JSON following a schema.
func (c *Client) GenerateJSON(ctx context.Context, req *JSONSchemaRequest) (*JSONSchemaResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/json_schema", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result JSONSchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GenerateEmail generates a valid email address.
func (c *Client) GenerateEmail(ctx context.Context, prompt string) (string, error) {
	result, err := c.GenerateWithRegex(ctx, &RegexRequest{
		Prompt:  prompt,
		Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
	})
	if err != nil {
		return "", err
	}
	if result.Matches {
		return result.Text, nil
	}
	return "", fmt.Errorf("could not generate valid email")
}

// GeneratePhoneNumber generates a valid phone number.
func (c *Client) GeneratePhoneNumber(ctx context.Context, prompt string) (string, error) {
	result, err := c.GenerateWithRegex(ctx, &RegexRequest{
		Prompt:  prompt,
		Pattern: `\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}`,
	})
	if err != nil {
		return "", err
	}
	if result.Matches {
		return result.Text, nil
	}
	return "", fmt.Errorf("could not generate valid phone number")
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
