package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/database"
)

// ProtocolCacheManager handles caching for MCP, LSP, ACP, and Embedding protocols
type ProtocolCacheManager struct {
	repo  *database.ModelMetadataRepository
	cache *ProtocolCache
	log   *logrus.Logger
}

// ProtocolCacheEntry represents a cached protocol response
type ProtocolCacheEntry struct {
	Key       string      `json:"key"`
	Data      interface{} `json:"data"`
	ExpiresAt *time.Time  `json:"expiresAt"`
	Protocol  string      `json:"protocol"` // "mcp", "lsp", "acp", "embedding"
	Timestamp time.Time   `json:"timestamp"`
}

// NewProtocolCacheManager creates a new protocol cache manager
func NewProtocolCacheManager(repo *database.ModelMetadataRepository, cache *ProtocolCache, log *logrus.Logger) *ProtocolCacheManager {
	return &ProtocolCacheManager{
		repo:  repo,
		cache: cache,
		log:   log,
	}
}

// Set stores data in cache with TTL
func (p *ProtocolCacheManager) Set(ctx context.Context, protocol, key string, data interface{}, ttl time.Duration) error {
	p.log.WithFields(logrus.Fields{
		"protocol": protocol,
		"key":      key,
		"ttl":      ttl,
	}).Debug("Setting protocol cache entry")

	cacheKey := fmt.Sprintf("protocol_cache_%s_%s", protocol, key)
	tags := []string{protocol, CacheTagResults}

	if err := p.cache.Set(ctx, cacheKey, data, tags, ttl); err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	return nil
}

// Get retrieves data from cache
func (p *ProtocolCacheManager) Get(ctx context.Context, protocol, key string) (interface{}, bool, error) {
	p.log.WithFields(logrus.Fields{
		"protocol": protocol,
		"key":      key,
	}).Debug("Getting protocol cache entry")

	cacheKey := fmt.Sprintf("protocol_cache_%s_%s", protocol, key)
	data, found, err := p.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get cache entry: %w", err)
	}

	return data, found, nil
}

// Delete removes a cache entry
func (p *ProtocolCacheManager) Delete(ctx context.Context, protocol, key string) error {
	p.log.WithFields(logrus.Fields{
		"protocol": protocol,
		"key":      key,
	}).Debug("Deleting protocol cache entry")

	cacheKey := fmt.Sprintf("protocol_cache_%s_%s", protocol, key)
	if err := p.cache.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}

	return nil
}

// CleanupExpired removes expired cache entries
// Note: ProtocolCache already runs automatic cleanup in background,
// this method is for manual/on-demand cleanup
func (p *ProtocolCacheManager) CleanupExpired(ctx context.Context) error {
	p.log.Info("Cleaning up expired protocol cache entries")

	// Clear all protocol entries - the ProtocolCache handles TTL automatically
	// For a full cleanup, we invalidate by pattern
	for _, protocol := range []string{CacheTagMCP, CacheTagLSP, CacheTagACP, CacheTagEmbedding} {
		if err := p.cache.InvalidateByTags(ctx, []string{protocol}); err != nil {
			p.log.WithError(err).Warnf("Failed to cleanup %s cache entries", protocol)
		}
	}

	return nil
}

// GetCacheStats returns statistics about cache usage
func (p *ProtocolCacheManager) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	cacheStats := p.cache.GetStats()

	stats := map[string]interface{}{
		"cacheType":    "protocol",
		"timestamp":    time.Now(),
		"totalEntries": cacheStats.TotalEntries,
		"totalSize":    cacheStats.TotalSize,
		"hitRate":      cacheStats.HitRate,
		"missRate":     cacheStats.MissRate,
		"totalHits":    cacheStats.TotalHits,
		"totalMisses":  cacheStats.TotalMisses,
		"evictions":    cacheStats.Evictions,
	}

	p.log.WithFields(logrus.Fields{
		"entries": cacheStats.TotalEntries,
		"hitRate": cacheStats.HitRate,
	}).Debug("Protocol cache statistics retrieved")

	return stats, nil
}

