#!/bin/bash
# Streaming Chat Basic Challenge
# Tests basic streaming chat functionality with SSE

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "streaming_chat_basic" "Streaming Chat Basic Challenge"
load_env

log_info "Testing basic streaming chat functionality..."

# Test 1: Basic streaming response
test_basic_streaming() {
    log_info "Test 1: Basic streaming response"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Count from 1 to 5."}
        ],
        "max_tokens": 50,
        "temperature": 0.1,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_basic.log"
    local start_time=$(date +%s%N)

    # Capture streaming response
    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 60 > "$output_file" 2>&1 || true

    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    # Check if output contains SSE data
    if grep -q "data:" "$output_file"; then
        record_assertion "basic_stream" "sse_format" "true" "Response contains SSE data"

        # Check for streaming chunks
        local chunk_count=$(grep -c "data:" "$output_file" || echo "0")
        if [[ $chunk_count -gt 1 ]]; then
            record_assertion "basic_stream" "multiple_chunks" "true" "Received $chunk_count chunks"
        else
            record_assertion "basic_stream" "multiple_chunks" "false" "Only received $chunk_count chunk(s)"
        fi

        # Check for [DONE] marker
        if grep -q "data: \[DONE\]" "$output_file"; then
            record_assertion "basic_stream" "done_marker" "true" "Stream properly closed with [DONE]"
        else
            record_assertion "basic_stream" "done_marker" "false" "Missing [DONE] marker"
        fi
    else
        record_assertion "basic_stream" "sse_format" "false" "No SSE data found"
    fi

    record_metric "streaming_latency_ms" "$latency"
    record_metric "chunk_count" "$chunk_count"
}

# Test 2: Streaming with system message
test_streaming_with_system() {
    log_info "Test 2: Streaming with system message"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant. Be concise."},
            {"role": "user", "content": "What is the capital of France?"}
        ],
        "max_tokens": 30,
        "temperature": 0.1,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_system.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 60 > "$output_file" 2>&1 || true

    if grep -q "data:" "$output_file"; then
        record_assertion "system_stream" "streaming_works" "true" "Streaming with system message works"

        # Extract content from all chunks
        local full_content=$(grep "data:" "$output_file" | grep -v "\[DONE\]" | \
            sed 's/data: //g' | \
            jq -r 'select(.choices != null) | .choices[0].delta.content // ""' 2>/dev/null | \
            tr -d '\n')

        # Check if answer mentions Paris
        if echo "$full_content" | grep -qi "paris"; then
            record_assertion "system_stream" "correct_answer" "true" "Answer is correct (Paris)"
        else
            record_assertion "system_stream" "correct_answer" "false" "Expected Paris in answer"
        fi
    else
        record_assertion "system_stream" "streaming_works" "false" "Streaming failed"
    fi
}

# Test 3: Streaming chunk format validation
test_chunk_format() {
    log_info "Test 3: Streaming chunk format validation"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hi"}
        ],
        "max_tokens": 20,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_format.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 60 > "$output_file" 2>&1 || true

    # Validate first data chunk format (should be valid JSON)
    local first_chunk=$(grep "data:" "$output_file" | grep -v "\[DONE\]" | head -n1 | sed 's/data: //g')

    if [[ -n "$first_chunk" ]]; then
        # Try to parse as JSON
        if echo "$first_chunk" | jq empty 2>/dev/null; then
            record_assertion "chunk_format" "valid_json" "true" "Chunk is valid JSON"

            # Check for required fields
            local has_id=$(echo "$first_chunk" | jq -e '.id' >/dev/null 2>&1 && echo "true" || echo "false")
            local has_object=$(echo "$first_chunk" | jq -e '.object' >/dev/null 2>&1 && echo "true" || echo "false")
            local has_choices=$(echo "$first_chunk" | jq -e '.choices' >/dev/null 2>&1 && echo "true" || echo "false")

            if [[ "$has_id" == "true" ]] && [[ "$has_object" == "true" ]] && [[ "$has_choices" == "true" ]]; then
                record_assertion "chunk_format" "required_fields" "true" "All required fields present"
            else
                record_assertion "chunk_format" "required_fields" "false" "Missing required fields"
            fi

            # Check delta structure
            local has_delta=$(echo "$first_chunk" | jq -e '.choices[0].delta' >/dev/null 2>&1 && echo "true" || echo "false")
            record_assertion "chunk_format" "has_delta" "$has_delta" "Delta field present: $has_delta"
        else
            record_assertion "chunk_format" "valid_json" "false" "Chunk is not valid JSON"
        fi
    else
        record_assertion "chunk_format" "valid_json" "false" "No chunks found"
    fi
}

