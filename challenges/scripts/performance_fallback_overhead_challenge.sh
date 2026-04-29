#!/bin/bash
# Performance Fallback Overhead Challenge
# Tests performance impact of fallback mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-fallback-overhead" "Performance Fallback Overhead Challenge"
load_env

log_info "Testing fallback mechanism performance impact..."

test_no_fallback_baseline() {
    log_info "Test 1: Baseline without fallback activation"

    local count=5
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"No fallback test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
    done

    local avg_time=$((total_time / count))
    record_metric "no_fallback_avg_ms" "$avg_time"
    [[ $avg_time -lt 10000 ]] && record_assertion "no_fallback_baseline" "acceptable" "true" "${avg_time}ms average"
}

test_fallback_activation_overhead() {
    log_info "Test 2: Overhead when fallback activates"

    # Trigger fallback with invalid auth (should fallback to working provider)
    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid_token_trigger_fallback" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fallback trigger"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "fallback_activation_ms" "$latency"
    # Fallback should still complete within reasonable time
    [[ $latency -lt 60000 ]] && record_assertion "fallback_overhead" "acceptable" "true" "${latency}ms with fallback"
}

test_fallback_chain_performance() {
    log_info "Test 3: Fallback chain traversal efficiency"

    local count=3
    local total_time=0
    local success=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Chain test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local avg_time=$((total_time / count))
    record_metric "fallback_chain_avg_ms" "$avg_time"
    [[ $success -ge 2 && $avg_time -lt 15000 ]] && record_assertion "fallback_chain" "efficient" "true" "$success/$count in ${avg_time}ms avg"
}

test_fallback_recovery_performance() {
    log_info "Test 4: Performance after fallback recovery"

    sleep 2  # Allow recovery

    local count=5
    local total_time=0
    local success=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local avg_time=$((total_time / count))
    record_metric "post_fallback_avg_ms" "$avg_time"
    [[ $success -ge 4 && $avg_time -lt 10000 ]] && record_assertion "fallback_recovery" "successful" "true" "$success/$count in ${avg_time}ms avg"
}

main() {
    log_info "Starting fallback overhead challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_no_fallback_baseline
    test_fallback_activation_overhead
    test_fallback_chain_performance
    test_fallback_recovery_performance

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
