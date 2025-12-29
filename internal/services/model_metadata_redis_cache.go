package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/cache"
	"github.com/superagent/superagent/internal/database"
)

// ModelMetadataRedisCache implements Redis-based caching for model metadata
type ModelMetadataRedisCache struct {
	redisClient *cache.RedisClient
	prefix      string
	ttl         time.Duration
	log         *logrus.Logger
}

// NewModelMetadataRedisCache creates a new Redis cache for model metadata
func NewModelMetadataRedisCache(redisClient *cache.RedisClient, prefix string, ttl time.Duration, log *logrus.Logger) *ModelMetadataRedisCache {
	return &ModelMetadataRedisCache{
		redisClient: redisClient,
		prefix:      prefix,
		ttl:         ttl,
		log:         log,
	}
}

// Get retrieves model metadata from Redis cache
func (c *ModelMetadataRedisCache) Get(ctx context.Context, modelID string) (*database.ModelMetadata, bool, error) {
	key := c.getCacheKey(modelID)
	var metadata database.ModelMetadata

	err := c.redisClient.Get(ctx, key, &metadata)
	if err != nil {
		if err == redis.Nil {
			c.log.WithField("model_id", modelID).Debug("Cache miss in Redis")
			return nil, false, nil
		}
		c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to get from Redis cache")
		return nil, false, err
	}

	c.log.WithField("model_id", modelID).Debug("Cache hit in Redis")
	return &metadata, true, nil
}

// Set stores model metadata in Redis cache
func (c *ModelMetadataRedisCache) Set(ctx context.Context, modelID string, metadata *database.ModelMetadata) error {
	key := c.getCacheKey(modelID)

	err := c.redisClient.Set(ctx, key, metadata, c.ttl)
	if err != nil {
		c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to set in Redis cache")
		return err
	}

	c.log.WithField("model_id", modelID).Debug("Cache set in Redis")
	return nil
}

// Delete removes model metadata from Redis cache
func (c *ModelMetadataRedisCache) Delete(ctx context.Context, modelID string) error {
	key := c.getCacheKey(modelID)

	err := c.redisClient.Delete(ctx, key)
	if err != nil {
		c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to delete from Redis cache")
		return err
	}

	c.log.WithField("model_id", modelID).Debug("Cache deleted from Redis")
	return nil
}

// Clear removes all model metadata from Redis cache (with the prefix)
func (c *ModelMetadataRedisCache) Clear(ctx context.Context) error {
	// Note: This is a simplified implementation
	// In production, you might want to use SCAN or maintain a set of keys
	c.log.Info("Clearing Redis cache is not fully implemented - use Delete for individual models")
	return nil
}

// Size returns the approximate number of cached items using Redis SCAN
func (c *ModelMetadataRedisCache) Size(ctx context.Context) (int, error) {
	var count int
	var cursor uint64
	keysPattern := c.prefix + "*"

	for {
		keys, newCursor, err := c.redisClient.Client().Scan(ctx, cursor, keysPattern, 100).Result()
		if err != nil {
			c.log.WithError(err).Error("Failed to scan Redis keys")
			return 0, fmt.Errorf("failed to scan Redis keys: %w", err)
		}

		count += len(keys)
		cursor = newCursor

		// SCAN is complete when cursor returns to 0
		if cursor == 0 {
			break
		}

		// Safety check to prevent infinite loops
		if count > 10000 {
			c.log.Warn("Size estimation stopped at 10000 keys to prevent performance issues")
			break
		}
	}

	return count, nil
}

