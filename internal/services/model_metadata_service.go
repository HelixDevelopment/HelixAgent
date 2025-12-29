package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/database"
	"github.com/superagent/superagent/internal/modelsdev"
)

type ModelMetadataService struct {
	modelsdevClient *modelsdev.Client
	repository      *database.ModelMetadataRepository
	cache           CacheInterface
	config          *ModelMetadataConfig
	log             *logrus.Logger
}

type ModelMetadataConfig struct {
	RefreshInterval   time.Duration
	CacheTTL          time.Duration
	DefaultBatchSize  int
	MaxRetries        int
	RetryDelay        time.Duration
	EnableAutoRefresh bool
}

// CacheInterface defines the interface for model metadata caching
type CacheInterface interface {
	// Basic cache operations
	Get(ctx context.Context, modelID string) (*database.ModelMetadata, bool, error)
	Set(ctx context.Context, modelID string, metadata *database.ModelMetadata) error
	Delete(ctx context.Context, modelID string) error
	Clear(ctx context.Context) error
	Size(ctx context.Context) (int, error)

	// Bulk operations
	GetBulk(ctx context.Context, modelIDs []string) (map[string]*database.ModelMetadata, error)
	SetBulk(ctx context.Context, models map[string]*database.ModelMetadata) error

	// Provider and capability operations
	GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error)
	SetProviderModels(ctx context.Context, providerID string, models []*database.ModelMetadata) error
	DeleteProviderModels(ctx context.Context, providerID string) error
	GetByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error)
	SetByCapability(ctx context.Context, capability string, models []*database.ModelMetadata) error

	// Health check
	HealthCheck(ctx context.Context) error
}

func NewModelMetadataService(
	client *modelsdev.Client,
	repo *database.ModelMetadataRepository,
	cache CacheInterface,
	config *ModelMetadataConfig,
	log *logrus.Logger,
) *ModelMetadataService {
	if config == nil {
		config = getDefaultModelMetadataConfig()
	}

	service := &ModelMetadataService{
		modelsdevClient: client,
		repository:      repo,
		cache:           cache,
		config:          config,
		log:             log,
	}

	if config.EnableAutoRefresh {
		go service.startAutoRefresh()
	}

	return service
}

func getDefaultModelMetadataConfig() *ModelMetadataConfig {
	return &ModelMetadataConfig{
		RefreshInterval:   24 * time.Hour,
		CacheTTL:          1 * time.Hour,
		DefaultBatchSize:  100,
		MaxRetries:        3,
		RetryDelay:        5 * time.Second,
		EnableAutoRefresh: true,
	}
}

func (s *ModelMetadataService) GetModel(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
	if cached, exists, err := s.cache.Get(ctx, modelID); err == nil && exists {
		s.log.WithField("model_id", modelID).Debug("Cache hit for model")
		return cached, nil
	} else if err != nil {
		s.log.WithError(err).WithField("model_id", modelID).Warn("Cache error, falling back to database")
	}

	metadata, err := s.repository.GetModelMetadata(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get model metadata: %w", err)
	}

	// Store in cache (async to not block the request)
	go func() {
		ctx := context.Background()
		if err := s.cache.Set(ctx, modelID, metadata); err != nil {
			s.log.WithError(err).WithField("model_id", modelID).Warn("Failed to set cache")
		}
	}()

	return metadata, nil
}

func (s *ModelMetadataService) ListModels(ctx context.Context, providerID string, modelType string, page int, limit int) ([]*database.ModelMetadata, int, error) {
	offset := 0
	if page > 0 {
		offset = (page - 1) * limit
	}

	models, total, err := s.repository.ListModels(ctx, providerID, modelType, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list models: %w", err)
	}

	// Store in cache (async to not block the request)
	go func(models []*database.ModelMetadata) {
		ctx := context.Background()
		cacheEntries := make(map[string]*database.ModelMetadata)
		for _, model := range models {
			cacheEntries[model.ModelID] = model
		}

		if err := s.cache.SetBulk(ctx, cacheEntries); err != nil {
			s.log.WithError(err).Warn("Failed to set bulk cache")
		}
	}(models)

	return models, total, nil
}

func (s *ModelMetadataService) SearchModels(ctx context.Context, query string, page int, limit int) ([]*database.ModelMetadata, int, error) {
	offset := 0
	if page > 0 {
		offset = (page - 1) * limit
	}

	models, total, err := s.repository.SearchModels(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search models: %w", err)
	}

	// Store in cache (async to not block the request)
	go func(models []*database.ModelMetadata) {
		ctx := context.Background()
		cacheEntries := make(map[string]*database.ModelMetadata)
		for _, model := range models {
			cacheEntries[model.ModelID] = model
		}

		if err := s.cache.SetBulk(ctx, cacheEntries); err != nil {
			s.log.WithError(err).Warn("Failed to set bulk cache")
		}
	}(models)

	return models, total, nil
}

