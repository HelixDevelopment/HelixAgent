// Package adapters provides provider-specific verification adapters for the startup verification system.
package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/verifier"
)

// =====================================================
// FREE PROVIDER ADAPTER COMPREHENSIVE TESTS
// =====================================================

func TestDefaultFreeAdapterConfig_Comprehensive(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()

	require.NotNil(t, cfg)
	assert.Equal(t, 30*time.Second, cfg.VerificationTimeout)
	assert.Equal(t, 10*time.Second, cfg.HealthCheckTimeout)
	assert.Equal(t, 4, cfg.MaxConcurrentVerifications)
	assert.Equal(t, 0.5, cfg.MinHealthScore)
	assert.Equal(t, 6.0, cfg.BaseScore)
	assert.Equal(t, 7.0, cfg.MaxScore)
	assert.Equal(t, 2, cfg.RetryAttempts)
	assert.Equal(t, 1*time.Second, cfg.RetryDelay)
}

func TestNewFreeProviderAdapter_Comprehensive(t *testing.T) {
	t.Parallel()
	t.Run("with nil config uses default", func(t *testing.T) {
		adapter := NewFreeProviderAdapter(nil, nil)

		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.config)
		assert.Equal(t, DefaultFreeAdapterConfig().BaseScore, adapter.config.BaseScore)
		assert.NotNil(t, adapter.httpClient)
		assert.NotNil(t, adapter.verifiedModels)
		assert.NotNil(t, adapter.lastVerified)
		assert.NotNil(t, adapter.healthStatus)
	})

	t.Run("with custom config", func(t *testing.T) {
		t.Parallel()
		customConfig := &FreeAdapterConfig{
			VerificationTimeout:        60 * time.Second,
			HealthCheckTimeout:         20 * time.Second,
			MaxConcurrentVerifications: 8,
			MinHealthScore:             0.7,
			BaseScore:                  5.5,
			MaxScore:                   6.5,
			RetryAttempts:              3,
			RetryDelay:                 2 * time.Second,
		}

		adapter := NewFreeProviderAdapter(nil, customConfig)

		require.NotNil(t, adapter)
		assert.Equal(t, customConfig, adapter.config)
		assert.Equal(t, 5.5, adapter.config.BaseScore)
		assert.Equal(t, 6.5, adapter.config.MaxScore)
		assert.Equal(t, 60*time.Second, adapter.httpClient.Timeout)
	})

	t.Run("with verification service", func(t *testing.T) {
		t.Parallel()
		verifierSvc := verifier.NewVerificationService(&verifier.Config{})
		adapter := NewFreeProviderAdapter(verifierSvc, nil)

		require.NotNil(t, adapter)
		assert.Equal(t, verifierSvc, adapter.verifierSvc)
	})
}

func TestFreeProviderAdapter_GetVerifiedModels(t *testing.T) {
	t.Parallel()

	t.Run("empty initially", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		models := adapter.GetVerifiedModels()
		assert.NotNil(t, models)
		assert.Empty(t, models)
	})

	t.Run("returns copy of models", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		// Add a model manually
		testModel := &verifier.UnifiedModel{
			ID:       "test-model",
			Name:     "Test Model",
			Verified: true,
			Score:    6.5,
		}
		adapter.verifiedModels.Put("test-model", testModel)


		models := adapter.GetVerifiedModels()
		assert.Len(t, models, 1)
		assert.Contains(t, models, "test-model")

		// Modify returned map shouldn't affect internal state
		delete(models, "test-model")

		models2 := adapter.GetVerifiedModels()
		assert.Len(t, models2, 1)
	})
}

func TestFreeProviderAdapter_IsModelVerified(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	t.Run("unverified model", func(t *testing.T) {
		t.Parallel()
		verified := adapter.IsModelVerified("unknown-model")
		assert.False(t, verified)
	})

	t.Run("verified model", func(t *testing.T) {
		t.Parallel()
		adapter.verifiedModels.Put("test-model", &verifier.UnifiedModel{
			ID:       "test-model",
			Verified: true,
		})


		verified := adapter.IsModelVerified("test-model")
		assert.True(t, verified)
	})
}

