# HelixAgent Comprehensive Remediation Master Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remediate all 144+ issues across 8 dimensions — safety, dead code, tests, performance, docs, and hardening — achieving 100% test coverage, zero dead code, full documentation, and rock-solid production readiness.

**Architecture:** Six independent tiers executed in priority order. Each tier is self-contained and produces verifiable, testable results. Changes are backward-compatible and non-breaking. All modifications follow existing patterns and conventions.

**Tech Stack:** Go 1.25.3, Gin v1.12.0, PostgreSQL 15/pgx, Redis 7, testify, Prometheus, OpenTelemetry, Docker/Podman, Snyk, SonarQube, Gosec

---

## Tier 1: Critical Safety Fixes (HIGHEST PRIORITY)

### Task 1.1: Fix TLS InsecureSkipVerify Default

**Files:**
- Modify: `internal/llm/providers/helixllm/provider.go:70`
- Test: `internal/llm/providers/helixllm/provider_test.go`

- [ ] **Step 1: Write failing test for secure TLS default**

```go
func TestProvider_TLSSecureByDefault(t *testing.T) {
	// When no TLS config is provided, InsecureSkipVerify must be false
	os.Unsetenv("HELIX_LLM_TLS_SKIP_VERIFY")
	cfg := Config{
		Endpoint: "https://localhost:8443",
	}
	p := NewProvider(cfg)
	transport := p.httpClient.Transport.(*http.Transport)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify,
		"TLS verification must be enabled by default")
}

func TestProvider_TLSSkipVerifyExplicitOptIn(t *testing.T) {
	cfg := Config{
		Endpoint:      "https://localhost:8443",
		TLSSkipVerify: true,
	}
	p := NewProvider(cfg)
	transport := p.httpClient.Transport.(*http.Transport)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify,
		"TLS verification should be skippable with explicit opt-in")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOMAXPROCS=2 go test -v -run TestProvider_TLSSecureByDefault ./internal/llm/providers/helixllm/ -count=1 -p 1`
Expected: FAIL — currently defaults to `true`

- [ ] **Step 3: Fix the insecure default**

Change `internal/llm/providers/helixllm/provider.go:70` from:
```go
InsecureSkipVerify: cfg.TLSSkipVerify || getEnvBool("HELIX_LLM_TLS_SKIP_VERIFY", true),
```
to:
```go
InsecureSkipVerify: cfg.TLSSkipVerify || getEnvBool("HELIX_LLM_TLS_SKIP_VERIFY", false),
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOMAXPROCS=2 go test -v -run TestProvider_TLS ./internal/llm/providers/helixllm/ -count=1 -p 1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/llm/providers/helixllm/provider.go internal/llm/providers/helixllm/provider_test.go
git commit -m "fix(security): change TLS InsecureSkipVerify default to false

CRITICAL: The default was true, allowing MITM attacks on HelixLLM connections.
Now requires explicit opt-in via config or HELIX_LLM_TLS_SKIP_VERIFY=true env var."
```

---

### Task 1.2: Fix Rate Limiter Goroutine Leak and Deadlock Risk

**Files:**
- Modify: `internal/middleware/rate_limit.go:49-104`
- Test: `internal/middleware/rate_limit_test.go`

- [ ] **Step 1: Write failing test for graceful shutdown**

```go
func TestRateLimiter_GracefulShutdown(t *testing.T) {
	rl := NewRateLimiter(nil)
	// Should have a Stop method
	require.NotNil(t, rl)
	
	// Stop should not panic and should terminate cleanup goroutine
	rl.Stop()
	
	// After stop, cleanup goroutine should not be running
	// Verify by ensuring no further bucket operations occur
	time.Sleep(100 * time.Millisecond)
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(nil)
	defer rl.Stop()
	
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("client-%d", n%10)
			rl.checkInMemory(key, rl.defaultCfg)
		}(i)
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOMAXPROCS=2 go test -v -run TestRateLimiter_GracefulShutdown ./internal/middleware/ -count=1 -p 1`
Expected: FAIL — no Stop method exists

