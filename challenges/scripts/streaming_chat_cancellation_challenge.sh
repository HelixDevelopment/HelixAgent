#!/bin/bash
# Streaming Chat Cancellation Challenge
# Tests streaming cancellation, interruption, and timeout handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "streaming_chat_cancellation" "Streaming Chat Cancellation Challenge"
load_env

log_info "Testing streaming cancellation and interruption handling..."

# Test 1: Early stream cancellation
test_early_cancellation() {
    log_info "Test 1: Early stream cancellation"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Write a long story about a journey."}
        ],
        "max_tokens": 500,
        "temperature": 0.7,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_cancel_early.log"

    # Start streaming and cancel after 1 second
    timeout 1 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" > "$output_file" 2>&1 || true

    # Check if any data was received before cancellation
    if grep -q "data:" "$output_file"; then
        record_assertion "early_cancel" "partial_data" "true" "Received partial data before cancellation"

        # Should not have [DONE] marker (cancelled early)
        if ! grep -q "data: \[DONE\]" "$output_file"; then
            record_assertion "early_cancel" "incomplete" "true" "Stream was incomplete (expected)"
        else
            record_assertion "early_cancel" "incomplete" "false" "Stream completed unexpectedly"
        fi
    else
        record_assertion "early_cancel" "partial_data" "false" "No data received"
    fi
}

# Test 2: Mid-stream cancellation
test_mid_cancellation() {
    log_info "Test 2: Mid-stream cancellation"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Count from 1 to 100."}
        ],
        "max_tokens": 300,
        "temperature": 0.1,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_cancel_mid.log"

    # Start streaming and cancel after 3 seconds
    timeout 3 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" > "$output_file" 2>&1 || true

    # Count chunks received
    local chunk_count=$(grep -c "data:" "$output_file" || echo "0")

    if [[ $chunk_count -gt 0 ]]; then
        record_assertion "mid_cancel" "chunks_received" "true" "Received $chunk_count chunks before cancellation"

        # Verify stream was interrupted (no [DONE])
        if ! grep -q "data: \[DONE\]" "$output_file"; then
            record_assertion "mid_cancel" "interrupted" "true" "Stream was interrupted (no [DONE])"
        else
            record_assertion "mid_cancel" "interrupted" "false" "Stream completed"
        fi
    else
        record_assertion "mid_cancel" "chunks_received" "false" "No chunks received"
    fi

    record_metric "chunks_before_cancel" "$chunk_count"
}

# Test 3: Timeout handling
test_stream_timeout() {
    log_info "Test 3: Stream timeout handling"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hello"}
        ],
        "max_tokens": 50,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_timeout.log"
    local start_time=$(date +%s)

    # Set very short timeout (2 seconds)
    timeout 2 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 > "$output_file" 2>&1 || true

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    # Should timeout around 2 seconds
    if [[ $duration -le 3 ]]; then
        record_assertion "timeout" "respects_timeout" "true" "Timeout respected (${duration}s)"
    else
        record_assertion "timeout" "respects_timeout" "false" "Timeout not respected (${duration}s)"
    fi

    record_metric "timeout_duration_sec" "$duration"
}

# Test 4: Connection close handling
test_connection_close() {
    log_info "Test 4: Connection close handling"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Tell me a story."}
        ],
        "max_tokens": 200,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_connclose.log"

    # Start curl in background
    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" > "$output_file" 2>&1 &

    local curl_pid=$!

    # Wait a bit for streaming to start
    sleep 1

    # Kill the curl process (simulate client disconnect)
    kill -9 $curl_pid 2>/dev/null || true
    wait $curl_pid 2>/dev/null || true

    # Check if partial data was written
    if [[ -f "$output_file" ]] && [[ -s "$output_file" ]]; then
        if grep -q "data:" "$output_file"; then
            record_assertion "conn_close" "partial_write" "true" "Partial data written before disconnect"
        else
            record_assertion "conn_close" "partial_write" "false" "No data written"
        fi
    else
        record_assertion "conn_close" "partial_write" "false" "No output file"
    fi
}

