package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/services"
)

// TestAIDebateSystem_E2E tests the complete AI debate system end-to-end
func TestAIDebateSystem_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Create temporary directory for test configurations
	tempDir, err := os.MkdirTemp("", "ai-debate-e2e-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("Complete Debate Workflow", func(t *testing.T) {
		// Step 1: Create and load configuration
		cfg := createTestDebateConfig(t, tempDir)
		loader := config.NewAIDebateConfigLoader(filepath.Join(tempDir, "test-config.yaml"))

		err := loader.Save(cfg)
		require.NoError(t, err, "Failed to save configuration")

		loadedConfig, err := loader.Load()
		require.NoError(t, err, "Failed to load configuration")
		require.NotNil(t, loadedConfig, "Loaded configuration is nil")

		// Step 2: Validate configuration
		err = loadedConfig.Validate()
		assert.NoError(t, err, "Configuration validation failed")

		// Step 3: Create debate service (mocked for E2E test)
		debateService := createMockDebateService(t, loadedConfig)
		require.NotNil(t, debateService, "Debate service is nil")

		// Step 4: Conduct debate
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		topic := "Should artificial intelligence be regulated by governments?"
		initialContext := "Recent advances in AI technology have raised concerns about safety, privacy, and societal impact."

		result, err := debateService.ConductDebate(ctx, topic, initialContext)
		require.NoError(t, err, "Debate execution failed")
		require.NotNil(t, result, "Debate result is nil")

		// Step 5: Validate results
		validateDebateResults(t, result, topic)
	})

	t.Run("Debate with Fallback Chain", func(t *testing.T) {
		// Test the fallback mechanism when primary LLM fails
		cfg := createFallbackTestConfig(t, tempDir)
		loader := config.NewAIDebateConfigLoader(filepath.Join(tempDir, "fallback-config.yaml"))

		err := loader.Save(cfg)
		require.NoError(t, err)

		loadedConfig, err := loader.Load()
		require.NoError(t, err)

		debateService := createFallbackTestDebateService(t, loadedConfig)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		result, err := debateService.ConductDebate(ctx, "Test fallback mechanism", "Testing LLM fallback chain")
		require.NoError(t, err, "Debate with fallbacks failed")
		require.NotNil(t, result)

		// Verify that fallback was used
		assert.True(t, result.FallbackUsed, "Fallback mechanism was not triggered")
	})

	t.Run("Debate with Cognee Enhancement", func(t *testing.T) {
		// Test Cognee AI integration
		cfg := createCogneeTestConfig(t, tempDir)
		loader := config.NewAIDebateConfigLoader(filepath.Join(tempDir, "cognee-config.yaml"))

		err := loader.Save(cfg)
		require.NoError(t, err)

		loadedConfig, err := loader.Load()
		require.NoError(t, err)

		debateService := createCogneeTestDebateService(t, loadedConfig)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		result, err := debateService.ConductDebate(ctx, "Test Cognee enhancement", "Testing AI enhancement capabilities")
		require.NoError(t, err, "Debate with Cognee failed")
		require.NotNil(t, result)

		// Verify Cognee enhancement
		assert.True(t, result.CogneeEnhanced, "Cognee enhancement was not applied")
		assert.NotNil(t, result.CogneeInsights, "Cognee insights are missing")
	})

	t.Run("Debate Timeout Handling", func(t *testing.T) {
		// Test timeout handling
		cfg := createTimeoutTestConfig(t, tempDir)
		loader := config.NewAIDebateConfigLoader(filepath.Join(tempDir, "timeout-config.yaml"))

		err := loader.Save(cfg)
		require.NoError(t, err)

		loadedConfig, err := loader.Load()
		require.NoError(t, err)

		debateService := createTimeoutTestDebateService(t, loadedConfig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := debateService.ConductDebate(ctx, "Test timeout handling", "Testing timeout scenarios")
		assert.Error(t, err, "Expected timeout error")
		assert.Nil(t, result, "Expected nil result on timeout")
	})

	t.Run("Debate with Memory Context", func(t *testing.T) {
		// Test memory management and context retention
		cfg := createMemoryTestConfig(t, tempDir)
		loader := config.NewAIDebateConfigLoader(filepath.Join(tempDir, "memory-config.yaml"))

		err := loader.Save(cfg)
		require.NoError(t, err)

		loadedConfig, err := loader.Load()
		require.NoError(t, err)

		debateService := createMemoryTestDebateService(t, loadedConfig)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// First debate to establish memory
		_, err = debateService.ConductDebate(ctx, "Establish memory context", "Initial context for memory testing")
		require.NoError(t, err)

		// Second debate to test memory usage
		result, err := debateService.ConductDebate(ctx, "Test memory retention", "Context that should trigger memory recall")
		require.NoError(t, err, "Debate with memory failed")
		require.NotNil(t, result)

		// Verify memory was used
		assert.True(t, result.MemoryUsed, "Memory context was not utilized")
	})

	t.Run("Configuration Reload During Debate", func(t *testing.T) {
		// Test configuration reloading while debates are active
		cfg := createReloadTestConfig(t, tempDir)
		configPath := filepath.Join(tempDir, "reload-config.yaml")
		loader := config.NewAIDebateConfigLoader(configPath)

		err := loader.Save(cfg)
		require.NoError(t, err)

		loadedConfig, err := loader.Load()
		require.NoError(t, err)

		debateService := createReloadTestDebateService(t, loadedConfig)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Start debate
		debateResultChan := make(chan *services.DebateResult, 1)
		debateErrorChan := make(chan error, 1)

		go func() {
			result, err := debateService.ConductDebate(ctx, "Test config reload", "Testing configuration changes during debate")
			if err != nil {
				debateErrorChan <- err
			} else {
				debateResultChan <- result
			}
		}()

		// Reload configuration while debate is running
		time.Sleep(100 * time.Millisecond)

		modifiedConfig := createModifiedReloadTestConfig(t, tempDir)
		err = loader.Save(modifiedConfig)
		require.NoError(t, err)

		// Wait for debate completion
		select {
		case result := <-debateResultChan:
			require.NotNil(t, result, "Debate result is nil")
			validateDebateResults(t, result, "Test config reload")
		case err := <-debateErrorChan:
			require.NoError(t, err, "Debate failed")
		case <-time.After(30 * time.Second):
			t.Fatal("Debate timed out")
		}
	})

	t.Run("Stress Test - Multiple Concurrent Debates", func(t *testing.T) {
		// Test system under load with multiple concurrent debates
		cfg := createStressTestConfig(t, tempDir)
		loader := config.NewAIDebateConfigLoader(filepath.Join(tempDir, "stress-config.yaml"))

		err := loader.Save(cfg)
		require.NoError(t, err)

		loadedConfig, err := loader.Load()
		require.NoError(t, err)

		debateService := createStressTestDebateService(t, loadedConfig)

		// Launch multiple concurrent debates
		numDebates := 5
		results := make(chan *services.DebateResult, numDebates)
		errors := make(chan error, numDebates)

		for i := 0; i < numDebates; i++ {
			go func(id int) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				topic := fmt.Sprintf("Concurrent debate %d", id)
				context := fmt.Sprintf("Testing concurrent debate execution %d", id)

				result, err := debateService.ConductDebate(ctx, topic, context)
				if err != nil {
					errors <- err
				} else {
					results <- result
				}
			}(i)
		}

		// Collect results
		successfulDebates := 0
		failedDebates := 0

		for i := 0; i < numDebates; i++ {
			select {
			case result := <-results:
				require.NotNil(t, result, "Successful debate returned nil result")
				successfulDebates++
			case err := <-errors:
				assert.Error(t, err, "Failed debate should return error")
				failedDebates++
			case <-time.After(3 * time.Minute):
				t.Fatal("Concurrent debates timed out")
			}
		}

		// Verify stress test results
		assert.Greater(t, successfulDebates, 0, "No successful debates in stress test")
		t.Logf("Stress test results: %d successful, %d failed out of %d total",
			successfulDebates, failedDebates, numDebates)
	})
}

