# HelixAgent Comprehensive Project Completion Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring the entire HelixAgent project (main repo + all 21 modules) to 100% test coverage, zero dead code, zero race conditions, clean security scans, complete documentation, updated video courses and website, and passing all challenges — with no broken, disabled, or undocumented component.

**Architecture:** 8 parallel streams gated at 3 synchronization points. Streams 1–5 run immediately; Streams 6–7 begin after Stream 2 (module extraction); Stream 8 begins after Streams 1, 3, 4 complete. All work follows TDD, Conventional Commits, SSH-only Git, and the CLAUDE.md mandatory standards.

**Tech Stack:** Go 1.24+, Gin v1.11, PostgreSQL 15 (pgx/v5), Redis 7, testify v1.11, goleak v1.3, Prometheus/OpenTelemetry, Docker/Podman, SonarQube community, Snyk, quic-go, andybalholm/brotli.

**Design doc:** `docs/plans/2026-02-23-comprehensive-completion-design.md`

**Resource limits (ALL test commands):** `GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1`

---

## Gate Structure

```
GATE 0: Streams 1–5 start immediately (parallel)
GATE 1: After Stream 2 → Streams 6–7 start
GATE 2: After Streams 1, 3, 4 → Stream 8 starts
GATE 3: All streams done → Final validation
```

---

# STREAM 1: Memory Safety & Race Conditions

**Prerequisite:** None — starts immediately.

---

### Task 1.1: Fix BootManager.Results Data Race

**Files:**
- Modify: `internal/services/boot_manager.go`
- Create: `tests/stress/boot_manager_concurrent_stress_test.go`

**Step 1: Write the failing race-detection test**

```go
// tests/stress/boot_manager_concurrent_stress_test.go
package stress_test

import (
    "sync"
    "testing"
    "dev.helix.agent/internal/services"
)

func TestBootManager_ConcurrentAccess_NoDataRace(t *testing.T) {
    bm := services.NewBootManager(nil)
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _ = bm.GetResults()
        }()
    }
    wg.Wait()
}
```

**Step 2: Run with race detector to confirm failure**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 -run TestBootManager_ConcurrentAccess_NoDataRace ./tests/stress/
```
Expected: DATA RACE detected or test fails (confirms the bug exists).

**Step 3: Add resultsMu to BootManager**

Open `internal/services/boot_manager.go`. Add mutex field and `GetResults()` method:

```go
type BootManager struct {
    // ... existing fields ...
    Results   map[string]*BootResult
    resultsMu sync.RWMutex
}

// GetResults returns a copy of results under read lock.
func (bm *BootManager) GetResults() map[string]*BootResult {
    bm.resultsMu.RLock()
    defer bm.resultsMu.RUnlock()
    out := make(map[string]*BootResult, len(bm.Results))
    for k, v := range bm.Results {
        out[k] = v
    }
    return out
}

// setResult writes a single result under write lock.
func (bm *BootManager) setResult(name string, result *BootResult) {
    bm.resultsMu.Lock()
    defer bm.resultsMu.Unlock()
    bm.Results[name] = result
}
```

Replace all direct `bm.Results[name] = ...` writes with `bm.setResult(name, ...)`. Replace all direct map reads outside the struct with `bm.GetResults()`.

**Step 4: Run race-detection test to confirm fix**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 -run TestBootManager_ConcurrentAccess_NoDataRace ./tests/stress/
```
Expected: PASS, no DATA RACE.

**Step 5: Run existing boot manager tests**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 ./internal/services/ -run TestBootManager
```
Expected: All pass.

**Step 6: Commit**

```bash
git add internal/services/boot_manager.go tests/stress/boot_manager_concurrent_stress_test.go
git commit -m "fix(boot_manager): add resultsMu to prevent data race on Results map"
```

---

### Task 1.2: Fix context.Background() in Cache Invalidation

**Files:**
- Modify: `internal/cache/invalidation.go`

**Step 1: Read the file to understand the handleEvent signature**

Read `internal/cache/invalidation.go` around line 340. Look for the `handleEvent` function and how it is called from the goroutine.

**Step 2: Add context parameter to handleEvent**

```go
// Before:
func (i *EventDrivenInvalidation) handleEvent(event Event) {
    ctx := context.Background()
    // ...
}

// After:
func (i *EventDrivenInvalidation) handleEvent(ctx context.Context, event Event) {
    // use ctx directly — no context.Background()
}
```

Update the goroutine caller to pass the stored `i.ctx`:

```go
go func() {
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
```

**Step 3: Build to verify no compilation errors**

```bash
nice -n 19 go build ./internal/cache/...
```
Expected: Exits 0.

**Step 4: Run cache tests**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 ./internal/cache/...
```
Expected: All pass.

**Step 5: Commit**

```bash
git add internal/cache/invalidation.go
git commit -m "fix(cache): propagate caller context in EventDrivenInvalidation.handleEvent"
```

---

### Task 1.3: Add Circuit Breaker Listener Timeout Logging

**Files:**
- Modify: `internal/llm/circuit_breaker.go`

**Step 1: Find the listener notification goroutines** (around line 276)

**Step 2: Add log statement on timeout**

```go
select {
case <-done:
case <-time.After(5 * time.Second):
    log.Printf("[circuit_breaker] WARNING: listener timeout for provider %s (state: %s → %s)",
        cb.providerID, oldState, newState)
}
```

**Step 3: Build and test**

```bash
nice -n 19 go build ./internal/llm/...
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/llm/...
```
Expected: Exits 0.

**Step 4: Commit**

```bash
git add internal/llm/circuit_breaker.go
git commit -m "fix(circuit_breaker): log listener timeout events instead of silently ignoring"
```

---

### Task 1.4: Add goleak to TestMain for Goroutine Leak Detection

**Files:**
- Modify: `tests/testmain_test.go`

**Step 1: Check current TestMain**

Read `tests/testmain_test.go` to see current structure.

**Step 2: Verify goleak is in go.sum**

```bash
grep goleak go.sum
```
Expected: `go.uber.org/goleak v1.3.0` present.

**Step 3: Add goleak to go.mod if not in direct dependencies**

```bash
nice -n 19 go get go.uber.org/goleak@v1.3.0
```

**Step 4: Wire goleak into TestMain**

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    // existing setup...
    goleak.VerifyTestMain(m,
        goleak.IgnoreTopFunction("testing.(*M).Run"),
        goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
    )
}
```

**Step 5: Run tests with goleak active**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./tests/...
```
Expected: Pass (or reveals goroutine leaks to fix next).

**Step 6: Fix any goroutine leaks detected**

For each leaked goroutine reported by goleak, find the source (goroutine name in stack trace), ensure it is properly shut down via context cancellation or channel close.

**Step 7: Commit**

```bash
git add tests/testmain_test.go go.mod go.sum
git commit -m "test: add goleak goroutine leak detection to TestMain"
```

---

### Task 1.5: Full Race Detection Run and Fix

**Step 1: Run race detector across all project packages**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 -short ./internal/... ./cmd/... 2>&1 | tee /tmp/race-results.txt
```

**Step 2: Triage results**

```bash
grep -A5 "DATA RACE" /tmp/race-results.txt
```

**Step 3: For each race found**

- Identify the shared variable
- Add appropriate mutex or switch to `sync/atomic`
- Re-run race detector to confirm fix
- Commit each fix separately with `fix(race): ...`

**Step 4: Run full race scan to confirm clean**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 -short ./internal/... ./cmd/... 2>&1 | grep -c "DATA RACE"
```
Expected: `0`

