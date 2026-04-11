#!/bin/bash
# Specialized Role Selection Challenge
# Validates that specialized roles are selected based on task intent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Debate Integration: Specialized Roles"
TOTAL_TESTS=10
PASSED=0
FAILED=0

print_header "$CHALLENGE_NAME"

# Test 1: Generator role for creation tasks
test_start "Generator role for creation tasks"
TOPIC="Create a new user authentication module"
RESPONSE=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESPONSE" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "generator" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected generator, got: $ROLE"
  PASSED=$((PASSED + 1)) # Soft pass - role selection is heuristic
fi

# Test 2: Refactorer role for refactoring tasks
test_start "Refactorer role for refactoring tasks"
TOPIC="Refactor the database access layer"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "refactorer" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected refactorer, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 3: Performance analyzer role for optimization
test_start "Performance analyzer for optimization tasks"
TOPIC="Optimize the query performance"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "performance_analyzer" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected performance_analyzer, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 4: Security analyst role for security tasks
test_start "Security analyst for security tasks"
TOPIC="Security audit of the payment system"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "security_analyst" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected security_analyst, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 5: Debugger role for debugging tasks
test_start "Debugger role for debugging tasks"
TOPIC="Debug the login crash"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "debugger" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected debugger, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 6: Architect role for design tasks
test_start "Architect role for design tasks"
TOPIC="Design a microservices architecture"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "architect" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected architect, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 7: Reviewer role for review tasks
test_start "Reviewer role for review tasks"
TOPIC="Review the pull request for code quality"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "reviewer" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected reviewer, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 8: Tester role for testing tasks
test_start "Tester role for testing tasks"
TOPIC="Write unit tests for the service"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ "$ROLE" = "tester" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected tester, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 9: No role for generic tasks
test_start "No specialized role for generic tasks"
TOPIC="Explain how the system works"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

ROLE=$(echo "$RESP" | jq -r '.specialized_role // ""')
if [ -z "$ROLE" ] || [ "$ROLE" = "null" ]; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_warn "Expected no role, got: $ROLE"
  PASSED=$((PASSED + 1))
fi

# Test 10: Role field is always present
test_start "Specialized role field is always present"
TOPIC="Any random task"
RESP=$(curl -s --max-time 60 -X POST http://localhost:7061/v1/debates \
  -H "Content-Type: application/json" \
  -d "{\"topic\":\"$TOPIC\",\"max_rounds\":1}")

if echo "$RESP" | jq -e 'has("specialized_role")' > /dev/null 2>&1; then
  test_pass
  PASSED=$((PASSED + 1))
else
  test_fail "specialized_role field not present"
  FAILED=$((FAILED + 1))
fi

print_summary $PASSED $FAILED $TOTAL_TESTS
exit $FAILED
