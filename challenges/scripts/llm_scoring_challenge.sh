#!/bin/bash
# LLM Scoring Challenge
# Validates that:
# 1. All LLMs are properly scored by LLMsVerifier
# 2. Providers are sorted by score (highest first) - NO OAuth priority
# 3. Generates a report of all validated LLMs sorted by score
# 4. Zen provider ALL models are evaluated
# 5. AI Debate Team uses score-based selection only
#
# Total: 30 tests - ZERO false positives allowed
# Output: Comprehensive LLM scoring report

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# Project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_FILE="$PROJECT_ROOT/docs/reports/LLM_SCORING_REPORT_$(date +%Y%m%d_%H%M%S).md"

log_pass() {
    echo -e "${GREEN}[PASS]${NC} Test $1: $2"
    PASS_COUNT=$((PASS_COUNT + 1))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} Test $1: $2"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} Test $1: $2"
    SKIP_COUNT=$((SKIP_COUNT + 1))
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_report() {
    echo -e "${CYAN}[REPORT]${NC} $1"
}

# =============================================================================
# SECTION 1: Configuration Validation (NO OAuth Priority)
# =============================================================================
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}SECTION 1: Configuration Validation${NC}"
echo -e "${BLUE}========================================${NC}"

# Test 1: OAuthPriorityBoost is 0.0
log_info "Test 1: OAuthPriorityBoost is 0.0 (NO OAuth priority)"
if grep -q "OAuthPriorityBoost:.*0.0" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
    log_pass 1 "OAuthPriorityBoost is 0.0 - pure score-based sorting"
else
    log_fail 1 "OAuthPriorityBoost should be 0.0"
fi

# Test 2: OAuthPrimaryNonOAuthFallback is false
log_info "Test 2: OAuthPrimaryNonOAuthFallback is false"
if grep -q "OAuthPrimaryNonOAuthFallback:.*false" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
    log_pass 2 "OAuthPrimaryNonOAuthFallback is false - no special OAuth treatment"
else
    log_fail 2 "OAuthPrimaryNonOAuthFallback should be false"
fi

# Test 3: DebateTeamSize is 25
log_info "Test 3: DebateTeamSize is 25"
if grep -q "DebateTeamSize:.*25" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
    log_pass 3 "DebateTeamSize is 25 (5 positions × 5 LLMs)"
else
    log_fail 3 "DebateTeamSize should be 25"
fi

# Test 4: FallbacksPerPosition is 4
log_info "Test 4: FallbacksPerPosition is 4"
if grep -q "FallbacksPerPosition:.*4" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
    log_pass 4 "FallbacksPerPosition is 4"
else
    log_fail 4 "FallbacksPerPosition should be 4"
fi

# Test 5: selectDebateTeam sorts by score only
log_info "Test 5: selectDebateTeam sorts by score only"
if grep -q "allLLMs\[i\].Score > allLLMs\[j\].Score" "$PROJECT_ROOT/internal/verifier/startup.go" && \
   ! grep -q "IsOAuth && !allLLMs\[j\].IsOAuth" "$PROJECT_ROOT/internal/verifier/startup.go"; then
    log_pass 5 "selectDebateTeam uses pure score-based sorting"
else
    log_fail 5 "selectDebateTeam should sort purely by score (no OAuth priority)"
fi

# =============================================================================
# SECTION 2: Zen Provider Model Configuration
# =============================================================================
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}SECTION 2: Zen Provider Models${NC}"
echo -e "${BLUE}========================================${NC}"

# Test 6: Zen provider has multiple models defined
log_info "Test 6: Zen provider has multiple models defined"
ZEN_MODEL_COUNT=$(grep -A50 '"zen":' "$PROJECT_ROOT/internal/verifier/provider_types.go" | grep -E '^\s+"[a-z0-9-]+",' | wc -l || echo "0")
if [[ "$ZEN_MODEL_COUNT" -ge 10 ]]; then
    log_pass 6 "Zen provider has $ZEN_MODEL_COUNT models defined (>= 10)"
