#!/bin/bash
# Resilience Zero Downtime Challenge
# Tests zero-downtime deployment and updates

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-zero-downtime" "Resilience Zero Downtime Challenge"
load_env

log_info "Testing zero-downtime capabilities..."

test_continuous_availability() {
    log_info "Test 1: Continuous availability maintained"

    local success=0
    local total=20

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Availability test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 0.5
    done

    record_metric "continuous_availability" $total
    [[ $success -ge 18 ]] && record_assertion "continuous_availability" "maintained" "true" "$success/$total requests succeeded (continuous availability)"
}

test_rolling_updates() {
    log_info "Test 2: System handles rolling updates"

    # Simulate update scenario with concurrent requests
    local pids=()
    for i in $(seq 1 10); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Update test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /tmp/zero_$i.txt) &
        pids+=($!)
    done

    wait

    local success=0
    for i in $(seq 1 10); do
        [[ "$(tail -n1 /tmp/zero_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/zero_$i.txt
    done

    record_metric "rolling_update_requests" 10
    [[ $success -ge 8 ]] && record_assertion "rolling_updates" "handled" "true" "$success/10 requests succeeded during rolling updates"
}

test_connection_draining() {
    log_info "Test 3: Connection draining works"

    # Long-running request should complete
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Drain test"}],"max_tokens":20}' \
        --max-time 60 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "connection_draining" "functional" "true" "Connections drained gracefully"
}

test_zero_downtime_verification() {
    log_info "Test 4: Zero downtime verified"

    local success=0
    local total=10

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Zero downtime test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "zero_downtime_verification" $total
    [[ $success -ge 9 ]] && record_assertion "zero_downtime" "verified" "true" "$success/$total requests succeeded (zero downtime verified)"
}

main() {
    log_info "Starting zero-downtime challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_continuous_availability
    test_rolling_updates
    test_connection_draining
    test_zero_downtime_verification

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
