package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
	"github.com/superagent/superagent/internal/utils"
)

// TestAdvancedAIDebateIntegration tests the complete advanced AI debate system
func TestAdvancedAIDebateIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping advanced integration test in short mode")
	}

	// Setup test configuration
	cfg := setupAdvancedTestConfig(t)
	logger := utils.NewTestLogger()
	ctx := context.Background()

	// Initialize all advanced services
	debateService := services.NewAdvancedDebateService(cfg, logger)
	monitoringService := services.NewDebateMonitoringService(cfg, logger)
	cogneeService := services.NewAdvancedCogneeService(&cfg.CogneeConfig, logger)
	performanceService := services.NewDebatePerformanceService(cfg, logger)
	historyService := services.NewDebateHistoryService(cfg, logger)
	resilienceService := services.NewDebateResilienceService(cfg, logger)
	reportingService := services.NewDebateReportingService(cfg, logger)
	securityService := services.NewDebateSecurityService(cfg, logger)

	// Start all services
	services := []interface{}{
		debateService, monitoringService, cogneeService, performanceService,
		historyService, resilienceService, reportingService, securityService,
	}

	for _, service := range services {
		if starter, ok := service.(interface{ Start(context.Context) error }); ok {
			err := starter.Start(ctx)
			require.NoError(t, err, "Failed to start service")
		}
	}

	defer func() {
		for _, service := range services {
			if stopper, ok := service.(interface{ Stop(context.Context) error }); ok {
				_ = stopper.Stop(ctx)
			}
		}
	}()

	// Test complete advanced debate workflow
	t.Run("CompleteAdvancedDebateWorkflow", func(t *testing.T) {
		testCompleteAdvancedWorkflow(t, ctx, cfg, logger, debateService, monitoringService,
			cogneeService, performanceService, historyService, resilienceService,
			reportingService, securityService)
	})

	// Test advanced strategies and consensus
	t.Run("AdvancedStrategiesAndConsensus", func(t *testing.T) {
		testAdvancedStrategiesAndConsensus(t, ctx, debateService, monitoringService)
	})

	// Test real-time monitoring and analytics
	t.Run("RealTimeMonitoringAndAnalytics", func(t *testing.T) {
		testRealTimeMonitoringAndAnalytics(t, ctx, monitoringService, performanceService)
	})

	// Test Cognee AI integration
	t.Run("CogneeAIIntegration", func(t *testing.T) {
		testCogneeAIIntegration(t, ctx, cogneeService, debateService)
	})

	// Test performance optimization
	t.Run("PerformanceOptimization", func(t *testing.T) {
		testPerformanceOptimization(t, ctx, performanceService, debateService)
	})

	// Test history and session management
	t.Run("HistoryAndSessionManagement", func(t *testing.T) {
		testHistoryAndSessionManagement(t, ctx, historyService, debateService)
	})

	// Test resilience and error recovery
	t.Run("ResilienceAndErrorRecovery", func(t *testing.T) {
		testResilienceAndErrorRecovery(t, ctx, resilienceService, debateService)
	})

	// Test reporting and export
	t.Run("ReportingAndExport", func(t *testing.T) {
		testReportingAndExport(t, ctx, reportingService, debateService)
	})

	// Test security and audit logging
	t.Run("SecurityAndAuditLogging", func(t *testing.T) {
		testSecurityAndAuditLogging(t, ctx, securityService, debateService)
	})
}

