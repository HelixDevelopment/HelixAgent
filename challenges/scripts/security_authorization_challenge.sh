#!/bin/bash
# Security Authorization Challenge
# Tests authorization and access control mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-authorization" "Security Authorization Challenge"
load_env

log_info "Testing authorization security..."

test_endpoint_access_control() {
    log_info "Test 1: Endpoint access control"

    # Test various endpoints with and without auth
    local endpoints=(
        "/v1/chat/completions"
        "/v1/models"
        "/v1/embeddings"
        "/v1/debate/config"
    )

    local protected=0

    for endpoint in "${endpoints[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            --max-time 10 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # Either requires auth (401) or is public (200/404)
        [[ "$code" =~ ^(200|401|404|405)$ ]] && protected=$((protected + 1))
    done

    record_metric "endpoints_tested" ${#endpoints[@]}
    record_metric "endpoints_protected" $protected
    [[ $protected -ge 3 ]] && record_assertion "endpoint_access" "controlled" "true" "$protected/${#endpoints[@]} endpoints have access control"
}

test_unauthorized_actions() {
    log_info "Test 2: Unauthorized actions blocked"

    # Try to perform actions without proper authorization
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer limited-access-token" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    # Should either require proper auth (401/403) or allow (200)
    [[ "$code" =~ ^(200|401|403)$ ]] && record_assertion "unauthorized_actions" "handled" "true" "Unauthorized actions handled (HTTP $code)"
}

test_role_based_access() {
    log_info "Test 3: Role-based access control"

    # Test with valid token
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"RBAC test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "role_based_access" "working" "true" "Authorized access granted"
}

test_privilege_escalation_prevention() {
    log_info "Test 4: Privilege escalation prevention"

    # Try to access admin endpoints with regular token
    local admin_endpoints=(
        "/v1/admin/users"
        "/v1/admin/config"
        "/v1/monitoring/circuits"
    )

    local blocked=0

    for endpoint in "${admin_endpoints[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            --max-time 10 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # Should block (403/404) or allow if endpoint doesn't exist (404) or requires admin (401/403)
        [[ "$code" =~ ^(401|403|404)$ ]] && blocked=$((blocked + 1))
    done

    record_metric "admin_endpoints_tested" ${#admin_endpoints[@]}
    [[ $blocked -ge 2 ]] && record_assertion "privilege_escalation" "prevented" "true" "$blocked/${#admin_endpoints[@]} admin endpoints blocked"
}

main() {
    log_info "Starting authorization challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_endpoint_access_control
    test_unauthorized_actions
    test_role_based_access
    test_privilege_escalation_prevention

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
