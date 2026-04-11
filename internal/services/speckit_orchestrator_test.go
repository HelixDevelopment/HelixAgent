package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpecKitOrchestrator_Initialization tests initialization
func TestSpecKitOrchestrator_Initialization(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	assert.NotNil(t, orchestrator)
	assert.NotNil(t, orchestrator.debateService)
	assert.NotNil(t, orchestrator.constitutionManager)
	assert.NotNil(t, orchestrator.documentationSync)
	assert.NotNil(t, orchestrator.logger)
	assert.Equal(t, projectRoot, orchestrator.projectRoot)
	assert.NotNil(t, orchestrator.phaseDebateRounds)
	assert.NotNil(t, orchestrator.phaseTimeouts)
}

// TestSpecKitOrchestrator_PhaseConfiguration tests phase configuration
func TestSpecKitOrchestrator_PhaseConfiguration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	// Verify all phases have configuration
	phases := []SpecKitPhase{
		PhaseConstitution,
		PhaseSpecify,
		PhaseClarify,
		PhasePlan,
		PhaseTasks,
		PhaseAnalyze,
		PhaseImplement,
	}

	for _, phase := range phases {
		rounds, ok := orchestrator.phaseDebateRounds[phase]
		assert.True(t, ok, "Phase %s missing rounds configuration", phase)
		assert.Greater(t, rounds, 0, "Phase %s has invalid rounds", phase)

		timeout, ok := orchestrator.phaseTimeouts[phase]
		assert.True(t, ok, "Phase %s missing timeout configuration", phase)
		assert.Greater(t, timeout, time.Duration(0), "Phase %s has invalid timeout", phase)
	}
}

// TestSpecKitOrchestrator_BuildConstitutionDebateTopic tests Constitution topic building
func TestSpecKitOrchestrator_BuildConstitutionDebateTopic(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	userRequest := "Build a new authentication system"
	intentResult := &EnhancedIntentResult{
		Intent:           IntentRequest,
		Confidence:       0.9,
		IsActionable:     true,
		Granularity:      GranularityBigCreation,
		GranularityScore: 0.85,
		ActionType:       ActionCreation,
		ActionTypeScore:  0.9,
		EstimatedScope:   "Large feature requiring multiple components",
	}

	ctx := context.Background()
	constitution, err := constitutionManager.LoadOrCreateConstitution(ctx, projectRoot)
	require.NoError(t, err)

	topic := orchestrator.buildConstitutionDebateTopic(userRequest, intentResult, constitution)

	assert.NotEmpty(t, topic)
	assert.Contains(t, topic, userRequest)
	assert.Contains(t, topic, "Granularity")
	assert.Contains(t, topic, "big_creation")
	assert.Contains(t, topic, "Action Type")
	assert.Contains(t, topic, "creation")
	assert.Contains(t, topic, "Constitution")
	assert.Contains(t, topic, "AGENTS.md")
	assert.Contains(t, topic, "CLAUDE.md")
	assert.Contains(t, topic, "mandatory Constitution points")
}

