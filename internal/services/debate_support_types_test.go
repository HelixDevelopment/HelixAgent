package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDebateStatus_Struct(t *testing.T) {
	now := time.Now()
	status := DebateStatus{
		DebateID:         "debate-123",
		Status:           "in_progress",
		CurrentRound:     2,
		TotalRounds:      5,
		StartTime:        now,
		EstimatedEndTime: now.Add(time.Hour),
		Participants: []ParticipantStatus{
			{
				ParticipantID:   "p1",
				ParticipantName: "Claude",
				Status:          "active",
			},
		},
		Errors:   []string{},
		Metadata: map[string]any{"topic": "AI safety"},
	}

	assert.Equal(t, "debate-123", status.DebateID)
	assert.Equal(t, "in_progress", status.Status)
	assert.Equal(t, 2, status.CurrentRound)
	assert.Equal(t, 5, status.TotalRounds)
	assert.Equal(t, 1, len(status.Participants))
	assert.Equal(t, "AI safety", status.Metadata["topic"])
}

func TestParticipantStatus_Struct(t *testing.T) {
	status := ParticipantStatus{
		ParticipantID:   "participant-1",
		ParticipantName: "DeepSeek",
		Status:          "responding",
		CurrentResponse: "In my analysis...",
		ResponseTime:    500 * time.Millisecond,
		Error:           "",
	}

	assert.Equal(t, "participant-1", status.ParticipantID)
	assert.Equal(t, "DeepSeek", status.ParticipantName)
	assert.Equal(t, "responding", status.Status)
	assert.Equal(t, "In my analysis...", status.CurrentResponse)
	assert.Equal(t, 500*time.Millisecond, status.ResponseTime)
	assert.Empty(t, status.Error)
}

func TestParticipantStatus_WithError(t *testing.T) {
	status := ParticipantStatus{
		ParticipantID:   "participant-2",
		ParticipantName: "Gemini",
		Status:          "error",
		CurrentResponse: "",
		ResponseTime:    0,
		Error:           "timeout exceeded",
	}

	assert.Equal(t, "error", status.Status)
	assert.Equal(t, "timeout exceeded", status.Error)
}

func TestPerformanceMetrics_Struct(t *testing.T) {
	metrics := PerformanceMetrics{
		Duration:     10 * time.Minute,
		TotalRounds:  5,
		QualityScore: 0.95,
		Throughput:   100.5,
		Latency:      50 * time.Millisecond,
		ErrorRate:    0.02,
		ResourceUsage: ResourceUsage{
			CPU:     45.5,
			Memory:  1024 * 1024 * 512,
			Network: 1024 * 100,
		},
	}

	assert.Equal(t, 10*time.Minute, metrics.Duration)
	assert.Equal(t, 5, metrics.TotalRounds)
	assert.Equal(t, 0.95, metrics.QualityScore)
	assert.Equal(t, 100.5, metrics.Throughput)
	assert.Equal(t, 50*time.Millisecond, metrics.Latency)
	assert.Equal(t, 0.02, metrics.ErrorRate)
	assert.Equal(t, 45.5, metrics.ResourceUsage.CPU)
}

func TestResourceUsage_Struct(t *testing.T) {
	usage := ResourceUsage{
		CPU:     75.5,
		Memory:  1024 * 1024 * 1024,
		Network: 1024 * 1024 * 10,
	}

	assert.Equal(t, 75.5, usage.CPU)
	assert.Equal(t, uint64(1024*1024*1024), usage.Memory)
	assert.Equal(t, uint64(1024*1024*10), usage.Network)
}

func TestHistoryFilters_Struct(t *testing.T) {
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()
	minScore := 0.5
	maxScore := 0.9

	filters := HistoryFilters{
		StartTime:       &startTime,
		EndTime:         &endTime,
		ParticipantIDs:  []string{"p1", "p2"},
		MinQualityScore: &minScore,
		MaxQualityScore: &maxScore,
		Limit:           100,
		Offset:          0,
	}

	assert.NotNil(t, filters.StartTime)
	assert.NotNil(t, filters.EndTime)
	assert.Equal(t, 2, len(filters.ParticipantIDs))
	assert.Equal(t, 0.5, *filters.MinQualityScore)
	assert.Equal(t, 0.9, *filters.MaxQualityScore)
	assert.Equal(t, 100, filters.Limit)
	assert.Equal(t, 0, filters.Offset)
}

func TestHistoryFilters_Empty(t *testing.T) {
	filters := HistoryFilters{}

	assert.Nil(t, filters.StartTime)
	assert.Nil(t, filters.EndTime)
	assert.Nil(t, filters.ParticipantIDs)
	assert.Nil(t, filters.MinQualityScore)
	assert.Nil(t, filters.MaxQualityScore)
	assert.Equal(t, 0, filters.Limit)
	assert.Equal(t, 0, filters.Offset)
}

func TestTimeRange_Struct(t *testing.T) {
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	timeRange := TimeRange{
		StartTime: startTime,
		EndTime:   endTime,
	}

	assert.Equal(t, startTime, timeRange.StartTime)
	assert.Equal(t, endTime, timeRange.EndTime)
	assert.True(t, timeRange.EndTime.After(timeRange.StartTime))
}

