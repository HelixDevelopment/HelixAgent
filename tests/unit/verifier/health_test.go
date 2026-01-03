package verifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/verifier"
)

func TestHealthService_AddRemoveProvider(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	// Add provider
	service.AddProvider("openai-1", "openai")

	// Get health
	health, err := service.GetProviderHealth("openai-1")
	require.NoError(t, err)
	assert.Equal(t, "openai-1", health.ProviderID)
	assert.Equal(t, "openai", health.ProviderName)
	assert.True(t, health.Healthy)
	assert.Equal(t, "closed", health.CircuitState)

	// Remove provider
	service.RemoveProvider("openai-1")

	// Should return error for removed provider
	_, err = service.GetProviderHealth("openai-1")
	assert.Error(t, err)
}

func TestHealthService_GetAllProviderHealth(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	providers := []struct {
		id   string
		name string
	}{
		{"openai-1", "openai"},
		{"anthropic-1", "anthropic"},
		{"google-1", "google"},
	}

	for _, p := range providers {
		service.AddProvider(p.id, p.name)
	}

	allHealth := service.GetAllProviderHealth()
	assert.Len(t, allHealth, 3)

	// Should be sorted by name
	for i := 1; i < len(allHealth); i++ {
		assert.LessOrEqual(t, allHealth[i-1].ProviderName, allHealth[i].ProviderName)
	}
}

func TestHealthService_GetHealthyProviders(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("openai-1", "openai")
	service.AddProvider("anthropic-1", "anthropic")

	healthy := service.GetHealthyProviders()
	assert.Len(t, healthy, 2)
}

func TestHealthService_RecordSuccessFailure(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("test-1", "test")

	// Record successes
	for i := 0; i < 5; i++ {
		service.RecordSuccess("test-1")
	}

	health, err := service.GetProviderHealth("test-1")
	require.NoError(t, err)
	assert.Equal(t, 5, health.SuccessCount)
	assert.True(t, health.Healthy)

	// Record failures
	for i := 0; i < 5; i++ {
		service.RecordFailure("test-1")
	}

	health, err = service.GetProviderHealth("test-1")
	require.NoError(t, err)
	assert.Equal(t, 5, health.FailureCount)
}

func TestHealthService_IsProviderAvailable(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("test-1", "test")

	// Initially available
	assert.True(t, service.IsProviderAvailable("test-1"))

	// Non-existent provider
	assert.False(t, service.IsProviderAvailable("non-existent"))
}

func TestHealthService_GetProviderLatency(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("test-1", "test")

	latency, err := service.GetProviderLatency("test-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, latency, int64(0))

	// Non-existent provider
	_, err = service.GetProviderLatency("non-existent")
	assert.Error(t, err)
}

func TestHealthService_GetFastestProvider(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("fast-1", "fast")
	service.AddProvider("slow-1", "slow")

	providers := []string{"fast-1", "slow-1"}
	fastest, err := service.GetFastestProvider(providers)

	require.NoError(t, err)
	assert.Contains(t, providers, fastest)

	// Empty list
	_, err = service.GetFastestProvider([]string{})
	assert.Error(t, err)
}

func TestHealthService_ExecuteWithFailover(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("primary-1", "primary")
	service.AddProvider("backup-1", "backup")

	ctx := context.Background()
	callCount := 0

	err := service.ExecuteWithFailover(ctx, []string{"primary-1", "backup-1"}, func(providerID string) error {
		callCount++
		if providerID == "primary-1" {
			return assert.AnError
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, callCount) // Primary failed, backup succeeded
}

func TestHealthService_CircuitBreaker(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("test-1", "test")

	cb := service.GetCircuitBreaker("test-1")
	require.NotNil(t, cb)
	assert.True(t, cb.IsAvailable())

	// Non-existent provider
	cb = service.GetCircuitBreaker("non-existent")
	assert.Nil(t, cb)
}

func TestHealthService_StartStop(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	cfg.Health.CheckInterval = 100 * time.Millisecond
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("test-1", "test")

	// Start
	err := service.Start()
	require.NoError(t, err)

	// Starting again should error
	err = service.Start()
	assert.Error(t, err)

	// Wait a bit for health check to run
	time.Sleep(150 * time.Millisecond)

	// Stop
	service.Stop()

	// Stopping again should be safe
	service.Stop()
}

func TestHealthService_Concurrency(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	for i := 0; i < 10; i++ {
		service.AddProvider("test-"+string(rune('a'+i)), "test")
	}

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			service.GetAllProviderHealth()
			service.GetHealthyProviders()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(i int) {
			service.RecordSuccess("test-a")
			service.RecordFailure("test-b")
			done <- true
		}(i)
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestHealthService_UptimeCalculation(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service := verifier.NewHealthService(nil, cfg)

	service.AddProvider("test-1", "test")

	// Record 7 successes and 3 failures = 70% uptime
	for i := 0; i < 7; i++ {
		service.RecordSuccess("test-1")
	}
	for i := 0; i < 3; i++ {
		service.RecordFailure("test-1")
	}

	health, err := service.GetProviderHealth("test-1")
	require.NoError(t, err)
	assert.InDelta(t, 70.0, health.UptimePercent, 1.0)
}
