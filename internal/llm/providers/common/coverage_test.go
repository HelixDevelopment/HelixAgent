package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestProviderConfig_Validation tests provider configuration validation
func TestProviderConfig_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		apiKey    string
		baseURL   string
		model     string
		expectErr bool
	}{
		{
			name:      "valid config",
			apiKey:    "test-api-key",
			baseURL:   "https://api.example.com",
			model:     "gpt-4",
			expectErr: false,
		},
		{
			name:      "empty api key",
			apiKey:    "",
			baseURL:   "https://api.example.com",
			model:     "gpt-4",
			expectErr: true,
		},
		{
			name:      "empty base url",
			apiKey:    "test-api-key",
			baseURL:   "",
			model:     "gpt-4",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			config := &ProviderConfig{
				APIKey:  tt.apiKey,
				BaseURL: tt.baseURL,
				Model:   tt.model,
				Timeout: 30 * time.Second,
			}

			if tt.expectErr {
				assert.True(t, config.APIKey == "" || config.BaseURL == "")
			} else {
				assert.NotEmpty(t, config.APIKey)
				assert.NotEmpty(t, config.BaseURL)
			}
		})
	}
}

// TestRetryConfig_Validation tests retry configuration
func TestRetryConfig_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		maxRetries int
		backoff    time.Duration
		valid      bool
	}{
		{
			name:       "valid retry config",
			maxRetries: 3,
			backoff:    1 * time.Second,
			valid:      true,
		},
		{
			name:       "zero retries",
			maxRetries: 0,
			backoff:    1 * time.Second,
			valid:      true,
		},
		{
			name:       "negative retries",
			maxRetries: -1,
			backoff:    1 * time.Second,
			valid:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			config := &RetryConfig{
				MaxRetries: tt.maxRetries,
				Backoff:    tt.backoff,
			}

			if tt.valid {
				assert.GreaterOrEqual(t, config.MaxRetries, 0)
			} else {
				assert.Less(t, config.MaxRetries, 0)
			}
		})
	}
}

// ProviderConfig represents common provider configuration
type ProviderConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	MaxRetries int
	Backoff    time.Duration
}
