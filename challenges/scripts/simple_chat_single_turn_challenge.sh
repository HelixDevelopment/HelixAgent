#!/bin/bash
# Simple Chat Single Turn Challenge
# Tests basic single-turn chat conversation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "simple_chat_single_turn" "Simple Chat Single Turn Challenge"
load_env

log_info "Testing simple single-turn chat conversation..."

# Test 1: Basic single-turn chat
test_single_turn_chat() {
    log_info "Test 1: Basic single-turn chat request"
    
    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is 2+2? Answer with just the number."}
        ],
        "max_tokens": 10,
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
        record_assertion "chat" "http_status" "true" "HTTP status is 200"
        
        # Check response contains content
        if echo "$body" | grep -q '"content"'; then
            record_assertion "chat" "has_content" "true" "Response has content field"
            
            # Check if answer is correct (contains "4")
            if echo "$body" | grep -qi "4"; then
                record_assertion "chat" "correct_answer" "true" "Answer is correct (4)"
            else
                record_assertion "chat" "correct_answer" "false" "Answer may be incorrect"
            fi
        else
            record_assertion "chat" "has_content" "false" "Response missing content"
        fi
    else
        record_assertion "chat" "http_status" "false" "HTTP status is $http_code (expected 200)"
    fi
    
    record_metric "chat_latency_ms" "$latency"
    record_metric "http_status_code" "$http_code"
}

# Test 2: Single-turn with system message
test_single_turn_with_system() {
    log_info "Test 2: Single-turn chat with system message"
    
    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant. Be concise."},
            {"role": "user", "content": "What is the capital of France?"}
        ],
        "max_tokens": 20,
        "temperature": 0.1
    }'
    
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)
    
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)
    
    if [[ "$http_code" == "200" ]]; then
        record_assertion "system" "http_status" "true" "System message request succeeded"
        
        # Check if answer mentions Paris
        if echo "$body" | grep -qi "paris"; then
            record_assertion "system" "correct_answer" "true" "Answer is correct (Paris)"
        else
            record_assertion "system" "correct_answer" "false" "Expected Paris in answer"
        fi
    else
        record_assertion "system" "http_status" "false" "Request failed with $http_code"
    fi
}

# Test 3: Response format validation
test_response_format() {
    log_info "Test 3: Response format validation"
    
    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hi"}
        ],
        "max_tokens": 50
    }'
    
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)
    
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)
    
    if [[ "$http_code" == "200" ]]; then
        # Validate response structure (OpenAI format)
        local has_id=$(echo "$body" | jq -e '.id' > /dev/null 2>&1 && echo "true" || echo "false")
        local has_object=$(echo "$body" | jq -e '.object' > /dev/null 2>&1 && echo "true" || echo "false")
        local has_created=$(echo "$body" | jq -e '.created' > /dev/null 2>&1 && echo "true" || echo "false")
        local has_model=$(echo "$body" | jq -e '.model' > /dev/null 2>&1 && echo "true" || echo "false")
        local has_choices=$(echo "$body" | jq -e '.choices' > /dev/null 2>&1 && echo "true" || echo "false")
        
        record_assertion "format" "has_id" "$has_id" "Response has id field"
        record_assertion "format" "has_object" "$has_object" "Response has object field"
        record_assertion "format" "has_created" "$has_created" "Response has created field"
        record_assertion "format" "has_model" "$has_model" "Response has model field"
        record_assertion "format" "has_choices" "$has_choices" "Response has choices array"
        
        # Check choices structure
        if [[ "$has_choices" == "true" ]]; then
            local has_message=$(echo "$body" | jq -e '.choices[0].message' > /dev/null 2>&1 && echo "true" || echo "false")
            local has_role=$(echo "$body" | jq -e '.choices[0].message.role' > /dev/null 2>&1 && echo "true" || echo "false")
            local has_content=$(echo "$body" | jq -e '.choices[0].message.content' > /dev/null 2>&1 && echo "true" || echo "false")
            
            record_assertion "format" "has_message" "$has_message" "Choice has message"
            record_assertion "format" "has_role" "$has_role" "Message has role"
            record_assertion "format" "has_content" "$has_content" "Message has content"
        fi
    else
        record_assertion "format" "http_status" "false" "Format test failed with $http_code"
    fi
}

# Main execution
main() {
    log_info "Starting Simple Chat Single Turn Challenge..."

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
    test_single_turn_chat
    test_single_turn_with_system
    test_response_format

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
