package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/models"
)

// CacheService provides comprehensive caching functionality.
//
// Concurrent-safe by construction (CONST-029):
//
//   - userKeys is a safe.Store keyed by userID; the stored value is an
//     IMMUTABLE `map[string]struct{}` snapshot of the user's cache
//     keys. Every mutation goes through safe.Store.Update and
//     allocates a brand-new inner map (copy-on-write); readers that
//     called Get receive a frozen snapshot they can iterate safely
//     without any lock. This structurally prevents the "the lock only
//     guarded the outer map, not the pointed-at struct" race class.
//   - The old userKeysMu sync.RWMutex is retired.
type CacheService struct {
	redisClient *RedisClient
	enabled     bool
	defaultTTL  time.Duration

	// userKeys tracks cache keys per user for efficient invalidation.
	// userID -> frozen set of cache keys (inner map is immutable
	// after being stored).
	userKeys *safe.Store[string, map[string]struct{}]
}

// CacheKey represents different types of cache keys
type CacheKey struct {
	Type      string
	ID        string
	Provider  string
	UserID    string
	SessionID string
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	HitCount  int64       `json:"hit_count"`
}

// NewCacheService creates a new cache service instance
func NewCacheService(cfg *config.Config) (*CacheService, error) {
	redisClient := NewRedisClient(cfg)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx); err != nil {
		return &CacheService{
			enabled:    false,
			defaultTTL: 30 * time.Minute,
			userKeys:   safe.NewStore[string, map[string]struct{}](),
		}, fmt.Errorf("Redis connection failed, caching disabled: %w", err)
	}

	return &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}, nil
}

// IsEnabled returns whether caching is enabled
func (c *CacheService) IsEnabled() bool {
	return c.enabled
}

// GetLLMResponse retrieves cached LLM response
func (c *CacheService) GetLLMResponse(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if !c.enabled {
		return nil, fmt.Errorf("caching disabled")
	}

	key := c.generateCacheKey(req)
	var response models.LLMResponse

	err := c.redisClient.Get(ctx, key, &response)
	if err != nil {
		return nil, err
	}

	// Update hit count
	c.incrementHitCount(ctx, key)

	return &response, nil
}

// SetLLMResponse caches an LLM response
func (c *CacheService) SetLLMResponse(ctx context.Context, req *models.LLMRequest, resp *models.LLMResponse, ttl time.Duration) error {
	if !c.enabled {
		return nil
	}

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	key := c.generateCacheKey(req)
	return c.redisClient.Set(ctx, key, resp, ttl)
}

// GetMemorySources retrieves cached memory sources
func (c *CacheService) GetMemorySources(ctx context.Context, query string, dataset string) ([]models.MemorySource, error) {
	if !c.enabled {
		return nil, fmt.Errorf("caching disabled")
	}

	key := fmt.Sprintf("memory:%s:%s", dataset, c.hashString(query))
	var sources []models.MemorySource

	err := c.redisClient.Get(ctx, key, &sources)
	if err != nil {
		return nil, err
	}

	return sources, nil
}

// SetMemorySources caches memory sources
func (c *CacheService) SetMemorySources(ctx context.Context, query string, dataset string, sources []models.MemorySource, ttl time.Duration) error {
	if !c.enabled {
		return nil
	}

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	key := fmt.Sprintf("memory:%s:%s", dataset, c.hashString(query))
	return c.redisClient.Set(ctx, key, sources, ttl)
}

// GetProviderHealth retrieves cached provider health status
func (c *CacheService) GetProviderHealth(ctx context.Context, providerName string) (map[string]interface{}, error) {
	if !c.enabled {
		return nil, fmt.Errorf("caching disabled")
	}

	key := fmt.Sprintf("health:%s", providerName)
	var health map[string]interface{}

	err := c.redisClient.Get(ctx, key, &health)
	if err != nil {
		return nil, err
	}

	return health, nil
}

// SetProviderHealth caches provider health status
func (c *CacheService) SetProviderHealth(ctx context.Context, providerName string, health map[string]interface{}, ttl time.Duration) error {
	if !c.enabled {
		return nil
	}

	if ttl == 0 {
		ttl = 5 * time.Minute // Health data expires faster
	}

	key := fmt.Sprintf("health:%s", providerName)
	return c.redisClient.Set(ctx, key, health, ttl)
}

// GetUserSession retrieves cached user session
func (c *CacheService) GetUserSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
	if !c.enabled {
		return nil, fmt.Errorf("caching disabled")
	}

	key := fmt.Sprintf("session:%s", sessionID)
	var session models.UserSession

	err := c.redisClient.Get(ctx, key, &session)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// SetUserSession caches user session
