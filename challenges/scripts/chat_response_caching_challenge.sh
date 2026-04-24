#!/bin/bash
# Chat Response Caching Challenge
# Tests response caching behavior and cache invalidation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_response_caching" "Chat Response Caching Challenge"
load_env

log_info "Testing response caching behavior..."

# Test 1: Identical requests (may be cached)
test_identical_requests() {
    log_info "Test 1: Identical requests behavior"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is 2+2?"}],
        "max_tokens": 10,
        "temperature": 0.0
    }'

    # First request
    local start1=$(date +%s%N)
    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)
    local end1=$(date +%s%N)
    local latency1=$(( (end1 - start1) / 1000000 ))

    sleep 1

    # Second identical request
    local start2=$(date +%s%N)
    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)
    local end2=$(date +%s%N)
    local latency2=$(( (end2 - start2) / 1000000 ))

    local http_code1=$(echo "$response1" | tail -n1)
    local http_code2=$(echo "$response2" | tail -n1)

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "identical" "both_success" "true" "Both requests succeeded"

        # Compare latencies (cached may be faster)
        record_metric "first_request_ms" "$latency1"
        record_metric "second_request_ms" "$latency2"

        if [[ $latency2 -lt $latency1 ]]; then
            record_assertion "identical" "second_faster" "true" "Second request faster (possible cache hit)"
        else
            record_assertion "identical" "second_faster" "false" "Second not faster (no cache or disabled)"
        fi
    else
        record_assertion "identical" "both_success" "false" "One failed ($http_code1, $http_code2)"
    fi
}

# Test 2: Temperature variation affects caching
test_temperature_variation() {
    log_info "Test 2: Temperature variation and caching"

    # Same message, different temperature - should not be cached
    local request1='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Random word"}],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    local request2='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Random word"}],
        "max_tokens": 10,
        "temperature": 0.9
    }'

    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 30 2>/dev/null || true)

    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" --max-time 30 2>/dev/null || true)

    local http_code1=$(echo "$response1" | tail -n1)
    local http_code2=$(echo "$response2" | tail -n1)

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "temp_variation" "both_success" "true" "Both temperatures work"

        # Responses may differ due to temperature
        local body1=$(echo "$response1" | head -n -1)
        local body2=$(echo "$response2" | head -n -1)

        local content1=$(echo "$body1" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local content2=$(echo "$body2" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")

        if [[ "$content1" != "$content2" ]]; then
            record_assertion "temp_variation" "different_responses" "true" "Responses differ (cache respects temperature)"
        else
            record_assertion "temp_variation" "different_responses" "false" "Responses identical"
        fi
    else
        record_assertion "temp_variation" "both_success" "false" "One failed"
    fi
}

# Test 3: Model change affects caching
test_model_change() {
    log_info "Test 3: Model change and caching"

    # Get available models
    local models_response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local model1="helixagent-debate"
    local model2=$(echo "$models_response" | jq -r '.data[1].id // "helixagent-debate"' 2>/dev/null || echo "helixagent-debate")

    # Same message, different models
    local request1="{
        \"model\": \"$model1\",
        \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}],
        \"max_tokens\": 10,
        \"temperature\": 0.1
    }"

    local request2="{
        \"model\": \"$model2\",
        \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}],
        \"max_tokens\": 10,
        \"temperature\": 0.1
    }"

    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 30 2>/dev/null || true)

    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" --max-time 30 2>/dev/null || true)

    local http_code1=$(echo "$response1" | tail -n1)
    local http_code2=$(echo "$response2" | tail -n1)

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "model_change" "both_models" "true" "Both models work"

        # Check model field in responses
        local body1=$(echo "$response1" | head -n -1)
        local body2=$(echo "$response2" | head -n -1)

        local resp_model1=$(echo "$body1" | jq -r '.model' 2>/dev/null || echo "")
        local resp_model2=$(echo "$body2" | jq -r '.model' 2>/dev/null || echo "")

        if [[ "$resp_model1" != "$resp_model2" ]]; then
            record_assertion "model_change" "different_models" "true" "Responses from different models"
        else
            record_assertion "model_change" "different_models" "false" "Model field identical"
        fi
    else
        record_assertion "model_change" "both_models" "false" "One failed"
    fi
}

# Test 4: Message order affects caching
test_message_order() {
    log_info "Test 4: Message order and caching"

    # Different message order should not be cached together
    local request1='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "First"},
            {"role": "assistant", "content": "OK"},
            {"role": "user", "content": "Second"}
        ],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    local request2='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Second"},
            {"role": "assistant", "content": "OK"},
            {"role": "user", "content": "First"}
        ],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 30 2>/dev/null || true)

    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" --max-time 30 2>/dev/null || true)

    local http_code1=$(echo "$response1" | tail -n1)
    local http_code2=$(echo "$response2" | tail -n1)

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "msg_order" "both_success" "true" "Both message orders work"
        # Different order should produce different results (context-dependent)
    else
        record_assertion "msg_order" "both_success" "false" "One failed"
    fi
}

# Test 5: Cache with streaming
test_cache_with_streaming() {
    log_info "Test 5: Caching behavior with streaming"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Count to 3"}],
        "max_tokens": 20,
        "temperature": 0.0,
        "stream": true
    }'

    # First streaming request
    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/cache_stream_1.log" 2>&1 || true

    sleep 1

    # Second identical streaming request
    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/cache_stream_2.log" 2>&1 || true

    # Check both have data
    local has_data1=$(grep -c "data:" "$OUTPUT_DIR/logs/cache_stream_1.log" || echo "0")
    local has_data2=$(grep -c "data:" "$OUTPUT_DIR/logs/cache_stream_2.log" || echo "0")

    if [[ $has_data1 -gt 0 ]] && [[ $has_data2 -gt 0 ]]; then
        record_assertion "cache_stream" "both_streamed" "true" "Both streaming requests work"
    else
        record_assertion "cache_stream" "both_streamed" "false" "Stream counts: $has_data1, $has_data2"
    fi
}

# Main execution
main() {
    log_info "Starting Chat Response Caching Challenge..."

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
    test_identical_requests
    test_temperature_variation
    test_model_change
    test_message_order
    test_cache_with_streaming

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
