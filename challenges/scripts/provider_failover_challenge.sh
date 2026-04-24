#!/bin/bash
# Provider Failover Challenge
# Tests provider failover and fallback mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "provider_failover" "Provider Failover Challenge"
load_env

log_info "Testing provider failover and fallback..."

# Test 1: Request with invalid model falls back
test_invalid_model_fallback() {
    log_info "Test 1: Invalid model triggers fallback"

    local request='{
        "model": "totally-nonexistent-model-12345",
        "messages": [{"role": "user", "content": "Test fallback"}],
        "max_tokens": 20
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 45 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    # May fallback to default model (200) or reject (400/404)
    if [[ "$http_code" == "200" ]]; then
        record_assertion "invalid_fallback" "handled" "true" "Invalid model falls back to default (200)"

        # Check if response indicates fallback
        if echo "$body" | grep -qi "fallback\|default"; then
            record_assertion "invalid_fallback" "indicated" "true" "Fallback explicitly indicated"
        else
            record_assertion "invalid_fallback" "indicated" "false" "No fallback indication"
        fi
    elif [[ "$http_code" == "400" ]] || [[ "$http_code" == "404" ]]; then
        record_assertion "invalid_fallback" "handled" "true" "Invalid model rejected ($http_code)"
    else
        record_assertion "invalid_fallback" "handled" "false" "Unexpected status: $http_code"
    fi
}

# Test 2: Provider health after failures
test_provider_health() {
    log_info "Test 2: Provider health endpoint"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/monitoring/provider-health" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "health" "endpoint_works" "true" "Provider health endpoint works"

        # Check for provider health status
        local has_providers=$(echo "$body" | jq -e '.providers' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_providers" == "true" ]]; then
            local provider_count=$(echo "$body" | jq '.providers | length' 2>/dev/null || echo "0")
            record_assertion "health" "has_status" "true" "Health status for $provider_count providers"
        else
            record_assertion "health" "has_status" "false" "No provider health data"
        fi
    else
        record_assertion "health" "endpoint_works" "false" "Health endpoint returned $http_code"
    fi
}

# Test 3: Fallback chain order
test_fallback_chain() {
    log_info "Test 3: Fallback chain configuration"

    local response=$(curl -s "$BASE_URL/v1/monitoring/fallback-chain" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    # Check if endpoint exists
    if echo "$response" | jq empty 2>/dev/null; then
        record_assertion "chain" "endpoint_exists" "true" "Fallback chain endpoint exists"

        # Check for chain configuration
        local has_chain=$(echo "$response" | jq -e '.fallback_chain' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_chain" == "true" ]]; then
            local chain_length=$(echo "$response" | jq '.fallback_chain | length' 2>/dev/null || echo "0")
            if [[ $chain_length -gt 0 ]]; then
                record_assertion "chain" "has_chain" "true" "Fallback chain has $chain_length levels"
            else
                record_assertion "chain" "has_chain" "false" "Empty fallback chain"
            fi
        else
            record_assertion "chain" "has_chain" "false" "No fallback chain data"
        fi
    else
        record_assertion "chain" "endpoint_exists" "false" "Fallback chain endpoint not available"
    fi
}

# Test 4: Retry behavior
test_retry_behavior() {
    log_info "Test 4: Retry behavior on transient failures"

    # Normal request that should succeed
    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test retry"}],
        "max_tokens": 10
    }'

    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "retry" "succeeds" "true" "Request succeeds (with retries if needed)"
        record_metric "retry_latency_ms" "$latency"

        # Higher latency might indicate retries (but not definitive)
        if [[ $latency -gt 10000 ]]; then
            record_assertion "retry" "possible_retries" "true" "High latency suggests retries ($latency ms)"
        else
            record_assertion "retry" "possible_retries" "false" "Fast response ($latency ms)"
        fi
    else
        record_assertion "retry" "succeeds" "false" "Request failed: $http_code"
    fi
}

# Test 5: Circuit breaker status
test_circuit_breaker() {
    log_info "Test 5: Circuit breaker status"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/monitoring/circuit-breakers" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "circuit" "endpoint_works" "true" "Circuit breaker endpoint works"

        # Check for breaker states
        local has_breakers=$(echo "$body" | jq -e '.circuit_breakers' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_breakers" == "true" ]]; then
            local breaker_count=$(echo "$body" | jq '.circuit_breakers | length' 2>/dev/null || echo "0")
            record_assertion "circuit" "has_breakers" "true" "$breaker_count circuit breakers configured"

            # Check for open breakers
            local open_count=$(echo "$body" | jq '[.circuit_breakers[] | select(.state == "open")] | length' 2>/dev/null || echo "0")
            record_metric "open_circuit_breakers" "$open_count"

            if [[ $open_count -eq 0 ]]; then
                record_assertion "circuit" "all_closed" "true" "All circuit breakers closed (healthy)"
            else
                record_assertion "circuit" "all_closed" "false" "$open_count circuit breakers open"
            fi
        else
            record_assertion "circuit" "has_breakers" "false" "No circuit breaker data"
        fi
    else
        record_assertion "circuit" "endpoint_works" "false" "Endpoint returned $http_code"
    fi
}

# Test 6: Fallback error reporting
test_fallback_error_reporting() {
    log_info "Test 6: Fallback error reporting in responses"

    # Request that might trigger fallback
    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test error reporting"}],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 45 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "error_report" "request_succeeds" "true" "Request succeeds"

        # Check for fallback metadata in response
        local has_metadata=$(echo "$body" | jq -e '.metadata' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_warnings=$(echo "$body" | jq -e '.warnings' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_metadata" == "true" ]] || [[ "$has_warnings" == "true" ]]; then
            record_assertion "error_report" "has_metadata" "true" "Response includes metadata/warnings"
        else
            record_assertion "error_report" "has_metadata" "false" "No fallback metadata"
        fi
    else
        record_assertion "error_report" "request_succeeds" "false" "Request failed: $http_code"
    fi
}

# Test 7: Multiple provider fallback
test_multiple_fallback() {
    log_info "Test 7: Multiple provider fallback resilience"

    # Make multiple requests to test fallback under load
    local success_count=0
    local total_requests=5

    for i in $(seq 1 $total_requests); do
        local request='{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "Test '${i}'"}],
            "max_tokens": 10
        }'

        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 45 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        [[ "$http_code" == "200" ]] && success_count=$((success_count + 1))

        sleep 1
    done

    record_metric "fallback_success_count" "$success_count"

    if [[ $success_count -eq $total_requests ]]; then
        record_assertion "multi_fallback" "all_succeed" "true" "All $total_requests requests succeeded (resilient)"
    elif [[ $success_count -ge 3 ]]; then
        record_assertion "multi_fallback" "all_succeed" "false" "$success_count/$total_requests succeeded (acceptable)"
    else
        record_assertion "multi_fallback" "all_succeed" "false" "Only $success_count/$total_requests succeeded"
    fi
}

# Main execution
main() {
    log_info "Starting Provider Failover Challenge..."

    # Check if server is running
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_warning "HelixAgent not running, attempting to start..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    # Run tests
    test_invalid_model_fallback
    test_provider_health
    test_fallback_chain
    test_retry_behavior
    test_circuit_breaker
    test_fallback_error_reporting
    test_multiple_fallback

    # Calculate results
    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo "0")
    failed_count=${failed_count:-0}

    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
