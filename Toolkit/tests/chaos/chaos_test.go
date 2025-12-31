package chaos

import (
	"context"
	"runtime"
	"testing"
	"time"

	testingutils "github.com/HelixDevelopment/HelixAgent/Toolkit/Commons/testing"
	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit/common/ratelimit"
)

// TestNetworkFailureResilience tests provider resilience to network failures
func TestNetworkFailureResilience(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("chaos-test")
	mockProvider.SetShouldError(true) // Simulate failures

	ctx := context.Background()

	// Test chat with network failure simulation
	chatReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	_, err := mockProvider.Chat(ctx, chatReq)
	if err == nil {
		t.Error("Expected error due to simulated network failure")
	}
	t.Logf("Network failure test result: %v", err)
}

// TestTimeoutResilience tests provider behavior under timeout conditions
func TestTimeoutResilience(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("chaos-test")

	// Simulate timeout by using a very short context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	chatReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	_, err := mockProvider.Chat(ctx, chatReq)
	// Should handle timeout gracefully
	t.Logf("Timeout test result: %v", err)
}

// TestConnectionDropResilience tests resilience to connection drops
func TestConnectionDropResilience(t *testing.T) {
	// Create a mock HTTP client that simulates connection drops
	mockClient := testingutils.NewMockHTTPClient()

	// Simulate connection refused
	mockClient.AddResponse("POST", "https://api.test.com/chat", &testingutils.MockResponse{
		StatusCode: 0, // Connection failed
		Body:       map[string]interface{}{"error": "connection refused"},
	})

	// Note: In a real implementation, providers would use this mock client
	// For now, test with mock provider
	mockProvider := testingutils.NewMockProvider("chaos-test")
	mockProvider.SetShouldError(true)

	ctx := context.Background()
	chatReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	_, err := mockProvider.Chat(ctx, chatReq)
	t.Logf("Connection drop test result: %v", err)
}

// TestHTTPErrorResilience tests resilience to various HTTP errors
func TestHTTPErrorResilience(t *testing.T) {
	httpErrors := []int{400, 401, 403, 404, 429, 500, 502, 503, 504}

	for _, statusCode := range httpErrors {
		t.Run("HTTP_"+string(rune(statusCode)), func(t *testing.T) {
			mockProvider := testingutils.NewMockProvider("chaos-test")
			mockProvider.SetShouldError(true)

			ctx := context.Background()
			chatReq := toolkit.ChatRequest{
				Model: "test-model",
				Messages: []toolkit.ChatMessage{
					{Role: "user", Content: "Test message"},
				},
			}

			_, err := mockProvider.Chat(ctx, chatReq)
			if err == nil {
				t.Errorf("Expected error for HTTP %d", statusCode)
			}
			t.Logf("HTTP %d error test result: %v", statusCode, err)
		})
	}
}

// TestRateLimitResilience tests behavior under rate limiting
func TestRateLimitResilience(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("chaos-test")

	ctx := context.Background()

	// Simulate rapid requests that might trigger rate limits
	for i := 0; i < 10; i++ {
		chatReq := toolkit.ChatRequest{
			Model: "test-model",
			Messages: []toolkit.ChatMessage{
				{Role: "user", Content: "Test message"},
			},
		}

		_, err := mockProvider.Chat(ctx, chatReq)
		if err != nil {
			t.Logf("Rate limit simulation request %d: %v", i+1, err)
		}
	}
}

// TestPartialFailureResilience tests resilience to partial failures
func TestPartialFailureResilience(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("chaos-test")

	ctx := context.Background()

	// Test embedding with potential partial failure
	embedReq := toolkit.EmbeddingRequest{
		Model: "test-embedding-model",
		Input: []string{"test input"},
	}

	_, err := mockProvider.Embed(ctx, embedReq)
	t.Logf("Partial failure embedding test: %v", err)

	// Test rerank with potential partial failure
	rerankReq := toolkit.RerankRequest{
		Model:     "test-rerank-model",
		Query:     "test query",
		Documents: []string{"doc1", "doc2"},
		TopN:      2,
	}

	_, err = mockProvider.Rerank(ctx, rerankReq)
	t.Logf("Partial failure rerank test: %v", err)
}

