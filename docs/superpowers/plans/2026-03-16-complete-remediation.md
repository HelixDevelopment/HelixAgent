# HelixAgent Complete Remediation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remediate all safety issues, close test coverage gaps, run security scanning, optimize performance, update all documentation and content — achieving 100% project completeness across all dimensions.

**Architecture:** 12-phase bottom-up sweep where each phase builds on the previous. Safety fixes come first (goroutine leaks, race conditions), then test coverage expansion, security scanning, performance optimization, monitoring tests, stress tests, documentation, content, final validation, and commit/push. Feedback loops allow revisiting earlier phases if later phases reveal issues.

**Tech Stack:** Go 1.24+, Gin, PostgreSQL/pgx, Redis, Prometheus, OpenTelemetry, Docker/Podman, testify, quic-go, Brotli

**Spec:** `docs/superpowers/specs/2026-03-16-complete-remediation-design.md`

**Resource constraint:** ALL test/build commands MUST use `GOMAXPROCS=2 nice -n 19 ionice -c 3` prefix to limit to 30-40% host resources.

---

## Chunk 1: Phase 0 — Prerequisites

### Task 1: Capture Benchmark Baseline

**Files:**
- Create: `reports/benchmarks/baseline-2026-03-16.txt`

- [ ] **Step 1: Create reports directory**

```bash
mkdir -p reports/benchmarks
```

- [ ] **Step 2: Run benchmarks and capture baseline**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -bench=. -benchmem -benchtime=3s -count=1 -timeout=30m ./internal/... 2>&1 | tee reports/benchmarks/baseline-2026-03-16.txt
```

Expected: Benchmark results captured to file. Some benchmarks may skip in short mode — that is acceptable for baseline.

- [ ] **Step 3: Commit baseline**

```bash
git add reports/benchmarks/baseline-2026-03-16.txt
git commit -m "chore(benchmark): capture pre-remediation baseline"
```

---

### Task 2: Synchronize CONSTITUTION.md to v1.2.0

**Files:**
- Modify: `CONSTITUTION.md` (currently v1.0.0, 20 rules → v1.2.0, 26 rules)

- [ ] **Step 1: Read current CONSTITUTION.md and CLAUDE.md Constitution section**

Read `CONSTITUTION.md` (v1.0.0, 20 rules) and the Constitution section in `CLAUDE.md` (v1.2.0, 26 rules). Identify the 6 missing rules.

- [ ] **Step 2: Update CONSTITUTION.md**

Update version to 1.2.0, date to 2026-03-16, rule count to 26. Add 6 missing rules:

| ID | Category | Title |
|---|---|---|
| CONST-011a | Containerization | Mandatory Container Rebuild |
| CONST-014a | Testing | Infrastructure Before Tests |
| CONST-022a | Configuration | Non-Interactive Execution |
| CONST-023 | Networking | HTTP/3 (QUIC) with Brotli Compression |
| CONST-024 | Resource Management | Test and Challenge Resource Limits |
| CONST-015a | Containerization | Mandatory Container Orchestration Flow |

Note: CONST-015a and CONST-022 already exist in CONSTITUTION.json. The 4 truly missing from JSON are: Mandatory Container Rebuild, Non-Interactive Execution, HTTP/3, Resource Limits. Align numbering with existing JSON IDs where possible.

Update the header summary line to: `Constitution with 26 rules (26 mandatory) across categories: Quality: 2, Safety: 1, Security: 1, Performance: 2, Containerization: 3, Configuration: 1, Testing: 4, Documentation: 2, Principles: 2, Stability: 1, Observability: 1, GitOps: 2, CI/CD: 1, Architecture: 1, Networking: 1, Resource Management: 1`

- [ ] **Step 3: Verify sync with CLAUDE.md**

Verify that every rule in CLAUDE.md's Constitution section has a corresponding entry in CONSTITUTION.md and vice versa. Count must match: 26.

- [ ] **Step 4: Commit**

```bash
git add CONSTITUTION.md
git commit -m "docs(constitution): synchronize CONSTITUTION.md to v1.2.0 with all 26 rules"
```

---

### Task 3: Synchronize CONSTITUTION.json to 26 rules

**Files:**
- Modify: `CONSTITUTION.json` (currently 22 rules → 26 rules)

- [ ] **Step 1: Add 4 missing rules to CONSTITUTION.json**

Add JSON entries for:
1. `CONST-011a` — Mandatory Container Rebuild (Containerization, priority 1)
2. `CONST-023` — HTTP/3 (QUIC) with Brotli Compression (Networking, priority 1)
3. `CONST-024` — Test and Challenge Resource Limits (Resource Management, priority 1)
4. `CONST-025` — Non-Interactive Execution (Configuration, priority 1)

Update summary line to match 26 rules.

- [ ] **Step 2: Verify JSON is valid**

```bash
python3 -c "import json; json.load(open('CONSTITUTION.json')); print('Valid JSON')"
```

Expected: `Valid JSON`

- [ ] **Step 3: Verify rule count**

```bash
python3 -c "import json; d=json.load(open('CONSTITUTION.json')); print(f'Rules: {len(d[\"rules\"])}')"
```

Expected: `Rules: 26`

- [ ] **Step 4: Commit**

```bash
git add CONSTITUTION.json
git commit -m "docs(constitution): synchronize CONSTITUTION.json to 26 rules"
```

---

### Task 4: Scan for Mocks/Stubs in Production Code

**Files:**
- No files created; this is a verification scan

- [ ] **Step 1: Scan for mock patterns in non-test production files**

```bash
grep -rn --include="*.go" -E "(mock|Mock|MOCK|stub|Stub|STUB|fake|Fake|FAKE|placeholder|Placeholder)" internal/ --exclude="*_test.go" --exclude-dir="testutil" --exclude-dir="testing" | grep -v "// " | grep -v "vendor/" | head -50
```

Expected: Either no results, or results that are legitimate (e.g., variable names like `mockLLM` in production code that actually connects to mock-llm test infrastructure, or interface names like `MockProvider` that are test helpers accidentally in non-test files).

- [ ] **Step 2: Investigate any findings**

For each finding, verify it's either:
- A legitimate production reference (e.g., connecting to mock-llm container)
- A false positive (e.g., comment, documentation string)
- A violation that needs fixing

- [ ] **Step 3: Fix violations if any**

Remove or replace any mock/stub/placeholder patterns in production code.

- [ ] **Step 4: Commit if changes made**

```bash
git add -A internal/
git commit -m "fix(quality): remove mock/stub patterns from production code"
```

---

## Chunk 2: Phase 1 — Safety & Concurrency Fixes

### Task 5: Fix Discovery Handler Goroutine Leak

**Files:**
- Modify: `internal/handlers/discovery_handler.go:172-179`
- Test: `internal/handlers/discovery_handler_test.go`

- [ ] **Step 1: Read the full discovery handler file to understand context**

Read `internal/handlers/discovery_handler.go` to understand the `discoveryService` interface and how `Start()` is called.

- [ ] **Step 2: Fix the goroutine to use request-derived context**

Replace the bare `go func()` at line 175 with a context-aware version:

```go
// Trigger discovery in background with bounded lifecycle.
// Use a detached context with timeout since the HTTP request context
// will cancel when the response is sent, but discovery should continue.
discoveryCtx, discoveryCancel := context.WithTimeout(context.Background(), 30*time.Minute)
go func() {
    defer discoveryCancel()
    if err := h.discoveryService.Start(credentials); err != nil {
        log.Printf("Discovery trigger failed: %v", err)
    }
}()
```

Note: We keep `context.Background()` here because discovery must outlive the HTTP request. The fix is adding a timeout and ensuring cancel is deferred. The discovery service already manages its own lifecycle via `stopCh/wg` as noted in the comment.

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/handlers/...
```

