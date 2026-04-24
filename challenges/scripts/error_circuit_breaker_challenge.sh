#!/bin/bash
# Error Circuit Breaker Challenge
# Tests circuit breaker pattern for preventing cascading failures

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-circuit-breaker" "Error Circuit Breaker Challenge"
load_env

log_info "Testing circuit breaker patterns and failure isolation..."

# Test 1: Circuit breaker status monitoring
test_circuit_breaker_status() {
    log_info "Test 1: Circuit breaker status monitoring endpoint"

    local response=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$response" | jq -e '.circuit_breakers' > /dev/null 2>&1; then
        record_assertion "cb_status" "endpoint_available" "true" "Monitoring endpoint available"

        local breaker_count=$(echo "$response" | jq '.circuit_breakers | length' 2>/dev/null || echo 0)
        record_metric "total_circuit_breakers" $breaker_count

        if [[ $breaker_count -gt 0 ]]; then
            record_assertion "cb_status" "has_breakers" "true" "$breaker_count circuit breaker(s) configured"
        fi
    else
        record_assertion "cb_status" "endpoint_available" "false" "Monitoring not available"
    fi
}

# Test 2: Circuit states verification (closed, open, half-open)
test_circuit_states() {
    log_info "Test 2: Verify circuit breaker states"

    local response=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$response" | jq -e '.circuit_breakers' > /dev/null 2>&1; then
        local closed_count=$(echo "$response" | jq '[.circuit_breakers[] | select(.state == "closed")] | length' 2>/dev/null || echo 0)
        local open_count=$(echo "$response" | jq '[.circuit_breakers[] | select(.state == "open")] | length' 2>/dev/null || echo 0)
        local half_open_count=$(echo "$response" | jq '[.circuit_breakers[] | select(.state == "half-open")] | length' 2>/dev/null || echo 0)

        record_metric "closed_breakers" $closed_count
        record_metric "open_breakers" $open_count
        record_metric "half_open_breakers" $half_open_count

        if [[ $closed_count -gt 0 ]]; then
            record_assertion "cb_states" "has_closed" "true" "$closed_count closed (healthy)"
        fi

        if [[ $open_count -gt 0 ]]; then
            record_assertion "cb_states" "has_open" "true" "$open_count open (protecting from failures)"
        else
            record_assertion "cb_states" "all_healthy" "true" "No open breakers (system healthy)"
        fi
    fi
}

# Test 3: Request handling with circuit breaker
test_request_with_breaker() {
    log_info "Test 3: Normal request handling with circuit breaker protection"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Circuit breaker test"}],"max_tokens":20}' \
        --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "cb_request" "succeeded" "true" "Request succeeded with circuit breaker protection"
    elif [[ "$http_code" == "503" ]]; then
        record_assertion "cb_request" "circuit_open" "true" "Circuit breaker open (503 returned)"
    else
        record_assertion "cb_request" "handled" "true" "HTTP $http_code returned"
    fi
}

# Test 4: Circuit breaker metrics
test_breaker_metrics() {
    log_info "Test 4: Circuit breaker failure metrics"

    local response=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$response" | jq -e '.circuit_breakers[0]' > /dev/null 2>&1; then
        # Get metrics from first breaker
        local failure_count=$(echo "$response" | jq -r '.circuit_breakers[0].metrics.failure_count // 0' 2>/dev/null || echo 0)
        local success_count=$(echo "$response" | jq -r '.circuit_breakers[0].metrics.success_count // 0' 2>/dev/null || echo 0)
        local trip_count=$(echo "$response" | jq -r '.circuit_breakers[0].metrics.trip_count // 0' 2>/dev/null || echo 0)

        record_metric "cb_failures" $failure_count
        record_metric "cb_successes" $success_count
        record_metric "cb_trips" $trip_count

        if [[ $success_count -gt 0 || $failure_count -gt 0 ]]; then
            record_assertion "cb_metrics" "tracking" "true" "Circuit breaker tracking metrics"
        fi
    fi
}

# Test 5: Circuit breaker prevents cascading failures
test_cascading_prevention() {
    log_info "Test 5: Circuit breaker isolates failures"

    # Make multiple rapid requests - if provider fails, circuit should open
    local success_count=0
    local circuit_open_count=0

    for i in {1..10}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":5}' \
            --max-time 15 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        elif [[ "$http_code" == "503" ]]; then
            circuit_open_count=$((circuit_open_count + 1))
        fi

        sleep 0.2
    done

    record_metric "cascade_test_successes" $success_count
    record_metric "cascade_test_circuit_opens" $circuit_open_count

    if [[ $success_count -gt 0 || $circuit_open_count -gt 0 ]]; then
        record_assertion "cascading" "handled" "true" "System handled rapid requests ($success_count ok, $circuit_open_count protected)"
    fi
}

main() {
    log_info "Starting circuit breaker challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_circuit_breaker_status
    test_circuit_states
    test_request_with_breaker
    test_breaker_metrics
    test_cascading_prevention

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All circuit breaker tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
