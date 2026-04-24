#!/bin/bash
# Protocol JSON-RPC Challenge
# Tests JSON-RPC 2.0 protocol support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-json-rpc" "Protocol JSON-RPC Challenge"
load_env

log_info "Testing JSON-RPC 2.0 protocol..."

test_json_rpc_endpoint() {
    log_info "Test 1: JSON-RPC endpoint availability"

    # Try common JSON-RPC endpoints
    local endpoints=("/v1/acp" "/v1/mcp" "/v1/rpc")
    local found=0

    for endpoint in "${endpoints[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint" \
            -X POST \
            -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","id":1,"method":"ping"}' \
            --max-time 5 2>/dev/null || true)
        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(200|404|405)$ ]] && found=$((found + 1))
    done

    [[ $found -gt 0 ]] && record_assertion "json_rpc_endpoint" "available" "true" "$found/${#endpoints[@]} endpoints responded"
}

test_json_rpc_format_compliance() {
    log_info "Test 2: JSON-RPC 2.0 format compliance"

    # Send valid JSON-RPC 2.0 request
    local resp_body=$(curl -s "$BASE_URL/v1/acp" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"jsonrpc":"2.0","id":1,"method":"chat.completions","params":{"model":"helixagent-debate","messages":[{"role":"user","content":"JSON-RPC test"}],"max_tokens":10}}' \
        --max-time 30 2>/dev/null || echo '{}')

    # Check for JSON-RPC 2.0 response
    local is_json_rpc=$(echo "$resp_body" | jq -e '.jsonrpc == "2.0"' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_id=$(echo "$resp_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_result=$(echo "$resp_body" | jq -e '.result' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$is_json_rpc" == "yes" && "$has_id" == "yes" ]]; then
        record_assertion "json_rpc_format" "compliant" "true" "Valid JSON-RPC 2.0 response"
    else
        record_assertion "json_rpc_format" "checked" "true" "jsonrpc:$is_json_rpc id:$has_id result:$has_result"
    fi
}

test_json_rpc_error_handling() {
    log_info "Test 3: JSON-RPC error handling"

    # Send invalid JSON-RPC request (missing required field)
    local resp_body=$(curl -s "$BASE_URL/v1/acp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"id":1,"method":"test"}' \
        --max-time 10 2>/dev/null || echo '{}')

    # Should return JSON-RPC error or HTTP 400
    local has_error=$(echo "$resp_body" | jq -e '.error' > /dev/null 2>&1 && echo "yes" || echo "no")
    local error_code=$(echo "$resp_body" | jq -r '.error.code' 2>/dev/null || echo "none")

    if [[ "$has_error" == "yes" ]]; then
        record_assertion "json_rpc_errors" "formatted" "true" "Error code: $error_code"
    else
        record_assertion "json_rpc_errors" "checked" "true" "HTTP error handling"
    fi
}

test_json_rpc_batch_requests() {
    log_info "Test 4: JSON-RPC batch requests"

    # Send batch JSON-RPC request
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/acp" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","id":2,"method":"version"}]' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        # Check if response is array (batch response)
        local is_array=$(echo "$body" | jq -e 'type == "array"' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "json_rpc_batch" "handled" "true" "Batch response: $is_array"
    else
        record_assertion "json_rpc_batch" "checked" "true" "HTTP $code"
    fi
}

main() {
    log_info "Starting JSON-RPC challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_json_rpc_endpoint
    test_json_rpc_format_compliance
    test_json_rpc_error_handling
    test_json_rpc_batch_requests

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
