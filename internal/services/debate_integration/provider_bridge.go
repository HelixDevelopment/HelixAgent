// Package debate_integration provides integration between the debate framework
// and HelixAgent services.
package debate_integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"digital.vasic.debate"
	"digital.vasic.debate/orchestrator"
	"digital.vasic.llmprovider"
	digitalvasicmodels "digital.vasic.llmprovider/pkg/models"
)

// =============================================================================
// Provider Adapter - Converts between internal and extracted LLM provider interfaces
// =============================================================================

// adaptedProvider wraps an internal llm.LLMProvider to implement llmprovider.LLMProvider
type adaptedProvider struct {
	inner llm.LLMProvider
}

// NewAdaptedProvider creates a new adapter
func NewAdaptedProvider(inner llm.LLMProvider) llmprovider.LLMProvider {
	return &adaptedProvider{inner: inner}
}

// convertLLMRequest converts digital.vasic.models.LLMRequest to internal models.LLMRequest
func convertLLMRequest(req *digitalvasicmodels.LLMRequest) (*models.LLMRequest, error) {
	// Use JSON marshaling for deep conversion (structs are identical)
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var internalReq models.LLMRequest
	if err := json.Unmarshal(data, &internalReq); err != nil {
		return nil, err
	}
	return &internalReq, nil
}

// convertLLMResponse converts internal models.LLMResponse to digital.vasic.models.LLMResponse
func convertLLMResponse(resp *models.LLMResponse) (*digitalvasicmodels.LLMResponse, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var externalResp digitalvasicmodels.LLMResponse
	if err := json.Unmarshal(data, &externalResp); err != nil {
		return nil, err
	}
	return &externalResp, nil
}

// Complete implements llmprovider.LLMProvider
func (a *adaptedProvider) Complete(ctx context.Context, req *digitalvasicmodels.LLMRequest) (*digitalvasicmodels.LLMResponse, error) {
	internalReq, err := convertLLMRequest(req)
	if err != nil {
		return nil, err
	}
	internalResp, err := a.inner.Complete(ctx, internalReq)
	if err != nil {
		return nil, err
	}
	return convertLLMResponse(internalResp)
}

// CompleteStream implements llmprovider.LLMProvider
//
// Goroutine lifecycle: the forwarder goroutine exits when EITHER
//
//	(a) internalCh closes, OR
//	(b) ctx.Done() fires.
//
// Both the receive from internalCh and the send to externalCh are guarded
// by ctx.Done() so a cancelled caller cannot pin this goroutine even if
// the inner provider never closes internalCh. See BUGFIXES.md Issue #14.
func (a *adaptedProvider) CompleteStream(ctx context.Context, req *digitalvasicmodels.LLMRequest) (<-chan *digitalvasicmodels.LLMResponse, error) {
	internalReq, err := convertLLMRequest(req)
	if err != nil {
		return nil, err
	}
	internalCh, err := a.inner.CompleteStream(ctx, internalReq)
	if err != nil {
		return nil, err
	}
	externalCh := make(chan *digitalvasicmodels.LLMResponse, 1)
	go func() {
		defer close(externalCh)
		for {
			select {
			case <-ctx.Done():
				return
			case internalResp, ok := <-internalCh:
				if !ok {
					return
				}
				externalResp, err := convertLLMResponse(internalResp)
				if err != nil {
					// Conversion failure is non-fatal; skip this frame.
					continue
				}
				select {
				case <-ctx.Done():
					return
				case externalCh <- externalResp:
				}
			}
		}
	}()
	return externalCh, nil
}

// HealthCheck implements llmprovider.LLMProvider
func (a *adaptedProvider) HealthCheck() error {
	return a.inner.HealthCheck()
}

// GetCapabilities implements llmprovider.LLMProvider
func (a *adaptedProvider) GetCapabilities() *digitalvasicmodels.ProviderCapabilities {
	internalCaps := a.inner.GetCapabilities()
	data, err := json.Marshal(internalCaps)
	if err != nil {
		return nil
	}
	var externalCaps digitalvasicmodels.ProviderCapabilities
	if err := json.Unmarshal(data, &externalCaps); err != nil {
		return nil
	}
	return &externalCaps
}

// ValidateConfig implements llmprovider.LLMProvider
func (a *adaptedProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return a.inner.ValidateConfig(config)
}

// Ensure adaptedProvider implements llmprovider.LLMProvider
var _ llmprovider.LLMProvider = (*adaptedProvider)(nil)

// =============================================================================
// Provider Bridge - Adapts services.ProviderRegistry to orchestrator interface
// =============================================================================

// ProviderRegistryBridge adapts the existing services.ProviderRegistry
// to the orchestrator.ProviderRegistry interface.
// It also supports OAuth and CLI-based providers via DebateTeamConfig fallback.
type ProviderRegistryBridge struct {
	registry      *services.ProviderRegistry
	debateTeamCfg *services.DebateTeamConfig
}

