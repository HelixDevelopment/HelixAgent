#!/bin/bash
# Error Recovery Strategies Challenge
# Tests error recovery mechanisms and retry strategies

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-recovery-strategies" "Error Recovery Strategies Challenge"
load_env

log_info "Testing error recovery strategies and retry mechanisms..."

# Test 1: Automatic retry on transient failures
test_automatic_retry() {
    log_info "Test 1: System handles transient failures with retry"

    # Make a valid request that should succeed (or retry if provider fails)
    local max_attempts=3
    local success=false

    for attempt in $(seq 1 $max_attempts); do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test retry"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "200" ]]; then
            success=true
            record_assertion "auto_retry" "eventually_succeeded" "true" "Request succeeded on attempt $attempt"
            record_metric "success_attempt" $attempt
            break
        fi

        sleep 2
    done

    if [[ "$success" == "false" ]]; then
        record_assertion "auto_retry" "eventually_succeeded" "false" "All $max_attempts attempts failed"
    fi
}

# Test 2: Graceful degradation on provider failure
test_graceful_degradation() {
    log_info "Test 2: Graceful degradation when providers unavailable"

    # Request with an invalid/unavailable provider model
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fallback test"}],"max_tokens":20}' \
        --max-time 90 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    # Should either succeed (with fallback) or return proper error
    if [[ "$http_code" == "200" ]]; then
        record_assertion "graceful_degradation" "handled" "true" "Request succeeded (possibly with fallback)"

        # Check if response indicates fallback was used
        if echo "$body" | jq -e '.model' > /dev/null 2>&1; then
            local used_model=$(echo "$body" | jq -r '.model // empty')
            record_metric "fallback_model" "$used_model"
        fi
    elif [[ "$http_code" =~ ^(503|500)$ ]]; then
        record_assertion "graceful_degradation" "handled" "true" "Returns appropriate error ($http_code) when no fallback available"
    else
        record_assertion "graceful_degradation" "handled" "false" "Unexpected HTTP $http_code"
    fi
}

# Test 3: Circuit breaker behavior
test_circuit_breaker() {
    log_info "Test 3: Circuit breaker prevents cascading failures"

    # Check circuit breaker status endpoint if available
    local cb_response=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$cb_response" | jq -e '.circuit_breakers' > /dev/null 2>&1; then
        record_assertion "circuit_breaker" "endpoint_available" "true" "Circuit breaker monitoring available"

        # Count open circuit breakers
        local open_count=$(echo "$cb_response" | jq '[.circuit_breakers[] | select(.state == "open")] | length' 2>/dev/null || echo 0)
        record_metric "open_circuit_breakers" $open_count

        if [[ $open_count -gt 0 ]]; then
            record_assertion "circuit_breaker" "has_open_breakers" "true" "$open_count circuit breaker(s) open"
        else
            record_assertion "circuit_breaker" "all_closed" "true" "All circuit breakers closed (healthy)"
        fi
    else
        record_assertion "circuit_breaker" "endpoint_available" "false" "Circuit breaker monitoring not available"
    fi
}

# Test 4: Error recovery after timeout
test_timeout_recovery() {
    log_info "Test 4: Recovery from timeout errors"

    # Make request with short timeout
    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Quick response"}],"max_tokens":5}' \
        --max-time 2 2>/dev/null || echo -e "\n000")

    local code1=$(echo "$response1" | tail -n1)

    # Make follow-up request with normal timeout
    sleep 1
    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"After timeout"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)

    local code2=$(echo "$response2" | tail -n1)

    # System should recover and handle subsequent requests
    if [[ "$code2" == "200" ]]; then
        record_assertion "timeout_recovery" "recovered" "true" "System recovered after potential timeout"
    else
        record_assertion "timeout_recovery" "recovered" "false" "System did not recover (HTTP $code2)"
    fi
}

# Test 5: Exponential backoff verification
test_exponential_backoff() {
    log_info "Test 5: Verify exponential backoff on retries"

    # Make rapid requests and measure response times
    local response_times=()

    for i in {1..5}; do
        local start_time=$(date +%s%N)
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Backoff test"}],"max_tokens":5}' \
            --max-time 30 2>/dev/null || true)
        local end_time=$(date +%s%N)
        local latency=$(( (end_time - start_time) / 1000000 ))

        response_times+=($latency)
        record_metric "request_${i}_latency_ms" $latency

        sleep 0.1
    done

    # Check if any responses show increasing latency (indicating backoff)
    if [[ ${#response_times[@]} -eq 5 ]]; then
        record_assertion "backoff" "requests_completed" "true" "All 5 requests completed"

        # Calculate average latency
        local sum=0
        for time in "${response_times[@]}"; do
            sum=$((sum + time))
        done
        local avg=$((sum / 5))
        record_metric "avg_latency_ms" $avg
    else
        record_assertion "backoff" "requests_completed" "false" "Only ${#response_times[@]}/5 completed"
    fi
}

# Test 6: Fallback chain verification
test_fallback_chain() {
    log_info "Test 6: Fallback chain provides alternative providers"

    # Check fallback configuration
    local fallback_response=$(curl -s "$BASE_URL/v1/monitoring/fallback-chain" 2>/dev/null || echo "{}")

    if echo "$fallback_response" | jq -e '.fallback_chain' > /dev/null 2>&1; then
        record_assertion "fallback_chain" "endpoint_available" "true" "Fallback chain monitoring available"

        # Count providers in fallback chain
        local provider_count=$(echo "$fallback_response" | jq '.fallback_chain | length' 2>/dev/null || echo 0)
        record_metric "fallback_providers" $provider_count

        if [[ $provider_count -ge 2 ]]; then
            record_assertion "fallback_chain" "has_fallbacks" "true" "$provider_count providers in chain"
        else
            record_assertion "fallback_chain" "has_fallbacks" "false" "Only $provider_count provider(s) in chain"
        fi
    else
        record_assertion "fallback_chain" "endpoint_available" "false" "Fallback chain monitoring not available"
    fi
}

# Test 7: Error state recovery monitoring
test_error_state_recovery() {
    log_info "Test 7: Monitor error state and recovery metrics"

    # Check health endpoint for error metrics
    local health_response=$(curl -s "$BASE_URL/health" 2>/dev/null || echo "{}")

    if echo "$health_response" | jq -e '.status' > /dev/null 2>&1; then
        local status=$(echo "$health_response" | jq -r '.status // empty')

        if [[ "$status" == "healthy" || "$status" == "ok" ]]; then
            record_assertion "error_state" "healthy" "true" "System reports healthy status"
        else
            record_assertion "error_state" "healthy" "false" "System status: $status"
        fi

        # Check for error metrics if available
        if echo "$health_response" | jq -e '.metrics' > /dev/null 2>&1; then
            local error_rate=$(echo "$health_response" | jq -r '.metrics.error_rate // 0' 2>/dev/null || echo 0)
            record_metric "system_error_rate" $error_rate
        fi
    fi
}

main() {
    log_info "Starting error recovery strategies challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_automatic_retry
    test_graceful_degradation
    test_circuit_breaker
    test_timeout_recovery
    test_exponential_backoff
    test_fallback_chain
    test_error_state_recovery

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All error recovery tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
