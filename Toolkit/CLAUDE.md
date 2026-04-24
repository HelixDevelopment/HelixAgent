# CLAUDE.md - Toolkit Module


## Definition of Done

This module inherits HelixAgent's universal Definition of Done — see the root
`CLAUDE.md` and `docs/development/definition-of-done.md`. In one line: **no
task is done without pasted output from a real run of the real system in the
same session as the change.** Coverage and green suites are not evidence.

### Acceptance demo for this module

<!-- TODO: replace this block with the exact command(s) that exercise this
     module end-to-end against real dependencies, and the expected output.
     The commands must run the real artifact (built binary, deployed
     container, real service) — no in-process fakes, no mocks, no
     `httptest.NewServer`, no Robolectric, no JSDOM as proof of done. -->

```bash
# TODO
```

## Overview

`github.com/HelixDevelopment/HelixAgent/Toolkit` is a Go library for building AI-powered applications
with multi-provider support, specialized agents, and common infrastructure utilities. It provides
unified interfaces for chat completion, embeddings, reranking, and model discovery across multiple
AI providers, plus reusable packages for authentication, configuration, HTTP clients, rate limiting,
error handling, response parsing, and testing.

**Module**: `github.com/HelixDevelopment/HelixAgent/Toolkit` (Go 1.24+)

## Build & Test

```bash
make build                # Build all packages
make build-cli            # Build the CLI tool (bin/toolkit)
make build-all            # Build for linux/amd64, darwin/amd64, darwin/arm64, windows/amd64
make test                 # Run all tests
make test-unit            # Unit tests only (-short)
make test-integration     # Integration tests (tests/integration/)
make test-e2e             # End-to-end tests (tests/e2e/)
make test-performance     # Benchmarks (tests/performance/)
make test-security        # Security tests (tests/security/)
make test-chaos           # Chaos tests (tests/chaos/)
make test-coverage        # Coverage with HTML report
make test-fuzz            # Fuzz tests (10s)
make fmt                  # go fmt
make vet                  # go vet
make lint                 # golangci-lint
make security-scan        # gosec
make bench                # Benchmarks with memory profiling
```

Single test: `go test -v -run TestName ./path/to/package`

## Package Structure

| Package | Purpose |
|---------|---------|
| `cmd/toolkit` | CLI entry point: test, chat, agent, version commands (cobra) |
| `pkg/toolkit` | Core types: `Toolkit`, `Provider`, `Agent` interfaces, request/response types, provider factory registry |
| `pkg/toolkit/agents` | Built-in agents: `GenericAgent`, `CodeReviewAgent` |
| `pkg/toolkit/common/discovery` | Model discovery service with caching, filtering, sorting |
| `pkg/toolkit/common/http` | HTTP client with retry logic, exponential backoff, auth headers |
| `pkg/toolkit/common/ratelimit` | Token bucket, sliding window, per-key limiters, circuit breaker, HTTP middleware |
| `Commons/auth` | Authentication management: API key, OAuth2 token refresh, HTTP interceptor/middleware |
| `Commons/config` | Configuration map with typed getters, validation rules, env var loading, `ProviderConfig` |
| `Commons/discovery` | Generic model discovery framework: capability/category inference, model formatting |
| `Commons/errors` | Standardized error types: `ProviderError`, `APIError`, `RateLimitError`, `AuthenticationError`, `NetworkError`, `TimeoutError`, `ValidationError` with retryability checks |
| `Commons/http` | Advanced HTTP client with rate limiting, request/response interceptors, retry with backoff |
| `Commons/ratelimit` | Token bucket rate limiter (simple variant used by `Commons/http`) |
| `Commons/response` | Response parsing: JSON, SSE streaming, pagination, chunked, error detection, validation |
| `Commons/testing` | Test utilities: `MockHTTPClient`, `MockProvider`, `TestServer`, `TestFixtures`, assertion helpers |
| `Providers/Chutes` | Chutes provider: client, config builder, model discovery |
| `Providers/SiliconFlow` | SiliconFlow provider: client, config builder, model discovery |
| `tests/integration` | Integration test framework and provider tests |
| `tests/e2e` | End-to-end tests |
| `tests/performance` | Benchmark tests |
| `tests/security` | Security tests |
| `tests/chaos` | Chaos engineering tests |

## Key Types

### Core Interfaces (`pkg/toolkit`)

- `Provider` -- AI provider contract: `Name()`, `Chat()`, `Embed()`, `Rerank()`, `DiscoverModels()`, `ValidateConfig()`
- `Agent` -- AI agent contract: `Name()`, `Execute()`, `ValidateConfig()`, `Capabilities()`
- `ProviderFactory` -- Function type `func(config map[string]interface{}) (Provider, error)`
- `ProviderFactoryRegistry` -- Registry for provider factories with `Register()`, `Create()`, `ListProviders()`

### Request/Response Types (`pkg/toolkit`)

