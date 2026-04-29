#!/bin/bash
# Performance Latency Measurement Challenge
# Tests latency measurement accuracy and consistency

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-latency-measurement" "Performance Latency Measurement Challenge"
load_env

log_info "Testing latency measurement and consistency..."

test_baseline_latency() {
    log_info "Test 1: Baseline latency measurement"

    local count=5
    local total_latency=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Baseline latency"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_latency=$((total_latency + latency))
    done

    local avg_latency=$((total_latency / count))
    record_metric "baseline_latency_avg_ms" "$avg_latency"
    [[ $avg_latency -lt 10000 ]] && record_assertion "baseline_latency" "acceptable" "true" "${avg_latency}ms average"
}

test_latency_percentiles() {
    log_info "Test 2: Latency distribution (P50, P95, P99)"

    local count=20
    local latencies=()

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Percentile test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        latencies+=($latency)
    done

    # Sort latencies
    IFS=$'\n' sorted=($(sort -n <<<"${latencies[*]}"))
    unset IFS

    local p50_idx=$((count / 2))
    local p95_idx=$((count * 95 / 100))
    local p99_idx=$((count * 99 / 100))

    local p50=${sorted[$p50_idx]:-0}
    local p95=${sorted[$p95_idx]:-0}
    local p99=${sorted[$p99_idx]:-0}

    record_metric "latency_p50_ms" "$p50"
    record_metric "latency_p95_ms" "$p95"
    record_metric "latency_p99_ms" "$p99"
    [[ $p95 -lt 20000 ]] && record_assertion "latency_percentiles" "acceptable" "true" "P50: ${p50}ms, P95: ${p95}ms, P99: ${p99}ms"
}

test_latency_variance() {
    log_info "Test 3: Latency variance and consistency"

    local count=10
    local latencies=()
    local sum=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Variance test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        latencies+=($latency)
        sum=$((sum + latency))
    done

    local avg=$((sum / count))
    local min=${latencies[0]}
    local max=${latencies[0]}
    for lat in "${latencies[@]}"; do
        [[ $lat -lt $min ]] && min=$lat
        [[ $lat -gt $max ]] && max=$lat
    done
    local variance=$((max - min))

    record_metric "latency_avg_ms" "$avg"
    record_metric "latency_min_ms" "$min"
    record_metric "latency_max_ms" "$max"
    record_metric "latency_variance_ms" "$variance"
    [[ $variance -lt 10000 ]] && record_assertion "latency_variance" "acceptable" "true" "Variance: ${variance}ms (${min}-${max}ms)"
}

test_cold_vs_warm_latency() {
    log_info "Test 4: Cold vs warm request latency"

    # Cold request
    local start_cold=$(date +%s%N)
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Cold start"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null > /dev/null
    local cold_latency=$(( ($(date +%s%N) - start_cold) / 1000000 ))

    # Warm requests
    local warm_count=3
    local warm_total=0
    for i in $(seq 1 $warm_count); do
        local start_warm=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Warm request"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start_warm) / 1000000 ))
        warm_total=$((warm_total + latency))
    done
    local warm_avg=$((warm_total / warm_count))

    record_metric "cold_latency_ms" "$cold_latency"
    record_metric "warm_latency_avg_ms" "$warm_avg"
    [[ $warm_avg -lt 15000 ]] && record_assertion "cold_vs_warm" "measured" "true" "Cold: ${cold_latency}ms, Warm: ${warm_avg}ms"
}

main() {
    log_info "Starting latency measurement challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_latency
    test_latency_percentiles
    test_latency_variance
    test_cold_vs_warm_latency

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
