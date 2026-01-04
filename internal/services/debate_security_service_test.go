package services

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDebateConfigForSecurity(debateID string) *DebateConfig {
	return &DebateConfig{
		DebateID:  debateID,
		Topic:     "Security Test Topic",
		MaxRounds: 5,
		Timeout:   5 * time.Minute,
		Strategy:  "consensus",
		Participants: []ParticipantConfig{
			{ParticipantID: "p1", Name: "Participant 1", LLMProvider: "openai", LLMModel: "gpt-4"},
			{ParticipantID: "p2", Name: "Participant 2", LLMProvider: "anthropic", LLMModel: "claude-3"},
		},
	}
}

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 100000, config.MaxPromptLength)
	assert.Equal(t, 500000, config.MaxResponseLength)
	assert.NotEmpty(t, config.SensitivePatterns)
	assert.Equal(t, 100, config.RateLimitRequests)
	assert.Equal(t, time.Minute, config.RateLimitWindow)
	assert.True(t, config.AuditEnabled)
	assert.True(t, config.ContentFilterEnabled)
	assert.True(t, config.PIIDetectionEnabled)
}

func TestDebateSecurityService_New(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.config)
	assert.NotNil(t, svc.violations)
	assert.NotNil(t, svc.auditLog)
	assert.NotNil(t, svc.rateLimiter)
}

func TestNewDebateSecurityServiceWithConfig(t *testing.T) {
	logger := createTestLogger()

	t.Run("with custom config", func(t *testing.T) {
		config := &SecurityConfig{
			MaxPromptLength:      50000,
			MaxResponseLength:    200000,
			RateLimitRequests:    50,
			RateLimitWindow:      30 * time.Second,
			AuditEnabled:         false,
			ContentFilterEnabled: false,
			PIIDetectionEnabled:  false,
		}
		svc := NewDebateSecurityServiceWithConfig(logger, config)

		assert.NotNil(t, svc)
		assert.Equal(t, 50000, svc.config.MaxPromptLength)
		assert.Equal(t, 50, svc.config.RateLimitRequests)
		assert.False(t, svc.config.AuditEnabled)
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		svc := NewDebateSecurityServiceWithConfig(logger, nil)

		assert.NotNil(t, svc)
		assert.Equal(t, 100000, svc.config.MaxPromptLength)
		assert.True(t, svc.config.AuditEnabled)
	})

	t.Run("with blocked patterns", func(t *testing.T) {
		config := &SecurityConfig{
			MaxPromptLength:   100000,
			MaxResponseLength: 500000,
			BlockedPatterns:   []string{`\bforbidden\b`, `\bblocked\b`},
		}
		svc := NewDebateSecurityServiceWithConfig(logger, config)

		assert.Equal(t, 2, len(svc.blockedPatterns))
	})
}

func TestDebateSecurityService_Validate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("validate valid config", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-valid")

		err := svc.ValidateDebateRequest(ctx, config)
		assert.NoError(t, err)
	})

	t.Run("validate nil config", func(t *testing.T) {
		err := svc.ValidateDebateRequest(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Debate config is nil")
	})

	t.Run("validate empty debate ID", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("")

		err := svc.ValidateDebateRequest(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Debate ID is empty")
	})

	t.Run("validate topic exceeds max length", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-long-topic")
		config.Topic = strings.Repeat("a", 100001)

		err := svc.ValidateDebateRequest(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Topic exceeds maximum length")
	})

	t.Run("validate invalid max rounds - zero", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-zero-rounds")
		config.MaxRounds = 0

		err := svc.ValidateDebateRequest(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid max rounds")
	})

	t.Run("validate invalid max rounds - too high", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-too-many-rounds")
		config.MaxRounds = 101

		err := svc.ValidateDebateRequest(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid max rounds")
	})

	t.Run("validate no participants", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-no-participants")
		config.Participants = nil

		err := svc.ValidateDebateRequest(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "No participants configured")
	})

	t.Run("validate invalid participant - empty ID", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-invalid-participant")
		config.Participants = []ParticipantConfig{
			{ParticipantID: "", Name: "Participant 1"},
		}

		err := svc.ValidateDebateRequest(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid participant")
	})

	t.Run("validate invalid participant - empty name", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-invalid-name")
		config.Participants = []ParticipantConfig{
			{ParticipantID: "p1", Name: ""},
		}

		err := svc.ValidateDebateRequest(ctx, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid participant")
	})
}

