#!/bin/bash
# =============================================================================
# MCP SSE Connectivity Challenge
# =============================================================================
# This challenge validates MCP/ACP/LSP/Embeddings/Vision/Cognee SSE endpoints
# for CLI agent integration (OpenCode, Crush, HelixCode, Kilo Code).
#
# Key Requirements:
# 1. All protocol SSE endpoints respond correctly (GET /v1/{protocol})
# 2. JSON-RPC 2.0 initialize method works
# 3. tools/list returns available tools
# 4. tools/call executes tools correctly
# 5. Error handling works properly
# 6. Concurrent connections work
#
# Tests:
# 1. MCP SSE endpoint responds
# 2. ACP SSE endpoint responds
# 3. LSP SSE endpoint responds
# 4. Embeddings SSE endpoint responds
# 5. Vision SSE endpoint responds
# 6. Cognee SSE endpoint responds
# 7. MCP initialize method works
# 8. tools/list returns tools
# 9. tools/call executes tools
# 10. Error handling for unknown methods
# 11. Multiple protocols work concurrently
# 12. CLI agent initialization flow works
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-7061}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "mcp_sse_connectivity_challenge" "MCP SSE Connectivity Challenge (CLI Agent Integration)"
load_env

# Test 1: MCP SSE endpoint responds
test_mcp_sse_endpoint() {
    log_info "Test 1: Testing MCP SSE endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 5 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)

    # SSE endpoint should return 200 with event-stream content
    if [[ "$http_code" == "200" ]]; then
        record_assertion "mcp" "sse_endpoint" "true" "MCP SSE endpoint responds with 200"
        record_metric "mcp_sse_status" "200"
    else
        record_assertion "mcp" "sse_endpoint" "false" "MCP SSE endpoint failed: $http_code"
        record_metric "mcp_sse_status" "$http_code"
    fi
}

# Test 2: ACP SSE endpoint responds
test_acp_sse_endpoint() {
    log_info "Test 2: Testing ACP SSE endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/acp" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 5 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "acp" "sse_endpoint" "true" "ACP SSE endpoint responds with 200"
        record_metric "acp_sse_status" "200"
    else
        record_assertion "acp" "sse_endpoint" "false" "ACP SSE endpoint failed: $http_code"
        record_metric "acp_sse_status" "$http_code"
    fi
}

# Test 3: LSP SSE endpoint responds
test_lsp_sse_endpoint() {
    log_info "Test 3: Testing LSP SSE endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/lsp" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 5 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "lsp" "sse_endpoint" "true" "LSP SSE endpoint responds with 200"
        record_metric "lsp_sse_status" "200"
    else
        record_assertion "lsp" "sse_endpoint" "false" "LSP SSE endpoint failed: $http_code"
        record_metric "lsp_sse_status" "$http_code"
    fi
}

# Test 4: Embeddings SSE endpoint responds
test_embeddings_sse_endpoint() {
    log_info "Test 4: Testing Embeddings SSE endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/embeddings" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 5 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)

    # Note: embeddings endpoint might return different status for GET vs POST
    if [[ "$http_code" == "200" ]] || [[ "$http_code" == "400" ]] || [[ "$http_code" == "405" ]]; then
        record_assertion "embeddings" "sse_endpoint" "true" "Embeddings SSE endpoint responds"
        record_metric "embeddings_sse_status" "$http_code"
    else
        record_assertion "embeddings" "sse_endpoint" "false" "Embeddings SSE endpoint failed: $http_code"
        record_metric "embeddings_sse_status" "$http_code"
    fi
}

# Test 5: Vision SSE endpoint responds
test_vision_sse_endpoint() {
    log_info "Test 5: Testing Vision SSE endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/vision" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 5 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "vision" "sse_endpoint" "true" "Vision SSE endpoint responds with 200"
        record_metric "vision_sse_status" "200"
    else
        record_assertion "vision" "sse_endpoint" "false" "Vision SSE endpoint failed: $http_code"
        record_metric "vision_sse_status" "$http_code"
    fi
}

# Test 6: Cognee SSE endpoint responds
test_cognee_sse_endpoint() {
    log_info "Test 6: Testing Cognee SSE endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/cognee" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 5 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "cognee" "sse_endpoint" "true" "Cognee SSE endpoint responds with 200"
        record_metric "cognee_sse_status" "200"
    else
        record_assertion "cognee" "sse_endpoint" "false" "Cognee SSE endpoint failed: $http_code"
        record_metric "cognee_sse_status" "$http_code"
    fi
}

