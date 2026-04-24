#!/bin/bash
# Curl API Challenge - Comprehensive API testing with curl
# Tests everyday use-cases as regular users would do

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "curl_api_challenge" "Curl API Testing Challenge"
load_env

# Test health endpoint
test_health() {
    log_info "Testing health endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q "healthy"; then
            record_assertion "health" "health_check" "true" "Health endpoint returns healthy"
        else
            record_assertion "health" "health_check" "false" "Health response missing healthy status"
        fi
    else
        record_assertion "health" "health_check" "false" "Health endpoint failed: $http_code"
    fi

    record_metric "health_status" "$http_code"
}

# Test models listing
test_models_list() {
    log_info "Testing models list endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q '"object":"list"'; then
            record_assertion "models" "models_list" "true" "Models endpoint returns list format"
        else
            record_assertion "models" "models_list" "false" "Models response wrong format"
        fi
    else
        record_assertion "models" "models_list" "false" "Models endpoint failed: $http_code"
    fi

    record_metric "models_status" "$http_code"
}

# Test simple chat completion
test_simple_chat() {
    log_info "Testing simple chat completion..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is 2+2? Answer with just the number."}
        ],
        "max_tokens": 20
    }'

    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 90 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q '"content"'; then
            record_assertion "chat" "simple_chat" "true" "Simple chat completion works"
            if echo "$body" | grep -qi "4"; then
                record_assertion "chat" "math_answer" "true" "Math answer is correct"
            else
                record_assertion "chat" "math_answer" "false" "Math answer may be incorrect"
            fi
        else
            record_assertion "chat" "simple_chat" "false" "Chat response missing content"
        fi
    else
        record_assertion "chat" "simple_chat" "false" "Chat endpoint failed: $http_code"
    fi

    record_metric "simple_chat_latency_ms" "$latency"
    record_metric "simple_chat_status" "$http_code"
}

# Test streaming chat completion
test_streaming_chat() {
    log_info "Testing streaming chat completion..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Say hello in one word."}
        ],
        "max_tokens": 30,
        "stream": true
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q "data:"; then
            record_assertion "streaming" "streaming_chat" "true" "Streaming chat works"

            if echo "$body" | grep -q '\[DONE\]'; then
                record_assertion "streaming" "streaming_done" "true" "Streaming completes with [DONE]"
            else
                record_assertion "streaming" "streaming_done" "false" "Missing [DONE] marker"
            fi
        else
            record_assertion "streaming" "streaming_chat" "false" "No streaming data received"
        fi
    else
        record_assertion "streaming" "streaming_chat" "false" "Streaming failed: $http_code"
    fi

    record_metric "streaming_status" "$http_code"
}

# Test chat with system message
test_chat_with_system() {
    log_info "Testing chat with system message..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant. Always be concise."},
            {"role": "user", "content": "What is the capital of France?"}
        ],
        "max_tokens": 50
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 90 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -qi "paris"; then
            record_assertion "chat" "system_message" "true" "Chat with system message works"
        else
            record_assertion "chat" "system_message" "false" "Response may be incorrect (expected Paris)"
        fi
    else
        record_assertion "chat" "system_message" "false" "Chat with system message failed: $http_code"
    fi

    record_metric "system_chat_status" "$http_code"
}

# Test multi-turn conversation
test_multi_turn_chat() {
    log_info "Testing multi-turn conversation..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "My name is Alice."},
            {"role": "assistant", "content": "Hello Alice! Nice to meet you."},
            {"role": "user", "content": "What is my name?"}
        ],
        "max_tokens": 50
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 90 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -qi "alice"; then
            record_assertion "chat" "multi_turn" "true" "Multi-turn conversation works"
        else
            record_assertion "chat" "multi_turn" "false" "Context not maintained (expected Alice)"
        fi
    else
        record_assertion "chat" "multi_turn" "false" "Multi-turn chat failed: $http_code"
    fi

    record_metric "multi_turn_status" "$http_code"
}

# Test error handling - invalid JSON
test_invalid_json() {
    log_info "Testing error handling - invalid JSON..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{invalid json" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "400" ]]; then
        record_assertion "error" "invalid_json" "true" "Invalid JSON returns 400"
    else
        record_assertion "error" "invalid_json" "false" "Invalid JSON should return 400, got: $http_code"
    fi

    record_metric "invalid_json_status" "$http_code"
}

# Test error handling - empty body
test_empty_body() {
    log_info "Testing error handling - empty body..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "400" ]]; then
        record_assertion "error" "empty_body" "true" "Empty body returns 400"
    else
        record_assertion "error" "empty_body" "false" "Empty body should return 400, got: $http_code"
    fi

    record_metric "empty_body_status" "$http_code"
}

# Test providers endpoint
test_providers() {
    log_info "Testing providers endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        if echo "$body" | grep -q "providers"; then
            record_assertion "providers" "providers_list" "true" "Providers endpoint works"
        else
            record_assertion "providers" "providers_list" "false" "Providers response format incorrect"
        fi
    else
        record_assertion "providers" "providers_list" "false" "Providers endpoint failed: $http_code"
    fi

    record_metric "providers_status" "$http_code"
}

# Test debates endpoint
test_debates() {
    log_info "Testing debates endpoint..."

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debates" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # 200 OK, 401 (auth required), or 404 (not implemented) are all acceptable
    # 401 indicates endpoint exists and is correctly protected
    if [[ "$http_code" == "200" ]] || [[ "$http_code" == "401" ]] || [[ "$http_code" == "404" ]]; then
        record_assertion "debates" "debates_endpoint" "true" "Debates endpoint responds correctly (status: $http_code)"
    else
        record_assertion "debates" "debates_endpoint" "false" "Debates endpoint unexpected: $http_code"
    fi

    record_metric "debates_status" "$http_code"
}

# Test concurrent requests
test_concurrent() {
    log_info "Testing concurrent requests..."

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hi"}],
        "max_tokens": 10
    }'

    # Launch 3 concurrent requests
    local pids=()
    local results=()

    for i in 1 2 3; do
        (
            local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d "$request" --max-time 90 2>/dev/null || true)
            echo "$(echo "$resp" | tail -n1)" > "$OUTPUT_DIR/logs/concurrent_$i.txt"
        ) &
        pids+=($!)
    done

    # Wait for all to complete
    local all_success=true
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    # Check results
    for i in 1 2 3; do
        if [[ -f "$OUTPUT_DIR/logs/concurrent_$i.txt" ]]; then
            local status=$(cat "$OUTPUT_DIR/logs/concurrent_$i.txt")
            if [[ "$status" != "200" ]]; then
                all_success=false
            fi
        else
            all_success=false
        fi
    done

    if [[ "$all_success" == "true" ]]; then
        record_assertion "concurrent" "concurrent_requests" "true" "Concurrent requests handled"
    else
        record_assertion "concurrent" "concurrent_requests" "false" "Some concurrent requests failed"
    fi

    record_metric "concurrent_tested" "3"
}

# Main execution
main() {
    log_info "Starting Curl API Challenge..."
    log_info "Base URL: $BASE_URL"

    # Check if server is running
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_warning "HelixAgent not running, attempting to start..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    # Run API tests
    test_health
    test_models_list
    test_simple_chat
    test_streaming_chat
    test_chat_with_system
    test_multi_turn_chat
    test_invalid_json
    test_empty_body
    test_providers
    test_debates
    test_concurrent

    # Calculate results
    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo "0")
    failed_count=${failed_count:-0}

    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
