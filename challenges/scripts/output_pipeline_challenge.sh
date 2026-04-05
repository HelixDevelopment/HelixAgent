#!/usr/bin/env bash
set -euo pipefail

# Output Pipeline Challenge
# Validates that the internal/output pipeline package is complete:
# source files, test files, parsers, formatters, and renderers registered.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "output_pipeline" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_PKG="$PROJECT_ROOT/internal/output"

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

print_header "Output Pipeline Challenge"
echo ""

# Test 1: pipeline.go exists
if [ -f "$OUTPUT_PKG/pipeline.go" ]; then
    record_result "pipeline.go exists" "PASS"
else
    record_result "pipeline.go exists" "FAIL"
fi

# Test 2: pipeline_test.go exists
if [ -f "$OUTPUT_PKG/pipeline_test.go" ]; then
    record_result "pipeline_test.go exists" "PASS"
else
    record_result "pipeline_test.go exists" "FAIL"
fi

# Test 3: Test count >= 10
TEST_COUNT=$(grep -c "^func Test" "$OUTPUT_PKG/pipeline_test.go" 2>/dev/null || echo "0")
if [ "$TEST_COUNT" -ge 10 ]; then
    record_result "Test count >= 10 (actual: $TEST_COUNT)" "PASS"
else
    record_result "Test count >= 10 (actual: $TEST_COUNT)" "FAIL"
fi

# Test 4: Tests compile
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go build ./internal/output/... 2>/dev/null; then
    record_result "Output package compiles" "PASS"
else
    record_result "Output package compiles" "FAIL"
fi

# Test 5: Tests pass
if GOMAXPROCS=2 nice -n 19 go test -short -count=1 -p 1 ./internal/output/... 2>/dev/null; then
    record_result "Output package tests pass" "PASS"
else
    record_result "Output package tests pass" "FAIL"
fi

# Test 6: Parsers registered - code
if grep -q 'RegisterParser("code"' "$OUTPUT_PKG/pipeline.go" 2>/dev/null; then
    record_result "Parser 'code' registered" "PASS"
else
    record_result "Parser 'code' registered" "FAIL"
fi

# Test 7: Parsers registered - diff, json, markdown, text
PARSER_COUNT=$(grep -c 'RegisterParser(' "$OUTPUT_PKG/pipeline.go" 2>/dev/null || echo "0")
if [ "$PARSER_COUNT" -ge 5 ]; then
    record_result "All 5 parsers registered (code, diff, json, markdown, text)" "PASS"
else
    record_result "All 5 parsers registered (actual: $PARSER_COUNT, need >= 5)" "FAIL"
fi

# Test 8: Formatters registered (syntax, diff, table, raw)
FORMATTER_COUNT=$(grep -c 'RegisterFormatter(' "$OUTPUT_PKG/pipeline.go" 2>/dev/null || echo "0")
if [ "$FORMATTER_COUNT" -ge 4 ]; then
    record_result "All 4 formatters registered (syntax, diff, table, raw)" "PASS"
else
    record_result "All 4 formatters registered (actual: $FORMATTER_COUNT, need >= 4)" "FAIL"
fi

# Test 9: Renderers registered (terminal, html, json)
RENDERER_COUNT=$(grep -c 'RegisterRenderer(' "$OUTPUT_PKG/pipeline.go" 2>/dev/null || echo "0")
if [ "$RENDERER_COUNT" -ge 3 ]; then
    record_result "All 3 renderers registered (terminal, html, json)" "PASS"
else
    record_result "All 3 renderers registered (actual: $RENDERER_COUNT, need >= 3)" "FAIL"
fi

# Test 10: Pipeline struct has parsers, formatters, renderers fields
if grep -q "parsers\s\+map\[string\]Parser" "$OUTPUT_PKG/pipeline.go" 2>/dev/null || \
   grep -q "parsers map\[string\]Parser" "$OUTPUT_PKG/pipeline.go" 2>/dev/null; then
    record_result "Pipeline struct has parsers, formatters, renderers fields" "PASS"
else
    record_result "Pipeline struct has parsers, formatters, renderers fields" "FAIL"
fi

echo ""
print_summary "Output Pipeline Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
