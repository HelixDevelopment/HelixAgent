#!/bin/bash
# Chat Temperature Variation Challenge
# Tests different temperature settings and their impact on responses

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_temperature_variation" "Chat Temperature Variation Challenge"
load_env

log_info "Testing temperature parameter variations..."

# Test 1: Temperature 0.0 (deterministic)
test_temperature_zero() {
    log_info "Test 1: Temperature 0.0 (deterministic responses)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is 2+2?"}],
        "max_tokens": 10,
        "temperature": 0.0
    }'

    # Run same request twice
    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code1=$(echo "$response1" | tail -n1)
    local body1=$(echo "$response1" | head -n -1)

    sleep 1

    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code2=$(echo "$response2" | tail -n1)
    local body2=$(echo "$response2" | head -n -1)

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "temp_zero" "both_success" "true" "Both requests succeeded"

        local content1=$(echo "$body1" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local content2=$(echo "$body2" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")

        # Responses should be very similar (both should mention 4)
        if echo "$content1" | grep -qi "4" && echo "$content2" | grep -qi "4"; then
            record_assertion "temp_zero" "consistent" "true" "Both responses mention 4 (deterministic)"
        else
            record_assertion "temp_zero" "consistent" "false" "Responses differ"
        fi
    else
        record_assertion "temp_zero" "both_success" "false" "One or both requests failed"
    fi
}

# Test 2: Temperature 0.7 (balanced)
test_temperature_balanced() {
    log_info "Test 2: Temperature 0.7 (balanced creativity)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Tell me a creative adjective."}],
        "max_tokens": 20,
        "temperature": 0.7
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "temp_balanced" "success" "true" "Temperature 0.7 request succeeded"

        local content=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        if [[ -n "$content" ]] && [[ ${#content} -gt 0 ]]; then
            record_assertion "temp_balanced" "has_content" "true" "Response has content"
        else
            record_assertion "temp_balanced" "has_content" "false" "No content"
        fi
    else
        record_assertion "temp_balanced" "success" "false" "Request failed with $http_code"
    fi
}

# Test 3: Temperature 1.0 (creative)
test_temperature_high() {
    log_info "Test 3: Temperature 1.0 (high creativity)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Say hello creatively."}],
        "max_tokens": 30,
        "temperature": 1.0
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "temp_high" "success" "true" "Temperature 1.0 request succeeded"

        local content=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        if [[ -n "$content" ]]; then
            record_assertion "temp_high" "has_response" "true" "Creative response generated"
        else
            record_assertion "temp_high" "has_response" "false" "No response"
        fi
    else
        record_assertion "temp_high" "success" "false" "Request failed with $http_code"
    fi
}

# Test 4: Temperature 2.0 (maximum)
test_temperature_max() {
    log_info "Test 4: Temperature 2.0 (maximum creativity)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Random word."}],
        "max_tokens": 10,
        "temperature": 2.0
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # May accept or reject temperature 2.0
    if [[ "$http_code" == "200" ]]; then
        record_assertion "temp_max" "accepted" "true" "Temperature 2.0 accepted"
    elif [[ "$http_code" == "400" ]]; then
        record_assertion "temp_max" "accepted" "false" "Temperature 2.0 rejected (400)"
    else
        record_assertion "temp_max" "accepted" "false" "Unexpected status: $http_code"
    fi
}

# Test 5: No temperature specified (default)
test_temperature_default() {
    log_info "Test 5: No temperature specified (default behavior)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hello"}],
        "max_tokens": 20
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "temp_default" "success" "true" "Default temperature works"
    else
        record_assertion "temp_default" "success" "false" "Request failed with $http_code"
    fi
}

# Test 6: Temperature impact on consistency
test_temperature_consistency() {
    log_info "Test 6: Temperature impact on response consistency"

    # Low temperature - expect similar responses
    local request_low='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Count: 1"}],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    local responses_low=()
    for i in {1..3}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request_low" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        if [[ "$http_code" == "200" ]]; then
            local body=$(echo "$response" | head -n -1)
            local content=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
            responses_low+=("$content")
        fi
        sleep 0.5
    done

    # Check if responses are similar
    if [[ ${#responses_low[@]} -eq 3 ]]; then
        # All responses should contain similar patterns
        local similar_count=0
        for response in "${responses_low[@]}"; do
            if echo "$response" | grep -qE "(1|2|one|two)"; then
                similar_count=$((similar_count + 1))
            fi
        done

        if [[ $similar_count -ge 2 ]]; then
            record_assertion "temp_consistency" "low_temp" "true" "Low temperature shows consistency ($similar_count/3 similar)"
        else
            record_assertion "temp_consistency" "low_temp" "false" "Low temperature inconsistent"
        fi
    else
        record_assertion "temp_consistency" "low_temp" "false" "Failed to get 3 responses"
    fi
}

# Test 7: Temperature with streaming
test_temperature_with_streaming() {
    log_info "Test 7: Temperature with streaming enabled"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hi"}],
        "max_tokens": 20,
        "temperature": 0.5,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/temp_stream.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 30 > "$output_file" 2>&1 || true

    if grep -q "data:" "$output_file"; then
        record_assertion "temp_stream" "works" "true" "Temperature works with streaming"
    else
        record_assertion "temp_stream" "works" "false" "Streaming failed with temperature"
    fi
}

# Main execution
main() {
    log_info "Starting Chat Temperature Variation Challenge..."

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
    test_temperature_zero
    test_temperature_balanced
    test_temperature_high
    test_temperature_max
    test_temperature_default
    test_temperature_consistency
    test_temperature_with_streaming

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
