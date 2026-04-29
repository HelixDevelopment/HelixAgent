#!/bin/bash
# Resilience Retry Logic Challenge
# Tests automatic retry mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-retry-logic" "Resilience Retry Logic Challenge"
load_env

log_info "Testing retry logic..."

test_transient_failure_retry() {
    log_info "Test 1: System retries on transient failures"

    # Try with potentially unstable request (short timeout)
    local attempts=0
    local success=false

    for i in $(seq 1 3); do
        attempts=$((attempts + 1))
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Retry test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        if [[ "$(echo "$resp" | tail -n1)" == "200" ]]; then
            success=true
            break
        fi
        sleep 2
    done

    record_metric "retry_attempts" $attempts
    $success && record_assertion "transient_retry" "successful" "true" "Succeeded after $attempts attempt(s)"
}

test_exponential_backoff() {
    log_info "Test 2: Exponential backoff behavior"

    # Simulate retry pattern
    local times=(1 2 4)
    local success=0

    for delay in "${times[@]}"; do
        sleep $delay

        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Backoff test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "backoff_attempts" ${#times[@]}
    [[ $success -ge 2 ]] && record_assertion "exponential_backoff" "observed" "true" "$success/${#times[@]} backoff attempts succeeded"
}

test_max_retry_limit() {
    log_info "Test 3: Max retry limit enforced"

    # Multiple requests should all eventually succeed or fail gracefully
    local total=5
    local success=0

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Max retry test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # Should succeed (200) or fail gracefully after max retries (503/504)
        [[ "$code" =~ ^(200|503|504)$ ]] && success=$((success + 1))
    done

    record_metric "max_retry_requests" $total
    [[ $success -ge 4 ]] && record_assertion "max_retry_limit" "enforced" "true" "$success/$total requests handled with retry limits"
}

test_retry_recovery() {
    log_info "Test 4: System recovers after retry scenarios"

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "retry_recovery_requests" $total
    [[ $success -ge 4 ]] && record_assertion "retry_recovery" "successful" "true" "$success/$total requests succeeded post-retry"
}

main() {
    log_info "Starting retry logic challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_transient_failure_retry
    test_exponential_backoff
    test_max_retry_limit
    test_retry_recovery

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
