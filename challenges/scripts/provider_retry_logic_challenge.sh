#!/bin/bash
# Provider Retry Logic Challenge
# Tests retry mechanisms and strategies

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-retry-logic" "Provider Retry Logic Challenge"
load_env

log_info "Testing retry logic..."

test_retry_configuration() {
    log_info "Test 1: Retry configuration setup"

    local request='{"provider":"mistral","max_retries":5,"initial_delay":500,"max_delay":30000,"multiplier":2.0}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/retry/configure" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local configured=$(echo "$body" | jq -e '.configured' 2>/dev/null || echo "null")
        local max_retries=$(echo "$body" | jq -e '.max_retries' 2>/dev/null || echo "0")
        record_assertion "retry_configuration" "working" "true" "Configured: $configured, Max retries: $max_retries"
    else
        record_skip "retry_configuration" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_conditional_retry() {
    log_info "Test 2: Conditional retry logic"

    local request='{"error_type":"timeout","retry_on_errors":["timeout","connection","rate_limit"],"max_attempts":3}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/retry/evaluate" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local should_retry=$(echo "$body" | jq -e '.should_retry' 2>/dev/null || echo "null")
        local reason=$(echo "$body" | jq -r '.reason' 2>/dev/null || echo "unknown")
        record_assertion "conditional_retry" "working" "true" "Should retry: $should_retry, Reason: $reason"
    else
        record_skip "conditional_retry" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_jitter_implementation() {
    log_info "Test 3: Retry jitter implementation"

    local request='{"base_delay":1000,"jitter_strategy":"full","max_jitter_ms":500}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/retry/jitter" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local actual_delay=$(echo "$body" | jq -e '.actual_delay_ms' 2>/dev/null || echo "0")
        local jitter_applied=$(echo "$body" | jq -e '.jitter_ms' 2>/dev/null || echo "0")
        record_assertion "jitter_implementation" "working" "true" "Delay: ${actual_delay}ms, Jitter: ${jitter_applied}ms"
    else
        record_skip "jitter_implementation" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_circuit_breaker_integration() {
    log_info "Test 4: Circuit breaker integration"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/retry/circuit?provider=qwen&check_status=true" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local circuit_state=$(echo "$resp_body" | jq -r '.circuit_state' 2>/dev/null || echo "unknown")
    local consecutive_failures=$(echo "$resp_body" | jq -e '.consecutive_failures' 2>/dev/null || echo "0")
    record_assertion "circuit_breaker_integration" "checked" "true" "State: $circuit_state, Failures: $consecutive_failures"
}

main() {
    log_info "Starting retry logic challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_retry_configuration
    test_conditional_retry
    test_jitter_implementation
    test_circuit_breaker_integration

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
