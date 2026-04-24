package llm

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/models"
)

// failingProvider is a mock that can be configured to fail
type failingProvider struct {
	shouldFail bool
	mu         sync.Mutex
}

func (p *failingProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	p.mu.Lock()
	fail := p.shouldFail
	p.mu.Unlock()

	if fail {
		return nil, errors.New("provider error")
	}
	return &models.LLMResponse{Content: "success"}, nil
}

func (p *failingProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse)
	go func() {
		defer close(ch)
		p.mu.Lock()
		fail := p.shouldFail
		p.mu.Unlock()

		if !fail {
			ch <- &models.LLMResponse{Content: "stream chunk"}
		}
	}()
	return ch, nil
}

func (p *failingProvider) HealthCheck() error {
	return nil
}

func (p *failingProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{}
}

func (p *failingProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

func (p *failingProvider) SetShouldFail(fail bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.shouldFail = fail
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 2, config.SuccessThreshold)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.HalfOpenMaxRequests)
}

func TestCircuitBreaker_StartsInClosedState(t *testing.T) {
	provider := &failingProvider{}
	cb := NewDefaultCircuitBreaker("test", provider)

	assert.Equal(t, CircuitClosed, cb.GetState())
	assert.True(t, cb.IsClosed())
	assert.False(t, cb.IsOpen())
	assert.False(t, cb.IsHalfOpen())
}

func TestCircuitBreaker_Complete_Success(t *testing.T) {
	provider := &failingProvider{}
	cb := NewDefaultCircuitBreaker("test", provider)

	req := &models.LLMRequest{ID: "test"}
	resp, err := cb.Complete(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "success", resp.Content)

	stats := cb.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.TotalSuccesses)
	assert.Equal(t, int64(0), stats.TotalFailures)
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             1 * time.Minute,
		HalfOpenMaxRequests: 2,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	// Cause failures to open the circuit
	for i := 0; i < 3; i++ {
		_, err := cb.Complete(context.Background(), req)
		assert.Error(t, err)
	}

	assert.Equal(t, CircuitOpen, cb.GetState())
	assert.True(t, cb.IsOpen())
}

func TestCircuitBreaker_RejectsWhenOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		Timeout:             1 * time.Minute,
		HalfOpenMaxRequests: 1,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	// Open the circuit
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)

	assert.True(t, cb.IsOpen())

	// Next request should be rejected immediately
	_, err := cb.Complete(context.Background(), req)
	assert.Equal(t, ErrCircuitOpen, err)
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    3,                      // High threshold so we can observe half-open
		Timeout:             100 * time.Millisecond, // Short timeout for testing
		HalfOpenMaxRequests: 5,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	// Open the circuit
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)
	assert.True(t, cb.IsOpen())

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Provider now succeeds
	provider.SetShouldFail(false)

	// Next request should be allowed and transition to half-open
	_, err := cb.Complete(context.Background(), req)
	assert.NoError(t, err)

	// Should be in half-open (need more successes to close)
	assert.True(t, cb.IsHalfOpen(), "Circuit should be half-open after first success")
}

func TestCircuitBreaker_ClosesAfterSuccessesInHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 5,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	// Open the circuit
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)
	assert.True(t, cb.IsOpen())

	// Wait for timeout and make provider succeed
	time.Sleep(150 * time.Millisecond)
	provider.SetShouldFail(false)

	// Successful requests in half-open should close circuit
	_, _ = cb.Complete(context.Background(), req) // Transitions to half-open
	_, _ = cb.Complete(context.Background(), req)

	assert.True(t, cb.IsClosed())
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 5,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	// Open the circuit
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Provider still failing - request in half-open should reopen
	_, _ = cb.Complete(context.Background(), req)
	assert.True(t, cb.IsOpen())
}

func TestCircuitBreaker_HalfOpenLimitsRequests(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    5,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	// Open the circuit
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	provider.SetShouldFail(false)

	// First request transitions to half-open
	_, _ = cb.Complete(context.Background(), req)
	// Second request allowed
	_, _ = cb.Complete(context.Background(), req)

	// Third request should be rejected (over limit)
	_, err := cb.Complete(context.Background(), req)
	assert.Equal(t, ErrCircuitHalfOpenRejected, err)
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	// Open the circuit
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)
	assert.True(t, cb.IsOpen())

	// Reset
	cb.Reset()
	assert.True(t, cb.IsClosed())

	stats := cb.GetStats()
	assert.Equal(t, 0, stats.ConsecutiveFailures)
}

