#!/bin/bash
# Memory Safety Challenge
# Validates goroutine lifecycle management, no leaks, proper shutdown
# across all critical services: rate limiter, worker pool, memory service

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

init_challenge "memory_safety" "Memory Safety and Goroutine Lifecycle"

PASS_COUNT=0
FAIL_COUNT=0
TOTAL_COUNT=0

check() {
    local desc="$1"
    local result="$2"
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    if [ "$result" -eq 0 ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        log_pass "$desc"
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        log_fail "$desc"
    fi
}

# --- Section 1: Verify safety patterns in source code ---

log_section "Section 1: Goroutine Lifecycle Patterns"

# 1.1: Rate limiter has context cancellation
grep -q "context.WithCancel" "$PROJECT_ROOT/internal/middleware/rate_limit.go"
check "Rate limiter uses context.WithCancel for goroutine lifecycle" $?

# 1.2: Rate limiter has WaitGroup
grep -q "sync.WaitGroup" "$PROJECT_ROOT/internal/middleware/rate_limit.go"
check "Rate limiter uses sync.WaitGroup for shutdown tracking" $?

# 1.3: Rate limiter has Stop method
grep -q "func (rl \*RateLimiter) Stop()" "$PROJECT_ROOT/internal/middleware/rate_limit.go"
check "Rate limiter has Stop() method for graceful shutdown" $?

# 1.4: Rate limiter cleanup uses select with ctx.Done
grep -q "rl.ctx.Done()" "$PROJECT_ROOT/internal/middleware/rate_limit.go"
check "Rate limiter cleanup loop uses select with ctx.Done()" $?

# 1.5: Rate limiter cleanup has defer wg.Done
grep -q "defer rl.wg.Done()" "$PROJECT_ROOT/internal/middleware/rate_limit.go"
check "Rate limiter cleanup goroutine defers wg.Done()" $?

# 1.6: Worker pool started flag is atomic
grep -q "atomic.LoadInt32(&wp.started)" "$PROJECT_ROOT/internal/background/worker_pool.go"
check "Worker pool started flag uses atomic operations" $?

# 1.7: Worker pool has IsStarted method
grep -q "func (wp \*AdaptiveWorkerPool) IsStarted() bool" "$PROJECT_ROOT/internal/background/worker_pool.go"
check "Worker pool has thread-safe IsStarted() method" $?

# 1.8: Worker pool uses atomic.StoreInt32 for started
grep -q "atomic.StoreInt32(&wp.started" "$PROJECT_ROOT/internal/background/worker_pool.go"
check "Worker pool uses atomic.StoreInt32 for started flag" $?

# 1.9: Memory service has WaitGroup
grep -q "sync.WaitGroup" "$PROJECT_ROOT/internal/services/memory_service.go"
check "Memory service uses sync.WaitGroup for shutdown tracking" $?

# 1.10: Memory service cleanup defers wg.Done
grep -q "defer m.wg.Done()" "$PROJECT_ROOT/internal/services/memory_service.go"
check "Memory service cleanup goroutine defers wg.Done()" $?

# 1.11: Memory service Stop waits for WaitGroup
grep -q "m.wg.Wait()" "$PROJECT_ROOT/internal/services/memory_service.go"
check "Memory service Stop() waits for WaitGroup" $?

# 1.12: Modelsdev cache has proper cleanup
grep -q "cleanupDone" "$PROJECT_ROOT/internal/modelsdev/cache.go"
check "Modelsdev cache uses cleanupDone channel for shutdown confirmation" $?

# 1.13: Modelsdev cache Close waits on cleanupDone
grep -q "<-c.cleanupDone" "$PROJECT_ROOT/internal/modelsdev/cache.go"
check "Modelsdev cache Close() waits on cleanupDone channel" $?

# --- Section 2: Verify TLS security defaults ---

log_section "Section 2: TLS Security Defaults"

# 2.1: InsecureSkipVerify defaults to false
grep -q 'getEnvBool("HELIX_LLM_TLS_SKIP_VERIFY", false)' "$PROJECT_ROOT/internal/llm/providers/helixllm/provider.go"
check "HelixLLM TLS InsecureSkipVerify defaults to false (secure)" $?

# 2.2: No default true for TLS skip anywhere
! grep -q 'HELIX_LLM_TLS_SKIP_VERIFY.*true)' "$PROJECT_ROOT/internal/llm/providers/helixllm/provider.go"
check "No insecure TLS default (true) in HelixLLM provider" $?

# --- Section 3: Verify concurrency limiter is wired ---

log_section "Section 3: Concurrency Safety Infrastructure"

# 3.1: Concurrency limiter exists
test -f "$PROJECT_ROOT/internal/middleware/concurrency_limiter.go"
check "Concurrency limiter middleware exists" $?

# 3.2: Concurrency limiter is wired into router
grep -q "ConcurrencyLimiter" "$PROJECT_ROOT/internal/router/router.go"
check "Concurrency limiter is wired into the main router" $?

# 3.3: Worker pool uses sync.Once for stop safety
grep -q "stopOnce" "$PROJECT_ROOT/internal/background/worker_pool.go"
check "Worker pool uses sync.Once to prevent double-close panic" $?

# --- Section 4: Run race-sensitive unit tests ---

log_section "Section 4: Race Condition Detection"

# 4.1: Run rate limiter tests with race detector
GOMAXPROCS=2 nice -n 19 go test -race -run "TestRateLimiter_GracefulShutdown|TestRateLimiter_ConcurrentAccess" \
    ./internal/middleware/ -count=1 -p 1 -short > /dev/null 2>&1
check "Rate limiter passes race detector" $?

# 4.2: Run worker pool tests with race detector
GOMAXPROCS=2 nice -n 19 go test -race -run "TestAdaptiveWorkerPool|TestWorkerState" \
    ./internal/background/ -count=1 -p 1 -short > /dev/null 2>&1
check "Worker pool passes race detector" $?

# 4.3: Run memory service tests with race detector
GOMAXPROCS=2 nice -n 19 go test -race -run "TestMemoryService_StopCleanupRoutine|TestMemoryService_ConcurrentAccess" \
    ./internal/services/ -count=1 -p 1 -short > /dev/null 2>&1
check "Memory service passes race detector" $?

# --- Section 5: Goroutine leak detection ---

log_section "Section 5: Goroutine Leak Detection"

# 5.1: Run load test goroutine leak detector
GOMAXPROCS=2 nice -n 19 go test -run "TestLoad_GoroutineLeakDetection" \
    ./tests/load/ -count=1 -p 1 > /dev/null 2>&1
check "Load test goroutine leak detection passes" $?

# --- Section 6: CONST-029 Pattern-A drained blockers ---

log_section "Section 6: CONST-029 Structural Safety (Drained Blockers)"

# 6.1: EnhancedBM25Index uses atomic.Pointer[bm25State]
grep -q "atomic.Pointer\[bm25State\]" "$PROJECT_ROOT/internal/rag/qdrant_enhanced.go"
check "EnhancedBM25Index uses atomic.Pointer[bm25State] (Pattern Gamma+Epsilon)" $?

# 6.2: EnhancedBM25Index race-free verification test present
grep -q "TestEnhancedBM25Index_RaceFree" "$PROJECT_ROOT/internal/rag/qdrant_enhanced_test.go"
check "EnhancedBM25Index has TestEnhancedBM25Index_RaceFree verification test" $?

# 6.3: WorkflowState mutex retired; Snapshot API present
grep -q "func (s \*WorkflowState) Snapshot() \*WorkflowState" "$PROJECT_ROOT/internal/agentic/workflow.go"
check "WorkflowState exposes Snapshot() for cross-goroutine reads" $?

# 6.4: WorkflowState snapshot verification test present
grep -q "TestWorkflowState_Snapshot" "$PROJECT_ROOT/internal/agentic/workflow_test.go"
check "WorkflowState has TestWorkflowState_Snapshot verification test" $?

# 6.5: RepoMap mutex retired; Snapshot API present
grep -q "func (r \*RepoMap) Snapshot() \*RepoMap" "$PROJECT_ROOT/internal/tools/repomap/repomap.go"
check "RepoMap exposes Snapshot() for cross-goroutine reads" $?

# 6.6: CacheService userKeys is safe.Store
grep -q "userKeys \*safe.Store" "$PROJECT_ROOT/internal/cache/cache_service.go"
check "CacheService.userKeys migrated to safe.Store" $?

# 6.7: CacheService has COW-based race-free verification test
grep -q "TestCacheService_UserKeys_RaceFree" "$PROJECT_ROOT/internal/cache/cache_service_unit_test.go"
check "CacheService has TestCacheService_UserKeys_RaceFree verification test" $?

# 6.8: Broker (inmemory) migrated to safe.Store + atomic.Bool
grep -q "queues      \*safe.Store" "$PROJECT_ROOT/internal/messaging/inmemory/broker.go"
check "Broker (inmemory).queues migrated to safe.Store" $?

grep -q "connected   atomic.Bool" "$PROJECT_ROOT/internal/messaging/inmemory/broker.go"
check "Broker (inmemory).connected migrated to atomic.Bool" $?

# 6.9: Broker race-free verification test present
grep -q "TestBroker_RaceFree" "$PROJECT_ROOT/internal/messaging/inmemory/broker_test.go"
check "Broker (inmemory) has TestBroker_RaceFree verification test" $?

# 6.10: ConcurrencyMonitor migrated to safe.* + atomic.Bool
grep -q "listeners            \*safe.Slice" "$PROJECT_ROOT/internal/services/concurrency_monitor.go"
check "ConcurrencyMonitor.listeners migrated to safe.Slice" $?

grep -q "running              atomic.Bool" "$PROJECT_ROOT/internal/services/concurrency_monitor.go"
check "ConcurrencyMonitor.running migrated to atomic.Bool" $?

# 6.11: DebateService intent caches migrated to safe.Store
grep -q "intentCache         \*safe.Store" "$PROJECT_ROOT/internal/services/debate_service.go"
check "DebateService.intentCache migrated to safe.Store" $?

grep -q "func (ds \*DebateService) initCaches()" "$PROJECT_ROOT/internal/services/debate_service.go"
check "DebateService has initCaches() helper for struct-literal fixtures" $?

# 6.12: BootManager Results migrated to safe.Store
grep -q "Results        \*safe.Store" "$PROJECT_ROOT/internal/services/boot_manager.go"
check "BootManager.Results migrated to safe.Store" $?

# 6.13: Run the per-site race-free verification tests
GOMAXPROCS=2 nice -n 19 go test -race -run "TestEnhancedBM25Index_RaceFree" \
    ./internal/rag/ -count=1 -p 1 > /dev/null 2>&1
check "EnhancedBM25Index race-free test passes under -race" $?

GOMAXPROCS=2 nice -n 19 go test -race -run "TestWorkflowState_Snapshot" \
    ./internal/agentic/ -count=1 -p 1 > /dev/null 2>&1
check "WorkflowState snapshot test passes under -race" $?

GOMAXPROCS=2 nice -n 19 go test -race -run "TestRepoMap_Snapshot" \
    ./internal/tools/repomap/ -count=1 -p 1 > /dev/null 2>&1
check "RepoMap snapshot test passes under -race" $?

GOMAXPROCS=2 nice -n 19 go test -race -run "TestBroker_RaceFree" \
    ./internal/messaging/inmemory/ -count=1 -p 1 > /dev/null 2>&1
check "Broker (inmemory) race-free test passes under -race" $?

# 6.14: The concurrency-audit gate itself is green (no new Pattern-A regressions)
bash "$PROJECT_ROOT/scripts/concurrency-audit.sh" > /dev/null 2>&1
check "Concurrency audit gate green (no new Pattern-A regressions)" $?

# --- Summary ---

log_section "Challenge Summary"

echo ""
echo -e "${CYAN}Memory Safety Challenge Results${NC}"
echo -e "  Total:  $TOTAL_COUNT"
echo -e "  ${GREEN}Passed: $PASS_COUNT${NC}"
echo -e "  ${RED}Failed: $FAIL_COUNT${NC}"
echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    log_fail "Memory Safety Challenge FAILED ($FAIL_COUNT failures)"
    finalize_challenge "FAILED"
    exit 1
else
    log_pass "Memory Safety Challenge PASSED ($PASS_COUNT/$TOTAL_COUNT)"
    finalize_challenge "PASSED"
    exit 0
fi
