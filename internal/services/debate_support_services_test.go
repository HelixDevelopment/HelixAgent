package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDebateTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// DebateMonitoringService Tests

func TestNewDebateMonitoringService(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateMonitoringService(log)

	require.NotNil(t, service)
	assert.NotNil(t, service.logger)
}

func TestDebateMonitoringService_StartMonitoring(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateMonitoringService(log)
	ctx := context.Background()

	config := &DebateConfig{
		DebateID: "debate-123",
		Topic:    "Test Topic",
		MaxRounds: 3,
	}

	monitoringID, err := service.StartMonitoring(ctx, config)
	require.NoError(t, err)
	assert.NotEmpty(t, monitoringID)
	assert.Contains(t, monitoringID, "monitoring-")
}

func TestDebateMonitoringService_StopMonitoring(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateMonitoringService(log)
	ctx := context.Background()

	err := service.StopMonitoring(ctx, "monitoring-123")
	assert.NoError(t, err)
}

func TestDebateMonitoringService_GetStatus(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateMonitoringService(log)
	ctx := context.Background()

	status, err := service.GetStatus(ctx, "debate-123")
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "debate-123", status.DebateID)
	assert.Equal(t, "completed", status.Status)
	assert.Equal(t, 3, status.CurrentRound)
	assert.Equal(t, 3, status.TotalRounds)
	assert.NotEmpty(t, status.Participants)
}

// DebatePerformanceService Tests

func TestNewDebatePerformanceService(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebatePerformanceService(log)

	require.NotNil(t, service)
	assert.NotNil(t, service.logger)
}

func TestDebatePerformanceService_CalculateMetrics(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebatePerformanceService(log)

	result := &DebateResult{
		DebateID:     "debate-123",
		Duration:     5 * time.Minute,
		TotalRounds:  3,
		QualityScore: 0.85,
	}

	metrics := service.CalculateMetrics(result)
	require.NotNil(t, metrics)
	assert.Equal(t, 5*time.Minute, metrics.Duration)
	assert.Equal(t, 3, metrics.TotalRounds)
	assert.Equal(t, 0.85, metrics.QualityScore)
	assert.Greater(t, metrics.Throughput, 0.0)
}

func TestDebatePerformanceService_RecordMetrics(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebatePerformanceService(log)
	ctx := context.Background()

	metrics := &PerformanceMetrics{
		Duration:     5 * time.Minute,
		TotalRounds:  3,
		QualityScore: 0.85,
	}

	err := service.RecordMetrics(ctx, metrics)
	assert.NoError(t, err)
}

func TestDebatePerformanceService_GetMetrics(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebatePerformanceService(log)
	ctx := context.Background()

	timeRange := TimeRange{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
	}

	metrics, err := service.GetMetrics(ctx, timeRange)
	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.Equal(t, 3, metrics.TotalRounds)
	assert.Equal(t, 0.85, metrics.QualityScore)
}

// DebateHistoryService Tests

func TestNewDebateHistoryService(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateHistoryService(log)

	require.NotNil(t, service)
	assert.NotNil(t, service.logger)
}

func TestDebateHistoryService_SaveDebateResult(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateHistoryService(log)
	ctx := context.Background()

	result := &DebateResult{
		DebateID:     "debate-123",
		TotalRounds:  3,
		QualityScore: 0.85,
	}

	err := service.SaveDebateResult(ctx, result)
	assert.NoError(t, err)
}

func TestDebateHistoryService_QueryHistory(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateHistoryService(log)
	ctx := context.Background()

	filters := &HistoryFilters{
		Limit:  10,
		Offset: 0,
	}

	results, err := service.QueryHistory(ctx, filters)
	require.NoError(t, err)
	require.NotNil(t, results)
	// Currently returns empty slice
	assert.Len(t, results, 0)
}

// DebateResilienceService Tests

func TestNewDebateResilienceService(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateResilienceService(log)

	require.NotNil(t, service)
	assert.NotNil(t, service.logger)
}

func TestDebateResilienceService_HandleFailure(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateResilienceService(log)
	ctx := context.Background()

	err := service.HandleFailure(ctx, errors.New("test error"))
	assert.NoError(t, err)
}

func TestDebateResilienceService_RecoverDebate(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateResilienceService(log)
	ctx := context.Background()

	result, err := service.RecoverDebate(ctx, "debate-123")
	assert.NoError(t, err)
	// Currently returns nil
	assert.Nil(t, result)
}

// DebateReportingService Tests

func TestNewDebateReportingService(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateReportingService(log)

	require.NotNil(t, service)
	assert.NotNil(t, service.logger)
}