**Step 5: Commit clean state**

```bash
git commit -m "fix(race): resolve all data races detected by go test -race"
```

---

### Task 1.6: Write memory_race_challenge.sh

**Files:**
- Create: `challenges/scripts/memory_race_challenge.sh`

**Step 1: Create challenge script**

```bash
#!/usr/bin/env bash
# memory_race_challenge.sh - Validates race-free concurrent execution
# Tests: 15

set -euo pipefail
PASS=0; FAIL=0
pass() { echo "✓ $1"; ((PASS++)); }
fail() { echo "✗ $1"; ((FAIL++)); }

# Test 1: BootManager GetResults is thread-safe
if GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 -run TestBootManager_ConcurrentAccess_NoDataRace ./tests/stress/ 2>&1 | grep -q "PASS"; then
    pass "BootManager concurrent access is race-free"
else
    fail "BootManager concurrent access has DATA RACE"
fi

# ... (15+ tests total: race detector, goroutine leak detection, mutex coverage)

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
```

**Step 2: Make executable and test**

```bash
chmod +x challenges/scripts/memory_race_challenge.sh
bash challenges/scripts/memory_race_challenge.sh
```
Expected: All tests pass.

**Step 3: Commit**

```bash
git add challenges/scripts/memory_race_challenge.sh
git commit -m "test(challenge): add memory_race_challenge.sh with 15+ race-condition tests"
```

---

# STREAM 2: Dead Code → 5 New Modules

**Prerequisite:** None — starts immediately in parallel with Stream 1.
**Triggers Gate 1** when complete.

Each sub-stream (2a–2e) extracts one package. They can be done in parallel.

---

### Task 2.1: Extract internal/agentic → Agentic Module

**Files:**
- Create: `Agentic/` directory tree
- Modify: `go.mod` (add replace directive)
- Modify: `.gitmodules`
- Create: `internal/adapters/agentic/adapter.go`
- Modify: `cmd/helixagent/main.go` (wire in)

**Step 1: Create module directory structure**

```bash
mkdir -p Agentic/agentic
mkdir -p Agentic/docs
mkdir -p Agentic/challenges/scripts
```

**Step 2: Initialize go.mod**

```bash
cat > Agentic/go.mod << 'EOF'
module digital.vasic.agentic

go 1.24

require (
    github.com/google/uuid v1.6.0
)
EOF
```

**Step 3: Copy source files**

```bash
cp internal/agentic/*.go Agentic/agentic/
```

Update package declaration in all copied files from `package agentic` (stays the same — verify it matches).

**Step 4: Write failing test for adapter**

```go
// internal/adapters/agentic/adapter_test.go
package agentic_test

import (
    "testing"
    "context"
    agenticadapter "dev.helix.agent/internal/adapters/agentic"
)

func TestAgenticAdapter_NewWorkflow_NotNil(t *testing.T) {
    adapter := agenticadapter.New()
    if adapter == nil {
        t.Fatal("expected non-nil adapter")
    }
}

func TestAgenticAdapter_ExecuteWorkflow_ReturnsResult(t *testing.T) {
    adapter := agenticadapter.New()
    ctx := context.Background()
    result, err := adapter.ExecuteWorkflow(ctx, "test-workflow", map[string]any{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result == nil {
        t.Fatal("expected non-nil result")
    }
}
```

**Step 5: Run test to verify it fails**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/adapters/agentic/...
```
Expected: FAIL — package not found yet.

**Step 6: Write adapter**

```go
// internal/adapters/agentic/adapter.go
package agentic

import (
    "context"
    "digital.vasic.agentic/agentic"
)

// AgenticAdapter bridges HelixAgent internals to the Agentic module.
type AgenticAdapter struct {
    engine *agentic.WorkflowEngine
}

func New() *AgenticAdapter {
    return &AgenticAdapter{
        engine: agentic.NewWorkflowEngine(),
    }
}

func (a *AgenticAdapter) ExecuteWorkflow(ctx context.Context, name string, params map[string]any) (map[string]any, error) {
    return a.engine.Execute(ctx, name, params)
}
```

**Step 7: Add replace directive to root go.mod**

```
replace digital.vasic.agentic => ./Agentic
```

```bash
nice -n 19 go mod tidy
```

**Step 8: Run test to verify it passes**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/adapters/agentic/...
```
Expected: PASS.

**Step 9: Wire into main — add HTTP endpoint**

In `cmd/helixagent/main.go` or appropriate handler file, add route:

```go
// Register agentic workflow endpoint
agenticAdapter := agentic.New()
router.POST("/v1/agentic/workflow", handlers.NewAgenticHandler(agenticAdapter).Execute)
```

**Step 10: Create Agentic module required files**

```bash
# README.md, CLAUDE.md, AGENTS.md — follow existing module templates exactly
# docs/ directory with full API reference
# challenges/scripts/agentic_workflow_challenge.sh (placeholder, filled in Stream 8)
```

**Step 11: Build to verify no breakage**

```bash
nice -n 19 go build ./cmd/helixagent/...
```
Expected: Exits 0.

**Step 12: Commit**

```bash
git add Agentic/ internal/adapters/agentic/ go.mod go.sum
git commit -m "feat(agentic): extract internal/agentic to Agentic module (digital.vasic.agentic)"
```

---

### Task 2.2: Extract internal/llmops → LLMOps Module

**Files:**
- Create: `LLMOps/` directory tree
- Create: `internal/adapters/llmops/adapter.go`

**Step 1: Create module structure**

```bash
mkdir -p LLMOps/llmops LLMOps/docs LLMOps/challenges/scripts
```

**Step 2: Initialize go.mod**

```
module digital.vasic.llmops
go 1.24
```

**Step 3: Copy and update source**

```bash
cp internal/llmops/*.go LLMOps/llmops/
```

**Step 4: Write failing adapter test**

```go
// internal/adapters/llmops/adapter_test.go
func TestLLMOpsAdapter_TrackExperiment_NoError(t *testing.T) {
    adapter := llmopsadapter.New()
    err := adapter.TrackExperiment(context.Background(), "test-exp", map[string]any{"model": "test"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

**Step 5: Write adapter**

```go
// internal/adapters/llmops/adapter.go
package llmops

import (
    "context"
    "digital.vasic.llmops/llmops"
)

type LLMOpsAdapter struct {
    tracker *llmops.ExperimentTracker
}

func New() *LLMOpsAdapter {
    return &LLMOpsAdapter{tracker: llmops.NewExperimentTracker()}
}

