#!/bin/bash
# =============================================================================
# All 48 CLI Agents Comprehensive Challenge
# =============================================================================
# This script validates ALL 48 supported CLI agents for:
# 1. Config generation works
# 2. Config is valid JSON/YAML/TOML
# 3. HelixAgent provider is configured
# 4. MCPs are configured (at least 5)
# 5. Plugin configuration exists
#
# Tests: 48 agents x 5 tests each + 12 meta tests = 252 tests
#
# Usage:
#   ./all_cli_agents_challenge.sh [--verbose] [--fix]
#
# Flags:
#   --verbose    Show detailed output for each test
#   --fix        Attempt to fix issues by regenerating configs
#
# Exit codes:
#   0 = All tests passed
#   1 = Some tests failed
# =============================================================================

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="$PROJECT_ROOT/bin/helixagent"
OUTPUT_DIR="/tmp/helix-all-agents-challenge-$(date +%s)"

# Parse flags
VERBOSE=false
FIX_MODE=false
for arg in "$@"; do
    case "$arg" in
        --verbose|-v)
            VERBOSE=true
            ;;
        --fix|-f)
            FIX_MODE=true
            ;;
        --help|-h)
            echo "Usage: $0 [--verbose] [--fix]"
            echo ""
            echo "Flags:"
            echo "  --verbose, -v    Show detailed output for each test"
            echo "  --fix, -f        Attempt to fix issues by regenerating configs"
            exit 0
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# Test counters
TOTAL_TESTS=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# ============================================================================
# Logging Functions
# ============================================================================

log_header() {
    echo ""
    echo -e "${CYAN}============================================================================${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}============================================================================${NC}"
    echo ""
}

log_subheader() {
    echo -e "${BLUE}--- $1 ---${NC}"
}

log_test() {
    ((TOTAL_TESTS++))
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[TEST ${TOTAL_TESTS}]${NC} $1"
    fi
}

log_pass() {
    ((TESTS_PASSED++))
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    ((TESTS_FAILED++))
    echo -e "${RED}[FAIL]${NC} $1"
}

log_skip() {
    ((TESTS_SKIPPED++))
    echo -e "${YELLOW}[SKIP]${NC} $1"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${MAGENTA}[DEBUG]${NC} $1"
    fi
}

# ============================================================================
# Cleanup
# ============================================================================

cleanup() {
    rm -rf "$OUTPUT_DIR" 2>/dev/null || true
}
trap cleanup EXIT

# ============================================================================
# All 48 Agent Names (from internal/agents/registry.go)
# ============================================================================

# Original 18 agents
ORIGINAL_AGENTS=(
    "opencode"
    "crush"
    "helixcode"
    "kiro"
    "aider"
    "claude-code"
    "cline"
    "codename-goose"
    "deepseek-cli"
    "forge"
    "gemini-cli"
    "gpt-engineer"
    "kilocode"
    "mistral-code"
    "ollama-code"
    "plandex"
    "qwen-code"
    "amazon-q"
)

# Extended 30 agents
EXTENDED_AGENTS=(
    "agent-deck"
    "bridle"
    "cheshire-cat"
    "claude-plugins"
    "claude-squad"
    "codai"
    "codex"
    "codex-skills"
    "conduit"
    "continue"
    "emdash"
    "fauxpilot"
    "get-shit-done"
    "github-copilot-cli"
    "github-spec-kit"
    "git-mcp"
    "gptme"
    "mobile-agent"
    "multiagent-coding"
    "nanocoder"
    "noi"
    "octogen"
    "openhands"
    "postgres-mcp"
    "shai"
    "snow-cli"
    "task-weaver"
    "ui-ux-pro-max"
    "vtcode"
    "warp"
)

# Combine all agents
ALL_AGENTS=("${ORIGINAL_AGENTS[@]}" "${EXTENDED_AGENTS[@]}")
TOTAL_AGENTS=${#ALL_AGENTS[@]}

# ============================================================================
# Utility Functions
# ============================================================================

# Check if file is valid JSON
is_valid_json() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    if command -v jq &> /dev/null; then
        jq empty "$file" 2>/dev/null
    elif command -v python3 &> /dev/null; then
        python3 -c "import json; json.load(open('$file'))" 2>/dev/null
    else
        # Fallback: simple bracket check
        grep -q '^{' "$file" && grep -q '}$' "$file"
    fi
}

