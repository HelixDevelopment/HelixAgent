package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/database"
	"github.com/superagent/superagent/internal/modelsdev"
)

// Test helper functions

func newModelMetadataTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// MockModelsDevClient mocks the modelsdev.Client
type MockModelsDevClient struct {
	listProvidersFunc        func(ctx context.Context) (*modelsdev.ProvidersListResponse, error)
	listModelsFunc           func(ctx context.Context, opts *modelsdev.ListModelsOptions) (*modelsdev.ModelsListResponse, error)
	listProviderModelsFunc   func(ctx context.Context, providerID string, opts *modelsdev.ListModelsOptions) (*modelsdev.ModelsListResponse, error)
}

func (m *MockModelsDevClient) ListProviders(ctx context.Context) (*modelsdev.ProvidersListResponse, error) {
	if m.listProvidersFunc != nil {
		return m.listProvidersFunc(ctx)
	}
	return &modelsdev.ProvidersListResponse{Providers: []modelsdev.ProviderInfo{}}, nil
}

func (m *MockModelsDevClient) ListModels(ctx context.Context, opts *modelsdev.ListModelsOptions) (*modelsdev.ModelsListResponse, error) {
	if m.listModelsFunc != nil {
		return m.listModelsFunc(ctx, opts)
	}
	return &modelsdev.ModelsListResponse{Models: []modelsdev.ModelInfo{}}, nil
}

// MockModelMetadataRepository mocks database.ModelMetadataRepository
type MockModelMetadataRepository struct {
	getModelMetadataFunc       func(ctx context.Context, modelID string) (*database.ModelMetadata, error)
	listModelsFunc             func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error)
	searchModelsFunc           func(ctx context.Context, query string, limit, offset int) ([]*database.ModelMetadata, int, error)
	createModelMetadataFunc    func(ctx context.Context, metadata *database.ModelMetadata) error
	createRefreshHistoryFunc   func(ctx context.Context, history *database.ModelsRefreshHistory) error
	getLatestRefreshHistoryFunc func(ctx context.Context, limit int) ([]*database.ModelsRefreshHistory, error)
	updateProviderSyncInfoFunc func(ctx context.Context, providerID string, totalModels, syncedModels int) error
	createBenchmarkFunc        func(ctx context.Context, benchmark *database.ModelBenchmark) error
}

func (m *MockModelMetadataRepository) GetModelMetadata(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
	if m.getModelMetadataFunc != nil {
		return m.getModelMetadataFunc(ctx, modelID)
	}
	return nil, errors.New("not found")
}

func (m *MockModelMetadataRepository) ListModels(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
	if m.listModelsFunc != nil {
		return m.listModelsFunc(ctx, providerID, modelType, limit, offset)
	}
	return []*database.ModelMetadata{}, 0, nil
}

func (m *MockModelMetadataRepository) SearchModels(ctx context.Context, query string, limit, offset int) ([]*database.ModelMetadata, int, error) {
	if m.searchModelsFunc != nil {
		return m.searchModelsFunc(ctx, query, limit, offset)
	}
	return []*database.ModelMetadata{}, 0, nil
}

func (m *MockModelMetadataRepository) CreateModelMetadata(ctx context.Context, metadata *database.ModelMetadata) error {
	if m.createModelMetadataFunc != nil {
		return m.createModelMetadataFunc(ctx, metadata)
	}
	return nil
}

func (m *MockModelMetadataRepository) CreateRefreshHistory(ctx context.Context, history *database.ModelsRefreshHistory) error {
	if m.createRefreshHistoryFunc != nil {
		return m.createRefreshHistoryFunc(ctx, history)
	}
	return nil
}

func (m *MockModelMetadataRepository) GetLatestRefreshHistory(ctx context.Context, limit int) ([]*database.ModelsRefreshHistory, error) {
	if m.getLatestRefreshHistoryFunc != nil {
		return m.getLatestRefreshHistoryFunc(ctx, limit)
	}
	return []*database.ModelsRefreshHistory{}, nil
}

func (m *MockModelMetadataRepository) UpdateProviderSyncInfo(ctx context.Context, providerID string, totalModels, syncedModels int) error {
	if m.updateProviderSyncInfoFunc != nil {
		return m.updateProviderSyncInfoFunc(ctx, providerID, totalModels, syncedModels)
	}
	return nil
}

