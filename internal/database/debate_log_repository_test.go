package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestPool returns a pgxpool connection for testing
func getTestPool(t *testing.T) *pgxpool.Pool {
	// Use test database URL or skip if not available
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping database test")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("Failed to connect to test database: %v", err)
	}

	return pool
}

func TestLogRetentionPolicy(t *testing.T) {
	t.Run("DefaultRetentionPolicy", func(t *testing.T) {
		policy := DefaultRetentionPolicy()
		assert.Equal(t, 5, policy.RetentionDays)
		assert.False(t, policy.NoExpiration)
	})

	t.Run("NoExpirationPolicy", func(t *testing.T) {
		policy := NoExpirationPolicy()
		assert.True(t, policy.NoExpiration)
		assert.Equal(t, 0, policy.RetentionDays)
	})

	t.Run("CustomRetentionDays", func(t *testing.T) {
		policy := LogRetentionPolicy{
			RetentionDays: 30,
			NoExpiration:  false,
		}
		assert.Equal(t, 30, policy.RetentionDays)
		assert.False(t, policy.NoExpiration)
	})

	t.Run("CustomRetentionHours", func(t *testing.T) {
		policy := LogRetentionPolicy{
			RetentionTime: 24 * time.Hour,
			NoExpiration:  false,
		}
		assert.Equal(t, 24*time.Hour, policy.RetentionTime)
	})
}

func TestDebateLogRepository_CalculateExpiration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	t.Run("DefaultRetention5Days", func(t *testing.T) {
		repo := NewDebateLogRepository(nil, logger, DefaultRetentionPolicy())

		expiration := repo.calculateExpiration()

		require.NotNil(t, expiration)
		expectedMin := time.Now().AddDate(0, 0, 5).Add(-time.Minute)
		expectedMax := time.Now().AddDate(0, 0, 5).Add(time.Minute)
		assert.True(t, expiration.After(expectedMin), "Expiration should be after 5 days minus margin")
		assert.True(t, expiration.Before(expectedMax), "Expiration should be before 5 days plus margin")
	})

	t.Run("NoExpiration", func(t *testing.T) {
		repo := NewDebateLogRepository(nil, logger, NoExpirationPolicy())

		expiration := repo.calculateExpiration()

		assert.Nil(t, expiration, "NoExpiration policy should return nil")
	})

	t.Run("CustomRetentionDays", func(t *testing.T) {
		policy := LogRetentionPolicy{RetentionDays: 10}
		repo := NewDebateLogRepository(nil, logger, policy)

		expiration := repo.calculateExpiration()

		require.NotNil(t, expiration)
		expectedMin := time.Now().AddDate(0, 0, 10).Add(-time.Minute)
		expectedMax := time.Now().AddDate(0, 0, 10).Add(time.Minute)
		assert.True(t, expiration.After(expectedMin))
		assert.True(t, expiration.Before(expectedMax))
	})

	t.Run("CustomRetentionHours", func(t *testing.T) {
		policy := LogRetentionPolicy{RetentionTime: 48 * time.Hour}
		repo := NewDebateLogRepository(nil, logger, policy)

		expiration := repo.calculateExpiration()

		require.NotNil(t, expiration)
		expectedMin := time.Now().Add(48 * time.Hour).Add(-time.Minute)
		expectedMax := time.Now().Add(48 * time.Hour).Add(time.Minute)
		assert.True(t, expiration.After(expectedMin))
		assert.True(t, expiration.Before(expectedMax))
	})

	t.Run("RetentionTimeOverridesDays", func(t *testing.T) {
		// When both are set, RetentionTime takes precedence
		policy := LogRetentionPolicy{
			RetentionDays: 10,
			RetentionTime: 24 * time.Hour,
		}
		repo := NewDebateLogRepository(nil, logger, policy)

		expiration := repo.calculateExpiration()

		require.NotNil(t, expiration)
		// Should be ~24 hours, not 10 days
		expectedMin := time.Now().Add(24 * time.Hour).Add(-time.Minute)
		expectedMax := time.Now().Add(24 * time.Hour).Add(time.Minute)
		assert.True(t, expiration.After(expectedMin))
		assert.True(t, expiration.Before(expectedMax))
	})
}

func TestDebateLogRepository_SetRetentionPolicy(t *testing.T) {
	logger := logrus.New()
	repo := NewDebateLogRepository(nil, logger, DefaultRetentionPolicy())

	t.Run("InitialPolicy", func(t *testing.T) {
		policy := repo.GetRetentionPolicy()
		assert.Equal(t, 5, policy.RetentionDays)
		assert.False(t, policy.NoExpiration)
	})

	t.Run("UpdateToNoExpiration", func(t *testing.T) {
		repo.SetRetentionPolicy(NoExpirationPolicy())

		policy := repo.GetRetentionPolicy()
		assert.True(t, policy.NoExpiration)
	})

	t.Run("UpdateToCustomRetention", func(t *testing.T) {
		repo.SetRetentionPolicy(LogRetentionPolicy{RetentionDays: 30})

		policy := repo.GetRetentionPolicy()
		assert.Equal(t, 30, policy.RetentionDays)
		assert.False(t, policy.NoExpiration)
	})
}

