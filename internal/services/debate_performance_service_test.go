package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDebateResultForPerf(debateID string, rounds int, duration time.Duration) *DebateResult {
	now := time.Now()
	responses := make([]ParticipantResponse, 0)
	for i := 0; i < rounds; i++ {
		responses = append(responses, ParticipantResponse{
			ParticipantID:   "p1",
			ParticipantName: "Participant 1",
			Round:           i + 1,
			Response:        "Test response",
			Confidence:      0.8,
			ResponseTime:    100 * time.Millisecond,
		})
	}

	return &DebateResult{
		DebateID:     debateID,
		Topic:        "Performance Test Topic",
		StartTime:    now.Add(-duration),
		EndTime:      now,
		Duration:     duration,
		TotalRounds:  rounds,
		QualityScore: 0.85,
		Success:      true,
		AllResponses: responses,
		Participants: []ParticipantResponse{
			{ParticipantID: "p1", ParticipantName: "Participant 1"},
			{ParticipantID: "p2", ParticipantName: "Participant 2"},
		},
	}
}

func TestDebatePerformanceService_New(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.records)
	assert.Equal(t, 10000, svc.maxRecords)
	assert.NotNil(t, svc.systemMetrics)
}

func TestNewDebatePerformanceServiceWithMaxRecords(t *testing.T) {
	tests := []struct {
		name        string
		maxRecords  int
		expectedMax int
	}{
		{"positive value", 100, 100},
		{"zero value", 0, 10000},
		{"negative value", -50, 10000},
		{"large value", 50000, 50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := createTestLogger()
			svc := NewDebatePerformanceServiceWithMaxRecords(logger, tt.maxRecords)

			assert.NotNil(t, svc)
			assert.Equal(t, tt.expectedMax, svc.maxRecords)
		})
	}
}

func TestDebatePerformanceService_Calculate(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)

	t.Run("calculate metrics for valid result", func(t *testing.T) {
		result := createTestDebateResultForPerf("debate-1", 5, 5*time.Minute)

		metrics := svc.CalculateMetrics(result)
		assert.NotNil(t, metrics)
		assert.Equal(t, 5*time.Minute, metrics.Duration)
		assert.Equal(t, 5, metrics.TotalRounds)
		assert.Equal(t, 0.85, metrics.QualityScore)
		assert.Greater(t, metrics.Throughput, float64(0))
		assert.Greater(t, metrics.Latency, time.Duration(0))
	})

	t.Run("calculate metrics for nil result", func(t *testing.T) {
		metrics := svc.CalculateMetrics(nil)
		assert.NotNil(t, metrics)
		assert.Equal(t, time.Duration(0), metrics.Duration)
		assert.Equal(t, 0, metrics.TotalRounds)
	})

	t.Run("calculate metrics with zero duration", func(t *testing.T) {
		result := createTestDebateResultForPerf("debate-zero", 3, 0)

		metrics := svc.CalculateMetrics(result)
		assert.NotNil(t, metrics)
		assert.Equal(t, float64(0), metrics.Throughput)
	})

	t.Run("calculate metrics with zero rounds", func(t *testing.T) {
		result := createTestDebateResultForPerf("debate-no-rounds", 0, 5*time.Minute)
		result.TotalRounds = 0

		metrics := svc.CalculateMetrics(result)
		assert.NotNil(t, metrics)
		assert.Equal(t, time.Duration(0), metrics.Latency)
	})

	t.Run("calculate error rate from low confidence responses", func(t *testing.T) {
		result := createTestDebateResultForPerf("debate-errors", 5, 5*time.Minute)
		// Add low confidence responses
		result.AllResponses = append(result.AllResponses, ParticipantResponse{
			ParticipantID: "p1",
			Response:      "Test",
			Confidence:    0.05, // Below 0.1 threshold
		})
		result.AllResponses = append(result.AllResponses, ParticipantResponse{
			ParticipantID: "p1",
			Response:      "", // Empty response
			Confidence:    0.9,
		})

		metrics := svc.CalculateMetrics(result)
		assert.Greater(t, metrics.ErrorRate, float64(0))
	})
}

func TestDebatePerformanceService_CalculateMetricsWithID(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)

	t.Run("with valid result", func(t *testing.T) {
		result := createTestDebateResultForPerf("debate-with-id", 3, 3*time.Minute)

		metrics, debateID := svc.CalculateMetricsWithID(result)
		assert.NotNil(t, metrics)
		assert.Equal(t, "debate-with-id", debateID)
	})

	t.Run("with nil result", func(t *testing.T) {
		metrics, debateID := svc.CalculateMetricsWithID(nil)
		assert.NotNil(t, metrics)
		assert.Empty(t, debateID)
	})
}

