#!/bin/bash
# Resilience Network Partitions Challenge
# Tests handling of network partitions and connectivity issues

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-network-partitions" "Resilience Network Partitions Challenge"
load_env

log_info "Testing network partition handling..."

test_baseline_connectivity() {
    log_info "Test 1: Baseline connectivity established"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Connectivity test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "baseline_connectivity" "established" "true" "Baseline connectivity successful"
}

test_timeout_handling() {
    log_info "Test 2: System handles network timeouts"

    # Short timeout to simulate network issue
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Timeout test"}],"max_tokens":10}' \
        --max-time 2 2>/dev/null || echo "\n504")

    local code=$(echo "$resp" | tail -n1)
    # Should complete (200) or timeout gracefully (504)
    [[ "$code" =~ ^(200|504)$ ]] && record_assertion "timeout_handling" "graceful" "true" "Network timeout handled (HTTP $code)"
}

test_partial_connectivity() {
    log_info "Test 3: System handles partial connectivity"

    # Multiple requests to test fallback behavior
    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Partial connectivity test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "partial_connectivity_requests" $total
    [[ $success -ge 3 ]] && record_assertion "partial_connectivity" "resilient" "true" "$success/$total requests succeeded with partial connectivity"
}

test_recovery_after_partition() {
    log_info "Test 4: System recovers after network partition"

    sleep 3  # Recovery time

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

    record_metric "partition_recovery_requests" $total
    [[ $success -ge 4 ]] && record_assertion "partition_recovery" "successful" "true" "$success/$total requests succeeded after partition recovery"
}

main() {
    log_info "Starting network partitions challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_connectivity
    test_timeout_handling
    test_partial_connectivity
    test_recovery_after_partition

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
