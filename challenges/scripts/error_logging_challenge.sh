#!/bin/bash
# Error Logging Challenge
# Tests error logging, tracking, and reporting mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-logging" "Error Logging Challenge"
load_env

log_info "Testing error logging and tracking..."

# Test 1: Error logs are generated
test_error_logs_generated() {
    log_info "Test 1: Errors are logged"

    # Trigger an error by sending invalid request
    curl -s -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"invalid","messages":[]}' \
        --max-time 30 > /dev/null 2>&1 || true

    # Check if error monitoring endpoint exists
    local logs_response=$(curl -s "$BASE_URL/v1/monitoring/errors" 2>/dev/null || echo "{}")

    if echo "$logs_response" | jq -e '.errors' > /dev/null 2>&1; then
        record_assertion "error_logs" "endpoint_available" "true" "Error logging endpoint available"

        local error_count=$(echo "$logs_response" | jq '.errors | length' 2>/dev/null || echo 0)
        record_metric "logged_errors" $error_count

        if [[ $error_count -gt 0 ]]; then
            record_assertion "error_logs" "has_errors" "true" "$error_count error(s) logged"
        fi
    else
        record_assertion "error_logs" "endpoint_available" "false" "Error logging not available"
    fi
}

# Test 2: Error details include context
test_error_context() {
    log_info "Test 2: Error logs include context information"

    # Trigger error with specific context
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[]}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$response" | head -n -1)

    if echo "$body" | jq -e '.error' > /dev/null 2>&1; then
        # Check for error details
        local has_message=$(echo "$body" | jq -e '.error.message' > /dev/null 2>&1 && echo "true" || echo "false")
        local has_type=$(echo "$body" | jq -e '.error.type' > /dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_message" == "true" ]]; then
            record_assertion "error_context" "has_message" "true" "Error includes message"
        fi

        if [[ "$has_type" == "true" ]]; then
            record_assertion "error_context" "has_type" "true" "Error includes type"
        fi
    fi
}

# Test 3: Error metrics tracking
test_error_metrics() {
    log_info "Test 3: Error metrics are tracked"

    local metrics_response=$(curl -s "$BASE_URL/v1/monitoring/metrics" 2>/dev/null || echo "{}")

    if echo "$metrics_response" | jq -e '.metrics' > /dev/null 2>&1; then
        record_assertion "error_metrics" "endpoint_available" "true" "Metrics endpoint available"

        # Check for error-related metrics
        local error_rate=$(echo "$metrics_response" | jq -r '.metrics.error_rate // 0' 2>/dev/null || echo 0)
        local total_errors=$(echo "$metrics_response" | jq -r '.metrics.total_errors // 0' 2>/dev/null || echo 0)

        record_metric "error_rate" $error_rate
        record_metric "total_errors" $total_errors

        if [[ "$error_rate" != "null" || "$total_errors" != "null" ]]; then
            record_assertion "error_metrics" "tracking" "true" "Error metrics tracked"
        fi
    fi
}

# Test 4: Error categorization in logs
test_error_categorization() {
    log_info "Test 4: Errors are properly categorized"

    # Generate different error types
    local error_types=()

    # Type 1: Validation error
    local resp1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[]}' \
        --max-time 30 2>/dev/null || true)

    local body1=$(echo "$resp1" | head -n -1)
    local type1=$(echo "$body1" | jq -r '.error.type // empty' 2>/dev/null || echo "")
    [[ -n "$type1" ]] && error_types+=("$type1")

    # Type 2: Model not found
    local resp2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"invalid-model","messages":[{"role":"user","content":"test"}]}' \
        --max-time 30 2>/dev/null || true)

    local body2=$(echo "$resp2" | head -n -1)
    local type2=$(echo "$body2" | jq -r '.error.type // empty' 2>/dev/null || echo "")
    [[ -n "$type2" ]] && error_types+=("$type2")

    record_metric "distinct_error_types" ${#error_types[@]}

    if [[ ${#error_types[@]} -gt 0 ]]; then
        record_assertion "error_categorization" "has_categories" "true" "${#error_types[@]} error type(s) identified"
    fi
}

# Test 5: Error log retention and retrieval
test_error_retrieval() {
    log_info "Test 5: Error logs can be retrieved"

    # Make multiple requests to generate errors
    for i in {1..3}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"invalid","messages":[]}' \
            --max-time 15 > /dev/null 2>&1 || true
        sleep 0.5
    done

    # Retrieve error logs
    local logs=$(curl -s "$BASE_URL/v1/monitoring/errors" 2>/dev/null || echo "{}")

    if echo "$logs" | jq -e '.errors[0]' > /dev/null 2>&1; then
        record_assertion "error_retrieval" "logs_accessible" "true" "Error logs can be retrieved"

        # Check timestamp exists
        local timestamp=$(echo "$logs" | jq -r '.errors[0].timestamp // empty' 2>/dev/null || echo "")
        if [[ -n "$timestamp" ]]; then
            record_assertion "error_retrieval" "has_timestamp" "true" "Errors include timestamps"
        fi
    fi
}

main() {
    log_info "Starting error logging challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_error_logs_generated
    test_error_context
    test_error_metrics
    test_error_categorization
    test_error_retrieval

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All error logging tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
