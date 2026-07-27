// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// --- Mock providers for provider stress tests ---

// providerStressMock is a configurable mock LLM provider for stress tests.
type providerStressMock struct {
	name       string
	latency    time.Duration
	confidence float64
	failAfter  int64 // if > 0, fail after this many calls
	calls      int64
	streaming  bool
}

func (p *providerStressMock) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	n := atomic.AddInt64(&p.calls, 1)

	if p.failAfter > 0 && n > p.failAfter {
		return nil, fmt.Errorf("provider %s: simulated failure after %d calls", p.name, p.failAfter)
	}

	select {
	case <-time.After(p.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return &models.LLMResponse{
		ID:           fmt.Sprintf("resp-%s-%d", p.name, n),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      fmt.Sprintf("Response from %s (call %d)", p.name, n),
		Confidence:   p.confidence,
		TokensUsed:   50,
		ResponseTime: p.latency.Milliseconds(),
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *providerStressMock) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 3)
	go func() {
		defer close(ch)
		resp, err := p.Complete(ctx, req)
		if err != nil {
			return
		}
		// Emit multiple streaming chunks
		for i := 0; i < 3; i++ {
			chunk := *resp
			chunk.Content = fmt.Sprintf("chunk-%d: %s", i, resp.Content)
			select {
			case ch <- &chunk:
			case <-ctx.Done():
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	return ch, nil
}

// --- Provider stress tests ---

// TestStress_ProviderRegistry_ConcurrentAccess exercises concurrent register,
// get, list, and update operations on the provider registry.
func TestStress_ProviderRegistry_ConcurrentAccess(t *testing.T) {
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
		numReaders = 100
		numWriters = 30
		numScorers = 20
	)

	var wg sync.WaitGroup
	var panics int64
	var readersDone, writersDone, scorersDone int64

	start := make(chan struct{})

	// Readers: list and get providers
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

			_ = registry.ListProviders()
			_ = registry.ListProvidersOrderedByScore()
			_ = registry.GetAllProviderHealth()
			_ = registry.GetBestProvidersForDebate(2, 3)
			atomic.AddInt64(&readersDone, 1)
		}(i)
	}

	// Writers: update provider scores
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < 10; j++ {
				providerName := fmt.Sprintf("stress-provider-%d", id%5)
				registry.UpdateProviderScore(providerName, "model-x", float64(j))
			}
			atomic.AddInt64(&writersDone, 1)
		}(i)
	}

	// Score readers: concurrent GetBestProviders
	for i := 0; i < numScorers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < 20; j++ {
				_ = registry.GetBestProvidersForDebate(1, 5)
			}
			atomic.AddInt64(&scorersDone, 1)
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
		t.Fatal("DEADLOCK DETECTED: provider registry concurrent access timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during concurrent registry access")
	assert.Equal(t, int64(numReaders), readersDone, "all readers must finish")
	assert.Equal(t, int64(numWriters), writersDone, "all writers must finish")
	assert.Equal(t, int64(numScorers), scorersDone, "all scorers must finish")
	t.Logf("Registry stress: readers=%d, writers=%d, scorers=%d, panics=%d",
		readersDone, writersDone, scorersDone, panics)
}

// TestStress_EnsembleVoting_HighLoad runs 50 concurrent voting operations
// with multiple providers and voting strategies to verify thread safety.
func TestStress_EnsembleVoting_HighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	strategies := []string{"confidence_weighted", "majority_vote", "quality_weighted"}

	for _, strategy := range strategies {
		strategy := strategy
		t.Run(strategy, func(t *testing.T) {
			ensemble := services.NewEnsembleService(strategy, 5*time.Second)
			for i := 0; i < 5; i++ {
				ensemble.RegisterProvider(
					fmt.Sprintf("vote-stress-%d", i),
					&providerStressMock{
						name:       fmt.Sprintf("vote-stress-%d", i),
						latency:    time.Duration(i+1) * time.Millisecond,
						confidence: 0.65 + float64(i)*0.06,
					},
				)
			}

			const callers = 50
			var wg sync.WaitGroup
			var successes, failures, panics int64

			start := make(chan struct{})

			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							atomic.AddInt64(&panics, 1)
						}
					}()
					<-start

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					req := &models.LLMRequest{
						Prompt: fmt.Sprintf("voting stress %s %d", strategy, id),
						ModelParams: models.ModelParameters{
							Model:       "test-model",
							Temperature: 0.7,
							MaxTokens:   100,
						},
					}

					result, err := ensemble.RunEnsemble(ctx, req)
					if err == nil && result != nil && result.Selected != nil {
						atomic.AddInt64(&successes, 1)
					} else {
						atomic.AddInt64(&failures, 1)
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
				t.Fatalf("DEADLOCK DETECTED: ensemble voting stress (%s) timed out", strategy)
			}

			assert.Zero(t, panics, "no panic during voting stress (%s)", strategy)
			assert.Equal(t, int64(callers), successes+failures,
				"all callers must complete for strategy %s", strategy)
			assert.Greater(t, successes, int64(0),
				"at least some calls should succeed for strategy %s", strategy)
			t.Logf("Voting %s: %d/%d succeeded, %d panics", strategy, successes, callers, panics)
		})
	}
}

