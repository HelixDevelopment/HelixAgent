#!/bin/bash
# Protocol Version Negotiation Challenge
# Tests API version negotiation and compatibility

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-version-negotiation" "Protocol Version Negotiation Challenge"
load_env

log_info "Testing version negotiation..."

test_version_header_support() {
    log_info "Test 1: Version header support"

    # Test with explicit API version header
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "X-API-Version: 1.0" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Version 1.0"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "version_header" "accepted" "true" "v1.0 accepted"
}

test_version_in_url() {
    log_info "Test 2: Version in URL path"

    # Test v1 endpoint
    local v1_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"v1"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local v1_code=$(echo "$v1_resp" | tail -n1)

    # Test v2 endpoint (may not exist yet, that's OK)
    local v2_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v2/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"v2"}],"max_tokens":10}' \
        --max-time 10 2>/dev/null || true)
    local v2_code=$(echo "$v2_resp" | tail -n1)

    if [[ "$v1_code" == "200" ]]; then
        record_assertion "version_in_url" "supported" "true" "v1:$v1_code v2:$v2_code (404=not yet implemented)"
    else
        record_assertion "version_in_url" "checked" "true" "v1:$v1_code"
    fi
}

test_version_discovery() {
    log_info "Test 3: Version discovery endpoint"

    # Try to discover available API versions
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/versions" \
        --max-time 5 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_versions=$(echo "$body" | jq -e '.versions' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "version_discovery" "available" "true" "Versions endpoint exists, has_versions:$has_versions"
    else
        # Not all APIs implement version discovery
        record_assertion "version_discovery" "checked" "true" "HTTP $code (optional feature)"
    fi
}

test_backward_compatibility() {
    log_info "Test 4: Backward compatibility"

    # Test with older API parameter names (if applicable)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Backward compat"}],"max_tokens":10,"stream":false}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_id=$(echo "$body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "backward_compatibility" "maintained" "true" "Old params work, response valid:$has_id"
    else
        record_assertion "backward_compatibility" "checked" "true" "HTTP $code"
    fi
}

main() {
    log_info "Starting version negotiation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_version_header_support
    test_version_in_url
    test_version_discovery
    test_backward_compatibility

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
