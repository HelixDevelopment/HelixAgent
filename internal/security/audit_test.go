package security

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryAuditLogger(t *testing.T) {
	t.Parallel()
	t.Run("with default values", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(0, nil)

		assert.NotNil(t, logger)
		assert.Equal(t, 10000, logger.maxEvents)
	})

	t.Run("with custom values", func(t *testing.T) {
		t.Parallel()
		customLogger := logrus.New()
		logger := NewInMemoryAuditLogger(5000, customLogger)

		assert.NotNil(t, logger)
		assert.Equal(t, 5000, logger.maxEvents)
	})
}

func TestInMemoryAuditLogger_Log(t *testing.T) {
	t.Parallel()
	logger := NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	t.Run("logs event successfully", func(t *testing.T) {
		t.Parallel()
		event := &AuditEvent{
			EventType: AuditEventToolCall,
			Action:    "test-action",
			Result:    "success",
			Risk:      SeverityLow,
			UserID:    "user-123",
		}

		err := logger.Log(ctx, event)

		require.NoError(t, err)
		assert.NotEmpty(t, event.ID)
		assert.False(t, event.Timestamp.IsZero())
	})

	t.Run("preserves existing ID and timestamp", func(t *testing.T) {
		t.Parallel()
		now := time.Now().Add(-1 * time.Hour)
		event := &AuditEvent{
			ID:        "custom-id",
			Timestamp: now,
			EventType: AuditEventToolCall,
		}

		err := logger.Log(ctx, event)

		require.NoError(t, err)
		assert.Equal(t, "custom-id", event.ID)
		assert.Equal(t, now, event.Timestamp)
	})

	t.Run("enforces max events limit", func(t *testing.T) {
		t.Parallel()
		smallLogger := NewInMemoryAuditLogger(10, nil)

		// Add more events than the limit
		for i := 0; i < 15; i++ {
			event := &AuditEvent{
				EventType: AuditEventToolCall,
				Action:    "action",
			}
			err := smallLogger.Log(ctx, event)
			require.NoError(t, err)
		}

		// Should have removed oldest events
		assert.LessOrEqual(t, len(smallLogger.events), 15)
	})
}

