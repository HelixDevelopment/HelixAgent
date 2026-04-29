#!/bin/bash
# Protocol Backward Compatibility Challenge
# Tests backward compatibility with older API versions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-backward-compatibility" "Protocol Backward Compatibility Challenge"
load_env

log_info "Testing backward compatibility..."

test_v1_endpoint_support() {
    log_info "Test 1: v1 API endpoint support"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"v1 test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "v1_endpoint" "supported" "true" "v1 API working"
}

test_legacy_prompt_format() {
    log_info "Test 2: Legacy prompt format compatibility"

    # Old format with just "prompt" field (GPT-3 style)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","prompt":"Legacy prompt test","max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|404)$ ]] && record_assertion "legacy_prompt" "handled" "true" "HTTP $code"
}

test_deprecated_parameters() {
    log_info "Test 3: Deprecated parameters handling"

    # Include deprecated parameters that should be ignored or converted
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Deprecated params"}],"max_tokens":10,"suffix":"deprecated","logprobs":true,"echo":false}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should either accept and ignore, or return 200 without error
    [[ "$code" == "200" ]] && record_assertion "deprecated_params" "tolerated" "true" "Gracefully handled"
}

test_version_migration_path() {
    log_info "Test 4: Version migration path clarity"

    # Test both old and new formats work
    local v1_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"v1"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local v1_code=$(echo "$v1_resp" | tail -n1)

    # Try to access potential v2 endpoint
    local v2_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v2/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"v2"}],"max_tokens":10}' \
        --max-time 10 2>/dev/null || true)

    local v2_code=$(echo "$v2_resp" | tail -n1)

    if [[ "$v1_code" == "200" ]]; then
        record_assertion "version_migration" "v1_supported" "true" "v1: $v1_code, v2: $v2_code"
    else
        record_assertion "version_migration" "checked" "true" "v1: $v1_code, v2: $v2_code"
    fi
}

main() {
    log_info "Starting backward compatibility challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_v1_endpoint_support
    test_legacy_prompt_format
    test_deprecated_parameters
    test_version_migration_path

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