func TestCircuitBreaker_Stats(t *testing.T) {
	provider := &failingProvider{}
	cb := NewDefaultCircuitBreaker("test-provider", provider)

	req := &models.LLMRequest{ID: "test"}

	// Make some requests
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)
	provider.SetShouldFail(true)
	_, _ = cb.Complete(context.Background(), req)

	stats := cb.GetStats()
	assert.Equal(t, "test-provider", stats.ProviderID)
	assert.Equal(t, int64(3), stats.TotalRequests)
	assert.Equal(t, int64(2), stats.TotalSuccesses)
	assert.Equal(t, int64(1), stats.TotalFailures)
}

func TestCircuitBreaker_Listener(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("test", provider, config)

	stateChanges := make([]CircuitState, 0)
	var mu sync.Mutex

	cb.AddListener(func(providerID string, oldState, newState CircuitState) {
		mu.Lock()
		stateChanges = append(stateChanges, newState)
		mu.Unlock()
	})

	req := &models.LLMRequest{ID: "test"}

	// Trigger state change
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)

	// Wait for listener
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.Contains(t, stateChanges, CircuitOpen)
	mu.Unlock()
}

func TestCircuitBreakerManager_Register(t *testing.T) {
	mgr := NewDefaultCircuitBreakerManager()
	provider := &failingProvider{}

	cb := mgr.Register("test", provider)
	assert.NotNil(t, cb)

	retrieved, exists := mgr.Get("test")
	assert.True(t, exists)
	assert.Equal(t, cb, retrieved)
}

func TestCircuitBreakerManager_Unregister(t *testing.T) {
	mgr := NewDefaultCircuitBreakerManager()
	provider := &failingProvider{}

	mgr.Register("test", provider)
	mgr.Unregister("test")

	_, exists := mgr.Get("test")
	assert.False(t, exists)
}

func TestCircuitBreakerManager_GetAllStats(t *testing.T) {
	mgr := NewDefaultCircuitBreakerManager()

	mgr.Register("provider1", &failingProvider{})
	mgr.Register("provider2", &failingProvider{})

	stats := mgr.GetAllStats()
	assert.Len(t, stats, 2)
	assert.Contains(t, stats, "provider1")
	assert.Contains(t, stats, "provider2")
}

func TestCircuitBreakerManager_GetAvailableProviders(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
	}
	mgr := NewCircuitBreakerManager(config)

	mgr.Register("healthy", &failingProvider{})
	cb := mgr.Register("unhealthy", &failingProvider{shouldFail: true})

	// Open the unhealthy circuit
	req := &models.LLMRequest{ID: "test"}
	_, _ = cb.Complete(context.Background(), req)
	_, _ = cb.Complete(context.Background(), req)

	available := mgr.GetAvailableProviders()
	assert.Contains(t, available, "healthy")
	assert.NotContains(t, available, "unhealthy")
}

func TestCircuitBreakerManager_ResetAll(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 2,
	}
	mgr := NewCircuitBreakerManager(config)

	cb1 := mgr.Register("p1", &failingProvider{shouldFail: true})
	cb2 := mgr.Register("p2", &failingProvider{shouldFail: true})

	req := &models.LLMRequest{ID: "test"}

	// Open both circuits
	_, _ = cb1.Complete(context.Background(), req)
	_, _ = cb1.Complete(context.Background(), req)
	_, _ = cb2.Complete(context.Background(), req)
	_, _ = cb2.Complete(context.Background(), req)

	assert.True(t, cb1.IsOpen())
	assert.True(t, cb2.IsOpen())

	mgr.ResetAll()

	assert.True(t, cb1.IsClosed())
	assert.True(t, cb2.IsClosed())
}

func TestCircuitBreaker_CompleteStream_Success(t *testing.T) {
	provider := &failingProvider{}
	cb := NewDefaultCircuitBreaker("test", provider)

	req := &models.LLMRequest{ID: "test"}
	ch, err := cb.CompleteStream(context.Background(), req)

	assert.NoError(t, err)

	// Drain the channel
	for range ch {
	}

	time.Sleep(50 * time.Millisecond) // Wait for goroutine

	stats := cb.GetStats()
	assert.Equal(t, int64(1), stats.TotalSuccesses)
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:    10,
		SuccessThreshold:    5,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 5,
	}
	provider := &failingProvider{}
	cb := NewCircuitBreaker("test", provider, config)

	req := &models.LLMRequest{ID: "test"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cb.Complete(context.Background(), req)
			_ = cb.GetStats()
			_ = cb.GetState()
		}()
	}

	wg.Wait()

	stats := cb.GetStats()
	assert.Equal(t, int64(100), stats.TotalRequests)
}

