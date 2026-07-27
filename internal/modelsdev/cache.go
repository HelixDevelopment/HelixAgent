package modelsdev

import (
	"context"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// Cache provides in-memory caching for Models.dev data.
//
// Concurrent-safe by construction (CONST-029): the (models, providers,
// modelsByProvider) triple lives under a sentinel key in a single
// safe.Store so SetModel / InvalidateProvider / cleanup can mutate the
// three maps atomically inside one Update callback (Pattern Epsilon —
// joint atomicity via state struct). lastRefresh and the hit/miss
// counters are atomics; the cleanup goroutine is gated by stopCleanup.
type Cache struct {
	state       *safe.Store[string, *cacheState]
	config      CacheConfig
	hits        atomic.Int64
	misses      atomic.Int64
	lastRefresh atomic.Pointer[time.Time]
	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

// cacheState is the joint mutable state held under cacheStateKey.
type cacheState struct {
	models           map[string]*CachedModel
	providers        map[string]*CachedProvider
	modelsByProvider map[string][]string // provider ID -> model IDs
}

const cacheStateKey = "_"

func newCacheState() *cacheState {
	return &cacheState{
		models:           make(map[string]*CachedModel),
		providers:        make(map[string]*CachedProvider),
		modelsByProvider: make(map[string][]string),
	}
}

// NewCache creates a new cache instance
func NewCache(config *CacheConfig) *Cache {
	if config == nil {
		defaultConfig := DefaultCacheConfig()
		config = &defaultConfig
	}

	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 10 * time.Minute
	}

	store := safe.NewStore[string, *cacheState]()
	store.Put(cacheStateKey, newCacheState())

	c := &Cache{
		state:       store,
		config:      *config,
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}

	go c.cleanupLoop()
	return c
}

// withState runs fn under the state Store's write lock; fn may mutate the maps.
func (c *Cache) withState(fn func(*cacheState)) {
	c.state.Update(cacheStateKey, func(s *cacheState, _ bool) (*cacheState, bool) {
		if s == nil {
			s = newCacheState()
		}
		fn(s)
		return s, true
	})
}

// GetModel retrieves a model from cache
func (c *Cache) GetModel(ctx context.Context, modelID string) (*Model, bool) {
	var (
		result    *Model
		found     bool
		expiredID string
	)
	c.withState(func(s *cacheState) {
		cached, exists := s.models[modelID]
		if !exists {
			return
		}
		if cached.IsExpired() {
			expiredID = modelID
			return
		}
		atomic.AddInt64(&cached.HitCount, 1)
		result = cached.Model
		found = true
	})

	if found {
		c.hits.Add(1)
		return result, true
	}
	c.misses.Add(1)
	if expiredID != "" {
		go c.removeExpiredModel(expiredID)
	}
	return nil, false
}

// SetModel stores a model in cache
func (c *Cache) SetModel(ctx context.Context, model *Model) {
	if model == nil || model.ID == "" {
		return
	}

	c.withState(func(s *cacheState) {
		if len(s.models) >= c.config.MaxModels {
			c.evictOldestModelsLocked(s, len(s.models)-c.config.MaxModels+1)
		}

		now := time.Now()
		s.models[model.ID] = &CachedModel{
			Model:     model,
			CachedAt:  now,
			ExpiresAt: now.Add(c.config.ModelTTL),
			HitCount:  0,
		}

		if model.Provider != "" {
			s.modelsByProvider[model.Provider] = appendIfMissing(s.modelsByProvider[model.Provider], model.ID)
		}
	})
}

// SetModels stores multiple models in cache
func (c *Cache) SetModels(ctx context.Context, models []Model) {
	if len(models) == 0 {
		return
	}

	c.withState(func(s *cacheState) {
		now := time.Now()
		expiresAt := now.Add(c.config.ModelTTL)

		for i := range models {
			model := &models[i]
			if model.ID == "" {
				continue
			}
			if len(s.models) >= c.config.MaxModels {
				c.evictOldestModelsLocked(s, 1)
			}
			s.models[model.ID] = &CachedModel{
				Model:     model,
				CachedAt:  now,
				ExpiresAt: expiresAt,
				HitCount:  0,
			}
			if model.Provider != "" {
				s.modelsByProvider[model.Provider] = appendIfMissing(s.modelsByProvider[model.Provider], model.ID)
			}
		}
	})
}

// GetProvider retrieves a provider from cache
func (c *Cache) GetProvider(ctx context.Context, providerID string) (*Provider, bool) {
	var (
		result    *Provider
		found     bool
		expiredID string
	)
	c.withState(func(s *cacheState) {
		cached, exists := s.providers[providerID]
		if !exists {
			return
		}
		if cached.IsExpired() {
			expiredID = providerID
			return
		}
		atomic.AddInt64(&cached.HitCount, 1)
		result = cached.Provider
		found = true
	})

	if found {
		c.hits.Add(1)
		return result, true
	}
	c.misses.Add(1)
	if expiredID != "" {
		go c.removeExpiredProvider(expiredID)
	}
	return nil, false
}

// SetProvider stores a provider in cache
func (c *Cache) SetProvider(ctx context.Context, provider *Provider) {
	if provider == nil || provider.ID == "" {
		return
	}

	c.withState(func(s *cacheState) {
		if len(s.providers) >= c.config.MaxProviders {
			c.evictOldestProvidersLocked(s, len(s.providers)-c.config.MaxProviders+1)
		}
		now := time.Now()
		s.providers[provider.ID] = &CachedProvider{
			Provider:  provider,
			CachedAt:  now,
			ExpiresAt: now.Add(c.config.ProviderTTL),
			HitCount:  0,
		}
	})
}

// SetProviders stores multiple providers in cache
func (c *Cache) SetProviders(ctx context.Context, providers []Provider) {
	if len(providers) == 0 {
		return
	}

	c.withState(func(s *cacheState) {
		now := time.Now()
		expiresAt := now.Add(c.config.ProviderTTL)

		for i := range providers {
			provider := &providers[i]
			if provider.ID == "" {
				continue
			}
			if len(s.providers) >= c.config.MaxProviders {
				c.evictOldestProvidersLocked(s, 1)
			}
			s.providers[provider.ID] = &CachedProvider{
				Provider:  provider,
				CachedAt:  now,
				ExpiresAt: expiresAt,
				HitCount:  0,
			}
		}
	})
}

// GetModelsByProvider returns all cached models for a provider
func (c *Cache) GetModelsByProvider(ctx context.Context, providerID string) ([]*Model, bool) {
	var models []*Model
	c.withState(func(s *cacheState) {
		modelIDs, exists := s.modelsByProvider[providerID]
		if !exists || len(modelIDs) == 0 {
			return
		}
		models = make([]*Model, 0, len(modelIDs))
		for _, modelID := range modelIDs {
			if cached, ok := s.models[modelID]; ok && !cached.IsExpired() {
				models = append(models, cached.Model)
			}
		}
	})
	if len(models) == 0 {
		return nil, false
	}
	return models, true
}

// GetAllModels returns all cached models
func (c *Cache) GetAllModels(ctx context.Context) []*Model {
	var models []*Model
	c.withState(func(s *cacheState) {
		models = make([]*Model, 0, len(s.models))
		for _, cached := range s.models {
			if !cached.IsExpired() {
				models = append(models, cached.Model)
			}
		}
	})
	return models
}

// GetAllProviders returns all cached providers
func (c *Cache) GetAllProviders(ctx context.Context) []*Provider {
	var providers []*Provider
	c.withState(func(s *cacheState) {
		providers = make([]*Provider, 0, len(s.providers))
		for _, cached := range s.providers {
			if !cached.IsExpired() {
				providers = append(providers, cached.Provider)
			}
		}
	})
	return providers
}

// InvalidateModel removes a model from cache
func (c *Cache) InvalidateModel(ctx context.Context, modelID string) {
	c.withState(func(s *cacheState) {
		if cached, exists := s.models[modelID]; exists {
			if cached.Model.Provider != "" {
				s.modelsByProvider[cached.Model.Provider] = removeString(s.modelsByProvider[cached.Model.Provider], modelID)
			}
			delete(s.models, modelID)
		}
	})
}

// InvalidateProvider removes a provider and its models from cache
func (c *Cache) InvalidateProvider(ctx context.Context, providerID string) {
	c.withState(func(s *cacheState) {
		delete(s.providers, providerID)
		if modelIDs, exists := s.modelsByProvider[providerID]; exists {
			for _, modelID := range modelIDs {
				delete(s.models, modelID)
			}
			delete(s.modelsByProvider, providerID)
		}
	})
}

// InvalidateAll clears the entire cache
func (c *Cache) InvalidateAll(ctx context.Context) {
	c.state.Put(cacheStateKey, newCacheState())
}

// UpdateLastRefresh updates the last refresh timestamp
func (c *Cache) UpdateLastRefresh() {
	now := time.Now()
	c.lastRefresh.Store(&now)
}

// Stats returns current cache statistics
func (c *Cache) Stats() CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()

	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	var (
		modelCount, providerCount int
		oldestEntry               time.Time
		memoryUsage               int64
	)
	c.withState(func(s *cacheState) {
		modelCount = len(s.models)
		providerCount = len(s.providers)
		for _, cached := range s.models {
			if oldestEntry.IsZero() || cached.CachedAt.Before(oldestEntry) {
				oldestEntry = cached.CachedAt
			}
		}
		for _, cached := range s.providers {
			if oldestEntry.IsZero() || cached.CachedAt.Before(oldestEntry) {
				oldestEntry = cached.CachedAt
			}
		}
		memoryUsage = int64(modelCount*500 + providerCount*200)
	})

	var lastRefresh time.Time
	if t := c.lastRefresh.Load(); t != nil {
		lastRefresh = *t
	}

	return CacheStats{
		ModelCount:       modelCount,
		ProviderCount:    providerCount,
		TotalHits:        hits,
		TotalMisses:      misses,
		HitRate:          hitRate,
		LastRefresh:      lastRefresh,
		OldestEntry:      oldestEntry,
		MemoryUsageBytes: memoryUsage,
	}
}

