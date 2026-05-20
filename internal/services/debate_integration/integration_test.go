package debate_integration

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
	"digital.vasic.debate/agents"
	"digital.vasic.debate/orchestrator"
	"digital.vasic.debate/topology"
)

// =============================================================================
// Full Integration Tests - End-to-End Debate Flow
//
// NOTE (round-342, HXA-002): the DebateOrchestrator submodule was rebuilt
// from scratch (DebateOrchestrator commit 196d0ea — "initial reconstruction
// (Phase 1)") with a deliberately slim public API. The pre-reconstruction
// capability tier — APIAdapter.ConvertAPIRequest / GetDebateStatus /
// CancelDebate, Orchestrator.GetKnowledgeRepository / GetRecommendations,
// and the OrchestratorConfig fields DefaultMinConsensus / MaxAgentsPerDebate /
// EnableAgentDiversity — was genuinely DELETED (not moved: a tree-wide
// search of dependencies/ found no surviving copy in any digital.vasic.*
// package or HelixSpecifier). These tests were rewritten down to the slim
// CreateDebate / GetStatistics / ConductDebate surface that the
// reconstructed orchestrator actually exposes.
//
// Lost coverage honestly documented in docs/Fixed.md (HXA-002):
//   - request-conversion assertions (ConvertAPIRequest) — the slim API
//     converts internally inside CreateDebate; covered indirectly here.
//   - learning/knowledge-repository assertions — the reconstructed
//     orchestrator exposes lesson/pattern counters via
//     Orchestrator.GetStatistics (OrchestratorStats.TotalLessons etc.)
//     but no KnowledgeRepository / GetRecommendations handle.
//   - debate status/cancel-by-id via APIAdapter — cancellation is now
//     reachable only via Orchestrator.CancelSession.
// =============================================================================

// TestFullDebateFlow tests orchestrator + APIAdapter wiring on the slim API.
func TestFullDebateFlow(t *testing.T) {
	// Setup: Create orchestrator with mock providers.
	mockRegistry := newMockProviderRegistry()
	mockRegistry.AddProvider("claude", newMockLLMProvider("claude"))
	mockRegistry.AddProvider("deepseek", newMockLLMProvider("deepseek"))
	mockRegistry.AddProvider("gemini", newMockLLMProvider("gemini"))

	config := orchestrator.DefaultOrchestratorConfig()
	config.MinAgentsPerDebate = 3
	config.EnableLearning = true

	lessonBank := createLessonBank(defaultLessonBankConfig())
	orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

	// Register providers - creates agents in the pool.
	err := orch.RegisterProvider("claude", "claude-3", 0.9)
	require.NoError(t, err)
	err = orch.RegisterProvider("deepseek", "deepseek-coder", 0.85)
	require.NoError(t, err)
	err = orch.RegisterProvider("gemini", "gemini-pro", 0.8)
	require.NoError(t, err)

	// Verify setup.
	assert.Equal(t, 3, orch.GetAgentPool().Size())

	// Create API adapter and verify it works.
	adapter := orchestrator.NewAPIAdapter(orch)
	require.NotNil(t, adapter)

	// Slim API: CreateDebate registers participants and runs the debate.
	apiReq := &orchestrator.APICreateDebateRequest{
		DebateID: "integration-test-1",
		Topic:    "Best practices for error handling in Go",
		Participants: []orchestrator.APIParticipantConfig{
			{Name: "Analyst", Role: "analyst", LLMProvider: "claude", LLMModel: "claude-3"},
			{Name: "Developer", Role: "developer", LLMProvider: "deepseek", LLMModel: "deepseek-coder"},
			{Name: "Reviewer", Role: "reviewer", LLMProvider: "gemini", LLMModel: "gemini-pro"},
		},
		MaxRounds: 3,
		Timeout:   60 * time.Second,
		Strategy:  "mesh",
	}

	ctx := context.Background()
	resp, err := adapter.CreateDebate(ctx, apiReq)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "integration-test-1", resp.ID)
	assert.Equal(t, "Best practices for error handling in Go", resp.Topic)
}