func (a *LLMOpsAdapter) TrackExperiment(ctx context.Context, name string, params map[string]any) error {
    return a.tracker.Track(ctx, name, params)
}
```

**Step 6: Add replace directive, tidy, test, build**

```bash
# Add to go.mod: replace digital.vasic.llmops => ./LLMOps
nice -n 19 go mod tidy
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/adapters/llmops/...
nice -n 19 go build ./cmd/helixagent/...
```

**Step 7: Wire into provider registry**

In `internal/services/provider_registry.go`, after each LLM call, call `llmopsAdapter.TrackExperiment()` with request/response metadata for experiment tracking.

**Step 8: Commit**

```bash
git add LLMOps/ internal/adapters/llmops/ go.mod go.sum
git commit -m "feat(llmops): extract internal/llmops to LLMOps module (digital.vasic.llmops)"
```

---

### Task 2.3: Extract internal/selfimprove → SelfImprove Module

**Files:**
- Create: `SelfImprove/` directory tree
- Create: `internal/adapters/selfimprove/adapter.go`

Follow the same pattern as Tasks 2.1 and 2.2:

1. `mkdir -p SelfImprove/selfimprove SelfImprove/docs SelfImprove/challenges/scripts`
2. `go.mod` with `module digital.vasic.selfimprove`
3. Copy `internal/selfimprove/*.go` to `SelfImprove/selfimprove/`
4. Write failing adapter test for `FeedbackCollector`
5. Write `internal/adapters/selfimprove/adapter.go`
6. Wire into debate service: after debate outcome, call `selfimproveAdapter.CollectFeedback(ctx, outcome)`
7. Add replace directive, tidy, test, build
8. Create docs files (README, CLAUDE, AGENTS, docs/)

**Step 9: Commit**

```bash
git add SelfImprove/ internal/adapters/selfimprove/ go.mod go.sum
git commit -m "feat(selfimprove): extract internal/selfimprove to SelfImprove module (digital.vasic.selfimprove)"
```

---

### Task 2.4: Extract internal/planning → Planning Module

**Files:**
- Create: `Planning/` directory tree
- Create: `internal/adapters/planning/adapter.go`

Follow same pattern:

1. `mkdir -p Planning/planning Planning/docs Planning/challenges/scripts`
2. `go.mod` with `module digital.vasic.planning`
3. Copy `internal/planning/*.go` to `Planning/planning/`
4. Write failing test for MCTS planner adapter
5. Write adapter exposing `Plan(ctx, goal, constraints)` method
6. Add `/v1/planning/` HTTP endpoint
7. Add replace directive, tidy, test, build

**Step 8: Commit**

```bash
git add Planning/ internal/adapters/planning/ go.mod go.sum
git commit -m "feat(planning): extract internal/planning to Planning module (digital.vasic.planning)"
```

---

### Task 2.5: Extract internal/benchmark → Benchmark Module

**Files:**
- Create: `Benchmark/` directory tree
- Create: `internal/adapters/benchmark/adapter.go`

Follow same pattern:

1. `mkdir -p Benchmark/benchmark Benchmark/docs Benchmark/challenges/scripts`
2. `go.mod` with `module digital.vasic.benchmark`
3. Copy `internal/benchmark/*.go` to `Benchmark/benchmark/`
4. Write failing test for BenchmarkRunner adapter
5. Write adapter exposing `RunBenchmark(ctx, suite, config)` method
6. Add `/v1/benchmark/` HTTP endpoint
7. Wire into monitoring: publish benchmark results to Prometheus metrics
8. Add replace directive, tidy, test, build

**Step 9: Commit**

```bash
git add Benchmark/ internal/adapters/benchmark/ go.mod go.sum
git commit -m "feat(benchmark): extract internal/benchmark to Benchmark module (digital.vasic.benchmark)"
```

---

### Task 2.6: Update CLAUDE.md, AGENTS.md, go.mod for 5 New Modules

**Step 1: Update docs/MODULES.md**

Add 5 new entries under "Integration (Phase 5 — New)":
```
- **Agentic** (`Agentic/`, `digital.vasic.agentic`) — Graph-based workflow orchestration. N packages.
- **LLMOps** (`LLMOps/`, `digital.vasic.llmops`) — Experiment tracking, evaluation. N packages.
- **SelfImprove** (`SelfImprove/`, `digital.vasic.selfimprove`) — Feedback and reward systems. N packages.
- **Planning** (`Planning/`, `digital.vasic.planning`) — MCTS, HiPlan, Tree-of-Thoughts. N packages.
- **Benchmark** (`Benchmark/`, `digital.vasic.benchmark`) — Performance measurement. N packages.
```

**Step 2: Update CLAUDE.md**

Add 5 modules to "Extracted Modules" section. Update "Architecture" section with new endpoints.

**Step 3: Update AGENTS.md**

Add 5 modules to agent capabilities section.

**Step 4: Verify go.mod has all 5 replace directives**

```bash
grep "digital.vasic" go.mod | grep "replace"
```
Expected: 26 entries (21 existing + 5 new).

**Step 5: Run full build**

```bash
nice -n 19 go build ./...
```
Expected: Exits 0.

**Step 6: Commit**

```bash
git add CLAUDE.md AGENTS.md docs/MODULES.md go.mod
git commit -m "docs: update CLAUDE.md, AGENTS.md, MODULES.md with 5 new extracted modules"
```

**→ GATE 1 REACHED: Streams 6 and 7 may now begin.**

---

# STREAM 3: Test Coverage → 100%

**Prerequisite:** None — starts immediately.

---

### Task 3.1: Fix mcp_container_test.go go vet Issues

**Files:**
- Modify: `tests/integration/mcp_container_test.go`

**Step 1: Read the file around lines 106–109, 170–171 (unkeyed fields)**

**Step 2: Add field names to MCPContainerPort struct literals**

```go
// Before (line 106):
config.MCPContainerPort{8080, "http", "tcp"}

// After:
config.MCPContainerPort{Port: 8080, Protocol: "http", Transport: "tcp"}
```
Apply to all 6 occurrences.

**Step 3: Fix IPv6-incompatible format (lines 32, 361)**

```go
// Before:
address := fmt.Sprintf("%s:%d", host, port)
conn, err := net.Dial("tcp", address)

// After:
address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
conn, err := net.Dial("tcp", address)
```

**Step 4: Run go vet to confirm clean**

```bash
nice -n 19 go vet ./tests/integration/...
```
Expected: Exits 0.

**Step 5: Commit**

```bash
git add tests/integration/mcp_container_test.go
git commit -m "fix(test): fix unkeyed struct fields and IPv6 format in mcp_container_test.go"
```

---

### Task 3.2: Add Tests for Internal Adapter Files (0% → 100%)

For each of the 11 untested adapter files, follow this pattern:

**Pattern for each adapter:**

1. Read the adapter file to understand exported functions
2. Write test file: `internal/adapters/<pkg>/<file>_test.go`
3. Test every exported function/method with at least 3 test cases (happy path, error path, nil input)
4. Run `go test -count=1 -p 1 ./internal/adapters/<pkg>/...`
5. Commit with `test(adapters): add 100% coverage for <pkg> adapter`

**Adapters to cover (in order of priority):**

1. `internal/adapters/eventbus.go` → `internal/adapters/eventbus_test.go`
2. `internal/adapters/cache/adapter.go` → `cache/adapter_test.go`
3. `internal/adapters/database/` (3 files) → 3 test files
4. `internal/adapters/memory/adapter.go` → `memory/adapter_test.go`
5. `internal/adapters/optimization/adapter.go` → `optimization/adapter_test.go`
6. `internal/adapters/plugins/adapter.go` → `plugins/adapter_test.go`
7. `internal/adapters/rag/adapter.go` → `rag/adapter_test.go`
8. `internal/adapters/security/security.go` → `security/security_test.go`
9. `internal/adapters/mcp/mcp.go` → `mcp/mcp_test.go`
10. `internal/adapters/cloud/adapter.go` → `cloud/adapter_test.go`

**Example for eventbus adapter:**

```go
// internal/adapters/eventbus_test.go
package adapters_test

import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "dev.helix.agent/internal/adapters"
)

func TestEventBusAdapter_New_NotNil(t *testing.T) {
    a := adapters.NewEventBusAdapter()
    require.NotNil(t, a)
}

func TestEventBusAdapter_Publish_Subscribe_ReceivesEvent(t *testing.T) {
    a := adapters.NewEventBusAdapter()
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    received := make(chan string, 1)
    err := a.Subscribe(ctx, "test.topic", func(data string) {
        received <- data
    })
    require.NoError(t, err)

    err = a.Publish(ctx, "test.topic", "hello")
    require.NoError(t, err)

    select {
    case msg := <-received:
        assert.Equal(t, "hello", msg)
    case <-ctx.Done():
        t.Fatal("timeout waiting for event")
    }
}

func TestEventBusAdapter_Publish_NilContext_ReturnsError(t *testing.T) {
    a := adapters.NewEventBusAdapter()
    //nolint:staticcheck
    err := a.Publish(nil, "test.topic", "data") //nolint:golangci-lint
    assert.Error(t, err)
}
```

**Step (for each adapter): Run test and commit**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 ./internal/adapters/...
git add internal/adapters/
git commit -m "test(adapters): add 100% test coverage for all internal adapter packages"
```

---

### Task 3.3: Add Tests for internal/utils (6 Untested Files)

**Step 1: List untested utility files**

```bash
ls internal/utils/*.go | grep -v _test.go
```

**Step 2: For each untested file, write companion test**

Template:
```go
// internal/utils/<file>_test.go
package utils_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "dev.helix.agent/internal/utils"
)

// Test every exported function with table-driven tests:
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    interface{}
        expected interface{}
        wantErr  bool
    }{
        {name: "happy path", input: validInput, expected: validOutput},
        {name: "error case", input: invalidInput, wantErr: true},
        {name: "edge case", input: edgeInput, expected: edgeOutput},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := utils.FunctionName(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, got)
        })
    }
}
```

**Step 3: Run and verify**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/utils/...
```
Expected: PASS.

**Step 4: Commit**

```bash
git add internal/utils/
git commit -m "test(utils): add 100% test coverage for internal/utils package"
```

---

### Task 3.4: Improve Optimization Module Coverage (38% → 100%)

**Step 1: List untested files**

```bash
ls Optimization/*/*.go | grep -v _test.go
```

**Step 2: For each of the 10 untested files, write test**

Follow table-driven test pattern. Ensure:
- Every exported type is constructed
- Every exported method has at least happy path + error path
- Context cancellation is tested where applicable

**Step 3: Run**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./Optimization/...
```

