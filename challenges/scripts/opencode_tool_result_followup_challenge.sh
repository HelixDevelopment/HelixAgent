#!/bin/bash
# OpenCode Tool-Result Follow-up Challenge
#
# CONST-032 reproduction script for the user-reported failure:
# "All providers failed for tool-call request: [retrying in 1s attem..."
# observed when OpenCode sends a follow-up after a tool execution
# (the second leg of the tool-calling round-trip).
#
# Diagnosis from boot.log:
#   level=error msg="Cerebras API error response"
#     body="messages.13.tool.tool_call_id: Field required..."
#   level=error msg="Mistral API returned error"
#     error_msg="Tool call id has to be defined."
#
# Upstream providers (Cerebras, Mistral, …) require every role=tool
# message to carry a tool_call_id matching a prior assistant tool_call.
# Our convertOpenAIChatRequest drops msg.ToolCallID on the floor
# because models.Message has no ToolCallID field — so when OpenCode
# sends the standard {"role":"tool","tool_call_id":"call_abc",
# "content":"<result>"} the field is gone by the time we POST to
# the upstream and every provider 4xx-rejects the request.
#
# This challenge intentionally goes BEYOND single-shot tool tests:
# it covers the FULL multi-turn flow OpenCode actually performs,
# plus the long-context shape (160 tools + tool result follow-up)
# that surfaces this bug under real CLI workloads.
#
# Pass criteria (all must hold):
#   1. HTTP 200 from a chat completion that contains role=tool
#      messages carrying tool_call_id
#   2. Response is well-formed (parseable JSON, has choices, has
#      message.content OR tool_calls)
#   3. The same request with stream=true also yields 200 and a
#      properly-terminated SSE stream
#   4. A request with tool_call_id MISSING produces a CLEAR client
#      error (4xx with descriptive message), not a 503 "all providers
#      failed" wall (so operators / clients can distinguish "we
#      malformed the request" from "infrastructure broke")
#   5. A request with 160 tools + a tool-result follow-up succeeds
#      (mirrors OpenCode's real load with the full skill registry)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "opencode_tool_result_followup" \
    "OpenCode Tool-Result Follow-up Challenge (CONST-032 reproduction guard)"
load_env

# ----- Multi-turn: assistant tool_call → tool result → final answer -----
# Body shape OpenCode sends after a model invokes a tool: assistant
# message with tool_calls + tool message with tool_call_id + content.
build_followup_body() {
    local stream=$1
    local include_tool_call_id=$2
    STREAM_FLAG="$stream" INCLUDE_TCID="$include_tool_call_id" python3 -c '
import json, os
stream = os.environ["STREAM_FLAG"].lower() == "true"
include_tcid = os.environ["INCLUDE_TCID"].lower() == "true"
tool_msg = {"role": "tool", "content": "Hello, world!\nThis is line 2."}
if include_tcid:
    tool_msg["tool_call_id"] = "call_abc123"
print(json.dumps({
    "model": "helix-llm",
    "messages": [
        {"role": "system", "content": "You are a coding assistant."},
        {"role": "user", "content": "Read /tmp/example.txt please."},
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [{
                "id": "call_abc123",
                "type": "function",
                "function": {"name": "read_file", "arguments": "{\"path\":\"/tmp/example.txt\"}"}
            }]
        },
        tool_msg,
        {"role": "user", "content": "What did the file say? Reply in one short sentence."}
    ],
    "max_tokens": 100,
    "stream": stream,
    "tools": [{
        "type": "function",
        "function": {
            "name": "read_file",
            "description": "Read a file from disk",
            "parameters": {"type": "object", "properties": {"path": {"type": "string"}}, "required": ["path"]}
        }
    }]
}))
'
}

