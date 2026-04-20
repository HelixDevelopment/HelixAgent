# go-plinius-common

[![Go Reference](https://pkg.go.dev/badge/github.com/elder-plinius/go-plinius-common.svg)](https://pkg.go.dev/github.com/elder-plinius/go-plinius-common)
[![Go Report Card](https://goreportcard.com/badge/github.com/elder-plinius/go-plinius-common)](https://goreportcard.com/report/github.com/elder-plinius/go-plinius-common)

Common infrastructure for all Plinius Go service clients. This shared module provides
unified configuration management, structured errors, gRPC client foundations, and
shared types used across the entire go-elder-plinius ecosystem.

## Purpose

The `go-plinius-common` module eliminates duplication and ensures consistency across
all Plinius service client libraries. It handles the cross-cutting concerns that every
service client needs:

- **Configuration**: Environment variables, YAML files, and programmatic configuration
- **Error Handling**: Structured errors with codes, retry hints, and causal chains
- **gRPC Infrastructure**: Connection management, retry logic, authentication, compression
- **Shared Types**: Health status, pagination, token usage, score breakdowns, progress tracking

## Installation

```bash
go get github.com/elder-plinius/go-plinius-common
```

## Packages

### `pkg/config` - Configuration Management

Centralized configuration with support for environment variables, YAML files, and
functional options pattern.

```go
import "github.com/elder-plinius/go-plinius-common/pkg/config"

// Programmatic configuration
cfg := config.New("autotemp",
    config.WithAddress("localhost:50051"),
    config.WithTimeout(30 * time.Second),
    config.WithMaxRetries(3),
    config.WithAuthToken("your-token"),
)

// From environment variables (AUTOTEMP_ADDRESS, AUTOTEMP_TIMEOUT, etc.)
cfg := config.FromEnv("autotemp")

// From YAML file
cfg, err := config.FromFile("/etc/plinius/config.yaml", "autotemp")

// Validate
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}
```

### `pkg/errors` - Structured Error Handling

Typed errors with codes, retry hints, and causal chains compatible with Go 1.13+ error wrapping.

```go
import pliniuserr "github.com/elder-plinius/go-plinius-common/pkg/errors"

// Create error
err := pliniuserr.New(pliniuserr.ErrCodeUnavailable, "autotemp", "service is down")

// Wrap with cause
err := pliniuserr.Wrap(pliniuserr.ErrCodeConnection, "autotemp", "request failed", cause)

// Check error code
if pliniuserr.Is(err, pliniuserr.ErrCodeUnavailable) {
    // Handle unavailable
}

// Check if retryable
if pliniuserr.IsRetryableError(err) {
    // Retry with backoff
}
```

### `pkg/grpcclient` - gRPC Client Infrastructure

Reusable gRPC client with connection management, retry logic, and interceptors.

```go
import "github.com/elder-plinius/go-plinius-common/pkg/grpcclient"

cfg := config.New("autotemp", config.WithAddress("localhost:50051"))
client := grpcclient.New(cfg)

ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := client.Connect(ctx); err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### `pkg/types` - Shared Data Types

Common types used across all Plinius services.

```go
import "github.com/elder-plinius/go-plinius-common/pkg/types"

// Health check
status, err := client.Health(ctx)
if status.IsHealthy() {
    fmt.Println("Service is healthy")
}

// Token usage tracking
usage := types.TokenUsage{
    PromptTokens:     100,
    CompletionTokens: 200,
    TotalTokens:      300,
}
```

## Configuration Reference

All Plinius services use the same configuration pattern with service-specific prefixes:

| Variable | Default | Description |
|----------|---------|-------------|
| `{SERVICE}_ADDRESS` | `localhost:50051` | gRPC server address |
| `{SERVICE}_TIMEOUT` | `30s` | RPC timeout |
| `{SERVICE}_CONNECTION_TIMEOUT` | `10s` | Connection timeout |
| `{SERVICE}_MAX_RETRIES` | `3` | Max retry attempts |
| `{SERVICE}_RETRY_BACKOFF` | `1s` | Base retry backoff |
| `{SERVICE}_MAX_RETRY_BACKOFF` | `30s` | Max retry backoff |
| `{SERVICE}_ENABLE_TLS` | `false` | Enable TLS |
| `{SERVICE}_TLS_CERT_PATH` | - | TLS certificate path |
| `{SERVICE}_AUTH_TOKEN` | - | Bearer token |
| `{SERVICE}_COMPRESSION` | - | Compression: gzip, snappy |

## Testing

```bash
cd go-plinius-common
go test ./... -v
```

## License

Apache-2.0 - See [LICENSE](LICENSE)
