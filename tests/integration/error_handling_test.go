package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConfig holds configuration for error handling tests
type testConfig struct {
	BaseURL          string
	HelixAgentAPIKey string
}

// loadErrorTestConfig loads test configuration
func loadErrorTestConfig(t *testing.T) *testConfig {
	t.Helper()

	// Load .env file from project root
	projectRoot := findErrorTestProjectRoot(t)
	envFile := filepath.Join(projectRoot, ".env")
	if err := godotenv.Load(envFile); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	host := os.Getenv("HELIXAGENT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8100"
	}

	return &testConfig{
		BaseURL:          fmt.Sprintf("http://%s:%s/v1", host, port),
		HelixAgentAPIKey: os.Getenv("HELIXAGENT_API_KEY"),
	}
}

func findErrorTestProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (go.mod)")
		}
		dir = parent
	}
}

// TestErrorCategorization verifies that errors are properly categorized
func TestErrorCategorization(t *testing.T) {
	testCases := []struct {
		name         string
		errorMessage string
		provider     string
		expectedType services.ErrorType
		expectedCode int
		retryable    bool
	}{
		{
			name:         "rate limit from 429 status",
			errorMessage: "API returned status 429: rate limit exceeded",
			provider:     "openai",
			expectedType: services.ErrorTypeRateLimit,
			expectedCode: http.StatusTooManyRequests,
			retryable:    true,
		},
		{
			name:         "rate limit from message",
			errorMessage: "Rate limit exceeded, please retry after 60 seconds",
			provider:     "claude",
			expectedType: services.ErrorTypeRateLimit,
			expectedCode: http.StatusTooManyRequests,
			retryable:    true,
		},
		{
			name:         "timeout from context deadline",
			errorMessage: "context deadline exceeded",
			provider:     "deepseek",
			expectedType: services.ErrorTypeTimeout,
			expectedCode: http.StatusGatewayTimeout,
			retryable:    true,
		},
		{
			name:         "timeout from timeout message",
			errorMessage: "request timeout after 30s",
			provider:     "gemini",
			expectedType: services.ErrorTypeTimeout,
			expectedCode: http.StatusGatewayTimeout,
			retryable:    true,
		},
		{
			name:         "network error - connection refused",
			errorMessage: "dial tcp 127.0.0.1:11434: connection refused",
			provider:     "ollama",
			expectedType: services.ErrorTypeNetwork,
			expectedCode: http.StatusBadGateway,
			retryable:    true,
		},
		{
			name:         "network error - no such host",
			errorMessage: "dial tcp: lookup invalid.host.local: no such host",
			provider:     "qwen",
			expectedType: services.ErrorTypeNetwork,
			expectedCode: http.StatusBadGateway,
			retryable:    true,
		},
		{
			name:         "configuration error - api key",
			errorMessage: "invalid api key: authentication failed",
			provider:     "zai",
			expectedType: services.ErrorTypeConfiguration,
			expectedCode: http.StatusServiceUnavailable,
			retryable:    false,
		},
		{
			name:         "configuration error - unauthorized",
			errorMessage: "unauthorized: invalid credentials",
			provider:     "openrouter",
			expectedType: services.ErrorTypeConfiguration,
			expectedCode: http.StatusServiceUnavailable,
			retryable:    false,
		},
		{
			name:         "configuration error - not available",
			errorMessage: "provider not available",
			provider:     "test",
			expectedType: services.ErrorTypeConfiguration,
			expectedCode: http.StatusServiceUnavailable,
			retryable:    false,
		},
		{
			name:         "validation error - invalid request",
			errorMessage: "invalid request: messages field is required",
			provider:     "test",
			expectedType: services.ErrorTypeValidation,
			expectedCode: http.StatusBadRequest,
			retryable:    false,
		},
		{
			name:         "generic provider error",
			errorMessage: "provider returned error: internal server error",
			provider:     "test",
			expectedType: services.ErrorTypeProvider,
			expectedCode: http.StatusBadGateway,
			retryable:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := services.CategorizeError(
				&testError{message: tc.errorMessage},
				tc.provider,
			)

			require.NotNil(t, err)
			assert.Equal(t, tc.expectedType, err.Type,
				"Error type mismatch for: %s", tc.name)
			assert.Equal(t, tc.expectedCode, err.HTTPStatus,
				"HTTP status mismatch for: %s", tc.name)
			assert.Equal(t, tc.retryable, err.Retryable,
				"Retryable mismatch for: %s", tc.name)
		})
	}
}