# Body that mirrors OpenCode's real shape: ~160 tools, multi-turn with
# a tool result. Covers the load case that triggered the user's
# "All providers failed for tool-call request".
build_full_load_body() {
    local stream=$1
    STREAM_FLAG="$stream" python3 -c '
import json, os
stream = os.environ["STREAM_FLAG"].lower() == "true"
# Generate 160 fake tools matching OpenCodes skill-registry shape.
tools = []
for i in range(160):
    tools.append({
        "type": "function",
        "function": {
            "name": f"skill_{i:03d}",
            "description": f"Skill #{i} for some specialized task",
            "parameters": {
                "type": "object",
                "properties": {"input": {"type": "string"}},
                "required": ["input"]
            }
        }
    })
print(json.dumps({
    "model": "helix-llm",
    "messages": [
        {"role": "system", "content": "You are an agent with access to many skills."},
        {"role": "user", "content": "Read /tmp/example.txt then summarize it."},
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [{
                "id": "call_load_test_xyz",
                "type": "function",
                "function": {"name": "skill_005", "arguments": "{\"input\":\"/tmp/example.txt\"}"}
            }]
        },
        {
            "role": "tool",
            "tool_call_id": "call_load_test_xyz",
            "content": "File contents: Hello from the test!"
        },
        {"role": "user", "content": "Summarize in one sentence."}
    ],
    "max_tokens": 80,
    "stream": stream,
    "tools": tools
}))
'
}