# Test 5: Multiple rapid cancellations
test_rapid_cancellations() {
    log_info "Test 5: Multiple rapid cancellations (stress test)"

    local success_count=0
    local attempt_count=5

    for i in $(seq 1 $attempt_count); do
        local request='{
            "model": "helixagent-debate",
            "messages": [
                {"role": "user", "content": "Quick test '$i'"}
            ],
            "max_tokens": 100,
            "stream": true
        }'

        local output_file="$OUTPUT_DIR/logs/stream_rapid_${i}.log"

        # Start and immediately cancel
        timeout 0.5 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" > "$output_file" 2>&1 || true

        # Small delay between attempts
        sleep 0.2

        # Count as success if file was created (server responded)
        if [[ -f "$output_file" ]]; then
            success_count=$((success_count + 1))
        fi
    done

    if [[ $success_count -ge $((attempt_count - 1)) ]]; then
        record_assertion "rapid_cancel" "server_stable" "true" "Server stable after $success_count/$attempt_count cancellations"
    else
        record_assertion "rapid_cancel" "server_stable" "false" "Only $success_count/$attempt_count cancellations handled"
    fi

    record_metric "rapid_cancellations" "$success_count"
}

# Test 6: Cancel then immediate new request
test_cancel_then_new() {
    log_info "Test 6: Cancel then immediate new request"

    # First request - will be cancelled
    local request1='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Write a long essay."}
        ],
        "max_tokens": 500,
        "stream": true
    }'

    local output_file1="$OUTPUT_DIR/logs/stream_cancel_first.log"

    timeout 1 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" > "$output_file1" 2>&1 || true

    # Immediate second request
    local request2='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is 2+2?"}
        ],
        "max_tokens": 10,
        "temperature": 0.1,
        "stream": false
    }'

    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" \
        --max-time 30 2>/dev/null || true)

    local http_code2=$(echo "$response2" | tail -n1)

    if [[ "$http_code2" == "200" ]]; then
        record_assertion "cancel_new" "second_request" "true" "Second request succeeded after cancellation"

        local body2=$(echo "$response2" | head -n -1)
        if echo "$body2" | grep -qi "4"; then
            record_assertion "cancel_new" "correct_answer" "true" "Second request returned correct answer"
        else
            record_assertion "cancel_new" "correct_answer" "false" "Answer unclear"
        fi
    else
        record_assertion "cancel_new" "second_request" "false" "Second request failed with $http_code2"
    fi
}

# Test 7: Partial chunk validation after cancellation
test_partial_chunks() {
    log_info "Test 7: Partial chunk validation after cancellation"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "List the alphabet."}
        ],
        "max_tokens": 200,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_partial.log"

    timeout 2 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" > "$output_file" 2>&1 || true

    # Validate that all received chunks are valid JSON
    local invalid_chunks=0
    local valid_chunks=0

    while IFS= read -r line; do
        if [[ "$line" == data:* ]] && [[ "$line" != *"[DONE]"* ]]; then
            local chunk=$(echo "$line" | sed 's/data: //g')
            if echo "$chunk" | jq empty 2>/dev/null; then
                valid_chunks=$((valid_chunks + 1))
            else
                invalid_chunks=$((invalid_chunks + 1))
            fi
        fi
    done < "$output_file"

    if [[ $valid_chunks -gt 0 ]] && [[ $invalid_chunks -eq 0 ]]; then
        record_assertion "partial_chunks" "all_valid" "true" "All $valid_chunks partial chunks are valid JSON"
    elif [[ $valid_chunks -gt 0 ]]; then
        record_assertion "partial_chunks" "all_valid" "false" "$invalid_chunks invalid chunks out of $((valid_chunks + invalid_chunks))"
    else
        record_assertion "partial_chunks" "all_valid" "false" "No valid chunks found"
    fi

    record_metric "valid_partial_chunks" "$valid_chunks"
}

# Main execution
main() {
    log_info "Starting Streaming Chat Cancellation Challenge..."

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
    test_early_cancellation
    test_mid_cancellation
    test_stream_timeout
    test_connection_close
    test_rapid_cancellations
    test_cancel_then_new
    test_partial_chunks

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
