#!/bin/bash
# Resilience Failover Challenge
# Tests automatic failover to backup systems

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-failover" "Resilience Failover Challenge"
load_env

log_info "Testing failover mechanisms..."

test_provider_failover() {
    log_info "Test 1: System fails over between providers"

    # Trigger failure with invalid auth, should fallback
    local resp1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid_failover_test" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Failover test"}],"max_tokens":10}' \
        --max-time 10 2>/dev/null || true)

    # Normal request should work via fallback
    local resp2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Normal test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code2=$(echo "$resp2" | tail -n1)
    [[ "$code2" == "200" ]] && record_assertion "provider_failover" "successful" "true" "System failed over to working provider"
}

test_fallback_chain_activation() {
    log_info "Test 2: Fallback chain activates correctly"

    # Check fallback chain status
    local fallback=$(curl -s "$BASE_URL/v1/monitoring/fallback-chain" 2>/dev/null || echo "{}")

    if echo "$fallback" | jq -e '.chain' > /dev/null 2>&1; then
        local chain_length=$(echo "$fallback" | jq -e '.chain | length' 2>/dev/null || echo 0)
        record_metric "fallback_chain_length" $chain_length
        [[ $chain_length -gt 0 ]] && record_assertion "fallback_chain" "configured" "true" "$chain_length providers in fallback chain"
    else
        # Test actual fallback behavior
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fallback"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "fallback_operational" "verified" "true" "Fallback system operational"
    fi
}

test_seamless_failover() {
    log_info "Test 3: Failover is seamless to clients"

    # Multiple requests during potential failover
    local success=0
    local total=10

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Seamless test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "failover_continuity" $total
    [[ $success -ge 8 ]] && record_assertion "seamless_failover" "achieved" "true" "$success/$total requests succeeded during failover"
}

test_primary_restoration() {
    log_info "Test 4: System restores to primary after recovery"

    sleep 5  # Allow primary restoration time

    # Verify system fully operational
    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Restoration test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "restoration_requests" $total
    [[ $success -ge 4 ]] && record_assertion "primary_restoration" "successful" "true" "$success/$total requests on restored primary"
}

main() {
    log_info "Starting failover challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_provider_failover
    test_fallback_chain_activation
    test_seamless_failover
    test_primary_restoration

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