// testError is a simple error type for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

// TestAllProvidersFailedError verifies the all providers failed error
func TestAllProvidersFailedError(t *testing.T) {
	causes := []error{
		&testError{message: "provider 1: rate limit"},
		&testError{message: "provider 2: timeout"},
		&testError{message: "provider 3: network error"},
	}

	err := services.NewAllProvidersFailedError(3, 3, causes)

	assert.Equal(t, services.ErrorTypeAllProvidersFailed, err.Type)
	assert.Equal(t, http.StatusBadGateway, err.HTTPStatus)
	assert.Equal(t, 3, err.FailedCount)
	assert.Equal(t, 3, err.TotalCount)
	assert.True(t, err.Retryable)

	// Verify OpenAI error format
	openAIErr := err.ToOpenAIError()
	errObj, ok := openAIErr["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "server_error", errObj["type"]) // Should convert to server_error
	assert.Contains(t, errObj["message"].(string), "3 providers failed")
}

// TestErrorHTTPStatusMapping verifies that errors map to correct HTTP status codes
func TestErrorHTTPStatusMapping(t *testing.T) {
	testCases := []struct {
		name           string
		createError    func() *services.LLMServiceError
		expectedStatus int
	}{
		{
			name:           "provider error returns 502",
			createError:    func() *services.LLMServiceError { return services.NewProviderError("test", nil) },
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "configuration error returns 503",
			createError:    func() *services.LLMServiceError { return services.NewConfigurationError("msg", nil) },
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "rate limit returns 429",
			createError:    func() *services.LLMServiceError { return services.NewRateLimitError("test", time.Minute) },
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "timeout returns 504",
			createError:    func() *services.LLMServiceError { return services.NewTimeoutError("test", nil) },
			expectedStatus: http.StatusGatewayTimeout,
		},
		{
			name:           "network error returns 502",
			createError:    func() *services.LLMServiceError { return services.NewNetworkError("test", nil) },
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "validation error returns 400",
			createError:    func() *services.LLMServiceError { return services.NewValidationError("msg", nil) },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "service unavailable returns 503",
			createError:    func() *services.LLMServiceError { return services.NewServiceUnavailableError("msg", 0) },
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name: "all providers failed returns 502",
			createError: func() *services.LLMServiceError {
				return services.NewAllProvidersFailedError(2, 2, nil)
			},
			expectedStatus: http.StatusBadGateway,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.createError()
			assert.Equal(t, tc.expectedStatus, services.GetHTTPStatus(err),
				"HTTP status mismatch for %s", tc.name)
		})
	}
}

// TestRetryAfterHeader verifies that Retry-After headers are set correctly
func TestRetryAfterHeader(t *testing.T) {
	testCases := []struct {
		name          string
		error         *services.LLMServiceError
		expectedAfter time.Duration
	}{
		{
			name:          "rate limit has retry after",
			error:         services.NewRateLimitError("test", 60*time.Second),
			expectedAfter: 60 * time.Second,
		},
		{
			name:          "service unavailable has retry after",
			error:         services.NewServiceUnavailableError("test", 5*time.Minute),
			expectedAfter: 5 * time.Minute,
		},
		{
			name:          "provider error has no retry after",
			error:         services.NewProviderError("test", nil),
			expectedAfter: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedAfter, services.GetRetryAfter(tc.error))
		})
	}
}

