#!/bin/bash
# Chat Rate Limiting Challenge
# Tests rate limiting and throttling behavior

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_rate_limiting" "Chat Rate Limiting Challenge"
load_env

log_info "Testing rate limiting and throttling..."

# Test 1: Baseline request (no rate limiting)
test_baseline_request() {
    log_info "Test 1: Baseline request"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "baseline" "works" "true" "Baseline request succeeds"
    else
        record_assertion "baseline" "works" "false" "Baseline failed: $http_code"
    fi
}

# Test 2: Burst of requests
test_burst_requests() {
    log_info "Test 2: Burst of rapid requests (10 requests)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Quick"}],
        "max_tokens": 5
    }'

    local success_count=0
    local rate_limited_count=0

    for i in {1..10}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        elif [[ "$http_code" == "429" ]]; then
            rate_limited_count=$((rate_limited_count + 1))
        fi
    done

    record_metric "burst_success" "$success_count"
    record_metric "burst_rate_limited" "$rate_limited_count"

    if [[ $success_count -gt 0 ]]; then
        record_assertion "burst" "some_succeed" "true" "$success_count/10 requests succeeded"

        if [[ $rate_limited_count -gt 0 ]]; then
            record_assertion "burst" "rate_limiting" "true" "Rate limiting active ($rate_limited_count 429s)"
        else
            record_assertion "burst" "rate_limiting" "false" "No rate limiting observed"
        fi
    else
        record_assertion "burst" "some_succeed" "false" "All requests failed"
    fi
}

# Test 3: Rate limit headers
test_rate_limit_headers() {
    log_info "Test 3: Rate limit headers in response"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10
    }'

    local output_file="$OUTPUT_DIR/logs/rate_limit_headers.log"

    curl -s -D "$output_file" -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > /dev/null 2>&1 || true

    # Check for common rate limit headers
    local has_rate_limit=$(grep -iE "(X-RateLimit|RateLimit)" "$output_file" || echo "")

    if [[ -n "$has_rate_limit" ]]; then
        record_assertion "headers" "present" "true" "Rate limit headers present"

        # Check for specific headers
        if grep -qi "X-RateLimit-Limit" "$output_file" || grep -qi "RateLimit-Limit" "$output_file"; then
            record_assertion "headers" "limit_header" "true" "Limit header present"
        else
            record_assertion "headers" "limit_header" "false" "No limit header"
        fi

        if grep -qi "X-RateLimit-Remaining" "$output_file" || grep -qi "RateLimit-Remaining" "$output_file"; then
            record_assertion "headers" "remaining_header" "true" "Remaining header present"
        else
            record_assertion "headers" "remaining_header" "false" "No remaining header"
        fi
    else
        record_assertion "headers" "present" "false" "No rate limit headers found"
    fi
}

# Test 4: 429 response format
test_429_response_format() {
    log_info "Test 4: 429 response format (if rate limited)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 5
    }'

    # Send many requests to trigger rate limiting
    local got_429=false
    local response_body=""

    for i in {1..20}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "429" ]]; then
            got_429=true
            response_body=$(echo "$response" | head -n -1)
            break
        fi

        sleep 0.1
    done

    if [[ "$got_429" == "true" ]]; then
        record_assertion "429_format" "triggered" "true" "Rate limit triggered (429)"

        # Check error format
        local has_error=$(echo "$response_body" | jq -e '.error' >/dev/null 2>&1 && echo "true" || echo "false")
        if [[ "$has_error" == "true" ]]; then
            record_assertion "429_format" "error_object" "true" "429 response has error object"

            # Check for retry-after or similar info
            if echo "$response_body" | grep -qiE "(retry|wait|rate|limit)"; then
                record_assertion "429_format" "helpful_message" "true" "Error message mentions rate limiting"
            else
                record_assertion "429_format" "helpful_message" "false" "No rate limit details"
            fi
        else
            record_assertion "429_format" "error_object" "false" "No error object in 429"
        fi
    else
        record_assertion "429_format" "triggered" "false" "Could not trigger rate limit"
    fi
}

# Test 5: Recovery after rate limit
test_recovery_after_limit() {
    log_info "Test 5: Recovery after rate limit"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 5
    }'

    # Burst to potentially hit limit
    for i in {1..15}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 > /dev/null 2>&1 || true
    done

    # Wait for recovery
    sleep 3

    # Try again
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "recovery" "works" "true" "Recovered after rate limit"
    elif [[ "$http_code" == "429" ]]; then
        record_assertion "recovery" "works" "false" "Still rate limited after 3s"
    else
        record_assertion "recovery" "works" "false" "Unexpected status: $http_code"
    fi
}

# Test 6: Per-model rate limits (if different)
test_per_model_limits() {
    log_info "Test 6: Per-model rate limits"

    # Test debate model
    local request_debate='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 5
    }'

    local success_debate=0
    for i in {1..5}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request_debate" --max-time 30 2>/dev/null || true)

        [[ $(echo "$response" | tail -n1) == "200" ]] && success_debate=$((success_debate + 1))
    done

    if [[ $success_debate -gt 0 ]]; then
        record_assertion "per_model" "debate_works" "true" "Debate model: $success_debate/5 succeeded"
    else
        record_assertion "per_model" "debate_works" "false" "All debate requests failed"
    fi

    record_metric "per_model_success" "$success_debate"
}

# Test 7: Sustained load (gradual requests)
test_sustained_load() {
    log_info "Test 7: Sustained load over time"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 5
    }'

    local success_count=0
    local total_count=10

    for i in $(seq 1 $total_count); do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        [[ "$http_code" == "200" ]] && success_count=$((success_count + 1))

        # Gradual: 0.5s between requests
        sleep 0.5
    done

    local success_rate=$((success_count * 100 / total_count))

    if [[ $success_rate -ge 80 ]]; then
        record_assertion "sustained" "high_success" "true" "$success_count/$total_count succeeded ($success_rate%)"
    elif [[ $success_rate -ge 50 ]]; then
        record_assertion "sustained" "high_success" "false" "Only $success_rate% success (moderate)"
    else
        record_assertion "sustained" "high_success" "false" "Only $success_rate% success (low)"
    fi

    record_metric "sustained_success_rate" "$success_rate"
}

# Main execution
main() {
    log_info "Starting Chat Rate Limiting Challenge..."

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
    test_baseline_request
    test_burst_requests
    test_rate_limit_headers
    test_429_response_format
    test_recovery_after_limit
    test_per_model_limits
    test_sustained_load

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