func (c *CacheService) SetUserSession(ctx context.Context, session *models.UserSession, ttl time.Duration) error {
	if !c.enabled {
		return nil
	}

	if session == nil {
		return nil
	}

	if ttl == 0 {
		ttl = 24 * time.Hour // Session data
	}

	key := fmt.Sprintf("session:%s", session.SessionToken)

	// Track this key for user-based invalidation
	if session.UserID != "" {
		c.trackUserKey(session.UserID, key)
	}

	return c.redisClient.Set(ctx, key, session, ttl)
}

// GetAPIKey retrieves cached API key information
func (c *CacheService) GetAPIKey(ctx context.Context, apiKey string) (map[string]interface{}, error) {
	if !c.enabled {
		return nil, fmt.Errorf("caching disabled")
	}

	key := fmt.Sprintf("apikey:%s", c.hashString(apiKey))
	var keyInfo map[string]interface{}

	err := c.redisClient.Get(ctx, key, &keyInfo)
	if err != nil {
		return nil, err
	}

	return keyInfo, nil
}

// SetAPIKey caches API key information
func (c *CacheService) SetAPIKey(ctx context.Context, apiKey string, keyInfo map[string]interface{}, ttl time.Duration) error {
	if !c.enabled {
		return nil
	}

	if ttl == 0 {
		ttl = 1 * time.Hour // API key cache
	}

	key := fmt.Sprintf("apikey:%s", c.hashString(apiKey))
	return c.redisClient.Set(ctx, key, keyInfo, ttl)
}

// InvalidateUserCache invalidates all cache entries for a user
func (c *CacheService) InvalidateUserCache(ctx context.Context, userID string) error {
	if !c.enabled {
		return nil
	}

	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	// Atomically take-and-clear the user's tracked key set.
	// Store.Delete returns the prior value and whether it was present;
	// no other writer can see the half-emptied state because Delete
	// holds the store's write lock for the whole take-and-clear.
	userKeySet, _ := c.userKeys.Delete(userID)

	// Delete all tracked keys from Redis
	if len(userKeySet) > 0 {
		for key := range userKeySet {
			if err := c.redisClient.Delete(ctx, key); err != nil {
				// Log error but continue deleting other keys
				continue
			}
		}
	}

	// Also use Redis SCAN to find and delete any user-prefixed keys
	// This catches keys that may have been set without tracking
	pattern := fmt.Sprintf("user:%s:*", userID)
	if err := c.deleteByPattern(ctx, pattern); err != nil {
		return fmt.Errorf("failed to delete user keys by pattern: %w", err)
	}

	return nil
}