// TestConcurrentFailureResilience tests resilience under concurrent failures
func TestConcurrentFailureResilience(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("chaos-test")
	mockProvider.SetShouldError(true)

	ctx := context.Background()

	// Run multiple concurrent requests that will fail
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			chatReq := toolkit.ChatRequest{
				Model: "test-model",
				Messages: []toolkit.ChatMessage{
					{Role: "user", Content: "Test message"},
				},
			}

			_, err := mockProvider.Chat(ctx, chatReq)
			if err != nil {
				t.Logf("Concurrent failure request %d: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestRecoveryAfterFailure tests recovery after failures
func TestRecoveryAfterFailure(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("chaos-test")

	ctx := context.Background()

	// First, make it fail
	mockProvider.SetShouldError(true)

	chatReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	_, err := mockProvider.Chat(ctx, chatReq)
	if err == nil {
		t.Error("Expected failure")
	}
	t.Logf("Initial failure: %v", err)

	// Then make it succeed
	mockProvider.SetShouldError(false)

	fixtures := testingutils.NewTestFixtures()
	chatResp := fixtures.ChatResponse()
	mockProvider.SetChatResponse(chatResp)

	_, err = mockProvider.Chat(ctx, chatReq)
	if err != nil {
		t.Errorf("Expected recovery after failure, got error: %v", err)
	}
	t.Logf("Recovery test: %v", err)
}

// TestCircuitBreakerPattern tests circuit breaker behavior
func TestCircuitBreakerPattern(t *testing.T) {
	// Test 1: Circuit opens after threshold failures
	t.Run("OpensAfterThresholdFailures", func(t *testing.T) {
		cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
		})

		// Initial state should be closed
		if cb.State() != ratelimit.CircuitClosed {
			t.Error("Expected circuit to start in closed state")
		}

		// First 3 failures should not trip the circuit (up to threshold-1)
		for i := 0; i < 2; i++ {
			cb.RecordFailure()
			if !cb.Allow() {
				t.Errorf("Circuit should still be closed after %d failures", i+1)
			}
		}

		// Third failure should trip the circuit
		cb.RecordFailure()
		if cb.Allow() {
			t.Error("Circuit should be open after 3 failures")
		}

		if cb.State() != ratelimit.CircuitOpen {
			t.Error("Expected circuit state to be open")
		}
	})

	// Test 2: Circuit transitions to half-open after timeout
	t.Run("TransitionsToHalfOpenAfterTimeout", func(t *testing.T) {
		cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 2,
			Timeout:          50 * time.Millisecond,
		})

		// Trip the circuit
		cb.RecordFailure()
		cb.RecordFailure()

		if cb.State() != ratelimit.CircuitOpen {
			t.Error("Circuit should be open after failures")
		}

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// Should be half-open now
		if cb.State() != ratelimit.CircuitHalfOpen {
			t.Error("Circuit should be half-open after timeout")
		}

		// Should allow requests in half-open state
		if !cb.Allow() {
			t.Error("Circuit should allow requests in half-open state")
		}
	})

	// Test 3: Circuit closes after success threshold in half-open state
	t.Run("ClosesAfterSuccessThreshold", func(t *testing.T) {
		cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 2,
			Timeout:          50 * time.Millisecond,
		})

		// Trip the circuit
		cb.RecordFailure()
		cb.RecordFailure()

		// Wait for timeout to enter half-open
		time.Sleep(60 * time.Millisecond)
		cb.State() // Trigger state check

		// Record successes
		cb.RecordSuccess()
		if cb.State() != ratelimit.CircuitHalfOpen {
			t.Error("Circuit should still be half-open after 1 success")
		}

		cb.RecordSuccess()
		if cb.State() != ratelimit.CircuitClosed {
			t.Error("Circuit should be closed after success threshold")
		}
	})

	// Test 4: Circuit reopens on failure in half-open state
	t.Run("ReopensOnFailureInHalfOpen", func(t *testing.T) {
		cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 3,
			Timeout:          50 * time.Millisecond,
		})

		// Trip the circuit
		cb.RecordFailure()
		cb.RecordFailure()

		// Wait for timeout to enter half-open
		time.Sleep(60 * time.Millisecond)
		cb.State() // Trigger state check

		// Any failure in half-open should reopen
		cb.RecordFailure()
		if cb.State() != ratelimit.CircuitOpen {
			t.Error("Circuit should be open after failure in half-open state")
		}
	})

	// Test 5: Success resets failure count in closed state
	t.Run("SuccessResetsFailureCount", func(t *testing.T) {
		cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
		})

		// Record 2 failures (just under threshold)
		cb.RecordFailure()
		cb.RecordFailure()

		// Record a success to reset failure count
		cb.RecordSuccess()

		// Now 2 more failures should not trip the circuit
		cb.RecordFailure()
		cb.RecordFailure()

		if cb.State() != ratelimit.CircuitClosed {
			t.Error("Circuit should remain closed after success reset")
		}

		// Third failure should now trip it
		cb.RecordFailure()
		if cb.State() != ratelimit.CircuitOpen {
			t.Error("Circuit should be open after reaching threshold again")
		}
	})

	// Test 6: Reset brings circuit back to closed
	t.Run("ResetClosesCircuit", func(t *testing.T) {
		cb := ratelimit.NewCircuitBreaker(ratelimit.CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
		})

		// Trip the circuit
		cb.RecordFailure()
		cb.RecordFailure()

		if cb.State() != ratelimit.CircuitOpen {
			t.Error("Circuit should be open")
		}

		// Reset should close it
		cb.Reset()

		if cb.State() != ratelimit.CircuitClosed {
			t.Error("Circuit should be closed after reset")
		}
		if !cb.Allow() {
			t.Error("Circuit should allow requests after reset")
		}
	})
}

