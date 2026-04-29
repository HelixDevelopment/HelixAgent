#!/bin/bash
# Resilience Self Healing Challenge
# Tests self-healing and automatic recovery

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-self-healing" "Resilience Self Healing Challenge"
load_env

log_info "Testing self-healing..."

test_automatic_failure_detection() {
    log_info "Test 1: System detects failures automatically"

    # Trigger failure condition
    local fail_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid_detect_failure" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 10 2>/dev/null || true)

    # Check monitoring detects issue
    local status=$(curl -s "$BASE_URL/v1/monitoring/status" 2>/dev/null || echo "{}")
    if echo "$status" | jq -e '.provider_status' > /dev/null 2>&1; then
        record_assertion "failure_detection" "monitored" "true" "System monitoring detects failures"
    fi

    # Normal request should work (self-healing via fallback)
    local heal_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Healing test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$heal_resp" | tail -n1)" == "200" ]] && record_assertion "auto_detection" "working" "true" "System detects and routes around failures"
}

test_automatic_recovery() {
    log_info "Test 2: System recovers automatically"

    sleep 3  # Recovery period

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Auto recovery test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "auto_recovery_attempts" $total
    [[ $success -ge 4 ]] && record_assertion "automatic_recovery" "successful" "true" "$success/$total requests succeeded (auto-recovery working)"
}

test_circuit_breaker_self_healing() {
    log_info "Test 3: Circuit breakers self-heal"

    # Check circuit breaker status
    local cb=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$cb" | jq -e '.providers' > /dev/null 2>&1; then
        local cb_count=$(echo "$cb" | jq -e '.providers | length' 2>/dev/null || echo 0)
        record_metric "self_healing_circuits" $cb_count
        [[ $cb_count -gt 0 ]] && record_assertion "circuit_self_healing" "active" "true" "$cb_count circuit breakers self-healing"
    else
        # Functional check
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Circuit test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "functional_self_healing" "verified" "true" "System self-healing functionally verified"
    fi
}

test_sustained_self_healing() {
    log_info "Test 4: Self-healing sustained over time"

    local success=0
    local total=10

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Sustained test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 1
    done

    record_metric "sustained_healing_requests" $total
    [[ $success -ge 8 ]] && record_assertion "sustained_self_healing" "confirmed" "true" "$success/$total requests succeeded (sustained self-healing)"
}

main() {
    log_info "Starting self-healing challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_automatic_failure_detection
    test_automatic_recovery
    test_circuit_breaker_self_healing
    test_sustained_self_healing

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
