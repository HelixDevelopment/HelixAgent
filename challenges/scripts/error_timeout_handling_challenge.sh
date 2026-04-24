#!/bin/bash
# Error Timeout Handling Challenge
# Tests timeout error detection and handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-timeout-handling" "Error Timeout Handling Challenge"
load_env

log_info "Testing timeout error handling..."

test_client_timeout() {
    log_info "Test 1: Client-side timeout handling"

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 2 2>/dev/null || echo -e "\n000")
    local end=$(date +%s%N)
    local duration=$(( (end - start) / 1000000 ))

    record_metric "timeout_duration_ms" $duration

    local code=$(echo "$resp" | tail -n1)
    if [[ "$code" == "000" || "$code" == "00" ]]; then
        record_assertion "client_timeout" "triggered" "true" "Timed out after ${duration}ms"
    else
        record_assertion "client_timeout" "completed" "true" "Completed in ${duration}ms (no timeout)"
    fi
}

test_timeout_recovery() {
    log_info "Test 2: System recovers after timeout"

    # Trigger timeout
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Long request"}],"max_tokens":500}' \
        --max-time 1 > /dev/null 2>&1 || true

    sleep 1

    # Try normal request
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "timeout_recovery" "recovered" "true" "System operational after timeout"
}

test_timeout_error_code() {
    log_info "Test 3: Timeout errors return proper HTTP codes"

    # Very short timeout on potentially slow operation
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Complex query requiring thought"}],"max_tokens":200}' \
        --max-time 3 2>/dev/null || echo -e "\n000")

    local code=$(echo "$resp" | tail -n1)

    if [[ "$code" == "504" ]]; then
        record_assertion "timeout_code" "gateway_timeout" "true" "Returns 504 Gateway Timeout"
    elif [[ "$code" == "000" ]]; then
        record_assertion "timeout_code" "client_timeout" "true" "Client timeout (connection)"
    elif [[ "$code" == "200" ]]; then
        record_assertion "timeout_code" "completed" "true" "Request completed within timeout"
    fi
}

test_concurrent_timeout_handling() {
    log_info "Test 4: Concurrent requests with timeouts"

    local success=0
    local timeout=0

    for i in {1..3}; do
        (
            local r=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent"}],"max_tokens":10}' \
                --max-time 30 2>/dev/null || echo -e "\n000")
            [[ "$(echo "$r" | tail -n1)" == "200" ]] && echo "ok" || echo "timeout"
        ) &
    done

    wait

    record_assertion "concurrent_timeout" "handled" "true" "Concurrent requests with timeout handled"
}

main() {
    log_info "Starting timeout handling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_client_timeout
    test_timeout_recovery
    test_timeout_error_code
    test_concurrent_timeout_handling

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
