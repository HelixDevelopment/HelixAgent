# Architecture Documentation

## Overview

The go-elder-plinius project is a multi-module Go workspace that provides
production-ready gRPC clients for Python services in the elder-plinius
GitHub organization. It follows a layered architecture pattern with clear
separation of concerns.

## Design Principles

1. **Modularity** -- Each service is a separate Go module with its own lifecycle
2. **Reusability** -- Common infrastructure is extracted into `go-plinius-common`
3. **Type Safety** -- Rich Go types with validation for all service inputs/outputs
4. **Resilience** -- Built-in retry, backoff, health checking, and graceful degradation
5. **Configurability** -- Three-tier configuration: programmatic, env vars, YAML files
6. **Testability** -- Comprehensive unit tests with table-driven patterns
7. **Documentation** -- Extensive Go doc comments and README guides

## Layer Architecture

```
+-----------------------------------------------------------+
|                    Application Layer                       |
|         (Your Go application using the clients)            |
+-----------------------------------------------------------+
|                                                           |
|  +-------------+  +-------------+  +-------------+       |
|  | go-autotemp |  |go-obliteratus|  |   go-eos    |       |
|  |   Client    |  |   Client    |  |   Client    |       |
|  +------+------+  +------+------+  +------+------+       |
|  +-------------+  +-------------+  +-------------+       |
|  | go-almeche  |                                  |       |
|  |   Client    |                                  |       |
|  +------+------+                                  |       |
+-----------------------------------------------------------+
|                    Service Client Layer                    |
|  (Types, validation, request building, response parsing)  |
+-----------------------------------------------------------+
|                                                           |
|              go-plinius-common                            |
|  +----------+  +----------+  +----------+  +---------+   |
|  |  config  |  |  errors  |  |grpcclient|  |  types  |   |
|  | (pkg/config)| (pkg/errors)| (pkg/grpc)| (pkg/types)|   |
|  +----------+  +----------+  +----------+  +---------+   |
+-----------------------------------------------------------+
|                    Infrastructure Layer                    |
|       (gRPC, TLS, keepalive, compression, metadata)       |
+-----------------------------------------------------------+
|                    Transport Layer                         |
|              (HTTP/2, TCP, Unix Domain Sockets)           |
+-----------------------------------------------------------+
|              Python gRPC Server ( counterpart )           |
+-----------------------------------------------------------+
```

## Common Module (go-plinius-common)

### Configuration (`pkg/config`)

The configuration system uses the functional options pattern:

```go
type Option func(*Config)

func New(serviceName string, opts ...Option) *Config
```

This allows flexible configuration:

```go
// Minimal
cfg := config.New("autotemp")

// With options
cfg := config.New("autotemp",
    config.WithAddress("host:port"),
    config.WithTimeout(30*time.Second),
)

// From environment
cfg := config.FromEnv("autotemp")

// From YAML
cfg, err := config.FromFile("config.yaml", "autotemp")
```

Configuration precedence (highest first):
1. Programmatic options passed to `New()`
2. Environment variables
3. YAML file values
4. Built-in defaults

### Error Handling (`pkg/errors`)

Structured errors with typed error codes:

```go
type PliniusError struct {
    Code      ErrorCode
    Message   string
    Service   string
    Retryable bool
    Details   map[string]interface{}
    cause     error
}
```

Error codes map to gRPC status codes and include retry hints:
- `UNAVAILABLE`, `TIMEOUT`, `CONNECTION_ERROR` -- retryable
- `INVALID_ARGUMENT`, `NOT_FOUND`, `PERMISSION_DENIED` -- non-retryable
- `INTERNAL`, `UNKNOWN` -- conservative retry (may be transient)

### gRPC Client (`pkg/grpcclient`)

The `grpcclient.Client` wraps the standard gRPC client with:

- **Connection management** -- Lazy connection, reconnection, state tracking
- **Retry logic** -- Configurable max retries with exponential backoff and jitter
- **Authentication** -- Bearer token injection via metadata
- **Compression** -- Optional gzip compression
- **Keepalive** -- Configurable keepalive pings
- **Message size limits** -- Configurable send/receive limits
- **TLS support** -- Certificate-based encryption
- **Unary RPC helper** -- `InvokeUnary` with built-in retry

### Shared Types (`pkg/types`)

Common types used across all services:

- `ServiceClient` -- Common interface for all service clients
- `HealthStatus` / `HealthCheck` -- Health checking
- `Pagination` -- Pagination parameters
- `VersionInfo` -- Service version information
- `TokenUsage` -- LLM token consumption tracking
- `ScoreBreakdown` -- Multi-dimensional scoring
- `ProgressUpdate` / `ProgressCallback` -- Streaming progress
- `PipelineResult` / `StageResult` -- Multi-stage pipeline results

## Service Module Pattern

Each service module follows a consistent structure:

### Protocol Buffer Definition (`proto/*.proto`)

Defines the gRPC service interface and message types. Each service defines:
- Service methods with request/response messages
- Streaming methods for long-running operations
- Health and version endpoints
- Service-specific message types

### Types Package (`pkg/types/*.go`)

Idiomatic Go types mirroring the protobuf messages:

```go
// Options type with validation and defaults
type RunOptions struct { ... }
func (o *RunOptions) Validate() error
func (o *RunOptions) Defaults()

// Result types
 type RunResult struct { ... }
```

### Client Package (`pkg/client/*.go`)

The service client implements the `ServiceClient` interface:

```go
type Client struct {
    grpc   *grpcclient.Client
    proto  xxxproto.XXXServiceClient
    cfg    *config.Config
}

func (c *Client) Connect(ctx context.Context) error
func (c *Client) Close() error
func (c *Client) Health(ctx context.Context) (*HealthStatus, error)
func (c *Client) IsConnected() bool
```

Plus service-specific methods with proper validation, error wrapping,
and response conversion.

### Conversion Helpers

Each client includes `protoTo*()` helper functions to convert between
protobuf and Go types, handling nil pointers and type conversions safely.

## Communication Patterns

### Unary RPC

For simple request/response operations:

```
Client -> Validate Options -> Build Request -> gRPC Unary -> Parse Response
```

### Streaming RPC

For long-running operations with progress tracking:

```
Client -> Validate Options -> Open Stream -> [Recv Progress]* -> Final Result
                                       |                    |
                                       +---> ProgressCallback +----> Result
```

### Health Checking

```
Client -> Health() -> gRPC -> Parse Status -> IsHealthy()
```

## Retry Behavior

Retry is triggered for:
- `UNAVAILABLE` status
- `DEADLINE_EXCEEDED` (timeout)
- `ABORTED` status
- Connection errors

Retry is NOT triggered for:
- `INVALID_ARGUMENT`
- `NOT_FOUND`
- `PERMISSION_DENIED`
- `UNAUTHENTICATED`
- `UNIMPLEMENTED`

Retry strategy: Exponential backoff with jitter:
```
delay = base * 2^(attempt-1) +/- 25% jitter
```

## Future Enhancements

1. **Proto code generation** -- Makefile target for `protoc` generation
2. **Integration tests** -- Docker-compose based integration test suite
3. **Metrics** -- OpenTelemetry integration for request metrics
4. **Caching** -- Optional response caching for idempotent operations
5. **Circuit breaker** -- Circuit breaker pattern for failing services
6. **Load balancing** -- Client-side load balancing for multiple backends
7. **Authentication refresh** -- Automatic token refresh for long-lived clients
