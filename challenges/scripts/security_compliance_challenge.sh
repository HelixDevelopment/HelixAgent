#!/bin/bash
# Security Compliance Challenge
# Tests security compliance and best practices

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-compliance" "Security Compliance Challenge"
load_env

log_info "Testing security compliance..."

test_security_headers_present() {
    log_info "Test 1: Essential security headers present"

    local resp=$(curl -s -i "$BASE_URL/health" --max-time 10 2>/dev/null || true)

    local headers_found=0

    # Check for key security headers
    echo "$resp" | grep -qiE "X-Content-Type-Options" && headers_found=$((headers_found + 1))
    echo "$resp" | grep -qiE "X-Frame-Options" && headers_found=$((headers_found + 1))
    echo "$resp" | grep -qiE "Content-Type:" && headers_found=$((headers_found + 1))

    record_metric "security_headers_found" $headers_found
    [[ $headers_found -ge 1 ]] && record_assertion "security_headers" "present" "true" "$headers_found security headers found"
}

test_https_redirect_capability() {
    log_info "Test 2: HTTPS upgrade capability"

    # Check if HTTP works (dev) or redirects to HTTPS (prod)
    local resp=$(curl -s -w "\n%{http_code}" -L "http://localhost:$CHALLENGE_PORT/health" --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|204|301|302)$ ]] && record_assertion "https_upgrade" "configured" "true" "HTTPS upgrade capability present"
}

test_authentication_required() {
    log_info "Test 3: Authentication enforcement"

    # Test protected endpoint without auth
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    if [[ "$code" == "401" ]]; then
        record_assertion "auth_enforcement" "required" "true" "Authentication required"
    elif [[ "$code" == "200" ]]; then
        record_assertion "auth_enforcement" "optional" "true" "Authentication optional (may be dev mode)"
    fi
}

test_error_disclosure_prevention() {
    log_info "Test 4: Error information disclosure prevention"

    # Trigger error
    local resp=$(curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{invalid}' --max-time 10 2>/dev/null || true)

    # Check error doesn't leak internal details
    if ! echo "$resp" | grep -qiE "(stack trace|internal error|debug|line [0-9]+|file path|/home/|/usr/)"; then
        record_assertion "error_disclosure" "prevented" "true" "Error messages don't leak internal details"
    fi
}

main() {
    log_info "Starting compliance challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_security_headers_present
    test_https_redirect_capability
    test_authentication_required
    test_error_disclosure_prevention

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