# Test 7: MCP initialize method works
test_mcp_initialize() {
    log_info "Test 7: Testing MCP initialize method..."

    local request='{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": "challenge-test-client",
                "version": "1.0.0"
            }
        }
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q '"jsonrpc":"2.0"'; then
            if echo "$body" | grep -q '"serverInfo"'; then
                if echo "$body" | grep -q '"helixagent-mcp"'; then
                    record_assertion "mcp" "initialize" "true" "MCP initialize returns correct server info"
                else
                    record_assertion "mcp" "initialize" "false" "MCP initialize missing server name"
                fi
            else
                record_assertion "mcp" "initialize" "false" "MCP initialize missing serverInfo"
            fi
        else
            record_assertion "mcp" "initialize" "false" "MCP initialize not JSON-RPC 2.0"
        fi
    else
        record_assertion "mcp" "initialize" "false" "MCP initialize failed: $http_code"
    fi

    record_metric "mcp_initialize_status" "$http_code"
}

# Test 8: tools/list returns tools
test_mcp_tools_list() {
    log_info "Test 8: Testing MCP tools/list method..."

    local request='{
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/list"
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q '"tools"'; then
            if echo "$body" | grep -q '"mcp_list_providers"' || echo "$body" | grep -q '"mcp_get_capabilities"'; then
                record_assertion "mcp" "tools_list" "true" "MCP tools/list returns expected tools"
            else
                record_assertion "mcp" "tools_list" "false" "MCP tools/list missing expected tools"
            fi
        else
            record_assertion "mcp" "tools_list" "false" "MCP tools/list response missing tools array"
        fi
    else
        record_assertion "mcp" "tools_list" "false" "MCP tools/list failed: $http_code"
    fi

    record_metric "mcp_tools_list_status" "$http_code"
}

# Test 9: tools/call executes tools
test_mcp_tools_call() {
    log_info "Test 9: Testing MCP tools/call method..."

    local request='{
        "jsonrpc": "2.0",
        "id": 3,
        "method": "tools/call",
        "params": {
            "name": "mcp_get_capabilities",
            "arguments": {}
        }
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q '"result"'; then
            if echo "$body" | grep -q '"content"'; then
                record_assertion "mcp" "tools_call" "true" "MCP tools/call executes correctly"
            else
                record_assertion "mcp" "tools_call" "false" "MCP tools/call response missing content"
            fi
        else
            record_assertion "mcp" "tools_call" "false" "MCP tools/call response missing result"
        fi
    else
        record_assertion "mcp" "tools_call" "false" "MCP tools/call failed: $http_code"
    fi

    record_metric "mcp_tools_call_status" "$http_code"
}

# Test 10: Error handling for unknown methods
test_mcp_error_handling() {
    log_info "Test 10: Testing MCP error handling..."

    local request='{
        "jsonrpc": "2.0",
        "id": 4,
        "method": "unknown/method"
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q '"error"'; then
            if echo "$body" | grep -q '"-32601"' || echo "$body" | grep -q '"Method not found"'; then
                record_assertion "mcp" "error_handling" "true" "MCP returns proper JSON-RPC error"
            else
                record_assertion "mcp" "error_handling" "false" "MCP error response missing error code"
            fi
        else
            record_assertion "mcp" "error_handling" "false" "MCP should return error for unknown method"
        fi
    else
        record_assertion "mcp" "error_handling" "false" "MCP error handling failed: $http_code"
    fi

    record_metric "mcp_error_handling_status" "$http_code"
}

# Test 11: Multiple protocols work concurrently
test_concurrent_protocols() {
    log_info "Test 11: Testing concurrent protocol requests..."

    local success_count=0
    local total_count=6

    # Run requests concurrently
    for protocol in mcp acp lsp embeddings vision cognee; do
        (
            local request='{
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize"
            }'
            curl -s -w "\n%{http_code}" "$BASE_URL/v1/$protocol" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d "$request" --max-time 10 > /tmp/concurrent_$protocol.txt 2>/dev/null || true
        ) &
    done

    # Wait for all to complete
    wait

    # Check results
    for protocol in mcp acp lsp embeddings vision cognee; do
        if [[ -f "/tmp/concurrent_$protocol.txt" ]]; then
            local http_code=$(tail -n1 /tmp/concurrent_$protocol.txt)
            if [[ "$http_code" == "200" ]]; then
                ((success_count++))
            fi
            rm -f /tmp/concurrent_$protocol.txt
        fi
    done

    if [[ "$success_count" -eq "$total_count" ]]; then
        record_assertion "concurrent" "all_protocols" "true" "All $total_count protocols respond concurrently"
    else
        record_assertion "concurrent" "all_protocols" "false" "Only $success_count/$total_count protocols responded"
    fi

    record_metric "concurrent_success" "$success_count"
    record_metric "concurrent_total" "$total_count"
}

