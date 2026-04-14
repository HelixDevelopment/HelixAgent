#!/bin/bash
# HelixAgent Challenge - Performance Baseline Framework
# Validates that the performance baseline infrastructure is in place:
# benchmark script, baseline directory, Makefile targets, regression test,
# build-tag compilation, resource limits, and benchmark coverage.
#
# Tests: 15

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
echo "  Performance Baseline Challenge"
echo "=========================================="
echo ""

# --------------------------------------------------------------------------
# Test 1: benchmark-baseline.sh exists and is executable
# --------------------------------------------------------------------------
BASELINE_SCRIPT="$PROJECT_ROOT/scripts/benchmark-baseline.sh"
if [ -f "$BASELINE_SCRIPT" ] && [ -x "$BASELINE_SCRIPT" ]; then
    record_result "scripts/benchmark-baseline.sh exists and is executable" "PASS"
else
    record_result "scripts/benchmark-baseline.sh exists and is executable" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 2: benchmarks/baselines/ directory exists
# --------------------------------------------------------------------------
if [ -d "$PROJECT_ROOT/benchmarks/baselines" ]; then
    record_result "benchmarks/baselines/ directory exists" "PASS"
else
    record_result "benchmarks/baselines/ directory exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 3: Makefile has benchmark-baseline target
# --------------------------------------------------------------------------
if grep -q "^benchmark-baseline:" "$PROJECT_ROOT/Makefile" 2>/dev/null; then
    record_result "Makefile has benchmark-baseline target" "PASS"
else
    record_result "Makefile has benchmark-baseline target" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 4: Makefile has benchmark-check target
# --------------------------------------------------------------------------
if grep -q "^benchmark-check:" "$PROJECT_ROOT/Makefile" 2>/dev/null; then
    record_result "Makefile has benchmark-check target" "PASS"
else
    record_result "Makefile has benchmark-check target" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 5: baseline_regression_test.go exists
# --------------------------------------------------------------------------
REGRESSION_TEST=$(find "$PROJECT_ROOT/tests" -name "baseline_regression_test.go" 2>/dev/null | head -1)
if [ -n "$REGRESSION_TEST" ]; then
    record_result "baseline_regression_test.go exists" "PASS"
else
    record_result "baseline_regression_test.go exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 6: baseline_regression_test.go compiles with performance tag
# --------------------------------------------------------------------------
if [ -n "$REGRESSION_TEST" ]; then
    # Test-only package: use go test -run=^$ -count=0 to verify compilation
    if GOMAXPROCS=2 nice -n 19 go test -mod=mod -tags performance -run=^$ -count=0 \
           -p 1 ./tests/performance/... 2>/dev/null; then
        record_result "baseline_regression_test.go compiles with performance tag" "PASS"
    else
        record_result "baseline_regression_test.go compiles with performance tag" "FAIL"
    fi
else
    record_result "baseline_regression_test.go compiles with performance tag (file missing)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 7: Script contains GOMAXPROCS=2 resource limits
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "GOMAXPROCS=2" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh enforces GOMAXPROCS=2" "PASS"
else
    record_result "benchmark-baseline.sh enforces GOMAXPROCS=2" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 8: Script enforces nice -n 19
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "nice -n 19" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh enforces nice -n 19" "PASS"
else
    record_result "benchmark-baseline.sh enforces nice -n 19" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 9: Script captures handler benchmarks
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "handlers" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh captures handler benchmarks" "PASS"
else
    record_result "benchmark-baseline.sh captures handler benchmarks" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 10: Script captures pool benchmarks
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "pool\|clis" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh captures pool benchmarks" "PASS"
else
    record_result "benchmark-baseline.sh captures pool benchmarks" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 11: Script captures HTTP benchmarks
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "internal/http\b" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh captures HTTP benchmarks" "PASS"
else
    record_result "benchmark-baseline.sh captures HTTP benchmarks" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 12: Script captures ensemble benchmarks
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "ensemble" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh captures ensemble benchmarks" "PASS"
else
    record_result "benchmark-baseline.sh captures ensemble benchmarks" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 13: Script writes CAPTURED_AT.txt timestamp
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "CAPTURED_AT" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh writes CAPTURED_AT.txt timestamp" "PASS"
else
    record_result "benchmark-baseline.sh writes CAPTURED_AT.txt timestamp" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 14: Script uses -p 1 parallelism flag for resource safety
# --------------------------------------------------------------------------
if [ -f "$BASELINE_SCRIPT" ] && grep -q "\-p 1" "$BASELINE_SCRIPT"; then
    record_result "benchmark-baseline.sh uses -p 1 parallelism limit" "PASS"
else
    record_result "benchmark-baseline.sh uses -p 1 parallelism limit" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 15: Performance test package builds cleanly with performance tag
# --------------------------------------------------------------------------
PERF_DIR="$PROJECT_ROOT/tests/performance"
if [ -d "$PERF_DIR" ]; then
    if GOMAXPROCS=2 nice -n 19 go build -tags performance "$PERF_DIR/..." 2>/dev/null; then
        record_result "tests/performance package builds with performance tag" "PASS"
    else
        record_result "tests/performance package builds with performance tag" "FAIL"
    fi
else
    record_result "tests/performance directory exists" "FAIL"
fi

echo ""
echo "=========================================="
echo "  Results: $PASSED/$TOTAL passed, $FAILED failed"
echo "=========================================="

[ $FAILED -eq 0 ] && exit 0 || exit 1
