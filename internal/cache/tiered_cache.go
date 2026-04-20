package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/redis/go-redis/v9"
)

// TieredCacheConfig holds configuration for the tiered cache
type TieredCacheConfig struct {
	// L1 (in-memory) settings
	L1MaxSize         int           // Maximum items in memory
	L1TTL             time.Duration // Memory cache TTL
	L1CleanupInterval time.Duration // Cleanup interval for expired entries

	// L2 (Redis) settings
	L2TTL         time.Duration // Redis cache TTL
	L2Compression bool          // Enable compression for L2 values
	L2KeyPrefix   string        // Prefix for all L2 keys

	// General settings
	NegativeTTL time.Duration // Cache negative results (not found)
	EnableL1    bool          // Enable L1 cache
	EnableL2    bool          // Enable L2 cache
}

// DefaultTieredCacheConfig returns default configuration
func DefaultTieredCacheConfig() *TieredCacheConfig {
	return &TieredCacheConfig{
		L1MaxSize:         10000,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		L2TTL:             30 * time.Minute,
		L2Compression:     true,
		L2KeyPrefix:       "tiered:",
		NegativeTTL:       30 * time.Second,
		EnableL1:          true,
		EnableL2:          true,
	}
}

// TieredCache provides L1 (memory) + L2 (Redis) caching with compression
type TieredCache struct {
	l1       *l1Cache
	l2       *redis.Client
	config   *TieredCacheConfig
	metrics  *TieredCacheMetrics
	tagIndex *tagIndex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// TieredCacheMetrics tracks cache performance
type TieredCacheMetrics struct {
	L1Hits           int64
	L1Misses         int64
	L2Hits           int64
	L2Misses         int64
	Invalidations    int64
	Expirations      int64
	CompressionSaved int64 // Bytes saved by compression
	L1Size           int64
	L1Evictions      int64
}

// l1Cache is the in-memory L1 cache.
//
// Concurrency model (CONST-029): entries is a *safe.Store.
// evictionMu (Pattern Zeta) serialises the compound "check size →
// evict LRU → insert" flow in Set; readers use Get lock-free.
type l1Cache struct {
	entries    *safe.Store[string, *l1Entry]
	evictionMu sync.Mutex
	maxSize    int
	metrics    *TieredCacheMetrics
}

type l1Entry struct {
	value     []byte
	expiresAt time.Time
	tags      []string
	hitCount  int64
}

// tagIndex maps tags to cache keys for efficient invalidation.
//
// Concurrency model (CONST-029): index is a *safe.Store whose inner
// per-tag key sets are replaced atomically via Store.Update with
// copy-on-write.
type tagIndex struct {
	index *safe.Store[string, map[string]struct{}] // tag -> keys
}

func newTagIndex() *tagIndex {
	return &tagIndex{
		index: safe.NewStore[string, map[string]struct{}](),
	}
}

func (t *tagIndex) Add(key string, tags ...string) {
	for _, tag := range tags {
		t.index.Update(tag, func(cur map[string]struct{}, _ bool) (map[string]struct{}, bool) {
			next := make(map[string]struct{}, len(cur)+1)
			for k := range cur {
				next[k] = struct{}{}
			}
			next[key] = struct{}{}
			return next, true
		})
	}
}

func (t *tagIndex) Remove(key string) {
	var candidates []string
	for tag, keys := range t.index.Snapshot() {
		if _, has := keys[key]; has {
			candidates = append(candidates, tag)
		}
	}
	for _, tag := range candidates {
		t.index.Update(tag, func(cur map[string]struct{}, present bool) (map[string]struct{}, bool) {
			if !present {
				return nil, false
			}
			if _, has := cur[key]; !has {
				return cur, true
			}
			if len(cur) == 1 {
				return nil, false
			}
			next := make(map[string]struct{}, len(cur)-1)
			for k := range cur {
				if k != key {
					next[k] = struct{}{}
				}
			}
			return next, true
		})
	}
}

func (t *tagIndex) GetKeys(tag string) []string {
	keys, ok := t.index.Get(tag)
	if !ok || keys == nil {
		return nil
	}

	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	return result
}

// NewTieredCache creates a new tiered cache
func NewTieredCache(redisClient *redis.Client, config *TieredCacheConfig) *TieredCache {
	if config == nil {
		config = DefaultTieredCacheConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	metrics := &TieredCacheMetrics{}

	tc := &TieredCache{
		l1: &l1Cache{
			entries: safe.NewStore[string, *l1Entry](),
			maxSize: config.L1MaxSize,
			metrics: metrics,
		},
		l2:       redisClient,
		config:   config,
		metrics:  metrics,
		tagIndex: newTagIndex(),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start L1 cleanup goroutine
	if config.EnableL1 {
		tc.wg.Add(1)
		go func() {
			defer tc.wg.Done()
			tc.l1CleanupLoop()
		}()
	}

	return tc
}

// Get retrieves a value from the cache, checking L1 first then L2
func (c *TieredCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	// Try L1 first
	if c.config.EnableL1 {
		if data, ok := c.l1Get(key); ok {
			atomic.AddInt64(&c.metrics.L1Hits, 1)
			return true, json.Unmarshal(data, dest)
		}
		atomic.AddInt64(&c.metrics.L1Misses, 1)
	}

	// Try L2
	if c.config.EnableL2 && c.l2 != nil {
		data, err := c.l2Get(ctx, key)
		if err == nil && data != nil {
			atomic.AddInt64(&c.metrics.L2Hits, 1)

			// Promote to L1
			if c.config.EnableL1 {
				c.l1Set(key, data, c.config.L1TTL, nil)
			}

			return true, json.Unmarshal(data, dest)
		}
		if err != nil && err != redis.Nil {
			return false, fmt.Errorf("l2 get: %w", err)
		}
		atomic.AddInt64(&c.metrics.L2Misses, 1)
	}

	return false, nil
}

// Set stores a value in both L1 and L2 caches
func (c *TieredCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}

	// Set in L1
	if c.config.EnableL1 {
		l1TTL := ttl
		if l1TTL > c.config.L1TTL {
			l1TTL = c.config.L1TTL
		}
		c.l1Set(key, data, l1TTL, tags)
	}

	// Set in L2
	if c.config.EnableL2 && c.l2 != nil {
		if err := c.l2Set(ctx, key, data, ttl); err != nil {
			return fmt.Errorf("l2 set: %w", err)
		}
	}

	// Update tag index
	if len(tags) > 0 {
		c.tagIndex.Add(key, tags...)
	}

	return nil
}

// Delete removes a value from both caches
func (c *TieredCache) Delete(ctx context.Context, key string) error {
	// Delete from L1
	if c.config.EnableL1 {
		c.l1Delete(key)
	}

	// Delete from L2
	if c.config.EnableL2 && c.l2 != nil {
		if err := c.l2.Del(ctx, c.config.L2KeyPrefix+key).Err(); err != nil {
			return fmt.Errorf("l2 delete: %w", err)
		}
	}

	// Remove from tag index
	c.tagIndex.Remove(key)

	atomic.AddInt64(&c.metrics.Invalidations, 1)
	return nil
}

// InvalidateByTag invalidates all entries with the given tag
func (c *TieredCache) InvalidateByTag(ctx context.Context, tag string) (int, error) {
	keys := c.tagIndex.GetKeys(tag)
	if len(keys) == 0 {
		return 0, nil
	}

	count := 0
	for _, key := range keys {
		if err := c.Delete(ctx, key); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// InvalidateByTags invalidates all entries with any of the given tags
func (c *TieredCache) InvalidateByTags(ctx context.Context, tags ...string) (int, error) {
	totalCount := 0
	for _, tag := range tags {
		count, err := c.InvalidateByTag(ctx, tag)
		if err != nil {
			return totalCount, err
		}
		totalCount += count
	}
	return totalCount, nil
}

// InvalidatePrefix invalidates all entries with keys matching the prefix
func (c *TieredCache) InvalidatePrefix(ctx context.Context, prefix string) (int, error) {
	count := 0

	// Invalidate L1
	if c.config.EnableL1 {
		var toDelete []string
		c.l1.entries.Range(func(key string, _ *l1Entry) bool {
			if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
				toDelete = append(toDelete, key)
			}
			return true
		})
		for _, key := range toDelete {
			c.l1.entries.Delete(key)
			count++
		}
	}

	// Invalidate L2 using SCAN
	if c.config.EnableL2 && c.l2 != nil {
		pattern := c.config.L2KeyPrefix + prefix + "*"
		var cursor uint64
		for {
			keys, nextCursor, err := c.l2.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return count, fmt.Errorf("l2 scan: %w", err)
			}

			if len(keys) > 0 {
				if err := c.l2.Del(ctx, keys...).Err(); err != nil {
					return count, fmt.Errorf("l2 delete: %w", err)
				}
				count += len(keys)
			}

			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}

	atomic.AddInt64(&c.metrics.Invalidations, int64(count))
	return count, nil
}

// Metrics returns current cache metrics
func (c *TieredCache) Metrics() *TieredCacheMetrics {
	l1Size := int64(c.l1.entries.Len())

	return &TieredCacheMetrics{
		L1Hits:           atomic.LoadInt64(&c.metrics.L1Hits),
		L1Misses:         atomic.LoadInt64(&c.metrics.L1Misses),
		L2Hits:           atomic.LoadInt64(&c.metrics.L2Hits),
		L2Misses:         atomic.LoadInt64(&c.metrics.L2Misses),
		Invalidations:    atomic.LoadInt64(&c.metrics.Invalidations),
		Expirations:      atomic.LoadInt64(&c.metrics.Expirations),
		CompressionSaved: atomic.LoadInt64(&c.metrics.CompressionSaved),
		L1Size:           l1Size,
		L1Evictions:      atomic.LoadInt64(&c.metrics.L1Evictions),
	}
}

// HitRate returns the overall cache hit rate
func (c *TieredCache) HitRate() float64 {
	l1Hits := atomic.LoadInt64(&c.metrics.L1Hits)
	l2Hits := atomic.LoadInt64(&c.metrics.L2Hits)
	l1Misses := atomic.LoadInt64(&c.metrics.L1Misses)
	l2Misses := atomic.LoadInt64(&c.metrics.L2Misses)

	total := l1Hits + l2Hits + l1Misses + l2Misses
	if total == 0 {
		return 0
	}

	return float64(l1Hits+l2Hits) / float64(total) * 100
}

// Close closes the tiered cache
func (c *TieredCache) Close() error {
	c.cancel()
	c.wg.Wait()
	return nil
}

// L1 cache operations

func (c *TieredCache) l1Get(key string) ([]byte, bool) {
	entry, exists := c.l1.entries.Get(key)
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.l1Delete(key)
		return nil, false
	}

	atomic.AddInt64(&entry.hitCount, 1)
	return entry.value, true
}

func (c *TieredCache) l1Set(key string, value []byte, ttl time.Duration, tags []string) {
	c.l1.evictionMu.Lock()
	defer c.l1.evictionMu.Unlock()

	// Evict if at capacity
	if c.l1.entries.Len() >= c.l1.maxSize {
		c.l1EvictLRU()
	}

	c.l1.entries.Put(key, &l1Entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
		tags:      tags,
	})

	atomic.StoreInt64(&c.metrics.L1Size, int64(c.l1.entries.Len()))
}

func (c *TieredCache) l1Delete(key string) {
	c.l1.entries.Delete(key)
	atomic.StoreInt64(&c.metrics.L1Size, int64(c.l1.entries.Len()))
}

// l1EvictLRU must be called with c.l1.evictionMu held.
func (c *TieredCache) l1EvictLRU() {
	// Find entry with lowest hit count (simple LRU approximation)
	var lowestKey string
	var lowestHits int64 = -1

	c.l1.entries.Range(func(key string, entry *l1Entry) bool {
		if lowestHits < 0 || entry.hitCount < lowestHits {
			lowestKey = key
			lowestHits = entry.hitCount
		}
		return true
	})

	if lowestKey != "" {
		c.l1.entries.Delete(lowestKey)
		atomic.AddInt64(&c.metrics.L1Evictions, 1)
	}
}

func (c *TieredCache) l1CleanupLoop() {
	interval := c.config.L1CleanupInterval
	if interval <= 0 {
		interval = time.Minute // Default cleanup interval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.l1Cleanup()
		}
	}
}

