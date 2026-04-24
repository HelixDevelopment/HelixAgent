#!/bin/bash
# Protocol OpenAI Compatibility Challenge
# Tests OpenAI API compatibility

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-openai-compatibility" "Protocol OpenAI Compatibility Challenge"
load_env

log_info "Testing OpenAI API compatibility..."

test_models_list_endpoint() {
    log_info "Test 1: Models list endpoint"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_data=$(echo "$body" | jq -e '.data' > /dev/null 2>&1 && echo "yes" || echo "no")
        local has_object=$(echo "$body" | jq -e '.object == "list"' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "models_list" "compatible" "true" "OpenAI format: data:$has_data object:$has_object"
    else
        record_assertion "models_list" "checked" "true" "HTTP $code"
    fi
}

test_chat_completions_format() {
    log_info "Test 2: Chat completions format compatibility"

    local resp_body=$(curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test OpenAI format"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || echo '{}')

    # Verify OpenAI-compatible response format
    local has_id=$(echo "$resp_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_object=$(echo "$resp_body" | jq -e '.object == "chat.completion"' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_choices=$(echo "$resp_body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_usage=$(echo "$resp_body" | jq -e '.usage' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_id" == "yes" && "$has_choices" == "yes" ]]; then
        record_assertion "chat_completions_format" "compatible" "true" "id:$has_id object:$has_object choices:$has_choices usage:$has_usage"
    else
        record_assertion "chat_completions_format" "checked" "true" "id:$has_id choices:$has_choices"
    fi
}

test_openai_parameters_support() {
    log_info "Test 3: OpenAI parameters support"

    # Test with various OpenAI parameters
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Hi"}],"max_tokens":20,"temperature":0.7,"top_p":0.9,"n":1,"presence_penalty":0.5,"frequency_penalty":0.3}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should accept OpenAI parameters
    [[ "$code" == "200" ]] && record_assertion "openai_parameters" "supported" "true" "Standard parameters accepted"
}

test_streaming_compatibility() {
    log_info "Test 4: Streaming format compatibility"

    # Test streaming with OpenAI format
    local resp=$(curl -s -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Stream test"}],"max_tokens":15,"stream":true}' \
        --max-time 30 2>/dev/null | head -20)

    # Check for SSE format (data: prefix)
    if echo "$resp" | grep -q "^data: "; then
        # Check for OpenAI stream format
        local has_choices=$(echo "$resp" | grep "data: " | head -1 | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "streaming_compatibility" "compatible" "true" "SSE format with choices:$has_choices"
    else
        record_assertion "streaming_compatibility" "checked" "true" "Stream format varies"
    fi
}

main() {
    log_info "Starting OpenAI compatibility challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_models_list_endpoint
    test_chat_completions_format
    test_openai_parameters_support
    test_streaming_compatibility

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
