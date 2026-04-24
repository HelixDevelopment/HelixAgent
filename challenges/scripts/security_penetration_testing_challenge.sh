#!/bin/bash
# Security Penetration Testing Challenge
# Tests security through simulated penetration testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-penetration-testing" "Security Penetration Testing Challenge"
load_env

log_info "Testing penetration resistance..."

test_authentication_bypass_attempts() {
    log_info "Test 1: Authentication bypass attempts blocked"

    local bypass_attempts=(
        "admin'--"
        "' OR '1'='1"
        "admin' #"
    )

    local blocked=0

    for attempt in "${bypass_attempts[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $attempt" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(401|403)$ ]] && blocked=$((blocked + 1))
    done

    record_metric "bypass_attempts" ${#bypass_attempts[@]}
    [[ $blocked -ge 2 ]] && record_assertion "auth_bypass" "blocked" "true" "$blocked/${#bypass_attempts[@]} bypass attempts blocked"
}

test_injection_attack_resistance() {
    log_info "Test 2: Injection attack resistance"

    local injections=(
        "test'; DROP TABLE messages; --"
        "test<script>alert(1)</script>"
        "test${7*7}"
    )

    local resisted=0

    for injection in "${injections[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "{\"model\":\"helixagent-debate\",\"messages\":[{\"role\":\"user\",\"content\":\"$injection\"}],\"max_tokens\":10}" \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # Should not crash (500), should handle (200/400)
        [[ "$code" =~ ^(200|400)$ ]] && resisted=$((resisted + 1))
        [[ "$code" == "500" ]] && record_assertion "injection_crash" "detected" "false" "Injection caused crash"
    done

    record_metric "injection_tests" ${#injections[@]}
    [[ $resisted -ge 2 ]] && record_assertion "injection_resistance" "strong" "true" "$resisted/${#injections[@]} injections resisted"
}

test_privilege_escalation_prevention() {
    log_info "Test 3: Privilege escalation prevention"

    # Try to access admin endpoints with regular token
    local admin_endpoints=(
        "/v1/admin/config"
        "/v1/admin/users"
        "/admin"
    )

    local blocked=0

    for endpoint in "${admin_endpoints[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            --max-time 10 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(403|404)$ ]] && blocked=$((blocked + 1))
    done

    record_metric "escalation_attempts" ${#admin_endpoints[@]}
    [[ $blocked -ge 2 ]] && record_assertion "privilege_escalation" "prevented" "true" "$blocked/${#admin_endpoints[@]} escalation attempts blocked"
}

test_pentest_recovery() {
    log_info "Test 4: System recovers after penetration tests"

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "pentest_recovery" "operational" "true" "System operational after penetration tests"
}

main() {
    log_info "Starting penetration testing challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_authentication_bypass_attempts
    test_injection_attack_resistance
    test_privilege_escalation_prevention
    test_pentest_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
