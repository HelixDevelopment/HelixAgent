#!/usr/bin/env bash
set -euo pipefail

# Search Handler Challenge
# Validates that the search handler exists, is registered in the router,
# has test coverage, and exposes semantic search and index trigger endpoints.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "search_handler" "$@"

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

print_header "Search Handler Challenge"
echo ""

# Test 1: search_handler.go exists
if [ -f "$HANDLERS_PKG/search_handler.go" ]; then
    record_result "search_handler.go exists" "PASS"
else
    record_result "search_handler.go exists" "FAIL"
fi

# Test 2: search_handler registered in router
if grep -q "SearchHandler\|searchHandler\|NewSearchHandler" "$ROUTER_FILE" 2>/dev/null; then
    record_result "SearchHandler registered in router.go" "PASS"
else
    record_result "SearchHandler registered in router.go" "FAIL"
fi

# Test 3: search_handler_test.go exists
if [ -f "$HANDLERS_PKG/search_handler_test.go" ]; then
    record_result "search_handler_test.go exists" "PASS"
else
    record_result "search_handler_test.go exists" "FAIL"
fi

# Test 4: Package compiles
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/handlers/... 2>/dev/null; then
    record_result "handlers package compiles" "PASS"
else
    record_result "handlers package compiles" "FAIL"
fi

# Test 5: Tests compile and pass
if GOMAXPROCS=2 nice -n 19 go test -short -count=1 -p 1 -run "Search" ./internal/handlers/... 2>/dev/null; then
    record_result "Search handler tests pass" "PASS"
else
    record_result "Search handler tests pass" "FAIL"
fi

# Test 6: SemanticSearch endpoint exists
if grep -q "func.*SemanticSearch\b" "$HANDLERS_PKG/search_handler.go" 2>/dev/null; then
    record_result "SemanticSearch handler method exists" "PASS"
else
    record_result "SemanticSearch handler method exists" "FAIL"
fi

# Test 7: TriggerIndex endpoint exists
if grep -q "func.*TriggerIndex\b\|func.*Index\b" "$HANDLERS_PKG/search_handler.go" 2>/dev/null; then
    record_result "TriggerIndex handler method exists" "PASS"
else
    record_result "TriggerIndex handler method exists" "FAIL"
fi

# Test 8: RegisterRoutes method exists
if grep -q "func.*RegisterRoutes" "$HANDLERS_PKG/search_handler.go" 2>/dev/null; then
    record_result "RegisterRoutes method exists on SearchHandler" "PASS"
else
    record_result "RegisterRoutes method exists on SearchHandler" "FAIL"
fi

# Test 9: Semantic search route registered (/v1/search/semantic or /semantic)
if grep -q '"/semantic\|/search' "$HANDLERS_PKG/search_handler.go" 2>/dev/null || \
   grep -q "semantic\|search" "$HANDLERS_PKG/search_handler.go" 2>/dev/null; then
    record_result "Semantic search route configured in handler" "PASS"
else
    record_result "Semantic search route configured in handler" "FAIL"
fi

# Test 10: Router registers search routes
if grep -q "Semantic Search\|searchHandler\|SearchHandler" "$ROUTER_FILE" 2>/dev/null; then
    record_result "Search routes registered in router" "PASS"
else
    record_result "Search routes registered in router" "FAIL"
fi

# Test 11: NewSearchHandler constructor exists
if grep -q "func NewSearchHandler" "$HANDLERS_PKG/search_handler.go" 2>/dev/null; then
    record_result "NewSearchHandler constructor exists" "PASS"
else
    record_result "NewSearchHandler constructor exists" "FAIL"
fi

# Test 12: Test file has at least 3 test functions
TEST_COUNT=$(grep -c "^func Test" "$HANDLERS_PKG/search_handler_test.go" 2>/dev/null || echo "0")
if [ "$TEST_COUNT" -ge 3 ]; then
    record_result "search_handler_test.go has >= 3 test functions (actual: $TEST_COUNT)" "PASS"
else
    record_result "search_handler_test.go has >= 3 test functions (actual: $TEST_COUNT)" "FAIL"
fi

echo ""
print_summary "Search Handler Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
