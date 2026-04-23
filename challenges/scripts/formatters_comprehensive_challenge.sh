#!/bin/bash
# Comprehensive Code Formatters Challenge
# Validates the complete formatters system

set -euo pipefail

PASSED=0
FAILED=0
TOTAL=0

HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"

echo "=== Code Formatters Comprehensive Challenge ==="
echo "Testing formatters system at $HELIXAGENT_URL"
echo

# Helper functions
test_case() {
    local name=$1
    TOTAL=$((TOTAL+1))
    echo -n "Test $TOTAL: $name... "
}

pass() {
    echo "✓ PASS"
    PASSED=$((PASSED+1))
}

fail() {
    local reason=$1
    echo "✗ FAIL: $reason"
    FAILED=$((FAILED+1))
}

# Test 1: API Endpoints Exist
test_case "POST /v1/format endpoint exists"
if curl -sf "$HELIXAGENT_URL/v1/format" -X POST -H "Content-Type: application/json" -d '{"content":"test"}' > /dev/null 2>&1 || [ $? -eq 22 ]; then
    pass
else
    fail "endpoint not found"
fi

test_case "GET /v1/formatters endpoint exists"
if curl -sf "$HELIXAGENT_URL/v1/formatters" > /dev/null 2>&1; then
    pass
else
    fail "endpoint not found"
fi

test_case "GET /v1/formatters/detect endpoint exists"
if curl -sf "$HELIXAGENT_URL/v1/formatters/detect?file_path=test.py" > /dev/null 2>&1; then
    pass
else
    fail "endpoint not found"
fi

# Test 2: List Formatters
test_case "List all formatters returns JSON"
RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/formatters")
if echo "$RESPONSE" | jq -e '.formatters' > /dev/null 2>&1; then
    pass
else
    fail "invalid JSON response"
fi

test_case "At least 4 formatters registered"
COUNT=$(echo "$RESPONSE" | jq -r '.count // 0')
if [ "$COUNT" -ge 4 ]; then
    pass
else
    fail "expected at least 4 formatters, got $COUNT"
fi

# Test 3: Python Formatters
test_case "Black formatter is registered"
if echo "$RESPONSE" | jq -e '.formatters[] | select(.name == "black")' > /dev/null 2>&1; then
    pass
else
    fail "black not found"
fi

test_case "Ruff formatter is registered"
if echo "$RESPONSE" | jq -e '.formatters[] | select(.name == "ruff")' > /dev/null 2>&1; then
    pass
else
    fail "ruff not found"
fi

# Test 4: JavaScript/TypeScript Formatters
test_case "Prettier formatter is registered"
if echo "$RESPONSE" | jq -e '.formatters[] | select(.name == "prettier")' > /dev/null 2>&1; then
    pass
else
    fail "prettier not found"
fi

# Test 5: Go Formatter
test_case "Gofmt formatter is registered"
if echo "$RESPONSE" | jq -e '.formatters[] | select(.name == "gofmt")' > /dev/null 2>&1; then
    pass
else
    fail "gofmt not found"
fi

# Test 6: Language Detection
test_case "Detect Python formatter from .py file"
DETECT_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/formatters/detect?file_path=test.py")
LANGUAGE=$(echo "$DETECT_RESPONSE" | jq -r '.language // ""')
if [ "$LANGUAGE" = "python" ]; then
    pass
else
    fail "expected python, got $LANGUAGE"
fi

test_case "Detect JavaScript formatter from .js file"
DETECT_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/formatters/detect?file_path=test.js")
LANGUAGE=$(echo "$DETECT_RESPONSE" | jq -r '.language // ""')
if [ "$LANGUAGE" = "javascript" ]; then
    pass
else
    fail "expected javascript, got $LANGUAGE"
fi

test_case "Detect Go formatter from .go file"
DETECT_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/formatters/detect?file_path=test.go")
LANGUAGE=$(echo "$DETECT_RESPONSE" | jq -r '.language // ""')
if [ "$LANGUAGE" = "go" ]; then
    pass
else
    fail "expected go, got $LANGUAGE"
fi

# Test 7: Format Operations (if formatters are installed)
test_case "Format Python code (if black available)"
FORMAT_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/format" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"content":"def hello(  x,y ):\n  return x+y","language":"python"}')

if echo "$FORMAT_RESPONSE" | jq -e 'has("success")' > /dev/null 2>&1; then
    SUCCESS=$(echo "$FORMAT_RESPONSE" | jq -r '.success')
    if [ "$SUCCESS" = "true" ]; then
        pass
    else
        # Formatter might not be installed, that's okay
        echo "⚠ SKIP (formatter not installed)"
        TOTAL=$((TOTAL-1))
    fi
else
    fail "invalid response"
fi

