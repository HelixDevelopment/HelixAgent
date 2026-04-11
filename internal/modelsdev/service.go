package modelsdev

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Service provides high-level access to Models.dev data with caching
type Service struct {
	client *Client
	cache  *Cache
	config ServiceConfig
	log    *logrus.Logger

	// Background refresh management
	stopRefresh chan struct{}
	refreshDone chan struct{}
	mu          sync.RWMutex
	isRunning   bool
}

// NewService creates a new ModelsDevService
func NewService(config *ServiceConfig, log *logrus.Logger) *Service {
	if config == nil {
		defaultConfig := DefaultServiceConfig()
		config = &defaultConfig
	}

	if log == nil {
		log = logrus.New()
	}

	clientConfig := &ClientConfig{
		BaseURL:   config.Client.BaseURL,
		APIKey:    config.Client.APIKey,
		Timeout:   config.Client.Timeout,
		UserAgent: config.Client.UserAgent,
	}

	return &Service{
		client:      NewClient(clientConfig),
		cache:       NewCache(&config.Cache),
		config:      *config,
		log:         log,
		stopRefresh: make(chan struct{}),
		refreshDone: make(chan struct{}),
	}
}

// NewServiceWithClient creates a service with a custom client
func NewServiceWithClient(client *Client, cache *Cache, config *ServiceConfig, log *logrus.Logger) *Service {
	if config == nil {
		defaultConfig := DefaultServiceConfig()
		config = &defaultConfig
	}

	if log == nil {
		log = logrus.New()
	}

	if cache == nil {
		cache = NewCache(&config.Cache)
	}

	return &Service{
		client:      client,
		cache:       cache,
		config:      *config,
		log:         log,
		stopRefresh: make(chan struct{}),
		refreshDone: make(chan struct{}),
	}
}

// Start initializes the service and starts background refresh if configured
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return nil
	}
	s.isRunning = true
	s.mu.Unlock()

	// Perform initial refresh if configured
	if s.config.RefreshOnStart {
		s.log.Info("Performing initial Models.dev cache refresh")
		result := s.RefreshCache(ctx)
		if !result.Success {
			s.log.WithField("errors", result.Errors).Warn("Initial cache refresh completed with errors")
		} else {
			s.log.WithFields(logrus.Fields{
				"models_refreshed":    result.ModelsRefreshed,
				"providers_refreshed": result.ProvidersRefreshed,
				"duration":            result.Duration,
			}).Info("Initial cache refresh completed")
		}
	}

	// Start background refresh if configured
	if s.config.AutoRefresh {
		//nolint:gosec // G118: long-lived refresh loop, intentionally decoupled from the caller context
		go s.backgroundRefreshLoop()
	}

	return nil
}

// Stop stops the service and background refresh
func (s *Service) Stop() error {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return nil
	}
	s.isRunning = false
	s.mu.Unlock()

	if s.config.AutoRefresh {
		close(s.stopRefresh)
		<-s.refreshDone
	}

	return s.cache.Close()
}

// GetModel retrieves a model by ID, using cache when available
func (s *Service) GetModel(ctx context.Context, modelID string) (*Model, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID is required")
	}

	// Try cache first
	if model, found := s.cache.GetModel(ctx, modelID); found {
		return model, nil
	}

	// Fetch from API
	s.log.WithField("model_id", modelID).Debug("Cache miss, fetching from API")
	response, err := s.client.GetModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get model from API: %w", err)
	}

	// Convert and cache
	model := s.convertModelInfoToModel(&response.Model)
	s.cache.SetModel(ctx, model)

	return model, nil
}

// GetModelWithCache retrieves a model with explicit cache control
func (s *Service) GetModelWithCache(ctx context.Context, modelID string, useCache bool) (*Model, error) {
	if !useCache {
		return s.fetchModelFromAPI(ctx, modelID)
	}
	return s.GetModel(ctx, modelID)
}

// ListModels lists models with optional filters
func (s *Service) ListModels(ctx context.Context, filters *ModelFilters) ([]*Model, int, error) {
	// Convert filters to ListModelsOptions
	opts := &ListModelsOptions{}
	if filters != nil {
		opts.Provider = filters.Provider
		opts.Page = filters.Page
		opts.Limit = filters.Limit
		if filters.Category != "" {
			opts.ModelType = filters.Category
		}
	}

	response, err := s.client.ListModels(ctx, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list models: %w", err)
	}

	models := make([]*Model, 0, len(response.Models))
	for i := range response.Models {
		model := s.convertModelInfoToModel(&response.Models[i])
		models = append(models, model)
		// Cache each model
		s.cache.SetModel(ctx, model)
	}

	return models, response.Total, nil
}