func TestDebatePerformanceService_Record(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	t.Run("record valid metrics", func(t *testing.T) {
		metrics := &PerformanceMetrics{
			Duration:     5 * time.Minute,
			TotalRounds:  5,
			QualityScore: 0.85,
			Throughput:   1.0,
			Latency:      time.Minute,
			ErrorRate:    0.05,
		}

		err := svc.RecordMetrics(ctx, "debate-1", metrics)
		assert.NoError(t, err)
		assert.Equal(t, 1, svc.GetRecordCount())
	})

	t.Run("record nil metrics", func(t *testing.T) {
		err := svc.RecordMetrics(ctx, "debate-2", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metrics cannot be nil")
	})

	t.Run("record multiple metrics for same debate", func(t *testing.T) {
		metrics := &PerformanceMetrics{
			Duration:    time.Minute,
			TotalRounds: 3,
		}

		err := svc.RecordMetrics(ctx, "debate-multi", metrics)
		assert.NoError(t, err)

		err = svc.RecordMetrics(ctx, "debate-multi", metrics)
		assert.NoError(t, err)

		// Both should be recorded with different record IDs
		retrieved, err := svc.GetMetricsByDebateID(ctx, "debate-multi")
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(retrieved), 2)
	})
}

func TestDebatePerformanceService_RecordMetricsEviction(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceServiceWithMaxRecords(logger, 3)
	ctx := context.Background()

	// Fill to capacity
	for i := 0; i < 3; i++ {
		metrics := &PerformanceMetrics{
			Duration:     time.Duration(i+1) * time.Minute,
			QualityScore: float64(i) * 0.1,
		}
		err := svc.RecordMetrics(ctx, "debate-"+string(rune('A'+i)), metrics)
		require.NoError(t, err)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	assert.Equal(t, 3, svc.GetRecordCount())

	// Add one more - should trigger eviction
	metrics := &PerformanceMetrics{Duration: 10 * time.Minute}
	err := svc.RecordMetrics(ctx, "debate-D", metrics)
	require.NoError(t, err)

	assert.Equal(t, 3, svc.GetRecordCount())
}

func TestDebatePerformanceService_Get(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	// Add test data
	for i := 0; i < 5; i++ {
		metrics := &PerformanceMetrics{
			Duration:     time.Duration(i+1) * time.Minute,
			TotalRounds:  i + 1,
			QualityScore: 0.7 + float64(i)*0.05,
			Throughput:   float64(i + 1),
			Latency:      time.Second * time.Duration(i+1),
			ErrorRate:    0.01 * float64(i),
			ResourceUsage: ResourceUsage{
				CPU:    float64(10 + i*5),
				Memory: uint64(1000 + i*100),
			},
		}
		err := svc.RecordMetrics(ctx, "debate-metrics-"+string(rune('A'+i)), metrics)
		require.NoError(t, err)
	}

	t.Run("get metrics for all time", func(t *testing.T) {
		timeRange := TimeRange{} // Empty time range = all time
		metrics, err := svc.GetMetrics(ctx, timeRange)
		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Greater(t, metrics.Duration, time.Duration(0))
	})

	t.Run("get metrics for specific time range", func(t *testing.T) {
		timeRange := TimeRange{
			StartTime: time.Now().Add(-time.Hour),
			EndTime:   time.Now().Add(time.Hour),
		}
		metrics, err := svc.GetMetrics(ctx, timeRange)
		assert.NoError(t, err)
		assert.NotNil(t, metrics)
	})
}

func TestDebatePerformanceService_GetAggregatedMetrics(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	// Add test data
	now := time.Now()
	for i := 0; i < 5; i++ {
		metrics := &PerformanceMetrics{
			Duration:     time.Duration(i+1) * time.Minute,
			TotalRounds:  i + 1,
			QualityScore: 0.7 + float64(i)*0.05,
			Throughput:   float64(i + 1),
			Latency:      time.Second * time.Duration(i+1),
			ErrorRate:    0.01 * float64(i),
			ResourceUsage: ResourceUsage{
				CPU:    float64(10 + i*5),
				Memory: uint64(1000 + i*100),
			},
		}
		err := svc.RecordMetrics(ctx, "debate-agg-"+string(rune('A'+i)), metrics)
		require.NoError(t, err)
	}

	t.Run("aggregation with all records", func(t *testing.T) {
		timeRange := TimeRange{
			StartTime: now.Add(-time.Hour),
			EndTime:   now.Add(time.Hour),
		}
		aggregation, err := svc.GetAggregatedMetrics(ctx, timeRange)
		assert.NoError(t, err)
		assert.NotNil(t, aggregation)
		assert.Equal(t, 5, aggregation.TotalDebates)
		assert.Equal(t, 15, aggregation.TotalRounds) // 1+2+3+4+5
		assert.Greater(t, aggregation.AverageQuality, float64(0))
		assert.Greater(t, aggregation.AverageThroughput, float64(0))
		assert.Greater(t, aggregation.PeakCPU, float64(0))
		assert.Greater(t, aggregation.PeakMemory, uint64(0))
	})

	t.Run("aggregation with empty time range", func(t *testing.T) {
		timeRange := TimeRange{
			StartTime: now.Add(-2 * time.Hour),
			EndTime:   now.Add(-time.Hour),
		}
		aggregation, err := svc.GetAggregatedMetrics(ctx, timeRange)
		assert.NoError(t, err)
		assert.NotNil(t, aggregation)
		assert.Equal(t, 0, aggregation.TotalDebates)
	})
}

func TestDebatePerformanceService_GetMetricsByDebateID(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	// Add test data
	for i := 0; i < 3; i++ {
		metrics := &PerformanceMetrics{
			Duration:     time.Duration(i+1) * time.Minute,
			QualityScore: 0.8 + float64(i)*0.05,
		}
		err := svc.RecordMetrics(ctx, "target-debate", metrics)
		require.NoError(t, err)
	}

	// Add other debate's metrics
	metrics := &PerformanceMetrics{Duration: time.Minute}
	err := svc.RecordMetrics(ctx, "other-debate", metrics)
	require.NoError(t, err)

	t.Run("get metrics for specific debate", func(t *testing.T) {
		retrieved, err := svc.GetMetricsByDebateID(ctx, "target-debate")
		assert.NoError(t, err)
		assert.Equal(t, 3, len(retrieved))
	})

	t.Run("get metrics for non-existing debate", func(t *testing.T) {
		retrieved, err := svc.GetMetricsByDebateID(ctx, "nonexistent")
		assert.NoError(t, err)
		assert.Empty(t, retrieved)
	})
}

func TestDebatePerformanceService_GetCurrentResourceUsage(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)

	usage := svc.GetCurrentResourceUsage()
	assert.GreaterOrEqual(t, usage.CPU, float64(0))
	assert.LessOrEqual(t, usage.CPU, float64(100))
	assert.Greater(t, usage.Memory, uint64(0))
}

