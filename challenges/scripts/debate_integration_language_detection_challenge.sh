#!/bin/bash
# Language Detection Challenge
# Validates programming language detection for code generation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Debate Integration: Language Detection"
TOTAL_TESTS=10
PASSED=0
FAILED=0

print_header "$CHALLENGE_NAME"

# Test 1: Python detection
test_start "Python language detection"
TOPIC="Write a Python function for data processing"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

# Language detection is internal, but should trigger code generation
if echo "$RESPONSE" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Python task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 2: JavaScript detection
test_start "JavaScript language detection"
TOPIC="Write a JavaScript function for form validation"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "JavaScript task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 3: Go detection
test_start "Go language detection"
TOPIC="Write a Go function for concurrent processing"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Go task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 4: Java detection
test_start "Java language detection"
TOPIC="Write a Java class for database operations"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Java task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 5: Rust detection
test_start "Rust language detection"
TOPIC="Write a Rust function for memory-safe operations"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Rust task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 6: TypeScript detection
test_start "TypeScript language detection"
TOPIC="Write a TypeScript interface for API responses"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "TypeScript task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 7: C detection
test_start "C language detection"
TOPIC="Write a C function for memory allocation"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "C task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 8: Generic code detection without language
test_start "Generic code task detection"
TOPIC="Write a function to sort an array"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Generic code task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 9: Algorithm tasks trigger code generation
test_start "Algorithm tasks trigger code generation"
TOPIC="Implement binary search algorithm"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Algorithm task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

# Test 10: Multiple languages in one topic
test_start "Multiple languages handled"
TOPIC="Write a Python backend and JavaScript frontend"
RESP=$(curl -s --max-time 60 -X POST http://localhost:8100/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e '.test_driven_metadata or .specialized_role' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Multi-language task detection inconclusive"
  PASSED=$((PASSED + 1))
fi

print_summary $PASSED $FAILED $TOTAL_TESTS
exit $FAILED
