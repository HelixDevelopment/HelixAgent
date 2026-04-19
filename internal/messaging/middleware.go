package messaging

import (
	"context"
	"log"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// MessageMiddleware is a function that wraps a MessageHandler.
type MessageMiddleware func(MessageHandler) MessageHandler

// MiddlewareChain represents a chain of middleware.
// Concurrent-safe by construction (CONST-029): middleware is a safe.Slice.
type MiddlewareChain struct {
	middleware *safe.Slice[MessageMiddleware]
}

// NewMiddlewareChain creates a new middleware chain.
func NewMiddlewareChain(middleware ...MessageMiddleware) *MiddlewareChain {
	return &MiddlewareChain{
		middleware: safe.NewSlice(middleware...),
	}
}

// Add adds middleware to the chain.
func (c *MiddlewareChain) Add(middleware ...MessageMiddleware) {
	c.middleware.AppendAll(middleware...)
}

// Prepend adds middleware to the beginning of the chain.
// safe.Slice has no first-class prepend, but Replace with a fresh
// concatenated slice gives the same effect under the store's write lock.
func (c *MiddlewareChain) Prepend(middleware ...MessageMiddleware) {
	current := c.middleware.Snapshot()
	c.middleware.Replace(append(append([]MessageMiddleware(nil), middleware...), current...))
}

// Clear removes all middleware from the chain.
func (c *MiddlewareChain) Clear() {
	c.middleware.Clear()
}

// Wrap wraps a handler with all middleware in the chain.
func (c *MiddlewareChain) Wrap(handler MessageHandler) MessageHandler {
	snapshot := c.middleware.Snapshot()
	// Apply middleware in reverse order so they execute in order
	for i := len(snapshot) - 1; i >= 0; i-- {
		handler = snapshot[i](handler)
	}
	return handler
}

// Logger interface for middleware logging.
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// DefaultLogger is a simple default logger implementation.
type DefaultLogger struct{}

// Debug logs a debug message.
func (l *DefaultLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

// Info logs an info message.
func (l *DefaultLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, fields)
}

// Warn logs a warning message.
func (l *DefaultLogger) Warn(msg string, fields ...interface{}) {
	log.Printf("[WARN] %s %v", msg, fields)
}

// Error logs an error message.
func (l *DefaultLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, fields)
}

// LoggingMiddleware creates a middleware that logs message processing.
func LoggingMiddleware(logger Logger) MessageMiddleware {
	if logger == nil {
		logger = &DefaultLogger{}
	}
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			start := time.Now()
			logger.Debug("Processing message",
				"id", msg.ID,
				"type", msg.Type,
				"size", len(msg.Payload),
			)

			err := next(ctx, msg)

			duration := time.Since(start)
			if err != nil {
				logger.Error("Message processing failed",
					"id", msg.ID,
					"type", msg.Type,
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Debug("Message processed successfully",
					"id", msg.ID,
					"type", msg.Type,
					"duration", duration,
				)
			}

			return err
		}
	}
}

// RetryConfig holds configuration for retry middleware.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// InitialDelay is the initial delay between retries.
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// Multiplier is the delay multiplier for exponential backoff.
	Multiplier float64
	// RetryableErrors is a list of error codes that should be retried.
	RetryableErrors []ErrorCode
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		RetryableErrors: []ErrorCode{
			ErrCodeConnectionFailed,
			ErrCodeConnectionTimeout,
			ErrCodeBrokerUnavailable,
		},
	}
}

// RetryMiddleware creates a middleware that retries failed messages.
func RetryMiddleware(config *RetryConfig) MessageMiddleware {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			var lastErr error
			delay := config.InitialDelay

			for attempt := 0; attempt <= config.MaxRetries; attempt++ {
				if attempt > 0 {
					msg.IncrementRetry()
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(delay):
					}
					// Calculate next delay with exponential backoff
					delay = time.Duration(float64(delay) * config.Multiplier)
					if delay > config.MaxDelay {
						delay = config.MaxDelay
					}
				}

				err := next(ctx, msg)
				if err == nil {
					return nil
				}

				lastErr = err

				// Check if error is retryable
				if !shouldRetry(err, config.RetryableErrors) {
					return err
				}
			}

			return lastErr
		}
	}
}

