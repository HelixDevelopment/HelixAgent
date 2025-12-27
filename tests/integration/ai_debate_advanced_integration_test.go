package integration

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/services"
)

// TestAdvancedAIDebateIntegration tests the complete advanced AI debate system
func TestAdvancedAIDebateIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping advanced integration test in short mode")
	}

	// Setup test configuration
	cfg := setupAdvancedTestConfig(t)
	logger := logrus.New()
	ctx := context.Background()

	// Initialize all advanced services
	debateService := services.NewDebateService(logger)
	monitoringService := services.NewDebateMonitoringService(logger)
	performanceService := services.NewDebatePerformanceService(logger)
	historyService := services.NewDebateHistoryService(logger)
	resilienceService := services.NewDebateResilienceService(logger)
	reportingService := services.NewDebateReportingService(logger)
	securityService := services.NewDebateSecurityService(logger)

	advancedDebateService := services.NewAdvancedDebateService(
		debateService,
		monitoringService,
		performanceService,
		historyService,
		resilienceService,
		reportingService,
		securityService,
		logger,
	)

	// Test complete advanced debate workflow
	t.Run("CompleteAdvancedDebateWorkflow", func(t *testing.T) {
		testCompleteAdvancedWorkflow(t, ctx, advancedDebateService, cfg)
	})

	// Test advanced strategies and consensus
	t.Run("AdvancedStrategiesAndConsensus", func(t *testing.T) {
		testAdvancedStrategiesAndConsensus(t, ctx, advancedDebateService, cfg)
	})

	// Test real-time monitoring and analytics
	t.Run("RealTimeMonitoringAndAnalytics", func(t *testing.T) {
		testRealTimeMonitoringAndAnalytics(t, ctx, monitoringService, performanceService)
	})

	// Test performance optimization
	t.Run("PerformanceOptimization", func(t *testing.T) {
		testPerformanceOptimization(t, ctx, performanceService)
	})

	// Test history and session management
	t.Run("HistoryAndSessionManagement", func(t *testing.T) {
		testHistoryAndSessionManagement(t, ctx, historyService)
	})

	// Test resilience and error recovery
	t.Run("ResilienceAndErrorRecovery", func(t *testing.T) {
		testResilienceAndErrorRecovery(t, ctx, resilienceService)
	})

	// Test reporting and export
	t.Run("ReportingAndExport", func(t *testing.T) {
		testReportingAndExport(t, ctx, reportingService)
	})

	// Test security and audit logging
	t.Run("SecurityAndAuditLogging", func(t *testing.T) {
		testSecurityAndAuditLogging(t, ctx, securityService)
	})
}

func testCompleteAdvancedWorkflow(t *testing.T, ctx context.Context, advancedService *services.AdvancedDebateService, cfg *config.AIDebateConfig) {
	// Create debate configuration
	debateConfig := &services.DebateConfig{
		DebateID: "integration-advanced-001",
		Topic:    "Test advanced debate workflow",
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Alice",
				Role:          "proponent",
				LLMProvider:   "claude",
				LLMModel:      "claude-3-opus-20240229",
				MaxRounds:     3,
				Timeout:       30 * time.Second,
				Weight:        1.0,
			},
			{
				ParticipantID: "participant-2",
				Name:          "Bob",
				Role:          "opponent",
				LLMProvider:   "deepseek",
				LLMModel:      "deepseek-chat",
				MaxRounds:     3,
				Timeout:       30 * time.Second,
				Weight:        1.0,
			},
		},
		MaxRounds:    3,
		Timeout:      5 * time.Minute,
		Strategy:     "structured",
		EnableCognee: true,
	}

	// Conduct advanced debate
	result, err := advancedService.ConductAdvancedDebate(ctx, debateConfig)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, debateConfig.DebateID, result.DebateID)
	assert.True(t, result.Success)
	assert.Greater(t, result.TotalRounds, 0)
	assert.Len(t, result.Participants, 2)
	assert.NotNil(t, result.Consensus)
	assert.NotNil(t, result.CogneeInsights)
}

func testAdvancedStrategiesAndConsensus(t *testing.T, ctx context.Context, advancedService *services.AdvancedDebateService, cfg *config.AIDebateConfig) {
	// Test different debate strategies
	strategies := []string{"structured", "socratic", "round_robin"}

	for _, strategy := range strategies {
		debateConfig := &services.DebateConfig{
			DebateID: "integration-strategy-" + strategy,
			Topic:    "Testing strategy: " + strategy,
			Participants: []services.ParticipantConfig{
				{
					ParticipantID: "participant-1",
					Name:          "Alice",
					Role:          "proponent",
					LLMProvider:   "claude",
					LLMModel:      "claude-3-opus-20240229",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
				{
					ParticipantID: "participant-2",
					Name:          "Bob",
					Role:          "opponent",
					LLMProvider:   "deepseek",
					LLMModel:      "deepseek-chat",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
			},
			MaxRounds:    2,
			Timeout:      3 * time.Minute,
			Strategy:     strategy,
			EnableCognee: true,
		}

		result, err := advancedService.ConductAdvancedDebate(ctx, debateConfig)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, strategy, debateConfig.Strategy)
	}
}

func testRealTimeMonitoringAndAnalytics(t *testing.T, ctx context.Context, monitoringService *services.DebateMonitoringService, performanceService *services.DebatePerformanceService) {
	// Test monitoring service
	debateID := "integration-monitoring-001"
	status, err := monitoringService.GetStatus(ctx, debateID)
	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, debateID, status.DebateID)

	// Test performance metrics
	timeRange := services.TimeRange{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
	}
	metrics, err := performanceService.GetMetrics(ctx, timeRange)
	require.NoError(t, err)
	assert.NotNil(t, metrics)
}

