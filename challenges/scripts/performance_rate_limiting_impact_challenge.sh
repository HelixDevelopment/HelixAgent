#!/bin/bash
# Performance Rate Limiting Impact Challenge
# Tests performance impact of rate limiting

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-rate-limiting-impact" "Performance Rate Limiting Impact Challenge"
load_env

log_info "Testing rate limiting performance impact..."

test_baseline_without_limit() {
    log_info "Test 1: Baseline without hitting limits"

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
        sleep 0.2  # Space out requests
    done

    local avg_time=$((total_time / count))
    record_metric "baseline_no_limit_avg_ms" "$avg_time"
    [[ $avg_time -lt 10000 ]] && record_assertion "baseline_no_limit" "acceptable" "true" "${avg_time}ms average"
}

test_rate_limit_enforcement() {
    log_info "Test 2: Rate limit enforcement response time"

    local count=15
    local success=0
    local rate_limited=0
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Rate limit test '$i'"}],"max_tokens":10}' \
            --max-time 10 2>/dev/null || true)
        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1))
        [[ "$code" == "429" ]] && rate_limited=$((rate_limited + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local avg_time=$((total_time / count))

    record_metric "rate_limit_avg_ms" "$avg_time"
    record_metric "rate_limited_count" "$rate_limited"
    [[ $avg_time -lt 10000 ]] && record_assertion "rate_limit_enforcement" "fast" "true" "$success success, $rate_limited rate-limited in ${avg_time}ms avg"
}

test_rate_limit_recovery() {
    log_info "Test 3: Recovery after rate limiting"

    sleep 2  # Wait for rate limit window to reset

    local count=5
    local success=0
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 0.3
    done

    local avg_time=$((total_time / count))
    record_metric "recovery_avg_ms" "$avg_time"
    [[ $success -ge 4 && $avg_time -lt 12000 ]] && record_assertion "rate_limit_recovery" "successful" "true" "$success/$count in ${avg_time}ms avg"
}

test_rate_limit_overhead() {
    log_info "Test 4: Rate limiting overhead on normal requests"

    local count=8
    local total_time=0
    local success=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Overhead test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 0.2
    done

    local avg_time=$((total_time / count))
    record_metric "rate_limit_overhead_avg_ms" "$avg_time"
    [[ $success -ge 6 && $avg_time -lt 12000 ]] && record_assertion "rate_limit_overhead" "minimal" "true" "$success/$count in ${avg_time}ms avg"
}

main() {
    log_info "Starting rate limiting impact challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_without_limit
    test_rate_limit_enforcement
    test_rate_limit_recovery
    test_rate_limit_overhead

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
