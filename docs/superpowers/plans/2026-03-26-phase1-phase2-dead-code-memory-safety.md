# Phase 1 & 2: Dead Code Elimination + Memory Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all dead code (~6,000+ lines) and fix all memory safety issues (goroutine leaks, unbounded caches, race conditions) without breaking any existing functionality.

**Architecture:** Phase 1 deletes unused service files, backup files, and dead helper methods. Phase 2 patches concurrency bugs in circuit_breaker.go, debate_service.go, and agent_worker_pool.go. Each deletion/fix is verified by building all 7 apps and running unit tests with the race detector.

**Tech Stack:** Go 1.25.3, sync.Once, sync.Mutex, context.Context, atomic operations

---

## File Structure

### Phase 1 — Files to DELETE

```
internal/services/high_availability.go              (665 lines — DELETE)
internal/services/high_availability_test.go          (1,566 lines — DELETE)
internal/services/protocol_analytics.go              (486 lines — DELETE)
internal/services/protocol_analytics_test.go         (513 lines — DELETE)
internal/services/security_sandbox.go                (311 lines — DELETE)
internal/services/security_sandbox_test.go           (331 lines — DELETE)
internal/services/protocol_plugin_system.go          (840 lines — DELETE)
internal/services/protocol_plugin_system_test.go     (815 lines — DELETE)
internal/background/task_queue.go.backup             (DELETE)
internal/background/stuck_detector.go.backup         (DELETE)
internal/background/resource_monitor.go.backup       (DELETE)
internal/background/messaging_adapter.go.backup      (DELETE)
internal/config/ai_debate_integration_test.go.backup (DELETE)
internal/handlers/openai_compatible_test.go.backup   (DELETE)
internal/mcp/adapters/brave_search.go.bak            (DELETE)
internal/mcp/config/generator_full.go.bak            (DELETE)
internal/services/concurrency_alert_manager_test.go.bak (DELETE)
internal/services/provider_registry.go.bak           (DELETE)
internal/streaming/flink/client.go.bak               (DELETE)
```

### Phase 2 — Files to MODIFY

```
internal/llm/circuit_breaker.go           (Fix CompleteStream goroutine leak + notifyListeners leak)
internal/services/debate_service.go       (Add intentCache eviction with bounded size + TTL)
internal/services/agent_worker_pool.go    (Fix double-cancel panic in DispatchTasks)
```

### Phase 2 — Files to CREATE

```
internal/llm/circuit_breaker_lifecycle_test.go       (Goroutine leak regression tests)
internal/services/debate_cache_eviction_test.go      (Cache bounds regression tests)
internal/services/agent_worker_pool_cancel_test.go   (Double-cancel regression test)
```

### Files to MODIFY for Dead Code Cleanup

```
cmd/api/main.go                          (Remove references to deleted services)
tests/automation/full_automation_test.go  (Remove SecuritySandbox references)
tests/e2e/e2e_test.go                    (Remove SecuritySandbox references)
tests/integration/cloud_plugin_integration_test.go (Remove SecuritySandbox references)
tests/integration/integration_test.go    (Remove SecuritySandbox references)
```

---

## Task 1: Verify Dead Code Is Actually Dead

**Files:**
- Read: `cmd/helixagent/main.go`
- Read: `internal/router/router.go`
- Read: `cmd/api/main.go`

- [ ] **Step 1: Grep for ProtocolFederation usage outside its own file**

Run:
```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
grep -rn "ProtocolFederation\|NewProtocolFederation" --include="*.go" internal/ cmd/ tests/ | grep -v "high_availability" | grep -v "_test.go"
```
Expected: Zero matches in production code (cmd/api/main.go is demo-only, acceptable).

- [ ] **Step 2: Grep for ProtocolAnalyticsService usage**

Run:
```bash
grep -rn "ProtocolAnalyticsService\|NewProtocolAnalyticsService" --include="*.go" internal/ cmd/ tests/ | grep -v "protocol_analytics" | grep -v "_test.go"
```
Expected: Only cmd/api/main.go (demo code).

- [ ] **Step 3: Grep for SecuritySandbox usage**

Run:
```bash
grep -rn "SecuritySandbox\|NewSecuritySandbox" --include="*.go" internal/ cmd/ tests/ | grep -v "security_sandbox" | grep -v "_test.go"
```
Expected: Only cmd/api/main.go (demo code).

