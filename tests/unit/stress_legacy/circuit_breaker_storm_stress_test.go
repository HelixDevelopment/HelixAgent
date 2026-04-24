// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// stormMockProvider implements llm.LLMProvider for circuit breaker storm tests.
// The alwaysFail flag controls whether Complete returns an error or a response.
type stormMockProvider struct {
	id         string
	alwaysFail bool
	callCount  atomic.Int64
}

func (p *stormMockProvider) Complete(
	_ context.Context, _ *models.LLMRequest,
) (*models.LLMResponse, error) {
	p.callCount.Add(1)
	if p.alwaysFail {
		return nil, fmt.Errorf("provider %s: simulated failure", p.id)
	}
	return &models.LLMResponse{
		Content:    "ok",
		ProviderID: p.id,
	}, nil
}

func (p *stormMockProvider) CompleteStream(
	_ context.Context, _ *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 1)
	if p.alwaysFail {
		close(ch)
		return ch, fmt.Errorf("provider %s: stream failure", p.id)
	}
	go func() {
		defer close(ch)
		ch <- &models.LLMResponse{Content: "streamed", ProviderID: p.id}
	}()
	return ch, nil
}

func (p *stormMockProvider) HealthCheck() error {
	if p.alwaysFail {
		return fmt.Errorf("provider %s: unhealthy", p.id)
	}
	return nil
}

func (p *stormMockProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{"test-model"},
	}
}

func (p *stormMockProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// TestStress_CircuitBreaker_FailureStorm creates 10 circuit breakers (one per
// provider) and sends 100 concurrent failures to each. Verifies all transition
// to open state and that state transitions are safe under concurrency.
func TestStress_CircuitBreaker_FailureStorm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const numProviders = 10
	const failuresPerProvider = 100

	breakers := make([]*llm.CircuitBreaker, numProviders)
	providers := make([]*stormMockProvider, numProviders)

	config := llm.CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             5 * time.Second,
		HalfOpenMaxRequests: 2,
	}

	for i := 0; i < numProviders; i++ {
		id := fmt.Sprintf("storm-provider-%d", i)
		providers[i] = &stormMockProvider{id: id, alwaysFail: true}
		breakers[i] = llm.NewCircuitBreaker(id, providers[i], config)
	}

	var wg sync.WaitGroup
	var totalRejected, totalFailed atomic.Int64

	start := make(chan struct{})

	// Send concurrent failures to all circuit breakers simultaneously
	for i := 0; i < numProviders; i++ {
		cb := breakers[i]
		wg.Add(1)
		go func(breaker *llm.CircuitBreaker) {
			defer wg.Done()
			<-start

			for j := 0; j < failuresPerProvider; j++ {
				req := &models.LLMRequest{
					Prompt: "test",
					ModelParams: models.ModelParameters{
						Model: "test-model",
					},
				}
				_, err := breaker.Complete(context.Background(), req)
				if err != nil {
					if errors.Is(err, llm.ErrCircuitOpen) ||
						errors.Is(err, llm.ErrCircuitHalfOpenRejected) {
						totalRejected.Add(1)
					} else {
						totalFailed.Add(1)
					}
				}
			}
		}(cb)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: circuit breaker failure storm timed out")
	}

	// All circuit breakers should be open after the storm
	openCount := 0
	for i, cb := range breakers {
		state := cb.GetState()
		if state == llm.CircuitOpen {
			openCount++
		}
		stats := cb.GetStats()
		t.Logf("Provider %d: state=%s, total_requests=%d, failures=%d",
			i, state, stats.TotalRequests, stats.TotalFailures)
	}

	assert.Equal(t, numProviders, openCount,
		"all circuit breakers should be open after failure storm")

	t.Logf("Failure storm: %d breakers open, rejected=%d, failed=%d",
		openCount, totalRejected.Load(), totalFailed.Load())
}

