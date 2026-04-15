package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Verification Tests for IsSuspiciouslyFastResponse
// =============================================================================
//
// BUGFIX #31: Integration test mock providers had 10ms latency with short
// response strings (< 100 chars). IsSuspiciouslyFastResponse checks:
//   responseTime < 100ms && contentLength < 100
// This caused ALL integration debate tests to fail because mock responses
// triggered the "suspiciously fast response" validation designed to detect
// real-world cached error responses from LLMs.
//
// ROOT CAUSE: integrationMockProvider.latency was 10ms (default), and response
// strings were 8-56 chars. Both thresholds were violated simultaneously.
//
// FIX: Increased integrationMockProvider.latency to 150ms to simulate realistic
// LLM response times, ensuring integration tests never trigger this check.
//
// These verification tests ensure the threshold logic never regresses.

func TestIsSuspiciouslyFastResponse_ThresholdBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		responseTime  time.Duration
		contentLength int
		expected      bool
		reason        string
	}{
		{
			name:          "fast_and_short_triggers",
			responseTime:  50 * time.Millisecond,
			contentLength: 50,
			expected:      true,
			reason:        "<100ms AND <100 chars = suspicious",
		},
		{
			name:          "fast_but_long_content_passes",
			responseTime:  50 * time.Millisecond,
			contentLength: 100,
			expected:      false,
			reason:        "long content (>=100 chars) is not suspicious even if fast",
		},
		{
			name:          "slow_but_short_content_passes",
			responseTime:  150 * time.Millisecond,
			contentLength: 50,
			expected:      false,
			reason:        "slow response (>=100ms) is not suspicious even if short",
		},
		{
			name:          "slow_and_long_passes",
			responseTime:  500 * time.Millisecond,
			contentLength: 500,
			expected:      false,
			reason:        "normal LLM response timing and length",
		},
		{
			name:          "exactly_100ms_passes",
			responseTime:  100 * time.Millisecond,
			contentLength: 50,
			expected:      false,
			reason:        "exactly 100ms is NOT < 100ms",
		},
		{
			name:          "just_under_100ms_short_fails",
			responseTime:  99 * time.Millisecond,
			contentLength: 99,
			expected:      true,
			reason:        "99ms < 100ms AND 99 chars < 100 = suspicious",
		},
		{
			name:          "exactly_100_chars_passes",
			responseTime:  50 * time.Millisecond,
			contentLength: 100,
			expected:      false,
			reason:        "exactly 100 chars is NOT < 100",
		},
		{
			name:          "zero_time_zero_length_triggers",
			responseTime:  0,
			contentLength: 0,
			expected:      true,
			reason:        "instant empty response = definitely suspicious",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSuspiciouslyFastResponse(tt.responseTime, tt.contentLength)
			assert.Equal(t, tt.expected, result, "%s (time=%v, len=%d)", tt.reason, tt.responseTime, tt.contentLength)
		})
	}
}

func TestIntegrationMockProviderLatency_IsNotSuspiciouslyFast(t *testing.T) {
	t.Parallel()

	provider := newIntegrationMockProvider("test", "short", 0.9)

	assert.GreaterOrEqual(t, provider.latency, 100*time.Millisecond,
		"integrationMockProvider latency must be >= 100ms to avoid IsSuspiciouslyFastResponse detection")
}

func TestIsSuspiciouslyFastResponse_NeverRejectsRealisticLatency(t *testing.T) {
	t.Parallel()

	latencies := []time.Duration{
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		3 * time.Second,
	}

	for _, latency := range latencies {
		for _, contentLen := range []int{0, 10, 50, 100, 500} {
			result := IsSuspiciouslyFastResponse(latency, contentLen)
			assert.False(t, result,
				"Latency %v with %d chars should NEVER be flagged as suspiciously fast",
				latency, contentLen)
		}
	}
}

func TestIsSuspiciouslyFastResponse_NeverRejectsLongContent(t *testing.T) {
	t.Parallel()

	contentLengths := []int{100, 150, 200, 500, 1000}

	for _, contentLen := range contentLengths {
		for _, latency := range []time.Duration{0, 1 * time.Millisecond, 10 * time.Millisecond, 50 * time.Millisecond, 99 * time.Millisecond} {
			result := IsSuspiciouslyFastResponse(latency, contentLen)
			assert.False(t, result,
				"Content length %d at %v latency should NEVER be flagged as suspiciously fast",
				contentLen, latency)
		}
	}
}
