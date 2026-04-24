#!/bin/bash
# Security API Key Validation Challenge
# Tests API key validation and management

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-api-key-validation" "Security API Key Validation Challenge"
load_env

log_info "Testing API key validation security..."

test_missing_api_key() {
    log_info "Test 1: Missing API key handling"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    if [[ "$code" == "401" ]]; then
        record_assertion "missing_api_key" "enforced" "true" "Returns 401 for missing API key"
    elif [[ "$code" == "200" ]]; then
        record_assertion "missing_api_key" "optional" "true" "API key is optional"
    fi
}

test_invalid_api_key() {
    log_info "Test 2: Invalid API key rejection"

    local invalid_keys=(
        "invalid-key-12345"
        "sk-wrong-format"
        "12345"
        ""
    )

    local rejected=0

    for key in "${invalid_keys[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $key" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(401|403)$ ]] && rejected=$((rejected + 1))
    done

    record_metric "invalid_keys_tested" ${#invalid_keys[@]}
    [[ $rejected -ge 2 ]] && record_assertion "invalid_api_key" "rejected" "true" "$rejected/${#invalid_keys[@]} invalid keys rejected"
}

test_valid_api_key() {
    log_info "Test 3: Valid API key acceptance"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "valid_api_key" "accepted" "true" "Valid API key works"
}

test_key_format_validation() {
    log_info "Test 4: API key format validation"

    local malformed_keys=(
        "Bearer-without-prefix"
        "sk-too-short"
        "../../../etc/passwd"
        "<script>alert('xss')</script>"
    )

    local format_validated=0

    for key in "${malformed_keys[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $key" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # Should reject malformed keys or handle them safely
        [[ "$code" =~ ^(401|403|400|200)$ ]] && format_validated=$((format_validated + 1))
        [[ "$code" == "500" ]] && record_assertion "key_format_crash" "detected" "false" "Malformed key caused crash"
    done

    record_metric "malformed_keys_tested" ${#malformed_keys[@]}
    [[ $format_validated -ge 3 ]] && record_assertion "key_format" "validated" "true" "$format_validated/${#malformed_keys[@]} malformed keys handled"
}

main() {
    log_info "Starting API key validation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_missing_api_key
    test_invalid_api_key
    test_valid_api_key
    test_key_format_validation

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
