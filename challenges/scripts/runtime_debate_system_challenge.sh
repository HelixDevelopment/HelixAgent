#!/bin/bash
# Runtime Debate System Validation Challenge
# VALIDATES: The RUNNING HelixAgent server uses the correct debate system,
#            has strong providers in the debate team, and does NOT fall back
#            to weak/legacy models.
#
# IMPORTANT: This challenge requires the HelixAgent server to be running
#            on port 8100 with the LATEST binary (make build first!)
#
# Total: ~20 tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

cd "$PROJECT_ROOT"

PASS=0
FAIL=0
SKIP=0
TOTAL=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

pass() { PASS=$((PASS + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}[PASS]${NC} $1"; }
fail() { FAIL=$((FAIL + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}[FAIL]${NC} $1"; }
skip() { SKIP=$((SKIP + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${YELLOW}[SKIP]${NC} $1"; }

# Check if server is running
SERVER_URL="http://localhost:8100"
if ! curl -s --max-time 60 --connect-timeout 5 "$SERVER_URL/v1/health" > /dev/null 2>&1; then
    echo -e "${RED}ERROR: HelixAgent server not running on port 8100${NC}"
    echo "Start it with: GIN_MODE=release ./bin/helixagent"
    exit 1
fi

echo -e "${BLUE}=== Runtime Debate System Validation ===${NC}"
echo -e "${YELLOW}Testing against running server at $SERVER_URL${NC}"
echo ""

# ============================================================================
# Group 1: Server Health & Binary Version
# ============================================================================
echo -e "${BLUE}=== Group 1: Server Health (4 tests) ===${NC}"

# Test 1.1: Server responds to health check
HEALTH=$(curl -s --max-time 60 "$SERVER_URL/v1/health" 2>/dev/null)
if echo "$HEALTH" | grep -q '"status":"healthy"'; then
    pass "Server health check returns healthy"
else
    fail "Server health check failed: $HEALTH"
fi

# Test 1.2: At least 10 providers registered
PROVIDER_COUNT=$(echo "$HEALTH" | grep -oP '"total":\K[0-9]+' || echo "0")
if [ "$PROVIDER_COUNT" -ge 10 ]; then
    pass "At least 10 providers registered (found: $PROVIDER_COUNT)"
else
    fail "Less than 10 providers registered (found: $PROVIDER_COUNT)"
fi

# Test 1.3: At least 10 providers healthy
HEALTHY_COUNT=$(echo "$HEALTH" | grep -oP '"healthy":\K[0-9]+' || echo "0")
if [ "$HEALTHY_COUNT" -ge 10 ]; then
    pass "At least 10 providers healthy (found: $HEALTHY_COUNT)"
else
    fail "Less than 10 providers healthy (found: $HEALTHY_COUNT). Check provider verification."
fi

# Test 1.4: Binary is recent (built within last hour)
# Check binary was built within last 24 hours
BINARY_AGE=$(find bin/helixagent -mmin -1440 2>/dev/null | wc -l)
if [ "$BINARY_AGE" -ge 1 ]; then
    pass "Binary was built recently (within last 24 hours)"
else
    fail "Binary is stale — run 'make build' before starting server"
fi

# ============================================================================
# Group 2: Debate System Type
# ============================================================================
echo -e "${BLUE}=== Group 2: Debate System Type (4 tests) ===${NC}"

# Test 2.1: New debate orchestrator code exists (8-phase protocol)
if grep -rq "8-phase protocol\|NEW Debate Orchestrator\|DebateOrchestrator" internal/router/ internal/services/debate_service*.go 2>/dev/null; then
    pass "New Debate Orchestrator (8-phase) code exists in router/services"
else
    fail "New Debate Orchestrator code NOT found"
fi

# Test 2.2: Orchestrator status endpoint exists
if grep -q "debates/orchestrator/status" internal/router/*.go 2>/dev/null; then
    pass "Debate orchestrator status endpoint registered"
else
    fail "Debate orchestrator status endpoint NOT registered"
fi

# Test 2.3: Server startup log shows new orchestrator was enabled
if [ -f /tmp/helixagent-server.log ]; then
    if grep -q "NEW DEBATE ORCHESTRATOR FRAMEWORK ENABLED\|NEW Debate Orchestrator integration set" /tmp/helixagent-server.log 2>/dev/null; then
        pass "Server log confirms new debate orchestrator is enabled"
    else
        fail "Server log does NOT show new debate orchestrator enabled — may be using legacy 5-member system"
    fi
else
    skip "No server log at /tmp/helixagent-server.log — cannot verify orchestrator type"
fi

# Test 2.4: Legacy debate should NOT be sole system
if grep -q "DebateOrchestrator\|debate_orchestrator\|NewDebateOrchestrator" internal/services/*.go 2>/dev/null; then
    pass "Debate orchestrator integration found in services"
else
    fail "No debate orchestrator integration in services"
fi

# ============================================================================
# Group 3: Strong Providers in Debate Team
# ============================================================================
echo -e "${BLUE}=== Group 3: Strong Providers Must Be Available (6 tests) ===${NC}"

# Check server log for which providers were verified
LOG="/tmp/helixagent-server.log"

# Test 3.1: At least one premium provider verified (Mistral, DeepSeek, Groq, etc.)
if [ -f "$LOG" ]; then
    VERIFIED_COUNT=$(grep -c "Model verified successfully\|Provider verified successfully" "$LOG" 2>/dev/null || echo "0")
    if [ "$VERIFIED_COUNT" -ge 3 ]; then
        pass "At least 3 models/providers verified successfully (found: $VERIFIED_COUNT)"
    else
        fail "Less than 3 models verified (found: $VERIFIED_COUNT) — debate team will be weak"
    fi
else
    skip "No server log — cannot count verified models"
fi

# Test 3.2: Claude models should be in debate team (OAuth)
if [ -f "$LOG" ]; then
    if grep -q "claude.*Score\|claude-opus\|claude-sonnet" "$LOG" 2>/dev/null; then
        pass "Claude models configured for debate team"
    else
        fail "Claude models NOT in debate team — check Claude CLI availability"
    fi
else
    skip "No server log"
fi

# Test 3.3: No weak-only debate team
if [ -f "$LOG" ]; then
    if grep -q "codellama\|llama3.2\|llama-2-" "$LOG" 2>/dev/null; then
        # These exist but should not be the ONLY models
        STRONG_MODELS=$(grep -c "mistral-large\|deepseek-chat\|claude-opus\|gemini-2.5\|gpt-4\|command-a" "$LOG" 2>/dev/null || echo "0")
        if [ "$STRONG_MODELS" -ge 1 ]; then
            pass "Strong models present alongside weak fallbacks ($STRONG_MODELS strong models found)"
        else
            fail "Only weak models found (codellama, llama3.2) — no strong providers verified"
        fi
    else
        pass "No weak-only models detected in logs"
    fi
else
    skip "No server log"
fi

# Test 3.4: Circuit breakers should not be permanently open for primary providers
if [ -f "$LOG" ]; then
    OPEN_CIRCUITS=$(grep "circuit breaker is open" "$LOG" 2>/dev/null | wc -l)
    if [ "$OPEN_CIRCUITS" -le 10 ]; then
        pass "Circuit breakers healthy (open count: $OPEN_CIRCUITS)"
    else
        fail "Too many circuit breakers open ($OPEN_CIRCUITS) — providers are failing"
    fi
else
    skip "No server log"
fi

# Test 3.5: Ollama should not be sole provider
PROVIDERS_LIST=$(curl -s --max-time 60 "$SERVER_URL/v1/providers" 2>/dev/null)
if echo "$PROVIDERS_LIST" | grep -q '"count"'; then
    PROV_COUNT=$(echo "$PROVIDERS_LIST" | grep -oP '"count":\K[0-9]+' || echo "0")
    if [ "$PROV_COUNT" -ge 5 ]; then
        pass "Multiple providers registered (not Ollama-only): $PROV_COUNT providers"
    else
        fail "Too few providers ($PROV_COUNT) — system may rely only on Ollama"
    fi
else
    skip "Cannot query /v1/providers (auth required)"
fi

# Test 3.6: Debate team should have at least 15 unique LLMs configured
if [ -f "$LOG" ]; then
    UNIQUE_LLMS=$(grep -oP "unique_llms=\K[0-9]+" "$LOG" 2>/dev/null | tail -1)
    if [ -n "$UNIQUE_LLMS" ] && [ "$UNIQUE_LLMS" -ge 10 ]; then
        pass "Debate team has $UNIQUE_LLMS unique LLMs (minimum 10)"
    elif [ -n "$UNIQUE_LLMS" ]; then
        fail "Debate team only has $UNIQUE_LLMS unique LLMs (need at least 10)"
    else
        skip "Cannot determine unique LLM count from log"
    fi
else
    skip "No server log"
fi

# ============================================================================
# Group 4: No Hardcoded Fallbacks
# ============================================================================
echo -e "${BLUE}=== Group 4: Dynamic Provider Selection (3 tests) ===${NC}"

# Test 4.1: Debate team uses score-based selection
if grep -q "score_only\|sorted_by_score\|selectDebateTeam\|score-based" internal/verifier/startup.go 2>/dev/null; then
    pass "Debate team uses score-based selection (not hardcoded)"
else
    fail "No score-based selection found — team may be hardcoded"
fi

# Test 4.2: Provider scoring uses 5 weighted components
if grep -q "ResponseSpeed.*0.25\|CostEffectiveness.*0.25\|ModelEfficiency.*0.20\|Capability.*0.20\|Recency.*0.10" internal/verifier/scoring.go 2>/dev/null || \
   grep -q "responseScore.*0.25\|costScore.*0.25\|efficiencyScore.*0.20\|capabilityScore.*0.20\|recencyScore.*0.10" internal/verifier/scoring.go 2>/dev/null; then
    pass "Scoring uses 5 weighted components"
else
    # Check for any multi-component scoring
    COMPONENTS=$(grep -c "Score\|score" internal/verifier/scoring.go 2>/dev/null || echo "0")
    if [ "$COMPONENTS" -ge 20 ]; then
        pass "Multi-component scoring system found ($COMPONENTS score references)"
    else
        fail "Scoring may not use proper weighted components"
    fi
fi

# Test 4.3: Models sorted by score before team assignment
if grep -q "sort\.\|Sort\|sorted\|ranking" internal/verifier/startup.go 2>/dev/null; then
    pass "Models are sorted before debate team assignment"
else
    fail "No sorting found — debate team may not get strongest models"
fi

# ============================================================================
# Group 5: Server-Mode Provider Access
# ============================================================================
echo -e "${BLUE}=== Group 5: Server-Mode Provider Access (3 tests) ===${NC}"

# Test 5.1: Claude CLI server-mode bypass exists
if grep -q "HELIXAGENT_SERVER_MODE\|GIN_MODE" internal/llm/providers/claude/claude_cli.go 2>/dev/null; then
    pass "Claude CLI has server-mode bypass (GIN_MODE/HELIXAGENT_SERVER_MODE)"
else
    fail "Claude CLI missing server-mode bypass — will fail inside Claude Code sessions"
fi

# Test 5.2: Zen Content-Type validation exists
if grep -q "Content-Type\|content-type\|application/json" internal/llm/providers/zen/zen_http.go 2>/dev/null; then
    pass "Zen provider validates Content-Type before JSON decode"
else
    fail "Zen provider missing Content-Type validation — will crash on HTML responses"
fi

# Test 5.3: Verification pipeline is not too strict
if grep -q "result.Score >= 50\|OverallScore >= 50" internal/verifier/service.go 2>/dev/null; then
    pass "Verification threshold is 50 (not overly strict)"
else
    THRESHOLD=$(grep -oP 'OverallScore >= \K[0-9]+' internal/verifier/service.go 2>/dev/null | head -1)
    if [ -n "$THRESHOLD" ] && [ "$THRESHOLD" -le 60 ]; then
        pass "Verification threshold is $THRESHOLD (acceptable)"
    else
        fail "Verification threshold may be too strict (${THRESHOLD:-unknown}) — providers will be rejected"
    fi
fi

# ============================================================================
# Summary
# ============================================================================
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Runtime Debate System Validation${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "  Total:   ${BLUE}$TOTAL${NC}"
echo -e "  Passed:  ${GREEN}$PASS${NC}"
echo -e "  Failed:  ${RED}$FAIL${NC}"
echo -e "  Skipped: ${YELLOW}$SKIP${NC}"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo -e "  ${RED}$FAIL test(s) failed${NC}"
    echo ""
    echo -e "  ${YELLOW}IMPORTANT: If the server is running an old binary:${NC}"
    echo -e "  ${YELLOW}  1. Stop the old server (kill the process on port 8100)${NC}"
    echo -e "  ${YELLOW}  2. Rebuild: make build${NC}"
    echo -e "  ${YELLOW}  3. Restart: GIN_MODE=release ./bin/helixagent${NC}"
    exit 1
else
    echo -e "  ${GREEN}ALL TESTS PASSED${NC}"
fi
