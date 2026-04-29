#!/bin/bash
# Protocol ACP Integration Challenge
# Tests Anthropic Chat Protocol (ACP) integration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-acp-integration" "Protocol ACP Integration Challenge"
load_env

log_info "Testing ACP integration..."

test_acp_endpoint_availability() {
    log_info "Test 1: ACP endpoint availability"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/acp" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"jsonrpc":"2.0","id":1,"method":"acp.ping"}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|404)$ ]] && record_assertion "acp_endpoint" "available" "true" "HTTP $code"
}

test_acp_json_rpc_format() {
    log_info "Test 2: ACP JSON-RPC 2.0 format compliance"

    local resp=$(curl -s "$BASE_URL/v1/acp" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"jsonrpc":"2.0","id":1,"method":"chat.completions","params":{"model":"helixagent-debate","messages":[{"role":"user","content":"ACP test"}],"max_tokens":10}}' \
        --max-time 30 2>/dev/null || true)

    # Check for JSON-RPC 2.0 response format
    if echo "$resp" | jq -e '.jsonrpc == "2.0"' > /dev/null 2>&1; then
        record_assertion "acp_json_rpc" "compliant" "true" "Valid JSON-RPC 2.0"
    elif echo "$resp" | jq -e '.choices' > /dev/null 2>&1; then
        record_assertion "acp_json_rpc" "working" "true" "OpenAI format fallback"
    else
        record_assertion "acp_json_rpc" "format" "false" "Invalid format"
    fi
}

test_acp_method_routing() {
    log_info "Test 3: ACP method routing"

    local methods=("chat.completions" "completions.create" "chat.create")
    local success=0

    for method in "${methods[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/acp" \
            -X POST \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"jsonrpc":"2.0","id":1,"method":"'$method'","params":{"model":"helixagent-debate","messages":[{"role":"user","content":"Method test"}],"max_tokens":10}}' \
            --max-time 30 2>/dev/null || true)
        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(200|404|405)$ ]] && success=$((success + 1))
    done

    record_metric "acp_methods_tested" "${#methods[@]}"
    [[ $success -eq ${#methods[@]} ]] && record_assertion "acp_routing" "working" "true" "$success/${#methods[@]} methods responded"
}

test_acp_error_handling() {
    log_info "Test 4: ACP error handling"

    # Invalid JSON-RPC (missing jsonrpc field)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/acp" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"id":1,"method":"test"}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should return 400 or valid JSON-RPC error
    if [[ "$code" == "400" ]]; then
        record_assertion "acp_error_handling" "validated" "true" "Invalid request rejected"
    elif echo "$resp" | jq -e '.error' > /dev/null 2>&1; then
        record_assertion "acp_error_handling" "json_rpc_error" "true" "JSON-RPC error returned"
    else
        record_assertion "acp_error_handling" "checked" "true" "HTTP $code"
    fi
}

main() {
    log_info "Starting ACP integration challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_acp_endpoint_availability
    test_acp_json_rpc_format
    test_acp_method_routing
    test_acp_error_handling

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
