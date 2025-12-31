package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/cache"
	"github.com/superagent/superagent/internal/database"
)

func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}

func TestNewCacheFactory(t *testing.T) {
	logger := newTestLogger()

	t.Run("with nil redis client", func(t *testing.T) {
		factory := NewCacheFactory(nil, logger)
		require.NotNil(t, factory)
		assert.Nil(t, factory.redisClient)
		assert.NotNil(t, factory.log)
	})
}

func TestCacheFactory_CreateCache(t *testing.T) {
	logger := newTestLogger()
	factory := NewCacheFactory(nil, logger)

	t.Run("memory cache type", func(t *testing.T) {
		cache := factory.CreateCache("memory", 5*time.Minute)
		require.NotNil(t, cache)
		// Should be InMemoryCache
	})

	t.Run("empty cache type defaults to memory", func(t *testing.T) {
		cache := factory.CreateCache("", 5*time.Minute)
		require.NotNil(t, cache)
	})

	t.Run("redis cache type falls back to memory when no client", func(t *testing.T) {
		cache := factory.CreateCache("redis", 5*time.Minute)
		require.NotNil(t, cache)
		// Should fall back to InMemoryCache since no Redis client
	})

	t.Run("multi cache type falls back to memory when no client", func(t *testing.T) {
		cache := factory.CreateCache("multi", 5*time.Minute)
		require.NotNil(t, cache)
		// Should fall back to InMemoryCache since no Redis client
	})

	t.Run("unknown cache type defaults to memory", func(t *testing.T) {
		cache := factory.CreateCache("unknown", 5*time.Minute)
		require.NotNil(t, cache)
	})
}

func TestCacheFactory_TestCacheConnection(t *testing.T) {
	logger := newTestLogger()

	t.Run("returns false when no redis client", func(t *testing.T) {
		factory := NewCacheFactory(nil, logger)
		result := factory.TestCacheConnection(context.Background())
		assert.False(t, result)
	})
}

func TestCacheFactory_CreateDefaultCache(t *testing.T) {
	logger := newTestLogger()

	t.Run("creates in-memory cache when no redis", func(t *testing.T) {
		factory := NewCacheFactory(nil, logger)
		cache := factory.CreateDefaultCache(5 * time.Minute)
		require.NotNil(t, cache)
	})
}

func TestInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache(5 * time.Minute)
	ctx := context.Background()

	t.Run("set and get", func(t *testing.T) {
		metadata := &database.ModelMetadata{
			ModelID:   "test-model-1",
			ModelName: "Test Model",
		}
		err := cache.Set(ctx, "test-model-1", metadata)
		require.NoError(t, err)

		value, exists, err := cache.Get(ctx, "test-model-1")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.NotNil(t, value)
		assert.Equal(t, "Test Model", value.ModelName)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		value, exists, err := cache.Get(ctx, "non-existent-key")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("delete", func(t *testing.T) {
		metadata := &database.ModelMetadata{ModelID: "delete-key"}
		err := cache.Set(ctx, "delete-key", metadata)
		require.NoError(t, err)

		err = cache.Delete(ctx, "delete-key")
		require.NoError(t, err)

		value, exists, err := cache.Get(ctx, "delete-key")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		err := cache.Delete(ctx, "non-existent-delete-key")
		require.NoError(t, err) // Should not error
	})

	t.Run("clear", func(t *testing.T) {
		err := cache.Set(ctx, "clear-key-1", &database.ModelMetadata{ModelID: "clear-key-1"})
		require.NoError(t, err)
		err = cache.Set(ctx, "clear-key-2", &database.ModelMetadata{ModelID: "clear-key-2"})
		require.NoError(t, err)

		err = cache.Clear(ctx)
		require.NoError(t, err)

		_, exists1, _ := cache.Get(ctx, "clear-key-1")
		_, exists2, _ := cache.Get(ctx, "clear-key-2")

		assert.False(t, exists1)
		assert.False(t, exists2)
	})

	t.Run("size", func(t *testing.T) {
		freshCache := NewInMemoryCache(5 * time.Minute)

		size, err := freshCache.Size(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, size)

		_ = freshCache.Set(ctx, "size-key-1", &database.ModelMetadata{ModelID: "size-key-1"})
		_ = freshCache.Set(ctx, "size-key-2", &database.ModelMetadata{ModelID: "size-key-2"})

		size, err = freshCache.Size(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, size)
	})

	t.Run("get bulk", func(t *testing.T) {
		freshCache := NewInMemoryCache(5 * time.Minute)
		_ = freshCache.Set(ctx, "bulk-1", &database.ModelMetadata{ModelID: "bulk-1", ModelName: "Model 1"})
		_ = freshCache.Set(ctx, "bulk-2", &database.ModelMetadata{ModelID: "bulk-2", ModelName: "Model 2"})

		result, err := freshCache.GetBulk(ctx, []string{"bulk-1", "bulk-2", "bulk-3"})
		require.NoError(t, err)
		assert.Len(t, result, 2) // bulk-3 doesn't exist
		assert.Equal(t, "Model 1", result["bulk-1"].ModelName)
		assert.Equal(t, "Model 2", result["bulk-2"].ModelName)
	})

	t.Run("set bulk", func(t *testing.T) {
		freshCache := NewInMemoryCache(5 * time.Minute)
		models := map[string]*database.ModelMetadata{
			"setbulk-1": {ModelID: "setbulk-1", ModelName: "Set Bulk 1"},
			"setbulk-2": {ModelID: "setbulk-2", ModelName: "Set Bulk 2"},
		}

		err := freshCache.SetBulk(ctx, models)
		require.NoError(t, err)

		val1, exists1, _ := freshCache.Get(ctx, "setbulk-1")
		val2, exists2, _ := freshCache.Get(ctx, "setbulk-2")

		assert.True(t, exists1)
		assert.True(t, exists2)
		assert.Equal(t, "Set Bulk 1", val1.ModelName)
		assert.Equal(t, "Set Bulk 2", val2.ModelName)
	})

	t.Run("health check", func(t *testing.T) {
		err := cache.HealthCheck(ctx)
		require.NoError(t, err) // In-memory cache is always healthy
	})

	t.Run("provider models returns nil", func(t *testing.T) {
		result, err := cache.GetProviderModels(ctx, "test-provider")
		require.NoError(t, err)
		assert.Nil(t, result)

		err = cache.SetProviderModels(ctx, "test-provider", []*database.ModelMetadata{})
		require.NoError(t, err)

		err = cache.DeleteProviderModels(ctx, "test-provider")
		require.NoError(t, err)
	})

	t.Run("capability operations return nil", func(t *testing.T) {
		result, err := cache.GetByCapability(ctx, "chat")
		require.NoError(t, err)
		assert.Nil(t, result)

		err = cache.SetByCapability(ctx, "chat", []*database.ModelMetadata{})
		require.NoError(t, err)
	})
}

func TestInMemoryCache_Expiration(t *testing.T) {
	cache := NewInMemoryCache(100 * time.Millisecond)
	ctx := context.Background()

	t.Run("item expires after TTL", func(t *testing.T) {
		metadata := &database.ModelMetadata{
			ModelID:   "expire-key",
			ModelName: "Expiring Model",
		}
		err := cache.Set(ctx, "expire-key", metadata)
		require.NoError(t, err)

		// Should exist immediately
		value, exists, err := cache.Get(ctx, "expire-key")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "Expiring Model", value.ModelName)

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Should be expired now
		value, exists, err = cache.Get(ctx, "expire-key")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, value)
	})
}

