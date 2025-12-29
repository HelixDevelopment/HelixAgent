package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/database"
)

// ModelMetadataServiceInterface defines the interface for model metadata service
type ModelMetadataServiceInterface interface {
	GetModel(ctx context.Context, modelID string) (*database.ModelMetadata, error)
	ListModels(ctx context.Context, providerID string, modelType string, page int, limit int) ([]*database.ModelMetadata, int, error)
	SearchModels(ctx context.Context, query string, page int, limit int) ([]*database.ModelMetadata, int, error)
	RefreshModels(ctx context.Context) error
	RefreshProviderModels(ctx context.Context, providerID string) error
	GetRefreshHistory(ctx context.Context, limit int) ([]*database.ModelsRefreshHistory, error)
	GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error)
	CompareModels(ctx context.Context, modelIDs []string) ([]*database.ModelMetadata, error)
	GetModelsByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error)
}

// ProviderRegistryInterface defines the interface for provider registry
type ProviderRegistryInterface interface {
	ListProviders() []string
	GetProviderConfig(name string) (*ProviderConfig, error)
	ConfigureProvider(name string, config *ProviderConfig) error
}

// ProviderMetadataService integrates Models.dev data with provider registry
type ProviderMetadataService struct {
	modelMetadataService ModelMetadataServiceInterface
	providerRegistry     ProviderRegistryInterface
	log                  *logrus.Logger
	mu                   sync.RWMutex
	providerToModels     map[string][]*database.ModelMetadata
}

// NewProviderMetadataService creates a new provider metadata service
func NewProviderMetadataService(
	modelMetadataService ModelMetadataServiceInterface,
	providerRegistry ProviderRegistryInterface,
	log *logrus.Logger,
) *ProviderMetadataService {
	return &ProviderMetadataService{
		modelMetadataService: modelMetadataService,
		providerRegistry:     providerRegistry,
		log:                  log,
		providerToModels:     make(map[string][]*database.ModelMetadata),
	}
}

// LoadProviderMetadata loads Models.dev metadata for all providers
func (s *ProviderMetadataService) LoadProviderMetadata(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing data
	s.providerToModels = make(map[string][]*database.ModelMetadata)

	// Get all providers from registry
	providers := s.providerRegistry.ListProviders()

	for _, providerName := range providers {
		s.log.WithField("provider", providerName).Info("Loading metadata for provider")

		// Get models for this provider
		models, err := s.modelMetadataService.GetProviderModels(ctx, providerName)
		if err != nil {
			s.log.WithError(err).WithField("provider", providerName).Warn("Failed to load provider models")
			continue
		}

		s.providerToModels[providerName] = models
		s.log.WithFields(logrus.Fields{
			"provider":    providerName,
			"model_count": len(models),
		}).Info("Loaded models for provider")
	}

	s.log.WithField("total_providers", len(s.providerToModels)).Info("Provider metadata loaded")
	return nil
}

// UpdateProviderConfigs updates provider configurations with Models.dev metadata
func (s *ProviderMetadataService) UpdateProviderConfigs(ctx context.Context) (map[string]*ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	updatedConfigs := make(map[string]*ProviderConfig)

	for providerName, models := range s.providerToModels {
		config, err := s.providerRegistry.GetProviderConfig(providerName)
		if err != nil {
			s.log.WithError(err).WithField("provider", providerName).Warn("Failed to get provider config")
			continue
		}

		// Update model configurations with Models.dev metadata
		updatedConfig := s.enhanceProviderConfig(config, models)
		updatedConfigs[providerName] = updatedConfig

		// Update the provider in the registry
		if err := s.providerRegistry.ConfigureProvider(providerName, updatedConfig); err != nil {
			s.log.WithError(err).WithField("provider", providerName).Warn("Failed to update provider config")
		}
	}

	return updatedConfigs, nil
}

