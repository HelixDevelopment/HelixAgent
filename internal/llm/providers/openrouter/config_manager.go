package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// OpenRouterConfigManager handles multi-tenancy and advanced routing for OpenRouter
type OpenRouterConfigManager struct {
	apiKey    string
	baseURL   string
	providers map[string]*OpenRouterProvider
	clients   map[string]*http.Client
	mu        sync.RWMutex
	logger    func(string, ...interface{})
	stats     *OpenRouterStats
	config    *OpenRouterManagerConfig
}

// OpenRouterManagerConfig represents configuration for the manager
type OpenRouterManagerConfig struct {
	DefaultAPIKey    string                  `json:"default_api_key"`
	BaseURL          string                  `json:"base_url"`
	Timeout          time.Duration           `json:"timeout"`
	MaxRetries       int                     `json:"max_retries"`
	DefaultStrategy  string                  `json:"default_strategy"`
	FallbackStrategy string                  `json:"fallback_strategy"`
	ModelPreferences map[string][]string     `json:"model_preferences"`
	PerformanceCache *PerformanceCacheConfig `json:"performance_cache"`
	RateLimitConfig  *RateLimitConfig        `json:"rate_limit_config"`
}

// PerformanceCacheConfig for model performance tracking
type PerformanceCacheConfig struct {
	Enabled         bool          `json:"enabled"`
	TTL             time.Duration `json:"ttl"`
	Metrics         []string      `json:"metrics"`
	UpdateThreshold int           `json:"update_threshold"`
}

// RateLimitConfig for provider-specific rate limiting
type RateLimitConfig struct {
	Enabled           bool `json:"enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	RequestsPerHour   int  `json:"requests_per_hour"`
	TokensPerMinute   int  `json:"tokens_per_minute"`
}

// OpenRouterStats tracks usage and performance statistics
type OpenRouterStats struct {
	TotalRequests      int64            `json:"total_requests"`
	SuccessfulRequests int64            `json:"successful_requests"`
	FailedRequests     int64            `json:"failed_requests"`
	AvgResponseTime    time.Duration    `json:"avg_response_time"`
	ModelUsage         map[string]int64 `json:"model_usage"`
	ProviderUsage      map[string]int64 `json:"provider_usage"`
	ErrorCounts        map[string]int64 `json:"error_counts"`
	LastReset          time.Time        `json:"last_reset"`
}

// OpenRouterProvider represents a provider instance with specific configuration
type OpenRouterProvider struct {
	APIKey        string                   `json:"api_key"`
	BaseURL       string                   `json:"base_url"`
	Models        []OpenRouterModel        `json:"models"`
	DefaultModel  string                   `json:"default_model"`
	RouteStrategy string                   `json:"route_strategy"`
	RateLimits    *ModelRateLimits         `json:"rate_limits"`
	HTTPHeaders   map[string]string        `json:"http_headers"`
	Client        *http.Client             `json:"-"`
	Stats         *OpenRouterProviderStats `json:"-"`
}

// OpenRouterProviderStats tracks per-provider statistics
type OpenRouterProviderStats struct {
	Requests        int64         `json:"requests"`
	Successes       int64         `json:"successes"`
	Errors          int64         `json:"errors"`
	AvgLatency      time.Duration `json:"avg_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
}

// RoutingStrategies provides different routing algorithms
type RoutingStrategies struct {
	basic          BasicStrategy
	costOptimized  CostOptimizedStrategy
	performanceOpt PerformanceOptimizedStrategy
	multiModel     MultiModelStrategy
	roundRobin     RoundRobinStrategy
	weighted       WeightedStrategy
}

// NewOpenRouterConfigManager creates a new configuration manager
func NewOpenRouterConfigManager(config *OpenRouterManagerConfig, logger func(string, ...interface{})) *OpenRouterConfigManager {
	if config == nil {
		config = &OpenRouterManagerConfig{
			BaseURL:          "https://openrouter.ai/api/v1",
			Timeout:          30 * time.Second,
			MaxRetries:       3,
			DefaultStrategy:  "cost_optimized",
			FallbackStrategy: "basic",
			ModelPreferences: make(map[string][]string),
			PerformanceCache: &PerformanceCacheConfig{
				Enabled: true,
				TTL:     5 * time.Minute,
				Metrics: []string{"response_time", "success_rate", "cost_per_token"},
			},
			RateLimitConfig: &RateLimitConfig{
				Enabled: true,
			},
		}
	}

	manager := &OpenRouterConfigManager{
		apiKey:    config.DefaultAPIKey,
		baseURL:   config.BaseURL,
		providers: make(map[string]*OpenRouterProvider),
		clients:   make(map[string]*http.Client),
		logger:    logger,
		stats: &OpenRouterStats{
			ModelUsage:    make(map[string]int64),
			ProviderUsage: make(map[string]int64),
			ErrorCounts:   make(map[string]int64),
			LastReset:     time.Now(),
		},
		config: config,
	}

	// Initialize strategies
	manager.initializeStrategies()

	return manager
}

