# Cache Package

The cache package provides comprehensive caching functionality for SuperAgent using Redis as the backend.

## Overview

This package implements a caching layer that:
- Caches LLM responses to reduce API costs and latency
- Stores session data for stateful conversations
- Provides user-scoped cache management
- Supports automatic expiration with configurable TTL

## Components

### CacheService

Main service that provides caching operations:

```go
service, err := cache.NewCacheService(cfg)
```

#### Key Methods

- `CacheResponse(ctx, key, response, ttl)` - Cache an LLM response
- `GetCachedResponse(ctx, key)` - Retrieve cached response
- `InvalidateUserCache(ctx, userID)` - Invalidate all cache entries for a user
- `InvalidateByPattern(ctx, pattern)` - Invalidate cache entries matching a pattern

### RedisClient

Low-level Redis client wrapper:

```go
client := cache.NewRedisClient(cfg)
```

## Configuration

Configure caching in your config file:

```yaml
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0

cache:
  enabled: true
  default_ttl: "30m"
```

## Usage Example

```go
// Create cache service
cacheService, err := cache.NewCacheService(cfg)
if err != nil {
    // Caching disabled, but service is still usable
    log.Printf("Cache warning: %v", err)
}

// Cache a response
key := cache.CacheKey{
    Type:     "llm_response",
    Provider: "openai",
    UserID:   "user123",
}
err = cacheService.CacheResponse(ctx, key, response, time.Hour)

// Retrieve cached response
cached, err := cacheService.GetCachedResponse(ctx, key)
```

## Cache Key Types

- `llm_response` - Cached LLM completions
- `session` - User session data
- `provider_models` - Provider model listings
- `verification` - Model verification results

## Graceful Degradation

If Redis is unavailable, the cache service operates in a degraded mode where:
- All cache operations return immediately without errors
- The application continues to function normally
- Requests hit the LLM providers directly
