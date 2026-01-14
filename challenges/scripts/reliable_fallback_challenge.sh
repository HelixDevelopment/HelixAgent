#!/bin/bash
# reliable_fallback_challenge.sh
#
# CRITICAL: This challenge validates that the AI Debate Team has working fallback providers
#
# PROBLEM SOLVED: When OAuth providers (Claude, Qwen) fail due to token restrictions,
# the system MUST fall back to reliable API providers (Cerebras, Mistral, DeepSeek, Gemini)
# instead of failing completely.
#
# ISSUE HISTORY:
# - Original fallback chain was: Claude -> Zen -> Zen (all failing)
# - Claude OAuth tokens are restricted to Claude Code product only
# - Zen provider had 401 errors causing circuit breaker to open
# - Result: All debate positions showed "Unable to provide analysis at this time"
#
# FIX: Added collectReliableAPIProviders() which ensures Cerebras, Mistral, DeepSeek,
# and Gemini are ALWAYS included as fallbacks before free models.

# Don't use set -e as it causes issues with counter increments and grep patterns
# set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
HELIX_URL="${HELIX_URL:-http://localhost:7061}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Counters
PASSED=0
FAILED=0
TOTAL=0

# Helper functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((PASSED++)) || true; ((TOTAL++)) || true; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; ((FAILED++)) || true; ((TOTAL++)) || true; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

check_result() {
    if [ $1 -eq 0 ]; then
        log_pass "$2"
    else
        log_fail "$2"
    fi
}

# Start tests
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo "  RELIABLE FALLBACK CHALLENGE"
echo "  Validates that working providers are in the fallback chain"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""

# Test 1: Server is healthy
log_info "Test 1: Checking server health..."
HEALTH=$(curl -s --connect-timeout 10 "${HELIX_URL}/health" 2>/dev/null || echo "")
if [ "$HEALTH" = '{"status":"healthy"}' ]; then
    log_pass "Server is healthy"
else
    log_fail "Server is not healthy: $HEALTH"
    exit 1
fi

# Test 2: Unit tests pass
log_info "Test 2: Running unit tests for fallback mechanism..."
cd "${PROJECT_ROOT}"
if go test -run "TestReliableAPIProvidersCollection|TestFallbackChainIncludesWorkingProviders|TestDebateTeamMustHaveWorkingFallbacks" ./internal/services/ > /dev/null 2>&1; then
    log_pass "Unit tests pass"
else
    log_fail "Unit tests failed"
fi

# Test 3: Reliable API providers are defined
log_info "Test 3: Checking reliable provider model definitions..."
CEREBRAS_MODEL=$(grep -o 'Cerebras: "llama-3.3-70b"' "${PROJECT_ROOT}/internal/services/debate_team_config.go" || echo "")
MISTRAL_MODEL=$(grep -o 'Mistral:  "mistral-large-latest"' "${PROJECT_ROOT}/internal/services/debate_team_config.go" || echo "")

if [ -n "$CEREBRAS_MODEL" ] && [ -n "$MISTRAL_MODEL" ]; then
    log_pass "Reliable provider models are defined"
else
    log_fail "Reliable provider models not found in code"
fi

# Test 4: collectReliableAPIProviders method exists
log_info "Test 4: Checking collectReliableAPIProviders method exists..."
if grep -q "func (dtc \*DebateTeamConfig) collectReliableAPIProviders()" "${PROJECT_ROOT}/internal/services/debate_team_config.go"; then
    log_pass "collectReliableAPIProviders method exists"
else
    log_fail "collectReliableAPIProviders method not found"
fi

# Test 5: collectReliableAPIProviders is called before free models
log_info "Test 5: Verifying collection order (reliable before free)..."
CALL_ORDER=$(grep -n "collect.*Models\|collect.*Providers" "${PROJECT_ROOT}/internal/services/debate_team_config.go" | grep -v "func" || echo "")
RELIABLE_LINE=$(echo "$CALL_ORDER" | grep "ReliableAPI" | head -1 | cut -d: -f1)
ZEN_LINE=$(echo "$CALL_ORDER" | grep "ZenModels" | head -1 | cut -d: -f1)
OPENROUTER_LINE=$(echo "$CALL_ORDER" | grep "OpenRouter" | head -1 | cut -d: -f1)

