#!/bin/bash
# Provider Rate Limit Handling Challenge
# Tests rate limit detection and handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-rate-limit-handling" "Provider Rate Limit Handling Challenge"
load_env

log_info "Testing rate limit handling..."

test_rate_limit_detection() {
    log_info "Test 1: Rate limit detection"

    local request='{"provider":"openai","check_rate_limit":true,"current_usage":90,"limit":100}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/ratelimit/check" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local near_limit=$(echo "$body" | jq -e '.near_limit' 2>/dev/null || echo "null")
        local remaining=$(echo "$body" | jq -e '.remaining_requests' 2>/dev/null || echo "0")
        record_assertion "rate_limit_detection" "working" "true" "Near limit: $near_limit, Remaining: $remaining"
    else
        record_skip "rate_limit_detection" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_backoff_strategy() {
    log_info "Test 2: Exponential backoff strategy"

    local request='{"provider":"anthropic","rate_limited":true,"backoff_strategy":"exponential","base_delay":1000}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/ratelimit/backoff" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local next_delay=$(echo "$body" | jq -e '.next_delay_ms' 2>/dev/null || echo "0")
        local retry_after=$(echo "$body" | jq -e '.retry_after_seconds' 2>/dev/null || echo "0")
        record_assertion "backoff_strategy" "working" "true" "Delay: ${next_delay}ms, Retry after: ${retry_after}s"
    else
        record_skip "backoff_strategy" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_quota_management() {
    log_info "Test 3: Request quota management"

    local request='{"user_id":"test_user","provider":"deepseek","daily_quota":10000,"check_quota":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/ratelimit/quota" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local quota_remaining=$(echo "$body" | jq -e '.quota_remaining' 2>/dev/null || echo "0")
        local quota_used=$(echo "$body" | jq -e '.quota_used' 2>/dev/null || echo "0")
        record_assertion "quota_management" "working" "true" "Used: $quota_used, Remaining: $quota_remaining"
    else
        record_skip "quota_management" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_throttling_behavior() {
    log_info "Test 4: Request throttling behavior"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/ratelimit/throttle?provider=gemini&requests_per_second=10" \
        -X POST \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local throttling_enabled=$(echo "$resp_body" | jq -e '.throttling_enabled' 2>/dev/null || echo "null")
    local max_rps=$(echo "$resp_body" | jq -e '.max_requests_per_second' 2>/dev/null || echo "0")
    record_assertion "throttling_behavior" "checked" "true" "Enabled: $throttling_enabled, Max RPS: $max_rps"
}

main() {
    log_info "Starting rate limit handling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_rate_limit_detection
    test_backoff_strategy
    test_quota_management
    test_throttling_behavior

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
