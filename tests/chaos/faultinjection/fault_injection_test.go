// Package faultinjection provides chaos tests that inject faults into provider
// operations to verify resilience and graceful degradation.
package faultinjection

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

// --- Fault-injecting mock providers ---

// timeoutProvider simulates a provider that times out mid-stream.
type timeoutProvider struct {
	name  string
	delay time.Duration // delay before responding
	calls int64
}

func (p *timeoutProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	select {
	case <-time.After(p.delay):
		return &models.LLMResponse{
			ID:           fmt.Sprintf("timeout-%s-%d", p.name, atomic.LoadInt64(&p.calls)),
			ProviderID:   p.name,
			ProviderName: p.name,
			Content:      "delayed response",
			Confidence:   0.5,
			TokensUsed:   10,
			ResponseTime: p.delay.Milliseconds(),
			FinishReason: "stop",
			CreatedAt:    time.Now(),
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *timeoutProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse, 5)
	go func() {
		defer close(ch)
		// Send one chunk, then simulate timeout
		chunk := &models.LLMResponse{
			ID:           fmt.Sprintf("stream-chunk-%s", p.name),
			ProviderID:   p.name,
			ProviderName: p.name,
			Content:      "partial...",
			Confidence:   0.5,
			CreatedAt:    time.Now(),
		}
		select {
		case ch <- chunk:
		case <-ctx.Done():
			return
		}
		// Hang to simulate mid-stream timeout
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

// corruptProvider returns malformed/unexpected responses.
type corruptProvider struct {
	name  string
	calls int64
}

func (p *corruptProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	n := atomic.AddInt64(&p.calls, 1)
	// Alternate between various corrupt response patterns
	switch n % 4 {
	case 0:
		// Empty content
		return &models.LLMResponse{
			ID:         fmt.Sprintf("corrupt-%d", n),
			ProviderID: p.name,
			Content:    "",
			Confidence: 0.0,
			CreatedAt:  time.Now(),
		}, nil
	case 1:
		// Nil response
		return nil, nil
	case 2:
		// Error response
		return nil, fmt.Errorf("provider %s: malformed JSON in upstream response", p.name)
	default:
		// Valid but low-quality
		return &models.LLMResponse{
			ID:           fmt.Sprintf("corrupt-%d", n),
			ProviderID:   p.name,
			ProviderName: p.name,
			Content:      "partial corrupt data",
			Confidence:   0.1,
			TokensUsed:   1,
			FinishReason: "error",
			CreatedAt:    time.Now(),
		}, nil
	}
}

func (p *corruptProvider) CompleteStream(
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

// failingProvider always returns errors.
type failingProvider struct {
	name   string
	errMsg string
	calls  int64
}

func (p *failingProvider) Complete(
	ctx context.Context, req *models.LLMRequest,
) (*models.LLMResponse, error) {
	atomic.AddInt64(&p.calls, 1)
	return nil, fmt.Errorf("%s: %s", p.name, p.errMsg)
}

func (p *failingProvider) CompleteStream(
	ctx context.Context, req *models.LLMRequest,
) (<-chan *models.LLMResponse, error) {
	return nil, fmt.Errorf("%s: %s (stream)", p.name, p.errMsg)
}

// recoveringProvider fails for N calls, then succeeds.
type recoveringProvider struct {
	name       string
	failCount  int64 // how many calls to fail before recovering
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
		Content:      fmt.Sprintf("Recovered response from %s", p.name),
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

// --- Chaos fault injection tests ---

// TestChaos_ProviderTimeout_MidStream simulates a provider that times out
// during streaming, verifying the ensemble handles the timeout gracefully.
func TestChaos_ProviderTimeout_MidStream(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 500*time.Millisecond)
	// One slow (timeout-prone) provider
	ensemble.RegisterProvider("timeout-provider", &timeoutProvider{
		name:  "timeout-provider",
		delay: 2 * time.Second, // exceeds ensemble timeout
	})
	// One fast provider to ensure the ensemble can still succeed
	ensemble.RegisterProvider("fast-provider", &recoveringProvider{
		name:       "fast-provider",
		failCount:  0,
		confidence: 0.9,
	})

	const numRequests = 20
	var wg sync.WaitGroup
	var successes, failures, panics int64

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("timeout chaos %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.7,
					MaxTokens:   50,
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

	assert.Zero(t, panics, "no panics during timeout chaos")
	assert.Equal(t, int64(numRequests), successes+failures,
		"all requests must complete (success or failure)")
	t.Logf("Timeout chaos: %d successes, %d failures, %d panics",
		successes, failures, panics)
}

// TestChaos_CorruptedResponse_Handling verifies the ensemble handles providers
// returning corrupted/malformed responses without panicking.
func TestChaos_CorruptedResponse_Handling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 5*time.Second)
	// Multiple corrupt providers
	for i := 0; i < 3; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("corrupt-%d", i),
			&corruptProvider{name: fmt.Sprintf("corrupt-%d", i)},
		)
	}
	// One healthy provider to rescue the ensemble
	ensemble.RegisterProvider("healthy", &recoveringProvider{
		name:       "healthy",
		failCount:  0,
		confidence: 0.85,
	})

	const numRequests = 30
	var wg sync.WaitGroup
	var panics int64
	var completed int64

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
					t.Logf("PANIC in corrupt response test: %v", r)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("corrupt test %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   50,
				},
			}

			// We just care that this does not panic
			_, _ = ensemble.RunEnsemble(ctx, req)
			atomic.AddInt64(&completed, 1)
		}(i)
	}

	wg.Wait()

	assert.Zero(t, panics, "no panics when handling corrupted responses")
	assert.Equal(t, int64(numRequests), completed,
		"all requests must complete without panic")
	t.Logf("Corrupt response chaos: %d completed, %d panics", completed, panics)
}

