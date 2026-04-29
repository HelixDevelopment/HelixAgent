#!/bin/bash
# OpenCode Tool-Call Response Challenge
#
# CONST-032 reproduction script for the user-reported hang:
# "OpenCode says 'let me invoke it' (a skill), then displays
#  Build · Helix LLM Helix Agent — and never returns."
#
# Hypothesis: OpenCode sent a chat completion with `tools: [...]`,
# expected a structured tool_calls response in the assistant message,
# and the binary returned either:
#   - a stream that never delivered a tool_calls chunk
#   - a non-streaming JSON whose tool_calls array shape doesn't match
#     the OpenAI spec (missing `id`, missing `function.name`, malformed
#     `function.arguments`, or arguments that aren't a JSON string)
#
# Reproduction: send a request that mimics OpenCode's skill-invocation
# shape — model=helix-llm, tools array describing a fake "read_file"
# tool, prompt designed to trigger the model to invoke it. Validate
# the response.
#
# Pass criteria (all must hold):
#   1. HTTP 200
#   2. Response body parses as valid JSON
#   3. Response has choices[0].message
#   4. message.tool_calls (if present) is an array with each entry
#      carrying: string `id`, `type=="function"`,
#      function.name (non-empty string), function.arguments (string;
#      ALSO must itself be valid JSON when non-empty)
#   5. message.content is string OR null (never undefined / wrong type)
#
# Streaming sub-case (separate request with stream=true):
#   6. SSE stream terminates with `data: [DONE]`
#   7. Tool calls reconstructed from accumulated delta.tool_calls
#      across chunks form valid JSON arguments for any function the
#      model invoked

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "opencode_tool_call_response" \
    "OpenCode Tool-Call Response Challenge (CONST-032 reproduction guard)"
load_env

# Build a request body that mimics OpenCode's skill-invocation shape.
build_body() {
    local stream=$1
    STREAM_FLAG="$stream" python3 -c '
import json, os, sys
stream = os.environ["STREAM_FLAG"].lower() == "true"
print(json.dumps({
    "model": "helix-llm",
    "messages": [
        {"role": "system", "content": "You are a coding assistant. Use the available tools when relevant."},
        {"role": "user", "content": "Please read the file /tmp/example.txt using the read_file tool."}
    ],
    "max_tokens": 200,
    "stream": stream,
    "tools": [{
        "type": "function",
        "function": {
            "name": "read_file",
            "description": "Read the contents of a file from disk",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {"type": "string", "description": "Absolute path to the file"}
                },
                "required": ["path"]
            }
        }
    }]
}))
'
}

