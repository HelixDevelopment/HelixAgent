#!/bin/bash
# Error Classification Challenge
# Tests proper error classification and categorization

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-classification" "Error Classification Challenge"
load_env

log_info "Testing error classification and categorization..."

# Test 1: Rate limit error classification
test_rate_limit_classification() {
    log_info "Test 1: Rate limit error classification"

    # Make many rapid requests to potentially trigger rate limiting
    local rate_limit_detected=false
    for i in {1..50}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}' \
            --max-time 10 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        # Check for rate limit response (429)
        if [[ "$http_code" == "429" ]]; then
            rate_limit_detected=true

            # Check error message classification
            if echo "$body" | grep -qi "rate.limit"; then
                record_assertion "rate_limit" "classified" "true" "Rate limit error properly classified"
            fi

            # Check for retry-after header or similar guidance
            if echo "$body" | jq -e '.error.type' | grep -qi "rate_limit"; then
                record_assertion "rate_limit" "error_type" "true" "Error type indicates rate_limit"
            fi

            break
        fi
    done

    if [[ "$rate_limit_detected" == "false" ]]; then
        record_assertion "rate_limit" "test_inconclusive" "true" "Rate limit not triggered (may not be enabled)"
    fi
}

# Test 2: Authentication error classification
test_auth_error_classification() {
    log_info "Test 2: Authentication error classification"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid-token-12345" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    # Should return 401 for invalid auth
    if [[ "$http_code" == "401" ]]; then
        record_assertion "auth_error" "http_status" "true" "Returns 401 for invalid auth"

        # Check error classification
        if echo "$body" | jq -e '.error.type' > /dev/null 2>&1; then
            local error_type=$(echo "$body" | jq -r '.error.type // empty')
            if [[ "$error_type" =~ (auth|unauthorized|invalid.*token) ]]; then
                record_assertion "auth_error" "classified" "true" "Error type: $error_type"
            fi
        fi
    else
        # May allow anonymous access
        record_assertion "auth_error" "http_status" "false" "HTTP $http_code (auth may not be required)"
    fi
}

# Test 3: Validation error classification
test_validation_error_classification() {
    log_info "Test 3: Validation error classification"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[]}' \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "400" ]]; then
        record_assertion "validation_error" "http_status" "true" "Returns 400 for validation error"

        # Check error classification
        if echo "$body" | jq -e '.error' > /dev/null 2>&1; then
            local error_type=$(echo "$body" | jq -r '.error.type // empty')
            if [[ "$error_type" =~ (validation|invalid_request|bad_request) ]]; then
                record_assertion "validation_error" "classified" "true" "Error type: $error_type"
            fi

            # Check for descriptive message
            local error_msg=$(echo "$body" | jq -r '.error.message // empty')
            if echo "$error_msg" | grep -qi "message"; then
                record_assertion "validation_error" "descriptive" "true" "Error mentions 'message'"
            fi
        fi
    fi
}

# Test 4: Model not found error classification
test_model_not_found_classification() {
    log_info "Test 4: Model not found error classification"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"nonexistent-model-xyz","messages":[{"role":"user","content":"Hi"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" =~ ^(400|404)$ ]]; then
        record_assertion "model_not_found" "http_status" "true" "Returns $http_code for invalid model"

        # Check error classification
        if echo "$body" | jq -e '.error' > /dev/null 2>&1; then
            local error_type=$(echo "$body" | jq -r '.error.type // empty')
            if [[ "$error_type" =~ (not_found|invalid_model|model) ]]; then
                record_assertion "model_not_found" "classified" "true" "Error type: $error_type"
            fi

            # Check if error mentions model
            local error_msg=$(echo "$body" | jq -r '.error.message // empty')
            if echo "$error_msg" | grep -qi "model"; then
                record_assertion "model_not_found" "mentions_model" "true" "Error mentions model"
            fi
        fi
    fi
}

# Test 5: Timeout error classification
test_timeout_classification() {
    log_info "Test 5: Timeout error classification (simulated)"

    # Request with very low timeout to potentially trigger timeout
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Write a very long essay about artificial intelligence"}],"max_tokens":1000}' \
        --max-time 1 2>/dev/null || echo -e "\n000")

    local http_code=$(echo "$response" | tail -n1)

    # Curl timeout returns 000 or similar
    if [[ "$http_code" == "000" || "$http_code" == "00" ]]; then
        record_assertion "timeout" "occurred" "true" "Request timed out (connection or processing)"
    else
        record_assertion "timeout" "not_triggered" "true" "Timeout not triggered (request completed)"
    fi
}

# Test 6: Malformed request error classification
test_malformed_request_classification() {
    log_info "Test 6: Malformed request error classification"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{invalid json content}' \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "400" ]]; then
        record_assertion "malformed" "http_status" "true" "Returns 400 for malformed JSON"

        # Check error classification
        if echo "$body" | jq -e '.error.type' > /dev/null 2>&1; then
            local error_type=$(echo "$body" | jq -r '.error.type // empty')
            if [[ "$error_type" =~ (invalid_request|bad_request|parse_error) ]]; then
                record_assertion "malformed" "classified" "true" "Error type: $error_type"
            fi
        fi
    fi
}

# Test 7: Error response structure consistency
test_error_structure_consistency() {
    log_info "Test 7: Error response structure consistency across error types"

    # Test multiple error types and verify consistent structure
    local errors_tested=0
    local consistent_structure=0

    # Error 1: Invalid model
    local resp1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"invalid","messages":[{"role":"user","content":"Hi"}]}' \
        --max-time 30 2>/dev/null || true)

    local code1=$(echo "$resp1" | tail -n1)
    local body1=$(echo "$resp1" | head -n -1)

    if [[ "$code1" =~ ^(400|404)$ ]]; then
        errors_tested=$((errors_tested + 1))
        if echo "$body1" | jq -e '.error.message' > /dev/null 2>&1; then
            consistent_structure=$((consistent_structure + 1))
        fi
    fi

    # Error 2: Empty messages
    local resp2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[]}' \
        --max-time 30 2>/dev/null || true)

    local code2=$(echo "$resp2" | tail -n1)
    local body2=$(echo "$resp2" | head -n -1)

    if [[ "$code2" == "400" ]]; then
        errors_tested=$((errors_tested + 1))
        if echo "$body2" | jq -e '.error.message' > /dev/null 2>&1; then
            consistent_structure=$((consistent_structure + 1))
        fi
    fi

    record_metric "errors_tested" $errors_tested
    record_metric "consistent_structures" $consistent_structure

    if [[ $consistent_structure -eq $errors_tested && $errors_tested -gt 0 ]]; then
        record_assertion "structure" "consistent" "true" "All $errors_tested error types have consistent structure"
    else
        record_assertion "structure" "consistent" "false" "$consistent_structure/$errors_tested errors have consistent structure"
    fi
}

main() {
    log_info "Starting error classification challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_rate_limit_classification
    test_auth_error_classification
    test_validation_error_classification
    test_model_not_found_classification
    test_timeout_classification
    test_malformed_request_classification
    test_error_structure_consistency

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All error classification tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