- `ChatRequest` / `ChatResponse` -- Chat completion with messages, model, temperature, max tokens, stop, penalties, logit bias
- `EmbeddingRequest` / `EmbeddingResponse` -- Text embeddings with encoding format and dimensions
- `RerankRequest` / `RerankResponse` -- Document reranking with query, documents, top-N
- `ModelInfo` / `ModelCapabilities` / `ModelCategory` -- Model metadata and capability flags (chat, embedding, rerank, audio, video, vision, function calling, context window)
- `Message` / `Choice` / `Usage` / `Embedding` / `RerankData` / `RerankResult` -- Supporting types

### Agents (`pkg/toolkit/agents`)

- `GenericAgent` -- General-purpose AI assistant with configurable model, temperature, max tokens
- `CodeReviewAgent` -- Specialized code review agent analyzing security, performance, maintainability, best practices, bugs

### Rate Limiting (`pkg/toolkit/common/ratelimit`)

- `TokenBucket` / `TokenBucketConfig` -- Token bucket rate limiter with `Allow()` and `Wait(ctx)`
- `SlidingWindowLimiter` -- Sliding window rate limiter
- `PerKeyLimiter` -- Per-key rate limiting (per IP, per user) with cleanup
- `CircuitBreaker` / `CircuitBreakerConfig` -- Circuit breaker with closed/open/half-open states
- `RateLimiter` -- Common interface: `Allow() bool`, `Wait(ctx) error`
- `Middleware` -- HTTP middleware for rate limiting with `Handler()` and `WaitHandler()`

### Authentication (`Commons/auth`)

- `AuthManager` -- Manages API key and OAuth2 token auth with thread-safe refresh
- `TokenRefresher` / `OAuth2Refresher` -- Token refresh interface and OAuth2 implementation (client credentials, refresh token grants)
- `AuthInterceptor` -- HTTP request interceptor adding auth headers
- `Middleware` -- HTTP client wrapper adding auth transport

### Configuration (`Commons/config`)

- `Config` -- Generic `map[string]interface{}` with typed getters (`GetString`, `GetInt`, `GetBool`, `GetFloat` with defaults)
- `Validator` / `ValidateFunc` -- Validation with `Required()`, `OneOf()`, `MinLength()` rules
- `ProviderConfig` -- Common provider config struct (API key, base URL, timeout, retries, rate limit)
- `LoadFromEnv()` / `LoadProviderConfigFromEnv()` -- Environment variable loading with prefix filtering

### Errors (`Commons/errors`)

- `ProviderError` -- Provider-specific with code, status, details
- `APIError` -- Parsed API error response
- `RateLimitError` -- Rate limit with retry-after
- `AuthenticationError` -- Auth failures
- `NetworkError` -- Network errors with unwrap
- `TimeoutError` -- Operation timeouts
- `ValidationError` -- Field validation
- `ErrorHandler` -- HTTP error handling and classification
- `IsRetryable()` / `IsRateLimit()` / `IsAuth()` / `GetRetryAfter()` -- Error type checking utilities

### Response Parsing (`Commons/response`)

- `JSONParser` -- JSON response/bytes parsing
- `StreamingParser` -- SSE streaming with data/error callbacks
- `ErrorDetector` -- HTTP status and JSON error detection
- `ResponseValidator` -- Required field validation
- `PaginationParser` -- Paginated response handling with next-page detection
- `ChunkedParser` -- Chunked response reading
- `ResponseBuilder` -- Response construction and sanitization

## Providers

Providers self-register via `init()` using `toolkit.RegisterProviderFactory()`. Import the provider
package with a blank import to auto-register:

```go
import (
    _ "github.com/HelixDevelopment/HelixAgent/Toolkit/Providers/Chutes"
    _ "github.com/HelixDevelopment/HelixAgent/Toolkit/Providers/SiliconFlow"
)
```

Each provider package contains:
- `<name>.go` -- Provider implementation (`Name`, `Chat`, `Embed`, `Rerank`, `DiscoverModels`, `ValidateConfig`)
- `builder.go` -- Config builder with `Build()`, `Validate()`, `Merge()`
- `client.go` -- API client for HTTP communication
- `discovery.go` -- Model discovery from provider API

## CLI Tool

The CLI (`cmd/toolkit`) provides four commands via cobra:

- `toolkit version` -- Show version
- `toolkit test` -- Run integration tests (provider creation, model discovery)
- `toolkit chat --provider <name> --api-key <key> [--model <model>] [--base-url <url>]` -- Interactive chat
- `toolkit agent --type <generic|codereview> --task <task> --api-key <key> [--provider <name>] [--model <model>]` -- Agent task execution

## Mandatory Development Standards

- 100% test coverage across unit, integration, E2E, security, chaos, performance, and fuzz tests
- No mocks outside unit tests -- all other tests use real implementations
- Challenges must validate real-life use cases, not just return codes
- Follow Conventional Commits: `feat(toolkit): ...`, `fix(toolkit): ...`
- Run `make fmt vet lint` before committing

## Integration Seams

| Direction | Sibling modules |
|-----------|-----------------|
| Upstream (this module imports) | none |
| Downstream (these import this module) | none |

*Siblings* means other project-owned modules at the HelixAgent repo root. The root HelixAgent app and external systems are not listed here — the list above is intentionally scoped to module-to-module seams, because drift *between* sibling modules is where the "tests pass, product broken" class of bug most often lives. See root `CLAUDE.md` for the rules that keep these seams contract-tested.