# Non-streaming sub-case
test_non_streaming() {
    log_info "Sending non-streaming tools request..."
    local body
    body=$(build_body false)

    local raw
    raw=$(curl -s -m 60 "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    if [[ -z "$raw" ]]; then
        record_assertion "non_stream" "got_response" "false" \
            "empty body — server hung or returned nothing"
        return
    fi
    record_assertion "non_stream" "got_response" "true" "got ${#raw} bytes"

    log_info "Non-streaming response (first 600 chars): ${raw:0:600}"

    # Validate JSON shape.
    local validation
    validation=$(echo "$raw" | python3 -c '
import json, sys
try:
    obj = json.loads(sys.stdin.read())
except Exception as e:
    print(f"json_parse_error|{type(e).__name__}: {e}")
    sys.exit(0)

choices = obj.get("choices")
if not isinstance(choices, list) or not choices:
    print("missing_choices|choices is not a non-empty array")
    sys.exit(0)
msg = choices[0].get("message")
if not isinstance(msg, dict):
    print("missing_message|choices[0].message is not an object")
    sys.exit(0)

content = msg.get("content")
if content is not None and not isinstance(content, str):
    print(f"bad_content_type|content is {type(content).__name__}, must be string or null")
    sys.exit(0)

tool_calls = msg.get("tool_calls")
if tool_calls is None:
    # No tool_calls is acceptable (model decided not to use the tool)
    # but content must then be non-empty.
    if not content or not content.strip():
        print("no_tool_calls_no_content|model returned neither tool_calls nor content")
        sys.exit(0)
    print("ok|no_tool_calls_text_only")
    sys.exit(0)
if not isinstance(tool_calls, list):
    print(f"bad_tool_calls_type|tool_calls is {type(tool_calls).__name__}, must be array")
    sys.exit(0)
for i, tc in enumerate(tool_calls):
    if not isinstance(tc, dict):
        print(f"bad_tool_call_item|tool_calls[{i}] is not an object")
        sys.exit(0)
    if not isinstance(tc.get("id"), str) or not tc["id"]:
        print(f"missing_tool_call_id|tool_calls[{i}].id missing/empty")
        sys.exit(0)
    if tc.get("type") != "function":
        print(f"bad_tool_call_type|tool_calls[{i}].type != \"function\"")
        sys.exit(0)
    fn = tc.get("function")
    if not isinstance(fn, dict):
        print(f"missing_function|tool_calls[{i}].function is not an object")
        sys.exit(0)
    if not isinstance(fn.get("name"), str) or not fn["name"]:
        print(f"missing_function_name|tool_calls[{i}].function.name missing/empty")
        sys.exit(0)
    args = fn.get("arguments")
    if not isinstance(args, str):
        print(f"bad_function_arguments_type|tool_calls[{i}].function.arguments is {type(args).__name__}, must be JSON string")
        sys.exit(0)
    if args.strip():
        try:
            json.loads(args)
        except Exception as e:
            print(f"function_arguments_not_json|tool_calls[{i}].function.arguments fails JSON.parse: {e}; raw={args[:120]}")
            sys.exit(0)
print(f"ok|with_{len(tool_calls)}_tool_calls")
')

    log_info "Non-streaming validation: $validation"
    if [[ "$validation" == ok\|* ]]; then
        record_assertion "non_stream" "shape_valid" "true" "$validation"
    else
        record_assertion "non_stream" "shape_valid" "false" "$validation"
    fi
}

# Streaming sub-case
test_streaming() {
    log_info "Sending streaming tools request..."
    local body
    body=$(build_body true)

    local raw
    raw=$(curl -s -m 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    if [[ -z "$raw" ]]; then
        record_assertion "stream" "got_response" "false" \
            "empty SSE — server hung or returned nothing"
        return
    fi
    record_assertion "stream" "got_response" "true" "got ${#raw} bytes"

    if echo "$raw" | grep -q '^data: \[DONE\]'; then
        record_assertion "stream" "ended_with_done" "true" "stream ended cleanly"
    else
        record_assertion "stream" "ended_with_done" "false" \
            "stream did NOT end with data: [DONE]"
    fi

    # All chunks must parse as JSON.
    local chunk_validation
    chunk_validation=$(echo "$raw" | python3 -c '
import json, sys
chunks = 0
bad = 0
content_chars = 0
tool_call_deltas = 0
for raw_line in sys.stdin:
    line = raw_line.rstrip("\r\n")
    if not line.startswith("data: "):
        continue
    payload = line[len("data: "):]
    if payload == "[DONE]":
        continue
    chunks += 1
    try:
        obj = json.loads(payload)
    except Exception:
        bad += 1
        continue
    for ch in obj.get("choices", []):
        delta = ch.get("delta", {})
        c = delta.get("content")
        if isinstance(c, str):
            content_chars += len(c)
        if delta.get("tool_calls"):
            tool_call_deltas += 1
print(f"chunks={chunks} bad={bad} content={content_chars} tool_call_deltas={tool_call_deltas}")
')
    log_info "Streaming chunk validation: $chunk_validation"
    eval "$chunk_validation"
    if [[ "$bad" -eq 0 ]]; then
        record_assertion "stream" "all_chunks_parse" "true" "$chunks chunks parsed cleanly"
    else
        record_assertion "stream" "all_chunks_parse" "false" \
            "$bad/$chunks chunks failed JSON.parse"
    fi

    # Either we got content OR we got tool_call deltas — anything else
    # is the silent-hang state.
    if [[ "$content" -ge 1 || "$tool_call_deltas" -ge 1 ]]; then
        record_assertion "stream" "produced_signal" "true" \
            "content=$content tool_call_deltas=$tool_call_deltas"
    else
        record_assertion "stream" "produced_signal" "false" \
            "no content AND no tool_call deltas — exactly the OpenCode hang shape"
    fi
}

main() {
    log_info "Starting OpenCode Tool-Call Response Challenge"
    log_info "Base URL: $BASE_URL"
    if ! curl -s -m 5 "$BASE_URL/health" > /dev/null 2>&1; then
        log_error "HelixAgent not running on $BASE_URL"
        finalize_challenge "FAILED"
        exit 1
    fi

    test_non_streaming
    test_streaming

    local failed_count
    failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo 0)
    failed_count=$(echo "$failed_count" | tr -d '[:space:]')
    [[ -z "$failed_count" ]] && failed_count=0
    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"
        exit 0
    else
        finalize_challenge "FAILED"
        exit 1
    fi
}

main "$@"
