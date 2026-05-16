package modelsdev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultBaseURL   = "https://api.models.dev/v1"
	DefaultTimeout   = 30 * time.Second
	DefaultUserAgent = "helix_agent/1.0"
	RateLimitRetries = 3
	RateLimitBackoff = 1 * time.Second
)

type Client struct {
	httpClient  *http.Client
	baseURL     string
	apiKey      string
	userAgent   string
	rateLimiter *RateLimiter
}

type ClientConfig struct {
	BaseURL   string
	APIKey    string
	Timeout   time.Duration
	UserAgent string
}

func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = &ClientConfig{}
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		baseURL:     baseURL,
		apiKey:      config.APIKey,
		userAgent:   userAgent,
		rateLimiter: NewRateLimiter(100, time.Minute),
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, result interface{}) error {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	if err = c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil {
			return &apiErr
		}
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) doGet(ctx context.Context, path string, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, result)
}

func (c *Client) doPost(ctx context.Context, path string, body interface{}, result interface{}) error { //nolint:unused
	bodyReader, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	return c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(bodyReader), result)
}
