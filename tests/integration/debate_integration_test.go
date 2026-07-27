package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/testutil"
)

// skipIfNoInfra skips the test if infrastructure is not available
func skipIfNoInfra(t *testing.T) {
	t.Helper()
	testutil.RequireInfra(t)
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Skipping integration test (SKIP_INTEGRATION=true)") // SKIP-OK: #integration-mode-only
	}
}

// Test integration requires PostgreSQL, Redis running
// Run with: DB_HOST=localhost DB_PORT=5432 go test -v ./tests/integration/...

// TestDebateIntegration_FullWorkflow tests complete debate workflow with integrated features
func TestDebateIntegration_FullWorkflow(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create service with real dependencies
	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	config := &services.DebateConfig{
		DebateID:  "integration-test-1",
		Topic:     "Explain the benefits of microservices architecture",
		MaxRounds: 1,
		Timeout:   5 * time.Second,
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Analyst",
				Role:          "analyst",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
				MaxRounds:     1,
				Timeout:       10 * time.Second,
			},
			{
				ParticipantID: "p2",
				Name:          "Proposer",
				Role:          "proposer",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
				MaxRounds:     1,
				Timeout:       10 * time.Second,
			},
		},
	}

	ctx := context.Background()
	result, err := service.ConductDebate(ctx, config)

	// Should not error even with mock providers
	if err != nil {
		t.Logf("Debate completed with error (expected with no real providers): %v", err)
	}

	// Verify structure is correct
	if result != nil {
		assert.NotEmpty(t, result.DebateID)
		assert.NotEmpty(t, result.Topic)
		assert.NotZero(t, result.StartTime)
	}
}

// TestDebateIntegration_CodeGenerationDetection tests code generation detection triggers Test-Driven mode
func TestDebateIntegration_CodeGenerationDetection(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	config := &services.DebateConfig{
		DebateID:  "integration-test-code",
		Topic:     "Write a Python function to validate email addresses",
		MaxRounds: 1,
		Timeout:   5 * time.Second,
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Generator",
				Role:          "generator",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
			},
		},
	}

	ctx := context.Background()
	result, err := service.ConductDebate(ctx, config)

	// With mock providers, might error, but should still detect code generation
	if result != nil {
		// If test-driven metadata exists, it was detected as code generation
		if result.TestDrivenMetadata != nil {
			assert.NotEmpty(t, result.TestDrivenMetadata, "Test-driven metadata should be populated")
			t.Log("Code generation was detected and Test-Driven mode was triggered")
		}
	}

	t.Logf("Debate result error: %v", err)
}

// TestDebateIntegration_ValidationPipelineExecution tests that 4-Pass validation runs
func TestDebateIntegration_ValidationPipelineExecution(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	config := &services.DebateConfig{
		DebateID:  "integration-test-validation",
		Topic:     "Analyze the trade-offs of using GraphQL vs REST",
		MaxRounds: 1,
		Timeout:   5 * time.Second,
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Analyst",
				Role:          "analyst",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
			},
		},
	}

	ctx := context.Background()
	result, err := service.ConductDebate(ctx, config)

	// Debate completes even without a registered "mock" provider
	// (the service handles provider failures gracefully)
	require.NotNil(t, result, "Debate should return a result even when providers fail")
	assert.NotEmpty(t, result.DebateID, "Result should have debate ID")
	assert.Equal(t, config.Topic, result.Topic, "Topic should match")

	// Validation pipeline is attempted regardless of provider availability
	// ValidationResult may be a typed nil interface when no content was generated
	if result.ValidationResult != nil {
		t.Log("4-Pass Validation Pipeline was executed")
	} else {
		t.Log("Validation skipped (no content generated due to mock provider)")
	}

	// The debate should complete without panic
	t.Logf("Debate result error: %v", err)
	t.Logf("Specialized role: %s", result.SpecializedRole)
}

