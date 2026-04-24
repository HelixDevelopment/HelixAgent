#!/bin/bash
# Chat Concurrent Requests Challenge
# Tests handling of multiple concurrent chat requests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_concurrent_requests" "Chat Concurrent Requests Challenge"
load_env

log_info "Testing concurrent request handling..."

# Test 1: Two concurrent requests
test_two_concurrent() {
    log_info "Test 1: Two concurrent requests"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is 2+2?"}],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    # Start both requests in background
    curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/concurrent_1.log" 2>&1 &
    local pid1=$!

    curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/concurrent_2.log" 2>&1 &
    local pid2=$!

    # Wait for both to complete
    wait $pid1
    wait $pid2

    # Check results
    local http_code1=$(tail -n1 "$OUTPUT_DIR/logs/concurrent_1.log")
    local http_code2=$(tail -n1 "$OUTPUT_DIR/logs/concurrent_2.log")

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "two_concurrent" "both_success" "true" "Both concurrent requests succeeded"
    else
        record_assertion "two_concurrent" "both_success" "false" "One failed ($http_code1, $http_code2)"
    fi
}

# Test 2: Five concurrent requests
test_five_concurrent() {
    log_info "Test 2: Five concurrent requests"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hello"}],
        "max_tokens": 10
    }'

    local pids=()
    for i in {1..5}; do
        curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/concurrent_5_${i}.log" 2>&1 &
        pids+=($!)
    done

    # Wait for all
    for pid in "${pids[@]}"; do
        wait $pid
    done

    # Count successes
    local success_count=0
    for i in {1..5}; do
        local http_code=$(tail -n1 "$OUTPUT_DIR/logs/concurrent_5_${i}.log")
        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        fi
    done

    if [[ $success_count -eq 5 ]]; then
        record_assertion "five_concurrent" "all_success" "true" "All 5 concurrent requests succeeded"
    elif [[ $success_count -ge 3 ]]; then
        record_assertion "five_concurrent" "all_success" "false" "Only $success_count/5 succeeded (partial)"
    else
        record_assertion "five_concurrent" "all_success" "false" "Only $success_count/5 succeeded (failed)"
    fi

    record_metric "concurrent_success_count" "$success_count"
}

# Test 3: Concurrent requests with different models
test_concurrent_different_models() {
    log_info "Test 3: Concurrent requests with different models"

    # Get available models
    local models_response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local model1="helixagent-debate"
    local model2=$(echo "$models_response" | jq -r '.data[1].id // "helixagent-debate"' 2>/dev/null || echo "helixagent-debate")

    # Start concurrent requests with different models
    local request1="{\"model\": \"$model1\", \"messages\": [{\"role\": \"user\", \"content\": \"Test 1\"}], \"max_tokens\": 10}"
    local request2="{\"model\": \"$model2\", \"messages\": [{\"role\": \"user\", \"content\": \"Test 2\"}], \"max_tokens\": 10}"

    curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 30 > "$OUTPUT_DIR/logs/concurrent_model_1.log" 2>&1 &
    local pid1=$!

    curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" --max-time 30 > "$OUTPUT_DIR/logs/concurrent_model_2.log" 2>&1 &
    local pid2=$!

    wait $pid1
    wait $pid2

    local http_code1=$(tail -n1 "$OUTPUT_DIR/logs/concurrent_model_1.log")
    local http_code2=$(tail -n1 "$OUTPUT_DIR/logs/concurrent_model_2.log")

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "diff_models" "both_success" "true" "Different models handled concurrently"
    else
        record_assertion "diff_models" "both_success" "false" "One failed ($http_code1, $http_code2)"
    fi
}

