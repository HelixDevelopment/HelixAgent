// Package adapters provides provider-specific verification adapters for the startup verification system.
package adapters

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	verifier "dev.helix.agent/internal/verifier"
)

func TestFreeProviderType_Constants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, FreeProviderType("zen"), FreeProviderZen)
	assert.Equal(t, FreeProviderType("openrouter"), FreeProviderOpenRouter)
}

func TestDefaultFreeAdapterConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, 30*time.Second, cfg.VerificationTimeout)
	assert.Equal(t, 10*time.Second, cfg.HealthCheckTimeout)
	assert.Equal(t, 4, cfg.MaxConcurrentVerifications)
	assert.Equal(t, 0.5, cfg.MinHealthScore)
	assert.Equal(t, 6.0, cfg.BaseScore)
	assert.Equal(t, 7.0, cfg.MaxScore)
	assert.Equal(t, 2, cfg.RetryAttempts)
	assert.Equal(t, 1*time.Second, cfg.RetryDelay)
}

func TestNewFreeProviderAdapter(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	require.NotNil(t, adapter)
	assert.NotNil(t, adapter.config)
	assert.NotNil(t, adapter.httpClient)
	assert.NotNil(t, adapter.verifiedModels)
	assert.NotNil(t, adapter.lastVerified)
	assert.NotNil(t, adapter.healthStatus)
	assert.Nil(t, adapter.verifierSvc)
	assert.Nil(t, adapter.zenProvider)
}

func TestNewFreeProviderAdapter_WithConfig(t *testing.T) {
	t.Parallel()
	customConfig := &FreeAdapterConfig{
		VerificationTimeout:        45 * time.Second,
		HealthCheckTimeout:         15 * time.Second,
		MaxConcurrentVerifications: 8,
		MinHealthScore:             0.7,
		BaseScore:                  5.5,
		MaxScore:                   7.5,
		RetryAttempts:              3,
		RetryDelay:                 2 * time.Second,
	}

	adapter := NewFreeProviderAdapter(nil, customConfig)

	require.NotNil(t, adapter)
	assert.Equal(t, 45*time.Second, adapter.config.VerificationTimeout)
	assert.Equal(t, 15*time.Second, adapter.config.HealthCheckTimeout)
	assert.Equal(t, 8, adapter.config.MaxConcurrentVerifications)
	assert.Equal(t, 0.7, adapter.config.MinHealthScore)
	assert.Equal(t, 5.5, adapter.config.BaseScore)
	assert.Equal(t, 7.5, adapter.config.MaxScore)
	assert.Equal(t, 3, adapter.config.RetryAttempts)
	assert.Equal(t, 2*time.Second, adapter.config.RetryDelay)
}

func TestFreeAdapterConfig_Fields(t *testing.T) {
	t.Parallel()
	cfg := &FreeAdapterConfig{
		VerificationTimeout:        20 * time.Second,
		HealthCheckTimeout:         5 * time.Second,
		MaxConcurrentVerifications: 2,
		MinHealthScore:             0.6,
		BaseScore:                  6.5,
		MaxScore:                   8.0,
		RetryAttempts:              1,
		RetryDelay:                 500 * time.Millisecond,
	}

	assert.Equal(t, 20*time.Second, cfg.VerificationTimeout)
	assert.Equal(t, 5*time.Second, cfg.HealthCheckTimeout)
	assert.Equal(t, 2, cfg.MaxConcurrentVerifications)
	assert.Equal(t, 0.6, cfg.MinHealthScore)
	assert.Equal(t, 6.5, cfg.BaseScore)
	assert.Equal(t, 8.0, cfg.MaxScore)
	assert.Equal(t, 1, cfg.RetryAttempts)
	assert.Equal(t, 500*time.Millisecond, cfg.RetryDelay)
}

func TestFreeAdapterConfig_ZeroValue(t *testing.T) {
	t.Parallel()
	cfg := &FreeAdapterConfig{}

	assert.Equal(t, time.Duration(0), cfg.VerificationTimeout)
	assert.Equal(t, time.Duration(0), cfg.HealthCheckTimeout)
	assert.Equal(t, 0, cfg.MaxConcurrentVerifications)
	assert.Equal(t, 0.0, cfg.MinHealthScore)
	assert.Equal(t, 0.0, cfg.BaseScore)
	assert.Equal(t, 0.0, cfg.MaxScore)
	assert.Equal(t, 0, cfg.RetryAttempts)
	assert.Equal(t, time.Duration(0), cfg.RetryDelay)
}