func TestDebateSecurityService_ValidateWithBlockedContent(t *testing.T) {
	logger := createTestLogger()
	config := &SecurityConfig{
		MaxPromptLength:      100000,
		MaxResponseLength:    500000,
		BlockedPatterns:      []string{`\bforbidden\b`, `\bblocked\b`},
		ContentFilterEnabled: true,
	}
	svc := NewDebateSecurityServiceWithConfig(logger, config)
	ctx := context.Background()

	t.Run("topic with blocked content", func(t *testing.T) {
		debateConfig := createTestDebateConfigForSecurity("debate-blocked")
		debateConfig.Topic = "This topic contains forbidden words"

		err := svc.ValidateDebateRequest(ctx, debateConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Blocked content pattern detected")
	})

	t.Run("topic without blocked content", func(t *testing.T) {
		debateConfig := createTestDebateConfigForSecurity("debate-safe")
		debateConfig.Topic = "This is a safe topic"

		err := svc.ValidateDebateRequest(ctx, debateConfig)
		assert.NoError(t, err)
	})
}

func TestDebateSecurityService_ValidateWithPIIDetection(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("topic with SSN pattern (warning only)", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-ssn")
		config.Topic = "Topic with SSN 123-45-6789"

		// PII detection is warning only, should not block
		err := svc.ValidateDebateRequest(ctx, config)
		assert.NoError(t, err)
	})

	t.Run("topic with credit card pattern (warning only)", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("debate-cc")
		config.Topic = "Topic with card 1234567890123456"

		// PII detection is warning only, should not block
		err := svc.ValidateDebateRequest(ctx, config)
		assert.NoError(t, err)
	})
}

func TestDebateSecurityService_Sanitize(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("sanitize empty response", func(t *testing.T) {
		sanitized, err := svc.SanitizeResponse(ctx, "")
		assert.NoError(t, err)
		assert.Empty(t, sanitized)
	})

	t.Run("sanitize response with PII (SSN)", func(t *testing.T) {
		response := "The SSN is 123-45-6789 and should be masked"

		sanitized, err := svc.SanitizeResponse(ctx, response)
		assert.NoError(t, err)
		assert.NotContains(t, sanitized, "123-45-6789")
		assert.Contains(t, sanitized, "6789") // Last 4 should remain
		assert.Contains(t, sanitized, "*")
	})

	t.Run("sanitize response exceeding max length", func(t *testing.T) {
		response := strings.Repeat("a", 600000)

		sanitized, err := svc.SanitizeResponse(ctx, response)
		assert.NoError(t, err)
		assert.Equal(t, 500000, len(sanitized))
	})

	t.Run("sanitize response with blocked patterns", func(t *testing.T) {
		config := &SecurityConfig{
			MaxPromptLength:      100000,
			MaxResponseLength:    500000,
			BlockedPatterns:      []string{`secret\-\w+`},
			ContentFilterEnabled: true,
		}
		svc := NewDebateSecurityServiceWithConfig(logger, config)

		response := "This contains secret-password123"
		sanitized, err := svc.SanitizeResponse(ctx, response)
		assert.NoError(t, err)
		assert.Contains(t, sanitized, "[FILTERED]")
	})

	t.Run("sanitize response without changes", func(t *testing.T) {
		response := "This is a clean response without any sensitive data"

		sanitized, err := svc.SanitizeResponse(ctx, response)
		assert.NoError(t, err)
		assert.Equal(t, response, sanitized)
	})
}

func TestDebateSecurityService_Audit(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("audit with audit enabled", func(t *testing.T) {
		err := svc.AuditDebate(ctx, "debate-audit-1")
		assert.NoError(t, err)

		entries := svc.GetAuditLog()
		assert.NotEmpty(t, entries)
	})

	t.Run("audit with audit disabled", func(t *testing.T) {
		config := &SecurityConfig{
			MaxPromptLength:   100000,
			MaxResponseLength: 500000,
			AuditEnabled:      false,
		}
		svc := NewDebateSecurityServiceWithConfig(logger, config)

		err := svc.AuditDebate(ctx, "debate-audit-disabled")
		assert.NoError(t, err)

		entries := svc.GetAuditLog()
		assert.Empty(t, entries)
	})
}