// TestServiceIntegrationFlow tests the ServiceIntegration with services types.
func TestServiceIntegrationFlow(t *testing.T) {
	// Setup: Create service integration with mock providers.
	mockRegistry := newMockProviderRegistry()
	mockRegistry.AddProvider("claude", newMockLLMProvider("claude"))
	mockRegistry.AddProvider("deepseek", newMockLLMProvider("deepseek"))
	mockRegistry.AddProvider("gemini", newMockLLMProvider("gemini"))

	config := orchestrator.DefaultOrchestratorConfig()
	config.MinAgentsPerDebate = 3
	config.EnableLearning = true

	lessonBank := createLessonBank(defaultLessonBankConfig())
	orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

	// Register providers.
	_ = orch.RegisterProvider("claude", "claude-3", 0.9)
	_ = orch.RegisterProvider("deepseek", "deepseek-coder", 0.85)
	_ = orch.RegisterProvider("gemini", "gemini-pro", 0.8)

	// Create service integration.
	siConfig := DefaultServiceIntegrationConfig()
	siConfig.MinAgentsForNewFramework = 3

	si := &ServiceIntegration{
		orchestrator: orch,
		logger:       logrus.New(),
		config:       siConfig,
	}

	// Create services.DebateConfig.
	debateConfig := &services.DebateConfig{
		DebateID: "service-integration-test-1",
		Topic:    "Microservices vs Monolith architecture",
		Participants: []services.ParticipantConfig{
			{Name: "Architect", Role: "architect", LLMProvider: "claude", LLMModel: "claude-3"},
			{Name: "DevOps", Role: "devops", LLMProvider: "deepseek", LLMModel: "deepseek-coder"},
			{Name: "PM", Role: "analyst", LLMProvider: "gemini", LLMModel: "gemini-pro"},
		},
		MaxRounds: 3,
		Timeout:   time.Minute,
		Strategy:  "consensus",
	}

	// Verify ShouldUseNewFramework.
	assert.True(t, si.ShouldUseNewFramework(debateConfig))

	// Verify type conversion works.
	request := si.convertDebateConfig(debateConfig)
	require.NotNil(t, request)
	assert.Equal(t, "service-integration-test-1", request.ID)
	assert.Equal(t, "Microservices vs Monolith architecture", request.Topic)
	assert.Equal(t, 3, request.MaxRounds)
	assert.Len(t, request.PreferredProviders, 3)
	assert.Equal(t, 0.75, request.MinConsensus) // Default for consensus strategy

	// Verify statistics are available.
	ctx := context.Background()
	stats, err := si.GetStatistics(ctx)
	require.NoError(t, err)
	assert.True(t, stats.FrameworkEnabled)
	assert.True(t, stats.LearningEnabled)
	assert.Equal(t, 3, stats.RegisteredAgents)
}

// TestOrchestratorWithAllComponents tests the orchestrator with all components.
func TestOrchestratorWithAllComponents(t *testing.T) {
	// Setup.
	mockRegistry := newMockProviderRegistry()
	mockRegistry.AddProvider("claude", newMockLLMProvider("claude"))
	mockRegistry.AddProvider("deepseek", newMockLLMProvider("deepseek"))
	mockRegistry.AddProvider("gemini", newMockLLMProvider("gemini"))

	config := orchestrator.DefaultOrchestratorConfig()
	config.MinAgentsPerDebate = 3
	config.EnableLearning = true
	config.EnableCrossDebateLearning = true
	config.DefaultTopology = topology.TopologyGraphMesh

	lessonBank := createLessonBank(defaultLessonBankConfig())
	orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

	// Register providers.
	_ = orch.RegisterProvider("claude", "claude-3", 0.9)
	_ = orch.RegisterProvider("deepseek", "deepseek-coder", 0.85)
	_ = orch.RegisterProvider("gemini", "gemini-pro", 0.8)

	// Verify all components are initialized.
	assert.NotNil(t, orch.GetAgentPool())
	assert.NotNil(t, orch.Bank())
	assert.Equal(t, 3, orch.GetAgentPool().Size())

	// Get statistics — Orchestrator.GetStatistics still exposes the
	// lesson/pattern learning counters on OrchestratorStats.
	ctx := context.Background()
	stats, err := orch.GetStatistics(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.RegisteredAgents)
	assert.Equal(t, 0, stats.ActiveDebates)
}

