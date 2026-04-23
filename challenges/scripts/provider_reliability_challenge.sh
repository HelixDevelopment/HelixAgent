#!/bin/bash
#===============================================================================
# HELIXAGENT PROVIDER RELIABILITY CHALLENGE
#===============================================================================
# This challenge validates that the LLM provider system is reliable and does not
# return empty responses under normal operating conditions.
#
# Created after an incident where the OpenCode challenge failed because 23/25
# CLI tests returned empty responses after the first 2 requests succeeded.
#
# Tests:
#   1. Consecutive requests - API handles 10+ requests without empty responses
#   2. Rapid requests - Parallel requests are handled correctly
#   3. Circuit breaker recovery - System recovers after provider failures
#   4. Non-empty responses - All response types return content
#   5. Response times - Responses are timely (< 60s)
#
# Usage:
#   ./challenges/scripts/provider_reliability_challenge.sh [options]
#
# Options:
#   --skip-if-offline    Skip tests if HelixAgent is not running
#   --verbose            Enable verbose output
#
#===============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
RESULTS_DIR="$PROJECT_ROOT/challenges/results/provider_reliability_challenge"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Parse arguments
SKIP_OFFLINE=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-if-offline)
            SKIP_OFFLINE=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

log() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log_info() {
    log "${BLUE}[INFO]${NC} $*"
}

log_success() {
    log "${GREEN}[SUCCESS]${NC} $*"
}

log_error() {
    log "${RED}[ERROR]${NC} $*"
}

log_warning() {
    log "${YELLOW}[WARNING]${NC} $*"
}

#===============================================================================
# MAIN EXECUTION
#===============================================================================

log_info "${PURPLE}========================================${NC}"
log_info "${PURPLE}  PROVIDER RELIABILITY CHALLENGE${NC}"
log_info "${PURPLE}========================================${NC}"
echo ""

# Check if HelixAgent is running
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"

log_info "Checking HelixAgent at $HELIXAGENT_URL..."

if ! curl -s --max-time 60 "$HELIXAGENT_URL/health" > /dev/null 2>&1; then
    if [ "$SKIP_OFFLINE" = true ]; then
        log_warning "HelixAgent is not running - skipping challenge (--skip-if-offline)"
        exit 0
    else
        log_error "HelixAgent is not running!"
        log_info "Start HelixAgent with: ./bin/helixagent"
        exit 1
    fi
fi

log_success "HelixAgent is running"

# Create results directory
mkdir -p "$RESULTS_DIR/$TIMESTAMP"
RESULTS_FILE="$RESULTS_DIR/$TIMESTAMP/results.txt"
SUMMARY_FILE="$RESULTS_DIR/$TIMESTAMP/summary.json"

# Run the Go tests
log_info "Running provider reliability tests..."

cd "$PROJECT_ROOT"

TEST_OUTPUT=$(go test -v -timeout 300s ./tests/integration/provider_reliability_test.go 2>&1)
TEST_EXIT_CODE=$?

echo "$TEST_OUTPUT" > "$RESULTS_FILE"

# Parse test results
TOTAL_TESTS=$(echo "$TEST_OUTPUT" | grep -c "^=== RUN" || echo "0")
PASSED_TESTS=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS" || echo "0")
FAILED_TESTS=$(echo "$TEST_OUTPUT" | grep -c "^--- FAIL" || echo "0")

# Extract specific test results - look for "--- PASS: TestName" pattern
CONSECUTIVE_PASS=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS: TestProviderReliability_ConsecutiveRequests" || echo "0")
RAPID_PASS=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS: TestProviderReliability_RapidRequests" || echo "0")
CIRCUIT_PASS=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS: TestProviderReliability_CircuitBreakerRecovery" || echo "0")
NONEMPTY_PASS=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS: TestAPIResponse_NonEmpty " || echo "0")
RESPONSE_TIME_PASS=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS: TestAPIResponse_ResponseTime" || echo "0")

# Display results
echo ""
log_info "=========================================="
log_info "  TEST RESULTS"
log_info "=========================================="
echo ""

# Check for consecutive request success rate
CONSECUTIVE_SUCCESS=$(echo "$TEST_OUTPUT" | grep "Final Results:" | grep -oE '[0-9]+/[0-9]+' | cut -d'/' -f1 || echo "0")
CONSECUTIVE_TOTAL=$(echo "$TEST_OUTPUT" | grep "Final Results:" | grep -oE '[0-9]+/[0-9]+' | cut -d'/' -f2 || echo "10")

if [ "$CONSECUTIVE_PASS" -gt 0 ]; then
    log_success "Consecutive Requests: PASSED ($CONSECUTIVE_SUCCESS/$CONSECUTIVE_TOTAL)"
else
    log_error "Consecutive Requests: FAILED"
fi

if [ "$RAPID_PASS" -gt 0 ]; then
    log_success "Rapid Requests: PASSED"
else
    log_error "Rapid Requests: FAILED"
fi

if [ "$CIRCUIT_PASS" -gt 0 ]; then
    log_success "Circuit Breaker Recovery: PASSED"
else
    log_error "Circuit Breaker Recovery: FAILED"
fi

if [ "$NONEMPTY_PASS" -gt 0 ]; then
    log_success "Non-Empty Responses: PASSED"
else
    log_error "Non-Empty Responses: FAILED"
fi

if [ "$RESPONSE_TIME_PASS" -gt 0 ]; then
    log_success "Response Times: PASSED"
else
    log_error "Response Times: FAILED"
fi

echo ""
log_info "Total Tests: $TOTAL_TESTS"
log_info "Passed: ${GREEN}$PASSED_TESTS${NC}"
if [ "${FAILED_TESTS:-0}" -gt 0 ] 2>/dev/null; then
    log_info "Failed: ${RED}$FAILED_TESTS${NC}"
else
    log_info "Failed: 0"
fi
echo ""

# Generate summary JSON
cat > "$SUMMARY_FILE" << EOF
{
    "challenge": "Provider Reliability",
    "timestamp": "$(date -Iseconds)",
    "helixagent_url": "$HELIXAGENT_URL",
    "tests": {
        "total": $TOTAL_TESTS,
        "passed": $PASSED_TESTS,
        "failed": $FAILED_TESTS
    },
    "results": {
        "consecutive_requests": $([ "$CONSECUTIVE_PASS" -gt 0 ] && echo "true" || echo "false"),
        "rapid_requests": $([ "$RAPID_PASS" -gt 0 ] && echo "true" || echo "false"),
        "circuit_breaker_recovery": $([ "$CIRCUIT_PASS" -gt 0 ] && echo "true" || echo "false"),
        "non_empty_responses": $([ "$NONEMPTY_PASS" -gt 0 ] && echo "true" || echo "false"),
        "response_times": $([ "$RESPONSE_TIME_PASS" -gt 0 ] && echo "true" || echo "false")
    },
    "overall": $([ $TEST_EXIT_CODE -eq 0 ] && echo "true" || echo "false"),
    "results_file": "$RESULTS_FILE"
}
EOF

log_info "Results saved to: $RESULTS_DIR/$TIMESTAMP/"

# Final verdict
echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
    log_success "=========================================="
    log_success "  PROVIDER RELIABILITY CHALLENGE: PASSED"
    log_success "=========================================="
    exit 0
else
    log_error "=========================================="
    log_error "  PROVIDER RELIABILITY CHALLENGE: FAILED"
    log_error "=========================================="

    if [ "$VERBOSE" = true ]; then
        echo ""
        log_info "Full test output:"
        echo "$TEST_OUTPUT"
    fi

    exit 1
fi
