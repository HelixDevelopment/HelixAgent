package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

// ProviderRegistry manages LLM provider registration and configuration
type ProviderRegistry struct {
	providers      map[string]llm.LLMProvider
	ensemble       *EnsembleService
	requestService *RequestService
	memory         *MemoryService
	mu             sync.RWMutex
}

// ProviderConfig holds configuration for an LLM provider
type ProviderConfig struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Enabled        bool              `json:"enabled"`
	APIKey         string            `json:"api_key"`
	BaseURL        string            `json:"base_url"`
	Models         []ModelConfig     `json:"models"`
	Timeout        time.Duration     `json:"timeout"`
	MaxRetries     int               `json:"max_retries"`
	HealthCheckURL string            `json:"health_check_url"`
	Weight         float64           `json:"weight"`
	Tags           []string          `json:"tags"`
	Capabilities   map[string]string `json:"capabilities"`
	CustomSettings map[string]any    `json:"custom_settings"`
}

// ModelConfig holds configuration for a specific model
type ModelConfig struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Enabled      bool           `json:"enabled"`
	Weight       float64        `json:"weight"`
	Capabilities []string       `json:"capabilities"`
	CustomParams map[string]any `json:"custom_params"`
}

// RegistryConfig holds configuration for provider registry
type RegistryConfig struct {
	DefaultTimeout time.Duration              `json:"default_timeout"`
	MaxRetries     int                        `json:"max_retries"`
	HealthCheck    HealthCheckConfig          `json:"health_check"`
	Providers      map[string]*ProviderConfig `json:"providers"`
	Ensemble       *models.EnsembleConfig     `json:"ensemble"`
	Routing        *RoutingConfig             `json:"routing"`
}

// HealthCheckConfig holds health check configuration
type HealthCheckConfig struct {
	Enabled          bool          `json:"enabled"`
	Interval         time.Duration `json:"interval"`
	Timeout          time.Duration `json:"timeout"`
	FailureThreshold int           `json:"failure_threshold"`
}

// RoutingConfig holds routing configuration
type RoutingConfig struct {
	Strategy string             `json:"strategy"`
	Weights  map[string]float64 `json:"weights"`
}

func NewProviderRegistry(cfg *RegistryConfig, memory *MemoryService) *ProviderRegistry {
	if cfg == nil {
		cfg = getDefaultRegistryConfig()
	}

	registry := &ProviderRegistry{
		providers: make(map[string]llm.LLMProvider),
		memory:    memory,
	}

	// Initialize ensemble service
	ensembleStrategy := "confidence_weighted"
	if cfg.Ensemble != nil {
		ensembleStrategy = cfg.Ensemble.Strategy
	}
	registry.ensemble = NewEnsembleService(ensembleStrategy, cfg.DefaultTimeout)

	// Initialize request service
	routingStrategy := "weighted"
	if cfg.Routing != nil {
		routingStrategy = cfg.Routing.Strategy
	}
	registry.requestService = NewRequestService(routingStrategy, registry.ensemble, memory)

	// Register default providers
	registry.registerDefaultProviders(cfg)

	return registry
}

