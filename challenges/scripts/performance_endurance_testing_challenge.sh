#!/bin/bash
# Performance Endurance Testing Challenge
# Tests sustained performance over extended periods

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-endurance-testing" "Performance Endurance Testing Challenge"
load_env

log_info "Testing endurance and sustained performance..."

test_sustained_throughput() {
    log_info "Test 1: Sustained throughput over time"

    local duration=30
    local count=0
    local success=0
    local start=$(date +%s)

    while [[ $(( $(date +%s) - start )) -lt $duration ]]; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Sustained test '$count'"}],"max_tokens":10}' \
            --max-time 10 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        count=$((count + 1))
        sleep 0.5
    done

    local throughput=$((success / (duration / 60)))
    record_metric "sustained_throughput_per_min" "$throughput"
    record_metric "sustained_total_requests" "$count"
    record_metric "sustained_success_count" "$success"
    [[ $success -ge $((count * 80 / 100)) ]] && record_assertion "sustained_throughput" "acceptable" "true" "$success/$count succeeded"
}

test_latency_stability() {
    log_info "Test 2: Latency stability over time"

    local count=20
    local latencies=()

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Stability test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        latencies+=($latency)
    done

    local sum=0
    for lat in "${latencies[@]}"; do
        sum=$((sum + lat))
    done
    local avg=$((sum / count))

    record_metric "latency_stability_avg_ms" "$avg"
    [[ $avg -lt 15000 ]] && record_assertion "latency_stability" "stable" "true" "${avg}ms average"
}

test_memory_stability() {
    log_info "Test 3: Memory usage stability"

    local count=15
    local mem_readings=()

    for i in $(seq 1 $count); do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Memory test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null

        local mem=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$4} END {print sum}' || echo "0")
        mem_readings+=($mem)
        sleep 1
    done

    local first_mem=${mem_readings[0]}
    local last_mem=${mem_readings[-1]}
    local mem_growth=$(echo "$last_mem - $first_mem" | bc -l 2>/dev/null || echo "0")

    record_metric "memory_growth_percent" "$mem_growth"
    [[ $(echo "$mem_growth < 5" | bc -l) -eq 1 ]] && record_assertion "memory_stability" "acceptable" "true" "${mem_growth}% growth"
}

test_error_rate_over_time() {
    log_info "Test 4: Error rate remains low over time"

    local count=25
    local success=0
    local errors=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Error rate test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1)) || errors=$((errors + 1))
    done

    local error_rate=$((errors * 100 / count))
    record_metric "endurance_error_rate_percent" "$error_rate"
    record_metric "endurance_success_count" "$success"
    [[ $error_rate -lt 20 ]] && record_assertion "low_error_rate" "maintained" "true" "$success/$count succeeded (${error_rate}% errors)"
}

main() {
    log_info "Starting endurance testing challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_sustained_throughput
    test_latency_stability
    test_memory_stability
    test_error_rate_over_time

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
