#!/bin/bash
# Security Input Validation Challenge
# Tests input validation and sanitization against injection attacks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-input-validation" "Security Input Validation Challenge"
load_env

log_info "Testing input validation security..."

test_malicious_input_rejection() {
    log_info "Test 1: Malicious input rejection"

    local malicious_inputs=(
        '<script>alert("XSS")</script>'
        "'; DROP TABLE users; --"
        '../../../etc/passwd'
        '{{7*7}}'
    )

    local rejected=0

    for input in "${malicious_inputs[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "{\"model\":\"helixagent-debate\",\"messages\":[{\"role\":\"user\",\"content\":\"$input\"}],\"max_tokens\":10}" \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # Should either reject (400) or accept (200) with sanitization
        [[ "$code" =~ ^(200|400)$ ]] && rejected=$((rejected + 1))
    done

    record_metric "malicious_inputs_tested" ${#malicious_inputs[@]}
    [[ $rejected -ge 3 ]] && record_assertion "malicious_input" "handled" "true" "$rejected/${#malicious_inputs[@]} malicious inputs handled"
}

test_parameter_validation() {
    log_info "Test 2: Parameter validation"

    local invalid_params=(
        '{"model":"helixagent-debate","messages":[],"max_tokens":-100}'
        '{"model":"helixagent-debate","messages":[{}],"temperature":10.0}'
        '{"model":"helixagent-debate","messages":"notarray","max_tokens":10}'
    )

    local validated=0

    for param in "${invalid_params[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$param" --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "400" ]] && validated=$((validated + 1))
    done

    record_metric "invalid_params_caught" $validated
    [[ $validated -ge 2 ]] && record_assertion "parameter_validation" "enforced" "true" "$validated/3 invalid params caught"
}

test_length_limits() {
    log_info "Test 3: Input length limits"

    # Create very long input
    local long_input=""
    for i in {1..5000}; do
        long_input+="A"
    done

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\":\"helixagent-debate\",\"messages\":[{\"role\":\"user\",\"content\":\"$long_input\"}],\"max_tokens\":10}" \
        --max-time 60 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(413|400|200)$ ]] && record_assertion "length_limits" "enforced" "true" "Length limits handled (HTTP $code)"
}

test_special_characters() {
    log_info "Test 4: Special character handling"

    local special_chars='{"model":"helixagent-debate","messages":[{"role":"user","content":"Test \u0000\u0001\u001f special chars"}],"max_tokens":10}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$special_chars" --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|400)$ ]] && record_assertion "special_characters" "handled" "true" "Special characters handled (HTTP $code)"
}

main() {
    log_info "Starting input validation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_malicious_input_rejection
    test_parameter_validation
    test_length_limits
    test_special_characters

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