func (m *MockModelMetadataRepository) CreateBenchmark(ctx context.Context, benchmark *database.ModelBenchmark) error {
	if m.createBenchmarkFunc != nil {
		return m.createBenchmarkFunc(ctx, benchmark)
	}
	return nil
}

// MockCache implements CacheInterface
type MockCache struct {
	mu                  sync.RWMutex
	models              map[string]*database.ModelMetadata
	getFunc             func(ctx context.Context, modelID string) (*database.ModelMetadata, bool, error)
	setFunc             func(ctx context.Context, modelID string, metadata *database.ModelMetadata) error
	deleteFunc          func(ctx context.Context, modelID string) error
	clearFunc           func(ctx context.Context) error
	sizeFunc            func(ctx context.Context) (int, error)
	getBulkFunc         func(ctx context.Context, modelIDs []string) (map[string]*database.ModelMetadata, error)
	setBulkFunc         func(ctx context.Context, models map[string]*database.ModelMetadata) error
	getProviderFunc     func(ctx context.Context, providerID string) ([]*database.ModelMetadata, error)
	setProviderFunc     func(ctx context.Context, providerID string, models []*database.ModelMetadata) error
	deleteProviderFunc  func(ctx context.Context, providerID string) error
	getByCapabilityFunc func(ctx context.Context, capability string) ([]*database.ModelMetadata, error)
	setByCapabilityFunc func(ctx context.Context, capability string, models []*database.ModelMetadata) error
	healthCheckFunc     func(ctx context.Context) error
}

func NewMockCache() *MockCache {
	return &MockCache{
		models: make(map[string]*database.ModelMetadata),
	}
}

func (c *MockCache) Get(ctx context.Context, modelID string) (*database.ModelMetadata, bool, error) {
	if c.getFunc != nil {
		return c.getFunc(ctx, modelID)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	model, exists := c.models[modelID]
	return model, exists, nil
}

func (c *MockCache) Set(ctx context.Context, modelID string, metadata *database.ModelMetadata) error {
	if c.setFunc != nil {
		return c.setFunc(ctx, modelID, metadata)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.models[modelID] = metadata
	return nil
}

func (c *MockCache) Delete(ctx context.Context, modelID string) error {
	if c.deleteFunc != nil {
		return c.deleteFunc(ctx, modelID)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.models, modelID)
	return nil
}

func (c *MockCache) Clear(ctx context.Context) error {
	if c.clearFunc != nil {
		return c.clearFunc(ctx)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.models = make(map[string]*database.ModelMetadata)
	return nil
}

func (c *MockCache) Size(ctx context.Context) (int, error) {
	if c.sizeFunc != nil {
		return c.sizeFunc(ctx)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.models), nil
}

func (c *MockCache) GetBulk(ctx context.Context, modelIDs []string) (map[string]*database.ModelMetadata, error) {
	if c.getBulkFunc != nil {
		return c.getBulkFunc(ctx, modelIDs)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]*database.ModelMetadata)
	for _, id := range modelIDs {
		if model, exists := c.models[id]; exists {
			result[id] = model
		}
	}
	return result, nil
}

func (c *MockCache) SetBulk(ctx context.Context, models map[string]*database.ModelMetadata) error {
	if c.setBulkFunc != nil {
		return c.setBulkFunc(ctx, models)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, model := range models {
		c.models[id] = model
	}
	return nil
}

func (c *MockCache) GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error) {
	if c.getProviderFunc != nil {
		return c.getProviderFunc(ctx, providerID)
	}
	return nil, nil
}

func (c *MockCache) SetProviderModels(ctx context.Context, providerID string, models []*database.ModelMetadata) error {
	if c.setProviderFunc != nil {
		return c.setProviderFunc(ctx, providerID, models)
	}
	return nil
}

func (c *MockCache) DeleteProviderModels(ctx context.Context, providerID string) error {
	if c.deleteProviderFunc != nil {
		return c.deleteProviderFunc(ctx, providerID)
	}
	return nil
}

func (c *MockCache) GetByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error) {
	if c.getByCapabilityFunc != nil {
		return c.getByCapabilityFunc(ctx, capability)
	}
	return nil, nil
}

func (c *MockCache) SetByCapability(ctx context.Context, capability string, models []*database.ModelMetadata) error {
	if c.setByCapabilityFunc != nil {
		return c.setByCapabilityFunc(ctx, capability, models)
	}
	return nil
}

func (c *MockCache) HealthCheck(ctx context.Context) error {
	if c.healthCheckFunc != nil {
		return c.healthCheckFunc(ctx)
	}
	return nil
}

// Tests for ModelMetadataConfig

func TestGetDefaultModelMetadataConfig(t *testing.T) {
	config := getDefaultModelMetadataConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 24*time.Hour, config.RefreshInterval)
	assert.Equal(t, 1*time.Hour, config.CacheTTL)
	assert.Equal(t, 100, config.DefaultBatchSize)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
	assert.True(t, config.EnableAutoRefresh)
}

