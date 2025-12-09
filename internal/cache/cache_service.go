package cache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
)

// CacheService provides comprehensive caching functionality
type CacheService struct {
	redisClient *RedisClient
	enabled     bool
	defaultTTL  time.Duration
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
		}, fmt.Errorf("Redis connection failed, caching disabled: %w", err)
	}

	return &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
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

	if ttl == 0 {
		ttl = 24 * time.Hour // Session data
	}

	key := fmt.Sprintf("session:%s", session.SessionToken)
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

	// This would require maintaining a set of keys per user
	// For now, we'll use a simple pattern-based deletion
	// Redis doesn't support pattern deletion directly with go-redis
	// This would require SCAN or maintaining separate key sets
	// For now, we'll skip this implementation
	_ = userID // Mark as used to avoid linting error

	return nil
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
	jsonData, _ := json.Marshal(keyData)
	hash := c.hashString(string(jsonData))

	return fmt.Sprintf("llm:%s", hash)
}

// hashString creates an MD5 hash of a string
func (c *CacheService) hashString(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
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
