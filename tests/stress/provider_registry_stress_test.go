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

// TestProviderRegistry_ConcurrentReads exercises ListProviders and
// ListProvidersOrderedByScore under 1000 concurrent goroutines to
// verify no data races or deadlocks occur on the registry's RWMutex.
func TestProviderRegistry_ConcurrentReads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Honour resource limits: max 2 OS threads (CLAUDE.md §15)
	runtime.GOMAXPROCS(2)

	// Build a minimal registry with auto-discovery disabled so no
	// network calls are made during the stress run.
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
	require.NotNil(t, registry, "registry must be created")

	const numGoroutines = 1000
	var (
		wg        sync.WaitGroup
		readsDone int64
		panics    int64
	)

	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			// Gate all goroutines so they hammer the registry simultaneously
			<-start

			// Alternate between the two primary read-heavy methods
			if id%2 == 0 {
				_ = registry.ListProviders()
			} else {
				_ = registry.ListProvidersOrderedByScore()
			}
			atomic.AddInt64(&readsDone, 1)
		}(i)
	}

	// Release all goroutines at once
	close(start)

	// Wait with a generous timeout to catch deadlocks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent reads timed out after 30s")
	}

	assert.Zero(t, panics, "no goroutine should panic during concurrent reads")
	assert.Equal(t, int64(numGoroutines), readsDone,
		"all %d goroutines must complete", numGoroutines)

	t.Logf("Completed %d concurrent registry reads with 0 panics", readsDone)
}

// TestProviderRegistry_ConcurrentReadWrite exercises concurrent reads
// alongside a single writer calling GetAllProviderHealth and
// UpdateProviderScore, verifying RWMutex correctness.
func TestProviderRegistry_ConcurrentReadWrite(t *testing.T) {
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
		numReaders = 200
		numWrites  = 50
	)

	var (
		wg      sync.WaitGroup
		panics  int64
		readers int64
		writers int64
	)

	start := make(chan struct{})

	// Spawn readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			_ = registry.GetAllProviderHealth()
			_ = registry.ListProviders()
			atomic.AddInt64(&readers, 1)
		}(i)
	}

	// Spawn writers — UpdateProviderScore acquires write-path locks
	for i := 0; i < numWrites; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			providerName := fmt.Sprintf("stress-provider-%d", id%5)
			registry.UpdateProviderScore(providerName, "model-x", float64(id%10))
			atomic.AddInt64(&writers, 1)
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
		t.Fatal("DEADLOCK DETECTED: read/write stress timed out after 30s")
	}

	assert.Zero(t, panics, "no goroutine should panic during concurrent read/write")
	assert.Equal(t, int64(numReaders), readers, "all readers must finish")
	assert.Equal(t, int64(numWrites), writers, "all writers must finish")

	t.Logf("Completed %d readers and %d writers with 0 panics", readers, writers)
}

// TestProviderRegistry_GetBestProviders_Concurrent verifies
// GetBestProvidersForDebate is race-free under concurrent access.
func TestProviderRegistry_GetBestProviders_Concurrent(t *testing.T) {
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

	const numGoroutines = 300
	var wg sync.WaitGroup
	var panics int64

	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			_ = registry.GetBestProvidersForDebate(2, 5)
		}()
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
		t.Fatal("DEADLOCK DETECTED: GetBestProvidersForDebate stress timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during GetBestProvidersForDebate stress")
	t.Logf("Completed %d concurrent GetBestProvidersForDebate calls with 0 panics", numGoroutines)
}
