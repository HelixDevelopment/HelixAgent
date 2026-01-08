package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"dev.helix.agent/internal/auth/oauth_credentials"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/llm/providers/claude"
	"dev.helix.agent/internal/llm/providers/deepseek"
	"dev.helix.agent/internal/llm/providers/gemini"
	"dev.helix.agent/internal/llm/providers/openrouter"
	"dev.helix.agent/internal/llm/providers/qwen"
	"dev.helix.agent/internal/models"
)

// ProviderRegistry manages LLM provider registration and configuration
type ProviderRegistry struct {
	providers        map[string]llm.LLMProvider
	circuitBreakers  map[string]*CircuitBreaker
	providerConfigs  map[string]*ProviderConfig             // Stores provider configurations
	providerHealth   map[string]*ProviderVerificationResult // Stores provider health verification results
	activeRequests   map[string]*int64                      // Atomic counters for active requests per provider
	config           *RegistryConfig
	ensemble         *EnsembleService
	requestService   *RequestService
	memory           *MemoryService
	discovery        *ProviderDiscovery // Auto-discovery service for environment-based provider detection
	mu               sync.RWMutex
	drainTimeout     time.Duration // Timeout for graceful shutdown request draining
	autoDiscovery    bool          // Whether auto-discovery is enabled
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

// ProviderHealthStatus represents the verified health status of a provider
type ProviderHealthStatus string

const (
	ProviderStatusUnknown     ProviderHealthStatus = "unknown"
	ProviderStatusHealthy     ProviderHealthStatus = "healthy"
	ProviderStatusRateLimited ProviderHealthStatus = "rate_limited"
	ProviderStatusAuthFailed  ProviderHealthStatus = "auth_failed"
	ProviderStatusUnhealthy   ProviderHealthStatus = "unhealthy"
)

// ProviderVerificationResult contains the result of verifying a provider
type ProviderVerificationResult struct {
	Provider     string               `json:"provider"`
	Status       ProviderHealthStatus `json:"status"`
	Verified     bool                 `json:"verified"`
	ResponseTime time.Duration        `json:"response_time_ms"`
	Error        string               `json:"error,omitempty"`
	TestedAt     time.Time            `json:"tested_at"`
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
	CircuitBreaker CircuitBreakerConfig       `json:"circuit_breaker"`
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

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`
	SuccessThreshold int           `json:"success_threshold"`
}

// circuitBreakerProvider wraps an LLMProvider with circuit breaker functionality
type circuitBreakerProvider struct {
	provider       llm.LLMProvider
	circuitBreaker *CircuitBreaker
	name           string
}

// Complete wraps the provider's Complete method with circuit breaker protection
func (cbp *circuitBreakerProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	var resp *models.LLMResponse
	err := cbp.circuitBreaker.Call(func() error {
		var callErr error
		resp, callErr = cbp.provider.Complete(ctx, req)
		return callErr
	})
	return resp, err
}

// CompleteStream wraps the provider's CompleteStream method with circuit breaker protection
func (cbp *circuitBreakerProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	var stream <-chan *models.LLMResponse
	err := cbp.circuitBreaker.Call(func() error {
		var callErr error
		stream, callErr = cbp.provider.CompleteStream(ctx, req)
		return callErr
	})
	return stream, err
}

// HealthCheck wraps the provider's HealthCheck method with circuit breaker protection
func (cbp *circuitBreakerProvider) HealthCheck() error {
	return cbp.circuitBreaker.Call(func() error {
		return cbp.provider.HealthCheck()
	})
}

// GetCapabilities delegates to the underlying provider
func (cbp *circuitBreakerProvider) GetCapabilities() *models.ProviderCapabilities {
	return cbp.provider.GetCapabilities()
}

// ValidateConfig delegates to the underlying provider
func (cbp *circuitBreakerProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return cbp.provider.ValidateConfig(config)
}

func NewProviderRegistry(cfg *RegistryConfig, memory *MemoryService) *ProviderRegistry {
	return newProviderRegistry(cfg, memory, true) // Enable auto-discovery by default
}

// NewProviderRegistryWithoutAutoDiscovery creates a provider registry without auto-discovery
// This is useful for testing where you want to control exactly which providers are registered
func NewProviderRegistryWithoutAutoDiscovery(cfg *RegistryConfig, memory *MemoryService) *ProviderRegistry {
	return newProviderRegistry(cfg, memory, false)
}

func newProviderRegistry(cfg *RegistryConfig, memory *MemoryService, enableAutoDiscovery bool) *ProviderRegistry {
	if cfg == nil {
		cfg = getDefaultRegistryConfig()
	}

	// Initialize logger for registry
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	registry := &ProviderRegistry{
		providers:       make(map[string]llm.LLMProvider),
		circuitBreakers: make(map[string]*CircuitBreaker),
		providerConfigs: make(map[string]*ProviderConfig),
		providerHealth:  make(map[string]*ProviderVerificationResult),
		activeRequests:  make(map[string]*int64),
		config:          cfg,
		memory:          memory,
		drainTimeout:    30 * time.Second, // Default 30 second drain timeout
		autoDiscovery:   enableAutoDiscovery,
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

	// Register providers from config file (backward compatibility)
	registry.registerDefaultProviders(cfg)

	// Auto-discover additional providers from environment variables
	// This supplements config file providers with any additional API keys found
	if enableAutoDiscovery {
		registry.initAutoDiscovery(logger)
	}

	return registry
}

// initAutoDiscovery initializes the auto-discovery service and discovers providers from env vars
func (r *ProviderRegistry) initAutoDiscovery(logger *logrus.Logger) {
	if !r.autoDiscovery {
		return
	}

	// Create discovery service (verify on startup disabled - we'll do it on-demand)
	r.discovery = NewProviderDiscovery(logger, false)

	// Discover providers from environment variables
	discovered, err := r.discovery.DiscoverProviders()
	if err != nil {
		logger.WithError(err).Warn("Provider auto-discovery failed")
		return
	}

	if len(discovered) == 0 {
		logger.Info("No additional providers discovered from environment")
		return
	}

	// Register discovered providers that aren't already registered via config
	existingProviders := r.ListProviders()
	existingMap := make(map[string]bool)
	for _, name := range existingProviders {
		existingMap[name] = true
	}

	newProviders := 0
	for _, dp := range discovered {
		// Skip if already registered via config (config takes precedence)
		if existingMap[dp.Name] {
			logger.WithField("provider", dp.Name).Debug("Provider already registered via config, skipping auto-discovery")
			continue
		}

		// Register the discovered provider
		if dp.Provider != nil {
			if err := r.RegisterProvider(dp.Name, dp.Provider); err != nil {
				logger.WithError(err).WithField("provider", dp.Name).Warn("Failed to register auto-discovered provider")
			} else {
				newProviders++
				logger.WithFields(logrus.Fields{
					"provider": dp.Name,
					"type":     dp.Type,
					"env_var":  dp.APIKeyEnvVar,
				}).Info("Auto-discovered and registered provider from environment")
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"discovered": len(discovered),
		"registered": newProviders,
		"skipped":    len(discovered) - newProviders,
	}).Info("Provider auto-discovery completed")
}

// GetDiscovery returns the provider discovery service
func (r *ProviderRegistry) GetDiscovery() *ProviderDiscovery {
	return r.discovery
}

// DiscoverAndVerifyProviders runs provider discovery and verification
// Returns a summary of discovered and verified providers
func (r *ProviderRegistry) DiscoverAndVerifyProviders(ctx context.Context) map[string]interface{} {
	if r.discovery == nil {
		return map[string]interface{}{
			"error":   "auto-discovery not initialized",
			"enabled": false,
		}
	}

	// Verify all discovered providers
	r.discovery.VerifyAllProviders(ctx)

	// Get and return summary
	summary := r.discovery.Summary()
	summary["auto_discovery_enabled"] = r.autoDiscovery

	return summary
}

// GetBestProvidersForDebate returns the best verified providers for the debate group
func (r *ProviderRegistry) GetBestProvidersForDebate(minProviders, maxProviders int) []string {
	if r.discovery == nil {
		// Fall back to healthy providers from verification
		return r.GetHealthyProviders()
	}

	bestProviders := r.discovery.GetDebateGroupProviders(minProviders, maxProviders)
	names := make([]string, 0, len(bestProviders))
	for _, p := range bestProviders {
		names = append(names, p.Name)
	}
	return names
}

// SetAutoDiscovery enables or disables auto-discovery
func (r *ProviderRegistry) SetAutoDiscovery(enabled bool) {
	r.mu.Lock()
	r.autoDiscovery = enabled
	r.mu.Unlock()
}

func (r *ProviderRegistry) registerDefaultProviders(cfg *RegistryConfig) {
	// Register DeepSeek provider (only if API key is configured)
	deepseekConfig := cfg.Providers["deepseek"]
	if deepseekConfig == nil {
		deepseekConfig = &ProviderConfig{
			Name:    "deepseek",
			Type:    "deepseek",
			Enabled: false, // Disabled by default - requires API key
			Models: []ModelConfig{{
				ID:      "deepseek-coder",
				Name:    "DeepSeek Coder",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	if deepseekConfig.Enabled && deepseekConfig.APIKey != "" {
		r.RegisterProvider(deepseekConfig.Name, deepseek.NewDeepSeekProvider(
			deepseekConfig.APIKey,
			deepseekConfig.BaseURL,
			deepseekConfig.Models[0].ID,
		))
	}

	// Register Claude provider (using OAuth if enabled and available, otherwise API key)
	claudeConfig := cfg.Providers["claude"]
	if claudeConfig == nil {
		claudeConfig = &ProviderConfig{
			Name:    "claude",
			Type:    "claude",
			Enabled: false, // Disabled by default - requires API key or OAuth
			Models: []ModelConfig{{
				ID:      "claude-3-sonnet-20240229",
				Name:    "Claude 3 Sonnet",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	// Try OAuth first if enabled
	if claudeConfig.Enabled || oauth_credentials.IsClaudeOAuthEnabled() {
		if oauth_credentials.IsClaudeOAuthEnabled() {
			credReader := oauth_credentials.GetGlobalReader()
			if credReader.HasValidClaudeCredentials() {
				claudeProvider, err := claude.NewClaudeProviderWithOAuth(
					claudeConfig.BaseURL,
					claudeConfig.Models[0].ID,
				)
				if err == nil {
					r.RegisterProvider(claudeConfig.Name, claudeProvider)
					logrus.Info("Registered Claude provider with OAuth credentials from Claude Code CLI")
				} else {
					logrus.Warnf("Failed to create Claude OAuth provider: %v, falling back to API key", err)
				}
			}
		}
		// Fall back to API key if OAuth not available
		if _, err := r.GetProvider(claudeConfig.Name); err != nil && claudeConfig.APIKey != "" {
			r.RegisterProvider(claudeConfig.Name, claude.NewClaudeProvider(
				claudeConfig.APIKey,
				claudeConfig.BaseURL,
				claudeConfig.Models[0].ID,
			))
			logrus.Info("Registered Claude provider with API key")
		}
	}

	// Register Gemini provider (only if API key is configured)
	geminiConfig := cfg.Providers["gemini"]
	if geminiConfig == nil {
		geminiConfig = &ProviderConfig{
			Name:    "gemini",
			Type:    "gemini",
			Enabled: false, // Disabled by default - requires API key
			Models: []ModelConfig{{
				ID:      "gemini-pro",
				Name:    "Gemini Pro",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	if geminiConfig.Enabled && geminiConfig.APIKey != "" {
		r.RegisterProvider(geminiConfig.Name, gemini.NewGeminiProvider(
			geminiConfig.APIKey,
			geminiConfig.BaseURL,
			geminiConfig.Models[0].ID,
		))
	}

	// Register Qwen provider (using OAuth if enabled and available, otherwise API key)
	qwenConfig := cfg.Providers["qwen"]
	if qwenConfig == nil {
		qwenConfig = &ProviderConfig{
			Name:    "qwen",
			Type:    "qwen",
			Enabled: false, // Disabled by default - requires API key or OAuth
			Models: []ModelConfig{{
				ID:      "qwen-turbo",
				Name:    "Qwen Turbo",
				Enabled: true,
				Weight:  1.0,
			}},
		}
	}
	// Try OAuth first if enabled
	if qwenConfig.Enabled || oauth_credentials.IsQwenOAuthEnabled() {
		if oauth_credentials.IsQwenOAuthEnabled() {
			credReader := oauth_credentials.GetGlobalReader()
			if credReader.HasValidQwenCredentials() {
				qwenProvider, err := qwen.NewQwenProviderWithOAuth(
					qwenConfig.BaseURL,
					qwenConfig.Models[0].ID,
				)
				if err == nil {
					r.RegisterProvider(qwenConfig.Name, qwenProvider)
					logrus.Info("Registered Qwen provider with OAuth credentials from Qwen Code CLI")
				} else {
					logrus.Warnf("Failed to create Qwen OAuth provider: %v, falling back to API key", err)
				}
			}
		}
		// Fall back to API key if OAuth not available
		if _, err := r.GetProvider(qwenConfig.Name); err != nil && qwenConfig.APIKey != "" {
			r.RegisterProvider(qwenConfig.Name, qwen.NewQwenProvider(
				qwenConfig.APIKey,
				qwenConfig.BaseURL,
				qwenConfig.Models[0].ID,
			))
			logrus.Info("Registered Qwen provider with API key")
		}
	}

	// Register OpenRouter provider (only if API key is configured)
	openrouterConfig := cfg.Providers["openrouter"]
	if openrouterConfig == nil {
		openrouterConfig = &ProviderConfig{
			Name:    "openrouter",
			Type:    "openrouter",
			Enabled: false, // Disabled by default - requires API key
			Models: []ModelConfig{{
				ID:      "x-ai/grok-4",
				Name:    "Grok-4 via OpenRouter",
				Enabled: true,
				Weight:  1.3,
			}},
		}
	}
	if openrouterConfig.Enabled && openrouterConfig.APIKey != "" {
		r.RegisterProvider(openrouterConfig.Name, openrouter.NewSimpleOpenRouterProvider(
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

	// Wrap provider with circuit breaker if enabled
	var wrappedProvider llm.LLMProvider = provider
	if r.config.CircuitBreaker.Enabled {
		cb := NewCircuitBreaker(
			r.config.CircuitBreaker.FailureThreshold,
			r.config.CircuitBreaker.SuccessThreshold,
			r.config.CircuitBreaker.RecoveryTimeout,
		)
		r.circuitBreakers[name] = cb
		wrappedProvider = &circuitBreakerProvider{
			provider:       provider,
			circuitBreaker: cb,
			name:           name,
		}
	}

	r.providers[name] = wrappedProvider

	// Initialize atomic counter for active requests
	var counter int64
	r.activeRequests[name] = &counter

	// Also register with ensemble and request services
	r.ensemble.RegisterProvider(name, &providerAdapter{provider: wrappedProvider})
	r.requestService.RegisterProvider(name, &providerAdapter{provider: wrappedProvider})

	return nil
}

// GetCircuitBreaker returns the circuit breaker for a provider (for internal use)
func (r *ProviderRegistry) GetCircuitBreaker(name string) *CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.circuitBreakers[name]
}

func (r *ProviderRegistry) UnregisterProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.unregisterProviderLocked(name)
}

// unregisterProviderLocked removes a provider (caller must hold the lock)
func (r *ProviderRegistry) unregisterProviderLocked(name string) error {
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

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	// If disabling the provider, unregister it
	if !config.Enabled {
		return r.unregisterProviderLocked(name)
	}

	// Store or update the configuration in memory
	// Make a copy to avoid external modification
	storedConfig := &ProviderConfig{
		Name:           config.Name,
		Type:           config.Type,
		Enabled:        config.Enabled,
		APIKey:         config.APIKey,
		BaseURL:        config.BaseURL,
		Timeout:        config.Timeout,
		MaxRetries:     config.MaxRetries,
		HealthCheckURL: config.HealthCheckURL,
		Weight:         config.Weight,
		Tags:           make([]string, len(config.Tags)),
		Capabilities:   make(map[string]string),
		CustomSettings: make(map[string]any),
	}

	// Copy slices and maps
	copy(storedConfig.Tags, config.Tags)
	for k, v := range config.Capabilities {
		storedConfig.Capabilities[k] = v
	}
	for k, v := range config.CustomSettings {
		storedConfig.CustomSettings[k] = v
	}

	// Copy models
	if len(config.Models) > 0 {
		storedConfig.Models = make([]ModelConfig, len(config.Models))
		for i, m := range config.Models {
			storedConfig.Models[i] = ModelConfig{
				ID:           m.ID,
				Name:         m.Name,
				Enabled:      m.Enabled,
				Weight:       m.Weight,
				Capabilities: make([]string, len(m.Capabilities)),
				CustomParams: make(map[string]any),
			}
			copy(storedConfig.Models[i].Capabilities, m.Capabilities)
			for k, v := range m.CustomParams {
				storedConfig.Models[i].CustomParams[k] = v
			}
		}
	}

	r.providerConfigs[name] = storedConfig

	return nil
}

func (r *ProviderRegistry) GetProviderConfig(name string) (*ProviderConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// First check if provider exists
	if _, exists := r.providers[name]; !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	// Return stored configuration if available
	if storedConfig, exists := r.providerConfigs[name]; exists {
		// Return a copy to prevent external modification
		configCopy := &ProviderConfig{
			Name:           storedConfig.Name,
			Type:           storedConfig.Type,
			Enabled:        storedConfig.Enabled,
			APIKey:         storedConfig.APIKey,
			BaseURL:        storedConfig.BaseURL,
			Timeout:        storedConfig.Timeout,
			MaxRetries:     storedConfig.MaxRetries,
			HealthCheckURL: storedConfig.HealthCheckURL,
			Weight:         storedConfig.Weight,
			Tags:           make([]string, len(storedConfig.Tags)),
			Capabilities:   make(map[string]string),
			CustomSettings: make(map[string]any),
		}
		copy(configCopy.Tags, storedConfig.Tags)
		for k, v := range storedConfig.Capabilities {
			configCopy.Capabilities[k] = v
		}
		for k, v := range storedConfig.CustomSettings {
			configCopy.CustomSettings[k] = v
		}
		if len(storedConfig.Models) > 0 {
			configCopy.Models = make([]ModelConfig, len(storedConfig.Models))
			for i, m := range storedConfig.Models {
				configCopy.Models[i] = ModelConfig{
					ID:           m.ID,
					Name:         m.Name,
					Enabled:      m.Enabled,
					Weight:       m.Weight,
					Capabilities: make([]string, len(m.Capabilities)),
					CustomParams: make(map[string]any),
				}
				copy(configCopy.Models[i].Capabilities, m.Capabilities)
				for k, v := range m.CustomParams {
					configCopy.Models[i].CustomParams[k] = v
				}
			}
		}
		return configCopy, nil
	}

	// Return default config if no stored configuration exists
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
	return a.provider.Complete(ctx, req)
}

func (a *providerAdapter) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return a.provider.CompleteStream(ctx, req)
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
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			RecoveryTimeout:  60 * time.Second,
			SuccessThreshold: 2,
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

	// Override with application config if provided
	if appConfig != nil {
		if appConfig.LLM.DefaultTimeout > 0 {
			cfg.DefaultTimeout = appConfig.LLM.DefaultTimeout
		}

		if appConfig.LLM.MaxRetries > 0 {
			cfg.MaxRetries = appConfig.LLM.MaxRetries
		}
	}

	// Load provider configurations from environment variables
	// Providers are only enabled if their API key is configured

	deepseekKey := os.Getenv("DEEPSEEK_API_KEY")
	cfg.Providers["deepseek"] = &ProviderConfig{
		Name:    "deepseek",
		Type:    "deepseek",
		Enabled: deepseekKey != "",
		Models: []ModelConfig{{
			ID:      getEnvOrDefault("DEEPSEEK_MODEL", "deepseek-coder"),
			Name:    "DeepSeek Coder",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  deepseekKey,
		BaseURL: os.Getenv("DEEPSEEK_BASE_URL"),
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	cfg.Providers["claude"] = &ProviderConfig{
		Name:    "claude",
		Type:    "claude",
		Enabled: claudeKey != "",
		Models: []ModelConfig{{
			ID:      getEnvOrDefault("CLAUDE_MODEL", "claude-3-sonnet-20240229"),
			Name:    "Claude 3 Sonnet",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  claudeKey,
		BaseURL: os.Getenv("ANTHROPIC_BASE_URL"),
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	geminiKey := os.Getenv("GEMINI_API_KEY")
	cfg.Providers["gemini"] = &ProviderConfig{
		Name:    "gemini",
		Type:    "gemini",
		Enabled: geminiKey != "",
		Models: []ModelConfig{{
			ID:      getEnvOrDefault("GEMINI_MODEL", "gemini-pro"),
			Name:    "Gemini Pro",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  geminiKey,
		BaseURL: os.Getenv("GEMINI_BASE_URL"),
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	qwenKey := os.Getenv("QWEN_API_KEY")
	cfg.Providers["qwen"] = &ProviderConfig{
		Name:    "qwen",
		Type:    "qwen",
		Enabled: qwenKey != "",
		Models: []ModelConfig{{
			ID:      getEnvOrDefault("QWEN_MODEL", "qwen-turbo"),
			Name:    "Qwen Turbo",
			Enabled: true,
			Weight:  1.0,
		}},
		APIKey:  qwenKey,
		BaseURL: os.Getenv("QWEN_BASE_URL"),
		Timeout: cfg.DefaultTimeout,
		Weight:  1.0,
	}

	openrouterKey := os.Getenv("OPENROUTER_API_KEY")
	cfg.Providers["openrouter"] = &ProviderConfig{
		Name:    "openrouter",
		Type:    "openrouter",
		Enabled: openrouterKey != "",
		Models: []ModelConfig{{
			ID:      getEnvOrDefault("OPENROUTER_MODEL", "x-ai/grok-4"),
			Name:    "Grok-4 via OpenRouter",
			Enabled: true,
			Weight:  1.3,
		}},
		APIKey:  openrouterKey,
		Timeout: cfg.DefaultTimeout,
		Weight:  1.3,
	}

	return cfg
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// VerifyProvider tests a provider with an actual API call and returns the verification result
// This is a critical function that ensures providers actually work before being used in the ensemble
func (r *ProviderRegistry) VerifyProvider(ctx context.Context, providerName string) *ProviderVerificationResult {
	start := time.Now()
	result := &ProviderVerificationResult{
		Provider: providerName,
		Status:   ProviderStatusUnknown,
		Verified: false,
		TestedAt: time.Now(),
	}

	// Get the provider
	r.mu.RLock()
	provider, exists := r.providers[providerName]
	r.mu.RUnlock()

	if !exists {
		result.Status = ProviderStatusUnhealthy
		result.Error = "provider not registered"
		return result
	}

	// Create a simple test request
	testReq := &models.LLMRequest{
		ID:        fmt.Sprintf("verify_%s_%d", providerName, time.Now().UnixNano()),
		SessionID: "verification",
		Prompt:    "Say OK",
		Messages: []models.Message{
			{Role: "user", Content: "Say OK"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   5,
			Temperature: 0.1,
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	// Create context with timeout for verification
	verifyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Make the actual API call
	resp, err := provider.Complete(verifyCtx, testReq)
	result.ResponseTime = time.Since(start)

	if err != nil {
		errStr := err.Error()

		// Categorize the error
		switch {
		case containsAny(errStr, "429", "quota", "rate", "RESOURCE_EXHAUSTED"):
			result.Status = ProviderStatusRateLimited
			result.Error = "rate limited or quota exceeded"
		case containsAny(errStr, "401", "403", "unauthorized", "invalid", "authentication", "API_KEY"):
			result.Status = ProviderStatusAuthFailed
			result.Error = "authentication failed or invalid API key"
		default:
			result.Status = ProviderStatusUnhealthy
			result.Error = errStr
		}
		return result
	}

	// Check for valid response
	if resp != nil && resp.Content != "" {
		result.Status = ProviderStatusHealthy
		result.Verified = true
	} else {
		result.Status = ProviderStatusUnhealthy
		result.Error = "empty response from provider"
	}

	// Store the result
	r.mu.Lock()
	r.providerHealth[providerName] = result
	r.mu.Unlock()

	return result
}

// VerifyAllProviders verifies all registered providers and returns their status
func (r *ProviderRegistry) VerifyAllProviders(ctx context.Context) map[string]*ProviderVerificationResult {
	results := make(map[string]*ProviderVerificationResult)

	r.mu.RLock()
	providerNames := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providerNames = append(providerNames, name)
	}
	r.mu.RUnlock()

	// Verify providers concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan *ProviderVerificationResult, len(providerNames))

	for _, name := range providerNames {
		wg.Add(1)
		go func(providerName string) {
			defer wg.Done()
			result := r.VerifyProvider(ctx, providerName)
			resultsChan <- result
		}(name)
	}

	// Wait for all verifications to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		results[result.Provider] = result
	}

	return results
}

// GetProviderHealth returns the last verification result for a provider
func (r *ProviderRegistry) GetProviderHealth(providerName string) *ProviderVerificationResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providerHealth[providerName]
}

// GetAllProviderHealth returns all provider health verification results
func (r *ProviderRegistry) GetAllProviderHealth() map[string]*ProviderVerificationResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]*ProviderVerificationResult, len(r.providerHealth))
	for k, v := range r.providerHealth {
		results[k] = v
	}
	return results
}

// IsProviderHealthy returns true if the provider has been verified as healthy
func (r *ProviderRegistry) IsProviderHealthy(providerName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	health, exists := r.providerHealth[providerName]
	if !exists {
		return false // Not verified yet, assume unhealthy
	}
	return health.Status == ProviderStatusHealthy && health.Verified
}

// GetHealthyProviders returns a list of providers that have been verified as healthy
func (r *ProviderRegistry) GetHealthyProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthy := make([]string, 0)
	for name, health := range r.providerHealth {
		if health.Status == ProviderStatusHealthy && health.Verified {
			healthy = append(healthy, name)
		}
	}
	return healthy
}

// containsAny checks if the string contains any of the substrings (case-insensitive)
func containsAny(s string, substrs ...string) bool {
	sLower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(sLower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// RegisterProviderFromConfig creates and registers a provider from configuration
func (r *ProviderRegistry) RegisterProviderFromConfig(cfg ProviderConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	// Create provider based on type
	var provider llm.LLMProvider
	model := getFirstModel(cfg.Models)
	baseURL := cfg.BaseURL

	switch cfg.Type {
	case "claude":
		provider = claude.NewClaudeProvider(cfg.APIKey, baseURL, model)
	case "deepseek":
		provider = deepseek.NewDeepSeekProvider(cfg.APIKey, baseURL, model)
	case "gemini":
		provider = gemini.NewGeminiProvider(cfg.APIKey, baseURL, model)
	case "qwen":
		provider = qwen.NewQwenProvider(cfg.APIKey, baseURL, model)
	case "openrouter":
		provider = openrouter.NewSimpleOpenRouterProviderWithBaseURL(cfg.APIKey, baseURL)
	default:
		return fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}

	// Store the config
	r.mu.Lock()
	r.config.Providers[cfg.Name] = &cfg
	r.mu.Unlock()

	// Register the provider
	return r.RegisterProvider(cfg.Name, provider)
}

// UpdateProvider updates a provider's configuration
func (r *ProviderRegistry) UpdateProvider(name string, cfg ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	// Update stored config
	if existingConfig, exists := r.config.Providers[name]; exists {
		if cfg.APIKey != "" {
			existingConfig.APIKey = cfg.APIKey
		}
		if cfg.BaseURL != "" {
			existingConfig.BaseURL = cfg.BaseURL
		}
		if cfg.Weight != 0 {
			existingConfig.Weight = cfg.Weight
		}
		if len(cfg.Models) > 0 {
			existingConfig.Models = cfg.Models
		}
		existingConfig.Enabled = cfg.Enabled
	}

	return nil
}

// RemoveProvider removes a provider with optional force flag
// If force is false and there are active requests, it will attempt graceful shutdown
// by waiting for requests to drain up to the configured drain timeout
func (r *ProviderRegistry) RemoveProvider(name string, force bool) error {
	r.mu.Lock()

	if _, exists := r.providers[name]; !exists {
		r.mu.Unlock()
		return fmt.Errorf("provider %s not found", name)
	}

	// Check for active requests
	counter, hasCounter := r.activeRequests[name]
	if !force && hasCounter && counter != nil {
		activeCount := atomic.LoadInt64(counter)
		if activeCount > 0 {
			// Release lock and attempt graceful drain
			r.mu.Unlock()
			if err := r.drainProviderRequests(name); err != nil {
				return fmt.Errorf("provider %s has active requests and drain failed: %w", name, err)
			}
			// Re-acquire lock after draining
			r.mu.Lock()
		}
	}

	// Remove provider and associated data
	delete(r.providers, name)
	delete(r.config.Providers, name)
	delete(r.providerConfigs, name)
	delete(r.circuitBreakers, name)
	delete(r.activeRequests, name)

	r.ensemble.RemoveProvider(name)
	r.requestService.RemoveProvider(name)

	r.mu.Unlock()
	return nil
}

// drainProviderRequests waits for active requests to complete up to the drain timeout
func (r *ProviderRegistry) drainProviderRequests(name string) error {
	r.mu.RLock()
	counter, exists := r.activeRequests[name]
	drainTimeout := r.drainTimeout
	r.mu.RUnlock()

	if !exists || counter == nil {
		return nil
	}

	deadline := time.Now().Add(drainTimeout)
	pollInterval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		activeCount := atomic.LoadInt64(counter)
		if activeCount <= 0 {
			return nil
		}
		time.Sleep(pollInterval)
	}

	// Check one final time
	finalCount := atomic.LoadInt64(counter)
	if finalCount > 0 {
		return fmt.Errorf("timeout waiting for %d active requests to complete", finalCount)
	}

	return nil
}

// IncrementActiveRequests increments the active request counter for a provider
// Returns false if the provider doesn't exist
func (r *ProviderRegistry) IncrementActiveRequests(name string) bool {
	r.mu.RLock()
	counter, exists := r.activeRequests[name]
	r.mu.RUnlock()

	if !exists || counter == nil {
		return false
	}

	atomic.AddInt64(counter, 1)
	return true
}

// DecrementActiveRequests decrements the active request counter for a provider
// Returns false if the provider doesn't exist
func (r *ProviderRegistry) DecrementActiveRequests(name string) bool {
	r.mu.RLock()
	counter, exists := r.activeRequests[name]
	r.mu.RUnlock()

	if !exists || counter == nil {
		return false
	}

	atomic.AddInt64(counter, -1)
	return true
}

// GetActiveRequestCount returns the number of active requests for a provider
// Returns -1 if the provider doesn't exist
func (r *ProviderRegistry) GetActiveRequestCount(name string) int64 {
	r.mu.RLock()
	counter, exists := r.activeRequests[name]
	r.mu.RUnlock()

	if !exists || counter == nil {
		return -1
	}

	return atomic.LoadInt64(counter)
}

// SetDrainTimeout sets the timeout for graceful shutdown request draining
func (r *ProviderRegistry) SetDrainTimeout(timeout time.Duration) {
	r.mu.Lock()
	r.drainTimeout = timeout
	r.mu.Unlock()
}

// getFirstModel returns the first model ID from a list of models
func getFirstModel(models []ModelConfig) string {
	if len(models) > 0 {
		return models[0].ID
	}
	return ""
}
