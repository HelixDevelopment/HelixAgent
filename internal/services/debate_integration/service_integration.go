// Package debate_integration provides integration between the debate framework
// and HelixAgent services.
package debate_integration

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/services"
	"digital.vasic.debate"
	"digital.vasic.debate/comprehensive"
	"digital.vasic.debate/orchestrator"
)

// =============================================================================
// Service Integration - Connects Orchestrator with DebateService
// =============================================================================

// ServiceIntegration provides integration between the new debate framework
// and the existing DebateService. It allows gradual migration to the new system.
type ServiceIntegration struct {
	orchestrator             *orchestrator.Orchestrator
	providerRegistry         *services.ProviderRegistry
	providerBridge           *ProviderRegistryBridge
	comprehensiveIntegration *comprehensive.IntegrationManager // Shared pool target
	logger                   *logrus.Logger
	config                   ServiceIntegrationConfig
}

// SetComprehensiveIntegration sets the comprehensive IntegrationManager so its
// agent pool gets populated alongside the orchestrator pool.
func (si *ServiceIntegration) SetComprehensiveIntegration(ci *comprehensive.IntegrationManager) {
	si.comprehensiveIntegration = ci
}

// ServiceIntegrationConfig configures the service integration.
type ServiceIntegrationConfig struct {
	// EnableNewFramework enables the new debate framework
	EnableNewFramework bool `json:"enable_new_framework"`

	// FallbackToLegacy falls back to legacy if new framework fails
	FallbackToLegacy bool `json:"fallback_to_legacy"`

	// EnableLearning enables the knowledge/learning features
	EnableLearning bool `json:"enable_learning"`

	// MinAgentsForNewFramework minimum agents to use new framework
	MinAgentsForNewFramework int `json:"min_agents_for_new_framework"`

	// LogDebateDetails logs detailed debate information
	LogDebateDetails bool `json:"log_debate_details"`
}

// DefaultServiceIntegrationConfig returns default integration config.
// Per docs/requests/debate requirements: 5 positions × 3 LLMs = 15 agents
func DefaultServiceIntegrationConfig() ServiceIntegrationConfig {
	return ServiceIntegrationConfig{
		EnableNewFramework:       true,  // NEW FRAMEWORK IS DEFAULT
		FallbackToLegacy:         false, // Don't fallback - use new system
		EnableLearning:           true,
		MinAgentsForNewFramework: 3, // Minimum to start (ideally 15 for full 5×3)
		LogDebateDetails:         true,
	}
}

// NewServiceIntegration creates a new service integration.
func NewServiceIntegration(
	providerRegistry *services.ProviderRegistry,
	logger *logrus.Logger,
	config ServiceIntegrationConfig,
) *ServiceIntegration {
	// Create orchestrator using the factory
	factory := NewOrchestratorFactory(providerRegistry)

	orchConfig := orchestrator.DefaultOrchestratorConfig()
	orchConfig.EnableLearning = config.EnableLearning
	orchConfig.EnableCrossDebateLearning = config.EnableLearning
	orchConfig.MinAgentsPerDebate = config.MinAgentsForNewFramework
	// Allow sufficient timeout for 2 rounds × 8 phases with slow providers (Mistral: 10-22s each)
	orchConfig.DefaultMaxRounds = 2
	orchConfig.DefaultTimeout = 5 * time.Minute

	orch, bridge := factory.CreateOrchestratorWithBridge(orchConfig)

	return &ServiceIntegration{
		orchestrator:     orch,
		providerRegistry: providerRegistry,
		providerBridge:   bridge,
		logger:           logger,
		config:           config,
	}
}

// =============================================================================
// Debate Execution
// =============================================================================

// ConductDebate conducts a debate using the new framework.
func (si *ServiceIntegration) ConductDebate(
	ctx context.Context,
	config *services.DebateConfig,
) (*services.DebateResult, error) {
	if !si.config.EnableNewFramework {
		return nil, fmt.Errorf("new framework is disabled")
	}

	if si.logger != nil && si.config.LogDebateDetails {
		si.logger.WithFields(logrus.Fields{
			"debate_id": config.DebateID,
			"topic":     config.Topic,
			"framework": "new",
		}).Info("Conducting debate with new framework")
	}

	// Convert services.DebateConfig to DebateRequest
	debateReq := si.convertDebateConfig(config)

	// Conduct the debate
	response, err := si.orchestrator.ConductDebate(ctx, debateReq)
	if err != nil {
		if si.logger != nil {
			si.logger.WithError(err).Error("New framework debate failed")
		}
		return nil, err
	}

	// Convert DebateResponse to services.DebateResult
	result := si.convertToDebateResult(response, config)

	if si.logger != nil && si.config.LogDebateDetails {
		si.logger.WithFields(logrus.Fields{
			"debate_id":         result.DebateID,
			"success":           result.Success,
			"final_score":       result.FinalScore,
			"lessons_learned":   response.LessonsLearned,
			"patterns_detected": response.PatternsDetected,
		}).Info("New framework debate completed")
	}

	return result, nil
}

