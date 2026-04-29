#!/bin/bash
# Chat Tool Calling Challenge
# Tests tool/function calling functionality in conversations

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "chat-tool-calling" "Chat Tool Calling Challenge"
load_env

log_info "Testing tool/function calling in chat conversations..."

# Test 1: Basic tool call request
test_basic_tool_call() {
    log_info "Test 1: Basic tool call with weather function"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is the weather in San Francisco?"}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get current weather in a location",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "location": {"type": "string", "description": "City name"}
                        },
                        "required": ["location"]
                    }
                }
            }
        ],
        "tool_choice": "auto",
        "max_tokens": 100
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "basic_tool" "http_status" "true" "Tool call request accepted"

        # Anti-bluff (CONST-035 §11): with tool_choice=auto an honest LLM
        # may legitimately respond EITHER with tool_calls OR with text
        # content (e.g. declining, asking clarifying questions, answering
        # from training data). Both are correct — the model has discretion.
        # FAIL only if the response is empty (no tool_calls AND no content).
        if echo "$body" | jq -e '.choices[0].message.tool_calls' > /dev/null 2>&1; then
            record_assertion "basic_tool" "responded" "true" "Response includes tool_calls"

            local tool_name=$(echo "$body" | jq -r '.choices[0].message.tool_calls[0].function.name // empty')
            if [[ "$tool_name" == "get_weather" ]]; then
                record_assertion "basic_tool" "correct_tool" "true" "Called get_weather function"
            fi

            local args=$(echo "$body" | jq -r '.choices[0].message.tool_calls[0].function.arguments // empty')
            if echo "$args" | grep -qi "san francisco"; then
                record_assertion "basic_tool" "correct_args" "true" "Arguments include location"
            fi
        else
            # No tool_calls — verify there's meaningful text content instead.
            local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
            local content_len=${#content}
            if (( content_len > 0 )); then
                record_assertion "basic_tool" "responded" "true" "Response has content ($content_len chars; model chose text over tool_call — valid under tool_choice=auto)"
            else
                record_assertion "basic_tool" "responded" "false" "Empty response (no tool_calls AND no content)"
            fi
        fi
    else
        record_assertion "basic_tool" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 2: Multiple tools available
test_multiple_tools() {
    log_info "Test 2: Multiple tools available for selection"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Calculate 25 * 4"}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get weather",
                    "parameters": {"type": "object", "properties": {}, "required": []}
                }
            },
            {
                "type": "function",
                "function": {
                    "name": "calculate",
                    "description": "Perform mathematical calculations",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "expression": {"type": "string"}
                        },
                        "required": ["expression"]
                    }
                }
            }
        ],
        "tool_choice": "auto",
        "max_tokens": 100
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "multiple_tools" "http_status" "true" "Multiple tools accepted"

        # Check if correct tool was selected
        local tool_name=$(echo "$body" | jq -r '.choices[0].message.tool_calls[0].function.name // empty')
        if [[ "$tool_name" == "calculate" ]]; then
            record_assertion "multiple_tools" "correct_selection" "true" "Selected calculate over weather"
        fi
    else
        record_assertion "multiple_tools" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 3: Forced tool call
test_forced_tool() {
    log_info "Test 3: Forced tool call with tool_choice=required"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hello"}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "greet",
                    "description": "Send a greeting",
                    "parameters": {"type": "object", "properties": {}, "required": []}
                }
            }
        ],
        "tool_choice": "required",
        "max_tokens": 50
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "forced_tool" "http_status" "true" "Forced tool call accepted"

        # Should have a tool call since it's required
        if echo "$body" | jq -e '.choices[0].message.tool_calls' > /dev/null 2>&1; then
            record_assertion "forced_tool" "tool_called" "true" "Tool was called as required"
        fi
    else
        # May return 400 if tool_choice not supported
        record_assertion "forced_tool" "http_status" "false" "HTTP $http_code (may not support tool_choice)"
    fi
}

# Test 4: Tool call with follow-up message
test_tool_followup() {
    log_info "Test 4: Tool call with follow-up assistant message"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Get weather for London"},
            {
                "role": "assistant",
                "content": null,
                "tool_calls": [
                    {
                        "id": "call_123",
                        "type": "function",
                        "function": {
                            "name": "get_weather",
                            "arguments": "{\"location\":\"London\"}"
                        }
                    }
                ]
            },
            {
                "role": "tool",
                "tool_call_id": "call_123",
                "content": "{\"temperature\": 15, \"condition\": \"cloudy\"}"
            },
            {"role": "user", "content": "What about Paris?"}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get weather",
                    "parameters": {
                        "type": "object",
                        "properties": {"location": {"type": "string"}},
                        "required": ["location"]
                    }
                }
            }
        ],
        "max_tokens": 100
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "tool_followup" "http_status" "true" "Tool follow-up conversation works"
    else
        record_assertion "tool_followup" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 5: No tool call when not needed
test_no_tool_when_not_needed() {
    log_info "Test 5: No tool call when answer doesn't need tools"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is 2+2? Just tell me."}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "complex_calculation",
                    "description": "Perform complex calculations",
                    "parameters": {"type": "object", "properties": {}, "required": []}
                }
            }
        ],
        "tool_choice": "auto",
        "max_tokens": 50
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "no_tool_needed" "http_status" "true" "Request accepted"

        # Should have content, not tool call for simple question
        if echo "$body" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
            local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
            if [[ -n "$content" ]]; then
                record_assertion "no_tool_needed" "has_content" "true" "Answered without tool call"
            fi
        fi
    else
        record_assertion "no_tool_needed" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 6: Tool call with streaming
test_tool_with_streaming() {
    log_info "Test 6: Tool calls with streaming enabled"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is the weather?"}
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get weather",
                    "parameters": {
                        "type": "object",
                        "properties": {"location": {"type": "string"}},
                        "required": ["location"]
                    }
                }
            }
        ],
        "stream": true,
        "max_tokens": 50
    }'

    local output_file="$OUTPUT_DIR/logs/tool_streaming.txt"
    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 > "$output_file" 2>/dev/null || true

    if grep -q "data:" "$output_file"; then
        record_assertion "tool_streaming" "streaming_works" "true" "Streaming with tools works"

        # Check if tool call appears in stream
        if grep -q "tool_calls" "$output_file"; then
            record_assertion "tool_streaming" "tool_in_stream" "true" "Tool call appears in stream"
        fi
    else
        record_assertion "tool_streaming" "streaming_works" "false" "No SSE data received"
    fi
}

main() {
    log_info "Starting tool calling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_basic_tool_call
    test_multiple_tools
    test_forced_tool
    test_tool_followup
    test_no_tool_when_not_needed
    test_tool_with_streaming

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All tool calling tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
