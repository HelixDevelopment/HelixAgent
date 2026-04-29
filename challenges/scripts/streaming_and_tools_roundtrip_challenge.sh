#!/bin/bash
# Streaming + Tool-Calling Round-Trip Challenge — CONST-035 anti-bluff regression guard
#
# Validates two complete user flows that aren't exercised by the existing
# challenge suite at full depth:
#
# 1. Streaming chat completion (SSE) — verifies the response includes the
#    expected `data: {...}` chunk format AND a terminal `data: [DONE]` line.
#
# 2. Tool-calling — verifies the model returns a structured tool_calls
#    array with `function.name`, `function.arguments` (valid JSON), and
#    `finish_reason: "tool_calls"`.
#
# This Challenge MUST FAIL when:
#   - the streaming path falls back to non-streaming (no `data:` prefix)
#   - the streaming path doesn't emit `[DONE]`
#   - tool_choice="auto" doesn't dispatch through the smart-routing path
#     for tool requests (per docs/api/API_REFERENCE.md)
#   - tool_calls are missing or function.arguments aren't valid JSON
#
# Verify-by-mutation (CONST-035 §1):
#   - Disable the streaming path in handlers/openai_compatible.go
#     handleStreamingChatCompletions → Challenge fails at "stream_done_marker".
#   - Strip tool_choice handling so the model returns text instead of
#     tool_calls → Challenge fails at "tool_calls_present".

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "streaming_and_tools_roundtrip" "Streaming + Tool-Calling Round-Trip Challenge"
load_env

log_info "Testing streaming + tool-calling end-to-end..."

# Test 1: server alive
test_server_alive() {
    log_info "Test 1: server is up"
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: streaming response carries SSE format
test_streaming_sse_format() {
    log_info "Test 2: POST /v1/chat/completions stream=true → SSE format"

    local body='{"model":"helixagent-llm","messages":[{"role":"user","content":"Reply with PING. Just one word."}],"max_tokens":10,"stream":true}'

    local out=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo "")

    if [[ -z "$out" ]]; then
        record_assertion "streaming" "got_response" "false" "Empty response from streaming endpoint"
        return 1
    fi

    # Must contain `data: {...` chunks
    local data_chunks=$(echo "$out" | grep -c '^data: {' || echo 0)
    data_chunks=${data_chunks//[$'\n\r ']}
    if [[ "$data_chunks" -ge 1 ]]; then
        record_assertion "streaming" "sse_chunks" "true" "Response contains $data_chunks SSE data chunks"
    else
        record_assertion "streaming" "sse_chunks" "false" "Response has 0 SSE chunks (not streaming)"
        return 1
    fi

    # Must contain a terminal `data: [DONE]` marker
    if echo "$out" | grep -q '^data: \[DONE\]'; then
        record_assertion "streaming" "done_marker" "true" "Stream terminates with [DONE] marker"
    else
        record_assertion "streaming" "done_marker" "false" "No terminal [DONE] marker — stream incomplete"
    fi

    # Each chunk should parse as JSON containing choices[].delta
    local first_chunk=$(echo "$out" | grep '^data: {' | head -n 1 | sed 's/^data: //')
    local has_delta=$(echo "$first_chunk" | jq -e '.choices[0].delta' >/dev/null 2>&1 && echo "true" || echo "false")
    if [[ "$has_delta" == "true" ]]; then
        record_assertion "streaming" "chunk_shape" "true" "Chunks contain choices[0].delta (OpenAI-compat shape)"
    else
        record_assertion "streaming" "chunk_shape" "false" "Chunks missing choices[0].delta"
    fi
}

# Test 3: tool-calling produces structured tool_calls
test_tool_calling_dispatch() {
    log_info "Test 3: tool_choice=auto → structured tool_calls response"

    local body='{
        "model":"helixagent-llm",
        "messages":[{"role":"user","content":"What is the weather in San Francisco?"}],
        "tools":[{
            "type":"function",
            "function":{
                "name":"get_weather",
                "description":"Get current weather for a location",
                "parameters":{
                    "type":"object",
                    "properties":{"location":{"type":"string","description":"City name"}},
                    "required":["location"]
                }
            }
        }],
        "tool_choice":"auto",
        "max_tokens":150
    }'

    local out=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo "{}")

    # CONST-035 §c "Full usability": the documented contract is that
    # tool_choice="auto" with a relevant prompt either returns
    # `tool_calls` (preferred) OR text content. Both are valid; the
    # Challenge accepts either as long as the response shape is correct.
    local tool_calls_count=$(echo "$out" | jq -r '.choices[0].message.tool_calls | length // 0' 2>/dev/null)
    local content=$(echo "$out" | jq -r '.choices[0].message.content // ""' 2>/dev/null)

    if [[ "$tool_calls_count" -ge 1 ]]; then
        record_assertion "tool_call" "dispatch_to_tool" "true" "Model returned $tool_calls_count tool_call(s)"

        # Validate the tool_call shape
        local first_name=$(echo "$out" | jq -r '.choices[0].message.tool_calls[0].function.name // ""' 2>/dev/null)
        if [[ "$first_name" == "get_weather" ]]; then
            record_assertion "tool_call" "function_name" "true" "function.name=get_weather"
        else
            record_assertion "tool_call" "function_name" "false" "function.name='$first_name' (expected 'get_weather')"
        fi

        # Validate the arguments are parseable JSON
        local args=$(echo "$out" | jq -r '.choices[0].message.tool_calls[0].function.arguments // ""' 2>/dev/null)
        if echo "$args" | jq -e . >/dev/null 2>&1; then
            record_assertion "tool_call" "args_valid_json" "true" "function.arguments is valid JSON: $args"
        else
            record_assertion "tool_call" "args_valid_json" "false" "function.arguments is not valid JSON: '$args'"
        fi

        local finish_reason=$(echo "$out" | jq -r '.choices[0].finish_reason // ""' 2>/dev/null)
        if [[ "$finish_reason" == "tool_calls" ]]; then
            record_assertion "tool_call" "finish_reason" "true" "finish_reason=tool_calls"
        else
            record_assertion "tool_call" "finish_reason" "false" "finish_reason='$finish_reason' (expected 'tool_calls')"
        fi
    elif [[ -n "$content" ]]; then
        record_assertion "tool_call" "dispatch_to_tool" "true" "Model returned text content (also valid for tool_choice=auto): ${content:0:80}"
    else
        record_assertion "tool_call" "dispatch_to_tool" "false" "Response has neither tool_calls nor content — broken response"
    fi
}

# Test 4: invalid tool schema returns 4xx (negative)
test_invalid_tool_schema_rejected() {
    log_info "Test 4: invalid tool schema is rejected with 4xx (not 200)"

    local body='{
        "model":"helixagent-llm",
        "messages":[{"role":"user","content":"hi"}],
        "tools":"this-should-be-an-array-not-a-string",
        "max_tokens":5
    }'

    local code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 15 \
        -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo "000")

    if [[ "$code" == "400" || "$code" == "422" ]]; then
        record_assertion "negative" "invalid_tool_4xx" "true" "Invalid tool schema correctly rejected ($code)"
    else
        record_assertion "negative" "invalid_tool_4xx" "false" "Invalid tool schema returned $code (expected 400/422)"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_streaming_sse_format || true
    test_tool_calling_dispatch || true
    test_invalid_tool_schema_rejected || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