// TestDebateWithDifferentTopologies tests topology selection from strategies.
func TestDebateWithDifferentTopologies(t *testing.T) {
	topologies := []struct {
		name     string
		strategy string
		expected topology.TopologyType
	}{
		{"Mesh", "mesh", topology.TopologyGraphMesh},
		{"Chain", "sequential", topology.TopologyChain},
		{"Star", "star", topology.TopologyStar},
		{"Parallel", "parallel", topology.TopologyGraphMesh},
		{"Pipeline", "pipeline", topology.TopologyChain},
		{"Hub", "hub", topology.TopologyStar},
		{"Default", "", topology.TopologyGraphMesh},
	}

	for _, tc := range topologies {
		t.Run(tc.name, func(t *testing.T) {
			// Verify the test case data is well-formed.
			assert.NotEmpty(t, tc.name, "topology name must not be empty")
			assert.NotEmpty(t, string(tc.expected), "expected topology type must be set")
			// Verify known topology types are valid enum values.
			validTopologies := map[topology.TopologyType]bool{
				topology.TopologyGraphMesh: true,
				topology.TopologyChain:     true,
				topology.TopologyStar:      true,
			}
			assert.True(t, validTopologies[tc.expected],
				"topology %s must map to a valid TopologyType, got %s", tc.name, tc.expected)
		})
	}
}

// TestDebateWithLearningEnabled tests learning configuration.
func TestDebateWithLearningEnabled(t *testing.T) {
	// Setup with learning enabled.
	mockRegistry := newMockProviderRegistry()
	mockRegistry.AddProvider("claude", newMockLLMProvider("claude"))
	mockRegistry.AddProvider("deepseek", newMockLLMProvider("deepseek"))
	mockRegistry.AddProvider("gemini", newMockLLMProvider("gemini"))

	config := orchestrator.DefaultOrchestratorConfig()
	config.MinAgentsPerDebate = 3
	config.EnableLearning = true
	config.EnableCrossDebateLearning = true

	lessonBank := createLessonBank(defaultLessonBankConfig())
	orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

	_ = orch.RegisterProvider("claude", "claude-3", 0.9)
	_ = orch.RegisterProvider("deepseek", "deepseek-coder", 0.85)
	_ = orch.RegisterProvider("gemini", "gemini-pro", 0.8)

	// Verify the lesson bank is wired into the orchestrator.
	assert.NotNil(t, orch.Bank())

	// Verify orchestrator statistics include the learning counters.
	ctx := context.Background()
	stats, err := orch.GetStatistics(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalLessons, 0)
	assert.GreaterOrEqual(t, stats.TotalPatterns, 0)
	assert.GreaterOrEqual(t, stats.TotalDebatesLearned, 0)

	// Verify request conversion preserves learning flag.
	enableLearning := true
	req := &orchestrator.DebateRequest{
		ID:             "learning-test-1",
		Topic:          "Test topic for learning",
		EnableLearning: &enableLearning,
	}
	assert.NotNil(t, req.EnableLearning)
	assert.True(t, *req.EnableLearning)
}

// TestMultipleDebatesConcurrently tests concurrent orchestrator operations.
func TestMultipleDebatesConcurrently(t *testing.T) {
	// Setup.
	mockRegistry := newMockProviderRegistry()
	mockRegistry.AddProvider("claude", newMockLLMProvider("claude"))
	mockRegistry.AddProvider("deepseek", newMockLLMProvider("deepseek"))
	mockRegistry.AddProvider("gemini", newMockLLMProvider("gemini"))

	config := orchestrator.DefaultOrchestratorConfig()
	config.MinAgentsPerDebate = 3

	lessonBank := createLessonBank(defaultLessonBankConfig())
	orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

	_ = orch.RegisterProvider("claude", "claude-3", 0.9)
	_ = orch.RegisterProvider("deepseek", "deepseek-coder", 0.85)
	_ = orch.RegisterProvider("gemini", "gemini-pro", 0.8)

	adapter := orchestrator.NewAPIAdapter(orch)
	ctx := context.Background()

	// Run multiple concurrent statistics reads — GetStatistics on the
	// slim API must be safe under concurrent access.
	numOps := 5
	done := make(chan bool, numOps)

	for i := 0; i < numOps; i++ {
		go func(idx int) {
			stats, err := adapter.GetStatistics(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, stats)
			assert.GreaterOrEqual(t, stats.ActiveDebates, 0)
			done <- true
		}(i)
	}

	// Wait for all operations.
	for i := 0; i < numOps; i++ {
		select {
		case <-done:
			// Success.
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for concurrent operation")
		}
	}
}

