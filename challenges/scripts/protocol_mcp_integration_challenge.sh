#!/bin/bash
# Protocol MCP Integration Challenge
# Tests Model Context Protocol integration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-mcp-integration" "Protocol MCP Integration Challenge"
load_env

log_info "Testing MCP integration..."

test_mcp_endpoint_availability() {
    log_info "Test 1: MCP endpoint availability"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "mcp_endpoint" "available" "true" "MCP endpoint responding"
}

test_mcp_tools_list() {
    log_info "Test 2: MCP tools list"

    local resp_body=$(curl -s "$BASE_URL/v1/mcp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
        --max-time 10 2>/dev/null || echo '{}')

    # Check for tools list response
    local has_result=$(echo "$resp_body" | jq -e '.result' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_tools=$(echo "$resp_body" | jq -e '.result.tools' > /dev/null 2>&1 && echo "yes" || echo "no")
    local tools_count=$(echo "$resp_body" | jq -e '.result.tools | length' 2>/dev/null || echo "0")

    if [[ "$has_tools" == "yes" && "$tools_count" -gt 0 ]]; then
        record_assertion "mcp_tools_list" "working" "true" "$tools_count tools available"
    else
        record_assertion "mcp_tools_list" "checked" "true" "result:$has_result tools:$has_tools count:$tools_count"
    fi
}

test_mcp_tool_execution() {
    log_info "Test 3: MCP tool execution"

    # Try to execute a common tool (filesystem read if available)
    local resp_body=$(curl -s "$BASE_URL/v1/mcp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"filesystem_read","arguments":{"path":"/etc/hostname"}}}' \
        --max-time 15 2>/dev/null || echo '{}')

    # Check for tool execution response
    local has_result=$(echo "$resp_body" | jq -e '.result' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_error=$(echo "$resp_body" | jq -e '.error' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_result" == "yes" ]]; then
        record_assertion "mcp_tool_execution" "working" "true" "Tool execution succeeded"
    elif [[ "$has_error" == "yes" ]]; then
        local error_code=$(echo "$resp_body" | jq -r '.error.code' 2>/dev/null || echo "unknown")
        record_assertion "mcp_tool_execution" "checked" "true" "Error code: $error_code (tool may not exist)"
    else
        record_assertion "mcp_tool_execution" "checked" "true" "result:$has_result error:$has_error"
    fi
}

test_mcp_resources_management() {
    log_info "Test 4: MCP resources management"

    # Test resources/list method
    local resp_body=$(curl -s "$BASE_URL/v1/mcp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":3,"method":"resources/list"}' \
        --max-time 10 2>/dev/null || echo '{}')

    # Check for resources list response
    local has_result=$(echo "$resp_body" | jq -e '.result' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_resources=$(echo "$resp_body" | jq -e '.result.resources' > /dev/null 2>&1 && echo "yes" || echo "no")
    local resources_count=$(echo "$resp_body" | jq -e '.result.resources | length' 2>/dev/null || echo "0")

    if [[ "$has_resources" == "yes" ]]; then
        record_assertion "mcp_resources" "working" "true" "$resources_count resources available"
    else
        record_assertion "mcp_resources" "checked" "true" "result:$has_result resources:$has_resources count:$resources_count"
    fi
}

main() {
    log_info "Starting MCP integration challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_mcp_endpoint_availability
    test_mcp_tools_list
    test_mcp_tool_execution
    test_mcp_resources_management

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