func TestDebatePerformanceService_CleanupOldRecords(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	// Add records
	for i := 0; i < 5; i++ {
		metrics := &PerformanceMetrics{Duration: time.Minute}
		err := svc.RecordMetrics(ctx, "debate-cleanup-"+string(rune('A'+i)), metrics)
		require.NoError(t, err)
	}

	// Manually set old timestamps for some records
	svc.recordsMu.Lock()
	count := 0
	for _, record := range svc.records {
		if count < 3 {
			record.CreatedAt = time.Now().Add(-2 * time.Hour)
		}
		count++
	}
	svc.recordsMu.Unlock()

	// Cleanup records older than 1 hour
	removed, err := svc.CleanupOldRecords(ctx, time.Hour)
	assert.NoError(t, err)
	assert.Equal(t, 3, removed)
	assert.Equal(t, 2, svc.GetRecordCount())
}

func TestDebatePerformanceService_GetRecordCount(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	assert.Equal(t, 0, svc.GetRecordCount())

	for i := 0; i < 5; i++ {
		metrics := &PerformanceMetrics{Duration: time.Minute}
		err := svc.RecordMetrics(ctx, "debate-count-"+string(rune('A'+i)), metrics)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, svc.GetRecordCount())
}

func TestDebatePerformanceService_GetStats(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	t.Run("empty stats", func(t *testing.T) {
		stats := svc.GetStats()
		assert.Equal(t, 0, stats["total_records"].(int))
		assert.Equal(t, 10000, stats["max_records"].(int))
		assert.GreaterOrEqual(t, stats["current_cpu"].(float64), float64(0))
		assert.Greater(t, stats["current_memory"].(uint64), uint64(0))
	})

	t.Run("stats with records", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			metrics := &PerformanceMetrics{
				Duration:     time.Duration(i+1) * time.Minute,
				QualityScore: 0.8,
			}
			err := svc.RecordMetrics(ctx, "debate-stats-"+string(rune('A'+i)), metrics)
			require.NoError(t, err)
		}

		stats := svc.GetStats()
		assert.Equal(t, 5, stats["total_records"].(int))
		assert.Equal(t, 0.8, stats["average_quality"].(float64))
		assert.Greater(t, stats["average_duration_ms"].(int64), int64(0))
	})
}