// InitializeStrategies sets up routing strategies
func (m *OpenRouterConfigManager) initializeStrategies() {
	m.strategies = RoutingStrategies{
		basic:          NewBasicStrategy(),
		costOptimized:  NewCostOptimizedStrategy(m.config.PerformanceCache),
		performanceOpt: NewPerformanceOptimizedStrategy(m.config.PerformanceCache),
		multiModel:     NewMultiModelStrategy(),
		roundRobin:     NewRoundRobinStrategy(),
		weighted:       NewWeightedStrategy(),
	}
}

// AddProvider adds a new OpenRouter provider configuration
func (m *OpenRouterConfigManager) AddProvider(config *OpenRouterProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.APIKey == "" {
		return fmt.Errorf("API key is required for OpenRouter provider")
	}

	providerID := generateProviderID(config)
	config.Client = &http.Client{
		Timeout: m.config.Timeout,
	}

	m.providers[providerID] = providerID
	m.clients[providerID] = config.Client
	m.logger("Added OpenRouter provider: %s", providerID)

	return nil
}

// RemoveProvider removes a provider configuration
func (m *OpenRouterConfigManager) RemoveProvider(providerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[providerID]; !exists {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	delete(m.providers, providerID)
	delete(m.clients, providerID)
	m.logger("Removed OpenRouter provider: %s", providerID)

	return nil
}

// SelectBestProvider selects the best provider for a request
func (m *OpenRouterConfigManager) SelectBestProvider(ctx context.Context, req *models.LLMRequest) (*OpenRouterProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Get strategy
	strategy := m.getStrategy(req.EnsembleConfig.Strategy)

	// Prepare available providers
	availableProviders := make([]*OpenRouterProvider, 0, len(m.providers))
	for _, provider := range m.providers {
		if m.isProviderSuitable(provider, req) {
			availableProviders = append(availableProviders, provider)
		}
	}

	// Select provider using strategy
	selectedProvider, err := strategy.SelectProvider(ctx, req, availableProviders)
	if err != nil {
		m.logger("Strategy selection failed: %v", err)
		// Fallback to basic strategy
		return m.strategies.basic.SelectProvider(ctx, req, availableProviders)
	}

	return selectedProvider, nil
}

// GetProviderModels returns available models from all providers
func (m *OpenRouterConfigManager) GetProviderModels(ctx context.Context) ([]OpenRouterModel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allModels []OpenRouterModel
	seenModels := make(map[string]bool)

	for _, provider := range m.providers {
		models, err := m.fetchProviderModels(ctx, provider)
		if err != nil {
			m.logger("Failed to fetch models from provider: %v", err)
			continue
		}

		for _, model := range models {
			if !seenModels[model.ID] {
				allModels = append(allModels, model)
				seenModels[model.ID] = true
			}
		}
	}

	return allModels, nil
}

// UpdateProviderConfig updates provider configuration
func (m *OpenRouterConfigManager) UpdateProviderConfig(providerID string, config *OpenRouterProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[providerID]; !exists {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	// Update configuration while preserving stats
	oldStats := m.providers[providerID].Stats
	config.Stats = oldStats
	config.Client = &http.Client{
		Timeout: m.config.Timeout,
	}

	m.providers[providerID] = config
	m.clients[providerID] = config.Client
	m.logger("Updated OpenRouter provider configuration: %s", providerID)

	return nil
}

// GetStats returns aggregated statistics
func (m *OpenRouterConfigManager) GetStats() *OpenRouterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// GetProviderStats returns per-provider statistics
func (m *OpenRouterConfigManager) GetProviderStats() map[string]*OpenRouterProviderStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]*OpenRouterProviderStats)
	for id, provider := range m.providers {
		stats[id] = provider.Stats
	}
	return stats
}

// HealthCheck performs health checks on all providers
func (m *OpenRouterConfigManager) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var healthyProviders int
	for providerID, provider := range m.providers {
		err := m.checkProviderHealth(ctx, providerID, provider)
		if err != nil {
			m.logger("Provider health check failed: %s - %v", providerID, err)
		} else {
			healthyProviders++
		}
	}

	if healthyProviders == 0 {
		return fmt.Errorf("no healthy providers")
	}

	if healthyProviders < len(m.providers) {
		m.logger("Warning: %d/%d providers healthy", healthyProviders, len(m.providers))
	}

	return nil
}

// Helper methods