// shouldRetry checks if an error should be retried.
func shouldRetry(err error, retryableCodes []ErrorCode) bool {
	if IsRetryableError(err) {
		return true
	}
	if brokerErr := GetBrokerError(err); brokerErr != nil {
		for _, code := range retryableCodes {
			if brokerErr.Code == code {
				return true
			}
		}
	}
	return false
}

// TimeoutMiddleware creates a middleware that enforces a timeout.
func TimeoutMiddleware(timeout time.Duration) MessageMiddleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				done <- next(ctx, msg)
			}()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				return NewBrokerError(ErrCodeOperationCanceled, "operation timed out", ctx.Err())
			}
		}
	}
}

// RecoveryMiddleware creates a middleware that recovers from panics.
func RecoveryMiddleware(logger Logger) MessageMiddleware {
	if logger == nil {
		logger = &DefaultLogger{}
	}
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic recovered in message handler",
						"id", msg.ID,
						"type", msg.Type,
						"panic", r,
					)
					err = NewBrokerError(ErrCodeHandlerError, "panic recovered", nil).
						WithMessageID(msg.ID).
						WithDetail("panic", r)
				}
			}()
			return next(ctx, msg)
		}
	}
}

// TracingMiddleware creates a middleware that adds tracing context.
func TracingMiddleware() MessageMiddleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			// Extract or generate trace ID
			traceID := msg.TraceID
			if traceID == "" {
				traceID = generateTraceID()
				msg.TraceID = traceID
			}

			// Add trace ID to context
			ctx = context.WithValue(ctx, traceIDKey, traceID)

			return next(ctx, msg)
		}
	}
}

// contextKey type for context keys.
type contextKey string

const (
	traceIDKey contextKey = "trace_id"
)

// GetTraceID extracts the trace ID from context.
func GetTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

// generateTraceID generates a unique trace ID.
func generateTraceID() string {
	return time.Now().UTC().Format("20060102150405.000000") + "-" + randomString(8)
}

// MetricsMiddleware creates a middleware that records metrics.
func MetricsMiddleware(metrics *BrokerMetrics) MessageMiddleware {
	if metrics == nil {
		metrics = NewBrokerMetrics()
	}
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			start := time.Now()

			err := next(ctx, msg)

			duration := time.Since(start)
			metrics.RecordReceive(int64(len(msg.Payload)), duration)

			if err != nil {
				metrics.RecordFailed()
			} else {
				metrics.RecordProcessed()
			}

			return err
		}
	}
}

// ValidationMiddleware creates a middleware that validates messages.
func ValidationMiddleware(validators ...MessageValidator) MessageMiddleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			for _, validator := range validators {
				if err := validator(msg); err != nil {
					return NewBrokerError(ErrCodeMessageInvalid, "message validation failed", err).
						WithMessageID(msg.ID)
				}
			}
			return next(ctx, msg)
		}
	}
}

// MessageValidator is a function that validates a message.
type MessageValidator func(msg *Message) error

// RequiredFieldsValidator validates that required fields are present.
func RequiredFieldsValidator(msg *Message) error {
	if msg.ID == "" {
		return NewBrokerError(ErrCodeMessageInvalid, "message ID is required", nil)
	}
	if msg.Type == "" {
		return NewBrokerError(ErrCodeMessageInvalid, "message type is required", nil)
	}
	if len(msg.Payload) == 0 {
		return NewBrokerError(ErrCodeMessageInvalid, "message payload is required", nil)
	}
	return nil
}

// MaxSizeValidator creates a validator that checks message size.
func MaxSizeValidator(maxSize int) MessageValidator {
	return func(msg *Message) error {
		if len(msg.Payload) > maxSize {
			return NewBrokerError(ErrCodeMessageTooLarge, "message exceeds maximum size", nil).
				WithDetail("size", len(msg.Payload)).
				WithDetail("max_size", maxSize)
		}
		return nil
	}
}

// ExpirationValidator validates that message has not expired.
func ExpirationValidator(msg *Message) error {
	if msg.IsExpired() {
		return NewBrokerError(ErrCodeMessageExpired, "message has expired", nil).
			WithMessageID(msg.ID)
	}
	return nil
}