// GetProvider retrieves a provider by ID, using cache when available
func (s *Service) GetProvider(ctx context.Context, providerID string) (*Provider, error) {
	if providerID == "" {
		return nil, fmt.Errorf("provider ID is required")
	}

	// Try cache first
	if provider, found := s.cache.GetProvider(ctx, providerID); found {
		return provider, nil
	}

	// Fetch from API
	s.log.WithField("provider_id", providerID).Debug("Cache miss, fetching provider from API")
	response, err := s.client.GetProvider(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider from API: %w", err)
	}

	// Convert and cache
	provider := s.convertProviderInfoToProvider(response)
	s.cache.SetProvider(ctx, provider)

	return provider, nil
}

// ListProviders lists all providers
func (s *Service) ListProviders(ctx context.Context) ([]*Provider, error) {
	// Check cache first for all providers
	cachedProviders := s.cache.GetAllProviders(ctx)
	if len(cachedProviders) > 0 {
		return cachedProviders, nil
	}

	// Fetch from API
	response, err := s.client.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	providers := make([]*Provider, 0, len(response.Providers))
	for i := range response.Providers {
		provider := s.convertProviderInfoToProvider(&response.Providers[i])
		providers = append(providers, provider)
		// Cache each provider
		s.cache.SetProvider(ctx, provider)
	}

	return providers, nil
}

// GetProviderModels retrieves all models for a specific provider
func (s *Service) GetProviderModels(ctx context.Context, providerID string) ([]*Model, error) {
	if providerID == "" {
		return nil, fmt.Errorf("provider ID is required")
	}

	// Try cache first
	if models, found := s.cache.GetModelsByProvider(ctx, providerID); found {
		return models, nil
	}

	// Fetch from API
	s.log.WithField("provider_id", providerID).Debug("Cache miss, fetching provider models from API")
	response, err := s.client.ListProviderModels(ctx, providerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider models: %w", err)
	}

	models := make([]*Model, 0, len(response.Models))
	for i := range response.Models {
		model := s.convertModelInfoToModel(&response.Models[i])
		models = append(models, model)
		s.cache.SetModel(ctx, model)
	}

	return models, nil
}

// SearchModels searches for models by query
func (s *Service) SearchModels(ctx context.Context, query string, filters *ModelFilters) ([]*Model, int, error) {
	opts := &ListModelsOptions{
		Search: query,
	}
	if filters != nil {
		opts.Provider = filters.Provider
		opts.Page = filters.Page
		opts.Limit = filters.Limit
	}

	response, err := s.client.SearchModels(ctx, query, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search models: %w", err)
	}

	models := make([]*Model, 0, len(response.Models))
	for i := range response.Models {
		model := s.convertModelInfoToModel(&response.Models[i])
		models = append(models, model)
	}

	return models, response.Total, nil
}

// RefreshCache refreshes the entire cache from the API
func (s *Service) RefreshCache(ctx context.Context) RefreshResult {
	startTime := time.Now()
	result := RefreshResult{
		Errors: make([]string, 0),
	}

	// Refresh providers
	providers, err := s.refreshProviders(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to refresh providers: %v", err))
	} else {
		result.ProvidersRefreshed = len(providers)
	}

	// Refresh models (fetch all models)
	modelsRefreshed, err := s.refreshModels(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to refresh models: %v", err))
	} else {
		result.ModelsRefreshed = modelsRefreshed
	}

	result.Duration = time.Since(startTime)
	result.Success = len(result.Errors) == 0

	s.cache.UpdateLastRefresh()

	return result
}

// RefreshProviderModels refreshes models for a specific provider
func (s *Service) RefreshProviderModels(ctx context.Context, providerID string) error {
	// Invalidate current cache for this provider
	s.cache.InvalidateProvider(ctx, providerID)

	// Fetch fresh data
	_, err := s.GetProviderModels(ctx, providerID)
	return err
}

// InvalidateCache invalidates specific entries or the entire cache
func (s *Service) InvalidateCache(ctx context.Context, modelID string, providerID string) {
	if modelID != "" {
		s.cache.InvalidateModel(ctx, modelID)
	}
	if providerID != "" {
		s.cache.InvalidateProvider(ctx, providerID)
	}
}

// InvalidateAll invalidates the entire cache
func (s *Service) InvalidateAll(ctx context.Context) {
	s.cache.InvalidateAll(ctx)
}

// CacheStats returns current cache statistics
func (s *Service) CacheStats() CacheStats {
	return s.cache.Stats()
}

// Internal methods

func (s *Service) backgroundRefreshLoop() {
	defer close(s.refreshDone)

	ticker := time.NewTicker(s.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopRefresh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			result := s.RefreshCache(ctx)
			cancel()

			if !result.Success {
				s.log.WithField("errors", result.Errors).Warn("Background cache refresh completed with errors")
			} else {
				s.log.WithFields(logrus.Fields{
					"models_refreshed":    result.ModelsRefreshed,
					"providers_refreshed": result.ProvidersRefreshed,
					"duration":            result.Duration,
				}).Info("Background cache refresh completed")
			}
		}
	}
}