func TestFreeProviderAdapter_GetVerifiedModels_Empty(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	models := adapter.GetVerifiedModels()

	assert.NotNil(t, models)
	assert.Empty(t, models)
}

func TestFreeProviderAdapter_GetVerifiedModels_WithModels(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	// Add some verified models manually
	adapter.verifiedModels.Put("model-1", &verifier.UnifiedModel{
		ID:       "model-1",
		Name:     "Test Model 1",
		Provider: "zen",
		Verified: true,
		Score:    6.5,
	})
	adapter.verifiedModels.Put("model-2", &verifier.UnifiedModel{
		ID:       "model-2",
		Name:     "Test Model 2",
		Provider: "zen",
		Verified: true,
		Score:    6.8,
	})


	models := adapter.GetVerifiedModels()

	assert.Len(t, models, 2)
	assert.Contains(t, models, "model-1")
	assert.Contains(t, models, "model-2")
}

func TestFreeProviderAdapter_IsModelVerified_False(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	verified := adapter.IsModelVerified("non-existent-model")

	assert.False(t, verified)
}

func TestFreeProviderAdapter_IsModelVerified_True(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	// Add a verified model
	adapter.verifiedModels.Put("test-model", &verifier.UnifiedModel{
		ID:       "test-model",
		Name:     "Test Model",
		Provider: "zen",
		Verified: true,
	})


	verified := adapter.IsModelVerified("test-model")

	assert.True(t, verified)
}

func TestFreeProviderAdapter_GetHealthStatus_Empty(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	status := adapter.GetHealthStatus()

	assert.NotNil(t, status)
	assert.Empty(t, status)
}

func TestFreeProviderAdapter_GetHealthStatus_WithStatus(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	// Add health status
	
	adapter.healthStatus.Put("zen", true)
	adapter.healthStatus.Put("openrouter", false)
	

	status := adapter.GetHealthStatus()

	assert.Len(t, status, 2)
	assert.True(t, status["zen"])
	assert.False(t, status["openrouter"])
}

func TestFreeProviderAdapter_CalculateModelScore_NotVerified(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	score := adapter.calculateModelScore(100*time.Millisecond, false)

	assert.Equal(t, 0.0, score)
}

func TestFreeProviderAdapter_CalculateModelScore_FastLatency(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	score := adapter.calculateModelScore(200*time.Millisecond, true)

	// Base score (6.0) + fast latency bonus (0.6) = 6.6
	assert.Equal(t, 6.6, score)
}

func TestFreeProviderAdapter_CalculateModelScore_MediumLatency(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	score := adapter.calculateModelScore(400*time.Millisecond, true)

	// Base score (6.0) + medium latency bonus (0.4) = 6.4
	assert.Equal(t, 6.4, score)
}

func TestFreeProviderAdapter_CalculateModelScore_SlowLatency(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	score := adapter.calculateModelScore(800*time.Millisecond, true)

	// Base score (6.0) + slow latency bonus (0.2) = 6.2
	assert.Equal(t, 6.2, score)
}

func TestFreeProviderAdapter_CalculateModelScore_VerySlowLatency(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	score := adapter.calculateModelScore(1500*time.Millisecond, true)

	// Base score (6.0) + very slow latency bonus (0.1) = 6.1
	assert.Equal(t, 6.1, score)
}

func TestFreeProviderAdapter_CalculateModelScore_ExtremelySlowLatency(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	score := adapter.calculateModelScore(5*time.Second, true)

	// Base score (6.0) + no bonus = 6.0
	assert.Equal(t, 6.0, score)
}

func TestFreeProviderAdapter_CalculateModelScore_CappedAtMax(t *testing.T) {
	t.Parallel()
	cfg := &FreeAdapterConfig{
		BaseScore: 6.5,
		MaxScore:  7.0,
	}
	adapter := NewFreeProviderAdapter(nil, cfg)

	// With fast latency (200ms), would be 6.5 + 0.6 = 7.1, but capped at 7.0
	score := adapter.calculateModelScore(200*time.Millisecond, true)

	assert.Equal(t, 7.0, score)
}