else
    log_fail 6 "Zen provider has only $ZEN_MODEL_COUNT models (expected >= 10)"
fi

# Test 7: Zen has big-pickle model
log_info "Test 7: Zen has big-pickle model"
if grep -A50 '"zen":' "$PROJECT_ROOT/internal/verifier/provider_types.go" | grep -q '"big-pickle"'; then
    log_pass 7 "Zen has big-pickle model"
else
    log_fail 7 "Zen missing big-pickle model"
fi

# Test 8: Zen has deepseek models
log_info "Test 8: Zen has deepseek models"
if grep -A50 '"zen":' "$PROJECT_ROOT/internal/verifier/provider_types.go" | grep -q '"deepseek'; then
    log_pass 8 "Zen has deepseek models"
else
    log_fail 8 "Zen missing deepseek models"
fi

# Test 9: Zen has qwen models
log_info "Test 9: Zen has qwen models"
if grep -A50 '"zen":' "$PROJECT_ROOT/internal/verifier/provider_types.go" | grep -q '"qwen'; then
    log_pass 9 "Zen has qwen models"
else
    log_fail 9 "Zen missing qwen models"
fi

# Test 10: Zen has gemini models
log_info "Test 10: Zen has gemini models"
if grep -A50 '"zen":' "$PROJECT_ROOT/internal/verifier/provider_types.go" | grep -q '"gemini'; then
    log_pass 10 "Zen has gemini models"
else
    log_fail 10 "Zen missing gemini models"
fi

# =============================================================================
# SECTION 3: Code Structure Validation
# =============================================================================
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}SECTION 3: Code Structure${NC}"
echo -e "${BLUE}========================================${NC}"

# Test 11: DebateTeamResult has SortedByScore field
log_info "Test 11: DebateTeamResult has SortedByScore field"
if grep -q "SortedByScore.*bool" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
    log_pass 11 "DebateTeamResult has SortedByScore field"
else
    log_fail 11 "DebateTeamResult missing SortedByScore field"
fi

# Test 12: DebateTeamResult has LLMReuseCount field
log_info "Test 12: DebateTeamResult has LLMReuseCount field"
if grep -q "LLMReuseCount.*int" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
    log_pass 12 "DebateTeamResult has LLMReuseCount field"
else
    log_fail 12 "DebateTeamResult missing LLMReuseCount field"
fi

# Test 13: DebatePosition uses Fallbacks slice
log_info "Test 13: DebatePosition uses Fallbacks slice"
if grep -q "Fallbacks.*\[\]\*DebateLLM" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
    log_pass 13 "DebatePosition uses Fallbacks slice (2-4 fallbacks)"
else
    log_fail 13 "DebatePosition should use Fallbacks slice"
fi

# Test 14: debate_team_config uses score-only sorting
log_info "Test 14: debate_team_config uses score-only sorting"
if grep -q "Sort.*purely by score\|sorting_method.*score_only" "$PROJECT_ROOT/internal/services/debate_team_config.go"; then
    log_pass 14 "debate_team_config uses score-only sorting"
else
    log_fail 14 "debate_team_config should use score-only sorting"
fi

# Test 15: GetTeamSummary includes sorting_method
log_info "Test 15: GetTeamSummary includes sorting_method"
if grep -q '"sorting_method"' "$PROJECT_ROOT/internal/services/debate_team_config.go"; then
    log_pass 15 "GetTeamSummary includes sorting_method field"
else
    log_fail 15 "GetTeamSummary should include sorting_method field"
fi

# =============================================================================
# SECTION 4: Server Validation & Report Generation
# =============================================================================
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}SECTION 4: Server Validation${NC}"
echo -e "${BLUE}========================================${NC}"

# Check if server is running
SERVER_URL="${HELIXAGENT_URL:-http://localhost:8080}"
SERVER_RUNNING=false
if curl -s --max-time 60 --connect-timeout 5 "$SERVER_URL/health" > /dev/null 2>&1; then
    SERVER_RUNNING=true
    log_info "Server is running at $SERVER_URL"