**Step 4: Commit**

```bash
git add Optimization/
git commit -m "test(optimization): improve coverage from 38% to 100% across all packages"
```

---

### Task 3.5: Improve LLMsVerifier Coverage (80% → 100%)

**Step 1: Identify the 52 untested source files**

```bash
find LLMsVerifier -name "*.go" | grep -v _test.go | while read f; do
    test_file="${f%.go}_test.go"
    if [ ! -f "$test_file" ]; then
        echo "MISSING: $test_file"
    fi
done
```

**Step 2: Prioritize by package import frequency**

Focus on packages imported most often: `pkg/cliagents/`, `pkg/providers/`, `pkg/scoring/`.

**Step 3: For each missing test file, write comprehensive tests**

Use table-driven tests. Mock external HTTP calls with `httptest.NewServer`. Use real struct instances, not mocks.

**Step 4: Run LLMsVerifier tests**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./LLMsVerifier/...
```

**Step 5: Commit in batches by package**

```bash
git add LLMsVerifier/
git commit -m "test(llmsverifier): improve coverage from 80% to 100% across all packages"
```

---

### Task 3.6: Improve Containers Module Coverage (78% → 100%)

**Step 1: List untested files**

```bash
find Containers -name "*.go" | grep -v _test.go | while read f; do
    [ ! -f "${f%.go}_test.go" ] && echo "MISSING: $f"
done
```

**Step 2: Write tests for the 21 uncovered files**

Focus on runtime detection, connection management, and health check packages.

**Step 3: Run**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./containers/...
```

**Step 4: Commit**

```bash
git add containers/
git commit -m "test(containers): improve coverage from 78% to 100% across all packages"
```

---

### Task 3.7: Expand tests/chaos and tests/compliance

**Step 1: Expand chaos tests** — `tests/chaos/`