// enhanceProviderConfig enhances provider configuration with Models.dev metadata
func (s *ProviderMetadataService) enhanceProviderConfig(
	config *ProviderConfig,
	models []*database.ModelMetadata,
) *ProviderConfig {
	enhancedConfig := &ProviderConfig{
		Name:           config.Name,
		Type:           config.Type,
		Enabled:        config.Enabled,
		APIKey:         config.APIKey,
		BaseURL:        config.BaseURL,
		Timeout:        config.Timeout,
		MaxRetries:     config.MaxRetries,
		HealthCheckURL: config.HealthCheckURL,
		Weight:         config.Weight,
		Tags:           config.Tags,
		Capabilities:   config.Capabilities,
		CustomSettings: config.CustomSettings,
		Models:         make([]ModelConfig, 0),
	}

	// Convert Models.dev models to provider model configurations
	for _, model := range models {
		modelConfig := ModelConfig{
			ID:           model.ModelID,
			Name:         model.ModelName,
			Enabled:      true,
			Weight:       s.calculateModelWeight(model),
			Capabilities: s.extractCapabilities(model),
			CustomParams: s.createCustomParams(model),
		}

		enhancedConfig.Models = append(enhancedConfig.Models, modelConfig)
	}

	return enhancedConfig
}

// calculateModelWeight calculates weight based on model metadata
func (s *ProviderMetadataService) calculateModelWeight(model *database.ModelMetadata) float64 {
	weight := 1.0

	// Adjust weight based on benchmark scores
	if model.BenchmarkScore != nil {
		// Normalize benchmark score (assuming 0-100 scale)
		benchmarkWeight := *model.BenchmarkScore / 100.0
		weight += benchmarkWeight * 0.5 // Add up to 50% based on benchmarks
	}

	// Adjust weight based on popularity
	if model.PopularityScore != nil {
		// Assuming popularity score is 0-100
		popularityWeight := float64(*model.PopularityScore) / 100.0
		weight += popularityWeight * 0.3 // Add up to 30% based on popularity
	}

	// Adjust weight based on reliability
	if model.ReliabilityScore != nil {
		reliabilityWeight := *model.ReliabilityScore / 1.0 // Assuming 0-1 scale
		weight += reliabilityWeight * 0.2                  // Add up to 20% based on reliability
	}

	// Cap weight at reasonable limits
	if weight > 2.0 {
		weight = 2.0
	}
	if weight < 0.5 {
		weight = 0.5
	}

	return weight
}

// extractCapabilities extracts capabilities from model metadata
func (s *ProviderMetadataService) extractCapabilities(model *database.ModelMetadata) []string {
	capabilities := make([]string, 0)

	// Map boolean fields to capability names
	if model.SupportsVision {
		capabilities = append(capabilities, "vision")
	}
	if model.SupportsFunctionCalling {
		capabilities = append(capabilities, "function_calling")
	}
	if model.SupportsStreaming {
		capabilities = append(capabilities, "streaming")
	}
	if model.SupportsJSONMode {
		capabilities = append(capabilities, "json_mode")
	}
	if model.SupportsImageGeneration {
		capabilities = append(capabilities, "image_generation")
	}
	if model.SupportsAudio {
		capabilities = append(capabilities, "audio")
	}
	if model.SupportsCodeGeneration {
		capabilities = append(capabilities, "code_generation")
	}
	if model.SupportsReasoning {
		capabilities = append(capabilities, "reasoning")
	}

	// Add model type as capability
	if model.ModelType != nil {
		capabilities = append(capabilities, strings.ToLower(*model.ModelType))
	}

	return capabilities
}

// createCustomParams creates custom parameters for model configuration
func (s *ProviderMetadataService) createCustomParams(model *database.ModelMetadata) map[string]any {
	params := make(map[string]any)

	// Add context window if available
	if model.ContextWindow != nil {
		params["max_tokens"] = *model.ContextWindow
	}

	// Add max tokens if available
	if model.MaxTokens != nil {
		params["max_completion_tokens"] = *model.MaxTokens
	}

	// Add pricing information if available
	if model.PricingInput != nil {
		params["pricing_input_per_million"] = *model.PricingInput
	}
	if model.PricingOutput != nil {
		params["pricing_output_per_million"] = *model.PricingOutput
	}
	if model.PricingCurrency != "" {
		params["pricing_currency"] = model.PricingCurrency
	}

	// Add metadata flags
	params["supports_vision"] = model.SupportsVision
	params["supports_function_calling"] = model.SupportsFunctionCalling
	params["supports_streaming"] = model.SupportsStreaming
	params["supports_json_mode"] = model.SupportsJSONMode
	params["supports_image_generation"] = model.SupportsImageGeneration
	params["supports_audio"] = model.SupportsAudio
	params["supports_code_generation"] = model.SupportsCodeGeneration
	params["supports_reasoning"] = model.SupportsReasoning

	// Add benchmark information if available
	if model.BenchmarkScore != nil {
		params["benchmark_score"] = *model.BenchmarkScore
	}

	return params
}

