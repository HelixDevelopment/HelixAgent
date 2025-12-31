package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockResponse creates a mock HTTP response with a body
func mockResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
	assert.Equal(t, 0.1, config.JitterFactor)
}

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"200 OK", http.StatusOK, false},
		{"201 Created", http.StatusCreated, false},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableStatusCode(tt.statusCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, false},
		{"context deadline exceeded", context.DeadlineExceeded, false},
		{"generic error", errors.New("network error"), true},
		{"wrapped error", errors.New("connection refused"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecuteWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	attemptCount := 0
	fn := func() (*http.Response, error) {
		attemptCount++
		return mockResponse(http.StatusOK), nil
	}

	result, err := ExecuteWithRetry(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.Attempts)
	assert.Equal(t, http.StatusOK, result.Response.StatusCode)
	assert.Equal(t, 1, attemptCount)
}

func TestExecuteWithRetry_SuccessAfterRetries(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	attemptCount := int32(0)
	fn := func() (*http.Response, error) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count < 3 {
			return mockResponse(http.StatusServiceUnavailable), nil
		}
		return mockResponse(http.StatusOK), nil
	}

	result, err := ExecuteWithRetry(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, result.Attempts)
	assert.Equal(t, http.StatusOK, result.Response.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attemptCount))
}

func TestExecuteWithRetry_MaxRetriesExceeded(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	attemptCount := int32(0)
	fn := func() (*http.Response, error) {
		atomic.AddInt32(&attemptCount, 1)
		return mockResponse(http.StatusServiceUnavailable), nil
	}

	result, err := ExecuteWithRetry(context.Background(), config, fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 attempts failed")
	assert.Equal(t, 3, result.Attempts) // 1 initial + 2 retries
	assert.Equal(t, int32(3), atomic.LoadInt32(&attemptCount))
}

func TestExecuteWithRetry_NetworkError(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	attemptCount := int32(0)
	fn := func() (*http.Response, error) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count < 3 {
			return nil, errors.New("connection refused")
		}
		return mockResponse(http.StatusOK), nil
	}

	result, err := ExecuteWithRetry(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.Equal(t, 3, result.Attempts)
	assert.Equal(t, http.StatusOK, result.Response.StatusCode)
}

func TestExecuteWithRetry_NonRetryableError(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	attemptCount := int32(0)
	fn := func() (*http.Response, error) {
		atomic.AddInt32(&attemptCount, 1)
		// Return a non-retryable status code
		return mockResponse(http.StatusBadRequest), nil
	}

	result, err := ExecuteWithRetry(context.Background(), config, fn)

	assert.NoError(t, err)
	assert.Equal(t, 1, result.Attempts) // Should not retry
	assert.Equal(t, http.StatusBadRequest, result.Response.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attemptCount))
}

func TestExecuteWithRetry_ContextCancellation(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	attemptCount := int32(0)

	fn := func() (*http.Response, error) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count == 2 {
			cancel() // Cancel after second attempt
		}
		return mockResponse(http.StatusServiceUnavailable), nil
	}

	result, err := ExecuteWithRetry(ctx, config, fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
	assert.LessOrEqual(t, result.Attempts, 3) // Should stop early due to cancellation
}

func TestExecuteWithRetry_ContextCancelledBeforeStart(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before starting

	fn := func() (*http.Response, error) {
		return mockResponse(http.StatusOK), nil
	}

	result, err := ExecuteWithRetry(ctx, config, fn)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled before attempt")
	assert.Equal(t, 1, result.Attempts)
}

func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0, // No jitter for predictable testing
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second}, // Capped at MaxDelay
		{7, 30 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		t.Run("attempt_"+string(rune('0'+tt.attempt)), func(t *testing.T) {
			result := CalculateBackoff(tt.attempt, config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryableHTTPClient_Do(t *testing.T) {
	attemptCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	client := NewRetryableHTTPClient(nil, config)

	req, err := http.NewRequest("GET", server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attemptCount))
	resp.Body.Close()
}

func TestRetryableHTTPClient_RateLimitRetry(t *testing.T) {
	attemptCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	client := NewRetryableHTTPClient(nil, config)

	req, err := http.NewRequest("GET", server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attemptCount))
	resp.Body.Close()
}

func TestNewRetryableHTTPClient_NilClient(t *testing.T) {
	config := DefaultRetryConfig()
	client := NewRetryableHTTPClient(nil, config)

	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.Equal(t, 60*time.Second, client.client.Timeout)
}

func TestNewRetryableHTTPClient_CustomClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	config := DefaultRetryConfig()
	client := NewRetryableHTTPClient(customClient, config)

	assert.NotNil(t, client)
	assert.Equal(t, customClient, client.client)
	assert.Equal(t, 30*time.Second, client.client.Timeout)
}

func TestRetryableHTTPClient_GetConfig(t *testing.T) {
	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
		JitterFactor: 0.2,
	}

	client := NewRetryableHTTPClient(nil, config)
	retrievedConfig := client.GetConfig()

	assert.Equal(t, config.MaxRetries, retrievedConfig.MaxRetries)
	assert.Equal(t, config.InitialDelay, retrievedConfig.InitialDelay)
	assert.Equal(t, config.MaxDelay, retrievedConfig.MaxDelay)
	assert.Equal(t, config.Multiplier, retrievedConfig.Multiplier)
	assert.Equal(t, config.JitterFactor, retrievedConfig.JitterFactor)
}