- [ ] **Step 3: Add context-based lifecycle to RateLimiter**

Add to `internal/middleware/rate_limit.go`:
```go
type RateLimiter struct {
	cache      *cache.CacheService
	mu         sync.RWMutex
	limits     map[string]*RateLimitConfig
	defaultCfg *RateLimitConfig
	buckets    map[string]*tokenBucket
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}
```

Update `NewRateLimiter`:
```go
func NewRateLimiter(cacheService *cache.CacheService) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		cache:   cacheService,
		limits:  make(map[string]*RateLimitConfig),
		buckets: make(map[string]*tokenBucket),
		ctx:     ctx,
		cancel:  cancel,
		defaultCfg: &RateLimitConfig{
			Requests: 100,
			Window:   time.Minute,
			KeyFunc:  defaultKeyFunc,
		},
	}
	rl.wg.Add(1)
	go rl.cleanupExpiredBuckets()
	return rl
}
```

Update `cleanupExpiredBuckets`:
```go
func (rl *RateLimiter) cleanupExpiredBuckets() {
	defer rl.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, bucket := range rl.buckets {
				bucket.mu.Lock()
				if now.Sub(bucket.lastRefill) > 10*time.Minute {
					delete(rl.buckets, key)
				}
				bucket.mu.Unlock()
			}
			rl.mu.Unlock()
		}
	}
}
```

Add `Stop` method:
```go
func (rl *RateLimiter) Stop() {
	rl.cancel()
	rl.wg.Wait()
}
```

- [ ] **Step 4: Run tests**

Run: `GOMAXPROCS=2 go test -v -run TestRateLimiter ./internal/middleware/ -count=1 -p 1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/rate_limit.go internal/middleware/rate_limit_test.go
git commit -m "fix(middleware): add context-based lifecycle to rate limiter

Fixes goroutine leak: cleanup goroutine now cancels on Stop().
Fixes deadlock risk: uses select-based loop with context cancellation.
Adds WaitGroup tracking for graceful shutdown."
```

---

### Task 1.3: Fix Worker Pool Race Condition on `started` Boolean

**Files:**
- Modify: `internal/background/worker_pool.go:79`
- Test: `internal/background/worker_pool_test.go`

- [ ] **Step 1: Write race detection test**

```go
func TestWorkerPool_StartedFlagRaceSafety(t *testing.T) {
	cfg := DefaultWorkerPoolConfig()
	cfg.MinWorkers = 1
	cfg.MaxWorkers = 2
	pool := NewAdaptiveWorkerPool(cfg, nil, nil, nil, nil, logrus.New())
	
	var wg sync.WaitGroup
	// Concurrent read/write of started flag
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.IsStarted()
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run with race detector**

Run: `GOMAXPROCS=2 go test -v -race -run TestWorkerPool_StartedFlagRaceSafety ./internal/background/ -count=1 -p 1`
Expected: Potential race detection warning

- [ ] **Step 3: Convert started to atomic**

In `internal/background/worker_pool.go`, change:
```go
// Before:
started bool

// After:
started int32 // 0=not started, 1=started — use atomic operations
```

Update all `started` references to use `atomic.LoadInt32(&wp.started)` / `atomic.StoreInt32(&wp.started, 1)`.

Add method:
```go
func (wp *AdaptiveWorkerPool) IsStarted() bool {
	return atomic.LoadInt32(&wp.started) == 1
}
```

- [ ] **Step 4: Run with race detector**

Run: `GOMAXPROCS=2 go test -v -race -run TestWorkerPool ./internal/background/ -count=1 -p 1`
Expected: PASS with no race warnings

- [ ] **Step 5: Commit**

```bash
git add internal/background/worker_pool.go internal/background/worker_pool_test.go
git commit -m "fix(background): convert started flag to atomic int32

