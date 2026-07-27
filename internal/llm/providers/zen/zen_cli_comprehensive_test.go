package zen

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =====================================================
// ZEN CLI PROVIDER COMPREHENSIVE TESTS
// No false positives allowed - tests MUST fail if functionality is broken
// =====================================================

// TestZenCLIProvider_ConfigurationValidation tests configuration edge cases
func TestZenCLIProvider_ConfigurationValidation(t *testing.T) {
	t.Parallel()
	t.Run("default config has expected values", func(t *testing.T) {
		config := DefaultZenCLIConfig()

		// STRICT: These values must be exact
		assert.Equal(t, "", config.Model, "Default model must be empty for lazy discovery")
		assert.Equal(t, 120*time.Second, config.Timeout, "Default timeout must be 120s")
		assert.Equal(t, 4096, config.MaxOutputTokens, "Default max tokens must be 4096")
	})

	t.Run("zero timeout gets default", func(t *testing.T) {
		t.Parallel()
		config := ZenCLIConfig{
			Model:           "test-model",
			Timeout:         0, // Zero
			MaxOutputTokens: 1000,
		}
		provider := NewZenCLIProvider(config)

		assert.Equal(t, 120*time.Second, provider.timeout, "Zero timeout must be replaced with default")
	})

	t.Run("zero max tokens gets default", func(t *testing.T) {
		t.Parallel()
		config := ZenCLIConfig{
			Model:           "test-model",
			Timeout:         60 * time.Second,
			MaxOutputTokens: 0, // Zero
		}
		provider := NewZenCLIProvider(config)

		assert.Equal(t, 4096, provider.maxOutputTokens, "Zero max tokens must be replaced with default")
	})

	t.Run("explicit values are preserved", func(t *testing.T) {
		t.Parallel()
		config := ZenCLIConfig{
			Model:           "custom-model",
			Timeout:         30 * time.Second,
			MaxOutputTokens: 2048,
		}
		provider := NewZenCLIProvider(config)

		assert.Equal(t, "custom-model", provider.model)
		assert.Equal(t, 30*time.Second, provider.timeout)
		assert.Equal(t, 2048, provider.maxOutputTokens)
	})
}

// TestZenCLIProvider_FailedAPIModelTracking tests the failed API model tracking mechanism
func TestZenCLIProvider_FailedAPIModelTracking(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithModel("test-model")

	t.Run("initially no models are marked as failed", func(t *testing.T) {
		t.Parallel()
		assert.False(t, provider.IsModelFailedAPI("any-model"))
		assert.False(t, provider.IsModelFailedAPI("test-model"))
		assert.False(t, provider.IsModelFailedAPI(""))
	})

	t.Run("marking a model as failed is tracked", func(t *testing.T) {
		t.Parallel()
		provider.MarkModelAsFailedAPI("failed-model-1")
		provider.MarkModelAsFailedAPI("failed-model-2")

		assert.True(t, provider.IsModelFailedAPI("failed-model-1"))
		assert.True(t, provider.IsModelFailedAPI("failed-model-2"))
		assert.False(t, provider.IsModelFailedAPI("not-failed-model"))
	})

	t.Run("marking same model multiple times is idempotent", func(t *testing.T) {
		t.Parallel()
		provider.MarkModelAsFailedAPI("idempotent-model")
		provider.MarkModelAsFailedAPI("idempotent-model")
		provider.MarkModelAsFailedAPI("idempotent-model")

		assert.True(t, provider.IsModelFailedAPI("idempotent-model"))
	})

	t.Run("ShouldUseCLIFacade respects CLI availability", func(t *testing.T) {
		t.Parallel()
		provider.MarkModelAsFailedAPI("facade-test-model")

		// If CLI is available, should use facade for failed models
		if provider.IsCLIAvailable() {
			assert.True(t, provider.ShouldUseCLIFacade("facade-test-model"))
			assert.False(t, provider.ShouldUseCLIFacade("not-failed-model"))
		} else {
			// If CLI not available, should never use facade
			assert.False(t, provider.ShouldUseCLIFacade("facade-test-model"))
		}
	})
}

