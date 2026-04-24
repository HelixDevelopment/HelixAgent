// Previously tests/chaos/core/chaos_test.go. CONST-030 forbids in-process
// mocks in non-unit tests; this file exercises a local mockProvider and
// mockCircuitBreaker to validate retry / cb semantics under synthetic
// failure injection — pure package-level unit logic. Real chaos
// engineering requires toxiproxy against a live service and is
// separate work (Pattern-2 in the audit). PR30 of the CONST-030 campaign.
package chaos_core_legacy_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockProvider simulates an LLM provider that fails randomly based on a
// configurable failure rate and latency.
type mockProvider struct {
	name     string
	failRate float64
	latency  time.Duration
}

// Complete simulates an LLM completion call with configurable failure.
func (p *mockProvider) Complete(ctx context.Context, prompt string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(p.latency):
	}

	if rand.Float64() < p.failRate {
		return "", errors.New("provider temporarily unavailable")
	}
	return "response from " + p.name + " to: " + prompt, nil
}

// mockCircuitBreaker provides a simplified circuit breaker for testing.
type mockCircuitBreaker struct {
	mu           sync.Mutex
	failures     int
	threshold    int
	open         bool
	lastFailure  time.Time
	resetTimeout time.Duration
}

func newMockCircuitBreaker(threshold int, resetTimeout time.Duration) *mockCircuitBreaker {
	return &mockCircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *mockCircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.open {
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.open = false
			cb.failures = 0
			return true
		}
		return false
	}
	return true
}

func (cb *mockCircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
}

func (cb *mockCircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.open = true
	}
}

// TestChaos_IntermittentFailures validates that the system handles random
// provider failures gracefully without panics or goroutine leaks.
func TestChaos_IntermittentFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	provider := &mockProvider{
		name:     "chaos-provider",
		failRate: 0.5,
		latency:  time.Millisecond,
	}

	const totalRequests = 1000
	const workers = 50
	requestsPerWorker := totalRequests / workers

	var successes, failures, panics int64
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < requestsPerWorker; j++ {
				_, err := provider.Complete(ctx, "test")
				if err != nil {
					atomic.AddInt64(&failures, 1)
				} else {
					atomic.AddInt64(&successes, 1)
				}
			}
		}()
	}

	close(start)
	wg.Wait()

	assert.Zero(t, panics, "no panics under intermittent failures")
	assert.Greater(t, successes, int64(0), "some requests should succeed")
	assert.Greater(t, failures, int64(0), "some requests should fail with 50% fail rate")

	successRate := float64(successes) / float64(successes+failures) * 100
	t.Logf("Chaos intermittent: successes=%d, failures=%d, panics=%d, success_rate=%.1f%%",
		successes, failures, panics, successRate)
}

// TestChaos_TimeoutRecovery validates that the system properly handles
// slow providers by respecting context deadlines.
func TestChaos_TimeoutRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	slowProvider := &mockProvider{
		name:     "slow-provider",
		failRate: 0,
		latency:  5 * time.Second,
	}

	const workers = 20
	var timeouts, successes, panics int64
	var wg sync.WaitGroup

	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := slowProvider.Complete(ctx, "test")
			if err != nil {
				atomic.AddInt64(&timeouts, 1)
			} else {
				atomic.AddInt64(&successes, 1)
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("DEADLOCK DETECTED: timeout recovery test hung")
	}

	assert.Zero(t, panics, "no panics during timeout recovery")
	assert.Equal(t, int64(workers), timeouts,
		"all requests to slow provider should time out")
	assert.Zero(t, successes,
		"no requests should succeed against a 5s provider with 100ms timeout")

	t.Logf("Chaos timeout: timeouts=%d, successes=%d, panics=%d",
		timeouts, successes, panics)
}

