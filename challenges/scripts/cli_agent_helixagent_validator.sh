#!/bin/bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# CLI Agent HelixAgent Validator
# Uses locally built CLI agents with exported configs to validate HelixAgent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RESULTS_DIR="${PROJECT_ROOT}/challenge-results/cli-agents"
CONFIGS_DIR="${PROJECT_ROOT}/cli_agents_configs"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Ensure directories exist
mkdir -p "${RESULTS_DIR}"

# Agent configurations
declare -A AGENT_CONFIGS
declare -A AGENT_FEATURES

AGENT_CONFIGS[claude_code]="${CONFIGS_DIR}/claude-code.yaml"
AGENT_CONFIGS[aider]="${CONFIGS_DIR}/aider.yaml"
AGENT_CONFIGS[openhands]="${CONFIGS_DIR}/openhands.yaml"
AGENT_CONFIGS[codex]="${CONFIGS_DIR}/codex.yaml"
AGENT_CONFIGS[cline]="${CONFIGS_DIR}/cline.yaml"
AGENT_CONFIGS[gemini]="${CONFIGS_DIR}/gemini.yaml"
AGENT_CONFIGS[continue]="${CONFIGS_DIR}/continue.yaml"
AGENT_CONFIGS[amazonq]="${CONFIGS_DIR}/amazonq.yaml"
AGENT_CONFIGS[kiro]="${CONFIGS_DIR}/kiro.yaml"
AGENT_CONFIGS[cursor]="${CONFIGS_DIR}/cursor.yaml"

AGENT_FEATURES[claude_code]="terminal,mcp,debate,git,bash"
AGENT_FEATURES[aider]="repo_map,architect,multi_file,testing"
AGENT_FEATURES[openhands]="docker,sandbox,nvidia_rag,multi_agent"
AGENT_FEATURES[codex]="agent_mode,tools,commands"
AGENT_FEATURES[cline]="ide,context,auto_approve"
AGENT_FEATURES[gemini]="multi_modal,vision,documents"
AGENT_FEATURES[continue]="open_source,local,privacy"
AGENT_FEATURES[amazonq]="enterprise,security,compliance"
AGENT_FEATURES[kiro]="context_engine,intent,relationships"
AGENT_FEATURES[cursor]="editor,completion,chat,composer"

# HelixAgent endpoint
HELIXAGENT_ENDPOINT="${HELIXAGENT_ENDPOINT:-http://localhost:8100}"

usage() {
    cat <<EOF
CLI Agent HelixAgent Validator

Usage: $0 [OPTIONS] [AGENT]

Options:
    -a, --agent AGENT       Test specific agent (claude_code, aider, openhands, etc.)
    -A, --all               Test all configured agents
    -e, --endpoint URL      HelixAgent endpoint (default: ${HELIXAGENT_ENDPOINT})
    -f, --feature FEATURE   Test specific feature (mcp, debate, ensemble, etc.)
    -l, --list              List available agents
    -v, --verbose           Verbose output
    -h, --help              Show this help

Examples:
    $0 --agent claude_code
    $0 --agent aider --feature ensemble
    $0 --all
    $0 --list

EOF
}

list_agents() {
    echo "Available CLI Agents:"
    echo "===================="
    for agent in "${!AGENT_CONFIGS[@]}"; do
        config="${AGENT_CONFIGS[$agent]}"
        features="${AGENT_FEATURES[$agent]}"
        if [ -f "$config" ]; then
            echo "  ✓ $agent"
            echo "    Features: $features"
            echo "    Config: $config"
        else
            echo "  ✗ $agent (config missing)"
        fi
        echo
    done
}

check_helixagent_health() {
    log_info "Checking HelixAgent health at ${HELIXAGENT_ENDPOINT}..."
    
    if ! curl -sf "${HELIXAGENT_ENDPOINT}/health" > /dev/null 2>&1; then
        log_error "HelixAgent is not running at ${HELIXAGENT_ENDPOINT}"
        return 1
    fi
    
    log_success "HelixAgent is healthy"
    return 0
}