// deleteByPattern uses Redis SCAN to find and delete keys matching a pattern
func (c *CacheService) deleteByPattern(ctx context.Context, pattern string) error {
	if c.redisClient == nil || c.redisClient.client == nil {
		return nil
	}

	var cursor uint64
	var keysToDelete []string

	// Use SCAN to iterate through keys matching the pattern
	for {
		keys, nextCursor, err := c.redisClient.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		keysToDelete = append(keysToDelete, keys...)
		cursor = nextCursor

		if cursor == 0 {
			break
		}
	}

	// Delete all found keys
	if len(keysToDelete) > 0 {
		if err := c.redisClient.client.Del(ctx, keysToDelete...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
	}

	return nil
}

// trackUserKey associates a cache key with a user for later invalidation.
// The inner set is treated as immutable; each mutation allocates a new
// map via copy-on-write inside the Store.Update callback, so a reader
// that already called Get holds a snapshot that cannot be mutated
// underneath them.
func (c *CacheService) trackUserKey(userID, cacheKey string) {
	if userID == "" || cacheKey == "" {
		return
	}
	c.userKeys.Update(userID, func(cur map[string]struct{}, _ bool) (map[string]struct{}, bool) {
		if _, already := cur[cacheKey]; already {
			return cur, true
		}
		next := make(map[string]struct{}, len(cur)+1)
		for k := range cur {
			next[k] = struct{}{}
		}
		next[cacheKey] = struct{}{}
		return next, true
	})
}

// untrackUserKey removes a cache key from user tracking. If the removal
// empties the set the user entry is deleted from the outer store.
func (c *CacheService) untrackUserKey(userID, cacheKey string) {
	if userID == "" || cacheKey == "" {
		return
	}
	c.userKeys.Update(userID, func(cur map[string]struct{}, present bool) (map[string]struct{}, bool) {
		if !present {
			return nil, false
		}
		if _, ok := cur[cacheKey]; !ok {
			return cur, true
		}
		if len(cur) == 1 {
			return nil, false // drop the whole user entry
		}
		next := make(map[string]struct{}, len(cur)-1)
		for k := range cur {
			if k != cacheKey {
				next[k] = struct{}{}
			}
		}
		return next, true
	})
}

// GetUserKeyCount returns the number of cached keys for a user (for testing/monitoring).
// Reads a frozen inner-map snapshot; len is safe without holding any lock.
func (c *CacheService) GetUserKeyCount(userID string) int {
	set, _ := c.userKeys.Get(userID)
	return len(set)
}

// SetUserData caches user-specific data with tracking for invalidation
func (c *CacheService) SetUserData(ctx context.Context, userID string, dataKey string, value interface{}, ttl time.Duration) error {
	if !c.enabled {
		return nil
	}

	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	// Create a user-prefixed key for easy pattern matching
	key := fmt.Sprintf("user:%s:%s", userID, dataKey)

	// Track this key for user-based invalidation
	c.trackUserKey(userID, key)

	return c.redisClient.Set(ctx, key, value, ttl)
}

// GetUserData retrieves user-specific cached data
func (c *CacheService) GetUserData(ctx context.Context, userID string, dataKey string, dest interface{}) error {
	if !c.enabled {
		return fmt.Errorf("caching disabled")
	}

	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	key := fmt.Sprintf("user:%s:%s", userID, dataKey)
	return c.redisClient.Get(ctx, key, dest)
}

// DeleteUserData deletes a specific user data entry
func (c *CacheService) DeleteUserData(ctx context.Context, userID string, dataKey string) error {
	if !c.enabled {
		return nil
	}

	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	key := fmt.Sprintf("user:%s:%s", userID, dataKey)

	// Remove from tracking
	c.untrackUserKey(userID, key)

	return c.redisClient.Delete(ctx, key)
}

// ClearExpired removes expired cache entries
func (c *CacheService) ClearExpired(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	// Redis handles expiration automatically
	// This method could be used for custom cleanup logic
	return nil
}

// GetStats returns cache statistics
func (c *CacheService) GetStats(ctx context.Context) map[string]interface{} {
	if !c.enabled {
		return map[string]interface{}{
			"enabled": false,
			"status":  "disabled",
		}
	}

	// Get Redis info
	info := c.redisClient.client.Info(ctx, "memory").Val()

	return map[string]interface{}{
		"enabled":     true,
		"status":      "connected",
		"default_ttl": c.defaultTTL.String(),
		"redis_info":  info,
	}
}

// generateCacheKey generates a cache key for LLM requests
func (c *CacheService) generateCacheKey(req *models.LLMRequest) string {
	// Create a deterministic key based on request content
	keyData := map[string]interface{}{
		"prompt":      req.Prompt,
		"messages":    req.Messages,
		"model":       req.ModelParams.Model,
		"temperature": req.ModelParams.Temperature,
		"max_tokens":  req.ModelParams.MaxTokens,
		"top_p":       req.ModelParams.TopP,
		"stop":        req.ModelParams.StopSequences,
	}

	// Convert to JSON and hash
	jsonData, err := json.Marshal(keyData)
	if err != nil {
		// Fallback to hash of prompt and model
		fallback := fmt.Sprintf("%s:%s", req.Prompt, req.ModelParams.Model)
		hash := c.hashString(fallback)
		return fmt.Sprintf("llm:%s", hash)
	}
	hash := c.hashString(string(jsonData))

	return fmt.Sprintf("llm:%s", hash)
}

// hashString creates a FNV hash of a string for cache key generation
func (c *CacheService) hashString(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%016x", h.Sum64())
}

// incrementHitCount increments the hit count for a cache key
func (c *CacheService) incrementHitCount(ctx context.Context, key string) {
	hitKey := fmt.Sprintf("hits:%s", key)
	c.redisClient.client.Incr(ctx, hitKey)
}

// GetHitCount returns the hit count for a cache key
func (c *CacheService) GetHitCount(ctx context.Context, key string) (int64, error) {
	if !c.enabled {
		return 0, fmt.Errorf("caching disabled")
	}

	hitKey := fmt.Sprintf("hits:%s", key)
	count, err := c.redisClient.client.Get(ctx, hitKey).Int64()
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Close closes the cache service
func (c *CacheService) Close() error {
	if c.redisClient != nil {
		return c.redisClient.Close()
	}
	return nil
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	Enabled    bool           `json:"enabled"`
	DefaultTTL time.Duration  `json:"default_ttl"`
	Redis      *config.Config `json:"redis"`
}

// NewCacheConfig creates default cache configuration
func NewCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:    true,
		DefaultTTL: 30 * time.Minute,
	}
}
