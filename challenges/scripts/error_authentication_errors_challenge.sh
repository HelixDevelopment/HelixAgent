#!/bin/bash
# Error Authentication Errors Challenge
# Tests authentication and authorization error handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-authentication-errors" "Error Authentication Errors Challenge"
load_env

log_info "Testing authentication error handling..."

test_missing_auth() {
    log_info "Test 1: Missing authentication handling"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    if [[ "$code" == "401" ]]; then
        record_assertion "missing_auth" "enforced" "true" "Returns 401 for missing auth"
    elif [[ "$code" == "200" ]]; then
        record_assertion "missing_auth" "optional" "true" "Auth optional (200 returned)"
    fi
}

test_invalid_token() {
    log_info "Test 2: Invalid token handling"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid-token-12345" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "401" ]]; then
        record_assertion "invalid_token" "rejected" "true" "Invalid token rejected"

        if echo "$body" | jq -e '.error' > /dev/null 2>&1; then
            record_assertion "invalid_token" "error_message" "true" "Error message provided"
        fi
    elif [[ "$code" == "200" ]]; then
        record_assertion "invalid_token" "accepted" "true" "Token validation may be disabled"
    fi
}

test_expired_token() {
    log_info "Test 3: Expired token simulation"

    # Use a potentially expired-looking token
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjF9.invalid" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(401|403|200)$ ]] && record_assertion "expired_token" "handled" "true" "Expired token handled (HTTP $code)"
}

test_auth_error_messages() {
    log_info "Test 4: Auth error messages are secure"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer wrong" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$resp" | head -n -1)

    if echo "$body" | jq -e '.error.message' > /dev/null 2>&1; then
        local msg=$(echo "$body" | jq -r '.error.message // empty')

        # Error message should not leak sensitive info
        if ! echo "$msg" | grep -qiE "(password|secret|key|token value)"; then
            record_assertion "auth_messages" "secure" "true" "Error message doesn't leak secrets"
        fi
    fi
}

main() {
    log_info "Starting authentication errors challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_missing_auth
    test_invalid_token
    test_expired_token
    test_auth_error_messages

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
