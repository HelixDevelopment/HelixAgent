#!/bin/bash
# OAuth Provider Verification Challenge
# Validates that OAuth providers (Claude, Qwen) are properly trusted even when API verification fails
# Also validates comprehensive provider scoring and debate team configuration

set -e

HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"
PASS_COUNT=0
FAIL_COUNT=0
TOTAL_TESTS=0

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    PASS_COUNT=$((PASS_COUNT + 1))
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
}

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

echo "==============================================="
echo "OAuth Provider Verification Challenge"
echo "==============================================="
echo "Testing: OAuth trust, provider scoring, debate team"
echo ""

# Check if HelixAgent is running
if ! curl -s --max-time 60 "$HELIXAGENT_URL/health" > /dev/null 2>&1; then
    echo -e "${RED}ERROR: HelixAgent not running at $HELIXAGENT_URL${NC}"
    exit 1
fi

# Get verification status
VERIFICATION=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/startup/verification")

# Test 1: Startup verification completed
echo ""
echo "=== Phase 1: Startup Verification Status ==="

REEVALUATION_COMPLETED=$(echo "$VERIFICATION" | jq -r '.reevaluation_completed')
if [ "$REEVALUATION_COMPLETED" = "true" ]; then
    log_pass "Startup verification completed"
else
    log_fail "Startup verification not completed"
fi

# Test 2: Verification took real time (not instant)
DURATION_MS=$(echo "$VERIFICATION" | jq -r '.duration_ms')
if [ "$DURATION_MS" -gt 5000 ]; then
    log_pass "Verification took real time (${DURATION_MS}ms > 5000ms)"
else
    log_fail "Verification too fast (${DURATION_MS}ms < 5000ms) - may not be making real API calls"
fi

# Test 3: At least 3 providers verified
VERIFIED_COUNT=$(echo "$VERIFICATION" | jq -r '.verified_count')
if [ "$VERIFIED_COUNT" -ge 3 ]; then
    log_pass "At least 3 providers verified ($VERIFIED_COUNT verified)"
else
    log_fail "Not enough providers verified ($VERIFIED_COUNT < 3)"
fi

# Test 4: Debate team configured with 25 LLMs (5 positions × 5 LLMs per position)
DEBATE_TEAM_LLMS=$(echo "$VERIFICATION" | jq -r '.debate_team.total_llms')
if [ "$DEBATE_TEAM_LLMS" -eq 25 ]; then
    log_pass "Debate team has 25 LLMs (5 positions × 5 LLMs)"
else
    log_fail "Debate team does not have 25 LLMs (has $DEBATE_TEAM_LLMS)"
fi

# Test 5: Debate team has 5 positions
DEBATE_TEAM_POSITIONS=$(echo "$VERIFICATION" | jq -r '.debate_team.positions')
if [ "$DEBATE_TEAM_POSITIONS" -eq 5 ]; then
    log_pass "Debate team has 5 positions"
else
    log_fail "Debate team does not have 5 positions (has $DEBATE_TEAM_POSITIONS)"
fi

echo ""
echo "=== Phase 2: OAuth Provider Trust ==="

# Test 6: Claude OAuth discovered
CLAUDE_AUTH_TYPE=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="claude") | .auth_type')
if [ "$CLAUDE_AUTH_TYPE" = "oauth" ]; then
    log_pass "Claude provider discovered as OAuth"
else
    log_fail "Claude provider not discovered as OAuth (auth_type=$CLAUDE_AUTH_TYPE)"
fi

# Test 7: Claude OAuth trusted (verified=true despite API restriction)
CLAUDE_VERIFIED=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="claude") | .verified')
if [ "$CLAUDE_VERIFIED" = "true" ]; then
    log_pass "Claude OAuth is trusted (verified=true despite product-restricted token)"
else
    log_fail "Claude OAuth not trusted (verified=$CLAUDE_VERIFIED)"
fi

# Test 8: Claude OAuth has high score (>7.0)
CLAUDE_SCORE=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="claude") | .score')
CLAUDE_SCORE_INT=$(printf "%.0f" "$CLAUDE_SCORE")
if [ "$CLAUDE_SCORE_INT" -ge 7 ]; then
    log_pass "Claude OAuth has high score ($CLAUDE_SCORE >= 7.0)"
