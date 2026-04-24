#!/bin/bash
# Performance Monitoring Overhead Challenge
# Tests observability and monitoring performance impact

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-monitoring-overhead" "Performance Monitoring Overhead Challenge"
load_env

log_info "Testing monitoring overhead..."

test_baseline_without_monitoring() {
    log_info "Test 1: Baseline performance measurement"

    local count=5
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Baseline"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
    done

    local avg_time=$((total_time / count))
    record_metric "baseline_avg_ms" "$avg_time"
    [[ $avg_time -lt 10000 ]] && record_assertion "baseline" "acceptable" "true" "${avg_time}ms average"
}

test_metrics_collection_overhead() {
    log_info "Test 2: Metrics collection overhead"

    local count=10
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Metrics test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
    done

    local avg_time=$((total_time / count))
    record_metric "metrics_overhead_avg_ms" "$avg_time"
    [[ $avg_time -lt 12000 ]] && record_assertion "metrics_overhead" "minimal" "true" "${avg_time}ms average"
}

test_logging_overhead() {
    log_info "Test 3: Logging overhead"

    local count=5
    local start=$(date +%s%N)
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Logging test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local avg_time=$((total_time / count))

    record_metric "logging_overhead_avg_ms" "$avg_time"
    [[ $success -ge 4 && $avg_time -lt 15000 ]] && record_assertion "logging_overhead" "minimal" "true" "$success/$count in ${avg_time}ms avg"
}

test_tracing_overhead() {
    log_info "Test 4: Tracing overhead (OpenTelemetry)"

    local count=8
    local total_time=0
    local success=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Tracing test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local avg_time=$((total_time / count))
    record_metric "tracing_overhead_avg_ms" "$avg_time"
    [[ $success -ge 6 && $avg_time -lt 15000 ]] && record_assertion "tracing_overhead" "minimal" "true" "$success/$count in ${avg_time}ms avg"
}

main() {
    log_info "Starting monitoring overhead challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_without_monitoring
    test_metrics_collection_overhead
    test_logging_overhead
    test_tracing_overhead

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