test_agent_config() {
    local agent=$1
    local config="${AGENT_CONFIGS[$agent]}"
    
    log_info "Testing $agent configuration..."
    
    if [ ! -f "$config" ]; then
        log_error "Config not found: $config"
        return 1
    fi
    
    # Validate YAML syntax
    if command -v yq &> /dev/null; then
        if ! yq eval '.' "$config" > /dev/null 2>&1; then
            log_error "Invalid YAML in $config"
            return 1
        fi
    fi
    
    # Check required fields
    if ! grep -q "endpoint:" "$config"; then
        log_error "Missing 'endpoint' in $config"
        return 1
    fi
    
    if ! grep -q "agent_type:" "$config"; then
        log_error "Missing 'agent_type' in $config"
        return 1
    fi
    
    log_success "$agent config is valid"
    return 0
}

test_ensemble_with_agent() {
    local agent=$1
    
    log_info "Testing ensemble completion via $agent..."
    
    # Simulate agent making ensemble request
    local response
    response=$(curl -sf -X POST "${HELIXAGENT_ENDPOINT}/v1/ensemble/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "ensemble",
            "messages": [{"role": "user", "content": "Hello from '$agent'"}],
            "max_tokens": 100
        }' 2>/dev/null) || {
        log_error "Ensemble request failed for $agent"
        return 1
    }
    
    # Check response
    if echo "$response" | grep -q '"choices"'; then
        log_success "$agent ensemble completion works"
        return 0
    else
        log_error "Invalid ensemble response for $agent"
        return 1
    fi
}

test_mcp_with_agent() {
    local agent=$1
    
    log_info "Testing MCP integration via $agent..."
    
    # Test MCP tools/list
    local response
    response=$(curl -sf -X POST "${HELIXAGENT_ENDPOINT}/v1/mcp/tools/list" \
        -H "Content-Type: application/json" \
        -d '{}' 2>/dev/null) || {
        log_error "MCP tools/list failed for $agent"
        return 1
    }
    
    # Count tools
    local tool_count
    tool_count=$(echo "$response" | grep -o '"name"' | wc -l)
    
    if [ "$tool_count" -ge 45 ]; then
        log_success "$agent MCP integration works ($tool_count tools)"
        return 0
    else
        log_warn "$agent MCP returned only $tool_count tools (expected 45+)"
        return 1
    fi
}

test_debate_with_agent() {
    local agent=$1
    
    log_info "Testing debate orchestrator via $agent..."
    
    # Start a debate
    local response
    response=$(curl -sf -X POST "${HELIXAGENT_ENDPOINT}/v1/debate/start" \
        -H "Content-Type: application/json" \
        -d '{
            "topic": "Test topic from '$agent'",
            "topology": "mesh",
            "agents": [
                {"type": "claude", "role": "proponent"},
                {"type": "deepseek", "role": "opponent"}
            ]
        }' 2>/dev/null) || {
        log_error "Debate start failed for $agent"
        return 1
    }
    
    if echo "$response" | grep -q '"debate_id"'; then
        log_success "$agent debate orchestration works"
        return 0
    else
        log_error "Invalid debate response for $agent"
        return 1
    fi
}

test_streaming_with_agent() {
    local agent=$1
    
    log_info "Testing streaming via $agent..."
    
    # Test streaming endpoint
    local response
    response=$(curl -sf -X POST "${HELIXAGENT_ENDPOINT}/v1/completions/stream" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -d '{
            "model": "ensemble",
            "messages": [{"role": "user", "content": "Stream test from '$agent'"}],
            "stream": true
        }' 2>/dev/null | head -c 1000) || {
        log_error "Streaming request failed for $agent"
        return 1
    }
    
    if echo "$response" | grep -q "data:"; then
        log_success "$agent streaming works"
        return 0
    else
        log_error "Invalid streaming response for $agent"
        return 1
    fi
}

test_providers_with_agent() {
    local agent=$1
    
    log_info "Testing providers via $agent..."
    
    local response
    response=$(curl -sf "${HELIXAGENT_ENDPOINT}/v1/providers" 2>/dev/null) || {
        log_error "Providers request failed for $agent"
        return 1
    }
    
    local provider_count
    provider_count=$(echo "$response" | grep -o '"type"' | wc -l)
    
    if [ "$provider_count" -ge 22 ]; then
        log_success "$agent sees $provider_count providers"
        return 0
    else
        log_warn "$agent sees only $provider_count providers (expected 22+)"
        return 1
    fi
}

