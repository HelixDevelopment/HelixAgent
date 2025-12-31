package ratelimit

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// TokenBucketConfig holds configuration for a token bucket rate limiter
type TokenBucketConfig struct {
	Capacity   float64 // Maximum number of tokens
	RefillRate float64 // Tokens added per second
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(config TokenBucketConfig) *TokenBucket {
	return &TokenBucket{
		tokens:     config.Capacity,
		capacity:   config.Capacity,
		refillRate: config.RefillRate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available
func (tb *TokenBucket) Wait(ctx context.Context) error {
	for {
		if tb.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Continue trying
		}
	}
}

// Allow checks if a token is available
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1.0 {
		tb.tokens--
		return true
	}

	return false
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	tokensToAdd := elapsed.Seconds() * tb.refillRate
	tb.tokens += tokensToAdd

	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	tb.lastRefill = now
}

// Limiter represents a rate limiter (alias for TokenBucket for backward compatibility)
type Limiter = TokenBucket

// NewLimiter creates a new rate limiter (backward compatibility)
func NewLimiter(capacity float64, refillRate float64) *Limiter {
	config := TokenBucketConfig{
		Capacity:   capacity,
		RefillRate: refillRate,
	}
	return NewTokenBucket(config)
}

// SlidingWindowLimiter implements a sliding window rate limiter
type SlidingWindowLimiter struct {
	mu          sync.Mutex
	window      time.Duration
	maxRequests int
	requests    []time.Time
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter
func NewSlidingWindowLimiter(window time.Duration, maxRequests int) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		window:      window,
		maxRequests: maxRequests,
		requests:    make([]time.Time, 0),
	}
}

// Allow checks if a request should be allowed
func (sw *SlidingWindowLimiter) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)

	// Remove old requests outside the window
	validRequests := make([]time.Time, 0)
	for _, req := range sw.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	sw.requests = validRequests

	if len(sw.requests) < sw.maxRequests {
		sw.requests = append(sw.requests, now)
		return true
	}

	return false
}

// Wait blocks until a request can be allowed
func (sw *SlidingWindowLimiter) Wait(ctx context.Context) error {
	for {
		if sw.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Continue trying
		}
	}
}

// Middleware creates HTTP middleware for rate limiting
type Middleware struct {
	limiter RateLimiter
}

// RateLimiter interface for different rate limiting strategies
type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
}

// NewMiddleware creates new rate limiting middleware
func NewMiddleware(limiter RateLimiter) *Middleware {
	return &Middleware{
		limiter: limiter,
	}
}

// Handler wraps an HTTP handler with rate limiting
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// WaitHandler wraps an HTTP handler with rate limiting that waits
func (m *Middleware) WaitHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.limiter.Wait(r.Context()); err != nil {
			http.Error(w, "Request cancelled", http.StatusRequestTimeout)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// PerKeyLimiter implements per-key rate limiting (e.g., per IP, per user)
type PerKeyLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*TokenBucket
	config   TokenBucketConfig
}

// NewPerKeyLimiter creates a new per-key rate limiter
func NewPerKeyLimiter(config TokenBucketConfig) *PerKeyLimiter {
	return &PerKeyLimiter{
		limiters: make(map[string]*TokenBucket),
		config:   config,
	}
}

// Allow checks if a request for the given key should be allowed
func (p *PerKeyLimiter) Allow(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	limiter, exists := p.limiters[key]
	if !exists {
		limiter = NewTokenBucket(p.config)
		p.limiters[key] = limiter
	}

	return limiter.Allow()
}

// Wait blocks until a request for the given key can be allowed
func (p *PerKeyLimiter) Wait(ctx context.Context, key string) error {
	for {
		if p.Allow(key) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Continue trying
		}
	}
}

// Cleanup removes stale limiters that haven't been used
func (p *PerKeyLimiter) Cleanup(maxAge time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, limiter := range p.limiters {
		if now.Sub(limiter.lastRefill) > maxAge {
			delete(p.limiters, key)
		}
	}
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// CircuitClosed is the normal operating state
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen is the failure state (requests are rejected)
	CircuitOpen
	// CircuitHalfOpen is the recovery state (limited requests allowed)
	CircuitHalfOpen
)

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures to open the circuit
	SuccessThreshold int           // Number of successes to close the circuit (in half-open state)
	Timeout          time.Duration // Time to wait before transitioning from open to half-open
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitBreakerState
	failures         int
	successes        int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailure      time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: config.FailureThreshold,
		successThreshold: config.SuccessThreshold,
		timeout:          config.Timeout,
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if we should transition from open to half-open
	if cb.state == CircuitOpen && time.Since(cb.lastFailure) >= cb.timeout {
		cb.state = CircuitHalfOpen
		cb.successes = 0
	}

	return cb.state
}

// Allow checks if a request should be allowed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if we should transition from open to half-open
	if cb.state == CircuitOpen && time.Since(cb.lastFailure) >= cb.timeout {
		cb.state = CircuitHalfOpen
		cb.successes = 0
	}

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		return false
	case CircuitHalfOpen:
		return true // Allow limited requests in half-open state
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
		}
	} else if cb.state == CircuitClosed {
		cb.failures = 0 // Reset failures on success
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == CircuitHalfOpen {
		// Any failure in half-open state opens the circuit
		cb.state = CircuitOpen
		cb.failures = 1
	} else if cb.failures >= cb.failureThreshold {
		cb.state = CircuitOpen
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
}
