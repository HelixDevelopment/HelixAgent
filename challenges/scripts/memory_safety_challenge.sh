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
