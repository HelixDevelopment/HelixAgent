#!/bin/bash
#
# Intent-Based Ensemble Routing Challenge
#
# Validates that the system correctly differentiates between:
# - Simple requests (greetings, confirmations, queries) → Single provider (fastest/highest score)
# - Complex requests (debugging, refactoring, implementation) → Full AI debate ensemble
#
# This ensures optimal resource usage and response quality.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$PROJECT_ROOT/challenges/results/intent-routing"
mkdir -p "$RESULTS_DIR"

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

log_info() {
    echo -e "${BLUE}[INFO]${NC} $(date '+%H:%M:%S') $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $(date '+%H:%M:%S') $1"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $(date '+%H:%M:%S') $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $(date '+%H:%M:%S') $1"
    ((TESTS_SKIPPED++))
}

log_section() {
    echo ""
    echo -e "${CYAN}==============================================${NC}"
    echo -e "${CYAN}[SECTION] $1${NC}"
    echo -e "${CYAN}==============================================${NC}"
}

check_helixagent() {
    if curl -s --max-time 60 http://localhost:7061/health > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

test_intent_router_exists() {
    log_section "Section 1: IntentBasedRouter Service Existence"
    
    local router_file="$PROJECT_ROOT/internal/services/intent_based_router.go"
    
    if [[ -f "$router_file" ]]; then
        log_success "IntentBasedRouter file exists"
    else
        log_error "IntentBasedRouter file not found at $router_file"
    fi
    
    if grep -q "type IntentBasedRouter struct" "$router_file" 2>/dev/null; then
        log_success "IntentBasedRouter struct defined"
    else
        log_error "IntentBasedRouter struct not defined"
    fi
    
    if grep -q "ShouldUseEnsemble" "$router_file" 2>/dev/null; then
        log_success "ShouldUseEnsemble method exists"
    else
        log_error "ShouldUseEnsemble method not found"
    fi
    
    if grep -q "GetRoutingDecision" "$router_file" 2>/dev/null; then
        log_success "GetRoutingDecision method exists"
    else
        log_error "GetRoutingDecision method not found"
    fi
    
    if grep -q "AnalyzeRequest" "$router_file" 2>/dev/null; then
        log_success "AnalyzeRequest method exists"
    else
        log_error "AnalyzeRequest method not found"
    fi
}

test_routing_types() {
    log_section "Section 2: Routing Decision Types"
    
    local router_file="$PROJECT_ROOT/internal/services/intent_based_router.go"
    
    local routing_types=(
        "RoutingDecision"
        "RoutingEnsemble"
        "RoutingSingle"
        "RoutingResult"
    )
    
    for rtype in "${routing_types[@]}"; do
        if grep -q "$rtype" "$router_file" 2>/dev/null; then
            log_success "$rtype type defined"
        else
            log_error "$rtype type not found"
        fi
    done
}

test_simple_detection() {
    log_section "Section 3: Simple Message Detection"
    
    local router_file="$PROJECT_ROOT/internal/services/intent_based_router.go"
    
    local simple_patterns=(
        "isGreeting"
        "isSimpleConfirmation"
        "isSimpleQuery"
        "greetingPatterns"
    )
    
    for pattern in "${simple_patterns[@]}"; do
        if grep -q "$pattern" "$router_file" 2>/dev/null; then
            log_success "$pattern function/field exists"
        else
            log_error "$pattern not found"
        fi
    done
}

test_complex_detection() {
    log_section "Section 4: Complex Request Detection"
    
    local router_file="$PROJECT_ROOT/internal/services/intent_based_router.go"
    
    local complex_patterns=(
        "isComplexRequest"
        "complexPatterns"
        "calculateComplexityScore"
        "hasCodebaseContext"
    )
    
    for pattern in "${complex_patterns[@]}"; do
        if grep -q "$pattern" "$router_file" 2>/dev/null; then
            log_success "$pattern function/field exists"
        else
            log_error "$pattern not found"
        fi
    done
}

test_handler_integration() {
    log_section "Section 5: Handler Integration Check"
    
    local openai_handler="$PROJECT_ROOT/internal/handlers/openai_compatible.go"
    
    if grep -q "IntentBasedRouter\|intentRouter" "$openai_handler" 2>/dev/null; then
        log_success "IntentBasedRouter integrated in openai_compatible.go"
    else
        log_warning "IntentBasedRouter not yet integrated in openai_compatible.go (pending implementation)"
    fi
    
    if grep -q "ShouldUseEnsemble\|shouldUseEnsemble" "$openai_handler" 2>/dev/null; then
        log_success "ShouldUseEnsemble called in handler"
    else
        log_warning "ShouldUseEnsemble not yet called in handler (pending implementation)"
    fi
}

test_unit_tests() {
    log_section "Section 6: Unit Tests"
    
    local test_file="$PROJECT_ROOT/internal/services/intent_based_router_test.go"
    
    if [[ -f "$test_file" ]]; then
        log_success "IntentBasedRouter test file exists"
    else
        log_error "IntentBasedRouter test file not found"
        return
    fi
    
    local test_cases=(
        "TestIntentBasedRouter_Greeting"
        "TestIntentBasedRouter_SimpleConfirmation"
        "TestIntentBasedRouter_SimpleQuery"
        "TestIntentBasedRouter_ComplexRequest"
        "TestIntentBasedRouter_CodebaseContext"
        "TestIntentBasedRouter_LongMessage"
        "TestIntentBasedRouter_ShouldUseEnsemble"
    )
    
    for tc in "${test_cases[@]}"; do
        if grep -q "$tc" "$test_file" 2>/dev/null; then
            log_success "$tc test exists"
        else
            log_error "$tc test not found"
        fi
    done
    
    cd "$PROJECT_ROOT"
    if GOMAXPROCS=2 go test -v -run "TestIntentBasedRouter" ./internal/services/ -timeout 60s 2>&1 | grep -q "PASS"; then
        log_success "IntentBasedRouter tests pass"
    else
        log_error "IntentBasedRouter tests failed"
    fi
}

test_routing_logic() {
    log_section "Section 7: Routing Logic Validation"
    
    cd "$PROJECT_ROOT"
    
    local test_results
    test_results=$(GOMAXPROCS=2 go test -v -run "TestIntentBasedRouter" ./internal/services/ -timeout 60s 2>&1)
    
    if echo "$test_results" | grep -q "PASS.*TestIntentBasedRouter_Greeting"; then
        log_success "Greeting detection works correctly"
    else
        log_error "Greeting detection test failed"
    fi
    
    if echo "$test_results" | grep -q "PASS.*TestIntentBasedRouter_SimpleConfirmation"; then
        log_success "Simple confirmation detection works correctly"
    else
        log_error "Simple confirmation detection test failed"
    fi
    
    if echo "$test_results" | grep -q "PASS.*TestIntentBasedRouter_SimpleQuery"; then
        log_success "Simple query detection works correctly"
    else
        log_error "Simple query detection test failed"
    fi
    
    if echo "$test_results" | grep -q "PASS.*TestIntentBasedRouter_ComplexRequest"; then
        log_success "Complex request detection works correctly"
    else
        log_error "Complex request detection test failed"
    fi
    
    if echo "$test_results" | grep -q "PASS.*TestIntentBasedRouter_ShouldUseEnsemble"; then
        log_success "ShouldUseEnsemble method works correctly"
    else
        log_error "ShouldUseEnsemble method test failed"
    fi
}

test_routing_patterns() {
    log_section "Section 8: Routing Pattern Coverage"
    
    local router_file="$PROJECT_ROOT/internal/services/intent_based_router.go"
    
    local greeting_keywords=("hello" "hi" "hey" "greetings" "thanks")
    local complex_keywords=("debug" "refactor" "implement" "analyze" "optimize")
    
    for keyword in "${greeting_keywords[@]}"; do
        if grep -qi "$keyword" "$router_file" 2>/dev/null; then
            log_success "Greeting keyword '$keyword' pattern exists"
        else
            log_error "Greeting keyword '$keyword' pattern missing"
        fi
    done
    
    for keyword in "${complex_keywords[@]}"; do
        if grep -qi "$keyword" "$router_file" 2>/dev/null; then
            log_success "Complex keyword '$keyword' pattern exists"
        else
            log_error "Complex keyword '$keyword' pattern missing"
        fi
    done
}

print_summary() {
    log_section "Summary"
    
    echo ""
    echo -e "${CYAN}Test Results:${NC}"
    echo -e "  ${GREEN}Passed:${NC}   $TESTS_PASSED"
    echo -e "  ${RED}Failed:${NC}   $TESTS_FAILED"
    echo -e "  ${YELLOW}Skipped:${NC}  $TESTS_SKIPPED"
    echo -e "  ${BLUE}Total:${NC}    $((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))"
    echo ""
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}✓ All IntentBasedRouter tests passed!${NC}"
        echo ""
        return 0
    else
        echo -e "${RED}✗ Some tests failed. Please review the output above.${NC}"
        echo ""
        return 1
    fi
}

main() {
    echo ""
    echo -e "${CYAN}==============================================${NC}"
    echo -e "${CYAN}Intent-Based Ensemble Routing Challenge${NC}"
    echo -e "${CYAN}==============================================${NC}"
    echo ""
    
    cd "$PROJECT_ROOT"
    
    test_intent_router_exists
    test_routing_types
    test_simple_detection
    test_complex_detection
    test_handler_integration
    test_unit_tests
    test_routing_logic
    test_routing_patterns
    
    print_summary
}

main "$@"
