// Package http provides a generic HTTP client with retry logic, rate limiting, and interceptors.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit/common/ratelimit"
)

// RequestInterceptor is a function that can modify a request before it's sent.
type RequestInterceptor func(req *http.Request) error

// ResponseInterceptor is a function that can process a response after it's received.
type ResponseInterceptor func(resp *http.Response) error

// Client represents a generic HTTP client with advanced features.
type Client struct {
	baseURL              string
	httpClient           *http.Client
	rateLimiter          *ratelimit.TokenBucket
	requestInterceptors  []RequestInterceptor
	responseInterceptors []ResponseInterceptor
	authHeader           string
	authValue            string
	maxRetries           int
	baseBackoff          time.Duration
}

// ClientConfig holds configuration for the HTTP client.
type ClientConfig struct {
	BaseURL     string
	Timeout     time.Duration
	RateLimit   *ratelimit.TokenBucketConfig
	MaxRetries  int
	BaseBackoff time.Duration
}

// NewClient creates a new HTTP client with the given configuration.
func NewClient(config ClientConfig) *Client {
	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	var rateLimiter *ratelimit.TokenBucket
	if config.RateLimit != nil {
		rateLimiter = ratelimit.NewTokenBucket(*config.RateLimit)
	}

	maxRetries := config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	baseBackoff := config.BaseBackoff
	if baseBackoff == 0 {
		baseBackoff = time.Second
	}

	return &Client{
		baseURL:              config.BaseURL,
		httpClient:           httpClient,
		rateLimiter:          rateLimiter,
		requestInterceptors:  []RequestInterceptor{},
		responseInterceptors: []ResponseInterceptor{},
		maxRetries:           maxRetries,
		baseBackoff:          baseBackoff,
	}
}

// SetAuth sets the authentication header.
func (c *Client) SetAuth(header, value string) {
	c.authHeader = header
	c.authValue = value
}

// AddRequestInterceptor adds a request interceptor.
func (c *Client) AddRequestInterceptor(interceptor RequestInterceptor) {
	c.requestInterceptors = append(c.requestInterceptors, interceptor)
}

// AddResponseInterceptor adds a response interceptor.
func (c *Client) AddResponseInterceptor(interceptor ResponseInterceptor) {
	c.responseInterceptors = append(c.responseInterceptors, interceptor)
}

// DoRequest performs an HTTP request with retry logic and interceptors.
func (c *Client) DoRequest(ctx context.Context, method, endpoint string, payload interface{}, result interface{}) error {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + endpoint

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if c.rateLimiter != nil {
			if err := c.rateLimiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limit error: %w", err)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set auth header if configured
		if c.authHeader != "" && c.authValue != "" {
			req.Header.Set(c.authHeader, c.authValue)
		}

		// Set content type for JSON payloads
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		// Apply request interceptors
		for _, interceptor := range c.requestInterceptors {
			if err := interceptor(req); err != nil {
				return fmt.Errorf("request interceptor error: %w", err)
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to make request: %w", err)
			if attempt < c.maxRetries {
				time.Sleep(c.baseBackoff * time.Duration(1<<attempt))
				continue
			}
			return lastErr
		}

		// Apply response interceptors
		for _, interceptor := range c.responseInterceptors {
			if err := interceptor(resp); err != nil {
				resp.Body.Close()
				return fmt.Errorf("response interceptor error: %w", err)
			}
		}

		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if result != nil {
				if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
					return fmt.Errorf("failed to decode response: %w", err)
				}
			}
			return nil
		}

		// Handle error responses
		bodyBytes, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))

		// Retry on server errors or rate limits
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			if attempt < c.maxRetries {
				backoff := c.baseBackoff * time.Duration(1<<attempt)
				if resp.StatusCode == 429 {
					// For rate limits, use longer backoff
					backoff *= 2
				}
				time.Sleep(backoff)
				continue
			}
		}

		return lastErr
	}

	return lastErr
}
