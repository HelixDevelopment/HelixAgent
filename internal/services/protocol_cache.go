package services

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.concurrency/pkg/safe"
)

// ProtocolCache provides advanced caching for protocol operations.
//
// Concurrent-safe by construction (CONST-029 Tier 1): `cache` and
// `invalidators` are independent safe.Store instances — no method
// touches both, so joint atomicity is not required. Entries are stored
// by value; access-stat updates happen inside a safe.Store.Update
// callback, which eliminates the pre-migration pointer-mutation race
// that concurrent Get calls had on entry.AccessedAt / entry.Hits.
type ProtocolCache struct {
	cache        *safe.Store[string, CacheEntry]
	invalidators *safe.Store[string, []CacheInvalidator]
	maxSize      int
	ttl          time.Duration
	log          *logrus.Logger
	stopCh       chan struct{}
	stopOnce     sync.Once
	stopped      atomic.Bool
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
	c := &ProtocolCache{
		cache:        safe.NewStore[string, CacheEntry](),
		invalidators: safe.NewStore[string, []CacheInvalidator](),
		maxSize:      maxSize,
		ttl:          ttl,
		log:          logger,
		stopCh:       make(chan struct{}),
	}
	go c.cleanupRoutine()
	return c
}

// Stop stops the cleanup goroutine gracefully. Idempotent.
func (c *ProtocolCache) Stop() {
	c.stopOnce.Do(func() {
		c.stopped.Store(true)
		close(c.stopCh)
		c.log.Info("Protocol cache stopped")
	})
}

// Stopped reports whether Stop has been called.
func (c *ProtocolCache) Stopped() bool {
	return c.stopped.Load()
}

// Get retrieves an item from cache and atomically updates access stats.
// Expired entries are removed in-place.
func (c *ProtocolCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	var data interface{}
	var hit bool
	c.cache.Update(key, func(entry CacheEntry, ok bool) (CacheEntry, bool) {
		if !ok {
			return CacheEntry{}, false
		}
		if time.Since(entry.CreatedAt) > entry.TTL {
			return CacheEntry{}, false // delete expired
		}
		entry.AccessedAt = time.Now()
		entry.Hits++
		data = entry.Data
		hit = true
		c.log.WithFields(logrus.Fields{
			"key":  key,
			"hits": entry.Hits,
			"size": entry.Size,
		}).Debug("Cache hit")
		return entry, true
	})
	if !hit {
		return nil, false, nil
	}
	return data, true, nil
}

// Set stores an item in cache with tags. LRU eviction is approximate
// under concurrent Sets (the len-check and evict happen across
// multiple lock acquisitions); this matches the behavior of classic
// LRU caches under contention.
func (c *ProtocolCache) Set(ctx context.Context, key string, data interface{}, tags []string, ttl time.Duration) error {
	size := c.calculateSize(data)
	if ttl == 0 {
		ttl = c.ttl
	}
	entry := CacheEntry{
		Key:        key,
		Data:       data,
		Tags:       tags,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        ttl,
		Hits:       0,
		Size:       size,
	}

	for c.cache.Len() >= c.maxSize {
		c.evictLRU()
	}

	c.cache.Put(key, entry)
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
	c.cache.Delete(key)
	c.log.WithField("key", key).Debug("Cache delete")
	return nil
}