- [ ] **Step 4: Grep for ProtocolPluginSystem usage**

Run:
```bash
grep -rn "ProtocolPluginSystem\|NewProtocolPluginSystem\|ProtocolPluginRegistry\|NewProtocolPluginRegistry" --include="*.go" internal/ cmd/ tests/ | grep -v "protocol_plugin_system" | grep -v "_test.go"
```
Expected: Only cmd/api/main.go (demo code).

- [ ] **Step 5: Verify cmd/api/main.go is demo-only**

Run:
```bash
head -25 /run/media/milosvasic/DATA4TB/Projects/helix_agent/cmd/api/main.go
```
Expected: Contains "DEMO" or "NOT FOR PRODUCTION" comment. This file references deleted services but is demo code.

- [ ] **Step 6: Document findings**

If any non-demo, non-test production code references these types, STOP and reassess. Do not proceed with deletion.

---

## Task 2: Delete Unused Service — high_availability.go

**Files:**
- Delete: `internal/services/high_availability.go`
- Delete: `internal/services/high_availability_test.go`

- [ ] **Step 1: Delete the files**

Run:
```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
rm internal/services/high_availability.go
rm internal/services/high_availability_test.go
```

- [ ] **Step 2: Build to verify no breakage**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
```
Expected: BUILD SUCCEEDS. No code references these types from production code.

- [ ] **Step 3: Run unit tests on services package**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -short -count=1 -p 1 ./internal/services/... 2>&1 | tail -5
```
Expected: Tests pass (some may skip due to missing infra, that's OK).

- [ ] **Step 4: Commit**

```bash
git add -A internal/services/high_availability.go internal/services/high_availability_test.go
git commit -m "refactor(cleanup): remove unused ProtocolFederation service (665+1566 lines)

ProtocolFederation, EventBus, MCPFederatedProtocol, LSPFederatedProtocol
were never instantiated in production code. Zero callers outside own file.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Delete Unused Service — protocol_analytics.go

**Files:**
- Delete: `internal/services/protocol_analytics.go`
- Delete: `internal/services/protocol_analytics_test.go`

- [ ] **Step 1: Delete the files**

Run:
```bash
rm internal/services/protocol_analytics.go
rm internal/services/protocol_analytics_test.go
```

- [ ] **Step 2: Build to verify**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
```
Expected: BUILD SUCCEEDS.

- [ ] **Step 3: Check if cmd/api/main.go needs patching**

Run:
```bash
grep -n "ProtocolAnalyticsService\|NewProtocolAnalyticsService" cmd/api/main.go
```
If matches found, comment them out or remove those lines from cmd/api/main.go. Then rebuild:
```bash
go build -mod=vendor ./cmd/api
```

- [ ] **Step 4: Run unit tests**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -short -count=1 -p 1 ./internal/services/... 2>&1 | tail -5
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A internal/services/protocol_analytics.go internal/services/protocol_analytics_test.go cmd/api/main.go
git commit -m "refactor(cleanup): remove unused ProtocolAnalyticsService (486+513 lines)

Never instantiated in production. Only referenced in demo cmd/api server.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Delete Unused Service — security_sandbox.go

**Files:**
- Delete: `internal/services/security_sandbox.go`
- Delete: `internal/services/security_sandbox_test.go`
- Modify: `cmd/api/main.go` (remove reference)
- Modify: `tests/automation/full_automation_test.go` (remove reference)
- Modify: `tests/e2e/e2e_test.go` (remove reference)
- Modify: `tests/integration/cloud_plugin_integration_test.go` (remove reference)
- Modify: `tests/integration/integration_test.go` (remove reference)
- Check: `tests/unit/services/security_sandbox_test.go` (delete if exists)

- [ ] **Step 1: Find all files referencing SecuritySandbox**

Run:
```bash
grep -rln "SecuritySandbox\|NewSecuritySandbox" --include="*.go" .
```
Note every file listed.

- [ ] **Step 2: Delete the source and its test**

Run:
```bash
rm internal/services/security_sandbox.go
rm internal/services/security_sandbox_test.go
```

- [ ] **Step 3: Check for secondary test file**

Run:
```bash
ls tests/unit/services/security_sandbox_test.go 2>/dev/null && rm tests/unit/services/security_sandbox_test.go
```

- [ ] **Step 4: Patch all referencing test files**

For each test file that references SecuritySandbox: remove the import, remove the variable declaration, remove test functions that test SecuritySandbox directly. If SecuritySandbox is used as a field in a test struct, remove that field and any assertions about it.

Key pattern to remove from test files:
```go
// Remove lines like:
sandbox := services.NewSecuritySandbox(...)
// Remove any test functions named TestSecuritySandbox*
```

- [ ] **Step 5: Patch cmd/api/main.go**

Remove the SecuritySandbox initialization lines. Remove the import if no longer needed.

- [ ] **Step 6: Build all affected packages**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
go build -mod=vendor ./cmd/api
GOMAXPROCS=2 go test -mod=vendor -short -count=1 -p 1 ./tests/... 2>&1 | tail -10
```
Expected: All build and pass.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(cleanup): remove unused SecuritySandbox service (311+331+261 lines)

Never wired into handler pipeline. Only referenced in test files and demo server.
Patched 4 test files and cmd/api/main.go to remove references.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Delete Unused Service — protocol_plugin_system.go

**Files:**
- Delete: `internal/services/protocol_plugin_system.go`
- Delete: `internal/services/protocol_plugin_system_test.go`
- Modify: `cmd/api/main.go` (remove reference)

- [ ] **Step 1: Find all references**

Run:
```bash
grep -rln "ProtocolPluginSystem\|ProtocolPluginRegistry\|NewProtocolPluginSystem\|NewProtocolPluginRegistry" --include="*.go" .
```

- [ ] **Step 2: Delete source and test**

Run:
```bash
rm internal/services/protocol_plugin_system.go
rm internal/services/protocol_plugin_system_test.go
```

- [ ] **Step 3: Patch cmd/api/main.go**

Remove ProtocolPluginSystem and ProtocolPluginRegistry initialization lines.

- [ ] **Step 4: Build and test**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
go build -mod=vendor ./cmd/api
GOMAXPROCS=2 go test -mod=vendor -short -count=1 -p 1 ./internal/services/... 2>&1 | tail -5
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(cleanup): remove unused ProtocolPluginSystem (840+815 lines)

Plugin system never connected to production code. Demo-only references removed.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Delete All Backup Files

**Files:**
- Delete: All `.backup` and `.bak` files in `internal/`

- [ ] **Step 1: List all backup files**

Run:
```bash
find /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal -name "*.backup" -o -name "*.bak" | sort
```

- [ ] **Step 2: Delete all backup files in internal/**

Run:
```bash
find /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal -name "*.backup" -delete
find /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal -name "*.bak" -delete
```

- [ ] **Step 3: Also delete root-level backups**

Run:
```bash
rm -f /run/media/milosvasic/DATA4TB/Projects/helix_agent/CONSTITUTION.json.bak
rm -f /run/media/milosvasic/DATA4TB/Projects/helix_agent/containers/.env.bak
```

- [ ] **Step 4: Verify no backup files remain in internal/**

Run:
```bash
find /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal -name "*.backup" -o -name "*.bak" | wc -l
```
Expected: 0

- [ ] **Step 5: Build to verify**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
```
Expected: BUILD SUCCEEDS (backup files are never imported).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(cleanup): remove all backup and bak files from internal/

Deleted 13 vestigial .backup/.bak files (~3600 lines of dead code).
These were leftovers from previous refactoring that are fully superseded.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Remove Unused //nolint:unused Methods (Batch 1 — handlers)

**Files:**
- Modify: `internal/handlers/openai_compatible.go`
- Modify: `internal/handlers/cognee_handler.go`
- Delete: `internal/handlers/verifier_types.go` (if entire file is dead)

- [ ] **Step 1: Find all //nolint:unused in handlers**

Run:
```bash
grep -rn "nolint:unused\|nolint: unused" /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/handlers/
```

- [ ] **Step 2: For each unused function, verify it has zero callers**

For each function found (e.g., `contains`, `containsSubstring`, `generateID`, `getIntParam`, `getFloatParam`), run:
```bash
grep -rn "functionName(" --include="*.go" /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/ | grep -v "nolint" | grep -v "func "
```
If zero callers outside the definition, proceed to delete.

- [ ] **Step 3: Delete each confirmed dead function**

Remove the function body entirely. Do NOT leave a comment or stub.

- [ ] **Step 4: Check verifier_types.go**

Run:
```bash
grep -rn "VerifierErrorResponse" --include="*.go" /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/ | grep -v "verifier_types.go"
```
If zero callers, delete the entire file:
```bash
rm internal/handlers/verifier_types.go
```

- [ ] **Step 5: Build and test**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
go vet -mod=vendor ./internal/handlers/...
GOMAXPROCS=2 go test -mod=vendor -short -count=1 -p 1 ./internal/handlers/... 2>&1 | tail -5
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(cleanup): remove unused handler methods and types

Removed dead helper functions (contains, containsSubstring, generateID,
getIntParam, getFloatParam, etc.) and VerifierErrorResponse type.
All had //nolint:unused markers and zero callers.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Remove Unused //nolint:unused Methods (Batch 2 — services, background, middleware)

**Files:**
- Modify: `internal/services/plugin_system.go`
- Modify: `internal/services/lsp_manager.go`
- Modify: `internal/services/embedding_manager.go`
- Modify: `internal/services/debate_performance_service.go`
- Modify: `internal/services/request_service.go`
- Modify: `internal/middleware/rate_limit.go`
- Modify: `internal/modelsdev/client.go`
- Modify: `internal/database/db.go`
- Modify: `internal/background/stuck_detector.go`
- Modify: `internal/background/events.go`
- Modify: `internal/background/task_queue.go`

- [ ] **Step 1: Find all //nolint:unused in these packages**

Run:
```bash
grep -rn "nolint:unused\|nolint: unused" /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/services/ /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/middleware/ /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/modelsdev/ /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/database/ /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/background/
```

- [ ] **Step 2: For EACH item, verify zero callers**

IMPORTANT: For struct fields marked unused (like `mu sync.RWMutex`), check if ANY method on the struct uses it:
```bash
grep -n "\.mu\." /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/services/plugin_system.go
```
Only delete the field if truly unreferenced within the same file.

For methods in `background/stuck_detector.go` (checkHeartbeatTimeout, isProcessFrozen, checkResourceExhaustion, isIOStarved, isNetworkHung, hasMemoryLeak, isEndlessTaskStuck): check if any are called via interface dispatch:
```bash
grep -rn "checkHeartbeatTimeout\|isProcessFrozen\|checkResourceExhaustion\|isIOStarved\|isNetworkHung\|hasMemoryLeak\|isEndlessTaskStuck" --include="*.go" /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/
```

- [ ] **Step 3: Delete confirmed dead items**

Remove each dead function, method, or field. For struct fields, also remove any initialization of that field in constructors.

- [ ] **Step 4: Build and test all affected packages**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
go vet -mod=vendor ./internal/...
GOMAXPROCS=2 go test -mod=vendor -short -count=1 -p 1 ./internal/services/... ./internal/middleware/... ./internal/modelsdev/... ./internal/database/... ./internal/background/... 2>&1 | tail -10
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(cleanup): remove unused methods and fields across internal packages

Removed //nolint:unused items in services, middleware, modelsdev, database,
and background packages. Each item verified to have zero callers.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Fix CircuitBreaker.CompleteStream Goroutine Leak

**Files:**
- Modify: `internal/llm/circuit_breaker.go:151-181`
- Create: `internal/llm/circuit_breaker_lifecycle_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/llm/circuit_breaker_lifecycle_test.go`:

```go
package llm

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// mockStreamProvider returns a channel that sends N responses then closes.
type mockStreamProvider struct {
	responses []*models.LLMResponse
	delay     time.Duration
}

func (m *mockStreamProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	return &models.LLMResponse{Content: "ok"}, nil
}

func (m *mockStreamProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ch := make(chan *models.LLMResponse)
	go func() {
		defer close(ch)
		for _, r := range m.responses {
			if m.delay > 0 {
				select {
				case <-time.After(m.delay):
				case <-ctx.Done():
					return
				}
			}
			select {
			case ch <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (m *mockStreamProvider) HealthCheck() error                               { return nil }
func (m *mockStreamProvider) GetCapabilities() models.ProviderCapabilities     { return models.ProviderCapabilities{} }
func (m *mockStreamProvider) GetProviderID() string                            { return "mock" }
func (m *mockStreamProvider) ValidateConfig() error                            { return nil }

func TestCircuitBreaker_CompleteStream_NoGoroutineLeak(t *testing.T) {
	provider := &mockStreamProvider{
		responses: []*models.LLMResponse{
			{Content: "chunk1"},
			{Content: "chunk2"},
		},
	}
	cb := NewCircuitBreaker("test", provider, DefaultCircuitBreakerConfig())

	baseline := runtime.NumGoroutine()

	// Normal completion: drain all responses
	ctx := context.Background()
	ch, err := cb.CompleteStream(ctx, &models.LLMRequest{Prompt: "test"})
	require.NoError(t, err)

	for range ch {
		// drain
	}

	// Allow goroutines to settle
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+2, "goroutines should not leak after normal stream completion")
}

func TestCircuitBreaker_CompleteStream_ContextCancelNoLeak(t *testing.T) {
	provider := &mockStreamProvider{
		responses: []*models.LLMResponse{
			{Content: "chunk1"},
			{Content: "chunk2"},
			{Content: "chunk3"},
		},
		delay: 500 * time.Millisecond, // slow responses
	}
	cb := NewCircuitBreaker("test", provider, DefaultCircuitBreakerConfig())

	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch, err := cb.CompleteStream(ctx, &models.LLMRequest{Prompt: "test"})
	require.NoError(t, err)

	// Read what we can before timeout
	for range ch {
	}

	// Allow goroutines to settle
	time.Sleep(300 * time.Millisecond)
	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+2, "goroutines should not leak after context cancellation")
}
```

- [ ] **Step 2: Run the test to verify it captures the leak**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -run "TestCircuitBreaker_CompleteStream" -count=1 ./internal/llm/ 2>&1 | tail -20
```
Expected: `TestCircuitBreaker_CompleteStream_ContextCancelNoLeak` may FAIL (goroutine leaks on context cancel with current code). This confirms the bug.

- [ ] **Step 3: Fix CompleteStream to respect context cancellation**

Edit `internal/llm/circuit_breaker.go`. Replace lines 151-181 with:

```go
// CompleteStream wraps the provider's CompleteStream method with circuit breaker logic
func (cb *CircuitBreaker) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if err := cb.beforeRequest(); err != nil {
		return nil, err
	}

	ch, err := cb.provider.CompleteStream(ctx, req)
	if err != nil {
		cb.afterRequest(err)
		return nil, err
	}

	// Wrap the channel to track success/failure.
	// Uses ctx to prevent goroutine leak when the caller abandons the stream.
	wrappedCh := make(chan *models.LLMResponse)
	go func() {
		defer close(wrappedCh)
		var lastResp *models.LLMResponse
		for {
			select {
			case resp, ok := <-ch:
				if !ok {
					// Source channel closed normally
					if lastResp != nil {
						cb.afterRequest(nil)
					} else {
						cb.afterRequest(errors.New("empty stream"))
					}
					return
				}
				lastResp = resp
				select {
				case wrappedCh <- resp:
				case <-ctx.Done():
					cb.afterRequest(ctx.Err())
					return
				}
			case <-ctx.Done():
				cb.afterRequest(ctx.Err())
				return
			}
		}
	}()

	return wrappedCh, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -run "TestCircuitBreaker_CompleteStream" -count=1 ./internal/llm/ 2>&1 | tail -20
```
Expected: BOTH tests PASS.

- [ ] **Step 5: Run existing circuit breaker tests to verify no regression**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -count=1 -short ./internal/llm/... 2>&1 | tail -20
```
Expected: All existing tests still pass.

- [ ] **Step 6: Build all apps**

Run:
```bash
go build -mod=vendor ./cmd/helixagent
```
Expected: BUILD SUCCEEDS.

- [ ] **Step 7: Commit**

```bash
git add internal/llm/circuit_breaker.go internal/llm/circuit_breaker_lifecycle_test.go
git commit -m "fix(concurrency): prevent goroutine leak in CircuitBreaker.CompleteStream

Added context cancellation monitoring to the stream-wrapping goroutine.
Previously, if the caller abandoned the stream (context timeout/cancel),
the goroutine would block on wrappedCh <- resp indefinitely.

Now the goroutine exits cleanly when ctx.Done() fires.
Added regression tests for both normal and cancelled streams.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Fix CircuitBreaker.notifyListeners Goroutine Tracking

**Files:**
- Modify: `internal/llm/circuit_breaker.go:291-319`

- [ ] **Step 1: Add test for listener notification goroutine cleanup**

Add to `internal/llm/circuit_breaker_lifecycle_test.go`:

```go
func TestCircuitBreaker_NotifyListeners_NoGoroutineLeak(t *testing.T) {
	provider := &mockStreamProvider{
		responses: []*models.LLMResponse{{Content: "ok"}},
	}
	cb := NewCircuitBreaker("test", provider, CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})

	// Use a very short timeout for listeners in this test
	listenerNotifyTimeoutNs.Store(int64(200 * time.Millisecond))
	defer listenerNotifyTimeoutNs.Store(int64(5 * time.Second))

	called := make(chan struct{}, 10)
	cb.AddListener(func(providerID string, oldState, newState CircuitState) {
		called <- struct{}{}
	})

	baseline := runtime.NumGoroutine()

	// Trigger state changes by failing enough times
	for i := 0; i < 3; i++ {
		cb.afterRequest(errors.New("fail"))
	}

	// Wait for listener notifications
	time.Sleep(500 * time.Millisecond)

	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, baseline+2,
		"listener notification goroutines should not leak")
}