# Test 4: Streaming response reconstruction
test_response_reconstruction() {
    log_info "Test 4: Streaming response reconstruction"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Say: Hello World"}
        ],
        "max_tokens": 20,
        "temperature": 0.1,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_reconstruct.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 60 > "$output_file" 2>&1 || true

    # Reconstruct full message from chunks
    local reconstructed=""
    while IFS= read -r line; do
        if [[ "$line" == data:* ]] && [[ "$line" != *"[DONE]"* ]]; then
            local chunk=$(echo "$line" | sed 's/data: //g')
            local content=$(echo "$chunk" | jq -r '.choices[0].delta.content // ""' 2>/dev/null)
            reconstructed="${reconstructed}${content}"
        fi
    done < "$output_file"

    if [[ -n "$reconstructed" ]]; then
        record_assertion "reconstruct" "has_content" "true" "Reconstructed content: ${reconstructed:0:50}..."

        # Check if reconstructed message makes sense
        if echo "$reconstructed" | grep -qi "hello"; then
            record_assertion "reconstruct" "coherent" "true" "Reconstructed message is coherent"
        else
            record_assertion "reconstruct" "coherent" "false" "Reconstructed message may not be coherent"
        fi
    else
        record_assertion "reconstruct" "has_content" "false" "No content reconstructed"
    fi
}

# Test 5: Streaming vs non-streaming comparison
test_streaming_vs_nonstreaming() {
    log_info "Test 5: Streaming vs non-streaming comparison"

    local request_base='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is 2+2?"}
        ],
        "max_tokens": 10,
        "temperature": 0.1
    }'

    # Non-streaming request
    local request_nostream=$(echo "$request_base" | jq '. + {"stream": false}')
    local response_nostream=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_nostream" \
        --max-time 60 2>/dev/null || true)

    local http_code_nostream=$(echo "$response_nostream" | tail -n1)
    local body_nostream=$(echo "$response_nostream" | head -n -1)

    # Streaming request
    local request_stream=$(echo "$request_base" | jq '. + {"stream": true}')
    local output_file="$OUTPUT_DIR/logs/stream_compare.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_stream" \
        --max-time 60 > "$output_file" 2>&1 || true

    if [[ "$http_code_nostream" == "200" ]] && grep -q "data:" "$output_file"; then
        record_assertion "comparison" "both_modes" "true" "Both streaming and non-streaming work"

        # Compare content similarity (both should answer 4)
        local content_nostream=$(echo "$body_nostream" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local content_stream=$(grep "data:" "$output_file" | grep -v "\[DONE\]" | \
            sed 's/data: //g' | \
            jq -r 'select(.choices != null) | .choices[0].delta.content // ""' 2>/dev/null | \
            tr -d '\n')

        if echo "$content_nostream" | grep -qi "4" && echo "$content_stream" | grep -qi "4"; then
            record_assertion "comparison" "consistent_answer" "true" "Both modes give consistent answer"
        else
            record_assertion "comparison" "consistent_answer" "false" "Answers may differ"
        fi
    else
        record_assertion "comparison" "both_modes" "false" "One mode failed"
    fi
}

# Test 6: Streaming error handling
test_streaming_error() {
    log_info "Test 6: Streaming error handling"

    # Request with invalid model (should error gracefully)
    local request='{
        "model": "nonexistent-model",
        "messages": [
            {"role": "user", "content": "Test"}
        ],
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/stream_error.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 30 > "$output_file" 2>&1 || true

    # Should either return error immediately or stream error
    if grep -qE "(error|invalid|not found)" "$output_file"; then
        record_assertion "stream_error" "error_reported" "true" "Error reported gracefully"
    else
        # Check HTTP status via stderr (if captured)
        record_assertion "stream_error" "error_reported" "false" "Error handling unclear"
    fi
}

# Main execution
main() {
    log_info "Starting Streaming Chat Basic Challenge..."

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
    test_basic_streaming
    test_streaming_with_system
    test_chunk_format
    test_response_reconstruction
    test_streaming_vs_nonstreaming
    test_streaming_error

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