// InvalidateByPattern removes cache entries matching a pattern
func (p *ProtocolCacheManager) InvalidateByPattern(ctx context.Context, protocol, pattern string) error {
	p.log.WithFields(logrus.Fields{
		"protocol": protocol,
		"pattern":  pattern,
	}).Info("Invalidating cache entries by pattern")

	// Build the full pattern with protocol prefix
	fullPattern := fmt.Sprintf("protocol_cache_%s_%s", protocol, pattern)
	if err := p.cache.InvalidateByPattern(ctx, fullPattern); err != nil {
		return fmt.Errorf("failed to invalidate cache by pattern: %w", err)
	}

	return nil
}

// SetWithInvalidation marks cache entries as invalid (for cache invalidation)
func (p *ProtocolCacheManager) SetWithInvalidation(ctx context.Context, protocol, key string, data interface{}, ttl time.Duration, invalidateOn string) error {
	err := p.Set(ctx, protocol, key, data, ttl)
	if err != nil {
		return err
	}

	// Mark related cache entries for invalidation
	for _, pattern := range strings.Split(invalidateOn, ",") {
		err = p.Delete(ctx, protocol, pattern)
		if err != nil {
			p.log.WithError(err).Error("Failed to mark cache entries for invalidation")
		}
	}

	return nil
}

// WarmupCache warms up cache with frequently accessed data
func (p *ProtocolCacheManager) WarmupCache(ctx context.Context) error {
	p.log.Info("Warming up protocol cache with frequently accessed data")

	// Pre-populate with empty protocol registries for faster first access
	warmupData := map[string]interface{}{
		"protocol_cache_mcp_registry":       map[string]interface{}{"initialized": true},
		"protocol_cache_lsp_registry":       map[string]interface{}{"initialized": true},
		"protocol_cache_acp_registry":       map[string]interface{}{"initialized": true},
		"protocol_cache_embedding_registry": map[string]interface{}{"initialized": true},
	}

	if err := p.cache.Warmup(ctx, warmupData); err != nil {
		return fmt.Errorf("failed to warmup cache: %w", err)
	}

	return nil
}

// GetProtocolsWithCache returns cache entries grouped by protocol
func (p *ProtocolCacheManager) GetProtocolsWithCache(ctx context.Context) (map[string]map[string]interface{}, error) {
	p.log.Info("Retrieving cache entries grouped by protocol")

	result := make(map[string]map[string]interface{})

	// Check each protocol for cached data
	protocols := []string{CacheTagMCP, CacheTagLSP, CacheTagACP, CacheTagEmbedding}
	for _, protocol := range protocols {
		registryKey := fmt.Sprintf("protocol_cache_%s_registry", protocol)
		data, found, err := p.cache.Get(ctx, registryKey)
		if err != nil {
			p.log.WithError(err).Warnf("Failed to get %s registry", protocol)
			continue
		}
		if found {
			if dataMap, ok := data.(map[string]interface{}); ok {
				result[protocol] = dataMap
			}
		}
	}

	return result, nil
}

// MonitorCacheHealth monitors cache health and performance
func (p *ProtocolCacheManager) MonitorCacheHealth(ctx context.Context) error {
	p.log.Info("Monitoring protocol cache health")

	stats := p.cache.GetStats()

	// Log health metrics
	p.log.WithFields(logrus.Fields{
		"entries":  stats.TotalEntries,
		"size":     stats.TotalSize,
		"hitRate":  stats.HitRate,
		"missRate": stats.MissRate,
	}).Info("Cache health status")

	// Check for concerning metrics
	if stats.HitRate < 0.5 && stats.TotalHits > 100 {
		p.log.Warn("Cache hit rate is below 50% - consider increasing cache size or TTL")
	}

	return nil
}
