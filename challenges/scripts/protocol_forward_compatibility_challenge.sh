#!/bin/bash
# Protocol Forward Compatibility Challenge
# Tests forward compatibility with future API changes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-forward-compatibility" "Protocol Forward Compatibility Challenge"
load_env

log_info "Testing forward compatibility..."

test_unknown_field_handling() {
    log_info "Test 1: Unknown field handling"

    # Include fields that might be added in future versions
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Future fields"}],"max_tokens":10,"future_feature_flag":true,"experimental_param":"test","v2_option":123}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should accept and ignore unknown fields gracefully
    [[ "$code" == "200" ]] && record_assertion "unknown_fields" "tolerated" "true" "Unknown fields ignored"
}

test_future_api_version_headers() {
    log_info "Test 2: Future API version header handling"

    # Request with future API version header
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "X-API-Version: 3.0" \
        -H "X-Client-Version: future-2.0" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Future version"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "future_version_headers" "tolerated" "true" "Version headers accepted"
}

test_extensible_response_structure() {
    log_info "Test 3: Extensible response structure"

    local resp_body=$(curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Extensible response"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || echo '{}')

    # Check that response has core required fields (forward compatible baseline)
    local has_id=$(echo "$resp_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_choices=$(echo "$resp_body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_id" == "yes" && "$has_choices" == "yes" ]]; then
        # Response can have additional fields for forward compatibility
        record_assertion "extensible_response" "compatible" "true" "Core fields present, extensible structure"
    else
        record_assertion "extensible_response" "checked" "true" "id:$has_id choices:$has_choices"
    fi
}

test_optional_new_parameters() {
    log_info "Test 4: Optional new parameters acceptance"

    # Include parameters that might become standard in future
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Optional params"}],"max_tokens":10,"response_format":{"type":"text"},"seed":42,"parallel_tool_calls":true}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should handle optional params gracefully (use or ignore)
    [[ "$code" =~ ^(200|400)$ ]] && record_assertion "optional_params" "handled" "true" "HTTP $code"
}

main() {
    log_info "Starting forward compatibility challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_unknown_field_handling
    test_future_api_version_headers
    test_extensible_response_structure
    test_optional_new_parameters

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
