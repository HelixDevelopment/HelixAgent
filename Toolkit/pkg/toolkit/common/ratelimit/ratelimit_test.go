package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewTokenBucket(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   10.0,
		RefillRate: 1.0,
	}

	tb := NewTokenBucket(config)

	if tb == nil {
		t.Fatal("NewTokenBucket returned nil")
	}
	if tb.capacity != 10.0 {
		t.Errorf("Expected capacity 10.0, got %f", tb.capacity)
	}
	if tb.refillRate != 1.0 {
		t.Errorf("Expected refill rate 1.0, got %f", tb.refillRate)
	}
	if tb.tokens != 10.0 {
		t.Errorf("Expected initial tokens 10.0, got %f", tb.tokens)
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   2.0,
		RefillRate: 1.0,
	}

	tb := NewTokenBucket(config)

	// Should allow first two requests
	if !tb.Allow() {
		t.Error("First request should be allowed")
	}
	if !tb.Allow() {
		t.Error("Second request should be allowed")
	}

	// Third should be denied
	if tb.Allow() {
		t.Error("Third request should be denied")
	}

	// Wait for refill
	time.Sleep(1100 * time.Millisecond) // Wait for 1 token to refill

	// Should allow again
	if !tb.Allow() {
		t.Error("Request after refill should be allowed")
	}
}

func TestTokenBucket_Wait(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   1.0,
		RefillRate: 2.0,
	}

	tb := NewTokenBucket(config)

	// Use up the token
	if !tb.Allow() {
		t.Error("First request should be allowed")
	}

	// Start wait in goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := tb.Wait(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Should have waited at least 500ms (0.5 seconds for 1 token at 2 tokens/sec)
	if duration < 500*time.Millisecond {
		t.Errorf("Waited too short: %v", duration)
	}
}

func TestTokenBucket_Wait_Timeout(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   0.0, // No initial tokens
		RefillRate: 0.1, // Very slow refill
	}

	tb := NewTokenBucket(config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := tb.Wait(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestNewLimiter(t *testing.T) {
	limiter := NewLimiter(5.0, 1.0)

	if limiter == nil {
		t.Fatal("NewLimiter returned nil")
	}

	// Since Limiter is an alias for TokenBucket, we can access fields directly
	if limiter.capacity != 5.0 {
		t.Errorf("Expected capacity 5.0, got %f", limiter.capacity)
	}
	if limiter.refillRate != 1.0 {
		t.Errorf("Expected refill rate 1.0, got %f", limiter.refillRate)
	}
}

func TestNewSlidingWindowLimiter(t *testing.T) {
	window := 1 * time.Minute
	maxRequests := 10

	sw := NewSlidingWindowLimiter(window, maxRequests)

	if sw == nil {
		t.Fatal("NewSlidingWindowLimiter returned nil")
	}
	if sw.window != window {
		t.Errorf("Expected window %v, got %v", window, sw.window)
	}
	if sw.maxRequests != maxRequests {
		t.Errorf("Expected maxRequests %d, got %d", maxRequests, sw.maxRequests)
	}
	if len(sw.requests) != 0 {
		t.Errorf("Expected empty requests slice, got length %d", len(sw.requests))
	}
}

func TestSlidingWindowLimiter_Allow(t *testing.T) {
	window := 1 * time.Second
	maxRequests := 2

	sw := NewSlidingWindowLimiter(window, maxRequests)

	// Should allow first two requests
	if !sw.Allow() {
		t.Error("First request should be allowed")
	}
	if !sw.Allow() {
		t.Error("Second request should be allowed")
	}

	// Third should be denied
	if sw.Allow() {
		t.Error("Third request should be denied")
	}

	// Wait for window to slide
	time.Sleep(1100 * time.Millisecond)

	// Should allow again
	if !sw.Allow() {
		t.Error("Request after window slide should be allowed")
	}
}

func TestSlidingWindowLimiter_Wait(t *testing.T) {
	window := 500 * time.Millisecond
	maxRequests := 1

	sw := NewSlidingWindowLimiter(window, maxRequests)

	// Use up the request
	if !sw.Allow() {
		t.Error("First request should be allowed")
	}

	// Start wait
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := sw.Wait(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Should have waited at least 500ms
	if duration < 450*time.Millisecond {
		t.Errorf("Waited too short: %v", duration)
	}
}

func TestSlidingWindowLimiter_Wait_Timeout(t *testing.T) {
	window := 2 * time.Second // Long window
	maxRequests := 1

	sw := NewSlidingWindowLimiter(window, maxRequests)

	// Use up the request
	if !sw.Allow() {
		t.Error("First request should be allowed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := sw.Wait(ctx)
	if err == nil {
		t.Error("Expected timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestNewMiddleware(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{Capacity: 1.0, RefillRate: 1.0})

	middleware := NewMiddleware(tb)

	if middleware == nil {
		t.Fatal("NewMiddleware returned nil")
	}
	if middleware.limiter != tb {
		t.Error("Limiter not set correctly")
	}
}

func TestMiddleware_Handler(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{Capacity: 1.0, RefillRate: 1.0})

	middleware := NewMiddleware(tb)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First request should succeed
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Second request should be rate limited
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w2.Code)
	}
}

func TestMiddleware_WaitHandler(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{Capacity: 1.0, RefillRate: 2.0})

	middleware := NewMiddleware(tb)

	handler := middleware.WaitHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First request should succeed
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Second request should wait and then succeed
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 after wait, got %d", w2.Code)
	}
}

func TestMiddleware_WaitHandler_Timeout(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{Capacity: 0.0, RefillRate: 0.0}) // No refill

	middleware := NewMiddleware(tb)

	handler := middleware.WaitHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status 408, got %d", w.Code)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tb := NewTokenBucket(TokenBucketConfig{Capacity: 10.0, RefillRate: 10.0})

	var wg sync.WaitGroup
	requests := 20
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tb.Allow() {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Should allow exactly 10 requests
	if successCount != 10 {
		t.Errorf("Expected 10 successful requests, got %d", successCount)
	}
}

func TestSlidingWindowConcurrentAccess(t *testing.T) {
	sw := NewSlidingWindowLimiter(1*time.Second, 5)

	var wg sync.WaitGroup
	requests := 10
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sw.Allow() {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Should allow exactly 5 requests
	if successCount != 5 {
		t.Errorf("Expected 5 successful requests, got %d", successCount)
	}
}