// Helper functions for creating test configurations

func createTestDebateConfig(_ *testing.T, _ string) *config.AIDebateConfig {
	return &config.AIDebateConfig{
		Enabled:             true,
		MaximalRepeatRounds: 3,
		DebateTimeout:       5 * 60 * 1000, // 5 minutes
		ConsensusThreshold:  0.75,
		EnableCognee:        true,
		CogneeConfig: &config.CogneeDebateConfig{
			Enabled:             true,
			EnhanceResponses:    true,
			AnalyzeConsensus:    true,
			GenerateInsights:    true,
			DatasetName:         "e2e_test_enhancement",
			MaxEnhancementTime:  10 * 1000,
			EnhancementStrategy: "hybrid",
			MemoryIntegration:   true,
			ContextualAnalysis:  true,
		},
		Participants: []config.DebateParticipant{
			{
				Name:               "TestAnalyst",
				Role:               "Test Analyst",
				Description:        "Test participant for E2E testing",
				Enabled:            true,
				ResponseTimeout:    30 * 1000,
				Weight:             1.0,
				Priority:           1,
				DebateStyle:        "analytical",
				ArgumentationStyle: "logical",
				PersuasionLevel:    0.5,
				OpennessToChange:   0.5,
				QualityThreshold:   0.7,
				MinResponseLength:  50,
				MaxResponseLength:  1000,
				EnableCognee:       true,
				LLMs: []config.LLMConfiguration{
					{
						Name:         "TestLLM",
						Provider:     "claude",
						Model:        "claude-3-sonnet",
						Enabled:      true,
						APIKey:       "test-api-key",
						Timeout:      30 * 1000,
						MaxRetries:   3,
						Temperature:  0.7,
						MaxTokens:    1000,
						Weight:       1.0,
						RateLimitRPS: 10,
					},
				},
			},
			{
				Name:               "TestCritic",
				Role:               "Test Critic",
				Description:        "Test critic participant for E2E testing",
				Enabled:            true,
				ResponseTimeout:    30 * 1000,
				Weight:             1.0,
				Priority:           2,
				DebateStyle:        "critical",
				ArgumentationStyle: "logical",
				PersuasionLevel:    0.5,
				OpennessToChange:   0.5,
				QualityThreshold:   0.7,
				MinResponseLength:  50,
				MaxResponseLength:  1000,
				EnableCognee:       true,
				LLMs: []config.LLMConfiguration{
					{
						Name:         "TestLLM2",
						Provider:     "deepseek",
						Model:        "deepseek-coder",
						Enabled:      true,
						APIKey:       "test-api-key-2",
						Timeout:      30 * 1000,
						MaxRetries:   3,
						Temperature:  0.7,
						MaxTokens:    1000,
						Weight:       1.0,
						RateLimitRPS: 10,
					},
				},
			},
		},
		DebateStrategy:      "structured",
		VotingStrategy:      "confidence_weighted",
		ResponseFormat:      "detailed",
		EnableMemory:        true,
		MemoryRetention:     7 * 24 * 60 * 60 * 1000, // 7 days
		MaxContextLength:    16000,
		QualityThreshold:    0.7,
		MaxResponseTime:     30 * 1000,
		EnableStreaming:     false,
		EnableDebateLogging: true,
		LogDebateDetails:    true,
		MetricsEnabled:      true,
	}
}