func (m *OpenRouterConfigManager) getStrategy(strategyName string) RoutingStrategyInterface {
	switch strategyName {
	case "basic":
		return m.strategies.basic
	case "cost_optimized":
		return m.strategies.costOptimized
	case "performance_optimized":
		return m.strategies.performanceOpt
	case "multi_model":
		return m.strategies.multiModel
	case "round_robin":
		return m.strategies.roundRobin
	case "weighted":
		return m.strategies.weighted
	default:
		return m.strategies.basic // fallback
	}
}

func (m *OpenRouterConfigManager) isProviderSuitable(provider *OpenRouterProvider, req *models.LLMRequest) bool {
	// Check streaming requirement
	if req.Stream {
		for _, model := range provider.Models {
			for _, cap := range model.Capabilities {
				if cap == "streaming" {
					return true
				}
			}
		}
	}

	// Check model capabilities
	if len(req.Tools) > 0 {
		for _, model := range provider.Models {
			for _, cap := range model.Capabilities {
				if cap == "function_calling" {
					return true
				}
			}
		}
	}

	return true // Basic suitability check
}

func (m *OpenRouterConfigManager) fetchProviderModels(ctx context.Context, provider *OpenRouterProvider) ([]OpenRouterModel, error) {
	// In a real implementation, this would fetch from the provider
	// For now, return configured models
	return provider.Models, nil
}

func (m *OpenRouterConfigManager) checkProviderHealth(ctx context.Context, providerID string, provider *OpenRouterProvider) error {
	// Simple health check - in real implementation, this would ping the provider
	req, err := http.NewRequestWithContext(ctx, "GET", provider.BaseURL+"/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	req.Header.Set("HTTP-Referer", "superagent")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
}

func generateProviderID(provider *OpenRouterProvider) string {
	// Generate a unique ID based on API key (first 8 chars)
	if len(provider.APIKey) >= 8 {
		return provider.APIKey[:8]
	}
	return fmt.Sprintf("provider_%d", time.Now().Unix())
}

// UpdateRequestStats updates statistics after a request
func (m *OpenRouterConfigManager) UpdateRequestStats(providerID string, success bool, responseTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.TotalRequests++
	if success {
		m.stats.SuccessfulRequests++
	} else {
		m.stats.FailedRequests++
	}

	// Update provider stats
	if provider := m.providers[providerID]; provider != nil {
		if provider.Stats == nil {
			provider.Stats = &OpenRouterProviderStats{}
		}

		provider.Stats.Requests++
		if success {
			provider.Stats.Successes++
		} else {
			provider.Stats.Errors++
		}

		provider.Stats.LastRequestTime = time.Now()
		provider.Stats.AvgLatency = time.Duration(
			(int64(provider.Stats.AvgLatency)*int64(provider.Stats.Requests-1) + int64(responseTime)) /
				int64(provider.Stats.Requests),
		)
	}
}

// RoutingStrategy interface
type RoutingStrategyInterface interface {
	Name() string
	SelectProvider(ctx context.Context, req *models.LLMRequest, providers []*OpenRouterProvider) (*OpenRouterProvider, error)
}

// BasicStrategy implements simple provider selection
type BasicStrategy struct{}

func NewBasicStrategy() BasicStrategy {
	return BasicStrategy{}
}

func (s BasicStrategy) Name() string {
	return "basic"
}

func (s BasicStrategy) SelectProvider(ctx context.Context, req *models.LLMRequest, providers []*OpenRouterProvider) (*OpenRouterProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}
	return providers[0], nil
}

// CostOptimizedStrategy implements cost-based selection
type CostOptimizedStrategy struct {
	cache *PerformanceCacheConfig
}

func NewCostOptimizedStrategy(cache *PerformanceCacheConfig) CostOptimizedStrategy {
	return CostOptimizedStrategy{cache: cache}
}

func (s CostOptimizedStrategy) Name() string {
	return "cost_optimized"
}

func (s CostOptimizedStrategy) SelectProvider(ctx context.Context, req *models.LLMRequest, providers []*OpenRouterProvider) (*OpenRouterProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	var bestProvider *OpenRouterProvider
	var bestScore float64 = -1

	for _, provider := range providers {
		score := s.calculateCostScore(provider, req)
		if score > bestScore {
			bestScore = score
			bestProvider = provider
		}
	}

	return bestProvider, nil
}

func (s CostOptimizedStrategy) calculateCostScore(provider *OpenRouterProvider, req *models.LLMRequest) float64 {
	// Simple cost calculation based on model pricing
	for _, model := range provider.Models {
		if model.Pricing != nil && model.Pricing.UnitPrice > 0 {
			// Lower price = higher score
			return 1.0 / model.Pricing.UnitPrice
		}
	}
	return 0.0 // No pricing available
}

// PerformanceOptimizedStrategy implements performance-based selection
type PerformanceOptimizedStrategy struct {
	cache *PerformanceCacheConfig
}

