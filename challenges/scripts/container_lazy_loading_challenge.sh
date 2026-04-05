#!/usr/bin/env bash
set -euo pipefail

# Container Lazy Loading Challenge
# Validates that the internal/containers lazy loading integration is complete:
# source files, test files, key structs and functions.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "container_lazy_loading" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONTAINERS_PKG="$PROJECT_ROOT/internal/containers"

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

print_header "Container Lazy Loading Challenge"
echo ""

# Test 1: lazy_integration.go exists
if [ -f "$CONTAINERS_PKG/lazy_integration.go" ]; then
    record_result "lazy_integration.go exists" "PASS"
else
    record_result "lazy_integration.go exists" "FAIL"
fi

# Test 2: lazy_integration_test.go exists
if [ -f "$CONTAINERS_PKG/lazy_integration_test.go" ]; then
    record_result "lazy_integration_test.go exists" "PASS"
else
    record_result "lazy_integration_test.go exists" "FAIL"
fi

# Test 3: logger_adapter.go exists
if [ -f "$CONTAINERS_PKG/logger_adapter.go" ]; then
    record_result "logger_adapter.go exists" "PASS"
else
    record_result "logger_adapter.go exists" "FAIL"
fi

# Test 4: logger_adapter_test.go exists
if [ -f "$CONTAINERS_PKG/logger_adapter_test.go" ]; then
    record_result "logger_adapter_test.go exists" "PASS"
else
    record_result "logger_adapter_test.go exists" "FAIL"
fi

# Test 5: Package compiles
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/containers/... 2>/dev/null; then
    record_result "containers package compiles" "PASS"
else
    record_result "containers package compiles" "FAIL"
fi

# Test 6: Tests compile and pass
if GOMAXPROCS=2 nice -n 19 go test -short -count=1 -p 1 ./internal/containers/... 2>/dev/null; then
    record_result "containers package tests pass" "PASS"
else
    record_result "containers package tests pass" "FAIL"
fi

# Test 7: LazyOrchestrator struct exists
if grep -q "type LazyOrchestrator struct" "$CONTAINERS_PKG/lazy_integration.go" 2>/dev/null; then
    record_result "LazyOrchestrator struct defined" "PASS"
else
    record_result "LazyOrchestrator struct defined" "FAIL"
fi

# Test 8: ServiceDefinition struct exists
if grep -q "type ServiceDefinition struct" "$CONTAINERS_PKG/lazy_integration.go" 2>/dev/null; then
    record_result "ServiceDefinition struct defined" "PASS"
else
    record_result "ServiceDefinition struct defined" "FAIL"
fi

# Test 9: RegisterService function exists
if grep -q "func.*RegisterService" "$CONTAINERS_PKG/lazy_integration.go" 2>/dev/null; then
    record_result "RegisterService function exists" "PASS"
else
    record_result "RegisterService function exists" "FAIL"
fi

# Test 10: StartService function exists
if grep -q "func.*StartService" "$CONTAINERS_PKG/lazy_integration.go" 2>/dev/null; then
    record_result "StartService function exists" "PASS"
else
    record_result "StartService function exists" "FAIL"
fi

# Test 11: NewLazyOrchestrator constructor exists
if grep -q "func NewLazyOrchestrator" "$CONTAINERS_PKG/lazy_integration.go" 2>/dev/null; then
    record_result "NewLazyOrchestrator constructor exists" "PASS"
else
    record_result "NewLazyOrchestrator constructor exists" "FAIL"
fi

# Test 12: Test file has at least 5 test functions
TEST_COUNT=$(grep -c "^func Test" "$CONTAINERS_PKG/lazy_integration_test.go" 2>/dev/null || echo "0")
if [ "$TEST_COUNT" -ge 5 ]; then
    record_result "lazy_integration_test.go has >= 5 test functions (actual: $TEST_COUNT)" "PASS"
else
    record_result "lazy_integration_test.go has >= 5 test functions (actual: $TEST_COUNT)" "FAIL"
fi

echo ""
print_summary "Container Lazy Loading Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