Add 9 more test files covering:
- Provider failure cascade (kill one, verify others still work)
- Redis connection drop (verify graceful degradation)
- Database unavailable (verify error responses)
- Memory pressure (verify OOM doesn't crash)
- Slow provider response (verify timeout/circuit breaker)
- Concurrent request storm (verify queue doesn't overflow)
- Invalid JSON response from LLM (verify parsing error handling)
- Partial stream disconnect (verify stream recovery)
- Configuration hot-reload during request (verify thread safety)

**Step 2: Expand compliance tests** — `tests/compliance/`

Add 9 more test files covering:
- OpenAI API compatibility (`/v1/chat/completions` format)
- HTTP/3 QUIC compliance (connection establishment)
- Brotli compression compliance (Content-Encoding header)
- JWT token format compliance (RFC 7519)
- Rate limit header compliance (RFC 6585)
- JSON-RPC 2.0 compliance (MCP protocol)
- OpenTelemetry span format compliance
- Prometheus metrics format compliance
- gRPC status code compliance

**Step 3: Run**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./tests/chaos/ ./tests/compliance/
```

**Step 4: Commit**

```bash
git add tests/chaos/ tests/compliance/
git commit -m "test(chaos,compliance): expand from 1 file each to comprehensive test suites"
```

---

### Task 3.8: Cover Remaining Internal Package Gaps

For each remaining gap, write test files following the same table-driven pattern:

1. `internal/llmops/` — 3 untested files → write tests
2. `internal/selfimprove/` — 3 untested files → write tests
3. `internal/streaming/` — 3 untested files → write tests
4. `internal/plugins/` — 3 untested files → write tests

```bash
# After each package:
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 ./internal/<pkg>/...
git add internal/<pkg>/
git commit -m "test(<pkg>): achieve 100% coverage in internal/<pkg> package"
```

---

# STREAM 4: Security Scanning + Resolution

**Prerequisite:** None — starts immediately.

---

### Task 4.1: Start Security Scanning Infrastructure

**Step 1: Verify compose file exists**

```bash
ls docker-compose.security.yml
```
Expected: File exists.

**Step 2: Start services (non-interactive)**

```bash
podman-compose -f docker-compose.security.yml up -d sonarqube sonarqube-db 2>&1
# OR if using Docker:
docker compose -f docker-compose.security.yml up -d sonarqube sonarqube-db 2>&1
```
Expected: Containers start.

**Step 3: Wait for SonarQube to be ready**

```bash
for i in $(seq 1 30); do
    if curl -sf http://localhost:9000/api/system/status | grep -q '"status":"UP"'; then
        echo "SonarQube ready"
        break
    fi
    echo "Waiting for SonarQube... ($i/30)"
    sleep 10
done
```
Expected: "SonarQube ready" within 5 minutes.

**Step 4: Do NOT start Snyk container (one-shot, runs separately)**

---

### Task 4.2: Generate Coverage Report for SonarQube

**Step 1: Generate test coverage**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 -coverprofile=coverage.out ./internal/... ./cmd/... 2>&1
```

**Step 2: Verify coverage file exists**

```bash
ls -la coverage.out
```
Expected: Non-empty file.

---

### Task 4.3: Run SonarQube Analysis

**Step 1: Check sonar-scanner is available**

```bash
which sonar-scanner || echo "Need to install sonar-scanner"
```

If not installed:
```bash
# Install sonar-scanner CLI (non-interactive)
wget -q https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/sonar-scanner-cli-6.2.1.4610-linux-x64.zip
unzip -q sonar-scanner-cli-6.2.1.4610-linux-x64.zip
export PATH=$PATH:$(pwd)/sonar-scanner-6.2.1.4610-linux-x64/bin
```

**Step 2: Run scanner**

```bash
sonar-scanner \
  -Dsonar.host.url=http://localhost:9000 \
  -Dsonar.login=admin \
  -Dsonar.password=admin \
  -Dsonar.projectKey=helixagent \
  2>&1 | tee /tmp/sonar-results.txt
```

**Step 3: Retrieve findings via API**

```bash
curl -s "http://localhost:9000/api/issues/search?projectKeys=helixagent&severities=CRITICAL,BLOCKER&resolved=false" \
  | python3 -m json.tool > /tmp/sonar-critical.json
echo "Critical/Blocker issues:"
cat /tmp/sonar-critical.json | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Total: {d[\"total\"]}')"
```

---

### Task 4.4: Analyze and Fix SonarQube Findings

**Step 1: For each Critical/Blocker finding**

Group by type:
- **Bugs** → fix the underlying code issue
- **Vulnerabilities** → patch or replace the vulnerable pattern
- **Security Hotspots** → review, add justification comment, or fix

**Step 2: Fix findings in batches by file**

For each file with findings:
1. Read the file and the specific finding
2. Implement the fix
3. Run `go vet` and `go build` on the file's package
4. Commit: `fix(security): resolve SonarQube [type] in [package]`

**Step 3: Re-run scanner to verify fix**

```bash
sonar-scanner -Dsonar.host.url=http://localhost:9000 -Dsonar.login=admin -Dsonar.password=admin -Dsonar.projectKey=helixagent
```

Repeat until zero Critical/Blocker issues remain.

**Step 4: Document findings and resolutions**

```bash
mkdir -p docs/security
cat > docs/security/SCAN_RESULTS_2026-02-23.md << 'EOF'
# Security Scan Results — 2026-02-23
## SonarQube Findings
[populated from scan output]
## Snyk Findings
[populated from snyk output]
## Resolutions
[per-finding resolution notes]
EOF
```

---

### Task 4.5: Run Snyk Scan

**Step 1: Run Snyk via container (non-interactive, no token required for OSS)**

```bash
podman run --rm \
  -v $(pwd):/app:ro \
  snyk/snyk:golang \
  test --all-projects --json 2>/dev/null > /tmp/snyk-results.json || true
```

**Step 2: Parse critical vulnerabilities**

```bash
cat /tmp/snyk-results.json | python3 -c "
import sys, json
data = json.load(sys.stdin)
vulns = data.get('vulnerabilities', [])
critical = [v for v in vulns if v.get('severity') in ['critical', 'high']]
for v in critical:
    print(f'{v[\"severity\"].upper()}: {v[\"packageName\"]}@{v[\"version\"]} — {v[\"title\"]}')
print(f'Total critical/high: {len(critical)}')
"
```

**Step 3: Fix dependency vulnerabilities**

For each vulnerable dependency:
```bash
# Update to patched version
nice -n 19 go get <package>@<patched-version>
nice -n 19 go mod tidy
nice -n 19 go mod vendor
```

**Step 4: Re-run Snyk to confirm clean**

```bash
podman run --rm -v $(pwd):/app:ro snyk/snyk:golang test --all-projects --json 2>/dev/null > /tmp/snyk-results-2.json || true
```

**Step 5: Commit all dependency updates**

```bash
git add go.mod go.sum vendor/
git commit -m "fix(security): update dependencies to resolve Snyk critical/high vulnerabilities"
```

---

### Task 4.6: Enhance Security Challenge Scripts

**Step 1: Enhance security_scanning_challenge.sh (10 → 30+ tests)**

Add tests for:
- SonarQube container running
- SonarQube API accessible
- Last scan results clean (0 critical/blocker issues)
- Snyk results file exists and shows 0 critical/high CVEs
- `.snyk` policy file is present and valid
- `sonar-project.properties` present and correctly configured
- Coverage report generated and fed to SonarQube
- Security scan runs non-interactively

**Step 2: Create sonarqube_challenge.sh (20+ tests)**

```bash
chmod +x challenges/scripts/sonarqube_challenge.sh
```

**Step 3: Create snyk_dependency_challenge.sh (15+ tests)**

**Step 4: Run all security challenges**

```bash
bash challenges/scripts/security_scanning_challenge.sh
bash challenges/scripts/sonarqube_challenge.sh
bash challenges/scripts/snyk_dependency_challenge.sh
```
Expected: All pass.

**Step 5: Commit**

```bash
git add challenges/scripts/
git commit -m "test(challenge): enhance security scanning challenges (30+ tests each)"
```

---

# STREAM 5: Stress, Performance & Monitoring Tests

**Prerequisite:** None — starts immediately.

---

### Task 5.1: Monitoring Metrics Collection Test

**Files:**
- Create: `tests/performance/monitoring_metrics_test.go`

**Step 1: Write test that generates load and collects metrics**

```go
// tests/performance/monitoring_metrics_test.go
package performance_test

import (
    "net/http"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestMonitoringMetrics_UnderLoad verifies Prometheus metrics are
// populated correctly when the system processes requests.
func TestMonitoringMetrics_UnderLoad(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping performance test in short mode")
    }

    baseURL := "http://localhost:7061"

    // Generate 50 requests
    for i := 0; i < 50; i++ {
        resp, err := http.Post(baseURL+"/v1/chat/completions",
            "application/json",
            strings.NewReader(`{"model":"helixagent","messages":[{"role":"user","content":"ping"}]}`))
        if err != nil {
            continue
        }
        resp.Body.Close()
    }

    // Allow metrics to aggregate
    time.Sleep(2 * time.Second)

    // Scrape Prometheus metrics
    resp, err := http.Get(baseURL + "/metrics")
    require.NoError(t, err)
    defer resp.Body.Close()
    assert.Equal(t, 200, resp.StatusCode)

    body, err := io.ReadAll(resp.Body)
    require.NoError(t, err)

    // Validate key metrics are present and non-zero
    metrics := string(body)
    assert.Contains(t, metrics, "helixagent_requests_total")
    assert.Contains(t, metrics, "helixagent_request_duration_seconds")
    assert.Contains(t, metrics, "go_goroutines")
    assert.Contains(t, metrics, "go_memstats_alloc_bytes")
}
```

**Step 2: Run (requires server running — skip in CI without infra)**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 -run TestMonitoringMetrics_UnderLoad ./tests/performance/
```

**Step 3: Commit**

```bash
git add tests/performance/monitoring_metrics_test.go
git commit -m "test(performance): add monitoring metrics collection test under load"
```

---

### Task 5.2: Provider Registry Stress Test

**Files:**
- Create: `tests/stress/provider_registry_stress_test.go`

**Step 1: Write stress test**

```go
func TestProviderRegistry_ConcurrentRequests_NoDeadlock(t *testing.T) {
    if testing.Short() {
        t.Skip()
    }
    registry := setupTestRegistry(t) // use test helper
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    var wg sync.WaitGroup
    errors := make(chan error, 1000)

    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            _, err := registry.GetProvider(ctx, "test-provider")
            if err != nil {
                errors <- err
            }
        }(i)
    }
    wg.Wait()
    close(errors)

    var errCount int
    for range errors {
        errCount++
    }
    assert.Equal(t, 0, errCount, "expected zero errors from concurrent provider access")
}
```

**Step 2: Run with race detector**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 -run TestProviderRegistry_ConcurrentRequests_NoDeadlock ./tests/stress/
```
Expected: PASS, no DATA RACE.

**Step 3: Commit**

```bash
git add tests/stress/provider_registry_stress_test.go
git commit -m "test(stress): add concurrent provider registry stress test (1000 goroutines)"
```