func TestModelMetadataConfig_CustomValues(t *testing.T) {
	config := &ModelMetadataConfig{
		RefreshInterval:   12 * time.Hour,
		CacheTTL:          30 * time.Minute,
		DefaultBatchSize:  50,
		MaxRetries:        5,
		RetryDelay:        10 * time.Second,
		EnableAutoRefresh: false,
	}

	assert.Equal(t, 12*time.Hour, config.RefreshInterval)
	assert.Equal(t, 30*time.Minute, config.CacheTTL)
	assert.Equal(t, 50, config.DefaultBatchSize)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 10*time.Second, config.RetryDelay)
	assert.False(t, config.EnableAutoRefresh)
}

// Tests for InMemoryCache

func TestInMemoryCache_NewInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)

	require.NotNil(t, cache)
	assert.NotNil(t, cache.models)
	assert.NotNil(t, cache.timers)
	assert.Equal(t, 1*time.Hour, cache.ttl)
}

func TestInMemoryCache_Get(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	t.Run("get non-existent model", func(t *testing.T) {
		model, exists, err := cache.Get(ctx, "non-existent")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, model)
	})

	t.Run("get existing model", func(t *testing.T) {
		testModel := &database.ModelMetadata{
			ModelID:   "test-model",
			ModelName: "Test Model",
		}
		cache.models["test-model"] = testModel

		model, exists, err := cache.Get(ctx, "test-model")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "test-model", model.ModelID)
	})
}

func TestInMemoryCache_Set(t *testing.T) {
	cache := NewInMemoryCache(100 * time.Millisecond)
	ctx := context.Background()

	testModel := &database.ModelMetadata{
		ModelID:   "test-model",
		ModelName: "Test Model",
	}

	err := cache.Set(ctx, "test-model", testModel)
	require.NoError(t, err)

	model, exists, err := cache.Get(ctx, "test-model")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "test-model", model.ModelID)
}

func TestInMemoryCache_Set_TTLExpiration(t *testing.T) {
	cache := NewInMemoryCache(50 * time.Millisecond)
	ctx := context.Background()

	testModel := &database.ModelMetadata{
		ModelID:   "expiring-model",
		ModelName: "Expiring Model",
	}

	err := cache.Set(ctx, "expiring-model", testModel)
	require.NoError(t, err)

	// Model should exist immediately
	model, exists, _ := cache.Get(ctx, "expiring-model")
	assert.True(t, exists)
	assert.NotNil(t, model)

	// Wait for TTL expiration
	time.Sleep(100 * time.Millisecond)

	// Model should be expired
	model, exists, _ = cache.Get(ctx, "expiring-model")
	assert.False(t, exists)
	assert.Nil(t, model)
}

func TestInMemoryCache_Set_OverwriteExisting(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	model1 := &database.ModelMetadata{ModelID: "test", ModelName: "First"}
	model2 := &database.ModelMetadata{ModelID: "test", ModelName: "Second"}

	_ = cache.Set(ctx, "test", model1)
	_ = cache.Set(ctx, "test", model2)

	result, exists, _ := cache.Get(ctx, "test")
	assert.True(t, exists)
	assert.Equal(t, "Second", result.ModelName)
}

func TestInMemoryCache_Delete(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	cache.models["delete-me"] = &database.ModelMetadata{ModelID: "delete-me"}

	err := cache.Delete(ctx, "delete-me")
	require.NoError(t, err)

	_, exists, _ := cache.Get(ctx, "delete-me")
	assert.False(t, exists)
}

func TestInMemoryCache_Clear(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	cache.models["model1"] = &database.ModelMetadata{ModelID: "model1"}
	cache.models["model2"] = &database.ModelMetadata{ModelID: "model2"}

	err := cache.Clear(ctx)
	require.NoError(t, err)

	size, _ := cache.Size(ctx)
	assert.Equal(t, 0, size)
}

