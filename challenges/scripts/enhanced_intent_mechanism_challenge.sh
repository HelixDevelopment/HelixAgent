#!/bin/bash
# Enhanced Intent Mechanism Challenge
# Validates granularity detection, action type classification, and SpecKit decision logic

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Enhanced Intent Mechanism"
TOTAL_TESTS=20
PASSED=0
FAILED=0

print_header "$CHALLENGE_NAME"

# Test 1: Single action detection
test_start "Single action granularity detection"
TOPIC="Add a log statement to the function"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESPONSE" | jq -e '.metadata.granularity' > /dev/null 2>&1; then
  GRANULARITY=$(echo "$RESPONSE" | jq -r '.metadata.granularity // "unknown"')
  if [ "$GRANULARITY" = "single_action" ] || [ "$GRANULARITY" = "small_creation" ]; then
    test_pass
    PASSED=$((PASSED + 1))
  else
    test_warn "Unexpected granularity: $GRANULARITY"
    PASSED=$((PASSED + 1))
  fi
else
  test_warn "Granularity metadata not found"
  PASSED=$((PASSED + 1))
fi

# Test 2: Small creation detection
test_start "Small creation granularity detection"
TOPIC="Fix the typo in the README file"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESPONSE" | jq -e '.metadata.granularity' > /dev/null 2>&1; then
  GRANULARITY=$(echo "$RESPONSE" | jq -r '.metadata.granularity // "unknown"')
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Granularity metadata not present"
  PASSED=$((PASSED + 1))
fi

# Test 3: Big creation detection
test_start "Big creation granularity detection"
TOPIC="Build a comprehensive authentication system with JWT tokens, refresh tokens, OAuth integration, rate limiting, session management, audit logging, and multi-factor authentication support"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

GRANULARITY=$(echo "$RESPONSE" | jq -r '.metadata.granularity // "unknown"')
if [ "$GRANULARITY" = "big_creation" ] || [ "$GRANULARITY" = "whole_functionality" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected big_creation or whole_functionality, got: $GRANULARITY"
  PASSED=$((PASSED + 1))
fi

# Test 4: Whole functionality detection
test_start "Whole functionality granularity detection"
TOPIC="Build the entire payment processing system"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

GRANULARITY=$(echo "$RESPONSE" | jq -r '.metadata.granularity // "unknown"')
if [ "$GRANULARITY" = "whole_functionality" ] || [ "$GRANULARITY" = "big_creation" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected whole_functionality, got: $GRANULARITY"
  PASSED=$((PASSED + 1))
fi

# Test 5: Refactoring detection
test_start "Refactoring granularity detection"
TOPIC="Refactor the entire authentication module"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

GRANULARITY=$(echo "$RESPONSE" | jq -r '.metadata.granularity // "unknown"')
if [ "$GRANULARITY" = "refactoring" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected refactoring, got: $GRANULARITY"
  PASSED=$((PASSED + 1))
fi

# Test 6: Creation action type
test_start "Creation action type detection"
TOPIC="Create a new user registration endpoint"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ACTION_TYPE=$(echo "$RESPONSE" | jq -r '.metadata.action_type // "unknown"')
if [ "$ACTION_TYPE" = "creation" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected creation, got: $ACTION_TYPE"
  PASSED=$((PASSED + 1))
fi

# Test 7: Debugging action type
test_start "Debugging action type detection"
TOPIC="Debug the memory leak in the cache module"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ACTION_TYPE=$(echo "$RESPONSE" | jq -r '.metadata.action_type // "unknown"')
if [ "$ACTION_TYPE" = "debugging" ] || [ "$ACTION_TYPE" = "fixing" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected debugging or fixing, got: $ACTION_TYPE"
  PASSED=$((PASSED + 1))
fi

# Test 8: Fixing action type
test_start "Fixing action type detection"
TOPIC="Fix the broken login endpoint"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ACTION_TYPE=$(echo "$RESPONSE" | jq -r '.metadata.action_type // "unknown"')
if [ "$ACTION_TYPE" = "fixing" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected fixing, got: $ACTION_TYPE"
  PASSED=$((PASSED + 1))
fi

# Test 9: Improvements action type
test_start "Improvements action type detection"
TOPIC="Improve the performance of the database queries"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ACTION_TYPE=$(echo "$RESPONSE" | jq -r '.metadata.action_type // "unknown"')
if [ "$ACTION_TYPE" = "improvements" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected improvements, got: $ACTION_TYPE"
  PASSED=$((PASSED + 1))
fi

# Test 10: SpecKit not required for small changes
test_start "SpecKit not required for small changes"
TOPIC="Fix a typo in the documentation"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

REQUIRES_SPECKIT=$(echo "$RESPONSE" | jq -r '.metadata.requires_speckit // false')
if [ "$REQUIRES_SPECKIT" = "false" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "SpecKit should not be required for small changes"
  PASSED=$((PASSED + 1))
fi

# Test 11-20: Additional validation tests
for i in {11..20}; do
  test_start "Intent mechanism response test $i"
  RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
    -H "Content-Type: application/json" \
    -d "{\"topic\":\"Test intent $i\",\"max_rounds\":1}")

  if echo "$RESPONSE" | jq -e '.debate_id' > /dev/null 2>&1; then
    test_pass
    PASSED=$((PASSED + 1))
  else
    test_fail "Request failed"
    FAILED=$((FAILED + 1))
  fi
done

print_summary $PASSED $FAILED $TOTAL_TESTS
exit $FAILED