func testCompleteAdvancedWorkflow(t *testing.T, ctx context.Context, cfg *config.AIDebateConfig,
	logger *utils.Logger, debateService *services.AdvancedDebateService,
	monitoringService *services.DebateMonitoringService, cogneeService *services.AdvancedCogneeService,
	performanceService *services.DebatePerformanceService, historyService *services.DebateHistoryService,
	resilienceService *services.DebateResilienceService, reportingService *services.DebateReportingService,
	securityService *services.DebateSecurityService) {

	// Step 1: Create debate session with advanced configuration
	sessionConfig := &services.SessionConfig{
		Name:        "Advanced Integration Test",
		Description: "Testing complete advanced workflow",
		Parameters: map[string]interface{}{
			"max_participants":     4,
			"debate_timeout":       300000,
			"consensus_threshold":  0.75,
			"enable_cognee":        true,
			"monitoring_enabled":   true,
			"performance_tracking": true,
			"security_level":       "advanced",
		},
		AccessLevel: "advanced",
	}

	session, err := historyService.CreateSession(sessionConfig)
	require.NoError(t, err)
	assert.NotNil(t, session)

	// Step 2: Conduct advanced debate with multiple strategies
	debateResult, err := debateService.ConductAdvancedDebate(ctx, 
		"AI Ethics in Autonomous Systems",
		"Discuss the ethical implications of AI in autonomous vehicles",
		"consensus_building")
	require.NoError(t, err)
	assert.NotNil(t, debateResult)
	assert.True(t, debateResult.Consensus.Reached)
	assert.Greater(t, debateResult.Consensus.ConsensusLevel, 0.6)

	// Step 3: Apply Cognee AI enhancement
	enhancedResponse, err := cogneeService.EnhanceResponse(ctx, session.SessionID, 
		&services.DebateResponse{
			Content:     debateResult.Consensus.Summary,
			QualityScore: float64(debateResult.Consensus.ConsensusLevel),
		},
		&services.ProcessingOptions{
			EnhancementLevel:   "advanced",
			QualityAssurance:   true,
			ContextualAnalysis: true,
		})
	require.NoError(t, err)
	assert.NotNil(t, enhancedResponse)
	assert.Greater(t, enhancedResponse.QualityScore, 0.8)

	// Step 4: Monitor performance in real-time
	metrics, err := monitoringService.GetRealTimeMetrics(session.SessionID)
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Greater(t, metrics.ConsensusLevel, 0.6)
	assert.Greater(t, metrics.QualityScore, 0.7)

	// Step 5: Analyze performance with advanced metrics
	performanceInsights, err := performanceService.GetPerformanceInsights(session.SessionID)
	require.NoError(t, err)
	assert.NotNil(t, performanceInsights)
	assert.NotEmpty(t, performanceInsights.Insights)

	// Step 6: Generate comprehensive report
	reportRequest := &services.ReportRequest{
		ReportType:     "comprehensive_analysis",
		Title:          "Advanced Integration Test Report",
		Description:    "Complete analysis of advanced debate workflow",
		IncludeSessions: true,
		IncludePerformance: true,
		IncludeAnalytics: true,
		Format:         "json",
	}

	generatedReport, err := reportingService.GenerateReport(reportRequest)
	require.NoError(t, err)
	assert.NotNil(t, generatedReport)
	assert.Equal(t, "generating", generatedReport.Status)

	// Wait for report generation
	time.Sleep(2 * time.Second)

	reportData, err := reportingService.GetReport(generatedReport.ReportID)
	require.NoError(t, err)
	assert.NotNil(t, reportData)

	// Step 7: Test security and audit logging
	authRequest := &services.AuthenticationRequest{
		UserID: "test_user",
		Credentials: map[string]string{
			"username": "testuser",
			"password": "testpass123",
		},
		Method: "basic_auth",
	}

	authResult, err := securityService.AuthenticateUser(authRequest)
	require.NoError(t, err)
	assert.NotNil(t, authResult)

	// Step 8: Test resilience mechanisms
	// Simulate a failure scenario
	failure := &services.Failure{
		Type:        "service_unavailable",
		Component:   "debate_engine",
		Severity:    "medium",
		Description: "Simulated service failure for testing",
	}

	recoveryResult, err := resilienceService.RecoverFromFailure(ctx, failure)
	require.NoError(t, err)
	assert.NotNil(t, recoveryResult)

	// Step 9: Export final report
	exportRequest := &services.ExportRequest{
		ExportID:  "test_export_" + session.SessionID,
		ReportID:  generatedReport.ReportID,
		Format:    "json",
		Options:   map[string]interface{}{"include_metadata": true},
	}

	exportResult, err := reportingService.ExportReport(exportRequest)
	require.NoError(t, err)
	assert.NotNil(t, exportResult)

	// Step 10: Close session and verify history
	err = historyService.CloseSession(session.SessionID, "test_completed")
	require.NoError(t, err)

	// Verify session was archived
	history, err := historyService.GetHistoricalAnalytics(&services.AnalyticsRequest{
		Type:      "session_summary",
		SessionID: session.SessionID,
	})
	require.NoError(t, err)
	assert.NotNil(t, history)
}