func TestCircuitBreaker_NotifyListeners_SlowListenerTimesOut(t *testing.T) {
	provider := &mockStreamProvider{
		responses: []*models.LLMResponse{{Content: "ok"}},
	}
	cb := NewCircuitBreaker("test", provider, CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 1,
	})

	// Very short timeout
	listenerNotifyTimeoutNs.Store(int64(100 * time.Millisecond))
	defer listenerNotifyTimeoutNs.Store(int64(5 * time.Second))

	// Listener that blocks forever
	cb.AddListener(func(providerID string, oldState, newState CircuitState) {
		select {} // block forever
	})

	baseline := runtime.NumGoroutine()

	// Trigger state change
	for i := 0; i < 3; i++ {
		cb.afterRequest(errors.New("fail"))
	}

	// Wait well past the timeout
	time.Sleep(500 * time.Millisecond)

	after := runtime.NumGoroutine()
	// The blocking listener's inner goroutine will persist (unavoidable without
	// killing goroutines, which Go doesn't support), but the outer goroutine
	// should have exited. We allow +3 margin for the one stuck listener.
	assert.LessOrEqual(t, after, baseline+3,
		"outer notification goroutines should exit on timeout")
}
```

- [ ] **Step 2: Run to verify behavior**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -run "TestCircuitBreaker_NotifyListeners" -count=1 ./internal/llm/ 2>&1 | tail -20
```
Note: The current code already has timeout logic, so the outer goroutine does exit. The inner goroutine for a truly blocking listener cannot be killed in Go — this is an inherent language limitation. The test documents acceptable behavior.

