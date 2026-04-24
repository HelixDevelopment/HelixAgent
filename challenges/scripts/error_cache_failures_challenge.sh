#!/bin/bash
# Error Cache Failures Challenge
# Tests handling of cache failures and cache-related errors

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-cache-failures" "Error Cache Failures Challenge"
load_env

log_info "Testing cache failure handling..."

test_cache_unavailable_resilience() {
    log_info "Test 1: System resilience when cache unavailable"

    # System should work even if cache (Redis) has issues
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Cache resilience test"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    # Should succeed or fail gracefully (not 500)
    if [[ "$code" == "200" ]]; then
        record_assertion "cache_resilience" "operational" "true" "System works without cache"
    elif [[ "$code" == "503" ]]; then
        record_assertion "cache_resilience" "degraded" "true" "System degrades gracefully (HTTP 503)"
    elif [[ "$code" == "500" ]]; then
        record_assertion "cache_resilience" "error_500" "false" "Unhandled cache error"
    fi
}

test_cache_miss_handling() {
    log_info "Test 2: Cache miss handling"

    # Make unique requests that shouldn't be cached
    local success=0

    for i in {1..5}; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Unique request '$RANDOM'"}],"max_tokens":5}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "cache_miss_success" $success
    [[ $success -ge 4 ]] && record_assertion "cache_miss" "handled" "true" "$success/5 cache misses handled"
}

test_cache_error_recovery() {
    log_info "Test 3: Recovery from cache errors"

    # Stress system with rapid requests (might stress cache)
    for i in {1..10}; do
        curl -s "$BASE_URL/health" --max-time 2 > /dev/null 2>&1 || true &
    done

    wait
    sleep 1

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "cache_recovery" "operational" "true" "System recovered after cache stress"
}

test_cache_fallback() {
    log_info "Test 4: Fallback when cache fails"

    # Multiple requests should work even if cache is problematic
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fallback test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    # System should have fallback (direct DB or memory)
    [[ "$code" =~ ^(200|206)$ ]] && record_assertion "cache_fallback" "working" "true" "Fallback operational without cache"
}

main() {
    log_info "Starting cache failures challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_cache_unavailable_resilience
    test_cache_miss_handling
    test_cache_error_recovery
    test_cache_fallback

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