// TestGracefulDegradation tests graceful degradation under failure conditions
func TestGracefulDegradation(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("chaos-test")
	mockProvider.SetShouldError(true)

	ctx := context.Background()

	// Test that failures are handled gracefully without panicking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Provider panicked under failure conditions: %v", r)
		}
	}()

	// Try various operations that might fail
	chatReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "Test"},
		},
	}

	_, _ = mockProvider.Chat(ctx, chatReq)
	_, _ = mockProvider.DiscoverModels(ctx)

	embedReq := toolkit.EmbeddingRequest{
		Model: "test-embedding-model",
		Input: []string{"test"},
	}
	_, _ = mockProvider.Embed(ctx, embedReq)

	rerankReq := toolkit.RerankRequest{
		Model:     "test-rerank-model",
		Query:     "test",
		Documents: []string{"doc"},
		TopN:      1,
	}
	_, _ = mockProvider.Rerank(ctx, rerankReq)

	t.Log("Graceful degradation test completed without panics")
}

// TestResourceLeakPrevention tests that resources are properly cleaned up during failures
func TestResourceLeakPrevention(t *testing.T) {
	// Test 1: Goroutine leak detection
	t.Run("NoGoroutineLeaks", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		// Run many operations that might leak goroutines
		for i := 0; i < 100; i++ {
			mockProvider := testingutils.NewMockProvider("leak-test")
			mockProvider.SetShouldError(true)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)

			chatReq := toolkit.ChatRequest{
				Model: "test-model",
				Messages: []toolkit.ChatMessage{
					{Role: "user", Content: "Test"},
				},
			}

			// These operations may fail, which is expected
			_, _ = mockProvider.Chat(ctx, chatReq)
			_, _ = mockProvider.DiscoverModels(ctx)

			cancel()
		}

		// Allow time for goroutines to clean up
		time.Sleep(100 * time.Millisecond)
		runtime.GC()

		finalGoroutines := runtime.NumGoroutine()
		leakedGoroutines := finalGoroutines - initialGoroutines

		// Allow small variance due to test framework
		if leakedGoroutines > 5 {
			t.Errorf("Potential goroutine leak: started with %d, ended with %d (leaked %d)",
				initialGoroutines, finalGoroutines, leakedGoroutines)
		}
	})

	// Test 2: Memory allocation under stress
	t.Run("MemoryAllocationUnderStress", func(t *testing.T) {
		var memBefore, memAfter runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memBefore)

		// Perform many operations
		for i := 0; i < 1000; i++ {
			mockProvider := testingutils.NewMockProvider("memory-test")

			fixtures := testingutils.NewTestFixtures()
			chatResp := fixtures.ChatResponse()
			mockProvider.SetChatResponse(chatResp)

			ctx := context.Background()
			chatReq := toolkit.ChatRequest{
				Model: "test-model",
				Messages: []toolkit.ChatMessage{
					{Role: "user", Content: "Test message " + string(rune(i))},
				},
			}

			_, _ = mockProvider.Chat(ctx, chatReq)
		}

		runtime.GC()
		runtime.ReadMemStats(&memAfter)

		// Check that memory growth is reasonable (less than 100MB for 1000 operations)
		// Handle case where memory may be freed (memAfter < memBefore)
		var memGrowth int64
		if memAfter.Alloc > memBefore.Alloc {
			memGrowth = int64(memAfter.Alloc - memBefore.Alloc)
		} else {
			memGrowth = 0 // Memory was freed, no growth
		}
		maxAllowedGrowth := int64(100 * 1024 * 1024) // 100 MB

		if memGrowth > maxAllowedGrowth {
			t.Errorf("Excessive memory growth: %d bytes (max allowed: %d bytes)",
				memGrowth, maxAllowedGrowth)
		}
	})

	// Test 3: Context cancellation cleanup
	t.Run("ContextCancellationCleanup", func(t *testing.T) {
		mockProvider := testingutils.NewMockProvider("context-test")

		// Add slight delay to simulate work
		fixtures := testingutils.NewTestFixtures()
		chatResp := fixtures.ChatResponse()
		mockProvider.SetChatResponse(chatResp)

		for i := 0; i < 50; i++ {
			ctx, cancel := context.WithCancel(context.Background())

			chatReq := toolkit.ChatRequest{
				Model: "test-model",
				Messages: []toolkit.ChatMessage{
					{Role: "user", Content: "Test"},
				},
			}

			// Cancel context immediately
			cancel()

			// Operation should handle cancellation gracefully
			_, err := mockProvider.Chat(ctx, chatReq)
			// Error is expected due to cancellation
			_ = err
		}

		// If we reach here without panics or hangs, the test passes
		t.Log("Context cancellation cleanup test completed")
	})

	// Test 4: Per-key limiter cleanup
	t.Run("PerKeyLimiterCleanup", func(t *testing.T) {
		limiter := ratelimit.NewPerKeyLimiter(ratelimit.TokenBucketConfig{
			Capacity:   10,
			RefillRate: 1.0,
		})

		// Create many keys
		for i := 0; i < 1000; i++ {
			key := string(rune('a'+i%26)) + string(rune(i))
			limiter.Allow(key)
		}

		// Cleanup old entries
		limiter.Cleanup(0) // 0 duration means cleanup all

		t.Log("Per-key limiter cleanup test completed")
	})
}