// logrusWarnHook captures logrus Warn-level entries for test assertions.
//
// Concurrency model (CONST-029): entries → *safe.Slice.
type logrusWarnHook struct {
	entries *safe.Slice[string]
}

func newLogrusWarnHook() *logrusWarnHook {
	return &logrusWarnHook{entries: safe.NewSlice[string]()}
}

func (h *logrusWarnHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.WarnLevel}
}

func (h *logrusWarnHook) Fire(entry *logrus.Entry) error {
	if h.entries == nil {
		h.entries = safe.NewSlice[string]()
	}
	h.entries.Append(entry.Message)
	return nil
}

func (h *logrusWarnHook) messages() []string {
	if h.entries == nil {
		return nil
	}
	return h.entries.Snapshot()
}

// TestCircuitBreaker_ListenerNotifyTimeout_TransitionTo verifies that a
// slow listener triggers a warn log via transitionTo.
func TestCircuitBreaker_ListenerNotifyTimeout_TransitionTo(t *testing.T) {
	// Override the global timeout to a short value so the test finishes quickly.
	orig := listenerNotifyTimeoutNs.Load()
	listenerNotifyTimeoutNs.Store(int64(50 * time.Millisecond))
	defer listenerNotifyTimeoutNs.Store(orig)

	hook := newLogrusWarnHook()
	logrus.AddHook(hook)
	defer logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})

	config := CircuitBreakerConfig{
		FailureThreshold:    1,
		SuccessThreshold:    1,
		Timeout:             500 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("timeout-test", provider, config)

	// Register a listener that blocks longer than the notify timeout.
	blockCh := make(chan struct{})
	cb.AddListener(func(providerID string, oldState, newState CircuitState) {
		<-blockCh // blocks until test closes the channel
	})

	// Force a state transition (closed → open) by recording a failure.
	req := &models.LLMRequest{ID: "r1"}
	_, _ = cb.Complete(context.Background(), req)

	// Poll for the timed-out log up to 2s. A fixed 200ms was too aggressive
	// under CPU contention (seen empty message list under concurrent
	// release-all build); the timeout+goroutine+log-emission+hook-capture
	// chain can take longer than the notify timeout itself.
	found := false
	deadline := time.Now().Add(2 * time.Second)
	var msgs []string
	for time.Now().Before(deadline) {
		msgs = hook.messages()
		for _, m := range msgs {
			if strings.Contains(m, "timed out") {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(blockCh) // unblock the listener goroutine
	assert.True(t, found, "expected a 'timed out' warn log, got: %v", msgs)
}

// TestCircuitBreaker_ListenerNotifyTimeout_Reset verifies that a slow
// listener during Reset triggers a warn log.
func TestCircuitBreaker_ListenerNotifyTimeout_Reset(t *testing.T) {
	orig := listenerNotifyTimeoutNs.Load()
	listenerNotifyTimeoutNs.Store(int64(50 * time.Millisecond))
	defer listenerNotifyTimeoutNs.Store(orig)

	hook := newLogrusWarnHook()
	logrus.AddHook(hook)
	defer logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})

	config := CircuitBreakerConfig{
		FailureThreshold:    1,
		SuccessThreshold:    1,
		Timeout:             500 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	}
	provider := &failingProvider{shouldFail: true}
	cb := NewCircuitBreaker("reset-timeout-test", provider, config)

	// First listener unblocks immediately; second blocks to force Reset timeout.
	blockCh := make(chan struct{})
	cb.AddListener(func(providerID string, oldState, newState CircuitState) {
		<-blockCh
	})

	// Open the circuit (triggers listener which blocks, then times out).
	req := &models.LLMRequest{ID: "r1"}
	_, _ = cb.Complete(context.Background(), req)

	// Unblock the first (transitionTo) listener and wait for its goroutine to settle.
	close(blockCh)
	time.Sleep(100 * time.Millisecond)

	// Add a fresh blocking listener specifically for the Reset notification.
	blockCh2 := make(chan struct{})
	cb.AddListener(func(providerID string, oldState, newState CircuitState) {
		<-blockCh2
	})

	cb.Reset()

	time.Sleep(200 * time.Millisecond)
	close(blockCh2)

	msgs := hook.messages()
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "timed out") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected a 'timed out' warn log on reset, got: %v", msgs)
}
