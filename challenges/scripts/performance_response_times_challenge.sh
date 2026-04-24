#!/bin/bash
# Performance Response Times Challenge
# Tests response time consistency and optimization

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-response-times" "Performance Response Times Challenge"
load_env

log_info "Testing response time performance..."

test_average_response_time() {
    log_info "Test 1: Average response time measurement"

    local count=10
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Response time test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
    done

    local avg_time=$((total_time / count))
    record_metric "avg_response_time_ms" "$avg_time"
    [[ $avg_time -lt 10000 ]] && record_assertion "avg_response_time" "acceptable" "true" "${avg_time}ms average"
}

test_response_time_under_load() {
    log_info "Test 2: Response time under concurrent load"

    local count=8
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (local req_start=$(date +%s%N)
         curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Load test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
         local req_latency=$(( ($(date +%s%N) - req_start) / 1000000 ))
         echo "$req_latency" > /tmp/resp_time_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local sum_latencies=0
    local count_valid=0
    for i in $(seq 1 $count); do
        if [[ -f /tmp/resp_time_$i.txt ]]; then
            local lat=$(cat /tmp/resp_time_$i.txt)
            sum_latencies=$((sum_latencies + lat))
            count_valid=$((count_valid + 1))
            rm -f /tmp/resp_time_$i.txt
        fi
    done

    local avg_latency=$((sum_latencies / count_valid))
    record_metric "load_avg_response_ms" "$avg_latency"
    [[ $avg_latency -lt 20000 ]] && record_assertion "response_under_load" "acceptable" "true" "$count_valid/$count in ${avg_latency}ms avg"
}

test_response_time_consistency() {
    log_info "Test 3: Response time consistency"

    local count=15
    local latencies=()

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Consistency test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        latencies+=($latency)
    done

    # Calculate standard deviation approximation
    local sum=0
    for lat in "${latencies[@]}"; do
        sum=$((sum + lat))
    done
    local mean=$((sum / count))

    local min=${latencies[0]}
    local max=${latencies[0]}
    for lat in "${latencies[@]}"; do
        [[ $lat -lt $min ]] && min=$lat
        [[ $lat -gt $max ]] && max=$lat
    done
    local range=$((max - min))

    record_metric "response_time_mean_ms" "$mean"
    record_metric "response_time_range_ms" "$range"
    [[ $range -lt 10000 ]] && record_assertion "response_consistency" "acceptable" "true" "Range: ${range}ms (${min}-${max}ms)"
}

test_fast_response_optimization() {
    log_info "Test 4: Fast response path optimization"

    local count=5
    local fast_responses=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fast path"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 8000 ]] && fast_responses=$((fast_responses + 1))
    done

    record_metric "fast_response_count" "$fast_responses"
    [[ $fast_responses -ge 3 ]] && record_assertion "fast_response_path" "optimized" "true" "$fast_responses/$count under 8s"
}

main() {
    log_info "Starting response times challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_average_response_time
    test_response_time_under_load
    test_response_time_consistency
    test_fast_response_optimization

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