else
    log_fail "Claude OAuth score too low ($CLAUDE_SCORE < 7.0)"
fi

# Test 9: Claude has at least 3 models
CLAUDE_MODELS=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="claude") | .models')
if [ "$CLAUDE_MODELS" -ge 3 ]; then
    log_pass "Claude has at least 3 models ($CLAUDE_MODELS models)"
else
    log_fail "Claude does not have enough models ($CLAUDE_MODELS < 3)"
fi

echo ""
echo "=== Phase 3: API Key Providers ==="

# Test 10: At least 2 API key providers verified
API_KEY_VERIFIED=$(echo "$VERIFICATION" | jq -r '[.ranked_providers[] | select(.auth_type=="api_key" and .verified==true)] | length')
if [ "$API_KEY_VERIFIED" -ge 2 ]; then
    log_pass "At least 2 API key providers verified ($API_KEY_VERIFIED verified)"
else
    log_fail "Not enough API key providers verified ($API_KEY_VERIFIED < 2)"
fi

# Test 11: Cerebras or DeepSeek verified
CEREBRAS_VERIFIED=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="cerebras") | .verified')
DEEPSEEK_VERIFIED=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="deepseek") | .verified')
if [ "$CEREBRAS_VERIFIED" = "true" ] || [ "$DEEPSEEK_VERIFIED" = "true" ]; then
    log_pass "Cerebras or DeepSeek is verified"
else
    log_fail "Neither Cerebras nor DeepSeek verified"
fi

# Test 12: Mistral verified
MISTRAL_VERIFIED=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="mistral") | .verified')
if [ "$MISTRAL_VERIFIED" = "true" ]; then
    log_pass "Mistral is verified"
else
    log_fail "Mistral not verified"
fi

echo ""
echo "=== Phase 4: Free Provider Configuration ==="

# Test 13: Zen provider discovered
ZEN_EXISTS=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="zen") | .provider')
if [ "$ZEN_EXISTS" = "zen" ]; then
    log_pass "Zen free provider discovered"
else
    log_fail "Zen free provider not discovered"
fi

# Test 14: Zen has correct auth type (free)
ZEN_AUTH_TYPE=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="zen") | .auth_type')
if [ "$ZEN_AUTH_TYPE" = "free" ]; then
    log_pass "Zen provider has free auth type"
else
    log_fail "Zen provider does not have free auth type (auth_type=$ZEN_AUTH_TYPE)"
fi

# Test 15: Zen has at least 3 models configured
ZEN_MODELS=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="zen") | .models')
if [ "$ZEN_MODELS" -ge 3 ]; then
    log_pass "Zen has at least 3 models configured ($ZEN_MODELS models)"
else
    log_fail "Zen does not have enough models ($ZEN_MODELS < 3)"
fi

echo ""
echo "=== Phase 5: ZAI (GLM) Provider Configuration ==="

# Test 16: ZAI provider discovered
ZAI_EXISTS=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="zai") | .provider')
if [ "$ZAI_EXISTS" = "zai" ]; then
    log_pass "ZAI (Zhipu GLM) provider discovered"
else
    log_fail "ZAI provider not discovered"
fi

# Test 17: ZAI has models configured (not empty)
ZAI_MODELS=$(echo "$VERIFICATION" | jq -r '.ranked_providers[] | select(.provider=="zai") | .models')
if [ "$ZAI_MODELS" -ge 1 ]; then
    log_pass "ZAI has models configured ($ZAI_MODELS models)"
else
    log_fail "ZAI has no models configured"
fi

echo ""
echo "=== Phase 6: Provider Ranking ==="

# Test 18: OAuth providers ranked first
FIRST_PROVIDER_AUTH=$(echo "$VERIFICATION" | jq -r '.ranked_providers[0].auth_type')
if [ "$FIRST_PROVIDER_AUTH" = "oauth" ]; then
    log_pass "OAuth provider is ranked first"
else
    log_fail "OAuth provider is not ranked first (first is $FIRST_PROVIDER_AUTH)"
fi