func (s *ModelMetadataService) RefreshModels(ctx context.Context) error {
	s.log.Info("Starting models refresh")

	history := &database.ModelsRefreshHistory{
		RefreshType: "full",
		Status:      "in_progress",
		StartedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	if err := s.repository.CreateRefreshHistory(ctx, history); err != nil {
		return fmt.Errorf("failed to create refresh history: %w", err)
	}

	providers, err := s.modelsdevClient.ListProviders(ctx)
	if err != nil {
		s.completeRefreshHistory(ctx, history, "failed", 0, 0, err.Error())
		return fmt.Errorf("failed to list providers: %w", err)
	}

	totalModels := 0
	failedModels := 0

	for _, provider := range providers.Providers {
		s.log.WithField("provider", provider.ID).Info("Refreshing models for provider")

		providerModels, err := s.refreshProviderModels(ctx, provider.ID)
		if err != nil {
			s.log.WithError(err).WithField("provider", provider.ID).Error("Failed to refresh provider models")
			failedModels++
			continue
		}

		totalModels += providerModels

		if err := s.repository.UpdateProviderSyncInfo(ctx, provider.ID, providerModels, providerModels); err != nil {
			s.log.WithError(err).WithField("provider", provider.ID).Error("Failed to update provider sync info")
		}
	}

	history.ModelsRefreshed = totalModels
	history.ModelsFailed = failedModels
	history.Status = "completed"

	completedAt := time.Now()
	history.CompletedAt = &completedAt
	duration := int(completedAt.Sub(history.StartedAt).Seconds())
	history.DurationSeconds = &duration

	if err := s.repository.CreateRefreshHistory(ctx, history); err != nil {
		s.log.WithError(err).Error("Failed to update refresh history")
	}

	s.log.WithFields(logrus.Fields{
		"total_models":  totalModels,
		"failed_models": failedModels,
		"duration":      history.DurationSeconds,
	}).Info("Models refresh completed")

	return nil
}

func (s *ModelMetadataService) RefreshProviderModels(ctx context.Context, providerID string) error {
	s.log.WithField("provider", providerID).Info("Refreshing models for provider")

	history := &database.ModelsRefreshHistory{
		RefreshType: "provider",
		Status:      "in_progress",
		StartedAt:   time.Now(),
		Metadata:    map[string]interface{}{"provider_id": providerID},
	}

	if err := s.repository.CreateRefreshHistory(ctx, history); err != nil {
		return fmt.Errorf("failed to create refresh history: %w", err)
	}

	modelCount, err := s.refreshProviderModels(ctx, providerID)
	if err != nil {
		s.completeRefreshHistory(ctx, history, "failed", 0, modelCount, err.Error())
		return err
	}

	history.ModelsRefreshed = modelCount
	history.Status = "completed"

	completedAt := time.Now()
	history.CompletedAt = &completedAt
	duration := int(completedAt.Sub(history.StartedAt).Seconds())
	history.DurationSeconds = &duration

	if err := s.repository.CreateRefreshHistory(ctx, history); err != nil {
		s.log.WithError(err).Error("Failed to update refresh history")
	}

	if err := s.repository.UpdateProviderSyncInfo(ctx, providerID, modelCount, modelCount); err != nil {
		s.log.WithError(err).Error("Failed to update provider sync info")
	}

	return nil
}

func (s *ModelMetadataService) GetRefreshHistory(ctx context.Context, limit int) ([]*database.ModelsRefreshHistory, error) {
	histories, err := s.repository.GetLatestRefreshHistory(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh history: %w", err)
	}

	return histories, nil
}

func (s *ModelMetadataService) GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error) {
	models, _, err := s.repository.ListModels(ctx, providerID, "", 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider models: %w", err)
	}

	return models, nil
}

func (s *ModelMetadataService) CompareModels(ctx context.Context, modelIDs []string) ([]*database.ModelMetadata, error) {
	models := make([]*database.ModelMetadata, 0, len(modelIDs))

	for _, modelID := range modelIDs {
		metadata, err := s.GetModel(ctx, modelID)
		if err != nil {
			s.log.WithError(err).WithField("model_id", modelID).Warn("Failed to get model for comparison")
			continue
		}
		models = append(models, metadata)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no valid models found for comparison")
	}

	return models, nil
}

