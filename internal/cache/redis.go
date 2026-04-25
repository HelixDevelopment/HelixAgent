package cache

import (
	"context"
	"encoding/json"
	"runtime/debug"
	"time"

	"digital.vasic.cache/pkg/cache"
	cacheRedis "digital.vasic.cache/pkg/redis"

	"dev.helix.agent/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// RedisClient wraps the extracted cache module's Redis client
// while maintaining backward compatibility with existing HelixAgent code.
type RedisClient struct {
	// client is the raw go-redis client for advanced operations
	client *redis.Client
	// cacheClient is the extracted module's cache interface
	cacheClient *cacheRedis.Client
}

// NewRedisClient creates a new Redis client from HelixAgent config.
// Uses the extracted digital.vasic.cache module internally.
func NewRedisClient(cfg *config.Config) *RedisClient {
	if cfg == nil {
		// Return a client that will fail on connection attempts.
		// This ensures caching is disabled when config is nil.
		//
		// Drainage report 2026-04-25 Finding #12: log a stack-traced WARN
		// when this branch fires so future occurrences of "redis: dial tcp
		// 127.0.0.1:0" in any consumer log can be traced back to the actual
		// caller passing nil cfg. Production callers (router.go) guard
		// correctly; the warn here flags every site that doesn't.
		logrus.WithField("stack", string(debug.Stack())).
			Warn("cache.NewRedisClient called with nil config — returning no-op client (localhost:0). Caller should guard or wire real config.")
		// Close the underlying client immediately so its connection-pool
		// goroutines (checkMinIdleConns/dialConn) tear down cleanly. Without
		// this, every nil-cfg call leaks pool goroutines that hang trying to
		// dial localhost:0 forever — caught by goleak in the unit tests once
		// the WARN above made the timing observable. The closed client still
		// fulfils the *redis.Client contract (subsequent Set/Get/etc. return
		// "client is closed" errors instead of "dial 127.0.0.1:0" hangs).
		c := redis.NewClient(&redis.Options{
			Addr: "localhost:0", // Invalid address to ensure connection fails
		})
		_ = c.Close()
		return &RedisClient{client: c}
	}

	// Create the extracted module's Redis client
	redisCfg := &cacheRedis.Config{
		Addr:         cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
	cacheClient := cacheRedis.New(redisCfg)

	return &RedisClient{
		client:      cacheClient.Underlying(),
		cacheClient: cacheClient,
	}
}

// Set stores a value with JSON serialization.
// Uses extracted module when available, falls back to raw client.
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Use cacheClient if available (preferred path)
	if r.cacheClient != nil {
		return r.cacheClient.Set(ctx, key, data, expiration)
	}

	// Fallback to raw client for backward compatibility with tests
	return r.client.Set(ctx, key, data, expiration).Err()
}

// Get retrieves and deserializes a value.
// Uses extracted module when available, falls back to raw client.
func (r *RedisClient) Get(ctx context.Context, key string, dest interface{}) error {
	// Use cacheClient if available (preferred path)
	if r.cacheClient != nil {
		data, err := r.cacheClient.Get(ctx, key)
		if err != nil {
			return err
		}
		if data == nil {
			return redis.Nil
		}
		return json.Unmarshal(data, dest)
	}

	// Fallback to raw client for backward compatibility with tests
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// Delete removes a key.
// Uses extracted module when available, falls back to raw client.
func (r *RedisClient) Delete(ctx context.Context, key string) error {
	if r.cacheClient != nil {
		return r.cacheClient.Delete(ctx, key)
	}
	return r.client.Del(ctx, key).Err()
}

// MGet retrieves multiple values (uses raw client for compatibility)
func (r *RedisClient) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	return r.client.MGet(ctx, keys...).Result()
}

// Pipeline returns a Redis pipeline (uses raw client)
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// Client returns the raw go-redis client for advanced operations
func (r *RedisClient) Client() *redis.Client {
	return r.client
}

// CacheClient returns the extracted module's cache interface
func (r *RedisClient) CacheClient() cache.Cache {
	return r.cacheClient
}

// Ping checks Redis connectivity.
// Uses extracted module when available, falls back to raw client.
func (r *RedisClient) Ping(ctx context.Context) error {
	if r.cacheClient != nil {
		return r.cacheClient.HealthCheck(ctx)
	}
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection.
// Uses extracted module when available, falls back to raw client.
func (r *RedisClient) Close() error {
	if r.cacheClient != nil {
		return r.cacheClient.Close()
	}
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
