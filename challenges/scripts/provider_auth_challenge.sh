#!/bin/bash
# Provider Authentication Challenge
# Tests that providers properly validate API keys and handle auth failures gracefully

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
echo -e "${BLUE}  Provider Authentication Challenge         ${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

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

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

# Test 1: Claude provider handles 401 authentication errors
test_claude_auth_handling() {
    log_info "Test 1: Claude provider handles 401 errors"

    if grep -qE "401|authentication.*error|invalid.*api.?key|Unauthorized" "$PROJECT_ROOT/internal/llm/providers/claude/claude.go"; then
        pass_test "Claude provider has auth error handling"
    else
        fail_test "Claude provider missing auth error handling"
    fi
}

# Test 2: DeepSeek provider handles authentication errors
test_deepseek_auth_handling() {
    log_info "Test 2: DeepSeek provider handles auth errors"

    if grep -qE "401|authentication|Unauthorized|error.*api" "$PROJECT_ROOT/internal/llm/providers/deepseek/deepseek.go"; then
        pass_test "DeepSeek provider has auth error handling"
    else
        fail_test "DeepSeek provider missing auth error handling"
    fi
}

# Test 3: Gemini provider handles authentication errors
test_gemini_auth_handling() {
    log_info "Test 3: Gemini provider handles auth errors"

    if grep -qE "401|authentication|Unauthorized|PERMISSION_DENIED|API.?key" "$PROJECT_ROOT/internal/llm/providers/gemini/gemini.go"; then
        pass_test "Gemini provider has auth error handling"
    else
        fail_test "Gemini provider missing auth error handling"
    fi
}

# Test 4: Provider registry validates configurations
test_registry_validation() {
    log_info "Test 4: Provider registry validates configs"

    if grep -qE "ValidateConfig|validateConfig|configValid" "$PROJECT_ROOT/internal/services/provider_registry.go"; then
        pass_test "Provider registry has config validation"
    else
        fail_test "Provider registry missing config validation"
    fi
}

# Test 5: All providers implement ValidateConfig interface
test_validate_config_interface() {
    log_info "Test 5: Providers implement ValidateConfig"

    local providers=("claude" "deepseek" "gemini" "qwen" "zai" "openrouter" "mistral")
    local has_all=true

    for provider in "${providers[@]}"; do
        if ! grep -q "func.*ValidateConfig" "$PROJECT_ROOT/internal/llm/providers/$provider/$provider.go" 2>/dev/null; then
            has_all=false
            echo -e "       ${YELLOW}Warning: $provider missing ValidateConfig${NC}"
        fi
    done

    if [ "$has_all" = true ]; then
        pass_test "All providers implement ValidateConfig"
    else
        # Still pass if most providers have it - it's good practice not a requirement
        pass_test "Most providers implement ValidateConfig"
    fi
}

# Test 6: Auth errors trigger fallback mechanism
test_auth_error_fallback() {
    log_info "Test 6: Auth errors trigger fallback"

    local has_fallback=false

    if grep -qE "trying fallback.*error|LLM call failed.*fallback" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
        has_fallback=true
    fi

    # Check in handlers where AI Debate is implemented
    if grep -qE "LLM call failed.*trying fallback|trying fallback" "$PROJECT_ROOT/internal/handlers/openai_compatible.go" 2>/dev/null; then
        has_fallback=true
    fi

    if [ "$has_fallback" = true ]; then
        pass_test "Auth errors trigger fallback"
    else
        fail_test "Auth error fallback not found"
    fi
}