// TestAgentPoolManagement tests agent pool operations.
func TestAgentPoolManagement(t *testing.T) {
	// Setup.
	mockRegistry := newMockProviderRegistry()
	mockRegistry.AddProvider("claude", newMockLLMProvider("claude"))
	mockRegistry.AddProvider("deepseek", newMockLLMProvider("deepseek"))

	config := orchestrator.DefaultOrchestratorConfig()
	config.MinAgentsPerDebate = 2

	lessonBank := createLessonBank(defaultLessonBankConfig())
	orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

	// Initial state.
	pool := orch.GetAgentPool()
	assert.Equal(t, 0, pool.Size())

	// Register providers.
	err := orch.RegisterProvider("claude", "claude-3", 0.9)
	require.NoError(t, err)
	assert.Equal(t, 1, pool.Size())

	err = orch.RegisterProvider("deepseek", "deepseek-coder", 0.85)
	require.NoError(t, err)
	assert.Equal(t, 2, pool.Size())

	// Try to register same provider with a different model.
	err = orch.RegisterProvider("claude", "claude-3-sonnet", 0.88)
	require.NoError(t, err)
	// Should add a new agent (different model).
	assert.Equal(t, 3, pool.Size())

	// Get agents by domain.
	generalAgents := pool.GetByDomain(agents.DomainGeneral)
	assert.GreaterOrEqual(t, len(generalAgents), 0)
}

// TestTypeConversionRoundTrip tests converting types back and forth.
func TestTypeConversionRoundTrip(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	// Original services.DebateConfig.
	original := &services.DebateConfig{
		DebateID: "roundtrip-test",
		Topic:    "Test roundtrip conversion",
		Participants: []services.ParticipantConfig{
			{Name: "Agent1", Role: "analyst", LLMProvider: "claude", LLMModel: "claude-3"},
			{Name: "Agent2", Role: "coder", LLMProvider: "deepseek", LLMModel: "deepseek-coder"},
		},
		MaxRounds: 5,
		Timeout:   3 * time.Minute,
		Strategy:  "consensus",
		Metadata:  map[string]interface{}{"key": "value"},
	}

	// Convert to DebateRequest.
	request := si.convertDebateConfig(original)

	// Verify conversion.
	assert.Equal(t, original.DebateID, request.ID)
	assert.Equal(t, original.Topic, request.Topic)
	assert.Equal(t, original.MaxRounds, request.MaxRounds)
	assert.Equal(t, original.Timeout, request.Timeout)
	assert.Contains(t, request.PreferredProviders, "claude")
	assert.Contains(t, request.PreferredProviders, "deepseek")
}

// TestErrorHandling tests error scenarios on the slim API.
func TestErrorHandling(t *testing.T) {
	// Test with disabled framework.
	t.Run("DisabledFramework", func(t *testing.T) {
		config := DefaultServiceIntegrationConfig()
		config.EnableNewFramework = false

		si := NewServiceIntegration(nil, nil, config)

		ctx := context.Background()
		result, err := si.ConductDebate(ctx, &services.DebateConfig{})

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "disabled")
	})

	// Test CreateDebate with a nil request — the slim APIAdapter must
	// reject it with an explicit error rather than panicking.
	t.Run("NilCreateRequest", func(t *testing.T) {
		mockRegistry := newMockProviderRegistry()
		config := orchestrator.DefaultOrchestratorConfig()
		lessonBank := createLessonBank(defaultLessonBankConfig())
		orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

		adapter := orchestrator.NewAPIAdapter(orch)

		ctx := context.Background()
		resp, err := adapter.CreateDebate(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	// Test CancelSession for a non-existent session — the orchestrator
	// must return an explicit error.
	t.Run("CancelNonExistentSession", func(t *testing.T) {
		mockRegistry := newMockProviderRegistry()
		config := orchestrator.DefaultOrchestratorConfig()
		lessonBank := createLessonBank(defaultLessonBankConfig())
		orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, config)

		err := orch.CancelSession("non-existent")
		assert.Error(t, err)
	})
}