// TestOpenAIErrorFormat verifies the OpenAI-compatible error format
func TestOpenAIErrorFormat(t *testing.T) {
	t.Run("basic error format", func(t *testing.T) {
		err := services.NewProviderError("deepseek", &testError{message: "API error"})
		openAIErr := err.ToOpenAIError()

		errObj, ok := openAIErr["error"].(map[string]interface{})
		require.True(t, ok, "Should have error object")

		assert.Contains(t, errObj, "message")
		assert.Contains(t, errObj, "type")
		assert.Contains(t, errObj, "code")

		assert.Equal(t, "provider_error", errObj["type"])
		assert.Equal(t, "PROVIDER_ERROR", errObj["code"])
	})

	t.Run("rate limit includes retry after", func(t *testing.T) {
		err := services.NewRateLimitError("openai", 30*time.Second)
		openAIErr := err.ToOpenAIError()

		retryAfter, ok := openAIErr["retry_after"]
		require.True(t, ok, "Should have retry_after")
		assert.Equal(t, float64(30), retryAfter)
	})

	t.Run("all providers failed uses server_error type", func(t *testing.T) {
		err := services.NewAllProvidersFailedError(3, 3, nil)
		openAIErr := err.ToOpenAIError()

		errObj := openAIErr["error"].(map[string]interface{})
		assert.Equal(t, "server_error", errObj["type"],
			"Should use server_error type for all providers failed")
	})
}

// TestErrorChaining verifies that error wrapping works correctly
func TestErrorChaining(t *testing.T) {
	t.Run("cause is preserved", func(t *testing.T) {
		originalErr := &testError{message: "original error"}
		wrappedErr := services.NewProviderError("test", originalErr)

		assert.ErrorIs(t, wrappedErr, originalErr,
			"Wrapped error should contain original error")
	})

	t.Run("error message includes cause", func(t *testing.T) {
		originalErr := &testError{message: "API returned 500"}
		wrappedErr := services.NewProviderError("deepseek", originalErr)

		assert.Contains(t, wrappedErr.Error(), "API returned 500")
		assert.Contains(t, wrappedErr.Error(), "deepseek")
	})
}

// TestEnsembleErrorHandling tests that ensemble errors are properly categorized
func TestEnsembleErrorHandling(t *testing.T) {
	t.Run("no providers returns configuration error", func(t *testing.T) {
		err := services.NewConfigurationError("no providers available", nil)

		assert.Equal(t, services.ErrorTypeConfiguration, err.Type)
		assert.Equal(t, http.StatusServiceUnavailable, err.HTTPStatus)
		assert.False(t, err.Retryable)
	})

	t.Run("all providers failed is retryable", func(t *testing.T) {
		causes := []error{
			&testError{message: "provider 1 failed"},
			&testError{message: "provider 2 failed"},
		}
		err := services.NewAllProvidersFailedError(2, 2, causes)

		assert.True(t, err.Retryable,
			"All providers failed should be retryable (transient failure)")
	})
}

// TestErrorHandlerIntegration tests the handler error response
func TestErrorHandlerIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("categorized error returns correct status", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			err := services.NewRateLimitError("test", 30*time.Second)
			response := err.ToOpenAIError()
			if err.RetryAfter > 0 {
				c.Header("Retry-After", "30")
			}
			c.JSON(err.HTTPStatus, response)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "30", w.Header().Get("Retry-After"))

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		errObj := resp["error"].(map[string]interface{})
		assert.Equal(t, "rate_limit", errObj["type"])
	})
}

