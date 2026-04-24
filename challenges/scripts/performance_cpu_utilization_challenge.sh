#!/bin/bash
# Performance CPU Utilization Challenge
# Tests CPU efficiency under various loads

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-cpu-utilization" "Performance CPU Utilization Challenge"
load_env

log_info "Testing CPU utilization efficiency..."

test_baseline_cpu_usage() {
    log_info "Test 1: Baseline CPU usage measurement"

    local cpu_before=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$3} END {print sum}' || echo "0")

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"CPU baseline test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local cpu_after=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$3} END {print sum}' || echo "0")
    local cpu_delta=$(echo "$cpu_after - $cpu_before" | bc -l 2>/dev/null || echo "0")

    record_metric "baseline_cpu_percent" "$cpu_delta"
    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "baseline_cpu" "measured" "true" "CPU delta: ${cpu_delta}%"
}

test_cpu_under_load() {
    log_info "Test 2: CPU utilization under concurrent load"

    local count=8
    local pids=()

    for i in $(seq 1 $count); do
        (curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"CPU load test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null) &
        pids+=($!)
    done

    sleep 1
    local cpu_peak=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$3} END {print sum}' || echo "0")

    wait

    record_metric "load_cpu_percent" "$cpu_peak"
    local cpu_int=$(echo "$cpu_peak" | cut -d. -f1)
    [[ ${cpu_int:-0} -lt 400 ]] && record_assertion "cpu_under_load" "efficient" "true" "Peak: ${cpu_peak}%"
}

test_cpu_efficiency_per_request() {
    log_info "Test 3: CPU efficiency per request"

    local count=5
    local total_cpu=0

    for i in $(seq 1 $count); do
        local cpu_before=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$3} END {print sum}' || echo "0")

        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Efficiency test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null

        local cpu_after=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$3} END {print sum}' || echo "0")
        local cpu_delta=$(echo "$cpu_after - $cpu_before" | bc -l 2>/dev/null || echo "0")
        total_cpu=$(echo "$total_cpu + $cpu_delta" | bc -l 2>/dev/null || echo "$total_cpu")
    done

    local avg_cpu=$(echo "scale=2; $total_cpu / $count" | bc -l 2>/dev/null || echo "0")
    record_metric "avg_cpu_per_request" "$avg_cpu"
    [[ $(echo "$avg_cpu < 50" | bc -l) -eq 1 ]] && record_assertion "cpu_efficiency" "acceptable" "true" "${avg_cpu}% average"
}

test_cpu_recovery_after_load() {
    log_info "Test 4: CPU returns to baseline after load"

    sleep 2  # Allow system to stabilize

    local cpu_idle=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$3} END {print sum}' || echo "0")
    local cpu_int=$(echo "$cpu_idle" | cut -d. -f1)

    record_metric "idle_cpu_percent" "$cpu_idle"
    [[ ${cpu_int:-0} -lt 50 ]] && record_assertion "cpu_recovery" "successful" "true" "Idle: ${cpu_idle}%"
}

main() {
    log_info "Starting CPU utilization challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_cpu_usage
    test_cpu_under_load
    test_cpu_efficiency_per_request
    test_cpu_recovery_after_load

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
