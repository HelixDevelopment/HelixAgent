// Package api provides Anthropic API client implementations for Claude Code integration.
// This package implements all API endpoints documented in the Claude Code source analysis.
package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Base URLs for different environments
const (
	ProductionBaseURL = "https://api.anthropic.com"
	StagingBaseURL    = "https://api-staging.anthropic.com"
	MCPProxyBaseURL   = "https://mcp-proxy.anthropic.com/v1/mcp"
)

// Beta headers used by Claude Code
const (
	BetaOAuth               = "oauth-2025-04-20"
	BetaInterleavedThinking = "interleaved-thinking-2025-05-14"
	BetaContext1M           = "context-1m-2025-08-07"
	BetaStructuredOutputs   = "structured-outputs-2025-12-15"
	BetaWebSearch           = "web-search-2025-03-05"
	BetaFastMode            = "fast-mode-2026-02-01"
	BetaAFKMode             = "afk-mode-2026-01-31"
	BetaRedactThinking      = "redact-thinking-2026-02-12"
	BetaFilesAPI            = "files-api-2025-04-14"
)

// Client represents an Anthropic API client
type Client struct {
	baseURL          string
	httpClient       *http.Client
	apiKey           string
	oauthToken       string
	anthropicVersion string
	betaHeaders      []string
}

// ClientOption configures the Client
type ClientOption func(*Client)

// NewClient creates a new Anthropic API client
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL:          ProductionBaseURL,
		httpClient:       &http.Client{Timeout: 120 * time.Second},
		anthropicVersion: "2023-06-01",
		betaHeaders:      []string{BetaOAuth},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithBaseURL sets the base URL
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithAPIKey sets the API key for authentication
func WithAPIKey(key string) ClientOption {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithOAuthToken sets the OAuth token for authentication
func WithOAuthToken(token string) ClientOption {
	return func(c *Client) {
		c.oauthToken = token
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithBetaHeaders adds beta feature headers
func WithBetaHeaders(headers ...string) ClientOption {
	return func(c *Client) {
		c.betaHeaders = append(c.betaHeaders, headers...)
	}
}

// setAuthHeaders sets the authentication headers on the request
func (c *Client) setAuthHeaders(req *http.Header) {
	req.Set("Content-Type", "application/json")
	req.Set("anthropic-version", c.anthropicVersion)

	if c.oauthToken != "" {
		req.Set("Authorization", "Bearer "+c.oauthToken)
	} else if c.apiKey != "" {
		req.Set("x-api-key", c.apiKey)
	}

	if len(c.betaHeaders) > 0 {
		req.Set("anthropic-beta", strings.Join(c.betaHeaders, ","))
	}

	// Claude Code specific headers
	req.Set("User-Agent", "Claude-Code/1.0.0")
	req.Set("x-app", "cli")
	req.Set("x-client-request-id", generateRequestID())
}

// doRequest performs an HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setAuthHeaders(&req.Header)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	return resp, nil
}

// handleErrorResponse handles API error responses
func handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return &apiErr
}

// APIError represents an Anthropic API error
type APIError struct {
	Type string       `json:"type"`
	Err  APIErrDetail `json:"error"`
}

// APIErrDetail holds error details
type APIErrDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Err.Type, e.Err.Message)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// IsRateLimitError checks if the error is a rate limit error
func IsRateLimitError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Err.Type == "rate_limit_error"
	}
	return false
}

// IsAuthenticationError checks if the error is an authentication error
func IsAuthenticationError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Err.Type == "authentication_error"
	}
	return false
}

// EventStreamReader handles SSE (Server-Sent Events) streams
type EventStreamReader struct {
	reader *bufio.Reader
}

// NewEventStreamReader creates a new event stream reader
func NewEventStreamReader(r io.Reader) *EventStreamReader {
	return &EventStreamReader{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent reads the next event from the stream
func (r *EventStreamReader) ReadEvent() (string, []byte, error) {
	var eventType string
	var data []byte

	for {
		line, err := r.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && len(data) > 0 {
				return eventType, data, nil
			}
			return "", nil, err
		}

		line = bytes.TrimSpace(line)

		if len(line) == 0 {
			// Empty line marks end of event
			if len(data) > 0 {
				return eventType, data, nil
			}
			continue
		}

		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(line[6:]))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			data = append(data, line[5:]...)
		}
	}
}