func TestInMemoryCache_Size(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	size, err := cache.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, size)

	cache.models["m1"] = &database.ModelMetadata{}
	cache.models["m2"] = &database.ModelMetadata{}

	size, err = cache.Size(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, size)
}

func TestInMemoryCache_GetBulk(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	cache.models["model1"] = &database.ModelMetadata{ModelID: "model1"}
	cache.models["model2"] = &database.ModelMetadata{ModelID: "model2"}
	cache.models["model3"] = &database.ModelMetadata{ModelID: "model3"}

	result, err := cache.GetBulk(ctx, []string{"model1", "model3", "non-existent"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Contains(t, result, "model1")
	assert.Contains(t, result, "model3")
}

func TestInMemoryCache_SetBulk(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	models := map[string]*database.ModelMetadata{
		"bulk1": {ModelID: "bulk1"},
		"bulk2": {ModelID: "bulk2"},
	}

	err := cache.SetBulk(ctx, models)
	require.NoError(t, err)

	size, _ := cache.Size(ctx)
	assert.Equal(t, 2, size)
}

func TestInMemoryCache_ProviderMethods_ReturnNil(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	// These methods return nil for InMemoryCache
	models, err := cache.GetProviderModels(ctx, "openai")
	assert.NoError(t, err)
	assert.Nil(t, models)

	err = cache.SetProviderModels(ctx, "openai", []*database.ModelMetadata{})
	assert.NoError(t, err)

	err = cache.DeleteProviderModels(ctx, "openai")
	assert.NoError(t, err)

	models, err = cache.GetByCapability(ctx, "vision")
	assert.NoError(t, err)
	assert.Nil(t, models)

	err = cache.SetByCapability(ctx, "vision", []*database.ModelMetadata{})
	assert.NoError(t, err)
}

func TestInMemoryCache_HealthCheck(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	err := cache.HealthCheck(ctx)
	assert.NoError(t, err)
}

// Concurrent access tests for InMemoryCache

func TestInMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			modelID := "model-" + string(rune(id))

			// Set
			_ = cache.Set(ctx, modelID, &database.ModelMetadata{ModelID: modelID})

			// Get
			_, _, _ = cache.Get(ctx, modelID)

			// Size
			_, _ = cache.Size(ctx)

			// Delete
			_ = cache.Delete(ctx, modelID)
		}(i)
	}

	wg.Wait()
}

// Tests for ModelMetadataService.GetModel

