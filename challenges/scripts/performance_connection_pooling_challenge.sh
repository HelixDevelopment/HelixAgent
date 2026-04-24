#!/bin/bash
# Performance Connection Pooling Challenge
# Tests connection pooling performance

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-connection-pooling" "Performance Connection Pooling Challenge"
load_env

log_info "Testing connection pooling performance..."

test_connection_reuse() {
    log_info "Test 1: Connection reuse reduces latency"

    local count=5
    local total_latency=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Reuse test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_latency=$((total_latency + latency))
    done

    local avg_latency=$((total_latency / count))
    record_metric "avg_connection_reuse_ms" "$avg_latency"
    [[ $avg_latency -lt 10000 ]] && record_assertion "connection_reuse" "efficient" "true" "${avg_latency}ms average"
}

test_pool_under_load() {
    log_info "Test 2: Connection pool performs under load"

    local count=10
    local start=$(date +%s%N)
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Pool load"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    record_metric "pool_load_total_ms" "$total_time"
    [[ $success -ge 8 && $total_time -lt 50000 ]] && record_assertion "pool_under_load" "acceptable" "true" "$success/$count in ${total_time}ms"
}

test_concurrent_connections() {
    log_info "Test 3: Pool handles concurrent connections"

    local count=8
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent pool"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /tmp/pool_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        [[ "$(tail -n1 /tmp/pool_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/pool_$i.txt
    done

    record_metric "pool_concurrent_ms" "$total_time"
    [[ $success -ge 6 && $total_time -lt 20000 ]] && record_assertion "concurrent_connections" "efficient" "true" "$success/$count in ${total_time}ms"
}

test_pool_recovery() {
    log_info "Test 4: Connection pool recovers after stress"

    sleep 2  # Allow pool recovery

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "pool_recovery_ms" "$latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 30000 ]] && record_assertion "pool_recovery" "successful" "true" "${latency}ms"
}

main() {
    log_info "Starting connection pooling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_connection_reuse
    test_pool_under_load
    test_concurrent_connections
    test_pool_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
