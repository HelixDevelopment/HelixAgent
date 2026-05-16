# Toolkit - Architecture

**Module:** `github.com/HelixDevelopment/helix_agent/Toolkit`

## Overview

The Toolkit module is a Go library for building AI-powered applications with
multi-provider support, specialized agents, and common infrastructure utilities.
It provides unified interfaces for chat completion, embeddings, reranking, and
model discovery across multiple AI providers.

## Layer Diagram

```
+--------------------------------------------------+
|                  CLI (cmd/toolkit)                |
|  version | test | chat | agent                   |
+--------------------------------------------------+
        |                       |
        v                       v
+------------------+   +------------------+
|   pkg/toolkit    |   | pkg/toolkit/     |
|   Toolkit struct |   | agents           |
|   Provider iface |   | GenericAgent     |
|   Agent iface    |   | CodeReviewAgent  |
|   Factory Reg.   |   |                  |
+------------------+   +------------------+
        |                       |
        v                       v
+--------------------------------------------------+
|           pkg/toolkit/common                      |
|   discovery | http/client | ratelimit             |
|   (caching, retry, token bucket, circuit breaker) |
+--------------------------------------------------+
        |
        v
+--------------------------------------------------+
|                    Commons                        |
|   auth | config | discovery | errors | http |     |
|   ratelimit | response | testing                  |
+--------------------------------------------------+
        |
        v
+--------------------------------------------------+
|                   Providers                       |
|          Chutes  |  SiliconFlow                   |
+--------------------------------------------------+
```

## Package Organization

### Core Layer (`pkg/toolkit`)

The core package defines the two fundamental interfaces -- `Provider` and
`Agent` -- plus the `Toolkit` struct that manages provider and agent
registries. It also contains request/response types (`ChatRequest`,
`EmbeddingRequest`, `RerankRequest`) and the global `ProviderFactoryRegistry`.

### Agents (`pkg/toolkit/agents`)

Built-in agent implementations:

| Agent | Purpose |
|-------|---------|
| `GenericAgent` | General-purpose AI assistant with configurable model/temperature |
| `CodeReviewAgent` | Specialized code review: security, performance, maintainability |

### High-Level Commons (`pkg/toolkit/common`)

Utilities that depend on the `toolkit` package types:

| Package | Purpose |
|---------|---------|
| `common/discovery` | Model discovery with caching, filtering, sorting |
| `common/http` | HTTP client with retry, exponential backoff, auth |
| `common/ratelimit` | Token bucket, sliding window, per-key, circuit breaker |

### Low-Level Commons (`Commons/`)

Standalone utilities with zero dependency on `toolkit` types:

| Package | Purpose |
|---------|---------|
| `auth` | API key auth, OAuth2 token refresh, HTTP interceptor |
| `config` | Typed config map, validation rules, env var loading |
| `discovery` | Model capability inference, category classification |
| `errors` | Structured error types with retryability checks |
| `http` | Advanced HTTP client with interceptors and retry |
| `ratelimit` | Simple token bucket rate limiter |
| `response` | JSON, SSE streaming, pagination, chunked parsers |
| `testing` | MockHTTPClient, MockProvider, TestServer, fixtures |

### Providers (`Providers/`)

Each provider is a self-contained package that self-registers via `init()`:

| Provider | Files |
|----------|-------|
| `Chutes` | `chutes.go`, `builder.go`, `client.go`, `discovery.go` |
| `SiliconFlow` | `siliconflow.go`, `builder.go`, `client.go`, `discovery.go` |

## Design Decisions

### 1. Interface-Driven Architecture

All provider implementations conform to `Provider`; all agent
implementations conform to `Agent`. This enables polymorphic use and
straightforward testing.

### 2. Factory Pattern

Providers self-register via `init()` using `toolkit.RegisterProviderFactory()`.
New providers are added by simply importing the package with a blank import:

```go
import _ "github.com/HelixDevelopment/helix_agent/Toolkit/Providers/Chutes"
```

### 3. Two-Tier Common Packages

Separating `pkg/toolkit/common/` (depends on `toolkit` types) from `Commons/`
(standalone) avoids circular imports and allows `Commons/` to be reused
outside of the Toolkit context.

### 4. Capability Inference

The discovery framework infers model capabilities (chat, embedding, rerank,
vision, audio, video, function calling) from model IDs and type strings
using keyword matching rather than requiring explicit provider metadata.

## CLI Tool

The CLI (`cmd/toolkit`) uses Cobra and provides four subcommands:

| Command | Description |
|---------|-------------|
| `version` | Show build version |
| `test` | Run integration tests (provider creation, model discovery) |
| `chat` | Interactive chat with a provider and model |
| `agent` | Execute an agent task (generic or code review) |

## Concurrency and Thread Safety

- `Toolkit` struct uses `sync.RWMutex` for registry access
- `ProviderFactoryRegistry` is globally locked for concurrent registration
- `TokenBucket` and `CircuitBreaker` are thread-safe
- `PerKeyLimiter` includes automatic cleanup of expired entries
