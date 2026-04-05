package servers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/testutil"
)

func TestNewRedisAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.Equal(t, "localhost", adapter.config.Host)
	assert.Equal(t, 6379, adapter.config.Port)
}

func TestDefaultRedisAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 6379, config.Port)
	assert.Equal(t, 0, config.Database)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 5*1000000000, int(config.DialTimeout))
	assert.Equal(t, 3*1000000000, int(config.ReadTimeout))
	assert.Equal(t, 3*1000000000, int(config.WriteTimeout))
	assert.False(t, config.ReadOnly)
	assert.Equal(t, 3, config.MaxRetries)
}

func TestNewRedisAdapter_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := RedisAdapterConfig{}
	adapter := NewRedisAdapter(config, logrus.New())

	assert.Equal(t, "localhost", adapter.config.Host)
	assert.Equal(t, 6379, adapter.config.Port)
	assert.Equal(t, 10, adapter.config.PoolSize)
}

func TestRedisAdapter_Health_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Get_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Get(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Set_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	err := adapter.Set(context.Background(), "key", "value", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Delete_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Delete(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Exists_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Exists(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Keys_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Keys(context.Background(), "*")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Expire_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Expire(context.Background(), "key", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_TTL_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.TTL(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_HSet_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	err := adapter.HSet(context.Background(), "key", map[string]interface{}{"field": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_HGet_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.HGet(context.Background(), "key", "field")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_HGetAll_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.HGetAll(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_LPush_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.LPush(context.Background(), "key", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_LRange_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.LRange(context.Background(), "key", 0, -1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_SAdd_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.SAdd(context.Background(), "key", "member")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_SMembers_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.SMembers(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Incr_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Incr(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Decr_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Decr(context.Background(), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Info_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.Info(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_DBSize_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.DBSize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_Close(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())
	adapter.initialized = true

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestRedisAdapter_Close_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestRedisAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 14)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "redis_get")
	assert.Contains(t, toolNames, "redis_set")
	assert.Contains(t, toolNames, "redis_delete")
	assert.Contains(t, toolNames, "redis_exists")
	assert.Contains(t, toolNames, "redis_keys")
	assert.Contains(t, toolNames, "redis_hset")
	assert.Contains(t, toolNames, "redis_hgetall")
	assert.Contains(t, toolNames, "redis_lpush")
	assert.Contains(t, toolNames, "redis_lrange")
	assert.Contains(t, toolNames, "redis_sadd")
	assert.Contains(t, toolNames, "redis_smembers")
	assert.Contains(t, toolNames, "redis_incr")
	assert.Contains(t, toolNames, "redis_info")
	assert.Contains(t, toolNames, "redis_dbsize")
}

func TestRedisAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "redis_get", map[string]interface{}{
		"key": "test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRedisAdapter_ExecuteTool_UnknownTool(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())
	adapter.initialized = true

	_, err := adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestRedisAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	config.KeyPrefix = "test:"
	adapter := NewRedisAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, "redis", caps["name"])
	assert.Equal(t, "localhost", caps["host"])
	assert.Equal(t, 6379, caps["port"])
	assert.Equal(t, 0, caps["database"])
	assert.Equal(t, false, caps["read_only"])
	assert.Equal(t, "test:", caps["key_prefix"])
	assert.Equal(t, 14, caps["tools"])
	assert.Equal(t, false, caps["initialized"])
}

func TestRedisAdapter_prefixKey(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	// No prefix
	assert.Equal(t, "mykey", adapter.prefixKey("mykey"))

	// With prefix
	adapter.config.KeyPrefix = "app:"
	assert.Equal(t, "app:mykey", adapter.prefixKey("mykey"))
}

func TestRedisAdapter_MarshalJSON(t *testing.T) {
	t.Parallel()
	config := DefaultRedisAdapterConfig()
	adapter := NewRedisAdapter(config, logrus.New())

	data, err := adapter.MarshalJSON()
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.Contains(t, result, "initialized")
	assert.Contains(t, result, "capabilities")
}

// Integration tests that require a running Redis instance
// These tests are skipped by default
func TestRedisAdapter_Integration(t *testing.T) {
	t.Parallel()
	testutil.RequireRedis(t)

	config := RedisAdapterConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		Database: 15, // Use a separate database for testing
	}

	adapter := NewRedisAdapter(config, logrus.New())

	// Initialize
	err := adapter.Initialize(context.Background())
	if err != nil {
		t.Skipf("Could not connect to Redis: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	// Test Set and Get
	err = adapter.Set(context.Background(), "test:key1", "value1", 0)
	assert.NoError(t, err)

	val, err := adapter.Get(context.Background(), "test:key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)

	// Test Delete
	deleted, err := adapter.Delete(context.Background(), "test:key1")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Test Exists
	count, err := adapter.Exists(context.Background(), "test:key1")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Test Hash
	err = adapter.HSet(context.Background(), "test:hash1", map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	})
	assert.NoError(t, err)

	hashVal, err := adapter.HGetAll(context.Background(), "test:hash1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", hashVal["field1"])
	assert.Equal(t, "value2", hashVal["field2"])

	// Clean up
	_, _ = adapter.Delete(context.Background(), "test:hash1")

	// Test Info
	info, err := adapter.Info(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, info.Server)

	// Test Health
	err = adapter.Health(context.Background())
	assert.NoError(t, err)
}