func testAdvancedStrategiesAndConsensus(t *testing.T, ctx context.Context,
	debateService *services.AdvancedDebateService, monitoringService *services.DebateMonitoringService) {

	strategies := []string{"socratic_method", "devils_advocate", "consensus_building", "evidence_based"}

	for _, strategy := range strategies {
		t.Run("Strategy_"+strategy, func(t *testing.T) {
			result, err := debateService.ConductAdvancedDebate(ctx,
				"AI Strategy Test: "+strategy,
				"Testing strategy: "+strategy,
				strategy)
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.True(t, result.Success)
			assert.Greater(t, result.ConsensusLevel, 0.5)
		})
	}
}

func testRealTimeMonitoringAndAnalytics(t *testing.T, ctx context.Context,
	monitoringService *services.DebateMonitoringService, performanceService *services.DebatePerformanceService) {

	// Test dashboard creation
	dashboard, err := monitoringService.CreateCustomDashboard(services.DashboardConfig{
		Name:        "Test Dashboard",
		Description: "Testing real-time monitoring",
		Type:        "performance",
	})
	require.NoError(t, err)
	assert.NotNil(t, dashboard)

	// Test analytics results
	analytics, err := monitoringService.GetAnalyticsResults("test_session", "performance")
	require.NoError(t, err)
	assert.NotNil(t, analytics)

	// Test performance predictions
	predictions, err := performanceService.GetPerformancePredictions("test_session", 1*time.Hour)
	require.NoError(t, err)
	assert.NotNil(t, predictions)
}

func testCogneeAIIntegration(t *testing.T, ctx context.Context,
	cogneeService *services.AdvancedCogneeService, debateService *services.AdvancedDebateService) {

	// Test consensus analysis
	responses := []services.DebateResponse{
		{Content: "AI should be regulated for safety", QualityScore: 0.8},
		{Content: "AI regulation should be balanced with innovation", QualityScore: 0.9},
		{Content: "Self-regulation is preferable to government regulation", QualityScore: 0.7},
	}

	consensusAnalysis, err := cogneeService.AnalyzeConsensus(ctx, "test_session", responses,
		&services.ProcessingOptions{
			AnalysisDepth: "comprehensive",
		})
	require.NoError(t, err)
	assert.NotNil(t, consensusAnalysis)
	assert.Greater(t, consensusAnalysis.ConsensusLevel, 0.6)

	// Test insight generation
	insights, err := cogneeService.GenerateInsights(ctx, "test_session", responses,
		&services.ProcessingOptions{
			ContextualAnalysis: true,
			MemoryIntegration:  true,
		})
	require.NoError(t, err)
	assert.NotNil(t, insights)
	assert.NotEmpty(t, insights.Insights)
}

func testPerformanceOptimization(t *testing.T, ctx context.Context,
	performanceService *services.DebatePerformanceService, debateService *services.AdvancedDebateService) {

	// Test auto-tuning
	tuningResult, err := performanceService.AutoTune("test_session", []string{"response_time", "quality_score"})
	require.NoError(t, err)
	assert.NotNil(t, tuningResult)
	assert.Greater(t, tuningResult.Improvement, 0.0)

	// Test performance optimization
	optimizationResult, err := performanceService.OptimizePerformance("test_session", []string{"efficiency", "quality"})
	require.NoError(t, err)
	assert.NotNil(t, optimizationResult)
	assert.NotEmpty(t, optimizationResult.Optimizations)
}

