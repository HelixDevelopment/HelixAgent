#!/bin/bash
# HelixLLM Integration Challenge
# Comprehensive challenge script for HelixLLM submodule integration

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CHALLENGE_NAME="helixllm_integration"
REPORT_FILE="${PROJECT_ROOT}/challenge-results/${CHALLENGE_NAME}-$(date +%Y%m%d-%H%M%S).json"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0

# Results array
RESULTS=()

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_result="${3:-0}"
    
    ((TESTS_TOTAL++))
    
    log_info "Running: ${test_name}"
    
    if eval "${test_command}"; then
        if [ "${expected_result}" -eq 0 ]; then
            log_pass "${test_name}"
            RESULTS+=("{\"test\":\"${test_name}\",\"status\":\"passed\"}")
            return 0
        else
            log_fail "${test_name} (expected failure but passed)"
            RESULTS+=("{\"test\":\"${test_name}\",\"status\":\"failed\",\"error\":\"expected failure but passed\"}")
            return 1
        fi
    else
        if [ "${expected_result}" -eq 1 ]; then
            log_pass "${test_name} (expected failure)"
            RESULTS+=("{\"test\":\"${test_name}\",\"status\":\"passed\",\"note\":\"expected failure\"}")
            return 0
        else
            log_fail "${test_name}"
            RESULTS+=("{\"test\":\"${test_name}\",\"status\":\"failed\"}")
            return 1
        fi
    fi
}

# Test 1: Submodule exists
test_submodule_exists() {
    [ -d "${PROJECT_ROOT}/HelixLLM" ] && [ -f "${PROJECT_ROOT}/HelixLLM/README.md" ]
}

# Test 2: Submodule has correct structure
test_submodule_structure() {
    [ -d "${PROJECT_ROOT}/HelixLLM/internal" ] && \
    [ -d "${PROJECT_ROOT}/HelixLLM/cmd" ] && \
    [ -d "${PROJECT_ROOT}/HelixLLM/deploy" ]
}

# Test 3: Environment variables configured
test_env_configuration() {
    grep -q "USE_HELIX_LLM" "${PROJECT_ROOT}/.env" && \
    grep -q "HELIX_LLM_ENDPOINT" "${PROJECT_ROOT}/.env"
}

# Test 4: Docker compose file exists
test_compose_file() {
    [ -f "${PROJECT_ROOT}/docker-compose.helixllm.yml" ]
}

# Test 5: Provider implementation exists
test_provider_implementation() {
    [ -f "${PROJECT_ROOT}/internal/llm/providers/helixllm/provider.go" ]
}

# Test 6: Adapter implementation exists
test_adapter_implementation() {
    [ -f "${PROJECT_ROOT}/internal/adapters/helixllm/adapter.go" ]
}

# Test 7: HelixQA test bank exists
test_helixqa_bank() {
    [ -f "${PROJECT_ROOT}/../helix_qa/banks/helixllm.yaml" ]
}

# Test 8: Integration test script exists
test_integration_script() {
    [ -f "${PROJECT_ROOT}/tests/helixllm/test_helixllm_integration.sh" ]
}

# Test 9: Documentation exists
test_documentation() {
    [ -f "${PROJECT_ROOT}/docs/HELIXLLM_INTEGRATION.md" ]
}

# Test 10: Gitmodules entry exists
test_gitmodules_entry() {
    grep -q "HelixLLM" "${PROJECT_ROOT}/.gitmodules"
}

# Test 11: Provider compiles
test_provider_compiles() {
    cd "${PROJECT_ROOT}"
    go build ./internal/llm/providers/helixllm/... > /dev/null 2>&1
}

# Test 12: Adapter compiles
test_adapter_compiles() {
    cd "${PROJECT_ROOT}"
    go build ./internal/adapters/helixllm/... > /dev/null 2>&1
}

# Test 13: Docker compose syntax valid
test_compose_syntax() {
    docker-compose -f "${PROJECT_ROOT}/docker-compose.helixllm.yml" config > /dev/null 2>&1
}

# Test 14: Environment file valid
test_env_file() {
    [ -f "${PROJECT_ROOT}/.env" ] && grep -q "USE_HELIX_LLM=true" "${PROJECT_ROOT}/.env"
}

# Test 15: Test script is executable
test_script_executable() {
    [ -x "${PROJECT_ROOT}/tests/helixllm/test_helixllm_integration.sh" ]
}

# Test 16: Challenge script is executable
test_challenge_executable() {
    [ -x "$0" ]
}

# Generate report
generate_report() {
    mkdir -p "$(dirname "${REPORT_FILE}")"
    
    local results_json=$(IFS=,; echo "${RESULTS[*]}")
    
    cat > "${REPORT_FILE}" << EOF
{
  "challenge": "${CHALLENGE_NAME}",
  "timestamp": "$(date -Iseconds)",
  "summary": {
    "total": ${TESTS_TOTAL},
    "passed": ${TESTS_PASSED},
    "failed": ${TESTS_FAILED},
    "success_rate": $(awk "BEGIN {printf \"%.2f\", (${TESTS_PASSED}/${TESTS_TOTAL})*100}")
  },
  "results": [${results_json}],
  "status": "$([ ${TESTS_FAILED} -eq 0 ] && echo "PASSED" || echo "FAILED")"
}
EOF
    
    log_info "Report saved to: ${REPORT_FILE}"
}

# Main execution
main() {
    echo "==================================="
    echo "HelixLLM Integration Challenge"
    echo "==================================="
    echo ""
    
    mkdir -p "${PROJECT_ROOT}/challenge-results"
    
    # Run all tests
    run_test "Submodule exists" "test_submodule_exists"
    run_test "Submodule structure" "test_submodule_structure"
    run_test "Environment configuration" "test_env_configuration"
    run_test "Docker compose file" "test_compose_file"
    run_test "Provider implementation" "test_provider_implementation"
    run_test "Adapter implementation" "test_adapter_implementation"
    run_test "HelixQA test bank" "test_helixqa_bank"
    run_test "Integration test script" "test_integration_script"
    run_test "Documentation" "test_documentation"
    run_test "Gitmodules entry" "test_gitmodules_entry"
    run_test "Provider compiles" "test_provider_compiles"
    run_test "Adapter compiles" "test_adapter_compiles"
    run_test "Docker compose syntax" "test_compose_syntax"
    run_test "Environment file" "test_env_file"
    run_test "Test script executable" "test_script_executable"
    run_test "Challenge executable" "test_challenge_executable"
    
    echo ""
    echo "==================================="
    echo "Results:"
    echo "  Total:  ${TESTS_TOTAL}"
    echo "  Passed: ${TESTS_PASSED}"
    echo "  Failed: ${TESTS_FAILED}"
    echo "==================================="
    
    generate_report
    
    if [ ${TESTS_FAILED} -eq 0 ]; then
        echo -e "${GREEN}CHALLENGE PASSED!${NC}"
        exit 0
    else
        echo -e "${RED}CHALLENGE FAILED!${NC}"
        exit 1
    fi
}

main "$@"