// GetBulk retrieves multiple models from cache
func (c *ModelMetadataRedisCache) GetBulk(ctx context.Context, modelIDs []string) (map[string]*database.ModelMetadata, error) {
	if len(modelIDs) == 0 {
		return make(map[string]*database.ModelMetadata), nil
	}

	keys := make([]string, len(modelIDs))
	for i, modelID := range modelIDs {
		keys[i] = c.getCacheKey(modelID)
	}

	// Get from Redis using MGET
	results, err := c.redisClient.MGet(ctx, keys...)
	if err != nil {
		c.log.WithError(err).Warn("Failed to get bulk from Redis cache")
		return nil, err
	}

	cacheHits := make(map[string]*database.ModelMetadata)
	for i, result := range results {
		if result == nil {
			continue
		}

		var metadata database.ModelMetadata
		if err := json.Unmarshal([]byte(result.(string)), &metadata); err != nil {
			c.log.WithError(err).WithField("model_id", modelIDs[i]).Warn("Failed to unmarshal cached model")
			continue
		}

		cacheHits[modelIDs[i]] = &metadata
	}

	c.log.WithField("requested", len(modelIDs)).WithField("found", len(cacheHits)).Debug("Bulk cache operation")
	return cacheHits, nil
}

// SetBulk stores multiple models in cache
func (c *ModelMetadataRedisCache) SetBulk(ctx context.Context, models map[string]*database.ModelMetadata) error {
	if len(models) == 0 {
		return nil
	}

	// Use pipeline for better performance
	pipe := c.redisClient.Pipeline()

	for modelID, metadata := range models {
		key := c.getCacheKey(modelID)
		data, err := json.Marshal(metadata)
		if err != nil {
			c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to marshal model for cache")
			continue
		}

		pipe.Set(ctx, key, data, c.ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		c.log.WithError(err).Warn("Failed to set bulk in Redis cache")
		return err
	}

	c.log.WithField("count", len(models)).Debug("Bulk cache set operation")
	return nil
}

// GetProviderModels retrieves all models for a provider from cache
func (c *ModelMetadataRedisCache) GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error) {
	// Use a separate key pattern for provider models
	providerKey := fmt.Sprintf("%s:provider:%s", c.prefix, providerID)

	var models []*database.ModelMetadata
	err := c.redisClient.Get(ctx, providerKey, &models)
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		c.log.WithError(err).WithField("provider_id", providerID).Warn("Failed to get provider models from cache")
		return nil, err
	}

	c.log.WithField("provider_id", providerID).WithField("count", len(models)).Debug("Provider models cache hit")
	return models, nil
}

// SetProviderModels stores all models for a provider in cache
func (c *ModelMetadataRedisCache) SetProviderModels(ctx context.Context, providerID string, models []*database.ModelMetadata) error {
	providerKey := fmt.Sprintf("%s:provider:%s", c.prefix, providerID)

	err := c.redisClient.Set(ctx, providerKey, models, c.ttl)
	if err != nil {
		c.log.WithError(err).WithField("provider_id", providerID).Warn("Failed to set provider models in cache")
		return err
	}

	c.log.WithField("provider_id", providerID).WithField("count", len(models)).Debug("Provider models cache set")
	return nil
}

// DeleteProviderModels removes provider models from cache
func (c *ModelMetadataRedisCache) DeleteProviderModels(ctx context.Context, providerID string) error {
	providerKey := fmt.Sprintf("%s:provider:%s", c.prefix, providerID)

	err := c.redisClient.Delete(ctx, providerKey)
	if err != nil {
		c.log.WithError(err).WithField("provider_id", providerID).Warn("Failed to delete provider models from cache")
		return err
	}

	c.log.WithField("provider_id", providerID).Debug("Provider models cache deleted")
	return nil
}

// GetByCapability retrieves models with specific capability from cache
func (c *ModelMetadataRedisCache) GetByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error) {
	capabilityKey := fmt.Sprintf("%s:capability:%s", c.prefix, capability)

	var models []*database.ModelMetadata
	err := c.redisClient.Get(ctx, capabilityKey, &models)
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		c.log.WithError(err).WithField("capability", capability).Warn("Failed to get capability models from cache")
		return nil, err
	}

	c.log.WithField("capability", capability).WithField("count", len(models)).Debug("Capability models cache hit")
	return models, nil
}

