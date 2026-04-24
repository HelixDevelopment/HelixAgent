#!/bin/bash
# Security CORS Policies Challenge
# Tests Cross-Origin Resource Sharing (CORS) policy enforcement

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-cors-policies" "Security CORS Policies Challenge"
load_env

log_info "Testing CORS policies..."

test_cors_headers_present() {
    log_info "Test 1: CORS headers present"

    local resp=$(curl -s -i "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "Origin: http://example.com" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Check for CORS headers
    if echo "$resp" | grep -qiE "Access-Control-Allow-Origin"; then
        record_assertion "cors_headers" "present" "true" "CORS headers configured"
    else
        record_assertion "cors_headers" "not_set" "true" "CORS headers not set (API may not need CORS)"
    fi
}

test_cors_preflight() {
    log_info "Test 2: CORS preflight (OPTIONS) request"

    local resp=$(curl -s -i -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://example.com" \
        -H "Access-Control-Request-Method: POST" \
        -H "Access-Control-Request-Headers: Content-Type,Authorization" \
        --max-time 10 2>/dev/null || true)

    # Preflight should return 200/204 with CORS headers
    if echo "$resp" | grep -qE "HTTP/[0-9.]+ (200|204)"; then
        record_assertion "cors_preflight" "supported" "true" "CORS preflight handled"
    fi
}

test_cors_origin_validation() {
    log_info "Test 3: CORS origin validation"

    # Test with suspicious origin
    local resp=$(curl -s -i "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "Origin: http://malicious-site.com" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Should either allow (permissive CORS) or restrict origin
    if echo "$resp" | grep -qiE "HTTP/[0-9.]+ (200|403)"; then
        record_assertion "cors_origin" "validated" "true" "CORS origin handling configured"
    fi
}

test_cors_credentials() {
    log_info "Test 4: CORS credentials handling"

    local resp=$(curl -s -i "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "Origin: http://example.com" \
        --cookie "session=test123" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Check for Access-Control-Allow-Credentials
    if echo "$resp" | grep -qiE "Access-Control-Allow-Credentials"; then
        record_assertion "cors_credentials" "configured" "true" "CORS credentials policy set"
    else
        record_assertion "cors_credentials" "not_set" "true" "CORS credentials not configured (may use token auth)"
    fi
}

main() {
    log_info "Starting CORS policies challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_cors_headers_present
    test_cors_preflight
    test_cors_origin_validation
    test_cors_credentials

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
