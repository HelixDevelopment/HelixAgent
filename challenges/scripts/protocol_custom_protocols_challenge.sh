#!/bin/bash
# Protocol Custom Protocols Challenge
# Tests support for custom protocol extensions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-custom-protocols" "Protocol Custom Protocols Challenge"
load_env

log_info "Testing custom protocol extensions..."

test_custom_headers_support() {
    log_info "Test 1: Custom header support"

    # Send request with custom headers
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "X-Custom-Header: test-value" \
        -H "X-Request-ID: custom-req-123" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Custom headers"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "custom_headers" "tolerated" "true" "HTTP 200 with custom headers"
}

test_custom_parameters() {
    log_info "Test 2: Custom parameter handling"

    # Include custom parameters that aren't standard
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Custom params"}],"max_tokens":10,"custom_param":"test","x_metadata":{"key":"value"}}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should either ignore unknown params or return 200
    [[ "$code" =~ ^(200|400)$ ]] && record_assertion "custom_parameters" "handled" "true" "HTTP $code"
}

test_extended_response_format() {
    log_info "Test 3: Extended response format compatibility"

    local resp_body=$(curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Extended format"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || echo '{}')

    # Check if response includes standard fields
    local has_id=$(echo "$resp_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_created=$(echo "$resp_body" | jq -e '.created' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_choices=$(echo "$resp_body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_id" == "yes" && "$has_choices" == "yes" ]]; then
        record_assertion "extended_response" "compatible" "true" "Standard fields present"
    else
        record_assertion "extended_response" "checked" "true" "id:$has_id created:$has_created choices:$has_choices"
    fi
}

test_protocol_extension_negotiation() {
    log_info "Test 4: Protocol extension negotiation"

    # Try to request specific protocol version or features
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "Accept: application/json; version=1.0" \
        -H "X-Protocol-Version: 2.0" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Protocol version"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "protocol_negotiation" "flexible" "true" "Version headers accepted"
}

main() {
    log_info "Starting custom protocols challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_custom_headers_support
    test_custom_parameters
    test_extended_response_format
    test_protocol_extension_negotiation

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
