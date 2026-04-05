#!/usr/bin/env bash
set -euo pipefail

# Fuzz Test Validation Challenge
# Validates that the fuzz test suite is complete:
# sufficient files, function count, compilation, and area coverage.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

setup_challenge "fuzz_test_validation" "$@"

PASSED=0
FAILED=0
TOTAL=0
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FUZZ_DIR="$PROJECT_ROOT/tests/fuzz"

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

print_header "Fuzz Test Validation Challenge"
echo ""

# Test 1: tests/fuzz/ directory exists
if [ -d "$FUZZ_DIR" ]; then
    record_result "tests/fuzz/ directory exists" "PASS"
else
    record_result "tests/fuzz/ directory exists" "FAIL"
fi

# Test 2: At least 15 fuzz files
FUZZ_FILE_COUNT=$(find "$FUZZ_DIR" -name "*_test.go" -type f 2>/dev/null | wc -l)
if [ "$FUZZ_FILE_COUNT" -ge 15 ]; then
    record_result "tests/fuzz/ has >= 15 fuzz files (actual: $FUZZ_FILE_COUNT)" "PASS"
else
    record_result "tests/fuzz/ has >= 15 fuzz files (actual: $FUZZ_FILE_COUNT)" "FAIL"
fi

# Test 3: Fuzz functions exist
FUZZ_FUNC_COUNT=$(grep -r "func Fuzz" "$FUZZ_DIR" 2>/dev/null | wc -l)
if [ "$FUZZ_FUNC_COUNT" -gt 0 ]; then
    record_result "Fuzz functions exist (func Fuzz...)" "PASS"
else
    record_result "Fuzz functions exist (func Fuzz...)" "FAIL"
fi

# Test 4: At least 40 fuzz functions
if [ "$FUZZ_FUNC_COUNT" -ge 40 ]; then
    record_result "Fuzz function count >= 40 (actual: $FUZZ_FUNC_COUNT)" "PASS"
else
    record_result "Fuzz function count >= 40 (actual: $FUZZ_FUNC_COUNT)" "FAIL"
fi

# Test 5: Fuzz files compile
cd "$PROJECT_ROOT"
if GOMAXPROCS=2 nice -n 19 go test -run='^$' -count=1 -p 1 ./tests/fuzz/... 2>/dev/null; then
    record_result "Fuzz test files compile" "PASS"
else
    record_result "Fuzz test files compile" "FAIL"
fi

# Test 6: Auth area covered (auth_token_fuzz_test.go)
if [ -f "$FUZZ_DIR/auth_token_fuzz_test.go" ]; then
    record_result "Auth area covered (auth_token_fuzz_test.go)" "PASS"
else
    record_result "Auth area covered (auth_token_fuzz_test.go)" "FAIL"
fi

# Test 7: Cache area covered (cache_key_fuzz_test.go)
if [ -f "$FUZZ_DIR/cache_key_fuzz_test.go" ]; then
    record_result "Cache area covered (cache_key_fuzz_test.go)" "PASS"
else
    record_result "Cache area covered (cache_key_fuzz_test.go)" "FAIL"
fi

# Test 8: Config parsing area covered
if [ -f "$FUZZ_DIR/config_parsing_fuzz_test.go" ]; then
    record_result "Config parsing area covered (config_parsing_fuzz_test.go)" "PASS"
else
    record_result "Config parsing area covered (config_parsing_fuzz_test.go)" "FAIL"
fi

# Test 9: Debate area covered
if [ -f "$FUZZ_DIR/debate_message_fuzz_test.go" ]; then
    record_result "Debate area covered (debate_message_fuzz_test.go)" "PASS"
else
    record_result "Debate area covered (debate_message_fuzz_test.go)" "FAIL"
fi

# Test 10: JSON parsing area covered
if [ -f "$FUZZ_DIR/json_parsing_fuzz_test.go" ]; then
    record_result "JSON parsing area covered (json_parsing_fuzz_test.go)" "PASS"
else
    record_result "JSON parsing area covered (json_parsing_fuzz_test.go)" "FAIL"
fi

# Test 11: MCP message area covered
if [ -f "$FUZZ_DIR/mcp_message_fuzz_test.go" ]; then
    record_result "MCP message area covered (mcp_message_fuzz_test.go)" "PASS"
else
    record_result "MCP message area covered (mcp_message_fuzz_test.go)" "FAIL"
fi

# Test 12: Memory/RAG area covered
if [ -f "$FUZZ_DIR/memory_rag_fuzz_test.go" ]; then
    record_result "Memory/RAG area covered (memory_rag_fuzz_test.go)" "PASS"
else
    record_result "Memory/RAG area covered (memory_rag_fuzz_test.go)" "FAIL"
fi

# Test 13: Rate limit area covered
if [ -f "$FUZZ_DIR/rate_limit_fuzz_test.go" ]; then
    record_result "Rate limit area covered (rate_limit_fuzz_test.go)" "PASS"
else
    record_result "Rate limit area covered (rate_limit_fuzz_test.go)" "FAIL"
fi

# Test 14: Streaming area covered
if [ -f "$FUZZ_DIR/streaming_data_fuzz_test.go" ]; then
    record_result "Streaming area covered (streaming_data_fuzz_test.go)" "PASS"
else
    record_result "Streaming area covered (streaming_data_fuzz_test.go)" "FAIL"
fi

# Test 15: Tool schema area covered
if [ -f "$FUZZ_DIR/tool_schema_fuzz_test.go" ]; then
    record_result "Tool schema area covered (tool_schema_fuzz_test.go)" "PASS"
else
    record_result "Tool schema area covered (tool_schema_fuzz_test.go)" "FAIL"
fi

echo ""
print_summary "Fuzz Test Validation Challenge" "$PASSED" "$FAILED"
[ "$FAILED" -eq 0 ] && exit 0 || exit 1
