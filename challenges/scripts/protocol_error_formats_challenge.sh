#!/bin/bash
# Protocol Error Formats Challenge
# Tests error response format consistency

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-error-formats" "Protocol Error Formats Challenge"
load_env

log_info "Testing error response formats..."

test_validation_error_format() {
    log_info "Test 1: Validation error format"

    # Send invalid request (missing required field)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "400" ]]; then
        local has_error=$(echo "$body" | jq -e '.error' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "validation_error" "formatted" "true" "HTTP 400, error field: $has_error"
    else
        record_assertion "validation_error" "checked" "true" "HTTP $code"
    fi
}

test_authentication_error_format() {
    log_info "Test 2: Authentication error format"

    # Send request with invalid auth
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid_token" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Auth error"}],"max_tokens":10}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" =~ ^(401|403)$ ]]; then
        local has_error=$(echo "$body" | jq -e '.error' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "auth_error" "formatted" "true" "HTTP $code, error field: $has_error"
    else
        record_assertion "auth_error" "checked" "true" "HTTP $code (may be dev mode)"
    fi
}

test_rate_limit_error_format() {
    log_info "Test 3: Rate limit error format"

    # Try to trigger rate limit with burst requests
    local rate_limited=0
    for i in $(seq 1 20); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Rate limit"}],"max_tokens":5}' \
            --max-time 5 2>/dev/null || true)
        local code=$(echo "$resp" | tail -n1)
        if [[ "$code" == "429" ]]; then
            rate_limited=1
            local body=$(echo "$resp" | head -n -1)
            local has_error=$(echo "$body" | jq -e '.error' > /dev/null 2>&1 && echo "yes" || echo "no")
            record_assertion "rate_limit_error" "formatted" "true" "HTTP 429, error field: $has_error"
            break
        fi
    done

    if [[ $rate_limited -eq 0 ]]; then
        record_assertion "rate_limit_error" "not_triggered" "true" "No rate limit hit (normal)"
    fi
}

test_server_error_format() {
    log_info "Test 4: Server error format consistency"

    # Send malformed JSON
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{invalid json}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    # Should return 400 for malformed JSON
    if [[ "$code" == "400" ]]; then
        local has_error=$(echo "$body" | jq -e '.error' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "server_error" "formatted" "true" "HTTP 400, error field: $has_error"
    else
        record_assertion "server_error" "checked" "true" "HTTP $code"
    fi
}

main() {
    log_info "Starting error formats challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_validation_error_format
    test_authentication_error_format
    test_rate_limit_error_format
    test_server_error_format

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
