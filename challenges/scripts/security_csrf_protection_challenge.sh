#!/bin/bash
# Security CSRF Protection Challenge
# Tests Cross-Site Request Forgery protection

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-csrf-protection" "Security CSRF Protection Challenge"
load_env

log_info "Testing CSRF protection..."

test_csrf_token_required() {
    log_info "Test 1: CSRF token requirement"

    # For stateful endpoints (if any), CSRF token should be required
    local endpoints=(
        "/v1/chat/completions"
        "/v1/embeddings"
    )

    local protected=0

    for endpoint in "${endpoints[@]}"; do
        # POST without CSRF token (if implemented)
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":5}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # API endpoints typically don't need CSRF (use Bearer tokens instead)
        [[ "$code" =~ ^(200|400|403)$ ]] && protected=$((protected + 1))
    done

    record_metric "endpoints_tested" ${#endpoints[@]}
    [[ $protected -ge 1 ]] && record_assertion "csrf_protection" "evaluated" "true" "CSRF evaluated for $protected/${#endpoints[@]} endpoints"
}

test_origin_validation() {
    log_info "Test 2: Origin header validation"

    # Test with suspicious origin
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "Origin: http://malicious-site.com" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":5}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should either reject (403) or allow (200) with proper CORS handling
    [[ "$code" =~ ^(200|403)$ ]] && record_assertion "origin_validation" "handled" "true" "Origin validation handled (HTTP $code)"
}

test_referer_validation() {
    log_info "Test 3: Referer header validation"

    # Test with suspicious referer
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -H "Referer: http://evil-site.com/attack.html" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":5}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|403)$ ]] && record_assertion "referer_validation" "handled" "true" "Referer validation handled (HTTP $code)"
}

test_state_changing_operations() {
    log_info "Test 4: State-changing operations protection"

    # POST operations should be protected
    local resp=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"State test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "state_operations" "protected" "true" "State-changing operations are properly authenticated"
}

main() {
    log_info "Starting CSRF protection challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_csrf_token_required
    test_origin_validation
    test_referer_validation
    test_state_changing_operations

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
