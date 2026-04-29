#!/bin/bash
# Chat Max Tokens Challenge
# Tests max_tokens parameter enforcement and behavior

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "chat-max-tokens" "Chat Max Tokens Challenge"
load_env

log_info "Testing max_tokens parameter behavior..."

# Test 1: Very small max_tokens
test_small_max_tokens() {
    log_info "Test 1: Very small max_tokens (5 tokens)"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Write a long story about space exploration"}
        ],
        "max_tokens": 5,
        "temperature": 0.7
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "small_tokens" "http_status" "true" "Small max_tokens accepted"

        # Check that response is actually short
        local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
        local word_count=$(echo "$content" | wc -w)
        record_metric "small_tokens_word_count" $word_count

        # With 5 tokens max, should get very short response (rough approximation: ~5-10 words)
        if [[ $word_count -le 15 ]]; then
            record_assertion "small_tokens" "enforced" "true" "Response is short ($word_count words)"
        else
            record_assertion "small_tokens" "enforced" "false" "Response too long ($word_count words)"
        fi

        # Check finish_reason
        local finish_reason=$(echo "$body" | jq -r '.choices[0].finish_reason // empty')
        if [[ "$finish_reason" == "length" ]]; then
            record_assertion "small_tokens" "finish_reason" "true" "Finish reason is 'length'"
        else
            record_assertion "small_tokens" "finish_reason" "false" "Finish reason: $finish_reason"
        fi
    else
        record_assertion "small_tokens" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 2: Large max_tokens
test_large_max_tokens() {
    log_info "Test 2: Large max_tokens (500 tokens)"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is 2+2?"}
        ],
        "max_tokens": 500,
        "temperature": 0.1
    }'

    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "large_tokens" "http_status" "true" "Large max_tokens accepted"
        record_metric "large_tokens_latency_ms" $latency

        # Simple question shouldn't use all 500 tokens
        local usage=$(echo "$body" | jq -r '.usage.completion_tokens // 0')
        record_metric "large_tokens_completion_tokens" $usage

        if [[ $usage -lt 500 ]]; then
            record_assertion "large_tokens" "not_overused" "true" "Used $usage/500 tokens"
        fi

        # Finish reason should be 'stop' for complete response
        local finish_reason=$(echo "$body" | jq -r '.choices[0].finish_reason // empty')
        if [[ "$finish_reason" == "stop" ]]; then
            record_assertion "large_tokens" "finish_reason_stop" "true" "Finished naturally"
        fi
    else
        record_assertion "large_tokens" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 3: Progressive max_tokens scaling
test_progressive_max_tokens() {
    log_info "Test 3: Progressive max_tokens scaling"

    local token_limits=(10 50 100 200)
    local response_lengths=()

    for limit in "${token_limits[@]}"; do
        local request="{
            \"model\": \"helixagent-debate\",
            \"messages\": [{\"role\": \"user\", \"content\": \"Write about artificial intelligence\"}],
            \"max_tokens\": $limit,
            \"temperature\": 0.5
        }"

        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 60 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        if [[ "$http_code" == "200" ]]; then
            local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
            local length=${#content}
            response_lengths+=($length)
            record_metric "tokens_${limit}_response_length" $length
        fi
    done

    # Check that responses generally get longer with more tokens
    if [[ ${#response_lengths[@]} -eq 4 ]]; then
        record_assertion "progressive" "all_completed" "true" "All token limits tested"

        # Rough check: larger limits should generally produce longer responses
        # (though not guaranteed due to model behavior)
        if [[ ${response_lengths[3]} -ge ${response_lengths[0]} ]]; then
            record_assertion "progressive" "scaling" "true" "Response length scales with max_tokens"
        else
            record_assertion "progressive" "scaling" "false" "No clear scaling pattern"
        fi
    else
        record_assertion "progressive" "all_completed" "false" "Only ${#response_lengths[@]}/4 completed"
    fi
}

# Test 4: Max tokens with streaming
test_max_tokens_streaming() {
    log_info "Test 4: Max tokens with streaming enabled"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Count to 100"}
        ],
        "max_tokens": 20,
        "temperature": 0.5,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_max_tokens.txt"
    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 > "$output_file" 2>/dev/null || true

    if grep -q "data:" "$output_file"; then
        record_assertion "stream_tokens" "streaming_works" "true" "Streaming with max_tokens works"

        # Count chunks
        local chunk_count=$(grep -c "data:" "$output_file" || echo 0)
        record_metric "stream_tokens_chunks" $chunk_count

        # Check for [DONE] marker
        if grep -q "data: \[DONE\]" "$output_file"; then
            record_assertion "stream_tokens" "completion_marker" "true" "Stream completed with [DONE]"
        fi
    else
        record_assertion "stream_tokens" "streaming_works" "false" "No SSE data received"
    fi
}

# Test 5: Edge case - max_tokens = 1
test_single_token() {
    log_info "Test 5: Edge case - max_tokens = 1"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Say yes or no"}
        ],
        "max_tokens": 1,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "single_token" "http_status" "true" "Single token request accepted"

        local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
        local word_count=$(echo "$content" | wc -w)
        record_metric "single_token_words" $word_count

        # With 1 token, should get 1-2 words at most
        if [[ $word_count -le 2 ]]; then
            record_assertion "single_token" "enforced" "true" "Response limited to $word_count word(s)"
        else
            record_assertion "single_token" "enforced" "false" "Response has $word_count words"
        fi
    else
        record_assertion "single_token" "http_status" "false" "HTTP $http_code"
    fi
}

main() {
    log_info "Starting max_tokens challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_small_max_tokens
    test_large_max_tokens
    test_progressive_max_tokens
    test_max_tokens_streaming
    test_single_token

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All max_tokens tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
