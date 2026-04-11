#!/bin/bash
# Metadata Propagation Challenge
# Validates that metadata is properly propagated through integrated features

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Debate Integration: Metadata Propagation"
TOTAL_TESTS=10
PASSED=0
FAILED=0

print_header "$CHALLENGE_NAME"

# Test 1: Basic metadata is present
test_start "Basic debate metadata is present"
TOPIC="Test metadata propagation"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESPONSE" | jq -e '.metadata' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "metadata field not found"
  FAILED=$((FAILED + 1))
fi

# Test 2: Debate ID is unique
test_start "Debate ID is unique"
RESP1=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"First debate\",\"max_rounds\":1}")

RESP2=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"Second debate\",\"max_rounds\":1}")

ID1=$(echo "$RESP1" | jq -r '.debate_id')
ID2=$(echo "$RESP2" | jq -r '.debate_id')

if [ "$ID1" != "$ID2" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Debate IDs are not unique"
  FAILED=$((FAILED + 1))
fi

# Test 3: Session ID is present
test_start "Session ID is present"
if echo "$RESPONSE" | jq -e '.session_id' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "session_id not found"
  FAILED=$((FAILED + 1))
fi

# Test 4: Topic is preserved
test_start "Topic is preserved in response"
TOPIC="Custom topic for preservation test"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

RESP_TOPIC=$(echo "$RESP" | jq -r '.topic')
if [ "$RESP_TOPIC" = "$TOPIC" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Topic not preserved: $RESP_TOPIC"
  FAILED=$((FAILED + 1))
fi

# Test 5: Start time is present
test_start "Start time is present"
if echo "$RESP" | jq -e '.start_time' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "start_time not found"
  FAILED=$((FAILED + 1))
fi

# Test 6: End time is present
test_start "End time is present"
if echo "$RESP" | jq -e '.end_time' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "end_time not found"
  FAILED=$((FAILED + 1))
fi

# Test 7: Duration is calculated
test_start "Duration is calculated"
if echo "$RESP" | jq -e '.duration' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "duration not found"
  FAILED=$((FAILED + 1))
fi

# Test 8: Rounds conducted is accurate
test_start "Rounds conducted matches max rounds"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"Test rounds\",\"max_rounds\":2}")

ROUNDS=$(echo "$RESP" | jq -r '.rounds_conducted // 0')
if [ "$ROUNDS" -ge 1 ] && [ "$ROUNDS" -le 2 ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Rounds conducted: $ROUNDS (expected 1-2)"
  FAILED=$((FAILED + 1))
fi

# Test 9: Quality score is present
test_start "Quality score is present"
if echo "$RESP" | jq -e '.quality_score' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "quality_score not found"
  FAILED=$((FAILED + 1))
fi

# Test 10: Success flag indicates completion
test_start "Success flag indicates completion status"
if echo "$RESP" | jq -e '.success' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "success flag not found"
  FAILED=$((FAILED + 1))
fi

print_summary $PASSED $FAILED $TOTAL_TESTS
exit $FAILED
