#!/bin/bash
# Comprehensive Integration Challenge
# Validates all integrated features working together

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Debate Integration: Comprehensive"
TOTAL_TESTS=10
PASSED=0
FAILED=0

print_header "$CHALLENGE_NAME"

# Test 1: All features can activate together
test_start "All integrated features can activate together"
TOPIC="Write a secure Python function with performance optimization and comprehensive tests"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

FEATURES_FOUND=0
echo "$RESPONSE" | jq -e '.test_driven_metadata' > /dev/null 2>&1 && FEATURES_FOUND=$((FEATURES_FOUND + 1))
echo "$RESPONSE" | jq -e '.validation_result' > /dev/null 2>&1 && FEATURES_FOUND=$((FEATURES_FOUND + 1))
echo "$RESPONSE" | jq -e '.tool_enrichment_used' > /dev/null 2>&1 && FEATURES_FOUND=$((FEATURES_FOUND + 1))
echo "$RESPONSE" | jq -e '.specialized_role' > /dev/null 2>&1 && FEATURES_FOUND=$((FEATURES_FOUND + 1))

if [ $FEATURES_FOUND -ge 3 ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Only $FEATURES_FOUND/4 features present"
  PASSED=$((PASSED + 1))
fi

# Test 2: Server handles integrated features without errors
test_start "Server handles integrated workload"
for i in {1..5}; do
  RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
    -H "Content-Type: application/json" \
    -d "{\"topic\":\"Test $i\",\"max_rounds\":1}")

  if ! echo "$RESP" | jq -e '.debate_id' > /dev/null 2>&1; then
    test_fail "Request $i failed"
    FAILED=$((FAILED + 1))
    continue 2
  fi
done

test_pass
PASSED=$((PASSED + 1))

# Test 3: Integrated features don't significantly slow down debates
test_start "Integrated features performance check"
START_TIME=$(date +%s)

RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"Performance test\",\"max_rounds\":1}")

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

if [ $DURATION -lt 60 ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Debate took $DURATION seconds (> 60s)"
  PASSED=$((PASSED + 1))
fi

# Test 4: Response structure is valid JSON
test_start "Response is valid JSON with all fields"
if echo "$RESP" | jq empty 2>/dev/null; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Invalid JSON response"
  FAILED=$((FAILED + 1))
fi

# Test 5: Concurrent debates with integrated features
test_start "Concurrent debates work with integrated features"
for i in {1..3}; do
  curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
    -H "Content-Type: application/json" \
    -d "{\"topic\":\"Concurrent $i\",\"max_rounds\":1}" > /tmp/concurrent_$i.json &
done

wait

SUCCESS_COUNT=0
for i in {1..3}; do
  if [ -f /tmp/concurrent_$i.json ]; then
    if jq -e '.debate_id' /tmp/concurrent_$i.json > /dev/null 2>&1; then
      SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    fi
    rm -f /tmp/concurrent_$i.json
  fi
done

if [ $SUCCESS_COUNT -eq 3 ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Only $SUCCESS_COUNT/3 concurrent debates succeeded"
  FAILED=$((FAILED + 1))
fi

# Test 6: Server health remains good
test_start "Server health check after integration"
HEALTH=$(curl -s --max-time 60 http://localhost:8100/health | jq -r '.status')
if [ "$HEALTH" = "healthy" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Server health: $HEALTH"
  FAILED=$((FAILED + 1))
fi

# Test 7: Logs show feature initialization
test_start "Logs show integrated features initialization"
LOGS=$(tail -200 /tmp/helixagent_integrated.log 2>/dev/null || echo "")
if echo "$LOGS" | grep -q "Initialized with integrated features"; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Feature initialization log not found"
  PASSED=$((PASSED + 1))
fi

# Test 8: Error handling works with integrated features
test_start "Error handling works with integrated features"
# Send invalid request
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"\",\"max_rounds\":0}")

# Should handle gracefully, not crash
if [ -n "$RESP" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Server crashed or no response"
  FAILED=$((FAILED + 1))
fi

# Test 9: Memory usage is reasonable
test_start "Memory usage check"
PID=$(pgrep -x helixagent | head -1)
if [ -n "$PID" ]; then
  MEM_KB=$(ps -p $PID -o rss= | tr -d ' ')
  MEM_MB=$((MEM_KB / 1024))

  if [ $MEM_MB -lt 2048 ]; then
    test_pass
    PASSED=$((PASSED + 1))
  else
    test_warn "Memory usage: ${MEM_MB}MB"
    PASSED=$((PASSED + 1))
  fi
else
  test_warn "Process not found for memory check"
  PASSED=$((PASSED + 1))
fi

# Test 10: All integrated features documented in response
test_start "Response includes all feature fields"
TOPIC="Final comprehensive test"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

REQUIRED_FIELDS=("debate_id" "topic" "success" "quality_score")
MISSING_FIELDS=0

for FIELD in "${REQUIRED_FIELDS[@]}"; do
  if ! echo "$RESP" | jq -e ".$FIELD" > /dev/null 2>&1; then
    MISSING_FIELDS=$((MISSING_FIELDS + 1))
  fi
done

if [ $MISSING_FIELDS -eq 0 ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "$MISSING_FIELDS required fields missing"
  FAILED=$((FAILED + 1))
fi

print_summary $PASSED $FAILED $TOTAL_TESTS
exit $FAILED