func BenchmarkInMemoryCache_Set(b *testing.B) {
	cache := NewInMemoryCache(5 * time.Minute)
	ctx := context.Background()
	metadata := &database.ModelMetadata{ModelID: "bench-key", ModelName: "Benchmark Model"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, "bench-key", metadata)
	}
}

func BenchmarkInMemoryCache_Get(b *testing.B) {
	cache := NewInMemoryCache(5 * time.Minute)
	ctx := context.Background()
	_ = cache.Set(ctx, "bench-key", &database.ModelMetadata{ModelID: "bench-key"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = cache.Get(ctx, "bench-key")
	}
}

func TestCacheFactory_TestCacheConnection_WithClient(t *testing.T) {
	logger := newTestLogger()
	// Create a RedisClient with invalid address - it will fail ping
	invalidRedisClient := cache.NewRedisClient(nil)

	t.Run("returns false when redis ping fails", func(t *testing.T) {
		factory := NewCacheFactory(invalidRedisClient, logger)
		result := factory.TestCacheConnection(context.Background())
		assert.False(t, result) // Should fail because address is invalid
	})
}

func TestCacheFactory_CreateDefaultCache_WithFailingRedis(t *testing.T) {
	logger := newTestLogger()
	// Create a RedisClient with invalid address
	invalidRedisClient := cache.NewRedisClient(nil)

	t.Run("falls back to in-memory when redis connection fails", func(t *testing.T) {
		factory := NewCacheFactory(invalidRedisClient, logger)
		resultCache := factory.CreateDefaultCache(5 * time.Minute)
		require.NotNil(t, resultCache)
		// Should have fallen back to in-memory cache since Redis ping fails
	})
}

func TestCacheFactory_CreateCache_AllTypes(t *testing.T) {
	logger := newTestLogger()

	t.Run("redis type with failing client falls back to memory", func(t *testing.T) {
		invalidRedisClient := cache.NewRedisClient(nil)
		factory := NewCacheFactory(invalidRedisClient, logger)
		resultCache := factory.CreateCache("redis", 5*time.Minute)
		require.NotNil(t, resultCache)
		// Creates redis cache wrapper even if connection will fail
	})

	t.Run("multi type with failing client creates multi-level cache", func(t *testing.T) {
		invalidRedisClient := cache.NewRedisClient(nil)
		factory := NewCacheFactory(invalidRedisClient, logger)
		resultCache := factory.CreateCache("multi", 5*time.Minute)
		require.NotNil(t, resultCache)
		// Creates multi-level cache even if Redis will fail
	})
}