// Close stops the cache cleanup goroutine
func (c *Cache) Close() error {
	close(c.stopCleanup)
	<-c.cleanupDone
	return nil
}

// Internal methods

func (c *Cache) cleanupLoop() {
	defer close(c.cleanupDone)

	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCleanup:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

func (c *Cache) cleanup() {
	now := time.Now()

	c.withState(func(s *cacheState) {
		for modelID, cached := range s.models {
			if now.After(cached.ExpiresAt) {
				if cached.Model.Provider != "" {
					s.modelsByProvider[cached.Model.Provider] = removeString(s.modelsByProvider[cached.Model.Provider], modelID)
				}
				delete(s.models, modelID)
			}
		}
		for providerID, cached := range s.providers {
			if now.After(cached.ExpiresAt) {
				delete(s.providers, providerID)
			}
		}
		for providerID, modelIDs := range s.modelsByProvider {
			if len(modelIDs) == 0 {
				delete(s.modelsByProvider, providerID)
			}
		}
	})
}

func (c *Cache) removeExpiredModel(modelID string) {
	c.withState(func(s *cacheState) {
		if cached, exists := s.models[modelID]; exists && cached.IsExpired() {
			if cached.Model.Provider != "" {
				s.modelsByProvider[cached.Model.Provider] = removeString(s.modelsByProvider[cached.Model.Provider], modelID)
			}
			delete(s.models, modelID)
		}
	})
}

func (c *Cache) removeExpiredProvider(providerID string) {
	c.withState(func(s *cacheState) {
		if cached, exists := s.providers[providerID]; exists && cached.IsExpired() {
			delete(s.providers, providerID)
		}
	})
}

// evictOldestModelsLocked must be called inside a withState callback.
func (c *Cache) evictOldestModelsLocked(s *cacheState, count int) {
	type modelEntry struct {
		id       string
		cachedAt time.Time
	}

	entries := make([]modelEntry, 0, len(s.models))
	for id, cached := range s.models {
		entries = append(entries, modelEntry{id: id, cachedAt: cached.CachedAt})
	}

	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].cachedAt.Before(entries[i].cachedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	for i := 0; i < count && i < len(entries); i++ {
		modelID := entries[i].id
		if cached, exists := s.models[modelID]; exists {
			if cached.Model.Provider != "" {
				s.modelsByProvider[cached.Model.Provider] = removeString(s.modelsByProvider[cached.Model.Provider], modelID)
			}
			delete(s.models, modelID)
		}
	}
}

// evictOldestProvidersLocked must be called inside a withState callback.
func (c *Cache) evictOldestProvidersLocked(s *cacheState, count int) {
	type providerEntry struct {
		id       string
		cachedAt time.Time
	}

	entries := make([]providerEntry, 0, len(s.providers))
	for id, cached := range s.providers {
		entries = append(entries, providerEntry{id: id, cachedAt: cached.CachedAt})
	}

	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].cachedAt.Before(entries[i].cachedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	for i := 0; i < count && i < len(entries); i++ {
		delete(s.providers, entries[i].id)
	}
}

// Helper functions

func appendIfMissing(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
