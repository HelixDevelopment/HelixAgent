package llm

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// slowStreamProvider sends responses with a delay, allowing tests to cancel
// mid-stream and verify goroutine cleanup.
type slowStreamProvider struct {
	mu            sync.Mutex
	chunkCount    int
	chunkInterval time.Duration
}

func (p *slowStreamProvider) Complete(_ context.Context, _ *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{Content: "ok"}, nil
}

func (p *slowStreamProvider) CompleteStream(_ context.Context, _ *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	p.mu.Lock()
	count := p.chunkCount
	interval := p.chunkInterval
	p.mu.Unlock()

	ch := make(chan *models.LLMResponse)
	go func() {
		defer close(ch)
		for i := 0; i < count; i++ {
			if interval > 0 {
				time.Sleep(interval)
			}
			ch <- &models.LLMResponse{Content: "chunk"}
		}
	}()
	return ch, nil
}

func (p *slowStreamProvider) HealthCheck() error {
	return nil
}

func (p *slowStreamProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{}
}

func (p *slowStreamProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// TestCircuitBreaker_CompleteStream_NoGoroutineLeak verifies that after a
// stream completes normally, the forwarding goroutine exits and does not leak.
func TestCircuitBreaker_CompleteStream_NoGoroutineLeak(t *testing.T) {
	// Let the runtime settle before capturing baseline.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	provider := &slowStreamProvider{chunkCount: 5, chunkInterval: 10 * time.Millisecond}
	cb := NewDefaultCircuitBreaker("leak-test-normal", provider)
	req := &models.LLMRequest{ID: "test"}

	ch, err := cb.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	// Drain the channel completely.
	count := 0
	for range ch {
		count++
	}
	assert.Equal(t, 5, count, "should receive all 5 chunks")

	// Allow goroutines to finish.
	time.Sleep(300 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+3,
		"goroutine count should return near baseline after normal stream completion (baseline=%d, after=%d)", baseline, after)

	// Verify circuit breaker recorded a success.
	stats := cb.GetStats()
	assert.Equal(t, int64(1), stats.TotalSuccesses, "stream should be recorded as success")
}

// longStreamProvider sends chunks continuously until its stop channel is
// closed. Unlike a truly hanging provider, its goroutine can be cleaned up
// by the test so we can accurately measure CB wrapper goroutine leaks.
type longStreamProvider struct {
	mu   sync.Mutex
	stop chan struct{} // closed by the test to stop all active streams
}

func newLongStreamProvider() *longStreamProvider {
	return &longStreamProvider{stop: make(chan struct{})}
}

func (p *longStreamProvider) Complete(_ context.Context, _ *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{Content: "ok"}, nil
}

func (p *longStreamProvider) CompleteStream(_ context.Context, _ *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	p.mu.Lock()
	stop := p.stop
	p.mu.Unlock()

	ch := make(chan *models.LLMResponse)
	go func() {
		defer close(ch)
		for {
			select {
			case <-stop:
				return
			case ch <- &models.LLMResponse{Content: "chunk"}:
			}
		}
	}()
	return ch, nil
}

func (p *longStreamProvider) HealthCheck() error { return nil }
func (p *longStreamProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{}
}
func (p *longStreamProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// TestCircuitBreaker_CompleteStream_ContextCancelNoLeak verifies that when the
// caller cancels the context mid-stream, the forwarding goroutine exits
// promptly and does not leak. We open 10 streams and cancel them all; without
// the fix each leaked stream leaves at least 1 goroutine stuck on wrappedCh,
// so the goroutine count would grow by 10+ above baseline.
func TestCircuitBreaker_CompleteStream_ContextCancelNoLeak(t *testing.T) {
	// Let the runtime settle before capturing baseline.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const iterations = 10
	provider := newLongStreamProvider()
	// Use a high failure threshold so context cancellation errors don't
	// trip the circuit open during the test loop.
	cfg := CircuitBreakerConfig{
		FailureThreshold:    iterations + 5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
	cb := NewCircuitBreaker("leak-test-cancel", provider, cfg)
	req := &models.LLMRequest{ID: "test"}

	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		ch, err := cb.CompleteStream(ctx, req)
		require.NoError(t, err)

		// Read one chunk to prove the stream is active, then cancel.
		resp, ok := <-ch
		require.True(t, ok, "should receive at least one chunk")
		require.NotNil(t, resp)

		cancel()

		// Brief pause to let goroutine observe cancellation.
		time.Sleep(20 * time.Millisecond)
	}

	// Stop the provider so its goroutines exit, isolating CB wrapper leaks.
	close(provider.stop)

	// Allow goroutines to clean up after the last cancel.
	time.Sleep(300 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	// With the fix, the wrapper goroutines exit on ctx.Done(). Without the
	// fix, each iteration leaks at least 1 goroutine (stuck on wrappedCh send),
	// so after would be baseline + 10+. We allow a margin of 3 for runtime jitter.
	assert.LessOrEqual(t, after, baseline+3,
		"goroutine count should return near baseline after %d cancelled streams (baseline=%d, after=%d)",
		iterations, baseline, after)
}

// alwaysFailingProvider always returns an error from Complete, allowing tests
// to drive the circuit breaker into the open state by calling afterRequest.
type alwaysFailingProvider struct{}

func (p *alwaysFailingProvider) Complete(_ context.Context, _ *models.LLMRequest) (*models.LLMResponse, error) {
	return nil, errors.New("always fails")
}
func (p *alwaysFailingProvider) CompleteStream(_ context.Context, _ *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return nil, errors.New("always fails")
}
func (p *alwaysFailingProvider) HealthCheck() error { return nil }
func (p *alwaysFailingProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{}
}
func (p *alwaysFailingProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// TestCircuitBreaker_NotifyListeners_NoGoroutineLeak verifies that after
// a listener is notified on state change (closed → open), the notification
// goroutines exit cleanly and do not leak.
func TestCircuitBreaker_NotifyListeners_NoGoroutineLeak(t *testing.T) {
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	provider := &alwaysFailingProvider{}
	cfg := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
	cb := NewCircuitBreaker("listener-leak-test", provider, cfg)

	var called atomic.Int32
	cb.AddListener(func(_ string, _, newState CircuitState) {
		called.Add(1)
		assert.Equal(t, CircuitOpen, newState, "listener should see transition to open")
	})

	// Drive the circuit breaker to open by recording consecutive failures.
	for i := 0; i < cfg.FailureThreshold; i++ {
		cb.afterRequest(errors.New("test failure"))
	}

	// Allow notification goroutines to complete.
	time.Sleep(300 * time.Millisecond)
	runtime.GC()

	assert.Equal(t, int32(1), called.Load(), "listener should have been called exactly once")
	assert.Equal(t, CircuitOpen, cb.GetState(), "circuit should be open")

	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+3,
		"goroutine count should return near baseline after listener notification (baseline=%d, after=%d)",
		baseline, after)
}

// TestCircuitBreaker_NotifyListeners_SlowListenerTimesOut verifies that a
// listener that blocks forever does not prevent the outer notification
// goroutine from exiting. The outer goroutine should time out and return,
// leaving at most 1 stuck inner goroutine per listener (which is expected
// and unavoidable when a listener never returns).
func TestCircuitBreaker_NotifyListeners_SlowListenerTimesOut(t *testing.T) {
	// Override the listener notification timeout to 100ms for fast testing.
	oldTimeout := listenerNotifyTimeoutNs.Load()
	listenerNotifyTimeoutNs.Store(int64(100 * time.Millisecond))
	defer listenerNotifyTimeoutNs.Store(oldTimeout)

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	provider := &alwaysFailingProvider{}
	cfg := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
	cb := NewCircuitBreaker("slow-listener-test", provider, cfg)

	// Register a listener that blocks forever.
	cb.AddListener(func(_ string, _, _ CircuitState) {
		select {} // block forever
	})

	// Drive the circuit breaker to open.
	for i := 0; i < cfg.FailureThreshold; i++ {
		cb.afterRequest(errors.New("test failure"))
	}

	// Wait past the timeout so the outer goroutine exits.
	time.Sleep(300 * time.Millisecond)
	runtime.GC()

	after := runtime.NumGoroutine()
	// The outer notification goroutine should have exited after timeout.
	// We allow +3 margin: 1 for the stuck inner goroutine (unavoidable since
	// the listener never returns) plus runtime jitter.
	assert.LessOrEqual(t, after, baseline+3,
		"outer notification goroutine should exit after timeout (baseline=%d, after=%d)",
		baseline, after)
	assert.Equal(t, CircuitOpen, cb.GetState(), "circuit should be open")
}
