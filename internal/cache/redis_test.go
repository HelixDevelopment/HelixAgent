package cache

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/config"
)

// setupMiniRedis creates a miniredis server for testing
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *RedisClient) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := &RedisClient{
		client: redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		}),
	}

	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})

	return mr, client
}

// TestNewRedisClient tests the RedisClient constructor
func TestNewRedisClient(t *testing.T) {
	t.Run("Creates client with nil config", func(t *testing.T) {
		client := NewRedisClient(nil)
		assert.NotNil(t, client)
		assert.NotNil(t, client.client)

		// Ping should fail since the client is configured with invalid address
		err := client.Ping(context.Background())
		assert.Error(t, err)

		client.Close()
	})

	t.Run("Creates client with valid config", func(t *testing.T) {
		cfg := &config.Config{
			Redis: config.RedisConfig{
				Host:     "localhost",
				Port:     "6379",
				Password: "",
				DB:       0,
			},
		}

		client := NewRedisClient(cfg)
		assert.NotNil(t, client)
		assert.NotNil(t, client.client)
		client.Close()
	})
}

// TestRedisClient_Set tests the Set method
func TestRedisClient_Set(t *testing.T) {
	t.Run("Sets string value successfully", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		ctx := context.Background()
		err := client.Set(ctx, "test-key", "test-value", time.Minute)

		require.NoError(t, err)
		val, _ := mr.Get("test-key")
		assert.Contains(t, val, "test-value")
	})

	t.Run("Sets struct value successfully", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		type TestData struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		ctx := context.Background()
		data := TestData{Name: "test", Value: 42}
		err := client.Set(ctx, "test-struct", data, time.Minute)

		require.NoError(t, err)

		var result TestData
		err = client.Get(ctx, "test-struct", &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 42, result.Value)
	})

	t.Run("Sets with zero expiration", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		err := client.Set(ctx, "no-expire-key", "value", 0)
		require.NoError(t, err)

		var result string
		err = client.Get(ctx, "no-expire-key", &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result)
	})

	t.Run("Overwrites existing key", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		err := client.Set(ctx, "overwrite-key", "initial", time.Minute)
		require.NoError(t, err)

		err = client.Set(ctx, "overwrite-key", "updated", time.Minute)
		require.NoError(t, err)

		var result string
		err = client.Get(ctx, "overwrite-key", &result)
		require.NoError(t, err)
		assert.Equal(t, "updated", result)
	})

	t.Run("Sets map value successfully", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		data := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}
		err := client.Set(ctx, "test-map", data, time.Minute)
		require.NoError(t, err)

		var result map[string]interface{}
		err = client.Get(ctx, "test-map", &result)
		require.NoError(t, err)
		assert.Equal(t, "value1", result["key1"])
		assert.Equal(t, float64(123), result["key2"]) // JSON unmarshals numbers as float64
		assert.Equal(t, true, result["key3"])
	})

	t.Run("Sets slice value successfully", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		data := []string{"item1", "item2", "item3"}
		err := client.Set(ctx, "test-slice", data, time.Minute)
		require.NoError(t, err)

		var result []string
		err = client.Get(ctx, "test-slice", &result)
		require.NoError(t, err)
		assert.Equal(t, []string{"item1", "item2", "item3"}, result)
	})
}

// TestRedisClient_Get tests the Get method
func TestRedisClient_Get(t *testing.T) {
	t.Run("Gets existing key successfully", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		value := `"test-value"`
		mr.Set("existing-key", value)

		ctx := context.Background()
		var result string
		err := client.Get(ctx, "existing-key", &result)

		require.NoError(t, err)
		assert.Equal(t, "test-value", result)
	})

	t.Run("Returns error for non-existent key", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		var result string
		err := client.Get(ctx, "non-existent-key", &result)

		assert.Error(t, err)
		assert.Equal(t, redis.Nil, err)
	})

	t.Run("Unmarshals complex struct", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		type ComplexData struct {
			ID        string   `json:"id"`
			Count     int      `json:"count"`
			Active    bool     `json:"active"`
			Tags      []string `json:"tags"`
			Timestamp int64    `json:"timestamp"`
		}

		data := ComplexData{
			ID:        "test-id",
			Count:     100,
			Active:    true,
			Tags:      []string{"tag1", "tag2"},
			Timestamp: time.Now().Unix(),
		}

		jsonData, _ := json.Marshal(data)
		mr.Set("complex-key", string(jsonData))

		ctx := context.Background()
		var result ComplexData
		err := client.Get(ctx, "complex-key", &result)

		require.NoError(t, err)
		assert.Equal(t, "test-id", result.ID)
		assert.Equal(t, 100, result.Count)
		assert.True(t, result.Active)
		assert.Equal(t, []string{"tag1", "tag2"}, result.Tags)
	})

	t.Run("Returns error for invalid JSON", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		mr.Set("invalid-json-key", "not-valid-json{")

		ctx := context.Background()
		var result map[string]interface{}
		err := client.Get(ctx, "invalid-json-key", &result)

		assert.Error(t, err)
	})
}