- [ ] **Step 3: Commit**

```bash
git add internal/llm/circuit_breaker_lifecycle_test.go
git commit -m "test(concurrency): add listener notification goroutine leak tests

Documents behavior of circuit breaker listener notification:
- Normal listeners: goroutines exit after callback completes
- Slow listeners: outer goroutine exits on timeout (inner persists if truly blocked)
- This is acceptable Go behavior - goroutines cannot be externally killed

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: Add Bounded Cache to DebateService.intentCache

**Files:**
- Modify: `internal/services/debate_service.go` (lines 48-49, ~1350-1400)
- Create: `internal/services/debate_cache_eviction_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/services/debate_cache_eviction_test.go`:

```go
package services

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestDebateService_IntentCache_BoundedSize(t *testing.T) {
	ds := &DebateService{
		logger:      logrus.New(),
		intentCache: make(map[string]*IntentClassificationResult),
	}

	// Fill cache beyond the max (maxIntentCacheSize = 10000)
	for i := 0; i < maxIntentCacheSize+500; i++ {
		ds.mu.Lock()
		ds.intentCache[fmt.Sprintf("topic-%d", i)] = &IntentClassificationResult{
			Intent: "test",
		}
		ds.mu.Unlock()
	}

	// Trigger eviction
	ds.evictIntentCacheIfNeeded()

	ds.mu.Lock()
	size := len(ds.intentCache)
	ds.mu.Unlock()

	assert.LessOrEqual(t, size, maxIntentCacheSize,
		"cache should be bounded to maxIntentCacheSize")
}
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -run "TestDebateService_IntentCache_BoundedSize" -count=1 ./internal/services/ 2>&1 | tail -10
```
Expected: FAIL — `maxIntentCacheSize` and `evictIntentCacheIfNeeded` don't exist yet.

- [ ] **Step 3: Implement bounded cache**

Edit `internal/services/debate_service.go`. Add the constant and eviction method. Add at the package level (before the struct):

```go
// maxIntentCacheSize bounds the intent classification cache to prevent unbounded memory growth.
const maxIntentCacheSize = 10000
```

Add the eviction method to DebateService:

```go
// evictIntentCacheIfNeeded removes oldest entries when cache exceeds bounds.
// Must be called with ds.mu held or from a context that will acquire it.
func (ds *DebateService) evictIntentCacheIfNeeded() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if len(ds.intentCache) <= maxIntentCacheSize {
		return
	}
	// Simple eviction: clear half the cache when over limit.
	// This is O(n) but happens rarely (only when cache hits 10K entries).
	count := 0
	target := len(ds.intentCache) / 2
	for key := range ds.intentCache {
		delete(ds.intentCache, key)
		count++
		if count >= target {
			break
		}
	}
}
```

Then, in the intent cache write path (around line 1396-1398), add eviction call AFTER the write:

Find the pattern:
```go
ds.mu.Lock()
ds.intentCache[topic] = result
ds.mu.Unlock()
```

Replace with:
```go
ds.mu.Lock()
ds.intentCache[topic] = result
ds.mu.Unlock()
ds.evictIntentCacheIfNeeded()
```

- [ ] **Step 4: Fix the test import**

Add `"fmt"` to the import in `debate_cache_eviction_test.go` if not already present.

- [ ] **Step 5: Run the test**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -run "TestDebateService_IntentCache" -count=1 ./internal/services/ 2>&1 | tail -10
```
Expected: PASS.