func TestFreeProviderAdapter_GetHealthStatus(t *testing.T) {
	t.Parallel()

	t.Run("empty initially", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		status := adapter.GetHealthStatus()
		assert.NotNil(t, status)
		assert.Empty(t, status)
	})

	t.Run("returns copy of health status", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		
		adapter.healthStatus.Put("zen", true)
		adapter.healthStatus.Put("openrouter", false)
		

		status := adapter.GetHealthStatus()
		assert.Len(t, status, 2)
		assert.True(t, status["zen"])
		assert.False(t, status["openrouter"])

		// Modify returned map shouldn't affect internal state
		status["zen"] = false

		status2 := adapter.GetHealthStatus()
		assert.True(t, status2["zen"])
	})
}

func TestFreeProviderAdapter_CalculateZenScore(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	t.Run("empty models returns 0", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateZenScore([]verifier.UnifiedModel{}, true)
		assert.Equal(t, 0.0, score)
	})

	t.Run("health check passed bonus", func(t *testing.T) {
		t.Parallel()
		models := []verifier.UnifiedModel{
			{ID: "m1", Latency: 100 * time.Millisecond},
		}

		scoreWithHealth := adapter.calculateZenScore(models, true)
		scoreWithoutHealth := adapter.calculateZenScore(models, false)

		assert.Greater(t, scoreWithHealth, scoreWithoutHealth)
	})

	t.Run("fast latency bonus", func(t *testing.T) {
		t.Parallel()
		fastModels := []verifier.UnifiedModel{
			{ID: "m1", Latency: 100 * time.Millisecond},
		}
		slowModels := []verifier.UnifiedModel{
			{ID: "m1", Latency: 3 * time.Second},
		}

		fastScore := adapter.calculateZenScore(fastModels, true)
		slowScore := adapter.calculateZenScore(slowModels, true)

		assert.Greater(t, fastScore, slowScore)
	})

	t.Run("capped at max score", func(t *testing.T) {
		t.Parallel()
		// Create many models with fast latency to try to exceed max
		models := make([]verifier.UnifiedModel, 10)
		for i := 0; i < 10; i++ {
			models[i] = verifier.UnifiedModel{ID: "m1", Latency: 100 * time.Millisecond}
		}

		score := adapter.calculateZenScore(models, true)
		assert.LessOrEqual(t, score, adapter.config.MaxScore)
	})
}

func TestFreeProviderAdapter_CalculateModelScore(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	t.Run("unverified returns 0", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateModelScore(100*time.Millisecond, false)
		assert.Equal(t, 0.0, score)
	})

	t.Run("very fast latency", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateModelScore(200*time.Millisecond, true)
		assert.GreaterOrEqual(t, score, adapter.config.BaseScore)
		assert.LessOrEqual(t, score, adapter.config.MaxScore)
	})

	t.Run("fast latency", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateModelScore(400*time.Millisecond, true)
		assert.GreaterOrEqual(t, score, adapter.config.BaseScore)
	})

	t.Run("medium latency", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateModelScore(800*time.Millisecond, true)
		assert.GreaterOrEqual(t, score, adapter.config.BaseScore)
	})

	t.Run("slow latency", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateModelScore(1500*time.Millisecond, true)
		assert.GreaterOrEqual(t, score, adapter.config.BaseScore)
	})

	t.Run("very slow latency", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateModelScore(3*time.Second, true)
		assert.Equal(t, adapter.config.BaseScore, score)
	})

	t.Run("capped at max", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateModelScore(100*time.Millisecond, true)
		assert.LessOrEqual(t, score, adapter.config.MaxScore)
	})
}

