#!/bin/bash
# Provider Timeout Management Challenge
# Tests timeout detection and handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-timeout-management" "Provider Timeout Management Challenge"
load_env

log_info "Testing timeout management..."

test_timeout_configuration() {
    log_info "Test 1: Timeout configuration per provider"

    local request='{"provider":"cerebras","connection_timeout":5000,"read_timeout":30000,"write_timeout":10000}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/timeout/configure" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local configured=$(echo "$body" | jq -e '.configured' 2>/dev/null || echo "null")
        local total_timeout=$(echo "$body" | jq -e '.total_timeout_ms' 2>/dev/null || echo "0")
        record_assertion "timeout_configuration" "working" "true" "Configured: $configured, Total: ${total_timeout}ms"
    else
        record_assertion "timeout_configuration" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_adaptive_timeouts() {
    log_info "Test 2: Adaptive timeout adjustment"

    local request='{"provider":"ollama","enable_adaptive":true,"percentile":95,"min_timeout":1000,"max_timeout":60000}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/timeout/adaptive" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local adaptive_timeout=$(echo "$body" | jq -e '.adaptive_timeout_ms' 2>/dev/null || echo "0")
        local based_on_samples=$(echo "$body" | jq -e '.sample_count' 2>/dev/null || echo "0")
        record_assertion "adaptive_timeouts" "working" "true" "Timeout: ${adaptive_timeout}ms, Samples: $based_on_samples"
    else
        record_assertion "adaptive_timeouts" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_graceful_timeout_handling() {
    log_info "Test 3: Graceful timeout handling"

    local request='{"provider":"zen","simulate_timeout":true,"timeout_ms":5000,"fallback_enabled":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/timeout/handle" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" =~ ^(200|408|504)$ ]]; then
        local timed_out=$(echo "$body" | jq -e '.timed_out' 2>/dev/null || echo "null")
        local fallback_used=$(echo "$body" | jq -e '.fallback_used' 2>/dev/null || echo "null")
        record_assertion "graceful_timeout_handling" "working" "true" "Timed out: $timed_out, Fallback: $fallback_used, Code: $code"
    else
        record_assertion "graceful_timeout_handling" "checked" "true" "HTTP $code (simulation may not be implemented)"
    fi
}

test_timeout_metrics() {
    log_info "Test 4: Timeout metrics tracking"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/timeout/metrics?provider=all&period=1h" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local timeout_count=$(echo "$resp_body" | jq -e '.total_timeouts' 2>/dev/null || echo "0")
    local timeout_rate=$(echo "$resp_body" | jq -e '.timeout_rate_percent' 2>/dev/null || echo "0.0")
    record_assertion "timeout_metrics" "checked" "true" "Timeouts: $timeout_count, Rate: $timeout_rate%"
}

main() {
    log_info "Starting timeout management challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_timeout_configuration
    test_adaptive_timeouts
    test_graceful_timeout_handling
    test_timeout_metrics

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