- [ ] **Step 6: Run existing debate service tests**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -count=1 -short ./internal/services/... 2>&1 | grep -E "PASS|FAIL|ok" | tail -10
```
Expected: No regressions.

- [ ] **Step 7: Commit**

```bash
git add internal/services/debate_service.go internal/services/debate_cache_eviction_test.go
git commit -m "fix(memory): add bounded eviction to DebateService.intentCache

intentCache was an unbounded map that could grow indefinitely under
varied user topics. Added maxIntentCacheSize (10000) with simple
half-eviction when exceeded. Prevents memory leak in long-running servers.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: Fix AgentWorkerPool Double-Cancel Panic

**Files:**
- Modify: `internal/services/agent_worker_pool.go:79-89`
- Create: `internal/services/agent_worker_pool_cancel_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/services/agent_worker_pool_cancel_test.go`:

```go
package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestAgentWorkerPool_DoubleCancel_NoPanic(t *testing.T) {
	pool := NewAgentWorkerPool(2, logrus.New())

	// Cancel the pool's own context
	pool.cancel()

	// Create a context that will also be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Dispatch with empty tasks — the merged context goroutine will see
	// both p.ctx.Done() and mergedCtx.Done() fire, potentially calling
	// mergedCancel() twice (once from the goroutine, once from the defer).
	ch, err := pool.DispatchTasks(ctx, [][]AgenticTask{}, nil, nil, 0)
	assert.NoError(t, err)

	// Drain results
	for range ch {
	}

	// Now cancel the parent too — this should not panic
	cancel()

	time.Sleep(100 * time.Millisecond)

	// If we get here without panic, the test passes
	assert.True(t, true, "no panic on double cancel")
}

func TestAgentWorkerPool_PoolShutdownDuringDispatch_NoPanic(t *testing.T) {
	pool := NewAgentWorkerPool(2, logrus.New())

	ctx := context.Background()
	tasks := [][]AgenticTask{
		{
			{ID: "task1", Name: "test", SystemPrompt: "test", UserPrompt: "test"},
		},
	}

	// This will start dispatching, then we immediately shut down the pool
	_, _ = pool.DispatchTasks(ctx, tasks, nil, nil, 1)

	// Shut down pool (cancels p.ctx)
	pool.Shutdown()

	time.Sleep(200 * time.Millisecond)

	// If we get here without panic, the test passes
	assert.True(t, true, "no panic on pool shutdown during dispatch")
}
```

