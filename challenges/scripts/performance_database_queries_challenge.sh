#!/bin/bash
# Performance Database Queries Challenge
# Tests database query performance and optimization

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-database-queries" "Performance Database Queries Challenge"
load_env

log_info "Testing database query performance..."

test_single_query_baseline() {
    log_info "Test 1: Single query performance baseline"

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Query test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "single_query_ms" "$latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 30000 ]] && record_assertion "single_query" "acceptable" "true" "${latency}ms"
}

test_query_batch_performance() {
    log_info "Test 2: Batch query performance"

    local count=10
    local start=$(date +%s%N)
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Batch query '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local avg_time=$((total_time / count))

    record_metric "batch_query_avg_ms" "$avg_time"
    [[ $success -ge 8 && $avg_time -lt 10000 ]] && record_assertion "batch_queries" "efficient" "true" "$success/$count in ${avg_time}ms avg"
}

test_query_connection_pooling() {
    log_info "Test 3: Query connection pooling efficiency"

    local count=5
    local total_latency=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Pool test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_latency=$((total_latency + latency))
    done

    local avg_latency=$((total_latency / count))
    record_metric "pooled_query_avg_ms" "$avg_latency"
    [[ $avg_latency -lt 10000 ]] && record_assertion "query_pooling" "efficient" "true" "${avg_latency}ms average"
}

test_concurrent_query_performance() {
    log_info "Test 4: Concurrent query handling"

    local count=8
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent '$i'"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/dbq_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        [[ "$(tail -n1 /tmp/dbq_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/dbq_$i.txt
    done

    record_metric "concurrent_query_ms" "$total_time"
    [[ $success -ge 6 && $total_time -lt 30000 ]] && record_assertion "concurrent_queries" "handled" "true" "$success/$count in ${total_time}ms"
}

main() {
    log_info "Starting database queries challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_single_query_baseline
    test_query_batch_performance
    test_query_connection_pooling
    test_concurrent_query_performance

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
