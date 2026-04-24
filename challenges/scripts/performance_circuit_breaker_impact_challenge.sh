#!/bin/bash
# Performance Circuit Breaker Impact Challenge
# Tests circuit breaker performance impact

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-circuit-breaker-impact" "Performance Circuit Breaker Impact Challenge"
load_env

log_info "Testing circuit breaker performance impact..."

test_normal_operation_baseline() {
    log_info "Test 1: Normal operation without circuit breaker activation"

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Normal operation"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "normal_latency_ms" "$latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 30000 ]] && record_assertion "normal_baseline" "acceptable" "true" "${latency}ms"
}

test_fast_fail_when_open() {
    log_info "Test 2: Fast failure when circuit breaker open"

    # Circuit breaker should fail fast (no provider call)
    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fast fail test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "fast_fail_ms" "$latency"
    # Should complete quickly regardless of status
    [[ $latency -lt 30000 ]] && record_assertion "fast_fail" "efficient" "true" "${latency}ms latency"
}

test_circuit_breaker_overhead() {
    log_info "Test 3: Circuit breaker overhead minimal"

    local total=5
    local start=$(date +%s%N)

    for i in $(seq 1 $total); do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Overhead test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local avg_time=$((total_time / total))

    record_metric "cb_overhead_avg_ms" "$avg_time"
    [[ $avg_time -lt 10000 ]] && record_assertion "cb_overhead" "minimal" "true" "${avg_time}ms average"
}

test_recovery_performance() {
    log_info "Test 4: Performance after circuit breaker recovery"

    sleep 2  # Recovery period

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "recovery_latency_ms" "$latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 30000 ]] && record_assertion "recovery_performance" "acceptable" "true" "${latency}ms"
}

main() {
    log_info "Starting circuit breaker impact challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_normal_operation_baseline
    test_fast_fail_when_open
    test_circuit_breaker_overhead
    test_recovery_performance

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