Expected: No errors

- [ ] **Step 4: Run existing tests**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/handlers/ -run TestDiscovery -v
```

Expected: All existing tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/discovery_handler.go
git commit -m "fix(safety): add timeout context to discovery handler goroutine"
```

---

### Task 6: Fix Model Metadata Handler Background Refresh

**Files:**
- Modify: `internal/handlers/model_metadata.go:205-211`

- [ ] **Step 1: Add deduplication and proper context**

Replace the goroutine at line 205 with a deduplicated, bounded version. Add a `refreshMu` and `refreshInProgress` field to the handler struct (if not already present):

```go
// Deduplicate concurrent refresh requests
if !h.tryStartRefresh() {
    c.JSON(http.StatusAccepted, RefreshResponse{
        Status:  "accepted",
        Message: "Full models refresh already in progress",
    })
    return
}

go func() {
    defer h.finishRefresh()
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()
    if err := h.service.RefreshModels(ctx); err != nil {
        logrus.WithError(err).Error("Background model refresh failed")
    }
}()
```

Add helper methods:
```go
func (h *ModelMetadataHandler) tryStartRefresh() bool {
    h.refreshMu.Lock()
    defer h.refreshMu.Unlock()
    if h.refreshInProgress {
        return false
    }
    h.refreshInProgress = true
    return true
}

func (h *ModelMetadataHandler) finishRefresh() {
    h.refreshMu.Lock()
    defer h.refreshMu.Unlock()
    h.refreshInProgress = false
}
```

- [ ] **Step 2: Add fields to handler struct**

Add to the `ModelMetadataHandler` struct:
```go
refreshMu         sync.Mutex
refreshInProgress bool
```

And add `"sync"` to imports.

- [ ] **Step 3: Verify compilation and run tests**

```bash
go build ./internal/handlers/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/handlers/ -run TestModelMetadata -v
```

Expected: Build succeeds, tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/model_metadata.go
git commit -m "fix(safety): deduplicate model refresh and add timeout to goroutine"
```

---

### Task 7: Fix Background Task Handler SSE Channel Race

**Files:**
- Modify: `internal/handlers/background_task_handler.go:409-430`

- [ ] **Step 1: Add safe channel wrapper**

Add a `safeClientChan` wrapper that prevents sends after the stream ends:

```go
// Create client channel with safe close semantics
clientChan := make(chan []byte, 100)
closed := make(chan struct{})

// Register client
if h.sseManager != nil {
    _ = h.sseManager.RegisterClient(taskID, clientChan)
    defer func() {
        close(closed)
        _ = h.sseManager.UnregisterClient(taskID, clientChan)
        // Drain remaining messages to prevent goroutine leaks in senders
        for range clientChan {
        }
    }()
}