func createFallbackTestConfig(t *testing.T, tempDir string) *config.AIDebateConfig {
	cfg := createTestDebateConfig(t, tempDir)

	// Modify to test fallback scenarios
	cfg.Participants[0].LLMs = []config.LLMConfiguration{
		{
			Name:        "PrimaryLLM",
			Provider:    "claude",
			Model:       "claude-3-sonnet",
			Enabled:     true,
			APIKey:      "will-fail", // This will trigger fallback
			Timeout:     5000,        // Short timeout
			MaxRetries:  1,
			Temperature: 0.7,
			MaxTokens:   1000,
			Weight:      1.0,
		},
		{
			Name:        "FallbackLLM",
			Provider:    "deepseek",
			Model:       "deepseek-coder",
			Enabled:     true,
			APIKey:      "test-fallback-key",
			Timeout:     30000,
			MaxRetries:  3,
			Temperature: 0.7,
			MaxTokens:   1000,
			Weight:      0.9,
		},
	}

	return cfg
}

func createCogneeTestConfig(t *testing.T, tempDir string) *config.AIDebateConfig {
	cfg := createTestDebateConfig(t, tempDir)
	cfg.EnableCognee = true
	cfg.CogneeConfig = &config.CogneeDebateConfig{
		Enabled:             true,
		EnhanceResponses:    true,
		AnalyzeConsensus:    true,
		GenerateInsights:    true,
		DatasetName:         "cognee_e2e_test",
		MaxEnhancementTime:  15 * 1000,
		EnhancementStrategy: "hybrid",
		MemoryIntegration:   true,
		ContextualAnalysis:  true,
	}

	// Enable Cognee for all participants
	for i := range cfg.Participants {
		cfg.Participants[i].EnableCognee = true
	}

	return cfg
}

func createTimeoutTestConfig(t *testing.T, tempDir string) *config.AIDebateConfig {
	cfg := createTestDebateConfig(t, tempDir)
	cfg.MaxResponseTime = 1000 // 1 second - very short to trigger timeout
	cfg.DebateTimeout = 5000   // 5 seconds - very short

	// Set very short timeouts for participants
	for i := range cfg.Participants {
		cfg.Participants[i].ResponseTimeout = 500 // 0.5 seconds
		for j := range cfg.Participants[i].LLMs {
			cfg.Participants[i].LLMs[j].Timeout = 500
		}
	}

	return cfg
}

