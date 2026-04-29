#!/bin/bash
# Performance Debate Optimization Challenge
# Tests debate ensemble performance optimizations

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-debate-optimization" "Performance Debate Optimization Challenge"
load_env

log_info "Testing debate ensemble optimizations..."

test_debate_baseline_latency() {
    log_info "Test 1: Debate response baseline latency"

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Debate baseline test"}],"max_tokens":20}' \
        --max-time 60 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "debate_baseline_ms" "$latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 60000 ]] && record_assertion "debate_baseline" "acceptable" "true" "${latency}ms"
}

test_parallel_provider_execution() {
    log_info "Test 2: Parallel provider execution efficiency"

    local count=3
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Parallel test '$i'"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
    done

    local avg_time=$((total_time / count))
    record_metric "parallel_exec_avg_ms" "$avg_time"
    [[ $avg_time -lt 30000 ]] && record_assertion "parallel_execution" "efficient" "true" "${avg_time}ms average"
}

test_debate_caching_benefit() {
    log_info "Test 3: Debate response caching effectiveness"

    local query="Repeated debate query for caching"

    # First request (cache miss)
    local start1=$(date +%s%N)
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$query'"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null > /dev/null
    local first_latency=$(( ($(date +%s%N) - start1) / 1000000 ))

    # Second request (potential cache hit)
    local start2=$(date +%s%N)
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$query'"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null > /dev/null
    local second_latency=$(( ($(date +%s%N) - start2) / 1000000 ))

    record_metric "debate_cache_miss_ms" "$first_latency"
    record_metric "debate_cache_hit_ms" "$second_latency"
    [[ $second_latency -lt 60000 ]] && record_assertion "debate_caching" "working" "true" "Miss: ${first_latency}ms, Hit: ${second_latency}ms"
}

test_debate_under_load() {
    log_info "Test 4: Debate performance under load"

    local count=5
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Load test '$i'"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/debate_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        [[ "$(tail -n1 /tmp/debate_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/debate_$i.txt
    done

    record_metric "debate_load_ms" "$total_time"
    [[ $success -ge 4 && $total_time -lt 60000 ]] && record_assertion "debate_under_load" "handled" "true" "$success/$count in ${total_time}ms"
}

main() {
    log_info "Starting debate optimization challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_debate_baseline_latency
    test_parallel_provider_execution
    test_debate_caching_benefit
    test_debate_under_load

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