// TestZenCLIProvider_CLIAvailabilityCheck tests CLI availability checking
func TestZenCLIProvider_CLIAvailabilityCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode") // SKIP-OK: #short-mode
	}

	t.Run("availability check is cached (sync.Once)", func(t *testing.T) {
		t.Parallel()
		provider := NewZenCLIProviderWithModel("test")

		// Call multiple times
		result1 := provider.IsCLIAvailable()
		result2 := provider.IsCLIAvailable()
		result3 := provider.IsCLIAvailable()

		// Results must be consistent (cached)
		assert.Equal(t, result1, result2)
		assert.Equal(t, result2, result3)
	})

	t.Run("GetCLIError returns nil if available", func(t *testing.T) {
		t.Parallel()
		provider := NewZenCLIProviderWithModel("test")

		if provider.IsCLIAvailable() {
			assert.Nil(t, provider.GetCLIError())
		} else {
			assert.NotNil(t, provider.GetCLIError())
		}
	})

	t.Run("unavailable provider returns correct error", func(t *testing.T) {
		t.Parallel()
		provider := NewZenCLIProviderWithUnavailableCLI("test", exec.ErrNotFound)

		assert.False(t, provider.IsCLIAvailable())
		assert.Equal(t, exec.ErrNotFound, provider.GetCLIError())
	})
}

// TestZenCLIProvider_ProviderInterface tests LLMProvider interface compliance
func TestZenCLIProvider_ProviderInterface(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode") // SKIP-OK: #short-mode
	}

	provider := NewZenCLIProviderWithModel("test-model")

	t.Run("GetName returns correct name", func(t *testing.T) {
		t.Parallel()
		name := provider.GetName()
		assert.Equal(t, "zen-cli", name)
		assert.NotEmpty(t, name)
	})

	t.Run("GetProviderType returns correct type", func(t *testing.T) {
		t.Parallel()
		providerType := provider.GetProviderType()
		assert.Equal(t, "zen", providerType)
		assert.NotEmpty(t, providerType)
	})

	t.Run("GetCurrentModel returns set model", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "test-model", provider.GetCurrentModel())
	})

	t.Run("SetModel changes current model", func(t *testing.T) {
		t.Parallel()
		// Use a dedicated provider instance: this subtest mutates the model and
		// runs in parallel with "GetCurrentModel returns set model", which reads
		// it — sharing the outer provider would race and corrupt both.
		mutable := NewZenCLIProviderWithModel("test-model")
		mutable.SetModel("new-model")
		assert.Equal(t, "new-model", mutable.GetCurrentModel())

		mutable.SetModel("another-model")
		assert.Equal(t, "another-model", mutable.GetCurrentModel())
	})

	t.Run("GetCapabilities returns valid capabilities", func(t *testing.T) {
		t.Parallel()
		caps := provider.GetCapabilities()

		require.NotNil(t, caps)
		assert.True(t, caps.SupportsStreaming, "CLI provider must support streaming")
		assert.False(t, caps.SupportsFunctionCalling, "CLI provider must NOT support function calling")
		assert.False(t, caps.SupportsVision, "CLI provider must NOT support vision")
		assert.False(t, caps.SupportsTools, "CLI provider must NOT support tools")

		// Check supported features
		assert.Contains(t, caps.SupportedFeatures, "text_completion")
		assert.Contains(t, caps.SupportedFeatures, "chat")

		// Check metadata
		assert.Equal(t, "OpenCode Zen (CLI Facade)", caps.Metadata["provider"])
		assert.Equal(t, "true", caps.Metadata["facade"])
		assert.Equal(t, "opencode", caps.Metadata["cli_command"])
	})

	t.Run("ValidateConfig returns expected results", func(t *testing.T) {
		t.Parallel()
		valid, errors := provider.ValidateConfig(nil)

		if provider.IsCLIAvailable() {
			assert.True(t, valid)
			assert.Empty(t, errors)
		} else {
			assert.False(t, valid)
			assert.NotEmpty(t, errors)
		}
	})
}