// TestStress_CircuitBreaker_RecoveryUnderConcurrentLoad verifies that after
// a failure storm opens all circuit breakers, they can recover to closed state
// via the half-open probe mechanism when a healthy provider is used.
func TestStress_CircuitBreaker_RecoveryUnderConcurrentLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := llm.CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond, // Short timeout for test speed
		HalfOpenMaxRequests: 3,
	}

	// Create a provider that starts failing, then recovers
	provider := &stormMockProvider{id: "recovery-provider", alwaysFail: true}
	cb := llm.NewCircuitBreaker("recovery-provider", provider, config)

	// Trip the circuit
	for i := 0; i < 5; i++ {
		req := &models.LLMRequest{Prompt: "trip"}
		_, _ = cb.Complete(context.Background(), req)
	}

	require.Equal(t, llm.CircuitOpen, cb.GetState(),
		"circuit should be open after failures")

	// Wait for timeout to allow half-open transition
	time.Sleep(120 * time.Millisecond)

	// Now make the provider healthy
	provider.alwaysFail = false

	// Send concurrent requests — some will probe in half-open, then close
	const goroutineCount = 30
	var wg sync.WaitGroup
	var successes, rejections atomic.Int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			req := &models.LLMRequest{Prompt: "recovery probe"}
			_, err := cb.Complete(context.Background(), req)
			if err == nil {
				successes.Add(1)
			} else {
				rejections.Add(1)
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: circuit breaker recovery test timed out")
	}

	finalState := cb.GetState()
	t.Logf("Recovery: final_state=%s, successes=%d, rejections=%d",
		finalState, successes.Load(), rejections.Load())

	// The circuit should have progressed from open toward closed
	assert.NotEqual(t, llm.CircuitOpen, finalState,
		"circuit should have transitioned away from open during recovery")
	assert.Greater(t, successes.Load(), int64(0),
		"some requests should succeed after recovery")
}

// TestStress_CircuitBreaker_StateTransitionLatency measures the wall-clock
// time from the Nth failure (that triggers open) to the circuit reporting
// open state. State transition must complete within 1ms to avoid blocking
// request processing.
func TestStress_CircuitBreaker_StateTransitionLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := llm.CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             10 * time.Second,
		HalfOpenMaxRequests: 2,
	}

	const iterations = 50
	var maxTransitionNs int64

	for iter := 0; iter < iterations; iter++ {
		provider := &stormMockProvider{id: fmt.Sprintf("latency-iter-%d", iter), alwaysFail: true}
		cb := llm.NewCircuitBreaker(provider.id, provider, config)

		// Send failures up to threshold-1 (stays closed)
		for i := 0; i < config.FailureThreshold-1; i++ {
			req := &models.LLMRequest{Prompt: "pre-trip"}
			_, _ = cb.Complete(context.Background(), req)
		}

		// Time the transition-triggering failure
		req := &models.LLMRequest{Prompt: "trip"}
		tripStart := time.Now()
		_, _ = cb.Complete(context.Background(), req)
		state := cb.GetState()
		elapsed := time.Since(tripStart).Nanoseconds()

		require.Equal(t, llm.CircuitOpen, state,
			"circuit should be open after reaching failure threshold (iter %d)", iter)

		if elapsed > maxTransitionNs {
			maxTransitionNs = elapsed
		}
	}

	maxTransitionMs := float64(maxTransitionNs) / 1e6
	t.Logf("State transition latency: max=%.3fms across %d iterations",
		maxTransitionMs, iterations)

	assert.Less(t, maxTransitionMs, 1.0,
		"circuit breaker state transition should complete within 1ms")
}

// TestStress_CircuitBreaker_ConcurrentStateReads verifies that concurrent
// GetState() calls during active state transitions (simultaneous failures
// from multiple goroutines) do not cause panics or return invalid states.
func TestStress_CircuitBreaker_ConcurrentStateReads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := llm.CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 3,
	}

	provider := &stormMockProvider{id: "state-read-provider", alwaysFail: true}
	cb := llm.NewCircuitBreaker("state-read-provider", provider, config)

	validStates := map[llm.CircuitState]bool{
		llm.CircuitClosed:   true,
		llm.CircuitOpen:     true,
		llm.CircuitHalfOpen: true,
	}

	const goroutineCount = 50
	var wg sync.WaitGroup
	var invalidStateCount, panicCount atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := make(chan struct{})

	// Writer goroutines: drive state transitions
	for i := 0; i < goroutineCount/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			req := &models.LLMRequest{Prompt: "concurrent-write"}
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, _ = cb.Complete(context.Background(), req)
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Reader goroutines: concurrent state reads
	for i := 0; i < goroutineCount/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			for {
				select {
				case <-ctx.Done():
					return
				default:
					state := cb.GetState()
					if !validStates[state] {
						invalidStateCount.Add(1)
					}
				}
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent state reads timed out")
	}

	assert.Zero(t, panicCount.Load(), "no panics during concurrent state reads")
	assert.Zero(t, invalidStateCount.Load(),
		"GetState should never return an invalid state")

	t.Logf("Concurrent state reads: panics=%d, invalid_states=%d, final_state=%s",
		panicCount.Load(), invalidStateCount.Load(), cb.GetState())
}
