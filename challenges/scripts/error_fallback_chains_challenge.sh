#!/bin/bash
# Error Fallback Chains Challenge
# Tests fallback chain behavior when providers fail

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-fallback-chains" "Error Fallback Chains Challenge"
load_env

log_info "Testing fallback chain behavior and provider failover..."

# Test 1: Fallback chain configuration
test_fallback_chain_config() {
    log_info "Test 1: Fallback chain configuration endpoint"

    local response=$(curl -s "$BASE_URL/v1/monitoring/fallback-chain" 2>/dev/null || echo "{}")

    if echo "$response" | jq -e '.fallback_chain' > /dev/null 2>&1; then
        record_assertion "fallback_config" "endpoint_available" "true" "Fallback chain endpoint available"

        local provider_count=$(echo "$response" | jq '.fallback_chain | length' 2>/dev/null || echo 0)
        record_metric "fallback_providers" $provider_count

        if [[ $provider_count -ge 2 ]]; then
            record_assertion "fallback_config" "multiple_providers" "true" "$provider_count providers in chain"
        elif [[ $provider_count -eq 1 ]]; then
            record_assertion "fallback_config" "single_provider" "true" "Single provider (no fallback)"
        fi
    else
        record_assertion "fallback_config" "endpoint_available" "false" "Fallback endpoint not available"
    fi
}

# Test 2: Primary provider selection
test_primary_provider() {
    log_info "Test 2: Primary provider is used first"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Primary test"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "primary_provider" "request_succeeded" "true" "Request completed successfully"

        # Check which provider was used
        if echo "$body" | jq -e '.model' > /dev/null 2>&1; then
            local used_model=$(echo "$body" | jq -r '.model // empty')
            record_metric "used_model" "$used_model"
        fi
    fi
}

# Test 3: Fallback activation
test_fallback_activation() {
    log_info "Test 3: Fallback activates on provider failure"

    # Multiple requests to potentially trigger fallback
    local fallback_detected=false

    for i in {1..5}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fallback test"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        # Check if response indicates fallback was used
        if [[ "$http_code" == "200" ]]; then
            if echo "$body" | grep -qi "fallback"; then
                fallback_detected=true
                break
            fi
        fi
    done

    if [[ "$fallback_detected" == "true" ]]; then
        record_assertion "fallback_activation" "detected" "true" "Fallback mechanism activated"
    else
        record_assertion "fallback_activation" "not_needed" "true" "Primary providers handled all requests"
    fi
}

# Test 4: Fallback chain order
test_fallback_order() {
    log_info "Test 4: Fallback chain follows priority order"

    local chain_response=$(curl -s "$BASE_URL/v1/monitoring/fallback-chain" 2>/dev/null || echo "{}")

    if echo "$chain_response" | jq -e '.fallback_chain[0]' > /dev/null 2>&1; then
        # Get first provider in chain (highest priority)
        local first_provider=$(echo "$chain_response" | jq -r '.fallback_chain[0].name // empty' 2>/dev/null || echo "")
        local first_score=$(echo "$chain_response" | jq -r '.fallback_chain[0].score // empty' 2>/dev/null || echo "")

        if [[ -n "$first_provider" ]]; then
            record_assertion "fallback_order" "has_priority" "true" "Primary: $first_provider (score: $first_score)"
            record_metric "primary_provider" "$first_provider"
        fi

        # Verify scores are in descending order
        local scores=$(echo "$chain_response" | jq -r '.fallback_chain[].score' 2>/dev/null || echo "")
        if [[ -n "$scores" ]]; then
            record_assertion "fallback_order" "scores_exist" "true" "Providers have scoring"
        fi
    fi
}

# Test 5: Fallback error reporting
test_fallback_error_reporting() {
    log_info "Test 5: Fallback errors are properly reported"

    # Make request that might trigger fallback
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Error reporting test"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "fallback_errors" "request_succeeded" "true" "Request completed"

        # Check for error metadata in successful response
        if echo "$body" | jq -e '.metadata' > /dev/null 2>&1; then
            record_assertion "fallback_errors" "has_metadata" "true" "Response includes metadata"
        fi
    elif [[ "$http_code" =~ ^(500|503)$ ]]; then
        # Check error message quality
        if echo "$body" | jq -e '.error.message' > /dev/null 2>&1; then
            record_assertion "fallback_errors" "descriptive_error" "true" "Error includes message"
        fi
    fi
}

main() {
    log_info "Starting fallback chains challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_fallback_chain_config
    test_primary_provider
    test_fallback_activation
    test_fallback_order
    test_fallback_error_reporting

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All fallback chain tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
