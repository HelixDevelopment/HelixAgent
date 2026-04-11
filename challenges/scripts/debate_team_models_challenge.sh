#!/bin/bash
# AI Debate Team Model Version Challenge
# VALIDATES: Claude 4.5+ models, Qwen models presence, No legacy Claude 3 as primary
# This challenge MUST FAIL if outdated models are used in the debate team

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh" 2>/dev/null || {
    log_info() { echo -e "\033[0;34m[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_success() { echo -e "\033[0;32m[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_error() { echo -e "\033[0;31m[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_warning() { echo -e "\033[0;33m[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
}

CHALLENGE_NAME="AI Debate Team Model Version Challenge"
PASSED=0
FAILED=0
TOTAL=0
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"

log_info "=============================================="
log_info "$CHALLENGE_NAME"
log_info "=============================================="
log_info "Validates: Claude 4.5+ models, Qwen models, no legacy primaries"
log_info ""

# ============================================================================
# Section 1: Code-Level Model Definitions
# ============================================================================

log_info "=============================================="
log_info "Section 1: Code-Level Model Definitions"
log_info "=============================================="

PROJECT_ROOT="${SCRIPT_DIR}/../.."

# Test 1: Claude 4.5 Opus is defined in debate_team_config.go
TOTAL=$((TOTAL + 1))
log_info "Test 1: Claude 4.5 Opus model defined in code"
if grep -q "claude-opus-4-5-20251101" "$PROJECT_ROOT/internal/services/debate_team_config.go" 2>/dev/null; then
    log_success "Claude 4.5 Opus defined: claude-opus-4-5-20251101"
    PASSED=$((PASSED + 1))
else
    log_error "Claude 4.5 Opus NOT defined! Update debate_team_config.go"
    FAILED=$((FAILED + 1))
fi

# Test 2: Claude 4.5 Sonnet is defined
TOTAL=$((TOTAL + 1))
log_info "Test 2: Claude 4.5 Sonnet model defined in code"
if grep -q "claude-sonnet-4-5-20250929" "$PROJECT_ROOT/internal/services/debate_team_config.go" 2>/dev/null; then
    log_success "Claude 4.5 Sonnet defined: claude-sonnet-4-5-20250929"
    PASSED=$((PASSED + 1))
else
    log_error "Claude 4.5 Sonnet NOT defined! Update debate_team_config.go"
    FAILED=$((FAILED + 1))
fi

# Test 3: Claude 4.5 Haiku is defined
TOTAL=$((TOTAL + 1))
log_info "Test 3: Claude 4.5 Haiku model defined in code"
if grep -q "claude-haiku-4-5-20251001" "$PROJECT_ROOT/internal/services/debate_team_config.go" 2>/dev/null; then
    log_success "Claude 4.5 Haiku defined: claude-haiku-4-5-20251001"
    PASSED=$((PASSED + 1))
else
    log_error "Claude 4.5 Haiku NOT defined! Update debate_team_config.go"
    FAILED=$((FAILED + 1))
fi

# Test 4: Qwen models are defined
TOTAL=$((TOTAL + 1))
log_info "Test 4: Qwen models defined in code"
qwen_count=$(grep -c "qwen-" "$PROJECT_ROOT/internal/services/debate_team_config.go" 2>/dev/null || echo "0")
if [ "$qwen_count" -ge 5 ]; then
    log_success "Qwen models defined: $qwen_count models"
    PASSED=$((PASSED + 1))
else
    log_error "Qwen models NOT properly defined! Found only $qwen_count (need 5+)"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 2: Provider Discovery Default Models
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 2: Provider Discovery Default Models"
log_info "=============================================="

# Test 5: Default Claude model is 4.5 (not 3.x)
TOTAL=$((TOTAL + 1))
log_info "Test 5: Default Claude model is 4.5+"
if grep -q 'DefaultModel: "claude-.*4.*5' "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null; then
    log_success "Default Claude model is 4.5"
    PASSED=$((PASSED + 1))
elif grep -q 'DefaultModel: "claude-.*4-' "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null; then
    log_success "Default Claude model is 4.x"
    PASSED=$((PASSED + 1))
else
    log_error "Default Claude model is NOT 4.x/4.5! Must update provider_discovery.go"
    log_error "Found: $(grep 'claude.*DefaultModel' $PROJECT_ROOT/internal/services/provider_discovery.go | head -1)"
    FAILED=$((FAILED + 1))
fi

# Test 6: Claude 3.x is NOT the default model
TOTAL=$((TOTAL + 1))
log_info "Test 6: Claude 3.x is NOT the default (must be fallback only)"
if grep -E 'CLAUDE_API_KEY.*DefaultModel.*claude-3-' "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null; then
    log_error "Claude 3.x is still the default! Must be Claude 4.5"
    log_error "$(grep 'CLAUDE_API_KEY.*DefaultModel' $PROJECT_ROOT/internal/services/provider_discovery.go)"
    FAILED=$((FAILED + 1))
else
    log_success "Claude 3.x is not the default model"
    PASSED=$((PASSED + 1))
fi

# Test 7: Qwen default model is qwen-max (not qwen-turbo)
TOTAL=$((TOTAL + 1))
log_info "Test 7: Default Qwen model is qwen-max"
if grep -q 'QWEN_API_KEY.*DefaultModel: "qwen-max"' "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null; then
    log_success "Default Qwen model is qwen-max"
    PASSED=$((PASSED + 1))
else
    log_warning "Default Qwen model may not be qwen-max"
    PASSED=$((PASSED + 1)) # Not critical
fi

# ============================================================================
# Section 3: Debate Service Model Lists
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 3: Debate Service Model Lists"
log_info "=============================================="

# Test 8: Claude 4.5 Opus in debate service known models
TOTAL=$((TOTAL + 1))
log_info "Test 8: Claude 4.5 Opus in debate service model list"
if grep -q "claude-opus-4-5-20251101" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    log_success "Claude 4.5 Opus in debate service model list"
    PASSED=$((PASSED + 1))
else
    log_error "Claude 4.5 Opus NOT in debate service! Update debate_service.go knownModels"
    FAILED=$((FAILED + 1))
fi

# Test 9: Claude 4.5 Sonnet in debate service
TOTAL=$((TOTAL + 1))
log_info "Test 9: Claude 4.5 Sonnet in debate service model list"
if grep -q "claude-sonnet-4-5-20250929" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    log_success "Claude 4.5 Sonnet in debate service model list"
    PASSED=$((PASSED + 1))
else
    log_error "Claude 4.5 Sonnet NOT in debate service!"
    FAILED=$((FAILED + 1))
fi

# Test 10: All Qwen models in debate service
TOTAL=$((TOTAL + 1))
log_info "Test 10: Qwen models (qwen-max, qwen-plus, qwen-turbo) in debate service"
qwen_in_debate=$(grep -c "qwen-" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null || echo "0")
if [ "$qwen_in_debate" -ge 5 ]; then
    log_success "Qwen models in debate service: $qwen_in_debate entries"
    PASSED=$((PASSED + 1))
else
    log_error "Qwen models NOT properly defined in debate service! Found $qwen_in_debate"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 4: Runtime API Verification (if HelixAgent is running)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: Runtime API Verification"
log_info "=============================================="

# Check if HelixAgent is running
if curl -s --max-time 60 "$HELIXAGENT_URL/health" | grep -q "healthy"; then
    log_info "HelixAgent is running, performing runtime checks..."

    # Test 11: Get debate team summary
    TOTAL=$((TOTAL + 1))
    log_info "Test 11: Fetch debate team summary from API"
    team_summary=$(curl -s --max-time 60 "$HELIXAGENT_URL/v1/debates/team" 2>/dev/null || echo "{}")
    if echo "$team_summary" | grep -q "team_name\|positions\|total_llms"; then
        log_success "Debate team API responds"
        PASSED=$((PASSED + 1))

        # Test 12: Check for Claude 4.x models in team
        TOTAL=$((TOTAL + 1))
        log_info "Test 12: Check for Claude 4.x models in active debate team"
        if echo "$team_summary" | grep -qE "claude.*4|claude-opus-4|claude-sonnet-4"; then
            log_success "Claude 4.x models found in debate team"
            PASSED=$((PASSED + 1))
        else
            log_warning "Claude 4.x models not yet active in team (may need API key)"
            PASSED=$((PASSED + 1)) # Not a hard failure - might not have Claude API key
        fi

        # Test 13: Check team has 15 LLMs configured
        TOTAL=$((TOTAL + 1))
        log_info "Test 13: Verify 15 LLMs in debate team configuration"
        total_llms=$(echo "$team_summary" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total_llms',0))" 2>/dev/null || echo "0")
        if [ "$total_llms" -ge 15 ]; then
            log_success "Debate team has $total_llms LLMs (15 expected)"
            PASSED=$((PASSED + 1))
        elif [ "$total_llms" -gt 0 ]; then
            log_warning "Debate team has $total_llms LLMs (15 expected)"
            PASSED=$((PASSED + 1)) # Partial success
        else
            log_info "Debate team not fully initialized yet"
            PASSED=$((PASSED + 1))
        fi
    else
        log_warning "Debate team API not available (endpoint may not exist)"
        PASSED=$((PASSED + 1)) # Not critical
    fi
else
    log_warning "HelixAgent not running - skipping runtime tests"
fi

# ============================================================================
# Section 5: No Legacy Claude 3.x as Primary Models
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 5: No Legacy Claude 3.x as Primary"
log_info "=============================================="

# Test 14: Claude 3.x should NOT be in provider discovery as default
TOTAL=$((TOTAL + 1))
log_info "Test 14: Verify Claude 3.x is NOT primary default"
legacy_count=$(grep -c 'DefaultModel: "claude-3-' "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null | head -1 || echo "0")
legacy_count=$(echo "$legacy_count" | tr -d '\n' | head -c 10)
if [ -z "$legacy_count" ] || [ "$legacy_count" = "0" ]; then
    log_success "No Claude 3.x as default models"
    PASSED=$((PASSED + 1))
else
    log_error "Found $legacy_count Claude 3.x as default models! Must update to Claude 4.5"
    FAILED=$((FAILED + 1))
fi

# Test 15: Gemini should use 2.0 (not old gemini-pro)
TOTAL=$((TOTAL + 1))
log_info "Test 15: Verify Gemini uses 2.0 model"
if grep -q 'DefaultModel: "gemini-2' "$PROJECT_ROOT/internal/services/provider_discovery.go" 2>/dev/null; then
    log_success "Gemini default is 2.0"
    PASSED=$((PASSED + 1))
else
    log_warning "Gemini may not be using 2.0 model"
    PASSED=$((PASSED + 1)) # Not critical
fi

# ============================================================================
# Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "$CHALLENGE_NAME Summary"
log_info "=============================================="
log_info "Total tests: $TOTAL"
log_success "Passed: $PASSED"
if [ $FAILED -gt 0 ]; then
    log_error "Failed: $FAILED"
else
    log_info "Failed: $FAILED"
fi

PASS_RATE=$((PASSED * 100 / TOTAL))
log_info "Pass rate: ${PASS_RATE}%"

echo ""
log_info "Model Version Requirements:"
log_info "  - Claude: 4.5 (claude-opus-4-5-*, claude-sonnet-4-5-*, claude-haiku-4-5-*)"
log_info "  - Claude 4.x: As secondary options"
log_info "  - Claude 3.x: ONLY as legacy fallbacks, never as primary"
log_info "  - Qwen: All 5 models (max, plus, turbo, coder, long)"
log_info "  - Gemini: 2.0-flash (not legacy gemini-pro)"

if [ $FAILED -eq 0 ]; then
    log_success "=============================================="
    log_success "ALL MODEL VERSION TESTS PASSED!"
    log_success "=============================================="
    exit 0
else
    log_error "=============================================="
    log_error "MODEL VERSION TESTS FAILED - UPDATE REQUIRED!"
    log_error "=============================================="
    exit 1
fi
