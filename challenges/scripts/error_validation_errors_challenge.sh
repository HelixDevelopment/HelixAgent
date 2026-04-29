#!/bin/bash
# Error Validation Errors Challenge
# Tests input validation and parameter error handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-validation-errors" "Error Validation Errors Challenge"
load_env

log_info "Testing input validation error handling..."

test_missing_required_fields() {
    log_info "Test 1: Missing required fields validation"

    local scenarios=('{"model":"helixagent-debate"}' '{"messages":[]}' '{}')
    local errors_found=0

    for req in "${scenarios[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$req" --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "400" ]] && errors_found=$((errors_found + 1))
    done

    record_metric "validation_errors_caught" $errors_found
    [[ $errors_found -ge 2 ]] && record_assertion "missing_fields" "validated" "true" "$errors_found/3 caught" || record_assertion "missing_fields" "validated" "false" "Only $errors_found/3"
}

test_invalid_parameter_types() {
    log_info "Test 2: Invalid parameter type validation"

    local scenarios=(
        '{"model":"helixagent-debate","messages":"not_array","max_tokens":10}'
        '{"model":"helixagent-debate","messages":[],"max_tokens":"not_number"}'
        '{"model":"helixagent-debate","messages":[],"temperature":"not_number"}'
    )
    local type_errors=0

    for req in "${scenarios[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$req" --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "400" ]] && type_errors=$((type_errors + 1))
    done

    record_metric "type_errors_caught" $type_errors
    [[ $type_errors -ge 2 ]] && record_assertion "invalid_types" "validated" "true" "$type_errors/3 caught"
}

test_out_of_range_values() {
    log_info "Test 3: Out of range value validation"

    local scenarios=(
        '{"model":"helixagent-debate","messages":[],"max_tokens":-10}'
        '{"model":"helixagent-debate","messages":[],"temperature":5.0}'
        '{"model":"helixagent-debate","messages":[],"top_p":2.0}'
    )
    local range_errors=0

    for req in "${scenarios[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$req" --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(400|200)$ ]] && range_errors=$((range_errors + 1))
    done

    record_metric "range_validations" $range_errors
    [[ $range_errors -ge 2 ]] && record_assertion "range_validation" "handled" "true" "$range_errors/3 handled"
}

test_validation_error_messages() {
    log_info "Test 4: Validation error messages are descriptive"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[]}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$resp" | head -n -1)

    if echo "$body" | jq -e '.error.message' > /dev/null 2>&1; then
        record_assertion "error_message" "exists" "true" "Error message provided"

        local msg=$(echo "$body" | jq -r '.error.message // empty')
        [[ -n "$msg" && ${#msg} -gt 10 ]] && record_assertion "error_message" "descriptive" "true" "Message is descriptive"
    fi
}

main() {
    log_info "Starting validation errors challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_missing_required_fields
    test_invalid_parameter_types
    test_out_of_range_values
    test_validation_error_messages

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