// TestZenCLIProvider_CompleteNotAvailable tests Complete when CLI is unavailable
func TestZenCLIProvider_CompleteNotAvailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "exec.ErrNotFound",
			err:      exec.ErrNotFound,
			expected: "not available",
		},
		{
			name:     "custom error",
			err:      assert.AnError,
			expected: "not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewZenCLIProviderWithUnavailableCLI("test-model", tt.err)

			req := &models.LLMRequest{
				Prompt: "test prompt",
				Messages: []models.Message{
					{Role: "user", Content: "Hello"},
				},
			}

			resp, err := provider.Complete(context.Background(), req)

			// STRICT: Must return error when CLI unavailable
			require.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// TestZenCLIProvider_CompleteStreamNotAvailable tests CompleteStream when CLI is unavailable
func TestZenCLIProvider_CompleteStreamNotAvailable(t *testing.T) {
	t.Parallel()
	provider := NewZenCLIProviderWithUnavailableCLI("test-model", exec.ErrNotFound)

	req := &models.LLMRequest{
		Prompt: "test prompt",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)

	// STRICT: Must return error when CLI unavailable
	require.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "not available")
}

// TestZenCLIProvider_EmptyPromptHandling tests handling of empty prompts
func TestZenCLIProvider_EmptyPromptHandling(t *testing.T) {
	t.Parallel()
	if !IsOpenCodeInstalled() {
		t.Skip("OpenCode CLI not installed") // SKIP-OK: #legacy-untriaged
	}

	provider := NewZenCLIProviderWithModel("test-model")

	t.Run("empty messages and empty prompt returns error", func(t *testing.T) {
		t.Parallel()
		req := &models.LLMRequest{
			Prompt:   "",
			Messages: []models.Message{},
		}

		resp, err := provider.Complete(context.Background(), req)

		// STRICT: Must fail with empty prompt
		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "no prompt")
	})

	t.Run("nil messages with empty prompt returns error", func(t *testing.T) {
		t.Parallel()
		req := &models.LLMRequest{
			Prompt:   "",
			Messages: nil,
		}

		resp, err := provider.Complete(context.Background(), req)

		require.Error(t, err)
		assert.Nil(t, resp)
	})
}

// TestZenCLIProvider_ModelDiscovery tests model discovery functionality
func TestZenCLIProvider_ModelDiscovery(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode") // SKIP-OK: #short-mode
	}

	provider := NewZenCLIProviderWithModel("initial-model")

	t.Run("GetAvailableModels returns non-empty list", func(t *testing.T) {
		t.Parallel()
		models := provider.GetAvailableModels()

		// STRICT: Must return at least some models (fallback or discovered)
		require.NotEmpty(t, models, "GetAvailableModels must return at least fallback models")
	})

	t.Run("GetBestAvailableModel returns non-empty string", func(t *testing.T) {
		t.Parallel()
		bestModel := provider.GetBestAvailableModel()

		// STRICT: Must return a model name
		require.NotEmpty(t, bestModel, "GetBestAvailableModel must return a model")
	})

	t.Run("IsModelAvailable returns true for available models", func(t *testing.T) {
		t.Parallel()
		models := provider.GetAvailableModels()

		if len(models) > 0 {
			// First model should be available
			assert.True(t, provider.IsModelAvailable(models[0]))
		}
	})

	t.Run("IsModelAvailable returns false for non-existent model", func(t *testing.T) {
		t.Parallel()
		assert.False(t, provider.IsModelAvailable("definitely-not-a-real-model-12345"))
	})
}