func (r *ProviderRegistry) registerDefaultProviders(cfg *RegistryConfig) {
	// Register DeepSeek provider
	deepseekConfig := cfg.Providers["deepseek"]
	if deepseekConfig == nil {
		deepseekConfig = &ProviderConfig{
			Name:    "deepseek",
			Type:    "deepseek",
			Enabled: true,
			Models: []ModelConfig{{
				ID:      "deepseek-coder",
				Name:    "DeepSeek Coder",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	if deepseekConfig.Enabled {
		r.RegisterProvider(deepseekConfig.Name, llm.NewDeepSeekProvider(
			deepseekConfig.APIKey,
			deepseekConfig.BaseURL,
			deepseekConfig.Models[0].ID,
		))
	}

	// Register Claude provider
	claudeConfig := cfg.Providers["claude"]
	if claudeConfig == nil {
		claudeConfig = &ProviderConfig{
			Name:    "claude",
			Type:    "claude",
			Enabled: true,
			Models: []ModelConfig{{
				ID:      "claude-3-sonnet-20240229",
				Name:    "Claude 3 Sonnet",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	if claudeConfig.Enabled {
		r.RegisterProvider(claudeConfig.Name, llm.NewClaudeProvider(
			claudeConfig.APIKey,
			claudeConfig.BaseURL,
			claudeConfig.Models[0].ID,
		))
	}

	// Register Gemini provider
	geminiConfig := cfg.Providers["gemini"]
	if geminiConfig == nil {
		geminiConfig = &ProviderConfig{
			Name:    "gemini",
			Type:    "gemini",
			Enabled: true,
			Models: []ModelConfig{{
				ID:      "gemini-pro",
				Name:    "Gemini Pro",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	if geminiConfig.Enabled {
		r.RegisterProvider(geminiConfig.Name, llm.NewGeminiProvider(
			geminiConfig.APIKey,
			geminiConfig.BaseURL,
			geminiConfig.Models[0].ID,
		))
	}

	// Register Qwen provider
	qwenConfig := cfg.Providers["qwen"]
	if qwenConfig == nil {
		qwenConfig = &ProviderConfig{
			Name:    "qwen",
			Type:    "qwen",
			Enabled: true,
			Models: []ModelConfig{{
				ID:      "qwen-turbo",
				Name:    "Qwen Turbo",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	if qwenConfig.Enabled {
		r.RegisterProvider(qwenConfig.Name, llm.NewQwenProvider(
			qwenConfig.APIKey,
			qwenConfig.BaseURL,
			qwenConfig.Models[0].ID,
		))
	}

	// Register OpenRouter provider
	openrouterConfig := cfg.Providers["openrouter"]
	if openrouterConfig == nil {
		openrouterConfig = &ProviderConfig{
			Name:    "openrouter",
			Type:    "openrouter",
			Enabled: true,
			Models: []ModelConfig{{
				ID:      "x-ai/grok-4",
				Name:    "Grok-4 via OpenRouter",
				Enabled: true,
				Weight:  1.3,
			}},
		}
	}
	if openrouterConfig.Enabled {
		r.RegisterProvider(openrouterConfig.Name, llm.NewOpenRouterProvider(
			openrouterConfig.APIKey,
		))
	}
}

func (r *ProviderRegistry) RegisterProvider(name string, provider llm.LLMProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	r.providers[name] = provider

	// Also register with ensemble and request services
	r.ensemble.RegisterProvider(name, &providerAdapter{provider: provider})
	r.requestService.RegisterProvider(name, &providerAdapter{provider: provider})

	return nil
}

func (r *ProviderRegistry) UnregisterProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	delete(r.providers, name)
	r.ensemble.RemoveProvider(name)
	r.requestService.RemoveProvider(name)

	return nil
}

func (r *ProviderRegistry) GetProvider(name string) (llm.LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

func (r *ProviderRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func (r *ProviderRegistry) GetEnsembleService() *EnsembleService {
	return r.ensemble
}

func (r *ProviderRegistry) GetRequestService() *RequestService {
	return r.requestService
}

func (r *ProviderRegistry) ConfigureProvider(name string, config *ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[name]
	if !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	// Re-register provider with new configuration if needed
	if !config.Enabled {
		return r.UnregisterProvider(name)
	}

	// For now, we assume provider is already configured
	// In a real implementation, you might need to reinitialize the provider
	_ = provider

	return nil
}

func (r *ProviderRegistry) GetProviderConfig(name string) (*ProviderConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// For now, return a basic config
	// In a real implementation, this would return stored configuration
	if _, exists := r.providers[name]; !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return &ProviderConfig{
		Name:    name,
		Enabled: true,
	}, nil
}

func (r *ProviderRegistry) HealthCheck() map[string]error {
	r.mu.RLock()
	providers := make(map[string]llm.LLMProvider)
	for name, provider := range r.providers {
		providers[name] = provider
	}
	r.mu.RUnlock()

	results := make(map[string]error)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, provider := range providers {
		wg.Add(1)
		go func(name string, provider llm.LLMProvider) {
			defer wg.Done()

			err := provider.HealthCheck()

			mu.Lock()
			results[name] = err
			mu.Unlock()
		}(name, provider)
	}

	wg.Wait()
	return results
}

// providerAdapter adapts llm.LLMProvider to services.LLMProvider interface
type providerAdapter struct {
	provider llm.LLMProvider
}

func (a *providerAdapter) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Note: llm.LLMProvider interface doesn't have context parameter
	// This is a limitation of current interface design
	return a.provider.Complete(req)
}

func (a *providerAdapter) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	// The llm.LLMProvider interface doesn't support streaming yet
	// This would need to be added to the interface
	ch := make(chan *models.LLMResponse)
	close(ch)
	return ch, fmt.Errorf("streaming not supported by provider interface")
}

func getDefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
		HealthCheck: HealthCheckConfig{
			Enabled:          true,
			Interval:         60 * time.Second,
			Timeout:          10 * time.Second,
			FailureThreshold: 3,
		},
		Providers: make(map[string]*ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy:            "confidence_weighted",
			MinProviders:        2,
			ConfidenceThreshold: 0.8,
			FallbackToBest:      true,
			Timeout:             30,
			PreferredProviders:  []string{},
		},
		Routing: &RoutingConfig{
			Strategy: "weighted",
			Weights:  make(map[string]float64),
		},
	}
}

// LoadRegistryConfigFromAppConfig converts application config to registry config
func LoadRegistryConfigFromAppConfig(appConfig *config.Config) *RegistryConfig {
	cfg := getDefaultRegistryConfig()

	// Override with application config
	if appConfig.LLM.DefaultTimeout > 0 {
		cfg.DefaultTimeout = appConfig.LLM.DefaultTimeout
	}

	if appConfig.LLM.MaxRetries > 0 {
		cfg.MaxRetries = appConfig.LLM.MaxRetries
	}

	// Load provider configurations from environment variables or config
	// This is a simplified implementation
	cfg.Providers["deepseek"] = &ProviderConfig{
		Name:    "deepseek",
		Type:    "deepseek",
		Enabled: true,
		Models: []ModelConfig{{
			ID:      "deepseek-coder",
			Name:    "DeepSeek Coder",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  "", // Should come from config or env
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	cfg.Providers["claude"] = &ProviderConfig{
		Name:    "claude",
		Type:    "claude",
		Enabled: true,
		Models: []ModelConfig{{
			ID:      "claude-3-sonnet-20240229",
			Name:    "Claude 3 Sonnet",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  "", // Should come from config or env
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	cfg.Providers["gemini"] = &ProviderConfig{
		Name:    "gemini",
		Type:    "gemini",
		Enabled: true,
		Models: []ModelConfig{{
			ID:      "gemini-pro",
			Name:    "Gemini Pro",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  "", // Should come from config or env
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	cfg.Providers["qwen"] = &ProviderConfig{
		Name:    "qwen",
		Type:    "qwen",
		Enabled: true,
		Models: []ModelConfig{{
			ID:      "qwen-turbo",
			Name:    "Qwen Turbo",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  "", // Should come from config or env
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	cfg.Providers["openrouter"] = &ProviderConfig{
		Name:    "openrouter",
		Type:    "openrouter",
		Enabled: true,
		Models: []ModelConfig{{
			ID:      "x-ai/grok-4",
			Name:    "Grok-4 via OpenRouter",
			Enabled: true,
			Weight:  1.3,
		}},
		APIKey:  "", // Should come from config or env
		Timeout: cfg.DefaultTimeout,
		Weight:  1.3,
	}

	return cfg
}
