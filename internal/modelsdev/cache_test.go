package modelsdev

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	cache := NewCache(nil)
	require.NotNil(t, cache)
	defer func() { _ = cache.Close() }()

	assert.NotNil(t, cache.state)
	stats := cache.Stats()
	assert.Equal(t, 0, stats.ModelCount)
	assert.Equal(t, 0, stats.ProviderCount)
}

func TestNewCache_WithConfig(t *testing.T) {
	config := &CacheConfig{
		ModelTTL:        30 * time.Minute,
		ProviderTTL:     1 * time.Hour,
		MaxModels:       100,
		MaxProviders:    20,
		CleanupInterval: 5 * time.Minute,
	}

	cache := NewCache(config)
	require.NotNil(t, cache)
	defer func() { _ = cache.Close() }()

	assert.Equal(t, 30*time.Minute, cache.config.ModelTTL)
	assert.Equal(t, 1*time.Hour, cache.config.ProviderTTL)
	assert.Equal(t, 100, cache.config.MaxModels)
	assert.Equal(t, 20, cache.config.MaxProviders)
}

func TestCache_SetAndGetModel(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	model := &Model{
		ID:       "test-model-1",
		Name:     "Test Model",
		Provider: "test-provider",
	}

	// Set model
	cache.SetModel(ctx, model)

	// Get model
	retrieved, found := cache.GetModel(ctx, "test-model-1")
	assert.True(t, found)
	require.NotNil(t, retrieved)
	assert.Equal(t, "test-model-1", retrieved.ID)
	assert.Equal(t, "Test Model", retrieved.Name)
}

func TestCache_GetModel_NotFound(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	retrieved, found := cache.GetModel(ctx, "nonexistent")
	assert.False(t, found)
	assert.Nil(t, retrieved)
}

func TestCache_GetModel_Expired(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        10 * time.Millisecond,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	model := &Model{
		ID:   "test-model",
		Name: "Test Model",
	}

	cache.SetModel(ctx, model)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	retrieved, found := cache.GetModel(ctx, "test-model")
	assert.False(t, found)
	assert.Nil(t, retrieved)
}

func TestCache_SetAndGetProvider(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	provider := &Provider{
		ID:   "test-provider",
		Name: "Test Provider",
	}

	cache.SetProvider(ctx, provider)

	retrieved, found := cache.GetProvider(ctx, "test-provider")
	assert.True(t, found)
	require.NotNil(t, retrieved)
	assert.Equal(t, "test-provider", retrieved.ID)
	assert.Equal(t, "Test Provider", retrieved.Name)
}

func TestCache_GetProvider_NotFound(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	retrieved, found := cache.GetProvider(ctx, "nonexistent")
	assert.False(t, found)
	assert.Nil(t, retrieved)
}

func TestCache_SetModels(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	models := []Model{
		{ID: "model-1", Name: "Model 1", Provider: "provider-a"},
		{ID: "model-2", Name: "Model 2", Provider: "provider-a"},
		{ID: "model-3", Name: "Model 3", Provider: "provider-b"},
	}

	cache.SetModels(ctx, models)

	// Verify all models are cached
	for _, m := range models {
		retrieved, found := cache.GetModel(ctx, m.ID)
		assert.True(t, found)
		assert.Equal(t, m.ID, retrieved.ID)
	}
}

func TestCache_SetProviders(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	providers := []Provider{
		{ID: "provider-1", Name: "Provider 1"},
		{ID: "provider-2", Name: "Provider 2"},
	}

	cache.SetProviders(ctx, providers)

	for _, p := range providers {
		retrieved, found := cache.GetProvider(ctx, p.ID)
		assert.True(t, found)
		assert.Equal(t, p.ID, retrieved.ID)
	}
}

func TestCache_GetModelsByProvider(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	models := []Model{
		{ID: "model-1", Name: "Model 1", Provider: "provider-a"},
		{ID: "model-2", Name: "Model 2", Provider: "provider-a"},
		{ID: "model-3", Name: "Model 3", Provider: "provider-b"},
	}

	cache.SetModels(ctx, models)

	// Get models for provider-a
	providerModels, found := cache.GetModelsByProvider(ctx, "provider-a")
	assert.True(t, found)
	assert.Len(t, providerModels, 2)

	// Get models for provider-b
	providerModels, found = cache.GetModelsByProvider(ctx, "provider-b")
	assert.True(t, found)
	assert.Len(t, providerModels, 1)

	// Get models for nonexistent provider
	_, found = cache.GetModelsByProvider(ctx, "nonexistent")
	assert.False(t, found)
}