Prevents data race when multiple goroutines check pool started state.
Uses atomic.LoadInt32/StoreInt32 instead of plain bool access."
```

---

### Task 1.4: Fix SSE Handler Nested Map Race Condition

**Files:**
- Modify: `internal/handlers/protocol_sse.go:28-29`
- Test: `internal/handlers/protocol_sse_test.go`

- [ ] **Step 1: Write concurrent broadcast test**

```go
func TestProtocolSSEHandler_ConcurrentBroadcast(t *testing.T) {
	handler := NewProtocolSSEHandler(nil, nil, nil, nil, nil, logrus.New())
	
	var wg sync.WaitGroup
	// Simulate concurrent client registrations and broadcasts
	for i := 0; i < 50; i++ {
		wg.Add(2)
		protocol := fmt.Sprintf("protocol-%d", i%5)
		go func() {
			defer wg.Done()
			ch := make(chan []byte, 10)
			handler.registerClient(protocol, ch)
			time.Sleep(10 * time.Millisecond)
			handler.unregisterClient(protocol, ch)
		}()
		go func() {
			defer wg.Done()
			handler.broadcast(protocol, []byte("test message"))
		}()
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run with race detector**

Run: `GOMAXPROCS=2 go test -v -race -run TestProtocolSSEHandler_ConcurrentBroadcast ./internal/handlers/ -count=1 -p 1`
Expected: Race condition detected

- [ ] **Step 3: Fix by holding write lock during broadcast nested map operations**

In `protocol_sse.go`, ensure all operations on the nested map `clients[protocol][ch]` are done while holding `clientsMu` at the appropriate level. The `broadcast` method should use `RLock` to read the outer map and iterate, but never mutate the inner map without `Lock`.

- [ ] **Step 4: Run with race detector**

Run: `GOMAXPROCS=2 go test -v -race -run TestProtocolSSEHandler ./internal/handlers/ -count=1 -p 1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/protocol_sse.go internal/handlers/protocol_sse_test.go
git commit -m "fix(handlers): resolve race condition in SSE nested map access

Ensures all nested map operations on clients[protocol][ch] are properly
synchronized under clientsMu lock. Prevents concurrent map read/write panic."
```

---

### Task 1.5: Fix Memory Service Goroutine Leak

**Files:**
- Modify: `internal/services/memory_service.go`
- Test: `internal/services/memory_service_test.go`

- [ ] **Step 1: Add context cancellation and WaitGroup to memory service cleanup routine**

The pattern is the same as Task 1.2: add `ctx`, `cancel`, `wg` fields; update cleanup goroutine to use `select` with `ctx.Done()`; add `Stop()` method.

- [ ] **Step 2: Write test verifying Stop() terminates the cleanup goroutine**

- [ ] **Step 3: Commit**

```bash
git commit -m "fix(services): add graceful shutdown to memory service cleanup goroutine"
```

---

### Task 1.6: Fix Models Dev Cache Goroutine Leak

**Files:**
- Modify: `internal/modelsdev/cache.go`
- Test: `internal/modelsdev/cache_test.go`

- [ ] **Step 1: Add context cancellation to cache cleanup loop**

Same pattern: add `ctx`, `cancel`, `wg`; update `cleanupLoop()` to use `select` with `ctx.Done()`.

- [ ] **Step 2: Write test verifying cache Stop() works**

- [ ] **Step 3: Commit**

```bash
git commit -m "fix(modelsdev): add graceful shutdown to cache cleanup goroutine"
```

---

## Tier 2: Dead Code Cleanup & Missing Infrastructure

### Task 2.1: Fix Missing Docker-Compose Files

**Files:**
- Create: `docker/embeddings/docker-compose.embeddings.yml`
- Create: `docker/acp/docker-compose.acp.yml`
- Create: `docker/vision/docker-compose.vision.yml`
- Modify: `cmd/helixagent/infrastructure.go:88` (fix monitoring compose path)

- [ ] **Step 1: Create embeddings docker-compose**

```yaml
version: '3.8'
services:
  helixagent-embeddings:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "9310:9310"
    environment:
      - PORT=9310
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:9310/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

- [ ] **Step 2: Create ACP docker-compose**

```yaml
version: '3.8'
services:
  helixagent-acp:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "9320:9320"
    environment:
      - PORT=9320
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:9320/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

- [ ] **Step 3: Create vision docker-compose**

First create the directory:
```bash
mkdir -p docker/vision
```

```yaml
version: '3.8'
services:
  helixagent-vision:
    build:
      context: ../../VisionEngine
      dockerfile: Dockerfile
    ports:
      - "9330:9330"
    environment:
      - PORT=9330
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:9330/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

- [ ] **Step 4: Fix monitoring compose path in infrastructure.go**

Change `infrastructure.go:88` from:
```go
ComposeFile: "docker/monitoring/docker-compose.monitoring.yml",
```
to:
```go
ComposeFile: "docker/monitoring/docker-compose.yml",
```

- [ ] **Step 5: Verify all compose files resolve**

Run: `for f in docker/embeddings/docker-compose.embeddings.yml docker/acp/docker-compose.acp.yml docker/vision/docker-compose.vision.yml docker/monitoring/docker-compose.yml; do test -f "$f" && echo "OK: $f" || echo "MISSING: $f"; done`
Expected: All OK

- [ ] **Step 6: Commit**

```bash
git add docker/embeddings/ docker/acp/docker-compose.acp.yml docker/vision/ cmd/helixagent/infrastructure.go
git commit -m "fix(infra): create missing docker-compose files and fix monitoring path

Creates docker-compose for embeddings, ACP, and vision services.
Fixes monitoring compose path from docker-compose.monitoring.yml to docker-compose.yml."
```

---

### Task 2.2: Clean Up Empty Internal Directories

**Files:**
- Remove: `internal/desktop/` (empty)
- Remove: `internal/events/` (empty, has only 1 stub file)
- Remove: `internal/sandbox/` (empty)

- [ ] **Step 1: Verify directories are truly empty of production code**

```bash
find internal/desktop internal/events internal/sandbox -name "*.go" ! -name "*_test.go" -exec wc -l {} \;
```

- [ ] **Step 2: Remove empty directories**

If confirmed empty of meaningful code:
```bash
rm -rf internal/desktop internal/sandbox
```

For `internal/events`, check if stub file has content worth keeping or just boilerplate.

- [ ] **Step 3: Commit**

```bash
git add -A internal/desktop internal/events internal/sandbox
git commit -m "chore: remove empty internal packages (desktop, events, sandbox)

These directories contained no production code. Prevents dead code accumulation."
```

---

### Task 2.3: Wire or Remove Disconnected Packages

**Decision matrix for each disconnected package:**

| Package | Files | Decision | Reason |
|---------|-------|----------|--------|
| `internal/debate/` | 58+46 | **KEEP** — wire via adapter | Core debate orchestration, used by extracted DebateOrchestrator module |
| `internal/optimization/` | 31+24 | **KEEP** — already wired via adapter | `internal/adapters/optimization/adapter.go` imports it |
| `internal/context/` | 4+3 | **KEEP** — evaluate wiring | Context providers may be needed |
| `internal/codebase/` | 1+1 | **EVALUATE** | Check if used by debate or selfimprove |
| `internal/lsp/` | 2+2 | **KEEP** — wire into LSP handler | LSP server implementation |
| `internal/selfimprove/` | 5+3 | **KEEP** — wire via adapter | Self-improvement feature |
| `internal/toon/` | 6+6 | **EVALUATE** | Check purpose |
| `internal/repository/` | 1+1 | **EVALUATE** | Check if used by debate |
| `internal/vectordb/` | 5+11 | **KEEP** — wired via adapters | Bridge to extracted VectorDB module |
| `internal/verification/` | 1+1 | **EVALUATE** | May be superseded by verifier/ |

- [ ] **Step 1: For each EVALUATE package, grep for usage and decide keep/remove**
- [ ] **Step 2: For packages marked KEEP, verify adapter connectivity**
- [ ] **Step 3: Remove confirmed dead packages, commit**
- [ ] **Step 4: For KEEP packages, ensure imports are live, commit**

---

### Task 2.4: Clean Up Unused Environment Variables

**Files:**
- Modify: `.env.example`

- [ ] **Step 1: Grep each unused variable against all Go source**

```bash
for var in ALLOWED_ORIGINS DB_SSL_MODE DEBUG_ENABLED EXA_API_KEY; do
  echo "=== $var ===" && grep -r "$var" --include="*.go" -l | head -3
done
```

- [ ] **Step 2: Remove confirmed unused variables from .env.example**
- [ ] **Step 3: Keep variables that ARE used but under different patterns (e.g., viper binding)**
- [ ] **Step 4: Commit**

```bash
git commit -m "chore: remove unused environment variables from .env.example"
```

---

### Task 2.5: Fix Incomplete Planning Handler TODOs

**Files:**
- Modify: `internal/handlers/extended/planning.go`

- [ ] **Step 1: Implement database persistence for planning endpoints (lines 414, 489)**
- [ ] **Step 2: Complete TODO endpoint management (line 653)**
- [ ] **Step 3: Add tests for new functionality**
- [ ] **Step 4: Commit**

```bash
git commit -m "feat(handlers): complete planning handler database persistence"
```

---

## Tier 3: Test Coverage Expansion

### Task 3.1: Fill Critical Package Test Gaps

**Packages needing coverage increase (ordered by severity):**

1. `internal/search/` — 20% → target 80%+
2. `internal/utils/` — 27% → target 80%+
3. `internal/agents/` — 33% → target 80%+
4. `internal/benchmark/` — 33% → target 80%+
5. `internal/http/` — 33% → target 80%+
6. `internal/testing/` — 33% → target 80%+
7. `internal/llmops/` — 37% → target 80%+
8. `internal/observability/` — 37% → target 80%+
9. `internal/selfimprove/` — 37% → target 80%+
10. `internal/knowledge/` — 40% → target 80%+

For each package:
- [ ] **Step 1: List all exported functions without tests**
- [ ] **Step 2: Write table-driven unit tests for each**
- [ ] **Step 3: Run coverage: `go test -coverprofile=coverage.out ./internal/<pkg>/...`**
- [ ] **Step 4: Verify coverage ≥ 80%**
- [ ] **Step 5: Commit per-package**

---

### Task 3.2: Add Missing Test Types

- [ ] **Step 1: Create mutation testing framework**

```bash
# Install go-mutesting
go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest
```

Create `scripts/test-mutation.sh`:
```bash
#!/bin/bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go-mutesting ./internal/... 2>&1 | tee reports/mutation-report.txt
```

- [ ] **Step 2: Create load testing framework**

Create `tests/load/load_test.go` with sustained load simulation using Go's built-in HTTP testing.

- [ ] **Step 3: Create contract/API testing**

Create `tests/contract/api_contract_test.go` validating OpenAPI schema against actual responses.

- [ ] **Step 4: Expand fuzz tests from 55 to 100+**

Add fuzz tests for: all HTTP handlers, all provider request parsing, all configuration parsing.

- [ ] **Step 5: Consolidate benchmark tests**

Move scattered benchmark functions into `tests/benchmarks/` with structured organization:
- `tests/benchmarks/cache_bench_test.go`
- `tests/benchmarks/handler_bench_test.go`
- `tests/benchmarks/provider_bench_test.go`
- `tests/benchmarks/debate_bench_test.go`

- [ ] **Step 6: Commit each test type separately**

---

### Task 3.3: Fix Module Compilation Issues

**Files:**
- Modify: `Containers/pkg/lazyservice/orchestrator.go:32,59`
- Modify: `HelixMemory/pkg/consolidation/consolidation.go:22`
- Modify: `HelixMemory/pkg/provider/unified.go:26`

- [ ] **Step 1: Fix Containers type references**

Update `health.Check` → correct exported type, `runtime.RuntimeType` → correct exported type.

- [ ] **Step 2: Fix HelixMemory fusion.Engine and Config fields**

Run `go mod tidy` in HelixMemory, add missing Config fields.

- [ ] **Step 3: Verify compilation**

```bash
cd Containers && go build ./... && cd ..
cd HelixMemory && go build ./... && cd ..
```

- [ ] **Step 4: Commit**

```bash
git commit -m "fix(modules): resolve Containers and HelixMemory compilation errors"
```

---

## Tier 4: Performance Optimization

### Task 4.1: Parallelize Startup Health Checks

**Files:**
- Modify: `cmd/helixagent/main.go` (health check section)

- [ ] **Step 1: Replace sequential health checks with parallel goroutines**

```go
func parallelHealthChecks(ctx context.Context, checks map[string]func() error, logger *logrus.Logger) map[string]error {
	results := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	for name, check := range checks {
		wg.Add(1)
		go func(n string, c func() error) {
			defer wg.Done()
			err := c()
			mu.Lock()
			results[n] = err
			mu.Unlock()
		}(name, check)
	}
	
	wg.Wait()
	return results
}
```

- [ ] **Step 2: Replace time.Sleep retry loops with context-aware backoff**
- [ ] **Step 3: Test startup time improvement**
- [ ] **Step 4: Commit**

```bash
git commit -m "perf(boot): parallelize health checks and use context-aware backoff

Reduces startup time from 15-60s to 2-10s by running health checks concurrently.
Replaces blocking time.Sleep loops with ticker-based context-aware retries."
```

---

### Task 4.2: Add Prometheus Instrumentation to All Handlers

**Files:**
- Modify: `internal/observability/metrics/collector.go`
- Modify: `internal/router/gin_router.go` (add metrics middleware)

- [ ] **Step 1: Create per-handler metrics middleware**

```go
func MetricsMiddleware(collector *metrics.Collector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		collector.RecordHandlerLatency(c.Request.URL.Path, c.Request.Method, c.Writer.Status(), duration)
	}
}
```

- [ ] **Step 2: Add worker pool metrics export to Prometheus**
- [ ] **Step 3: Add database query latency tracking**
- [ ] **Step 4: Test metrics endpoint returns data**
- [ ] **Step 5: Commit**

```bash
git commit -m "feat(observability): add Prometheus metrics to all HTTP handlers

Instruments 41 handlers with per-handler latency histograms, error counters,
and request counters. Exports worker pool utilization and DB query latency."
```

---

### Task 4.3: Add sync.Pool for Hot-Path Buffers

**Files:**
- Modify: `internal/handlers/openai_compatible.go`
- Modify: `internal/handlers/debate_format_markdown.go`

- [ ] **Step 1: Add buffer pool**

```go
var responseBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}
```

- [ ] **Step 2: Use pool in hot-path JSON encoding**
- [ ] **Step 3: Benchmark before/after**
- [ ] **Step 4: Commit**

```bash
git commit -m "perf(handlers): add sync.Pool buffer reuse for response encoding"
```

---

### Task 4.4: Add Request-Level Semaphore

**Files:**
- Modify: `internal/router/gin_router.go`

- [ ] **Step 1: Add concurrency limiter middleware**

```go
func ConcurrencyLimiter(maxConcurrent int64) gin.HandlerFunc {
	sem := semaphore.NewWeighted(maxConcurrent)
	return func(c *gin.Context) {
		if !sem.TryAcquire(1) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server at capacity"})
			c.Abort()
			return
		}
		defer sem.Release(1)
		c.Next()
	}
}
```

- [ ] **Step 2: Add to router with configurable limit**
- [ ] **Step 3: Test with concurrent requests**
- [ ] **Step 4: Commit**

```bash
git commit -m "perf(router): add request-level semaphore to prevent thundering herd"
```

---

## Tier 5: Documentation & Content

### Task 5.1: Create MCP-Servers Documentation

**Files:**
- Create: `MCP-Servers/CLAUDE.md`
- Create: `MCP-Servers/AGENTS.md`
- Create: `MCP-Servers/docs/README.md`

- [ ] **Step 1: Create CLAUDE.md with module overview, architecture, and development guide**
- [ ] **Step 2: Create AGENTS.md with agent development patterns**
- [ ] **Step 3: Create docs/README.md with server documentation index**
- [ ] **Step 4: Commit**

```bash
git commit -m "docs(mcp-servers): add CLAUDE.md, AGENTS.md, and docs directory"
```

---

### Task 5.2: Create MCP Index Documentation

**Files:**
- Create: `MCP/CLAUDE.md` (index of third-party submodules)

- [ ] **Step 1: Create lightweight index CLAUDE.md linking to submodule repos**
- [ ] **Step 2: Commit**

```bash
git commit -m "docs(mcp): add CLAUDE.md index for third-party MCP submodules"
```

---

### Task 5.3: Extend Website Content

**Files:**
- Modify: `Website/user-manuals/` (add new manuals for gaps)
- Modify: `Website/video-courses/` (add new course modules)

- [ ] **Step 1: Add user manual for security scanning**

Create `Website/user-manuals/44-security-scanning-guide.md` covering Snyk, SonarQube, Gosec, Trivy usage.

- [ ] **Step 2: Add user manual for performance monitoring**

Create `Website/user-manuals/45-performance-optimization-guide.md` covering Prometheus metrics, lazy loading, semaphores.

- [ ] **Step 3: Add user manual for stress testing**

Create `Website/user-manuals/46-stress-testing-guide.md` covering load testing, chaos engineering.

- [ ] **Step 4: Add video course modules**

Create new video course scripts covering:
- Security scanning workflow
- Performance profiling and optimization
- Stress and load testing
- Dead code detection and cleanup

- [ ] **Step 5: Update Website README.md with new content**
- [ ] **Step 6: Commit**

```bash
git commit -m "docs(website): add security scanning, performance, and stress testing guides"
```

---

### Task 5.4: Extend Architecture Diagrams

**Files:**
- Create: `docs/diagrams/src/security-scanning-flow.mmd`
- Create: `docs/diagrams/src/performance-metrics-flow.mmd`
- Create: `docs/diagrams/src/concurrency-lifecycle.mmd`

- [ ] **Step 1: Create Mermaid diagrams for new features**
- [ ] **Step 2: Commit**

```bash
git commit -m "docs(diagrams): add security scanning, metrics, and concurrency diagrams"
```

---

### Task 5.5: Update CLAUDE.md and AGENTS.md

**Files:**
- Modify: `CLAUDE.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Sync all changes from this remediation into CLAUDE.md**
- [ ] **Step 2: Update AGENTS.md with new patterns and conventions**
- [ ] **Step 3: Verify Constitution synchronization**
- [ ] **Step 4: Commit**