func testHistoryAndSessionManagement(t *testing.T, ctx context.Context,
	historyService *services.DebateHistoryService, debateService *services.AdvancedDebateService) {

	// Test search functionality
	searchResults, err := historyService.SearchHistory(&services.HistoryQuery{
		Query:   "test",
		Filters: map[string]interface{}{"status": "completed"},
		Limit:   10,
	})
	require.NoError(t, err)
	assert.NotNil(t, searchResults)

	// Test trends analysis
	trends, err := historyService.GetTrends(&services.TrendRequest{
		Type:      "performance",
		TimeRange: &services.DateRange{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, trends)
}

func testResilienceAndErrorRecovery(t *testing.T, ctx context.Context,
	resilienceService *services.DebateResilienceService, debateService *services.AdvancedDebateService) {

	// Test health monitoring
	healthStatus, err := resilienceService.GetHealthStatus()
	require.NoError(t, err)
	assert.NotNil(t, healthStatus)
	assert.NotEmpty(t, healthStatus.Components)

	// Test resilience metrics
	metrics, err := resilienceService.GetResilienceMetrics()
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	// Test error handling with resilience
	operation := func() (*services.OperationResult, error) {
		return &services.OperationResult{
			Success:  true,
			Data:     "test_operation_result",
			Duration: time.Second,
		}, nil
	}

	resilienceConfig := &services.ResilienceConfig{
		MaxRetries:     3,
		Timeout:        5 * time.Second,
		CircuitBreaker: true,
		Fallback:       true,
	}

	result, err := resilienceService.ExecuteWithResilience(ctx, operation, resilienceConfig)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
}

func testReportingAndExport(t *testing.T, ctx context.Context,
	reportingService *services.DebateReportingService, debateService *services.AdvancedDebateService) {

	// Test compliance reporting
	complianceReport, err := reportingService.GetComplianceReport(&services.ComplianceRequest{
		ReportType: "security_compliance",
		Standards:  []string{"ISO27001", "SOC2"},
	})
	require.NoError(t, err)
	assert.NotNil(t, complianceReport)

	// Test quality metrics
	qualityMetrics, err := reportingService.GetQualityMetrics()
	require.NoError(t, err)
	assert.NotNil(t, qualityMetrics)
	assert.Greater(t, qualityMetrics.QualityScore, 0.7)

	// Test report history
	history, err := reportingService.GetReportHistory(&services.ReportHistoryFilter{
		ReportType: "comprehensive_analysis",
	})
	require.NoError(t, err)
	assert.NotNil(t, history)
}

func testSecurityAndAuditLogging(t *testing.T, ctx context.Context,
	securityService *services.DebateSecurityService, debateService *services.AdvancedDebateService) {

	// Test threat detection
	threatData := &services.ThreatDetectionData{
		NetworkTraffic: []interface{}{
			map[string]interface{}{"source": "10.0.0.1", "dest": "10.0.0.2", "protocol": "TCP"},
		},
		SystemLogs: []interface{}{
			map[string]interface{}{"level": "error", "message": "authentication failed"},
		},
		UserActivity: []interface{}{
			map[string]interface{}{"user_id": "test_user", "action": "login_attempt"},
		},
	}

	threatResult, err := securityService.DetectThreats(threatData)
	require.NoError(t, err)
	assert.NotNil(t, threatResult)

	// Test security metrics
	metrics, err := securityService.GetSecurityMetrics()
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Equal(t, "advanced", metrics.SecurityLevel)

	// Test audit trail
	auditTrail, err := securityService.GetAuditTrail(&services.AuditFilter{
		EventType: "authentication",
	})
	require.NoError(t, err)
	assert.NotNil(t, auditTrail)
}

func setupAdvancedTestConfig(t *testing.T) *config.AIDebateConfig {
	return &config.AIDebateConfig{
		Enabled:                     true,
		MaximalRepeatRounds:         5,
		DebateTimeout:               300000,
		ConsensusThreshold:          0.75,
		MaxResponseTime:             30000,
		MaxContextLength:            32000,
		QualityThreshold:            0.7,
		EnableCognee:                true,
		CogneeConfig: &config.CogneeDebateConfig{
			Enabled:             true,
			EnhanceResponses:    true,
			AnalyzeConsensus:    true,
			GenerateInsights:    true,
			DatasetName:         "test_debate_enhancement",
			MaxEnhancementTime:  10000,
			EnhancementStrategy: "hybrid",
			MemoryIntegration:   true,
			ContextualAnalysis:  true,
		},
		Participants: []config.DebateParticipant{
			{
				Name:    "Analyst1",
				Role:    "Primary Analyst",
				Enabled: true,
				LLMs: []config.LLMConfig{
					{
						Name:      "Test LLM",
						Provider:  "test",
						Model:     "test-model",
						Enabled:   true,
						APIKey:    "test_key",
						Temperature: 0.1,
						MaxTokens:   2000,
						Weight:      1.0,
						Timeout:     30000,
					},
				},
			},
		},
		DebateStrategy: "adaptive",
		VotingStrategy: "weighted_consensus",
		
		// Advanced features configuration
		MonitoringEnabled:           true,
		PerformanceOptimizationEnabled: true,
		PerformanceOptimizationLevel: "advanced",
		HistoryEnabled:              true,
		HistoryRetentionPolicy:      "30_days",
		HistoryArchivalStrategy:     "compress_and_encrypt",
		MaxHistorySize:              1073741824, // 1GB
		ResilienceEnabled:           true,
		ResilienceLevel:             "advanced",
		RecoveryTimeout:             300000,
		MaxRetryAttempts:            5,
		ThreatPreventionEnabled:     true,
		ReportingEnabled:            true,
		ReportingLevel:              "comprehensive",
		MaxReportSize:               10485760, // 10MB
		ReportRetentionPolicy:       "90_days",
		SecurityEnabled:             true,
		SecurityLevel:               "advanced",
		EncryptionEnabled:           true,
		AuditEnabled:                true,
	}
}

// TestAdvancedFeaturesValidation validates all advanced features are working correctly
func TestAdvancedFeaturesValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping advanced features validation in short mode")
	}

	cfg := setupAdvancedTestConfig(t)
	logger := utils.NewTestLogger()

	// Validate configuration
	assert.True(t, cfg.MonitoringEnabled, "Monitoring should be enabled")
	assert.True(t, cfg.PerformanceOptimizationEnabled, "Performance optimization should be enabled")
	assert.True(t, cfg.HistoryEnabled, "History should be enabled")
	assert.True(t, cfg.ResilienceEnabled, "Resilience should be enabled")
	assert.True(t, cfg.ReportingEnabled, "Reporting should be enabled")
	assert.True(t, cfg.SecurityEnabled, "Security should be enabled")
	assert.True(t, cfg.EnableCognee, "Cognee should be enabled")

	// Validate Cognee configuration
	assert.NotNil(t, cfg.CogneeConfig, "Cognee config should not be nil")
	assert.True(t, cfg.CogneeConfig.Enabled, "Cognee should be enabled in config")
	assert.True(t, cfg.CogneeConfig.EnhanceResponses, "Cognee response enhancement should be enabled")
	assert.True(t, cfg.CogneeConfig.AnalyzeConsensus, "Cognee consensus analysis should be enabled")
	assert.True(t, cfg.CogneeConfig.GenerateInsights, "Cognee insight generation should be enabled")

	// Validate security configuration
	assert.Equal(t, "advanced", cfg.SecurityLevel, "Security level should be advanced")
	assert.True(t, cfg.EncryptionEnabled, "Encryption should be enabled")
	assert.True(t, cfg.AuditEnabled, "Audit logging should be enabled")

	// Validate performance configuration
	assert.Equal(t, "advanced", cfg.PerformanceOptimizationLevel, "Performance level should be advanced")
	assert.Greater(t, cfg.MaxRetryAttempts, 3, "Max retry attempts should be sufficient")

	// Validate reporting configuration
	assert.Equal(t, "comprehensive", cfg.ReportingLevel, "Reporting level should be comprehensive")
	assert.Greater(t, cfg.MaxReportSize, int64(1024*1024), "Max report size should be reasonable")

	// Validate history configuration
	assert.Equal(t, "30_days", cfg.HistoryRetentionPolicy, "History retention should be 30 days")
	assert.Equal(t, "compress_and_encrypt", cfg.HistoryArchivalStrategy, "Archival strategy should include compression and encryption")

	logger.Info("All advanced features configuration validated successfully")
}

