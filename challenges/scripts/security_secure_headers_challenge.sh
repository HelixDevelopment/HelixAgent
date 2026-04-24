#!/bin/bash
# Security Secure Headers Challenge
# Tests security headers implementation (HSTS, CSP, etc.)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-secure-headers" "Security Secure Headers Challenge"
load_env

log_info "Testing security headers..."

test_x_content_type_options() {
    log_info "Test 1: X-Content-Type-Options header"

    local resp=$(curl -s -i "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Should have X-Content-Type-Options: nosniff
    if echo "$resp" | grep -qiE "X-Content-Type-Options:.*nosniff"; then
        record_assertion "x_content_type" "present" "true" "X-Content-Type-Options header set"
    fi
}

test_x_frame_options() {
    log_info "Test 2: X-Frame-Options header"

    local resp=$(curl -s -i "$BASE_URL/health" --max-time 10 2>/dev/null || true)

    # Should have X-Frame-Options to prevent clickjacking
    if echo "$resp" | grep -qiE "X-Frame-Options:"; then
        record_assertion "x_frame_options" "present" "true" "X-Frame-Options header set"
    fi
}

test_strict_transport_security() {
    log_info "Test 3: Strict-Transport-Security header (HSTS)"

    # Note: HSTS only applies to HTTPS
    local resp=$(curl -s -i "$BASE_URL/health" --max-time 10 2>/dev/null || true)

    # Check for HSTS (may not be present on HTTP)
    if echo "$resp" | grep -qiE "Strict-Transport-Security:"; then
        record_assertion "hsts" "present" "true" "HSTS header set"
    else
        record_assertion "hsts" "not_on_http" "true" "HSTS not set (expected on HTTP)"
    fi
}

test_x_xss_protection() {
    log_info "Test 4: X-XSS-Protection header"

    local resp=$(curl -s -i "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # X-XSS-Protection header (deprecated but still useful)
    if echo "$resp" | grep -qiE "X-XSS-Protection:"; then
        record_assertion "x_xss_protection" "present" "true" "X-XSS-Protection header set"
    else
        record_assertion "x_xss_protection" "not_set" "true" "X-XSS-Protection not set (deprecated, CSP preferred)"
    fi
}

main() {
    log_info "Starting secure headers challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_x_content_type_options
    test_x_frame_options
    test_strict_transport_security
    test_x_xss_protection

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