func TestDebateLogEntry(t *testing.T) {
	t.Run("CreateEntry", func(t *testing.T) {
		entry := &DebateLogEntry{
			DebateID:              "debate-123",
			SessionID:             "session-456",
			ParticipantID:         "single-provider-instance-1",
			ParticipantIdentifier: "DeepSeek-1",
			ParticipantName:       "Analytical Thinker (Instance 1)",
			Role:                  "analyst",
			Provider:              "deepseek",
			Model:                 "deepseek-chat",
			Round:                 1,
			Action:                "complete",
			ResponseTimeMs:        2340,
			QualityScore:          0.87,
			TokensUsed:            412,
			ContentLength:         1823,
		}

		assert.Equal(t, "DeepSeek-1", entry.ParticipantIdentifier)
		assert.Equal(t, "deepseek", entry.Provider)
		assert.Equal(t, int64(2340), entry.ResponseTimeMs)
	})

	t.Run("EntryWithError", func(t *testing.T) {
		entry := &DebateLogEntry{
			DebateID:              "debate-123",
			ParticipantID:         "p1",
			ParticipantIdentifier: "DeepSeek-1",
			Provider:              "deepseek",
			Action:                "error",
			ErrorMessage:          "rate limited or quota exceeded",
		}

		assert.Equal(t, "error", entry.Action)
		assert.Contains(t, entry.ErrorMessage, "rate limited")
	})
}

// Integration tests (require database)

func TestDebateLogRepository_Integration(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	ctx := context.Background()

	t.Run("CreateTableAndInsert", func(t *testing.T) {
		repo := NewDebateLogRepository(pool, logger, DefaultRetentionPolicy())

		// Create table
		err := repo.CreateTable(ctx)
		require.NoError(t, err)

		// Insert entry
		entry := &DebateLogEntry{
			DebateID:              "test-debate-" + time.Now().Format("20060102150405"),
			SessionID:             "test-session",
			ParticipantID:         "single-provider-instance-1",
			ParticipantIdentifier: "DeepSeek-1",
			ParticipantName:       "Analytical Thinker (Instance 1)",
			Role:                  "analyst",
			Provider:              "deepseek",
			Model:                 "deepseek-chat",
			Round:                 1,
			Action:                "complete",
			ResponseTimeMs:        2340,
			QualityScore:          0.87,
			TokensUsed:            412,
			ContentLength:         1823,
		}

		err = repo.Insert(ctx, entry)
		require.NoError(t, err)
		assert.Greater(t, entry.ID, int64(0))
		assert.NotNil(t, entry.ExpiresAt)

		// Verify expiration is ~5 days from now
		expectedExpiry := time.Now().AddDate(0, 0, 5)
		assert.True(t, entry.ExpiresAt.After(expectedExpiry.Add(-time.Minute)))
		assert.True(t, entry.ExpiresAt.Before(expectedExpiry.Add(time.Minute)))
	})

	t.Run("InsertWithNoExpiration", func(t *testing.T) {
		repo := NewDebateLogRepository(pool, logger, NoExpirationPolicy())

		entry := &DebateLogEntry{
			DebateID:              "test-permanent-" + time.Now().Format("20060102150405"),
			ParticipantID:         "p1",
			ParticipantIdentifier: "DeepSeek-1",
			Provider:              "deepseek",
			Action:                "complete",
		}

		err := repo.Insert(ctx, entry)
		require.NoError(t, err)
		assert.Nil(t, entry.ExpiresAt, "Entry with no-expiration policy should have nil expires_at")
	})

	t.Run("GetByDebateID", func(t *testing.T) {
		repo := NewDebateLogRepository(pool, logger, DefaultRetentionPolicy())

		debateID := "test-query-debate-" + time.Now().Format("20060102150405")

		// Insert multiple entries
		for i := 1; i <= 3; i++ {
			entry := &DebateLogEntry{
				DebateID:              debateID,
				ParticipantID:         "p" + string(rune('0'+i)),
				ParticipantIdentifier: "DeepSeek-" + string(rune('0'+i)),
				Provider:              "deepseek",
				Round:                 1,
				Action:                "complete",
			}
			err := repo.Insert(ctx, entry)
			require.NoError(t, err)
		}

		// Query by debate ID
		entries, err := repo.GetByDebateID(ctx, debateID)
		require.NoError(t, err)
		assert.Len(t, entries, 3)
	})

	t.Run("GetByParticipantIdentifier", func(t *testing.T) {
		repo := NewDebateLogRepository(pool, logger, DefaultRetentionPolicy())

		identifier := "TestParticipant-" + time.Now().Format("150405")

		// Insert entries
		for i := 1; i <= 2; i++ {
			entry := &DebateLogEntry{
				DebateID:              "debate-" + string(rune('0'+i)),
				ParticipantID:         "p1",
				ParticipantIdentifier: identifier,
				Provider:              "deepseek",
				Round:                 i,
				Action:                "complete",
			}
			err := repo.Insert(ctx, entry)
			require.NoError(t, err)
		}

		// Query by participant identifier
		entries, err := repo.GetByParticipantIdentifier(ctx, identifier)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2)
	})

	t.Run("GetLogStats", func(t *testing.T) {
		repo := NewDebateLogRepository(pool, logger, DefaultRetentionPolicy())

		stats, err := repo.GetLogStats(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, stats.TotalLogs, int64(0))
	})

	t.Run("CleanupExpiredLogs", func(t *testing.T) {
		// Use a very short retention for this test
		policy := LogRetentionPolicy{RetentionTime: time.Millisecond}
		repo := NewDebateLogRepository(pool, logger, policy)

		// Insert an entry that will immediately expire
		entry := &DebateLogEntry{
			DebateID:              "test-expire-" + time.Now().Format("20060102150405"),
			ParticipantID:         "p1",
			ParticipantIdentifier: "DeepSeek-1",
			Provider:              "deepseek",
			Action:                "complete",
		}
		err := repo.Insert(ctx, entry)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Cleanup should find and delete expired entries
		deleted, err := repo.CleanupExpiredLogs(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, int64(1))
	})
}