# ----- Test 1: non-streaming follow-up with tool_call_id ---------------
test_non_streaming_followup() {
    log_info "Test 1: non-streaming tool-result follow-up (with tool_call_id)..."
    local body
    body=$(build_followup_body false true)

    local raw status
    raw=$(curl -s -m 90 -w "\n___STATUS:%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    status=$(echo "$raw" | grep -oE "^___STATUS:[0-9]+" | tail -1 | cut -d: -f2)
    body_only=$(echo "$raw" | sed '$d')

    log_info "  Status: $status"
    log_info "  Body (first 500): ${body_only:0:500}"

    if [[ "$status" == "200" ]]; then
        record_assertion "followup" "http_200" "true" "non-streaming follow-up returned 200"
    else
        record_assertion "followup" "http_200" "false" \
            "non-streaming follow-up returned $status — server rejected the OpenCode-shape tool result"
    fi

    local content_check
    content_check=$(echo "$body_only" | python3 -c '
import json, sys
try:
    obj = json.loads(sys.stdin.read())
except Exception as e:
    print("parse_error|" + str(e))
    sys.exit(0)
choices = obj.get("choices") or []
if not choices:
    print("no_choices|empty choices array")
    sys.exit(0)
msg = choices[0].get("message") or {}
content = msg.get("content") or ""
tc = msg.get("tool_calls") or []
if content.strip() or len(tc) > 0:
    print("ok|content_chars=" + str(len(content)) + " tool_calls=" + str(len(tc)))
else:
    print("empty|model produced neither content nor tool_calls")
')
    log_info "  Body validation: $content_check"
    if [[ "$content_check" == ok\|* ]]; then
        record_assertion "followup" "produced_response" "true" "$content_check"
    else
        record_assertion "followup" "produced_response" "false" "$content_check"
    fi
}

# ----- Test 2: streaming follow-up -------------------------------------
test_streaming_followup() {
    log_info "Test 2: streaming tool-result follow-up..."
    local body
    body=$(build_followup_body true true)

    local raw
    raw=$(curl -s -m 90 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    log_info "  Bytes received: ${#raw}"

    if [[ -z "$raw" ]]; then
        record_assertion "followup_stream" "got_response" "false" "empty SSE"
        return
    fi
    record_assertion "followup_stream" "got_response" "true" "got ${#raw} bytes"

    if echo "$raw" | grep -q '^data: \[DONE\]'; then
        record_assertion "followup_stream" "ended_with_done" "true" "ended cleanly"
    else
        record_assertion "followup_stream" "ended_with_done" "false" "missing [DONE]"
    fi

    # Was there content or tool_calls in any chunk?
    local has_signal
    has_signal=$(echo "$raw" | python3 -c '
import json, sys
content = 0; tc = 0
for line in sys.stdin:
    line = line.rstrip("\r\n")
    if not line.startswith("data: "): continue
    payload = line[len("data: "):]
    if payload == "[DONE]": continue
    try: obj = json.loads(payload)
    except: continue
    for ch in obj.get("choices", []):
        delta = ch.get("delta", {})
        c = delta.get("content")
        if isinstance(c, str): content += len(c)
        if delta.get("tool_calls"): tc += 1
print(f"content={content} tool_calls={tc}")
')
    log_info "  Signal: $has_signal"
    eval "$has_signal"
    if [[ "${content:-0}" -ge 1 || "${tc:-0}" -ge 1 ]]; then
        record_assertion "followup_stream" "produced_signal" "true" \
            "content=${content:-0} tool_calls=${tc:-0}"
    else
        record_assertion "followup_stream" "produced_signal" "false" \
            "no content AND no tool_calls — exact OpenCode hang shape on follow-up"
    fi
}

# ----- Test 3: missing tool_call_id should give clear 4xx --------------
test_missing_tool_call_id_clear_error() {
    log_info "Test 3: missing tool_call_id should produce a clear client error (not 503 wall)..."
    local body
    body=$(build_followup_body false false)

    local raw status
    raw=$(curl -s -m 60 -w "\n___STATUS:%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    status=$(echo "$raw" | grep -oE "^___STATUS:[0-9]+" | tail -1 | cut -d: -f2)
    body_only=$(echo "$raw" | sed '$d')

    log_info "  Status: $status"
    log_info "  Body (first 400): ${body_only:0:400}"

    # 4xx with explanation about tool_call_id is the right behavior.
    # 503 with "all providers failed" is the bug — we hid the real cause.
    if [[ "$status" =~ ^4[0-9][0-9]$ ]]; then
        if echo "$body_only" | grep -qiE "tool.call.id|tool_call_id"; then
            record_assertion "validation" "missing_tcid_clear_400" "true" \
                "$status with explanatory message about tool_call_id"
        else
            record_assertion "validation" "missing_tcid_clear_400" "false" \
                "$status but message doesn't mention tool_call_id"
        fi
    else
        record_assertion "validation" "missing_tcid_clear_400" "false" \
            "expected 4xx with tool_call_id message, got $status"
    fi
}

# ----- Test 4: full OpenCode load shape (160 tools + tool result) ------
test_full_load_followup() {
    log_info "Test 4: full OpenCode load shape (160 tools + tool result follow-up)..."
    local body
    body=$(build_full_load_body false)

    local raw status
    raw=$(curl -s -m 120 -w "\n___STATUS:%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    status=$(echo "$raw" | grep -oE "^___STATUS:[0-9]+" | tail -1 | cut -d: -f2)
    body_only=$(echo "$raw" | sed '$d')

    log_info "  Status: $status (req body ~$(echo -n "$body" | wc -c) bytes)"
    log_info "  Body (first 400): ${body_only:0:400}"

    if [[ "$status" == "200" ]]; then
        record_assertion "load" "full_load_200" "true" \
            "160-tool follow-up returned 200"
    else
        record_assertion "load" "full_load_200" "false" \
            "160-tool follow-up returned $status — full OpenCode workload fails"
    fi
}

main() {
    log_info "Starting OpenCode Tool-Result Follow-up Challenge"
    if ! curl -s -m 5 "$BASE_URL/health" > /dev/null 2>&1; then
        log_error "HelixAgent not running"; finalize_challenge "FAILED"; exit 1
    fi

    test_non_streaming_followup
    test_streaming_followup
    test_missing_tool_call_id_clear_error
    test_full_load_followup

    local failed_count
    failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)
    failed_count=$(echo "$failed_count" | tr -d '[:space:]')
    [[ -z "$failed_count" ]] && failed_count=0
    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"; exit 0
    else
        finalize_challenge "FAILED"; exit 1
    fi
}

main "$@"
