package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/superagent/superagent/internal/config"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(cfg *config.Config) *RedisClient {
	if cfg == nil {
		// Return a client that will fail on connection attempts
		// This ensures caching is disabled when config is nil
		return &RedisClient{client: redis.NewClient(&redis.Options{
			Addr: "localhost:0", // Invalid address to ensure connection fails
		})}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	return &RedisClient{client: rdb}
}

func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, expiration).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisClient) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	return r.client.MGet(ctx, keys...).Result()
}

func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

func (r *RedisClient) Client() *redis.Client {
	return r.client
}

func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
