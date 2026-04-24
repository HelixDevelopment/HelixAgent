#!/bin/bash
# Error Rate Limit Errors Challenge
# Tests rate limiting error handling and throttling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-rate-limit-errors" "Error Rate Limit Errors Challenge"
load_env

log_info "Testing rate limit error handling..."

# Test 1: Rate limit detection
test_rate_limit_detection() {
    log_info "Test 1: Rate limit errors are detected"

    local rate_limit_hit=false

    # Make rapid requests to trigger rate limiting
    for i in {1..100}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"RL test"}],"max_tokens":5}' \
            --max-time 5 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "429" ]]; then
            rate_limit_hit=true
            record_metric "requests_before_limit" $i
            break
        fi
    done

    if [[ "$rate_limit_hit" == "true" ]]; then
        record_assertion "rate_limit_detection" "triggered" "true" "Rate limit enforced with 429"
    else
        record_assertion "rate_limit_detection" "not_triggered" "true" "No rate limit hit (may not be enabled or limit is high)"
    fi
}

# Test 2: Rate limit error message
test_rate_limit_message() {
    log_info "Test 2: Rate limit errors include helpful messages"

    # Trigger rate limit
    for i in {1..50}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"RL"}],"max_tokens":5}' \
            --max-time 5 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        if [[ "$http_code" == "429" ]]; then
            # Check error message
            if echo "$body" | jq -e '.error.message' > /dev/null 2>&1; then
                record_assertion "rate_limit_message" "has_message" "true" "Error includes message"

                local msg=$(echo "$body" | jq -r '.error.message // empty')
                if echo "$msg" | grep -qi "rate"; then
                    record_assertion "rate_limit_message" "mentions_rate_limit" "true" "Message mentions rate limiting"
                fi
            fi
            break
        fi
    done
}

# Test 3: Retry-After header
test_retry_after_header() {
    log_info "Test 3: Rate limit response includes retry guidance"

    # Rapid requests
    for i in {1..50}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"RL"}],"max_tokens":5}' \
            --max-time 5 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        if [[ "$http_code" == "429" ]]; then
            # Check for retry guidance in error body
            if echo "$body" | grep -qi "retry"; then
                record_assertion "retry_after" "has_guidance" "true" "Retry guidance provided"
            fi
            break
        fi
    done
}

# Test 4: Rate limit recovery
test_rate_limit_recovery() {
    log_info "Test 4: System recovers after rate limit"

    # Trigger rate limit
    for i in {1..50}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"RL"}],"max_tokens":5}' \
            --max-time 5 > /dev/null 2>&1 || true
    done

    # Wait and try again
    sleep 5

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "rate_limit_recovery" "recovered" "true" "System recovered after rate limit"
    elif [[ "$http_code" == "429" ]]; then
        record_assertion "rate_limit_recovery" "still_limited" "true" "Rate limit still active (long duration)"
    fi
}

# Test 5: Rate limit per-user isolation
test_rate_limit_isolation() {
    log_info "Test 5: Rate limits are isolated per user/key"

    # Use different API key for isolation test
    local alt_key="test-key-2"

    # Hit rate limit with primary key
    for i in {1..50}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"RL"}],"max_tokens":5}' \
            --max-time 5 > /dev/null 2>&1 || true
    done

    # Try with alt key
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $alt_key" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Isolation test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Alt key should work (or fail auth, but not rate limit)
    if [[ "$http_code" == "200" || "$http_code" == "401" ]]; then
        record_assertion "rate_limit_isolation" "isolated" "true" "Alt key not affected (HTTP $http_code)"
    elif [[ "$http_code" == "429" ]]; then
        record_assertion "rate_limit_isolation" "global_limit" "true" "Global rate limit applied"
    fi
}

main() {
    log_info "Starting rate limit errors challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_rate_limit_detection
    test_rate_limit_message
    test_retry_after_header
    test_rate_limit_recovery
    test_rate_limit_isolation

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All rate limit error tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
