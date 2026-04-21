// Previously tests/integration/provider_registry_advanced_integration_test.go.
// Demoted alongside provider_test.go in PR25 of the CONST-030 compliance
// campaign — the file exercises services.ProviderRegistry against local
// MockLLMProvider siblings in-process only.
package provider_legacy_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// =============================================================================
// Test: Provider Registry — Register and Get (round-trip validation)
// =============================================================================

func TestIntegration_ProviderRegistry_RegisterAndGet(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	provider := NewMockLLMProvider("integration-reg-get")
	provider.capabilities = &models.ProviderCapabilities{
		SupportedModels:   []string{"model-alpha", "model-beta"},
		SupportsStreaming: true,
		SupportsTools:     true,
		Limits: models.ModelLimits{
			MaxTokens:             8192,
			MaxInputLength:        16384,
			MaxOutputLength:       8192,
			MaxConcurrentRequests: 20,
		},
		Metadata: map[string]string{
			"tier": "premium",
		},
	}

	err := registry.RegisterProvider("integration-reg-get", provider)
	require.NoError(t, err, "RegisterProvider should succeed for new provider")

	// Retrieve the provider and verify it is the same instance
	retrieved, err := registry.GetProvider("integration-reg-get")
	require.NoError(t, err, "GetProvider should find the registered provider")
	require.NotNil(t, retrieved, "Retrieved provider must not be nil")

	// Validate capabilities survived the round-trip
	caps := retrieved.GetCapabilities()
	require.NotNil(t, caps)
	assert.Equal(t, []string{"model-alpha", "model-beta"}, caps.SupportedModels)
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.Equal(t, 8192, caps.Limits.MaxTokens)
	assert.Equal(t, "premium", caps.Metadata["tier"])

	// Verify the provider can actually serve requests
	ctx := context.Background()
	req := &models.LLMRequest{
		ID:       "roundtrip-req",
		Prompt:   "Hello from round-trip test",
		Messages: []models.Message{{Role: "user", Content: "Hello from round-trip test"}},
	}

	resp, err := retrieved.Complete(ctx, req)
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "Hello from round-trip test")
}

// =============================================================================
// Test: Provider Registry — List Providers (multiple registrations)
// =============================================================================

func TestIntegration_ProviderRegistry_ListProviders(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	providerNames := []string{
		"list-provider-alpha",
		"list-provider-beta",
		"list-provider-gamma",
		"list-provider-delta",
		"list-provider-epsilon",
	}

	for _, name := range providerNames {
		p := NewMockLLMProvider(name)
		err := registry.RegisterProvider(name, p)
		require.NoError(t, err, "RegisterProvider(%s) should succeed", name)
	}

	listed := registry.ListProviders()
	assert.Len(t, listed, len(providerNames), "Listed providers count must match registered count")

	for _, expected := range providerNames {
		assert.Contains(t, listed, expected, "Listed providers must contain %s", expected)
	}

	// Verify ordered listing also contains all providers
	ordered := registry.ListProvidersOrderedByScore()
	assert.Len(t, ordered, len(providerNames))
	for _, expected := range providerNames {
		assert.Contains(t, ordered, expected)
	}
}

// =============================================================================
// Test: Provider Registry — Duplicate Registration handling
// =============================================================================

func TestIntegration_ProviderRegistry_DuplicateRegistration(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	provider1 := NewMockLLMProvider("dup-provider")
	provider1.SetCompleteResponse(&models.LLMResponse{
		ID:      "resp-1",
		Content: "Response from provider1",
	})

	err := registry.RegisterProvider("dup-provider", provider1)
	require.NoError(t, err)

	// Attempt duplicate registration — must fail
	provider2 := NewMockLLMProvider("dup-provider")
	provider2.SetCompleteResponse(&models.LLMResponse{
		ID:      "resp-2",
		Content: "Response from provider2",
	})

	err = registry.RegisterProvider("dup-provider", provider2)
	assert.Error(t, err, "Duplicate registration must return an error")
	assert.Contains(t, err.Error(), "already registered",
		"Error message must indicate the provider is already registered")

	// Verify the original provider is still in place
	retrieved, err := registry.GetProvider("dup-provider")
	require.NoError(t, err)

	resp, err := retrieved.Complete(context.Background(), &models.LLMRequest{
		ID: "dup-check", Prompt: "check",
	})
	require.NoError(t, err)
	assert.Equal(t, "resp-1", resp.ID, "Original provider must remain registered")
}

// =============================================================================
// Test: Provider Registry — Get Non-Existent provider
// =============================================================================