```bash
git commit -m "docs: synchronize CLAUDE.md, AGENTS.md with remediation changes"
```

---

## Tier 6: Stress Testing & Hardening

### Task 6.1: Create OWASP Compliance Test Suite

**Files:**
- Create: `tests/security/owasp_compliance_test.go`

- [ ] **Step 1: Implement tests for OWASP Top 10**

```go
func TestOWASP_A01_BrokenAccessControl(t *testing.T) { ... }
func TestOWASP_A02_CryptographicFailures(t *testing.T) { ... }
func TestOWASP_A03_Injection(t *testing.T) { ... }
func TestOWASP_A04_InsecureDesign(t *testing.T) { ... }
func TestOWASP_A05_SecurityMisconfiguration(t *testing.T) { ... }
func TestOWASP_A06_VulnerableComponents(t *testing.T) { ... }
func TestOWASP_A07_AuthenticationFailures(t *testing.T) { ... }
func TestOWASP_A08_SoftwareIntegrityFailures(t *testing.T) { ... }
func TestOWASP_A09_LoggingMonitoringFailures(t *testing.T) { ... }
func TestOWASP_A10_SSRF(t *testing.T) { ... }
```

- [ ] **Step 2: Commit**

```bash
git commit -m "test(security): add OWASP Top 10 compliance test suite"
```