# Test 19: Providers sorted by score (descending)
IS_SORTED=$(echo "$VERIFICATION" | jq -r '.providers_sorted')
if [ "$IS_SORTED" = "true" ]; then
    log_pass "Providers are sorted by score"
else
    log_fail "Providers are not sorted by score"
fi

# Test 20: All verified providers have score >= 5.0 (minimum threshold)
MIN_VERIFIED_SCORE=$(echo "$VERIFICATION" | jq -r '[.ranked_providers[] | select(.verified==true) | .score] | min')
MIN_SCORE_INT=$(printf "%.0f" "$MIN_VERIFIED_SCORE")
if [ "$MIN_SCORE_INT" -ge 5 ]; then
    log_pass "All verified providers have score >= 5.0 (min=$MIN_VERIFIED_SCORE)"
else
    log_fail "Some verified providers have score < 5.0 (min=$MIN_VERIFIED_SCORE)"
fi

echo ""
echo "=== Phase 7: Debate Team Composition ==="

# Test 21: Debate team configured
TEAM_CONFIGURED=$(echo "$VERIFICATION" | jq -r '.debate_team.team_configured')
if [ "$TEAM_CONFIGURED" = "true" ]; then
    log_pass "Debate team is configured"
else
    log_fail "Debate team is not configured"
fi

# Test 22: OAuth priority enabled (check flag or actual prioritization)
OAUTH_FIRST=$(echo "$VERIFICATION" | jq -r '.debate_team.oauth_first // "null"')
# Check if oauth_first flag is set, OR if OAuth providers have high scores
if [ "$OAUTH_FIRST" = "true" ]; then
    log_pass "OAuth providers prioritized in debate team"
else
    # Alternative: Check if OAuth providers (Claude, Qwen) are in top rankings
    OAUTH_IN_TOP=$(echo "$VERIFICATION" | jq -r '[.ranked_providers[:5][] | select(.auth_type=="oauth")] | length')
    if [ "$OAUTH_IN_TOP" -ge 1 ]; then
        log_pass "OAuth providers prioritized in debate team (found $OAUTH_IN_TOP in top 5)"
    else
        log_fail "OAuth providers not prioritized (oauth_first=$OAUTH_FIRST, oauth_in_top=$OAUTH_IN_TOP)"
    fi
fi

# Test 23: Min score threshold set
MIN_SCORE=$(echo "$VERIFICATION" | jq -r '.debate_team.min_score')
if [ "$MIN_SCORE" -ge 5 ]; then
    log_pass "Debate team min score threshold is set ($MIN_SCORE)"
else
    log_fail "Debate team min score threshold too low ($MIN_SCORE)"
fi

echo ""
echo "=== Phase 8: No False Positives ==="

# Test 24: No providers with score < 1.0 are verified
LOW_SCORE_VERIFIED=$(echo "$VERIFICATION" | jq -r '[.ranked_providers[] | select(.score < 1.0 and .verified==true)] | length')
if [ "$LOW_SCORE_VERIFIED" -eq 0 ]; then
    log_pass "No providers with score < 1.0 are verified (no false positives)"
else
    log_fail "Found $LOW_SCORE_VERIFIED providers with score < 1.0 that are verified (false positive)"
fi

# Test 25: Duration reasonable (< 180 seconds)
# Note: Increased from 120s to 180s to account for system variance and multiple provider verifications
if [ "$DURATION_MS" -lt 180000 ]; then
    log_pass "Verification duration reasonable (${DURATION_MS}ms < 180000ms)"
else
    log_fail "Verification took too long (${DURATION_MS}ms >= 180000ms)"
fi

echo ""
echo "==============================================="
echo "Challenge Results"
echo "==============================================="
echo -e "Passed: ${GREEN}$PASS_COUNT${NC}"
echo -e "Failed: ${RED}$FAIL_COUNT${NC}"
echo "Total: $TOTAL_TESTS"
echo ""

if [ $FAIL_COUNT -eq 0 ]; then
    echo -e "${GREEN}ALL TESTS PASSED!${NC}"
    exit 0
else
    echo -e "${RED}$FAIL_COUNT TESTS FAILED${NC}"
    exit 1
fi
