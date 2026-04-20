package llm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
)

// CircuitState represents the state of the circuit breaker
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"    // Normal operation
	CircuitOpen     CircuitState = "open"      // Failing, rejecting requests
	CircuitHalfOpen CircuitState = "half_open" // Testing with limited requests
)

// listenerNotifyTimeoutNs stores the listener notification timeout in nanoseconds.
// Accessed via atomic operations so tests can override it safely from another goroutine.
var listenerNotifyTimeoutNs atomic.Int64

func init() { listenerNotifyTimeoutNs.Store(int64(5 * time.Second)) }

// ErrCircuitOpen is returned when circuit is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// ErrCircuitHalfOpenRejected is returned when half-open circuit rejects request
var ErrCircuitHalfOpenRejected = errors.New("circuit breaker in half-open state, request rejected")

// CircuitBreakerConfig configures the circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold    int           // Number of failures to open circuit
	SuccessThreshold    int           // Number of successes in half-open to close
	Timeout             time.Duration // How long to stay open before half-open
	HalfOpenMaxRequests int           // Max requests allowed in half-open state
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern for LLM providers.
//
// Concurrency: the mu survives as Pattern Zeta — state, counters, and
// timestamps have joint invariants (e.g., a state flip to Open must be
// paired with a lastStateChange update and a zeroed halfOpenRequests) so
// a single mutex still serialises them. What is removed from the bare
// map/slice Pattern-A is the listeners map: it now lives in a safe.Store
// and mu no longer guards it, so the audit is satisfied.
type CircuitBreaker struct {
	mu                   sync.RWMutex
	provider             LLMProvider
	providerID           string
	config               CircuitBreakerConfig
	state                CircuitState
	failures             int
	successes            int
	consecutiveFailures  int
	consecutiveSuccesses int
	lastFailure          time.Time
	lastStateChange      time.Time
	halfOpenRequests     int
	totalRequests        int64
	totalFailures        int64
	totalSuccesses       int64
	listeners            *safe.Store[int64, CircuitBreakerListener]
	nextListenerID       atomic.Int64
}

// MaxCircuitBreakerListeners limits listener count to prevent memory leaks
const MaxCircuitBreakerListeners = 100

// CircuitBreakerListener is called when circuit state changes
type CircuitBreakerListener func(providerID string, oldState, newState CircuitState)

// NewCircuitBreaker creates a new circuit breaker for a provider
func NewCircuitBreaker(providerID string, provider LLMProvider, config CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		provider:        provider,
		providerID:      providerID,
		config:          config,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
		listeners:       safe.NewStore[int64, CircuitBreakerListener](),
	}
	cb.nextListenerID.Store(1)
	return cb
}

// NewDefaultCircuitBreaker creates a circuit breaker with default config
func NewDefaultCircuitBreaker(providerID string, provider LLMProvider) *CircuitBreaker {
	return NewCircuitBreaker(providerID, provider, DefaultCircuitBreakerConfig())
}

// AddListener adds a listener for state changes and returns an ID for removal.
// Returns -1 and logs a warning if max listeners reached (listener not added).
func (cb *CircuitBreaker) AddListener(listener CircuitBreakerListener) int {
	// PERFORMANCE FIX: Limit listener count to prevent memory leaks.
	// Store.Len() + Store.Put() is not atomic together; the cap check
	// is best-effort and may admit one or two extra under heavy churn,
	// which is acceptable for a 100-slot warning threshold.
	if cb.listeners.Len() >= MaxCircuitBreakerListeners {
		logrus.Warnf("circuit breaker %s: listener limit reached (%d), listener not added",
			cb.providerID, MaxCircuitBreakerListeners)
		return -1
	}
	id := cb.nextListenerID.Add(1) - 1 // start IDs at 1 (matches old semantics)
	cb.listeners.Put(id, listener)
	if cb.listeners.Len() > MaxCircuitBreakerListeners*80/100 {
		logrus.Warnf("circuit breaker %s: listener count %d approaching max %d",
			cb.providerID, cb.listeners.Len(), MaxCircuitBreakerListeners)
	}
	return int(id)
}

// RemoveListener removes a listener by its ID. Returns true if found and removed.
// PERFORMANCE FIX: Added to prevent memory leaks from unremoved listeners.
func (cb *CircuitBreaker) RemoveListener(id int) bool {
	_, existed := cb.listeners.Delete(int64(id))
	return existed
}

// ListenerCount returns the current number of registered listeners.
func (cb *CircuitBreaker) ListenerCount() int {
	return cb.listeners.Len()
}