// Stream events
c.Stream(func(w io.Writer) bool {
    select {
    case msg, ok := <-clientChan:
        if !ok {
            return false
        }
        c.SSEvent("message", string(msg))
        return true
    case <-c.Request.Context().Done():
        return false
    case <-closed:
        return false
    }
})
```

- [ ] **Step 2: Verify compilation and run tests**

```bash
go build ./internal/handlers/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/handlers/ -run TestBackgroundTask -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/handlers/background_task_handler.go
git commit -m "fix(safety): prevent SSE channel race on client disconnect"
```

---

### Task 8: Fix ACP Handler Missing Shutdown Method

**Files:**
- Modify: `internal/handlers/acp_handler.go`

- [ ] **Step 1: Read the full ACP handler to understand struct and sessionCleanupWorker**

Read `internal/handlers/acp_handler.go` fully to find the `ACPHandler` struct definition and `sessionCleanupWorker` method.

- [ ] **Step 2: Add WaitGroup and Shutdown method**

Add `cleanupWg sync.WaitGroup` field to `ACPHandler` struct.

Update `NewACPHandler` to track the goroutine:

```go
h.cleanupWg.Add(1)
go func() {
    defer h.cleanupWg.Done()
    h.sessionCleanupWorker()
}()
```

Add `Shutdown` method:

```go
// Shutdown gracefully stops the ACP handler's background workers.
func (h *ACPHandler) Shutdown() {
    close(h.stopCleanup)
    h.cleanupWg.Wait()
}
```

- [ ] **Step 3: Verify compilation and run tests**

```bash
go build ./internal/handlers/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/handlers/ -run TestACP -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/handlers/acp_handler.go
git commit -m "fix(safety): add Shutdown method to ACP handler for goroutine lifecycle"
```

---

### Task 9: Fix Cache Invalidation Goroutine Lifecycle

**Files:**
- Modify: `internal/cache/invalidation.go:221-247`

- [ ] **Step 1: Add WaitGroup to EventDrivenInvalidation**

Add `wg sync.WaitGroup` field to the struct. Update `Start()`:

```go
func (i *EventDrivenInvalidation) Start() {
    if i.eventBus == nil {
        return
    }

    ch := i.eventBus.SubscribeAll()

    i.wg.Add(1)
    go func() {
        defer i.wg.Done()
        for {
            select {
            case <-i.ctx.Done():
                return
            case event, ok := <-ch:
                if !ok {
                    return
                }
                i.handleEvent(i.ctx, event)
            }
        }
    }()
}
```

Update `Stop()` to wait:

```go
func (i *EventDrivenInvalidation) Stop() {
    i.cancel()
    i.wg.Wait()
}
```

- [ ] **Step 2: Verify compilation and run tests**

```bash
go build ./internal/cache/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/cache/ -run TestEventDriven -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/cache/invalidation.go
git commit -m "fix(safety): add WaitGroup to cache invalidation goroutine lifecycle"
```

---

### Task 10: Fix Debate Log Cleanup Worker Lifecycle

**Files:**
- Modify: `internal/database/debate_log_repository.go:418-445`

- [ ] **Step 1: Add lifecycle tracking fields**

Add to `DebateLogRepository` struct:
```go
cleanupWg      sync.WaitGroup
cleanupRunning atomic.Bool
```

Update `StartCleanupWorker`:

```go
func (r *DebateLogRepository) StartCleanupWorker(ctx context.Context, interval time.Duration) {
    if !r.cleanupRunning.CompareAndSwap(false, true) {
        r.log.Warn("Cleanup worker already running")
        return
    }
    r.cleanupWg.Add(1)
    go func() {
        defer r.cleanupWg.Done()
        defer r.cleanupRunning.Store(false)
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        r.log.WithFields(logrus.Fields{
            "interval": interval,
        }).Info("Starting debate log cleanup worker")

        for {
            select {
            case <-ctx.Done():
                r.log.Info("Stopping debate log cleanup worker")
                return
            case <-ticker.C:
                deleted, err := r.CleanupExpiredLogs(ctx)
                if err != nil {
                    r.log.WithError(err).Error("Failed to cleanup expired debate logs")
                } else if deleted > 0 {
                    r.log.WithFields(logrus.Fields{
                        "deleted_count": deleted,
                    }).Info("Debate log cleanup completed")
                }
            }
        }
    }()
}

// StopCleanupWorker waits for the cleanup worker to finish.
func (r *DebateLogRepository) StopCleanupWorker() {
    r.cleanupWg.Wait()
}
```

Add imports: `"sync"`, `"sync/atomic"`.

- [ ] **Step 2: Verify compilation and run tests**

```bash
go build ./internal/database/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/database/ -run TestDebateLog -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/database/debate_log_repository.go
git commit -m "fix(safety): add lifecycle tracking to debate log cleanup worker"
```

---

### Task 11: Fix Cache Expiration Manager WaitGroup

**Files:**
- Modify: `internal/cache/expiration.go:82-93`

- [ ] **Step 1: Add WaitGroup tracking**

Add `wg sync.WaitGroup` field to `ExpirationManager` struct. Update `Start()`:

```go
func (m *ExpirationManager) Start() {
    m.wg.Add(1)
    go func() {
        defer m.wg.Done()
        m.cleanupLoop()
    }()
    if m.config.EnableValidation {
        m.wg.Add(1)
        go func() {
            defer m.wg.Done()
            m.validationLoop()
        }()
    }
}

func (m *ExpirationManager) Stop() {
    m.cancel()
    m.wg.Wait()
}
```

- [ ] **Step 2: Verify compilation and run tests**

```bash
go build ./internal/cache/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/cache/ -run TestExpiration -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/cache/expiration.go
git commit -m "fix(safety): add WaitGroup to expiration manager goroutine lifecycle"
```

---

### Task 12: Fix SpecAdapter Nil Receiver Safety

**Files:**
- Modify: `internal/adapters/specifier/adapter.go:14-20`

- [ ] **Step 1: Return no-op adapter instead of nil**

Replace `NewSpecAdapter`:

```go
// NewSpecAdapter creates a new spec adapter wrapping a SpecEngine.
// Returns a no-op adapter if engine is nil (safe to call any method on).
func NewSpecAdapter(engine helixspec.SpecEngine) *SpecAdapter {
    return &SpecAdapter{engine: engine}
}
```

Add nil checks to each method:

```go
func (a *SpecAdapter) ClassifyEffort(ctx context.Context, request string) (*helixspec.EffortClassification, error) {
    if a.engine == nil {
        return nil, fmt.Errorf("spec engine not initialized")
    }
    return a.engine.ClassifyEffort(ctx, request)
}
```

Apply same pattern to `ExecuteFlow`, `ResumeFlow`, and any other methods.

- [ ] **Step 2: Verify compilation and run tests**

```bash
go build ./internal/adapters/specifier/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/adapters/specifier/ -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/adapters/specifier/adapter.go
git commit -m "fix(safety): return no-op adapter instead of nil for SpecAdapter"
```

---

### Task 13: Create Goroutine Leak Verification Test

**Files:**
- Create: `internal/handlers/goroutine_leak_test.go`

- [ ] **Step 1: Write goroutine count verification test**

```go
package handlers_test

