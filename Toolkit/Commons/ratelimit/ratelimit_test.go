package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestNewTokenBucket(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   10,
		RefillRate: time.Second,
	}

	tb := NewTokenBucket(config)

	if tb.capacity != 10 {
		t.Errorf("Expected capacity 10, got %d", tb.capacity)
	}

	if tb.tokens != 10 {
		t.Errorf("Expected initial tokens 10, got %d", tb.tokens)
	}

	if tb.refillRate != time.Second {
		t.Errorf("Expected refill rate 1s, got %v", tb.refillRate)
	}
}

func TestTokenBucket_Wait_Success(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   5,
		RefillRate: 10 * time.Millisecond, // Fast refill for test
	}

	tb := NewTokenBucket(config)
	ctx := context.Background()

	// Should be able to take all initial tokens
	for i := 0; i < 5; i++ {
		err := tb.Wait(ctx)
		if err != nil {
			t.Fatalf("Expected no error for token %d, got %v", i+1, err)
		}
	}
}

func TestTokenBucket_Wait_RateLimited(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   2,
		RefillRate: 100 * time.Millisecond, // Slow refill
	}

	tb := NewTokenBucket(config)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Take initial tokens
	for i := 0; i < 2; i++ {
		err := tb.Wait(ctx)
		if err != nil {
			t.Fatalf("Expected no error for initial token %d, got %v", i+1, err)
		}
	}

	// Next request should be rate limited and timeout
	err := tb.Wait(ctx)
	if err == nil {
		t.Error("Expected timeout error when rate limited")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestTokenBucket_Wait_Refill(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   3,
		RefillRate: 10 * time.Millisecond,
	}

	tb := NewTokenBucket(config)
	ctx := context.Background()

	// Take all initial tokens
	for i := 0; i < 3; i++ {
		err := tb.Wait(ctx)
		if err != nil {
			t.Fatalf("Expected no error for initial token %d, got %v", i+1, err)
		}
	}

	// Wait for refill
	time.Sleep(15 * time.Millisecond)

	// Should be able to get another token
	err := tb.Wait(ctx)
	if err != nil {
		t.Fatalf("Expected no error after refill, got %v", err)
	}
}

func TestTokenBucket_Wait_ContextCancel(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   1,
		RefillRate: time.Second, // Very slow refill
	}

	tb := NewTokenBucket(config)
	ctx, cancel := context.WithCancel(context.Background())

	// Take the only token
	err := tb.Wait(ctx)
	if err != nil {
		t.Fatalf("Expected no error for initial token, got %v", err)
	}

	// Start waiting for next token in goroutine
	done := make(chan error, 1)
	go func() {
		done <- tb.Wait(ctx)
	}()

	// Cancel context
	cancel()

	// Should get context canceled error
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Expected Canceled, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected immediate return after context cancel")
	}
}

func TestTokenBucket_takeToken_Refill(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   5,
		RefillRate: 10 * time.Millisecond,
	}

	tb := NewTokenBucket(config)

	// Take all tokens
	for i := 0; i < 5; i++ {
		if !tb.takeToken() {
			t.Fatalf("Expected to get token %d", i+1)
		}
	}

	// Should not get more tokens immediately
	if tb.takeToken() {
		t.Error("Expected no token available immediately")
	}

	// Manually set lastRefill to past to simulate refill
	tb.mu.Lock()
	tb.lastRefill = time.Now().Add(-50 * time.Millisecond) // 5 tokens should refill
	tb.mu.Unlock()

	// Should get tokens now
	for i := 0; i < 5; i++ {
		if !tb.takeToken() {
			t.Fatalf("Expected to get refilled token %d", i+1)
		}
	}
}

func TestTokenBucket_takeToken_CapacityLimit(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   3,
		RefillRate: 10 * time.Millisecond,
	}

	tb := NewTokenBucket(config)

	// Take all tokens
	for i := 0; i < 3; i++ {
		if !tb.takeToken() {
			t.Fatalf("Expected to get token %d", i+1)
		}
	}

	// Manually set lastRefill to past with enough time for more than capacity
	tb.mu.Lock()
	tb.lastRefill = time.Now().Add(-100 * time.Millisecond) // 10 tokens would refill
	tb.mu.Unlock()

	// Take one token, which should refill and cap at capacity
	if !tb.takeToken() {
		t.Error("Expected to get token after refill")
	}

	// Check that tokens don't exceed capacity
	tb.mu.Lock()
	if tb.tokens > tb.capacity {
		t.Errorf("Tokens %d exceeded capacity %d", tb.tokens, tb.capacity)
	}
	tb.mu.Unlock()
}

func TestTokenBucket_takeToken_Concurrent(t *testing.T) {
	config := TokenBucketConfig{
		Capacity:   10,
		RefillRate: time.Millisecond, // Fast refill
	}

	tb := NewTokenBucket(config)

	// Test concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				tb.takeToken()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not have negative tokens or exceed capacity
	tb.mu.Lock()
	if tb.tokens < 0 {
		t.Errorf("Tokens went negative: %d", tb.tokens)
	}
	if tb.tokens > tb.capacity {
		t.Errorf("Tokens %d exceeded capacity %d", tb.tokens, tb.capacity)
	}
	tb.mu.Unlock()
}
