#!/bin/bash
# Resilience Cascading Failures Challenge
# Tests prevention of cascading failures across components

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-cascading-failures" "Resilience Cascading Failures Challenge"
load_env

log_info "Testing cascading failure prevention..."

test_provider_failure_isolation() {
    log_info "Test 1: Provider failures don't cascade"

    # Force one provider to fail by using invalid auth
    local fail_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid_key_force_fail" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 10 2>/dev/null || true)

    # Verify other requests still work (fallback engaged)
    local success=0
    for i in $(seq 1 3); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    [[ $success -ge 2 ]] && record_assertion "provider_failure_isolation" "prevented" "true" "$success/3 requests succeeded despite provider failure"
}

test_timeout_cascade_prevention() {
    log_info "Test 2: Timeout failures don't cascade"

    # Send request with very short timeout (likely to timeout)
    local timeout_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Long running"}],"max_tokens":100}' \
        --max-time 2 2>/dev/null || echo "\n504")

    # Verify system still responsive
    local health_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    local health_code=$(echo "$health_resp" | tail -n1)

    [[ "$health_code" =~ ^(200|204)$ ]] && record_assertion "timeout_cascade_prevention" "effective" "true" "System responsive after timeout"
}

test_resource_exhaustion_isolation() {
    log_info "Test 3: Resource exhaustion doesn't cascade"

    # Try to exhaust resources with many concurrent requests
    local pids=()
    for i in $(seq 1 20); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Resource test"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null) &
        pids+=($!)

        [[ $((i % 5)) -eq 0 ]] && sleep 1
    done

    # Test health while under load
    sleep 2
    local health=$(curl -s "$BASE_URL/health" --max-time 10 2>/dev/null || echo "")

    # Cleanup
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    wait 2>/dev/null

    [[ -n "$health" ]] && record_assertion "resource_exhaustion_isolation" "maintained" "true" "Health endpoint operational under resource stress"
}

test_cascade_recovery() {
    log_info "Test 4: System recovers from potential cascades"

    sleep 5  # Allow recovery time

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
    [[ $success -ge 4 ]] && record_assertion "cascade_recovery" "successful" "true" "$success/$total recovery requests succeeded"
}

main() {
    log_info "Starting cascading failures challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_provider_failure_isolation
    test_timeout_cascade_prevention
    test_resource_exhaustion_isolation
    test_cascade_recovery

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