---

### Task 6.2: Create Comprehensive Load Testing Framework

**Files:**
- Create: `tests/load/sustained_load_test.go`
- Create: `tests/load/spike_test.go`
- Create: `tests/load/soak_test.go`

- [ ] **Step 1: Implement sustained load test (constant rate for 5 minutes)**
- [ ] **Step 2: Implement spike test (sudden 10x traffic burst)**
- [ ] **Step 3: Implement soak test (low rate for extended period, checking for leaks)**
- [ ] **Step 4: Add resource utilization tracking (goroutines, memory, CPU)**
- [ ] **Step 5: Commit**

```bash
git commit -m "test(load): add sustained, spike, and soak load testing framework"
```

---

### Task 6.3: Run Security Scanners and Fix Findings

- [ ] **Step 1: Run Gosec**

```bash
make security-scan-gosec 2>&1 | tee reports/security/gosec-results.txt
```

- [ ] **Step 2: Run Trivy**

```bash
make security-scan-trivy 2>&1 | tee reports/security/trivy-results.txt
```

- [ ] **Step 3: Start SonarQube and run scan**

```bash
make security-scan-sonarqube 2>&1 | tee reports/security/sonarqube-results.txt
```

- [ ] **Step 4: Analyze findings and fix all CRITICAL/HIGH issues**
- [ ] **Step 5: Re-run scanners to verify fixes**
- [ ] **Step 6: Commit fixes**