// TestChaos_CircuitBreakerTripping validates that circuit breakers properly
// trip after consecutive failures and recover after the reset timeout.
func TestChaos_CircuitBreakerTripping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	cb := newMockCircuitBreaker(5, 200*time.Millisecond)
	failingProvider := &mockProvider{
		name:     "failing-provider",
		failRate: 1.0,
		latency:  time.Millisecond,
	}

	ctx := context.Background()

	// Send requests until circuit breaker opens
	var rejected, failedBeforeOpen int64
	for i := 0; i < 20; i++ {
		if !cb.Allow() {
			rejected++
			continue
		}
		_, err := failingProvider.Complete(ctx, "test")
		if err != nil {
			cb.RecordFailure()
			failedBeforeOpen++
		}
	}

	assert.Greater(t, rejected, int64(0),
		"circuit breaker should reject requests after threshold")
	assert.GreaterOrEqual(t, failedBeforeOpen, int64(5),
		"at least threshold number of failures should occur before tripping")

	// Wait for reset timeout
	time.Sleep(300 * time.Millisecond)

	// Circuit breaker should be half-open now
	assert.True(t, cb.Allow(),
		"circuit breaker should allow requests after reset timeout")

	t.Logf("Circuit breaker: rejected=%d, failed_before_open=%d",
		rejected, failedBeforeOpen)
}

// TestChaos_ProviderFallbackChain validates that when the primary provider
// fails, requests are properly routed through the fallback chain.
func TestChaos_ProviderFallbackChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	providers := []*mockProvider{
		{name: "primary", failRate: 1.0, latency: time.Millisecond},
		{name: "secondary", failRate: 0.5, latency: time.Millisecond},
		{name: "tertiary", failRate: 0.0, latency: time.Millisecond},
	}

	ctx := context.Background()
	const requests = 200
	var primaryFail, secondaryFail, tertiarySuccess, allFailed, panics int64
	var wg sync.WaitGroup

	start := make(chan struct{})
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < requests/50; j++ {
				handled := false
				for _, p := range providers {
					_, err := p.Complete(ctx, "test")
					if err == nil {
						switch p.name {
						case "secondary":
							// secondary handled it
						case "tertiary":
							atomic.AddInt64(&tertiarySuccess, 1)
						}
						handled = true
						break
					}
					switch p.name {
					case "primary":
						atomic.AddInt64(&primaryFail, 1)
					case "secondary":
						atomic.AddInt64(&secondaryFail, 1)
					}
				}
				if !handled {
					atomic.AddInt64(&allFailed, 1)
				}
			}
		}()
	}

	close(start)
	wg.Wait()

	assert.Zero(t, panics, "no panics during fallback chain traversal")
	assert.Equal(t, int64(requests), primaryFail,
		"primary should fail all requests (100% fail rate)")
	assert.Zero(t, allFailed,
		"tertiary with 0% fail rate should catch all requests")

	t.Logf("Fallback chain: primary_fail=%d, secondary_fail=%d, tertiary_ok=%d, all_failed=%d",
		primaryFail, secondaryFail, tertiarySuccess, allFailed)
}

// TestChaos_ConcurrentProviderRegistration validates that concurrent
// reads and writes to a shared provider registry do not cause races.
func TestChaos_ConcurrentProviderRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	var mu sync.RWMutex
	registry := make(map[string]*mockProvider)

	var wg sync.WaitGroup
	var panics int64

	start := make(chan struct{})

	// Writers: register providers concurrently
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < 50; j++ {
				name := fmt.Sprintf("provider-%d-%d", id, j)
				mu.Lock()
				registry[name] = &mockProvider{
					name:     name,
					failRate: float64(j%10) / 10.0,
					latency:  time.Duration(j) * time.Microsecond,
				}
				mu.Unlock()
			}
		}(i)
	}

	// Readers: list providers concurrently
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < 100; j++ {
				mu.RLock()
				_ = len(registry)
				for _, p := range registry {
					_ = p.name
				}
				mu.RUnlock()
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent provider registration timed out")
	}

	assert.Zero(t, panics,
		"no panics during concurrent provider registration and listing")

	mu.RLock()
	count := len(registry)
	mu.RUnlock()

	assert.Equal(t, 20*50, count,
		"all providers should be registered in the shared registry")
	t.Logf("Concurrent registration: %d providers registered, panics=%d",
		count, panics)
}