func (s *Service) fetchModelFromAPI(ctx context.Context, modelID string) (*Model, error) {
	response, err := s.client.GetModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get model from API: %w", err)
	}

	model := s.convertModelInfoToModel(&response.Model)
	s.cache.SetModel(ctx, model)

	return model, nil
}

func (s *Service) refreshProviders(ctx context.Context) ([]*Provider, error) {
	response, err := s.client.ListProviders(ctx)
	if err != nil {
		return nil, err
	}

	providers := make([]*Provider, 0, len(response.Providers))
	for i := range response.Providers {
		provider := s.convertProviderInfoToProvider(&response.Providers[i])
		providers = append(providers, provider)
		s.cache.SetProvider(ctx, provider)
	}

	return providers, nil
}

func (s *Service) refreshModels(ctx context.Context) (int, error) {
	var totalModels int
	page := 1
	limit := 100

	for {
		opts := &ListModelsOptions{
			Page:  page,
			Limit: limit,
		}

		response, err := s.client.ListModels(ctx, opts)
		if err != nil {
			return totalModels, err
		}

		models := make([]Model, 0, len(response.Models))
		for i := range response.Models {
			model := s.convertModelInfoToModel(&response.Models[i])
			models = append(models, *model)
		}

		s.cache.SetModels(ctx, models)
		totalModels += len(models)

		// Check if there are more pages
		if len(response.Models) < limit || totalModels >= response.Total {
			break
		}

		page++
	}

	return totalModels, nil
}

func (s *Service) convertModelInfoToModel(info *ModelInfo) *Model {
	if info == nil {
		return nil
	}

	model := &Model{
		ID:            info.ID,
		Name:          info.Name,
		Provider:      info.Provider,
		DisplayName:   info.DisplayName,
		Description:   info.Description,
		ContextWindow: info.ContextWindow,
		MaxTokens:     info.MaxTokens,
		Tags:          info.Tags,
		Categories:    info.Categories,
		Family:        info.Family,
		Version:       info.Version,
		Metadata:      info.Metadata,
	}

	// Convert pricing
	if info.Pricing != nil {
		model.Pricing = &Pricing{
			InputCost:  info.Pricing.InputPrice,
			OutputCost: info.Pricing.OutputPrice,
			Currency:   info.Pricing.Currency,
			Unit:       info.Pricing.Unit,
		}
	}

	// Convert capabilities
	model.Capabilities = &ModelCapabilities{
		Vision:          info.Capabilities.Vision,
		FunctionCalling: info.Capabilities.FunctionCalling,
		Streaming:       info.Capabilities.Streaming,
		JSONMode:        info.Capabilities.JSONMode,
		ImageGeneration: info.Capabilities.ImageGeneration,
		Audio:           info.Capabilities.Audio,
		CodeGeneration:  info.Capabilities.CodeGeneration,
		Reasoning:       info.Capabilities.Reasoning,
		ToolUse:         info.Capabilities.ToolUse,
	}

	// Convert performance
	if info.Performance != nil {
		model.Performance = &ModelPerformance{
			BenchmarkScore:   info.Performance.BenchmarkScore,
			PopularityScore:  info.Performance.PopularityScore,
			ReliabilityScore: info.Performance.ReliabilityScore,
			Benchmarks:       info.Performance.Benchmarks,
		}
	}

	return model
}

func (s *Service) convertProviderInfoToProvider(info *ProviderInfo) *Provider {
	if info == nil {
		return nil
	}

	return &Provider{
		ID:          info.ID,
		Name:        info.Name,
		DisplayName: info.DisplayName,
		Description: info.Description,
		Website:     info.Website,
		APIDocsURL:  info.APIDocsURL,
		ModelsCount: info.ModelsCount,
		Features:    info.Features,
	}
}

// Utility methods for integration with existing LLM providers

// GetModelCapabilities returns capabilities for a specific model
func (s *Service) GetModelCapabilities(ctx context.Context, modelID string) (*ModelCapabilities, error) {
	model, err := s.GetModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	return model.Capabilities, nil
}

// GetModelPricing returns pricing for a specific model
func (s *Service) GetModelPricing(ctx context.Context, modelID string) (*Pricing, error) {
	model, err := s.GetModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	return model.Pricing, nil
}

// GetModelsByCapability returns models that have a specific capability
func (s *Service) GetModelsByCapability(ctx context.Context, capability string) ([]*Model, error) {
	allModels := s.cache.GetAllModels(ctx)
	if len(allModels) == 0 {
		// Refresh cache if empty
		s.RefreshCache(ctx)
		allModels = s.cache.GetAllModels(ctx)
	}

	matching := make([]*Model, 0)
	for _, model := range allModels {
		if model.Capabilities != nil && model.Capabilities.HasCapability(capability) {
			matching = append(matching, model)
		}
	}

	return matching, nil
}

// FindModelByName searches for a model by name (case-insensitive partial match)
func (s *Service) FindModelByName(ctx context.Context, name string) ([]*Model, error) {
	models, _, err := s.SearchModels(ctx, name, nil)
	return models, err
}