func TestDebateSecurityService_AuditAction(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("audit action with details", func(t *testing.T) {
		details := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		}

		err := svc.AuditAction(ctx, "debate-action", "test_action", "test_actor", details)
		assert.NoError(t, err)

		entries := svc.GetAuditLogByDebate("debate-action")
		require.NotEmpty(t, entries)
		assert.Equal(t, "test_action", entries[0].Action)
		assert.Equal(t, "test_actor", entries[0].Actor)
	})
}

func TestDebateSecurityService_CheckRateLimit(t *testing.T) {
	logger := createTestLogger()
	config := &SecurityConfig{
		MaxPromptLength:   100000,
		MaxResponseLength: 500000,
		RateLimitRequests: 5,
		RateLimitWindow:   time.Second,
	}
	svc := NewDebateSecurityServiceWithConfig(logger, config)
	ctx := context.Background()

	t.Run("within rate limit", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			err := svc.CheckRateLimit(ctx, "key-within")
			assert.NoError(t, err)
		}
	})

	t.Run("exceeds rate limit", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			_ = svc.CheckRateLimit(ctx, "key-exceed")
		}

		err := svc.CheckRateLimit(ctx, "key-exceed")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Rate limit exceeded")
	})

	t.Run("rate limit resets after window", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			_ = svc.CheckRateLimit(ctx, "key-reset")
		}

		// Wait for window to expire
		time.Sleep(1100 * time.Millisecond)

		err := svc.CheckRateLimit(ctx, "key-reset")
		assert.NoError(t, err)
	})

	t.Run("different keys have separate limits", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			_ = svc.CheckRateLimit(ctx, "key-A")
		}

		// Key B should still be within limit
		err := svc.CheckRateLimit(ctx, "key-B")
		assert.NoError(t, err)
	})
}

func TestDebateSecurityService_GetViolations(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("no violations initially", func(t *testing.T) {
		violations := svc.GetViolations()
		assert.Empty(t, violations)
	})

	t.Run("violations after failed validation", func(t *testing.T) {
		config := createTestDebateConfigForSecurity("")
		_ = svc.ValidateDebateRequest(ctx, config)

		violations := svc.GetViolations()
		assert.NotEmpty(t, violations)
	})
}

func TestDebateSecurityService_GetViolationsByDebate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	// Create violations for specific debate
	config := createTestDebateConfigForSecurity("debate-violations")
	config.MaxRounds = 0
	_ = svc.ValidateDebateRequest(ctx, config)

	// Create violations for another debate
	config2 := createTestDebateConfigForSecurity("debate-other")
	config2.MaxRounds = 0
	_ = svc.ValidateDebateRequest(ctx, config2)

	t.Run("get violations for specific debate", func(t *testing.T) {
		violations := svc.GetViolationsByDebate("debate-violations")
		assert.NotEmpty(t, violations)
		for _, v := range violations {
			assert.Equal(t, "debate-violations", v.DebateID)
		}
	})

	t.Run("get violations for non-existing debate", func(t *testing.T) {
		violations := svc.GetViolationsByDebate("nonexistent")
		assert.Empty(t, violations)
	})
}

func TestDebateSecurityService_GetAuditLog(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("empty audit log", func(t *testing.T) {
		entries := svc.GetAuditLog()
		assert.Empty(t, entries)
	})

	t.Run("audit log after actions", func(t *testing.T) {
		_ = svc.AuditDebate(ctx, "debate-1")
		_ = svc.AuditDebate(ctx, "debate-2")

		entries := svc.GetAuditLog()
		assert.GreaterOrEqual(t, len(entries), 2)
	})
}

func TestDebateSecurityService_GetAuditLogByDebate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	_ = svc.AuditDebate(ctx, "debate-audit-specific")
	_ = svc.AuditAction(ctx, "debate-audit-specific", "action1", "", nil)
	_ = svc.AuditDebate(ctx, "debate-other")

	t.Run("get audit log for specific debate", func(t *testing.T) {
		entries := svc.GetAuditLogByDebate("debate-audit-specific")
		assert.Equal(t, 2, len(entries))
	})

	t.Run("get audit log for non-existing debate", func(t *testing.T) {
		entries := svc.GetAuditLogByDebate("nonexistent")
		assert.Empty(t, entries)
	})
}