// TestStress_LargePayload_Handling sends 1MB+ prompts through the ensemble
// to verify no OOM or excessive memory growth occurs.
func TestStress_LargePayload_Handling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 10*time.Second)
	for i := 0; i < 3; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("large-payload-%d", i),
			&providerStressMock{
				name:       fmt.Sprintf("large-payload-%d", i),
				latency:    2 * time.Millisecond,
				confidence: 0.8,
			},
		)
	}

	// Record baseline memory
	var baseline runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&baseline)

	const (
		numLargeRequests = 10
		payloadSizeBytes = 1024 * 1024 // 1 MB
	)

	var wg sync.WaitGroup
	var successes, failures int64

	for i := 0; i < numLargeRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create a 1MB+ prompt
			largePrompt := strings.Repeat("A", payloadSizeBytes)

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: largePrompt,
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   100,
				},
			}

			result, err := ensemble.RunEnsemble(ctx, req)
			if err == nil && result != nil {
				atomic.AddInt64(&successes, 1)
			} else {
				atomic.AddInt64(&failures, 1)
			}
		}(i)
	}

	wg.Wait()

	// Check memory after large payloads
	var afterMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&afterMem)

	heapGrowthMB := float64(afterMem.HeapInuse-baseline.HeapInuse) / 1024 / 1024

	t.Logf("Large payload: %d successes, %d failures, heap growth: %.2f MB",
		successes, failures, heapGrowthMB)

	assert.Equal(t, int64(numLargeRequests), successes+failures,
		"all large payload requests must complete")
	// With 10 x 1MB payloads, heap growth should be bounded (GC should reclaim)
	assert.Less(t, heapGrowthMB, 500.0,
		"heap growth after large payloads should be bounded")
}

// TestStress_StreamingUnderLoad exercises multiple concurrent streaming sessions
// through the ensemble to verify stream channels are properly managed.
func TestStress_StreamingUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 4; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("stream-stress-%d", i),
			&providerStressMock{
				name:       fmt.Sprintf("stream-stress-%d", i),
				latency:    time.Duration(i+1) * time.Millisecond,
				confidence: 0.75 + float64(i)*0.05,
				streaming:  true,
			},
		)
	}

	const numStreams = 30
	var wg sync.WaitGroup
	var streamOpened, streamFailed, panics int64
	var totalChunks int64

	// Goroutine baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	start := make(chan struct{})

	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("streaming stress %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.7,
					MaxTokens:   100,
				},
			}

			ch, err := ensemble.RunEnsembleStream(ctx, req)
			if err != nil {
				atomic.AddInt64(&streamFailed, 1)
				return
			}
			atomic.AddInt64(&streamOpened, 1)

			// Drain the stream channel
			for range ch {
				atomic.AddInt64(&totalChunks, 1)
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
		t.Fatal("DEADLOCK DETECTED: streaming under load timed out")
	}

	// Check goroutine leak
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panics, "no panics during streaming stress")
	assert.Equal(t, int64(numStreams), streamOpened+streamFailed,
		"all streaming requests must complete")
	assert.Less(t, leaked, 30,
		"goroutine count should not grow significantly after streaming stress")
	t.Logf("Streaming stress: %d opened, %d failed, %d total chunks, goroutine leak=%d, panics=%d",
		streamOpened, streamFailed, totalChunks, leaked, panics)
}

// TestStress_GracefulShutdown_UnderLoad starts load on the ensemble, then
// cancels the context to simulate shutdown, verifying all goroutines terminate
// cleanly without panics or resource leaks.
func TestStress_GracefulShutdown_UnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 5; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("shutdown-stress-%d", i),
			&providerStressMock{
				name:       fmt.Sprintf("shutdown-stress-%d", i),
				latency:    50 * time.Millisecond, // moderate latency so requests are in-flight
				confidence: 0.8,
			},
		)
	}

	// Goroutine baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	// Create a cancellable context to simulate shutdown
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var completed, cancelled, panics int64

	const numCallers = 40

	for i := 0; i < numCallers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			// Each caller makes multiple requests
			for j := 0; j < 10; j++ {
				reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
				req := &models.LLMRequest{
					Prompt: fmt.Sprintf("shutdown test %d-%d", id, j),
					ModelParams: models.ModelParameters{
						Model:       "test-model",
						Temperature: 0.5,
						MaxTokens:   50,
					},
				}

				_, err := ensemble.RunEnsemble(reqCtx, req)
				reqCancel()

				if err != nil {
					if ctx.Err() != nil {
						atomic.AddInt64(&cancelled, 1)
						return
					}
				} else {
					atomic.AddInt64(&completed, 1)
				}
			}
		}(i)
	}

	// Let some requests process, then cancel
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: graceful shutdown test timed out — goroutines did not terminate")
	}

	// Check goroutine cleanup
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panics, "no panics during shutdown under load")
	assert.Greater(t, completed+cancelled, int64(0),
		"some requests should have been processed or cancelled")
	assert.Less(t, leaked, 20,
		"goroutines should clean up after shutdown")
	t.Logf("Graceful shutdown: %d completed, %d cancelled, goroutine leak=%d, panics=%d",
		completed, cancelled, leaked, panics)
}
