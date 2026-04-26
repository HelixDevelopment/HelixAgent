#!/bin/bash
# OpenCode Streaming Tool-Call Challenge
#
# CONST-032 reproduction script for the user-reported regression:
# OpenCode says "I'll investigate the codebase first, then create AGENTS.md"
# but never actually invokes any tools — the streaming response carries
# only text, never a tool_call delta, so OpenCode hangs at
# "Build · Helix LLM Helix Agent" indefinitely.
#
# Hypothesis (after b6f6b7e8 fixed non-streaming tool_calls): the
# streaming path drops tool_calls because:
#   - peekFirstContent only checks chunk.Content (not chunk.ToolCalls)
#   - convertChunkToSSE only emits delta.content (not delta.tool_calls)
# So even if an upstream provider streams tool_call deltas, our code
# either skips them as "empty" or strips the field on the way out.
#
# Reproduction strategy: send a streaming request with model=helix-llm
# and a tool that the prompt FORCES the model to invoke (asking for the
# current time and providing a get_current_time tool — there's no way
# to honestly answer without invoking it). Capture the SSE. Assert at
# least one chunk contains delta.tool_calls.
#
# Pass criteria:
#   1. SSE stream terminates with [DONE]
#   2. At least one chunk has delta.tool_calls with a function name
#   3. Reconstructed tool_call has a function name and arguments that
#      parse as JSON

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "opencode_streaming_tool_call" \
    "OpenCode Streaming Tool-Call Challenge (CONST-032 reproduction guard)"
load_env

build_body() {
    python3 -c '
import json
print(json.dumps({
    "model": "helix-llm",
    "messages": [
        {"role": "system", "content": "You are a precise assistant. When the user needs the current time you MUST call get_current_time — never guess."},
        {"role": "user", "content": "What is the current time? Use the tool — do not guess."}
    ],
    "max_tokens": 200,
    "stream": True,
    "tools": [{
        "type": "function",
        "function": {
            "name": "get_current_time",
            "description": "Return the current time as ISO-8601",
            "parameters": {"type": "object", "properties": {}, "required": []}
        }
    }],
    "tool_choice": "required"
}))
'
}

test_streaming_tool_call() {
    log_info "Sending streaming request that requires tool invocation..."
    local body
    body=$(build_body)

    local raw
    raw=$(curl -s -m 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    if [[ -z "$raw" ]]; then
        record_assertion "transport" "got_response" "false" \
            "empty SSE — server unreachable"
        return
    fi
    record_assertion "transport" "got_response" "true" "got ${#raw} bytes"

    log_info "Raw response first 800 chars: ${raw:0:800}"

    # Stream end marker
    if echo "$raw" | grep -q '^data: \[DONE\]'; then
        record_assertion "stream" "ended_with_done" "true" "ended cleanly"
    else
        record_assertion "stream" "ended_with_done" "false" "missing [DONE]"
    fi

    # Parse all chunks, look for tool_call deltas
    local analysis
    analysis=$(echo "$raw" | python3 -c '
import json, sys
chunks = 0
bad = 0
content_chars = 0
tool_call_deltas = 0
function_name = ""
arguments_buf = []
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
        tcs = delta.get("tool_calls")
        if isinstance(tcs, list) and tcs:
            tool_call_deltas += 1
            for tc in tcs:
                fn = tc.get("function") or {}
                if fn.get("name"):
                    function_name = fn.get("name")
                if fn.get("arguments"):
                    arguments_buf.append(fn.get("arguments"))
joined_args = "".join(arguments_buf)
args_parse = "n/a"
if joined_args.strip():
    try:
        json.loads(joined_args)
        args_parse = "ok"
    except Exception as e:
        args_parse = f"fail: {e}"
print(f"chunks={chunks} bad={bad} content={content_chars} "
      f"tool_call_deltas={tool_call_deltas} function_name={function_name!r} "
      f"args_parse={args_parse} args_len={len(joined_args)}")
')
    log_info "Stream analysis: $analysis"
    eval "$analysis"

    if [[ "$tool_call_deltas" -ge 1 ]]; then
        record_assertion "tool_calls" "deltas_present" "true" \
            "$tool_call_deltas chunks carried tool_call deltas"
    else
        record_assertion "tool_calls" "deltas_present" "false" \
            "no chunk carried delta.tool_calls — model returned only text (${content:-0} chars), OpenCode would hang waiting for tool invocation"
    fi

    if [[ -n "$function_name" && "$function_name" != "''" ]]; then
        record_assertion "tool_calls" "function_named" "true" \
            "function name surfaced: $function_name"
    else
        record_assertion "tool_calls" "function_named" "false" \
            "no function name in any tool_call delta"
    fi

    if [[ "$args_parse" == "ok" || "$args_parse" == "n/a" ]]; then
        record_assertion "tool_calls" "args_well_formed" "true" \
            "arguments parse status: $args_parse"
    else
        record_assertion "tool_calls" "args_well_formed" "false" \
            "arguments parse status: $args_parse"
    fi

    record_metric "chunks" "$chunks"
    record_metric "bad_chunks" "$bad"
    record_metric "content_chars" "${content:-0}"
    record_metric "tool_call_deltas" "$tool_call_deltas"
}

main() {
    log_info "Starting OpenCode Streaming Tool-Call Challenge"
    log_info "Base URL: $BASE_URL"
    if ! curl -s -m 5 "$BASE_URL/health" > /dev/null 2>&1; then
        log_error "HelixAgent not running on $BASE_URL"
        finalize_challenge "FAILED"
        exit 1
    fi

    test_streaming_tool_call

    local failed_count
    failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)
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
