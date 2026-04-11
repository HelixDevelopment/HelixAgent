package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/models"
)

// ProviderCacheConfig holds configuration for provider response caching
type ProviderCacheConfig struct {
	// DefaultTTL for cached responses
	DefaultTTL time.Duration
	// MaxResponseSize to cache (bytes)
	MaxResponseSize int
	// EnableSemanticCache enables semantic similarity caching
	EnableSemanticCache bool
	// SimilarityThreshold for semantic matching (0.0-1.0)
	SimilarityThreshold float64
	// TTLByProvider allows different TTLs per provider
	TTLByProvider map[string]time.Duration
}

// DefaultProviderCacheConfig returns sensible defaults
func DefaultProviderCacheConfig() *ProviderCacheConfig {
	return &ProviderCacheConfig{
		DefaultTTL:          30 * time.Minute,
		MaxResponseSize:     1024 * 1024, // 1MB
		EnableSemanticCache: false,       // Requires embedding service
		SimilarityThreshold: 0.95,
		TTLByProvider: map[string]time.Duration{
			"claude":   1 * time.Hour,
			"deepseek": 1 * time.Hour,
			"gemini":   1 * time.Hour,
			"ollama":   15 * time.Minute,
		},
	}
}

// ProviderCache caches LLM provider responses
type ProviderCache struct {
	cache   *TieredCache
	tagInv  *TagBasedInvalidation
	config  *ProviderCacheConfig
	metrics *ProviderCacheMetrics
	mu      sync.RWMutex
}

// ProviderCacheMetrics tracks provider cache statistics
type ProviderCacheMetrics struct {
	Hits          int64
	Misses        int64
	Sets          int64
	Invalidations int64
	SkippedLarge  int64
	ByProvider    map[string]*providerStats
}

type providerStats struct {
	Hits   int64
	Misses int64
	Sets   int64
}

// NewProviderCache creates a new provider response cache
func NewProviderCache(cache *TieredCache, config *ProviderCacheConfig) *ProviderCache {
	if config == nil {
		config = DefaultProviderCacheConfig()
	}

	return &ProviderCache{
		cache:  cache,
		tagInv: NewTagBasedInvalidation(),
		config: config,
		metrics: &ProviderCacheMetrics{
			ByProvider: make(map[string]*providerStats),
		},
	}
}

// CacheKey generates a deterministic cache key from a request
func (c *ProviderCache) CacheKey(req *models.LLMRequest, provider string) string {
	// Build deterministic key components
	keyData := struct {
		Prompt      string           `json:"prompt"`
		Messages    []models.Message `json:"messages"`
		Model       string           `json:"model"`
		Temperature float64          `json:"temperature"`
		MaxTokens   int              `json:"max_tokens"`
		TopP        float64          `json:"top_p"`
		Provider    string           `json:"provider"`
	}{
		Prompt:      req.Prompt,
		Messages:    req.Messages,
		Model:       req.ModelParams.Model,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   req.ModelParams.MaxTokens,
		TopP:        req.ModelParams.TopP,
		Provider:    provider,
	}

	data, err := json.Marshal(keyData)
	if err != nil {
		// Fallback to simple key
		return "provider:" + provider + ":error"
	}
	hash := sha256.Sum256(data)
	return "provider:" + provider + ":" + hex.EncodeToString(hash[:16])
}

// Get retrieves a cached response
func (c *ProviderCache) Get(ctx context.Context, req *models.LLMRequest, provider string) (*models.LLMResponse, bool) {
	key := c.CacheKey(req, provider)

	var resp models.LLMResponse
	found, err := c.cache.Get(ctx, key, &resp)
	if err != nil || !found {
		atomic.AddInt64(&c.metrics.Misses, 1)
		c.trackProviderMiss(provider)
		return nil, false
	}

	atomic.AddInt64(&c.metrics.Hits, 1)
	c.trackProviderHit(provider)
	return &resp, true
}

// Set caches a response
func (c *ProviderCache) Set(ctx context.Context, req *models.LLMRequest, resp *models.LLMResponse, provider string) error {
	// Check response size
	respData, err := json.Marshal(resp)
	if err != nil {
		// Cannot marshal response, skip caching
		atomic.AddInt64(&c.metrics.SkippedLarge, 1)
		return nil
	}
	if len(respData) > c.config.MaxResponseSize {
		atomic.AddInt64(&c.metrics.SkippedLarge, 1)
		return nil
	}

	key := c.CacheKey(req, provider)
	ttl := c.getTTL(provider)

	// Add tags for invalidation
	tags := []string{
		"provider:" + provider,
		"llm-response",
	}
	if req.ModelParams.Model != "" {
		tags = append(tags, "model:"+req.ModelParams.Model)
	}

	if err := c.cache.Set(ctx, key, resp, ttl, tags...); err != nil {
		return err
	}

	// Track in tag invalidation
	c.tagInv.AddTag(key, tags...)

	atomic.AddInt64(&c.metrics.Sets, 1)
	c.trackProviderSet(provider)
	return nil
}