// DeduplicationMiddleware creates a middleware that deduplicates messages.
func DeduplicationMiddleware(store DeduplicationStore) MessageMiddleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			// Check if message was already processed
			if store.Exists(msg.ID) {
				// Message already processed, skip
				return nil
			}

			err := next(ctx, msg)
			if err == nil {
				// Mark message as processed
				store.Add(msg.ID)
			}

			return err
		}
	}
}

// DeduplicationStore is an interface for storing processed message IDs.
type DeduplicationStore interface {
	Add(id string)
	Exists(id string) bool
	Remove(id string)
	Clear()
}

// InMemoryDeduplicationStore is an in-memory implementation of DeduplicationStore.
// Concurrent-safe by construction (CONST-029): ids is a safe.Store.
type InMemoryDeduplicationStore struct {
	ids    *safe.Store[string, time.Time]
	ttl    time.Duration
	stopCh chan struct{}
}

// NewInMemoryDeduplicationStore creates a new in-memory deduplication store.
func NewInMemoryDeduplicationStore(ttl time.Duration) *InMemoryDeduplicationStore {
	store := &InMemoryDeduplicationStore{
		ids:    safe.NewStore[string, time.Time](),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go store.cleanup()
	return store
}

// Add adds a message ID to the store.
func (s *InMemoryDeduplicationStore) Add(id string) {
	s.ids.Put(id, time.Now().Add(s.ttl))
}

// Exists checks if a message ID exists in the store.
func (s *InMemoryDeduplicationStore) Exists(id string) bool {
	expiry, ok := s.ids.Get(id)
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

// Remove removes a message ID from the store.
func (s *InMemoryDeduplicationStore) Remove(id string) {
	s.ids.Delete(id)
}

// Clear removes all message IDs from the store.
func (s *InMemoryDeduplicationStore) Clear() {
	s.ids.Clear()
}

// Stop stops the cleanup goroutine.
func (s *InMemoryDeduplicationStore) Stop() {
	close(s.stopCh)
}

// cleanup removes expired entries periodically.
// Iterates a Snapshot so the per-key Delete doesn't trip the
// "mutate during Range" deadlock-avoidance comment in safe.Store.Range.
func (s *InMemoryDeduplicationStore) cleanup() {
	ticker := time.NewTicker(s.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			for id, expiry := range s.ids.Snapshot() {
				if now.After(expiry) {
					s.ids.Delete(id)
				}
			}
		case <-s.stopCh:
			return
		}
	}
}

// RateLimitMiddleware creates a middleware that rate limits message processing.
func RateLimitMiddleware(limiter RateLimiter) MessageMiddleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			if !limiter.Allow() {
				return NewBrokerError(ErrCodeOperationCanceled, "rate limit exceeded", nil).
					WithMessageID(msg.ID)
			}
			return next(ctx, msg)
		}
	}
}

// RateLimiter is an interface for rate limiting.
type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
}

// TokenBucketLimiter implements a token bucket rate limiter.
type TokenBucketLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketLimiter creates a new token bucket rate limiter.
func NewTokenBucketLimiter(maxTokens float64, refillRate float64) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed.
func (l *TokenBucketLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// Wait waits until a request is allowed.
func (l *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		if l.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(1/l.refillRate) * time.Second):
		}
	}
}

// refill refills tokens based on elapsed time.
func (l *TokenBucketLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
	l.lastRefill = now
}

// CircuitBreakerMiddleware creates a middleware that implements circuit breaker pattern.
func CircuitBreakerMiddleware(cb *CircuitBreaker) MessageMiddleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			if !cb.Allow() {
				return NewBrokerError(ErrCodeBrokerUnavailable, "circuit breaker open", nil).
					WithMessageID(msg.ID)
			}

			err := next(ctx, msg)
			if err != nil {
				cb.RecordFailure()
			} else {
				cb.RecordSuccess()
			}

			return err
		}
	}
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	state            CircuitBreakerState
	failures         int
	successes        int
	threshold        int
	resetTimeout     time.Duration
	halfOpenRequests int
	lastFailure      time.Time
	mu               sync.Mutex
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        CircuitClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Allow checks if a request is allowed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.halfOpenRequests = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		cb.halfOpenRequests++
		return cb.halfOpenRequests <= 3 // Allow limited requests in half-open state
	}
	return false
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= 3 {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
		}
	} else {
		cb.failures = 0
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = CircuitOpen
	}

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
