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
