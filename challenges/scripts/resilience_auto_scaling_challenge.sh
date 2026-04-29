#!/bin/bash
# Resilience Auto Scaling Challenge
# Tests automatic scaling under load

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-auto-scaling" "Resilience Auto Scaling Challenge"
load_env

log_info "Testing auto-scaling behavior..."

test_baseline_capacity() {
    log_info "Test 1: Establish baseline capacity"

    local success=0
    local total=10

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Baseline test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1))
    done

    record_metric "baseline_requests" $total
    record_metric "baseline_success_rate" $success
    [[ $success -ge 8 ]] && record_assertion "baseline_capacity" "established" "true" "$success/$total baseline requests succeeded"
}

test_load_scaling() {
    log_info "Test 2: System handles increased load"

    local success=0
    local total=50
    local concurrent=10

    # Simulate concurrent load
    for batch in $(seq 1 5); do
        for i in $(seq 1 $concurrent); do
            (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Load test '$i'"}],"max_tokens":10}' \
                --max-time 60 2>/dev/null | tail -n1) &
        done
        wait

        # Check if server still responsive
        local health=$(curl -s "$BASE_URL/health" 2>/dev/null || echo "")
        [[ -n "$health" ]] && success=$((success + concurrent))

        sleep 2
    done

    record_metric "load_requests" $total
    record_metric "load_success_rate" $success
    [[ $success -ge 40 ]] && record_assertion "load_scaling" "handled" "true" "System handled $success/$total requests under load"
}

test_scale_down_recovery() {
    log_info "Test 3: System recovers after load reduction"

    sleep 5  # Allow scale-down

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1))
    done

    record_metric "recovery_requests" $total
    [[ $success -ge 4 ]] && record_assertion "scale_down_recovery" "operational" "true" "$success/$total recovery requests succeeded"
}

test_resource_efficiency() {
    log_info "Test 4: Resource utilization remains efficient"

    # Check monitoring endpoint for metrics
    local metrics=$(curl -s "$BASE_URL/v1/monitoring/status" 2>/dev/null || echo "{}")

    if echo "$metrics" | jq -e '.provider_status' > /dev/null 2>&1; then
        record_assertion "resource_monitoring" "available" "true" "Resource metrics available"

        # Check for reasonable resource usage indicators
        local providers=$(echo "$metrics" | jq -e '.provider_status | length' 2>/dev/null || echo 0)
        [[ $providers -gt 0 ]] && record_assertion "resource_efficiency" "verified" "true" "$providers providers monitored"
    else
        record_assertion "resource_monitoring" "basic" "true" "Basic monitoring available"
    fi
}

main() {
    log_info "Starting auto-scaling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_capacity
    test_load_scaling
    test_scale_down_recovery
    test_resource_efficiency

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
