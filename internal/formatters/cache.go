package formatters

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// FormatterCache caches formatting results
type FormatterCache struct {
	cache       *safe.Store[string, *cacheEntry]
	config      *CacheConfig
	logger      *logrus.Logger
	stopCleanup chan struct{}
}

// CacheConfig configures the cache
type CacheConfig struct {
	TTL         time.Duration // Time to live for cache entries
	MaxSize     int           // Maximum number of cache entries
	CleanupFreq time.Duration // Cleanup frequency
}

// cacheEntry represents a cached result. Fields are set at
// construction and never mutated afterwards, so a snapshot pointer
// returned from the Store is safe to dereference without further
// synchronisation.
type cacheEntry struct {
	result    *FormatResult
	timestamp time.Time
}

// NewFormatterCache creates a new formatter cache
func NewFormatterCache(config *CacheConfig, logger *logrus.Logger) *FormatterCache {
	cache := &FormatterCache{
		cache:       safe.NewStore[string, *cacheEntry](),
		config:      config,
		logger:      logger,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached result
func (c *FormatterCache) Get(req *FormatRequest) (*FormatResult, bool) {
	key := c.cacheKey(req)
	entry, exists := c.cache.Get(key)
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.timestamp) > c.config.TTL {
		return nil, false
	}

	c.logger.Debugf("Cache hit for key: %s", key[:8])

	return entry.result, true
}

// Set stores a result in the cache
func (c *FormatterCache) Set(req *FormatRequest, result *FormatResult) {
	if c.cache.Len() >= c.config.MaxSize {
		c.evictOldest()
	}

	key := c.cacheKey(req)
	c.cache.Put(key, &cacheEntry{
		result:    result,
		timestamp: time.Now(),
	})

	c.logger.Debugf("Cached result for key: %s", key[:8])
}

// Clear clears the cache
func (c *FormatterCache) Clear() {
	c.cache.Clear()
	c.logger.Debug("Cache cleared")
}

// Size returns the number of cached entries
func (c *FormatterCache) Size() int {
	return c.cache.Len()
}

// Stop stops the cleanup goroutine
func (c *FormatterCache) Stop() {
	close(c.stopCleanup)
}

// cacheKey generates a cache key for a request
func (c *FormatterCache) cacheKey(req *FormatRequest) string {
	// Hash the content and relevant configuration
	h := sha256.New()
	h.Write([]byte(req.Content))
	h.Write([]byte(req.Language))
	h.Write([]byte(req.FilePath))

	return hex.EncodeToString(h.Sum(nil))
}

// evictOldest removes the oldest cache entry
func (c *FormatterCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	c.cache.Range(func(key string, entry *cacheEntry) bool {
		if oldestKey == "" || entry.timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.timestamp
		}
		return true
	})

	if oldestKey != "" {
		if _, existed := c.cache.Delete(oldestKey); existed {
			c.logger.Debugf("Evicted oldest cache entry: %s", oldestKey[:8])
		}
	}
}

// cleanupLoop periodically removes expired entries
func (c *FormatterCache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCleanup:
			return
		}
	}
}

// cleanup removes expired entries
func (c *FormatterCache) cleanup() {
	now := time.Now()
	var expired []string

	c.cache.Range(func(key string, entry *cacheEntry) bool {
		if now.Sub(entry.timestamp) > c.config.TTL {
			expired = append(expired, key)
		}
		return true
	})

	for _, key := range expired {
		c.cache.Delete(key)
	}

	if len(expired) > 0 {
		c.logger.Debugf("Cleaned up %d expired cache entries", len(expired))
	}
}

// Stats returns cache statistics
func (c *FormatterCache) Stats() CacheStats {
	return CacheStats{
		Size:    c.cache.Len(),
		MaxSize: c.config.MaxSize,
		TTL:     c.config.TTL,
	}
}

// CacheStats provides cache statistics
type CacheStats struct {
	Size    int
	MaxSize int
	TTL     time.Duration
}
