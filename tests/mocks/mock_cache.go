package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockRedisClient implements a mock Redis client for testing
type MockRedisClient struct {
	mock.Mock
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{}
}

func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	args := m.Called(ctx, keys)
	return args.Error(0)
}

func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	args := m.Called(ctx, keys)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	args := m.Called(ctx, key, expiration)
	return args.Bool(0), args.Error(1)
}

func (m *MockRedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(time.Duration), args.Error(1)
}

func (m *MockRedisClient) Incr(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRedisClient) Decr(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	args := m.Called(ctx, key, field)
	return args.String(0), args.Error(1)
}

func (m *MockRedisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	args := m.Called(ctx, key, values)
	return args.Error(0)
}

func (m *MockRedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockRedisClient) HDel(ctx context.Context, key string, fields ...string) error {
	args := m.Called(ctx, key, fields)
	return args.Error(0)
}

func (m *MockRedisClient) LPush(ctx context.Context, key string, values ...interface{}) error {
	args := m.Called(ctx, key, values)
	return args.Error(0)
}

func (m *MockRedisClient) RPush(ctx context.Context, key string, values ...interface{}) error {
	args := m.Called(ctx, key, values)
	return args.Error(0)
}

func (m *MockRedisClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := m.Called(ctx, key, start, stop)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRedisClient) LLen(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRedisClient) SAdd(ctx context.Context, key string, members ...interface{}) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

func (m *MockRedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRedisClient) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	args := m.Called(ctx, key, member)
	return args.Bool(0), args.Error(1)
}

func (m *MockRedisClient) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// InMemoryCache provides a simple in-memory cache for testing without Redis
type InMemoryCache struct {
	mu    sync.RWMutex
	data  map[string]cacheEntry
	lists map[string][]string
	sets  map[string]map[string]struct{}
	hash  map[string]map[string]string
}

type cacheEntry struct {
	value      string
	expiration time.Time
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		data:  make(map[string]cacheEntry),
		lists: make(map[string][]string),
		sets:  make(map[string]map[string]struct{}),
		hash:  make(map[string]map[string]string),
	}
}

func (c *InMemoryCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.data[key]
	if !ok {
		return "", nil // Key not found
	}
	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		return "", nil // Expired
	}
	return entry.value, nil
}

func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exp time.Time
	if expiration > 0 {
		exp = time.Now().Add(expiration)
	}
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case []byte:
		strValue = string(v)
	default:
		strValue = ""
	}
	c.data[key] = cacheEntry{value: strValue, expiration: exp}
	return nil
}

func (c *InMemoryCache) Del(ctx context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		delete(c.data, key)
		delete(c.lists, key)
		delete(c.sets, key)
		delete(c.hash, key)
	}
	return nil
}

func (c *InMemoryCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var count int64
	for _, key := range keys {
		if _, ok := c.data[key]; ok {
			count++
		}
	}
	return count, nil
}

func (c *InMemoryCache) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.data[key]; ok {
		entry.expiration = time.Now().Add(expiration)
		c.data[key] = entry
		return true, nil
	}
	return false, nil
}

func (c *InMemoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if entry, ok := c.data[key]; ok {
		if entry.expiration.IsZero() {
			return -1, nil // No expiration
		}
		ttl := time.Until(entry.expiration)
		if ttl < 0 {
			return -2, nil // Expired
		}
		return ttl, nil
	}
	return -2, nil // Key doesn't exist
}

func (c *InMemoryCache) Incr(ctx context.Context, key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Simple implementation - for testing
	return 1, nil
}

func (c *InMemoryCache) Decr(ctx context.Context, key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return 0, nil
}

func (c *InMemoryCache) HGet(ctx context.Context, key, field string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if h, ok := c.hash[key]; ok {
		return h[field], nil
	}
	return "", nil
}

func (c *InMemoryCache) HSet(ctx context.Context, key string, values ...interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hash[key] == nil {
		c.hash[key] = make(map[string]string)
	}
	for i := 0; i < len(values)-1; i += 2 {
		field, _ := values[i].(string)
		val, _ := values[i+1].(string)
		c.hash[key][field] = val
	}
	return nil
}

func (c *InMemoryCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if h, ok := c.hash[key]; ok {
		result := make(map[string]string)
		for k, v := range h {
			result[k] = v
		}
		return result, nil
	}
	return nil, nil
}

func (c *InMemoryCache) HDel(ctx context.Context, key string, fields ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if h, ok := c.hash[key]; ok {
		for _, field := range fields {
			delete(h, field)
		}
	}
	return nil
}

func (c *InMemoryCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range values {
		if str, ok := v.(string); ok {
			c.lists[key] = append([]string{str}, c.lists[key]...)
		}
	}
	return nil
}

func (c *InMemoryCache) RPush(ctx context.Context, key string, values ...interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range values {
		if str, ok := v.(string); ok {
			c.lists[key] = append(c.lists[key], str)
		}
	}
	return nil
}

func (c *InMemoryCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if list, ok := c.lists[key]; ok {
		// Handle negative indices
		length := int64(len(list))
		if start < 0 {
			start = length + start
		}
		if stop < 0 {
			stop = length + stop
		}
		if start < 0 {
			start = 0
		}
		if stop >= length {
			stop = length - 1
		}
		if start > stop {
			return []string{}, nil
		}
		return list[start : stop+1], nil
	}
	return []string{}, nil
}

func (c *InMemoryCache) LLen(ctx context.Context, key string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return int64(len(c.lists[key])), nil
}

func (c *InMemoryCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sets[key] == nil {
		c.sets[key] = make(map[string]struct{})
	}
	for _, m := range members {
		if str, ok := m.(string); ok {
			c.sets[key][str] = struct{}{}
		}
	}
	return nil
}

func (c *InMemoryCache) SMembers(ctx context.Context, key string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if set, ok := c.sets[key]; ok {
		result := make([]string, 0, len(set))
		for k := range set {
			result = append(result, k)
		}
		return result, nil
	}
	return []string{}, nil
}

func (c *InMemoryCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if set, ok := c.sets[key]; ok {
		if str, ok := member.(string); ok {
			_, exists := set[str]
			return exists, nil
		}
	}
	return false, nil
}

func (c *InMemoryCache) Ping(ctx context.Context) error {
	return nil
}

func (c *InMemoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]cacheEntry)
	c.lists = make(map[string][]string)
	c.sets = make(map[string]map[string]struct{})
	c.hash = make(map[string]map[string]string)
	return nil
}

// Clear clears all data from the cache
func (c *InMemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]cacheEntry)
	c.lists = make(map[string][]string)
	c.sets = make(map[string]map[string]struct{})
	c.hash = make(map[string]map[string]string)
}
