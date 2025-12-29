package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ProtocolCache provides advanced caching for protocol operations
type ProtocolCache struct {
	mu           sync.RWMutex
	cache        map[string]*CacheEntry
	invalidators map[string][]CacheInvalidator
	maxSize      int
	ttl          time.Duration
	log          *logrus.Logger
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Key        string
	Data       interface{}
	Tags       []string
	CreatedAt  time.Time
	AccessedAt time.Time
	TTL        time.Duration
	Hits       int
	Size       int
}

// CacheInvalidator defines invalidation rules
type CacheInvalidator struct {
	Pattern string
	Tags    []string
	TTL     time.Duration
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries int
	TotalSize    int
	HitRate      float64
	MissRate     float64
	Evictions    int
	TotalHits    int
	TotalMisses  int
}

// NewProtocolCache creates a new protocol-aware cache
func NewProtocolCache(maxSize int, ttl time.Duration, logger *logrus.Logger) *ProtocolCache {
	cache := &ProtocolCache{
		cache:        make(map[string]*CacheEntry),
		invalidators: make(map[string][]CacheInvalidator),
		maxSize:      maxSize,
		ttl:          ttl,
		log:          logger,
	}

	// Start cleanup goroutine
	go cache.cleanupRoutine()

	return cache
}

// Get retrieves an item from cache
func (c *ProtocolCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, false, nil
	}

	// Check TTL
	if time.Since(entry.CreatedAt) > entry.TTL {
		go c.evict(key) // Async eviction
		return nil, false, nil
	}

	// Update access time and hit count
	entry.AccessedAt = time.Now()
	entry.Hits++

	c.log.WithFields(logrus.Fields{
		"key":  key,
		"hits": entry.Hits,
		"size": entry.Size,
	}).Debug("Cache hit")

	return entry.Data, true, nil
}

// Set stores an item in cache with tags
func (c *ProtocolCache) Set(ctx context.Context, key string, data interface{}, tags []string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate data size
	size := c.calculateSize(data)

	// Check if we need to evict entries
	for len(c.cache) >= c.maxSize {
		c.evictLRU()
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.ttl
	}

	entry := &CacheEntry{
		Key:        key,
		Data:       data,
		Tags:       tags,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        ttl,
		Hits:       0,
		Size:       size,
	}

	c.cache[key] = entry

	c.log.WithFields(logrus.Fields{
		"key":  key,
		"tags": tags,
		"size": size,
		"ttl":  ttl,
	}).Debug("Cache set")

	return nil
}

// Delete removes an item from cache
func (c *ProtocolCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
	c.log.WithField("key", key).Debug("Cache delete")

	return nil
}

// InvalidateByTags invalidates cache entries by tags
func (c *ProtocolCache) InvalidateByTags(ctx context.Context, tags []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	invalidated := 0
	for key, entry := range c.cache {
		if c.hasMatchingTags(entry.Tags, tags) {
			delete(c.cache, key)
			invalidated++
		}
	}

	c.log.WithFields(logrus.Fields{
		"tags":        tags,
		"invalidated": invalidated,
	}).Info("Cache invalidation by tags")

	return nil
}

// InvalidateByPattern invalidates cache entries matching a pattern
func (c *ProtocolCache) InvalidateByPattern(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	invalidated := 0
	for key := range c.cache {
		if c.matchesPattern(key, pattern) {
			delete(c.cache, key)
			invalidated++
		}
	}

	c.log.WithFields(logrus.Fields{
		"pattern":     pattern,
		"invalidated": invalidated,
	}).Info("Cache invalidation by pattern")

	return nil
}

// Clear clears all cache entries
func (c *ProtocolCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.cache)
	c.cache = make(map[string]*CacheEntry)

	c.log.WithField("entries", count).Info("Cache cleared")

	return nil
}