func TestDebateReport_Struct(t *testing.T) {
	now := time.Now()
	report := DebateReport{
		ReportID:    "report-123",
		DebateID:    "debate-456",
		GeneratedAt: now,
		Summary:     "A comprehensive debate on AI safety",
		KeyFindings: []string{
			"Consensus on safety importance",
			"Disagreement on implementation timelines",
		},
		Recommendations: []string{
			"Implement phased approach",
			"Establish safety benchmarks",
		},
		Metrics: PerformanceMetrics{
			Duration:     15 * time.Minute,
			TotalRounds:  7,
			QualityScore: 0.92,
		},
		Appendices: map[string]any{
			"raw_data":   "base64_encoded_data",
			"statistics": map[string]float64{"mean": 0.85},
		},
	}

	assert.Equal(t, "report-123", report.ReportID)
	assert.Equal(t, "debate-456", report.DebateID)
	assert.Equal(t, now, report.GeneratedAt)
	assert.Contains(t, report.Summary, "AI safety")
	assert.Equal(t, 2, len(report.KeyFindings))
	assert.Equal(t, 2, len(report.Recommendations))
	assert.Equal(t, 7, report.Metrics.TotalRounds)
	assert.NotNil(t, report.Appendices)
}

func TestDebateConfig_Struct(t *testing.T) {
	config := DebateConfig{
		DebateID: "debate-789",
		Topic:    "Future of AI Development",
		Participants: []ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Claude",
				Role:          "advocate",
				LLMProvider:   "anthropic",
				LLMModel:      "claude-3-opus",
				MaxRounds:     10,
				Timeout:       30 * time.Second,
				Weight:        1.0,
			},
			{
				ParticipantID: "p2",
				Name:          "DeepSeek",
				Role:          "critic",
				LLMProvider:   "deepseek",
				LLMModel:      "deepseek-chat",
				MaxRounds:     10,
				Timeout:       30 * time.Second,
				Weight:        0.9,
			},
		},
		MaxRounds:    10,
		Timeout:      5 * time.Minute,
		Strategy:     "round_robin",
		EnableCognee: true,
		Metadata: map[string]any{
			"created_by": "user-123",
		},
	}

	assert.Equal(t, "debate-789", config.DebateID)
	assert.Equal(t, "Future of AI Development", config.Topic)
	assert.Equal(t, 2, len(config.Participants))
	assert.Equal(t, 10, config.MaxRounds)
	assert.Equal(t, 5*time.Minute, config.Timeout)
	assert.Equal(t, "round_robin", config.Strategy)
	assert.True(t, config.EnableCognee)
	assert.NotNil(t, config.Metadata)
}

func TestParticipantConfig_Struct(t *testing.T) {
	config := ParticipantConfig{
		ParticipantID: "participant-001",
		Name:          "Qwen",
		Role:          "moderator",
		LLMProvider:   "qwen",
		LLMModel:      "qwen-max",
		MaxRounds:     15,
		Timeout:       45 * time.Second,
		Weight:        1.2,
	}

	assert.Equal(t, "participant-001", config.ParticipantID)
	assert.Equal(t, "Qwen", config.Name)
	assert.Equal(t, "moderator", config.Role)
	assert.Equal(t, "qwen", config.LLMProvider)
	assert.Equal(t, "qwen-max", config.LLMModel)
	assert.Equal(t, 15, config.MaxRounds)
	assert.Equal(t, 45*time.Second, config.Timeout)
	assert.Equal(t, 1.2, config.Weight)
}

func TestDebateStatus_WithMultipleParticipants(t *testing.T) {
	status := DebateStatus{
		DebateID:     "multi-participant-debate",
		Status:       "active",
		CurrentRound: 3,
		TotalRounds:  10,
		StartTime:    time.Now(),
		Participants: []ParticipantStatus{
			{ParticipantID: "p1", ParticipantName: "Claude", Status: "completed"},
			{ParticipantID: "p2", ParticipantName: "DeepSeek", Status: "responding"},
			{ParticipantID: "p3", ParticipantName: "Gemini", Status: "waiting"},
			{ParticipantID: "p4", ParticipantName: "Qwen", Status: "waiting"},
		},
	}

	assert.Equal(t, 4, len(status.Participants))

	// Count participants by status
	statusCounts := make(map[string]int)
	for _, p := range status.Participants {
		statusCounts[p.Status]++
	}

	assert.Equal(t, 1, statusCounts["completed"])
	assert.Equal(t, 1, statusCounts["responding"])
	assert.Equal(t, 2, statusCounts["waiting"])
}

func TestDebateConfig_MinimalConfig(t *testing.T) {
	config := DebateConfig{
		DebateID: "minimal-debate",
		Topic:    "Simple Topic",
		Participants: []ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Solo",
			},
		},
	}

	assert.Equal(t, "minimal-debate", config.DebateID)
	assert.Equal(t, 1, len(config.Participants))
	assert.Equal(t, 0, config.MaxRounds)
	assert.Equal(t, time.Duration(0), config.Timeout)
	assert.False(t, config.EnableCognee)
	assert.Nil(t, config.Metadata)
}

func TestPerformanceMetrics_ZeroValues(t *testing.T) {
	metrics := PerformanceMetrics{}

	assert.Equal(t, time.Duration(0), metrics.Duration)
	assert.Equal(t, 0, metrics.TotalRounds)
	assert.Equal(t, 0.0, metrics.QualityScore)
	assert.Equal(t, 0.0, metrics.Throughput)
	assert.Equal(t, time.Duration(0), metrics.Latency)
	assert.Equal(t, 0.0, metrics.ErrorRate)
	assert.Equal(t, 0.0, metrics.ResourceUsage.CPU)
	assert.Equal(t, uint64(0), metrics.ResourceUsage.Memory)
	assert.Equal(t, uint64(0), metrics.ResourceUsage.Network)
}
