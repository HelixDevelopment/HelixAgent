// Package siliconflow provides a SiliconFlow provider implementation.
package siliconflow

import (
	"context"
	"fmt"
	"log"

	"github.com/HelixDevelopment/HelixAgent/Toolkit/pkg/toolkit"
)

func init() {
	toolkit.RegisterProviderFactory("siliconflow", NewProvider)
}

// Provider implements the Provider interface for SiliconFlow.
type Provider struct {
	client    *Client
	discovery *Discovery
	config    *Config
}

// NewProvider creates a new SiliconFlow provider.
func NewProvider(config map[string]interface{}) (toolkit.Provider, error) {
	builder := NewConfigBuilder()
	cfg, err := builder.Build(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	sfConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	return &Provider{
		client:    NewClient(sfConfig.APIKey),
		discovery: NewDiscovery(sfConfig.APIKey),
		config:    sfConfig,
	}, nil
}

// Name returns the name of the provider.
func (p *Provider) Name() string {
	return "siliconflow"
}

// Chat performs a chat completion request.
func (p *Provider) Chat(ctx context.Context, req toolkit.ChatRequest) (toolkit.ChatResponse, error) {
	log.Printf("SiliconFlow: Performing chat completion with model %s", req.Model)
	return p.client.ChatCompletion(ctx, req)
}

// Embed performs an embedding request.
func (p *Provider) Embed(ctx context.Context, req toolkit.EmbeddingRequest) (toolkit.EmbeddingResponse, error) {
	log.Printf("SiliconFlow: Performing embedding with model %s", req.Model)
	return p.client.CreateEmbeddings(ctx, req)
}

// Rerank performs a rerank request.
func (p *Provider) Rerank(ctx context.Context, req toolkit.RerankRequest) (toolkit.RerankResponse, error) {
	log.Printf("SiliconFlow: Performing rerank with model %s", req.Model)
	return p.client.CreateRerank(ctx, req)
}

// DiscoverModels discovers available models from the provider.
func (p *Provider) DiscoverModels(ctx context.Context) ([]toolkit.ModelInfo, error) {
	log.Println("SiliconFlow: Discovering models")
	return p.discovery.Discover(ctx)
}

// ValidateConfig validates the provider configuration.
func (p *Provider) ValidateConfig(config map[string]interface{}) error {
	builder := NewConfigBuilder()
	_, err := builder.Build(config)
	return err
}

// Factory function for creating SiliconFlow providers.
func Factory(config map[string]interface{}) (toolkit.Provider, error) {
	return NewProvider(config)
}

// Register registers the SiliconFlow provider with the registry.
func Register(registry *toolkit.ProviderFactoryRegistry) error {
	return registry.Register("siliconflow", Factory)
}

// Global registry for auto-registration
var globalProviderRegistry *toolkit.ProviderFactoryRegistry

// SetGlobalProviderRegistry sets the global provider registry for auto-registration.
func SetGlobalProviderRegistry(registry *toolkit.ProviderFactoryRegistry) {
	globalProviderRegistry = registry
}

// init registers the SiliconFlow provider when the package is imported.
func init() {
	// Register with global registry if available
	if globalProviderRegistry != nil {
		_ = globalProviderRegistry.Register("siliconflow", Factory)
	}
}
