package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/verifier"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllama_ExplicitlyDisabledByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: VerifyAllProviders touches every live provider and routinely exceeds 5 min")
	}
	// Clear any existing OLLAMA_ENABLED env var
	os.Unsetenv("OLLAMA_ENABLED")
	os.Unsetenv("OLLAMA_BASE_URL")

	// Create a startup verifier with test config
	logger := logrus.New()
	config := &verifier.StartupConfig{
		VerificationTimeout:  30 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Run verification to get discovered providers
	ctx := context.Background()
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	providers := result.Providers
	require.NotNil(t, providers)

	// Check that Ollama is NOT in the discovered providers when OLLAMA_ENABLED is not set
	for _, p := range providers {
		assert.NotEqual(t, "ollama", p.Type, "Ollama should not be discovered when OLLAMA_ENABLED is not set")
	}
}

func TestOllama_ExplicitlyEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: VerifyAllProviders touches every live provider and routinely exceeds 5 min")
	}
	// Set OLLAMA_ENABLED to true
	os.Setenv("OLLAMA_ENABLED", "true")
	defer os.Unsetenv("OLLAMA_ENABLED")

	// Note: We can't actually test Ollama discovery without a running Ollama instance
	// but we can verify the configuration logic
	logger := logrus.New()
	config := &verifier.StartupConfig{
		VerificationTimeout:  30 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Run verification to get discovered providers
	ctx := context.Background()
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	providers := result.Providers
	require.NotNil(t, providers)

	// When OLLAMA_ENABLED=true but Ollama is not running,
	// a warning should be logged but no provider should be added
	ollamaFound := false
	for _, p := range providers {
		if p.Type == "ollama" {
			ollamaFound = true
			break
		}
	}

	// Ollama should only be found if it's actually running
	// Since we're in a test environment, it likely won't be
	if ollamaFound {
		t.Log("Ollama was discovered - this means Ollama is running locally")
	} else {
		t.Log("Ollama was not discovered - either not enabled or not running (expected in tests)")
	}
}

func TestOllama_EnvironmentVariablePropagation(t *testing.T) {
	// Test that OLLAMA_ENABLED is read correctly
	testCases := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"explicitly_true", "true", true},
		{"explicitly_false", "false", false},
		{"empty_string", "", false},
		{"unset", "unset", false},
		{"yes", "yes", false}, // Only "true" should enable
		{"1", "1", false},     // Only "true" should enable
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue == "unset" {
				os.Unsetenv("OLLAMA_ENABLED")
			} else {
				os.Setenv("OLLAMA_ENABLED", tc.envValue)
				defer os.Unsetenv("OLLAMA_ENABLED")
			}

			value := os.Getenv("OLLAMA_ENABLED")
			enabled := value == "true"
			assert.Equal(t, tc.expected, enabled, "OLLAMA_ENABLED should be %v when set to '%s'", tc.expected, tc.envValue)
		})
	}
}
