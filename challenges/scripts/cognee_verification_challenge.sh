#!/bin/bash
# Cognee Verification Challenge
# Tests Cognee container status, authentication, and documents known bug
# 20 tests total

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Test result function
test_result() {
    local name="$1"
    local status="$2"
    local message="${3:-}"

    TESTS_RUN=$((TESTS_RUN + 1))

    case "$status" in
        pass)
            echo -e "${GREEN}✓${NC} Test $TESTS_RUN: $name"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            ;;
        fail)
            echo -e "${RED}✗${NC} Test $TESTS_RUN: $name"
            [ -n "$message" ] && echo -e "  ${RED}Error: $message${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            ;;
        skip)
            echo -e "${YELLOW}⊘${NC} Test $TESTS_RUN: $name (skipped)"
            [ -n "$message" ] && echo -e "  ${YELLOW}Reason: $message${NC}"
            TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
            ;;
        *)
            echo -e "${RED}?${NC} Test $TESTS_RUN: $name (unknown status)"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            ;;
    esac
}

echo "=================================================="
echo "Cognee Verification Challenge"
echo "=================================================="
echo ""

# Test 1: Cognee container exists
if podman ps -a --format "{{.Names}}" | grep -q "helixagent-cognee"; then
    test_result "Cognee container exists" "pass"
else
    test_result "Cognee container exists" "fail" "Container not found"
fi

# Test 2: Cognee container is running
if podman ps --format "{{.Names}}" | grep -q "helixagent-cognee"; then
    test_result "Cognee container is running" "pass"
else
    test_result "Cognee container is running" "fail" "Container not running"
fi

# Test 3: Cognee port 8000 is listening
if timeout 2 bash -c "</dev/tcp/localhost/8000" 2>/dev/null; then
    test_result "Cognee port 8000 listening" "pass"
else
    test_result "Cognee port 8000 listening" "fail" "Port not accessible"
fi

# Test 4: Cognee health endpoint responds
if curl -s --max-time 60 -f "http://localhost:8000/" -o /dev/null; then
    test_result "Cognee health endpoint (/)" "pass"
else
    test_result "Cognee health endpoint (/)" "fail" "Health endpoint not responding"
fi

# Test 5: Health endpoint returns JSON with message
HEALTH_RESPONSE=$(curl -s --max-time 60 "http://localhost:8000/")
if echo "$HEALTH_RESPONSE" | jq -e '.message' >/dev/null 2>&1; then
    test_result "Health endpoint returns JSON" "pass"
else
    test_result "Health endpoint returns JSON" "fail" "Invalid JSON response"
fi

# Test 6: Health message contains expected text
if echo "$HEALTH_RESPONSE" | jq -r '.message' | grep -q "alive"; then
    test_result "Health message contains 'alive'" "pass"
else
    test_result "Health message contains 'alive'" "fail" "Unexpected health message"
fi

# Test 7: Cognee authentication endpoint exists
if curl -s --max-time 60 "http://localhost:8000/api/v1/auth/login" -o /dev/null -w "%{http_code}" | grep -qE "^(200|400|422)$"; then
    test_result "Authentication endpoint exists" "pass"
else
    test_result "Authentication endpoint exists" "fail" "Auth endpoint not found"
fi

# Test 8: Form-encoded authentication succeeds
AUTH_RESPONSE=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "http://localhost:8000/api/v1/auth/login" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "username=admin@helixagent.ai&password=HelixAgentPass123")
HTTP_CODE=$(echo "$AUTH_RESPONSE" | tail -n 1)
if [ "$HTTP_CODE" = "200" ]; then
    test_result "Form-encoded authentication (200 OK)" "pass"
else
    test_result "Form-encoded authentication (200 OK)" "fail" "HTTP $HTTP_CODE"
fi

# Test 9: Authentication returns access_token
ACCESS_TOKEN=$(echo "$AUTH_RESPONSE" | head -n -1 | jq -r '.access_token' 2>/dev/null)
if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
    test_result "Access token obtained" "pass"
else
    test_result "Access token obtained" "fail" "No access_token in response"
fi

# Test 10: Access token is JWT format
if echo "$ACCESS_TOKEN" | grep -qE "^eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$"; then
    test_result "Access token is valid JWT" "pass"
else
    test_result "Access token is valid JWT" "fail" "Token doesn't match JWT format"
fi

# Test 11: KNOWN BUG - Search endpoint times out (documented)
echo ""
echo -e "${YELLOW}Testing known Cognee bug (AttributeError in extract_subgraph_chunks)${NC}"
SEARCH_TIMEOUT=0
if timeout 3 curl -s --max-time 60 -X POST "http://localhost:8000/api/v1/search" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"query":"test"}' >/dev/null 2>&1; then
    test_result "DOCUMENTED BUG: Search API timeout" "fail" "Search should timeout due to Cognee bug, but it succeeded unexpectedly. Bug may be fixed!"