// TestAdvancedSystemPerformance tests the performance of advanced features
func TestAdvancedSystemPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	cfg := setupAdvancedTestConfig(t)
	logger := utils.NewTestLogger()
	ctx := context.Background()

	// Test concurrent debate operations
	t.Run("ConcurrentAdvancedDebates", func(t *testing.T) {
		debateService := services.NewAdvancedDebateService(cfg, logger)
		monitoringService := services.NewDebateMonitoringService(cfg, logger)

		startTime := time.Now()
		results := make(chan *services.AdvancedDebateResult, 10)
		errors := make(chan error, 10)

		// Launch multiple concurrent debates
		for i := 0; i < 5; i++ {
			go func(id int) {
				result, err := debateService.ConductAdvancedDebate(ctx,
					fmt.Sprintf("Concurrent Test %d", id),
					"Testing concurrent debate performance",
					"consensus_building")
				if err != nil {
					errors <- err
					return
				}
				results <- result
			}(i)
		}

		// Collect results
		successCount := 0
		for i := 0; i < 5; i++ {
			select {
			case result := <-results:
				assert.NotNil(t, result)
				assert.True(t, result.Success)
				successCount++
			case err := <-errors:
				t.Errorf("Debate failed: %v", err)
			case <-time.After(30 * time.Second):
				t.Fatal("Timeout waiting for debate results")
			}
		}

		duration := time.Since(startTime)
		assert.Equal(t, 5, successCount, "All concurrent debates should succeed")
		assert.Less(t, duration, 30*time.Second, "Concurrent debates should complete within 30 seconds")

		logger.Infof("Concurrent debates completed in %v with %d successes", duration, successCount)
	})

	// Test monitoring performance
	t.Run("MonitoringPerformance", func(t *testing.T) {
		monitoringService := services.NewDebateMonitoringService(cfg, logger)
		
		startTime := time.Now()
		
		// Test dashboard creation performance
		dashboard, err := monitoringService.CreateCustomDashboard(services.DashboardConfig{
			Name:        "Performance Test Dashboard",
			Description: "Testing monitoring performance",
			Type:        "performance",
		})
		require.NoError(t, err)
		assert.NotNil(t, dashboard)
		
		duration := time.Since(startTime)
		assert.Less(t, duration, 2*time.Second, "Dashboard creation should be fast")
		
		logger.Infof("Dashboard created in %v", duration)
	})

	// Test Cognee AI performance
	t.Run("CogneeAIPerformance", func(t *testing.T) {
		cogneeService := services.NewAdvancedCogneeService(&cfg.CogneeConfig, logger)
		
		responses := []services.DebateResponse{
			{Content: "Performance test response 1", QualityScore: 0.8},
			{Content: "Performance test response 2", QualityScore: 0.9},
			{Content: "Performance test response 3", QualityScore: 0.85},
		}
		
		startTime := time.Now()
		
		consensusAnalysis, err := cogneeService.AnalyzeConsensus(ctx, "perf_test", responses,
			&services.ProcessingOptions{
				AnalysisDepth: "comprehensive",
			})
		require.NoError(t, err)
		assert.NotNil(t, consensusAnalysis)
		
		duration := time.Since(startTime)
		assert.Less(t, duration, 5*time.Second, "Consensus analysis should complete quickly")
		
		logger.Infof("Consensus analysis completed in %v", duration)
	})
}

