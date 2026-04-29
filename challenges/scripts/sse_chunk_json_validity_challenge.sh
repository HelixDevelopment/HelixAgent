#!/bin/bash
# SSE Chunk JSON Validity Challenge
#
# CONST-032 reproduction script for the user-reported bug:
#   "JSON parsing failed: Text: {"id":"chatcmpl-...","model":"helix-llm",
#    "choices":[{"index":0,"delta":{"content":"..
#    Error message: JSON Parse error: Unterminated string"
#
# The bug: convertChunkToSSE in internal/handlers/openai_compatible.go
# builds the SSE payload via fmt.Sprintf with chunk.Content
# interpolated directly into a JSON string field. When the upstream
# model returns content containing an unescaped " (quote) or \
# (backslash) or a literal newline, the resulting "JSON" is malformed
# — the chunk arrives at the client looking valid up to the bad
# character then truncates / has stray quotes mid-token.
#
# Reproduction strategy: send 5 chat requests whose prompts are likely
# to elicit content with quotes (asking the model to quote text, write
# code, or include a JSON example). Capture every `data:` line. Parse
# each as JSON. If ANY chunk fails to parse, the bug is reproduced.
#
# Pass criteria:
#   1. Across all 5 prompts, every emitted SSE `data:` chunk parses
#      as valid JSON (excluding the literal `[DONE]` sentinel).
#   2. At least one chunk per prompt has non-empty content (so we
#      know we actually exercised the code path; an empty stream
#      doesn't prove the parser is correct).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "sse_chunk_json_validity" \
    "SSE Chunk JSON Validity Challenge (CONST-032 reproduction guard)"
load_env

PROMPTS=(
    'Reply with exactly this string and nothing else: He said "hello world".'
    'Show me a JSON object example with three string fields.'
    'Write a Python one-liner: print("hi")'
    'Quote the first sentence of Moby Dick verbatim, in double quotes.'
    'Explain what {"a": "b"} means in JSON.'
)

declare -i total_chunks=0
declare -i bad_chunks=0
declare -i prompts_with_content=0

for i in "${!PROMPTS[@]}"; do
    prompt="${PROMPTS[$i]}"
    log_info "Prompt $((i+1))/${#PROMPTS[@]}: ${prompt:0:60}..."

    body=$(python3 -c "
import json, sys
print(json.dumps({
    'model': 'helix-llm',
    'messages': [{'role': 'user', 'content': sys.argv[1]}],
    'max_tokens': 200,
    'stream': True,
}))" "$prompt")

    raw=$(curl -s -m 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    if [[ -z "$raw" ]]; then
        log_warning "Prompt $((i+1)): empty response"
        continue
    fi

    # Parse each `data:` line.
    parse_result=$(echo "$raw" | python3 -c '
import json, sys
chunks = 0
bad = 0
content_chars = 0
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
    except Exception as e:
        bad += 1
        sys.stderr.write(f"BAD_CHUNK | {type(e).__name__}: {e} | first120={payload[:120]}\n")
        continue
    for ch in obj.get("choices", []):
        c = ch.get("delta", {}).get("content")
        if isinstance(c, str):
            content_chars += len(c)
print(f"chunks={chunks} bad={bad} content_chars={content_chars}")
')
    log_info "  $parse_result"
    eval "$parse_result"  # imports chunks=, bad=, content_chars=
    total_chunks+=$chunks
    bad_chunks+=$bad
    if [[ $content_chars -gt 0 ]]; then
        prompts_with_content+=1
    fi
done

log_info "Aggregate: total_chunks=$total_chunks bad_chunks=$bad_chunks prompts_with_content=$prompts_with_content/${#PROMPTS[@]}"

# Assertion #1: zero malformed chunks.
if [[ $bad_chunks -eq 0 ]]; then
    record_assertion "json" "all_chunks_parse" "true" \
        "all $total_chunks SSE chunks parse as valid JSON"
else
    record_assertion "json" "all_chunks_parse" "false" \
        "$bad_chunks/$total_chunks SSE chunks failed JSON.parse — convertChunkToSSE is producing malformed JSON for content containing quotes/backslashes/newlines"
fi

# Assertion #2: at least one prompt actually got content (so we
# didn't validate against an empty stream).
if [[ $prompts_with_content -ge 1 ]]; then
    record_assertion "coverage" "exercised_real_content" "true" \
        "$prompts_with_content/${#PROMPTS[@]} prompts produced non-empty content"
else
    record_assertion "coverage" "exercised_real_content" "false" \
        "no prompt produced non-empty content — assertion #1 didn't actually test the parser"
fi

record_metric "total_chunks" "$total_chunks"
record_metric "bad_chunks" "$bad_chunks"
record_metric "prompts_with_content" "$prompts_with_content"

main() {
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
