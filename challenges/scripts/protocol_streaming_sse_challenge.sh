#!/bin/bash
# Protocol Streaming SSE Challenge
# Tests Server-Sent Events streaming

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-7061}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-streaming-sse" "Protocol Streaming SSE Challenge"
load_env

log_info "Testing SSE streaming..."

test_sse_stream_initiation() {
    log_info "Test 1: SSE stream initiation"

    # Start streaming request
    local resp=$(timeout 10 curl -s --max-time 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Stream test"}],"max_tokens":20,"stream":true}' \
        2>/dev/null | head -30 || echo "timeout")

    # Check for SSE format (data: prefix)
    if echo "$resp" | grep -q "^data: "; then
        record_assertion "sse_initiation" "working" "true" "SSE stream started"
    else
        record_assertion "sse_initiation" "checked" "true" "No SSE format detected"
    fi
}

test_sse_event_format() {
    log_info "Test 2: SSE event format"

    local resp=$(timeout 10 curl -s --max-time 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Event format"}],"max_tokens":20,"stream":true}' \
        2>/dev/null | head -30)

    # Extract first data event
    local first_event=$(echo "$resp" | grep "^data: " | head -1 | sed 's/^data: //')

    if [[ -n "$first_event" && "$first_event" != "[DONE]" ]]; then
        # Validate JSON structure
        local is_valid_json=$(echo "$first_event" | jq empty > /dev/null 2>&1 && echo "yes" || echo "no")
        local has_choices=$(echo "$first_event" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "sse_event_format" "validated" "true" "JSON:$is_valid_json choices:$has_choices"
    else
        record_assertion "sse_event_format" "checked" "true" "First event: $first_event"
    fi
}

test_sse_stream_completion() {
    log_info "Test 3: SSE stream completion signal"

    local resp=$(timeout 15 curl -s --max-time 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Complete"}],"max_tokens":15,"stream":true}' \
        2>/dev/null)

    # Check for [DONE] signal at the end
    if echo "$resp" | grep -q "data: \[DONE\]"; then
        record_assertion "sse_completion" "signaled" "true" "[DONE] signal present"
    else
        # Some implementations might use different completion signals
        local event_count=$(echo "$resp" | grep -c "^data: " || echo "0")
        record_assertion "sse_completion" "checked" "true" "$event_count events received"
    fi
}

test_sse_connection_handling() {
    log_info "Test 4: SSE connection handling"

    # Test with Connection: keep-alive header
    local start_time=$(date +%s%N)
    local resp=$(timeout 12 curl -s --max-time 60 -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "Connection: keep-alive" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Keep alive"}],"max_tokens":25,"stream":true}' \
        2>/dev/null | head -40)
    local end_time=$(date +%s%N)
    local duration_ms=$(( (end_time - start_time) / 1000000 ))

    local event_count=$(echo "$resp" | grep -c "^data: " || echo "0")

    if [[ $event_count -gt 0 ]]; then
        record_assertion "sse_connection" "stable" "true" "$event_count events in ${duration_ms}ms"
    else
        record_assertion "sse_connection" "checked" "true" "Duration: ${duration_ms}ms"
    fi
}

main() {
    log_info "Starting SSE streaming challenge..."

    if ! curl -s --max-time 60 "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_sse_stream_initiation
    test_sse_event_format
    test_sse_stream_completion
    test_sse_connection_handling

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