// GetModelsForProvider returns models for a specific provider
func (s *ProviderMetadataService) GetModelsForProvider(ctx context.Context, providerName string) ([]*database.ModelMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models, exists := s.providerToModels[providerName]
	if !exists {
		// Try to load models for this provider
		s.mu.RUnlock()
		models, err := s.modelMetadataService.GetProviderModels(ctx, providerName)
		s.mu.RLock()
		if err != nil {
			return nil, fmt.Errorf("failed to get models for provider %s: %w", providerName, err)
		}
		s.providerToModels[providerName] = models
		return models, nil
	}

	return models, nil
}

// GetRecommendedModel returns the recommended model for a provider based on capabilities
func (s *ProviderMetadataService) GetRecommendedModel(
	ctx context.Context,
	providerName string,
	requiredCapabilities []string,
) (*database.ModelMetadata, error) {
	models, err := s.GetModelsForProvider(ctx, providerName)
	if err != nil {
		return nil, err
	}

	// Filter models by required capabilities
	var candidateModels []*database.ModelMetadata
	for _, model := range models {
		if s.hasRequiredCapabilities(model, requiredCapabilities) {
			candidateModels = append(candidateModels, model)
		}
	}

	if len(candidateModels) == 0 {
		return nil, fmt.Errorf("no models found for provider %s with required capabilities", providerName)
	}

	// Select best model based on scoring
	return s.selectBestModel(candidateModels), nil
}

// hasRequiredCapabilities checks if a model has all required capabilities
func (s *ProviderMetadataService) hasRequiredCapabilities(
	model *database.ModelMetadata,
	requiredCapabilities []string,
) bool {
	for _, required := range requiredCapabilities {
		hasCapability := false

		switch strings.ToLower(required) {
		case "vision":
			hasCapability = model.SupportsVision
		case "function_calling":
			hasCapability = model.SupportsFunctionCalling
		case "streaming":
			hasCapability = model.SupportsStreaming
		case "json_mode":
			hasCapability = model.SupportsJSONMode
		case "image_generation":
			hasCapability = model.SupportsImageGeneration
		case "audio":
			hasCapability = model.SupportsAudio
		case "code_generation":
			hasCapability = model.SupportsCodeGeneration
		case "reasoning":
			hasCapability = model.SupportsReasoning
		default:
			// Check if capability is in tags
			for _, tag := range model.Tags {
				if strings.EqualFold(tag, required) {
					hasCapability = true
					break
				}
			}
		}

		if !hasCapability {
			return false
		}
	}

	return true
}

// selectBestModel selects the best model based on scoring
func (s *ProviderMetadataService) selectBestModel(models []*database.ModelMetadata) *database.ModelMetadata {
	if len(models) == 0 {
		return nil
	}

	var bestModel *database.ModelMetadata
	bestScore := -1.0

	for _, model := range models {
		score := s.calculateModelScore(model)
		if score > bestScore {
			bestScore = score
			bestModel = model
		}
	}

	return bestModel
}

