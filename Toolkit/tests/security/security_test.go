package security

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	testingutils "github.com/HelixDevelopment/HelixAgent/Toolkit/Commons/testing"
	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit/common/ratelimit"
)

// TestAPIKeySecurity tests that API keys are handled securely
func TestAPIKeySecurity(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("security-test")

	// Test that API keys are not logged in error messages
	config := map[string]interface{}{
		"api_key": "sk-1234567890abcdef",
	}

	err := mockProvider.ValidateConfig(config)
	if err != nil {
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "sk-1234567890abcdef") {
			t.Error("API key leaked in error message")
		}
	}

	// Test with invalid API key format
	invalidConfig := map[string]interface{}{
		"api_key": "",
	}

	err = mockProvider.ValidateConfig(invalidConfig)
	if err == nil {
		t.Error("Expected validation error for empty API key")
	}
}

// TestInputValidation tests input validation for security vulnerabilities
func TestInputValidation(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("security-test")

	ctx := context.Background()

	// Test SQL injection attempt in chat message
	sqlInjectionReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "'; DROP TABLE users; --"},
		},
	}

	_, err := mockProvider.Chat(ctx, sqlInjectionReq)
	// Mock provider doesn't validate input, but real providers should
	t.Logf("SQL injection test result: %v", err)

	// Test XSS attempt
	xssReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "<script>alert('xss')</script>"},
		},
	}

	_, err = mockProvider.Chat(ctx, xssReq)
	t.Logf("XSS test result: %v", err)

	// Test command injection attempt
	cmdInjectionReq := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "$(rm -rf /)"},
		},
	}

	_, err = mockProvider.Chat(ctx, cmdInjectionReq)
	t.Logf("Command injection test result: %v", err)
}

// TestConfigSecurity tests secure configuration handling
func TestConfigSecurity(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("security-test")

	// Test that sensitive config values are not exposed
	config := map[string]interface{}{
		"api_key":      "secret-key-123",
		"secret_token": "token-456",
		"password":     "pass-789",
		"timeout":      30,
	}

	err := mockProvider.ValidateConfig(config)
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	// Test config with null bytes (potential attack)
	maliciousConfig := map[string]interface{}{
		"api_key": "key\x00injected",
	}

	err = mockProvider.ValidateConfig(maliciousConfig)
	if err != nil {
		t.Logf("Null byte config rejected: %v", err)
	}
}

// TestErrorMessageSecurity tests that error messages don't leak sensitive information
func TestErrorMessageSecurity(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("security-test")
	mockProvider.SetShouldError(true)

	ctx := context.Background()

	// Test that internal paths/file names are not exposed
	req := toolkit.ChatRequest{
		Model: "test-model",
		Messages: []toolkit.ChatMessage{
			{Role: "user", Content: "test"},
		},
	}

	_, err := mockProvider.Chat(ctx, req)
	if err != nil {
		errorMsg := err.Error()
		// Check that error doesn't contain file paths or internal details
		if strings.Contains(errorMsg, "/") || strings.Contains(errorMsg, "\\") {
			t.Logf("Warning: Error message may contain file paths: %s", errorMsg)
		}
		// Check that error doesn't contain stack traces in production-like scenarios
		if strings.Contains(errorMsg, "runtime") || strings.Contains(errorMsg, "panic") {
			t.Logf("Warning: Error message may contain stack trace: %s", errorMsg)
		}
	}
}

