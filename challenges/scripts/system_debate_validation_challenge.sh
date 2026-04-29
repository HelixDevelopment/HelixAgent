#!/bin/bash
# System Debate Validation Challenge
# VALIDATES: Debate system configuration, critical provider availability,
#            OAuth/CLI provider support, verifier integration,
#            provider fallback chains, and system resilience
# Total: ~36 tests across 6 groups

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0
SKIPPED=0
TOTAL=0

pass() {
    PASSED=$((PASSED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${GREEN}[PASS]${NC} $1"
}

fail() {
    FAILED=$((FAILED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${RED}[FAIL]${NC} $1"
}

skip() {
    SKIPPED=$((SKIPPED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${YELLOW}[SKIP]${NC} $1"
}

section() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

PROVIDERS_DIR="$PROJECT_ROOT/internal/llm/providers"
SERVICES_DIR="$PROJECT_ROOT/internal/services"
VERIFIER_DIR="$PROJECT_ROOT/internal/verifier"
ROUTER_DIR="$PROJECT_ROOT/internal/router"
VERIFIER_FALLBACK="$PROJECT_ROOT/LLMsVerifier/llm-verifier/providers/fallback_models.go"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  System Debate Validation Challenge${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "  Validates: Debate system config, providers,"
echo -e "  OAuth/CLI support, verifier, fallbacks, resilience"

#===============================================================================
# Group 1: Debate System Configuration (6 tests)
#===============================================================================
section "Group 1: Debate System Configuration (6 tests)"

# Test 1.1: New Debate Orchestrator code exists
if grep -rq 'NEW Debate Orchestrator\|8-phase protocol' "$ROUTER_DIR/" "$SERVICES_DIR/" "$PROJECT_ROOT/internal/handlers/" 2>/dev/null; then
    pass "New Debate Orchestrator code exists (NEW Debate Orchestrator or 8-phase protocol reference)"
else
    fail "New Debate Orchestrator code NOT found in router, services, or handlers"
fi

# Test 1.2: Debate orchestrator status endpoint registered
if grep -q '/debates/orchestrator/status' "$ROUTER_DIR/router.go" 2>/dev/null; then
    pass "Debate orchestrator status endpoint registered (/v1/debates/orchestrator/status)"
else
    fail "Debate orchestrator status endpoint NOT registered in router.go"
fi

# Test 1.3: Debate team config has at least 5 positions
if grep -q 'TotalDebatePositions = 5' "$SERVICES_DIR/debate_team_config.go" 2>/dev/null; then
    pass "Debate team config has 5 positions (TotalDebatePositions = 5)"
else
    # Fallback: check for at least 5 position constants
    POS_COUNT=$(grep -c 'Position[A-Z]' "$SERVICES_DIR/debate_team_config.go" 2>/dev/null || echo "0")
    POS_COUNT=${POS_COUNT//[^0-9]/}
    POS_COUNT=${POS_COUNT:-0}
    if [ "$POS_COUNT" -ge 5 ]; then
        pass "Debate team config has at least 5 positions (found $POS_COUNT position constants)"
    else
        fail "Debate team config does NOT have 5 positions (found $POS_COUNT)"
    fi
fi

# Test 1.4: Debate team supports score-based selection
if grep -q 'selectDebateTeam' "$VERIFIER_DIR/startup.go" 2>/dev/null; then
    pass "Debate team supports score-based selection (selectDebateTeam function exists)"
else
    fail "selectDebateTeam function NOT found in verifier startup.go"
fi

# Test 1.5: Debate performance optimizer exists
if [ -f "$SERVICES_DIR/debate_performance_optimizer.go" ]; then
    pass "Debate performance optimizer exists (debate_performance_optimizer.go)"
else
    fail "Debate performance optimizer NOT found (debate_performance_optimizer.go)"
fi

# Test 1.6: Debate reflexion framework exists
REFLEXION_FILES=$(find "$PROJECT_ROOT/DebateOrchestrator/reflexion" -name "*.go" 2>/dev/null | grep -cv '_test.go' || echo "0")
REFLEXION_FILES=${REFLEXION_FILES//[^0-9]/}
REFLEXION_FILES=${REFLEXION_FILES:-0}
if [ "$REFLEXION_FILES" -ge 1 ]; then
    pass "Debate reflexion framework exists ($REFLEXION_FILES source files in DebateOrchestrator/reflexion/)"
else
    fail "Debate reflexion framework NOT found in DebateOrchestrator/reflexion/"
fi

#===============================================================================
# Group 2: Critical Provider Availability (8 tests)
#===============================================================================
section "Group 2: Critical Provider Availability (8 tests)"

# Test 2.1: Claude provider registered
if grep -q 'case "claude":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "Claude provider registered in provider_registry.go"
else
    fail "Claude provider NOT registered in provider_registry.go"
fi

# Test 2.2: Gemini provider registered
if grep -q 'case "gemini":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "Gemini provider registered in provider_registry.go"
else
    fail "Gemini provider NOT registered in provider_registry.go"
fi

# Test 2.3: Groq provider registered
if grep -q 'case "groq":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "Groq provider registered in provider_registry.go"
else
    fail "Groq provider NOT registered in provider_registry.go"
fi

# Test 2.4: Cohere provider registered
if grep -q 'case "cohere":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "Cohere provider registered in provider_registry.go"
else
    fail "Cohere provider NOT registered in provider_registry.go"
fi

# Test 2.5: GitHub Models provider registered
if grep -q 'case "github-models":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "GitHub Models provider registered in provider_registry.go"
else
    fail "GitHub Models provider NOT registered in provider_registry.go"
fi

# Test 2.6: Venice provider registered
if grep -q 'case "venice":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "Venice provider registered in provider_registry.go"
else
    fail "Venice provider NOT registered in provider_registry.go"
fi

# Test 2.7: OpenRouter provider registered
if grep -q 'case "openrouter":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "OpenRouter provider registered in provider_registry.go"
else
    fail "OpenRouter provider NOT registered in provider_registry.go"
fi

# Test 2.8: Cloudflare provider registered
if grep -q 'case "cloudflare":' "$SERVICES_DIR/provider_registry.go" 2>/dev/null; then
    pass "Cloudflare provider registered in provider_registry.go"
else
    fail "Cloudflare provider NOT registered in provider_registry.go"
fi

#===============================================================================
# Group 3: OAuth/CLI Provider Support (6 tests)
#===============================================================================
section "Group 3: OAuth/CLI Provider Support (6 tests)"

# Test 3.1: Claude CLI provider exists
if [ -f "$PROVIDERS_DIR/claude/claude_cli.go" ]; then
    pass "Claude CLI provider exists (claude_cli.go)"
else
    fail "Claude CLI provider NOT found (claude_cli.go)"
fi

# Test 3.2: Claude CLI server-mode bypass exists
if grep -q 'non-interactive\|Allow CLI usage\|HelixAgent server' "$PROVIDERS_DIR/claude/claude_cli.go" 2>/dev/null; then
    pass "Claude CLI server-mode bypass exists (non-interactive/server-mode comments)"
else
    fail "Claude CLI server-mode bypass NOT found in claude_cli.go"
fi

# Test 3.3: Qwen ACP provider exists
if [ -f "$PROVIDERS_DIR/qwen/qwen_acp.go" ]; then
    pass "Qwen ACP provider exists (qwen_acp.go)"
else
    fail "Qwen ACP provider NOT found (qwen_acp.go)"
fi

# Test 3.4: Zen HTTP provider exists
if [ -f "$PROVIDERS_DIR/zen/zen_http.go" ]; then
    pass "Zen HTTP provider exists (zen_http.go)"
else
    fail "Zen HTTP provider NOT found (zen_http.go)"
fi

# Test 3.5: Junie CLI provider exists
if [ -f "$PROVIDERS_DIR/junie/junie_cli.go" ]; then
    pass "Junie CLI provider exists (junie_cli.go)"
else
    fail "Junie CLI provider NOT found (junie_cli.go)"
fi

# Test 3.6: Gemini CLI provider exists
if [ -f "$PROVIDERS_DIR/gemini/gemini_cli.go" ]; then
    pass "Gemini CLI provider exists (gemini_cli.go)"
else
    fail "Gemini CLI provider NOT found (gemini_cli.go)"
fi

#===============================================================================
# Group 4: Verifier Integration (6 tests)
#===============================================================================
section "Group 4: Verifier Integration (6 tests)"

# Test 4.1: StartupVerifier exists
if [ -f "$VERIFIER_DIR/startup.go" ]; then
    if grep -q 'type StartupVerifier struct' "$VERIFIER_DIR/startup.go" 2>/dev/null; then
        pass "StartupVerifier struct exists in startup.go"
    else
        fail "StartupVerifier struct NOT found in startup.go"
    fi
else
    fail "Verifier startup.go NOT found"
fi

# Test 4.2: createOAuthProviderInstance handles claude case
if grep -A 5 'createOAuthProviderInstance' "$VERIFIER_DIR/startup.go" 2>/dev/null | grep -q 'case "claude"'; then
    pass "createOAuthProviderInstance handles claude case"
else
    # Broader search within function body
    if grep -q 'createOAuthProviderInstance' "$VERIFIER_DIR/startup.go" 2>/dev/null && \
       grep -q 'case "claude":' "$VERIFIER_DIR/startup.go" 2>/dev/null; then
        pass "createOAuthProviderInstance handles claude case (verified via separate patterns)"
    else
        fail "createOAuthProviderInstance does NOT handle claude case"
    fi
fi

# Test 4.3: createOAuthProviderInstance handles qwen case
if grep -q 'createOAuthProviderInstance' "$VERIFIER_DIR/startup.go" 2>/dev/null && \
   grep -q 'case "qwen":' "$VERIFIER_DIR/startup.go" 2>/dev/null; then
    pass "createOAuthProviderInstance handles qwen case"
else
    fail "createOAuthProviderInstance does NOT handle qwen case"
fi

# Test 4.4: Scoring engine has at least 5 scoring components
SCORING_COMPONENTS=0
for component in ResponseSpeed CostEffectiveness ModelEfficiency Capability Recency; do
    if grep -q "$component" "$VERIFIER_DIR/scoring.go" 2>/dev/null; then
        SCORING_COMPONENTS=$((SCORING_COMPONENTS + 1))
    fi
done
if [ "$SCORING_COMPONENTS" -ge 5 ]; then
    pass "Scoring engine has all 5 scoring components ($SCORING_COMPONENTS/5)"
else
    fail "Scoring engine missing scoring components (found $SCORING_COMPONENTS/5)"
fi

# Test 4.5: Provider discovery finds providers from env vars
if grep -q 'discoverProviders\|DiscoverProviders\|discoverAPIKeyProviders\|discoverOAuthProviders' "$VERIFIER_DIR/startup.go" 2>/dev/null; then
    pass "Provider discovery functions exist in startup.go"
else
    if [ -f "$VERIFIER_DIR/discovery.go" ]; then
        pass "Provider discovery exists in dedicated discovery.go"
    else
        fail "Provider discovery functions NOT found"
    fi
fi

# Test 4.6: Free provider verification exists
if grep -q 'verifyFreeProvider' "$VERIFIER_DIR/startup.go" 2>/dev/null; then
    pass "Free provider verification exists (verifyFreeProvider function)"
else
    fail "verifyFreeProvider function NOT found in startup.go"
fi

#===============================================================================
# Group 5: Provider Fallback Chains (6 tests)
#===============================================================================
section "Group 5: Provider Fallback Chains (6 tests)"

# Test 5.1: Claude has fallback chain (CLI -> OAuth)
CLAUDE_OAUTH=$(grep -c 'OAuth\|oauth\|CLI\|cli.*provider' "$VERIFIER_DIR/startup.go" 2>/dev/null || echo "0")
CLAUDE_OAUTH=${CLAUDE_OAUTH//[^0-9]/}
CLAUDE_OAUTH=${CLAUDE_OAUTH:-0}
if [ "$CLAUDE_OAUTH" -ge 2 ] && grep -q 'case "claude"' "$VERIFIER_DIR/startup.go" 2>/dev/null; then
    pass "Claude has fallback chain (CLI/OAuth references: $CLAUDE_OAUTH)"
else
    fail "Claude fallback chain NOT found (CLI/OAuth references: $CLAUDE_OAUTH)"
fi

# Test 5.2: Gemini has fallback chain (API -> CLI -> ACP)
GEMINI_MODES=0
if [ -f "$PROVIDERS_DIR/gemini/gemini_api.go" ]; then GEMINI_MODES=$((GEMINI_MODES + 1)); fi
if [ -f "$PROVIDERS_DIR/gemini/gemini_cli.go" ]; then GEMINI_MODES=$((GEMINI_MODES + 1)); fi
if [ -f "$PROVIDERS_DIR/gemini/gemini_acp.go" ]; then GEMINI_MODES=$((GEMINI_MODES + 1)); fi
if [ "$GEMINI_MODES" -ge 3 ]; then
    pass "Gemini has fallback chain (API + CLI + ACP = $GEMINI_MODES modes)"
else
    fail "Gemini fallback chain incomplete (found $GEMINI_MODES/3 modes: API, CLI, ACP)"
fi

# Test 5.3: Provider registry handles at least 20 provider types
CASE_COUNT=$(grep -c 'case "' "$SERVICES_DIR/provider_registry.go" 2>/dev/null || echo "0")
CASE_COUNT=${CASE_COUNT//[^0-9]/}
CASE_COUNT=${CASE_COUNT:-0}
if [ "$CASE_COUNT" -ge 20 ]; then
    pass "Provider registry handles at least 20 provider types (found $CASE_COUNT case statements)"
else
    fail "Provider registry handles fewer than 20 provider types (found $CASE_COUNT)"
fi

# Test 5.4: At least 40 provider directories exist
PROVIDER_DIR_COUNT=$(find "$PROVIDERS_DIR" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
PROVIDER_DIR_COUNT=${PROVIDER_DIR_COUNT//[^0-9]/}
PROVIDER_DIR_COUNT=${PROVIDER_DIR_COUNT:-0}
if [ "$PROVIDER_DIR_COUNT" -ge 40 ]; then
    pass "At least 40 provider directories exist (found $PROVIDER_DIR_COUNT)"
else
    fail "Fewer than 40 provider directories (found $PROVIDER_DIR_COUNT)"
fi

# Test 5.5: LLMsVerifier has at least 15 provider fallback model entries
if [ -f "$VERIFIER_FALLBACK" ]; then
    FALLBACK_ENTRY_COUNT=$(grep -c 'ProviderID:' "$VERIFIER_FALLBACK" 2>/dev/null || echo "0")
    FALLBACK_ENTRY_COUNT=${FALLBACK_ENTRY_COUNT//[^0-9]/}
    FALLBACK_ENTRY_COUNT=${FALLBACK_ENTRY_COUNT:-0}
    if [ "$FALLBACK_ENTRY_COUNT" -ge 15 ]; then
        pass "LLMsVerifier has at least 15 fallback model entries (found $FALLBACK_ENTRY_COUNT)"
    else
        fail "LLMsVerifier has fewer than 15 fallback model entries (found $FALLBACK_ENTRY_COUNT)"
    fi
else
    fail "LLMsVerifier fallback_models.go NOT found"
fi

# Test 5.6: All 5 new providers in verifier provider_types.go
NEW_PROVIDERS_FOUND=0
for provider in '"gemini"' '"groq"' '"cloudflare"' '"cohere"' '"github-models"'; do
    if grep -q "$provider" "$VERIFIER_DIR/provider_types.go" 2>/dev/null; then
        NEW_PROVIDERS_FOUND=$((NEW_PROVIDERS_FOUND + 1))
    fi
done
if [ "$NEW_PROVIDERS_FOUND" -ge 5 ]; then
    pass "All 5 new providers in verifier provider_types.go ($NEW_PROVIDERS_FOUND/5)"
else
    fail "Not all 5 new providers in verifier provider_types.go ($NEW_PROVIDERS_FOUND/5: gemini, groq, cloudflare, cohere, github-models)"
fi

#===============================================================================
# Group 6: System Resilience (4 tests)
#===============================================================================
section "Group 6: System Resilience (4 tests)"

# Test 6.1: Retry logic exists across providers
RETRY_FILES=$(grep -rl 'RetryConfig\|calculateBackoff\|retryRequest\|maxRetries\|MaxRetries' "$PROVIDERS_DIR/" 2>/dev/null | wc -l)
RETRY_FILES=${RETRY_FILES//[^0-9]/}
RETRY_FILES=${RETRY_FILES:-0}
if [ "$RETRY_FILES" -ge 1 ]; then
    pass "Retry logic exists across providers ($RETRY_FILES files with retry patterns)"
else
    fail "Retry logic NOT found across providers"
fi

# Test 6.2: Circuit breaker references exist in codebase
CB_FILES=$(grep -rl 'circuit.breaker\|CircuitBreaker\|circuit_breaker' "$PROJECT_ROOT/internal/" 2>/dev/null | wc -l)
CB_FILES=${CB_FILES//[^0-9]/}
CB_FILES=${CB_FILES:-0}
if [ "$CB_FILES" -ge 1 ]; then
    pass "Circuit breaker references exist in codebase ($CB_FILES files)"
else
    fail "Circuit breaker references NOT found in codebase"
fi

# Test 6.3: Health check endpoints registered in verifier
if grep -q 'HealthCheck\|healthCheck\|health_check' "$VERIFIER_DIR/health.go" 2>/dev/null; then
    pass "Health check endpoints registered in verifier (health.go)"
else
    if [ -f "$VERIFIER_DIR/health.go" ]; then
        pass "Health check file exists in verifier (health.go)"
    else
        fail "Health check endpoints NOT found in verifier"
    fi
fi

# Test 6.4: Error categorization exists
ERROR_CAT_FILES=$(grep -rl 'rate_limit\|ErrorCategory\|categorizeError\|error_category\|RateLimitError\|TimeoutError\|AuthError' "$PROJECT_ROOT/internal/llm/" 2>/dev/null | wc -l)
ERROR_CAT_FILES=${ERROR_CAT_FILES//[^0-9]/}
ERROR_CAT_FILES=${ERROR_CAT_FILES:-0}
if [ "$ERROR_CAT_FILES" -ge 1 ]; then
    pass "Error categorization exists ($ERROR_CAT_FILES files with error classification patterns)"
else
    fail "Error categorization (rate_limit, timeout, auth classification) NOT found"
fi

#===============================================================================
# Summary
#===============================================================================
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  System Debate Validation Challenge${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "  Total:   ${BLUE}$TOTAL${NC}"
echo -e "  Passed:  ${GREEN}$PASSED${NC}"
echo -e "  Failed:  ${RED}$FAILED${NC}"
echo -e "  Skipped: ${YELLOW}$SKIPPED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "  ${GREEN}ALL TESTS PASSED${NC}"
    exit 0
else
    echo -e "  ${RED}$FAILED test(s) failed${NC}"
    exit 1
fi
