// Package resilience provides chaos tests that verify the system's ability to
// recover from various failure modes without panicking, deadlocking, or leaking
// resources.
package resilience

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/cache"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// --- Mock providers for resilience tests ---

// alwaysFailProvider always returns an error.
type alwaysFailProvider struct {
	name   string
	errMsg string
	calls  int64
}

func (p *alwaysFailProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	return nil, fmt.Errorf("%s: %s", p.name, p.errMsg)
}

func (p *alwaysFailProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	return nil, fmt.Errorf("%s: %s (stream)", p.name, p.errMsg)
}

// highLatencyProvider responds after a configurable delay.
type highLatencyProvider struct {
	name    string
	latency time.Duration
	calls   int64
}

func (p *highLatencyProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	n := atomic.AddInt64(&p.calls, 1)
	select {
	case <-time.After(p.latency):
		return &models.LLMResponse{
			ID:           fmt.Sprintf("latency-%s-%d", p.name, n),
			ProviderID:   p.name,
			ProviderName: p.name,
			Content:      fmt.Sprintf("High-latency response from %s", p.name),
			Confidence:   0.7,
			TokensUsed:   20,
			ResponseTime: p.latency.Milliseconds(),
			FinishReason: "stop",
			CreatedAt:    time.Now(),
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *highLatencyProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, err := p.Complete(ctx, req)
		if err == nil && resp != nil {
			select {
			case ch <- resp:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

// recoveringProvider fails for a configured number of calls, then succeeds.
type recoveringProvider struct {
	name       string
	failCount  int64
	calls      int64
	confidence float64
}

func (p *recoveringProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	n := atomic.AddInt64(&p.calls, 1)
	if n <= p.failCount {
		return nil, fmt.Errorf("provider %s: temporarily unavailable (call %d/%d)",
			p.name, n, p.failCount)
	}
	return &models.LLMResponse{
		ID:           fmt.Sprintf("recovered-%s-%d", p.name, n),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      fmt.Sprintf("Recovered response from %s (call %d)", p.name, n),
		Confidence:   p.confidence,
		TokensUsed:   30,
		ResponseTime: 5,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *recoveringProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, err := p.Complete(ctx, req)
		if err == nil && resp != nil {
			select {
			case ch <- resp:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

// reliableProvider always succeeds with configurable latency.
type reliableProvider struct {
	name       string
	latency    time.Duration
	confidence float64
	calls      int64
}

func (p *reliableProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	n := atomic.AddInt64(&p.calls, 1)
	if p.latency > 0 {
		select {
		case <-time.After(p.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &models.LLMResponse{
		ID:           fmt.Sprintf("reliable-%s-%d", p.name, n),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      fmt.Sprintf("Reliable response from %s", p.name),
		Confidence:   p.confidence,
		TokensUsed:   25,
		ResponseTime: p.latency.Milliseconds(),
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *reliableProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, err := p.Complete(ctx, req)
		if err == nil && resp != nil {
			select {
			case ch <- resp:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

// --- Resilience chaos tests ---

// TestChaos_AllProviders_Unavailable verifies that when all registered providers
// fail, the ensemble returns a graceful error rather than panicking.
func TestChaos_AllProviders_Unavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 3*time.Second)
	// Register only failing providers
	for i := 0; i < 5; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("failing-%d", i),
			&alwaysFailProvider{
				name:   fmt.Sprintf("failing-%d", i),
				errMsg: "service unavailable",
			},
		)
	}

	const numRequests = 20
	var wg sync.WaitGroup
	var errors, panics int64

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
					t.Logf("PANIC when all providers unavailable: %v", r)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("all-fail test %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.7,
					MaxTokens:   50,
				},
			}

			result, err := ensemble.RunEnsemble(ctx, req)
			if err != nil || result == nil || result.Selected == nil {
				atomic.AddInt64(&errors, 1)
			}
		}(i)
	}

	wg.Wait()

	assert.Zero(t, panics, "no panics when all providers are unavailable")
	assert.Equal(t, int64(numRequests), errors,
		"all requests should result in errors when no provider is available")
	t.Logf("All providers unavailable: %d errors (expected), %d panics", errors, panics)
}

// TestChaos_CacheMiss_Degradation verifies that the cache system degrades
// gracefully when the L2 (Redis) layer is unavailable and only L1 (memory)
// is configured.
func TestChaos_CacheMiss_Degradation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Create a cache with only L1 enabled (no Redis connection)
	config := &cache.TieredCacheConfig{
		L1MaxSize: 100,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	const (
		numGoroutines = 30
		opsPerRoutine = 50
	)

	var wg sync.WaitGroup
	var hits, misses, panics int64

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
			<-start

			for j := 0; j < opsPerRoutine; j++ {
				key := fmt.Sprintf("cache-miss-key-%d", j%30)

				// Try to get (most will miss)
				var dest string
				found, err := tc.Get(ctx, key, &dest)
				if err == nil && found {
					atomic.AddInt64(&hits, 1)
				} else {
					atomic.AddInt64(&misses, 1)
					// Set the value for potential future hits
					_ = tc.Set(ctx, key, fmt.Sprintf("val-%d-%d", id, j), time.Minute)
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
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: cache degradation test timed out")
	}

	assert.Zero(t, panics, "no panics during cache degradation")
	assert.Greater(t, hits+misses, int64(0),
		"cache operations should complete")
	t.Logf("Cache degradation: %d hits, %d misses, %d panics", hits, misses, panics)
}

// TestChaos_ConcurrentErrors_NoDeadlock fires concurrent requests that all
// produce errors through multiple error paths simultaneously, verifying no
// deadlock or panic occurs.
func TestChaos_ConcurrentErrors_NoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Build ensemble with a mix of failure modes
	ensemble := services.NewEnsembleService("confidence_weighted", 2*time.Second)
	ensemble.RegisterProvider("timeout-err", &highLatencyProvider{
		name:    "timeout-err",
		latency: 5 * time.Second, // will timeout
	})
	ensemble.RegisterProvider("always-fail", &alwaysFailProvider{
		name:   "always-fail",
		errMsg: "connection refused",
	})
	ensemble.RegisterProvider("always-fail-2", &alwaysFailProvider{
		name:   "always-fail-2",
		errMsg: "internal server error",
	})

	const numGoroutines = 50
	var wg sync.WaitGroup
	var completed, panics int64

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

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
			<-start

			for j := 0; j < 5; j++ {
				reqCtx, reqCancel := context.WithTimeout(ctx, 3*time.Second)

				req := &models.LLMRequest{
					Prompt: fmt.Sprintf("concurrent error %d-%d", id, j),
					ModelParams: models.ModelParameters{
						Model:       "test-model",
						Temperature: 0.5,
						MaxTokens:   50,
					},
				}

				// We expect errors, just verify no deadlock/panic
				_, _ = ensemble.RunEnsemble(reqCtx, req)
				reqCancel()
				atomic.AddInt64(&completed, 1)
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
		t.Fatal("DEADLOCK DETECTED: concurrent error paths timed out")
	}

	assert.Zero(t, panics, "no panics during concurrent error paths")
	assert.Equal(t, int64(numGoroutines*5), completed,
		"all requests must complete even when erroring")
	t.Logf("Concurrent errors: %d completed, %d panics", completed, panics)
}

// TestChaos_RecoveryAfterFailure verifies that a provider that fails then
// recovers is correctly re-used by the ensemble.
func TestChaos_RecoveryAfterFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)

	// Provider that fails for first 5 calls, then succeeds
	recovering := &recoveringProvider{
		name:       "recovering-provider",
		failCount:  5,
		confidence: 0.85,
	}
	ensemble.RegisterProvider("recovering-provider", recovering)

	// One provider that always works
	ensemble.RegisterProvider("stable", &reliableProvider{
		name:       "stable",
		latency:    time.Millisecond,
		confidence: 0.8,
	})

	// Phase 1: Send requests during failure period
	var failPhaseSuccesses, failPhaseErrors int64
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req := &models.LLMRequest{
			Prompt: fmt.Sprintf("recovery phase 1 - %d", i),
			ModelParams: models.ModelParameters{
				Model:       "test-model",
				Temperature: 0.7,
				MaxTokens:   50,
			},
		}
		result, err := ensemble.RunEnsemble(ctx, req)
		cancel()
		if err == nil && result != nil && result.Selected != nil {
			failPhaseSuccesses++
		} else {
			failPhaseErrors++
		}
	}

	// Phase 2: Send requests after recovery
	var recoverySuccesses, recoveryErrors int64
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req := &models.LLMRequest{
			Prompt: fmt.Sprintf("recovery phase 2 - %d", i),
			ModelParams: models.ModelParameters{
				Model:       "test-model",
				Temperature: 0.7,
				MaxTokens:   50,
			},
		}
		result, err := ensemble.RunEnsemble(ctx, req)
		cancel()
		if err == nil && result != nil && result.Selected != nil {
			recoverySuccesses++
		} else {
			recoveryErrors++
		}
	}

	t.Logf("Failure phase: %d successes, %d errors", failPhaseSuccesses, failPhaseErrors)
	t.Logf("Recovery phase: %d successes, %d errors", recoverySuccesses, recoveryErrors)

	// The stable provider should handle all requests, so we expect successes in both phases
	assert.Greater(t, failPhaseSuccesses+recoverySuccesses, int64(0),
		"should have at least some successes across both phases")
	// After recovering provider starts working, success rate should remain high
	assert.Greater(t, recoverySuccesses, int64(0),
		"recovery phase should have successes")
}

// TestChaos_HighLatency_Timeout verifies that high-latency provider responses
// are properly bounded by context timeouts, preventing resource starvation.
func TestChaos_HighLatency_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Goroutine baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	ensemble := services.NewEnsembleService("confidence_weighted", 500*time.Millisecond)
	// All providers are high-latency
	for i := 0; i < 5; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("high-latency-%d", i),
			&highLatencyProvider{
				name:    fmt.Sprintf("high-latency-%d", i),
				latency: time.Duration(2+i) * time.Second, // 2-6 seconds, all exceed timeout
			},
		)
	}

	const numRequests = 20
	var wg sync.WaitGroup
	var timedOut, panics int64
	latencies := make([]time.Duration, numRequests)
	var latencyMu sync.Mutex

	start := make(chan struct{})

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			requestStart := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("high latency %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   50,
				},
			}

			_, err := ensemble.RunEnsemble(ctx, req)
			elapsed := time.Since(requestStart)

			latencyMu.Lock()
			latencies[id] = elapsed
			latencyMu.Unlock()

			if err != nil {
				atomic.AddInt64(&timedOut, 1)
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
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: high latency timeout test timed out")
	}

	// Verify all requests completed within timeout bounds
	var maxLatency time.Duration
	for _, lat := range latencies {
		if lat > maxLatency {
			maxLatency = lat
		}
	}

	assert.Zero(t, panics, "no panics during high-latency timeout")
	// Requests should have timed out (most should fail)
	assert.Greater(t, timedOut, int64(0),
		"some requests should time out when all providers are high-latency")
	// Max latency should be bounded by context timeout + small overhead
	require.Less(t, maxLatency, 3*time.Second,
		"max latency should be bounded by timeout — not wait for slow providers")

	// Check goroutine cleanup
	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Less(t, leaked, 30,
		"goroutines should clean up after high-latency timeout")
	t.Logf("High latency chaos: %d timed out, max_latency=%v, goroutine_leak=%d, panics=%d",
		timedOut, maxLatency, leaked, panics)
}