func TestFreeProviderAdapter_CalculateZenScore_NoModels(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	score := adapter.calculateZenScore([]verifier.UnifiedModel{}, true)

	assert.Equal(t, 0.0, score)
}

func TestFreeProviderAdapter_CalculateZenScore_WithModels_HealthPassed(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	models := []verifier.UnifiedModel{
		{ID: "model-1", Latency: 200 * time.Millisecond},
		{ID: "model-2", Latency: 300 * time.Millisecond},
	}

	score := adapter.calculateZenScore(models, true)

	// Base score + health bonus + verified ratio bonus + latency bonus
	// Score should be between 6.0 and 7.0
	assert.GreaterOrEqual(t, score, 6.0)
	assert.LessOrEqual(t, score, 7.0)
}

func TestFreeProviderAdapter_CalculateZenScore_WithModels_HealthFailed(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	models := []verifier.UnifiedModel{
		{ID: "model-1", Latency: 200 * time.Millisecond},
	}

	// Health failed -> no health bonus
	scoreHealthFailed := adapter.calculateZenScore(models, false)
	scoreHealthPassed := adapter.calculateZenScore(models, true)

	// Score with health passed should be higher
	assert.Less(t, scoreHealthFailed, scoreHealthPassed)
}

func TestFreeProviderAdapter_CalculateOpenRouterScore_NoModels(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	score := adapter.calculateOpenRouterScore([]verifier.UnifiedModel{})

	assert.Equal(t, 0.0, score)
}

func TestFreeProviderAdapter_CalculateOpenRouterScore_OneModel(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	models := []verifier.UnifiedModel{
		{ID: "model-1", Latency: 200 * time.Millisecond},
	}

	score := adapter.calculateOpenRouterScore(models)

	// Score should be between 6.0 and 7.0
	assert.GreaterOrEqual(t, score, 6.0)
	assert.LessOrEqual(t, score, 7.0)
}

func TestFreeProviderAdapter_CalculateOpenRouterScore_MultipleModels(t *testing.T) {
	t.Parallel()
	cfg := DefaultFreeAdapterConfig()
	adapter := NewFreeProviderAdapter(nil, cfg)

	models := []verifier.UnifiedModel{
		{ID: "model-1", Latency: 200 * time.Millisecond},
		{ID: "model-2", Latency: 300 * time.Millisecond},
		{ID: "model-3", Latency: 400 * time.Millisecond},
	}

	score := adapter.calculateOpenRouterScore(models)

	// Score should be higher with multiple models
	assert.GreaterOrEqual(t, score, 6.0)
	assert.LessOrEqual(t, score, 7.0)
}

func TestGetModelDisplayName_Known(t *testing.T) {
	t.Parallel()
	tests := []struct {
		modelID  string
		expected string
	}{
		// Known models in nameMap
		{"big-pickle", "Big Pickle"},
		{"glm-5-free", "GLM 5 Free"},
		{"minimax-m2.5-free", "Minimax M2.5 Free"},
		{"minimax-m2.1-free", "Minimax M2.1 Free"},
		{"trinity-large-preview-free", "Trinity Large Preview"},
		// Unknown models - return as-is
		{"grok-code", "grok-code"},
		{"glm-4.7-free", "glm-4.7-free"},
		{"gpt-5-nano", "gpt-5-nano"},
		// Legacy model IDs (with opencode/ prefix) - also tested for backward compat
		{"opencode/big-pickle", "big-pickle"},
		{"opencode/grok-code", "grok-code"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			t.Parallel()
			name := getModelDisplayName(tt.modelID)
			assert.Equal(t, tt.expected, name)
		})
	}
}

func TestGetModelDisplayName_Unknown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		modelID  string
		expected string
	}{
		{"provider/model-name", "model-name"},
		{"some-model", "some-model"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			t.Parallel()
			name := getModelDisplayName(tt.modelID)
			assert.Equal(t, tt.expected, name)
		})
	}
}