// InvalidateProvider removes all cached entries for a provider
func (c *ProviderCache) InvalidateProvider(ctx context.Context, provider string) (int, error) {
	keys := c.tagInv.InvalidateByTag("provider:" + provider)

	for _, key := range keys {
		_ = c.cache.Delete(ctx, key) //nolint:errcheck
		c.tagInv.RemoveKey(key)
	}

	// Also invalidate by prefix
	count, err := c.cache.InvalidatePrefix(ctx, "provider:"+provider+":")
	if err != nil {
		return len(keys), err
	}

	atomic.AddInt64(&c.metrics.Invalidations, int64(len(keys)+count))
	return len(keys) + count, nil
}

// InvalidateModel removes all cached entries for a model
func (c *ProviderCache) InvalidateModel(ctx context.Context, model string) (int, error) {
	keys := c.tagInv.InvalidateByTag("model:" + model)

	for _, key := range keys {
		_ = c.cache.Delete(ctx, key) //nolint:errcheck
		c.tagInv.RemoveKey(key)
	}

	atomic.AddInt64(&c.metrics.Invalidations, int64(len(keys)))
	return len(keys), nil
}

// InvalidateAll removes all cached provider responses
func (c *ProviderCache) InvalidateAll(ctx context.Context) (int, error) {
	keys := c.tagInv.InvalidateByTag("llm-response")

	for _, key := range keys {
		_ = c.cache.Delete(ctx, key) //nolint:errcheck
		c.tagInv.RemoveKey(key)
	}

	// Also invalidate by prefix
	count, err := c.cache.InvalidatePrefix(ctx, "provider:")
	if err != nil {
		return len(keys), err
	}

	atomic.AddInt64(&c.metrics.Invalidations, int64(len(keys)+count))
	return len(keys) + count, nil
}

func (c *ProviderCache) getTTL(provider string) time.Duration {
	if ttl, ok := c.config.TTLByProvider[provider]; ok {
		return ttl
	}
	return c.config.DefaultTTL
}

// getOrCreateStats looks up (or lazily creates) the per-provider stats bucket.
// Uses defer on the write lock so a panic between the map lookup and the
// return cannot leak the mutex. The returned *providerStats pointer is stable
// for the lifetime of the cache, so callers may safely atomic-add after the
// lock is released.
func (c *ProviderCache) getOrCreateStats(provider string) *providerStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metrics.ByProvider[provider] == nil {
		c.metrics.ByProvider[provider] = &providerStats{}
	}
	return c.metrics.ByProvider[provider]
}

func (c *ProviderCache) trackProviderHit(provider string) {
	atomic.AddInt64(&c.getOrCreateStats(provider).Hits, 1)
}

func (c *ProviderCache) trackProviderMiss(provider string) {
	atomic.AddInt64(&c.getOrCreateStats(provider).Misses, 1)
}

func (c *ProviderCache) trackProviderSet(provider string) {
	atomic.AddInt64(&c.getOrCreateStats(provider).Sets, 1)
}

// Metrics returns current metrics
func (c *ProviderCache) Metrics() *ProviderCacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := &ProviderCacheMetrics{
		Hits:          atomic.LoadInt64(&c.metrics.Hits),
		Misses:        atomic.LoadInt64(&c.metrics.Misses),
		Sets:          atomic.LoadInt64(&c.metrics.Sets),
		Invalidations: atomic.LoadInt64(&c.metrics.Invalidations),
		SkippedLarge:  atomic.LoadInt64(&c.metrics.SkippedLarge),
		ByProvider:    make(map[string]*providerStats),
	}

	for provider, stats := range c.metrics.ByProvider {
		metrics.ByProvider[provider] = &providerStats{
			Hits:   atomic.LoadInt64(&stats.Hits),
			Misses: atomic.LoadInt64(&stats.Misses),
			Sets:   atomic.LoadInt64(&stats.Sets),
		}
	}

	return metrics
}

// HitRate returns the overall hit rate
func (c *ProviderCache) HitRate() float64 {
	hits := atomic.LoadInt64(&c.metrics.Hits)
	misses := atomic.LoadInt64(&c.metrics.Misses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// ProviderHitRate returns the hit rate for a specific provider
func (c *ProviderCache) ProviderHitRate(provider string) float64 {
	c.mu.RLock()
	stats := c.metrics.ByProvider[provider]
	c.mu.RUnlock()
	if stats == nil {
		return 0
	}

	hits := atomic.LoadInt64(&stats.Hits)
	misses := atomic.LoadInt64(&stats.Misses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}