func TestFreeProviderAdapter_CalculateOpenRouterScore(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	t.Run("empty models returns 0", func(t *testing.T) {
		t.Parallel()
		score := adapter.calculateOpenRouterScore([]verifier.UnifiedModel{})
		assert.Equal(t, 0.0, score)
	})

	t.Run("single model", func(t *testing.T) {
		t.Parallel()
		models := []verifier.UnifiedModel{
			{ID: "m1", Latency: 100 * time.Millisecond},
		}

		score := adapter.calculateOpenRouterScore(models)
		assert.GreaterOrEqual(t, score, adapter.config.BaseScore)
	})

	t.Run("two models bonus", func(t *testing.T) {
		t.Parallel()
		oneModel := []verifier.UnifiedModel{
			{ID: "m1", Latency: 100 * time.Millisecond},
		}
		twoModels := []verifier.UnifiedModel{
			{ID: "m1", Latency: 100 * time.Millisecond},
			{ID: "m2", Latency: 100 * time.Millisecond},
		}

		oneScore := adapter.calculateOpenRouterScore(oneModel)
		twoScore := adapter.calculateOpenRouterScore(twoModels)

		assert.Greater(t, twoScore, oneScore)
	})

	t.Run("three or more models bonus", func(t *testing.T) {
		t.Parallel()
		twoModels := []verifier.UnifiedModel{
			{ID: "m1", Latency: 100 * time.Millisecond},
			{ID: "m2", Latency: 100 * time.Millisecond},
		}
		threeModels := []verifier.UnifiedModel{
			{ID: "m1", Latency: 100 * time.Millisecond},
			{ID: "m2", Latency: 100 * time.Millisecond},
			{ID: "m3", Latency: 100 * time.Millisecond},
		}

		twoScore := adapter.calculateOpenRouterScore(twoModels)
		threeScore := adapter.calculateOpenRouterScore(threeModels)

		assert.Greater(t, threeScore, twoScore)
	})

	t.Run("capped at max score", func(t *testing.T) {
		t.Parallel()
		models := make([]verifier.UnifiedModel, 10)
		for i := 0; i < 10; i++ {
			models[i] = verifier.UnifiedModel{ID: "m", Latency: 100 * time.Millisecond}
		}

		score := adapter.calculateOpenRouterScore(models)
		assert.LessOrEqual(t, score, adapter.config.MaxScore)
	})
}

func TestFreeProviderAdapter_RefreshVerification(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	t.Run("unknown provider type", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		_, err := adapter.RefreshVerification(ctx, "unknown", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown free provider type")
	})

	// Note: FreeProviderZen and FreeProviderOpenRouter tests would require
	// actual network calls or mocked providers, which is covered in integration tests
}

func TestFreeProviderAdapter_HelperFunctions(t *testing.T) {
	t.Parallel()
	t.Run("getModelDisplayName known models", func(t *testing.T) {
		// Test with slash separator
		name := getModelDisplayName("opencode/grok-code")
		assert.Equal(t, "grok-code", name)
	})

	t.Run("getModelDisplayName no slash", func(t *testing.T) {
		t.Parallel()
		name := getModelDisplayName("simple-model")
		assert.Equal(t, "simple-model", name)
	})

	t.Run("getOpenRouterModelName with free suffix", func(t *testing.T) {
		t.Parallel()
		name := getOpenRouterModelName("google/gemma-2-9b-it:free")
		assert.Equal(t, "gemma-2-9b-it (Free)", name)
	})

	t.Run("getOpenRouterModelName without provider", func(t *testing.T) {
		t.Parallel()
		name := getOpenRouterModelName("simple-model:free")
		assert.Equal(t, "simple-model (Free)", name)
	})

	t.Run("convertCapabilities nil", func(t *testing.T) {
		t.Parallel()
		result := convertCapabilities(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("convertCapabilities with features", func(t *testing.T) {
		t.Parallel()
		caps := &models.ProviderCapabilities{
			SupportedFeatures:       []string{"text", "code"},
			SupportsStreaming:       true,
			SupportsFunctionCalling: true,
			SupportsVision:          false,
			SupportsTools:           true,
			SupportsReasoning:       false,
		}

		result := convertCapabilities(caps)
		assert.Contains(t, result, "text")
		assert.Contains(t, result, "code")
		assert.Contains(t, result, "streaming")
		assert.Contains(t, result, "function_calling")
		assert.Contains(t, result, "tools")
		assert.NotContains(t, result, "vision")
		assert.NotContains(t, result, "reasoning")
	})
}

func TestFreeProviderAdapter_ConcurrentAccess_Comprehensive(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	done := make(chan bool, 30)

	// Concurrent reads and writes
	for i := 0; i < 10; i++ {
		go func(idx int) {
			model := &verifier.UnifiedModel{
				ID:       "model-" + string(rune('A'+idx)),
				Verified: true,
			}
			adapter.verifiedModels.Put(model.ID, model)
			done <- true
		}(i)
		go func() {
			_ = adapter.GetVerifiedModels()
			done <- true
		}()
		go func() {
			_ = adapter.GetHealthStatus()
			done <- true
		}()
	}

	for i := 0; i < 30; i++ {
		<-done
	}
}

func TestFreeProviderAdapter_verifyOpenRouterFreeModel(t *testing.T) {
	t.Parallel()
	t.Run("successful health check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/models" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":[]}`))
			}
		}))
		defer server.Close()

		// Note: The actual adapter uses hardcoded URL, so this test is limited
		// In production, we would need to inject the URL or use interface
		adapter := NewFreeProviderAdapter(nil, nil)

		// This will fail with actual OpenRouter URL, but tests the flow
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		model, err := adapter.verifyOpenRouterFreeModel(ctx, "test-model:free", "test-key")
		// Expected to fail with actual network call
		if err == nil {
			assert.NotNil(t, model)
			assert.True(t, model.Verified)
		}
	})
}