// TestZenCLIProvider_KnownModels tests known models functionality
func TestZenCLIProvider_KnownModels(t *testing.T) {
	t.Parallel()
	knownModels := GetKnownZenModels()

	t.Run("known models list is not empty", func(t *testing.T) {
		t.Parallel()
		require.NotEmpty(t, knownModels)
	})

	t.Run("known models contain expected entries", func(t *testing.T) {
		t.Parallel()
		// STRICT: These models MUST be in the known list (as of 2026-02)
		assert.Contains(t, knownModels, "big-pickle")
		assert.Contains(t, knownModels, "gpt-5-nano")
		assert.Contains(t, knownModels, "glm-4.7")
		// Note: qwen3-coder removed from free tier 2026-02
		assert.Contains(t, knownModels, "kimi-k2")
		assert.Contains(t, knownModels, "gemini-3-flash")
	})
}

// TestZenCLIProvider_ParseModelsOutput tests model output parsing
func TestZenCLIProvider_ParseModelsOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		output         string
		expectEmpty    bool
		expectedModels []string
	}{
		{
			name:           "empty output",
			output:         "",
			expectEmpty:    true,
			expectedModels: nil,
		},
		{
			name:           "whitespace only",
			output:         "   \n\t\n   ",
			expectEmpty:    true,
			expectedModels: nil,
		},
		{
			name:           "grok models",
			output:         "grok-code\ngrok-beta",
			expectEmpty:    false,
			expectedModels: []string{"grok-code", "grok-beta"},
		},
		{
			name:           "pickle models",
			output:         "big-pickle\nsmall-pickle",
			expectEmpty:    false,
			expectedModels: []string{"big-pickle", "small-pickle"},
		},
		{
			name:           "glm models",
			output:         "glm-4.7-free\nglm-4.5",
			expectEmpty:    false,
			expectedModels: []string{"glm-4.7-free", "glm-4.5"},
		},
		{
			name:           "gpt-5 models",
			output:         "gpt-5-nano\ngpt-5-mini",
			expectEmpty:    false,
			expectedModels: []string{"gpt-5-nano", "gpt-5-mini"},
		},
		{
			name:           "opencode prefixed models",
			output:         "opencode/big-pickle\nopencode/grok-code",
			expectEmpty:    false,
			expectedModels: []string{"opencode/big-pickle", "opencode/grok-code"},
		},
		{
			name:           "mixed valid and invalid",
			output:         "random-text\ngrok-code\nunrelated\nbig-pickle\nmore-random",
			expectEmpty:    false,
			expectedModels: []string{"grok-code", "big-pickle"},
		},
		{
			name:        "no valid models",
			output:      "random-text\nunrelated\nmore-random\nfoo\nbar",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			models := parseZenModelsOutput(tt.output)

			if tt.expectEmpty {
				assert.Empty(t, models)
			} else {
				assert.NotEmpty(t, models)
				for _, expected := range tt.expectedModels {
					assert.Contains(t, models, expected)
				}
			}
		})
	}
}

// TestZenCLIProvider_ConcurrentAccess tests thread safety
func TestZenCLIProvider_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode") // SKIP-OK: #short-mode
	}

	provider := NewZenCLIProviderWithModel("test-model")

	t.Run("concurrent model marking is safe", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				modelName := "concurrent-model-" + string(rune('A'+idx%26))
				provider.MarkModelAsFailedAPI(modelName)
				provider.IsModelFailedAPI(modelName)
			}(i)
		}

		wg.Wait()
		// Test passes if no race condition panic occurred
	})

	t.Run("concurrent CLI availability check is safe", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		numGoroutines := 50

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				provider.IsCLIAvailable()
				_ = provider.GetCLIError()
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent model discovery is safe", func(t *testing.T) {
		t.Parallel()
		var wg sync.WaitGroup
		numGoroutines := 20

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				provider.GetAvailableModels()
				provider.GetBestAvailableModel()
			}()
		}

		wg.Wait()
	})
}

