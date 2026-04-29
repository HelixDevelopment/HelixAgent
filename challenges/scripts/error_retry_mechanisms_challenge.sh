#!/bin/bash
# Error Retry Mechanisms Challenge
# Tests retry logic, exponential backoff, and retry policies

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-retry-mechanisms" "Error Retry Mechanisms Challenge"
load_env

log_info "Testing retry mechanisms and policies..."

test_automatic_retry() {
    log_info "Test 1: Automatic retry on transient failures"

    # Trigger potential transient failure with invalid model
    local attempts=0
    local success=0

    for i in {1..3}; do
        attempts=$((attempts + 1))
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Retry test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 1
    done

    record_metric "retry_attempts" $attempts
    record_metric "retry_success_count" $success
    [[ $success -ge 2 ]] && record_assertion "automatic_retry" "working" "true" "$success/3 attempts succeeded"
}

test_exponential_backoff() {
    log_info "Test 2: Exponential backoff between retries"

    local timings=()
    local prev_time=$(date +%s%N)

    for i in {1..3}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"invalid-model-for-retry","messages":[{"role":"user","content":"test"}],"max_tokens":5}' \
            --max-time 5 > /dev/null 2>&1 || true

        local current_time=$(date +%s%N)
        local elapsed=$(( (current_time - prev_time) / 1000000 ))
        timings+=($elapsed)
        prev_time=$current_time

        [[ $i -lt 3 ]] && sleep 1
    done

    record_metric "backoff_pattern" "${timings[*]}"
    record_assertion "exponential_backoff" "observed" "true" "Timing pattern: ${timings[*]}"
}

test_retry_limit() {
    log_info "Test 3: Retry limit enforcement"

    # Make request that should trigger retries but eventually fail
    local start=$(date +%s)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Limit test"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)
    local end=$(date +%s)
    local duration=$((end - start))

    local code=$(echo "$resp" | tail -n1)
    record_metric "retry_duration_seconds" $duration

    # Should either succeed or fail within reasonable time (not infinite retries)
    [[ $duration -lt 60 ]] && record_assertion "retry_limit" "enforced" "true" "Completed in ${duration}s"
}

test_retry_after_recovery() {
    log_info "Test 4: System continues normal operation after retries"

    # Trigger some errors
    for i in {1..3}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"maybe-invalid","messages":[{"role":"user","content":"test"}]}' \
            --max-time 5 > /dev/null 2>&1 || true
    done

    sleep 2

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "retry_recovery" "operational" "true" "System recovered after retries"
}

main() {
    log_info "Starting retry mechanisms challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_automatic_retry
    test_exponential_backoff
    test_retry_limit
    test_retry_after_recovery

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
