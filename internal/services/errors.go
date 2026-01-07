package services

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrorType represents the category of an LLM service error
type ErrorType string

const (
	// ErrorTypeProvider indicates a provider-specific failure
	ErrorTypeProvider ErrorType = "provider_error"
	// ErrorTypeConfiguration indicates a configuration/setup issue
	ErrorTypeConfiguration ErrorType = "configuration_error"
	// ErrorTypeRateLimit indicates rate limiting
	ErrorTypeRateLimit ErrorType = "rate_limit"
	// ErrorTypeTimeout indicates a timeout occurred
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeNetwork indicates a network connectivity issue
	ErrorTypeNetwork ErrorType = "network_error"
	// ErrorTypeValidation indicates invalid request parameters
	ErrorTypeValidation ErrorType = "validation_error"
	// ErrorTypeServiceUnavailable indicates the service is temporarily unavailable
	ErrorTypeServiceUnavailable ErrorType = "service_unavailable"
	// ErrorTypeAllProvidersFailed indicates all providers in ensemble failed
	ErrorTypeAllProvidersFailed ErrorType = "all_providers_failed"
)

// LLMServiceError represents a categorized error from LLM services
type LLMServiceError struct {
	Type        ErrorType
	Message     string
	Code        string
	HTTPStatus  int
	Cause       error
	Provider    string
	RetryAfter  time.Duration
	Retryable   bool
	FailedCount int
	TotalCount  int
	Details     map[string]interface{}
}

func (e *LLMServiceError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s", e.Type, e.Message))
	if e.Provider != "" {
		sb.WriteString(fmt.Sprintf(" (provider: %s)", e.Provider))
	}
	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf(": %v", e.Cause))
	}
	return sb.String()
}

func (e *LLMServiceError) Unwrap() error {
	return e.Cause
}

// Is implements errors.Is for LLMServiceError
func (e *LLMServiceError) Is(target error) bool {
	if t, ok := target.(*LLMServiceError); ok {
		return e.Type == t.Type
	}
	return false
}

// NewProviderError creates an error for provider-specific failures
func NewProviderError(provider string, cause error) *LLMServiceError {
	return &LLMServiceError{
		Type:       ErrorTypeProvider,
		Message:    fmt.Sprintf("Provider %s failed", provider),
		Code:       "PROVIDER_ERROR",
		HTTPStatus: http.StatusBadGateway,
		Cause:      cause,
		Provider:   provider,
		Retryable:  true,
	}
}

// NewConfigurationError creates an error for configuration issues
func NewConfigurationError(message string, cause error) *LLMServiceError {
	return &LLMServiceError{
		Type:       ErrorTypeConfiguration,
		Message:    message,
		Code:       "CONFIGURATION_ERROR",
		HTTPStatus: http.StatusServiceUnavailable,
		Cause:      cause,
		Retryable:  false,
	}
}

// NewRateLimitError creates an error for rate limiting
func NewRateLimitError(provider string, retryAfter time.Duration) *LLMServiceError {
	return &LLMServiceError{
		Type:       ErrorTypeRateLimit,
		Message:    fmt.Sprintf("Rate limit exceeded for provider %s", provider),
		Code:       "RATE_LIMIT_EXCEEDED",
		HTTPStatus: http.StatusTooManyRequests,
		Provider:   provider,
		RetryAfter: retryAfter,
		Retryable:  true,
	}
}

// NewTimeoutError creates an error for timeout conditions
func NewTimeoutError(provider string, cause error) *LLMServiceError {
	return &LLMServiceError{
		Type:       ErrorTypeTimeout,
		Message:    fmt.Sprintf("Request to provider %s timed out", provider),
		Code:       "TIMEOUT",
		HTTPStatus: http.StatusGatewayTimeout,
		Cause:      cause,
		Provider:   provider,
		Retryable:  true,
	}
}

// NewNetworkError creates an error for network issues
func NewNetworkError(provider string, cause error) *LLMServiceError {
	return &LLMServiceError{
		Type:       ErrorTypeNetwork,
		Message:    fmt.Sprintf("Network error connecting to provider %s", provider),
		Code:       "NETWORK_ERROR",
		HTTPStatus: http.StatusBadGateway,
		Cause:      cause,
		Provider:   provider,
		Retryable:  true,
	}
}