run_agent_tests() {
    local agent=$1
    local specific_feature=$2
    
    log_info "=========================================="
    log_info "Testing $agent"
    log_info "=========================================="
    
    local results_file="${RESULTS_DIR}/${agent}_$(date +%Y%m%d_%H%M%S).json"
    local passed=0
    local failed=0
    
    # Test config
    if test_agent_config "$agent"; then
        ((passed++))
    else
        ((failed++))
    fi
    
    # Test ensemble
    if [ -z "$specific_feature" ] || [ "$specific_feature" = "ensemble" ]; then
        if test_ensemble_with_agent "$agent"; then
            ((passed++))
        else
            ((failed++))
        fi
    fi
    
    # Test MCP
    if [ -z "$specific_feature" ] || [ "$specific_feature" = "mcp" ]; then
        if test_mcp_with_agent "$agent"; then
            ((passed++))
        else
            ((failed++))
        fi
    fi
    
    # Test debate
    if [ -z "$specific_feature" ] || [ "$specific_feature" = "debate" ]; then
        if test_debate_with_agent "$agent"; then
            ((passed++))
        else
            ((failed++))
        fi
    fi
    
    # Test streaming
    if [ -z "$specific_feature" ] || [ "$specific_feature" = "streaming" ]; then
        if test_streaming_with_agent "$agent"; then
            ((passed++))
        else
            ((failed++))
        fi
    fi
    
    # Test providers
    if [ -z "$specific_feature" ] || [ "$specific_feature" = "providers" ]; then
        if test_providers_with_agent "$agent"; then
            ((passed++))
        else
            ((failed++))
        fi
    fi
    
    # Save results
    cat > "$results_file" <<EOF
{
    "agent": "$agent",
    "timestamp": "$(date -Iseconds)",
    "helixagent_endpoint": "$HELIXAGENT_ENDPOINT",
    "passed": $passed,
    "failed": $failed,
    "total": $((passed + failed)),
    "config_file": "${AGENT_CONFIGS[$agent]}",
    "features": "${AGENT_FEATURES[$agent]}"
}
EOF
    
    log_info "Results saved to: $results_file"
    log_info "Passed: $passed, Failed: $failed"
    
    return $failed
}

run_all_tests() {
    log_info "Running tests for all agents..."
    
    local total_failed=0
    local total_passed=0
    
    for agent in "${!AGENT_CONFIGS[@]}"; do
        if run_agent_tests "$agent"; then
            ((total_passed++))
        else
            ((total_failed++))
        fi
        echo
    done
    
    log_info "=========================================="
    log_info "Final Results"
    log_info "=========================================="
    log_info "Agents passed: $total_passed"
    log_info "Agents failed: $total_failed"
    
    if [ $total_failed -eq 0 ]; then
        log_success "All agents validated successfully!"
        return 0
    else
        log_error "Some agents failed validation"
        return 1
    fi
}

# Main
main() {
    local agent=""
    local feature=""
    local test_all=false
    local verbose=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -a|--agent)
                agent="$2"
                shift 2
                ;;
            -A|--all)
                test_all=true
                shift
                ;;
            -e|--endpoint)
                HELIXAGENT_ENDPOINT="$2"
                shift 2
                ;;
            -f|--feature)
                feature="$2"
                shift 2
                ;;
            -l|--list)
                list_agents
                exit 0
                ;;
            -v|--verbose)
                verbose=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    # Check HelixAgent health
    if ! check_helixagent_health; then
        exit 1
    fi
    
    # Run tests
    if $test_all; then
        run_all_tests
    elif [ -n "$agent" ]; then
        if [ -z "${AGENT_CONFIGS[$agent]}" ]; then
            log_error "Unknown agent: $agent"
            log_info "Use --list to see available agents"
            exit 1
        fi
        run_agent_tests "$agent" "$feature"
    else
        usage
        exit 1
    fi
}

main "$@"
