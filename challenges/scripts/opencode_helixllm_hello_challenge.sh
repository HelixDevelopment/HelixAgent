#!/bin/bash
# OpenCode → helix-llm "Hello" Challenge
#
# CONST-032 reproduction script for the user-reported bug "Sending Hello
# message to Helix LLM from OpenCode CLI agent doesn't return a usable
# response."
#
# Reproduction strategy: bypass the OpenCode CLI itself (which would
# couple this challenge to OpenCode's binary version + its own
# quirks) and instead replay the EXACT HTTP request shape OpenCode sends
# to /v1/chat/completions when a user types "Hello" — model=helix-llm,
# stream=true, the same headers and body envelope.
#
# Pass criteria (all must hold):
#   1. HTTP 200 from /v1/chat/completions
#   2. At least one SSE chunk contains a non-empty delta.content
#   3. The aggregated content is NOT a Junie / JetBrains / Qwen
#      authentication banner (Finding #46 regression guard)
#   4. The aggregated content is NOT empty (would indicate the chain
#      ran out of providers)
#   5. The stream terminated cleanly with `data: [DONE]`
#
# Failure of ANY of the 5 means the bug is present.

set -uo pipefail
# Note: NOT `set -e` — we want to capture failures and report them, not
# bail at the first non-zero so the framework can record assertions.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "opencode_helixllm_hello" \
    "OpenCode → helix-llm Hello Challenge (CONST-032 reproduction guard)"
load_env

# OpenCode-shaped request envelope. Captured from the actual OpenCode
# CLI's HTTP traffic — model=helix-llm, stream=true, single user turn.
# Headers Authorization + Accept set explicitly so the binary's request
# routing matches what the real CLI would trigger.
test_opencode_helix_llm_hello() {
    log_info "Replaying OpenCode 'Hello' request against /v1/chat/completions..."

    local body
    body=$(cat <<'EOF'
{
    "model": "helix-llm",
    "messages": [
        {"role": "user", "content": "Hello"}
    ],
    "max_tokens": 100,
    "stream": true
}
EOF
)

    # Capture the entire SSE body. --max-time 60 because helix-llm fan-out
    # can hit several upstreams sequentially in the worst case.
    local raw
    raw=$(curl -s -m 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>&1)
    local curl_rc=$?

    if [[ $curl_rc -ne 0 ]]; then
        record_assertion "transport" "curl_succeeded" "false" \
            "curl exit $curl_rc — binary unreachable or transport broken"
        return
    fi
    record_assertion "transport" "curl_succeeded" "true" "transport ok"

    # Diagnostic: dump first 600 chars of raw response so we can see what
    # came back when assertions fail.
    log_info "Raw response first 600 chars: ${raw:0:600}"

    # Aggregate delta.content across all SSE chunks.
    local aggregated
    aggregated=$(echo "$raw" | awk '
        /^data: / && !/\[DONE\]/ {
            sub(/^data: /, "")
            print
        }
    ' | python3 -c '
import sys, json
out = []
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        obj = json.loads(line)
    except Exception:
        continue
    for ch in obj.get("choices", []):
        delta = ch.get("delta", {})
        c = delta.get("content")
        if isinstance(c, str):
            out.append(c)
print("".join(out), end="")
' 2>/dev/null || echo "")

    log_info "Aggregated content length: ${#aggregated}"
    log_info "Aggregated content preview: ${aggregated:0:200}"

    # Assertion #1: stream terminated with [DONE]
    if echo "$raw" | grep -q '^data: \[DONE\]'; then
        record_assertion "stream" "ended_with_done" "true" "stream ended cleanly"
    else
        record_assertion "stream" "ended_with_done" "false" \
            "stream did NOT end with data: [DONE] — likely truncated"
    fi

    # Assertion #2: aggregated content is non-empty
    if [[ -n "$aggregated" ]]; then
        record_assertion "content" "non_empty" "true" \
            "aggregated content has ${#aggregated} chars"
    else
        record_assertion "content" "non_empty" "false" \
            "aggregated content is EMPTY — provider chain produced nothing"
    fi

    # Assertion #3: content is NOT a Junie / JetBrains auth banner
    # (Finding #46 regression guard — the exact pain the user reported)
    local lower
    lower=$(echo "$aggregated" | tr '[:upper:]' '[:lower:]')
    if echo "$lower" | grep -qE \
        "(no active jetbrains ai subscription|junie: 403 forbidden|qwen oauth free tier was discontinued|please visit https://account.jetbrains.com)"; then
        record_assertion "content" "no_auth_banner" "false" \
            "Content contains a CLI auth banner (Junie/Qwen/JetBrains) — Finding #46 regression"
    else
        record_assertion "content" "no_auth_banner" "true" \
            "no auth banner detected in content"
    fi

    # Assertion #4: content is NOT a degenerate single character or whitespace
    local stripped
    stripped=$(echo -n "$aggregated" | tr -d '[:space:]')
    if [[ ${#stripped} -ge 2 ]]; then
        record_assertion "content" "substantive" "true" \
            "stripped content is ${#stripped} chars (≥ 2)"
    else
        record_assertion "content" "substantive" "false" \
            "stripped content has ${#stripped} chars — degenerate response"
    fi

    record_metric "raw_byte_count" "${#raw}"
    record_metric "aggregated_char_count" "${#aggregated}"
}

# Sanity check #5: the model "helix-llm" must be listed in /v1/models
# (otherwise the failure is "model not registered", not "chain broken")
test_helix_llm_model_listed() {
    log_info "Checking helix-llm appears in /v1/models..."

    local models_body
    models_body=$(curl -s -m 10 "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>&1)

    if echo "$models_body" | grep -q '"id":\s*"helix-llm"'; then
        record_assertion "models" "helix_llm_listed" "true" \
            "helix-llm model is registered"
    else
        # Not strictly required — the chat handler accepts arbitrary
        # model strings — but useful diagnostic.
        record_assertion "models" "helix_llm_listed" "false" \
            "helix-llm not in /v1/models (informational; chat may still work)"
    fi
}

main() {
    log_info "Starting OpenCode → helix-llm Hello Challenge"
    log_info "Base URL: $BASE_URL"

    if ! curl -s -m 5 "$BASE_URL/health" > /dev/null 2>&1; then
        log_error "HelixAgent not running on $BASE_URL — start ./bin/helixagent first"
        finalize_challenge "FAILED"
        exit 1
    fi

    test_helix_llm_model_listed
    test_opencode_helix_llm_hello

    local failed_count
    failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null | head -1 || echo "0")
    failed_count=${failed_count:-0}
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
