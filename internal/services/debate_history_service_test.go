package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}

func createTestDebateResult(debateID string, quality float64, success bool) *DebateResult {
	now := time.Now()
	return &DebateResult{
		DebateID:     debateID,
		Topic:        "Test Topic",
		StartTime:    now.Add(-time.Hour),
		EndTime:      now,
		Duration:     time.Hour,
		TotalRounds:  3,
		QualityScore: quality,
		Success:      success,
		Participants: []ParticipantResponse{
			{ParticipantID: "p1", ParticipantName: "Participant 1"},
			{ParticipantID: "p2", ParticipantName: "Participant 2"},
		},
		CogneeEnhanced: true,
		MemoryUsed:     true,
	}
}

func TestDebateHistoryService_New(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.history)
	assert.Equal(t, 10000, svc.maxEntries)
	assert.Equal(t, 0, len(svc.history))
}

func TestNewDebateHistoryServiceWithMaxEntries(t *testing.T) {
	tests := []struct {
		name          string
		maxEntries    int
		expectedMax   int
	}{
		{"positive value", 100, 100},
		{"zero value", 0, 10000},
		{"negative value", -50, 10000},
		{"large value", 50000, 50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createTestLogger()
			svc := NewDebateHistoryServiceWithMaxEntries(logger, tt.maxEntries)

			assert.NotNil(t, svc)
			assert.Equal(t, tt.expectedMax, svc.maxEntries)
		})
	}
}

func TestDebateHistoryService_SaveResult(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	t.Run("save valid result", func(t *testing.T) {
		result := createTestDebateResult("debate-1", 0.85, true)

		err := svc.SaveDebateResult(ctx, result)
		assert.NoError(t, err)
		assert.Equal(t, 1, svc.GetCount())
	})

	t.Run("save nil result", func(t *testing.T) {
		err := svc.SaveDebateResult(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot save nil debate result")
	})

	t.Run("save result without debate ID", func(t *testing.T) {
		result := createTestDebateResult("", 0.85, true)

		err := svc.SaveDebateResult(ctx, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "debate result must have a debate ID")
	})

	t.Run("overwrite existing result", func(t *testing.T) {
		result1 := createTestDebateResult("debate-overwrite", 0.85, true)
		result2 := createTestDebateResult("debate-overwrite", 0.95, true)

		err := svc.SaveDebateResult(ctx, result1)
		assert.NoError(t, err)

		err = svc.SaveDebateResult(ctx, result2)
		assert.NoError(t, err)

		retrieved, err := svc.GetDebateByID(ctx, "debate-overwrite")
		assert.NoError(t, err)
		assert.Equal(t, 0.95, retrieved.QualityScore)
	})
}

func TestDebateHistoryService_SaveDebateResultEviction(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryServiceWithMaxEntries(logger, 3)
	ctx := context.Background()

	// Fill to capacity
	for i := 0; i < 3; i++ {
		result := createTestDebateResult("debate-"+string(rune('A'+i)), 0.5+float64(i)*0.1, true)
		result.StartTime = time.Now().Add(time.Duration(i) * time.Minute)
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, svc.GetCount())

	// Add one more - should trigger eviction
	result := createTestDebateResult("debate-D", 0.9, true)
	result.StartTime = time.Now().Add(4 * time.Minute)
	err := svc.SaveDebateResult(ctx, result)
	require.NoError(t, err)

	// Should still be at max capacity
	assert.Equal(t, 3, svc.GetCount())

	// The oldest should have been evicted
	_, err = svc.GetDebateByID(ctx, "debate-A")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "debate not found")
}