// Complete wraps the provider's Complete method with circuit breaker logic
func (cb *CircuitBreaker) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if err := cb.beforeRequest(); err != nil {
		return nil, err
	}

	resp, err := cb.provider.Complete(ctx, req)
	cb.afterRequest(err)

	return resp, err
}

// CompleteStream wraps the provider's CompleteStream method with circuit breaker logic
func (cb *CircuitBreaker) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if err := cb.beforeRequest(); err != nil {
		return nil, err
	}

	ch, err := cb.provider.CompleteStream(ctx, req)
	if err != nil {
		cb.afterRequest(err)
		return nil, err
	}

	// Wrap the channel to track success/failure.
	// The select on ctx.Done() prevents a goroutine leak when the caller
	// abandons the stream (context timeout/cancel).
	wrappedCh := make(chan *models.LLMResponse)
	go func() {
		defer close(wrappedCh)
		var lastResp *models.LLMResponse
		for {
			select {
			case resp, ok := <-ch:
				if !ok {
					// Upstream channel closed — stream completed normally.
					if lastResp != nil {
						cb.afterRequest(nil)
					} else {
						cb.afterRequest(errors.New("empty stream"))
					}
					return
				}
				lastResp = resp
				select {
				case wrappedCh <- resp:
				case <-ctx.Done():
					cb.afterRequest(ctx.Err())
					return
				}
			case <-ctx.Done():
				cb.afterRequest(ctx.Err())
				return
			}
		}
	}()

	return wrappedCh, nil
}

// HealthCheck wraps the provider's HealthCheck method
func (cb *CircuitBreaker) HealthCheck() error {
	return cb.provider.HealthCheck()
}

// GetCapabilities returns the provider's capabilities
func (cb *CircuitBreaker) GetCapabilities() *models.ProviderCapabilities {
	return cb.provider.GetCapabilities()
}

// ValidateConfig validates the provider's configuration
func (cb *CircuitBreaker) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return cb.provider.ValidateConfig(config)
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	switch cb.state {
	case CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.transitionTo(CircuitHalfOpen)
			cb.halfOpenRequests = 1
			return nil
		}
		return ErrCircuitOpen

	case CircuitHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests >= cb.config.HalfOpenMaxRequests {
			return ErrCircuitHalfOpenRejected
		}
		cb.halfOpenRequests++
		return nil

	case CircuitClosed:
		return nil
	}

	return nil
}

// afterRequest records the result of the request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failed request
func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.totalFailures++
	cb.consecutiveFailures++
	cb.consecutiveSuccesses = 0
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.consecutiveFailures >= cb.config.FailureThreshold {
			cb.transitionTo(CircuitOpen)
		}
	case CircuitHalfOpen:
		// Any failure in half-open returns to open
		cb.transitionTo(CircuitOpen)
	}
}

// recordSuccess records a successful request
func (cb *CircuitBreaker) recordSuccess() {
	cb.successes++
	cb.totalSuccesses++
	cb.consecutiveSuccesses++
	cb.consecutiveFailures = 0

	switch cb.state {
	case CircuitHalfOpen:
		if cb.consecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	}
}

// transitionTo changes the circuit state
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Reset counters on state change
	if newState == CircuitClosed {
		cb.consecutiveFailures = 0
		cb.failures = 0
	} else if newState == CircuitHalfOpen {
		cb.halfOpenRequests = 0
		cb.consecutiveSuccesses = 0
	}

	// CONCURRENCY FIX: Notify listeners with timeout to prevent goroutine leaks
	// PERFORMANCE FIX: Snapshot listeners via safe.Store.Values to avoid
	// holding cb.mu during notification (listeners is independently
	// synchronised via its own Store, so this read does not need mu).
	listeners := cb.listeners.Values()

	for _, listener := range listeners {
		go func(l CircuitBreakerListener) {
			// Use a timer to prevent infinite blocking in listener
			done := make(chan struct{})
			go func() {
				defer close(done)
				l(cb.providerID, oldState, newState)
			}()

			// Wait for listener with 5 second timeout
			select {
			case <-done:
				// Listener completed
			case <-time.After(time.Duration(listenerNotifyTimeoutNs.Load())):
				logrus.WithFields(logrus.Fields{
					"provider":  cb.providerID,
					"old_state": string(oldState),
					"new_state": string(newState),
				}).Warn("circuit breaker listener notification timed out")
			}
		}(listener)
	}
}

// GetState returns the current circuit state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		ProviderID:           cb.providerID,
		State:                cb.state,
		TotalRequests:        cb.totalRequests,
		TotalSuccesses:       cb.totalSuccesses,
		TotalFailures:        cb.totalFailures,
		ConsecutiveFailures:  cb.consecutiveFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		LastFailure:          cb.lastFailure,
		LastStateChange:      cb.lastStateChange,
	}
}