```bash
git commit -m "fix(security): resolve all critical/high security scanner findings"
```

---

### Task 6.4: Monitoring and Metrics Collection Tests

**Files:**
- Create: `tests/monitoring/metrics_collection_test.go`
- Create: `tests/monitoring/alerting_test.go`

- [ ] **Step 1: Test Prometheus metrics endpoint returns expected metrics**
- [ ] **Step 2: Test alerting rules trigger correctly**
- [ ] **Step 3: Test Grafana dashboard data sources**
- [ ] **Step 4: Commit**

```bash
git commit -m "test(monitoring): add metrics collection and alerting validation tests"
```

---

### Task 6.5: Create Challenge Scripts for New Features

**Files:**
- Create: `challenges/scripts/security_scanning_challenge.sh`
- Create: `challenges/scripts/performance_optimization_challenge.sh`
- Create: `challenges/scripts/memory_safety_challenge.sh`

- [ ] **Step 1: Security scanning challenge — validates all 7 scanners run and produce reports**
- [ ] **Step 2: Performance challenge — validates startup time, request latency, resource usage**
- [ ] **Step 3: Memory safety challenge — validates no goroutine leaks under stress**
- [ ] **Step 4: Commit**

```bash
git commit -m "test(challenges): add security, performance, and memory safety challenges"
```

---

## Execution Summary

| Tier | Tasks | Priority | Dependencies |
|------|-------|----------|--------------|
| 1: Critical Safety | 6 tasks | HIGHEST | None |
| 2: Dead Code Cleanup | 5 tasks | HIGH | None |
| 3: Test Coverage | 3 tasks | HIGH | Tier 1 (safety fixes must be in before testing) |
| 4: Performance | 4 tasks | MEDIUM | Tier 1 |
| 5: Documentation | 5 tasks | MEDIUM | Tiers 1-4 (document what was changed) |
| 6: Hardening | 5 tasks | MEDIUM | Tiers 1-4 |

**Total: 28 tasks across 6 tiers**

**Execution order:** Tiers 1 and 2 can run in parallel. Tiers 3 and 4 can run in parallel after Tier 1. Tiers 5 and 6 can run in parallel after Tiers 1-4.