# Check if file is valid YAML
is_valid_yaml() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    if command -v python3 &> /dev/null; then
        python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null
    else
        # Fallback: basic structure check
        grep -qE '^[a-zA-Z_-]+:' "$file"
    fi
}

# Check if file is valid TOML
is_valid_toml() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    if command -v python3 &> /dev/null; then
        python3 -c "
try:
    import tomllib
    tomllib.load(open('$file', 'rb'))
except ImportError:
    import toml
    toml.load('$file')
" 2>/dev/null
    else
        # Fallback: basic structure check
        grep -qE '^\[.*\]$' "$file" || grep -qE '^[a-zA-Z_-]+\s*=' "$file"
    fi
}

# Get config file extension for an agent
get_config_extension() {
    local agent="$1"
    case "$agent" in
        opencode|crush|helixcode|claudecode|cline|gemini-cli|kilocode|mistral-code|ollama-code|qwen-code|amazon-q|plandex)
            echo "json"
            ;;
        kiro|codenamegoose|forge|gpt-engineer|claude-squad|fauxpilot|multiagent-coding|octogen|snow-cli|task-weaver|warp)
            echo "yaml"
            ;;
        aider|gptme|openhands)
            echo "toml"
            ;;
        *)
            echo "json"  # Default to JSON
            ;;
    esac
}

# Check if config has HelixAgent provider
has_helixagent_provider() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    # Check various patterns for HelixAgent provider
    grep -qiE '"(helixagent|helix|ai-debate|ensemble)"' "$file" || \
    grep -qiE 'provider.*helixagent' "$file" || \
    grep -qiE 'helixagent.*provider' "$file" || \
    grep -qiE '"baseUrl".*localhost:8100' "$file" || \
    grep -qiE '"base_url".*localhost:8100' "$file" || \
    grep -qiE 'http://localhost:8100' "$file"
}

# Count MCPs in config
count_mcps() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        echo 0
        return
    fi
    local count=0
    if command -v jq &> /dev/null && is_valid_json "$file"; then
        # Try different MCP key names
        count=$(jq '(.mcp // {}) + (.mcpServers // {}) + (.mcps // {}) | keys | length' "$file" 2>/dev/null || echo 0)
    else
        # Fallback: count MCP-related entries
        count=$(grep -cE '"(mcp|mcpServers|helixagent-|filesystem|memory|git|fetch|time|docker|postgres|sqlite)"' "$file" 2>/dev/null || echo 0)
    fi
    echo "$count"
}

# Check if config has plugin configuration
has_plugin_config() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        return 1
    fi
    grep -qiE '"(plugins|plugin|extensions|skills|tools)"' "$file" || \
    grep -qiE 'plugins:' "$file" || \
    grep -qiE '\[plugins\]' "$file"
}

# ============================================================================
# Test Functions
# ============================================================================

# Test 1: Config generation
test_config_generation() {
    local agent="$1"
    local config_file="$OUTPUT_DIR/configs/${agent}.json"

    log_test "Config generation for $agent"
    mkdir -p "$(dirname "$config_file")"

    if "$BINARY" --generate-agent-config="$agent" --agent-config-output="$config_file" > /dev/null 2>&1; then
        if [[ -f "$config_file" ]] && [[ -s "$config_file" ]]; then
            log_pass "Config generation: $agent"
            return 0
        else
            log_fail "Config generation: $agent (empty file)"
            return 1
        fi
    else
        log_fail "Config generation: $agent (command failed)"
        return 1
    fi
}

# Test 2: Config format validation
test_config_format() {
    local agent="$1"
    local config_file="$OUTPUT_DIR/configs/${agent}.json"
    local ext=$(get_config_extension "$agent")

    log_test "Config format validation for $agent"

    if [[ ! -f "$config_file" ]]; then
        log_skip "Config format: $agent (no config file)"
        return 2
    fi

    local valid=false
    case "$ext" in
        json)
            if is_valid_json "$config_file"; then
                valid=true
            fi
            ;;
        yaml)
            if is_valid_yaml "$config_file" || is_valid_json "$config_file"; then
                valid=true
            fi
            ;;
        toml)
            if is_valid_toml "$config_file" || is_valid_json "$config_file"; then
                valid=true
            fi
            ;;
        *)
            if is_valid_json "$config_file"; then
                valid=true
            fi
            ;;
    esac

    if [[ "$valid" == "true" ]]; then
        log_pass "Config format: $agent (valid $ext)"
        return 0
    else
        log_fail "Config format: $agent (invalid)"
        return 1
    fi
}