# Test 12: CLI agent initialization flow (OpenCode-style)
test_cli_agent_flow() {
    log_info "Test 12: Testing CLI agent initialization flow..."

    # Step 1: Initialize
    local init_request='{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {
                "roots": {"listChanged": true},
                "sampling": {}
            },
            "clientInfo": {
                "name": "opencode-test",
                "version": "1.0.0"
            }
        }
    }'

    local init_response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$init_request" 2>/dev/null || true)
    local init_code=$(echo "$init_response" | tail -n1)

    if [[ "$init_code" != "200" ]]; then
        record_assertion "cli_agent" "init_flow" "false" "CLI agent initialize failed: $init_code"
        return
    fi

    # Step 2: Send initialized notification
    local inited_request='{
        "jsonrpc": "2.0",
        "method": "initialized"
    }'

    curl -s "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$inited_request" > /dev/null 2>&1

    # Step 3: List tools
    local tools_request='{
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/list"
    }'

    local tools_response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$tools_request" 2>/dev/null || true)
    local tools_code=$(echo "$tools_response" | tail -n1)
    local tools_body=$(echo "$tools_response" | head -n -1)

    if [[ "$tools_code" == "200" ]] && echo "$tools_body" | grep -q '"tools"'; then
        record_assertion "cli_agent" "init_flow" "true" "CLI agent initialization flow works"
    else
        record_assertion "cli_agent" "init_flow" "false" "CLI agent tools/list failed: $tools_code"
    fi

    record_metric "cli_agent_flow_status" "$tools_code"
}

# Test all protocols initialize
test_all_protocols_initialize() {
    log_info "Test 13: Testing all protocols initialize..."

    local protocols=("mcp" "acp" "lsp" "embeddings" "vision" "cognee")
    local success_count=0

    for protocol in "${protocols[@]}"; do
        local request='{
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize"
        }'

        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/$protocol" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" 2>/dev/null || true)
        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        if [[ "$http_code" == "200" ]] && echo "$body" | grep -q "helixagent-$protocol"; then
            ((success_count++))
            log_info "  $protocol initialize: PASSED"
        else
            log_warning "  $protocol initialize: FAILED (status: $http_code)"
        fi
    done

    if [[ "$success_count" -eq "${#protocols[@]}" ]]; then
        record_assertion "all_protocols" "initialize" "true" "All protocols initialize correctly"
    else
        record_assertion "all_protocols" "initialize" "false" "Only $success_count/${#protocols[@]} protocols initialized"
    fi

    record_metric "protocols_initialized" "$success_count"
}

# Test ping method for all protocols
test_all_protocols_ping() {
    log_info "Test 14: Testing all protocols ping..."

    local protocols=("mcp" "acp" "lsp" "embeddings" "vision" "cognee")
    local success_count=0

    for protocol in "${protocols[@]}"; do
        local request='{
            "jsonrpc": "2.0",
            "id": 1,
            "method": "ping"
        }'

        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/$protocol" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" 2>/dev/null || true)
        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        if [[ "$http_code" == "200" ]] && ! echo "$body" | grep -q '"error"'; then
            ((success_count++))
        fi
    done

    if [[ "$success_count" -eq "${#protocols[@]}" ]]; then
        record_assertion "all_protocols" "ping" "true" "All protocols respond to ping"
    else
        record_assertion "all_protocols" "ping" "false" "Only $success_count/${#protocols[@]} protocols respond to ping"
    fi

    record_metric "protocols_ping_success" "$success_count"
}

# Main execution
main() {
    log_info "Starting MCP SSE Connectivity Challenge..."
    log_info "Base URL: $BASE_URL"

    # Check if server is running
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_warning "HelixAgent not running, attempting to start..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
        # Wait for server to be ready
        sleep 3
    fi

    log_info "HelixAgent is running, starting tests..."

    # Run all tests
    test_mcp_sse_endpoint
    test_acp_sse_endpoint
    test_lsp_sse_endpoint
    test_embeddings_sse_endpoint
    test_vision_sse_endpoint
    test_cognee_sse_endpoint
    test_mcp_initialize
    test_mcp_tools_list
    test_mcp_tools_call
    test_mcp_error_handling
    test_concurrent_protocols
    test_cli_agent_flow
    test_all_protocols_initialize
    test_all_protocols_ping

    # Calculate results
    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo "0")
    failed_count=${failed_count:-0}

    log_info "==========================================="
    log_info "MCP SSE Connectivity Challenge Results"
    log_info "==========================================="

    if [[ "$failed_count" -eq 0 ]]; then
        log_info "ALL TESTS PASSED!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count TESTS FAILED"
        finalize_challenge "FAILED"
    fi
}

main "$@"
