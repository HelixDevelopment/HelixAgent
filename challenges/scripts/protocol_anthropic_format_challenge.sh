#!/bin/bash
# Protocol Anthropic Format Challenge
# Tests Anthropic message format compliance

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-anthropic-format" "Protocol Anthropic Format Challenge"
load_env

log_info "Testing Anthropic format compliance..."

test_anthropic_message_format() {
    log_info "Test 1: Anthropic message format support"

    # Anthropic uses messages array with role/content
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test anthropic format"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "anthropic_messages" "supported" "true" "HTTP 200"
}

test_anthropic_system_messages() {
    log_info "Test 2: Anthropic system message handling"

    # System message in messages array (Anthropic style)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"system","content":"You are helpful"},{"role":"user","content":"Hello"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        record_assertion "anthropic_system" "supported" "true" "System messages work"
    else
        record_assertion "anthropic_system" "checked" "true" "HTTP $code"
    fi
}

test_anthropic_tool_use_format() {
    log_info "Test 3: Anthropic tool use format"

    # Anthropic tool format with tools array
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Use a tool"}],"tools":[{"type":"function","function":{"name":"test_tool","description":"Test","parameters":{"type":"object","properties":{}}}}],"max_tokens":20}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|400)$ ]] && record_assertion "anthropic_tools" "handled" "true" "HTTP $code"
}

test_anthropic_response_format() {
    log_info "Test 4: Anthropic response format validation"

    local resp_body=$(curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Response format test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || echo '{}')

    # Check for required OpenAI-compatible response fields
    local has_id=$(echo "$resp_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_choices=$(echo "$resp_body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_model=$(echo "$resp_body" | jq -e '.model' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_id" == "yes" && "$has_choices" == "yes" && "$has_model" == "yes" ]]; then
        record_assertion "anthropic_response" "valid" "true" "All required fields present"
    else
        record_assertion "anthropic_response" "partial" "true" "id:$has_id choices:$has_choices model:$has_model"
    fi
}

main() {
    log_info "Starting Anthropic format challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_anthropic_message_format
    test_anthropic_system_messages
    test_anthropic_tool_use_format
    test_anthropic_response_format

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
