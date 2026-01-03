# Middleware Package

The middleware package provides HTTP middleware components for the Gin web framework.

## Overview

This package includes middleware for:
- Authentication (JWT)
- Rate limiting
- CORS handling
- Request validation
- Logging and metrics
- Error handling

## Components

### Authentication Middleware

JWT-based authentication:

```go
router.Use(middleware.AuthMiddleware(cfg))
```

Validates JWT tokens in the `Authorization` header:
```
Authorization: Bearer <jwt-token>
```

### Rate Limiting

Configurable rate limiting per user/IP:

```go
router.Use(middleware.RateLimitMiddleware(cfg))
```

Configuration:
```yaml
rate_limit:
  enabled: true
  requests_per_minute: 60
  burst: 10
```

### CORS

Cross-Origin Resource Sharing configuration:

```go
router.Use(middleware.CORSMiddleware(cfg))
```

Configuration:
```yaml
cors:
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE"]
  allowed_headers: ["Authorization", "Content-Type"]
  max_age: 3600
```

### Request Validation

Validates request bodies against schemas:

```go
router.POST("/api/completions",
    middleware.ValidateRequest(&CompletionRequest{}),
    handler.Complete,
)
```

### Logging Middleware

Structured request logging:

```go
router.Use(middleware.LoggingMiddleware(logger))
```

Logs:
- Request method and path
- Response status and duration
- Request ID for tracing

### Error Handling

Unified error response handling:

```go
router.Use(middleware.ErrorMiddleware())
```

## Middleware Chain

Recommended middleware order:

```go
router.Use(
    middleware.RecoveryMiddleware(),
    middleware.LoggingMiddleware(logger),
    middleware.CORSMiddleware(cfg),
    middleware.RateLimitMiddleware(cfg),
    middleware.AuthMiddleware(cfg),
)
```

## Custom Middleware

Create custom middleware:

```go
func CustomMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Before request
        c.Next()
        // After request
    }
}
```
