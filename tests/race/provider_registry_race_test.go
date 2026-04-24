// Package race provides comprehensive race condition detection tests for HelixAgent
// These tests validate that all concurrent operations are thread-safe
package race

import (
	"sync"
	"testing"

	"dev.helix.agent/internal/services"
)

// TestProviderRegistry_ConcurrentGetProvider tests concurrent GetProvider calls.
// The registry uses sync.RWMutex + sync.Once per provider — this test exercises
// both the fast read path and the slow initialization path simultaneously.
func TestProviderRegistry_ConcurrentGetProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	cfg := &services.RegistryConfig{
		DisableAutoDiscovery: true,
	}
	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Provider "openai" is not configured so this returns an error, but
			// the critical thing is that concurrent calls do not race on internal maps.
			_, _ = registry.GetProvider("openai")
			_, _ = registry.GetProvider("gemini")
			_, _ = registry.GetProvider("claude")
		}()
	}

	wg.Wait()
}

// TestProviderRegistry_ConcurrentListAndGet tests concurrent list + get operations.
func TestProviderRegistry_ConcurrentListAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")  // SKIP-OK: #short-mode
	}

	cfg := &services.RegistryConfig{
		DisableAutoDiscovery: true,
	}
	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	var wg sync.WaitGroup
	const goroutines = 15

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Mix read-heavy and write-heavy paths.
			if id%2 == 0 {
				_ = registry.ListProviders()
			} else {
				_, _ = registry.GetProvider("deepseek")
				_, _ = registry.GetProviderConfig("deepseek")
			}
		}(i)
	}

	wg.Wait()
}