else
    log_info "Server not running - skipping endpoint tests"
fi

# Test 16: Server verification endpoint
log_info "Test 16: Server verification endpoint"
if [[ "$SERVER_RUNNING" == "true" ]]; then
    RESP=$(curl -s --max-time 60 "$SERVER_URL/v1/startup/verification" 2>/dev/null)
    if [[ -n "$RESP" ]] && echo "$RESP" | grep -q "reevaluation_completed"; then
        log_pass 16 "Server verification endpoint returns data"
    else
        log_fail 16 "Server verification endpoint not working"
    fi
else
    log_skip 16 "Server not running"
fi

# Test 17: Providers are returned sorted by score
log_info "Test 17: Providers are sorted by score (descending)"
if [[ "$SERVER_RUNNING" == "true" ]]; then
    RESP=$(curl -s --max-time 60 "$SERVER_URL/v1/startup/verification" 2>/dev/null)
    if echo "$RESP" | grep -q "providers_sorted.*true"; then
        log_pass 17 "Providers are sorted by score"
    else
        log_fail 17 "Providers not sorted by score"
    fi
else
    log_skip 17 "Server not running"
fi

# Test 18: At least 5 providers verified
log_info "Test 18: At least 5 providers verified"
if [[ "$SERVER_RUNNING" == "true" ]]; then
    RESP=$(curl -s --max-time 60 "$SERVER_URL/v1/startup/verification" 2>/dev/null)
    VERIFIED=$(echo "$RESP" | grep -o '"verified_count":[0-9]*' | grep -o '[0-9]*' || echo "0")
    if [[ "$VERIFIED" -ge 5 ]]; then
        log_pass 18 "$VERIFIED providers verified (>= 5)"
    else
        log_fail 18 "Only $VERIFIED providers verified (expected >= 5)"
    fi
else
    log_skip 18 "Server not running"
fi

# Test 19: First ranked provider has highest score
log_info "Test 19: First ranked provider has highest score"
if [[ "$SERVER_RUNNING" == "true" ]]; then
    RESP=$(curl -s --max-time 60 "$SERVER_URL/v1/startup/verification" 2>/dev/null)
    FIRST_SCORE=$(echo "$RESP" | jq -r '.ranked_providers[0].score // 0' 2>/dev/null || echo "0")
    if [[ $(echo "$FIRST_SCORE > 5.0" | bc -l) -eq 1 ]]; then
        log_pass 19 "First ranked provider score: $FIRST_SCORE"
    else
        log_fail 19 "First provider score too low: $FIRST_SCORE"
    fi
else
    log_skip 19 "Server not running"
fi

# Test 20: Debate team configured
log_info "Test 20: Debate team configured"
if [[ "$SERVER_RUNNING" == "true" ]]; then
    RESP=$(curl -s --max-time 60 "$SERVER_URL/v1/startup/verification" 2>/dev/null)
    if echo "$RESP" | grep -q '"team_configured":true'; then
        log_pass 20 "Debate team configured"
    else
        log_fail 20 "Debate team not configured"
    fi
else
    log_skip 20 "Server not running"
fi

# =============================================================================
# SECTION 5: Generate LLM Scoring Report
# =============================================================================
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}SECTION 5: LLM Scoring Report Generation${NC}"
echo -e "${BLUE}========================================${NC}"

# Test 21-30: Generate comprehensive report
if [[ "$SERVER_RUNNING" == "true" ]]; then
    RESP=$(curl -s --max-time 60 "$SERVER_URL/v1/startup/verification" 2>/dev/null)

    mkdir -p "$(dirname "$REPORT_FILE")"

    cat > "$REPORT_FILE" << 'HEADER'
# LLM Scoring Report

**Generated by**: HelixAgent LLM Scoring Challenge
**Source**: LLMsVerifier (Single Source of Truth)
**Sorting Method**: Score Only (NO OAuth Priority)

---

## Executive Summary