func testPerformanceOptimization(t *testing.T, ctx context.Context, performanceService *services.DebatePerformanceService) {
	// Test performance metrics calculation
	result := &services.DebateResult{
		DebateID:     "integration-performance-001",
		TotalRounds:  3,
		Duration:     2 * time.Minute,
		QualityScore: 0.85,
	}

	metrics := performanceService.CalculateMetrics(result)
	assert.NotNil(t, metrics)
	assert.Equal(t, result.Duration, metrics.Duration)
	assert.Equal(t, result.TotalRounds, metrics.TotalRounds)
	assert.Equal(t, result.QualityScore, metrics.QualityScore)
}

func testHistoryAndSessionManagement(t *testing.T, ctx context.Context, historyService *services.DebateHistoryService) {
	// Test history service
	filters := &services.HistoryFilters{
		StartTime: &[]time.Time{time.Now().Add(-24 * time.Hour)}[0],
		EndTime:   &[]time.Time{time.Now()}[0],
		Limit:     10,
	}

	history, err := historyService.QueryHistory(ctx, filters)
	require.NoError(t, err)
	assert.NotNil(t, history)
	assert.Len(t, history, 0) // No history yet
}

func testResilienceAndErrorRecovery(t *testing.T, ctx context.Context, resilienceService *services.DebateResilienceService) {
	// Test resilience service
	err := resilienceService.HandleFailure(ctx, assert.AnError)
	assert.NoError(t, err)
}

func testReportingAndExport(t *testing.T, ctx context.Context, reportingService *services.DebateReportingService) {
	// Test reporting service
	result := &services.DebateResult{
		DebateID:     "integration-reporting-001",
		TotalRounds:  3,
		Duration:     2 * time.Minute,
		QualityScore: 0.85,
	}

	report, err := reportingService.GenerateReport(ctx, result)
	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, result.DebateID, report.DebateID)

	// Test report export
	export, err := reportingService.ExportReport(ctx, report.ReportID, "json")
	require.NoError(t, err)
	assert.NotEmpty(t, export)
}

func testSecurityAndAuditLogging(t *testing.T, ctx context.Context, securityService *services.DebateSecurityService) {
	// Test security service
	config := &services.DebateConfig{
		DebateID: "integration-security-001",
		Topic:    "Test security",
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "participant-1",
				Name:          "Alice",
				Role:          "proponent",
				LLMProvider:   "claude",
				LLMModel:      "claude-3-opus-20240229",
			},
		},
		MaxRounds:    1,
		Timeout:      30 * time.Second,
		Strategy:     "structured",
		EnableCognee: false,
	}

	err := securityService.ValidateDebateRequest(ctx, config)
	assert.NoError(t, err)

	// Test audit logging
	err = securityService.AuditDebate(ctx, config.DebateID)
	assert.NoError(t, err)
}

// setupAdvancedTestConfig creates a test configuration for advanced features
func setupAdvancedTestConfig(t *testing.T) *config.AIDebateConfig {
	return &config.AIDebateConfig{
		Enabled:             true,
		MaximalRepeatRounds: 5,
		DebateTimeout:       1800000, // 30 minutes in milliseconds
		ConsensusThreshold:  0.75,
		EnableCognee:        true,
		CogneeConfig: &config.CogneeDebateConfig{
			Enabled:             true,
			EnhanceResponses:    true,
			AnalyzeConsensus:    true,
			GenerateInsights:    true,
			DatasetName:         "test-dataset",
			MaxEnhancementTime:  60000, // 1 minute
			EnhancementStrategy: "comprehensive",
			MemoryIntegration:   true,
		},
		Participants: []config.DebateParticipant{
			{
				Name: "Alice",
				Role: "proponent",
				LLMs: []config.LLMConfiguration{
					{
						Name:     "claude-main",
						Provider: "claude",
						Model:    "claude-3-opus-20240229",
						Enabled:  true,
					},
				},
				Enabled: true,
				Weight:  1.0,
			},
			{
				Name: "Bob",
				Role: "opponent",
				LLMs: []config.LLMConfiguration{
					{
						Name:     "deepseek-main",
						Provider: "deepseek",
						Model:    "deepseek-chat",
						Enabled:  true,
					},
				},
				Enabled: true,
				Weight:  1.0,
			},
		},
		DebateStrategy:      "structured",
		VotingStrategy:      "weighted",
		ResponseFormat:      "text",
		EnableMemory:        true,
		MemoryRetention:     3600000, // 1 hour
		MaxContextLength:    8000,
		QualityThreshold:    0.8,
		MaxResponseTime:     30000, // 30 seconds
		EnableStreaming:     true,
		EnableDebateLogging: true,
		LogDebateDetails:    true,
		MetricsEnabled:      true,
	}
}
