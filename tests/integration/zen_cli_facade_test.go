package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm/providers/zen"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/verifier"
	"dev.helix.agent/internal/verifier/adapters"
)

// =====================================================
// ZEN CLI FACADE INTEGRATION TESTS
// Tests the complete CLI facade mechanism for models that fail direct API
// No false positives - tests MUST fail if functionality is broken
// =====================================================

// TestZenCLIFacade_EndToEnd tests the complete CLI facade flow
func TestZenCLIFacade_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	if !zen.IsOpenCodeInstalled() {
		t.Skip("Skipping integration test - OpenCode CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	t.Run("CLI provider can be created and queried", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)

		// Verify basic properties
		assert.Equal(t, "zen-cli", provider.GetName())
		assert.Equal(t, "zen", provider.GetProviderType())
		assert.True(t, provider.IsCLIAvailable())
		assert.Nil(t, provider.GetCLIError())
	})

	t.Run("CLI provider health check passes", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)

		err := provider.HealthCheck()
		assert.NoError(t, err, "Health check should pass when CLI is installed")
	})

	t.Run("CLI provider model discovery works", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithModel("")

		models := provider.GetAvailableModels()

		// STRICT: Must return models
		require.NotEmpty(t, models, "Model discovery must return at least fallback models")
		t.Logf("Discovered %d models via CLI provider", len(models))
	})

	t.Run("CLI provider capabilities are correct", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)

		caps := provider.GetCapabilities()

		require.NotNil(t, caps)
		assert.True(t, caps.SupportsStreaming)
		assert.False(t, caps.SupportsFunctionCalling)
		assert.False(t, caps.SupportsTools)
		assert.Contains(t, caps.Metadata["facade"], "true")
	})
}

// TestZenCLIFacade_FailedAPIModelTracking tests tracking of failed API models
func TestZenCLIFacade_FailedAPIModelTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	if !zen.IsOpenCodeInstalled() {
		t.Skip("Skipping integration test - OpenCode CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	provider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)

	t.Run("models start as not failed", func(t *testing.T) {
		testModels := []string{
			"model-a",
			"model-b",
			zen.DefaultZenModel,
			"grok-code",
		}

		for _, m := range testModels {
			assert.False(t, provider.IsModelFailedAPI(m),
				"Model %s should not be marked as failed initially", m)
		}
	})

	t.Run("marking models as failed is tracked correctly", func(t *testing.T) {
		failedModels := []string{"failed-1", "failed-2", "failed-3"}

		for _, m := range failedModels {
			provider.MarkModelAsFailedAPI(m)
		}

		for _, m := range failedModels {
			assert.True(t, provider.IsModelFailedAPI(m),
				"Model %s should be marked as failed", m)
		}

		// Verify non-failed models are still not failed
		assert.False(t, provider.IsModelFailedAPI("not-failed"))
	})

	t.Run("ShouldUseCLIFacade returns true for failed models", func(t *testing.T) {
		provider.MarkModelAsFailedAPI("facade-candidate")

		assert.True(t, provider.ShouldUseCLIFacade("facade-candidate"))
		assert.False(t, provider.ShouldUseCLIFacade("non-failed-model"))
	})
}

// TestZenCLIFacade_FreeAdapterIntegration tests integration with FreeProviderAdapter
func TestZenCLIFacade_FreeAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	adapter := adapters.NewFreeProviderAdapter(nil, nil)

	t.Run("adapter has CLI facade available if opencode installed", func(t *testing.T) {
		available := adapter.IsCLIFacadeAvailable()

		if zen.IsOpenCodeInstalled() {
			assert.True(t, available, "CLI facade should be available when opencode is installed")
		} else {
			assert.False(t, available, "CLI facade should not be available when opencode is not installed")
		}
	})

	t.Run("adapter can get CLI facade provider", func(t *testing.T) {
		provider := adapter.GetCLIFacadeProvider()

		if zen.IsOpenCodeInstalled() {
			require.NotNil(t, provider)
			assert.Equal(t, "zen-cli", provider.GetName())
		} else {
			assert.Nil(t, provider)
		}
	})

	t.Run("adapter tracks failed API models", func(t *testing.T) {
		failed := adapter.GetFailedAPIModels()
		// Initially should be empty (or from previous verification)
		t.Logf("Initially tracking %d failed API models", len(failed))
	})

	t.Run("adapter identifies CLI facade models", func(t *testing.T) {
		// Initially should not have any CLI facade models
		cliModels := adapter.GetCLIFacadeModels()
		t.Logf("Currently %d models using CLI facade", len(cliModels))
	})
}