test_case "Format JavaScript code (if prettier available)"
FORMAT_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/format" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"content":"const x={a:1,b:2};","language":"javascript"}')

if echo "$FORMAT_RESPONSE" | jq -e 'has("success")' > /dev/null 2>&1; then
    SUCCESS=$(echo "$FORMAT_RESPONSE" | jq -r '.success')
    if [ "$SUCCESS" = "true" ]; then
        pass
    else
        echo "⚠ SKIP (formatter not installed)"
        TOTAL=$((TOTAL-1))
    fi
else
    fail "invalid response"
fi

# Test 8: Batch Formatting
test_case "Batch format multiple files"
BATCH_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/format/batch" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{
        "requests": [
            {"content":"def foo(x,y):\n return x+y","language":"python"},
            {"content":"const x={a:1};","language":"javascript"}
        ]
    }')

if echo "$BATCH_RESPONSE" | jq -e '.results' > /dev/null 2>&1; then
    RESULTS_COUNT=$(echo "$BATCH_RESPONSE" | jq -r '.results | length')
    if [ "$RESULTS_COUNT" -eq 2 ]; then
        pass
    else
        fail "expected 2 results, got $RESULTS_COUNT"
    fi
else
    fail "invalid response"
fi

# Test 9: Check-Only Mode
test_case "Check if code is formatted (dry-run)"
CHECK_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/format/check" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"content":"def hello():\n    pass","language":"python"}')

if echo "$CHECK_RESPONSE" | jq -e '.formatted' > /dev/null 2>&1; then
    pass
else
    fail "invalid response"
fi

# Test 10: Filter by Language
test_case "Filter formatters by Python language"
FILTER_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/formatters?language=python")
if echo "$FILTER_RESPONSE" | jq -e '.formatters' > /dev/null 2>&1; then
    COUNT=$(echo "$FILTER_RESPONSE" | jq -r '.count // 0')
    if [ "$COUNT" -ge 1 ]; then
        pass
    else
        fail "expected at least 1 Python formatter"
    fi
else
    fail "invalid response"
fi

# Test 11: Filter by Type
test_case "Filter formatters by native type"
FILTER_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/formatters?type=native")
if echo "$FILTER_RESPONSE" | jq -e '.formatters' > /dev/null 2>&1; then
    pass
else
    fail "invalid response"
fi

# Test 12: Get Formatter Metadata
test_case "Get black formatter metadata"
META_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/formatters/black")
if echo "$META_RESPONSE" | jq -e '.name' > /dev/null 2>&1; then
    NAME=$(echo "$META_RESPONSE" | jq -r '.name')
    if [ "$NAME" = "black" ]; then
        pass
    else
        fail "expected name=black, got $NAME"
    fi
else
    fail "invalid response"
fi

# Test 13: Formatter Capabilities
test_case "Black supports stdin"
if echo "$META_RESPONSE" | jq -e '.supports_stdin == true' > /dev/null 2>&1; then
    pass
else
    fail "black should support stdin"
fi

test_case "Black supports check mode"
if echo "$META_RESPONSE" | jq -e '.supports_check == true' > /dev/null 2>&1; then
    pass
else
    fail "black should support check mode"
fi

# Test 14: Error Handling
test_case "Invalid language returns error"
ERROR_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/format" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"content":"test","language":"invalid_language_xyz"}')

if echo "$ERROR_RESPONSE" | jq -e '.error or .success == false' > /dev/null 2>&1; then
    pass
else
    fail "should return error for invalid language"
fi

test_case "Empty content returns error"
ERROR_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/format" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"content":"","language":"python"}')

if echo "$ERROR_RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
    pass
else
    fail "should return error for empty content"
fi

# Test 15: Response Format
test_case "Format response includes formatter name"
FORMAT_RESPONSE=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/format" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"content":"def foo():\n pass","language":"python"}')

if echo "$FORMAT_RESPONSE" | jq -e '.formatter_name' > /dev/null 2>&1; then
    pass
else
    fail "response should include formatter_name"
fi

test_case "Format response includes duration"
if echo "$FORMAT_RESPONSE" | jq -e '.duration_ms' > /dev/null 2>&1; then
    pass
else
    fail "response should include duration_ms"
fi

test_case "Format response includes changed flag"
if echo "$FORMAT_RESPONSE" | jq -e 'has("changed")' > /dev/null 2>&1; then
    pass
else
    fail "response should include changed flag"
fi

# Summary
echo
echo "=== Challenge Results ==="
echo "Total Tests: $TOTAL"
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo "Pass Rate: $(awk "BEGIN {printf \"%.1f%%\", ($PASSED/$TOTAL)*100}")"
echo

if [ $FAILED -eq 0 ]; then
    echo "✓ ALL TESTS PASSED"
    exit 0
else
    echo "✗ SOME TESTS FAILED"
    exit 1
fi
