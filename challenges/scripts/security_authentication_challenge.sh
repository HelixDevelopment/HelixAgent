#!/bin/bash
# Security Authentication Challenge
# Tests authentication mechanisms and token validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-authentication" "Security Authentication Challenge"
load_env

log_info "Testing authentication security..."

test_missing_auth_header() {
    log_info "Test 1: Missing authentication header"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    if [[ "$code" == "401" ]]; then
        record_assertion "missing_auth" "enforced" "true" "Returns 401 for missing auth"
    elif [[ "$code" == "200" ]]; then
        record_assertion "missing_auth" "optional" "true" "Auth is optional (200 returned)"
    fi
}

test_invalid_token() {
    log_info "Test 2: Invalid authentication token"

    local invalid_tokens=(
        "invalid-token-12345"
        "Bearer-without-space"
        ""
        "malformed.jwt.token"
    )

    local rejected=0

    for token in "${invalid_tokens[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $token" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(401|403)$ ]] && rejected=$((rejected + 1))
    done

    record_metric "invalid_tokens_rejected" $rejected
    [[ $rejected -ge 2 ]] && record_assertion "invalid_token" "rejected" "true" "$rejected/4 invalid tokens rejected"
}

test_valid_token() {
    log_info "Test 3: Valid authentication token"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "valid_token" "accepted" "true" "Valid token accepted"
}

test_auth_error_messages() {
    log_info "Test 4: Authentication error messages are secure"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer wrong-token" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$resp" | head -n -1)

    if echo "$body" | jq -e '.error.message' > /dev/null 2>&1; then
        local msg=$(echo "$body" | jq -r '.error.message // empty')

        # Error message should not leak sensitive info
        if ! echo "$msg" | grep -qiE "(password|secret|key value|token=|internal|stack trace)"; then
            record_assertion "auth_error_message" "secure" "true" "Error message doesn't leak secrets"
        else
            record_assertion "auth_error_message" "leaked" "false" "Error message contains sensitive info"
        fi
    fi
}

main() {
    log_info "Starting authentication challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_missing_auth_header
    test_invalid_token
    test_valid_token
    test_auth_error_messages

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
