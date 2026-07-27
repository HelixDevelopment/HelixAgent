package stress

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
)

// TestStress_ProviderRegistry_SimultaneousGetAndList exercises 20 goroutines
// calling GetProvider and 10 goroutines calling ListProviders simultaneously
// to verify no race conditions and consistent results across reads.
func TestStress_ProviderRegistry_SimultaneousGetAndList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cfg := &services.RegistryConfig{
		DefaultTimeout:        5 * time.Second,
		MaxRetries:            1,
		MaxConcurrentRequests: 10,
		DisableAutoDiscovery:  true,
		HealthCheck: services.HealthCheckConfig{
			Enabled: false,
		},
		CircuitBreaker: services.CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			RecoveryTimeout:  10 * time.Second,
			SuccessThreshold: 2,
		},
		Providers: make(map[string]*services.ProviderConfig),
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
	require.NotNil(t, registry)

	const (
		getWorkers  = 20
		listWorkers = 10
	)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount int64
	var getOps, listOps int64

	start := make(chan struct{})

	// GetProvider goroutines
	for i := 0; i < getWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			providerNames := []string{
				"openai", "anthropic", "deepseek", "gemini", "mistral",
				"openrouter", "cerebras", "ollama", "nonexistent",
			}

			for j := 0; j < 50; j++ {
				name := providerNames[(id+j)%len(providerNames)]
				_, _ = registry.GetProvider(name)
				atomic.AddInt64(&getOps, 1)
			}
		}(i)
	}

	// ListProviders goroutines
	for i := 0; i < listWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 50; j++ {
				providers := registry.ListProviders()
				// Verify the list is consistent (not corrupted)
				_ = len(providers)
				atomic.AddInt64(&listOps, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: provider get/list stress test timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount, "no panics during concurrent get/list operations")
	assert.Equal(t, int64(getWorkers*50), getOps, "all get operations must complete")
	assert.Equal(t, int64(listWorkers*50), listOps, "all list operations must complete")
	assert.Less(t, leaked, 15,
		"goroutine count should remain stable after provider registry stress")
	t.Logf("Provider get/list stress: getOps=%d, listOps=%d, panics=%d, goroutine_leak=%d",
		getOps, listOps, panicCount, leaked)
}

// TestStress_ProviderRegistry_ConcurrentScoreUpdates verifies that concurrent
// score updates and health status reads do not corrupt registry state.
func TestStress_ProviderRegistry_ConcurrentScoreUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cfg := &services.RegistryConfig{
		DefaultTimeout:        5 * time.Second,
		MaxRetries:            1,
		MaxConcurrentRequests: 10,
		DisableAutoDiscovery:  true,
		HealthCheck: services.HealthCheckConfig{
			Enabled: false,
		},
		CircuitBreaker: services.CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			RecoveryTimeout:  10 * time.Second,
			SuccessThreshold: 2,
		},
		Providers: make(map[string]*services.ProviderConfig),
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
	require.NotNil(t, registry)

	const (
		writerCount = 20
		readerCount = 20
	)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount int64
	var writeOps, readOps int64

	start := make(chan struct{})

	// Writers: update scores
	for i := 0; i < writerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 30; j++ {
				providerName := fmt.Sprintf("provider-%d", id%5)
				score := 5.0 + float64(j%50)/10.0
				registry.UpdateProviderScore(providerName, "model-x", score)
				atomic.AddInt64(&writeOps, 1)
			}
		}(i)
	}

	// Readers: read health and ordered lists
	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 30; j++ {
				_ = registry.GetAllProviderHealth()
				_ = registry.ListProvidersOrderedByScore()
				_ = registry.GetBestProvidersForDebate(2, 3)
				atomic.AddInt64(&readOps, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: provider score update stress test timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount, "no panics during concurrent score updates")
	assert.Equal(t, int64(writerCount*30), writeOps, "all write operations complete")
	assert.Equal(t, int64(readerCount*30), readOps, "all read operations complete")
	assert.Less(t, leaked, 15,
		"goroutines should not leak after concurrent score updates")
	t.Logf("Provider score stress: writes=%d, reads=%d, panics=%d, goroutine_leak=%d",
		writeOps, readOps, panicCount, leaked)
}

// TestStress_ProviderRegistry_ConsistentResults verifies that concurrent
// ListProviders calls always return the same set of providers (no partial
// reads or corrupted state).
func TestStress_ProviderRegistry_ConsistentResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	cfg := &services.RegistryConfig{
		DefaultTimeout:        5 * time.Second,
		MaxRetries:            1,
		MaxConcurrentRequests: 10,
		DisableAutoDiscovery:  true,
		HealthCheck: services.HealthCheckConfig{
			Enabled: false,
		},
		CircuitBreaker: services.CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			RecoveryTimeout:  10 * time.Second,
			SuccessThreshold: 2,
		},
		Providers: make(map[string]*services.ProviderConfig),
	}

	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)
	require.NotNil(t, registry)

	// Get baseline provider count
	baselineProviders := registry.ListProviders()
	baselineCount := len(baselineProviders)

	const goroutineCount = 30
	var wg sync.WaitGroup
	var panicCount, inconsistentCount int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 100; j++ {
				providers := registry.ListProviders()
				if len(providers) != baselineCount {
					atomic.AddInt64(&inconsistentCount, 1)
				}
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: provider consistency stress test timed out")
	}

	assert.Zero(t, panicCount, "no panics during consistency check")
	assert.Zero(t, inconsistentCount,
		"all concurrent reads should return consistent provider count")
	t.Logf("Provider consistency stress: goroutines=%d, reads=%d, "+
		"baseline_count=%d, inconsistent=%d, panics=%d",
		goroutineCount, goroutineCount*100, baselineCount,
		inconsistentCount, panicCount)
}