// InvalidateByTags invalidates cache entries by tags
func (c *ProtocolCache) InvalidateByTags(ctx context.Context, tags []string) error {
	invalidated := 0
	for key, entry := range c.cache.Snapshot() {
		if c.hasMatchingTags(entry.Tags, tags) {
			if _, ok := c.cache.Delete(key); ok {
				invalidated++
			}
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
	invalidated := 0
	for key := range c.cache.Snapshot() {
		if c.matchesPattern(key, pattern) {
			if _, ok := c.cache.Delete(key); ok {
				invalidated++
			}
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
	count := c.cache.Len()
	c.cache.Clear()
	c.log.WithField("entries", count).Info("Cache cleared")
	return nil
}

// GetStats returns cache statistics
func (c *ProtocolCache) GetStats() CacheStats {
	snap := c.cache.Snapshot()
	totalHits := 0
	totalSize := 0
	for _, entry := range snap {
		totalHits += entry.Hits
		totalSize += entry.Size
	}
	totalMisses := 0
	totalRequests := totalHits + totalMisses
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(totalHits) / float64(totalRequests)
	}
	return CacheStats{
		TotalEntries: len(snap),
		TotalSize:    totalSize,
		HitRate:      hitRate,
		MissRate:     1.0 - hitRate,
		TotalHits:    totalHits,
		TotalMisses:  totalMisses,
	}
}

// SetInvalidator sets an invalidation rule
func (c *ProtocolCache) SetInvalidator(key string, invalidator CacheInvalidator) {
	c.invalidators.Update(key, func(cur []CacheInvalidator, _ bool) ([]CacheInvalidator, bool) {
		return append(cur, invalidator), true
	})
}

// RemoveInvalidator removes an invalidation rule
func (c *ProtocolCache) RemoveInvalidator(key string) {
	c.invalidators.Delete(key)
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

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanupExpired()
		}
	}
}

func (c *ProtocolCache) cleanupExpired() {
	expired := 0
	for key, entry := range c.cache.Snapshot() {
		if time.Since(entry.CreatedAt) > entry.TTL {
			if _, ok := c.cache.Delete(key); ok {
				expired++
			}
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
	for key, entry := range c.cache.Snapshot() {
		if first || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
			first = false
		}
	}
	if oldestKey != "" {
		c.cache.Delete(oldestKey)
		c.log.WithField("key", oldestKey).Debug("Cache LRU eviction")
	}
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
	// Empty pattern matches nothing
	if pattern == "" {
		return false
	}

	// Full wildcard matches everything
	if pattern == "*" {
		return true
	}

	// Use glob-style pattern matching with wildcards
	return matchGlob(pattern, key)
}

// matchGlob performs glob-style pattern matching supporting:
// - '*' matches any sequence of characters (including empty)
// - '?' matches exactly one character
// - All other characters must match exactly
func matchGlob(pattern, text string) bool {
	// dp[i][j] represents whether pattern[0:i] matches text[0:j]
	// Using bottom-up dynamic programming for efficiency

	pLen := len(pattern)
	tLen := len(text)

	// Create DP table
	// dp[i][j] = true if pattern[0:i] matches text[0:j]
	dp := make([][]bool, pLen+1)
	for i := range dp {
		dp[i] = make([]bool, tLen+1)
	}

	// Empty pattern matches empty text
	dp[0][0] = true

	// Handle patterns that start with * (can match empty string)
	for i := 1; i <= pLen; i++ {
		if pattern[i-1] == '*' {
			dp[i][0] = dp[i-1][0]
		}
	}

	// Fill the DP table
	for i := 1; i <= pLen; i++ {
		for j := 1; j <= tLen; j++ {
			p := pattern[i-1]
			t := text[j-1]

			switch p {
			case '*':
				// '*' can match:
				// - Zero characters: dp[i-1][j] (skip the *)
				// - One or more characters: dp[i][j-1] (consume one char from text, keep *)
				dp[i][j] = dp[i-1][j] || dp[i][j-1]
			case '?':
				// '?' matches exactly one character
				dp[i][j] = dp[i-1][j-1]
			default:
				// Regular character must match exactly
				dp[i][j] = dp[i-1][j-1] && p == t
			}
		}
	}

	return dp[pLen][tLen]
}

func (c *ProtocolCache) calculateSize(data interface{}) int {
	// Rough size estimation
	switch v := data.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	case map[string]interface{}:
		jsonData, err := json.Marshal(v)
		if err != nil {
			return 0
		}
		return len(jsonData)
	case []interface{}:
		jsonData, err := json.Marshal(v)
		if err != nil {
			return 0
		}
		return len(jsonData)
	default:
		jsonData, err := json.Marshal(v)
		if err != nil {
			return 0
		}
		return len(jsonData)
	}
}

// GenerateCacheKey generates a consistent cache key
func GenerateCacheKey(protocol, operation string, params map[string]interface{}) string {
	// Create a deterministic key from parameters
	paramStr := ""
	if params != nil {
		paramBytes, err := json.Marshal(params)
		if err != nil {
			paramStr = ""
		} else {
			paramStr = string(paramBytes)
		}
	}

	key := fmt.Sprintf("%s:%s:%s", protocol, operation, paramStr)
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("%016x", h.Sum64())
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