// ShouldUseNewFramework determines if the new framework should be used.
func (si *ServiceIntegration) ShouldUseNewFramework(config *services.DebateConfig) bool {
	if !si.config.EnableNewFramework {
		return false
	}

	// Check if we have enough agents
	agentCount := si.orchestrator.GetAgentPool().Size()
	if agentCount < si.config.MinAgentsForNewFramework {
		if si.logger != nil {
			si.logger.WithFields(logrus.Fields{
				"available_agents": agentCount,
				"required_agents":  si.config.MinAgentsForNewFramework,
			}).Debug("Not enough agents for new framework")
		}
		return false
	}

	return true
}

// GetOrchestrator returns the underlying orchestrator.
func (si *ServiceIntegration) GetOrchestrator() *orchestrator.Orchestrator {
	return si.orchestrator
}

// =============================================================================
// Type Conversion
// =============================================================================

// convertDebateConfig converts services.DebateConfig to DebateRequest.
func (si *ServiceIntegration) convertDebateConfig(config *services.DebateConfig) *orchestrator.DebateRequest {
	request := &orchestrator.DebateRequest{
		ID:        config.DebateID,
		Topic:     config.Topic,
		MaxRounds: config.MaxRounds,
		Timeout:   config.Timeout, // Already a time.Duration
		Metadata:  config.Metadata,
	}

	// Extract preferred providers from participants
	for _, p := range config.Participants {
		if p.LLMProvider != "" {
			request.PreferredProviders = append(request.PreferredProviders, p.LLMProvider)
		}
	}

	// Set minimum consensus from config
	if config.Strategy == "consensus" || config.Strategy == "" {
		request.MinConsensus = 0.75 // Default
	}

	// Enable learning
	if si.config.EnableLearning {
		enableLearning := true
		request.EnableLearning = &enableLearning
	}

	return request
}

// convertToDebateResult converts DebateResponse to services.DebateResult.
func (si *ServiceIntegration) convertToDebateResult(
	response *orchestrator.DebateResponse,
	config *services.DebateConfig,
) *services.DebateResult {
	result := &services.DebateResult{
		DebateID:  response.ID,
		Topic:     response.Topic,
		StartTime: time.Now().Add(-response.Duration),
		EndTime:   time.Now(),
		Success:   response.Success,
		Metadata:  response.Metadata,
	}

	// Add framework indicator to metadata
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["framework"] = "new_debate_system"
	result.Metadata["lessons_learned"] = response.LessonsLearned
	result.Metadata["patterns_detected"] = response.PatternsDetected

	// Convert all responses
	result.AllResponses = make([]services.ParticipantResponse, 0)
	for _, phase := range response.Phases {
		for _, resp := range phase.Responses {
			result.AllResponses = append(result.AllResponses, services.ParticipantResponse{
				ParticipantID:   resp.AgentID,
				ParticipantName: resp.Provider + "/" + resp.Model,
				Role:            resp.Role,
				LLMProvider:     resp.Provider,
				LLMModel:        resp.Model,
				Content:         resp.Content,
				Confidence:      resp.Confidence,
				QualityScore:    resp.Score,
				ResponseTime:    resp.Latency,
				Round:           phase.Round,
				Metadata: map[string]interface{}{
					"phase": phase.Phase,
				},
			})
		}
	}

	// Convert consensus
	if response.Consensus != nil {
		result.Consensus = &services.ConsensusResult{
			Reached:        response.Success,
			AgreementLevel: response.Consensus.Confidence,
			FinalPosition:  response.Consensus.Summary,
			KeyPoints:      response.Consensus.KeyPoints,
			Disagreements:  response.Consensus.Dissents,
		}
		result.FinalScore = response.Consensus.Confidence
	}

	// Set scores from metrics
	if response.Metrics != nil {
		result.QualityScore = response.Metrics.AvgConfidence
		if result.FinalScore == 0 {
			result.FinalScore = response.Metrics.ConsensusScore
		}
	}

	return result
}

// =============================================================================
// Statistics and Management
// =============================================================================

// GetStatistics returns integration statistics.
func (si *ServiceIntegration) GetStatistics(ctx context.Context) (*IntegrationStatistics, error) {
	orchStats, err := si.orchestrator.GetStatistics(ctx)
	if err != nil {
		return nil, err
	}

	return &IntegrationStatistics{
		FrameworkEnabled:    si.config.EnableNewFramework,
		LearningEnabled:     si.config.EnableLearning,
		ActiveDebates:       orchStats.ActiveDebates,
		RegisteredAgents:    orchStats.RegisteredAgents,
		TotalLessons:        orchStats.TotalLessons,
		TotalPatterns:       orchStats.TotalPatterns,
		TotalDebatesLearned: orchStats.TotalDebatesLearned,
		OverallSuccessRate:  orchStats.OverallSuccessRate,
	}, nil
}

// IntegrationStatistics provides integration-level statistics.
type IntegrationStatistics struct {
	FrameworkEnabled    bool    `json:"framework_enabled"`
	LearningEnabled     bool    `json:"learning_enabled"`
	ActiveDebates       int     `json:"active_debates"`
	RegisteredAgents    int     `json:"registered_agents"`
	TotalLessons        int     `json:"total_lessons"`
	TotalPatterns       int     `json:"total_patterns"`
	TotalDebatesLearned int     `json:"total_debates_learned"`
	OverallSuccessRate  float64 `json:"overall_success_rate"`
}

