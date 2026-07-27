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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// --- Mock provider for ensemble stress tests ---

// ensembleStressMockProvider is a minimal LLMProvider that responds with
// configurable latency, enabling high-concurrency ensemble tests without
// network I/O.
type ensembleStressMockProvider struct {
	name       string
	latency    time.Duration
	confidence float64
	calls      int64
}

func (p *ensembleStressMockProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)

	select {
	case <-time.After(p.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return &models.LLMResponse{
		ID:           fmt.Sprintf("resp-%s-%d", p.name, atomic.LoadInt64(&p.calls)),
		ProviderID:   p.name,
		ProviderName: p.name,
		Content:      fmt.Sprintf("Response from %s", p.name),
		Confidence:   p.confidence,
		TokensUsed:   50,
		ResponseTime: p.latency.Milliseconds(),
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (p *ensembleStressMockProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	go func() {
		defer close(ch)
		resp, err := p.Complete(ctx, req)
		if err != nil {
			return
		}
		select {
		case ch <- resp:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

// --- Ensemble stress tests ---

// TestEnsemble_ConcurrentProviderCalls tests ensemble execution under 50, 100,
// and 200 concurrent callers. Each caller invokes RunEnsemble with multiple
// mock providers to verify the ensemble handles concurrent orchestration
// without panics or data corruption.
func TestEnsemble_ConcurrentProviderCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Register mock providers with varying latencies
	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 5; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("provider-%d", i),
			&ensembleStressMockProvider{
				name:       fmt.Sprintf("provider-%d", i),
				latency:    time.Duration(i+1) * time.Millisecond,
				confidence: 0.7 + float64(i)*0.05,
			},
		)
	}

	concurrencyLevels := []int{50, 100, 200}

	for _, concurrency := range concurrencyLevels {
		concurrency := concurrency
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			var wg sync.WaitGroup
			var successes int64
			var failures int64
			var panics int64

			start := make(chan struct{})

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							atomic.AddInt64(&panics, 1)
						}
					}()

					<-start

					ctx, cancel := context.WithTimeout(
						context.Background(), 10*time.Second,
					)
					defer cancel()

					req := &models.LLMRequest{
						Prompt: fmt.Sprintf("stress test %d", id),
						ModelParams: models.ModelParameters{
							Model:       "test-model",
							Temperature: 0.7,
							MaxTokens:   100,
						},
					}

					result, err := ensemble.RunEnsemble(ctx, req)
					if err != nil {
						atomic.AddInt64(&failures, 1)
						return
					}
					if result == nil || result.Selected == nil {
						atomic.AddInt64(&failures, 1)
						return
					}
					atomic.AddInt64(&successes, 1)
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
				t.Fatalf("DEADLOCK DETECTED: ensemble stress at concurrency %d "+
					"timed out after 30s", concurrency)
			}

			assert.Zero(t, panics,
				"no goroutine should panic at concurrency %d", concurrency)
			assert.Equal(t, int64(concurrency), successes+failures,
				"all goroutines must complete")
			assert.Greater(t, successes, int64(0),
				"at least some calls should succeed")

			t.Logf("Concurrency %d: %d successes, %d failures, 0 panics",
				concurrency, successes, failures)
		})
	}
}

// TestEnsemble_SemaphoreLimitsUnderLoad verifies that the ensemble respects
// its internal concurrency bounds. We register many slow providers and
// run many callers to ensure the system doesn't spawn unbounded goroutines.
func TestEnsemble_SemaphoreLimitsUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 3*time.Second)

	// Register 10 providers with moderate latency
	for i := 0; i < 10; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("slow-provider-%d", i),
			&ensembleStressMockProvider{
				name:       fmt.Sprintf("slow-provider-%d", i),
				latency:    10 * time.Millisecond,
				confidence: 0.6 + float64(i)*0.03,
			},
		)
	}

	goroutinesBefore := runtime.NumGoroutine()

	const callers = 100
	var wg sync.WaitGroup

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(
				context.Background(), 5*time.Second,
			)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("semaphore test %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   50,
				},
			}

			_, _ = ensemble.RunEnsemble(ctx, req)
		}(i)
	}

	wg.Wait()

	// Allow goroutines to settle
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	goroutinesAfter := runtime.NumGoroutine()

	// Goroutine count should not have grown drastically
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)

	assert.Less(t, leaked, 50,
		"goroutine count should not grow excessively after ensemble stress")
}

// TestEnsemble_ResponseTimeBounded verifies that response times remain
// bounded even under high concurrency by measuring P99 latency.
func TestEnsemble_ResponseTimeBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	for i := 0; i < 3; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("fast-provider-%d", i),
			&ensembleStressMockProvider{
				name:       fmt.Sprintf("fast-provider-%d", i),
				latency:    time.Duration(i+1) * time.Millisecond,
				confidence: 0.8,
			},
		)
	}

	const callers = 100
	latencies := make([]time.Duration, callers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(
				context.Background(), 10*time.Second,
			)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("latency test %d", idx),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.7,
					MaxTokens:   100,
				},
			}

			start := time.Now()
			_, _ = ensemble.RunEnsemble(ctx, req)
			elapsed := time.Since(start)

			mu.Lock()
			latencies[idx] = elapsed
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Find max latency (approximation of P99 with 100 samples)
	var maxLatency time.Duration
	var totalLatency time.Duration
	for _, lat := range latencies {
		totalLatency += lat
		if lat > maxLatency {
			maxLatency = lat
		}
	}
	avgLatency := totalLatency / time.Duration(callers)

	t.Logf("Response times: avg=%v, max=%v", avgLatency, maxLatency)

	// Max latency should stay under the ensemble timeout
	assert.Less(t, maxLatency, 5*time.Second,
		"max response time should be bounded by ensemble timeout")

	// Average should be well below timeout
	assert.Less(t, avgLatency, 2*time.Second,
		"average response time should be well below timeout")
}