---

### Task 5.3: HTTP/3 Connection Pool Stress Test

**Files:**
- Create: `tests/stress/http3_quic_stress_test.go`

**Step 1: Write stress test for HTTP/3 connection handling**

```go
func TestHTTP3_ConnectionPool_UnderSaturation(t *testing.T) {
    if testing.Short() {
        t.Skip()
    }
    // Start test QUIC server
    server := startTestQUICServer(t)
    defer server.Stop()

    var wg sync.WaitGroup
    successCount := atomic.Int64{}

    for i := 0; i < 200; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            client := newHTTP3Client(t)
            resp, err := client.Get(server.URL + "/health")
            if err == nil && resp.StatusCode == 200 {
                successCount.Add(1)
            }
        }()
    }
    wg.Wait()

    // At least 95% should succeed under saturation
    assert.GreaterOrEqual(t, successCount.Load(), int64(190))
}
```

**Step 2: Run**

```bash
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 -run TestHTTP3_ConnectionPool_UnderSaturation ./tests/stress/
```

**Step 3: Commit**

```bash
git add tests/stress/http3_quic_stress_test.go
git commit -m "test(stress): add HTTP/3 QUIC connection pool saturation stress test"
```

---

### Task 5.4: Lazy Initialization Audit and Improvements

**Step 1: Find all init() calls in production code**

```bash
grep -rn "^func init()" --include="*.go" internal/ cmd/ | grep -v "_test.go"
```

**Step 2: For each init() that initializes a non-critical resource**

Convert to `sync.Once` lazy initialization pattern:

```go
// Before:
var globalClient *SomeClient

func init() {
    globalClient = newSomeClient()
}

// After:
var (
    globalClientOnce sync.Once
    globalClient     *SomeClient
)

func getGlobalClient() *SomeClient {
    globalClientOnce.Do(func() {
        globalClient = newSomeClient()
    })
    return globalClient
}
```

**Step 3: Find large structs initialized at startup**

```bash
grep -rn "NewXxx()\|newXxx()" cmd/helixagent/main.go | grep -v "//.*skip"
```

For any expensive initialization that isn't needed at startup → wrap in `sync.Once`.

**Step 4: Build and test after each change**

```bash
nice -n 19 go build ./cmd/helixagent/...
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 -short ./internal/...
```

**Step 5: Commit**

```bash
git commit -m "perf: convert eager initialization to lazy sync.Once pattern in production code"
```

---

### Task 5.5: Semaphore Audit for High-Contention Paths

**Step 1: Find external API call paths without semaphore limits**

```bash
grep -rn "http.Do\|http.Get\|http.Post" internal/llm/providers/ --include="*.go" | grep -v "_test.go" | grep -v "semaphore"
```

**Step 2: For each provider without semaphore**

Verify the provider registry applies `concurrencySemaphores` — if direct HTTP calls bypass it, add semaphore:

```go
type SomeProvider struct {
    client *http.Client
    sem    *semaphore.Weighted // limit concurrent requests
}

func (p *SomeProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
    if err := p.sem.Acquire(ctx, 1); err != nil {
        return nil, fmt.Errorf("semaphore acquire: %w", err)
    }
    defer p.sem.Release(1)
    // ... http call
}
```

**Step 3: Add non-blocking fallback for best-effort operations**

```go
// For metrics/logging (best-effort, never block):
select {
case metricsChan <- event:
default: // drop if full — never block critical path
}
```

**Step 4: Build and test**

```bash
nice -n 19 go build ./internal/...
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 -short ./internal/...
```

**Step 5: Commit**

```bash
git commit -m "perf: add semaphore limits and non-blocking fallbacks to high-contention paths"
```

---

# STREAM 6: Documentation + User Manuals

**Prerequisite:** GATE 1 (Stream 2 complete).

---

### Task 6.1: Update CLAUDE.md with All New Modules and Fixes

**Step 1: Read current CLAUDE.md sections for Extracted Modules**

**Step 2: Add the 5 new modules under a new subsection "Integration (Phase 5 — New)"**

For each module, add the same format as existing entries:
```markdown
- **Agentic** (`Agentic/`, `digital.vasic.agentic`) — Graph-based workflow orchestration: graph engine, node registry, execution, state management. N packages.
```

**Step 3: Update Provider URLs section**

```markdown
- ZAI (Zhipu GLM): `api.z.ai/api/paas/v4` (international endpoint)
```

**Step 4: Update Memory section**

Replace all Cognee references with Mem0.

**Step 5: Verify CLAUDE.md stays under the 200-line truncation point for critical info**

**Step 6: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with 5 new modules, ZAI URL fix, Mem0 migration notes"
```

---

### Task 6.2: Create User Manuals for 5 New Modules

For each new module, create a step-by-step user manual. Template:

**`docs/guides/<module>-guide.md`**:

```markdown
# <Module> Guide

## Overview
What this module does, when to use it.

## Prerequisites
- HelixAgent running on port 7061
- Module wired in (default since v1.x)

## Quick Start
Step-by-step: 3 steps to first successful call

## API Reference
Every endpoint with request/response examples

## Configuration
All env vars with defaults and descriptions

## Integration Examples
5 real-world use cases with code

## Troubleshooting
Common errors and their solutions

## Performance Characteristics
Latency expectations, concurrency limits

## Architecture Diagram
ASCII or referenced Mermaid diagram
```

**Create for each:**
- `docs/guides/agentic-guide.md`
- `docs/guides/llmops-guide.md`
- `docs/guides/selfimprove-guide.md`
- `docs/guides/planning-guide.md`
- `docs/guides/benchmark-guide.md`

**Step: Commit**

```bash
git add docs/guides/
git commit -m "docs: add step-by-step user manuals for 5 new modules (Agentic, LLMOps, SelfImprove, Planning, Benchmark)"
```

---

### Task 6.3: Update docs/MODULES.md (Full Catalog)

**Step 1: Read current MODULES.md**

**Step 2: Add 5 new module entries in the correct phase section**

**Step 3: Update counts at top of file**

**Step 4: Verify all existing entries still accurate**

**Step 5: Commit**

```bash
git add docs/MODULES.md
git commit -m "docs: update MODULES.md with 5 new extracted modules (Phase 5)"
```

---

### Task 6.4: Update Architecture Diagrams

**Step 1: Find all architecture diagrams**

```bash
find docs/diagrams docs/website -name "*.md" | xargs grep -l "diagram\|architecture" -i
```

**Step 2: Add 5 new modules to each relevant diagram**

For ASCII diagrams, add boxes/connections for new modules.
For Mermaid diagrams, add nodes and edges.

**Step 3: Update docs/ARCHITECTURE.md**

Add section for each new module's place in the overall architecture.

**Step 4: Commit**

```bash
git add docs/diagrams/ docs/ARCHITECTURE.md
git commit -m "docs: update architecture diagrams with 5 new modules"
```

---

### Task 6.5: Update AGENTS.md

**Step 1: Read current AGENTS.md**

**Step 2: Add sections for 5 new modules** — capabilities, API endpoints, configuration

**Step 3: Verify Documentation Synchronization rule: anything in CLAUDE.md must be in AGENTS.md and CONSTITUTION.json**

**Step 4: Update CONSTITUTION.json if new architectural rules are introduced**

**Step 5: Commit**

```bash
git add AGENTS.md CONSTITUTION.json
git commit -m "docs: update AGENTS.md and CONSTITUTION.json with 5 new modules and Phase 5 pattern"
```

---

### Task 6.6: Full Documentation Accuracy Pass

**Step 1: Replace all remaining Cognee references with Mem0**

```bash
grep -rn "cognee\|Cognee" docs/ CLAUDE.md AGENTS.md --include="*.md" | grep -v ".git"
```

Fix each occurrence.

**Step 2: Update ZAI URL references**

```bash
grep -rn "open.bigmodel.cn\|api\.z\.ai" docs/ CLAUDE.md --include="*.md"
```

Ensure all say `api.z.ai/api/paas/v4`.

**Step 3: Update remote container distribution flow docs**

In `docs/guides/deployment-guide.md` and `docs/ARCHITECTURE.md`, reflect the final `containers/.env`-driven orchestration flow.

**Step 4: Commit**

```bash
git add docs/ CLAUDE.md AGENTS.md
git commit -m "docs: full accuracy pass — Mem0 migration, ZAI URL, remote container flow"
```

---

# STREAM 7: Video Courses + Website

**Prerequisite:** GATE 1 (Stream 2 complete).

---

### Task 7.1: Update Video Course Scripts

**Files:**
- Modify: `docs/video-course/MODULE_SCRIPTS.md`
- Modify: `docs/video-course/VIDEO_METADATA.md`

**Step 1: Read existing MODULE_SCRIPTS.md to understand format**

**Step 2: Append 5 new module sections**

For each module, add:
```markdown
## Module N+1: [Module Name] — [One-line description]