// calculateModelScore calculates a score for model selection
func (s *ProviderMetadataService) calculateModelScore(model *database.ModelMetadata) float64 {
	score := 0.0

	// Add benchmark score if available
	if model.BenchmarkScore != nil {
		score += *model.BenchmarkScore
	}

	// Add popularity score if available
	if model.PopularityScore != nil {
		score += float64(*model.PopularityScore) * 0.5
	}

	// Add reliability score if available
	if model.ReliabilityScore != nil {
		score += *model.ReliabilityScore * 100.0
	}

	// Prefer newer models (if version is available)
	if model.Version != nil {
		// Simple scoring for version (higher is better)
		score += 10.0
	}

	// Adjust for context window size
	if model.ContextWindow != nil {
		contextWindowScore := float64(*model.ContextWindow) / 1000.0
		if contextWindowScore > 100.0 {
			contextWindowScore = 100.0
		}
		score += contextWindowScore
	}

	return score
}

// RefreshProviderMetadata refreshes metadata for a specific provider
func (s *ProviderMetadataService) RefreshProviderMetadata(ctx context.Context, providerName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Refresh models from Models.dev
	s.log.WithField("provider", providerName).Info("Refreshing provider metadata")

	// Call ModelMetadataService to refresh provider models
	err := s.modelMetadataService.RefreshProviderModels(ctx, providerName)
	if err != nil {
		return fmt.Errorf("failed to refresh provider %s models: %w", providerName, err)
	}

	// Reload models for this provider
	models, err := s.modelMetadataService.GetProviderModels(ctx, providerName)
	if err != nil {
		return fmt.Errorf("failed to get refreshed models for provider %s: %w", providerName, err)
	}

	// Update cache
	s.providerToModels[providerName] = models

	// Update provider configuration
	config, err := s.providerRegistry.GetProviderConfig(providerName)
	if err == nil {
		updatedConfig := s.enhanceProviderConfig(config, models)
		if err := s.providerRegistry.ConfigureProvider(providerName, updatedConfig); err != nil {
			s.log.WithError(err).WithField("provider", providerName).Warn("Failed to update provider config after refresh")
		}
	}

	s.log.WithFields(logrus.Fields{
		"provider":    providerName,
		"model_count": len(models),
	}).Info("Provider metadata refreshed")

	return nil
}

// StartAutoRefresh starts automatic refresh of provider metadata
func (s *ProviderMetadataService) StartAutoRefresh(refreshInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			ctx := context.Background()
			if err := s.LoadProviderMetadata(ctx); err != nil {
				s.log.WithError(err).Error("Failed to auto-refresh provider metadata")
			} else {
				s.log.Info("Auto-refresh of provider metadata completed")
			}
		}
	}()
}

// StopAutoRefresh stops automatic refresh of provider metadata
func (s *ProviderMetadataService) StopAutoRefresh() {
	// Currently uses ticker which runs until service stops
	// Could be enhanced with context cancellation
}

// GetProviderStats returns statistics for all providers
func (s *ProviderMetadataService) GetProviderStats(ctx context.Context) (map[string]ProviderStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]ProviderStats)

	for providerName, models := range s.providerToModels {
		stat := ProviderStats{
			TotalModels:     len(models),
			EnabledModels:   len(models), // All models from Models.dev are considered enabled
			AverageScore:    0.0,
			Capabilities:    make(map[string]int),
			LastRefreshedAt: time.Now(),
		}

		// Calculate average benchmark score
		totalScore := 0.0
		modelsWithScore := 0

		for _, model := range models {
			// Count capabilities
			for _, capability := range s.extractCapabilities(model) {
				stat.Capabilities[capability]++
			}

			// Calculate average score
			if model.BenchmarkScore != nil {
				totalScore += *model.BenchmarkScore
				modelsWithScore++
			}

			// Find most recent refresh
			if model.LastRefreshedAt.After(stat.LastRefreshedAt) {
				stat.LastRefreshedAt = model.LastRefreshedAt
			}
		}

		if modelsWithScore > 0 {
			stat.AverageScore = totalScore / float64(modelsWithScore)
		}

		stats[providerName] = stat
	}

	return stats, nil
}

// ProviderStats holds statistics for a provider
type ProviderStats struct {
	TotalModels     int            `json:"total_models"`
	EnabledModels   int            `json:"enabled_models"`
	AverageScore    float64        `json:"average_score"`
	Capabilities    map[string]int `json:"capabilities"`
	LastRefreshedAt time.Time      `json:"last_refreshed_at"`
}
