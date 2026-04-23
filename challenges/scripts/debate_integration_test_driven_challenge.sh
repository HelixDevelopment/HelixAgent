#!/bin/bash
# Test-Driven Debate Integration Challenge
# Validates that Test-Driven Debate mode activates for code generation tasks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Debate Integration: Test-Driven Mode"
TOTAL_TESTS=10
PASSED=0
FAILED=0

print_header "$CHALLENGE_NAME"

# Test 1: Code generation triggers Test-Driven mode
test_start "Code generation triggers Test-Driven mode"
TOPIC="Write a Python function to calculate fibonacci numbers"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESPONSE" | jq -e '.test_driven_metadata' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "test_driven_metadata not found in response"
  FAILED=$((FAILED + 1))
fi

# Test 2: Test generation count is tracked
test_start "Test-Driven metadata includes test count"
if echo "$RESPONSE" | jq -e '.test_driven_metadata.tests_generated' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "tests_generated not found in metadata"
  FAILED=$((FAILED + 1))
fi

# Test 3: Passed test count is tracked
test_start "Test-Driven metadata includes passed count"
if echo "$RESPONSE" | jq -e '.test_driven_metadata.tests_passed' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "tests_passed not found in metadata"
  FAILED=$((FAILED + 1))
fi

# Test 4: Failed test count is tracked
test_start "Test-Driven metadata includes failed count"
if echo "$RESPONSE" | jq -e '.test_driven_metadata.tests_failed' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "tests_failed not found in metadata"
  FAILED=$((FAILED + 1))
fi

# Test 5: Refinement status is tracked
test_start "Test-Driven metadata includes refinement status"
if echo "$RESPONSE" | jq -e '.test_driven_metadata.refinement' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "refinement status not found in metadata"
  FAILED=$((FAILED + 1))
fi

# Test 6: Different code generation keywords trigger Test-Driven mode
test_start "Various code keywords trigger Test-Driven mode"
CODE_KEYWORDS=("implement" "create" "develop" "build")
TRIGGERED=0

for KEYWORD in "${CODE_KEYWORDS[@]}"; do
  TOPIC="$KEYWORD a sorting algorithm"
  RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
    -H "Content-Type: application/json" \
    -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

  if echo "$RESP" | jq -e '.test_driven_metadata' > /dev/null 2>&1; then
    TRIGGERED=$((TRIGGERED + 1))
  fi
done

if [ $TRIGGERED -ge 3 ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Only $TRIGGERED/4 keywords triggered Test-Driven mode"
  FAILED=$((FAILED + 1))
fi

# Test 7: Non-code tasks don't trigger Test-Driven mode
test_start "Non-code tasks don't trigger Test-Driven mode"
TOPIC="Explain how binary search works"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if ! echo "$RESPONSE" | jq -e '.test_driven_metadata' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Non-code task incorrectly triggered Test-Driven mode"
  FAILED=$((FAILED + 1))
fi

# Test 8: Test-Driven mode works with different languages
test_start "Test-Driven mode works with language-specific tasks"
LANGUAGES=("Python" "JavaScript" "Go")
LANG_COUNT=0

for LANG in "${LANGUAGES[@]}"; do
  TOPIC="Write a $LANG function for email validation"
  RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
    -H "Content-Type: application/json" \
    -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

  if echo "$RESP" | jq -e '.test_driven_metadata' > /dev/null 2>&1; then
    LANG_COUNT=$((LANG_COUNT + 1))
  fi
done

if [ $LANG_COUNT -ge 2 ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Only $LANG_COUNT/3 languages triggered Test-Driven mode"
  FAILED=$((FAILED + 1))
fi

# Test 9: Debate ID is preserved in Test-Driven mode
test_start "Debate ID is preserved in Test-Driven mode"
TOPIC="Write a function to validate credit card numbers"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

DEBATE_ID=$(echo "$RESPONSE" | jq -r '.debate_id')
if [ -n "$DEBATE_ID" ] && [ "$DEBATE_ID" != "null" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Debate ID missing in Test-Driven response"
  FAILED=$((FAILED + 1))
fi

# Test 10: Success flag is set correctly
test_start "Success flag is set in Test-Driven response"
if echo "$RESPONSE" | jq -e '.success' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "Success flag not found in response"
  FAILED=$((FAILED + 1))
fi

print_summary $PASSED $FAILED $TOTAL_TESTS
exit $FAILED