func TestDebateSecurityService_VerifyAuditIntegrity(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("verify empty audit log", func(t *testing.T) {
		valid, invalidEntries := svc.VerifyAuditIntegrity()
		assert.True(t, valid)
		assert.Empty(t, invalidEntries)
	})

	t.Run("verify valid audit entries", func(t *testing.T) {
		_ = svc.AuditDebate(ctx, "debate-integrity-1")
		_ = svc.AuditDebate(ctx, "debate-integrity-2")

		valid, invalidEntries := svc.VerifyAuditIntegrity()
		assert.True(t, valid)
		assert.Empty(t, invalidEntries)
	})

	t.Run("detect tampered audit entry", func(t *testing.T) {
		_ = svc.AuditDebate(ctx, "debate-tampered")

		// Tamper with an entry
		svc.auditMu.Lock()
		if len(svc.auditLog) > 0 {
			svc.auditLog[len(svc.auditLog)-1].Hash = "tampered-hash"
		}
		svc.auditMu.Unlock()

		valid, invalidEntries := svc.VerifyAuditIntegrity()
		assert.False(t, valid)
		assert.NotEmpty(t, invalidEntries)
	})
}

func TestDebateSecurityService_ClearViolations(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	// Create some violations
	config := createTestDebateConfigForSecurity("")
	_ = svc.ValidateDebateRequest(ctx, config)

	violations := svc.GetViolations()
	assert.NotEmpty(t, violations)

	// Clear violations
	svc.ClearViolations()

	violations = svc.GetViolations()
	assert.Empty(t, violations)
}

func TestDebateSecurityService_GetStats(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	t.Run("empty stats", func(t *testing.T) {
		stats := svc.GetStats()
		assert.Equal(t, 0, stats["total_violations"].(int))
		assert.Equal(t, 0, stats["audit_entries"].(int))
		assert.True(t, stats["audit_enabled"].(bool))
		assert.True(t, stats["content_filter"].(bool))
		assert.True(t, stats["pii_detection"].(bool))
	})

	t.Run("stats with data", func(t *testing.T) {
		// Create violations
		config := createTestDebateConfigForSecurity("")
		_ = svc.ValidateDebateRequest(ctx, config)

		// Create audit entries
		_ = svc.AuditDebate(ctx, "debate-stats")

		stats := svc.GetStats()
		assert.Greater(t, stats["total_violations"].(int), 0)
		assert.Greater(t, stats["audit_entries"].(int), 0)

		severityCounts := stats["violations_by_severity"].(map[string]int)
		assert.NotNil(t, severityCounts)
	})
}

func TestDebateSecurityService_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent validations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			config := createTestDebateConfigForSecurity("debate-concurrent-" + string(rune('A'+id%26)))
			_ = svc.ValidateDebateRequest(ctx, config)
		}(i)
	}

	// Concurrent sanitizations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.SanitizeResponse(ctx, "Test response with SSN 123-45-6789")
		}()
	}

	// Concurrent rate limit checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = svc.CheckRateLimit(ctx, "key-"+string(rune('A'+id%10)))
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.GetViolations()
			_ = svc.GetAuditLog()
			_ = svc.GetStats()
		}()
	}

	wg.Wait()
}

func TestSecurityViolation_Structure(t *testing.T) {
	now := time.Now()
	violation := SecurityViolation{
		ID:          "viol-123",
		Type:        "validation",
		Severity:    "high",
		Description: "Test violation",
		DebateID:    "debate-456",
		Timestamp:   now,
		Resolved:    false,
	}

	assert.Equal(t, "viol-123", violation.ID)
	assert.Equal(t, "validation", violation.Type)
	assert.Equal(t, "high", violation.Severity)
	assert.Equal(t, "Test violation", violation.Description)
	assert.Equal(t, "debate-456", violation.DebateID)
	assert.False(t, violation.Resolved)
}

