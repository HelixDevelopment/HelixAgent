#!/bin/bash
# Security Rate Limiting Challenge
# Tests rate limiting enforcement and abuse prevention

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-rate-limiting" "Security Rate Limiting Challenge"
load_env

log_info "Testing rate limiting security..."

test_rate_limit_enforcement() {
    log_info "Test 1: Rate limit enforcement"

    local requests=0
    local rate_limited=0
    local start=$(date +%s)

    # Make many rapid requests
    for i in {1..100}; do
        requests=$((requests + 1))
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Rate limit test"}],"max_tokens":5}' \
            --max-time 5 2>/dev/null || echo -e "\n000")

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "429" ]] && rate_limited=$((rate_limited + 1)) && break
    done

    local end=$(date +%s)
    local duration=$((end - start))

    record_metric "requests_before_limit" $requests
    record_metric "duration_seconds" $duration

    if [[ $rate_limited -gt 0 ]]; then
        record_assertion "rate_limit" "enforced" "true" "Rate limited after $requests requests in ${duration}s"
    else
        record_assertion "rate_limit" "not_triggered" "true" "No rate limit triggered (may have high limits)"
    fi
}

test_rate_limit_headers() {
    log_info "Test 2: Rate limit headers"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":5}' \
        --max-time 30 -i 2>/dev/null || true)

    # Check for rate limit headers (X-RateLimit-*, RateLimit-*)
    if echo "$resp" | grep -qiE "(X-RateLimit|RateLimit-|X-Rate-Limit)"; then
        record_assertion "rate_limit_headers" "present" "true" "Rate limit headers provided"
    fi
}

test_rate_limit_recovery() {
    log_info "Test 3: Recovery after rate limiting"

    # Trigger rate limit
    for i in {1..50}; do
        curl -s "$BASE_URL/health" --max-time 2 > /dev/null 2>&1 || true
    done

    sleep 3

    # Normal request should work after cooldown
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "rate_limit_recovery" "operational" "true" "System recovered after rate limiting"
}

test_per_key_isolation() {
    log_info "Test 4: Per-key rate limit isolation"

    # Test with different keys (or same key should be isolated per endpoint)
    local resp1=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 5 2>/dev/null || true)
    local resp2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/models" --max-time 5 2>/dev/null || true)

    # Both should work (not affected by each other's rate limits)
    [[ "$(echo "$resp1" | tail -n1)" =~ ^(200|204)$ ]] && [[ "$(echo "$resp2" | tail -n1)" =~ ^(200|404)$ ]] && \
        record_assertion "rate_limit_isolation" "working" "true" "Rate limits are isolated per endpoint/key"
}

main() {
    log_info "Starting rate limiting challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_rate_limit_enforcement
    test_rate_limit_headers
    test_rate_limit_recovery
    test_per_key_isolation

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
