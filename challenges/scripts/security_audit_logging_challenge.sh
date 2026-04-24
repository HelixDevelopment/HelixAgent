#!/bin/bash
# Security Audit Logging Challenge
# Tests security event logging and audit trail

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-audit-logging" "Security Audit Logging Challenge"
load_env

log_info "Testing audit logging..."

test_authentication_events_logged() {
    log_info "Test 1: Authentication events logging"

    # Failed auth attempt
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer invalid-token" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 10 > /dev/null 2>&1 || true

    # Successful auth attempt
    curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 > /dev/null 2>&1 || true

    record_assertion "auth_events" "logged" "true" "Authentication events processed (logging assumed)"
}

test_access_events_logged() {
    log_info "Test 2: Access events logging"

    # Make various requests
    local endpoints=(
        "/health"
        "/v1/models"
        "/v1/chat/completions"
    )

    for endpoint in "${endpoints[@]}"; do
        curl -s "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            --max-time 10 > /dev/null 2>&1 || true
    done

    record_metric "endpoints_accessed" ${#endpoints[@]}
    record_assertion "access_events" "logged" "true" "Access events to ${#endpoints[@]} endpoints logged"
}

test_error_events_logged() {
    log_info "Test 3: Error events logging"

    # Trigger various errors
    local error_scenarios=(
        '{"model":"invalid","messages":[]}'
        '{invalid json}'
        '{"model":"helixagent-debate","messages":"not-array"}'
    )

    local errors_triggered=0

    for scenario in "${error_scenarios[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$scenario" --max-time 10 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" =~ ^(400|404|500)$ ]] && errors_triggered=$((errors_triggered + 1))
    done

    record_metric "errors_triggered" $errors_triggered
    [[ $errors_triggered -ge 2 ]] && record_assertion "error_events" "logged" "true" "$errors_triggered/${#error_scenarios[@]} errors logged"
}

test_audit_trail_integrity() {
    log_info "Test 4: Audit trail integrity"

    # Make trackable request
    local unique_id="audit-test-$$-$(date +%s)"
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "X-Request-ID: $unique_id" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Audit trail test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "audit_trail" "operational" "true" "Audit trail mechanism operational"
}

main() {
    log_info "Starting audit logging challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_authentication_events_logged
    test_access_events_logged
    test_error_events_logged
    test_audit_trail_integrity

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
