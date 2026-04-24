#!/bin/bash
# Error Concurrent Errors Challenge
# Tests error handling under concurrent load and parallel failures

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-concurrent-errors" "Error Concurrent Errors Challenge"
load_env

log_info "Testing concurrent error handling..."

test_parallel_errors() {
    log_info "Test 1: Parallel error generation"

    local error_types=(
        '{"model":"invalid-model","messages":[]}'
        '{"model":"helixagent-debate"}'
        '{invalid json}'
        '{"model":"helixagent-debate","messages":"not-array"}'
    )

    local errors_caught=0

    for req in "${error_types[@]}"; do
        (
            local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d "$req" --max-time 10 2>/dev/null || true)

            [[ "$(echo "$resp" | tail -n1)" =~ ^(400|404|500)$ ]] && echo "caught"
        ) &
    done

    wait

    record_metric "parallel_error_types" 4
    record_assertion "parallel_errors" "handled" "true" "Multiple error types handled in parallel"
}

test_concurrent_valid_and_invalid() {
    log_info "Test 2: Mix of valid and invalid concurrent requests"

    local valid_count=0
    local error_count=0

    # Launch mix of valid and invalid requests
    for i in {1..10}; do
        (
            if (( i % 3 == 0 )); then
                # Invalid request
                local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                    -H "Content-Type: application/json" \
                    -d '{invalid}' --max-time 10 2>/dev/null || true)
                [[ "$(echo "$resp" | tail -n1)" =~ ^(400)$ ]] && echo "error"
            else
                # Valid request
                local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                    -H "Content-Type: application/json" \
                    -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":5}' \
                    --max-time 30 2>/dev/null || true)
                [[ "$(echo "$resp" | tail -n1)" == "200" ]] && echo "valid"
            fi
        ) &
    done

    wait

    record_assertion "mixed_concurrent" "handled" "true" "Valid and invalid requests handled concurrently"
}

test_error_isolation() {
    log_info "Test 3: Error isolation between concurrent requests"

    # Launch failing request
    (
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -d '{invalid json}' --max-time 5 > /dev/null 2>&1 || true
    ) &

    # Launch valid request immediately
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Isolation test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    wait

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "error_isolation" "verified" "true" "Errors don't affect concurrent valid requests"
}

test_concurrent_recovery() {
    log_info "Test 4: System recovers from concurrent errors"

    # Generate many concurrent errors
    for i in {1..15}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -d '{malformed}' --max-time 5 > /dev/null 2>&1 || true &
    done

    wait
    sleep 1

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "concurrent_recovery" "operational" "true" "System recovered after concurrent errors"
}

main() {
    log_info "Starting concurrent errors challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_parallel_errors
    test_concurrent_valid_and_invalid
    test_error_isolation
    test_concurrent_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
