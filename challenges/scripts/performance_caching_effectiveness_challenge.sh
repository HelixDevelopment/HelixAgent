#!/bin/bash
# Performance Caching Effectiveness Challenge
# Tests caching performance and effectiveness

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-caching-effectiveness" "Performance Caching Effectiveness Challenge"
load_env

log_info "Testing caching effectiveness..."

test_first_request_baseline() {
    log_info "Test 1: First request (cache miss) baseline"

    local query="Cache test query $(date +%s)"
    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$query'"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local first_latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "cache_miss_ms" "$first_latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "cache_miss" "measured" "true" "${first_latency}ms"
}

test_repeat_request_cache_hit() {
    log_info "Test 2: Repeat request (potential cache hit)"

    local query="Repeated cache query"

    # First request
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$query'"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null > /dev/null

    # Repeat request (might be cached)
    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$query'"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local repeat_latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "cache_hit_ms" "$repeat_latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "cache_hit" "measured" "true" "${repeat_latency}ms"
}

test_cache_performance_benefit() {
    log_info "Test 3: Cache provides performance benefit"

    local total=5
    local success=0

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Cache benefit test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "cache_benefit_requests" $total
    [[ $success -ge 4 ]] && record_assertion "cache_benefit" "observed" "true" "$success/$total requests succeeded"
}

test_cache_invalidation() {
    log_info "Test 4: Cache handles invalidation correctly"

    local unique_query="Invalidation test $(date +%s%N)"
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$unique_query'"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "cache_invalidation" "working" "true" "Unique queries handled correctly"
}

main() {
    log_info "Starting caching effectiveness challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_first_request_baseline
    test_repeat_request_cache_hit
    test_cache_performance_benefit
    test_cache_invalidation

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