import (
    "runtime"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

func TestNoGoroutineLeakAfterHandlerRequest(t *testing.T) {
    // Allow goroutines to settle
    runtime.GC()
    time.Sleep(100 * time.Millisecond)
    baseline := runtime.NumGoroutine()

    // Exercise handler endpoints that spawn goroutines
    // (discovery trigger, model refresh, SSE stream, ACP session)
    // ... test-specific handler invocations here ...

    // Allow goroutines to complete
    runtime.GC()
    time.Sleep(500 * time.Millisecond)
    after := runtime.NumGoroutine()

    // Goroutine count should not grow beyond baseline + small buffer
    assert.LessOrEqual(t, after, baseline+2,
        "Goroutine leak detected: before=%d after=%d", baseline, after)
}
```

- [ ] **Step 2: Run the test**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/handlers/ -run TestNoGoroutineLeak -v
```

Expected: Test passes, confirming no goroutine leaks after handler requests.

- [ ] **Step 3: Commit**

```bash
git add internal/handlers/goroutine_leak_test.go
git commit -m "test(safety): add goroutine leak verification test for handlers"
```

---

### Task 14: Run Full Safety Verification

- [ ] **Step 1: Run race detector on all affected packages**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=10m ./internal/handlers/... ./internal/cache/... ./internal/database/... ./internal/adapters/specifier/... -v 2>&1 | tail -20
```

Expected: All tests pass with race detector enabled, no race conditions detected.

- [ ] **Step 2: Run full build verification**

```bash
go build ./...
```

Expected: No compilation errors

- [ ] **Step 3: Run fmt/vet**

```bash
make fmt && make vet
```

Expected: No issues

---

## Chunk 3: Phase 1b — Dead Code Cleanup

### Task 14: Scan for Dead Code

**Files:**
- No new files; analysis only

- [ ] **Step 1: Find exported functions never referenced outside their package**

Use the `deadcode` tool from golang.org/x/tools for reliable detection:

```bash
go install golang.org/x/tools/cmd/deadcode@latest
GOMAXPROCS=2 nice -n 19 ionice -c 3 deadcode ./internal/... 2>&1 | head -50
```

If `deadcode` is unavailable, fall back to `go vet` and grep-based analysis:

```bash
grep -rn "^func [A-Z]" internal/ --include="*.go" --exclude="*_test.go" | wc -l
GOMAXPROCS=2 nice -n 19 ionice -c 3 go vet ./internal/... 2>&1 | head -30
```

Note: `go vet` alone cannot reliably detect unused exported symbols. `deadcode` is the recommended tool for this. Interface implementations and protobuf-generated code are expected false positives — exclude them from analysis.

- [ ] **Step 2: Check for unreferenced adapter implementations**

```bash
# Check each adapter package for types that aren't imported
for dir in internal/adapters/*/; do
    pkg=$(basename "$dir")
    echo "=== $pkg ==="
    grep -rn "adapter\.$pkg\|adapters/$pkg" internal/ --include="*.go" --exclude-dir="adapters" | head -5
done
```

- [ ] **Step 3: Remove confirmed dead code**

For each confirmed dead symbol:
- Remove the function/type
- Remove any imports that become unused
- Verify compilation: `go build ./internal/...`

- [ ] **Step 4: Commit**

```bash
git add internal/
git commit -m "refactor(quality): remove dead code per CONST-006"
```

---

## Chunk 4: Phase 2 — Test Coverage Expansion

### Task 15: Create Tests for internal/adapters/background/

**Files:**
- Create: `internal/adapters/background/adapter_test.go`
- Create: `internal/adapters/background/converter_test.go`
- Create: `internal/adapters/background/event_publisher_adapter_test.go`
- Create: `internal/adapters/background/repository_test.go`

- [ ] **Step 1: Write adapter_test.go with TaskQueueAdapter tests**

Write comprehensive table-driven tests covering:
- `NewTaskQueueAdapter` — constructor
- `Enqueue` — successful enqueue, nil task, context cancellation
- `Dequeue` — successful dequeue, empty queue, worker ID validation
- `Peek` — peek with count, empty queue
- All other `TaskQueueAdapter` methods
- `WorkerPoolAdapter` methods
- `ResourceMonitorAdapter` methods

Use mock implementations of the extracted interfaces for unit testing.

- [ ] **Step 2: Write converter_test.go with table-driven conversion tests**

Test all converter functions:
- `convertToInternalTask` — normal, nil input, malformed data
- `convertToExtractedTask` — normal, nil input
- `convertTaskPriority` — all priority levels
- `convertTaskStatus` — all status values
- Edge cases: empty strings, zero values, maximum values

- [ ] **Step 3: Write event_publisher_adapter_test.go**

Test `EventPublisherAdapter`:
- `Publish` — successful publish, nil event
- `Subscribe` — subscribe and receive events
- Error handling paths

- [ ] **Step 4: Write repository_test.go**

Test `RepositoryAdapter`:
- CRUD operations
- Error paths
- Nil handling

- [ ] **Step 5: Run all new tests**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./internal/adapters/background/... -v
```

Expected: All tests pass

- [ ] **Step 6: Check coverage**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -cover -count=1 -timeout=5m ./internal/adapters/background/...
```

Expected: Coverage >= 80%

- [ ] **Step 7: Commit**

```bash
git add internal/adapters/background/*_test.go
git commit -m "test(coverage): add comprehensive tests for internal/adapters/background"
```

---

### Task 16: Expand Tests for Under-Covered Packages

**Files:**
- Create/Modify: test files in 8 packages

For each of the 8 under-covered packages, follow this pattern:

- [ ] **Step 1: internal/utils — add missing test files**

Read all `.go` files in `internal/utils/` to identify untested functions. Create test files for each untested file. Run:

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -cover -count=1 -timeout=5m ./internal/utils/... -v
```

- [ ] **Step 2: internal/knowledge — add missing test files**

Same pattern as Step 1 for `internal/knowledge/`.

- [ ] **Step 3: internal/planning — add missing test files**

Same pattern for `internal/planning/`.

- [ ] **Step 4: internal/models — add missing test files**

Same pattern for `internal/models/`.

- [ ] **Step 5: internal/observability — add missing test files**

Same pattern for `internal/observability/`.

- [ ] **Step 6: internal/llmops — add missing test files**

Same pattern for `internal/llmops/`.

- [ ] **Step 7: internal/selfimprove — add missing test files**

Same pattern for `internal/selfimprove/`.

- [ ] **Step 8: internal/optimization — add missing test files**

Same pattern for `internal/optimization/`.

- [ ] **Step 9: Verify all packages meet coverage floor**

```bash
for pkg in utils knowledge planning models observability llmops selfimprove optimization; do
    echo "=== $pkg ==="
    GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -cover -count=1 -timeout=5m ./internal/$pkg/... 2>&1 | grep -E "coverage|FAIL"
done
```

Expected: All packages show >= 80% coverage, no failures.

- [ ] **Step 10: Commit**

```bash
git add internal/utils/ internal/knowledge/ internal/planning/ internal/models/ internal/observability/ internal/llmops/ internal/selfimprove/ internal/optimization/
git commit -m "test(coverage): expand test coverage for 8 under-covered packages"
```

---

### Task 17: Create New Challenge Scripts

**Files:**
- Create: `challenges/scripts/goroutine_lifecycle_challenge.sh`
- Create: `challenges/scripts/adapter_coverage_challenge.sh`
- Create: `challenges/scripts/race_condition_challenge.sh`

- [ ] **Step 1: Create goroutine lifecycle challenge**

Write a challenge script that:
1. Builds the project
2. Verifies all handler types with goroutines have `Shutdown()`/`Stop()` methods
3. Verifies all `go func()` in handlers/ have either context or WaitGroup
4. Runs race detector on handler tests

Follow the pattern from existing challenge scripts (source `common.sh`, use `assert_*` functions).

- [ ] **Step 2: Create adapter coverage challenge**

Write a challenge script that:
1. Runs `go test -cover` on all adapter packages
2. Asserts coverage >= 80% for each
3. Reports any package below threshold

- [ ] **Step 3: Create race condition challenge**

Write a challenge script that:
1. Runs `go test -race` on critical packages (handlers, cache, database, services)
2. Asserts zero race conditions detected
3. Reports any failures

- [ ] **Step 4: Make scripts executable and test**

```bash
chmod +x challenges/scripts/goroutine_lifecycle_challenge.sh challenges/scripts/adapter_coverage_challenge.sh challenges/scripts/race_condition_challenge.sh
```

- [ ] **Step 5: Commit**

```bash
git add challenges/scripts/goroutine_lifecycle_challenge.sh challenges/scripts/adapter_coverage_challenge.sh challenges/scripts/race_condition_challenge.sh
git commit -m "test(challenges): add goroutine lifecycle, adapter coverage, and race condition challenges"
```

---

## Chunk 5: Phase 3 — Security Scanning & Remediation

### Task 18: Run Non-Containerized Security Scanners

**Files:**
- Create: `reports/security/gosec-report-2026-03-16.json`
- Create: `reports/security/govet-report-2026-03-16.txt`

- [ ] **Step 1: Run gosec**

```bash
mkdir -p reports/security
which gosec || go install github.com/securego/gosec/v2/cmd/gosec@latest
GOMAXPROCS=2 nice -n 19 ionice -c 3 gosec -fmt=json -out=reports/security/gosec-report-2026-03-16.json ./internal/... 2>&1 || true
echo "Gosec scan complete"
```

- [ ] **Step 2: Run go vet**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go vet ./internal/... 2>&1 | tee reports/security/govet-report-2026-03-16.txt
```

- [ ] **Step 3: Analyze gosec findings**

Read the gosec report, categorize by severity. Fix all HIGH and MEDIUM findings. Document LOW findings with justification if not fixed.

- [ ] **Step 4: Fix identified issues**

For each finding requiring a fix, make the change, verify compilation, run affected tests.

- [ ] **Step 5: Commit fixes**

```bash
git add internal/ reports/security/
git commit -m "fix(security): remediate gosec and go vet findings"
```

---

### Task 19: Run Containerized Security Scanners (Snyk/SonarQube)

**Files:**
- Create: `reports/security/snyk-report-2026-03-16.json`

- [ ] **Step 1: Check if Snyk container infrastructure exists**

```bash
ls docker/security/snyk/docker-compose.yml docker/security/sonarqube/docker-compose.yml
cat .snyk | head -10
```

- [ ] **Step 2: Run Snyk via make target (if SNYK_TOKEN available)**

```bash
if [ -n "${SNYK_TOKEN:-}" ]; then
    make security-scan-snyk 2>&1 | tee reports/security/snyk-report-2026-03-16.txt
else
    echo "SNYK_TOKEN not set - skipping containerized Snyk scan. Gosec results used instead."
fi
```

- [ ] **Step 3: Analyze and remediate any additional findings**

Fix critical/high findings from Snyk if available. Update `.snyk` ignore rules only for verified false positives.

- [ ] **Step 4: Commit**

```bash
git add reports/security/ .snyk
git commit -m "fix(security): analyze and remediate security scan findings"
```

---

## Chunk 6: Phase 4 — Performance & Non-Blocking Optimization

### Task 20: Audit and Expand Lazy Loading

**Files:**
- Modify: Various service constructors as identified

- [ ] **Step 1: Audit all service constructors for blocking I/O**

```bash
grep -rn "func New" internal/services/ --include="*.go" --exclude="*_test.go" | head -40
```

For each constructor, check if it performs network I/O, file I/O, or database calls during construction.

- [ ] **Step 2: Apply LazyProvider pattern where needed**

For any service that performs blocking I/O during construction, wrap initialization using the `LazyProvider` pattern from `internal/llm/lazy_provider.go`.

- [ ] **Step 3: Verify no regression**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=10m ./internal/services/... -v 2>&1 | tail -20
```

- [ ] **Step 4: Commit**

```bash
git add internal/
git commit -m "perf(lazy): expand lazy loading to service constructors with blocking I/O"
```

---

### Task 21: Expand Semaphore Controls

**Files:**
- Modify: `internal/mcp/adapters/` (if unbounded concurrency found)
- Modify: `internal/formatters/` (if unbounded concurrency found)
- Modify: `internal/rag/` (if unbounded concurrency found)

- [ ] **Step 1: Audit for unbounded go func() spawns in production code**

```bash
grep -rn "go func()" internal/ --include="*.go" --exclude="*_test.go" | grep -v "vendor/" | wc -l
```

For each, check if it has a semaphore or bounded concurrency control.

- [ ] **Step 2: Add semaphore limits where missing**

Use `golang.org/x/sync/semaphore` weighted semaphore pattern (already used in `internal/services/provider_registry.go`).

- [ ] **Step 3: Verify and commit**

```bash
go build ./internal/... && GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=10m ./internal/... -short 2>&1 | tail -10
git add internal/
git commit -m "perf(concurrency): add semaphore controls to unbounded concurrent operations"
```

---

### Task 22: HTTP/3 (QUIC) & Brotli Compliance Verification

**Files:**
- Verify: `go.mod` for quic-go and brotli dependencies

- [ ] **Step 1: Check existing HTTP/3 and Brotli support**

```bash
grep -r "quic-go\|quic_go\|http3" go.mod internal/ --include="*.go" | head -20
grep -r "brotli\|andybalholm" go.mod internal/ --include="*.go" | head -20
```

- [ ] **Step 2: Verify HTTP server configuration**

Check `internal/http/` and `cmd/` for HTTP server setup. Verify HTTP/3 is configured as primary with HTTP/2 fallback.

- [ ] **Step 3: Verify HTTP client configuration**

Check all HTTP client creation points. Verify they prefer HTTP/3.

- [ ] **Step 4: Remediate if gaps found**

If HTTP/3 or Brotli support is missing, add it following existing patterns. If already present, document verification.

- [ ] **Step 5: Create HTTP/3 negotiation verification test**

**Files:**
- Create: `tests/integration/http3_negotiation_test.go`

```go
func TestHTTP3NegotiationSupported(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping HTTP/3 negotiation test in short mode")
    }
    // Verify HTTP server advertises HTTP/3 via Alt-Svc header
    // Verify HTTP client supports QUIC transport
    // Verify Brotli Accept-Encoding is sent by clients
    // Verify Brotli Content-Encoding is returned by server when supported
}
```

- [ ] **Step 6: Run and commit**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./tests/integration/ -run TestHTTP3 -v
git add internal/ go.mod go.sum tests/integration/http3_negotiation_test.go
git commit -m "feat(networking): verify and ensure HTTP/3 QUIC with Brotli compliance"
```

---

## Chunk 7: Phase 5 — Monitoring & Metrics Tests

### Task 23: Create Prometheus Metrics Validation Tests

**Files:**
- Create: `tests/integration/monitoring_metrics_test.go`

- [ ] **Step 1: Write test that validates Prometheus metric registration**

```go
func TestPrometheusMetricsRegistered(t *testing.T) {
    // Verify all custom metrics are registered
    // Check counter, gauge, histogram families exist
    // Verify metric names follow naming conventions
}
```

- [ ] **Step 2: Write test for metrics endpoint**

```go
func TestMetricsEndpointReturnsData(t *testing.T) {
    // Start a minimal server
    // Hit /metrics endpoint
    // Verify expected metric families are present
}
```

- [ ] **Step 3: Run tests**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./tests/integration/ -run TestPrometheus -v
```

- [ ] **Step 4: Commit**

```bash
git add tests/integration/monitoring_metrics_test.go
git commit -m "test(monitoring): add Prometheus metrics validation tests"
```

---

### Task 24: Create OpenTelemetry Trace Tests

**Files:**
- Create: `tests/integration/otel_tracing_test.go`

- [ ] **Step 1: Write test for trace span creation**

Test that LLM requests create proper trace spans with GenAI attributes.

- [ ] **Step 2: Write test for custom attribute propagation**

Test that HelixAgent-specific attributes (helix.request.id, etc.) are properly set.

- [ ] **Step 3: Run and commit**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./tests/integration/ -run TestOtel -v
git add tests/integration/otel_tracing_test.go
git commit -m "test(monitoring): add OpenTelemetry trace validation tests"
```

---

### Task 24b: Grafana Dashboard Data Validation

Note: Direct Grafana scrape testing requires running Grafana infrastructure. The `/metrics` endpoint tests in Task 23 verify that the data Grafana dashboards consume is correctly exposed. Full Grafana integration validation is covered by `make test-with-infra` which starts the monitoring stack. This is verified in Phase 9 validation.

---

### Task 25: Create Health Check Validation Tests

**Files:**
- Create: `tests/integration/health_check_validation_test.go`

- [ ] **Step 1: Write comprehensive health check tests**

Test all health endpoints, degradation when dependencies fail, circuit breaker state reflection.

- [ ] **Step 2: Run and commit**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=5m ./tests/integration/ -run TestHealthCheck -v
git add tests/integration/health_check_validation_test.go
git commit -m "test(monitoring): add health check validation tests"
```

---

## Chunk 8: Phase 6 — Stress & Integration Test Hardening

### Task 26: Create Extreme Stress Test Scenarios

**Files:**
- Create: `tests/stress/extreme_load_test.go`

- [ ] **Step 1: Write 10x concurrent load test**

```go
func TestExtreme10xConcurrentLoad(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping extreme stress test in short mode")
    }
    // Set GOMAXPROCS=2 constraint
    runtime.GOMAXPROCS(2)
    // Launch 10x normal concurrent requests
    // Verify all complete or gracefully degrade
    // Verify no panics, no OOM, no deadlocks
}
```

- [ ] **Step 2: Write provider cascade failure test**

```go
func TestProviderCascadeFailure(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping cascade failure test in short mode")
    }
    // Simulate all providers failing simultaneously
    // Verify graceful error response, not crash
}
```

- [ ] **Step 3: Write memory pressure test**

```go
func TestMemoryPressureGracefulDegradation(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping memory pressure test in short mode")
    }
    // Allocate memory to create GC pressure
    // Run requests concurrently
    // Verify system degrades gracefully
}
```

- [ ] **Step 4: Write connection pool exhaustion test**

```go
func TestConnectionPoolExhaustion(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping pool exhaustion test in short mode")
    }
    // Exhaust all DB connections
    // Verify timeout errors, not panics
}
```

- [ ] **Step 5: Write P99 latency measurement test**

```go
func TestP99LatencyBaseline(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping latency baseline test in short mode")
    }
    runtime.GOMAXPROCS(2)

    var durations []time.Duration
    // Run N requests and record durations
    for i := 0; i < 100; i++ {
        start := time.Now()
        // ... exercise a handler endpoint ...
        durations = append(durations, time.Since(start))
    }

    // Sort and compute P99
    sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
    p99 := durations[int(float64(len(durations))*0.99)]
    t.Logf("P99 latency: %v (from %d samples)", p99, len(durations))

    // Store baseline
    os.MkdirAll("../../reports/latency", 0o755)
    os.WriteFile("../../reports/latency/p99-baseline-2026-03-16.txt",
        []byte(fmt.Sprintf("P99: %v\nSamples: %d\n", p99, len(durations))), 0o644)
}
```

- [ ] **Step 6: Run stress tests**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -timeout=15m -p 1 ./tests/stress/ -run TestExtreme -v
```

- [ ] **Step 7: Commit**

```bash
git add tests/stress/extreme_load_test.go reports/latency/
git commit -m "test(stress): add extreme stress scenarios with P99 latency baseline"
```

---

### Task 27: Create Chaos Engineering Tests

**Files:**
- Create: `tests/stress/chaos_engineering_test.go`

- [ ] **Step 1: Write chaos tests**

Tests that simulate:
- Redis failure mid-request → verify fallback to in-memory cache
- Network timeout simulation → verify circuit breaker activation
- Concurrent goroutine spawning → verify semaphore limits hold

- [ ] **Step 2: Run and commit**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -timeout=15m -p 1 ./tests/stress/ -run TestChaos -v
git add tests/stress/chaos_engineering_test.go
git commit -m "test(stress): add chaos engineering tests for service failure scenarios"
```

---

## Chunk 9: Phase 7 — Documentation Updates

### Task 28: Update Core Documentation Files

**Files:**
- Modify: `CLAUDE.md` (add safety patterns, new challenges, security scan references)
- Modify: `AGENTS.md` (sync with new capabilities)
- Modify: `CONSTITUTION.md` (already updated in Task 2)
- Modify: `docs/MODULES.md` (if any module changes)

- [ ] **Step 1: Update CLAUDE.md**

Add to the appropriate sections:
- New challenges: goroutine_lifecycle, adapter_coverage, race_condition
- Safety patterns: WaitGroup lifecycle for goroutines, deduplication for background refreshes
- Security scan execution results summary
- Updated Constitution reference to v1.2.0 with 26 rules

- [ ] **Step 2: Update AGENTS.md**

Ensure AGENTS.md reflects all new capabilities, patterns, and challenge scripts added during remediation.

- [ ] **Step 3: Verify sync between CLAUDE.md, AGENTS.md, CONSTITUTION.md**

Check that all three documents reference the same version, rules, and capabilities. No contradictions.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md AGENTS.md CONSTITUTION.md docs/MODULES.md
git commit -m "docs(core): update CLAUDE.md, AGENTS.md with remediation changes and sync Constitution"
```

---

### Task 29: Update Architecture Diagrams

**Files:**
- Modify: `docs/diagrams/src/security-scanning-pipeline.puml`
- Modify: `docs/diagrams/src/test-pyramid.puml`
- Create: `docs/diagrams/src/goroutine-lifecycle.puml`

- [ ] **Step 1: Create goroutine lifecycle diagram**

PlantUML diagram showing the pattern: Handler creates goroutine → WaitGroup.Add(1) → goroutine runs with context → defer WaitGroup.Done() → Shutdown() calls cancel + WaitGroup.Wait().

- [ ] **Step 2: Update test pyramid diagram**

Add new test types if not already present: monitoring metrics tests, chaos engineering tests.

- [ ] **Step 3: Update security scanning pipeline**

Reflect the actual scanners used and the scan-fix-rescan cycle.

- [ ] **Step 4: Commit**

```bash
git add docs/diagrams/
git commit -m "docs(diagrams): add goroutine lifecycle diagram, update test pyramid and security pipeline"
```

---

## Chunk 10: Phase 8 — Video Course & Website Updates

### Task 30: Create New Video Course — Goroutine Safety

**Files:**
- Create: `Website/video-courses/courses-31-40/course-31-goroutine-safety.md`

Note: Course 26 already exists as `course-26-caching-strategies.md` in courses-21-30/. New course is numbered 31 in a new `courses-31-40/` directory.

- [ ] **Step 0: Create directory**

```bash
mkdir -p Website/video-courses/courses-31-40
```

- [ ] **Step 1: Write course content**

Course: "Goroutine Safety & Lifecycle Management"
- Module 1: Common goroutine leak patterns
- Module 2: WaitGroup lifecycle pattern
- Module 3: Context propagation best practices
- Module 4: Race condition detection with `-race`
- Module 5: Hands-on lab: fixing a goroutine leak

Follow the format of existing courses (timestamps, hands-on labs, quizzes).

- [ ] **Step 2: Commit**

```bash
git add Website/video-courses/courses-31-40/
git commit -m "docs(courses): add Course 31 — Goroutine Safety & Lifecycle Management"
```

---

### Task 31: Update Existing Video Courses

**Files:**
- Modify: `Website/video-courses/course-06-testing.md`
- Modify: `Website/video-courses/course-10-security-best-practices.md`
- Modify: `Website/video-courses/course-18-security-scanning.md`

- [ ] **Step 1: Update course 6 with new test types**

Add sections on: monitoring metrics tests, chaos engineering tests, race condition detection tests.

- [ ] **Step 2: Update course 10 with security scan execution**

Add section on running the full security scanner suite and analyzing results.

- [ ] **Step 3: Update course 18 with Snyk/SonarQube containerized scanning**

Add detailed walkthrough of containerized scanning setup and execution.

- [ ] **Step 4: Commit**

```bash
git add Website/video-courses/
git commit -m "docs(courses): update courses 6, 10, 18 with remediation content"
```

---

### Task 32: Update User Manuals

**Files:**
- Modify: `Website/user-manuals/17-security-scanning-guide.md`
- Modify: `Website/user-manuals/19-concurrency-patterns.md`
- Modify: `Website/user-manuals/20-testing-strategies.md`

- [ ] **Step 1: Update security scanning guide**

Add actual scan execution results, containerized scanner setup, common findings and fixes.

- [ ] **Step 2: Update concurrency patterns manual**

Add WaitGroup lifecycle pattern, deduplication pattern for background operations, SpecAdapter no-op pattern.

- [ ] **Step 3: Update testing strategies manual**

Add monitoring metrics tests, chaos engineering, extreme stress tests.

- [ ] **Step 4: Commit**

```bash
git add Website/user-manuals/
git commit -m "docs(manuals): update security, concurrency, and testing manuals"
```

---

### Task 33: Update Website Documentation Pages

**Files:**
- Modify: `Website/public/docs/security.html`
- Modify: `Website/public/docs/troubleshooting.html`
- Modify: `Website/public/changelog.html`

- [ ] **Step 1: Update security docs page**

Add security scanning results summary, remediation highlights.

- [ ] **Step 2: Update troubleshooting page**

Add common goroutine leak symptoms and fixes.

- [ ] **Step 3: Update changelog**

Add remediation release entry with all changes.

- [ ] **Step 4: Commit**

```bash
git add Website/public/
git commit -m "docs(website): update security, troubleshooting, and changelog pages"
```

---

## Chunk 11: Phase 9 — Final Validation & Container Rebuild

### Task 34: Mandatory Container Rebuild (CLAUDE.md rule 11a)

Since code was changed in handlers, cache, database, services, and adapters, all affected containers MUST be rebuilt.

- [ ] **Step 1: Rebuild the HelixAgent binary**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 make build
```

Expected: Binary builds successfully at `./bin/helixagent`.

- [ ] **Step 2: Rebuild affected container images**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 make docker-build 2>&1 | tail -10
```

Expected: Container images rebuild with updated code. If `make docker-build` is unavailable, use `make container-build`.

- [ ] **Step 3: Verify containers use updated code**

The HelixAgent binary handles all container orchestration on boot (per CONST-015a). Running the binary starts all containers with the rebuilt images. Verify by checking build timestamps in container logs after next boot cycle.

Note: Do NOT use manual `docker start/stop/restart` commands. All container lifecycle is managed by the HelixAgent binary reading `containers/.env`.

---

### Task 35: Full Build and Lint Verification

- [ ] **Step 1: Run full build**

```bash
make build
```

Expected: Build succeeds with zero errors.

- [ ] **Step 2: Run fmt, vet, lint**

```bash
make fmt && make vet
```

Expected: No issues. If `make lint` requires golangci-lint, run it if available:

```bash
make lint 2>&1 | tail -20 || echo "lint skipped if golangci-lint not installed"
```

---

### Task 35: Run Unit Tests with Race Detector

- [ ] **Step 1: Run all unit tests**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -race -count=1 -timeout=30m -p 1 ./internal/... -short -v 2>&1 | tail -50
```

Expected: All tests pass, no race conditions.

---

### Task 36: Run New Challenge Scripts

- [ ] **Step 1: Run goroutine lifecycle challenge**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 ./challenges/scripts/goroutine_lifecycle_challenge.sh
```

Expected: All checks pass.

- [ ] **Step 2: Run adapter coverage challenge**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 ./challenges/scripts/adapter_coverage_challenge.sh
```

Expected: All adapter packages meet coverage threshold.

- [ ] **Step 3: Run race condition challenge**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 ./challenges/scripts/race_condition_challenge.sh
```

Expected: Zero race conditions detected.

---

### Task 37: Run Security Re-Scan

- [ ] **Step 1: Re-run gosec to verify fixes**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 gosec -fmt=json -out=reports/security/gosec-rescan-2026-03-16.json ./internal/... 2>&1 || true
```

- [ ] **Step 2: Compare with original scan**

Verify critical/high findings from original scan are resolved.

---

### Task 38: Benchmark Regression Check

- [ ] **Step 1: Run benchmarks and compare with baseline**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -bench=. -benchmem -benchtime=3s -count=1 -timeout=30m ./internal/... 2>&1 | tee reports/benchmarks/post-remediation-2026-03-16.txt
```

- [ ] **Step 2: Compare key metrics**

```bash
diff reports/benchmarks/baseline-2026-03-16.txt reports/benchmarks/post-remediation-2026-03-16.txt | head -50
```

Verify no significant regressions.

---

## Chunk 12: Phase 10 — Commit & Push

### Task 39: Final Commit and Push

- [ ] **Step 1: Check git status**

```bash
git status
```

- [ ] **Step 2: Stage any remaining changes**

```bash
git add -A reports/ docs/
```

- [ ] **Step 3: Create final validation commit**

```bash
git commit -m "chore(validation): final remediation validation reports and benchmark comparison"
```

- [ ] **Step 4: Push to remote via SSH**

```bash
git push githubhelixdevelopment main
```

Expected: Push succeeds via SSH.

- [ ] **Step 5: Verify clean state**

```bash
git status
git log --oneline -10
```

Expected: Clean working tree, all remediation commits visible.

---

## Summary of Deliverables

| Phase | Tasks | Commits | Key Artifacts |
|-------|-------|---------|---------------|
| Phase 0 | 1-4 | 4 | Benchmark baseline, Constitution sync (3 files), mock scan |
| Phase 1 | 5-14 | 9 | Safety fixes in handlers, cache, database, adapters + goroutine leak verification test |
| Phase 1b | 15 | 1 | Dead code removal |
| Phase 2 | 16-18 | 3 | Test files for 9 packages, 3 new challenge scripts |
| Phase 3 | 19-20 | 2 | Security scan reports, vulnerability fixes |
| Phase 4 | 21-22 | 2 | Lazy loading expansion, semaphore controls |
| Phase 4b | 23 | 1 | HTTP/3 verification + negotiation test |
| Phase 5 | 24-26 | 3 | Prometheus, OTel, health check test files |
| Phase 6 | 27-28 | 2 | Extreme stress tests (with P99 baseline), chaos engineering tests |
| Phase 7 | 29-30 | 2 | Updated CLAUDE.md, AGENTS.md, diagrams |
| Phase 8 | 31-34 | 4 | New video course 31, updated courses/manuals/website |
| Phase 9 | 35-39 | 0 | Container rebuild + validation (reports committed in Phase 10) |
| Phase 10 | 40 | 1 | Final validation report, push |
| **Total** | **40** | **~34** | |