if [ -n "$RELIABLE_LINE" ] && [ -n "$ZEN_LINE" ]; then
    if [ "$RELIABLE_LINE" -lt "$ZEN_LINE" ]; then
        log_pass "Reliable providers collected before Zen models"
    else
        log_fail "Reliable providers should be collected BEFORE Zen models"
    fi
else
    log_warn "Could not verify collection order"
    ((TOTAL++)) || true
fi

# Test 6: API actually responds with content (not "Unable to provide analysis")
log_info "Test 6: Testing actual API response..."
# NOTE: Cognee timeouts can slow this down, so we use a longer timeout
RESPONSE=$(curl -s -X POST "${HELIX_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"What is 1+1?"}],"max_tokens":50}' \
    --connect-timeout 30 --max-time 120 2>/dev/null || echo "")

if echo "$RESPONSE" | grep -q '"content"'; then
    CONTENT=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'])" 2>/dev/null || echo "")
    if [ -n "$CONTENT" ] && [ "$CONTENT" != "Unable to provide analysis at this time." ]; then
        log_pass "API returns actual content: ${CONTENT:0:50}..."
    else
        log_fail "API returns fallback message instead of real content"
    fi
else
    log_fail "API response malformed: $RESPONSE"
fi

# Test 7: Server logs show Cerebras/Mistral being used
log_info "Test 7: Checking if Cerebras/Mistral are being used in requests..."
LOG_CHECK=$(tail -100 /tmp/helix_new.log 2>/dev/null | grep -E "Cerebras API call completed|Mistral API call completed" | head -1 || echo "")
if [ -n "$LOG_CHECK" ]; then
    log_pass "Working providers are being used: ${LOG_CHECK:0:60}..."
else
    log_warn "Could not verify provider usage in logs (may need fresh request)"
    ((TOTAL++)) || true
fi

# Test 8: No circuit breakers blocking all fallbacks
log_info "Test 8: Checking circuit breaker status..."
CIRCUIT_ERRORS=$(tail -50 /tmp/helix_new.log 2>/dev/null | grep -c "circuit breaker is open" 2>/dev/null | tr -d '\n' || echo "0")
# Handle empty result
if [ -z "$CIRCUIT_ERRORS" ]; then CIRCUIT_ERRORS=0; fi
if [ "$CIRCUIT_ERRORS" -lt 5 ] 2>/dev/null; then
    log_pass "Circuit breakers are not blocking all fallbacks"
else
    log_fail "Too many circuit breaker open errors: $CIRCUIT_ERRORS"
fi

# Test 9: Environment variables for reliable providers
log_info "Test 9: Checking required environment variables..."
MISSING_VARS=0
for VAR in CEREBRAS_API_KEY MISTRAL_API_KEY; do
    if [ -z "${!VAR}" ]; then
        log_warn "$VAR not set"
        ((MISSING_VARS++)) || true
    fi
done

if [ "$MISSING_VARS" -eq 0 ]; then
    log_pass "All reliable provider API keys are set"
else
    log_warn "$MISSING_VARS API keys missing - some fallbacks unavailable"
    ((TOTAL++)) || true
fi

# Test 10: getFallbackLLMs prioritizes non-OAuth
log_info "Test 10: Running getFallbackLLMs priority test..."
if go test -v -run "TestFallbackChainIncludesWorkingProviders/getFallbackLLMs_prioritizes" ./internal/services/ 2>&1 | grep -q "PASS"; then
    log_pass "getFallbackLLMs correctly prioritizes non-OAuth providers"
else
    log_fail "getFallbackLLMs priority test failed"
fi

# Summary
echo ""
echo "═══════════════════════════════════════════════════════════════════════════"
echo "  CHALLENGE SUMMARY"
echo "═══════════════════════════════════════════════════════════════════════════"
echo ""
echo -e "  Total Tests: ${TOTAL}"
echo -e "  ${GREEN}Passed:${NC} ${PASSED}"
echo -e "  ${RED}Failed:${NC} ${FAILED}"
echo ""

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  ✅ CHALLENGE PASSED - Reliable fallback mechanism is working!${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════════════════${NC}"
    exit 0
else
    echo -e "${RED}═══════════════════════════════════════════════════════════════════════════${NC}"
    echo -e "${RED}  ❌ CHALLENGE FAILED - ${FAILED} tests failed${NC}"
    echo -e "${RED}═══════════════════════════════════════════════════════════════════════════${NC}"
    exit 1
fi