// GetStats returns cache statistics
func (c *ProtocolCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalHits := 0
	totalMisses := 0
	totalSize := 0

	for _, entry := range c.cache {
		totalHits += entry.Hits
		totalSize += entry.Size
	}

	totalRequests := totalHits + totalMisses
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(totalHits) / float64(totalRequests)
	}

	return CacheStats{
		TotalEntries: len(c.cache),
		TotalSize:    totalSize,
		HitRate:      hitRate,
		MissRate:     1.0 - hitRate,
		TotalHits:    totalHits,
		TotalMisses:  totalMisses,
	}
}

// SetInvalidator sets an invalidation rule
func (c *ProtocolCache) SetInvalidator(key string, invalidator CacheInvalidator) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.invalidators[key] = append(c.invalidators[key], invalidator)
}

// RemoveInvalidator removes an invalidation rule
func (c *ProtocolCache) RemoveInvalidator(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.invalidators, key)
}

// Warmup pre-populates cache with common data
func (c *ProtocolCache) Warmup(ctx context.Context, data map[string]interface{}) error {
	for key, value := range data {
		tags := []string{"warmup"}
		if err := c.Set(ctx, key, value, tags, c.ttl); err != nil {
			return fmt.Errorf("failed to warmup cache entry %s: %w", key, err)
		}
	}

	c.log.WithField("entries", len(data)).Info("Cache warmup completed")
	return nil
}

// Private methods

func (c *ProtocolCache) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanupExpired()
	}
}

func (c *ProtocolCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	expired := 0
	for key, entry := range c.cache {
		if time.Since(entry.CreatedAt) > entry.TTL {
			delete(c.cache, key)
			expired++
		}
	}

	if expired > 0 {
		c.log.WithField("expired", expired).Debug("Cache cleanup completed")
	}
}

func (c *ProtocolCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range c.cache {
		if first || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
			first = false
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
		c.log.WithField("key", oldestKey).Debug("Cache LRU eviction")
	}
}

func (c *ProtocolCache) evict(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

func (c *ProtocolCache) hasMatchingTags(entryTags, queryTags []string) bool {
	if len(queryTags) == 0 {
		return false
	}

	for _, queryTag := range queryTags {
		for _, entryTag := range entryTags {
			if entryTag == queryTag {
				return true
			}
		}
	}

	return false
}

func (c *ProtocolCache) matchesPattern(key, pattern string) bool {
	// Simple wildcard matching
	if pattern == "*" {
		return true
	}

	// TODO: Implement more sophisticated pattern matching
	return len(key) >= len(pattern) && key[:len(pattern)] == pattern
}

func (c *ProtocolCache) calculateSize(data interface{}) int {
	// Rough size estimation
	switch v := data.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	case map[string]interface{}:
		jsonData, _ := json.Marshal(v)
		return len(jsonData)
	case []interface{}:
		jsonData, _ := json.Marshal(v)
		return len(jsonData)
	default:
		jsonData, _ := json.Marshal(v)
		return len(jsonData)
	}
}

// GenerateCacheKey generates a consistent cache key
func GenerateCacheKey(protocol, operation string, params map[string]interface{}) string {
	// Create a deterministic key from parameters
	paramStr := ""
	if params != nil {
		paramBytes, _ := json.Marshal(params)
		paramStr = string(paramBytes)
	}

	key := fmt.Sprintf("%s:%s:%s", protocol, operation, paramStr)
	return fmt.Sprintf("%x", md5.Sum([]byte(key)))
}

// Protocol-aware cache keys
const (
	CacheKeyMCPServer = "mcp:server:%s"
	CacheKeyMCPTools  = "mcp:tools:%s"
	CacheKeyMCPResult = "mcp:result:%s:%s"
	CacheKeyLSPServer = "lsp:server:%s"
	CacheKeyLSPResult = "lsp:result:%s:%s"
	CacheKeyACPServer = "acp:server:%s"
	CacheKeyACPResult = "acp:result:%s:%s"
	CacheKeyEmbedding = "embedding:%s"
)

// Cache tags for invalidation
const (
	CacheTagMCP       = "mcp"
	CacheTagLSP       = "lsp"
	CacheTagACP       = "acp"
	CacheTagEmbedding = "embedding"
	CacheTagServer    = "server"
	CacheTagTools     = "tools"
	CacheTagResults   = "results"
)