func TestModelMetadataService_GetModel_CacheHit(t *testing.T) {
	cache := NewMockCache()
	cachedModel := &database.ModelMetadata{ModelID: "cached-model", ModelName: "Cached Model"}
	cache.models["cached-model"] = cachedModel

	service := &ModelMetadataService{
		cache:  cache,
		config: getDefaultModelMetadataConfig(),
		log:    newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	result, err := service.GetModel(ctx, "cached-model")

	require.NoError(t, err)
	assert.Equal(t, "cached-model", result.ModelID)
	assert.Equal(t, "Cached Model", result.ModelName)
}

func TestModelMetadataService_GetModel_CacheMiss(t *testing.T) {
	cache := NewMockCache()
	repo := &MockModelMetadataRepository{
		getModelMetadataFunc: func(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
			return &database.ModelMetadata{ModelID: modelID, ModelName: "From DB"}, nil
		},
	}

	service := &ModelMetadataService{
		cache:      cache,
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	result, err := service.GetModel(ctx, "db-model")

	require.NoError(t, err)
	assert.Equal(t, "db-model", result.ModelID)
	assert.Equal(t, "From DB", result.ModelName)
}

func TestModelMetadataService_GetModel_CacheError(t *testing.T) {
	cache := NewMockCache()
	cache.getFunc = func(ctx context.Context, modelID string) (*database.ModelMetadata, bool, error) {
		return nil, false, errors.New("cache error")
	}

	repo := &MockModelMetadataRepository{
		getModelMetadataFunc: func(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
			return &database.ModelMetadata{ModelID: modelID, ModelName: "From DB"}, nil
		},
	}

	service := &ModelMetadataService{
		cache:      cache,
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	result, err := service.GetModel(ctx, "test-model")

	// Should fall back to database
	require.NoError(t, err)
	assert.Equal(t, "From DB", result.ModelName)
}

func TestModelMetadataService_GetModel_NotFound(t *testing.T) {
	cache := NewMockCache()
	repo := &MockModelMetadataRepository{
		getModelMetadataFunc: func(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
			return nil, errors.New("model not found")
		},
	}

	service := &ModelMetadataService{
		cache:      cache,
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	result, err := service.GetModel(ctx, "non-existent")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get model metadata")
}

// Tests for ModelMetadataService.ListModels

func TestModelMetadataService_ListModels(t *testing.T) {
	cache := NewMockCache()
	repo := &MockModelMetadataRepository{
		listModelsFunc: func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
			models := []*database.ModelMetadata{
				{ModelID: "model1", ProviderID: providerID},
				{ModelID: "model2", ProviderID: providerID},
			}
			return models, 2, nil
		},
	}

	service := &ModelMetadataService{
		cache:      cache,
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, total, err := service.ListModels(ctx, "openai", "chat", 1, 10)

	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, 2, total)
}

func TestModelMetadataService_ListModels_Pagination(t *testing.T) {
	repo := &MockModelMetadataRepository{
		listModelsFunc: func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
			// Verify pagination calculation
			assert.Equal(t, 10, limit)
			assert.Equal(t, 20, offset) // Page 3 with limit 10 = offset 20
			return []*database.ModelMetadata{}, 100, nil
		},
	}

	service := &ModelMetadataService{
		cache:      NewMockCache(),
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	_, _, err := service.ListModels(ctx, "", "", 3, 10)

	require.NoError(t, err)
}

func TestModelMetadataService_ListModels_Error(t *testing.T) {
	repo := &MockModelMetadataRepository{
		listModelsFunc: func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
			return nil, 0, errors.New("database error")
		},
	}

	service := &ModelMetadataService{
		cache:      NewMockCache(),
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, total, err := service.ListModels(ctx, "", "", 1, 10)

	assert.Error(t, err)
	assert.Nil(t, models)
	assert.Equal(t, 0, total)
}

// Tests for ModelMetadataService.SearchModels

func TestModelMetadataService_SearchModels(t *testing.T) {
	repo := &MockModelMetadataRepository{
		searchModelsFunc: func(ctx context.Context, query string, limit, offset int) ([]*database.ModelMetadata, int, error) {
			if query == "gpt" {
				return []*database.ModelMetadata{
					{ModelID: "gpt-4", ModelName: "GPT-4"},
					{ModelID: "gpt-3.5", ModelName: "GPT-3.5"},
				}, 2, nil
			}
			return []*database.ModelMetadata{}, 0, nil
		},
	}

	service := &ModelMetadataService{
		cache:      NewMockCache(),
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, total, err := service.SearchModels(ctx, "gpt", 1, 10)

	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Equal(t, 2, total)
}

func TestModelMetadataService_SearchModels_NoResults(t *testing.T) {
	repo := &MockModelMetadataRepository{
		searchModelsFunc: func(ctx context.Context, query string, limit, offset int) ([]*database.ModelMetadata, int, error) {
			return []*database.ModelMetadata{}, 0, nil
		},
	}

	service := &ModelMetadataService{
		cache:      NewMockCache(),
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, total, err := service.SearchModels(ctx, "nonexistent", 1, 10)

	require.NoError(t, err)
	assert.Empty(t, models)
	assert.Equal(t, 0, total)
}

// Tests for ModelMetadataService.GetRefreshHistory

func TestModelMetadataService_GetRefreshHistory(t *testing.T) {
	repo := &MockModelMetadataRepository{
		getLatestRefreshHistoryFunc: func(ctx context.Context, limit int) ([]*database.ModelsRefreshHistory, error) {
			return []*database.ModelsRefreshHistory{
				{ID: "1", Status: "completed", ModelsRefreshed: 100},
				{ID: "2", Status: "completed", ModelsRefreshed: 95},
			}, nil
		},
	}

	service := &ModelMetadataService{
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	history, err := service.GetRefreshHistory(ctx, 10)

	require.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestModelMetadataService_GetRefreshHistory_Error(t *testing.T) {
	repo := &MockModelMetadataRepository{
		getLatestRefreshHistoryFunc: func(ctx context.Context, limit int) ([]*database.ModelsRefreshHistory, error) {
			return nil, errors.New("database error")
		},
	}

	service := &ModelMetadataService{
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	history, err := service.GetRefreshHistory(ctx, 10)

	assert.Error(t, err)
	assert.Nil(t, history)
}

// Tests for ModelMetadataService.GetProviderModels

func TestModelMetadataService_GetProviderModels(t *testing.T) {
	repo := &MockModelMetadataRepository{
		listModelsFunc: func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
			if providerID == "anthropic" {
				return []*database.ModelMetadata{
					{ModelID: "claude-3", ProviderID: "anthropic"},
				}, 1, nil
			}
			return []*database.ModelMetadata{}, 0, nil
		},
	}

	service := &ModelMetadataService{
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, err := service.GetProviderModels(ctx, "anthropic")

	require.NoError(t, err)
	assert.Len(t, models, 1)
	assert.Equal(t, "anthropic", models[0].ProviderID)
}

// Tests for ModelMetadataService.CompareModels

func TestModelMetadataService_CompareModels(t *testing.T) {
	cache := NewMockCache()
	cache.models["model1"] = &database.ModelMetadata{ModelID: "model1", ModelName: "Model 1"}
	cache.models["model2"] = &database.ModelMetadata{ModelID: "model2", ModelName: "Model 2"}

	service := &ModelMetadataService{
		cache:  cache,
		config: getDefaultModelMetadataConfig(),
		log:    newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, err := service.CompareModels(ctx, []string{"model1", "model2"})

	require.NoError(t, err)
	assert.Len(t, models, 2)
}

func TestModelMetadataService_CompareModels_SomeNotFound(t *testing.T) {
	cache := NewMockCache()
	cache.models["model1"] = &database.ModelMetadata{ModelID: "model1"}
	// model2 not in cache

	repo := &MockModelMetadataRepository{
		getModelMetadataFunc: func(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
			return nil, errors.New("not found")
		},
	}

	service := &ModelMetadataService{
		cache:      cache,
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, err := service.CompareModels(ctx, []string{"model1", "model2"})

	// Should return the models that were found
	require.NoError(t, err)
	assert.Len(t, models, 1)
}

func TestModelMetadataService_CompareModels_NoneFound(t *testing.T) {
	cache := NewMockCache()
	repo := &MockModelMetadataRepository{
		getModelMetadataFunc: func(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
			return nil, errors.New("not found")
		},
	}

	service := &ModelMetadataService{
		cache:      cache,
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, err := service.CompareModels(ctx, []string{"unknown1", "unknown2"})

	assert.Error(t, err)
	assert.Nil(t, models)
	assert.Contains(t, err.Error(), "no valid models found")
}

// Tests for ModelMetadataService.GetModelsByCapability

func TestModelMetadataService_GetModelsByCapability(t *testing.T) {
	repo := &MockModelMetadataRepository{
		listModelsFunc: func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
			return []*database.ModelMetadata{
				{ModelID: "gpt-4v", SupportsVision: true},
				{ModelID: "claude-3", SupportsVision: true},
				{ModelID: "gpt-4", SupportsVision: false},
			}, 3, nil
		},
	}

	service := &ModelMetadataService{
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	models, err := service.GetModelsByCapability(ctx, "vision")

	require.NoError(t, err)
	assert.Len(t, models, 2)
}

func TestModelMetadataService_GetModelsByCapability_AllTypes(t *testing.T) {
	testCases := []struct {
		capability string
		models     []*database.ModelMetadata
		expected   int
	}{
		{
			capability: "vision",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsVision: true},
				{ModelID: "m2", SupportsVision: false},
			},
			expected: 1,
		},
		{
			capability: "function_calling",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsFunctionCalling: true},
				{ModelID: "m2", SupportsFunctionCalling: true},
			},
			expected: 2,
		},
		{
			capability: "streaming",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsStreaming: true},
			},
			expected: 1,
		},
		{
			capability: "json_mode",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsJSONMode: true},
			},
			expected: 1,
		},
		{
			capability: "image_generation",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsImageGeneration: true},
			},
			expected: 1,
		},
		{
			capability: "audio",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsAudio: true},
			},
			expected: 1,
		},
		{
			capability: "code_generation",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsCodeGeneration: true},
			},
			expected: 1,
		},
		{
			capability: "reasoning",
			models: []*database.ModelMetadata{
				{ModelID: "m1", SupportsReasoning: true},
			},
			expected: 1,
		},
		{
			capability: "unknown",
			models: []*database.ModelMetadata{
				{ModelID: "m1"},
			},
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.capability, func(t *testing.T) {
			repo := &MockModelMetadataRepository{
				listModelsFunc: func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
					return tc.models, len(tc.models), nil
				},
			}

			service := &ModelMetadataService{
				repository: repo,
				config:     getDefaultModelMetadataConfig(),
				log:        newModelMetadataTestLogger(),
			}

			ctx := context.Background()
			models, err := service.GetModelsByCapability(ctx, tc.capability)

			require.NoError(t, err)
			assert.Len(t, models, tc.expected)
		})
	}
}

