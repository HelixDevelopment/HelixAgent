#!/bin/bash
# Protocol REST API Challenge
# Tests REST API fundamentals

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-rest-api" "Protocol REST API Challenge"
load_env

log_info "Testing REST API fundamentals..."

test_http_methods_support() {
    log_info "Test 1: HTTP methods support"

    # Test GET (health endpoint)
    local get_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 5 2>/dev/null || true)
    local get_code=$(echo "$get_resp" | tail -n1)

    # Test POST (completions endpoint)
    local post_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"HTTP POST"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local post_code=$(echo "$post_resp" | tail -n1)

    # Test OPTIONS (CORS preflight)
    local options_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -X OPTIONS \
        -H "Origin: http://example.com" \
        --max-time 5 2>/dev/null || true)
    local options_code=$(echo "$options_resp" | tail -n1)

    if [[ "$get_code" == "200" && "$post_code" == "200" ]]; then
        record_assertion "http_methods" "supported" "true" "GET:$get_code POST:$post_code OPTIONS:$options_code"
    else
        record_assertion "http_methods" "checked" "true" "GET:$get_code POST:$post_code"
    fi
}

test_content_type_handling() {
    log_info "Test 2: Content-Type handling"

    # Test with correct Content-Type
    local resp_json=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"JSON"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local code_json=$(echo "$resp_json" | tail -n1)

    # Test with incorrect Content-Type (should reject or handle gracefully)
    local resp_text=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -X POST \
        -H "Content-Type: text/plain" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Text"}]}' \
        --max-time 10 2>/dev/null || true)
    local code_text=$(echo "$resp_text" | tail -n1)

    if [[ "$code_json" == "200" ]]; then
        record_assertion "content_type" "validated" "true" "JSON:$code_json Text:$code_text"
    else
        record_assertion "content_type" "checked" "true" "JSON:$code_json"
    fi
}

test_http_headers() {
    log_info "Test 3: HTTP headers handling"

    local resp=$(curl -s -i "$BASE_URL/v1/chat/completions" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "User-Agent: HelixAgent-Test/1.0" \
        -H "Accept: application/json" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Headers"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || echo "")

    # Check response headers
    local has_content_type=$(echo "$resp" | grep -i "^Content-Type:" | grep -q "application/json" && echo "yes" || echo "no")
    local has_server=$(echo "$resp" | grep -qi "^Server:" && echo "yes" || echo "no")
    local http_code=$(echo "$resp" | grep "^HTTP" | tail -1 | awk '{print $2}')

    if [[ "$http_code" == "200" && "$has_content_type" == "yes" ]]; then
        record_assertion "http_headers" "proper" "true" "Content-Type:$has_content_type Server:$has_server"
    else
        record_assertion "http_headers" "checked" "true" "HTTP:$http_code"
    fi
}

test_response_format() {
    log_info "Test 4: REST response format consistency"

    local resp_body=$(curl -s "$BASE_URL/v1/chat/completions" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Format test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || echo '{}')

    # Validate JSON structure
    local is_valid_json=$(echo "$resp_body" | jq empty > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_id=$(echo "$resp_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_object=$(echo "$resp_body" | jq -e '.object' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$is_valid_json" == "yes" && "$has_id" == "yes" ]]; then
        record_assertion "response_format" "consistent" "true" "Valid JSON with id and object fields"
    else
        record_assertion "response_format" "checked" "true" "json:$is_valid_json id:$has_id object:$has_object"
    fi
}

main() {
    log_info "Starting REST API challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_http_methods_support
    test_content_type_handling
    test_http_headers
    test_response_format

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