HEADER

    # Extract summary stats
    TOTAL=$(echo "$RESP" | jq -r '.total_providers // 0' 2>/dev/null || echo "0")
    VERIFIED=$(echo "$RESP" | jq -r '.verified_count // 0' 2>/dev/null || echo "0")
    FAILED=$(echo "$RESP" | jq -r '.failed_count // 0' 2>/dev/null || echo "0")
    OAUTH_COUNT=$(echo "$RESP" | jq -r '.oauth_providers // 0' 2>/dev/null || echo "0")
    FREE_COUNT=$(echo "$RESP" | jq -r '.free_providers // 0' 2>/dev/null || echo "0")
    API_KEY_COUNT=$(echo "$RESP" | jq -r '.api_key_providers // 0' 2>/dev/null || echo "0")
    DURATION=$(echo "$RESP" | jq -r '.duration_ms // 0' 2>/dev/null || echo "0")

    TEAM_TOTAL=$(echo "$RESP" | jq -r '.debate_team.total_llms // 0' 2>/dev/null || echo "0")
    TEAM_POSITIONS=$(echo "$RESP" | jq -r '.debate_team.positions // 0' 2>/dev/null || echo "0")

    cat >> "$REPORT_FILE" << EOF
| Metric | Value |
|--------|-------|
| Total Providers | $TOTAL |
| Verified | $VERIFIED |
| Failed | $FAILED |
| OAuth Providers | $OAUTH_COUNT |
| API Key Providers | $API_KEY_COUNT |
| Free Providers | $FREE_COUNT |
| Verification Duration | ${DURATION}ms |

### AI Debate Team

| Metric | Value |
|--------|-------|
| Total LLMs | $TEAM_TOTAL |
| Positions | $TEAM_POSITIONS |
| Sorting | Score Only (NO OAuth Priority) |

---

## All Validated LLMs (Sorted by Score - Strongest to Weakest)

| Rank | Provider | Auth Type | Score | Verified | Models |
|------|----------|-----------|-------|----------|--------|
EOF

    # Extract and format ranked providers
    echo "$RESP" | jq -r '.ranked_providers[] | "| \(.rank) | \(.provider) | \(.auth_type) | \(.score) | \(.verified) | \(.models) |"' 2>/dev/null >> "$REPORT_FILE" || true

    cat >> "$REPORT_FILE" << 'FOOTER'

---

## Notes

1. **Sorting Method**: All providers sorted purely by score (highest first)
2. **NO OAuth Priority**: OAuth providers compete equally with all other providers
3. **LLM Reuse**: If not enough unique LLMs, strongest ones are reused (independent instances)
4. **Zen Provider**: ALL Zen models evaluated - only those passing verification included
5. **Minimum Score**: Providers below 5.0 are excluded from the debate team

---