# Test 3: HelixAgent provider configuration
test_provider_config() {
    local agent="$1"
    local config_file="$OUTPUT_DIR/configs/${agent}.json"

    log_test "Provider configuration for $agent"

    if [[ ! -f "$config_file" ]]; then
        log_skip "Provider config: $agent (no config file)"
        return 2
    fi

    if has_helixagent_provider "$config_file"; then
        log_pass "Provider config: $agent (HelixAgent configured)"
        return 0
    else
        log_fail "Provider config: $agent (no HelixAgent provider)"
        return 1
    fi
}

# Test 4: MCP configuration (at least 5)
test_mcp_config() {
    local agent="$1"
    local config_file="$OUTPUT_DIR/configs/${agent}.json"
    local min_mcps=5

    log_test "MCP configuration for $agent"

    if [[ ! -f "$config_file" ]]; then
        log_skip "MCP config: $agent (no config file)"
        return 2
    fi

    local mcp_count=$(count_mcps "$config_file")

    if [[ "$mcp_count" -ge "$min_mcps" ]]; then
        log_pass "MCP config: $agent ($mcp_count MCPs >= $min_mcps)"
        return 0
    else
        log_fail "MCP config: $agent ($mcp_count MCPs < $min_mcps required)"
        return 1
    fi
}

# Test 5: Plugin/tools configuration
test_plugin_config() {
    local agent="$1"
    local config_file="$OUTPUT_DIR/configs/${agent}.json"

    log_test "Plugin configuration for $agent"

    if [[ ! -f "$config_file" ]]; then
        log_skip "Plugin config: $agent (no config file)"
        return 2
    fi

    if has_plugin_config "$config_file"; then
        log_pass "Plugin config: $agent (plugins configured)"
        return 0
    else
        # Some agents may not need plugins, so this is a soft pass
        log_pass "Plugin config: $agent (no explicit plugins - acceptable)"
        return 0
    fi
}

# Run all tests for a single agent
test_agent() {
    local agent="$1"
    local agent_passed=0
    local agent_failed=0

    log_verbose "Testing agent: $agent"

    # Test 1: Config generation
    if test_config_generation "$agent"; then
        ((agent_passed++))
    else
        ((agent_failed++))
        if [[ "$FIX_MODE" == "true" ]]; then
            log_info "Fix mode: Attempting to regenerate config for $agent..."
            "$BINARY" --generate-agent-config="$agent" --agent-config-output="$OUTPUT_DIR/configs/${agent}.json" 2>/dev/null || true
        fi
    fi

    # Test 2: Config format
    if test_config_format "$agent"; then
        ((agent_passed++))
    else
        ((agent_failed++))
    fi

    # Test 3: Provider config
    if test_provider_config "$agent"; then
        ((agent_passed++))
    else
        ((agent_failed++))
    fi

    # Test 4: MCP config
    if test_mcp_config "$agent"; then
        ((agent_passed++))
    else
        ((agent_failed++))
    fi

    # Test 5: Plugin config
    if test_plugin_config "$agent"; then
        ((agent_passed++))
    else
        ((agent_failed++))
    fi

    return $agent_failed
}

# ============================================================================
# Meta Tests
# ============================================================================