func TestDebateHistoryService_Query(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	// Add test data
	now := time.Now()
	for i := 0; i < 10; i++ {
		result := createTestDebateResult("debate-"+string(rune('A'+i)), 0.5+float64(i)*0.05, i%2 == 0)
		result.StartTime = now.Add(time.Duration(i) * time.Hour)
		result.EndTime = result.StartTime.Add(30 * time.Minute)
		result.Participants = []ParticipantResponse{
			{ParticipantID: "p1", ParticipantName: "Participant 1"},
			{ParticipantID: "p" + string(rune('1'+i)), ParticipantName: "Participant " + string(rune('1'+i))},
		}
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}

	t.Run("query all", func(t *testing.T) {
		results, err := svc.QueryHistory(ctx, nil)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(results))
	})

	t.Run("query with limit", func(t *testing.T) {
		filters := &HistoryFilters{Limit: 5}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.Equal(t, 5, len(results))
	})

	t.Run("query with offset", func(t *testing.T) {
		filters := &HistoryFilters{Offset: 7}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(results))
	})

	t.Run("query with offset beyond results", func(t *testing.T) {
		filters := &HistoryFilters{Offset: 100}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("query with limit and offset", func(t *testing.T) {
		filters := &HistoryFilters{Limit: 3, Offset: 2}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(results))
	})

	t.Run("query with min quality score", func(t *testing.T) {
		minScore := 0.7
		filters := &HistoryFilters{MinQualityScore: &minScore}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		for _, r := range results {
			assert.GreaterOrEqual(t, r.QualityScore, minScore)
		}
	})

	t.Run("query with max quality score", func(t *testing.T) {
		maxScore := 0.7
		filters := &HistoryFilters{MaxQualityScore: &maxScore}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		for _, r := range results {
			assert.LessOrEqual(t, r.QualityScore, maxScore)
		}
	})

	t.Run("query with participant filter", func(t *testing.T) {
		filters := &HistoryFilters{ParticipantIDs: []string{"p1"}}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.Equal(t, 10, len(results)) // All debates have p1
	})

	t.Run("query with non-existing participant", func(t *testing.T) {
		filters := &HistoryFilters{ParticipantIDs: []string{"nonexistent"}}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("query with time range", func(t *testing.T) {
		startTime := now.Add(2 * time.Hour)
		endTime := now.Add(6 * time.Hour)
		filters := &HistoryFilters{
			StartTime: &startTime,
			EndTime:   &endTime,
		}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.True(t, len(results) > 0)
	})
}

func TestDebateHistoryService_GetDebateByID(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	result := createTestDebateResult("debate-123", 0.85, true)
	err := svc.SaveDebateResult(ctx, result)
	require.NoError(t, err)

	t.Run("get existing debate", func(t *testing.T) {
		retrieved, err := svc.GetDebateByID(ctx, "debate-123")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "debate-123", retrieved.DebateID)
		assert.Equal(t, 0.85, retrieved.QualityScore)
	})

	t.Run("get non-existing debate", func(t *testing.T) {
		retrieved, err := svc.GetDebateByID(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Contains(t, err.Error(), "debate not found")
	})

	t.Run("returned result is a copy", func(t *testing.T) {
		retrieved, err := svc.GetDebateByID(ctx, "debate-123")
		assert.NoError(t, err)

		// Modify the retrieved result
		retrieved.QualityScore = 0.99

		// Original should be unchanged
		original, err := svc.GetDebateByID(ctx, "debate-123")
		assert.NoError(t, err)
		assert.Equal(t, 0.85, original.QualityScore)
	})
}

func TestDebateHistoryService_DeleteDebate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	result := createTestDebateResult("debate-to-delete", 0.85, true)
	err := svc.SaveDebateResult(ctx, result)
	require.NoError(t, err)

	t.Run("delete existing debate", func(t *testing.T) {
		err := svc.DeleteDebate(ctx, "debate-to-delete")
		assert.NoError(t, err)

		_, err = svc.GetDebateByID(ctx, "debate-to-delete")
		assert.Error(t, err)
	})

	t.Run("delete non-existing debate", func(t *testing.T) {
		err := svc.DeleteDebate(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "debate not found")
	})
}

func TestDebateHistoryService_GetSummary(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	t.Run("empty history", func(t *testing.T) {
		summary, err := svc.GetSummary(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, summary)
		assert.Equal(t, 0, summary.TotalDebates)
	})

	t.Run("with data", func(t *testing.T) {
		// Add test data
		for i := 0; i < 5; i++ {
			result := createTestDebateResult("debate-sum-"+string(rune('A'+i)), 0.6+float64(i)*0.05, i%2 == 0)
			result.TotalRounds = i + 1
			result.Duration = time.Duration(i+1) * time.Minute
			result.CogneeEnhanced = i < 3
			result.MemoryUsed = i < 2
			err := svc.SaveDebateResult(ctx, result)
			require.NoError(t, err)
		}

		summary, err := svc.GetSummary(ctx, nil)
		assert.NoError(t, err)
		assert.Equal(t, 5, summary.TotalDebates)
		assert.Equal(t, 3, summary.SuccessfulDebates)
		assert.Equal(t, 2, summary.FailedDebates)
		assert.True(t, summary.AverageQuality > 0)
		assert.True(t, summary.AverageDuration > 0)
		assert.Equal(t, 15, summary.TotalRounds) // 1+2+3+4+5
		assert.Equal(t, 3, summary.CogneeEnhanced)
		assert.Equal(t, 2, summary.MemoryUsed)
	})
}

func TestDebateHistoryService_CleanupOldEntries(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	// Add old and new entries
	now := time.Now()
	for i := 0; i < 5; i++ {
		result := createTestDebateResult("debate-old-"+string(rune('A'+i)), 0.85, true)
		result.EndTime = now.Add(-time.Duration(i+1) * 24 * time.Hour)
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}

	for i := 0; i < 3; i++ {
		result := createTestDebateResult("debate-new-"+string(rune('A'+i)), 0.85, true)
		result.EndTime = now
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}

	assert.Equal(t, 8, svc.GetCount())

	// Cleanup entries older than 2 days
	// Entries B, C, D, E are 2, 3, 4, 5 days old - at or older than 2 day cutoff
	removed, err := svc.CleanupOldEntries(ctx, 2*24*time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, 4, removed) // 4 entries are at or older than 2 days
	assert.Equal(t, 4, svc.GetCount()) // 1 old entry (A) + 3 new entries remain
}

func TestDebateHistoryService_GetCount(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	assert.Equal(t, 0, svc.GetCount())

	for i := 0; i < 5; i++ {
		result := createTestDebateResult("debate-count-"+string(rune('A'+i)), 0.85, true)
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, svc.GetCount())
}

func TestDebateHistoryService_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result := createTestDebateResult("debate-concurrent-"+string(rune('A'+id)), float64(id)/100.0, true)
			err := svc.SaveDebateResult(ctx, result)
			assert.NoError(t, err)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.QueryHistory(ctx, nil)
			_ = svc.GetCount()
		}()
	}

	wg.Wait()
}