// NewProviderRegistryBridge creates a new bridge to the services provider registry.
func NewProviderRegistryBridge(registry *services.ProviderRegistry) *ProviderRegistryBridge {
	return &ProviderRegistryBridge{registry: registry}
}

// SetDebateTeamConfig sets the debate team config so the bridge can look up
// OAuth and CLI-based providers (e.g. Claude, Qwen) that are not in the
// standard provider registry but have verified instances in the debate team.
func (b *ProviderRegistryBridge) SetDebateTeamConfig(teamConfig *services.DebateTeamConfig) {
	b.debateTeamCfg = teamConfig
}

// GetProvider implements orchestrator.ProviderRegistry. It first tries
// the standard registry, then falls back to the debate team config's
// verified provider instances for OAuth/CLI providers.
//
// The return type is interface{} so the bridge satisfies the
// orchestrator-facing contract verbatim; callers that know the concrete
// type (typically llmprovider.LLMProvider) should type-assert. Use
// GetLLMProvider for a typed accessor.
func (b *ProviderRegistryBridge) GetProvider(name string) (interface{}, error) {
	return b.GetLLMProvider(name)
}

// GetLLMProvider is the typed sibling of GetProvider — it returns the
// adapted llmprovider.LLMProvider so HelixAgent code that already has a
// concrete type to consume does not need to type-assert.
func (b *ProviderRegistryBridge) GetLLMProvider(name string) (llmprovider.LLMProvider, error) {
	internalProvider, err := b.registry.GetProvider(name)
	if err == nil {
		return NewAdaptedProvider(internalProvider), nil
	}

	// Fallback: look up from debate team config's verified providers.
	// This handles OAuth providers (Claude, Qwen) and CLI-based providers
	// that are not registered in the standard provider registry.
	if b.debateTeamCfg != nil {
		verifiedLLMs := b.debateTeamCfg.GetVerifiedLLMs()
		for _, vllm := range verifiedLLMs {
			if vllm.ProviderName == name && vllm.Provider != nil {
				return NewAdaptedProvider(vllm.Provider), nil
			}
		}
	}

	return nil, err
}

// GetAvailableProviders implements ProviderRegistry interface.
func (b *ProviderRegistryBridge) GetAvailableProviders() []string {
	return b.registry.ListProviders()
}

// GetProvidersByScore returns providers ordered by LLMsVerifier score.
func (b *ProviderRegistryBridge) GetProvidersByScore() []string {
	return b.registry.ListProvidersOrderedByScore()
}

// GetProviderScore returns the LLMsVerifier score for a provider.
func (b *ProviderRegistryBridge) GetProviderScore(name string) float64 {
	// Score is stored in the verification result, not config
	health := b.registry.GetProviderHealth(name)
	if health != nil && health.Score > 0 {
		return health.Score
	}

	// Fallback to config weight
	config, err := b.registry.GetProviderConfig(name)
	if err != nil || config == nil {
		return 0.0
	}
	return config.Weight
}