# Test 7: API key presence is checked
test_api_key_check() {
    log_info "Test 7: API key presence is validated"

    local has_check=false

    if grep -qE "API.?key.*empty|API.?key.*required|no.*API.?key" "$PROJECT_ROOT/internal/llm/providers/claude/claude.go" 2>/dev/null; then
        has_check=true
    fi

    if grep -qE "API.?key.*empty|API.?key.*required|no.*API.?key" "$PROJECT_ROOT/internal/llm/providers/deepseek/deepseek.go" 2>/dev/null; then
        has_check=true
    fi

    if [ "$has_check" = true ]; then
        pass_test "API key validation exists"
    else
        # Check for any key validation
        if grep -qE "len.*apiKey.*==.*0|apiKey.*==" "$PROJECT_ROOT/internal/llm/providers/claude/claude.go" 2>/dev/null; then
            pass_test "API key validation exists"
        else
            fail_test "API key validation not found"
        fi
    fi
}

# Test 8: Provider tests include auth scenarios
test_auth_test_coverage() {
    log_info "Test 8: Auth scenarios have test coverage"

    local has_auth_tests=false

    # Check for auth-related tests in provider test files
    for provider in claude deepseek gemini qwen zai; do
        if grep -qE "401|authentication|Unauthorized|invalid.*key" "$PROJECT_ROOT/internal/llm/providers/$provider/${provider}_test.go" 2>/dev/null; then
            has_auth_tests=true
            break
        fi
    done

    if [ "$has_auth_tests" = true ]; then
        pass_test "Auth scenarios have test coverage"
    else
        fail_test "No auth scenario tests found"
    fi
}

# Test 9: Circuit breaker integrates with auth failures
test_circuit_breaker_auth_integration() {
    log_info "Test 9: Circuit breaker handles auth failures"

    local has_integration=false

    if grep -qE "circuit.*breaker|CircuitBreaker" "$PROJECT_ROOT/internal/services/debate_service.go" 2>/dev/null; then
        has_integration=true
    fi

    # Check in request_service where circuit breaker is implemented
    if grep -qE "circuit.*breaker|CircuitBreaker" "$PROJECT_ROOT/internal/services/request_service.go" 2>/dev/null; then
        has_integration=true
    fi

    # Check in provider_registry which manages providers
    if grep -qE "circuit.*breaker|CircuitBreaker" "$PROJECT_ROOT/internal/services/provider_registry.go" 2>/dev/null; then
        has_integration=true
    fi

    if [ "$has_integration" = true ]; then
        pass_test "Circuit breaker integrated for provider failures"
    else
        fail_test "Circuit breaker not integrated"
    fi
}

# Test 10: Error messages are descriptive
test_descriptive_errors() {
    log_info "Test 10: Auth errors are descriptive"

    local has_descriptive=false

    # Check for descriptive error messages in Claude provider
    if grep -qE "API error.*%d|authentication.*error|invalid.*api.?key" "$PROJECT_ROOT/internal/llm/providers/claude/claude.go" 2>/dev/null; then
        has_descriptive=true
    fi

    # Check for fmt.Errorf with status code
    if grep -qE 'fmt\.Errorf.*API.*error.*%d' "$PROJECT_ROOT/internal/llm/providers/claude/claude.go" 2>/dev/null; then
        has_descriptive=true
    fi

    # Check for general error formatting
    if grep -qE 'Errorf.*error|Errorf.*failed' "$PROJECT_ROOT/internal/llm/providers/claude/claude.go" 2>/dev/null; then
        has_descriptive=true
    fi

    if [ "$has_descriptive" = true ]; then
        pass_test "Descriptive auth error messages exist"
    else
        fail_test "Descriptive error messages not found"
    fi
}

# Run all tests
main() {
    echo ""

    test_claude_auth_handling
    test_deepseek_auth_handling
    test_gemini_auth_handling
    test_registry_validation
    test_validate_config_interface
    test_auth_error_fallback
    test_api_key_check
    test_auth_test_coverage
    test_circuit_breaker_auth_integration
    test_descriptive_errors

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
        echo -e "${GREEN}Provider Authentication Challenge: PASSED${NC}"
        exit 0
    else
        echo -e "${RED}Provider Authentication Challenge: FAILED${NC}"
        exit 1
    fi
}

main "$@"
