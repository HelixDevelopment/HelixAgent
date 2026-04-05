#!/usr/bin/env bash
set -euo pipefail

# Browser Handler Challenge
# Validates that the browser handler exists, is registered in the router,
# has test coverage, and exposes navigation/interaction methods.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "browser_handler" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HANDLERS_PKG="$PROJECT_ROOT/internal/handlers"
ROUTER_FILE="$PROJECT_ROOT/internal/router/router.go"

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

print_header "Browser Handler Challenge"
echo ""

# Test 1: browser_handler.go exists
if [ -f "$HANDLERS_PKG/browser_handler.go" ]; then
    record_result "browser_handler.go exists" "PASS"
else
    record_result "browser_handler.go exists" "FAIL"
fi

# Test 2: browser_handler registered in router
if grep -q "BrowserHandler\|browserHandler\|browser" "$ROUTER_FILE" 2>/dev/null; then
    record_result "BrowserHandler registered in router.go" "PASS"
else
    record_result "BrowserHandler registered in router.go" "FAIL"
fi

# Test 3: browser_handler_test.go exists
if [ -f "$HANDLERS_PKG/browser_handler_test.go" ]; then
    record_result "browser_handler_test.go exists" "PASS"
else
    record_result "browser_handler_test.go exists" "FAIL"
fi

# Test 4: Package compiles
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/handlers/... 2>/dev/null; then
    record_result "handlers package compiles" "PASS"
else
    record_result "handlers package compiles" "FAIL"
fi

# Test 5: Tests compile and pass
if GOMAXPROCS=2 nice -n 19 go test -short -count=1 -p 1 -run "Browser" ./internal/handlers/... 2>/dev/null; then
    record_result "Browser handler tests pass" "PASS"
else
    record_result "Browser handler tests pass" "FAIL"
fi

# Test 6: Navigate method exists
if grep -q "func.*Navigate\b" "$HANDLERS_PKG/browser_handler.go" 2>/dev/null; then
    record_result "Navigate handler method exists" "PASS"
else
    record_result "Navigate handler method exists" "FAIL"
fi

# Test 7: Click method exists
if grep -q "func.*Click\b" "$HANDLERS_PKG/browser_handler.go" 2>/dev/null; then
    record_result "Click handler method exists" "PASS"
else
    record_result "Click handler method exists" "FAIL"
fi

# Test 8: Type method exists
if grep -q "func.*\bType\b" "$HANDLERS_PKG/browser_handler.go" 2>/dev/null; then
    record_result "Type handler method exists" "PASS"
else
    record_result "Type handler method exists" "FAIL"
fi

# Test 9: Screenshot method exists
if grep -q "func.*Screenshot\b" "$HANDLERS_PKG/browser_handler.go" 2>/dev/null; then
    record_result "Screenshot handler method exists" "PASS"
else
    record_result "Screenshot handler method exists" "FAIL"
fi

# Test 10: Test file has at least 3 test functions
TEST_COUNT=$(grep -c "^func Test" "$HANDLERS_PKG/browser_handler_test.go" 2>/dev/null || echo "0")
if [ "$TEST_COUNT" -ge 3 ]; then
    record_result "browser_handler_test.go has >= 3 test functions (actual: $TEST_COUNT)" "PASS"
else
    record_result "browser_handler_test.go has >= 3 test functions (actual: $TEST_COUNT)" "FAIL"
fi

echo ""
print_summary "Browser Handler Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