// Tests for convertModelInfoToMetadata

func TestModelMetadataService_ConvertModelInfoToMetadata(t *testing.T) {
	service := &ModelMetadataService{
		log: newModelMetadataTestLogger(),
	}

	inputPrice := 0.01
	outputPrice := 0.03
	benchmarkScore := 0.95
	popularityScore := 100
	reliabilityScore := 0.99

	info := modelsdev.ModelInfo{
		ID:            "gpt-4",
		Name:          "GPT-4",
		Provider:      "openai",
		Description:   "Advanced language model",
		ContextWindow: 128000,
		MaxTokens:     4096,
		Pricing: &modelsdev.ModelPricing{
			InputPrice:  inputPrice,
			OutputPrice: outputPrice,
		},
		Capabilities: modelsdev.ModelCapabilities{
			Vision:          true,
			FunctionCalling: true,
			Streaming:       true,
			JSONMode:        true,
			CodeGeneration:  true,
			Reasoning:       true,
		},
		Performance: &modelsdev.ModelPerformance{
			BenchmarkScore:   benchmarkScore,
			PopularityScore:  popularityScore,
			ReliabilityScore: reliabilityScore,
			Benchmarks:       map[string]float64{"mmlu": 0.9},
		},
		Tags:    []string{"premium", "flagship"},
		Family:  "gpt",
		Version: "4.0",
		Metadata: map[string]interface{}{
			"custom": "value",
		},
	}

	result := service.convertModelInfoToMetadata(info, "openai")

	assert.Equal(t, "gpt-4", result.ModelID)
	assert.Equal(t, "GPT-4", result.ModelName)
	assert.Equal(t, "openai", result.ProviderID)
	assert.Equal(t, "openai", result.ProviderName)
	assert.Equal(t, "Advanced language model", result.Description)
	assert.Equal(t, 128000, *result.ContextWindow)
	assert.Equal(t, 4096, *result.MaxTokens)
	assert.Equal(t, inputPrice, *result.PricingInput)
	assert.Equal(t, outputPrice, *result.PricingOutput)
	assert.True(t, result.SupportsVision)
	assert.True(t, result.SupportsFunctionCalling)
	assert.True(t, result.SupportsStreaming)
	assert.True(t, result.SupportsJSONMode)
	assert.True(t, result.SupportsCodeGeneration)
	assert.True(t, result.SupportsReasoning)
	assert.Equal(t, benchmarkScore, *result.BenchmarkScore)
	assert.Equal(t, popularityScore, *result.PopularityScore)
	assert.Equal(t, reliabilityScore, *result.ReliabilityScore)
	assert.Equal(t, "gpt", *result.ModelFamily)
	assert.Equal(t, "4.0", *result.Version)
	assert.Contains(t, result.Tags, "premium")
}

