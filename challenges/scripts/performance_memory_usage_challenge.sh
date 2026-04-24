#!/bin/bash
# Performance Memory Usage Challenge
# Tests memory efficiency and leak detection

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-memory-usage" "Performance Memory Usage Challenge"
load_env

log_info "Testing memory usage efficiency..."

test_baseline_memory() {
    log_info "Test 1: Baseline memory usage"

    local mem_before=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$4} END {print sum}' || echo "0")

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Memory baseline"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local mem_after=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$4} END {print sum}' || echo "0")
    local mem_delta=$(echo "$mem_after - $mem_before" | bc -l 2>/dev/null || echo "0")

    record_metric "baseline_memory_percent" "$mem_after"
    record_metric "memory_delta_percent" "$mem_delta"
    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "baseline_memory" "measured" "true" "Memory: ${mem_after}%, Delta: ${mem_delta}%"
}

test_memory_under_load() {
    log_info "Test 2: Memory usage under load"

    local count=10
    local mem_readings=()

    for i in $(seq 1 $count); do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Load test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null

        local mem=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$4} END {print sum}' || echo "0")
        mem_readings+=($mem)
    done

    local max_mem=0
    for mem in "${mem_readings[@]}"; do
        local mem_int=$(echo "$mem" | cut -d. -f1)
        [[ ${mem_int:-0} -gt ${max_mem:-0} ]] && max_mem=${mem_int}
    done

    record_metric "load_max_memory_percent" "$max_mem"
    [[ ${max_mem:-0} -lt 50 ]] && record_assertion "memory_under_load" "acceptable" "true" "Max: ${max_mem}%"
}

test_memory_leak_detection() {
    log_info "Test 3: Memory leak detection"

    local iterations=15
    local mem_start=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$4} END {print sum}' || echo "0")

    for i in $(seq 1 $iterations); do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Leak test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
    done

    local mem_end=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$4} END {print sum}' || echo "0")
    local mem_growth=$(echo "$mem_end - $mem_start" | bc -l 2>/dev/null || echo "0")

    record_metric "memory_growth_percent" "$mem_growth"
    [[ $(echo "$mem_growth < 3" | bc -l) -eq 1 ]] && record_assertion "memory_leak" "none_detected" "true" "Growth: ${mem_growth}% over $iterations requests"
}

test_memory_recovery() {
    log_info "Test 4: Memory recovery after load"

    sleep 3  # Allow GC to run

    local mem_idle=$(ps aux | awk '/helixagent/ && !/awk/ {sum+=$4} END {print sum}' || echo "0")
    local mem_int=$(echo "$mem_idle" | cut -d. -f1)

    record_metric "idle_memory_percent" "$mem_idle"
    [[ ${mem_int:-0} -lt 40 ]] && record_assertion "memory_recovery" "successful" "true" "Idle: ${mem_idle}%"
}

main() {
    log_info "Starting memory usage challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_memory
    test_memory_under_load
    test_memory_leak_detection
    test_memory_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