// PopulateFromDebateTeam registers verified providers from the DebateTeamConfig
// into the orchestrator's agent pool. This ensures the orchestrator uses the same
// verified, scored providers as the debate team rather than just the provider registry.
// Call this AFTER DebateTeamConfig.InitializeTeam() has completed.
func (si *ServiceIntegration) PopulateFromDebateTeam(teamConfig *services.DebateTeamConfig) {
	if teamConfig == nil || si.orchestrator == nil {
		return
	}

	// Give the provider bridge access to verified provider instances so that
	// OAuth/CLI providers (Claude, Qwen, etc.) can be resolved at invoke time.
	if si.providerBridge != nil {
		si.providerBridge.SetDebateTeamConfig(teamConfig)
	}

	verifiedLLMs := teamConfig.GetVerifiedLLMs()
	if len(verifiedLLMs) == 0 {
		if si.logger != nil {
			si.logger.Warn("No verified LLMs in debate team config - orchestrator agents unchanged")
		}
		return
	}

	registeredCount := 0
	for _, vllm := range verifiedLLMs {
		if vllm.Provider == nil {
			if si.logger != nil {
				si.logger.WithFields(logrus.Fields{
					"provider": vllm.ProviderName,
					"model":    vllm.ModelName,
				}).Debug("Skipping LLM with nil Provider for orchestrator registration")
			}
			continue
		}

		err := si.orchestrator.RegisterProvider(vllm.ProviderName, vllm.ModelName, vllm.Score)
		if err != nil {
			if si.logger != nil {
				si.logger.WithError(err).WithFields(logrus.Fields{
					"provider": vllm.ProviderName,
					"model":    vllm.ModelName,
				}).Warn("Failed to register verified provider in orchestrator")
			}
			continue
		}
		registeredCount++
	}

	// Register real LLM invokers in the comprehensive debate package so that
	// specialized agents (Architect, Generator, Critic, etc.) call real
	// providers instead of returning hardcoded template strings.
	invokerCount := 0
	for _, vllm := range verifiedLLMs {
		if vllm.Provider != nil {
			invoker := NewProviderLLMInvoker(vllm.Provider, vllm.ModelName)
			comprehensive.RegisterInvoker(vllm.ProviderName, vllm.ModelName, invoker)
			invokerCount++
		}
	}

	// Set the strongest verified provider as fallback invoker so agents
	// whose provider/model pair was not explicitly registered still get
	// real LLM calls.
	if len(verifiedLLMs) > 0 && verifiedLLMs[0].Provider != nil {
		comprehensive.SetFallbackInvoker(
			NewProviderLLMInvoker(verifiedLLMs[0].Provider, verifiedLLMs[0].ModelName),
		)
	}

	// Also populate the comprehensive IntegrationManager's agent pool
	// so System.ConductDebate() has agents to invoke via PhaseOrchestrator
	compPoolSize := 0
	if si.comprehensiveIntegration != nil {
		compPool := si.comprehensiveIntegration.GetAgentPool()
		if compPool != nil {
			// Map verified LLMs to comprehensive debate roles
			roles := []comprehensive.Role{
				comprehensive.RoleArchitect, comprehensive.RoleGenerator,
				comprehensive.RoleCritic, comprehensive.RoleRefactoring,
				comprehensive.RoleTester, comprehensive.RoleValidator,
				comprehensive.RoleSecurity, comprehensive.RolePerformance,
			}
			for i, vllm := range verifiedLLMs {
				if vllm.Provider == nil {
					continue
				}
				role := roles[i%len(roles)]
				agent := comprehensive.NewAgent(role, vllm.ProviderName, vllm.ModelName, vllm.Score)
				compPool.Add(agent)
			}
			compPoolSize = compPool.Size()
		}
	}

	if si.logger != nil {
		si.logger.WithFields(logrus.Fields{
			"registered_agents":   registeredCount,
			"registered_invokers": invokerCount,
			"total_verified":      len(verifiedLLMs),
			"pool_size":           si.orchestrator.GetAgentPool().Size(),
			"comp_pool_size":      compPoolSize,
		}).Info("Populated orchestrator agent pool from debate team verified providers")
	}
}

// =============================================================================
// Factory Functions for Easy Integration
// =============================================================================

// CreateIntegration creates a service integration with default config.
func CreateIntegration(
	providerRegistry *services.ProviderRegistry,
	logger *logrus.Logger,
) *ServiceIntegration {
	return NewServiceIntegration(
		providerRegistry,
		logger,
		DefaultServiceIntegrationConfig(),
	)
}

// CreateLessonBank creates a lesson bank for the integration.
// This can be used to provide a shared lesson bank across services.
func CreateLessonBank() *debate.LessonBank {
	config := debate.DefaultLessonBankConfig()
	config.EnableSemanticSearch = false // Disable until embeddings configured
	return debate.NewLessonBank(config, nil, nil)
}