// CircuitBreakerStats contains circuit breaker statistics
type CircuitBreakerStats struct {
	ProviderID           string       `json:"provider_id"`
	State                CircuitState `json:"state"`
	TotalRequests        int64        `json:"total_requests"`
	TotalSuccesses       int64        `json:"total_successes"`
	TotalFailures        int64        `json:"total_failures"`
	ConsecutiveFailures  int          `json:"consecutive_failures"`
	ConsecutiveSuccesses int          `json:"consecutive_successes"`
	LastFailure          time.Time    `json:"last_failure,omitempty"`
	LastStateChange      time.Time    `json:"last_state_change"`
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()

	oldState := cb.state
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.halfOpenRequests = 0
	cb.lastStateChange = time.Now()

	// CONCURRENCY FIX: snapshot listeners before unlocking (via safe.Store,
	// which needs no external lock — decoupled from cb.mu).
	var listeners []CircuitBreakerListener
	if oldState != CircuitClosed {
		listeners = cb.listeners.Values()
	}
	providerID := cb.providerID

	cb.mu.Unlock()

	// Notify listeners with timeout (after unlocking to avoid deadlock)
	for _, listener := range listeners {
		go func(l CircuitBreakerListener) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				l(providerID, oldState, CircuitClosed)
			}()

			select {
			case <-done:
			case <-time.After(time.Duration(listenerNotifyTimeoutNs.Load())):
				logrus.WithFields(logrus.Fields{
					"provider":  providerID,
					"old_state": string(oldState),
					"new_state": string(CircuitClosed),
				}).Warn("circuit breaker listener notification timed out on reset")
			}
		}(listener)
	}
}

// IsOpen returns true if the circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitOpen
}

// IsClosed returns true if the circuit is closed
func (cb *CircuitBreaker) IsClosed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitClosed
}

// IsHalfOpen returns true if the circuit is half-open
func (cb *CircuitBreaker) IsHalfOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitHalfOpen
}

// CircuitBreakerManager manages multiple circuit breakers.
//
// Concurrent-safe by construction: breakers is a safe.Store, so the
// prior sync.RWMutex is dropped entirely. Reads (Get, GetAllStats,
// GetAvailableProviders, ResetAll) iterate via Range/Values; writes
// (Register, Unregister) use Put/Delete.
type CircuitBreakerManager struct {
	breakers *safe.Store[string, *CircuitBreaker]
	config   CircuitBreakerConfig
}

// NewCircuitBreakerManager creates a new manager
func NewCircuitBreakerManager(config CircuitBreakerConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: safe.NewStore[string, *CircuitBreaker](),
		config:   config,
	}
}

// NewDefaultCircuitBreakerManager creates a manager with default config
func NewDefaultCircuitBreakerManager() *CircuitBreakerManager {
	return NewCircuitBreakerManager(DefaultCircuitBreakerConfig())
}

// Register registers a provider with a circuit breaker
func (cbm *CircuitBreakerManager) Register(providerID string, provider LLMProvider) *CircuitBreaker {
	cb := NewCircuitBreaker(providerID, provider, cbm.config)
	cbm.breakers.Put(providerID, cb)
	return cb
}

// Get returns the circuit breaker for a provider
func (cbm *CircuitBreakerManager) Get(providerID string) (*CircuitBreaker, bool) {
	return cbm.breakers.Get(providerID)
}

// Unregister removes a provider's circuit breaker
func (cbm *CircuitBreakerManager) Unregister(providerID string) {
	cbm.breakers.Delete(providerID)
}

// GetAllStats returns stats for all circuit breakers
func (cbm *CircuitBreakerManager) GetAllStats() map[string]CircuitBreakerStats {
	stats := make(map[string]CircuitBreakerStats)
	cbm.breakers.Range(func(id string, cb *CircuitBreaker) bool {
		stats[id] = cb.GetStats()
		return true
	})
	return stats
}

// GetAvailableProviders returns IDs of providers with closed circuits
func (cbm *CircuitBreakerManager) GetAvailableProviders() []string {
	var available []string
	cbm.breakers.Range(func(id string, cb *CircuitBreaker) bool {
		if cb.IsClosed() || cb.IsHalfOpen() {
			available = append(available, id)
		}
		return true
	})
	return available
}

// ResetAll resets all circuit breakers
func (cbm *CircuitBreakerManager) ResetAll() {
	cbm.breakers.Range(func(_ string, cb *CircuitBreaker) bool {
		cb.Reset()
		return true
	})
}