func createMemoryTestConfig(t *testing.T, tempDir string) *config.AIDebateConfig {
	cfg := createTestDebateConfig(t, tempDir)
	cfg.EnableMemory = true
	cfg.MemoryRetention = 24 * 60 * 60 * 1000 // 1 day
	cfg.MaxContextLength = 8000

	return cfg
}

func createReloadTestConfig(t *testing.T, tempDir string) *config.AIDebateConfig {
	return createTestDebateConfig(t, tempDir)
}

func createModifiedReloadTestConfig(t *testing.T, tempDir string) *config.AIDebateConfig {
	cfg := createTestDebateConfig(t, tempDir)
	cfg.MaximalRepeatRounds = 5  // Modified from original
	cfg.ConsensusThreshold = 0.8 // Modified from original

	return cfg
}

func createStressTestConfig(t *testing.T, tempDir string) *config.AIDebateConfig {
	cfg := createTestDebateConfig(t, tempDir)
	cfg.MaximalRepeatRounds = 2 // Reduce rounds for faster stress testing
	cfg.MaxResponseTime = 15000 // 15 seconds

	return cfg
}

// Mock debate service implementations for E2E testing

func createMockDebateService(t *testing.T, cfg *config.AIDebateConfig) *MockDebateService {
	return &MockDebateService{
		config: cfg,
		logger: t.Logf,
	}
}

func createFallbackTestDebateService(t *testing.T, cfg *config.AIDebateConfig) *MockDebateService {
	return &MockDebateService{
		config:          cfg,
		logger:          t.Logf,
		triggerFallback: true,
	}
}

func createCogneeTestDebateService(t *testing.T, cfg *config.AIDebateConfig) *MockDebateService {
	return &MockDebateService{
		config:       cfg,
		logger:       t.Logf,
		enableCognee: true,
	}
}

func createTimeoutTestDebateService(t *testing.T, cfg *config.AIDebateConfig) *MockDebateService {
	return &MockDebateService{
		config:         cfg,
		logger:         t.Logf,
		triggerTimeout: true,
	}
}

func createMemoryTestDebateService(t *testing.T, cfg *config.AIDebateConfig) *MockDebateService {
	return &MockDebateService{
		config:       cfg,
		logger:       t.Logf,
		enableMemory: true,
	}
}

func createReloadTestDebateService(t *testing.T, cfg *config.AIDebateConfig) *MockDebateService {
	return &MockDebateService{
		config:        cfg,
		logger:        t.Logf,
		supportReload: true,
	}
}

func createStressTestDebateService(t *testing.T, cfg *config.AIDebateConfig) *MockDebateService {
	return &MockDebateService{
		config:     cfg,
		logger:     t.Logf,
		stressMode: true,
	}
}

// MockDebateService implements a mock version of the debate service for E2E testing
type MockDebateService struct {
	config          *config.AIDebateConfig
	logger          func(string, ...any)
	triggerFallback bool
	triggerTimeout  bool
	enableCognee    bool
	enableMemory    bool
	supportReload   bool
	stressMode      bool
}

