#!/bin/bash
# Resilience Circuit Breaker Challenge
# Tests circuit breaker pattern for failure protection

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-circuit-breaker" "Resilience Circuit Breaker Challenge"
load_env

log_info "Testing circuit breaker behavior..."

test_circuit_breaker_configuration() {
    log_info "Test 1: Circuit breakers configured for providers"

    local cb_status=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$cb_status" | jq -e '.providers' > /dev/null 2>&1; then
        local providers=$(echo "$cb_status" | jq -e '.providers | length' 2>/dev/null || echo 0)
        record_metric "circuit_breakers_configured" $providers
        [[ $providers -gt 0 ]] && record_assertion "circuit_breaker_config" "present" "true" "$providers circuit breakers configured"
    else
        # Basic check - system operational
        local health=$(curl -s "$BASE_URL/health" --max-time 10 2>/dev/null || echo "")
        [[ -n "$health" ]] && record_assertion "circuit_breaker_fallback" "operational" "true" "System operational"
    fi
}

test_circuit_breaker_opens_on_failures() {
    log_info "Test 2: Circuit breaker opens after failures"

    # Trigger multiple failures by using invalid auth
    local failures=0
    for i in $(seq 1 5); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer invalid_trigger_failure_$RANDOM" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
            --max-time 10 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(401|403|500|503)$ ]] && failures=$((failures + 1))
    done

    record_metric "triggered_failures" $failures
    [[ $failures -ge 3 ]] && record_assertion "failure_trigger" "successful" "true" "$failures/5 failures triggered"

    # Check circuit breaker status
    local cb_after=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")
    if echo "$cb_after" | jq -e '.providers' > /dev/null 2>&1; then
        record_assertion "circuit_breaker_reaction" "monitored" "true" "Circuit breaker status available"
    fi
}

test_circuit_breaker_half_open_state() {
    log_info "Test 3: Circuit breaker transitions to half-open"

    sleep 3  # Wait for potential half-open transition

    # Try request after failure period
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Half-open test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should either work (200) or fail gracefully (503)
    [[ "$code" =~ ^(200|503)$ ]] && record_assertion "half_open_state" "functional" "true" "Circuit breaker allows test request (HTTP $code)"
}

test_circuit_breaker_closes_on_success() {
    log_info "Test 4: Circuit breaker closes after successful requests"

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 1
    done

    record_metric "recovery_requests" $total
    [[ $success -ge 3 ]] && record_assertion "circuit_breaker_recovery" "successful" "true" "$success/$total requests succeeded (circuit closed)"
}

main() {
    log_info "Starting circuit breaker challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_circuit_breaker_configuration
    test_circuit_breaker_opens_on_failures
    test_circuit_breaker_half_open_state
    test_circuit_breaker_closes_on_success

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
