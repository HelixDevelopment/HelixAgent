#!/usr/bin/env bash
set -euo pipefail

# Safety Regression Challenge
# Validates race-safety, sync.Once usage in pool, WaitGroup in cleanupExpired,
# atomic.Bool usage in managers, and nil guard in terminateInstance.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "safety_regression" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

record_result() {
    TOTAL=$((TOTAL + 1))
    test_start "$1"
    if [ "$2" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        test_pass
    else
        FAILED=$((FAILED + 1))
        test_fail "$1"
    fi
}

print_header "Safety Regression Challenge"
echo ""

# Test 1: Race detector passes on internal/clis/ (short tests)
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go test -short -race -count=1 -p 1 -timeout 60s ./internal/clis/... 2>/dev/null; then
    record_result "Race detector passes on internal/clis/" "PASS"
else
    record_result "Race detector passes on internal/clis/" "FAIL"
fi

# Test 2: Race detector passes on internal/ensemble/background/
if GOMAXPROCS=2 nice -n 19 go test -short -race -count=1 -p 1 -timeout 60s ./internal/ensemble/background/... 2>/dev/null; then
    record_result "Race detector passes on internal/ensemble/background/" "PASS"
else
    record_result "Race detector passes on internal/ensemble/background/" "FAIL"
fi

# Test 3: Race detector passes on internal/http/
if GOMAXPROCS=2 nice -n 19 go test -short -race -count=1 -p 1 -timeout 60s ./internal/http/... 2>/dev/null; then
    record_result "Race detector passes on internal/http/" "PASS"
else
    record_result "Race detector passes on internal/http/" "FAIL"
fi

# Test 4: sync/atomic primitives used in pool.go
POOL_FILE="$PROJECT_ROOT/internal/clis/pool.go"
if grep -q "sync\.\|atomic\." "$POOL_FILE" 2>/dev/null; then
    record_result "sync/atomic primitives used in clis/pool.go" "PASS"
else
    record_result "sync/atomic primitives used in clis/pool.go" "FAIL"
fi

# Test 5: placeholderSeq atomic counter exists in pool.go
if grep -q "placeholderSeq" "$POOL_FILE" 2>/dev/null; then
    record_result "placeholderSeq atomic counter present in pool.go" "PASS"
else
    record_result "placeholderSeq atomic counter present in pool.go" "FAIL"
fi

# Test 6: WaitGroup used in internal/clis
WG_COUNT=$(grep -r "sync.WaitGroup\|var wg sync" "$PROJECT_ROOT/internal/clis/" --include="*.go" 2>/dev/null | wc -l)
if [ "$WG_COUNT" -gt 0 ]; then
    record_result "sync.WaitGroup used in internal/clis ($WG_COUNT occurrences)" "PASS"
else
    record_result "sync.WaitGroup used in internal/clis ($WG_COUNT occurrences)" "FAIL"
fi

# Test 7: cleanupExpired function exists in session_manager
SESSION_MGR="$PROJECT_ROOT/internal/clis/agents/claude_code/session_manager.go"
if grep -q "func.*cleanupExpired\|cleanupExpired()" "$SESSION_MGR" 2>/dev/null; then
    record_result "cleanupExpired function exists in session_manager.go" "PASS"
else
    record_result "cleanupExpired function exists in session_manager.go" "FAIL"
fi

# Test 8: cleanupExpired uses concurrent-safe access
# CONST-029 migration: code formerly used bare mutex (Lock / mu.) was
# migrated to safe.Store APIs (sessions.Keys, sessions.Update, etc.) —
# both patterns are concurrent-safe. Accept EITHER form so the test
# tracks the actual safety property, not the obsolete pattern.
if grep -A 12 "func.*cleanupExpired" "$SESSION_MGR" 2>/dev/null | grep -qE "Lock|mu\.|sessions\.Update|sessions\.Range|sessions\.Keys"; then
    record_result "cleanupExpired uses concurrent-safe access" "PASS"
else
    record_result "cleanupExpired uses concurrent-safe access" "FAIL"
fi

# Test 9: atomic.Bool used in notifications (sse_manager)
SSE_MGR="$PROJECT_ROOT/internal/notifications/sse_manager.go"
if grep -q "atomic.Bool" "$SSE_MGR" 2>/dev/null; then
    record_result "atomic.Bool used in notifications/sse_manager.go" "PASS"
else
    record_result "atomic.Bool used in notifications/sse_manager.go" "FAIL"
fi

# Test 10: atomic.Bool used in mcp/connection_pool
MCP_POOL="$PROJECT_ROOT/internal/mcp/connection_pool.go"
if grep -q "atomic.Bool" "$MCP_POOL" 2>/dev/null; then
    record_result "atomic.Bool used in mcp/connection_pool.go" "PASS"
else
    record_result "atomic.Bool used in mcp/connection_pool.go" "FAIL"
fi

# Test 11: terminateInstance has nil guard in pool.go
if grep -A 5 "func.*terminateInstance" "$POOL_FILE" 2>/dev/null | grep -q "inst == nil\|== nil"; then
    record_result "terminateInstance has nil guard in pool.go" "PASS"
else
    record_result "terminateInstance has nil guard in pool.go" "FAIL"
fi

# Test 12: sync.Once used in http/pool.go
HTTP_POOL="$PROJECT_ROOT/internal/http/pool.go"
if grep -q "sync.Once\|globalPoolOnce" "$HTTP_POOL" 2>/dev/null; then
    record_result "sync.Once used in internal/http/pool.go" "PASS"
else
    record_result "sync.Once used in internal/http/pool.go" "FAIL"
fi

# Test 13: WaitGroup used in ensemble/background/worker_pool.go
WP_FILE="$PROJECT_ROOT/internal/ensemble/background/worker_pool.go"
if grep -q "sync.WaitGroup\|wg sync\|wp.wg" "$WP_FILE" 2>/dev/null; then
    record_result "sync.WaitGroup used in ensemble/background/worker_pool.go" "PASS"
else
    record_result "sync.WaitGroup used in ensemble/background/worker_pool.go" "FAIL"
fi

# Test 14: Race detector passes on internal/notifications/
if GOMAXPROCS=2 nice -n 19 go test -short -race -count=1 -p 1 -timeout 60s ./internal/notifications/... 2>/dev/null; then
    record_result "Race detector passes on internal/notifications/" "PASS"
else
    record_result "Race detector passes on internal/notifications/" "FAIL"
fi

# Test 15: sync.Once lazy init used in MCP adapters registry
MCP_REG="$PROJECT_ROOT/internal/mcp/adapters/registry.go"
if grep -q "sync.Once" "$MCP_REG" 2>/dev/null; then
    record_result "sync.Once lazy init used in mcp/adapters/registry.go" "PASS"
else
    record_result "sync.Once lazy init used in mcp/adapters/registry.go" "FAIL"
fi

echo ""
print_summary "Safety Regression Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
