#!/bin/bash
# Chat Max Tokens Enforcement Challenge
# Tests max_tokens parameter enforcement and token limiting

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_max_tokens_enforcement" "Chat Max Tokens Enforcement Challenge"
load_env

log_info "Testing max_tokens parameter enforcement..."

# Test 1: Very small max_tokens (1-5)
test_very_small_max_tokens() {
    log_info "Test 1: Very small max_tokens (5 tokens)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Tell me a long story about adventures."}],
        "max_tokens": 5,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "small_tokens" "success" "true" "Small max_tokens request succeeded"

        local content=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local word_count=$(echo "$content" | wc -w)

        # Response should be truncated (small number of words)
        if [[ $word_count -le 10 ]]; then
            record_assertion "small_tokens" "enforced" "true" "Response truncated ($word_count words)"
        else
            record_assertion "small_tokens" "enforced" "false" "Response too long ($word_count words)"
        fi

        # Check finish_reason
        local finish_reason=$(echo "$body" | jq -r '.choices[0].finish_reason' 2>/dev/null || echo "")
        if [[ "$finish_reason" == "length" ]]; then
            record_assertion "small_tokens" "finish_reason" "true" "finish_reason is 'length'"
        else
            record_assertion "small_tokens" "finish_reason" "false" "finish_reason is '$finish_reason'"
        fi
    else
        record_assertion "small_tokens" "success" "false" "Request failed with $http_code"
    fi
}

# Test 2: Moderate max_tokens (50)
test_moderate_max_tokens() {
    log_info "Test 2: Moderate max_tokens (50 tokens)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Explain quantum physics."}],
        "max_tokens": 50,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "moderate_tokens" "success" "true" "Moderate max_tokens succeeded"

        # Check usage information
        local total_tokens=$(echo "$body" | jq -r '.usage.total_tokens' 2>/dev/null || echo "0")
        local completion_tokens=$(echo "$body" | jq -r '.usage.completion_tokens' 2>/dev/null || echo "0")

        if [[ $completion_tokens -le 50 ]]; then
            record_assertion "moderate_tokens" "within_limit" "true" "Completion tokens: $completion_tokens <= 50"
        else
            record_assertion "moderate_tokens" "within_limit" "false" "Completion tokens: $completion_tokens > 50"
        fi
    else
        record_assertion "moderate_tokens" "success" "false" "Request failed with $http_code"
    fi
}

# Test 3: Large max_tokens (500)
test_large_max_tokens() {
    log_info "Test 3: Large max_tokens (500 tokens)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is 2+2?"}],
        "max_tokens": 500,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "large_tokens" "success" "true" "Large max_tokens succeeded"

        # Response should complete naturally (not hit limit)
        local finish_reason=$(echo "$body" | jq -r '.choices[0].finish_reason' 2>/dev/null || echo "")
        if [[ "$finish_reason" == "stop" ]]; then
            record_assertion "large_tokens" "natural_stop" "true" "Completed naturally (stop)"
        else
            record_assertion "large_tokens" "natural_stop" "false" "finish_reason: $finish_reason"
        fi
    else
        record_assertion "large_tokens" "success" "false" "Request failed with $http_code"
    fi
}

# Test 4: max_tokens = 1
test_single_token() {
    log_info "Test 4: max_tokens = 1 (single token)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Say yes or no."}],
        "max_tokens": 1,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "single_token" "success" "true" "Single token request succeeded"

        local content=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local char_count=${#content}

        if [[ $char_count -le 5 ]]; then
            record_assertion "single_token" "truncated" "true" "Response very short ($char_count chars)"
        else
            record_assertion "single_token" "truncated" "false" "Response longer than expected"
        fi
    else
        record_assertion "single_token" "success" "false" "Request failed with $http_code"
    fi
}

# Test 5: No max_tokens specified (default)
test_default_max_tokens() {
    log_info "Test 5: No max_tokens specified (default behavior)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hello"}],
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "default_tokens" "success" "true" "Default max_tokens works"
    else
        record_assertion "default_tokens" "success" "false" "Request failed with $http_code"
    fi
}

# Test 6: max_tokens with streaming
test_max_tokens_streaming() {
    log_info "Test 6: max_tokens enforcement with streaming"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Write a paragraph."}],
        "max_tokens": 20,
        "temperature": 0.1,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/tokens_stream.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 30 > "$output_file" 2>&1 || true

    if grep -q "data:" "$output_file"; then
        record_assertion "tokens_stream" "works" "true" "Streaming works with max_tokens"

        # Count chunks to ensure it stops
        local chunk_count=$(grep -c "data:" "$output_file" || echo "0")
        if [[ $chunk_count -gt 0 ]] && [[ $chunk_count -le 50 ]]; then
            record_assertion "tokens_stream" "limited" "true" "Stream limited ($chunk_count chunks)"
        else
            record_assertion "tokens_stream" "limited" "false" "Unexpected chunk count: $chunk_count"
        fi
    else
        record_assertion "tokens_stream" "works" "false" "Streaming failed"
    fi
}

# Test 7: Usage metadata verification
test_usage_metadata() {
    log_info "Test 7: Usage metadata in response"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Count to 5."}],
        "max_tokens": 30,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        # Check for usage fields
        local has_usage=$(echo "$body" | jq -e '.usage' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_prompt=$(echo "$body" | jq -e '.usage.prompt_tokens' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_completion=$(echo "$body" | jq -e '.usage.completion_tokens' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_total=$(echo "$body" | jq -e '.usage.total_tokens' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_usage" == "true" ]]; then
            record_assertion "usage_meta" "has_usage" "true" "Usage object present"
        else
            record_assertion "usage_meta" "has_usage" "false" "No usage object"
        fi

        if [[ "$has_prompt" == "true" ]] && [[ "$has_completion" == "true" ]] && [[ "$has_total" == "true" ]]; then
            record_assertion "usage_meta" "complete" "true" "All usage fields present"

            # Verify math: total = prompt + completion
            local prompt=$(echo "$body" | jq -r '.usage.prompt_tokens' 2>/dev/null || echo "0")
            local completion=$(echo "$body" | jq -r '.usage.completion_tokens' 2>/dev/null || echo "0")
            local total=$(echo "$body" | jq -r '.usage.total_tokens' 2>/dev/null || echo "0")
            local expected=$((prompt + completion))

            if [[ $total -eq $expected ]]; then
                record_assertion "usage_meta" "math_correct" "true" "Token math correct ($prompt + $completion = $total)"
            else
                record_assertion "usage_meta" "math_correct" "false" "Token math wrong (expected $expected, got $total)"
            fi
        else
            record_assertion "usage_meta" "complete" "false" "Missing usage fields"
        fi
    else
        record_assertion "usage_meta" "http_status" "false" "Request failed with $http_code"
    fi
}

# Main execution
main() {
    log_info "Starting Chat Max Tokens Enforcement Challenge..."

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
    test_very_small_max_tokens
    test_moderate_max_tokens
    test_large_max_tokens
    test_single_token
    test_default_max_tokens
    test_max_tokens_streaming
    test_usage_metadata

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
