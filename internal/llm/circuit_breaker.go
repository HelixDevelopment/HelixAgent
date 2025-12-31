package llm

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// CircuitState represents the state of the circuit breaker
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"   // Normal operation
	CircuitOpen     CircuitState = "open"     // Failing, rejecting requests
	CircuitHalfOpen CircuitState = "half_open" // Testing with limited requests
)

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

// CircuitBreaker implements the circuit breaker pattern for LLM providers
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
	listeners            []CircuitBreakerListener
}

// CircuitBreakerListener is called when circuit state changes
type CircuitBreakerListener func(providerID string, oldState, newState CircuitState)

// NewCircuitBreaker creates a new circuit breaker for a provider
func NewCircuitBreaker(providerID string, provider LLMProvider, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		provider:        provider,
		providerID:      providerID,
		config:          config,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
		listeners:       make([]CircuitBreakerListener, 0),
	}
}

// NewDefaultCircuitBreaker creates a circuit breaker with default config
func NewDefaultCircuitBreaker(providerID string, provider LLMProvider) *CircuitBreaker {
	return NewCircuitBreaker(providerID, provider, DefaultCircuitBreakerConfig())
}

// AddListener adds a listener for state changes
func (cb *CircuitBreaker) AddListener(listener CircuitBreakerListener) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.listeners = append(cb.listeners, listener)
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

	// Wrap the channel to track success/failure
	wrappedCh := make(chan *models.LLMResponse)
	go func() {
		defer close(wrappedCh)
		var lastResp *models.LLMResponse
		for resp := range ch {
			lastResp = resp
			wrappedCh <- resp
		}
		// Consider it a success if we got at least one response
		if lastResp != nil {
			cb.afterRequest(nil)
		} else {
			cb.afterRequest(errors.New("empty stream"))
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

	// Notify listeners
	for _, listener := range cb.listeners {
		go listener(cb.providerID, oldState, newState)
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
	ProviderID           string        `json:"provider_id"`
	State                CircuitState  `json:"state"`
	TotalRequests        int64         `json:"total_requests"`
	TotalSuccesses       int64         `json:"total_successes"`
	TotalFailures        int64         `json:"total_failures"`
	ConsecutiveFailures  int           `json:"consecutive_failures"`
	ConsecutiveSuccesses int           `json:"consecutive_successes"`
	LastFailure          time.Time     `json:"last_failure,omitempty"`
	LastStateChange      time.Time     `json:"last_state_change"`
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.halfOpenRequests = 0
	cb.lastStateChange = time.Now()

	if oldState != CircuitClosed {
		for _, listener := range cb.listeners {
			go listener(cb.providerID, oldState, CircuitClosed)
		}
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

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
}

// NewCircuitBreakerManager creates a new manager
func NewCircuitBreakerManager(config CircuitBreakerConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// NewDefaultCircuitBreakerManager creates a manager with default config
func NewDefaultCircuitBreakerManager() *CircuitBreakerManager {
	return NewCircuitBreakerManager(DefaultCircuitBreakerConfig())
}

// Register registers a provider with a circuit breaker
func (cbm *CircuitBreakerManager) Register(providerID string, provider LLMProvider) *CircuitBreaker {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	cb := NewCircuitBreaker(providerID, provider, cbm.config)
	cbm.breakers[providerID] = cb
	return cb
}

// Get returns the circuit breaker for a provider
func (cbm *CircuitBreakerManager) Get(providerID string) (*CircuitBreaker, bool) {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	cb, exists := cbm.breakers[providerID]
	return cb, exists
}

// Unregister removes a provider's circuit breaker
func (cbm *CircuitBreakerManager) Unregister(providerID string) {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()
	delete(cbm.breakers, providerID)
}

// GetAllStats returns stats for all circuit breakers
func (cbm *CircuitBreakerManager) GetAllStats() map[string]CircuitBreakerStats {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for id, cb := range cbm.breakers {
		stats[id] = cb.GetStats()
	}
	return stats
}

// GetAvailableProviders returns IDs of providers with closed circuits
func (cbm *CircuitBreakerManager) GetAvailableProviders() []string {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	var available []string
	for id, cb := range cbm.breakers {
		if cb.IsClosed() || cb.IsHalfOpen() {
			available = append(available, id)
		}
	}
	return available
}

// ResetAll resets all circuit breakers
func (cbm *CircuitBreakerManager) ResetAll() {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	for _, cb := range cbm.breakers {
		cb.Reset()
	}
}