### Duration: ~30 minutes
### Level: Intermediate

### Script Outline

**Section 1 (5 min): Introduction**
[Presenter narration script]
- Visual: Architecture diagram showing module's place
- Demo: Show the module running

**Section 2 (10 min): Core Concepts**
[Step-by-step walkthrough script with exact code]

**Section 3 (10 min): Live Demo**
[Demo script: what to type, what output to show]

**Section 4 (5 min): Integration Patterns**
[How to combine with other modules]

### Key Talking Points
1. ...
2. ...

### Code Examples for Screen Recording
[Exact commands to run during recording]
```

**Step 3: Update VIDEO_METADATA.md**

Add metadata for each new section: title, description, tags, thumbnail guidance.

**Step 4: Commit**

```bash
git add docs/video-course/
git commit -m "docs(video-course): add module scripts and metadata for 5 new modules"
```

---

### Task 7.2: Update Course Curriculum

**Files:**
- Modify: `docs/courses/COURSE_OUTLINE.md`
- Create: `docs/courses/slides/<module>-slides.md` (5 files)
- Create: `docs/courses/labs/<module>-lab.md` (5 files)
- Create: `docs/courses/assessments/<module>-quiz.md` (5 files)

**Step 1: Add 5 modules to COURSE_OUTLINE.md**

Under "Advanced Modules" section, add:
- Module N: Agentic Workflows (learning objectives, prerequisites, duration)
- Module N+1: LLMOps — Evaluation and Experimentation
- Module N+2: Planning — MCTS and Tree-of-Thoughts
- Module N+3: Self-Improvement Systems
- Module N+4: Benchmarking and Performance

**Step 2: Create slide decks for each module**

Each slide deck:
```markdown
# Module N: [Name]

## Slide 1: Overview
[Title + key message]

## Slide 2: Problem Statement
[What problem this solves]

## Slide 3: Architecture
[Diagram]

## Slide 4–8: Core Concepts
[One concept per slide with code example]

## Slide 9: Demo
[Live demo steps]

## Slide 10: Summary + Next Steps
[Key takeaways, links to docs]
```

**Step 3: Create hands-on labs**

Each lab:
```markdown
# Lab N: [Name]

## Objective
Build [specific thing] using [module].

## Prerequisites
- HelixAgent running
- API key set

## Steps (30 minutes)
1. [Step with exact command]
2. [Step with expected output]
...

## Expected Outcome
[Exactly what the student should see]

## Troubleshooting
[Common mistakes and fixes]
```

**Step 4: Commit**

```bash
git add docs/courses/
git commit -m "docs(courses): add curriculum, slides, labs, and quizzes for 5 new modules"
```

---

### Task 7.3: Update All Website Content

**Files:**
- Modify: all 10 files in `docs/website/`

**Step 1: Update FEATURES.md**

Add new feature entries for each of the 5 new modules:
```markdown
### Agentic Workflow Orchestration
Graph-based workflow engine with [N] built-in node types...

### LLMOps: Experiment Tracking
Track and compare LLM experiments with automatic metrics collection...
```

**Step 2: Update ARCHITECTURE.md**

Add new modules to the architecture diagram and description. Update module count (21 → 26 total).

**Step 3: Update LANDING_PAGE.md**

Add new capabilities to the "What HelixAgent does" section. Update module count in the hero text.

**Step 4: Update GETTING_STARTED.md**

Add references to new module APIs in the quick start guide.

**Step 5: Update INTEGRATIONS.md**

Add integration examples for new modules.

**Step 6: Review and update MEMORY_SYSTEM.md**

Ensure reflects latest Mem0-based implementation (not Cognee).

**Step 7: Review and update SECURITY.md**

Reflect latest security scan results and hardening measures.

**Step 8: Commit**

```bash
git add docs/website/
git commit -m "docs(website): update all 10 website pages with new modules and current state"
```

---

# STREAM 8: Challenges + Final Validation

**Prerequisite:** GATE 2 (Streams 1, 3, 4 complete AND Stream 2 complete).

---

### Task 8.1: Create Challenge Script for Agentic Module

**Files:**
- Create: `challenges/scripts/agentic_workflow_challenge.sh`

**Step 1: Write challenge script (20+ tests)**

```bash
#!/usr/bin/env bash
# agentic_workflow_challenge.sh - Validates Agentic workflow module
# Tests: 25

set -euo pipefail
PASS=0; FAIL=0
BASE_URL="${HELIX_URL:-http://localhost:7061}"
pass() { echo "✓ PASS: $1"; ((PASS++)); }
fail() { echo "✗ FAIL: $1"; ((FAIL++)); }

# Test 1: Agentic module package exists
if [ -d "Agentic/agentic" ]; then
    pass "Agentic module directory exists"
else
    fail "Agentic module directory missing"
fi

# Test 2: go.mod has correct module name
if grep -q "module digital.vasic.agentic" Agentic/go.mod 2>/dev/null; then
    pass "Agentic go.mod has correct module name"
else
    fail "Agentic go.mod missing or wrong module name"
fi

# Test 3: Adapter exists
if [ -f "internal/adapters/agentic/adapter.go" ]; then
    pass "Agentic adapter file exists"
else
    fail "Agentic adapter file missing"
fi

# Test 4: replace directive in root go.mod
if grep -q "digital.vasic.agentic.*./Agentic" go.mod; then
    pass "go.mod has replace directive for Agentic"
else
    fail "go.mod missing replace directive for Agentic"
fi

# Test 5: Unit tests pass
if GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 -short ./Agentic/... 2>&1 | grep -q "^ok"; then
    pass "Agentic module unit tests pass"
else
    fail "Agentic module unit tests fail"
fi

# Test 6: HTTP endpoint responds (requires running server)
if curl -sf -X POST "$BASE_URL/v1/agentic/workflow" \
    -H "Content-Type: application/json" \
    -d '{"workflow":"test","params":{}}' | grep -q "result\|error"; then
    pass "Agentic workflow endpoint responds"
else
    fail "Agentic workflow endpoint not responding"
fi

