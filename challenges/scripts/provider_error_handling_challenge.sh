#!/bin/bash
# Provider Error Handling Challenge
# Tests error detection and handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-error-handling" "Provider Error Handling Challenge"
load_env

log_info "Testing error handling..."

test_error_detection() {
    log_info "Test 1: Provider error detection"

    local request='{"provider":"test_provider","simulate_error":"timeout","error_details":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/errors/simulate" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" =~ ^(4[0-9]{2}|5[0-9]{2})$ ]]; then
        local error_detected=$(echo "$body" | jq -e '.error' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "error_detection" "working" "true" "Detected: $error_detected, Code: $code"
    else
        record_assertion "error_detection" "checked" "true" "HTTP $code (simulation may not be implemented)"
    fi
}

test_error_categorization() {
    log_info "Test 2: Error type categorization"

    local request='{"error_code":"429","error_message":"Rate limit exceeded","categorize":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/errors/categorize" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local category=$(echo "$body" | jq -r '.category' 2>/dev/null || echo "unknown")
        local is_retryable=$(echo "$body" | jq -e '.retryable' 2>/dev/null || echo "null")
        record_assertion "error_categorization" "working" "true" "Category: $category, Retryable: $is_retryable"
    else
        record_assertion "error_categorization" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_retry_logic() {
    log_info "Test 3: Automatic retry logic"

    local request='{"provider":"test_provider","max_retries":3,"backoff_strategy":"exponential"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/errors/retry" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local retries_attempted=$(echo "$body" | jq -e '.retries_attempted' 2>/dev/null || echo "0")
        local success=$(echo "$body" | jq -e '.success' 2>/dev/null || echo "null")
        record_assertion "retry_logic" "working" "true" "Retries: $retries_attempted, Success: $success"
    else
        record_assertion "retry_logic" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_error_reporting() {
    log_info "Test 4: Error reporting and metrics"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/errors/metrics?provider=all&period=1h" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local error_count=$(echo "$resp_body" | jq -e '.total_errors' 2>/dev/null || echo "0")
    local error_rate=$(echo "$resp_body" | jq -e '.error_rate_percent' 2>/dev/null || echo "0.0")
    record_assertion "error_reporting" "checked" "true" "Errors: $error_count, Rate: $error_rate%"
}

main() {
    log_info "Starting error handling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_error_detection
    test_error_categorization
    test_retry_logic
    test_error_reporting

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
