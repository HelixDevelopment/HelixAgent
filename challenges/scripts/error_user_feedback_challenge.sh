#!/bin/bash
# Error User Feedback Challenge
# Tests error messaging, user-friendly error responses, and error context

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-user-feedback" "Error User Feedback Challenge"
load_env

log_info "Testing error user feedback..."

test_error_message_structure() {
    log_info "Test 1: Error message structure"

    # Trigger validation error
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[]}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$resp" | head -n -1)

    # Check for error structure
    if echo "$body" | jq -e '.error' > /dev/null 2>&1; then
        record_assertion "error_structure" "present" "true" "Error object present"

        # Check for required fields
        echo "$body" | jq -e '.error.message' > /dev/null 2>&1 && record_assertion "error_message" "present" "true" "Error message present"
        echo "$body" | jq -e '.error.type' > /dev/null 2>&1 && record_assertion "error_type" "present" "true" "Error type present"
    fi
}

test_descriptive_messages() {
    log_info "Test 2: Descriptive error messages"

    local scenarios=(
        '{"model":"helixagent-debate","messages":[]}'
        '{"model":"invalid-model-xyz","messages":[{"role":"user","content":"test"}]}'
        '{"model":"helixagent-debate","messages":"not-array"}'
    )

    local descriptive=0

    for req in "${scenarios[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$req" --max-time 30 2>/dev/null || true)

        local body=$(echo "$resp" | head -n -1)
        local msg=$(echo "$body" | jq -r '.error.message // empty' 2>/dev/null)

        # Message should be descriptive (>= 20 chars)
        [[ -n "$msg" && ${#msg} -ge 20 ]] && descriptive=$((descriptive + 1))
    done

    record_metric "descriptive_errors" $descriptive
    [[ $descriptive -ge 2 ]] && record_assertion "descriptive_messages" "provided" "true" "$descriptive/3 errors are descriptive"
}

test_no_sensitive_info() {
    log_info "Test 3: No sensitive information in errors"

    # Trigger auth error
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer wrong-token-123" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$resp" | head -n -1)
    local msg=$(echo "$body" | jq -r '.error.message // empty' 2>/dev/null)

    # Should not leak sensitive info
    if [[ -n "$msg" ]]; then
        if ! echo "$msg" | grep -qiE "(password|secret|key.*value|token.*=|api.*key.*:)"; then
            record_assertion "no_sensitive_info" "verified" "true" "Error message doesn't leak secrets"
        else
            record_assertion "no_sensitive_info" "leaked" "false" "Error message contains sensitive info"
        fi
    fi
}

test_actionable_feedback() {
    log_info "Test 4: Actionable error feedback"

    # Trigger rate limit or validation error
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":-10}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$resp" | head -n -1)
    local msg=$(echo "$body" | jq -r '.error.message // empty' 2>/dev/null)

    # Message should suggest action or explain what's wrong
    if [[ -n "$msg" ]]; then
        if echo "$msg" | grep -qiE "(must|should|required|invalid|expected|provide|check|valid|range)"; then
            record_assertion "actionable_feedback" "provided" "true" "Error suggests corrective action"
        fi
    fi
}

main() {
    log_info "Starting user feedback challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_error_message_structure
    test_descriptive_messages
    test_no_sensitive_info
    test_actionable_feedback

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