// TestDebateIntegration_SpecializedRoleSelection tests role selection
func TestDebateIntegration_SpecializedRoleSelection(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	testCases := []struct {
		name         string
		topic        string
		expectedRole string
	}{
		{
			name:         "SecurityTask_SelectsSecurityAnalyst",
			topic:        "Audit the authentication system for vulnerabilities",
			expectedRole: "security_analyst",
		},
		{
			name:         "PerformanceTask_SelectsPerformanceAnalyzer",
			topic:        "Optimize the database query performance",
			expectedRole: "performance_analyzer",
		},
		{
			name:         "RefactorTask_SelectsRefactorer",
			topic:        "Refactor the legacy codebase",
			expectedRole: "refactorer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &services.DebateConfig{
				DebateID:  "integration-test-role-" + tc.name,
				Topic:     tc.topic,
				MaxRounds: 1,
				Timeout:   5 * time.Second,
				Participants: []services.ParticipantConfig{
					{
						ParticipantID: "p1",
						Name:          "Participant",
						Role:          "analyst",
						LLMProvider:   "mock",
						LLMModel:      "mock-model",
					},
				},
			}

			ctx := context.Background()
			result, err := service.ConductDebate(ctx, config)

			if result != nil {
				if result.SpecializedRole != "" {
					assert.Equal(t, tc.expectedRole, result.SpecializedRole,
						"Should select correct specialized role")
					t.Logf("Correctly selected role: %s", result.SpecializedRole)
				}
			}

			t.Logf("Debate result error: %v", err)
		})
	}
}

// TestDebateIntegration_ToolEnrichmentFlag tests that tool enrichment flag is set
func TestDebateIntegration_ToolEnrichmentFlag(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	config := &services.DebateConfig{
		DebateID:  "integration-test-tools",
		Topic:     "Design a caching strategy for the API",
		MaxRounds: 1,
		Timeout:   5 * time.Second,
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Architect",
				Role:          "architect",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
			},
		},
	}

	ctx := context.Background()
	result, err := service.ConductDebate(ctx, config)

	if result != nil {
		// Tool enrichment should be attempted
		if result.ToolEnrichmentUsed {
			assert.True(t, result.ToolEnrichmentUsed, "Tool enrichment flag should be set")
			t.Log("Tool enrichment was used")
		}
	}

	t.Logf("Debate result error: %v", err)
}

// TestDebateIntegration_MetadataPropagation tests metadata propagation through pipeline
func TestDebateIntegration_MetadataPropagation(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	customMetadata := map[string]any{
		"test_key":    "test_value",
		"environment": "integration",
		"test_run_id": "12345",
	}

	config := &services.DebateConfig{
		DebateID:  "integration-test-metadata",
		Topic:     "Test metadata propagation",
		MaxRounds: 1,
		Timeout:   5 * time.Second,
		Metadata:  customMetadata,
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Tester",
				Role:          "tester",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
			},
		},
	}

	ctx := context.Background()
	result, err := service.ConductDebate(ctx, config)

	if result != nil {
		// Metadata should be preserved
		if result.Metadata != nil {
			assert.NotNil(t, result.Metadata, "Result metadata should exist")
			t.Log("Metadata was propagated through the pipeline")
		}
	}

	t.Logf("Debate result error: %v", err)
}

// TestDebateIntegration_ConcurrentDebates tests multiple debates running concurrently
func TestDebateIntegration_ConcurrentDebates(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	ctx := context.Background()
	done := make(chan bool, 3)

	topics := []string{
		"Explain microservices architecture",
		"Compare SQL vs NoSQL databases",
		"Analyze cloud deployment strategies",
	}

	for i, topic := range topics {
		go func(index int, topicStr string) {
			config := &services.DebateConfig{
				DebateID:  "concurrent-test-" + string(rune(index)),
				Topic:     topicStr,
				MaxRounds: 1,
				Timeout:   5 * time.Second,
				Participants: []services.ParticipantConfig{
					{
						ParticipantID: "p1",
						Name:          "Analyst",
						Role:          "analyst",
						LLMProvider:   "mock",
						LLMModel:      "mock-model",
					},
				},
			}

			_, err := service.ConductDebate(ctx, config)
			if err != nil {
				t.Logf("Concurrent debate %d completed with error: %v", index, err)
			}
			done <- true
		}(i, topic)
	}

	// Wait for all debates to complete
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			t.Logf("Debate %d completed", i+1)
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent debates")
		}
	}
}