func TestIntegration_ProviderRegistry_GetNonExistent(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register one provider to ensure the registry is not empty
	err := registry.RegisterProvider("existing-one", NewMockLLMProvider("existing-one"))
	require.NoError(t, err)

	// Attempt to get a provider that does not exist
	_, err = registry.GetProvider("totally-nonexistent-provider")
	require.Error(t, err, "GetProvider for unknown name must return an error")

	// Verify the error is descriptive
	errMsg := err.Error()
	assert.True(t,
		len(errMsg) > 0,
		"Error message must not be empty")

	// Try with empty string
	_, err = registry.GetProvider("")
	assert.Error(t, err, "GetProvider with empty string must return an error")
}

// =============================================================================
// Test: Provider Registry — Concurrent Access (10+ goroutines)
// =============================================================================

func TestIntegration_ProviderRegistry_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent access test in short mode")
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Pre-register 5 providers
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("concurrent-pre-%d", i)
		err := registry.RegisterProvider(name, NewMockLLMProvider(name))
		require.NoError(t, err)
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	var registerSuccesses int64
	var registerFailures int64
	var getSuccesses int64
	var getFailures int64
	var listCalls int64

	// Each goroutine performs a mix of operations
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Register a unique provider per goroutine
			provName := fmt.Sprintf("concurrent-g%d", id)
			p := NewMockLLMProvider(provName)
			if err := registry.RegisterProvider(provName, p); err != nil {
				atomic.AddInt64(&registerFailures, 1)
			} else {
				atomic.AddInt64(&registerSuccesses, 1)
			}

			// Read operations
			for i := 0; i < 20; i++ {
				// List providers
				providers := registry.ListProviders()
				atomic.AddInt64(&listCalls, 1)
				_ = providers

				// Get a pre-registered provider
				target := fmt.Sprintf("concurrent-pre-%d", i%5)
				if _, err := registry.GetProvider(target); err == nil {
					atomic.AddInt64(&getSuccesses, 1)
				} else {
					atomic.AddInt64(&getFailures, 1)
				}

				// Health check (read-heavy)
				_ = registry.HealthCheck()

				// Ordered listing
				_ = registry.ListProvidersOrderedByScore()
			}
		}(g)
	}

	wg.Wait()

	// Validate: all goroutines should have registered successfully (unique names)
	assert.Equal(t, int64(numGoroutines), atomic.LoadInt64(&registerSuccesses),
		"All goroutine registrations should succeed (unique names)")
	assert.Equal(t, int64(0), atomic.LoadInt64(&registerFailures),
		"No registration should fail")

	// All gets of pre-registered providers should succeed
	assert.Equal(t, int64(0), atomic.LoadInt64(&getFailures),
		"All gets of pre-registered providers should succeed")
	assert.Greater(t, atomic.LoadInt64(&getSuccesses), int64(0))

	// List calls should have executed
	assert.Equal(t, int64(numGoroutines*20), atomic.LoadInt64(&listCalls))

	// Final state: 5 pre-registered + 10 goroutine-registered = 15
	allProviders := registry.ListProviders()
	assert.Len(t, allProviders, 15, "Must have 5 pre-registered + 10 goroutine providers")
}

// =============================================================================
// Test: Provider Registry — Verify and Health Check Workflow
// =============================================================================

func TestIntegration_ProviderRegistry_VerifyAndHealthCheckWorkflow(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Provider that starts healthy, then becomes unhealthy
	flaky := NewMockLLMProvider("flaky-provider")

	err := registry.RegisterProvider("flaky-provider", flaky)
	require.NoError(t, err)

	// Verify while healthy
	ctx := context.Background()
	result := registry.VerifyProvider(ctx, "flaky-provider")
	require.NotNil(t, result)
	assert.True(t, result.Verified, "Provider should verify while healthy")
	assert.Equal(t, services.ProviderStatusHealthy, result.Status)

	// Make provider unhealthy
	flaky.SetCompleteError(errors.New("internal server error"))
	flaky.SetHealthCheckError(errors.New("health check failed"))

	// Re-verify — should fail
	result = registry.VerifyProvider(ctx, "flaky-provider")
	require.NotNil(t, result)
	assert.False(t, result.Verified, "Provider should fail verification when unhealthy")
	assert.Equal(t, services.ProviderStatusUnhealthy, result.Status)

	// Health check via registry
	healthResults := registry.HealthCheck()
	assert.NotNil(t, healthResults["flaky-provider"],
		"Health check should report error for unhealthy provider")
}

// =============================================================================
// Test: Provider Registry — Active Request Tracking under concurrency
// =============================================================================