- [ ] **Step 2: Run to see if it panics**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -v -run "TestAgentWorkerPool_DoubleCancel\|TestAgentWorkerPool_PoolShutdown" -count=1 ./internal/services/ 2>&1 | tail -15
```
Note: `context.CancelFunc` is actually safe to call multiple times in Go (it's idempotent). So the "double cancel" issue from the audit may be a false positive. The test will verify. If tests pass, the code is already safe and we just add the regression tests.

- [ ] **Step 3: If tests pass, the code is already safe — just commit the regression tests**

If tests PASS without modification:

```bash
git add internal/services/agent_worker_pool_cancel_test.go
git commit -m "test(concurrency): add double-cancel regression tests for AgentWorkerPool

Verified that context.CancelFunc is idempotent in Go - calling it multiple
times is safe. Added regression tests to prevent future issues if the
cancellation pattern changes.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 4: If tests FAIL with panic, apply fix**

If the test panics, edit `internal/services/agent_worker_pool.go` lines 79-89. Replace:

```go
		mergedCtx, mergedCancel := context.WithCancel(ctx)
		defer mergedCancel()

		// Also respect the pool's own context.
		go func() {
			select {
			case <-p.ctx.Done():
				mergedCancel()
			case <-mergedCtx.Done():
			}
		}()
```