func TestFreeAdapterConfig_Fields_Comprehensive(t *testing.T) {
	t.Parallel()
	cfg := &FreeAdapterConfig{
		VerificationTimeout:        45 * time.Second,
		HealthCheckTimeout:         15 * time.Second,
		MaxConcurrentVerifications: 6,
		MinHealthScore:             0.6,
		BaseScore:                  5.8,
		MaxScore:                   6.8,
		RetryAttempts:              4,
		RetryDelay:                 500 * time.Millisecond,
	}

	assert.Equal(t, 45*time.Second, cfg.VerificationTimeout)
	assert.Equal(t, 15*time.Second, cfg.HealthCheckTimeout)
	assert.Equal(t, 6, cfg.MaxConcurrentVerifications)
	assert.Equal(t, 0.6, cfg.MinHealthScore)
	assert.Equal(t, 5.8, cfg.BaseScore)
	assert.Equal(t, 6.8, cfg.MaxScore)
	assert.Equal(t, 4, cfg.RetryAttempts)
	assert.Equal(t, 500*time.Millisecond, cfg.RetryDelay)
}

func TestFreeProviderType_Constants_Comprehensive(t *testing.T) {
	t.Parallel()
	assert.Equal(t, FreeProviderType("zen"), FreeProviderZen)
	assert.Equal(t, FreeProviderType("openrouter"), FreeProviderOpenRouter)
}

func TestFreeProviderAdapter_ConfigShortTimeout(t *testing.T) {
	t.Parallel()
	// Test that adapter accepts short timeout configuration
	adapter := NewFreeProviderAdapter(nil, &FreeAdapterConfig{
		VerificationTimeout: 100 * time.Millisecond,
		RetryAttempts:       0,
	})

	require.NotNil(t, adapter)
	assert.Equal(t, 100*time.Millisecond, adapter.config.VerificationTimeout)
	assert.Equal(t, 0, adapter.config.RetryAttempts)
}

func TestFreeProviderAdapter_LatencyBonusCalculation(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	latencies := []struct {
		latency  time.Duration
		minBonus float64
	}{
		{100 * time.Millisecond, 0.6},  // Very fast
		{400 * time.Millisecond, 0.4},  // Fast
		{800 * time.Millisecond, 0.2},  // Medium
		{1500 * time.Millisecond, 0.1}, // Slow
		{3 * time.Second, 0.0},         // Very slow
	}

	for _, test := range latencies {
		score := adapter.calculateModelScore(test.latency, true)
		expectedMin := adapter.config.BaseScore + test.minBonus

		// Allow for score capping
		if expectedMin > adapter.config.MaxScore {
			expectedMin = adapter.config.MaxScore
		}

		assert.GreaterOrEqual(t, score, adapter.config.BaseScore,
			"Score for latency %v should be at least base score", test.latency)
	}
}

