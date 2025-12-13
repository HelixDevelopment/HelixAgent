package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents an HTTP client with retry logic
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	retryCount int
	timeout    time.Duration
}

// NewClient creates a new HTTP client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:    baseURL,
		apiKey:     apiKey,
		retryCount: 3,
		timeout:    30 * time.Second,
	}
}

// SetTimeout sets the request timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.httpClient.Timeout = timeout
}

// SetRetryCount sets the number of retries for failed requests
func (c *Client) SetRetryCount(count int) {
	c.retryCount = count
}

// Do performs an HTTP request with retry logic
func (c *Client) Do(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryCount; attempt++ {
		resp, err := c.doRequest(ctx, method, path, body, headers)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		lastErr = err

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Don't retry on client errors (4xx)
		if err == nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return resp, fmt.Errorf("client error: %d", resp.StatusCode)
		}

		// Wait before retrying (exponential backoff)
		if attempt < c.retryCount {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryCount+1, lastErr)
}

// doRequest performs a single HTTP request
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "HelixAgent/1.0")

	// Set API key if provided
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, "GET", path, nil, headers)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, "POST", path, body, headers)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, "PUT", path, body, headers)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string, headers map[string]string) (*http.Response, error) {
	return c.Do(ctx, "DELETE", path, nil, headers)
}

// DoRequest performs a request and unmarshals the response into the result
func (c *Client) DoRequest(ctx context.Context, method, path string, payload interface{}, result interface{}) error {
	resp, err := c.Do(ctx, method, path, payload, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
