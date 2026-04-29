#!/bin/bash
# HelixAgent Challenge - Memory Safety & Race Condition Validation
# Validates absence of data races, goroutine leaks, and memory safety issues
# across core packages using the Go race detector.
#
# Requirements: Go installed. Does NOT require running infrastructure.

set -e

# Source challenge framework
source "$(dirname "${BASH_SOURCE[0]}")/challenge_framework.sh"

# Initialize challenge
init_challenge "memory-race" "Memory Safety & Race Condition Validation"
load_env

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Resource limits per CLAUDE.md: tests must use ≤30-40% of host resources
export GOMAXPROCS=2

# Test function
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    if eval "nice -n 19 $test_cmd" >> "$LOG_FILE" 2>&1; then
        log_success "PASS: $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        record_assertion "test" "$test_name" "true" ""
        return 0
    else
        log_error "FAIL: $test_name"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        record_assertion "test" "$test_name" "false" "Test command failed"
        return 1
    fi
}

# ============================================================================
# SECTION 1: RACE DETECTOR — LLM PACKAGE
# ============================================================================
log_info "=============================================="
log_info "SECTION 1: Race Detector — LLM Package"
log_info "=============================================="

cd "$PROJECT_ROOT"

run_test "llm package: circuit breaker races" \
    "go test -race -short -count=1 -timeout=60s ./internal/llm/ -run TestCircuitBreaker"

run_test "llm package: ensemble races" \
    "go test -race -short -count=1 -timeout=60s ./internal/llm/ -run TestRunEnsemble"

run_test "llm package: concurrent access" \
    "go test -race -short -count=1 -timeout=60s ./internal/llm/ -run TestCircuitBreaker_ConcurrentAccess"

run_test "llm package: listener notify timeout races" \
    "go test -race -short -count=1 -timeout=60s ./internal/llm/ -run TestCircuitBreaker_ListenerNotifyTimeout"

# ============================================================================
# SECTION 2: RACE DETECTOR — SERVICES PACKAGE
# ============================================================================
log_info "=============================================="
log_info "SECTION 2: Race Detector — Services Package"
log_info "=============================================="

run_test "services: boot manager races" \
    "go test -race -short -count=1 -timeout=30s ./internal/services/ -run TestNewBootManager"

run_test "services: cache factory ping timeout (no hang)" \
    "go test -race -short -count=1 -timeout=15s ./internal/services/ -run TestCacheFactory_CreateDefaultCache"

run_test "services: ACP discovery races" \
    "go test -race -short -count=1 -timeout=30s ./internal/services/ -run TestACPDiscovery"

# ============================================================================
# SECTION 3: RACE DETECTOR — BACKGROUND WORKERS
# ============================================================================
log_info "=============================================="
log_info "SECTION 3: Race Detector — Background Workers"
log_info "=============================================="

run_test "background: worker pool races" \
    "go test -race -short -count=1 -timeout=60s ./internal/background/ -run TestWorkerPool"

run_test "background: task queue races" \
    "go test -race -short -count=1 -timeout=60s ./internal/background/ -run TestInMemoryTaskQueue"

run_test "background: resource monitor races" \
    "go test -race -short -count=1 -timeout=60s ./internal/background/ -run TestResourceMonitor"

# ============================================================================
# SECTION 4: RACE DETECTOR — CACHE PACKAGE
# ============================================================================
log_info "=============================================="
log_info "SECTION 4: Race Detector — Cache Package"
log_info "=============================================="

run_test "cache package: no data races" \
    "go test -race -short -count=1 -timeout=60s ./internal/cache/"

# ============================================================================
# SECTION 5: RACE DETECTOR — CONCURRENCY PACKAGE
# ============================================================================
log_info "=============================================="
log_info "SECTION 5: Race Detector — Concurrency Package"
log_info "=============================================="