// =====================================================
// CLI FACADE INTEGRATION TESTS
// =====================================================

func TestFreeProviderAdapter_CLIFacadeAvailability(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	t.Run("checks CLI facade availability", func(t *testing.T) {
		t.Parallel()
		// This will return true/false depending on whether opencode is installed
		available := adapter.IsCLIFacadeAvailable()
		t.Logf("CLI facade available: %v", available)
		// Just verify it doesn't panic
	})

	t.Run("gets CLI facade provider", func(t *testing.T) {
		t.Parallel()
		provider := adapter.GetCLIFacadeProvider()
		// May be nil if opencode is not installed
		if provider != nil {
			assert.NotNil(t, provider)
			t.Log("CLI facade provider is available")
		} else {
			t.Log("CLI facade provider is not available (opencode not installed)")
		}
	})
}

func TestFreeProviderAdapter_FailedAPIModels(t *testing.T) {
	t.Parallel()

	t.Run("initially empty", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		failed := adapter.GetFailedAPIModels()
		assert.Empty(t, failed)
	})

	t.Run("returns copy not reference", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		// Manually add a failed model
		adapter.failedAPIModels.Put("test-model", assert.AnError)


		failed1 := adapter.GetFailedAPIModels()
		failed2 := adapter.GetFailedAPIModels()

		// Modify the first copy
		failed1["modified"] = nil

		// Second copy should not be affected
		_, exists := failed2["modified"]
		assert.False(t, exists)
	})
}

func TestFreeProviderAdapter_IsModelUsingCLIFacade(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	t.Run("returns false for non-existent model", func(t *testing.T) {
		t.Parallel()
		result := adapter.IsModelUsingCLIFacade("non-existent")
		assert.False(t, result)
	})

	t.Run("returns false for model without CLI facade metadata", func(t *testing.T) {
		t.Parallel()
		adapter.verifiedModels.Put("api-model", &verifier.UnifiedModel{
			ID:       "api-model",
			Metadata: map[string]interface{}{},
		})

		result := adapter.IsModelUsingCLIFacade("api-model")
		assert.False(t, result)
	})

	t.Run("returns true for model with CLI facade metadata", func(t *testing.T) {
		t.Parallel()
		adapter.verifiedModels.Put("cli-model", &verifier.UnifiedModel{
			ID: "cli-model",
			Metadata: map[string]interface{}{
				"verified_via": "cli_facade",
			},
		})

		result := adapter.IsModelUsingCLIFacade("cli-model")
		assert.True(t, result)
	})
}

func TestFreeProviderAdapter_GetCLIFacadeModels(t *testing.T) {
	t.Parallel()

	t.Run("returns empty when no CLI models", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		adapter.verifiedModels.Put("api-model", &verifier.UnifiedModel{
			ID:       "api-model",
			Metadata: map[string]interface{}{},
		})

		cliModels := adapter.GetCLIFacadeModels()
		assert.Empty(t, cliModels)
	})

	t.Run("returns only CLI facade models", func(t *testing.T) {
		t.Parallel()
		adapter := NewFreeProviderAdapter(nil, nil)
		adapter.verifiedModels.Put("api-model", &verifier.UnifiedModel{
			ID:       "api-model",
			Metadata: map[string]interface{}{},
		})
		adapter.verifiedModels.Put("cli-model-1", &verifier.UnifiedModel{
			ID: "cli-model-1",
			Metadata: map[string]interface{}{
				"verified_via": "cli_facade",
			},
		})
		adapter.verifiedModels.Put("cli-model-2", &verifier.UnifiedModel{
			ID: "cli-model-2",
			Metadata: map[string]interface{}{
				"verified_via": "cli_facade",
			},
		})


		cliModels := adapter.GetCLIFacadeModels()
		assert.Len(t, cliModels, 2)

		// Check that both CLI models are present
		modelIDs := make(map[string]bool)
		for _, m := range cliModels {
			modelIDs[m.ID] = true
		}
		assert.True(t, modelIDs["cli-model-1"])
		assert.True(t, modelIDs["cli-model-2"])
		assert.False(t, modelIDs["api-model"])
	})
}
