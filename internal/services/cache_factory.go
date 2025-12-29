package services

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/cache"
)

// CacheFactory creates cache instances based on configuration
type CacheFactory struct {
	redisClient *cache.RedisClient
	log         *logrus.Logger
}

// NewCacheFactory creates a new cache factory
func NewCacheFactory(redisClient *cache.RedisClient, log *logrus.Logger) *CacheFactory {
	return &CacheFactory{
		redisClient: redisClient,
		log:         log,
	}
}

// CreateCache creates a cache instance based on the type
func (f *CacheFactory) CreateCache(cacheType string, ttl time.Duration) CacheInterface {
	switch cacheType {
	case "redis":
		if f.redisClient != nil {
			f.log.Info("Using Redis cache for model metadata")
			return NewModelMetadataRedisCache(f.redisClient, "modelsdev", ttl, f.log)
		}
		f.log.Warn("Redis client not available, falling back to in-memory cache")
		fallthrough
	case "memory", "":
		f.log.Info("Using in-memory cache for model metadata")
		return NewInMemoryCache(ttl)
	case "multi":
		if f.redisClient != nil {
			f.log.Info("Using multi-level cache for model metadata")
			inMemoryCache := NewInMemoryCache(ttl)
			redisCache := NewModelMetadataRedisCache(f.redisClient, "modelsdev", ttl, f.log)
			return NewMultiLevelCache(inMemoryCache, redisCache, f.log)
		}
		f.log.Warn("Redis client not available, falling back to in-memory cache")
		return NewInMemoryCache(ttl)
	default:
		f.log.WithField("cache_type", cacheType).Warn("Unknown cache type, using in-memory")
		return NewInMemoryCache(ttl)
	}
}

// TestCacheConnection tests if Redis cache is available
func (f *CacheFactory) TestCacheConnection(ctx context.Context) bool {
	if f.redisClient == nil {
		return false
	}

	if err := f.redisClient.Ping(ctx); err != nil {
		f.log.WithError(err).Warn("Redis cache connection test failed")
		return false
	}

	f.log.Info("Redis cache connection test successful")
	return true
}

// CreateDefaultCache creates the default cache based on available components
func (f *CacheFactory) CreateDefaultCache(ttl time.Duration) CacheInterface {
	// Test Redis connection first
	if f.TestCacheConnection(context.Background()) {
		f.log.Info("Redis available, using multi-level cache")
		inMemoryCache := NewInMemoryCache(ttl)
		redisCache := NewModelMetadataRedisCache(f.redisClient, "modelsdev", ttl, f.log)
		return NewMultiLevelCache(inMemoryCache, redisCache, f.log)
	}

	// Fall back to in-memory cache
	f.log.Info("Redis not available, using in-memory cache")
	return NewInMemoryCache(ttl)
}