func NewPerformanceOptimizedStrategy(cache *PerformanceCacheConfig) PerformanceOptimizedStrategy {
	return PerformanceOptimizedStrategy{cache: cache}
}

func (s PerformanceOptimizedStrategy) Name() string {
	return "performance_optimized"
}

func (s PerformanceOptimizedStrategy) SelectProvider(ctx context.Context, req *models.LLMRequest, providers []*OpenRouterProvider) (*OpenRouterProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	var bestProvider *OpenRouterProvider
	var bestScore float64 = -1

	for _, provider := range providers {
		if provider.Stats != nil && provider.Stats.Requests > 0 {
			// Calculate score based on success rate and latency
			successRate := float64(provider.Stats.Successes) / float64(provider.Stats.Requests)
			latencyScore := 1.0 / float64(provider.Stats.AvgLatency.Milliseconds())
			score := (successRate * 0.7) + (latencyScore * 0.3) // Weight success rate higher

			if score > bestScore {
				bestScore = score
				bestProvider = provider
			}
		}
	}

	return bestProvider, nil
}

// MultiModelStrategy implements intelligent model routing
type MultiModelStrategy struct{}

func NewMultiModelStrategy() MultiModelStrategy {
	return MultiModelStrategy{}
}

func (s MultiModelStrategy) Name() string {
	return "multi_model"
}

func (s MultiModelStrategy) SelectProvider(ctx context.Context, req *models.LLMRequest, providers []*OpenRouterProvider) (*OpenRouterProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Analyze request and select appropriate provider
	providerType := s.analyzeRequestType(req)

	for _, provider := range providers {
		if s.isProviderSuitableForType(provider, providerType) {
			return provider
		}
	}

	return providers[0], nil // fallback
}

func (s MultiModelStrategy) analyzeRequestType(req *models.LLMRequest) string {
	prompt := strings.ToLower(req.Prompt)

	// Simple keyword-based analysis
	if strings.Contains(prompt, "code") || strings.Contains(prompt, "programming") {
		return "coding"
	}
	if strings.Contains(prompt, "image") || strings.Contains(prompt, "vision") {
		return "vision"
	}
	if strings.Contains(prompt, "analyze") || strings.Contains(prompt, "research") {
		return "reasoning"
	}

	return "general"
}

func (s MultiModelStrategy) isProviderSuitableForType(provider *OpenRouterProvider, requestType string) bool {
	for _, model := range provider.Models {
		for _, cap := range model.Capabilities {
			switch requestType {
			case "coding":
				if cap == "code_generation" || cap == "function_calling" {
					return true
				}
			case "vision":
				if cap == "vision" {
					return true
				}
			case "reasoning":
				if strings.Contains(strings.ToLower(model.Name), "gpt") ||
					strings.Contains(strings.ToLower(model.Name), "claude") ||
					strings.Contains(strings.ToLower(model.Name), "gemini") {
					return true
				}
			default:
				return true // general capability
			}
		}
	}
	return false
}

// RoundRobinStrategy implements round-robin load balancing
type RoundRobinStrategy struct {
	counter int
	mu      sync.Mutex
}

func NewRoundRobinStrategy() RoundRobinStrategy {
	return RoundRobinStrategy{counter: 0}
}

func (s *RoundRobinStrategy) Name() string {
	return "round_robin"
}

func (s *RoundRobinStrategy) SelectProvider(ctx context.Context, req *models.LLMRequest, providers []*OpenRouterProvider) (*OpenRouterProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	selected := providers[s.counter%len(providers)]
	s.counter++

	return selected, nil
}

// WeightedStrategy implements weighted load balancing
type WeightedStrategy struct{}

func NewWeightedStrategy() WeightedStrategy {
	return WeightedStrategy{}
}

func (s WeightedStrategy) Name() string {
	return "weighted"
}

func (s WeightedStrategy) SelectProvider(ctx context.Context, req *models.LLMRequest, providers []*OpenRouterProvider) (*OpenRouterProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Simple weighted selection based on historical performance
	totalWeight := 0.0
	weights := make([]float64, len(providers))

	for i, provider := range providers {
		if provider.Stats != nil && provider.Stats.Requests > 0 {
			successRate := float64(provider.Stats.Successes) / float64(provider.Stats.Requests)
			weights[i] = successRate
			totalWeight += successRate
		} else {
			weights[i] = 1.0 // default weight
			totalWeight += 1.0
		}
	}

	// Select provider probabilistically based on weights
	random := float64(1) / float64(len(providers))
	if totalWeight > 0 {
		random = float64(1) / totalWeight
	}

	for i, weight := range weights {
		if random < weight/totalWeight {
			return providers[i]
		}
		random -= weight
	}

	return providers[0], nil // fallback
}