func TestDebateReportingService_GenerateReport(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateReportingService(log)
	ctx := context.Background()

	result := &DebateResult{
		DebateID:     "debate-123",
		Duration:     5 * time.Minute,
		TotalRounds:  3,
		QualityScore: 0.85,
	}

	report, err := service.GenerateReport(ctx, result)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "report-debate-123", report.ReportID)
	assert.Equal(t, "debate-123", report.DebateID)
	assert.NotEmpty(t, report.Summary)
	assert.NotEmpty(t, report.KeyFindings)
	assert.NotEmpty(t, report.Recommendations)
}

func TestDebateReportingService_ExportReport(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateReportingService(log)
	ctx := context.Background()

	data, err := service.ExportReport(ctx, "report-123", "json")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// DebateSecurityService Tests

func TestNewDebateSecurityService(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateSecurityService(log)

	require.NotNil(t, service)
	assert.NotNil(t, service.logger)
}

func TestDebateSecurityService_ValidateDebateRequest(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateSecurityService(log)
	ctx := context.Background()

	config := &DebateConfig{
		DebateID: "debate-123",
		Topic:    "Test Topic",
		MaxRounds: 3,
	}

	err := service.ValidateDebateRequest(ctx, config)
	assert.NoError(t, err)
}

func TestDebateSecurityService_SanitizeResponse(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateSecurityService(log)
	ctx := context.Background()

	response := "Test response with <script>evil()</script>"
	sanitized, err := service.SanitizeResponse(ctx, response)
	require.NoError(t, err)
	assert.Equal(t, response, sanitized) // Current impl just returns the same
}

func TestDebateSecurityService_AuditDebate(t *testing.T) {
	log := newDebateTestLogger()
	service := NewDebateSecurityService(log)
	ctx := context.Background()

	err := service.AuditDebate(ctx, "debate-123")
	assert.NoError(t, err)
}

// AdvancedDebateService Tests

func TestNewAdvancedDebateService(t *testing.T) {
	log := newDebateTestLogger()

	monitoringService := NewDebateMonitoringService(log)
	performanceService := NewDebatePerformanceService(log)
	historyService := NewDebateHistoryService(log)
	resilienceService := NewDebateResilienceService(log)
	reportingService := NewDebateReportingService(log)
	securityService := NewDebateSecurityService(log)

	service := NewAdvancedDebateService(
		nil, // debateService - can be nil for constructor test
		monitoringService,
		performanceService,
		historyService,
		resilienceService,
		reportingService,
		securityService,
		log,
	)

	require.NotNil(t, service)
	assert.NotNil(t, service.monitoringService)
	assert.NotNil(t, service.performanceService)
	assert.NotNil(t, service.historyService)
	assert.NotNil(t, service.resilienceService)
	assert.NotNil(t, service.reportingService)
	assert.NotNil(t, service.securityService)
	assert.NotNil(t, service.logger)
}

func TestAdvancedDebateService_GetDebateStatus(t *testing.T) {
	log := newDebateTestLogger()

	monitoringService := NewDebateMonitoringService(log)
	performanceService := NewDebatePerformanceService(log)
	historyService := NewDebateHistoryService(log)
	resilienceService := NewDebateResilienceService(log)
	reportingService := NewDebateReportingService(log)
	securityService := NewDebateSecurityService(log)

	service := NewAdvancedDebateService(
		nil,
		monitoringService,
		performanceService,
		historyService,
		resilienceService,
		reportingService,
		securityService,
		log,
	)

	ctx := context.Background()
	status, err := service.GetDebateStatus(ctx, "debate-123")
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "debate-123", status.DebateID)
}

func TestAdvancedDebateService_GetDebateHistory(t *testing.T) {
	log := newDebateTestLogger()

	monitoringService := NewDebateMonitoringService(log)
	performanceService := NewDebatePerformanceService(log)
	historyService := NewDebateHistoryService(log)
	resilienceService := NewDebateResilienceService(log)
	reportingService := NewDebateReportingService(log)
	securityService := NewDebateSecurityService(log)

	service := NewAdvancedDebateService(
		nil,
		monitoringService,
		performanceService,
		historyService,
		resilienceService,
		reportingService,
		securityService,
		log,
	)

	ctx := context.Background()
	filters := &HistoryFilters{Limit: 10}
	results, err := service.GetDebateHistory(ctx, filters)
	require.NoError(t, err)
	require.NotNil(t, results)
}

func TestAdvancedDebateService_GetPerformanceMetrics(t *testing.T) {
	log := newDebateTestLogger()

	monitoringService := NewDebateMonitoringService(log)
	performanceService := NewDebatePerformanceService(log)
	historyService := NewDebateHistoryService(log)
	resilienceService := NewDebateResilienceService(log)
	reportingService := NewDebateReportingService(log)
	securityService := NewDebateSecurityService(log)

	service := NewAdvancedDebateService(
		nil,
		monitoringService,
		performanceService,
		historyService,
		resilienceService,
		reportingService,
		securityService,
		log,
	)

	ctx := context.Background()
	timeRange := TimeRange{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
	}
	metrics, err := service.GetPerformanceMetrics(ctx, timeRange)
	require.NoError(t, err)
	require.NotNil(t, metrics)
}