func TestDebateHistoryService_SortResults(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 5; i++ {
		result := createTestDebateResult("debate-sort-"+string(rune('A'+i)), 0.9-float64(i)*0.1, true)
		result.StartTime = now.Add(time.Duration(i) * time.Hour)
		result.Duration = time.Duration(5-i) * time.Minute
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}

	t.Run("default sort by start_time desc", func(t *testing.T) {
		results, err := svc.QueryHistory(ctx, nil)
		assert.NoError(t, err)
		assert.Equal(t, 5, len(results))
		// Results should be sorted by start_time descending
		for i := 0; i < len(results)-1; i++ {
			assert.True(t, results[i].StartTime.After(results[i+1].StartTime) || results[i].StartTime.Equal(results[i+1].StartTime))
		}
	})
}

func TestDebateHistoryService_FiltersByTimeRange(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	baseTime := time.Now()

	// Create debates at different times
	for i := 0; i < 10; i++ {
		result := createTestDebateResult("debate-time-"+string(rune('A'+i)), 0.85, true)
		result.StartTime = baseTime.Add(time.Duration(i) * time.Hour)
		result.EndTime = result.StartTime.Add(30 * time.Minute)
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}

	t.Run("filter by start time only", func(t *testing.T) {
		startTime := baseTime.Add(5 * time.Hour)
		filters := &HistoryFilters{StartTime: &startTime}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		for _, r := range results {
			assert.False(t, r.StartTime.Before(startTime))
		}
	})

	t.Run("filter by end time only", func(t *testing.T) {
		endTime := baseTime.Add(5 * time.Hour)
		filters := &HistoryFilters{EndTime: &endTime}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		for _, r := range results {
			assert.False(t, r.EndTime.After(endTime))
		}
	})
}

func TestDebateHistorySummary_Structure(t *testing.T) {
	summary := &DebateHistorySummary{
		TotalDebates:      100,
		SuccessfulDebates: 80,
		FailedDebates:     20,
		AverageQuality:    0.85,
		AverageDuration:   5 * time.Minute,
		TotalRounds:       300,
		CogneeEnhanced:    50,
		MemoryUsed:        30,
		HistoryTimeRange: TimeRange{
			StartTime: time.Now().Add(-24 * time.Hour),
			EndTime:   time.Now(),
		},
	}

	assert.Equal(t, 100, summary.TotalDebates)
	assert.Equal(t, 80, summary.SuccessfulDebates)
	assert.Equal(t, 20, summary.FailedDebates)
	assert.Equal(t, 0.85, summary.AverageQuality)
	assert.Equal(t, 5*time.Minute, summary.AverageDuration)
	assert.Equal(t, 300, summary.TotalRounds)
}

func TestDebateHistoryService_CombinedFilters(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebateHistoryService(logger)
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 20; i++ {
		result := createTestDebateResult("debate-combined-"+string(rune('A'+i)), 0.4+float64(i)*0.03, i%2 == 0)
		result.StartTime = now.Add(time.Duration(i) * time.Hour)
		result.EndTime = result.StartTime.Add(30 * time.Minute)
		result.Participants = []ParticipantResponse{
			{ParticipantID: "p1", ParticipantName: "Participant 1"},
		}
		if i%3 == 0 {
			result.Participants = append(result.Participants, ParticipantResponse{
				ParticipantID: "special", ParticipantName: "Special Participant",
			})
		}
		err := svc.SaveDebateResult(ctx, result)
		require.NoError(t, err)
	}

	t.Run("combined filters", func(t *testing.T) {
		minQuality := 0.5
		filters := &HistoryFilters{
			MinQualityScore: &minQuality,
			ParticipantIDs:  []string{"special"},
			Limit:           5,
		}
		results, err := svc.QueryHistory(ctx, filters)
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(results), 5)
		for _, r := range results {
			assert.GreaterOrEqual(t, r.QualityScore, minQuality)
		}
	})
}