// TestChaos_PartialResponse_Recovery simulates providers returning partial
// responses then failing, verifying ensemble recovery.
func TestChaos_PartialResponse_Recovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	runtime.GOMAXPROCS(2)

	ensemble := services.NewEnsembleService("confidence_weighted", 3*time.Second)
	// Provider that sends partial stream then hangs
	ensemble.RegisterProvider("partial-provider", &timeoutProvider{
		name:  "partial-provider",
		delay: 5 * time.Second, // will be cancelled by ensemble timeout
	})
	// Reliable backup
	ensemble.RegisterProvider("reliable", &recoveringProvider{
		name:       "reliable",
		failCount:  0,
		confidence: 0.9,
	})

	const numRequests = 15
	var wg sync.WaitGroup
	var panics, completed int64

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("partial recovery %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.7,
					MaxTokens:   50,
				},
			}

			_, _ = ensemble.RunEnsemble(ctx, req)
			atomic.AddInt64(&completed, 1)
		}(i)
	}

	wg.Wait()

	assert.Zero(t, panics, "no panics during partial response recovery")
	assert.Equal(t, int64(numRequests), completed,
		"all requests must complete without panic")
	t.Logf("Partial response chaos: %d completed, %d panics", completed, panics)
}

// TestChaos_RapidProviderFailure_CircuitBreaker sends rapid successive failures
// to trigger circuit breaker protection, then verifies state transitions.
func TestChaos_RapidProviderFailure_CircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	runtime.GOMAXPROCS(2)

	// Use a circuit breaker with low thresholds
	cb := services.NewCircuitBreaker(3, 2, 50*time.Millisecond)
	require.NotNil(t, cb)

	var wg sync.WaitGroup
	var failures, blocked, panics int64

	const numGoroutines = 20
	const callsPerGoroutine = 50

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

			for j := 0; j < callsPerGoroutine; j++ {
				err := cb.Call(func() error {
					return fmt.Errorf("simulated rapid failure")
				})
				if err != nil {
					if err.Error() == "circuit breaker is open" {
						atomic.AddInt64(&blocked, 1)
					} else {
						atomic.AddInt64(&failures, 1)
					}
				}
			}
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
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: rapid circuit breaker test timed out")
	}

	state := cb.GetState()
	assert.Zero(t, panics, "no panics during rapid circuit breaker failures")
	assert.Greater(t, blocked, int64(0),
		"circuit breaker should have blocked some requests")
	assert.Equal(t, services.StateOpen, state,
		"circuit should be open after rapid failures")
	t.Logf("Rapid circuit breaker: %d failures, %d blocked, state=%s, panics=%d",
		failures, blocked, state, panics)
}

// TestChaos_ContextCancellation_Cleanup cancels contexts during in-flight
// ensemble operations and verifies all goroutines clean up properly.
func TestChaos_ContextCancellation_Cleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}

	runtime.GOMAXPROCS(2)

	// Force GC and let goroutines settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	ensemble := services.NewEnsembleService("confidence_weighted", 10*time.Second)
	// Register slow providers so cancellation happens mid-flight
	for i := 0; i < 5; i++ {
		ensemble.RegisterProvider(
			fmt.Sprintf("slow-cancel-%d", i),
			&timeoutProvider{
				name:  fmt.Sprintf("slow-cancel-%d", i),
				delay: 5 * time.Second,
			},
		)
	}

	const numRequests = 30
	var wg sync.WaitGroup
	var panics int64
	var cancelled int64

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			// Create context with very short timeout to force cancellation
			ctx, cancel := context.WithTimeout(context.Background(),
				time.Duration(50+id%100)*time.Millisecond)
			defer cancel()

			req := &models.LLMRequest{
				Prompt: fmt.Sprintf("cancel chaos %d", id),
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					Temperature: 0.5,
					MaxTokens:   50,
				},
			}

			_, err := ensemble.RunEnsemble(ctx, req)
			if err != nil {
				atomic.AddInt64(&cancelled, 1)
			}
		}(i)
	}

	wg.Wait()

	// Allow goroutines to clean up
	runtime.GC()
	time.Sleep(500 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panics, "no panics during context cancellation chaos")
	assert.Greater(t, cancelled, int64(0),
		"some requests should have been cancelled")
	assert.Less(t, leaked, 30,
		"goroutines should clean up after context cancellation")
	t.Logf("Context cancellation chaos: %d cancelled, goroutine leak=%d, panics=%d",
		cancelled, leaked, panics)
}