# ... 19 more tests: adapter tests pass, documentation exists, challenge scripts present,
# CLAUDE.md updated, AGENTS.md updated, all exported functions tested, etc.

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
```

**Step 2: Make executable and run**

```bash
chmod +x challenges/scripts/agentic_workflow_challenge.sh
bash challenges/scripts/agentic_workflow_challenge.sh
```
Expected: All 25 tests pass.

---

### Task 8.2: Create Challenge Scripts for Remaining 4 Modules

Following identical pattern as Task 8.1, create:

- `challenges/scripts/llmops_challenge.sh` — 20+ tests
- `challenges/scripts/selfimprove_challenge.sh` — 20+ tests
- `challenges/scripts/planning_challenge.sh` — 20+ tests
- `challenges/scripts/benchmark_challenge.sh` — 20+ tests

Each validates: module exists, go.mod correct, adapter exists, replace directive, tests pass, HTTP endpoint responds, docs present, CLAUDE.md/AGENTS.md updated.

**Commit all:**

```bash
git add challenges/scripts/
git commit -m "test(challenge): add challenge scripts for 5 new modules (20+ tests each)"
```

---

### Task 8.3: Create lazy_init_challenge.sh and stress_responsiveness_challenge.sh

**lazy_init_challenge.sh (15+ tests):**
- Verify no blocking `init()` in startup-critical path
- Measure startup time (< 5 seconds without provider verification)
- Verify sync.Once is used for lazy resources
- Run benchmark for lazy vs eager initialization

**stress_responsiveness_challenge.sh (15+ tests):**
- System responds to health check within 100ms under 100 concurrent requests
- Zero failed requests at 50 concurrent users
- 95th percentile latency < 2 seconds
- Provider registry accessible under 1000 concurrent reads
- Memory usage doesn't grow unbounded under sustained load

**Commit:**

```bash
git add challenges/scripts/lazy_init_challenge.sh challenges/scripts/stress_responsiveness_challenge.sh
git commit -m "test(challenge): add lazy_init_challenge.sh and stress_responsiveness_challenge.sh"
```

---

### Task 8.4: Update run_all_challenges.sh

**Step 1: Read current run_all_challenges.sh**

**Step 2: Add all new challenge scripts to the execution list**

```bash
# New module challenges
run_challenge "agentic_workflow_challenge.sh"
run_challenge "llmops_challenge.sh"
run_challenge "selfimprove_challenge.sh"
run_challenge "planning_challenge.sh"
run_challenge "benchmark_challenge.sh"

# Safety challenges
run_challenge "memory_race_challenge.sh"
run_challenge "lazy_init_challenge.sh"
run_challenge "stress_responsiveness_challenge.sh"

# Security challenges
run_challenge "sonarqube_challenge.sh"
run_challenge "snyk_dependency_challenge.sh"
```

**Step 3: Commit**

```bash
git add challenges/scripts/run_all_challenges.sh
git commit -m "chore: add 10 new challenge scripts to run_all_challenges.sh"
```

---

### Task 8.4b: Resolve TODO at main.go:1368

**File:** `cmd/helixagent/main.go:1368`

**Step 1: Read the context**

```go
// Auto-start LSP and RAG services (non-blocking, runs in background)
logger.Info("Starting LSP and RAG infrastructure services...")
// TODO: Re-implement infrastructure orchestration if needed
// startAllInfrastructure(logger)
```

**Step 2: Determine correct action**

The BootManager now handles all infrastructure. `startAllInfrastructure()` is superseded. Remove the TODO and add an explanatory comment:

```go
// LSP and RAG infrastructure is now managed by BootManager.BootAll()
// called during startup verification. No separate orchestration needed.
logger.Info("Infrastructure startup delegated to BootManager")
```

**Step 3: Build and test**

```bash
nice -n 19 go build ./cmd/helixagent/...
GOMAXPROCS=2 nice -n 19 go test -count=1 -p 1 -short ./cmd/helixagent/...
```

**Step 4: Commit**

```bash
git add cmd/helixagent/main.go
git commit -m "fix(main): remove TODO — infrastructure orchestration delegated to BootManager"
```

---

### Task 8.5: Final go vet + lint Pass

**Step 1: Run go vet on all project-owned packages**

```bash
nice -n 19 go vet ./cmd/... ./internal/... ./tests/... ./Agentic/... ./LLMOps/... ./SelfImprove/... ./Planning/... ./Benchmark/... 2>&1
```
Expected: 0 issues.

**Step 2: Run golangci-lint**

```bash
nice -n 19 golangci-lint run ./... 2>&1 | tee /tmp/lint-results.txt
```

**Step 3: Fix all lint issues**

For each issue in lint-results.txt, fix the flagged line. Commit in batches by package.

**Step 4: Run fmt**

```bash
nice -n 19 gofmt -w ./internal/... ./cmd/...
git diff --name-only
```
Commit any formatting changes.

**Step 5: Commit**

```bash
git commit -m "style: fix all golangci-lint issues across project-owned packages"
```

---

### Task 8.6: Final Validation — Run All Challenges

**Step 1: Ensure infrastructure is running**

```bash
make test-infra-start
```

**Step 2: Run all challenges**

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
    bash challenges/scripts/run_all_challenges.sh 2>&1 | tee /tmp/challenge-results.txt
```

**Step 3: Verify all pass**

```bash
grep -c "FAIL" /tmp/challenge-results.txt
```
Expected: `0`

**Step 4: Fix any failures before proceeding**

For each failing challenge, identify the root cause, fix, and re-run just that challenge.

---

### Task 8.7: Final CI Validation

**Step 1: Run make ci-validate-all**

```bash
nice -n 19 make ci-validate-all 2>&1 | tee /tmp/ci-results.txt
```
Expected: All checks pass.

**Step 2: Run full test suite**

```bash
GOMAXPROCS=2 nice -n 19 make test-all-types 2>&1 | tee /tmp/test-results.txt
```

**Step 3: Run race detector**

```bash
GOMAXPROCS=2 nice -n 19 go test -race -count=1 -p 1 -short ./internal/... ./cmd/... 2>&1 | grep -E "FAIL|DATA RACE|ok"
```
Expected: All `ok`, zero `FAIL`, zero `DATA RACE`.

**Step 4: Verify release build works**

```bash
make release 2>&1 | tail -20
```
Expected: Exits 0.

**Step 5: Final commit — mark completion**

```bash
git add -A
git commit -m "feat: complete comprehensive project audit — all streams done, all challenges pass"
```

---

## Gate 3: Push All Remotes

After all streams and final validation:

```bash
git push githubhelixdevelopment main
git push github main
# Push all submodule updates
git submodule foreach 'git push origin main 2>/dev/null || true'
```

---

## Success Checklist

| Item | Command | Expected |
|---|---|---|
| go vet clean | `go vet ./cmd/... ./internal/...` | Exit 0 |
| lint clean | `golangci-lint run ./...` | Exit 0 |
| Race-free | `go test -race -short ./internal/... ./cmd/...` | 0 DATA RACE |
| Goroutine-leak-free | goleak in TestMain | 0 leaks |
| All challenges pass | `run_all_challenges.sh` | 0 FAIL |
| Full test suite | `make test-all-types` | All PASS |
| Coverage | `make test-coverage` | ≥ 95% |
| Security scan | Snyk + SonarQube | 0 critical/high |
| Release build | `make release` | Exit 0 |
| All remotes synced | `git fetch --all` | 0 ahead/behind |
| Dead code | zero packages with 0 production imports | ✓ |
| Documentation | all modules have README+CLAUDE+AGENTS+docs/ | ✓ |

---

*Plan complete. Saved to `docs/plans/2026-02-23-comprehensive-completion-plan.md`.*
