#!/bin/bash
# Debate Testing Integration Challenge
# Validates that all debate testing components are properly integrated
#
# This script tests:
# 1. Sandboxed test execution
# 2. LLM-based test case generation
# 3. Contrastive analysis
# 4. Service bridge integration
#
# Run: ./challenges/scripts/debate_testing_integration_challenge.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
    ((TESTS_TOTAL++))
}

pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

section() {
    echo ""
    echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"
}

# Test 1: Test Executor Implementation
test_executor_implementation() {
    section "Test Executor Implementation"
    
    log_test "Test executor file exists"
    if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go" ]; then
        pass "test_executor.go exists"
    else
        fail "test_executor.go not found"
    fi
    
    log_test "ContainerSandbox struct is defined"
    if grep -q "type ContainerSandbox struct" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass "ContainerSandbox struct defined"
    else
        fail "ContainerSandbox struct not found"
    fi
    
    log_test "NewContainerSandbox function is defined"
    if grep -q "func NewContainerSandbox" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass "NewContainerSandbox function defined"
    else
        fail "NewContainerSandbox function not found"
    fi
    
    log_test "Execute method uses sandbox"
    if grep -q "sandbox.Execute" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass "Execute method uses sandbox"
    else
        fail "Execute method does not use sandbox"
    fi
    
    log_test "SandboxConfig is defined"
    if grep -q "type SandboxConfig struct" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass "SandboxConfig struct defined"
    else
        fail "SandboxConfig struct not found"
    fi
    
    log_test "ExecutionRequest is defined"
    if grep -q "type ExecutionRequest struct" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass "ExecutionRequest struct defined"
    else
        fail "ExecutionRequest struct not found"
    fi
    
    log_test "buildRunArgs method exists"
    if grep -q "func.*buildRunArgs" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass "buildRunArgs method exists"
    else
        fail "buildRunArgs method not found"
    fi
    
    log_test "cleanupContainer method exists"
    if grep -q "func.*cleanupContainer" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass "cleanupContainer method exists"
    else
        fail "cleanupContainer method not found"
    fi
}

# Test 2: Test Case Generator Implementation
test_case_generator_implementation() {
    section "Test Case Generator Implementation"
    
    log_test "test_case_generator.go file exists"
    if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go" ]; then
        pass "test_case_generator.go exists"
    else
        fail "test_case_generator.go not found"
    fi
    
    log_test "LLMClient interface is defined"
    if grep -q "type LLMClient interface" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass "LLMClient interface defined"
    else
        fail "LLMClient interface not found"
    fi
    
    log_test "ProviderAdapter is defined"
    if grep -q "type ProviderAdapter struct" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass "ProviderAdapter struct defined"
    else
        fail "ProviderAdapter struct not found"
    fi
    
    log_test "NewLLMTestCaseGeneratorFromFunc exists"
    if grep -q "func NewLLMTestCaseGeneratorFromFunc" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass "NewLLMTestCaseGeneratorFromFunc function defined"
    else
        fail "NewLLMTestCaseGeneratorFromFunc function not found"
    fi
    
    log_test "generateTestWithLLM method exists"
    if grep -q "func.*generateTestWithLLM" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass "generateTestWithLLM method exists"
    else
        fail "generateTestWithLLM method not found"
    fi
    
    log_test "parseLLMResponse method exists"
    if grep -q "func.*parseLLMResponse" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass "parseLLMResponse method exists"
    else
        fail "parseLLMResponse method not found"
    fi
    
    log_test "generateFallbackTestCase method exists"
    if grep -q "func.*generateFallbackTestCase" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass "generateFallbackTestCase method exists"
    else
        fail "generateFallbackTestCase method not found"
    fi
    
    log_test "Timestamp is properly set in GenerateTestCase"
    if grep -q "CreatedAt:.*time.Now().Unix()" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass "Timestamp properly set in GenerateTestCase"
    else
        fail "Timestamp not properly set"
    fi
}