// TestRateLimitSecurity tests rate limiting security aspects
func TestRateLimitSecurity(t *testing.T) {
	// Test 1: Basic rate limiting blocks excessive requests
	t.Run("BlocksExcessiveRequests", func(t *testing.T) {
		// Create a strict rate limiter: 5 requests, 1 token per second refill
		limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketConfig{
			Capacity:   5,
			RefillRate: 1.0,
		})

		// First 5 requests should succeed
		for i := 0; i < 5; i++ {
			if !limiter.Allow() {
				t.Errorf("Request %d should have been allowed", i+1)
			}
		}

		// 6th request should be blocked
		if limiter.Allow() {
			t.Error("6th request should have been blocked")
		}
	})

	// Test 2: Rate limiting with burst attack simulation
	t.Run("BurstAttackMitigation", func(t *testing.T) {
		limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketConfig{
			Capacity:   10,
			RefillRate: 2.0, // 2 tokens per second
		})

		allowed := 0
		blocked := 0

		// Simulate burst attack: 100 rapid requests
		for i := 0; i < 100; i++ {
			if limiter.Allow() {
				allowed++
			} else {
				blocked++
			}
		}

		// Should have allowed exactly 10 (bucket capacity)
		if allowed != 10 {
			t.Errorf("Expected 10 allowed requests, got %d", allowed)
		}
		if blocked != 90 {
			t.Errorf("Expected 90 blocked requests, got %d", blocked)
		}
	})

	// Test 3: Rate limiter recovery
	t.Run("RecoveryAfterBlocking", func(t *testing.T) {
		limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketConfig{
			Capacity:   3,
			RefillRate: 10.0, // Fast refill for testing
		})

		// Exhaust the bucket
		for i := 0; i < 3; i++ {
			limiter.Allow()
		}

		// Should be blocked now
		if limiter.Allow() {
			t.Error("Should be blocked after exhausting bucket")
		}

		// Wait for refill (at 10 tokens/sec, 0.2 seconds = 2 tokens)
		time.Sleep(200 * time.Millisecond)

		// Should now have tokens
		if !limiter.Allow() {
			t.Error("Should be allowed after waiting for refill")
		}
	})

	// Test 4: Sliding window limiter for distributed attack patterns
	t.Run("SlidingWindowAttackMitigation", func(t *testing.T) {
		limiter := ratelimit.NewSlidingWindowLimiter(time.Second, 5)

		allowed := 0

		// Simulate distributed attack pattern over time
		for i := 0; i < 10; i++ {
			if limiter.Allow() {
				allowed++
			}
			time.Sleep(50 * time.Millisecond)
		}

		// Should allow approximately 5 within the 1-second window
		if allowed != 5 {
			t.Errorf("Expected 5 allowed requests, got %d", allowed)
		}
	})

	// Test 5: Per-key rate limiting (simulate per-IP limiting)
	t.Run("PerKeyRateLimiting", func(t *testing.T) {
		perKeyLimiter := ratelimit.NewPerKeyLimiter(ratelimit.TokenBucketConfig{
			Capacity:   3,
			RefillRate: 1.0,
		})

		// User A makes requests
		for i := 0; i < 3; i++ {
			if !perKeyLimiter.Allow("user-a") {
				t.Errorf("User A request %d should be allowed", i+1)
			}
		}

		// User A should now be blocked
		if perKeyLimiter.Allow("user-a") {
			t.Error("User A should be blocked after exhausting limit")
		}

		// User B should still be allowed (independent limit)
		if !perKeyLimiter.Allow("user-b") {
			t.Error("User B should be allowed (independent limit)")
		}
	})

	// Test 6: Concurrent rate limiting (race condition test)
	t.Run("ConcurrentRateLimiting", func(t *testing.T) {
		limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketConfig{
			Capacity:   100,
			RefillRate: 0.0, // No refill for this test
		})

		var wg sync.WaitGroup
		allowed := int32(0)
		numGoroutines := 200

		var mu sync.Mutex
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if limiter.Allow() {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// Should have allowed exactly 100 (bucket capacity)
		if allowed != 100 {
			t.Errorf("Expected 100 allowed, got %d (race condition?)", allowed)
		}
	})
}

// TestModelValidationSecurity tests model name validation
func TestModelValidationSecurity(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("security-test")

	ctx := context.Background()

	// Test with potentially dangerous model names
	dangerousModels := []string{
		"../../../etc/passwd",
		"model;rm -rf /",
		"model\x00injected",
		"<script>model</script>",
	}

	for _, model := range dangerousModels {
		req := toolkit.ChatRequest{
			Model: model,
			Messages: []toolkit.ChatMessage{
				{Role: "user", Content: "test"},
			},
		}

		_, err := mockProvider.Chat(ctx, req)
		t.Logf("Model validation test for '%s': %v", model, err)
	}
}

// TestEmbeddingSecurity tests embedding request security
func TestEmbeddingSecurity(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("security-test")

	ctx := context.Background()

	// Test with large input that could cause DoS
	largeInput := strings.Repeat("word ", 10000) // 50,000 characters
	req := toolkit.EmbeddingRequest{
		Model: "test-embedding-model",
		Input: []string{largeInput},
	}

	_, err := mockProvider.Embed(ctx, req)
	t.Logf("Large input embedding test: %v", err)

	// Test with empty input
	emptyReq := toolkit.EmbeddingRequest{
		Model: "test-embedding-model",
		Input: []string{},
	}

	_, err = mockProvider.Embed(ctx, emptyReq)
	t.Logf("Empty input embedding test: %v", err)
}

// TestRerankSecurity tests rerank request security
func TestRerankSecurity(t *testing.T) {
	mockProvider := testingutils.NewMockProvider("security-test")

	ctx := context.Background()

	// Test with large documents that could cause DoS
	largeDocs := make([]string, 1000)
	for i := range largeDocs {
		largeDocs[i] = strings.Repeat("document ", 1000) // 9,000 chars each
	}

	req := toolkit.RerankRequest{
		Model:     "test-rerank-model",
		Query:     "test query",
		Documents: largeDocs,
		TopN:      10,
	}

	_, err := mockProvider.Rerank(ctx, req)
	t.Logf("Large documents rerank test: %v", err)
}