// TestEnsemble_NoGoroutineLeaks verifies that after many ensemble
// invocations, goroutines do not leak. This creates and fully completes
// many ensemble calls, then checks goroutine count.
func TestEnsemble_NoGoroutineLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Force GC and let goroutines settle before baseline measurement
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	for iteration := 0; iteration < 20; iteration++ {
		ensemble := services.NewEnsembleService("confidence_weighted", 2*time.Second)
		for i := 0; i < 3; i++ {
			ensemble.RegisterProvider(
				fmt.Sprintf("leak-test-provider-%d", i),
				&ensembleStressMockProvider{
					name:       fmt.Sprintf("leak-test-provider-%d", i),
					latency:    time.Millisecond,
					confidence: 0.8,
				},
			)
		}

		ctx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		req := &models.LLMRequest{
			Prompt: fmt.Sprintf("leak test %d", iteration),
			ModelParams: models.ModelParameters{
				Model:       "test-model",
				Temperature: 0.7,
				MaxTokens:   100,
			},
		}
		_, _ = ensemble.RunEnsemble(ctx, req)
		cancel()
	}

	// Let goroutines clean up
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines

	t.Logf("Goroutine leak check: initial=%d, final=%d, leaked=%d",
		initialGoroutines, finalGoroutines, leaked)

	assert.Less(t, leaked, 20,
		"goroutine count should not grow significantly after 20 ensemble iterations")
}

// TestEnsemble_ConcurrentRegisterAndRun verifies that registering and
// removing providers concurrently with running ensemble calls does not
// cause panics or deadlocks.
func TestEnsemble_ConcurrentRegisterAndRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 3*time.Second)

	// Pre-seed with a stable provider so RunEnsemble can succeed
	ensemble.RegisterProvider("stable-0", &ensembleStressMockProvider{
		name:       "stable-0",
		latency:    time.Millisecond,
		confidence: 0.9,
	})

	var wg sync.WaitGroup
	var panics int64
	start := make(chan struct{})

	// Writers: register and remove providers
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

			name := fmt.Sprintf("dynamic-provider-%d", id)
			for j := 0; j < 50; j++ {
				ensemble.RegisterProvider(name, &ensembleStressMockProvider{
					name:       name,
					latency:    time.Millisecond,
					confidence: 0.7,
				})
				_ = ensemble.GetProviders()
				ensemble.RemoveProvider(name)
			}
		}(i)
	}

	// Readers: run ensemble
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < 20; j++ {
				ctx, cancel := context.WithTimeout(
					context.Background(), 3*time.Second,
				)
				req := &models.LLMRequest{
					Prompt: fmt.Sprintf("concurrent reg test %d-%d", id, j),
					ModelParams: models.ModelParameters{
						Model:       "test-model",
						Temperature: 0.5,
						MaxTokens:   50,
					},
				}
				_, _ = ensemble.RunEnsemble(ctx, req)
				cancel()
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
		t.Fatal("DEADLOCK DETECTED: concurrent register/run timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during concurrent register and run")
}

// TestEnsemble_VotingStrategiesUnderLoad exercises all three voting
// strategies (confidence_weighted, majority_vote, quality_weighted) under
// concurrent load to verify thread safety of the voting codepath.
func TestEnsemble_VotingStrategiesUnderLoad(t *testing.T) {
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
					fmt.Sprintf("vote-provider-%d", i),
					&ensembleStressMockProvider{
						name:       fmt.Sprintf("vote-provider-%d", i),
						latency:    time.Duration(i+1) * time.Millisecond,
						confidence: 0.6 + float64(i)*0.05,
					},
				)
			}

			const callers = 50
			var wg sync.WaitGroup
			var successes int64

			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					ctx, cancel := context.WithTimeout(
						context.Background(), 5*time.Second,
					)
					defer cancel()

					req := &models.LLMRequest{
						Prompt: fmt.Sprintf("voting strategy test %d", id),
						ModelParams: models.ModelParameters{
							Model:       "test-model",
							Temperature: 0.7,
							MaxTokens:   100,
						},
					}

					result, err := ensemble.RunEnsemble(ctx, req)
					if err == nil && result != nil && result.Selected != nil {
						atomic.AddInt64(&successes, 1)
					}
				}(i)
			}

			wg.Wait()

			require.Greater(t, successes, int64(0),
				"strategy %s should produce at least some successful results", strategy)
			t.Logf("Strategy %s: %d/%d succeeded", strategy, successes, callers)
		})
	}
}