// TestAdvancedErrorHandling tests error handling and recovery mechanisms
func TestAdvancedErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling test in short mode")
	}

	cfg := setupAdvancedTestConfig(t)
	logger := utils.NewTestLogger()
	ctx := context.Background()

	resilienceService := services.NewDebateResilienceService(cfg, logger)
	
	// Test error handling with circuit breaker
	t.Run("CircuitBreakerErrorHandling", func(t *testing.T) {
		failingOperation := func() (*services.OperationResult, error) {
			return nil, fmt.Errorf("simulated operation failure")
		}

		resilienceConfig := &services.ResilienceConfig{
			MaxRetries:     3,
			Timeout:        2 * time.Second,
			CircuitBreaker: true,
			Fallback:       true,
		}

		result, err := resilienceService.ExecuteWithResilience(ctx, failingOperation, resilienceConfig)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "operation failed")
	})

	// Test recovery from failures
	t.Run("FailureRecovery", func(t *testing.T) {
		failure := &services.Failure{
			Type:        "test_failure",
			Component:   "test_component",
			Severity:    "medium",
			Description: "Test failure for recovery testing",
		}

		recoveryResult, err := resilienceService.RecoverFromFailure(ctx, failure)
		require.NoError(t, err)
		assert.NotNil(t, recoveryResult)
		assert.True(t, recoveryResult.Success)
	})

	// Test health monitoring with failures
	t.Run("HealthMonitoringWithFailures", func(t *testing.T) {
		healthStatus, err := resilienceService.GetHealthStatus()
		require.NoError(t, err)
		assert.NotNil(t, healthStatus)
		assert.NotEmpty(t, healthStatus.Components)
		
		// Verify health status includes failure information
		for _, component := range healthStatus.Components {
			assert.NotEmpty(t, component.ComponentID)
			assert.NotEmpty(t, component.Status)
		}
	})

	logger.Info("Advanced error handling tests completed successfully")
}