func TestGetOpenRouterModelName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		modelID  string
		expected string
	}{
		{"google/gemma-2-9b-it:free", "gemma-2-9b-it (Free)"},
		{"meta-llama/llama-3-8b-instruct:free", "llama-3-8b-instruct (Free)"},
		{"mistralai/mistral-7b-instruct:free", "mistral-7b-instruct (Free)"},
		{"qwen/qwen-2-7b-instruct:free", "qwen-2-7b-instruct (Free)"},
		{"simple-model:free", "simple-model (Free)"},
		{"simple-model", "simple-model (Free)"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			t.Parallel()
			name := getOpenRouterModelName(tt.modelID)
			assert.Equal(t, tt.expected, name)
		})
	}
}

func TestConvertCapabilities_Nil(t *testing.T) {
	t.Parallel()
	caps := convertCapabilities(nil)

	assert.NotNil(t, caps)
	assert.Empty(t, caps)
}

func TestConvertCapabilities_Empty(t *testing.T) {
	t.Parallel()
	caps := convertCapabilities(&models.ProviderCapabilities{})

	assert.NotNil(t, caps)
	assert.Empty(t, caps)
}

func TestConvertCapabilities_WithFeatures(t *testing.T) {
	t.Parallel()
	caps := convertCapabilities(&models.ProviderCapabilities{
		SupportedFeatures:       []string{"chat", "completion"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoning:       true,
	})

	assert.Contains(t, caps, "chat")
	assert.Contains(t, caps, "completion")
	assert.Contains(t, caps, "streaming")
	assert.Contains(t, caps, "function_calling")
	assert.Contains(t, caps, "vision")
	assert.Contains(t, caps, "tools")
	assert.Contains(t, caps, "reasoning")
}

func TestFreeProviderAdapter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	done := make(chan bool, 4)

	// Writer for verified models
	go func() {
		for i := 0; i < 100; i++ {
			adapter.verifiedModels.Put("test-model", &verifier.UnifiedModel{
				ID:   "test-model",
				Name: "Test Model",
			})
		}
		done <- true
	}()

	// Reader for verified models
	go func() {
		for i := 0; i < 100; i++ {
			_ = adapter.GetVerifiedModels()
		}
		done <- true
	}()

	// Writer for health status
	go func() {
		for i := 0; i < 100; i++ {
			
			adapter.healthStatus.Put("zen", true)
			
		}
		done <- true
	}()

	// Reader for health status
	go func() {
		for i := 0; i < 100; i++ {
			_ = adapter.GetHealthStatus()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestFreeProviderAdapter_RefreshVerification_UnknownProvider(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, nil)

	_, err := adapter.RefreshVerification(nil, "unknown", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown free provider type")
}

// TestFreeProviderAdapter_ConcurrentVerify_NoRace is the CONST-029 regression
// sentinel. Pre-migration, the shared maps (verifiedModels, lastVerified,
// failedAPIModels, healthStatus) were guarded by fa.mu on reader paths and
// by a *per-call* modelsMu on writer paths — so a reader holding
// fa.mu.RLock() could observe partial map writes from a verify goroutine
// holding only the local modelsMu. After the migration to safe.Store the
// race is structurally impossible. This test stresses 100 concurrent
// verify/read cycles and MUST pass under `go test -race`.
func TestFreeProviderAdapter_ConcurrentVerify_NoRace(t *testing.T) {
	t.Parallel()
	adapter := NewFreeProviderAdapter(nil, DefaultFreeAdapterConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Direct map writes exercise the same code path writers take
			// under verify; safe.Store.Put is the serialisation point.
			adapter.verifiedModels.Put("race-model", &verifier.UnifiedModel{
				ID:       "race-model",
				Verified: true,
			})
			adapter.lastVerified.Put("race-model", time.Now())
			adapter.healthStatus.Put("zen", true)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = adapter.GetVerifiedModels()
			_ = adapter.GetHealthStatus()
			_ = adapter.GetFailedAPIModels()
			_ = adapter.IsModelVerified("race-model")
		}()
	}
	wg.Wait()
	_ = ctx

	// Final state must be internally consistent: any model present in
	// verifiedModels must also report IsModelVerified=true.
	models := adapter.GetVerifiedModels()
	for id := range models {
		if !adapter.IsModelVerified(id) {
			t.Fatalf("inconsistent state: %s in map but IsModelVerified=false", id)
		}
	}
}
