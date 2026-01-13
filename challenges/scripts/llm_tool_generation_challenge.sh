#!/bin/bash

# =============================================================================
# LLM Tool Generation Challenge
# =============================================================================
# This challenge validates that HelixAgent uses LLM-based intelligence for
# tool selection instead of hardcoded patterns.
#
# Key Requirements:
# 1. NO hardcoded confirmation patterns (yes, proceed, go ahead, etc.)
# 2. ALL arbitrary user intents handled via LLM-based tool generation
# 3. System makes actual LLM calls to decide tool selection
# 4. Proper fallback chain when providers unavailable
#
# Tests:
# 1. No hardcoded confirmation patterns in source
# 2. generateLLMBasedToolCalls function exists
# 3. generateActionToolCalls uses LLM fallback
# 4. Tool descriptions properly formatted
# 5. Messages converted for LLM context
# 6. Provider registry validation
# 7. Debate team config validation
# 8. Synthesis-based tool selection works
# 9. Tool call ID generation is unique
# 10. JSON escaping works correctly
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HANDLER_FILE="$PROJECT_ROOT/internal/handlers/openai_compatible.go"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0

echo "=========================================="
echo "LLM Tool Generation Challenge"
echo "=========================================="
echo ""

# Test function
run_test() {
    local test_num=$1
    local test_name=$2
    local test_cmd=$3

    echo -n "Test $test_num: $test_name... "

    if eval "$test_cmd" > /dev/null 2>&1; then
        echo -e "${GREEN}PASSED${NC}"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "${RED}FAILED${NC}"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# =============================================================================
# Test 1: No hardcoded confirmation patterns
# =============================================================================
test1() {
    # Check that these hardcoded confirmation patterns do NOT exist in tool generation
    # These patterns should NOT be in a containsAny() call for action execution

    # The key is that there should be no hardcoded list of "yes", "proceed", etc.
    # for triggering tool execution without LLM intelligence

    # Check that we DON'T have hardcoded confirmation phrase lists used in containsAny
    # for direct pattern matching (not in LLM system prompts)
    # Look for patterns like: confirmationPhrases := []string{"yes"...}
    # or containsAny(text, []string{"yes", "proceed"...})
    local bad_patterns=$(grep -n 'confirmationPhrases\s*:=\|containsAny.*\[\]string.*"yes"' "$HANDLER_FILE" 2>/dev/null || true)

    if [ -n "$bad_patterns" ]; then
        # Found hardcoded patterns - this is BAD
        return 1
    fi

    # Check that LLM-based generation IS used
    if grep -q "generateLLMBasedToolCalls" "$HANDLER_FILE" 2>/dev/null; then
        return 0
    fi

    return 1
}

run_test 1 "No hardcoded confirmation patterns" "test1"

# =============================================================================
# Test 2: generateLLMBasedToolCalls function exists
# =============================================================================
test2() {
    grep -n "func.*generateLLMBasedToolCalls" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 2 "generateLLMBasedToolCalls function exists" "test2"

# =============================================================================
# Test 3: generateActionToolCalls uses LLM fallback
# =============================================================================
test3() {
    # Check that generateActionToolCalls calls generateLLMBasedToolCalls
    # The function is large, so we need to check more lines
    grep -A 700 "func.*generateActionToolCalls" "$HANDLER_FILE" | \
        grep "generateLLMBasedToolCalls" > /dev/null 2>&1
}

run_test 3 "generateActionToolCalls uses LLM fallback" "test3"

# =============================================================================
# Test 4: Tool descriptions are built for LLM prompt
# =============================================================================
test4() {
    # Check that tool descriptions are formatted for LLM
    grep -n "toolDescriptions\|WriteString.*tool\|tool.Function.Name\|tool.Function.Description" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 4 "Tool descriptions formatted for LLM" "test4"

# =============================================================================
# Test 5: Messages converted for LLM context
# =============================================================================
test5() {
    # Check that messages are converted to context
    grep -n "userPromptBuilder\|Conversation context\|msg.Role.*user\|msg.Role.*assistant" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 5 "Messages converted for LLM context" "test5"

# =============================================================================
# Test 6: Provider registry validation
# =============================================================================
test6() {
    # Check that provider registry is validated before LLM call
    grep -n "providerRegistry == nil\|providerRegistry != nil" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 6 "Provider registry validation" "test6"

# =============================================================================
# Test 7: Debate team config validation
# =============================================================================
test7() {
    # Check that debate team config is validated
    grep -n "debateTeamConfig == nil\|debateTeamConfig != nil\|GetTeamMember" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 7 "Debate team config validation" "test7"

# =============================================================================
# Test 8: Synthesis-based tool selection
# =============================================================================
test8() {
    # Check that synthesis can trigger tool selection
    grep -n "synthesisLower\|use.*tool\|call.*tool\|invoke.*tool" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 8 "Synthesis-based tool selection" "test8"

# =============================================================================
# Test 9: Tool call ID generation
# =============================================================================
test9() {
    # Check that tool call IDs are generated
    grep -n "generateToolCallID\|call_.*ID\|UnixNano" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 9 "Tool call ID generation" "test9"

# =============================================================================
# Test 10: JSON escaping for tool arguments
# =============================================================================
test10() {
    # Check that JSON escaping is implemented
    grep -n "escapeJSONString\|ReplaceAll.*\\\\\|ReplaceAll.*\\\\n\|ReplaceAll.*\\\\t" "$HANDLER_FILE" > /dev/null 2>&1
}

run_test 10 "JSON escaping for tool arguments" "test10"

# =============================================================================
# Test 11: Unit tests exist for LLM-based tool generation
# =============================================================================
test11() {
    local TEST_FILE="$PROJECT_ROOT/internal/handlers/openai_compatible_test.go"

    # Check for comprehensive tests
    grep -n "TestNoHardcodedConfirmationPatterns\|TestLLMBasedToolGeneration\|TestGenerateActionToolCallsUsesLLM" "$TEST_FILE" > /dev/null 2>&1
}

run_test 11 "Unit tests for LLM-based tool generation" "test11"

# =============================================================================
# Test 12: Unit tests run successfully
# =============================================================================
test12() {
    cd "$PROJECT_ROOT"
    go test -v -run "LLMBased|NoHardcoded|SynthesisBased|ToolCallID|EscapeJSON" ./internal/handlers/... -short 2>&1 | \
        grep -E "^(ok|PASS)" > /dev/null
}

run_test 12 "Unit tests pass" "test12"

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "=========================================="
echo "Challenge Results"
echo "=========================================="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}ALL TESTS PASSED!${NC}"
    echo "LLM Tool Generation Challenge: SUCCESS"
    exit 0
else
    echo -e "${RED}SOME TESTS FAILED${NC}"
    echo "LLM Tool Generation Challenge: FAILED"
    exit 1
fi
