package debate_integration

import (
	"context"
	"encoding/json"
	"fmt"

	"dev.helix.agent/internal/models"
	"digital.vasic.llmprovider"
	digitalvasicmodels "digital.vasic.llmprovider/pkg/models"
)

// Helper conversion functions for tests (similar to provider_bridge.go)

func convertToInternalRequest(req *digitalvasicmodels.LLMRequest) (*models.LLMRequest, error) {
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

func convertToInternalResponse(resp *models.LLMResponse) (*digitalvasicmodels.LLMResponse, error) {
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

func convertToInternalCapabilities(caps *models.ProviderCapabilities) (*digitalvasicmodels.ProviderCapabilities, error) {
	data, err := json.Marshal(caps)
	if err != nil {
		return nil, err
	}
	var externalCaps digitalvasicmodels.ProviderCapabilities
	if err := json.Unmarshal(data, &externalCaps); err != nil {
		return nil, err
	}
	return &externalCaps, nil
}

// =============================================================================
// Mock Provider Registry
// =============================================================================

type mockProviderRegistry struct {
	providers map[string]*mockLLMProvider
}

func newMockProviderRegistry() *mockProviderRegistry {
	return &mockProviderRegistry{
		providers: make(map[string]*mockLLMProvider),
	}
}

func (r *mockProviderRegistry) GetProvider(name string) (interface{}, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

func (r *mockProviderRegistry) GetAvailableProviders() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func (r *mockProviderRegistry) AddProvider(name string, provider *mockLLMProvider) {
	r.providers[name] = provider
}

// GetProvidersByScore returns provider names. Mock just returns them in
// insertion order — score is not tracked.
func (r *mockProviderRegistry) GetProvidersByScore() []string {
	return r.GetAvailableProviders()
}

// IsProviderHealthy reports whether the named mock provider exists.
func (r *mockProviderRegistry) IsProviderHealthy(name string) bool {
	_, ok := r.providers[name]
	return ok
}

// ListProviders is an alias for GetAvailableProviders to satisfy the
// orchestrator.ProviderRegistry interface.
func (r *mockProviderRegistry) ListProviders() []string {
	return r.GetAvailableProviders()
}

// RegisterProvider adds a provider under the given name. The mock only
// accepts *mockLLMProvider values; any other type returns an error.
func (r *mockProviderRegistry) RegisterProvider(name string, provider interface{}) error {
	mp, ok := provider.(*mockLLMProvider)
	if !ok {
		return fmt.Errorf("mockProviderRegistry: only *mockLLMProvider accepted, got %T", provider)
	}
	r.providers[name] = mp
	return nil
}

// UnregisterProvider removes a provider by name.
func (r *mockProviderRegistry) UnregisterProvider(name string) error {
	delete(r.providers, name)
	return nil
}

// Count returns the number of registered providers.
func (r *mockProviderRegistry) Count() int {
	return len(r.providers)
}

// =============================================================================
// Mock LLM Provider
// =============================================================================

type mockLLMProvider struct {
	name      string
	responses map[string]string
	callCount int
}

func newMockLLMProvider(name string) *mockLLMProvider {
	return &mockLLMProvider{
		name:      name,
		responses: make(map[string]string),
	}
}

func (p *mockLLMProvider) Complete(ctx context.Context, request *digitalvasicmodels.LLMRequest) (*digitalvasicmodels.LLMResponse, error) {
	p.callCount++

	// Generate a mock response based on the model
	content := "This is a thoughtful analysis of the topic. Key points:\n"
	content += "- First point: Important consideration\n"
	content += "- Second point: Another key insight\n"
	content += "- Third point: Supporting evidence\n"
	content += "\nConfidence: 85%"

	// Create internal response then convert
	internalResp := &models.LLMResponse{
		Content:      content,
		ProviderName: p.name,
		TokensUsed:   150,
		FinishReason: "stop",
	}
	return convertToInternalResponse(internalResp)
}

func (p *mockLLMProvider) CompleteStream(ctx context.Context, request *digitalvasicmodels.LLMRequest) (<-chan *digitalvasicmodels.LLMResponse, error) {
	ch := make(chan *digitalvasicmodels.LLMResponse, 1)
	go func() {
		response, _ := p.Complete(ctx, request)
		ch <- response
		close(ch)
	}()
	return ch, nil
}

func (p *mockLLMProvider) HealthCheck() error {
	return nil
}

func (p *mockLLMProvider) GetCapabilities() *digitalvasicmodels.ProviderCapabilities {
	internalCaps := &models.ProviderCapabilities{
		SupportsStreaming: true,
		SupportsTools:     true,
	}
	externalCaps, err := convertToInternalCapabilities(internalCaps)
	if err != nil {
		return nil
	}
	return externalCaps
}

func (p *mockLLMProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

// Ensure mockLLMProvider implements llmprovider.LLMProvider
var _ llmprovider.LLMProvider = (*mockLLMProvider)(nil)