// SetByCapability stores models with specific capability in cache
func (c *ModelMetadataRedisCache) SetByCapability(ctx context.Context, capability string, models []*database.ModelMetadata) error {
	capabilityKey := fmt.Sprintf("%s:capability:%s", c.prefix, capability)

	err := c.redisClient.Set(ctx, capabilityKey, models, c.ttl)
	if err != nil {
		c.log.WithError(err).WithField("capability", capability).Warn("Failed to set capability models in cache")
		return err
	}

	c.log.WithField("capability", capability).WithField("count", len(models)).Debug("Capability models cache set")
	return nil
}

// HealthCheck checks if Redis cache is healthy
func (c *ModelMetadataRedisCache) HealthCheck(ctx context.Context) error {
	return c.redisClient.Ping(ctx)
}

// getCacheKey generates cache key for a model
func (c *ModelMetadataRedisCache) getCacheKey(modelID string) string {
	return fmt.Sprintf("%s:model:%s", c.prefix, modelID)
}

// MultiLevelCache combines in-memory and Redis caching
type MultiLevelCache struct {
	memoryCache *InMemoryCache
	redisCache  *ModelMetadataRedisCache
	log         *logrus.Logger
}

// NewMultiLevelCache creates a new multi-level cache
func NewMultiLevelCache(memoryCache *InMemoryCache, redisCache *ModelMetadataRedisCache, log *logrus.Logger) *MultiLevelCache {
	return &MultiLevelCache{
		memoryCache: memoryCache,
		redisCache:  redisCache,
		log:         log,
	}
}

// Get tries memory cache first, then Redis cache
func (c *MultiLevelCache) Get(ctx context.Context, modelID string) (*database.ModelMetadata, bool, error) {
	// Try memory cache first
	metadata, exists, err := c.memoryCache.Get(ctx, modelID)
	if err != nil {
		c.log.WithError(err).WithField("model_id", modelID).Warn("Memory cache error")
	} else if exists {
		c.log.WithField("model_id", modelID).Debug("Cache hit in memory")
		return metadata, true, nil
	}

	// Try Redis cache
	metadata, exists, err = c.redisCache.Get(ctx, modelID)
	if err != nil {
		c.log.WithError(err).WithField("model_id", modelID).Warn("Redis cache error, falling back")
		return nil, false, err
	}

	if exists {
		// Update memory cache
		if err := c.memoryCache.Set(ctx, modelID, metadata); err != nil {
			c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to update memory cache")
		}
		c.log.WithField("model_id", modelID).Debug("Cache hit in Redis, updated memory cache")
		return metadata, true, nil
	}

	c.log.WithField("model_id", modelID).Debug("Cache miss at all levels")
	return nil, false, nil
}

// Set stores in both memory and Redis caches
func (c *MultiLevelCache) Set(ctx context.Context, modelID string, metadata *database.ModelMetadata) error {
	// Set in memory cache
	if err := c.memoryCache.Set(ctx, modelID, metadata); err != nil {
		c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to set in memory cache")
	}

	// Set in Redis cache (background)
	go func() {
		ctx := context.Background()
		if err := c.redisCache.Set(ctx, modelID, metadata); err != nil {
			c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to set in Redis cache (background)")
		}
	}()

	c.log.WithField("model_id", modelID).Debug("Cache set in multi-level cache")
	return nil
}

// Delete removes from both memory and Redis caches
func (c *MultiLevelCache) Delete(ctx context.Context, modelID string) error {
	// Delete from memory cache
	if err := c.memoryCache.Delete(ctx, modelID); err != nil {
		c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to delete from memory cache")
	}

	// Delete from Redis cache
	if err := c.redisCache.Delete(ctx, modelID); err != nil {
		c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to delete from Redis cache")
		return err
	}

	c.log.WithField("model_id", modelID).Debug("Cache deleted from multi-level cache")
	return nil
}

// Clear clears both caches
func (c *MultiLevelCache) Clear(ctx context.Context) error {
	// Clear memory cache
	if err := c.memoryCache.Clear(ctx); err != nil {
		c.log.WithError(err).Warn("Failed to clear memory cache")
	}

	// Clear Redis cache
	if err := c.redisCache.Clear(ctx); err != nil {
		c.log.WithError(err).Warn("Failed to clear Redis cache")
		return err
	}

	c.log.Debug("Multi-level cache cleared")
	return nil
}