// TestRedisClient_Delete tests the Delete method
func TestRedisClient_Delete(t *testing.T) {
	t.Run("Deletes existing key successfully", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		mr.Set("delete-key", "value")

		ctx := context.Background()
		err := client.Delete(ctx, "delete-key")

		require.NoError(t, err)
		assert.False(t, mr.Exists("delete-key"))
	})

	t.Run("No error when deleting non-existent key", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		err := client.Delete(ctx, "non-existent-key")

		// Redis DEL does not return error for non-existent keys
		assert.NoError(t, err)
	})

	t.Run("Deletes multiple keys one at a time", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		mr.Set("key1", "value1")
		mr.Set("key2", "value2")
		mr.Set("key3", "value3")

		ctx := context.Background()

		err := client.Delete(ctx, "key1")
		require.NoError(t, err)
		assert.False(t, mr.Exists("key1"))
		assert.True(t, mr.Exists("key2"))
		assert.True(t, mr.Exists("key3"))

		err = client.Delete(ctx, "key2")
		require.NoError(t, err)
		assert.False(t, mr.Exists("key2"))
		assert.True(t, mr.Exists("key3"))
	})
}

// TestRedisClient_MGet tests the MGet method
func TestRedisClient_MGet(t *testing.T) {
	t.Run("Gets multiple keys successfully", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		mr.Set("mget-key1", "value1")
		mr.Set("mget-key2", "value2")
		mr.Set("mget-key3", "value3")

		ctx := context.Background()
		results, err := client.MGet(ctx, "mget-key1", "mget-key2", "mget-key3")

		require.NoError(t, err)
		assert.Len(t, results, 3)
		assert.Equal(t, "value1", results[0])
		assert.Equal(t, "value2", results[1])
		assert.Equal(t, "value3", results[2])
	})

	t.Run("Returns nil for missing keys", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		mr.Set("mget-existing", "exists")

		ctx := context.Background()
		results, err := client.MGet(ctx, "mget-existing", "mget-missing")

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "exists", results[0])
		assert.Nil(t, results[1])
	})

	t.Run("Handles empty keys list", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		_, err := client.MGet(ctx)

		// Redis MGET requires at least one key, so an error is expected
		assert.Error(t, err)
	})

	t.Run("Returns all nil for all missing keys", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		results, err := client.MGet(ctx, "missing1", "missing2", "missing3")

		require.NoError(t, err)
		assert.Len(t, results, 3)
		for _, r := range results {
			assert.Nil(t, r)
		}
	})
}

// TestRedisClient_Pipeline tests the Pipeline method
func TestRedisClient_Pipeline(t *testing.T) {
	t.Run("Returns valid pipeliner", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		pipe := client.Pipeline()
		assert.NotNil(t, pipe)
	})

	t.Run("Executes pipeline commands", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		ctx := context.Background()
		pipe := client.Pipeline()

		pipe.Set(ctx, "pipe-key1", "value1", 0)
		pipe.Set(ctx, "pipe-key2", "value2", 0)

		_, err := pipe.Exec(ctx)
		require.NoError(t, err)

		assert.True(t, mr.Exists("pipe-key1"))
		assert.True(t, mr.Exists("pipe-key2"))
	})
}

// TestRedisClient_Client tests the Client method
func TestRedisClient_Client(t *testing.T) {
	t.Run("Returns underlying redis client", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		underlyingClient := client.Client()
		assert.NotNil(t, underlyingClient)
	})
}

// TestRedisClient_Ping tests the Ping method
func TestRedisClient_Ping(t *testing.T) {
	t.Run("Ping succeeds with valid connection", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		err := client.Ping(ctx)

		assert.NoError(t, err)
	})

	t.Run("Ping fails with invalid connection", func(t *testing.T) {
		client := &RedisClient{
			client: redis.NewClient(&redis.Options{
				Addr: "localhost:0", // Invalid address
			}),
		}
		defer client.Close()

		ctx := context.Background()
		err := client.Ping(ctx)

		assert.Error(t, err)
	})
}

// TestRedisClient_Close tests the Close method
func TestRedisClient_Close(t *testing.T) {
	t.Run("Closes client successfully", func(t *testing.T) {
		mr, _ := miniredis.Run()
		defer mr.Close()

		client := &RedisClient{
			client: redis.NewClient(&redis.Options{
				Addr: mr.Addr(),
			}),
		}

		err := client.Close()
		assert.NoError(t, err)

		// Operations should fail after close
		ctx := context.Background()
		err = client.Ping(ctx)
		assert.Error(t, err)
	})
}