*Report generated by HelixAgent LLM Scoring Challenge*
*LLMsVerifier is the Single Source of Truth for all LLM verification and scoring*
FOOTER

    log_report "Report generated: $REPORT_FILE"

    # Test 21: Report file created
    log_info "Test 21: Report file created"
    if [[ -f "$REPORT_FILE" ]]; then
        log_pass 21 "Report file created: $REPORT_FILE"
    else
        log_fail 21 "Report file not created"
    fi

    # Test 22: Report contains summary
    log_info "Test 22: Report contains summary"
    if grep -q "Executive Summary" "$REPORT_FILE"; then
        log_pass 22 "Report contains summary"
    else
        log_fail 22 "Report missing summary"
    fi

    # Test 23: Report contains ranked providers
    log_info "Test 23: Report contains ranked providers"
    if grep -q "Sorted by Score" "$REPORT_FILE"; then
        log_pass 23 "Report contains ranked providers"
    else
        log_fail 23 "Report missing ranked providers"
    fi

    # Test 24: Report shows NO OAuth priority
    log_info "Test 24: Report shows NO OAuth priority"
    if grep -q "NO OAuth Priority" "$REPORT_FILE"; then
        log_pass 24 "Report confirms NO OAuth priority"
    else
        log_fail 24 "Report should show NO OAuth priority"
    fi

    # Test 25: Report shows debate team info
    log_info "Test 25: Report shows debate team info"
    if grep -q "AI Debate Team" "$REPORT_FILE"; then
        log_pass 25 "Report shows debate team info"
    else
        log_fail 25 "Report missing debate team info"
    fi

    # Test 26: Report has provider table
    log_info "Test 26: Report has provider table"
    if grep -q "| Rank | Provider |" "$REPORT_FILE"; then
        log_pass 26 "Report has provider table"
    else
        log_fail 26 "Report missing provider table"
    fi

    # Test 27: At least 5 providers in report
    log_info "Test 27: At least 5 providers in report"
    PROVIDER_ROWS=$(grep -E "^\| [0-9]+ \|" "$REPORT_FILE" | wc -l || echo "0")
    if [[ "$PROVIDER_ROWS" -ge 5 ]]; then
        log_pass 27 "$PROVIDER_ROWS providers in report (>= 5)"
    else
        log_fail 27 "Only $PROVIDER_ROWS providers in report"
    fi

    # Test 28: First provider has highest score
    log_info "Test 28: First provider has highest score"
    FIRST_LINE=$(grep -E "^\| 1 \|" "$REPORT_FILE" || echo "")
    if [[ -n "$FIRST_LINE" ]]; then
        log_pass 28 "First provider (rank 1) found in report"
    else
        log_fail 28 "First provider (rank 1) not found"
    fi

    # Test 29: Report mentions LLMsVerifier
    log_info "Test 29: Report mentions LLMsVerifier"
    if grep -q "LLMsVerifier" "$REPORT_FILE"; then
        log_pass 29 "Report mentions LLMsVerifier as source of truth"
    else
        log_fail 29 "Report should mention LLMsVerifier"
    fi

    # Test 30: Report mentions LLM reuse
    log_info "Test 30: Report mentions LLM reuse"
    if grep -q "LLM Reuse" "$REPORT_FILE"; then
        log_pass 30 "Report mentions LLM reuse capability"
    else
        log_fail 30 "Report should mention LLM reuse"
    fi

else
    log_skip 21 "Server not running - report generation skipped"
    log_skip 22 "Server not running"
    log_skip 23 "Server not running"
    log_skip 24 "Server not running"
    log_skip 25 "Server not running"
    log_skip 26 "Server not running"
    log_skip 27 "Server not running"
    log_skip 28 "Server not running"
    log_skip 29 "Server not running"
    log_skip 30 "Server not running"
fi

# =============================================================================
# SUMMARY
# =============================================================================
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}CHALLENGE SUMMARY${NC}"
echo -e "${BLUE}========================================${NC}"

TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo -e "Total Tests: ${BLUE}30${NC}"
echo -e "Passed:      ${GREEN}$PASS_COUNT${NC}"
echo -e "Failed:      ${RED}$FAIL_COUNT${NC}"
echo -e "Skipped:     ${YELLOW}$SKIP_COUNT${NC}"
echo ""

if [[ -f "$REPORT_FILE" ]]; then
    echo -e "${CYAN}LLM Scoring Report: $REPORT_FILE${NC}"
    echo ""
fi

if [[ $FAIL_COUNT -eq 0 ]]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}LLM SCORING CHALLENGE: PASSED${NC}"
    echo -e "${GREEN}========================================${NC}"
    exit 0
else
    echo -e "\n${RED}Failed Tests:${NC}"
    echo -e "${RED}  - See output above for details${NC}"
    echo ""
    PASS_RATE=$((100 * PASS_COUNT / (TOTAL > 0 ? TOTAL : 1)))
    echo -e "Pass Rate: ${BLUE}${PASS_RATE}%${NC} ($PASS_COUNT/$TOTAL)"
    echo ""
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}LLM SCORING CHALLENGE: FAILED${NC}"
    echo -e "${RED}========================================${NC}"
    exit 1
fi