# Test 4: Concurrent streaming requests
test_concurrent_streaming() {
    log_info "Test 4: Concurrent streaming requests"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Count to 3"}],
        "max_tokens": 20,
        "stream": true
    }'

    # Start two streaming requests
    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/stream_concurrent_1.log" 2>&1 &
    local pid1=$!

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/stream_concurrent_2.log" 2>&1 &
    local pid2=$!

    wait $pid1
    wait $pid2

    # Check if both received data
    local has_data1=$(grep -c "data:" "$OUTPUT_DIR/logs/stream_concurrent_1.log" || echo "0")
    local has_data2=$(grep -c "data:" "$OUTPUT_DIR/logs/stream_concurrent_2.log" || echo "0")

    if [[ $has_data1 -gt 0 ]] && [[ $has_data2 -gt 0 ]]; then
        record_assertion "stream_concurrent" "both_streamed" "true" "Both streaming requests succeeded"
    else
        record_assertion "stream_concurrent" "both_streamed" "false" "Stream counts: $has_data1, $has_data2"
    fi
}

# Test 5: Response isolation (concurrent requests don't mix)
test_response_isolation() {
    log_info "Test 5: Response isolation between concurrent requests"

    # Send different questions concurrently
    local request1='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Say: FIRST"}],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    local request2='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Say: SECOND"}],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 30 > "$OUTPUT_DIR/logs/isolation_1.log" 2>&1 &
    local pid1=$!

    curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" --max-time 30 > "$OUTPUT_DIR/logs/isolation_2.log" 2>&1 &
    local pid2=$!

    wait $pid1
    wait $pid2

    local http_code1=$(tail -n1 "$OUTPUT_DIR/logs/isolation_1.log")
    local http_code2=$(tail -n1 "$OUTPUT_DIR/logs/isolation_2.log")

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "isolation" "both_success" "true" "Both isolated requests succeeded"

        # Check response isolation (no cross-contamination)
        local body1=$(head -n -1 "$OUTPUT_DIR/logs/isolation_1.log")
        local body2=$(head -n -1 "$OUTPUT_DIR/logs/isolation_2.log")

        local content1=$(echo "$body1" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local content2=$(echo "$body2" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")

        # Responses should be different and match their prompts
        if echo "$content1" | grep -qi "first" && echo "$content2" | grep -qi "second"; then
            record_assertion "isolation" "no_mixing" "true" "Responses properly isolated"
        elif echo "$content1" | grep -qi "second" || echo "$content2" | grep -qi "first"; then
            record_assertion "isolation" "no_mixing" "false" "Possible response mixing"
        else
            record_assertion "isolation" "no_mixing" "false" "Cannot verify isolation"
        fi
    else
        record_assertion "isolation" "both_success" "false" "One failed ($http_code1, $http_code2)"
    fi
}

# Test 6: Latency under concurrent load
test_concurrent_latency() {
    log_info "Test 6: Latency under concurrent load"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Quick"}],
        "max_tokens": 5,
        "temperature": 0.1
    }'

    local start_time=$(date +%s%N)

    # Launch 3 concurrent requests
    for i in {1..3}; do
        curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 > "$OUTPUT_DIR/logs/latency_concurrent_${i}.log" 2>&1 &
    done

    # Wait for all
    wait

    local end_time=$(date +%s%N)
    local total_latency=$(( (end_time - start_time) / 1000000 ))

    # Count successes
    local success_count=0
    for i in {1..3}; do
        local http_code=$(tail -n1 "$OUTPUT_DIR/logs/latency_concurrent_${i}.log")
        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        fi
    done

    if [[ $success_count -eq 3 ]]; then
        record_assertion "latency" "all_completed" "true" "All completed in ${total_latency}ms"
    else
        record_assertion "latency" "all_completed" "false" "Only $success_count/3 completed"
    fi

    record_metric "concurrent_total_latency_ms" "$total_latency"
}

# Main execution
main() {
    log_info "Starting Chat Concurrent Requests Challenge..."

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
    test_two_concurrent
    test_five_concurrent
    test_concurrent_different_models
    test_concurrent_streaming
    test_response_isolation
    test_concurrent_latency

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
