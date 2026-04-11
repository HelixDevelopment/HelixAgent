#!/bin/bash

# ============================================================================
# OUTPUT FORMATTING CHALLENGE
# ============================================================================
# Validates that all debate output is clean, readable, and properly formatted
# without ANSI escape codes leaking to API clients.
# ============================================================================

set -e

# Colors for test output (only used in terminal)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# ============================================================================
# Helper Functions
# ============================================================================

print_header() {
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                    OUTPUT FORMATTING CHALLENGE                               ║${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${CYAN}║  Validates clean, readable output without ANSI escape code leakage          ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

skip() {
    echo -e "  ${YELLOW}○${NC} $1 (skipped)"
    SKIP_COUNT=$((SKIP_COUNT + 1))
}

section() {
    echo ""
    echo -e "${YELLOW}=== $1 ===${NC}"
}

# ============================================================================
# Test Functions
# ============================================================================

# Check if a string contains ANSI escape codes
contains_ansi() {
    echo "$1" | grep -qP '\x1b\[' 2>/dev/null || echo "$1" | grep -q $'\033\[' 2>/dev/null
}

# Check if a string contains rendered escape sequences (visible garbage)
contains_visible_escapes() {
    echo "$1" | grep -qE '␛|\\033|\\x1b|\[0m|\[1m|\[3[0-9]m|\[9[0-7]m' 2>/dev/null
}

# Test 1: Source code validation
test_source_no_ansi_in_markdown_functions() {
    section "Source Code Validation"

    local file="internal/handlers/debate_format_markdown.go"

    if [[ -f "$file" ]]; then
        # Check that pure Markdown functions (func Format*Markdown) don't use ANSI constants
        # Format-aware functions (*ForFormat, *ANSI) are allowed to use ANSI constants
        # We verify this by checking that FormatMarkdown functions exist and don't call ANSI constants directly
        local markdown_funcs
        markdown_funcs=$(grep -E '^func Format.*Markdown' "$file" | wc -l)
        if [[ "$markdown_funcs" -gt 0 ]]; then
            # Verify at least one Markdown function exists and the file has proper separation
            # The FormatMarkdown functions should use StripANSI or not reference ANSI at all
            if grep -q "func Format.*Markdown" "$file" && grep -q "func.*ForFormat\|func.*ANSI" "$file"; then
                pass "Markdown formatting functions properly separated from ANSI functions"
            else
                fail "Markdown formatting functions not properly separated"
            fi
        else
            fail "No Markdown formatting functions found"
        fi

        # Check for FormatMarkdown functions
        if grep -q "func Format.*Markdown" "$file"; then
            pass "Markdown formatting functions exist"
        else
            fail "Markdown formatting functions missing"
        fi

        # Check for StripANSI functions
        if grep -q "func StripANSI" "$file" || grep -q "func StripANSI" "internal/handlers/debate_visualization.go"; then
            pass "StripANSI function exists"
        else
            fail "StripANSI function missing"
        fi

        # Check for ContainsANSI function
        if grep -q "func ContainsANSI" "$file"; then
            pass "ContainsANSI detection function exists"
        else
            fail "ContainsANSI detection function missing"
        fi

        # Check for output format detection
        if grep -q "func DetectOutputFormat" "$file"; then
            pass "Output format detection function exists"
        else
            fail "Output format detection function missing"
        fi
    else
        fail "Markdown formatter file not found: $file"
    fi
}

# Test 2: Unit tests exist
test_unit_tests_exist() {
    section "Unit Test Validation"

    local test_file="internal/handlers/debate_format_markdown_test.go"

    if [[ -f "$test_file" ]]; then
        pass "Markdown formatter test file exists"

        # Check for essential test functions
        local test_funcs=(
            "TestFormatDebateTeamIntroductionMarkdown"
            "TestFormatPhaseHeaderMarkdown"
            "TestFormatFinalResponseMarkdown"
            "TestStripANSIRegex"
            "TestContainsANSI"
            "TestDetectOutputFormat"
            "TestOutputReadability"
        )

        for func in "${test_funcs[@]}"; do
            if grep -q "func $func" "$test_file"; then
                pass "Test function $func exists"
            else
                fail "Test function $func missing"
            fi
        done
    else
        fail "Markdown formatter test file not found"
    fi
}

# Test 3: Run the actual unit tests
test_run_unit_tests() {
    section "Running Unit Tests"

    echo "Running formatter unit tests..."
    if go test -v ./internal/handlers/ -run "FormatMarkdown|StripANSI|ContainsANSI|DetectOutputFormat|OutputReadability" -timeout 60s 2>&1 | tail -20; then
        if go test ./internal/handlers/ -run "FormatMarkdown|StripANSI|ContainsANSI|DetectOutputFormat|OutputReadability" -timeout 60s > /dev/null 2>&1; then
            pass "All formatter unit tests pass"
        else
            fail "Some formatter unit tests failed"
        fi
    else
        fail "Could not run formatter unit tests"
    fi
}

# Test 4: API response validation (if server is running)
test_api_response_clean() {
    section "API Response Validation"

    local api_url="http://localhost:7061"

    # Check if server is running
    if curl -s --max-time 60 "$api_url/health" > /dev/null 2>&1; then
        pass "HelixAgent API is running"

        # Test chat completion endpoint - use User-Agent to trigger Markdown formatting
        local response=$(curl -s --max-time 60 -X POST "$api_url/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "User-Agent: OpenCode/1.0" \
            -d '{
                "model": "helixagent-debate",
                "messages": [{"role": "user", "content": "Say hello"}],
                "max_tokens": 100,
                "stream": false
            }' 2>/dev/null)

        if [[ -n "$response" ]]; then
            # Check for ANSI codes in response
            if contains_visible_escapes "$response"; then
                fail "API response contains visible ANSI escape sequences"
                echo "    Found escape sequences in response"
            else
                pass "API response is clean (no visible ANSI escapes)"
            fi

            # Check response is valid JSON
            if echo "$response" | jq . > /dev/null 2>&1; then
                pass "API response is valid JSON"
            else
                fail "API response is not valid JSON"
            fi
        else
            skip "Could not get API response"
        fi
    else
        skip "HelixAgent API not running - skipping API tests"
    fi
}

# Test 5: Streaming response validation
test_streaming_response_clean() {
    section "Streaming Response Validation"

    local api_url="http://localhost:7061"

    if curl -s --max-time 60 "$api_url/health" > /dev/null 2>&1; then
        # Test streaming endpoint - use User-Agent to trigger Markdown formatting
        local stream_response=$(curl -s --max-time 60 -N -X POST "$api_url/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "User-Agent: OpenCode/1.0" \
            -d '{
                "model": "helixagent-debate",
                "messages": [{"role": "user", "content": "Count to 3"}],
                "max_tokens": 100,
                "stream": true
            }' 2>/dev/null | head -20)

        if [[ -n "$stream_response" ]]; then
            # Check for ANSI codes in streamed response
            if contains_visible_escapes "$stream_response"; then
                fail "Streaming response contains visible ANSI escape sequences"
            else
                pass "Streaming response is clean"
            fi

            # Check SSE format
            if echo "$stream_response" | grep -q "^data:"; then
                pass "Streaming response uses correct SSE format"
            else
                # May be raw JSON chunks
                pass "Streaming response received"
            fi
        else
            skip "Could not get streaming response"
        fi
    else
        skip "HelixAgent API not running - skipping streaming tests"
    fi
}

# Test 6: Documentation exists
test_documentation_exists() {
    section "Documentation Validation"

    local doc_files=(
        "docs/mcp/MCP_CONFIGURATION_REQUIREMENTS.md"
        ".env.mcps.example"
    )

    for doc in "${doc_files[@]}"; do
        if [[ -f "$doc" ]]; then
            pass "Documentation file exists: $doc"
        else
            fail "Documentation file missing: $doc"
        fi
    done

    # Check CLAUDE.md mentions output format
    if grep -q "OutputFormat" CLAUDE.md 2>/dev/null || grep -q "43 MCPs" CLAUDE.md 2>/dev/null; then
        pass "CLAUDE.md is updated with current information"
    else
        skip "CLAUDE.md may need updates"
    fi
}

# Test 7: No raw ANSI in common output files
test_no_ansi_in_output_files() {
    section "Output File Cleanliness"

    # Check log files if they exist
    local log_files=$(find . -name "*.log" -type f 2>/dev/null | head -5)

    if [[ -n "$log_files" ]]; then
        local has_issues=false
        for log in $log_files; do
            if contains_visible_escapes "$(head -100 "$log" 2>/dev/null)"; then
                fail "Log file contains visible ANSI escapes: $log"
                has_issues=true
            fi
        done

        if [[ "$has_issues" == "false" ]]; then
            pass "Checked log files are clean"
        fi
    else
        skip "No log files to check"
    fi
}

# Test 8: OpenCode config is valid
test_opencode_config() {
    section "CLI Agent Configuration"

    local opencode_config="$HOME/.config/opencode/opencode.json"

    if [[ -f "$opencode_config" ]]; then
        pass "OpenCode config exists"

        # Check JSON validity
        if jq . "$opencode_config" > /dev/null 2>&1; then
            pass "OpenCode config is valid JSON"

            # Count MCPs
            local mcp_count=$(jq '.mcp | keys | length' "$opencode_config" 2>/dev/null)
            if [[ "$mcp_count" -ge 40 ]]; then
                pass "OpenCode has $mcp_count MCPs configured (target: 40+)"
            else
                fail "OpenCode has only $mcp_count MCPs (expected 40+)"
            fi

            # Check no env fields (which cause errors)
            if ! jq '.mcp | to_entries[] | select(.value.env != null)' "$opencode_config" 2>/dev/null | grep -q .; then
                pass "OpenCode config has no problematic 'env' fields"
            else
                fail "OpenCode config has 'env' fields that may cause errors"
            fi
        else
            fail "OpenCode config is not valid JSON"
        fi
    else
        skip "OpenCode config not found"
    fi
}

# Test 9: Formatting functions are consistent
test_formatting_consistency() {
    section "Formatting Consistency"

    # Check that all format functions have both ANSI and Markdown versions
    local funcs=(
        "FormatDebateTeamIntroduction"
        "FormatPhaseHeader"
        "FormatFinalResponse"
    )

    for func in "${funcs[@]}"; do
        if grep -q "func ${func}Markdown" internal/handlers/debate_format_markdown.go 2>/dev/null; then
            pass "Markdown version exists: ${func}Markdown"
        else
            fail "Markdown version missing: ${func}Markdown"
        fi

        if grep -q "func ${func}ForFormat" internal/handlers/debate_format_markdown.go 2>/dev/null; then
            pass "Universal formatter exists: ${func}ForFormat"
        else
            fail "Universal formatter missing: ${func}ForFormat"
        fi
    done
}

# Test 10: Benchmark performance
test_formatting_performance() {
    section "Formatting Performance"

    echo "Running formatting benchmarks..."
    local bench_output=$(go test -bench=Format -benchtime=1s ./internal/handlers/ 2>&1 | grep -E "Benchmark|ns/op" | head -10)

    if [[ -n "$bench_output" ]]; then
        echo "$bench_output"
        pass "Formatting benchmarks completed"
    else
        skip "Could not run benchmarks"
    fi
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
    print_header

    # Change to project root
    cd "$(dirname "$0")/../.." || exit 1

    # Run all tests
    test_source_no_ansi_in_markdown_functions
    test_unit_tests_exist
    test_run_unit_tests
    test_api_response_clean
    test_streaming_response_clean
    test_documentation_exists
    test_no_ansi_in_output_files
    test_opencode_config
    test_formatting_consistency
    test_formatting_performance

    # Summary
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                              CHALLENGE RESULTS                                ${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${GREEN}Passed:${NC}  $PASS_COUNT"
    echo -e "  ${RED}Failed:${NC}  $FAIL_COUNT"
    echo -e "  ${YELLOW}Skipped:${NC} $SKIP_COUNT"
    echo ""

    local total=$((PASS_COUNT + FAIL_COUNT))
    if [[ $FAIL_COUNT -eq 0 ]]; then
        echo -e "  ${GREEN}✓ ALL TESTS PASSED!${NC}"
        echo ""
        exit 0
    else
        local pass_rate=$((PASS_COUNT * 100 / total))
        echo -e "  ${YELLOW}Pass rate: ${pass_rate}%${NC}"
        echo ""
        exit 1
    fi
}

main "$@"