// Unit tests for policy calculations without database

func TestRetentionPolicyCalculations(t *testing.T) {
	logger := logrus.New()

	testCases := []struct {
		name      string
		policy    LogRetentionPolicy
		expectNil bool
		minExpiry time.Duration
		maxExpiry time.Duration
	}{
		{
			name:      "Default 5 days",
			policy:    DefaultRetentionPolicy(),
			expectNil: false,
			minExpiry: 5*24*time.Hour - time.Hour,
			maxExpiry: 5*24*time.Hour + time.Hour,
		},
		{
			name:      "No expiration",
			policy:    NoExpirationPolicy(),
			expectNil: true,
		},
		{
			name:      "1 day retention",
			policy:    LogRetentionPolicy{RetentionDays: 1},
			expectNil: false,
			minExpiry: 24*time.Hour - time.Hour,
			maxExpiry: 24*time.Hour + time.Hour,
		},
		{
			name:      "12 hour retention",
			policy:    LogRetentionPolicy{RetentionTime: 12 * time.Hour},
			expectNil: false,
			minExpiry: 12*time.Hour - time.Hour,
			maxExpiry: 12*time.Hour + time.Hour,
		},
		{
			name:      "30 day retention",
			policy:    LogRetentionPolicy{RetentionDays: 30},
			expectNil: false,
			minExpiry: 30*24*time.Hour - time.Hour,
			maxExpiry: 30*24*time.Hour + time.Hour,
		},
		{
			name:      "90 day retention",
			policy:    LogRetentionPolicy{RetentionDays: 90},
			expectNil: false,
			minExpiry: 90*24*time.Hour - time.Hour,
			maxExpiry: 90*24*time.Hour + time.Hour,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewDebateLogRepository(nil, logger, tc.policy)
			expiration := repo.calculateExpiration()

			if tc.expectNil {
				assert.Nil(t, expiration, "Expected nil expiration")
			} else {
				require.NotNil(t, expiration, "Expected non-nil expiration")

				now := time.Now()
				minExpiry := now.Add(tc.minExpiry)
				maxExpiry := now.Add(tc.maxExpiry)

				assert.True(t, expiration.After(minExpiry),
					"Expiration %v should be after %v", expiration, minExpiry)
				assert.True(t, expiration.Before(maxExpiry),
					"Expiration %v should be before %v", expiration, maxExpiry)
			}
		})
	}
}

func TestLogEntryIdentifiers(t *testing.T) {
	testCases := []struct {
		name               string
		provider           string
		participantID      string
		expectedIdentifier string
	}{
		{
			name:               "DeepSeek instance 1",
			provider:           "deepseek",
			participantID:      "single-provider-instance-1",
			expectedIdentifier: "DeepSeek-1",
		},
		{
			name:               "Claude instance 3",
			provider:           "claude",
			participantID:      "single-provider-instance-3",
			expectedIdentifier: "Claude-3",
		},
		{
			name:               "Gemini instance 2",
			provider:           "gemini",
			participantID:      "single-provider-instance-2",
			expectedIdentifier: "Gemini-2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &DebateLogEntry{
				Provider:              tc.provider,
				ParticipantID:         tc.participantID,
				ParticipantIdentifier: tc.expectedIdentifier,
			}

			assert.Equal(t, tc.expectedIdentifier, entry.ParticipantIdentifier)
		})
	}
}