// TestRedisClient_ExpirationBehavior tests TTL/expiration behavior
func TestRedisClient_ExpirationBehavior(t *testing.T) {
	t.Run("Key expires after TTL", func(t *testing.T) {
		mr, client := setupMiniRedis(t)

		ctx := context.Background()
		err := client.Set(ctx, "expiring-key", "value", time.Second)
		require.NoError(t, err)

		// Key should exist initially
		assert.True(t, mr.Exists("expiring-key"))

		// Fast-forward time in miniredis
		mr.FastForward(2 * time.Second)

		// Key should be expired
		assert.False(t, mr.Exists("expiring-key"))
	})
}

// TestRedisClient_ContextCancellation tests context cancellation behavior
func TestRedisClient_ContextCancellation(t *testing.T) {
	t.Run("Respects cancelled context", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := client.Set(ctx, "test-key", "value", time.Minute)
		assert.Error(t, err)
	})
}

// TestRedisClient_Integration tests against real Redis if available
func TestRedisClient_Integration(t *testing.T) {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisPassword := os.Getenv("REDIS_PASSWORD")

	if redisHost == "" || redisPort == "" {
		t.Skip("Skipping integration test: REDIS_HOST and REDIS_PORT not set")
	}

	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     redisHost,
			Port:     redisPort,
			Password: redisPassword,
			DB:       0,
		},
	}

	client := NewRedisClient(cfg)
	defer client.Close()

	t.Run("Ping real Redis", func(t *testing.T) {
		ctx := context.Background()
		err := client.Ping(ctx)
		require.NoError(t, err)
	})

	t.Run("Set and Get on real Redis", func(t *testing.T) {
		ctx := context.Background()
		testKey := "integration-test-key-" + time.Now().Format("20060102150405")

		type TestData struct {
			Message string `json:"message"`
		}

		data := TestData{Message: "integration test"}
		err := client.Set(ctx, testKey, data, time.Minute)
		require.NoError(t, err)

		var result TestData
		err = client.Get(ctx, testKey, &result)
		require.NoError(t, err)
		assert.Equal(t, "integration test", result.Message)

		// Cleanup
		err = client.Delete(ctx, testKey)
		require.NoError(t, err)
	})
}

// TestRedisClient_EdgeCases tests edge cases and special scenarios
func TestRedisClient_EdgeCases(t *testing.T) {
	t.Run("Handles empty string value", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		ctx := context.Background()
		err := client.Set(ctx, "empty-value-key", "", time.Minute)
		require.NoError(t, err)

		var result string
		err = client.Get(ctx, "empty-value-key", &result)
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("Handles very long key", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		longKey := ""
		for i := 0; i < 1000; i++ {
			longKey += "a"
		}

		ctx := context.Background()
		err := client.Set(ctx, longKey, "value", time.Minute)
		require.NoError(t, err)

		var result string
		err = client.Get(ctx, longKey, &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result)
	})

	t.Run("Handles special characters in key", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		specialKey := "key:with:colons/slashes:and-dashes"

		ctx := context.Background()
		err := client.Set(ctx, specialKey, "special-value", time.Minute)
		require.NoError(t, err)

		var result string
		err = client.Get(ctx, specialKey, &result)
		require.NoError(t, err)
		assert.Equal(t, "special-value", result)
	})

	t.Run("Handles null value in struct", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		type DataWithNull struct {
			Name    string  `json:"name"`
			OptInt  *int    `json:"opt_int,omitempty"`
			OptStr  *string `json:"opt_str,omitempty"`
		}

		ctx := context.Background()
		data := DataWithNull{Name: "test", OptInt: nil, OptStr: nil}
		err := client.Set(ctx, "null-fields-key", data, time.Minute)
		require.NoError(t, err)

		var result DataWithNull
		err = client.Get(ctx, "null-fields-key", &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Nil(t, result.OptInt)
		assert.Nil(t, result.OptStr)
	})

	t.Run("Handles nested structs", func(t *testing.T) {
		_, client := setupMiniRedis(t)

		type Inner struct {
			Value string `json:"value"`
		}
		type Outer struct {
			Name  string `json:"name"`
			Inner Inner  `json:"inner"`
		}

		ctx := context.Background()
		data := Outer{
			Name:  "outer",
			Inner: Inner{Value: "inner-value"},
		}
		err := client.Set(ctx, "nested-key", data, time.Minute)
		require.NoError(t, err)

		var result Outer
		err = client.Get(ctx, "nested-key", &result)
		require.NoError(t, err)
		assert.Equal(t, "outer", result.Name)
		assert.Equal(t, "inner-value", result.Inner.Value)
	})
}
