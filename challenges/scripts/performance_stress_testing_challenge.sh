#!/bin/bash
# Performance Stress Testing Challenge
# Tests system behavior under stress conditions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-stress-testing" "Performance Stress Testing Challenge"
load_env

log_info "Testing stress conditions..."

test_baseline_before_stress() {
    log_info "Test 1: Baseline measurement before stress"

    local count=3
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Baseline"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "baseline_success_count" "$success"
    [[ $success -eq $count ]] && record_assertion "baseline_before_stress" "stable" "true" "$success/$count succeeded"
}

test_burst_traffic_handling() {
    log_info "Test 2: Burst traffic handling (20 concurrent)"

    local count=20
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Burst '$i'"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/burst_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        local code=$(tail -n1 /tmp/burst_$i.txt 2>/dev/null || echo "000")
        [[ "$code" =~ ^(200|429|503)$ ]] && success=$((success + 1))
        rm -f /tmp/burst_$i.txt
    done

    record_metric "burst_handled" "$success"
    record_metric "burst_total_ms" "$total_time"
    [[ $success -ge 12 ]] && record_assertion "burst_traffic" "handled" "true" "$success/$count handled gracefully"
}

test_sustained_load() {
    log_info "Test 3: Sustained load over time (30 requests, 45s)"

    local duration=45
    local count=0
    local success=0
    local start=$(date +%s)

    while [[ $(( $(date +%s) - start )) -lt $duration ]]; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Sustained '$count'"}],"max_tokens":10}' \
            --max-time 10 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" =~ ^(200|429)$ ]] && success=$((success + 1))
        count=$((count + 1))
        [[ $count -ge 30 ]] && break
        sleep 1.5
    done

    local success_rate=$((success * 100 / count))
    record_metric "sustained_total_requests" "$count"
    record_metric "sustained_success_rate" "$success_rate"
    [[ $success_rate -ge 70 ]] && record_assertion "sustained_load" "handled" "true" "$success/$count (${success_rate}%)"
}

test_recovery_after_stress() {
    log_info "Test 4: System recovery after stress"

    sleep 3  # Recovery period

    local count=5
    local success=0
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local avg_time=$((total_time / count))
    record_metric "recovery_avg_ms" "$avg_time"
    [[ $success -ge 4 && $avg_time -lt 15000 ]] && record_assertion "stress_recovery" "successful" "true" "$success/$count in ${avg_time}ms avg"
}

main() {
    log_info "Starting stress testing challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_before_stress
    test_burst_traffic_handling
    test_sustained_load
    test_recovery_after_stress

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