func (c *TieredCache) l1Cleanup() {
	now := time.Now()
	var expiredKeys []string

	c.l1.entries.Range(func(key string, entry *l1Entry) bool {
		if now.After(entry.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
		return true
	})

	for _, key := range expiredKeys {
		c.l1.entries.Delete(key)
	}

	if len(expiredKeys) > 0 {
		atomic.AddInt64(&c.metrics.Expirations, int64(len(expiredKeys)))
		atomic.StoreInt64(&c.metrics.L1Size, int64(c.l1.entries.Len()))
	}
}

// L2 cache operations

func (c *TieredCache) l2Get(ctx context.Context, key string) ([]byte, error) {
	data, err := c.l2.Get(ctx, c.config.L2KeyPrefix+key).Bytes()
	if err != nil {
		return nil, err
	}

	// Decompress if compression is enabled
	if c.config.L2Compression && len(data) > 0 && data[0] == 0x1f {
		return c.decompress(data)
	}

	return data, nil
}

func (c *TieredCache) l2Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	data := value

	// Compress if enabled and value is large enough
	if c.config.L2Compression && len(value) > 100 {
		compressed, err := c.compress(value)
		if err == nil && len(compressed) < len(value) {
			saved := int64(len(value) - len(compressed))
			atomic.AddInt64(&c.metrics.CompressionSaved, saved)
			data = compressed
		}
	}

	return c.l2.Set(ctx, c.config.L2KeyPrefix+key, data, ttl).Err()
}

func (c *TieredCache) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *TieredCache) decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}