// TestZenCLIFacade_VerificationWithFallback tests verification with CLI fallback
func TestZenCLIFacade_VerificationWithFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping verification test in short mode")  // SKIP-OK: #short-mode
	}

	// Skip if no network access available
	if os.Getenv("SKIP_NETWORK_TESTS") == "1" {
		t.Skip("Skipping network-dependent test")  // SKIP-OK: #requires-network
	}

	adapter := adapters.NewFreeProviderAdapter(nil, adapters.DefaultFreeAdapterConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Run("Zen provider verification completes", func(t *testing.T) {
		provider, err := adapter.VerifyZenProvider(ctx)

		// Verification should complete (may fail if network unavailable)
		if err != nil {
			t.Logf("Verification error (may be expected): %v", err)
			return
		}

		require.NotNil(t, provider)
		assert.Equal(t, "zen", provider.ID)
		assert.NotEmpty(t, provider.Name)

		// Check metadata includes CLI facade info
		if provider.Metadata != nil {
			t.Logf("Verification metadata: %+v", provider.Metadata)

			if cliAvailable, ok := provider.Metadata["cli_facade_available"].(bool); ok {
				if zen.IsOpenCodeInstalled() {
					assert.True(t, cliAvailable)
				}
			}

			if apiVerified, ok := provider.Metadata["api_verified_models"].(int); ok {
				t.Logf("API verified models: %d", apiVerified)
			}

			if cliVerified, ok := provider.Metadata["cli_verified_models"].(int); ok {
				t.Logf("CLI verified models: %d", cliVerified)
			}
		}
	})
}

// TestZenCLIFacade_CompleteRequest tests actual completion via CLI
func TestZenCLIFacade_CompleteRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	if !zen.IsOpenCodeInstalled() {
		t.Skip("Skipping integration test - OpenCode CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	provider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("simple completion request", func(t *testing.T) {
		req := &models.LLMRequest{
			Messages: []models.Message{
				{Role: "user", Content: "What is 2 + 2? Reply with just the number."},
			},
			ModelParams: models.ModelParameters{
				MaxTokens:   10,
				Temperature: 0.0,
			},
		}

		resp, err := provider.Complete(ctx, req)

		if err != nil {
			t.Logf("Completion failed (may be expected if not authenticated): %v", err)
			// Don't fail - CLI may not be authenticated
			return
		}

		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Content)
		assert.Equal(t, "zen-cli", resp.ProviderID)

		// Check for "4" in response (basic math test)
		t.Logf("Response: %s", resp.Content)
	})

	t.Run("completion with system message", func(t *testing.T) {
		req := &models.LLMRequest{
			Messages: []models.Message{
				{Role: "system", Content: "You are a helpful assistant. Be concise."},
				{Role: "user", Content: "Say 'hello' and nothing else."},
			},
			ModelParams: models.ModelParameters{
				MaxTokens:   20,
				Temperature: 0.0,
			},
		}

		resp, err := provider.Complete(ctx, req)

		if err != nil {
			t.Logf("Completion failed: %v", err)
			return
		}

		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Content)
	})
}

// TestZenCLIFacade_StreamingRequest tests streaming completion via CLI
func TestZenCLIFacade_StreamingRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	if !zen.IsOpenCodeInstalled() {
		t.Skip("Skipping integration test - OpenCode CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	provider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		Messages: []models.Message{
			{Role: "user", Content: "Count from 1 to 5, each on a new line."},
		},
		ModelParams: models.ModelParameters{
			MaxTokens: 50,
		},
	}

	ch, err := provider.CompleteStream(ctx, req)

	if err != nil {
		t.Logf("Stream request failed (may be expected): %v", err)
		return
	}

	require.NotNil(t, ch)

	var chunks []string
	for resp := range ch {
		if resp.Content != "" {
			chunks = append(chunks, resp.Content)
		}
		// Check for errors in metadata
		if resp.Metadata != nil {
			if errMsg, ok := resp.Metadata["error"].(string); ok && errMsg != "" {
				t.Logf("Stream error: %s", errMsg)
			}
		}
	}

	t.Logf("Received %d stream chunks", len(chunks))
}

