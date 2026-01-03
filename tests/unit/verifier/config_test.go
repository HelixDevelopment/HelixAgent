package verifier_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/verifier"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "./data/llm-verifier.db", cfg.Database.Path)
	assert.Equal(t, 60*time.Second, cfg.Verification.VerificationTimeout)
	assert.Equal(t, 3, cfg.Verification.RetryCount)
	assert.Equal(t, 5*time.Second, cfg.Verification.RetryDelay)
	assert.Equal(t, "Do you see my code?", cfg.Verification.CodeVisibilityPrompt)
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		weights     verifier.ScoringWeightsConfig
		expectError bool
	}{
		{
			name: "valid weights",
			weights: verifier.ScoringWeightsConfig{
				ResponseSpeed:     0.25,
				ModelEfficiency:   0.20,
				CostEffectiveness: 0.25,
				Capability:        0.20,
				Recency:           0.10,
			},
			expectError: false,
		},
		{
			name: "weights sum less than 1",
			weights: verifier.ScoringWeightsConfig{
				ResponseSpeed:     0.20,
				ModelEfficiency:   0.20,
				CostEffectiveness: 0.20,
				Capability:        0.20,
				Recency:           0.10,
			},
			expectError: true,
		},
		{
			name: "weights sum more than 1",
			weights: verifier.ScoringWeightsConfig{
				ResponseSpeed:     0.30,
				ModelEfficiency:   0.30,
				CostEffectiveness: 0.30,
				Capability:        0.20,
				Recency:           0.10,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := verifier.DefaultConfig()
			cfg.Scoring.Weights = tt.weights

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadConfigFromBytes(t *testing.T) {
	t.Parallel()

	configYAML := `
verifier:
  enabled: true
  database:
    path: "/custom/path/db.sqlite"
  verification:
    mandatory_code_check: true
    verification_timeout: 120s
    retry_count: 5
  scoring:
    weights:
      response_speed: 0.25
      model_efficiency: 0.20
      cost_effectiveness: 0.25
      capability: 0.20
      recency: 0.10
    cache_ttl: 12h
  health:
    check_interval: 60s
    timeout: 20s
  api:
    enabled: true
    port: "9000"
`

	cfg, err := verifier.LoadConfigFromBytes([]byte(configYAML))
	require.NoError(t, err)

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "/custom/path/db.sqlite", cfg.Database.Path)
	assert.True(t, cfg.Verification.MandatoryCodeCheck)
	assert.Equal(t, 120*time.Second, cfg.Verification.VerificationTimeout)
	assert.Equal(t, 5, cfg.Verification.RetryCount)
	assert.Equal(t, 12*time.Hour, cfg.Scoring.CacheTTL)
	assert.Equal(t, 60*time.Second, cfg.Health.CheckInterval)
	assert.Equal(t, "9000", cfg.API.Port)
}

func TestExpandEnvVars(t *testing.T) {
	t.Parallel()

	os.Setenv("TEST_API_KEY", "secret-key")
	defer os.Unsetenv("TEST_API_KEY")

	result := verifier.ExpandEnvVars("key=${TEST_API_KEY}")
	assert.Equal(t, "key=secret-key", result)

	result = verifier.ExpandEnvVars("no-vars-here")
	assert.Equal(t, "no-vars-here", result)
}

func TestMaskSensitiveData(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	cfg.Database.EncryptionKey = "super-secret-key"
	cfg.API.JWTSecret = "jwt-secret"
	cfg.Events.Slack.WebhookURL = "https://example.com/slack-webhook-placeholder"
	cfg.Events.Telegram.BotToken = "bot-token"

	masked := cfg.MaskSensitiveData()

	assert.Equal(t, "***", masked.Database.EncryptionKey)
	assert.Equal(t, "***", masked.API.JWTSecret)
	assert.Contains(t, masked.Events.Slack.WebhookURL, "***")
	assert.NotContains(t, masked.Events.Slack.WebhookURL, "slack-webhook-placeholder")
	assert.Equal(t, "***", masked.Events.Telegram.BotToken)
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	emptyConfigYAML := `
verifier:
  enabled: true
`

	cfg, err := verifier.LoadConfigFromBytes([]byte(emptyConfigYAML))
	require.NoError(t, err)

	// Check defaults are applied
	assert.Equal(t, "./data/llm-verifier.db", cfg.Database.Path)
	assert.Equal(t, 60*time.Second, cfg.Verification.VerificationTimeout)
	assert.Equal(t, 3, cfg.Verification.RetryCount)
	assert.Equal(t, 5*time.Second, cfg.Verification.RetryDelay)
	assert.Equal(t, 0.25, cfg.Scoring.Weights.ResponseSpeed)
	assert.Equal(t, 30*time.Second, cfg.Health.CheckInterval)
	assert.Equal(t, "8081", cfg.API.Port)
	assert.Equal(t, "/api/v1/verifier", cfg.API.BasePath)
}

func TestConfigValidationError(t *testing.T) {
	t.Parallel()

	err := &verifier.ConfigValidationError{
		Field:   "scoring.weights",
		Message: "weights must sum to 1.0",
	}

	assert.Equal(t, "scoring.weights: weights must sum to 1.0", err.Error())
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	t.Parallel()

	os.Setenv("TEST_DB_PATH", "/env/path/db.sqlite")
	defer os.Unsetenv("TEST_DB_PATH")

	configYAML := `
verifier:
  enabled: true
  database:
    path: "${TEST_DB_PATH}"
`

	cfg, err := verifier.LoadConfigFromBytes([]byte(configYAML))
	require.NoError(t, err)

	assert.Equal(t, "/env/path/db.sqlite", cfg.Database.Path)
}

func TestConfigVerificationTests(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()

	// Default tests
	expectedTests := []string{
		"existence", "responsiveness", "latency", "streaming",
		"function_calling", "coding_capability", "error_detection",
	}

	assert.Equal(t, expectedTests, cfg.Verification.Tests)
}

func TestConfigSchedulingDefaults(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()

	assert.Equal(t, 24*time.Hour, cfg.Scheduling.ReVerification.Interval)
	assert.Equal(t, 12*time.Hour, cfg.Scheduling.ScoreRecalculation.Interval)
}

func TestConfigBrotliDefaults(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()

	assert.Equal(t, 6, cfg.Brotli.CompressionLevel)
}

func TestConfigCircuitBreakerDefaults(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()

	assert.Equal(t, 60*time.Second, cfg.Health.CircuitBreaker.HalfOpenTimeout)
}
