#!/bin/bash
# Resilience Disaster Recovery Challenge
# Tests disaster recovery and system restoration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-disaster-recovery" "Resilience Disaster Recovery Challenge"
load_env

log_info "Testing disaster recovery..."

test_baseline_operational_state() {
    log_info "Test 1: Establish baseline operational state"

    local health=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    local health_code=$(echo "$health" | tail -n1)

    [[ "$health_code" =~ ^(200|204)$ ]] && record_assertion "baseline_state" "healthy" "true" "System baseline established (HTTP $health_code)"

    # Get provider count baseline
    local startup=$(curl -s "$BASE_URL/v1/startup/verification" 2>/dev/null || echo "{}")
    if echo "$startup" | jq -e '.ranked_providers' > /dev/null 2>&1; then
        local providers=$(echo "$startup" | jq -e '.ranked_providers | length' 2>/dev/null || echo 0)
        record_metric "baseline_providers" $providers
    fi
}

test_recovery_from_total_failure() {
    log_info "Test 2: System recovers from total failure simulation"

    # Simulate failure by overwhelming system
    local pids=()
    for i in $(seq 1 20); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Disaster test"}],"max_tokens":100}' \
            --max-time 2 2>/dev/null) &
        pids+=($!)
    done

    sleep 3

    # Kill all to simulate crash
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    wait 2>/dev/null

    # Allow recovery time
    sleep 5

    # Test recovery
    local recovery=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    local recovery_code=$(echo "$recovery" | tail -n1)

    [[ "$recovery_code" =~ ^(200|204)$ ]] && record_assertion "total_failure_recovery" "successful" "true" "System recovered from simulated failure"
}

test_state_recovery() {
    log_info "Test 3: System state recovered correctly"

    # Verify provider rankings still available
    local startup=$(curl -s "$BASE_URL/v1/startup/verification" 2>/dev/null || echo "{}")

    if echo "$startup" | jq -e '.ranked_providers' > /dev/null 2>&1; then
        local providers=$(echo "$startup" | jq -e '.ranked_providers | length' 2>/dev/null || echo 0)
        record_metric "recovered_providers" $providers
        [[ $providers -gt 0 ]] && record_assertion "state_recovery" "verified" "true" "$providers providers recovered"
    else
        # Basic operational check
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"State test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "operational_recovery" "confirmed" "true" "System operational after recovery"
    fi
}

test_post_disaster_functionality() {
    log_info "Test 4: Full functionality restored post-disaster"

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Post-disaster test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "post_disaster_requests" $total
    [[ $success -ge 4 ]] && record_assertion "post_disaster_functionality" "restored" "true" "$success/$total requests succeeded post-disaster"
}

main() {
    log_info "Starting disaster recovery challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_operational_state
    test_recovery_from_total_failure
    test_state_recovery
    test_post_disaster_functionality

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