func TestCache_GetAllModels(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	models := []Model{
		{ID: "model-1", Name: "Model 1"},
		{ID: "model-2", Name: "Model 2"},
		{ID: "model-3", Name: "Model 3"},
	}

	cache.SetModels(ctx, models)

	allModels := cache.GetAllModels(ctx)
	assert.Len(t, allModels, 3)
}

func TestCache_GetAllProviders(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	providers := []Provider{
		{ID: "provider-1", Name: "Provider 1"},
		{ID: "provider-2", Name: "Provider 2"},
	}

	cache.SetProviders(ctx, providers)

	allProviders := cache.GetAllProviders(ctx)
	assert.Len(t, allProviders, 2)
}

func TestCache_InvalidateModel(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()
	model := &Model{
		ID:       "test-model",
		Name:     "Test Model",
		Provider: "test-provider",
	}

	cache.SetModel(ctx, model)

	// Verify it's cached
	_, found := cache.GetModel(ctx, "test-model")
	assert.True(t, found)

	// Invalidate
	cache.InvalidateModel(ctx, "test-model")

	// Verify it's removed
	_, found = cache.GetModel(ctx, "test-model")
	assert.False(t, found)
}

func TestCache_InvalidateProvider(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		ProviderTTL:     1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
		MaxProviders:    50,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Add provider and models
	provider := &Provider{ID: "test-provider", Name: "Test Provider"}
	cache.SetProvider(ctx, provider)

	models := []Model{
		{ID: "model-1", Name: "Model 1", Provider: "test-provider"},
		{ID: "model-2", Name: "Model 2", Provider: "test-provider"},
	}
	cache.SetModels(ctx, models)

	// Verify cached
	_, found := cache.GetProvider(ctx, "test-provider")
	assert.True(t, found)
	providerModels, found := cache.GetModelsByProvider(ctx, "test-provider")
	assert.True(t, found)
	assert.Len(t, providerModels, 2)

	// Invalidate provider
	cache.InvalidateProvider(ctx, "test-provider")

	// Verify removed
	_, found = cache.GetProvider(ctx, "test-provider")
	assert.False(t, found)
	_, found = cache.GetModelsByProvider(ctx, "test-provider")
	assert.False(t, found)
}

func TestCache_InvalidateAll(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		ProviderTTL:     1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
		MaxProviders:    50,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Add data
	cache.SetModel(ctx, &Model{ID: "model-1", Name: "Model 1"})
	cache.SetProvider(ctx, &Provider{ID: "provider-1", Name: "Provider 1"})

	// Verify cached
	assert.Len(t, cache.GetAllModels(ctx), 1)
	assert.Len(t, cache.GetAllProviders(ctx), 1)

	// Invalidate all
	cache.InvalidateAll(ctx)

	// Verify cleared
	assert.Empty(t, cache.GetAllModels(ctx))
	assert.Empty(t, cache.GetAllProviders(ctx))
}

func TestCache_Stats(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		ProviderTTL:     1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
		MaxProviders:    50,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Initial stats
	stats := cache.Stats()
	assert.Equal(t, 0, stats.ModelCount)
	assert.Equal(t, 0, stats.ProviderCount)

	// Add data
	cache.SetModel(ctx, &Model{ID: "model-1", Name: "Model 1"})
	cache.SetProvider(ctx, &Provider{ID: "provider-1", Name: "Provider 1"})

	// Check stats
	stats = cache.Stats()
	assert.Equal(t, 1, stats.ModelCount)
	assert.Equal(t, 1, stats.ProviderCount)

	// Generate hits and misses
	cache.GetModel(ctx, "model-1")     // hit
	cache.GetModel(ctx, "nonexistent") // miss

	stats = cache.Stats()
	assert.Equal(t, int64(1), stats.TotalHits)
	assert.Equal(t, int64(1), stats.TotalMisses)
	assert.Equal(t, 0.5, stats.HitRate)
}

func TestCache_Eviction(t *testing.T) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       3,
	})
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Add more models than max
	for i := 0; i < 5; i++ {
		cache.SetModel(ctx, &Model{
			ID:   fmt.Sprintf("model-%d", i),
			Name: fmt.Sprintf("Model %d", i),
		})
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	// Should only have 3 models (MaxModels)
	allModels := cache.GetAllModels(ctx)
	assert.LessOrEqual(t, len(allModels), 3)
}

