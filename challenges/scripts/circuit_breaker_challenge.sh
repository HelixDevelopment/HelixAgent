#!/bin/bash
# Circuit Breaker Resilience Challenge
# Tests that the system handles provider failures gracefully using circuit breakers

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}  Circuit Breaker Resilience Challenge      ${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

pass_test() {
    local name="$1"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}[PASS]${NC} $name"
}

fail_test() {
    local name="$1"
    local reason="$2"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    FAILED_TESTS=$((FAILED_TESTS + 1))
    echo -e "${RED}[FAIL]${NC} $name"
    if [ -n "$reason" ]; then
        echo -e "       ${RED}Reason: $reason${NC}"
    fi
}

# Test 1: Verify CircuitBreaker struct exists in codebase
test_circuit_breaker_struct() {
    log_info "Test 1: CircuitBreaker struct exists"

    if grep -q "type CircuitBreaker struct" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
        pass_test "CircuitBreaker struct defined"
    else
        fail_test "CircuitBreaker struct not found"
    fi
}

# Test 2: Verify circuit breaker states are defined
test_circuit_breaker_states() {
    log_info "Test 2: CircuitBreaker states defined"

    local has_closed=false
    local has_open=false
    local has_half_open=false

    if grep -q "StateClosed" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
        has_closed=true
    fi

    if grep -q "StateOpen" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
        has_open=true
    fi

    if grep -q "StateHalfOpen" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
        has_half_open=true
    fi

    if [ "$has_closed" = true ] && [ "$has_open" = true ] && [ "$has_half_open" = true ]; then
        pass_test "All circuit breaker states defined (Closed, Open, HalfOpen)"
    else
        fail_test "Missing circuit breaker states" "closed=$has_closed, open=$has_open, half_open=$has_half_open"
    fi
}

# Test 3: Verify NewCircuitBreaker function exists
test_circuit_breaker_constructor() {
    log_info "Test 3: CircuitBreaker constructor exists"

    if grep -q "func NewCircuitBreaker" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
        pass_test "NewCircuitBreaker function defined"
    else
        fail_test "NewCircuitBreaker function not found"
    fi
}

# Test 4: Verify circuit breaker has Call method
test_circuit_breaker_execute() {
    log_info "Test 4: CircuitBreaker Call method exists"

    if grep -q "func (cb \*CircuitBreaker) Call" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
        pass_test "CircuitBreaker.Call method defined"
    else
        # Also check for Execute
        if grep -q "func (cb \*CircuitBreaker) Execute" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
            pass_test "CircuitBreaker.Execute method defined"
        else
            fail_test "CircuitBreaker call method not found"
        fi
    fi
}

# Test 5: Verify circuit breaker configuration in provider registry
test_circuit_breaker_config() {
    log_info "Test 5: CircuitBreaker configuration in registry"

    if grep -q "CircuitBreakerConfig\|circuitBreaker" "$PROJECT_ROOT/internal/services/provider_registry.go"; then
        pass_test "CircuitBreaker configuration in provider registry"
    else
        fail_test "CircuitBreaker configuration not found in registry"
    fi
}

# Test 6: Verify fallback mechanism for failed providers
test_fallback_mechanism() {
    log_info "Test 6: Fallback mechanism for provider failures"

    local has_fallback=false

    if grep -qE "trying fallback|FallbackProvider|fallback provider" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
        has_fallback=true
    fi

    if grep -qE "trying fallback" "$PROJECT_ROOT/internal/services/ensemble.go" 2>/dev/null; then
        has_fallback=true
    fi

    # Also check in handlers where AI Debate actually lives
    if grep -qE "trying fallback|fallback provider" "$PROJECT_ROOT/internal/handlers/openai_compatible.go" 2>/dev/null; then
        has_fallback=true
    fi

    if [ "$has_fallback" = true ]; then
        pass_test "Fallback mechanism implemented"
    else
        fail_test "Fallback mechanism not found"
    fi
}