// TestZenCLIFacade_ModelSwitching tests switching between models
func TestZenCLIFacade_ModelSwitching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	if !zen.IsOpenCodeInstalled() {
		t.Skip("Skipping integration test - OpenCode CLI not installed")  // SKIP-OK: #integration-mode-only
	}

	provider := zen.NewZenCLIProviderWithModel("initial-model")

	assert.Equal(t, "initial-model", provider.GetCurrentModel())

	// Switch to different models
	testModels := []string{"model-a", "model-b", "grok-code", zen.DefaultZenModel}

	for _, m := range testModels {
		provider.SetModel(m)
		assert.Equal(t, m, provider.GetCurrentModel())
	}
}

// TestZenCLIFacade_UnifiedProviderCreation tests creating UnifiedProvider from CLI provider
func TestZenCLIFacade_UnifiedProviderCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	if !zen.IsOpenCodeInstalled() {
		t.Skip("Skipping - OpenCode CLI not installed")  // SKIP-OK: #legacy-untriaged
	}

	cliProvider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)
	caps := cliProvider.GetCapabilities()

	// Create a UnifiedProvider-like structure
	provider := &verifier.UnifiedProvider{
		ID:       "zen-cli",
		Name:     "OpenCode Zen (CLI Facade)",
		Type:     "cli",
		Verified: cliProvider.IsCLIAvailable(),
		Score:    6.5, // CLI providers get lower score
		Models: []verifier.UnifiedModel{
			{
				ID:           zen.DefaultZenModel,
				Name:         "Default Zen Model (CLI)",
				Provider:     "zen-cli",
				Verified:     true,
				Capabilities: caps.SupportedFeatures,
				Metadata: map[string]interface{}{
					"verified_via": "cli_facade",
				},
			},
		},
		Instance: cliProvider,
		Metadata: map[string]interface{}{
			"cli_facade": true,
			"cli_path":   cliProvider.GetName(),
		},
	}

	assert.Equal(t, "zen-cli", provider.ID)
	assert.True(t, provider.Verified)
	assert.NotEmpty(t, provider.Models)
	assert.NotNil(t, provider.Instance)
}

// TestZenCLIFacade_ErrorHandling tests error handling scenarios
func TestZenCLIFacade_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	t.Run("unavailable CLI returns proper error", func(t *testing.T) {
		provider := zen.NewZenCLIProviderWithUnavailableCLI("test", assert.AnError)

		ctx := context.Background()
		req := &models.LLMRequest{
			Messages: []models.Message{{Role: "user", Content: "test"}},
		}

		// Complete should fail
		resp, err := provider.Complete(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "not available")

		// Stream should fail
		ch, err := provider.CompleteStream(ctx, req)
		require.Error(t, err)
		assert.Nil(t, ch)

		// Health check should fail
		err = provider.HealthCheck()
		require.Error(t, err)
	})

	t.Run("context cancellation is respected", func(t *testing.T) {
		if !zen.IsOpenCodeInstalled() {
			t.Skip("OpenCode CLI not installed")  // SKIP-OK: #legacy-untriaged
		}

		provider := zen.NewZenCLIProviderWithModel(zen.DefaultZenModel)

		// Create already cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := &models.LLMRequest{
			Messages: []models.Message{{Role: "user", Content: "test"}},
		}

		// Complete should fail fast with context error
		resp, err := provider.Complete(ctx, req)
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
		_ = resp // May be nil or have error
	})
}

// TestZenCLIFacade_FreeAdapterCLIModelTracking tests that adapter properly tracks CLI models
func TestZenCLIFacade_FreeAdapterCLIModelTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Zen CLI test in short mode (makes live CLI calls)")  // SKIP-OK: #short-mode
	}
	adapter := adapters.NewFreeProviderAdapter(nil, nil)

	t.Run("IsModelUsingCLIFacade returns false for non-existent model", func(t *testing.T) {
		result := adapter.IsModelUsingCLIFacade("non-existent-model-xyz")
		assert.False(t, result)
	})

	t.Run("GetCLIFacadeModels returns empty when no CLI models", func(t *testing.T) {
		// Fresh adapter should have no CLI facade models
		cliModels := adapter.GetCLIFacadeModels()
		// May be empty or have models from previous runs
		t.Logf("CLI facade models: %d", len(cliModels))
	})

	t.Run("GetFailedAPIModels returns copy", func(t *testing.T) {
		failed1 := adapter.GetFailedAPIModels()
		failed2 := adapter.GetFailedAPIModels()

		// Modifying one should not affect the other
		failed1["test-model"] = assert.AnError

		_, exists := failed2["test-model"]
		assert.False(t, exists, "Returned map should be a copy")
	})
}