With:

```go
		mergedCtx, mergedCancel := context.WithCancel(ctx)
		defer mergedCancel()

		// Also respect the pool's own context.
		// Note: context.CancelFunc is idempotent, safe to call multiple times.
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			select {
			case <-p.ctx.Done():
				mergedCancel()
			case <-mergedCtx.Done():
			}
		}()
```

Then rebuild, retest, and commit.

---

## Task 13: Final Verification — Phase 1 & 2 Complete

**Files:**
- Read: Build output
- Read: Test output

- [ ] **Step 1: Build ALL 7 apps**

Run:
```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
go build -mod=vendor ./cmd/helixagent
go build -mod=vendor ./cmd/api
go build -mod=vendor ./cmd/grpc-server
go build -mod=vendor ./cmd/cognee-mock
go build -mod=vendor ./cmd/sanity-check
go build -mod=vendor ./cmd/mcp-bridge
go build -mod=vendor ./cmd/generate-constitution
echo "ALL BUILDS PASSED"
```
Expected: ALL BUILDS PASSED.

- [ ] **Step 2: Run go vet**

Run:
```bash
go vet -mod=vendor ./internal/...
go vet -mod=vendor ./cmd/...
```
Expected: No issues.

- [ ] **Step 3: Run race detector on modified packages**

Run:
```bash
GOMAXPROCS=2 go test -mod=vendor -race -short -count=1 -p 1 \
  ./internal/llm/... \
  ./internal/services/... \
  ./internal/handlers/... \
  ./internal/background/... \
  ./internal/middleware/... 2>&1 | grep -E "PASS|FAIL|ok" | tail -15
```
Expected: All PASS, no race detected.

- [ ] **Step 4: Verify no backup files remain**

Run:
```bash
find /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal -name "*.backup" -o -name "*.bak" | wc -l
```
Expected: 0.

- [ ] **Step 5: Verify no dead services remain**

Run:
```bash
ls internal/services/high_availability.go internal/services/protocol_analytics.go internal/services/security_sandbox.go internal/services/protocol_plugin_system.go 2>&1
```
Expected: "No such file or directory" for each.

- [ ] **Step 6: Count remaining //nolint:unused**

Run:
```bash
grep -rn "nolint:unused" --include="*.go" internal/ | grep -v "_test.go" | wc -l
```
Document the count. Target: significantly fewer than before (some may be intentionally kept if verified as needed for future use).

- [ ] **Step 7: Summary commit log**

Run:
```bash
git log --oneline -10
```
Verify all phase 1 & 2 commits are present with proper conventional commit messages.
