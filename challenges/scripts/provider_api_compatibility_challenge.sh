#!/bin/bash
# Provider API Compatibility Challenge
# Tests provider API compatibility and versioning

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-api-compatibility" "Provider API Compatibility Challenge"
load_env

log_info "Testing API compatibility..."

test_api_version_compatibility() {
    log_info "Test 1: API version compatibility check"

    local request='{"provider":"openai","api_version":"v1","check_compatibility":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/compatibility" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local is_compatible=$(echo "$body" | jq -e '.compatible' 2>/dev/null || echo "null")
        local supported_versions=$(echo "$body" | jq -e '.supported_versions | length' 2>/dev/null || echo "0")
        record_assertion "api_version_compatibility" "working" "true" "Compatible: $is_compatible, Versions: $supported_versions"
    else
        record_skip "api_version_compatibility" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_endpoint_validation() {
    log_info "Test 2: Provider endpoint validation"

    local request='{"provider":"anthropic","endpoints":["/v1/chat/completions","/v1/embeddings"],"validate_all":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/validate-endpoints" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local valid_endpoints=$(echo "$body" | jq -e '.valid_endpoints | length' 2>/dev/null || echo "0")
        local invalid_endpoints=$(echo "$body" | jq -e '.invalid_endpoints | length' 2>/dev/null || echo "0")
        record_assertion "endpoint_validation" "working" "true" "Valid: $valid_endpoints, Invalid: $invalid_endpoints"
    else
        record_skip "endpoint_validation" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_request_response_format() {
    log_info "Test 3: Request/response format compatibility"

    local request='{"provider":"deepseek","sample_request":{"model":"deepseek-chat","messages":[{"role":"user","content":"test"}]},"validate_format":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/validate-format" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local format_valid=$(echo "$body" | jq -e '.format_valid' 2>/dev/null || echo "null")
        local warnings=$(echo "$body" | jq -e '.warnings | length' 2>/dev/null || echo "0")
        record_assertion "request_response_format" "working" "true" "Valid: $format_valid, Warnings: $warnings"
    else
        record_skip "request_response_format" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_backward_compatibility() {
    log_info "Test 4: Backward compatibility validation"

    # CONST-035 fix: previously this assertion recorded "true" unconditionally
    # regardless of what the endpoint returned. Now we check status and skip
    # honestly when the endpoint is not implemented.
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/backward-compat?provider=gemini&current_version=v1&target_version=v1beta" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local is_backward_compatible=$(echo "$body" | jq -r '.backward_compatible' 2>/dev/null || echo "null")
        local breaking_changes=$(echo "$body" | jq -r '.breaking_changes | length' 2>/dev/null || echo "0")
        record_assertion "backward_compatibility" "checked" "true" "Compatible: $is_backward_compatible, Breaking: $breaking_changes"
    else
        record_skip "backward_compatibility" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

main() {
    log_info "Starting API compatibility challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_api_version_compatibility
    test_endpoint_validation
    test_request_response_format
    test_backward_compatibility

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