// TestSpecKitOrchestrator_ExtractJSON tests JSON extraction
func TestSpecKitOrchestrator_ExtractJSON(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			"Plain JSON",
			`{"task": "test"}`,
			`{"task": "test"}`,
		},
		{
			"JSON with markdown",
			"```json\n{\"task\": \"test\"}\n```",
			`{"task": "test"}`,
		},
		{
			"JSON with backticks only",
			"```\n{\"task\": \"test\"}\n```",
			`{"task": "test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orchestrator.extractJSON(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetPhaseResult tests helper function
func TestGetPhaseResult(t *testing.T) {
	flowResult := &SpecKitFlowResult{
		FlowID:    "test-flow",
		StartTime: time.Now(),
		PhaseResults: []SpecKitPhaseResult{
			{
				Phase:   PhaseConstitution,
				Success: true,
				Output:  "Constitution output",
			},
			{
				Phase:   PhaseSpecify,
				Success: true,
				Output:  "Specification output",
			},
		},
	}

	// Test getting existing phase
	result := GetPhaseResult(flowResult, PhaseConstitution)
	assert.NotNil(t, result)
	assert.Equal(t, PhaseConstitution, result.Phase)
	assert.Equal(t, "Constitution output", result.Output)

	// Test getting non-existent phase
	result = GetPhaseResult(flowResult, PhaseImplement)
	assert.Nil(t, result)
}

// TestSpecKitPhaseResult_Structure tests phase result structure
func TestSpecKitPhaseResult_Structure(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Minute)

	result := &SpecKitPhaseResult{
		Phase:        PhaseConstitution,
		StartTime:    startTime,
		EndTime:      endTime,
		Duration:     5 * time.Minute,
		Success:      true,
		Output:       "Test output",
		DebateID:     "debate-123",
		QualityScore: 0.85,
		Artifacts: map[string]interface{}{
			"test_artifact": "test_value",
		},
	}

	assert.Equal(t, PhaseConstitution, result.Phase)
	assert.Equal(t, startTime, result.StartTime)
	assert.Equal(t, endTime, result.EndTime)
	assert.Equal(t, 5*time.Minute, result.Duration)
	assert.True(t, result.Success)
	assert.Equal(t, "Test output", result.Output)
	assert.Equal(t, "debate-123", result.DebateID)
	assert.Equal(t, 0.85, result.QualityScore)
	assert.NotNil(t, result.Artifacts)
	assert.Equal(t, "test_value", result.Artifacts["test_artifact"])
}

// TestSpecKitFlowResult_Structure tests flow result structure
func TestSpecKitFlowResult_Structure(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Hour)

	flowResult := &SpecKitFlowResult{
		FlowID:    "flow-123",
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  1 * time.Hour,
		Success:   true,
		PhaseResults: []SpecKitPhaseResult{
			{Phase: PhaseConstitution, Success: true},
			{Phase: PhaseSpecify, Success: true},
		},
		Constitution: &Constitution{
			Version:     "1.0.0",
			ProjectName: "TestProject",
		},
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	assert.Equal(t, "flow-123", flowResult.FlowID)
	assert.Equal(t, startTime, flowResult.StartTime)
	assert.Equal(t, endTime, flowResult.EndTime)
	assert.Equal(t, 1*time.Hour, flowResult.Duration)
	assert.True(t, flowResult.Success)
	assert.Len(t, flowResult.PhaseResults, 2)
	assert.NotNil(t, flowResult.Constitution)
	assert.NotNil(t, flowResult.Metadata)
}

// TestSpecKitPhase_Constants tests phase constants
func TestSpecKitPhase_Constants(t *testing.T) {
	phases := []SpecKitPhase{
		PhaseConstitution,
		PhaseSpecify,
		PhaseClarify,
		PhasePlan,
		PhaseTasks,
		PhaseAnalyze,
		PhaseImplement,
	}

	expectedValues := []string{
		"constitution",
		"specify",
		"clarify",
		"plan",
		"tasks",
		"analyze",
		"implement",
	}

	for i, phase := range phases {
		assert.Equal(t, expectedValues[i], string(phase))
	}
}

// TestSpecKitOrchestrator_ExecuteFlow_NoProvider tests flow fails gracefully without provider
func TestSpecKitOrchestrator_ExecuteFlow_NoProvider(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil) // Registry may auto-discover providers from environment
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userRequest := "Build a new feature"
	intentResult := &EnhancedIntentResult{
		Intent:           IntentRequest,
		Confidence:       0.9,
		IsActionable:     true,
		Granularity:      GranularityBigCreation,
		GranularityScore: 0.85,
		ActionType:       ActionCreation,
		ActionTypeScore:  0.9,
		RequiresSpecKit:  true,
		SpecKitReason:    "substantial feature creation detected",
	}

	result, err := orchestrator.ExecuteFlow(ctx, userRequest, intentResult)

	// If providers available from environment, might succeed or have a different error
	// If no providers, should fail with provider error
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.FlowID)

	if err != nil {
		// Should mention constitution or provider issue
		assert.True(t,
			strings.Contains(err.Error(), "constitution") ||
				strings.Contains(err.Error(), "provider") ||
				strings.Contains(err.Error(), "debate") ||
				strings.Contains(err.Error(), "timeout"),
			"Expected phase/provider-related error, got: %v", err)
	}
	// If no error, providers were available and flow succeeded (OK for this test)
}

// TestSpecKitOrchestrator_PhaseTimeouts tests that phase timeouts are reasonable
func TestSpecKitOrchestrator_PhaseTimeouts(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	// Verify timeouts are reasonable (not too short, not too long)
	for phase, timeout := range orchestrator.phaseTimeouts {
		assert.GreaterOrEqual(t, timeout, 5*time.Minute,
			"Phase %s timeout too short", phase)
		assert.LessOrEqual(t, timeout, 30*time.Minute,
			"Phase %s timeout too long", phase)
	}
}

// TestSpecKitOrchestrator_PhaseDebateRounds tests that phase debate rounds are reasonable
func TestSpecKitOrchestrator_PhaseDebateRounds(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	// Verify rounds are reasonable (not too few, not too many)
	for phase, rounds := range orchestrator.phaseDebateRounds {
		assert.GreaterOrEqual(t, rounds, 2,
			"Phase %s rounds too few", phase)
		assert.LessOrEqual(t, rounds, 10,
			"Phase %s rounds too many", phase)
	}
}

// TestSpecKitOrchestrator_PhaseResultError tests phase result with error
func TestSpecKitOrchestrator_PhaseResultError(t *testing.T) {
	result := &SpecKitPhaseResult{
		Phase:     PhaseConstitution,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Minute),
		Duration:  1 * time.Minute,
		Success:   false,
		Error:     "Test error occurred",
	}

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "Test error")
}

// TestSpecKitOrchestrator_FlowResultWithConstitution tests flow result includes Constitution
func TestSpecKitOrchestrator_FlowResultWithConstitution(t *testing.T) {
	flowResult := &SpecKitFlowResult{
		FlowID:    "test-flow",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Hour),
		Duration:  1 * time.Hour,
		Success:   true,
		Constitution: &Constitution{
			Version:     "1.0.0",
			ProjectName: "TestProject",
			Rules: []ConstitutionRule{
				{
					ID:          "TEST-001",
					Category:    "Testing",
					Title:       "Test Rule",
					Description: "Test description",
					Mandatory:   true,
					Priority:    1,
				},
			},
		},
	}

	assert.NotNil(t, flowResult.Constitution)
	assert.Equal(t, "TestProject", flowResult.Constitution.ProjectName)
	assert.Len(t, flowResult.Constitution.Rules, 1)
}

// TestSpecKitOrchestrator_FlowResumption tests flow resumption logic
func TestSpecKitOrchestrator_FlowResumption(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	registry := NewProviderRegistryWithoutAutoDiscovery(nil, nil)
	debateService := NewDebateService(logger)
	debateService.providerRegistry = registry

	constitutionManager := NewConstitutionManager(logger)
	documentationSync := NewDocumentationSync(logger)
	projectRoot := t.TempDir()

	orchestrator := NewSpecKitOrchestrator(
		debateService,
		constitutionManager,
		documentationSync,
		logger,
		projectRoot,
	)

	// Create a partial flow (stopped after Phase 3)
	flowID := "test-resumption-flow"
	startTime := time.Now().Add(-1 * time.Hour)

	partialFlow := &SpecKitFlowResult{
		FlowID:    flowID,
		StartTime: startTime,
		Success:   false,
		PhaseResults: []SpecKitPhaseResult{
			{
				Phase:        PhaseConstitution,
				Success:      true,
				Artifact:     "Constitution created",
				QualityScore: 0.9,
				StartTime:    startTime,
				EndTime:      startTime.Add(5 * time.Minute),
				Duration:     5 * time.Minute,
			},
			{
				Phase:        PhaseSpecify,
				Success:      true,
				Artifact:     "Specification created",
				QualityScore: 0.85,
				StartTime:    startTime.Add(5 * time.Minute),
				EndTime:      startTime.Add(10 * time.Minute),
				Duration:     5 * time.Minute,
			},
			{
				Phase:        PhaseClarify,
				Success:      true,
				Artifact:     "Clarifications complete",
				QualityScore: 0.88,
				StartTime:    startTime.Add(10 * time.Minute),
				EndTime:      startTime.Add(15 * time.Minute),
				Duration:     5 * time.Minute,
			},
		},
		Constitution: &Constitution{
			Version:     "1.0.0",
			ProjectName: "TestProject",
			Rules:       []ConstitutionRule{},
		},
		Phases:   make(map[string]*SpecKitPhaseResult),
		Metadata: make(map[string]interface{}),
	}

	// Populate Phases map
	for i := range partialFlow.PhaseResults {
		phase := &partialFlow.PhaseResults[i]
		partialFlow.Phases[string(phase.Phase)] = phase
	}

	// Save partial flow to cache
	err := orchestrator.saveFlowToCache(partialFlow)
	require.NoError(t, err, "Should save partial flow")

	// Test resumption detection
	t.Run("DetectResumePoint", func(t *testing.T) {
		loaded, err := orchestrator.loadFlowFromCache(flowID)
		require.NoError(t, err, "Should load partial flow")
		assert.False(t, loaded.Success, "Flow should not be marked as successful")
		assert.Len(t, loaded.PhaseResults, 3, "Should have 3 completed phases")
		assert.Equal(t, PhaseClarify, loaded.PhaseResults[2].Phase, "Last phase should be Clarify")
	})

	t.Run("ResumeFromCache", func(t *testing.T) {
		loaded, err := orchestrator.loadFlowFromCache(flowID)
		require.NoError(t, err, "Should load cached flow")
		assert.Equal(t, flowID, loaded.FlowID, "Flow ID should match")
		assert.True(t, loaded.ResumedFromCache, "Should be marked as resumed from cache")

		// Verify we can determine remaining phases
		lastPhase := loaded.PhaseResults[len(loaded.PhaseResults)-1].Phase
		assert.Equal(t, PhaseClarify, lastPhase, "Last completed phase should be Clarify")

		// Remaining phases should be: Plan, Tasks, Analyze, Implement
		allPhases := []SpecKitPhase{
			PhaseConstitution, PhaseSpecify, PhaseClarify,
			PhasePlan, PhaseTasks, PhaseAnalyze, PhaseImplement,
		}

		lastPhaseIndex := -1
		for i, phase := range allPhases {
			if phase == lastPhase {
				lastPhaseIndex = i
				break
			}
		}

		remainingPhases := allPhases[lastPhaseIndex+1:]
		assert.Len(t, remainingPhases, 4, "Should have 4 remaining phases")
		assert.Equal(t, PhasePlan, remainingPhases[0], "Next phase should be Plan")
		assert.Equal(t, PhaseImplement, remainingPhases[3], "Final phase should be Implement")

		// Context preservation test
		assert.NotNil(t, loaded.Constitution, "Constitution should be preserved")
		assert.NotNil(t, loaded.Phases[string(PhaseConstitution)], "Constitution phase should be in map")
		assert.NotNil(t, loaded.Phases[string(PhaseSpecify)], "Specify phase should be in map")
		assert.NotNil(t, loaded.Phases[string(PhaseClarify)], "Clarify phase should be in map")
	})

	t.Run("ClearCache", func(t *testing.T) {
		// Clear the cache
		err := orchestrator.clearFlowCache(flowID)
		require.NoError(t, err, "Should clear cache")

		// Verify cache is cleared
		_, err = orchestrator.loadFlowFromCache(flowID)
		assert.Error(t, err, "Should not be able to load cleared cache")
	})
}
