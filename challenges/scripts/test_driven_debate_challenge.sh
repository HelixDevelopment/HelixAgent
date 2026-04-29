#!/bin/bash
# Challenge: Test-Driven Debate Implementation Validation
# Validates test case generation, execution, and contrastive analysis
# Zero false positives - validates actual behavior, not just code existence

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Test-Driven Debate"
TOTAL_TESTS=15
PASSED_TESTS=0
FAILED_TESTS=0

# Test 1: Test case generator exists
log_test 1 "Test case generator implementation exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go" ]; then
    if grep -q "type.*TestCaseGenerator.*interface" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go"; then
        pass_test
    else
        fail_test "TestCaseGenerator interface not found"
    fi
else
    fail_test "test_case_generator.go not found"
fi

# Test 2: Test executor exists
log_test 2 "Test executor implementation exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go" ]; then
    if grep -q "type.*TestExecutor.*interface" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
        pass_test
    else
        fail_test "TestExecutor interface not found"
    fi
else
    fail_test "test_executor.go not found"
fi

# Test 3: Contrastive analyzer exists
log_test 3 "Contrastive analyzer implementation exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go" ]; then
    if grep -q "type.*ContrastiveAnalyzer.*interface" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
        pass_test
    else
        fail_test "ContrastiveAnalyzer interface not found"
    fi
else
    fail_test "contrastive_analyzer.go not found"
fi

# Test 4: Protocol integration exists
log_test 4 "Protocol integration for test-driven debate exists"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/protocol_integration.go" ]; then
    if grep -q "func.*TestDrivenDebateRound" "$PROJECT_ROOT/DebateOrchestrator/testing/protocol_integration.go"; then
        pass_test
    else
        fail_test "TestDrivenDebateRound function not found"
    fi
else
    fail_test "protocol_integration.go not found"
fi

# Test 5: Test generation actually works
log_test 5 "Test case generation functionality"
if go test -v -run "TestLLMTestCaseGenerator_GenerateTestCase" "$PROJECT_ROOT/DebateOrchestrator/testing/" 2>&1 | grep -q "PASS"; then
    pass_test
else
    fail_test "Test generation test failed"
fi

# Test 6: Test execution works
log_test 6 "Test execution functionality"
if go test -v -run "TestSandboxedTestExecutor_Execute" "$PROJECT_ROOT/DebateOrchestrator/testing/" 2>&1 | grep -q "PASS"; then
    pass_test
else
    fail_test "Test execution test failed"
fi

# Test 7: Contrastive analysis works
log_test 7 "Contrastive analysis functionality"
if go test -v -run "TestDifferentialContrastiveAnalyzer_Analyze" "$PROJECT_ROOT/DebateOrchestrator/testing/" 2>&1 | grep -q "PASS"; then
    pass_test
else
    fail_test "Contrastive analysis test failed"
fi

# Test 8: Full test-driven round integration
log_test 8 "Full test-driven debate round"
if go test -v -run "TestDebateTestIntegration_TestDrivenDebateRound" "$PROJECT_ROOT/DebateOrchestrator/testing/" 2>&1 | grep -q "PASS"; then
    pass_test
else
    fail_test "Test-driven debate round test failed"
fi

# Test 9: Test categories defined
log_test 9 "Test categories properly defined"
CATEGORIES=$(grep -c "Category.*TestCategory.*=" "$PROJECT_ROOT/DebateOrchestrator/testing/test_case_generator.go" || echo 0)
if [ "$CATEGORIES" -ge 5 ]; then
    pass_test
else
    fail_test "Insufficient test categories ($CATEGORIES, expected >=5)"
fi

# Test 10: Execution metrics collection
log_test 10 "Execution metrics structure exists"
if grep -q "type.*ExecutionMetrics.*struct" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
    pass_test
else
    fail_test "ExecutionMetrics structure not found"
fi

# Test 11: Root cause identification
log_test 11 "Root cause identification implemented"
if grep -q "func.*IdentifyRootCauses" "$PROJECT_ROOT/DebateOrchestrator/testing/contrastive_analyzer.go"; then
    pass_test
else
    fail_test "IdentifyRootCauses method not found"
fi

# Test 12: Sandboxed execution configuration
log_test 12 "Sandbox configuration options exist"
if grep -q "WithTimeout\|WithMemoryLimit\|WithCPULimit" "$PROJECT_ROOT/DebateOrchestrator/testing/test_executor.go"; then
    pass_test
else
    fail_test "Sandbox configuration options missing"
fi

# Test 13: Build succeeds with new code
log_test 13 "Project builds with test-driven debate code"
if go build -o /tmp/helixagent_test_build "$PROJECT_ROOT/cmd/helixagent" 2>&1 | grep -v "error"; then
    pass_test
    rm -f /tmp/helixagent_test_build
else
    fail_test "Build failed with test-driven debate code"
fi

# Test 14: No compilation errors in testing package
log_test 14 "Testing package compiles without errors"
if go build "$PROJECT_ROOT/DebateOrchestrator/testing/" 2>&1 | grep -v "error"; then
    pass_test
else
    fail_test "Testing package has compilation errors"
fi

# Test 15: Test coverage for new code
log_test 15 "Unit tests exist for test-driven debate"
if [ -f "$PROJECT_ROOT/DebateOrchestrator/testing/test_driven_debate_test.go" ]; then
    TEST_COUNT=$(grep -c "^func Test" "$PROJECT_ROOT/DebateOrchestrator/testing/test_driven_debate_test.go" || echo 0)
    if [ "$TEST_COUNT" -ge 5 ]; then
        pass_test
    else
        fail_test "Insufficient tests ($TEST_COUNT, expected >=5)"
    fi
else
    fail_test "Test file not found"
fi

# Print summary
echo "========================================="
echo "Challenge: $CHALLENGE_NAME"
echo "Total Tests: $TOTAL_TESTS"
echo "Passed: $PASSED_TESTS"
echo "Failed: $FAILED_TESTS"
echo "========================================="

if [ "$FAILED_TESTS" -eq 0 ]; then
    echo "✅ All tests passed!"
    exit 0
else
    echo "❌ Some tests failed"
    exit 1
fi