func TestInMemoryAuditLogger_Query(t *testing.T) {
	t.Parallel()
	logger := NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	// Add test events
	now := time.Now()
	events := []*AuditEvent{
		{ID: "1", EventType: AuditEventToolCall, UserID: "user1", Risk: SeverityLow, Timestamp: now.Add(-5 * time.Hour)},
		{ID: "2", EventType: AuditEventGuardrailBlock, UserID: "user2", Risk: SeverityHigh, Timestamp: now.Add(-3 * time.Hour)},
		{ID: "3", EventType: AuditEventToolCall, UserID: "user1", Risk: SeverityMedium, Timestamp: now.Add(-1 * time.Hour)},
		{ID: "4", EventType: AuditEventPermissionDeny, UserID: "user3", Risk: SeverityCritical, Timestamp: now},
	}

	for _, e := range events {
		err := logger.Log(ctx, e)
		require.NoError(t, err)
	}

	t.Run("filter by user ID", func(t *testing.T) {
		t.Parallel()
		results, err := logger.Query(ctx, &AuditFilter{UserID: "user1"})

		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, r := range results {
			assert.Equal(t, "user1", r.UserID)
		}
	})

	t.Run("filter by event types", func(t *testing.T) {
		t.Parallel()
		results, err := logger.Query(ctx, &AuditFilter{
			EventTypes: []AuditEventType{AuditEventToolCall},
		})

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by time range", func(t *testing.T) {
		t.Parallel()
		start := now.Add(-4 * time.Hour)
		end := now.Add(-30 * time.Minute)
		results, err := logger.Query(ctx, &AuditFilter{
			StartTime: &start,
			EndTime:   &end,
		})

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("filter by minimum risk", func(t *testing.T) {
		t.Parallel()
		results, err := logger.Query(ctx, &AuditFilter{MinRisk: SeverityHigh})

		require.NoError(t, err)
		assert.Len(t, results, 2) // High and Critical
	})

	t.Run("with limit", func(t *testing.T) {
		t.Parallel()
		results, err := logger.Query(ctx, &AuditFilter{Limit: 2})

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns results sorted by timestamp (newest first)", func(t *testing.T) {
		t.Parallel()
		results, err := logger.Query(ctx, &AuditFilter{})

		require.NoError(t, err)
		assert.Len(t, results, 4)
		for i := 1; i < len(results); i++ {
			assert.True(t, results[i-1].Timestamp.After(results[i].Timestamp) ||
				results[i-1].Timestamp.Equal(results[i].Timestamp))
		}
	})
}

func TestInMemoryAuditLogger_GetStats(t *testing.T) {
	t.Parallel()
	logger := NewInMemoryAuditLogger(100, nil)
	ctx := context.Background()

	// Add test events
	events := []*AuditEvent{
		{EventType: AuditEventToolCall, UserID: "user1", Risk: SeverityLow},
		{EventType: AuditEventGuardrailBlock, UserID: "user1", Risk: SeverityHigh},
		{EventType: AuditEventToolCall, UserID: "user2", Risk: SeverityMedium},
		{EventType: AuditEventPermissionDeny, UserID: "user1", Risk: SeverityCritical},
	}

	for _, e := range events {
		err := logger.Log(ctx, e)
		require.NoError(t, err)
	}

	t.Run("returns correct statistics", func(t *testing.T) {
		t.Parallel()
		since := time.Now().Add(-1 * time.Hour)
		stats, err := logger.GetStats(ctx, since)

		require.NoError(t, err)
		assert.Equal(t, int64(4), stats.TotalEvents)
		assert.Equal(t, int64(2), stats.EventsByType[AuditEventToolCall])
		assert.Equal(t, int64(1), stats.EventsByType[AuditEventGuardrailBlock])
		assert.Equal(t, int64(1), stats.EventsByType[AuditEventPermissionDeny])
	})

	t.Run("calculates user stats correctly", func(t *testing.T) {
		t.Parallel()
		since := time.Now().Add(-1 * time.Hour)
		stats, err := logger.GetStats(ctx, since)

		require.NoError(t, err)
		assert.NotEmpty(t, stats.TopUsers)

		// Find user1 stats
		var user1Stats *UserAuditStat
		for i := range stats.TopUsers {
			if stats.TopUsers[i].UserID == "user1" {
				user1Stats = &stats.TopUsers[i]
				break
			}
		}
		require.NotNil(t, user1Stats)
		assert.Equal(t, int64(3), user1Stats.Events)
		assert.Equal(t, int64(2), user1Stats.Blocks) // GuardrailBlock + PermissionDeny
	})

	t.Run("limits top users to 10", func(t *testing.T) {
		t.Parallel()
		// Use a dedicated logger so parallel sibling subtests that read
		// the parent's shared `logger` are not polluted by these 20
		// extra events. (Phase 9 fix: the previous implementation wrote
		// to the shared logger under t.Parallel(), which raced with the
		// "returns correct statistics" subtest's assertion of 4 events.)
		isolatedLogger := NewInMemoryAuditLogger(100, nil)
		for i := 0; i < 20; i++ {
			err := isolatedLogger.Log(ctx, &AuditEvent{
				EventType: AuditEventToolCall,
				UserID:    "user" + string(rune('A'+i)),
			})
			require.NoError(t, err)
		}

		since := time.Now().Add(-1 * time.Hour)
		stats, err := isolatedLogger.GetStats(ctx, since)

		require.NoError(t, err)
		assert.LessOrEqual(t, len(stats.TopUsers), 10)
	})
}

func TestInMemoryAuditLogger_isRiskAtLeast(t *testing.T) {
	t.Parallel()
	logger := NewInMemoryAuditLogger(100, nil)

	tests := []struct {
		actual   Severity
		minimum  Severity
		expected bool
	}{
		{SeverityInfo, SeverityInfo, true},
		{SeverityLow, SeverityInfo, true},
		{SeverityMedium, SeverityLow, true},
		{SeverityHigh, SeverityMedium, true},
		{SeverityCritical, SeverityHigh, true},
		{SeverityInfo, SeverityLow, false},
		{SeverityLow, SeverityMedium, false},
		{SeverityMedium, SeverityCritical, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.actual)+"_vs_"+string(tt.minimum), func(t *testing.T) {
			t.Parallel()
			result := logger.isRiskAtLeast(tt.actual, tt.minimum)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileAuditLogger(t *testing.T) {
	t.Parallel()
	t.Run("creates and logs to file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "audit_test_*.log")
		require.NoError(t, err)
		_ = tmpFile.Close()
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		logger, err := NewFileAuditLogger(tmpFile.Name(), nil)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		event := &AuditEvent{
			EventType: AuditEventToolCall,
			Action:    "test",
		}

		err = logger.Log(context.Background(), event)
		require.NoError(t, err)

		// Verify event was logged
		content, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		assert.Contains(t, string(content), "test")
	})

	t.Run("query returns error", func(t *testing.T) {
		t.Parallel()
		tmpFile, err := os.CreateTemp("", "audit_test_*.log")
		require.NoError(t, err)
		_ = tmpFile.Close()
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		logger, err := NewFileAuditLogger(tmpFile.Name(), nil)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		_, err = logger.Query(context.Background(), &AuditFilter{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})

	t.Run("get stats returns error", func(t *testing.T) {
		t.Parallel()
		tmpFile, err := os.CreateTemp("", "audit_test_*.log")
		require.NoError(t, err)
		_ = tmpFile.Close()
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		logger, err := NewFileAuditLogger(tmpFile.Name(), nil)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		_, err = logger.GetStats(context.Background(), time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})

	t.Run("fails on invalid file path", func(t *testing.T) {
		t.Parallel()
		_, err := NewFileAuditLogger("/nonexistent/path/audit.log", nil)
		assert.Error(t, err)
	})
}

func TestCompositeAuditLogger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("logs to all loggers", func(t *testing.T) {
		t.Parallel()
		logger1 := NewInMemoryAuditLogger(100, nil)
		logger2 := NewInMemoryAuditLogger(100, nil)

		composite := NewCompositeAuditLogger(logger1, logger2)

		event := &AuditEvent{
			EventType: AuditEventToolCall,
			Action:    "test",
		}

		err := composite.Log(ctx, event)
		require.NoError(t, err)

		// Both loggers should have the event
		results1, _ := logger1.Query(ctx, &AuditFilter{})
		results2, _ := logger2.Query(ctx, &AuditFilter{})

		assert.Len(t, results1, 1)
		assert.Len(t, results2, 1)
	})

	t.Run("add logger", func(t *testing.T) {
		t.Parallel()
		composite := NewCompositeAuditLogger()
		logger := NewInMemoryAuditLogger(100, nil)

		composite.AddLogger(logger)

		event := &AuditEvent{EventType: AuditEventToolCall}
		err := composite.Log(ctx, event)
		require.NoError(t, err)

		results, _ := logger.Query(ctx, &AuditFilter{})
		assert.Len(t, results, 1)
	})

	t.Run("query returns from first supporting logger", func(t *testing.T) {
		t.Parallel()
		logger := NewInMemoryAuditLogger(100, nil)
		composite := NewCompositeAuditLogger(logger)

		// Add an event
		_ = composite.Log(ctx, &AuditEvent{EventType: AuditEventToolCall})

		results, err := composite.Query(ctx, &AuditFilter{})
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("get stats returns from first supporting logger", func(t *testing.T) {
		t.Parallel()
		logger := NewInMemoryAuditLogger(100, nil)
		composite := NewCompositeAuditLogger(logger)

		// Add an event
		_ = composite.Log(ctx, &AuditEvent{EventType: AuditEventToolCall})

		stats, err := composite.GetStats(ctx, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(1), stats.TotalEvents)
	})

	t.Run("query returns error when no logger supports it", func(t *testing.T) {
		t.Parallel()
		composite := NewCompositeAuditLogger()

		_, err := composite.Query(ctx, &AuditFilter{})
		assert.Error(t, err)
	})

	t.Run("get stats returns error when no logger supports it", func(t *testing.T) {
		t.Parallel()
		composite := NewCompositeAuditLogger()

		_, err := composite.GetStats(ctx, time.Now())
		assert.Error(t, err)
	})
}
