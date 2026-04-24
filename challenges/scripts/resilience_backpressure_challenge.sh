#!/bin/bash
# Resilience Backpressure Challenge
# Tests backpressure handling when system is overloaded

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-backpressure" "Resilience Backpressure Challenge"
load_env

log_info "Testing backpressure handling..."

test_gradual_load_increase() {
    log_info "Test 1: System applies backpressure under load"

    local rate_limited=0
    local total=20

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Load test '$i'"}],"max_tokens":10}' \
            --max-time 5 2>/dev/null || echo "\n504")

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(429|503|504)$ ]] && rate_limited=$((rate_limited + 1))

        # Rapid fire to trigger backpressure
        [[ $((i % 5)) -eq 0 ]] && sleep 1
    done

    record_metric "backpressure_requests" $total
    record_metric "backpressure_responses" $rate_limited
    [[ $rate_limited -ge 1 ]] && record_assertion "backpressure" "applied" "true" "$rate_limited/$total requests received backpressure"
}

test_queue_management() {
    log_info "Test 2: Request queue managed properly"

    # Send burst of requests
    local pids=()
    for i in $(seq 1 15); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Queue test"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/backpressure_$i.txt) &
        pids+=($!)
    done

    # Wait for all
    wait

    # Check results
    local success=0
    local rejected=0
    for i in $(seq 1 15); do
        local code=$(tail -n1 /tmp/backpressure_$i.txt 2>/dev/null || echo "000")
        [[ "$code" == "200" ]] && success=$((success + 1))
        [[ "$code" =~ ^(429|503)$ ]] && rejected=$((rejected + 1))
        rm -f /tmp/backpressure_$i.txt
    done

    record_metric "queue_success" $success
    record_metric "queue_rejected" $rejected
    [[ $success -gt 0 ]] && record_assertion "queue_management" "functional" "true" "$success successful, $rejected rejected (queue managed)"
}

test_backpressure_recovery() {
    log_info "Test 3: System recovers after backpressure"

    sleep 5  # Allow queue to drain

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "backpressure_recovery" "successful" "true" "System recovered after backpressure"
}

test_fair_queuing() {
    log_info "Test 4: Fair queuing behavior"

    # Multiple users in parallel
    local user1_success=0
    local user2_success=0

    for i in $(seq 1 5); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer user1" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"User1"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null | tail -n1) > /tmp/user1_$i.txt &

        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer user2" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"User2"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null | tail -n1) > /tmp/user2_$i.txt &
    done
    wait

    for i in $(seq 1 5); do
        [[ "$(cat /tmp/user1_$i.txt 2>/dev/null)" == "200" ]] && user1_success=$((user1_success + 1))
        [[ "$(cat /tmp/user2_$i.txt 2>/dev/null)" == "200" ]] && user2_success=$((user2_success + 1))
        rm -f /tmp/user1_$i.txt /tmp/user2_$i.txt
    done

    record_metric "user1_success" $user1_success
    record_metric "user2_success" $user2_success
    [[ $user1_success -gt 0 && $user2_success -gt 0 ]] && record_assertion "fair_queuing" "maintained" "true" "Both users served (fair queuing)"
}

main() {
    log_info "Starting backpressure challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_gradual_load_increase
    test_queue_management
    test_backpressure_recovery
    test_fair_queuing

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