// NewValidationError creates an error for invalid requests
func NewValidationError(message string, details map[string]interface{}) *LLMServiceError {
	return &LLMServiceError{
		Type:       ErrorTypeValidation,
		Message:    message,
		Code:       "VALIDATION_ERROR",
		HTTPStatus: http.StatusBadRequest,
		Retryable:  false,
		Details:    details,
	}
}

// NewServiceUnavailableError creates an error for temporary service unavailability
func NewServiceUnavailableError(message string, retryAfter time.Duration) *LLMServiceError {
	return &LLMServiceError{
		Type:       ErrorTypeServiceUnavailable,
		Message:    message,
		Code:       "SERVICE_UNAVAILABLE",
		HTTPStatus: http.StatusServiceUnavailable,
		RetryAfter: retryAfter,
		Retryable:  true,
	}
}

// NewAllProvidersFailedError creates an error when all providers in ensemble fail
func NewAllProvidersFailedError(failedCount, totalCount int, causes []error) *LLMServiceError {
	var causeMessages []string
	for _, c := range causes {
		if c != nil {
			causeMessages = append(causeMessages, c.Error())
		}
	}

	return &LLMServiceError{
		Type:        ErrorTypeAllProvidersFailed,
		Message:     fmt.Sprintf("All %d providers failed", failedCount),
		Code:        "ALL_PROVIDERS_FAILED",
		HTTPStatus:  http.StatusBadGateway,
		Retryable:   true,
		FailedCount: failedCount,
		TotalCount:  totalCount,
		Details: map[string]interface{}{
			"failed_count": failedCount,
			"total_count":  totalCount,
			"errors":       causeMessages,
		},
	}
}

// CategorizeError analyzes an error and wraps it in the appropriate LLMServiceError type
func CategorizeError(err error, provider string) *LLMServiceError {
	if err == nil {
		return nil
	}

	// Check if already categorized
	var llmErr *LLMServiceError
	if errors.As(err, &llmErr) {
		return llmErr
	}

	errStr := strings.ToLower(err.Error())

	// Check for rate limiting
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "429") {
		return NewRateLimitError(provider, 60*time.Second)
	}

	// Check for timeout
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline") {
		return NewTimeoutError(provider, err)
	}

	// Check for network errors (including EOF, connection reset, etc.)
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "connection timed out") {
		return NewNetworkError(provider, err)
	}

	// Check for configuration errors
	if strings.Contains(errStr, "not configured") ||
		strings.Contains(errStr, "not available") ||
		strings.Contains(errStr, "api key") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") {
		return NewConfigurationError(err.Error(), err)
	}

	// Check for validation errors
	if strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "malformed") ||
		strings.Contains(errStr, "bad request") {
		return NewValidationError(err.Error(), nil)
	}

	// Default to provider error
	return NewProviderError(provider, err)
}

// IsRetryable returns true if the error is retryable
func IsRetryable(err error) bool {
	var llmErr *LLMServiceError
	if errors.As(err, &llmErr) {
		return llmErr.Retryable
	}
	return false
}

// GetHTTPStatus returns the appropriate HTTP status code for the error
func GetHTTPStatus(err error) int {
	var llmErr *LLMServiceError
	if errors.As(err, &llmErr) {
		return llmErr.HTTPStatus
	}
	return http.StatusInternalServerError
}

// GetRetryAfter returns the retry-after duration if applicable
func GetRetryAfter(err error) time.Duration {
	var llmErr *LLMServiceError
	if errors.As(err, &llmErr) {
		return llmErr.RetryAfter
	}
	return 0
}

// ToOpenAIError converts an LLMServiceError to OpenAI-compatible error format
func (e *LLMServiceError) ToOpenAIError() map[string]interface{} {
	errType := string(e.Type)
	if e.Type == ErrorTypeAllProvidersFailed {
		errType = "server_error"
	}

	result := map[string]interface{}{
		"error": map[string]interface{}{
			"message": e.Message,
			"type":    errType,
			"code":    e.Code,
		},
	}

	if e.RetryAfter > 0 {
		result["retry_after"] = e.RetryAfter.Seconds()
	}

	return result
}