func TestAuditEntry_Structure(t *testing.T) {
	now := time.Now()
	entry := AuditEntry{
		ID:       "audit-123",
		DebateID: "debate-456",
		Action:   "test_action",
		Actor:    "test_actor",
		Details: map[string]interface{}{
			"key": "value",
		},
		Timestamp: now,
		Hash:      "abc123",
	}

	assert.Equal(t, "audit-123", entry.ID)
	assert.Equal(t, "debate-456", entry.DebateID)
	assert.Equal(t, "test_action", entry.Action)
	assert.Equal(t, "test_actor", entry.Actor)
	assert.NotEmpty(t, entry.Hash)
}

func TestRateLimitEntry_Structure(t *testing.T) {
	now := time.Now()
	entry := RateLimitEntry{
		Key:         "test-key",
		Count:       5,
		WindowStart: now,
	}

	assert.Equal(t, "test-key", entry.Key)
	assert.Equal(t, 5, entry.Count)
	assert.Equal(t, now, entry.WindowStart)
}

func TestSecurityConfig_Structure(t *testing.T) {
	config := &SecurityConfig{
		MaxPromptLength:      100000,
		MaxResponseLength:    500000,
		BlockedPatterns:      []string{`pattern1`, `pattern2`},
		SensitivePatterns:    []string{`\d{3}-\d{2}-\d{4}`},
		RateLimitRequests:    100,
		RateLimitWindow:      time.Minute,
		AuditEnabled:         true,
		ContentFilterEnabled: true,
		PIIDetectionEnabled:  true,
	}

	assert.Equal(t, 100000, config.MaxPromptLength)
	assert.Equal(t, 500000, config.MaxResponseLength)
	assert.Len(t, config.BlockedPatterns, 2)
	assert.Len(t, config.SensitivePatterns, 1)
	assert.Equal(t, 100, config.RateLimitRequests)
	assert.True(t, config.AuditEnabled)
}

func TestDebateSecurityService_PIIMasking(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "mask SSN",
			input:    "SSN: 123-45-6789",
			expected: "6789",
		},
		{
			name:     "mask credit card",
			input:    "Card: 1234567890123456",
			expected: "3456",
		},
		{
			name:     "mask multiple PIIs",
			input:    "SSN: 123-45-6789, Card: 1234567890123456",
			expected: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized, err := svc.SanitizeResponse(ctx, tt.input)
			assert.NoError(t, err)
			assert.Contains(t, sanitized, tt.expected)
			assert.Contains(t, sanitized, "*")
		})
	}
}

func TestDebateSecurityService_AuditIntegrityHash(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateSecurityService(logger)
	ctx := context.Background()

	_ = svc.AuditDebate(ctx, "debate-hash-test")

	entries := svc.GetAuditLog()
	require.NotEmpty(t, entries)

	// Verify hash is not empty and is properly formatted (hex)
	hash := entries[0].Hash
	assert.NotEmpty(t, hash)
	assert.Equal(t, 64, len(hash)) // SHA-256 produces 64 hex characters
}

func TestDebateSecurityService_ViolationSeverityLevels(t *testing.T) {
	logger := createTestLogger()
	config := &SecurityConfig{
		MaxPromptLength:      100,
		MaxResponseLength:    100,
		RateLimitRequests:    1,
		RateLimitWindow:      time.Minute,
		ContentFilterEnabled: true,
		PIIDetectionEnabled:  true,
	}
	svc := NewDebateSecurityServiceWithConfig(logger, config)
	ctx := context.Background()

	// Create different severity violations
	// High severity - nil config
	_ = svc.ValidateDebateRequest(ctx, nil)

	// Medium severity - empty debate ID
	config2 := createTestDebateConfigForSecurity("")
	_ = svc.ValidateDebateRequest(ctx, config2)

	// Low severity - response too long
	_, _ = svc.SanitizeResponse(ctx, strings.Repeat("a", 200))

	violations := svc.GetViolations()
	assert.NotEmpty(t, violations)

	stats := svc.GetStats()
	severityCounts := stats["violations_by_severity"].(map[string]int)
	assert.Contains(t, severityCounts, "high")
	assert.Contains(t, severityCounts, "medium")
	assert.Contains(t, severityCounts, "low")
}