func TestModelMetadataService_ConvertModelInfoToMetadata_NilPricing(t *testing.T) {
	service := &ModelMetadataService{
		log: newModelMetadataTestLogger(),
	}

	info := modelsdev.ModelInfo{
		ID:       "simple-model",
		Name:     "Simple Model",
		Pricing:  nil,
		Performance: nil,
	}

	result := service.convertModelInfoToMetadata(info, "provider")

	assert.Nil(t, result.PricingInput)
	assert.Nil(t, result.PricingOutput)
	assert.Nil(t, result.BenchmarkScore)
	assert.Nil(t, result.PopularityScore)
}

func TestModelMetadataService_ConvertModelInfoToMetadata_EmptyFamilyVersion(t *testing.T) {
	service := &ModelMetadataService{
		log: newModelMetadataTestLogger(),
	}

	info := modelsdev.ModelInfo{
		ID:      "model",
		Name:    "Model",
		Family:  "",
		Version: "",
	}

	result := service.convertModelInfoToMetadata(info, "provider")

	assert.Nil(t, result.ModelFamily)
	assert.Nil(t, result.Version)
}

// Benchmark tests

func BenchmarkModelMetadataCache_Get(b *testing.B) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		cache.models[string(rune(i))] = &database.ModelMetadata{ModelID: string(rune(i))}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = cache.Get(ctx, string(rune(i%1000)))
	}
}