func (s *ModelMetadataService) GetModelsByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error) {
	allModels, _, err := s.repository.ListModels(ctx, "", "", 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	filtered := make([]*database.ModelMetadata, 0)
	for _, model := range allModels {
		var hasCapability bool

		switch capability {
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
		}

		if hasCapability {
			filtered = append(filtered, model)
		}
	}

	return filtered, nil
}

func (s *ModelMetadataService) refreshProviderModels(ctx context.Context, providerID string) (int, error) {
	opts := &modelsdev.ListModelsOptions{
		Provider: providerID,
		Limit:    s.config.DefaultBatchSize,
	}

	allModels := make([]modelsdev.ModelInfo, 0)
	page := 1

	for {
		opts.Page = page
		response, err := s.modelsdevClient.ListModels(ctx, opts)
		if err != nil {
			return 0, fmt.Errorf("failed to list models for provider %s: %w", providerID, err)
		}

		allModels = append(allModels, response.Models...)

		if len(response.Models) < opts.Limit {
			break
		}

		page++
	}

	for _, modelInfo := range allModels {
		metadata := s.convertModelInfoToMetadata(modelInfo, providerID)

		if err := s.repository.CreateModelMetadata(ctx, metadata); err != nil {
			s.log.WithError(err).WithField("model_id", modelInfo.ID).Error("Failed to store model metadata")
			continue
		}

		if modelInfo.Performance != nil && len(modelInfo.Performance.Benchmarks) > 0 {
			s.storeBenchmarks(ctx, modelInfo.ID, modelInfo.Performance.Benchmarks)
		}

		go func(modelID string, metadata *database.ModelMetadata) {
			ctx := context.Background()
			if err := s.cache.Set(ctx, modelID, metadata); err != nil {
				s.log.WithError(err).WithField("model_id", modelID).Warn("Failed to set cache")
			}
		}(modelInfo.ID, metadata)
	}

	return len(allModels), nil
}

func (s *ModelMetadataService) storeBenchmarks(ctx context.Context, modelID string, benchmarks map[string]float64) error {
	for name, score := range benchmarks {
		benchmark := &database.ModelBenchmark{
			ModelID:       modelID,
			BenchmarkName: name,
			Score:         &score,
		}

		if err := s.repository.CreateBenchmark(ctx, benchmark); err != nil {
			s.log.WithError(err).WithFields(logrus.Fields{
				"model_id":  modelID,
				"benchmark": name,
			}).Error("Failed to store benchmark")
		}
	}

	return nil
}

func (s *ModelMetadataService) convertModelInfoToMetadata(info modelsdev.ModelInfo, providerID string) *database.ModelMetadata {
	var pricingInput, pricingOutput *float64
	if info.Pricing != nil {
		pricingInput = &info.Pricing.InputPrice
		pricingOutput = &info.Pricing.OutputPrice
	}

	var benchmarkScore, reliabilityScore *float64
	var popularityScore *int
	if info.Performance != nil {
		benchmarkScore = &info.Performance.BenchmarkScore
		popularityScore = &info.Performance.PopularityScore
		reliabilityScore = &info.Performance.ReliabilityScore
	}

	modelFamily := &info.Family
	if info.Family == "" {
		modelFamily = nil
	}

	version := &info.Version
	if info.Version == "" {
		version = nil
	}

	return &database.ModelMetadata{
		ModelID:                 info.ID,
		ModelName:               info.Name,
		ProviderID:              providerID,
		ProviderName:            info.Provider,
		Description:             info.Description,
		ContextWindow:           &info.ContextWindow,
		MaxTokens:               &info.MaxTokens,
		PricingInput:            pricingInput,
		PricingOutput:           pricingOutput,
		PricingCurrency:         "USD",
		SupportsVision:          info.Capabilities.Vision,
		SupportsFunctionCalling: info.Capabilities.FunctionCalling,
		SupportsStreaming:       info.Capabilities.Streaming,
		SupportsJSONMode:        info.Capabilities.JSONMode,
		SupportsImageGeneration: info.Capabilities.ImageGeneration,
		SupportsAudio:           info.Capabilities.Audio,
		SupportsCodeGeneration:  info.Capabilities.CodeGeneration,
		SupportsReasoning:       info.Capabilities.Reasoning,
		BenchmarkScore:          benchmarkScore,
		PopularityScore:         popularityScore,
		ReliabilityScore:        reliabilityScore,
		ModelFamily:             modelFamily,
		Version:                 version,
		Tags:                    info.Tags,
		ModelsDevURL:            &info.ID,
		ModelsDevID:             &info.ID,
		RawMetadata:             info.Metadata,
		LastRefreshedAt:         time.Now(),
	}
}

func (s *ModelMetadataService) completeRefreshHistory(ctx context.Context, history *database.ModelsRefreshHistory, status string, modelsRefreshed, modelsFailed int, errorMessage string) {
	history.Status = status
	history.ModelsRefreshed = modelsRefreshed
	history.ModelsFailed = modelsFailed
	history.ErrorMessage = &errorMessage

	completedAt := time.Now()
	history.CompletedAt = &completedAt
	duration := int(completedAt.Sub(history.StartedAt).Seconds())
	history.DurationSeconds = &duration

	if err := s.repository.CreateRefreshHistory(ctx, history); err != nil {
		s.log.WithError(err).Error("Failed to complete refresh history")
	}
}

func (s *ModelMetadataService) startAutoRefresh() {
	ticker := time.NewTicker(s.config.RefreshInterval)
	defer ticker.Stop()

	s.log.WithField("interval", s.config.RefreshInterval).Info("Starting auto refresh")

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		if err := s.RefreshModels(ctx); err != nil {
			s.log.WithError(err).Error("Auto refresh failed")
		}
		cancel()
	}
}

