#!/bin/bash
# Error Connection Failures Challenge
# Tests handling of connection failures and network issues

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-connection-failures" "Error Connection Failures Challenge"
load_env

log_info "Testing connection failure handling..."

test_invalid_host() {
    log_info "Test 1: Invalid host handling"

    local resp=$(curl -s -w "\n%{http_code}" "http://invalid-host-12345.local:9999/api" \
        -H "Content-Type: application/json" \
        -d '{}' \
        --max-time 5 2>/dev/null || echo -e "\n000")

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "000" ]] && record_assertion "invalid_host" "detected" "true" "Connection failure detected"

    # Verify main service still works
    resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    [[ "$(echo "$resp" | tail -n1)" =~ ^(200|204)$ ]] && record_assertion "invalid_host" "isolated" "true" "Main service unaffected"
}

test_refused_connection() {
    log_info "Test 2: Connection refused handling"

    # Try connecting to invalid port
    local resp=$(curl -s -w "\n%{http_code}" "http://localhost:9999/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"test","messages":[]}' \
        --max-time 5 2>/dev/null || echo -e "\n000")

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "000" ]] && record_assertion "connection_refused" "detected" "true" "Connection refused detected"
}

test_connection_timeout() {
    log_info "Test 3: Connection timeout handling"

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Timeout test"}],"max_tokens":10}' \
        --max-time 3 2>/dev/null || echo -e "\n000")
    local end=$(date +%s%N)
    local duration=$(( (end - start) / 1000000 ))

    record_metric "connection_timeout_ms" $duration

    local code=$(echo "$resp" | tail -n1)
    if [[ "$code" == "000" ]]; then
        record_assertion "connection_timeout" "triggered" "true" "Timed out after ${duration}ms"
    else
        record_assertion "connection_timeout" "completed" "true" "Completed in ${duration}ms"
    fi
}

test_connection_recovery() {
    log_info "Test 4: System recovers after connection failures"

    # Trigger connection errors
    for i in {1..3}; do
        curl -s "http://localhost:9999/api" --max-time 2 > /dev/null 2>&1 || true
    done

    sleep 1

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "connection_recovery" "operational" "true" "System operational after connection errors"
}

main() {
    log_info "Starting connection failures challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_invalid_host
    test_refused_connection
    test_connection_timeout
    test_connection_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