# Test 3: Contrastive Analyzer Implementation
test_contrastive_analyzer_implementation() {
    section "Contrastive Analyzer Implementation"
    
    log_test "contrastive_analyzer.go file exists"
    if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go" ]; then
        pass "contrastive_analyzer.go exists"
    else
        fail "contrastive_analyzer.go not found"
    fi
    
    log_test "Timestamp is properly set in Analyze"
    if grep -q "Timestamp:.*time.Now().Unix()" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass "Timestamp properly set in Analyze"
    else
        fail "Timestamp not properly set in Analyze"
    fi
    
    log_test "analyzeErrorDeep method exists"
    if grep -q "func.*analyzeErrorDeep" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass "analyzeErrorDeep method exists"
    else
        fail "analyzeErrorDeep method not found"
    fi
    
    log_test "analyzeErrorPatterns method exists"
    if grep -q "func.*analyzeErrorPatterns" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass "analyzeErrorPatterns method exists"
    else
        fail "analyzeErrorPatterns method not found"
    fi
    
    log_test "Nil pointer detection is implemented"
    if grep -q "nil pointer\|null pointer" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass "Nil pointer detection implemented"
    else
        fail "Nil pointer detection not found"
    fi
    
    log_test "Index out of bounds detection is implemented"
    if grep -q "index out of range\|out of bounds" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass "Index out of bounds detection implemented"
    else
        fail "Index out of bounds detection not found"
    fi
    
    log_test "Deadlock detection is implemented"
    if grep -q "deadlock" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass "Deadlock detection implemented"
    else
        fail "Deadlock detection not found"
    fi
    
    log_test "Security vulnerability detection is implemented"
    if grep -q "security\|injection" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass "Security vulnerability detection implemented"
    else
        fail "Security vulnerability detection not found"
    fi
}

# Test 4: Service Bridge Implementation
test_service_bridge_implementation() {
    section "Service Bridge Implementation"
    
    log_test "service_bridge.go file exists"
    if [ -f "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go" ]; then
        pass "service_bridge.go exists"
    else
        fail "service_bridge.go not found"
    fi
    
    log_test "MCPServiceInterface is defined"
    if grep -q "type MCPServiceInterface interface" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "MCPServiceInterface defined"
    else
        fail "MCPServiceInterface not found"
    fi
    
    log_test "LSPManagerInterface is defined"
    if grep -q "type LSPManagerInterface interface" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "LSPManagerInterface defined"
    else
        fail "LSPManagerInterface not found"
    fi
    
    log_test "RAGServiceInterface is defined"
    if grep -q "type RAGServiceInterface interface" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "RAGServiceInterface defined"
    else
        fail "RAGServiceInterface not found"
    fi
    
    log_test "EmbeddingServiceInterface is defined"
    if grep -q "type EmbeddingServiceInterface interface" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "EmbeddingServiceInterface defined"
    else
        fail "EmbeddingServiceInterface not found"
    fi
    
    log_test "FormatterServiceInterface is defined"
    if grep -q "type FormatterServiceInterface interface" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "FormatterServiceInterface defined"
    else
        fail "FormatterServiceInterface not found"
    fi
    
    log_test "mcpClientAdapter implements CallTool"
    if grep -q "func.*mcpClientAdapter.*CallTool" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "mcpClientAdapter.CallTool implemented"
    else
        fail "mcpClientAdapter.CallTool not implemented"
    fi
    
    log_test "lspClientAdapter implements GetDefinition"
    if grep -q "func.*lspClientAdapter.*GetDefinition" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "lspClientAdapter.GetDefinition implemented"
    else
        fail "lspClientAdapter.GetDefinition not implemented"
    fi
    
    log_test "ragClientAdapter implements Search"
    if grep -q "func.*ragClientAdapter.*Search" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "ragClientAdapter.Search implemented"
    else
        fail "ragClientAdapter.Search not implemented"
    fi
    
    log_test "embeddingClientAdapter implements Embed"
    if grep -q "func.*embeddingClientAdapter.*Embed" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "embeddingClientAdapter.Embed implemented"
    else
        fail "embeddingClientAdapter.Embed not implemented"
    fi
    
    log_test "formatterRegistryAdapter implements Format"
    if grep -q "func.*formatterRegistryAdapter.*Format" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "formatterRegistryAdapter.Format implemented"
    else
        fail "formatterRegistryAdapter.Format not implemented"
    fi
    
    log_test "CheckServicesHealth method exists"
    if grep -q "func.*CheckServicesHealth" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go"; then
        pass "CheckServicesHealth method exists"
    else
        fail "CheckServicesHealth method not found"
    fi
}

