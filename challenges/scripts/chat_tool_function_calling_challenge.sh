#!/bin/bash
# Chat Tool/Function Calling Challenge
# Tests tool/function calling capabilities (if supported)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_tool_function_calling" "Chat Tool/Function Calling Challenge"
load_env

log_info "Testing tool/function calling capabilities..."

# Test 1: Request with tools parameter
test_basic_tools_request() {
    log_info "Test 1: Basic request with tools parameter"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is the weather in London?"}],
        "max_tokens": 50,
        "tools": [{
            "type": "function",
            "function": {
                "name": "get_weather",
                "description": "Get weather for a location",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "location": {"type": "string"}
                    },
                    "required": ["location"]
                }
            }
        }]
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "tools_basic" "accepted" "true" "Tools parameter accepted"

        # Check if response includes tool_calls or regular message
        local has_tool_calls=$(echo "$body" | jq -e '.choices[0].message.tool_calls' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_tool_calls" == "true" ]]; then
            record_assertion "tools_basic" "has_tool_calls" "true" "Response includes tool_calls"
        else
            record_assertion "tools_basic" "has_tool_calls" "false" "No tool_calls (feature may not be supported)"
        fi
    elif [[ "$http_code" == "400" ]]; then
        record_assertion "tools_basic" "accepted" "false" "Tools not supported (400)"
    else
        record_assertion "tools_basic" "accepted" "false" "Unexpected status: $http_code"
    fi
}

# Test 2: Multiple tools
test_multiple_tools() {
    log_info "Test 2: Request with multiple tools"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Get weather and stock price"}],
        "max_tokens": 50,
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get weather",
                    "parameters": {"type": "object", "properties": {}}
                }
            },
            {
                "type": "function",
                "function": {
                    "name": "get_stock_price",
                    "description": "Get stock price",
                    "parameters": {"type": "object", "properties": {}}
                }
            }
        ]
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "multi_tools" "accepted" "true" "Multiple tools accepted"
    elif [[ "$http_code" == "400" ]]; then
        record_assertion "multi_tools" "accepted" "false" "Multiple tools not supported"
    else
        record_assertion "multi_tools" "accepted" "false" "Status: $http_code"
    fi
}

# Test 3: Tool choice parameter
test_tool_choice() {
    log_info "Test 3: tool_choice parameter"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Calculate something"}],
        "max_tokens": 50,
        "tools": [{
            "type": "function",
            "function": {
                "name": "calculator",
                "description": "Calculate math",
                "parameters": {"type": "object", "properties": {}}
            }
        }],
        "tool_choice": "auto"
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "tool_choice" "works" "true" "tool_choice parameter works"
    else
        record_assertion "tool_choice" "works" "false" "Status: $http_code"
    fi
}

# Test 4: Tool result in conversation
test_tool_result_conversation() {
    log_info "Test 4: Conversation with tool result"

    # Simulate a conversation with tool call result
    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is the weather?"},
            {"role": "assistant", "content": null, "tool_calls": [{
                "id": "call_1",
                "type": "function",
                "function": {"name": "get_weather", "arguments": "{\"location\":\"London\"}"}
            }]},
            {"role": "tool", "tool_call_id": "call_1", "content": "{\"temperature\": 15}"}
        ],
        "max_tokens": 50
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "tool_result" "accepted" "true" "Tool result in conversation accepted"
    elif [[ "$http_code" == "400" ]]; then
        record_assertion "tool_result" "accepted" "false" "Tool message format not supported"
    else
        record_assertion "tool_result" "accepted" "false" "Status: $http_code"
    fi
}

# Test 5: Tools with streaming
test_tools_with_streaming() {
    log_info "Test 5: Tools with streaming enabled"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Check weather"}],
        "max_tokens": 50,
        "stream": true,
        "tools": [{
            "type": "function",
            "function": {
                "name": "get_weather",
                "description": "Get weather",
                "parameters": {"type": "object", "properties": {}}
            }
        }]
    }'

    local output_file="$OUTPUT_DIR/logs/tools_stream.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 30 > "$output_file" 2>&1 || true

    if grep -q "data:" "$output_file"; then
        record_assertion "tools_stream" "works" "true" "Tools with streaming works"
    else
        record_assertion "tools_stream" "works" "false" "No streaming data"
    fi
}

# Test 6: Invalid tool schema
test_invalid_tool_schema() {
    log_info "Test 6: Invalid tool schema handling"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10,
        "tools": [{
            "type": "function",
            "function": {
                "name": "invalid_tool"
            }
        }]
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "400" ]]; then
        record_assertion "invalid_tool" "rejected" "true" "Invalid tool schema rejected"
    elif [[ "$http_code" == "200" ]]; then
        record_assertion "invalid_tool" "rejected" "false" "Invalid tool accepted (may use defaults)"
    else
        record_assertion "invalid_tool" "rejected" "false" "Unexpected status: $http_code"
    fi
}

# Test 7: No tools provided (baseline)
test_no_tools() {
    log_info "Test 7: Request without tools (baseline)"

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
        record_assertion "no_tools" "works" "true" "Baseline request without tools works"
    else
        record_assertion "no_tools" "works" "false" "Baseline failed: $http_code"
    fi
}

# Main execution
main() {
    log_info "Starting Chat Tool/Function Calling Challenge..."

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
    test_basic_tools_request
    test_multiple_tools
    test_tool_choice
    test_tool_result_conversation
    test_tools_with_streaming
    test_invalid_tool_schema
    test_no_tools

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