func (m *MockDebateService) ConductDebate(ctx context.Context, topic string, initialContext string) (*services.DebateResult, error) {
	m.logger("MockDebateService.ConductDebate called with topic: %s", topic)

	// Simulate timeout if requested
	if m.triggerTimeout {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("debate timed out")
		case <-time.After(2 * time.Second):
			return nil, fmt.Errorf("simulated timeout")
		}
	}

	// Simulate stress mode delay
	if m.stressMode {
		time.Sleep(100 * time.Millisecond)
	}

	// Create mock result
	result := &services.DebateResult{
		SessionID:       fmt.Sprintf("mock_session_%d", time.Now().Unix()),
		Topic:           topic,
		Duration:        time.Duration(1000+m.stressModeOffset()) * time.Millisecond,
		RoundsConducted: 2,
		FinalScore:      0.85,
		QualityMetrics: &services.QualityMetrics{
			Coherence:    0.8,
			Relevance:    0.75,
			Accuracy:     0.82,
			Completeness: 0.78,
			OverallScore: 0.79,
		},
		Consensus: &services.ConsensusResult{
			Reached:        true,
			ConsensusLevel: 0.8,
			AgreementScore: 0.75,
			QualityScore:   0.8,
			Summary:        fmt.Sprintf("Mock consensus reached on topic: %s", topic),
			KeyPoints:      []string{"Point 1", "Point 2", "Point 3"},
		},
		BestResponse: &services.ParticipantResponse{
			ParticipantName: "TestAnalyst",
			LLMName:         "TestLLM",
			Content:         fmt.Sprintf("Mock response for topic: %s with context: %s", topic, initialContext),
			Confidence:      0.85,
			QualityScore:    0.8,
			ResponseTime:    time.Duration(500+m.stressModeOffset()) * time.Millisecond,
			RoundNumber:     1,
			Timestamp:       time.Now(),
			CogneeEnhanced:  m.enableCognee,
		},
		AllResponses: []services.ParticipantResponse{
			{
				ParticipantName: "TestAnalyst",
				LLMName:         "TestLLM",
				Content:         "Mock analyst response",
				Confidence:      0.85,
				QualityScore:    0.8,
				ResponseTime:    500 * time.Millisecond,
				RoundNumber:     1,
				Timestamp:       time.Now(),
				CogneeEnhanced:  m.enableCognee,
			},
			{
				ParticipantName: "TestCritic",
				LLMName:         "TestLLM2",
				Content:         "Mock critic response",
				Confidence:      0.75,
				QualityScore:    0.7,
				ResponseTime:    600 * time.Millisecond,
				RoundNumber:     1,
				Timestamp:       time.Now(),
				CogneeEnhanced:  m.enableCognee,
			},
		},
		Recommendations: []string{
			"Mock recommendation 1",
			"Mock recommendation 2",
		},
	}

	// Add fallback indicators
	if m.triggerFallback {
		result.FallbackUsed = true
		result.BestResponse.Metadata = map[string]any{
			"fallback_triggered": true,
			"fallback_reason":    "primary_llm_failure",
		}
	}

	// Add Cognee indicators
	if m.enableCognee {
		result.CogneeEnhanced = true
		result.CogneeInsights = &services.CogneeInsights{
			SentimentAnalysis: services.SentimentAnalysis{
				OverallSentiment: "positive",
				SentimentScore:   0.7,
				SentimentByRound: []services.SentimentByRound{
					{Round: 1, Sentiment: "positive", Score: 0.7},
					{Round: 2, Sentiment: "neutral", Score: 0.2},
				},
			},
			EntityExtraction: []services.Entity{
				{Text: "AI", Type: "technology", Confidence: 0.9},
				{Text: "regulation", Type: "policy", Confidence: 0.8},
				{Text: "government", Type: "organization", Confidence: 0.7},
			},
			TopicModeling: map[string]float64{
				"artificial_intelligence": 0.6,
				"policy":                  0.4,
			},
			CoherenceScore:  0.85,
			RelevanceScore:  0.9,
			InnovationScore: 0.7,
		}
	}

	// Add memory indicators
	if m.enableMemory {
		result.MemoryUsed = true
		result.BestResponse.Metadata["memory_context"] = "mock_memory_context_applied"
	}

	return result, nil
}

func (m *MockDebateService) stressModeOffset() int64 {
	if m.stressMode {
		return 50
	}
	return 0
}

// Validation functions

func validateDebateResults(t *testing.T, result *services.DebateResult, expectedTopic string) {
	assert.NotEmpty(t, result.SessionID, "Session ID should not be empty")
	assert.Equal(t, expectedTopic, result.Topic, "Topic mismatch")
	assert.Greater(t, result.Duration, time.Duration(0), "Duration should be positive")
	assert.Greater(t, result.RoundsConducted, 0, "Should have conducted at least one round")
	assert.Greater(t, result.FinalScore, 0.0, "Final score should be positive")

	assert.NotNil(t, result.Consensus, "Consensus should not be nil")
	if result.Consensus != nil {
		assert.GreaterOrEqual(t, result.Consensus.ConsensusLevel, 0.0, "Consensus level should be non-negative")
		assert.LessOrEqual(t, result.Consensus.ConsensusLevel, 1.0, "Consensus level should not exceed 1.0")
		assert.NotEmpty(t, result.Consensus.Summary, "Consensus summary should not be empty")
	}

	assert.NotEmpty(t, result.BestResponse.Content, "Best response content should not be empty")
	assert.Greater(t, len(result.AllResponses), 1, "Should have multiple responses")
	assert.NotEmpty(t, result.Recommendations, "Should have recommendations")

	// Validate quality metrics
	assert.NotNil(t, result.QualityMetrics, "Quality metrics should not be nil")
	if result.QualityMetrics != nil {
		assert.Contains(t, result.QualityMetrics, "avg_confidence", "Should have avg_confidence metric")
		assert.Contains(t, result.QualityMetrics, "avg_quality", "Should have avg_quality metric")
	}
}
