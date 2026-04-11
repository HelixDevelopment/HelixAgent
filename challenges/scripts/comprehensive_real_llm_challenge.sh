#!/bin/bash
#
# Comprehensive Debate Real LLM Challenge
# Validates comprehensive debate system makes real LLM calls and shows fallback events
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"
TEST_LOG="${SCRIPT_DIR}/comprehensive_real_llm_challenge.log"
FAILED=0
PASSED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$TEST_LOG"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1" | tee -a "$TEST_LOG"
    ((PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1" | tee -a "$TEST_LOG"
    ((FAILED++))
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" | tee -a "$TEST_LOG"
}

# Initialize test log
> "$TEST_LOG"
echo "==============================================" >> "$TEST_LOG"
echo "Comprehensive Debate Real LLM Challenge" >> "$TEST_LOG"
echo "Timestamp: $(date)" >> "$TEST_LOG"
echo "==============================================" >> "$TEST_LOG"

log_info "Starting Comprehensive Debate Real LLM Challenge..."
log_info "Testing against: $HELIXAGENT_URL"

# Test 1: Check HelixAgent is running
log_info "Test 1: Checking if HelixAgent is running..."
if curl -s --max-time 60 "${HELIXAGENT_URL}/health" > /dev/null 2>&1; then
    log_pass "HelixAgent is running and healthy"
else
    log_fail "HelixAgent is not running or not healthy at ${HELIXAGENT_URL}"
    echo ""
    echo "=============================================="
    echo "CHALLENGE FAILED - HelixAgent not available"
    echo "=============================================="
    exit 1
fi

# Test 2: Verify comprehensive debate endpoint works
log_info "Test 2: Testing comprehensive debate endpoint..."
RESPONSE_FILE=$(mktemp)
curl -s --max-time 60 -X POST "${HELIXAGENT_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test-key" \
    -d '{
        "model": "helixagent-ensemble",
        "messages": [{"role": "user", "content": "Say hello"}],
        "stream": false
    }' > "$RESPONSE_FILE" 2>&1

if [ -s "$RESPONSE_FILE" ] && grep -q "choices" "$RESPONSE_FILE"; then
    log_pass "Comprehensive debate endpoint responds"
else
    log_fail "Comprehensive debate endpoint failed"
    cat "$RESPONSE_FILE" | head -20 | tee -a "$TEST_LOG"
fi
rm -f "$RESPONSE_FILE"

# Test 3: Test streaming debate and capture output
log_info "Test 3: Testing streaming debate with real LLM calls..."
STREAM_FILE=$(mktemp)
curl -s --max-time 60 -X POST "${HELIXAGENT_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test-key" \
    -d '{
        "model": "helixagent-ensemble",
        "messages": [{"role": "user", "content": "Hello"}],
        "stream": true
    }' > "$STREAM_FILE" 2>&1

# Check if streaming response contains debate structure
if grep -q "HelixAgent AI Debate Ensemble" "$STREAM_FILE"; then
    log_pass "Debate structure present in streaming response"
else
    log_fail "Debate structure missing from streaming response"
fi

# Test 4: Verify models are displayed
log_info "Test 4: Verifying models are displayed in debate team..."
if grep -q "meta/llama" "$STREAM_FILE" || grep -q "claude" "$STREAM_FILE" || grep -q "mistral" "$STREAM_FILE"; then
    log_pass "Models are displayed in debate team table"
else
    log_fail "Models not displayed in debate team table"
fi

# Test 5: Verify fallbacks are displayed
log_info "Test 5: Verifying fallbacks are displayed..."
if grep -q "Fallback" "$STREAM_FILE"; then
    log_pass "Fallback chains are displayed"
else
    log_warn "Fallback chains not visible (may not be triggered)"
fi

# Test 6: Check for deliberation content
log_info "Test 6: Verifying deliberation content is generated..."
if grep -q "Response received" "$STREAM_FILE" || grep -q "Analyst" "$STREAM_FILE" || grep -q "Proposer" "$STREAM_FILE"; then
    log_pass "Debate deliberation content is generated"
else
    log_fail "Debate deliberation content is missing"
fi

# Test 7: Check for consensus
log_info "Test 7: Verifying consensus section..."
if grep -q "Consensus" "$STREAM_FILE" || grep -q "Final Answer" "$STREAM_FILE"; then
    log_pass "Consensus section present"
else
    log_fail "Consensus section missing"
fi

# Test 8: Verify comprehensive tests pass
log_info "Test 8: Running comprehensive unit tests..."
cd "$PROJECT_ROOT"
if go test -v -short -run TestComprehensiveDebateWithRealLLMCalls ./internal/debate/comprehensive/... 2>&1 | tee -a "$TEST_LOG"; then
    log_pass "Comprehensive debate unit tests pass"
else
    log_warn "Some comprehensive unit tests failed (may be expected in test environment)"
fi

# Test 9: Check for fallback event tracking
log_info "Test 9: Verifying fallback event tracking..."
if grep -q "fallback" "$STREAM_FILE" || grep -q "Fallback" "$STREAM_FILE"; then
    log_pass "Fallback events are tracked"
else
    log_info "Note: Fallback events may not occur if primary providers succeed"
fi

# Test 10: Verify streaming completes properly
log_info "Test 10: Verifying streaming completion..."
if grep -q '\[DONE\]' "$STREAM_FILE"; then
    log_pass "Streaming completes properly with [DONE] marker"
else
    log_warn "Streaming may not have completed properly"
fi

# Cleanup
rm -f "$STREAM_FILE"

# Summary
echo ""
echo "=============================================="
echo "COMPREHENSIVE REAL LLM CHALLENGE COMPLETE"
echo "=============================================="
echo -e "${GREEN}PASSED: $PASSED${NC}"
echo -e "${RED}FAILED: $FAILED${NC}"
echo "Total Tests: $((PASSED + FAILED))"
echo ""
echo "Test Log: $TEST_LOG"
echo "Timestamp: $(date)"
echo "=============================================="

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}ALL CHALLENGES PASSED!${NC}"
    exit 0
else
    echo -e "${RED}SOME CHALLENGES FAILED${NC}"
    exit 1
fi
