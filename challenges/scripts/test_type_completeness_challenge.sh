#!/usr/bin/env bash
set -euo pipefail

# Test Type Completeness Challenge
# Validates that all required packages have test files, fuzz functions
# are sufficient in count, stress tests exist with build tags,
# and benchmark functions reach the target count.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "test_type_completeness" "$@"

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

print_header "Test Type Completeness Challenge"
echo ""

# Test 1: internal/output has a test file
OUTPUT_TEST_COUNT=$(find "$PROJECT_ROOT/internal/output" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$OUTPUT_TEST_COUNT" -gt 0 ]; then
    record_result "internal/output has test files ($OUTPUT_TEST_COUNT found)" "PASS"
else
    record_result "internal/output has test files (0 found)" "FAIL"
fi

# Test 2: internal/containers has test files
CONTAINERS_TEST_COUNT=$(find "$PROJECT_ROOT/internal/containers" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$CONTAINERS_TEST_COUNT" -gt 0 ]; then
    record_result "internal/containers has test files ($CONTAINERS_TEST_COUNT found)" "PASS"
else
    record_result "internal/containers has test files (0 found)" "FAIL"
fi

# Test 3: browser_handler_test.go exists
if [ -f "$PROJECT_ROOT/internal/handlers/browser_handler_test.go" ]; then
    record_result "browser_handler_test.go exists" "PASS"
else
    record_result "browser_handler_test.go exists" "FAIL"
fi

# Test 4: ensemble_handler_test.go exists
if [ -f "$PROJECT_ROOT/internal/handlers/ensemble_handler_test.go" ]; then
    record_result "ensemble_handler_test.go exists" "PASS"
else
    record_result "ensemble_handler_test.go exists" "FAIL"
fi

# Test 5: search_handler_test.go exists
if [ -f "$PROJECT_ROOT/internal/handlers/search_handler_test.go" ]; then
    record_result "search_handler_test.go exists" "PASS"
else
    record_result "search_handler_test.go exists" "FAIL"
fi

# Test 6: verifier_types test file exists (handler coverage)
VERIFIER_TEST_COUNT=$(find "$PROJECT_ROOT/internal/handlers" -name "verifier*_test.go" -type f 2>/dev/null | wc -l)
if [ "$VERIFIER_TEST_COUNT" -gt 0 ]; then
    record_result "verifier handler test files exist ($VERIFIER_TEST_COUNT found)" "PASS"
else
    record_result "verifier handler test files exist (0 found)" "FAIL"
fi

# Test 7: Fuzz functions count >= 40
FUZZ_FUNC_COUNT=$(grep -r "func Fuzz" "$PROJECT_ROOT/tests/fuzz/" 2>/dev/null | wc -l)
if [ "$FUZZ_FUNC_COUNT" -ge 40 ]; then
    record_result "Fuzz function count >= 40 (actual: $FUZZ_FUNC_COUNT)" "PASS"
else
    record_result "Fuzz function count >= 40 (actual: $FUZZ_FUNC_COUNT)" "FAIL"
fi

# Test 8: Stress test files exist
STRESS_COUNT=$(find "$PROJECT_ROOT/tests/stress" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$STRESS_COUNT" -gt 0 ]; then
    record_result "Stress test files exist ($STRESS_COUNT found)" "PASS"
else
    record_result "Stress test files exist (0 found)" "FAIL"
fi

# Test 9: Stress test files have build tag (//go:build stress)
STRESS_TAG_COUNT=$(grep -l "//go:build stress\|// +build stress" "$PROJECT_ROOT/tests/stress/"*.go 2>/dev/null | wc -l)
if [ "$STRESS_TAG_COUNT" -gt 0 ]; then
    record_result "Stress test files have build tag ($STRESS_TAG_COUNT files tagged)" "PASS"
else
    record_result "Stress test files have build tag (0 found)" "FAIL"
fi

# Test 10: Benchmark functions exist
BENCH_COUNT=$(grep -r "func Benchmark" "$PROJECT_ROOT" --include="*_test.go" 2>/dev/null | wc -l)
if [ "$BENCH_COUNT" -gt 0 ]; then
    record_result "Benchmark functions exist ($BENCH_COUNT found)" "PASS"
else
    record_result "Benchmark functions exist (0 found)" "FAIL"
fi

# Test 11: Benchmark function count >= 700
if [ "$BENCH_COUNT" -ge 700 ]; then
    record_result "Benchmark function count >= 700 (actual: $BENCH_COUNT)" "PASS"
else
    record_result "Benchmark function count >= 700 (actual: $BENCH_COUNT)" "FAIL"
fi

# Test 12: Integration tests directory has test files
INTEGRATION_COUNT=$(find "$PROJECT_ROOT/tests/integration" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$INTEGRATION_COUNT" -gt 0 ]; then
    record_result "Integration test files exist ($INTEGRATION_COUNT found)" "PASS"
else
    record_result "Integration test files exist (0 found)" "FAIL"
fi

# Test 13: E2E tests directory has test files
E2E_COUNT=$(find "$PROJECT_ROOT/tests/e2e" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$E2E_COUNT" -gt 0 ]; then
    record_result "E2E test files exist ($E2E_COUNT found)" "PASS"
else
    record_result "E2E test files exist (0 found)" "FAIL"
fi

# Test 14: Security tests directory has test files
SEC_COUNT=$(find "$PROJECT_ROOT/tests/security" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$SEC_COUNT" -gt 0 ]; then
    record_result "Security test files exist ($SEC_COUNT found)" "PASS"
else
    record_result "Security test files exist (0 found)" "FAIL"
fi

# Test 15: Fuzz tests directory has >= 15 files
FUZZ_FILE_COUNT=$(find "$PROJECT_ROOT/tests/fuzz" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$FUZZ_FILE_COUNT" -ge 15 ]; then
    record_result "Fuzz test files >= 15 (actual: $FUZZ_FILE_COUNT)" "PASS"
else
    record_result "Fuzz test files >= 15 (actual: $FUZZ_FILE_COUNT)" "FAIL"
fi

# Test 16: Stress tests count >= 20 files
if [ "$STRESS_COUNT" -ge 20 ]; then
    record_result "Stress test files >= 20 (actual: $STRESS_COUNT)" "PASS"
else
    record_result "Stress test files >= 20 (actual: $STRESS_COUNT)" "FAIL"
fi

# Test 17: All handlers have test files (check handler package test coverage)
HANDLER_GO_COUNT=$(find "$PROJECT_ROOT/internal/handlers" -maxdepth 1 -name "*.go" ! -name "*_test.go" -type f 2>/dev/null | wc -l)
HANDLER_TEST_COUNT=$(find "$PROJECT_ROOT/internal/handlers" -maxdepth 1 -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$HANDLER_TEST_COUNT" -gt 5 ]; then
    record_result "Handlers package has substantial test coverage ($HANDLER_TEST_COUNT test files)" "PASS"
else
    record_result "Handlers package has substantial test coverage ($HANDLER_TEST_COUNT test files, need >5)" "FAIL"
fi

# Test 18: internal/output pipeline_test.go has >= 10 tests
OUTPUT_TEST_FUNC=$(grep -c "^func Test" "$PROJECT_ROOT/internal/output/pipeline_test.go" 2>/dev/null || echo "0")
if [ "$OUTPUT_TEST_FUNC" -ge 10 ]; then
    record_result "internal/output pipeline_test.go has >= 10 test functions (actual: $OUTPUT_TEST_FUNC)" "PASS"
else
    record_result "internal/output pipeline_test.go has >= 10 test functions (actual: $OUTPUT_TEST_FUNC)" "FAIL"
fi

# Test 19: internal/containers lazy_integration_test.go has tests
LAZY_TEST_COUNT=$(grep -c "^func Test" "$PROJECT_ROOT/internal/containers/lazy_integration_test.go" 2>/dev/null || echo "0")
if [ "$LAZY_TEST_COUNT" -ge 3 ]; then
    record_result "internal/containers lazy_integration_test.go has >= 3 tests (actual: $LAZY_TEST_COUNT)" "PASS"
else
    record_result "internal/containers lazy_integration_test.go has >= 3 tests (actual: $LAZY_TEST_COUNT)" "FAIL"
fi

# Test 20: All packages compile cleanly
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/... 2>/dev/null; then
    record_result "All internal packages compile cleanly" "PASS"
else
    record_result "All internal packages compile cleanly" "FAIL"
fi

echo ""
print_summary "Test Type Completeness Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