func TestDebatePerformanceService_ConcurrentAccess(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			metrics := &PerformanceMetrics{
				Duration:     time.Duration(id) * time.Millisecond,
				QualityScore: float64(id) / 100.0,
			}
			_ = svc.RecordMetrics(ctx, "debate-concurrent-"+string(rune('A'+id%26)), metrics)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.GetRecordCount()
			_ = svc.GetStats()
			_ = svc.GetCurrentResourceUsage()
		}()
	}

	wg.Wait()
}

func TestPerformanceRecord_Structure(t *testing.T) {
	now := time.Now()
	record := &PerformanceRecord{
		ID:       "perf-123",
		DebateID: "debate-456",
		Metrics: &PerformanceMetrics{
			Duration:     5 * time.Minute,
			TotalRounds:  5,
			QualityScore: 0.85,
		},
		CreatedAt: now,
	}

	assert.Equal(t, "perf-123", record.ID)
	assert.Equal(t, "debate-456", record.DebateID)
	assert.Equal(t, 5*time.Minute, record.Metrics.Duration)
	assert.Equal(t, now, record.CreatedAt)
}

func TestPerformanceAggregation_Structure(t *testing.T) {
	agg := &PerformanceAggregation{
		TotalDebates:      100,
		TotalDuration:     500 * time.Minute,
		AverageDuration:   5 * time.Minute,
		TotalRounds:       500,
		AverageRounds:     5.0,
		AverageQuality:    0.85,
		AverageThroughput: 1.0,
		AverageLatency:    time.Minute,
		AverageErrorRate:  0.05,
		PeakCPU:           80.0,
		PeakMemory:        1024 * 1024 * 1024,
		PerfTimeRange: TimeRange{
			StartTime: time.Now().Add(-24 * time.Hour),
			EndTime:   time.Now(),
		},
	}

	assert.Equal(t, 100, agg.TotalDebates)
	assert.Equal(t, 500*time.Minute, agg.TotalDuration)
	assert.Equal(t, 5*time.Minute, agg.AverageDuration)
	assert.Equal(t, 500, agg.TotalRounds)
	assert.Equal(t, 0.85, agg.AverageQuality)
	assert.Equal(t, 80.0, agg.PeakCPU)
}

func TestSystemMetricsCollector_Structure(t *testing.T) {
	collector := &SystemMetricsCollector{
		lastCPUTime: time.Now(),
	}

	assert.NotNil(t, collector)
	assert.False(t, collector.lastCPUTime.IsZero())
}

func TestDebatePerformanceService_ResourceUsageCollection(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)

	// Collect multiple times to verify consistency
	for i := 0; i < 3; i++ {
		usage := svc.collectResourceUsage()
		assert.GreaterOrEqual(t, usage.CPU, float64(0))
		assert.LessOrEqual(t, usage.CPU, float64(100))
		assert.Greater(t, usage.Memory, uint64(0))
	}
}

func TestDebatePerformanceService_ThroughputCalculation(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)

	tests := []struct {
		name             string
		rounds           int
		duration         time.Duration
		expectedPositive bool
	}{
		{"normal case", 10, 5 * time.Minute, true},
		{"high throughput", 100, time.Minute, true},
		{"low throughput", 1, 10 * time.Minute, true},
		{"zero duration", 5, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTestDebateResultForPerf("debate-"+tt.name, tt.rounds, tt.duration)
			result.TotalRounds = tt.rounds
			result.Duration = tt.duration

			metrics := svc.CalculateMetrics(result)
			if tt.expectedPositive {
				assert.Greater(t, metrics.Throughput, float64(0))
			} else {
				assert.Equal(t, float64(0), metrics.Throughput)
			}
		})
	}
}

func TestDebatePerformanceService_LatencyCalculation(t *testing.T) {
	logger := createTestLogger()
	svc := NewDebatePerformanceService(logger)

	tests := []struct {
		name            string
		rounds          int
		duration        time.Duration
		expectedLatency time.Duration
	}{
		{"normal case", 5, 5 * time.Minute, time.Minute},
		{"high frequency", 10, time.Minute, 6 * time.Second},
		{"single round", 1, time.Minute, time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createTestDebateResultForPerf("debate-"+tt.name, tt.rounds, tt.duration)
			result.TotalRounds = tt.rounds
			result.Duration = tt.duration

			metrics := svc.CalculateMetrics(result)
			assert.Equal(t, tt.expectedLatency, metrics.Latency)
		})
	}
}
