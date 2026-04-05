#!/bin/bash
# HelixAgent Challenge - Stress Resilience
# Validates that stress test infrastructure exists and meets quality standards:
# directory, file count, build tags, compilation, coverage of key components,
# resource limits, timeout detection, and goroutine leak checking.
#
# Tests: 20

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

record_result() {
    local name="$1" status="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$status" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "${GREEN}[PASS]${NC} $name"
    else
        FAILED=$((FAILED + 1))
        echo -e "${RED}[FAIL]${NC} $name"
    fi
}

echo "=========================================="
echo "  Stress Resilience Challenge"
echo "=========================================="
echo ""

STRESS_DIR="$PROJECT_ROOT/tests/stress"

# --------------------------------------------------------------------------
# Test 1: tests/stress/ directory exists
# --------------------------------------------------------------------------
if [ -d "$STRESS_DIR" ]; then
    record_result "tests/stress/ directory exists" "PASS"
else
    record_result "tests/stress/ directory exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 2: >= 15 stress test files
# --------------------------------------------------------------------------
STRESS_FILE_COUNT=$(find "$STRESS_DIR" -maxdepth 1 -name "*_test.go" 2>/dev/null | wc -l)
if [ "$STRESS_FILE_COUNT" -ge 15 ]; then
    record_result "stress test files >= 15 (found: $STRESS_FILE_COUNT)" "PASS"
else
    record_result "stress test files >= 15 (found: $STRESS_FILE_COUNT)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 3: >= 10 files use //go:build stress tag
# --------------------------------------------------------------------------
TAGGED_COUNT=$(grep -l "//go:build stress" "$STRESS_DIR"/*.go 2>/dev/null | wc -l)
if [ "$TAGGED_COUNT" -ge 10 ]; then
    record_result "stress files with //go:build stress tag >= 10 (found: $TAGGED_COUNT)" "PASS"
else
    record_result "stress files with //go:build stress tag >= 10 (found: $TAGGED_COUNT)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 4: Stress tests compile with stress build tag
# --------------------------------------------------------------------------
if GOMAXPROCS=2 nice -n 19 go test -tags stress -run=^$ -count=0 \
    -p 1 ./tests/stress/... 2>/dev/null; then
    record_result "stress tests compile with -tags stress" "PASS"
else
    record_result "stress tests compile with -tags stress" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 5: pool stress test file exists
# --------------------------------------------------------------------------
if [ -f "$STRESS_DIR/pool_stress_test.go" ] || \
   [ -f "$STRESS_DIR/pool_saturation_stress_test.go" ]; then
    record_result "pool stress test file exists" "PASS"
else
    record_result "pool stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 6: worker_pool stress test file exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/worker_pool*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "worker_pool stress test file exists" "PASS"
else
    record_result "worker_pool stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 7: event_bus stress test file exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/event_bus*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "event_bus stress test file exists" "PASS"
else
    record_result "event_bus stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 8: ensemble stress test file exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/*ensemble*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "ensemble stress test file exists" "PASS"
else
    record_result "ensemble stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 9: HTTP/API stress test file exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/api_stress* 2>/dev/null | head -1 | grep -q ".go" || \
   ls "$STRESS_DIR"/http_*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "HTTP/API stress test file exists" "PASS"
else
    record_result "HTTP/API stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 10: Tests use GOMAXPROCS(2) or runtime.GOMAXPROCS (resource limits)
# --------------------------------------------------------------------------
GOMAXPROCS_COUNT=$(grep -l "GOMAXPROCS(2)\|GOMAXPROCS(1)\|runtime\.GOMAXPROCS" \
    "$STRESS_DIR"/*.go 2>/dev/null | wc -l)
if [ "$GOMAXPROCS_COUNT" -ge 5 ]; then
    record_result "stress files enforcing GOMAXPROCS limit >= 5 (found: $GOMAXPROCS_COUNT)" "PASS"
else
    record_result "stress files enforcing GOMAXPROCS limit >= 5 (found: $GOMAXPROCS_COUNT)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 11: Tests use context.WithTimeout or context.WithDeadline
# --------------------------------------------------------------------------
TIMEOUT_COUNT=$(grep -l "context\.WithTimeout\|context\.WithDeadline\|t\.Deadline" \
    "$STRESS_DIR"/*.go 2>/dev/null | wc -l)
if [ "$TIMEOUT_COUNT" -ge 10 ]; then
    record_result "stress files with timeout/deadline >= 10 (found: $TIMEOUT_COUNT)" "PASS"
else
    record_result "stress files with timeout/deadline >= 10 (found: $TIMEOUT_COUNT)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 12: Tests check goroutine leaks (runtime.NumGoroutine)
# --------------------------------------------------------------------------
GOROUTINE_COUNT=$(grep -l "runtime\.NumGoroutine\|goroutineCount\|NumGoroutine\|goroutine leak" \
    "$STRESS_DIR"/*.go 2>/dev/null | wc -l)
if [ "$GOROUTINE_COUNT" -ge 3 ]; then
    record_result "stress files checking goroutine leaks >= 3 (found: $GOROUTINE_COUNT)" "PASS"
else
    record_result "stress files checking goroutine leaks >= 3 (found: $GOROUTINE_COUNT)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 13: Deadlock detection stress test exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/*deadlock*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "deadlock detection stress test file exists" "PASS"
else
    record_result "deadlock detection stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 14: Race condition stress test file exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/*race*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "race condition stress test file exists" "PASS"
else
    record_result "race condition stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 15: Circuit breaker stress test exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/*circuit_breaker*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "circuit breaker stress test file exists" "PASS"
else
    record_result "circuit breaker stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 16: Cache stress test exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/*cache*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "cache stress test file exists" "PASS"
else
    record_result "cache stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 17: Provider stress test exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/*provider*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "provider stress test file exists" "PASS"
else
    record_result "provider stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 18: Streaming stress test exists
# --------------------------------------------------------------------------
if ls "$STRESS_DIR"/*stream*stress* 2>/dev/null | head -1 | grep -q ".go"; then
    record_result "streaming stress test file exists" "PASS"
else
    record_result "streaming stress test file exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 19: Stress tests enforce concurrency via runtime.GOMAXPROCS or t.Parallel
# (stress tests use GOMAXPROCS directly rather than t.Parallel)
# --------------------------------------------------------------------------
CONCURRENCY_ENFORCE=$(grep -l "runtime\.GOMAXPROCS\|t\.Parallel()" \
    "$STRESS_DIR"/*.go 2>/dev/null | wc -l)
if [ "$CONCURRENCY_ENFORCE" -ge 5 ]; then
    record_result "stress files with concurrency enforcement >= 5 (found: $CONCURRENCY_ENFORCE)" "PASS"
else
    record_result "stress files with concurrency enforcement >= 5 (found: $CONCURRENCY_ENFORCE)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 20: Concurrency safety test covers multiple scenarios
# --------------------------------------------------------------------------
CONCURRENCY_FILE="$STRESS_DIR/concurrency_safety_test.go"
if [ -f "$CONCURRENCY_FILE" ] && [ "$(wc -l < "$CONCURRENCY_FILE")" -ge 50 ]; then
    record_result "concurrency_safety_test.go exists and is comprehensive (>= 50 lines)" "PASS"
else
    record_result "concurrency_safety_test.go exists and is comprehensive (>= 50 lines)" "FAIL"
fi

echo ""
echo "=========================================="
echo "  Results: $PASSED/$TOTAL passed, $FAILED failed"
echo "=========================================="

[ $FAILED -eq 0 ] && exit 0 || exit 1