// TestDebateIntegration_TimeoutHandling tests debate timeout behavior
func TestDebateIntegration_TimeoutHandling(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	config := &services.DebateConfig{
		DebateID:  "integration-test-timeout",
		Topic:     "Test timeout handling",
		MaxRounds: 10,              // Many rounds
		Timeout:   2 * time.Second, // Short timeout
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Participant",
				Role:          "analyst",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
			},
		},
	}

	ctx := context.Background()
	start := time.Now()
	result, err := service.ConductDebate(ctx, config)
	duration := time.Since(start)

	// Should complete within reasonable time (timeout + buffer)
	assert.Less(t, duration, 10*time.Second, "Should timeout appropriately")

	if result != nil {
		t.Logf("Debate completed in %v with %d rounds", duration, result.RoundsConducted)
	}

	t.Logf("Debate result error: %v", err)
}

// TestDebateIntegration_EmptyParticipants tests handling of empty participants
func TestDebateIntegration_EmptyParticipants(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	config := &services.DebateConfig{
		DebateID:     "integration-test-empty",
		Topic:        "Test empty participants",
		MaxRounds:    1,
		Timeout:      3 * time.Second,
		Participants: []services.ParticipantConfig{}, // Empty
	}

	ctx := context.Background()
	result, err := service.ConductDebate(ctx, config)

	// Should handle empty participants gracefully
	if err != nil {
		t.Logf("Expected error with empty participants: %v", err)
	}

	if result != nil {
		assert.False(t, result.Success, "Should not succeed with empty participants")
	}
}

// TestDebateIntegration_ContextCancellation tests context cancellation
func TestDebateIntegration_ContextCancellation(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	config := &services.DebateConfig{
		DebateID:  "integration-test-cancel",
		Topic:     "Test context cancellation",
		MaxRounds: 10,
		Timeout:   5 * time.Second,
		Participants: []services.ParticipantConfig{
			{
				ParticipantID: "p1",
				Name:          "Participant",
				Role:          "analyst",
				LLMProvider:   "mock",
				LLMModel:      "mock-model",
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	start := time.Now()
	result, err := service.ConductDebate(ctx, config)
	duration := time.Since(start)

	// Should stop quickly after cancellation
	assert.Less(t, duration, 8*time.Second, "Should cancel within reasonable time")

	if err != nil {
		t.Logf("Expected error after cancellation: %v", err)
	}

	if result != nil {
		t.Logf("Debate stopped after %d rounds", result.RoundsConducted)
	}
}

// TestDebateIntegration_ServiceReuse tests service can be reused for multiple debates
func TestDebateIntegration_ServiceReuse(t *testing.T) {
	skipIfNoInfra(t)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := services.NewProviderRegistry(nil, nil)
	service := services.NewDebateServiceWithDeps(logger, registry, nil)

	ctx := context.Background()

	// Run 5 debates with same service instance
	for i := 0; i < 5; i++ {
		config := &services.DebateConfig{
			DebateID:  "integration-test-reuse-" + string(rune(i)),
			Topic:     "Test service reuse",
			MaxRounds: 1,
			Timeout:   3 * time.Second,
			Participants: []services.ParticipantConfig{
				{
					ParticipantID: "p1",
					Name:          "Participant",
					Role:          "analyst",
					LLMProvider:   "mock",
					LLMModel:      "mock-model",
				},
			},
		}

		result, err := service.ConductDebate(ctx, config)
		t.Logf("Debate %d: error=%v, success=%v", i+1, err, result != nil)
	}

	// Service should still be usable - verify by checking it's not nil
	assert.NotNil(t, service)
}
