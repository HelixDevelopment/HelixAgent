#!/bin/bash

# debate_tool_triggering_challenge.sh - AI Debate Tool Triggering Challenge
# Tests that the AI Debate system properly collects and executes tool calls
# Ensures tool_calls are not discarded and action indicators are present

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default configuration
HOST="${HELIXAGENT_HOST:-localhost}"
PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://${HOST}:${PORT}"
RESULTS_DIR="${PROJECT_ROOT}/challenges/results/debate_tool_triggering/$(date +%Y%m%d_%H%M%S)"

echo ""
echo "======================================================================"
echo "          HELIXAGENT AI DEBATE TOOL TRIGGERING CHALLENGE"
echo "======================================================================"
echo ""
echo -e "${CYAN}This challenge verifies that the AI Debate system:${NC}"
echo "  1. Collects tool_calls from debate positions"
echo "  2. Uses collected tool_calls in ACTION PHASE"
echo "  3. Shows action indicators (<---, --->)"
echo "  4. Does NOT discard tool_calls from LLM responses"
echo ""
echo "Host: $HOST"
echo "Port: $PORT"
echo "Results: $RESULTS_DIR"
echo ""

# Create results directory
mkdir -p "$RESULTS_DIR/results"

# Challenge tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

record_result() {
    local test_name="$1"
    local status="$2"
    local details="$3"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    if [ "$status" == "pass" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "  ${GREEN}[PASS]${NC} $test_name"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "  ${RED}[FAIL]${NC} $test_name"
        echo "$details" >> "$RESULTS_DIR/results/failures.txt"
    fi
}

echo "----------------------------------------------------------------------"
echo "Phase 1: DebatePositionResponse Structure Tests"
echo "----------------------------------------------------------------------"

cd "$PROJECT_ROOT"

# Test DebatePositionResponse struct
echo -e "${BLUE}[RUN]${NC} Testing DebatePositionResponse struct..."
if go test -v -run "TestDebatePositionResponse_Struct" ./internal/handlers/... > "$RESULTS_DIR/results/struct_test.txt" 2>&1; then
    record_result "DebatePositionResponse struct holds content and tool_calls" "pass" ""
else
    record_result "DebatePositionResponse struct holds content and tool_calls" "fail" "$(cat $RESULTS_DIR/results/struct_test.txt)"
fi

# Test empty tool calls handling
echo -e "${BLUE}[RUN]${NC} Testing empty tool calls handling..."
if go test -v -run "TestDebatePositionResponse_EmptyToolCalls" ./internal/handlers/... > "$RESULTS_DIR/results/empty_test.txt" 2>&1; then
    record_result "Empty tool_calls handled correctly" "pass" ""
else
    record_result "Empty tool_calls handled correctly" "fail" "$(cat $RESULTS_DIR/results/empty_test.txt)"
fi

# Test multiple tool calls
echo -e "${BLUE}[RUN]${NC} Testing multiple tool calls handling..."
if go test -v -run "TestDebatePositionResponse_MultipleToolCalls" ./internal/handlers/... > "$RESULTS_DIR/results/multiple_test.txt" 2>&1; then
    record_result "Multiple tool_calls preserved correctly" "pass" ""
else
    record_result "Multiple tool_calls preserved correctly" "fail" "$(cat $RESULTS_DIR/results/multiple_test.txt)"
fi

# Test JSON serialization
echo -e "${BLUE}[RUN]${NC} Testing JSON serialization..."
if go test -v -run "TestDebatePositionResponse_JSONSerialization" ./internal/handlers/... > "$RESULTS_DIR/results/json_test.txt" 2>&1; then
    record_result "JSON serialization preserves tool_calls" "pass" ""
else
    record_result "JSON serialization preserves tool_calls" "fail" "$(cat $RESULTS_DIR/results/json_test.txt)"
fi

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 2: Tool Calls Collection Tests"
echo "----------------------------------------------------------------------"

# Test tool calls collection from debate
echo -e "${BLUE}[RUN]${NC} Testing tool calls collection from debate positions..."
if go test -v -run "TestToolCallsCollectionFromDebate" ./internal/handlers/... > "$RESULTS_DIR/results/collection_test.txt" 2>&1; then
    record_result "Tool calls collected from all debate positions" "pass" ""
else
    record_result "Tool calls collected from all debate positions" "fail" "$(cat $RESULTS_DIR/results/collection_test.txt)"
fi

# Test debate tool calls integration
echo -e "${BLUE}[RUN]${NC} Testing full debate tool calls integration..."
if go test -v -run "TestDebateToolCallsIntegration" ./internal/handlers/... > "$RESULTS_DIR/results/integration_test.txt" 2>&1; then
    record_result "Full debate-to-action tool calls flow" "pass" ""
else
    record_result "Full debate-to-action tool calls flow" "fail" "$(cat $RESULTS_DIR/results/integration_test.txt)"
fi

# Test tool calls not discarded
echo -e "${BLUE}[RUN]${NC} Testing tool calls preservation..."
if go test -v -run "TestToolCallsNotDiscarded" ./internal/handlers/... > "$RESULTS_DIR/results/preservation_test.txt" 2>&1; then
    record_result "Tool calls are NOT discarded" "pass" ""
else
    record_result "Tool calls are NOT discarded" "fail" "$(cat $RESULTS_DIR/results/preservation_test.txt)"
fi

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 3: Action Indicator Tests"
echo "----------------------------------------------------------------------"

# Test action indicator generation
echo -e "${BLUE}[RUN]${NC} Testing action indicator generation..."
if go test -v -run "TestActionIndicatorGeneration" ./internal/handlers/... > "$RESULTS_DIR/results/indicator_gen_test.txt" 2>&1; then
    record_result "Action indicators (<---, --->) generated correctly" "pass" ""
else
    record_result "Action indicators (<---, --->) generated correctly" "fail" "$(cat $RESULTS_DIR/results/indicator_gen_test.txt)"
fi

# Test action indicator visibility
echo -e "${BLUE}[RUN]${NC} Testing action indicator visibility..."
if go test -v -run "TestActionIndicatorVisibility" ./internal/handlers/... > "$RESULTS_DIR/results/indicator_vis_test.txt" 2>&1; then
    record_result "All action indicators visible in output" "pass" ""
else
    record_result "All action indicators visible in output" "fail" "$(cat $RESULTS_DIR/results/indicator_vis_test.txt)"
fi

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 4: Tool Call Generation Tests"
echo "----------------------------------------------------------------------"

# Test tool call generation with tools
echo -e "${BLUE}[RUN]${NC} Testing tool call generation with available tools..."
if go test -v -run "TestGenerateActionToolCalls_WithTools" ./internal/handlers/... > "$RESULTS_DIR/results/gen_with_tools_test.txt" 2>&1; then
    record_result "Tool calls generated when tools available" "pass" ""
else
    record_result "Tool calls generated when tools available" "fail" "$(cat $RESULTS_DIR/results/gen_with_tools_test.txt)"
fi

# Test tool call generation without tools
echo -e "${BLUE}[RUN]${NC} Testing tool call generation without tools..."
if go test -v -run "TestGenerateActionToolCalls_NoTools" ./internal/handlers/... > "$RESULTS_DIR/results/gen_no_tools_test.txt" 2>&1; then
    record_result "No tool calls generated when no tools available" "pass" ""
else
    record_result "No tool calls generated when no tools available" "fail" "$(cat $RESULTS_DIR/results/gen_no_tools_test.txt)"
fi

# Test search query tool call generation
echo -e "${BLUE}[RUN]${NC} Testing search query tool call generation..."
if go test -v -run "TestGenerateActionToolCalls_SearchQuery" ./internal/handlers/... > "$RESULTS_DIR/results/gen_search_test.txt" 2>&1; then
    record_result "Grep tool calls generated for search queries" "pass" ""
else
    record_result "Grep tool calls generated for search queries" "fail" "$(cat $RESULTS_DIR/results/gen_search_test.txt)"
fi

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 5: Tool Call Structure Validation Tests"
echo "----------------------------------------------------------------------"

# Test streaming tool call field validation
echo -e "${BLUE}[RUN]${NC} Testing streaming tool call field validation..."
if go test -v -run "TestStreamingToolCall_FieldValidation" ./internal/handlers/... > "$RESULTS_DIR/results/field_validation_test.txt" 2>&1; then
    record_result "StreamingToolCall fields validated correctly" "pass" ""
else
    record_result "StreamingToolCall fields validated correctly" "fail" "$(cat $RESULTS_DIR/results/field_validation_test.txt)"
fi

# Test tool call arguments validation
echo -e "${BLUE}[RUN]${NC} Testing tool call arguments validation..."
if go test -v -run "TestToolCallArgumentsValidation" ./internal/handlers/... > "$RESULTS_DIR/results/args_validation_test.txt" 2>&1; then
    record_result "Tool call arguments follow correct format (snake_case)" "pass" ""
else
    record_result "Tool call arguments follow correct format (snake_case)" "fail" "$(cat $RESULTS_DIR/results/args_validation_test.txt)"
fi

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 6: Code Compilation and Build Test"
echo "----------------------------------------------------------------------"

# Test that the code compiles
echo -e "${BLUE}[RUN]${NC} Testing code compilation..."
if go build -o /dev/null ./cmd/helixagent/ > "$RESULTS_DIR/results/build_test.txt" 2>&1; then
    record_result "HelixAgent compiles successfully with tool triggering changes" "pass" ""
else
    record_result "HelixAgent compiles successfully with tool triggering changes" "fail" "$(cat $RESULTS_DIR/results/build_test.txt)"
fi

# Test handlers package compiles
echo -e "${BLUE}[RUN]${NC} Testing handlers package compilation..."
if go build -o /dev/null ./internal/handlers/ > "$RESULTS_DIR/results/handlers_build_test.txt" 2>&1; then
    record_result "Handlers package compiles with DebatePositionResponse" "pass" ""
else
    record_result "Handlers package compiles with DebatePositionResponse" "fail" "$(cat $RESULTS_DIR/results/handlers_build_test.txt)"
fi

echo ""
echo "======================================================================"
echo "                      CHALLENGE RESULTS"
echo "======================================================================"
echo ""
echo -e "Total Tests:  ${BLUE}$TOTAL_TESTS${NC}"
echo -e "Passed:       ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:       ${RED}$FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}============================================${NC}"
    echo -e "${GREEN}  ALL TESTS PASSED - CHALLENGE COMPLETE!   ${NC}"
    echo -e "${GREEN}============================================${NC}"
    echo ""
    echo -e "${CYAN}Tool triggering in AI Debate is working correctly:${NC}"
    echo "  - DebatePositionResponse returns content AND tool_calls"
    echo "  - Tool calls collected from all 5 debate positions"
    echo "  - ACTION PHASE uses collected tool_calls"
    echo "  - Action indicators (<---, --->) are visible"
    echo "  - Tool_calls are NOT discarded"
    echo ""
    # Save success summary
    cat > "$RESULTS_DIR/CHALLENGE_PASSED.txt" << EOF
AI Debate Tool Triggering Challenge PASSED
==========================================

Date: $(date)
Total Tests: $TOTAL_TESTS
Passed: $PASSED_TESTS
Failed: $FAILED_TESTS

The AI Debate system correctly:
1. Returns tool_calls from generateRealDebateResponse()
2. Collects tool_calls from all debate positions
3. Uses collected tool_calls in ACTION PHASE
4. Shows action indicators (<---, --->)
5. Does NOT discard tool_calls from LLM responses
EOF
    exit 0
else
    echo -e "${RED}============================================${NC}"
    echo -e "${RED}  CHALLENGE FAILED - $FAILED_TESTS TESTS FAILED  ${NC}"
    echo -e "${RED}============================================${NC}"
    echo ""
    echo "See $RESULTS_DIR/results/failures.txt for details"
    exit 1
fi