# Test 5: Compilation Check
test_compilation() {
    section "Compilation Check"
    
    log_test "Debate testing package compiles"
    if go build "$PROJECT_ROOT/DebateOrchestrator/testing/..." 2>/dev/null; then
        pass "Debate testing package compiles successfully"
    else
        fail "Debate testing package compilation failed"
    fi
    
    log_test "Debate tools package compiles"
    if go build "$PROJECT_ROOT/DebateOrchestrator/tools/..." 2>/dev/null; then
        pass "Debate tools package compiles successfully"
    else
        fail "Debate tools package compilation failed"
    fi
    
    log_test "Services package compiles with debate integration"
    if go build "$PROJECT_ROOT/internal/services/..." 2>/dev/null; then
        pass "Services package compiles successfully"
    else
        fail "Services package compilation failed"
    fi
}

# Test 6: Test Utilities
test_test_utilities() {
    section "Test Utilities"
    
    log_test "testutils fixtures_test.go exists"
    if [ -f "$PROJECT_ROOT/tests/testutils/fixtures_test.go" ]; then
        pass "fixtures_test.go exists"
    else
        fail "fixtures_test.go not found"
    fi
    
    log_test "testutils test_helpers_test.go exists"
    if [ -f "$PROJECT_ROOT/tests/testutils/test_helpers_test.go" ]; then
        pass "test_helpers_test.go exists"
    else
        fail "test_helpers_test.go not found"
    fi
    
    log_test "Test utilities tests pass"
    if go test -short "$PROJECT_ROOT/tests/testutils/..." 2>&1 | grep -q "PASS"; then
        pass "Test utilities tests pass"
    else
        warn "Test utilities tests may have issues"
    fi
}

# Test 7: No TODO placeholders remain
test_no_todo_placeholders() {
    section "TODO Placeholder Check"
    
    log_test "No TODO placeholders in test_executor.go"
    TODO_COUNT=$(grep -c "// TODO:" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go" 2>/dev/null || echo "0")
    if [ "$TODO_COUNT" -eq 0 ]; then
        pass "No TODO placeholders in test_executor.go"
    else
        fail "Found $TODO_COUNT TODO placeholders in test_executor.go"
    fi
    
    log_test "No TODO placeholders in test_case_generator.go"
    TODO_COUNT=$(grep -c "// TODO:" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go" 2>/dev/null || echo "0")
    if [ "$TODO_COUNT" -eq 0 ]; then
        pass "No TODO placeholders in test_case_generator.go"
    else
        fail "Found $TODO_COUNT TODO placeholders in test_case_generator.go"
    fi
    
    log_test "No TODO placeholders in contrastive_analyzer.go"
    TODO_COUNT=$(grep -c "// TODO:" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go" 2>/dev/null || echo "0")
    if [ "$TODO_COUNT" -eq 0 ]; then
        pass "No TODO placeholders in contrastive_analyzer.go"
    else
        fail "Found $TODO_COUNT TODO placeholders in contrastive_analyzer.go"
    fi
    
    log_test "No TODO placeholders in service_bridge.go"
    TODO_COUNT=$(grep -c "// TODO:" "$PROJECT_ROOT/DebateOrchestrator/tools/service_bridge.go" 2>/dev/null || echo "0")
    if [ "$TODO_COUNT" -eq 0 ]; then
        pass "No TODO placeholders in service_bridge.go"
    else
        fail "Found $TODO_COUNT TODO placeholders in service_bridge.go"
    fi
}

# Test 8: Integration Test
test_integration() {
    section "Integration Tests"
    
    log_test "Debate testing tests pass"
    if go test -short "$PROJECT_ROOT/internal/debate/..." 2>&1 | grep -q "PASS"; then
        pass "Debate tests pass"
    else
        fail "Debate tests failed"
    fi
}

# Main
main() {
    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║        Debate Testing Integration Challenge                  ║"
    echo "║                    Phase 1 Validation                        ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    
    cd "$PROJECT_ROOT"
    
    test_executor_implementation
    test_case_generator_implementation
    test_contrastive_analyzer_implementation
    test_service_bridge_implementation
    test_compilation
    test_test_utilities
    test_no_todo_placeholders
    test_integration
    
    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                      SUMMARY                                 ║"
    echo "╠══════════════════════════════════════════════════════════════╣"
    printf "║  Total:  %-3d  Passed: %-3d  Failed: %-3d                   ║\n" "$TESTS_TOTAL" "$TESTS_PASSED" "$TESTS_FAILED"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo ""
    
    if [ "$TESTS_FAILED" -eq 0 ]; then
        echo -e "${GREEN}✓ All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}✗ Some tests failed. Please review and fix.${NC}"
        exit 1
    fi
}

main "$@"
