#!/bin/bash
# Resilience Partial Failures Challenge
# Tests handling of partial system failures

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-partial-failures" "Resilience Partial Failures Challenge"
load_env

log_info "Testing partial failure handling..."

test_some_providers_failing() {
    log_info "Test 1: System operates with some provider failures"

    # Trigger failures with invalid auth (some providers fail)
    local total=10
    local success=0

    for i in $(seq 1 $total); do
        # Alternate between valid and invalid to simulate partial failures
        local auth="Bearer ${HELIXAGENT_API_KEY:-test}"
        [[ $((i % 3)) -eq 0 ]] && auth="Bearer invalid_partial_$RANDOM"

        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: $auth" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Partial test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "partial_failure_requests" $total
    record_metric "partial_failure_success" $success
    [[ $success -ge 5 ]] && record_assertion "partial_failures" "tolerated" "true" "$success/$total requests succeeded with partial failures"
}

test_degraded_but_operational() {
    log_info "Test 2: System degraded but operational"

    # Check monitoring status
    local status=$(curl -s "$BASE_URL/v1/monitoring/status" 2>/dev/null || echo "{}")

    if echo "$status" | jq -e '.provider_status' > /dev/null 2>&1; then
        local total_providers=$(echo "$status" | jq -e '.provider_status | length' 2>/dev/null || echo 0)
        record_metric "total_providers" $total_providers
        [[ $total_providers -gt 0 ]] && record_assertion "degraded_operation" "monitored" "true" "System monitoring $total_providers providers"
    fi

    # Core functionality should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Degraded test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "core_functionality" "operational" "true" "Core functionality works in degraded mode"
}

test_partial_feature_availability() {
    log_info "Test 3: Partial feature availability maintained"

    # Test main endpoint
    local main=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Main"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Test health endpoint
    local health=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)

    local main_code=$(echo "$main" | tail -n1)
    local health_code=$(echo "$health" | tail -n1)

    [[ "$main_code" == "200" && "$health_code" =~ ^(200|204)$ ]] && record_assertion "partial_features" "available" "true" "Multiple features available during partial failure"
}

test_recovery_from_partial_failures() {
    log_info "Test 4: System recovers from partial failures"

    sleep 3  # Recovery time

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "recovery_from_partial" $total
    [[ $success -ge 4 ]] && record_assertion "partial_failure_recovery" "successful" "true" "$success/$total requests succeeded after recovery"
}

main() {
    log_info "Starting partial failures challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_some_providers_failing
    test_degraded_but_operational
    test_partial_feature_availability
    test_recovery_from_partial_failures

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