// InMemoryCache implements CacheInterface for in-memory caching
type InMemoryCache struct {
	mu     sync.RWMutex
	models map[string]*database.ModelMetadata
	ttl    time.Duration
	timers map[string]*time.Timer
}

func NewInMemoryCache(ttl time.Duration) *InMemoryCache {
	return &InMemoryCache{
		models: make(map[string]*database.ModelMetadata),
		ttl:    ttl,
		timers: make(map[string]*time.Timer),
	}
}

func (c *InMemoryCache) Get(ctx context.Context, modelID string) (*database.ModelMetadata, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	model, exists := c.models[modelID]
	return model, exists, nil
}

func (c *InMemoryCache) Set(ctx context.Context, modelID string, metadata *database.ModelMetadata) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if timer, exists := c.timers[modelID]; exists {
		timer.Stop()
	}

	c.models[modelID] = metadata
	c.timers[modelID] = time.AfterFunc(c.ttl, func() {
		ctx := context.Background()
		c.Delete(ctx, modelID)
	})

	return nil
}

func (c *InMemoryCache) Delete(ctx context.Context, modelID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.models, modelID)
	delete(c.timers, modelID)
	return nil
}

func (c *InMemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, timer := range c.timers {
		timer.Stop()
	}

	c.models = make(map[string]*database.ModelMetadata)
	c.timers = make(map[string]*time.Timer)
	return nil
}

func (c *InMemoryCache) Size(ctx context.Context) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.models), nil
}

func (c *InMemoryCache) GetBulk(ctx context.Context, modelIDs []string) (map[string]*database.ModelMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*database.ModelMetadata)
	for _, modelID := range modelIDs {
		if model, exists := c.models[modelID]; exists {
			result[modelID] = model
		}
	}

	return result, nil
}

func (c *InMemoryCache) SetBulk(ctx context.Context, models map[string]*database.ModelMetadata) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for modelID, metadata := range models {
		if timer, exists := c.timers[modelID]; exists {
			timer.Stop()
		}
		c.models[modelID] = metadata
		c.timers[modelID] = time.AfterFunc(c.ttl, func() {
			ctx := context.Background()
			c.Delete(ctx, modelID)
		})
	}

	return nil
}

func (c *InMemoryCache) GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error) {
	// In-memory cache doesn't support provider-specific queries
	return nil, nil
}

func (c *InMemoryCache) SetProviderModels(ctx context.Context, providerID string, models []*database.ModelMetadata) error {
	// In-memory cache doesn't support provider-specific caching
	return nil
}

func (c *InMemoryCache) DeleteProviderModels(ctx context.Context, providerID string) error {
	// In-memory cache doesn't support provider-specific deletion
	return nil
}

func (c *InMemoryCache) GetByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error) {
	// In-memory cache doesn't support capability-specific queries
	return nil, nil
}

func (c *InMemoryCache) SetByCapability(ctx context.Context, capability string, models []*database.ModelMetadata) error {
	// In-memory cache doesn't support capability-specific caching
	return nil
}

func (c *InMemoryCache) HealthCheck(ctx context.Context) error {
	// In-memory cache is always healthy
	return nil
}