// Size returns combined size (memory cache only for simplicity)
func (c *MultiLevelCache) Size(ctx context.Context) (int, error) {
	return c.memoryCache.Size(ctx)
}

// GetBulk tries memory cache first, then Redis cache
func (c *MultiLevelCache) GetBulk(ctx context.Context, modelIDs []string) (map[string]*database.ModelMetadata, error) {
	// Try memory cache first
	memoryResults, err := c.memoryCache.GetBulk(ctx, modelIDs)
	if err != nil {
		c.log.WithError(err).Warn("Memory cache bulk get error")
		memoryResults = make(map[string]*database.ModelMetadata)
	}

	// Find missing model IDs
	missingIDs := make([]string, 0)
	for _, modelID := range modelIDs {
		if _, found := memoryResults[modelID]; !found {
			missingIDs = append(missingIDs, modelID)
		}
	}

	if len(missingIDs) == 0 {
		return memoryResults, nil
	}

	// Try Redis cache for missing IDs
	redisResults, err := c.redisCache.GetBulk(ctx, missingIDs)
	if err != nil {
		c.log.WithError(err).Warn("Redis cache bulk get error")
		redisResults = make(map[string]*database.ModelMetadata)
	}

	// Update memory cache with Redis results
	for modelID, metadata := range redisResults {
		if err := c.memoryCache.Set(ctx, modelID, metadata); err != nil {
			c.log.WithError(err).WithField("model_id", modelID).Warn("Failed to update memory cache from bulk")
		}
		memoryResults[modelID] = metadata
	}

	return memoryResults, nil
}

// SetBulk stores in both memory and Redis caches
func (c *MultiLevelCache) SetBulk(ctx context.Context, models map[string]*database.ModelMetadata) error {
	// Set in memory cache
	if err := c.memoryCache.SetBulk(ctx, models); err != nil {
		c.log.WithError(err).Warn("Failed to set bulk in memory cache")
	}

	// Set in Redis cache (background)
	go func() {
		ctx := context.Background()
		if err := c.redisCache.SetBulk(ctx, models); err != nil {
			c.log.WithError(err).Warn("Failed to set bulk in Redis cache (background)")
		}
	}()

	c.log.WithField("count", len(models)).Debug("Bulk cache set in multi-level cache")
	return nil
}

// GetProviderModels retrieves all models for a provider from cache
func (c *MultiLevelCache) GetProviderModels(ctx context.Context, providerID string) ([]*database.ModelMetadata, error) {
	// For provider models, we only use Redis cache
	return c.redisCache.GetProviderModels(ctx, providerID)
}

// SetProviderModels stores provider models in Redis cache only
func (c *MultiLevelCache) SetProviderModels(ctx context.Context, providerID string, models []*database.ModelMetadata) error {
	return c.redisCache.SetProviderModels(ctx, providerID, models)
}

// DeleteProviderModels removes provider models from Redis cache only
func (c *MultiLevelCache) DeleteProviderModels(ctx context.Context, providerID string) error {
	return c.redisCache.DeleteProviderModels(ctx, providerID)
}

// GetByCapability retrieves models with specific capability from Redis cache
func (c *MultiLevelCache) GetByCapability(ctx context.Context, capability string) ([]*database.ModelMetadata, error) {
	return c.redisCache.GetByCapability(ctx, capability)
}

// SetByCapability stores models with specific capability in Redis cache
func (c *MultiLevelCache) SetByCapability(ctx context.Context, capability string, models []*database.ModelMetadata) error {
	return c.redisCache.SetByCapability(ctx, capability, models)
}

// HealthCheck checks if either cache is healthy
func (c *MultiLevelCache) HealthCheck(ctx context.Context) error {
	// Check Redis health
	if err := c.redisCache.HealthCheck(ctx); err != nil {
		c.log.WithError(err).Warn("Redis cache health check failed")
		return err
	}

	// Memory cache is always healthy
	return nil
}