run_test "concurrency package: no data races" \
    "go test -race -short -count=1 -timeout=60s ./internal/concurrency/"

# ============================================================================
# SECTION 6: RACE DETECTOR — DEBATE & HANDLERS
# ============================================================================
log_info "=============================================="
log_info "SECTION 6: Race Detector — Debate & Handlers"
log_info "=============================================="

run_test "debate agents: no data races" \
    "go test -race -short -count=1 -timeout=60s ./DebateOrchestrator/agents/ ./DebateOrchestrator/orchestrator/"

run_test "handlers: no data races" \
    "go test -race -short -count=1 -timeout=60s ./internal/handlers/"

# ============================================================================
# SECTION 7: COMPILE-TIME CHECKS
# ============================================================================
log_info "=============================================="
log_info "SECTION 7: Compile & Vet Checks"
log_info "=============================================="

run_test "go build succeeds (no compilation errors)" \
    "go build ./internal/... ./cmd/..."

run_test "go vet passes on llm package" \
    "go vet ./internal/llm/..."

run_test "go vet passes on services package" \
    "go vet ./internal/services/..."

run_test "go vet passes on background package" \
    "go vet ./internal/background/..."

# ============================================================================
# SECTION 8: GOROUTINE LEAK DETECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 8: Goroutine Leak Detection"
log_info "=============================================="

run_test "goleak: llm package TestMain present" \
    "grep -q 'func TestMain' ${PROJECT_ROOT}/internal/llm/main_test.go"

run_test "goleak: background package TestMain present" \
    "grep -q 'func TestMain' ${PROJECT_ROOT}/internal/background/main_test.go"

run_test "goleak: router package TestMain uses goleak" \
    "grep -q 'goleak.Find' ${PROJECT_ROOT}/internal/router/setup_router_comprehensive_test.go"

run_test "goleak: goleak dependency in go.mod" \
    "grep -q 'go.uber.org/goleak' ${PROJECT_ROOT}/go.mod"

# ============================================================================
# SECTION 9: AUTH PACKAGE — NO HANGING CLI TESTS
# ============================================================================
log_info "=============================================="
log_info "SECTION 9: Auth Package — CLI Tests Skip in Short Mode"
log_info "=============================================="

run_test "auth/oauth_credentials: short mode skips hanging CLI test" \
    "go test -race -short -count=1 -timeout=30s ./internal/auth/oauth_credentials/"

run_test "auth/oauth_credentials: TestRefreshQwenTokenWithFallback has Short() skip" \
    "grep -q 'testing.Short' ${PROJECT_ROOT}/internal/auth/oauth_credentials/cli_refresh_test.go"

# ============================================================================
# SECTION 10: RACE DETECTOR — MODELS & CONFIG
# ============================================================================
log_info "=============================================="
log_info "SECTION 10: Race Detector — Models & Config"
log_info "=============================================="

run_test "models package: no data races" \
    "go test -race -short -count=1 -timeout=30s ./internal/models/"

run_test "config package: no data races" \
    "go test -race -short -count=1 -timeout=30s ./internal/config/"

# ============================================================================
# FINAL REPORT
# ============================================================================
log_info "=============================================="
log_info "CHALLENGE RESULTS"
log_info "=============================================="
log_info "Total tests: $TESTS_TOTAL"
log_info "Passed:      $TESTS_PASSED"
log_info "Failed:      $TESTS_FAILED"

if [ "$TESTS_FAILED" -eq 0 ]; then
    log_success "ALL $TESTS_TOTAL MEMORY SAFETY TESTS PASSED"
    finalize_challenge "success" "$TESTS_PASSED/$TESTS_TOTAL tests passed"
    exit 0
else
    log_error "$TESTS_FAILED/$TESTS_TOTAL tests FAILED"
    finalize_challenge "failure" "$TESTS_PASSED/$TESTS_TOTAL tests passed, $TESTS_FAILED failed"
    exit 1
fi