func BenchmarkModelMetadataCache_Set(b *testing.B) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, string(rune(i%1000)), &database.ModelMetadata{ModelID: string(rune(i % 1000))})
	}
}

func BenchmarkInMemoryCache_GetBulk(b *testing.B) {
	cache := NewInMemoryCache(1 * time.Hour)
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		cache.models[string(rune(i))] = &database.ModelMetadata{ModelID: string(rune(i))}
	}

	ids := make([]string, 100)
	for i := 0; i < 100; i++ {
		ids[i] = string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetBulk(ctx, ids)
	}
}

// Table-driven tests

func TestModelMetadataService_ListModels_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		expectedOffset int
	}{
		{"page 0", 0, 10, 0},
		{"page 1", 1, 10, 0},
		{"page 2", 2, 10, 10},
		{"page 3 limit 20", 3, 20, 40},
		{"page 5 limit 5", 5, 5, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedOffset int
			repo := &MockModelMetadataRepository{
				listModelsFunc: func(ctx context.Context, providerID, modelType string, limit, offset int) ([]*database.ModelMetadata, int, error) {
					capturedOffset = offset
					return []*database.ModelMetadata{}, 0, nil
				},
			}

			service := &ModelMetadataService{
				cache:      NewMockCache(),
				repository: repo,
				config:     getDefaultModelMetadataConfig(),
				log:        newModelMetadataTestLogger(),
			}

			ctx := context.Background()
			_, _, _ = service.ListModels(ctx, "", "", tt.page, tt.limit)

			assert.Equal(t, tt.expectedOffset, capturedOffset)
		})
	}
}

// Tests for error handling edge cases

func TestModelMetadataService_GetModel_SetCacheError(t *testing.T) {
	cache := NewMockCache()
	cache.setFunc = func(ctx context.Context, modelID string, metadata *database.ModelMetadata) error {
		return errors.New("cache write error")
	}

	repo := &MockModelMetadataRepository{
		getModelMetadataFunc: func(ctx context.Context, modelID string) (*database.ModelMetadata, error) {
			return &database.ModelMetadata{ModelID: modelID}, nil
		},
	}

	service := &ModelMetadataService{
		cache:      cache,
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	result, err := service.GetModel(ctx, "test")

	// Should still succeed even if cache write fails
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// Tests for storeBenchmarks

func TestModelMetadataService_StoreBenchmarks(t *testing.T) {
	var storedBenchmarks []*database.ModelBenchmark
	repo := &MockModelMetadataRepository{
		createBenchmarkFunc: func(ctx context.Context, benchmark *database.ModelBenchmark) error {
			storedBenchmarks = append(storedBenchmarks, benchmark)
			return nil
		},
	}

	service := &ModelMetadataService{
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	benchmarks := map[string]float64{
		"mmlu":  0.9,
		"gsm8k": 0.85,
	}

	_ = service.storeBenchmarks(ctx, "model-123", benchmarks)

	assert.Len(t, storedBenchmarks, 2)
}

func TestModelMetadataService_StoreBenchmarks_Error(t *testing.T) {
	repo := &MockModelMetadataRepository{
		createBenchmarkFunc: func(ctx context.Context, benchmark *database.ModelBenchmark) error {
			return errors.New("database error")
		},
	}

	service := &ModelMetadataService{
		repository: repo,
		config:     getDefaultModelMetadataConfig(),
		log:        newModelMetadataTestLogger(),
	}

	ctx := context.Background()
	benchmarks := map[string]float64{"test": 0.9}

	// Should not panic even on error
	_ = service.storeBenchmarks(ctx, "model", benchmarks)
}

// Test for CacheInterface compliance

func TestInMemoryCache_ImplementsCacheInterface(t *testing.T) {
	var _ CacheInterface = (*InMemoryCache)(nil)
}

func TestMockCache_ImplementsCacheInterface(t *testing.T) {
	var _ CacheInterface = (*MockCache)(nil)
}
