#!/bin/bash
# 4-Pass Validation Integration Challenge
# Validates that 4-Pass Validation Pipeline is applied to debate results

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Debate Integration: 4-Pass Validation"
TOTAL_TESTS=10
PASSED=0
FAILED=0

print_header "$CHALLENGE_NAME"

# Test 1: Validation result is present in response
test_start "Validation result is present"
TOPIC="Analyze the benefits of microservices"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESPONSE" | jq -e '.validation_result' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "validation_result not found in response"
  FAILED=$((FAILED + 1))
fi

# Test 2: Overall score is present
test_start "Overall validation score is present"
if echo "$RESPONSE" | jq -e '.validation_result.overall_score' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "overall_score not found in validation_result"
  FAILED=$((FAILED + 1))
fi

# Test 3: Pass results are present
test_start "Pass results are present"
if echo "$RESPONSE" | jq -e '.validation_result.pass_results' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "pass_results not found in validation_result"
  FAILED=$((FAILED + 1))
fi

# Test 4: Overall passed status is present
test_start "Overall passed status is present"
if echo "$RESPONSE" | jq -e '.validation_result.overall_passed' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "overall_passed not found in validation_result"
  FAILED=$((FAILED + 1))
fi

# Test 5: Quality score updated from validation
test_start "Quality score updated from validation"
QUALITY_SCORE=$(echo "$RESPONSE" | jq -r '.quality_score')
if [ -n "$QUALITY_SCORE" ] && [ "$QUALITY_SCORE" != "null" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "quality_score not updated from validation"
  FAILED=$((FAILED + 1))
fi

# Test 6: Validation works for code content
test_start "Validation works for code generation tasks"
TOPIC="Write a function to parse JSON"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.validation_result' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Validation not applied to code generation"
  FAILED=$((FAILED + 1))
fi

# Test 7: Validation works for documentation
test_start "Validation works for documentation tasks"
TOPIC="Document the API endpoints"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.validation_result' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Validation not applied to documentation"
  FAILED=$((FAILED + 1))
fi

# Test 8: Validation score is numeric and valid
test_start "Validation score is numeric and in valid range"
SCORE=$(echo "$RESPONSE" | jq -r '.validation_result.overall_score // 0')
if (( $(echo "$SCORE >= 0" | bc -l) )) && (( $(echo "$SCORE <= 1" | bc -l) )); then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Validation score $SCORE is out of range [0,1]"
  FAILED=$((FAILED + 1))
fi

# Test 9: Failed pass information is tracked
test_start "Failed pass information is tracked"
if echo "$RESPONSE" | jq -e '.validation_result.failed_pass' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "failed_pass field not found"
  FAILED=$((FAILED + 1))
fi

# Test 10: Validation duration is tracked
test_start "Validation duration is tracked"
if echo "$RESPONSE" | jq -e '.validation_result.total_duration' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "total_duration not found in validation_result"
  FAILED=$((FAILED + 1))
fi

print_summary $PASSED $FAILED $TOTAL_TESTS
exit $FAILED
