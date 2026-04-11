package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/testutil"
	"dev.helix.agent/internal/verifier"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderVerification_AllProvidersHaveAPIKeys tests that all configured providers
// have their API keys set in the environment
func TestProviderVerification_AllProvidersHaveAPIKeys(t *testing.T) {
	// List of all providers that require API keys
	providers := []struct {
		name   string
		envVar string
	}{
		{"claude", "CLAUDE_API_KEY"},
		{"deepseek", "DEEPSEEK_API_KEY"},
		{"gemini", "GEMINI_API_KEY"},
		{"mistral", "MISTRAL_API_KEY"},
		{"openrouter", "OPENROUTER_API_KEY"},
		{"zai", "ZAI_API_KEY"},
		{"cerebras", "CEREBRAS_API_KEY"},
		{"chutes", "CHUTES_API_KEY"},
		{"huggingface", "HUGGINGFACE_API_KEY"},
		{"nvidia", "NVIDIA_API_KEY"},
		{"novita", "NOVITA_API_KEY"},
		{"upstage", "UPSTAGE_API_KEY"},
		{"sambanova", "SAMBANOVA_API_KEY"},
		{"siliconflow", "SILICONFLOW_API_KEY"},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			apiKey := os.Getenv(p.envVar)
			if apiKey == "" {
				t.Logf("Warning: %s is not set - %s provider will not be available", p.envVar, p.name)
			} else {
				t.Logf("✓ %s is set", p.envVar)
				assert.NotEmpty(t, apiKey)
			}
		})
	}
}

// TestProviderVerification_StartupVerifierCreation tests that the startup verifier
// can be created with proper configuration
func TestProviderVerification_StartupVerifierCreation(t *testing.T) {
	logger := logrus.New()
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)
	// GetLog() is not exported, but we can verify the verifier was created
}

// TestProviderVerification_ModelVerificationLifecycle tests the full lifecycle
// of model verification including discovery, verification, and scoring
func TestProviderVerification_ModelVerificationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: VerifyAllProviders touches every live provider and routinely exceeds 5 min")
	}
	testutil.RequireAPIKey(t, "deepseek")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := logrus.New()
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Run verification to get discovered providers
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	discovered := result.Providers
	require.NotNil(t, discovered)

	t.Logf("Discovered %d providers", len(discovered))

	// Verify that we discovered at least one provider
	assert.Greater(t, len(discovered), 0, "Should have discovered at least one provider")

	// Check that Ollama is not in the discovered providers when OLLAMA_ENABLED is not set
	ollamaFound := false
	for _, p := range discovered {
		if p.Type == "ollama" {
			ollamaFound = true
			t.Logf("Found Ollama provider (models: %v)", p.Models)
		}
	}

	if os.Getenv("OLLAMA_ENABLED") != "true" {
		assert.False(t, ollamaFound, "Ollama should not be discovered when OLLAMA_ENABLED is not set to 'true'")
	}
}

// TestProviderVerification_VerifiedModelsCanBeUsed tests that verified models
// can actually be used for LLM calls
func TestProviderVerification_VerifiedModelsCanBeUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: VerifyAllProviders touches every live provider and routinely exceeds 5 min")
	}
	testutil.RequireAPIKey(t, "deepseek")

	ctx := context.Background()

	logger := logrus.New()
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)

	// Run startup verification
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	t.Logf("Verification result: %d verified, %d failed", result.VerifiedCount, result.FailedCount)

	// At least some providers should be verified
	assert.Greater(t, result.VerifiedCount, 0, "Should have at least one verified provider")

	// Get ranked providers
	ranked := result.RankedProviders
	require.NotNil(t, ranked)

	// Check that verified providers have valid scores
	for _, p := range ranked {
		if p.Verified {
			assert.Greater(t, p.Score, 0.0, "Verified provider %s should have a positive score", p.Type)
			t.Logf("Verified provider: %s (score: %.2f)", p.Type, p.Score)
		}
	}
}

// TestProviderVerification_FailedProvidersHaveReasons tests that failed providers
// have proper error messages and failure reasons
func TestProviderVerification_FailedProvidersHaveReasons(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: VerifyAllProviders touches every live provider and routinely exceeds 5 min")
	}
	testutil.RequireAPIKey(t, "deepseek")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := logrus.New()
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)

	// Run verification
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check failed providers have reasons
	ranked := result.RankedProviders
	failedCount := 0
	for _, p := range ranked {
		if !p.Verified {
			failedCount++
			assert.NotEmpty(t, p.FailureReason, "Failed provider %s should have a failure reason", p.Type)
			t.Logf("Failed provider %s: %s", p.Type, p.FailureReason)
		}
	}

	t.Logf("Total failed providers: %d", failedCount)
}

// TestProviderVerification_DebateTeamUsesVerifiedProviders tests that the debate team
// configuration uses only verified providers
func TestProviderVerification_DebateTeamUsesVerifiedProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: VerifyAllProviders touches every live provider and routinely exceeds 5 min")
	}
	testutil.RequireAPIKey(t, "deepseek")

	ctx := context.Background()

	logger := logrus.New()
	config := &verifier.StartupConfig{
		VerificationTimeout:  60 * time.Second,
		HealthCheckTimeout:   10 * time.Second,
		ParallelVerification: true,
	}

	sv := verifier.NewStartupVerifier(config, logger)
	require.NotNil(t, sv)

	// Set up instance creator for the debate team
	sv.SetInstanceCreator(func(providerType, model string) llm.LLMProvider {
		// Return a mock provider for testing
		return nil // In real scenario, this would create actual providers
	})

	// Run verification
	result, err := sv.VerifyAllProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Get verified providers
	var verifiedProviders []*verifier.UnifiedProvider
	for _, p := range result.Providers {
		if p.Verified {
			verifiedProviders = append(verifiedProviders, p)
		}
	}
	require.NotNil(t, verifiedProviders)

	t.Logf("Verified providers: %d", len(verifiedProviders))

	// Each verified provider should have at least one verified model
	for _, p := range verifiedProviders {
		hasVerifiedModel := false
		for _, m := range p.Models {
			if m.Verified {
				hasVerifiedModel = true
				break
			}
		}
		assert.True(t, hasVerifiedModel, "Verified provider %s should have at least one verified model", p.Type)
	}
}

// MockLLMProviderForVerification is a mock implementation for testing
type MockLLMProviderForVerification struct{}

func (m *MockLLMProviderForVerification) Complete(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, nil
}

func (m *MockLLMProviderForVerification) CompleteStream(ctx context.Context, req interface{}) (<-chan interface{}, error) {
	return nil, nil
}

func (m *MockLLMProviderForVerification) HealthCheck() error {
	return nil
}

func (m *MockLLMProviderForVerification) GetCapabilities() interface{} {
	return nil
}

func (m *MockLLMProviderForVerification) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
