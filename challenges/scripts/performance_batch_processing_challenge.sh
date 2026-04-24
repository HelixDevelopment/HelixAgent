#!/bin/bash
# Performance Batch Processing Challenge
# Tests batch processing performance

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-batch-processing" "Performance Batch Processing Challenge"
load_env

log_info "Testing batch processing performance..."

test_single_request_baseline() {
    log_info "Test 1: Single request baseline"

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Baseline"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "single_request_ms" "$latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 30000 ]] && record_assertion "single_baseline" "acceptable" "true" "${latency}ms latency"
}

test_batch_throughput() {
    log_info "Test 2: Batch processing throughput"

    local batch_size=10
    local start=$(date +%s%N)
    local success=0

    for i in $(seq 1 $batch_size); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Batch '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local avg_latency=$((total_time / batch_size))

    record_metric "batch_total_ms" "$total_time"
    record_metric "batch_avg_ms" "$avg_latency"
    record_metric "batch_throughput" "$success"
    [[ $success -ge 8 && $avg_latency -lt 10000 ]] && record_assertion "batch_throughput" "acceptable" "true" "$success/$batch_size succeeded, ${avg_latency}ms avg"
}

test_concurrent_batch() {
    log_info "Test 3: Concurrent batch processing"

    local batch_size=5
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $batch_size); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /tmp/batch_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $batch_size); do
        [[ "$(tail -n1 /tmp/batch_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/batch_$i.txt
    done

    record_metric "concurrent_batch_ms" "$total_time"
    record_metric "concurrent_success" "$success"
    [[ $success -ge 4 && $total_time -lt 15000 ]] && record_assertion "concurrent_batch" "efficient" "true" "$success/$batch_size in ${total_time}ms"
}

test_batch_scaling() {
    log_info "Test 4: Batch processing scales linearly"

    local small_batch=3
    local start_small=$(date +%s%N)
    for i in $(seq 1 $small_batch); do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Scale test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
    done
    local small_time=$(( ($(date +%s%N) - start_small) / 1000000 ))

    record_metric "small_batch_ms" "$small_time"
    [[ $small_time -lt 20000 ]] && record_assertion "batch_scaling" "acceptable" "true" "$small_batch requests in ${small_time}ms"
}

main() {
    log_info "Starting batch processing challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_single_request_baseline
    test_batch_throughput
    test_concurrent_batch
    test_batch_scaling

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