// GetProviderModels returns available model names for a provider.
func (b *ProviderRegistryBridge) GetProviderModels(name string) []string {
	config, err := b.registry.GetProviderConfig(name)
	if err != nil || config == nil {
		return nil
	}

	// Extract model names from ModelConfig slice
	models := make([]string, 0, len(config.Models))
	for _, m := range config.Models {
		if m.Enabled && m.Name != "" {
			models = append(models, m.Name)
		} else if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	return models
}

// IsProviderHealthy checks if a provider is healthy.
func (b *ProviderRegistryBridge) IsProviderHealthy(name string) bool {
	health := b.registry.GetProviderHealth(name)
	if health == nil {
		return false
	}
	return health.Verified && health.Status == services.ProviderStatusHealthy
}

// GetAllProviderScores returns a map of provider names to their scores.
func (b *ProviderRegistryBridge) GetAllProviderScores() map[string]float64 {
	scores := make(map[string]float64)
	for _, name := range b.registry.ListProviders() {
		scores[name] = b.GetProviderScore(name)
	}
	return scores
}

// ListProviders implements orchestrator.ProviderRegistry. It returns
// the names of every provider known to the wrapped services registry.
func (b *ProviderRegistryBridge) ListProviders() []string {
	if b == nil || b.registry == nil {
		return nil
	}
	return b.registry.ListProviders()
}

// RegisterProvider implements orchestrator.ProviderRegistry. The wrapped
// services.ProviderRegistry is OAuth/CLI-driven and does not accept
// runtime provider injection through this surface, so callers receive an
// explicit error rather than a silent success.
func (b *ProviderRegistryBridge) RegisterProvider(name string, provider interface{}) error {
	return fmt.Errorf("debate_integration: ProviderRegistryBridge.RegisterProvider(%q) not supported via bridge — use services.ProviderRegistry directly", name)
}

// UnregisterProvider implements orchestrator.ProviderRegistry. Same
// rationale as RegisterProvider.
func (b *ProviderRegistryBridge) UnregisterProvider(name string) error {
	return fmt.Errorf("debate_integration: ProviderRegistryBridge.UnregisterProvider(%q) not supported via bridge — use services.ProviderRegistry directly", name)
}

// Count implements orchestrator.ProviderRegistry. It returns the number
// of providers visible to the bridge (standard registry providers).
// OAuth/CLI verified-team providers are intentionally not counted here
// because they live in a separate config object.
func (b *ProviderRegistryBridge) Count() int {
	if b == nil || b.registry == nil {
		return 0
	}
	return len(b.registry.ListProviders())
}

// Ensure ProviderRegistryBridge implements ProviderRegistry
var _ orchestrator.ProviderRegistry = (*ProviderRegistryBridge)(nil)

// =============================================================================
// Orchestrator Factory - Creates orchestrator with existing services
// =============================================================================

// OrchestratorFactory creates orchestrators integrated with existing services.
type OrchestratorFactory struct {
	providerRegistry *services.ProviderRegistry
}

// NewOrchestratorFactory creates a new orchestrator factory.
func NewOrchestratorFactory(providerRegistry *services.ProviderRegistry) *OrchestratorFactory {
	return &OrchestratorFactory{
		providerRegistry: providerRegistry,
	}
}

// CreateOrchestrator creates a new orchestrator integrated with services.
// lessonBank can be nil for fresh debates without learning history.
func (f *OrchestratorFactory) CreateOrchestrator(config orchestrator.OrchestratorConfig) *orchestrator.Orchestrator {
	orch, _ := f.CreateOrchestratorWithBridge(config)
	return orch
}

// CreateOrchestratorWithBridge creates a new orchestrator and returns both
// the orchestrator and the provider registry bridge. The bridge can be used
// to set the DebateTeamConfig for OAuth/CLI provider fallback lookup.
func (f *OrchestratorFactory) CreateOrchestratorWithBridge(config orchestrator.OrchestratorConfig) (*orchestrator.Orchestrator, *ProviderRegistryBridge) {
	bridge := NewProviderRegistryBridge(f.providerRegistry)

	// Create a fresh lesson bank if none provided
	// In production, this would be retrieved from a persistent store
	lessonBankConfig := defaultLessonBankConfig()
	lessonBank := createLessonBank(lessonBankConfig)

	orch := orchestrator.NewOrchestrator(bridge, lessonBank, config)

	// Auto-register verified providers
	f.registerVerifiedProviders(orch)

	return orch, bridge
}

// CreateOrchestratorWithDefaults creates an orchestrator with default config.
func (f *OrchestratorFactory) CreateOrchestratorWithDefaults() *orchestrator.Orchestrator {
	return f.CreateOrchestrator(orchestrator.DefaultOrchestratorConfig())
}

// registerVerifiedProviders registers all available providers from the registry.
// Providers are registered even if not yet verified, but unhealthy ones are skipped.
// Ollama/local providers are excluded since debate requires remote API providers.
// This ensures the debate system has agents available immediately on startup.
func (f *OrchestratorFactory) registerVerifiedProviders(orch *orchestrator.Orchestrator) {
	if f.providerRegistry == nil {
		return
	}

	providers := f.providerRegistry.ListProvidersOrderedByScore()

	for _, name := range providers {
		// Skip Ollama/local providers - debate requires remote API providers
		nameLower := strings.ToLower(name)
		if nameLower == "ollama" || nameLower == "local" {
			continue
		}

		config, err := f.providerRegistry.GetProviderConfig(name)
		if err != nil || config == nil {
			continue
		}

		// Check health status - only skip if explicitly unhealthy (not just unverified)
		health := f.providerRegistry.GetProviderHealth(name)
		if health != nil && health.Status == services.ProviderStatusUnhealthy {
			continue // Skip explicitly unhealthy providers
		}

		// Get score from health verification, or use default/config weight
		score := 5.0 // Default minimum score
		if health != nil && health.Score > 0 {
			score = health.Score
		} else if config.Weight > 0 {
			score = config.Weight
		}

		// Register each enabled model for this provider
		for _, model := range config.Models {
			if !model.Enabled {
				continue
			}

			modelName := model.Name
			if modelName == "" {
				modelName = model.ID
			}

			if modelName != "" {
				_ = orch.RegisterProvider(name, modelName, score) //nolint:errcheck
			}
		}
	}
}

// =============================================================================
// Helper functions for lesson bank integration
// =============================================================================

// defaultLessonBankConfig returns a default lesson bank configuration.
func defaultLessonBankConfig() debate.LessonBankConfig {
	config := debate.DefaultLessonBankConfig()
	config.EnableSemanticSearch = false // Disabled by default until embeddings configured
	return config
}

// createLessonBank creates a new lesson bank with the given config.
func createLessonBank(config debate.LessonBankConfig) *debate.LessonBank {
	return debate.NewLessonBank(config, nil, nil)
}
