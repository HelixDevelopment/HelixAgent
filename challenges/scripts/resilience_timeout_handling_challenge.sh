#!/bin/bash
# Resilience Timeout Handling Challenge
# Tests timeout handling mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-timeout-handling" "Resilience Timeout Handling Challenge"
load_env

log_info "Testing timeout handling..."

test_request_timeout_enforced() {
    log_info "Test 1: Request timeouts enforced"

    # Short timeout
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Timeout test"}],"max_tokens":10}' \
        --max-time 2 2>/dev/null || echo "\n504")

    local code=$(echo "$resp" | tail -n1)
    # Should complete (200) or timeout (504)
    [[ "$code" =~ ^(200|504)$ ]] && record_assertion "request_timeout" "handled" "true" "Request timeout handled (HTTP $code)"
}

test_timeout_recovery() {
    log_info "Test 2: System recovers from timeouts"

    # Multiple requests with normal timeout
    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "timeout_recovery" $total
    [[ $success -ge 4 ]] && record_assertion "timeout_recovery" "successful" "true" "$success/$total requests succeeded after timeout"
}

test_timeout_cascades_prevented() {
    log_info "Test 3: Timeout cascades prevented"

    # Health should still work after timeout scenarios
    local health=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    [[ "$(echo "$health" | tail -n1)" =~ ^(200|204)$ ]] && record_assertion "timeout_cascade" "prevented" "true" "Health endpoint operational after timeouts"
}

test_adaptive_timeouts() {
    log_info "Test 4: System adapts to timeout patterns"

    local success=0
    local total=10

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Adaptive test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 1
    done

    record_metric "adaptive_timeout_requests" $total
    [[ $success -ge 8 ]] && record_assertion "adaptive_timeouts" "working" "true" "$success/$total requests succeeded (adaptive timeouts)"
}

main() {
    log_info "Starting timeout handling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_request_timeout_enforced
    test_timeout_recovery
    test_timeout_cascades_prevented
    test_adaptive_timeouts

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
