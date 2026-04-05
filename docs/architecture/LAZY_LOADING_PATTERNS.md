# Lazy Loading Patterns

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent uses lazy initialization extensively to minimize startup time, reduce memory footprint, and ensure that expensive resources are only allocated when actually needed. This document catalogs the lazy loading patterns used across the codebase, their implementation details, and guidelines for adding new lazy-loaded components.

---

## Pattern 1: `sync.Once` for Global Singletons

**Use case:** Module-level resources that are expensive to create and shared across the entire application.

**Implementation:**
```go
var (
    globalPool     *Pool
    globalPoolOnce sync.Once
)

func GetPool() *Pool {
    globalPoolOnce.Do(func() {
        globalPool = newPool(DefaultPoolConfig())
    })
    return globalPool
}
```

**Characteristics:**
- Thread-safe: `sync.Once` guarantees exactly-one initialization even under concurrent access
- Zero cost if never called: the resource is never allocated if `GetPool()` is never invoked
- No mutex on the hot path: after initialization, `GetPool()` is a simple pointer read

**Applied to:**

| Component | File | What Is Lazy-Loaded |
|-----------|------|-------------------|
| HTTP Client Pool | `internal/http/pool.go` | Global HTTP connection pool with per-host clients |
| Event Bus | `internal/clis/event_bus.go` | Pub/sub event bus singleton |
| CLI Agent Registry | `internal/clis/agents/registry.go` | Agent definition registry |
| Master Agent | `internal/clis/agents/master/master.go` | Master agent coordinator |
| Claude Integration | `internal/clis/claude/integration.go` | Claude CLI integration |
| Embedding Model Registry | `internal/embeddings/models/registry.go` | Embedding model catalog |
| Tool Handler | `internal/tools/handler.go` | Tool schema registry |
| Test Utilities | `internal/testutil/infra.go` | Infrastructure test helpers |
| Container Adapter | `internal/adapters/containers/adapter.go` | Container runtime adapter |
| Database Adapter | `internal/adapters/database/adapter.go` | Database connection adapter |
| BigData Integration | `internal/bigdata/integration.go` | BigData component initialization |
| Formatter Registry | `internal/formatters/registry.go` | Code formatter registry |
| Feature Capabilities | `internal/features/capability.go` | Feature flag evaluation |
| Feature Config | `internal/features/config.go` | Feature configuration |
| Feature Flags | `internal/features/features.go` | Feature toggle state |
| GraphQL Schema | `internal/graphql/schema.go` | GraphQL schema compilation |
| Messaging Hub | `internal/messaging/hub.go` | Message broker hub |
| Kafka Streams | `internal/streaming/kafka_streams.go` | Kafka stream processor |

---

## Pattern 2: Per-Entity Lazy Initialization

**Use case:** Resources that are created on demand per entity (per provider, per MCP adapter, per formatter) rather than at startup.

**Implementation:**
```go
type LazyProvider struct {
    once     sync.Once
    provider LLMProvider
    factory  func() (LLMProvider, error)
    err      error
}

func (lp *LazyProvider) Get() (LLMProvider, error) {
    lp.once.Do(func() {
        lp.provider, lp.err = lp.factory()
    })
    return lp.provider, lp.err
}
```

**Applied to:**

| Component | File | What Is Lazy-Loaded |
|-----------|------|-------------------|
| LLM Provider | `internal/llm/lazy_provider.go` | Individual provider instances (created on first request) |
| MCP Adapter | `internal/mcp/preinstaller.go` | MCP adapter connections |
| Formatter | `internal/formatters/registry.go` | Individual formatter instances |

**Characteristics:**
- Each entity has its own `sync.Once` so initialization of one does not block others
- Factory errors are cached: if initialization fails, the error is returned on all subsequent calls (fail-fast)
- Provider health checks are deferred until the provider is actually needed

---

## Pattern 3: Configurable Timeouts

**Use case:** Lazy initialization that may involve network calls (provider health checks, container health probes) must have bounded initialization time.

**Implementation:**
```go
func (lp *LazyProvider) GetWithTimeout(ctx context.Context, timeout time.Duration) (LLMProvider, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    done := make(chan struct{})
    lp.once.Do(func() {
        defer close(done)
        lp.provider, lp.err = lp.factoryWithCtx(ctx)
    })

    select {
    case <-done:
        return lp.provider, lp.err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

**Applied to:**
- Provider initialization (default: 30s timeout per provider)
- Container health checks (default: 10s per container)
- MCP adapter connections (default: 15s per adapter)

---

## Pattern 4: GOMAXPROCS Awareness

**Use case:** Lazy initialization must respect the `GOMAXPROCS` setting to avoid spawning more initialization goroutines than the system can handle.

**Implementation:** The HelixAgent respects `GOMAXPROCS` at all levels:
- Startup verification runs at most `GOMAXPROCS` concurrent provider verifications
- Lazy provider initialization uses a semaphore bounded by `GOMAXPROCS`
- Background refresh goroutines are limited to `GOMAXPROCS / 2`

```go
maxConcurrent := runtime.GOMAXPROCS(0)
sem := make(chan struct{}, maxConcurrent)

for _, provider := range providers {
    sem <- struct{}{}
    go func(p Provider) {
        defer func() { <-sem }()
        p.LazyInit()
    }(provider)
}
```

**Why this matters:** The host machine has strict resource limits (Constitution: 30-40% of system resources). Setting `GOMAXPROCS=2` and respecting it in lazy initialization prevents resource spikes during the initialization burst.

---

## Guidelines for Adding New Lazy-Loaded Components

### When to Use Lazy Loading

- The component involves I/O (network, disk, database)
- The component may not be needed in every execution path
- The component is expensive to initialize (>10ms)
- The component holds scarce resources (connections, file handles)

### When NOT to Use Lazy Loading

- The component is cheap to initialize (<1ms) and always needed
- The component must be validated at startup (use eager initialization)
- Initialization order matters (use explicit initialization sequence in boot manager)

### Implementation Checklist

1. Use `sync.Once` for thread-safe initialization
2. Cache both the result and any error from initialization
3. Add a configurable timeout for any I/O during initialization
4. Respect `GOMAXPROCS` for concurrent initialization
5. Add a `Close()` or `Shutdown()` method for cleanup
6. Log initialization at `INFO` level (first use should be visible in logs)
7. Add benchmark tests in `tests/performance/lazy_loading_benchmark_test.go`
8. Validate via `./challenges/scripts/lazy_loading_validation_challenge.sh`

---

## Benchmarks

Lazy loading overhead is measured in `tests/performance/`:
- `lazy_loading_benchmark_test.go` -- measures `sync.Once` overhead per call
- `lazy_loading_comprehensive_test.go` -- measures end-to-end lazy initialization paths

Typical overhead: <5ns per `sync.Once.Do()` call after initialization (atomic load).

---

## Cross-References

- Constitution rule: "Lazy Loading and Non-Blocking" (Priority 2)
- Goroutine lifecycle: `docs/diagrams/src/goroutine-lifecycle.puml`
- Lazy loading diagram: `docs/diagrams/src/lazy-loading-architecture.puml`
- Performance baselines: `docs/performance/BASELINE_GUIDE.md`
- Challenge: `./challenges/scripts/lazy_loading_validation_challenge.sh`