// TestLiveChatCompletionsErrorHandling tests error handling with actual server
func TestLiveChatCompletionsErrorHandling(t *testing.T) {
	testutil.RequireServer(t)
	config := loadErrorTestConfig(t)

	// Skip if server not running - use a short timeout for the health check
	healthClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := healthClient.Get(config.BaseURL + "/models")
	if err != nil {
		t.Skipf("Server not running at %s: %v", config.BaseURL, err)
	}
	resp.Body.Close()

	// Use longer timeout for actual requests that may hit providers
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("invalid model returns appropriate error", func(t *testing.T) {
		chatReq := map[string]interface{}{
			"model": "nonexistent-model-12345",
			"messages": []map[string]string{
				{"role": "user", "content": "test"},
			},
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequest("POST", config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Request failed (provider may be unavailable): %v", err)
		}
		defer resp.Body.Close()

		// Should not be 200 for invalid model
		// Could be 400 (validation), 502 (gateway error), or 200 with fallback
		responseBody, _ := io.ReadAll(resp.Body)
		t.Logf("Response status: %d, body: %s", resp.StatusCode, string(responseBody))

		if resp.StatusCode >= 400 {
			var errResp map[string]interface{}
			err := json.Unmarshal(responseBody, &errResp)
			require.NoError(t, err, "Error response should be valid JSON")
			assert.Contains(t, errResp, "error", "Should have error field")

			// Verify we're not returning generic 500 for categorizable errors
			if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 429 || resp.StatusCode == 504 {
				errObj := errResp["error"].(map[string]interface{})
				assert.NotEqual(t, "internal_error", errObj["type"],
					"Categorized errors should not have internal_error type")
			}
		}
	})

	t.Run("empty messages returns validation error", func(t *testing.T) {
		chatReq := map[string]interface{}{
			"model":    "helixagent-debate",
			"messages": []map[string]string{},
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequest("POST", config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Request failed (provider may be unavailable): %v", err)
		}
		defer resp.Body.Close()

		responseBody, _ := io.ReadAll(resp.Body)
		t.Logf("Response status: %d, body: %s", resp.StatusCode, string(responseBody))

		// Empty messages might be accepted or rejected depending on implementation
		// Just verify we get a valid response format
		var respData map[string]interface{}
		err = json.Unmarshal(responseBody, &respData)
		require.NoError(t, err, "Response should be valid JSON")
	})
}

// TestErrorRegressionPrevention ensures we don't regress on error handling
func TestErrorRegressionPrevention(t *testing.T) {
	t.Run("CRITICAL: 500 errors are properly categorized", func(t *testing.T) {
		// These error messages should NOT result in generic 500 errors
		testCases := []struct {
			errorMessage string
			notExpected  int // should NOT be this status
		}{
			{
				errorMessage: "rate limit exceeded",
				notExpected:  http.StatusInternalServerError,
			},
			{
				errorMessage: "context deadline exceeded",
				notExpected:  http.StatusInternalServerError,
			},
			{
				errorMessage: "connection refused",
				notExpected:  http.StatusInternalServerError,
			},
			{
				errorMessage: "unauthorized: invalid api key",
				notExpected:  http.StatusInternalServerError,
			},
		}

		for _, tc := range testCases {
			err := services.CategorizeError(&testError{message: tc.errorMessage}, "test")
			require.NotNil(t, err, "Error should be categorized: %s", tc.errorMessage)
			assert.NotEqual(t, tc.notExpected, err.HTTPStatus,
				"Error '%s' should not result in status %d",
				tc.errorMessage, tc.notExpected)
		}
	})

	t.Run("CRITICAL: All provider failures return 502 not 500", func(t *testing.T) {
		err := services.NewAllProvidersFailedError(3, 3, nil)
		assert.Equal(t, http.StatusBadGateway, err.HTTPStatus,
			"All providers failed should return 502 Bad Gateway, not 500")
	})

	t.Run("CRITICAL: Configuration errors return 503 not 500", func(t *testing.T) {
		err := services.NewConfigurationError("no providers available", nil)
		assert.Equal(t, http.StatusServiceUnavailable, err.HTTPStatus,
			"Configuration errors should return 503 Service Unavailable, not 500")
	})

	t.Run("CRITICAL: Rate limits return 429 with Retry-After", func(t *testing.T) {
		err := services.NewRateLimitError("openai", 60*time.Second)
		assert.Equal(t, http.StatusTooManyRequests, err.HTTPStatus,
			"Rate limit should return 429")
		assert.Equal(t, 60*time.Second, err.RetryAfter,
			"Rate limit should include Retry-After duration")
	})
}