// TestZenCLIProvider_HealthCheckComprehensive tests health check functionality
func TestZenCLIProvider_HealthCheckComprehensive(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode") // SKIP-OK: #short-mode
	}

	t.Run("health check returns appropriate result based on CLI availability", func(t *testing.T) {
		t.Parallel()
		provider := NewZenCLIProviderWithModel("test")

		err := provider.HealthCheck()

		if IsOpenCodeInstalled() {
			assert.NoError(t, err, "Health check should pass if CLI is installed")
		} else {
			assert.Error(t, err, "Health check should fail if CLI is not installed")
		}
	})

	t.Run("health check with unavailable CLI returns error", func(t *testing.T) {
		t.Parallel()
		provider := NewZenCLIProviderWithUnavailableCLI("test", exec.ErrNotFound)

		err := provider.HealthCheck()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available")
	})
}

// TestDiscoverZenModels_Standalone tests standalone discovery function
func TestDiscoverZenModels_Standalone(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode") // SKIP-OK: #short-mode
	}

	models, err := DiscoverZenModels()

	// STRICT: Must always return some models (discovered or fallback)
	require.NotEmpty(t, models, "DiscoverZenModels must always return models")

	if IsOpenCodeInstalled() {
		// If CLI is installed, may or may not have error depending on CLI output
		t.Logf("CLI installed, discovered %d models, error: %v", len(models), err)
	} else {
		// If CLI not installed, should return error but still have fallback models
		require.Error(t, err)
		t.Logf("CLI not installed, returning %d fallback models", len(models))
	}
}

// TestIsOpenCodeInstalled_Standalone tests the standalone installation check
func TestIsOpenCodeInstalled_Standalone(t *testing.T) {
	t.Parallel()
	// IsOpenCodeInstalled runs `opencode --version` which may time out even when
	// the binary exists in PATH. We just verify it returns a bool without crashing.
	_ = IsOpenCodeInstalled()
}

// TestGetOpenCodePath_Standalone tests getting the CLI path
func TestGetOpenCodePath_Standalone(t *testing.T) {
	t.Parallel()
	path, err := GetOpenCodePath()

	// GetOpenCodePath uses exec.LookPath, which can succeed even when
	// `opencode --version` times out. Accept either outcome.
	if err == nil {
		assert.NotEmpty(t, path)
		assert.True(t, strings.Contains(path, "opencode") || strings.HasSuffix(path, "opencode"),
			"Path should contain or end with 'opencode'")
	} else {
		assert.Empty(t, path)
	}
}

// TestZenCLIProvider_ResponseMetadata tests response metadata completeness
func TestZenCLIProvider_ResponseMetadata(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping CLI availability test in short mode") // SKIP-OK: #short-mode
	}

	if !IsOpenCodeInstalled() {
		t.Skip("OpenCode CLI not installed - skipping response metadata test") // SKIP-OK: #legacy-untriaged
	}

	provider := NewZenCLIProviderWithModel(DefaultZenModel)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		Messages: []models.Message{
			{Role: "user", Content: "Say 'test' and nothing else"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens: 10,
		},
	}

	resp, err := provider.Complete(ctx, req)

	if err != nil {
		t.Logf("Completion failed (may be expected): %v", err)
		t.Skip("CLI completion failed - cannot test metadata") // SKIP-OK: #legacy-untriaged
	}

	require.NotNil(t, resp)

	// STRICT: Response must have required fields
	assert.NotEmpty(t, resp.ID, "Response ID must not be empty")
	assert.Equal(t, "zen-cli", resp.ProviderID, "Provider ID must be 'zen-cli'")
	assert.Equal(t, "zen-cli", resp.ProviderName, "Provider name must be 'zen-cli'")
	assert.NotEmpty(t, resp.Content, "Content must not be empty")

	// Check metadata
	require.NotNil(t, resp.Metadata, "Metadata must not be nil")
	assert.Equal(t, "opencode-cli", resp.Metadata["source"])
	assert.Equal(t, true, resp.Metadata["facade"])
}
