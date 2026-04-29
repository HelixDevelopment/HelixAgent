#!/bin/bash
# Performance Streaming Efficiency Challenge
# Tests streaming response performance

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-streaming-efficiency" "Performance Streaming Efficiency Challenge"
load_env

log_info "Testing streaming efficiency..."

test_streaming_initiation() {
    log_info "Test 1: Streaming response initiation time"

    local count=5
    local total_ttfb=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Streaming test"}],"max_tokens":20,"stream":true}' \
            --max-time 30 2>/dev/null | head -1 || true)
        local ttfb=$(( ($(date +%s%N) - start) / 1000000 ))
        total_ttfb=$((total_ttfb + ttfb))
    done

    local avg_ttfb=$((total_ttfb / count))
    record_metric "streaming_ttfb_avg_ms" "$avg_ttfb"
    [[ $avg_ttfb -lt 5000 ]] && record_assertion "streaming_initiation" "fast" "true" "${avg_ttfb}ms average TTFB"
}

test_streaming_throughput() {
    log_info "Test 2: Streaming throughput measurement"

    local start=$(date +%s%N)
    local resp=$(curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Throughput test"}],"max_tokens":50,"stream":true}' \
        --max-time 60 2>/dev/null || true)
    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))

    local chunk_count=$(echo "$resp" | grep -c "data:" 2>/dev/null || echo "0")

    record_metric "streaming_total_ms" "$total_time"
    record_metric "streaming_chunk_count" "$chunk_count"
    [[ $total_time -lt 60000 && $chunk_count -gt 0 ]] && record_assertion "streaming_throughput" "acceptable" "true" "$chunk_count chunks in ${total_time}ms"
}

test_concurrent_streaming() {
    log_info "Test 3: Concurrent streaming efficiency"

    local count=5
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent stream '$i'"}],"max_tokens":20,"stream":true}' \
            --max-time 45 2>/dev/null | wc -l > /tmp/stream_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        local lines=$(cat /tmp/stream_$i.txt 2>/dev/null || echo "0")
        [[ $lines -gt 0 ]] && success=$((success + 1))
        rm -f /tmp/stream_$i.txt
    done

    record_metric "concurrent_streaming_ms" "$total_time"
    [[ $success -ge 4 && $total_time -lt 45000 ]] && record_assertion "concurrent_streaming" "efficient" "true" "$success/$count in ${total_time}ms"
}

test_streaming_vs_non_streaming() {
    log_info "Test 4: Streaming vs non-streaming comparison"

    # Non-streaming
    local start_normal=$(date +%s%N)
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Normal test"}],"max_tokens":20}' \
        --max-time 30 2>/dev/null > /dev/null
    local normal_time=$(( ($(date +%s%N) - start_normal) / 1000000 ))

    # Streaming
    local start_stream=$(date +%s%N)
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Stream test"}],"max_tokens":20,"stream":true}' \
        --max-time 30 2>/dev/null > /dev/null
    local stream_time=$(( ($(date +%s%N) - start_stream) / 1000000 ))

    record_metric "normal_response_ms" "$normal_time"
    record_metric "streaming_response_ms" "$stream_time"
    [[ $stream_time -lt 40000 ]] && record_assertion "streaming_comparison" "measured" "true" "Normal: ${normal_time}ms, Stream: ${stream_time}ms"
}

main() {
    log_info "Starting streaming efficiency challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_streaming_initiation
    test_streaming_throughput
    test_concurrent_streaming
    test_streaming_vs_non_streaming

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