func TestCache_SetModel_NilModel(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Should not panic
	cache.SetModel(ctx, nil)
	assert.Empty(t, cache.GetAllModels(ctx))
}

func TestCache_SetModel_EmptyID(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Should not add model with empty ID
	cache.SetModel(ctx, &Model{ID: "", Name: "Test"})
	assert.Empty(t, cache.GetAllModels(ctx))
}

func TestCache_SetProvider_NilProvider(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Should not panic
	cache.SetProvider(ctx, nil)
	assert.Empty(t, cache.GetAllProviders(ctx))
}

func TestCache_SetProvider_EmptyID(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	ctx := context.Background()

	// Should not add provider with empty ID
	cache.SetProvider(ctx, &Provider{ID: "", Name: "Test"})
	assert.Empty(t, cache.GetAllProviders(ctx))
}

func TestCache_UpdateLastRefresh(t *testing.T) {
	cache := NewCache(nil)
	defer func() { _ = cache.Close() }()

	// Initial state - lastRefresh should be zero
	stats := cache.Stats()
	assert.True(t, stats.LastRefresh.IsZero())

	// Update last refresh
	cache.UpdateLastRefresh()

	// Check it's updated
	stats = cache.Stats()
	assert.False(t, stats.LastRefresh.IsZero())
	assert.True(t, time.Since(stats.LastRefresh) < 1*time.Second)
}

func TestCachedModel_IsExpired(t *testing.T) {
	now := time.Now()

	// Not expired
	cached := &CachedModel{
		Model:     &Model{ID: "test"},
		CachedAt:  now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	assert.False(t, cached.IsExpired())

	// Expired
	cached = &CachedModel{
		Model:     &Model{ID: "test"},
		CachedAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	assert.True(t, cached.IsExpired())
}

func TestCachedProvider_IsExpired(t *testing.T) {
	now := time.Now()

	// Not expired
	cached := &CachedProvider{
		Provider:  &Provider{ID: "test"},
		CachedAt:  now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	assert.False(t, cached.IsExpired())

	// Expired
	cached = &CachedProvider{
		Provider:  &Provider{ID: "test"},
		CachedAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	assert.True(t, cached.IsExpired())
}

func TestAppendIfMissing(t *testing.T) {
	slice := []string{"a", "b", "c"}

	// Append new item
	result := appendIfMissing(slice, "d")
	assert.Len(t, result, 4)
	assert.Contains(t, result, "d")

	// Don't append existing item
	result = appendIfMissing(slice, "b")
	assert.Len(t, result, 3)
}

func TestRemoveString(t *testing.T) {
	slice := []string{"a", "b", "c"}

	// Remove existing item
	result := removeString(slice, "b")
	assert.Len(t, result, 2)
	assert.NotContains(t, result, "b")

	// Remove non-existing item
	result = removeString(slice, "d")
	assert.Len(t, result, 3)
}

// Benchmarks

func BenchmarkNewCache(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewCache(nil)
		_ = c.Close()
	}
}

func BenchmarkCacheSetModel(b *testing.B) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       10000,
	})
	defer func() { _ = cache.Close() }()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.SetModel(ctx, &Model{
			ID:       fmt.Sprintf("model-%d", i),
			Name:     fmt.Sprintf("Model %d", i),
			Provider: "test-provider",
		})
	}
}

func BenchmarkCacheGetModel(b *testing.B) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       10000,
	})
	defer func() { _ = cache.Close() }()
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		cache.SetModel(ctx, &Model{
			ID:   fmt.Sprintf("model-%d", i),
			Name: fmt.Sprintf("Model %d", i),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetModel(ctx, fmt.Sprintf("model-%d", i%100))
	}
}

func BenchmarkCacheStats(b *testing.B) {
	cache := NewCache(&CacheConfig{
		ModelTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		MaxModels:       100,
	})
	defer func() { _ = cache.Close() }()
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		cache.SetModel(ctx, &Model{
			ID:   fmt.Sprintf("model-%d", i),
			Name: fmt.Sprintf("Model %d", i),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Stats()
	}
}

func BenchmarkAppendIfMissing(b *testing.B) {
	slice := []string{"a", "b", "c", "d", "e"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = appendIfMissing(slice, "f")
	}
}

// Suppress unused import warning
var _ = fmt.Sprintf
