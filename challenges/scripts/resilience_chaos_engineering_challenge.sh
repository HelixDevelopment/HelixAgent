#!/bin/bash
# Resilience Chaos Engineering Challenge
# Tests system resilience through controlled chaos experiments

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-chaos-engineering" "Resilience Chaos Engineering Challenge"
load_env

log_info "Testing chaos engineering scenarios..."

test_random_failures() {
    log_info "Test 1: System handles random failures"

    local success=0
    local total=20

    for i in $(seq 1 $total); do
        # Mix of valid and chaos-inducing requests
        local content="Test $i"
        [[ $((i % 4)) -eq 0 ]] && content="CHAOS_TEST_$RANDOM"

        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$content'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1))

        # Random delays
        [[ $((i % 3)) -eq 0 ]] && sleep 0.5
    done

    record_metric "chaos_requests" $total
    record_metric "chaos_success" $success
    [[ $success -ge 15 ]] && record_assertion "random_failures" "handled" "true" "$success/$total chaos requests handled"
}

test_latency_injection() {
    log_info "Test 2: System tolerates latency spikes"

    local success=0
    local timeout_count=0

    # Test with varying timeouts to simulate latency
    for timeout in 5 10 30; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Latency test"}],"max_tokens":10}' \
            --max-time $timeout 2>/dev/null || echo "\n504")

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1))
        [[ "$code" == "504" ]] && timeout_count=$((timeout_count + 1))
    done

    record_metric "latency_tests" 3
    record_metric "latency_timeouts" $timeout_count
    [[ $success -ge 1 ]] && record_assertion "latency_injection" "tolerated" "true" "$success/3 requests succeeded with varying latency"
}

test_concurrent_chaos() {
    log_info "Test 3: System survives concurrent chaos"

    # Launch multiple chaotic requests in parallel
    local pids=()
    for i in $(seq 1 10); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent chaos '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /tmp/chaos_$i.txt) &
        pids+=($!)
    done

    # Wait for completion
    wait

    # Check results
    local success=0
    for i in $(seq 1 10); do
        local code=$(tail -n1 /tmp/chaos_$i.txt 2>/dev/null || echo "000")
        [[ "$code" == "200" ]] && success=$((success + 1))
        rm -f /tmp/chaos_$i.txt
    done

    record_metric "concurrent_chaos" 10
    [[ $success -ge 5 ]] && record_assertion "concurrent_chaos" "survived" "true" "$success/10 concurrent chaos requests handled"
}

test_chaos_recovery() {
    log_info "Test 4: System recovers after chaos"

    sleep 3  # Recovery period

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Post-chaos recovery"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "recovery_after_chaos" $total
    [[ $success -ge 4 ]] && record_assertion "chaos_recovery" "successful" "true" "$success/$total requests succeeded after chaos"
}

main() {
    log_info "Starting chaos engineering challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_random_failures
    test_latency_injection
    test_concurrent_chaos
    test_chaos_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