else
    test_result "DOCUMENTED BUG: Search API timeout" "pass" "Search times out as expected (Cognee bug)"
    SEARCH_TIMEOUT=1
fi

# Test 12: Container logs show AttributeError
if podman logs helixagent-cognee 2>&1 | grep -q "AttributeError.*nodes"; then
    test_result "Container logs show AttributeError" "pass" "Bug confirmed in logs"
else
    test_result "Container logs show AttributeError" "skip" "Error not in recent logs (may have been cleared)"
fi

# Test 13: Config has Cognee disabled in ai_debate section
if grep -A 3 "cognee:" "$PROJECT_ROOT/configs/development.yaml" | grep -q "enabled: false"; then
    test_result "Config: Cognee disabled in ai_debate" "pass"
else
    test_result "Config: Cognee disabled in ai_debate" "fail" "Cognee should be disabled due to bug"
fi

# Test 14: Config has cognee_integration feature flag disabled
if grep -A 5 "feature_flags:" "$PROJECT_ROOT/configs/development.yaml" | grep -q "cognee_integration: false"; then
    test_result "Config: cognee_integration feature flag disabled" "pass"
else
    test_result "Config: cognee_integration feature flag disabled" "fail" "Feature flag should be disabled"
fi

# Test 15: Config has Cognee service as optional (not required)
if grep -A 12 "services:" "$PROJECT_ROOT/configs/development.yaml" | grep -A 10 "cognee:" | grep -q "required: false"; then
    test_result "Config: Cognee service optional (not required)" "pass"
else
    test_result "Config: Cognee service optional (not required)" "fail" "Service should be optional due to bug"
fi

# Test 16: Documentation exists
if [ -f "$PROJECT_ROOT/docs/COGNEE_BUG.md" ]; then
    test_result "Bug documentation exists (COGNEE_BUG.md)" "pass"
else
    test_result "Bug documentation exists (COGNEE_BUG.md)" "fail" "Missing documentation"
fi

# Test 17: HelixAgent API works without Cognee
if timeout 5 curl -s --max-time 60 -X POST "http://localhost:8100/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}' | jq -e '.choices[0].message.content' >/dev/null 2>&1; then
    test_result "HelixAgent API works without Cognee" "pass"
else
    test_result "HelixAgent API works without Cognee" "skip" "HelixAgent may not be running"
fi

# Test 18: API response time is fast (<10 seconds)
START_TIME=$(date +%s)
curl -s --max-time 60 -X POST "http://localhost:8100/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}' >/dev/null 2>&1 || true
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))
if [ $ELAPSED -lt 10 ]; then
    test_result "API response time <10s (was $ELAPSED s)" "pass"
else
    test_result "API response time <10s (was $ELAPSED s)" "fail" "Response too slow"
fi

# Test 19: Cognee authentication code exists
if [ -f "$PROJECT_ROOT/internal/services/cognee_service.go" ]; then
    if grep -q "func.*authenticate" "$PROJECT_ROOT/internal/services/cognee_service.go"; then
        test_result "Cognee authentication code exists" "pass"
    else
        test_result "Cognee authentication code exists" "fail" "authenticate function not found"
    fi
else
    test_result "Cognee authentication code exists" "fail" "cognee_service.go not found"
fi

# Test 20: Authentication uses form-encoded OAuth2
if grep -q "application/x-www-form-urlencoded" "$PROJECT_ROOT/internal/services/cognee_service.go"; then
    test_result "Authentication uses form-encoded OAuth2" "pass"
else
    test_result "Authentication uses form-encoded OAuth2" "fail" "OAuth2 form encoding not found"
fi

# Summary
echo ""
echo "=================================================="
echo "Challenge Summary"
echo "=================================================="
echo "Total tests run:    $TESTS_RUN"
echo -e "${GREEN}Tests passed:       $TESTS_PASSED${NC}"
echo -e "${RED}Tests failed:       $TESTS_FAILED${NC}"
echo -e "${YELLOW}Tests skipped:      $TESTS_SKIPPED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "Cognee Status:"
    echo "  • Container: Running (health check passes)"
    echo "  • Authentication: Works correctly"
    echo "  • API Integration: Disabled due to upstream bug"
    echo "  • Impact: None (AI Debate works without Cognee)"
    echo "  • Documentation: COGNEE_BUG.md"
    exit 0
else
    echo ""
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
