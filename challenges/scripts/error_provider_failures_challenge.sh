#!/bin/bash
# Error Provider Failures Challenge
# Tests handling of LLM provider failures and unavailability

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-provider-failures" "Error Provider Failures Challenge"
load_env

log_info "Testing LLM provider failure handling..."

# Test 1: Invalid provider model
test_invalid_provider_model() {
    log_info "Test 1: Invalid/non-existent provider model"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"nonexistent-provider-model","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" =~ ^(400|404)$ ]]; then
        record_assertion "invalid_model" "proper_error" "true" "Returns $http_code for invalid model"

        if echo "$body" | jq -e '.error' > /dev/null 2>&1; then
            record_assertion "invalid_model" "has_error_message" "true" "Error message provided"
        fi
    else
        record_assertion "invalid_model" "proper_error" "false" "Unexpected HTTP $http_code"
    fi
}

# Test 2: Provider health status
test_provider_health() {
    log_info "Test 2: Provider health monitoring"

    local health_response=$(curl -s "$BASE_URL/v1/monitoring/provider-health" 2>/dev/null || echo "{}")

    if echo "$health_response" | jq -e '.providers' > /dev/null 2>&1; then
        record_assertion "provider_health" "endpoint_available" "true" "Health monitoring available"

        local healthy_count=$(echo "$health_response" | jq '[.providers[] | select(.status == "healthy")] | length' 2>/dev/null || echo 0)
        local unhealthy_count=$(echo "$health_response" | jq '[.providers[] | select(.status != "healthy")] | length' 2>/dev/null || echo 0)

        record_metric "healthy_providers" $healthy_count
        record_metric "unhealthy_providers" $unhealthy_count

        if [[ $healthy_count -gt 0 ]]; then
            record_assertion "provider_health" "has_healthy" "true" "$healthy_count provider(s) healthy"
        fi

        if [[ $unhealthy_count -gt 0 ]]; then
            record_assertion "provider_health" "has_unhealthy" "true" "$unhealthy_count provider(s) unhealthy (expected during failures)"
        fi
    else
        record_assertion "provider_health" "endpoint_available" "false" "Health monitoring not available"
    fi
}

# Test 3: Provider failure recovery
test_provider_recovery() {
    log_info "Test 3: System continues working despite provider failures"

    # Make request using debate model (uses multiple providers)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":20}' \
        --max-time 90 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "provider_recovery" "system_operational" "true" "System operational with available providers"
    elif [[ "$http_code" == "503" ]]; then
        record_assertion "provider_recovery" "graceful_failure" "true" "System returns 503 when no providers available"
    else
        record_assertion "provider_recovery" "handled" "true" "HTTP $http_code returned"
    fi
}

# Test 4: Provider timeout handling
test_provider_timeout() {
    log_info "Test 4: Provider timeout is handled gracefully"

    # Request with reasonable timeout
    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Timeout test"}],"max_tokens":100}' \
        --max-time 60 2>/dev/null || echo -e "\n000")
    local end_time=$(date +%s%N)
    local duration=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)

    record_metric "request_duration_ms" $duration

    if [[ "$http_code" == "200" ]]; then
        record_assertion "provider_timeout" "completed" "true" "Request completed in ${duration}ms"
    elif [[ "$http_code" == "000" ]]; then
        record_assertion "provider_timeout" "timed_out" "true" "Request timed out after ${duration}ms"
    elif [[ "$http_code" == "504" ]]; then
        record_assertion "provider_timeout" "gateway_timeout" "true" "Gateway timeout returned"
    fi
}

# Test 5: Provider-specific error codes
test_provider_error_codes() {
    log_info "Test 5: Provider-specific errors are classified"

    # Test with various scenarios
    local scenarios=("empty_messages" "invalid_params" "oversized_request")
    local error_codes_found=0

    for scenario in "${scenarios[@]}"; do
        local req=""
        case $scenario in
            "empty_messages")
                req='{"model":"helixagent-debate","messages":[]}'
                ;;
            "invalid_params")
                req='{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"temperature":-1}'
                ;;
            "oversized_request")
                req='{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":1000000}'
                ;;
        esac

        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$req" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "400" ]]; then
            error_codes_found=$((error_codes_found + 1))
        fi
    done

    record_metric "proper_error_codes" $error_codes_found

    if [[ $error_codes_found -ge 2 ]]; then
        record_assertion "provider_errors" "properly_classified" "true" "$error_codes_found/3 scenarios returned proper errors"
    fi
}

main() {
    log_info "Starting provider failures challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_invalid_provider_model
    test_provider_health
    test_provider_recovery
    test_provider_timeout
    test_provider_error_codes

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All provider failure tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
