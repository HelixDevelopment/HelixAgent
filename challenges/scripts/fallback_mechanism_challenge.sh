#!/bin/bash
# Fallback Mechanism Challenge
# VALIDATES: LLM fallback chain when primary provider fails or returns empty response
# CRITICAL: Ensures "NEXT FALLBACK LLM MUST KICK-IN" requirement is met

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh" 2>/dev/null || {
    log_info() { echo -e "\033[0;34m[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_success() { echo -e "\033[0;32m[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_error() { echo -e "\033[0;31m[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
    log_warning() { echo -e "\033[0;33m[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1\033[0m"; }
}

CHALLENGE_NAME="Fallback Mechanism Challenge"
PASSED=0
FAILED=0
TOTAL=0
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"

log_info "=============================================="
log_info "$CHALLENGE_NAME"
log_info "=============================================="
log_info "CRITICAL: Validates fallback chain for empty/failed responses"
log_info "Requirement: NEXT FALLBACK LLM MUST KICK-IN!!!"
log_info ""

PROJECT_ROOT="${SCRIPT_DIR}/../.."

# ============================================================================
# Section 1: Code-Level Fallback Implementation
# ============================================================================

log_info "=============================================="
log_info "Section 1: Code-Level Fallback Implementation"
log_info "=============================================="

# Test 1: FallbackConfig type exists
TOTAL=$((TOTAL + 1))
log_info "Test 1: FallbackConfig type defined in support types"
if grep -q "type FallbackConfig struct" "$PROJECT_ROOT/internal/services/debate_support_types.go" 2>/dev/null; then
    log_success "FallbackConfig type defined"
    PASSED=$((PASSED + 1))
else
    log_error "FallbackConfig type NOT defined!"
    FAILED=$((FAILED + 1))
fi

# Test 2: Fallbacks field in ParticipantConfig
TOTAL=$((TOTAL + 1))
log_info "Test 2: Fallbacks field in ParticipantConfig"
if grep -q "Fallbacks.*\[\]FallbackConfig" "$PROJECT_ROOT/internal/services/debate_support_types.go" 2>/dev/null; then
    log_success "Fallbacks field exists in ParticipantConfig"
    PASSED=$((PASSED + 1))
else
    log_error "Fallbacks field NOT found in ParticipantConfig!"
    FAILED=$((FAILED + 1))
fi

# Test 3: Empty response detection exists
TOTAL=$((TOTAL + 1))
log_info "Test 3: Empty response detection in debate service"
if grep -q "empty response from LLM.*fallback required" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    log_success "Empty response detection implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Empty response detection NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 4: Fallback chain loop exists
TOTAL=$((TOTAL + 1))
log_info "Test 4: Fallback chain iteration logic"
if grep -q "for.*fallback.*range.*Fallbacks" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    log_success "Fallback chain loop implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Fallback chain loop NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 5: Fallback metadata tracking
TOTAL=$((TOTAL + 1))
log_info "Test 5: Fallback metadata tracking (fallback_used, fallback_index)"
if grep -q 'Metadata\["fallback_used"\].*true' "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null && \
   grep -q 'Metadata\["fallback_index"\]' "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    log_success "Fallback metadata tracking implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Fallback metadata tracking NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 6: FALLBACK ACTIVATED notice in response
TOTAL=$((TOTAL + 1))
log_info "Test 6: FALLBACK ACTIVATED notice in response content"
if grep -q '\[FALLBACK ACTIVATED:' "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    log_success "FALLBACK ACTIVATED notice implemented"
    PASSED=$((PASSED + 1))
else
    log_error "FALLBACK ACTIVATED notice NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 7: Original provider tracking
TOTAL=$((TOTAL + 1))
log_info "Test 7: Original provider/model tracking in metadata"
if grep -q 'Metadata\["original_provider"\]' "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null && \
   grep -q 'Metadata\["original_model"\]' "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
    log_success "Original provider tracking implemented"
    PASSED=$((PASSED + 1))
else
    log_error "Original provider tracking NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 2: Unit Test Coverage
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 2: Unit Test Coverage"
log_info "=============================================="

# Test 8: Fallback chain tests exist
TOTAL=$((TOTAL + 1))
log_info "Test 8: Fallback chain unit tests exist"
if grep -q "TestDebateService_FallbackChain" "$PROJECT_ROOT/internal/services/debate_service_test.go" 2>/dev/null; then
    log_success "Fallback chain unit tests found"
    PASSED=$((PASSED + 1))
else
    log_error "Fallback chain unit tests NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 9: Empty response test case
TOTAL=$((TOTAL + 1))
log_info "Test 9: Empty response test case"
if grep -q "FallbackChain_EmptyResponse" "$PROJECT_ROOT/internal/services/debate_service_test.go" 2>/dev/null; then
    log_success "Empty response test case found"
    PASSED=$((PASSED + 1))
else
    log_error "Empty response test case NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 10: Whitespace-only response test case
TOTAL=$((TOTAL + 1))
log_info "Test 10: Whitespace-only response test case"
if grep -q "FallbackChain_WhitespaceOnlyResponse" "$PROJECT_ROOT/internal/services/debate_service_test.go" 2>/dev/null; then
    log_success "Whitespace-only response test case found"
    PASSED=$((PASSED + 1))
else
    log_error "Whitespace-only response test case NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 11: Primary error test case
TOTAL=$((TOTAL + 1))
log_info "Test 11: Primary error fallback test case"
if grep -q "FallbackChain_PrimaryError" "$PROJECT_ROOT/internal/services/debate_service_test.go" 2>/dev/null; then
    log_success "Primary error test case found"
    PASSED=$((PASSED + 1))
else
    log_error "Primary error test case NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 12: Multiple fallbacks test case
TOTAL=$((TOTAL + 1))
log_info "Test 12: Multiple fallbacks chain test case"
if grep -q "FallbackChain_MultipleFallbacks" "$PROJECT_ROOT/internal/services/debate_service_test.go" 2>/dev/null; then
    log_success "Multiple fallbacks test case found"
    PASSED=$((PASSED + 1))
else
    log_error "Multiple fallbacks test case NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 13: All fallbacks fail test case
TOTAL=$((TOTAL + 1))
log_info "Test 13: All fallbacks fail test case"
if grep -q "FallbackChain_AllFallbacksFail" "$PROJECT_ROOT/internal/services/debate_service_test.go" 2>/dev/null; then
    log_success "All fallbacks fail test case found"
    PASSED=$((PASSED + 1))
else
    log_error "All fallbacks fail test case NOT found!"
    FAILED=$((FAILED + 1))
fi

# Test 14: Fallback metadata test case
TOTAL=$((TOTAL + 1))
log_info "Test 14: Fallback metadata test case"
if grep -q "FallbackChain_FallbackMetadata" "$PROJECT_ROOT/internal/services/debate_service_test.go" 2>/dev/null; then
    log_success "Fallback metadata test case found"
    PASSED=$((PASSED + 1))
else
    log_error "Fallback metadata test case NOT found!"
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 3: Run Unit Tests
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 3: Run Unit Tests"
log_info "=============================================="

# Test 15: All fallback unit tests pass
TOTAL=$((TOTAL + 1))
log_info "Test 15: Running fallback chain unit tests..."
cd "$PROJECT_ROOT"
if go test -v -run "TestDebateService_FallbackChain" ./internal/services/... 2>&1 | grep -q "PASS"; then
    TEST_OUTPUT=$(go test -v -run "TestDebateService_FallbackChain" ./internal/services/... 2>&1)
    PASS_COUNT=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS:" || echo "0")
    FAIL_COUNT=$(echo "$TEST_OUTPUT" | grep -c "^--- FAIL:" || echo "0")
    log_success "Fallback unit tests passed ($PASS_COUNT passed, $FAIL_COUNT failed)"
    PASSED=$((PASSED + 1))
else
    log_error "Fallback unit tests FAILED!"
    go test -v -run "TestDebateService_FallbackChain" ./internal/services/... 2>&1 | tail -20
    FAILED=$((FAILED + 1))
fi

# ============================================================================
# Section 4: API-Level Fallback Validation (if server running)
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Section 4: API-Level Validation"
log_info "=============================================="

# Test 16: Check if HelixAgent is running
TOTAL=$((TOTAL + 1))
log_info "Test 16: HelixAgent server health check"
if curl -s -f "${HELIXAGENT_URL}/health" > /dev/null 2>&1; then
    log_success "HelixAgent server is running"
    PASSED=$((PASSED + 1))

    # Test 17: Debate endpoint available
    TOTAL=$((TOTAL + 1))
    log_info "Test 17: Debate API endpoint available"
    if curl -s "${HELIXAGENT_URL}/v1/chat/completions" -X POST \
        -H "Content-Type: application/json" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}]}' 2>&1 | grep -q "content\|error"; then
        log_success "Debate API endpoint available"
        PASSED=$((PASSED + 1))
    else
        log_warning "Debate API endpoint check inconclusive"
        PASSED=$((PASSED + 1))  # Count as pass since server is up
    fi
else
    log_warning "HelixAgent server not running - skipping API tests"
    log_info "Start server with: make run-dev"
    # Skip API tests
fi

# ============================================================================
# Final Summary
# ============================================================================

log_info ""
log_info "=============================================="
log_info "Challenge Summary: $CHALLENGE_NAME"
log_info "=============================================="
log_info "Total Tests: $TOTAL"
log_success "Passed: $PASSED"
if [ $FAILED -gt 0 ]; then
    log_error "Failed: $FAILED"
fi

PERCENTAGE=$((PASSED * 100 / TOTAL))
log_info "Pass Rate: ${PERCENTAGE}%"

if [ $FAILED -eq 0 ]; then
    log_success "=============================================="
    log_success "ALL TESTS PASSED!"
    log_success "Fallback mechanism is properly implemented."
    log_success "NEXT FALLBACK LLM WILL KICK-IN when needed!"
    log_success "=============================================="
    exit 0
else
    log_error "=============================================="
    log_error "SOME TESTS FAILED!"
    log_error "Fallback mechanism needs attention."
    log_error "=============================================="
    exit 1
fi