// Debate Support Types Tests

func TestDebateStatus_Structure(t *testing.T) {
	now := time.Now()
	status := DebateStatus{
		DebateID:         "debate-123",
		Status:           "in_progress",
		CurrentRound:     2,
		TotalRounds:      5,
		StartTime:        now,
		EstimatedEndTime: now.Add(10 * time.Minute),
		Participants: []ParticipantStatus{
			{ParticipantID: "p1", ParticipantName: "Alice", Status: "active"},
		},
		Errors:   []string{},
		Metadata: map[string]any{"key": "value"},
	}

	assert.Equal(t, "debate-123", status.DebateID)
	assert.Equal(t, "in_progress", status.Status)
	assert.Equal(t, 2, status.CurrentRound)
	assert.Equal(t, 5, status.TotalRounds)
	assert.Len(t, status.Participants, 1)
}

func TestParticipantStatus_Structure(t *testing.T) {
	status := ParticipantStatus{
		ParticipantID:   "p1",
		ParticipantName: "Alice",
		Status:          "active",
		ResponseTime:    5 * time.Second,
		Error:           "",
	}

	assert.Equal(t, "p1", status.ParticipantID)
	assert.Equal(t, "Alice", status.ParticipantName)
	assert.Equal(t, "active", status.Status)
	assert.Empty(t, status.Error)
}

func TestHistoryFilters_Structure(t *testing.T) {
	now := time.Now()
	minScore := 0.5
	maxScore := 1.0

	filters := HistoryFilters{
		StartTime:       &now,
		EndTime:         &now,
		ParticipantIDs:  []string{"p1", "p2"},
		MinQualityScore: &minScore,
		MaxQualityScore: &maxScore,
		Limit:           100,
		Offset:          0,
	}

	assert.NotNil(t, filters.StartTime)
	assert.Len(t, filters.ParticipantIDs, 2)
	assert.Equal(t, 0.5, *filters.MinQualityScore)
	assert.Equal(t, 100, filters.Limit)
}

func TestTimeRange_Structure(t *testing.T) {
	now := time.Now()
	tr := TimeRange{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   now,
	}

	assert.True(t, tr.EndTime.After(tr.StartTime))
}

func TestDebateReport_Structure(t *testing.T) {
	report := DebateReport{
		ReportID:        "report-123",
		DebateID:        "debate-123",
		GeneratedAt:     time.Now(),
		Summary:         "Summary of debate",
		KeyFindings:     []string{"Finding 1", "Finding 2"},
		Recommendations: []string{"Rec 1"},
		Metrics: PerformanceMetrics{
			TotalRounds:  3,
			QualityScore: 0.85,
		},
	}

	assert.Equal(t, "report-123", report.ReportID)
	assert.Equal(t, "debate-123", report.DebateID)
	assert.Len(t, report.KeyFindings, 2)
}

func TestPerformanceMetrics_Structure(t *testing.T) {
	metrics := PerformanceMetrics{
		Duration:     5 * time.Minute,
		TotalRounds:  3,
		QualityScore: 0.85,
		Throughput:   0.6,
		Latency:      100 * time.Millisecond,
		ErrorRate:    0.01,
		ResourceUsage: ResourceUsage{
			CPU:     0.5,
			Memory:  1024 * 1024 * 100,
			Network: 1024 * 1024 * 10,
		},
	}

	assert.Equal(t, 5*time.Minute, metrics.Duration)
	assert.Equal(t, 3, metrics.TotalRounds)
	assert.Equal(t, 0.85, metrics.QualityScore)
	assert.Equal(t, 0.5, metrics.ResourceUsage.CPU)
}

func TestResourceUsage_Structure(t *testing.T) {
	usage := ResourceUsage{
		CPU:     0.75,
		Memory:  1024 * 1024 * 500, // 500MB
		Network: 1024 * 1024 * 50,  // 50MB
	}

	assert.Equal(t, 0.75, usage.CPU)
	assert.Greater(t, usage.Memory, uint64(0))
	assert.Greater(t, usage.Network, uint64(0))
}

// Benchmarks

func BenchmarkDebateMonitoringService_StartMonitoring(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	service := NewDebateMonitoringService(log)
	ctx := context.Background()
	config := &DebateConfig{DebateID: "debate-123", MaxRounds: 3}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.StartMonitoring(ctx, config)
	}
}

func BenchmarkDebatePerformanceService_CalculateMetrics(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	service := NewDebatePerformanceService(log)
	result := &DebateResult{Duration: 5 * time.Minute, TotalRounds: 3, QualityScore: 0.85}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.CalculateMetrics(result)
	}
}

func BenchmarkDebateReportingService_GenerateReport(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	service := NewDebateReportingService(log)
	ctx := context.Background()
	result := &DebateResult{DebateID: "debate-123", Duration: 5 * time.Minute, TotalRounds: 3}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GenerateReport(ctx, result)
	}
}