func TestIntegration_ProviderRegistry_ActiveRequestTracking_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent tracking test in short mode")
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	provider := NewMockLLMProvider("tracked-concurrent")
	err := registry.RegisterProvider("tracked-concurrent", provider)
	require.NoError(t, err)

	const workers = 10
	const opsPerWorker = 50
	var wg sync.WaitGroup

	// All workers increment, do work, then decrement
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				ok := registry.IncrementActiveRequests("tracked-concurrent")
				assert.True(t, ok)

				// Simulate brief work
				_ = registry.GetActiveRequestCount("tracked-concurrent")

				ok = registry.DecrementActiveRequests("tracked-concurrent")
				assert.True(t, ok)
			}
		}()
	}

	wg.Wait()

	// All increments should be balanced by decrements
	finalCount := registry.GetActiveRequestCount("tracked-concurrent")
	assert.Equal(t, int64(0), finalCount,
		"Active request count must return to 0 after balanced inc/dec")
}

// =============================================================================
// Test: Provider Registry — Multiple Provider Score Ordering
// =============================================================================

func TestIntegration_ProviderRegistry_ScoreOrdering(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	// Register providers with different health characteristics
	healthy1 := NewMockLLMProvider("score-healthy-1")
	healthy2 := NewMockLLMProvider("score-healthy-2")
	unhealthy := NewMockLLMProvider("score-unhealthy")
	unhealthy.SetCompleteError(errors.New("always fails"))

	for _, entry := range []struct {
		name     string
		provider *MockLLMProvider
	}{
		{"score-healthy-1", healthy1},
		{"score-healthy-2", healthy2},
		{"score-unhealthy", unhealthy},
	} {
		err := registry.RegisterProvider(entry.name, entry.provider)
		require.NoError(t, err)
	}

	// Verify all providers
	ctx := context.Background()
	results := registry.VerifyAllProviders(ctx)
	assert.Len(t, results, 3)

	// Healthy providers should appear in GetHealthyProviders
	healthyList := registry.GetHealthyProviders()
	assert.Contains(t, healthyList, "score-healthy-1")
	assert.Contains(t, healthyList, "score-healthy-2")
	assert.NotContains(t, healthyList, "score-unhealthy")

	// GetBestProvidersForDebate should prefer healthy ones
	best := registry.GetBestProvidersForDebate(2, 3)
	assert.LessOrEqual(t, len(best), 3)
}

// =============================================================================
// Test: Provider Registry — Unregister and Re-register
// =============================================================================

func TestIntegration_ProviderRegistry_UnregisterAndReregister(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	provider := NewMockLLMProvider("unreg-test")
	err := registry.RegisterProvider("unreg-test", provider)
	require.NoError(t, err)

	// Confirm it exists
	_, err = registry.GetProvider("unreg-test")
	require.NoError(t, err)

	// Unregister
	err = registry.UnregisterProvider("unreg-test")
	require.NoError(t, err)

	// Confirm it is gone
	_, err = registry.GetProvider("unreg-test")
	assert.Error(t, err, "Provider should not be found after unregistration")

	// Re-register with a new instance
	newProvider := NewMockLLMProvider("unreg-test")
	newProvider.SetCompleteResponse(&models.LLMResponse{
		ID:      "new-instance",
		Content: "From re-registered provider",
	})

	err = registry.RegisterProvider("unreg-test", newProvider)
	require.NoError(t, err, "Re-registration after unregister should succeed")

	// Verify new instance is in place
	retrieved, err := registry.GetProvider("unreg-test")
	require.NoError(t, err)
	resp, err := retrieved.Complete(context.Background(), &models.LLMRequest{
		ID: "rereg-check", Prompt: "check",
	})
	require.NoError(t, err)
	assert.Equal(t, "new-instance", resp.ID)
}

// =============================================================================
// Test: Provider Registry — Configuration Persistence
// =============================================================================

func TestIntegration_ProviderRegistry_ConfigurationPersistence(t *testing.T) {
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	provider := NewMockLLMProvider("config-persist")
	err := registry.RegisterProvider("config-persist", provider)
	require.NoError(t, err)

	cfg := &services.ProviderConfig{
		Name:       "config-persist",
		Type:       "mock",
		Enabled:    true,
		APIKey:     "test-api-key-12345",
		BaseURL:    "https://api.mock-provider.dev/v1",
		Timeout:    45 * time.Second,
		MaxRetries: 5,
		Weight:     2.0,
		Tags:       []string{"integration", "test", "fast"},
		Capabilities: map[string]string{
			"model_type": "chat",
			"tier":       "enterprise",
		},
	}

	err = registry.ConfigureProvider("config-persist", cfg)
	require.NoError(t, err)

	retrieved, err := registry.GetProviderConfig("config-persist")
	require.NoError(t, err)
	assert.Equal(t, "config-persist", retrieved.Name)
	assert.Equal(t, "mock", retrieved.Type)
	assert.True(t, retrieved.Enabled)
	assert.Equal(t, "https://api.mock-provider.dev/v1", retrieved.BaseURL)
	assert.Equal(t, 5, retrieved.MaxRetries)
	assert.Equal(t, 2.0, retrieved.Weight)
	assert.Contains(t, retrieved.Tags, "integration")
}
