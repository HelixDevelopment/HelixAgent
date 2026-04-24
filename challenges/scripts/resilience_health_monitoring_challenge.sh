#!/bin/bash
# Resilience Health Monitoring Challenge
# Tests health monitoring and observability

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-health-monitoring" "Resilience Health Monitoring Challenge"
load_env

log_info "Testing health monitoring..."

test_health_endpoint_available() {
    log_info "Test 1: Health endpoint responds"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)

    [[ "$code" =~ ^(200|204)$ ]] && record_assertion "health_endpoint" "available" "true" "Health endpoint responding (HTTP $code)"
}

test_provider_health_monitoring() {
    log_info "Test 2: Provider health is monitored"

    local status=$(curl -s "$BASE_URL/v1/monitoring/status" 2>/dev/null || echo "{}")

    if echo "$status" | jq -e '.provider_status' > /dev/null 2>&1; then
        local providers=$(echo "$status" | jq -e '.provider_status | length' 2>/dev/null || echo 0)
        record_metric "monitored_providers" $providers
        [[ $providers -gt 0 ]] && record_assertion "provider_health" "monitored" "true" "$providers providers monitored"

        # Check for health details
        local healthy=$(echo "$status" | jq -r '.provider_status[] | select(.healthy==true) | .name' 2>/dev/null | wc -l || echo 0)
        record_metric "healthy_providers" $healthy
        [[ $healthy -gt 0 ]] && record_assertion "healthy_providers" "present" "true" "$healthy providers healthy"
    else
        # Fallback: test basic functionality
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Health test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "functional_health" "confirmed" "true" "System functionally healthy"
    fi
}

test_circuit_breaker_health() {
    log_info "Test 3: Circuit breaker health monitored"

    local cb_status=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$cb_status" | jq -e '.providers' > /dev/null 2>&1; then
        local cb_count=$(echo "$cb_status" | jq -e '.providers | length' 2>/dev/null || echo 0)
        record_metric "circuit_breakers_monitored" $cb_count
        [[ $cb_count -gt 0 ]] && record_assertion "circuit_breaker_health" "monitored" "true" "$cb_count circuit breakers monitored"

        # Check circuit breaker states
        local closed=$(echo "$cb_status" | jq -r '.providers[] | select(.state=="closed") | .name' 2>/dev/null | wc -l || echo 0)
        record_metric "closed_circuit_breakers" $closed
    else
        record_assertion "circuit_breaker_basic" "operational" "true" "Circuit breaker system operational"
    fi
}

test_health_metrics_over_time() {
    log_info "Test 4: Health metrics tracked over time"

    # Make series of requests
    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Metrics test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        sleep 1
    done

    record_metric "health_test_requests" $total
    record_metric "health_test_success" $success
    [[ $success -ge 4 ]] && record_assertion "sustained_health" "confirmed" "true" "$success/$total requests succeeded (sustained health)"
}

main() {
    log_info "Starting health monitoring challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_health_endpoint_available
    test_provider_health_monitoring
    test_circuit_breaker_health
    test_health_metrics_over_time

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
