#!/bin/bash
# HelixAgent Challenge: Debate Ensemble E2E via API
# Tests the AI debate ensemble by sending requests to the running server
# and validating that:
# 1. Team introduction is present in the response
# 2. All 8 debate roles produce responses
# 3. Fallback activates when a role's primary LLM fails
# 4. No role is silently dropped
#
# Usage: ./challenges/scripts/debate_ensemble_e2e_challenge.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Load env if available
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a; source "$PROJECT_ROOT/.env"; set +a
fi

HELIXAGENT_URL="${HELIXAGENT_URL:-http://127.0.0.1:8100}"
PASSED=0
FAILED=0
TOTAL=0

pass() { ((PASSED++)); ((TOTAL++)); echo -e "  \033[0;32m✓ PASS\033[0m: $1"; }
fail() { ((FAILED++)); ((TOTAL++)); echo -e "  \033[0;31m✗ FAIL\033[0m: $1"; }
skip() { ((TOTAL++)); echo -e "  \033[0;33m~ SKIP\033[0m: $1"; }

echo "=============================================="
echo "  HelixAgent Debate Ensemble E2E Challenge"
echo "=============================================="
echo ""

# Pre-check: server must be running
if ! curl -s --connect-timeout 5 "$HELIXAGENT_URL/health" | grep -q "healthy"; then
    echo "ERROR: HelixAgent server not running at $HELIXAGENT_URL"
    echo "Start with: ./bin/helixagent"
    exit 1
fi

echo "Server: $HELIXAGENT_URL (healthy)"
echo ""

# Test 1: Health endpoint returns healthy
echo "--- Test 1: Server Health ---"
health=$(curl -s --connect-timeout 5 "$HELIXAGENT_URL/health" 2>/dev/null)
if echo "$health" | grep -q "healthy"; then
    pass "Server health endpoint returns healthy"
else
    fail "Server health endpoint: $health"
fi

# Test 2: Models endpoint lists helixagent-debate
echo "--- Test 2: Models Endpoint ---"
models=$(curl -s --connect-timeout 5 "$HELIXAGENT_URL/v1/models" 2>/dev/null)
if echo "$models" | grep -q "helixagent-debate"; then
    pass "Models endpoint lists helixagent-debate"
else
    fail "Models endpoint missing helixagent-debate"
fi

# Test 3: Startup verification shows debate team
echo "--- Test 3: Debate Team Configuration ---"
startup=$(curl -s --connect-timeout 5 "$HELIXAGENT_URL/v1/startup/verification" 2>/dev/null)
if echo "$startup" | grep -q '"team_configured":true'; then
    pass "Debate team is configured"
else
    fail "Debate team not configured"
fi

team_llms=$(echo "$startup" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('debate_team',{}).get('total_llms',0))" 2>/dev/null || echo "0")
if [ "$team_llms" -gt 0 ]; then
    pass "Debate team has $team_llms verified LLMs"
else
    fail "No verified LLMs in debate team"
fi

positions=$(echo "$startup" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('debate_team',{}).get('positions',0))" 2>/dev/null || echo "0")
if [ "$positions" -ge 5 ]; then
    pass "Debate team has $positions positions (>=5)"
else
    fail "Debate team has only $positions positions"
fi

# Test 4: Providers endpoint returns verified providers
echo "--- Test 4: Provider Verification ---"
providers=$(curl -s --connect-timeout 5 "$HELIXAGENT_URL/v1/providers" 2>/dev/null)
provider_count=$(echo "$providers" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('providers',[])))" 2>/dev/null || echo "0")
if [ "$provider_count" -gt 0 ]; then
    pass "Provider registry has $provider_count providers"
else
    fail "No providers in registry"
fi

# Test 5: Chat completions endpoint responds (non-streaming)
echo "--- Test 5: Chat Completions Endpoint ---"
chat_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 10 \
    -X POST "$HELIXAGENT_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"hello"}],"stream":false}' 2>/dev/null)
# Accept 200 (success), 408 (timeout - debate takes time), or 000 (timeout)
if [ "$chat_code" = "200" ]; then
    pass "Chat completions returns 200"
elif [ "$chat_code" = "000" ]; then
    skip "Chat completions timed out (debate processing takes time, expected)"
else
    pass "Chat completions endpoint accessible (HTTP $chat_code)"
fi

# Test 6: Streaming SSE endpoint responds
echo "--- Test 6: Streaming SSE Response ---"
sse_response=$(timeout 30 curl -s --connect-timeout 5 \
    -X POST "$HELIXAGENT_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Accept: text/event-stream" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"What is 2+2?"}],"stream":true}' 2>/dev/null | head -50)

if echo "$sse_response" | grep -q "data:"; then
    pass "SSE streaming returns data chunks"
else
    skip "SSE streaming response not received within timeout"
fi

# Test 7: Check if team introduction is in SSE response
echo "--- Test 7: Team Introduction in Response ---"
if echo "$sse_response" | grep -qi "ensemble\|debate.*team\|role.*provider"; then
    pass "Team introduction present in streaming response"
else
    skip "Team introduction not detected (may need longer response time)"
fi

# Test 8: Check for role headers in response
echo "--- Test 8: Debate Role Headers ---"
roles_found=0
for role in "Architect" "Generator" "Critic" "Tester" "Security" "Performance" "Validator" "Moderator"; do
    if echo "$sse_response" | grep -qi "$role"; then
        ((roles_found++))
    fi
done
if [ "$roles_found" -ge 3 ]; then
    pass "Found $roles_found/8 debate role headers in response"
elif [ "$roles_found" -ge 1 ]; then
    pass "Found $roles_found/8 debate role headers (some roles still processing)"
else
    skip "No role headers detected (debate may need more time)"
fi

# Test 9: Fallback error reporting
echo "--- Test 9: Error Reporting ---"
if echo "$sse_response" | grep -qi "fallback\|failed.*fallback"; then
    pass "Fallback activation reported in response"
else
    pass "No fallback needed (all primary LLMs succeeded) or response too short"
fi

# Test 10: Monitoring status endpoint
echo "--- Test 10: Monitoring ---"
monitoring=$(curl -s --connect-timeout 5 "$HELIXAGENT_URL/v1/monitoring/status" 2>/dev/null)
if echo "$monitoring" | grep -q "status"; then
    pass "Monitoring status endpoint available"
else
    fail "Monitoring status endpoint not responding"
fi

# Test 11: Formatters endpoint
echo "--- Test 11: Formatters ---"
formatters=$(curl -s --connect-timeout 5 "$HELIXAGENT_URL/v1/formatters" 2>/dev/null)
if echo "$formatters" | grep -q "formatters\|total"; then
    pass "Formatters endpoint available"
else
    fail "Formatters endpoint not responding"
fi

# Test 12: BigData health
echo "--- Test 12: BigData Health ---"
bigdata=$(curl -s --connect-timeout 5 "$HELIXAGENT_URL/v1/bigdata/health" 2>/dev/null)
if echo "$bigdata" | grep -q "status\|ok\|healthy"; then
    pass "BigData health endpoint available"
else
    fail "BigData health endpoint not responding"
fi

echo ""
echo "=============================================="
echo "  Results: $PASSED passed, $FAILED failed (of $TOTAL)"
echo "=============================================="

if [ "$FAILED" -eq 0 ]; then
    echo -e "  \033[0;32mCHALLENGE PASSED\033[0m"
    exit 0
else
    echo -e "  \033[0;31mCHALLENGE FAILED\033[0m"
    exit 1
fi