# Test 7: Run unit tests for circuit breaker
test_circuit_breaker_unit_tests() {
    log_info "Test 7: CircuitBreaker unit tests"

    cd "$PROJECT_ROOT"

    if go test -v -run "Circuit" ./internal/services/... 2>&1 | tail -10 | grep -q "PASS\|ok"; then
        pass_test "CircuitBreaker unit tests pass"
    else
        fail_test "CircuitBreaker unit tests failed"
    fi
}

# Test 8: Verify error logging for circuit breaker events
test_circuit_breaker_logging() {
    log_info "Test 8: CircuitBreaker error logging"

    local has_logging=false

    if grep -qE "circuit.*breaker.*open|circuit breaker is open" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
        has_logging=true
    fi

    if grep -qE "circuit.*breaker.*open|circuit breaker is open" "$PROJECT_ROOT/internal/services/ensemble.go" 2>/dev/null; then
        has_logging=true
    fi

    # Check in plugin system where circuit breaker is defined
    if grep -qE "circuit breaker is open" "$PROJECT_ROOT/internal/services/plugin_system.go" 2>/dev/null; then
        has_logging=true
    fi

    # Check in request service
    if grep -qE "circuit breaker is open" "$PROJECT_ROOT/internal/services/request_service.go" 2>/dev/null; then
        has_logging=true
    fi

    # Check in llm package
    if grep -qE "circuit breaker is open|ErrCircuitOpen" "$PROJECT_ROOT/internal/llm/circuit_breaker.go" 2>/dev/null; then
        has_logging=true
    fi

    if [ "$has_logging" = true ]; then
        pass_test "CircuitBreaker logging implemented"
    else
        fail_test "CircuitBreaker logging not found"
    fi
}

# Test 9: Verify graceful degradation (system continues when providers fail)
test_graceful_degradation() {
    log_info "Test 9: Graceful degradation support"

    local has_degradation=false

    # Check for fallback responses or graceful handling
    if grep -qE "using fallback|all providers failed" "$PROJECT_ROOT/internal/services/debate_service.go"; then
        has_degradation=true
    fi

    if grep -q "FallbackResponse\|fallbackResponse" "$PROJECT_ROOT/internal/handlers/openai_compatible.go"; then
        has_degradation=true
    fi

    if [ "$has_degradation" = true ]; then
        pass_test "Graceful degradation implemented"
    else
        fail_test "Graceful degradation not found"
    fi
}

# Test 10: Verify circuit breaker recovery (half-open state)
test_circuit_breaker_recovery() {
    log_info "Test 10: CircuitBreaker recovery logic"

    if grep -qE "halfOpen|half.?open|StateHalfOpen" "$PROJECT_ROOT/internal/services/plugin_system.go"; then
        pass_test "CircuitBreaker recovery (half-open) implemented"
    else
        fail_test "CircuitBreaker recovery logic not found"
    fi
}

# Run all tests
main() {
    echo ""

    test_circuit_breaker_struct
    test_circuit_breaker_states
    test_circuit_breaker_constructor
    test_circuit_breaker_execute
    test_circuit_breaker_config
    test_fallback_mechanism
    test_circuit_breaker_unit_tests
    test_circuit_breaker_logging
    test_graceful_degradation
    test_circuit_breaker_recovery

    echo ""
    echo -e "${BLUE}=============================================${NC}"
    echo -e "${BLUE}  Challenge Summary                         ${NC}"
    echo -e "${BLUE}=============================================${NC}"
    echo ""
    echo -e "Total Tests:   $TOTAL_TESTS"
    echo -e "${GREEN}Passed:        $PASSED_TESTS${NC}"
    echo -e "${RED}Failed:        $FAILED_TESTS${NC}"
    echo ""

    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}Circuit Breaker Challenge: PASSED${NC}"
        exit 0
    else
        echo -e "${RED}Circuit Breaker Challenge: FAILED${NC}"
        exit 1
    fi
}

main "$@"