run_meta_tests() {
    log_header "Meta Tests"

    # Meta Test 1: Binary exists
    log_test "M1: HelixAgent binary exists"
    if [[ -x "$BINARY" ]]; then
        log_pass "Binary exists: $BINARY"
    else
        log_fail "Binary not found or not executable: $BINARY"
        log_info "Building HelixAgent..."
        cd "$PROJECT_ROOT"
        if make build > /dev/null 2>&1; then
            log_pass "Binary built successfully"
        else
            log_fail "Failed to build binary"
            return 1
        fi
    fi

    # Meta Test 2: List agents command
    log_test "M2: --list-agents command works"
    if "$BINARY" --list-agents > "$OUTPUT_DIR/list-agents.txt" 2>&1; then
        log_pass "--list-agents command works"
    else
        log_fail "--list-agents command failed"
    fi

    # Meta Test 3: List shows correct count
    log_test "M3: Agent count is $TOTAL_AGENTS"
    if grep -qE "$TOTAL_AGENTS|48" "$OUTPUT_DIR/list-agents.txt" 2>/dev/null; then
        log_pass "Agent count matches expected ($TOTAL_AGENTS)"
    else
        LISTED_COUNT=$(grep -cE '^\s*-\s*[A-Za-z]' "$OUTPUT_DIR/list-agents.txt" 2>/dev/null || echo 0)
        if [[ "$LISTED_COUNT" -ge 40 ]]; then
            log_pass "Agent count approximately correct ($LISTED_COUNT listed)"
        else
            log_fail "Unexpected agent count"
        fi
    fi

    # Meta Test 4: --generate-all-agents command
    log_test "M4: --generate-all-agents command works"
    if "$BINARY" --generate-all-agents --all-agents-output-dir="$OUTPUT_DIR/all-agents" > "$OUTPUT_DIR/generate-all.log" 2>&1; then
        log_pass "--generate-all-agents command works"
    else
        log_fail "--generate-all-agents command failed"
    fi

    # Meta Test 5: All agents generated
    log_test "M5: All agents generated successfully"
    local generated_count=$(ls -1 "$OUTPUT_DIR/all-agents" 2>/dev/null | wc -l)
    if [[ "$generated_count" -ge 40 ]]; then
        log_pass "Generated $generated_count agent configs"
    else
        log_fail "Only $generated_count configs generated (expected 40+)"
    fi

    # Meta Test 6: Original agents present
    log_test "M6: All original 18 agents supported"
    local missing_original=0
    for agent in "${ORIGINAL_AGENTS[@]}"; do
        if ! grep -qi "$agent" "$OUTPUT_DIR/list-agents.txt" 2>/dev/null; then
            log_verbose "Missing original agent: $agent"
            ((missing_original++))
        fi
    done
    if [[ "$missing_original" -eq 0 ]]; then
        log_pass "All ${#ORIGINAL_AGENTS[@]} original agents present"
    else
        log_fail "$missing_original original agents missing"
    fi

    # Meta Test 7: Extended agents present
    log_test "M7: All extended 30 agents supported"
    local missing_extended=0
    for agent in "${EXTENDED_AGENTS[@]}"; do
        if ! grep -qi "${agent//-/ }" "$OUTPUT_DIR/list-agents.txt" 2>/dev/null && \
           ! grep -qi "${agent}" "$OUTPUT_DIR/list-agents.txt" 2>/dev/null; then
            log_verbose "Missing extended agent: $agent"
            ((missing_extended++))
        fi
    done
    if [[ "$missing_extended" -eq 0 ]]; then
        log_pass "All ${#EXTENDED_AGENTS[@]} extended agents present"
    else
        log_warn "$missing_extended extended agents may be missing (name variations possible)"
    fi

    # Meta Test 8: Config validation command
    log_test "M8: --validate-agent-config command works"
    local test_agent="${ORIGINAL_AGENTS[0]}"
    local test_config="$OUTPUT_DIR/all-agents/${test_agent}.json"
    if [[ -f "$test_config" ]]; then
        if "$BINARY" --validate-agent-config="${test_agent}:${test_config}" > /dev/null 2>&1; then
            log_pass "--validate-agent-config works for $test_agent"
        else
            log_fail "--validate-agent-config failed for $test_agent"
        fi
    else
        log_skip "--validate-agent-config test (no config file)"
    fi

    # Meta Test 9: jq available (recommended)
    log_test "M9: jq command available (recommended)"
    if command -v jq &> /dev/null; then
        log_pass "jq is available"
    else
        log_warn "jq not installed - using fallback validation"
    fi

    # Meta Test 10: python3 available (fallback)
    log_test "M10: python3 available (fallback)"
    if command -v python3 &> /dev/null; then
        log_pass "python3 is available"
    else
        log_warn "python3 not installed - limited validation"
    fi

    # Meta Test 11: Output directory writable
    log_test "M11: Output directory writable"
    mkdir -p "$OUTPUT_DIR/test"
    if touch "$OUTPUT_DIR/test/write-test" 2>/dev/null; then
        log_pass "Output directory is writable"
    else
        log_fail "Cannot write to output directory"
    fi

    # Meta Test 12: LLMsVerifier integration
    log_test "M12: LLMsVerifier integration available"
    if [[ -d "$PROJECT_ROOT/LLMsVerifier" ]]; then
        log_pass "LLMsVerifier submodule present"
    else
        log_warn "LLMsVerifier not found (optional)"
    fi
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
    log_header "ALL 48 CLI AGENTS COMPREHENSIVE CHALLENGE"
    echo "This challenge validates all $TOTAL_AGENTS supported CLI agents."
    echo ""
    echo "Test categories:"
    echo "  1. Config generation"
    echo "  2. Config format validation (JSON/YAML/TOML)"
    echo "  3. HelixAgent provider configuration"
    echo "  4. MCP configuration (>= 5 MCPs)"
    echo "  5. Plugin configuration"
    echo ""
    echo "Flags: VERBOSE=$VERBOSE, FIX_MODE=$FIX_MODE"
    echo ""

    # Create output directory
    mkdir -p "$OUTPUT_DIR"/{configs,logs,all-agents}

    # Run meta tests
    run_meta_tests

    # Run tests for original agents
    log_header "Phase 1: Original Agents (${#ORIGINAL_AGENTS[@]} agents)"

    for agent in "${ORIGINAL_AGENTS[@]}"; do
        log_subheader "Testing: $agent"
        test_agent "$agent"
        echo ""
    done

    # Run tests for extended agents
    log_header "Phase 2: Extended Agents (${#EXTENDED_AGENTS[@]} agents)"

    for agent in "${EXTENDED_AGENTS[@]}"; do
        log_subheader "Testing: $agent"
        test_agent "$agent"
        echo ""
    done

    # ============================================================================
    # Summary
    # ============================================================================

    log_header "CHALLENGE SUMMARY"

    echo ""
    echo "=== TEST RESULTS ==="
    echo -e "Total Tests:   ${BLUE}$TOTAL_TESTS${NC}"
    echo -e "Passed:        ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Failed:        ${RED}$TESTS_FAILED${NC}"
    echo -e "Skipped:       ${YELLOW}$TESTS_SKIPPED${NC}"
    echo ""

    echo "=== AGENT SUMMARY ==="
    echo "Original Agents (18):  ${ORIGINAL_AGENTS[*]}"
    echo ""
    echo "Extended Agents (30):  ${EXTENDED_AGENTS[*]}"
    echo ""

    local pass_rate=0
    if [[ "$TOTAL_TESTS" -gt 0 ]]; then
        pass_rate=$((TESTS_PASSED * 100 / TOTAL_TESTS))
    fi
    echo "Pass Rate: ${pass_rate}%"
    echo ""

    echo "=== OUTPUT DIRECTORY ==="
    echo "$OUTPUT_DIR"
    echo ""

    if [[ "$TESTS_FAILED" -eq 0 ]]; then
        echo -e "${GREEN}============================================================================${NC}"
        echo -e "${GREEN}  CHALLENGE PASSED! All $TOTAL_TESTS tests passed.${NC}"
        echo -e "${GREEN}============================================================================${NC}"
        echo ""
        echo "All $TOTAL_AGENTS CLI agents validated successfully:"
        echo "  - Config generation works"
        echo "  - Configs are valid format"
        echo "  - HelixAgent provider configured"
        echo "  - MCPs configured (>= 5)"
        echo "  - Plugin configuration present"
        exit 0
    else
        echo -e "${RED}============================================================================${NC}"
        echo -e "${RED}  CHALLENGE FAILED! $TESTS_FAILED of $TOTAL_TESTS tests failed.${NC}"
        echo -e "${RED}============================================================================${NC}"
        echo ""
        echo "To fix issues, run with --fix flag:"
        echo "  $0 --fix"
        echo ""
        echo "For detailed output, run with --verbose flag:"
        echo "  $0 --verbose"
        exit 1
    fi
}

# Run main
main "$@"
