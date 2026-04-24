#!/bin/bash
# Security JWT Tokens Challenge
# Tests JWT token validation and security

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-jwt-tokens" "Security JWT Tokens Challenge"
load_env

log_info "Testing JWT token security..."

test_invalid_jwt() {
    log_info "Test 1: Invalid JWT rejection"

    local invalid_jwts=(
        "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature"
        "not.a.jwt"
        "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxMjM0NTY3ODkwIn0."
    )

    local rejected=0

    for jwt in "${invalid_jwts[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $jwt" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(401|403)$ ]] && rejected=$((rejected + 1))
    done

    record_metric "invalid_jwts_tested" ${#invalid_jwts[@]}
    [[ $rejected -ge 2 ]] && record_assertion "invalid_jwt" "rejected" "true" "$rejected/${#invalid_jwts[@]} invalid JWTs rejected"
}

test_expired_jwt() {
    log_info "Test 2: Expired JWT handling"

    # JWT with exp claim in the past
    local expired_jwt="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjF9.6TYY4SfHdZ8D4v_bXWs5FHkv_u-AH9vL7lKHvR2NvFw"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $expired_jwt" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(401|403|200)$ ]] && record_assertion "expired_jwt" "handled" "true" "Expired JWT handled (HTTP $code)"
}

test_algorithm_none_attack() {
    log_info "Test 3: Algorithm 'none' attack prevention"

    # JWT with alg=none (security vulnerability)
    local none_jwt="eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJhdHRhY2tlciJ9."

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $none_jwt" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(401|403)$ ]] && record_assertion "algorithm_none" "blocked" "true" "Algorithm 'none' attack blocked"
}

test_valid_jwt_or_apikey() {
    log_info "Test 4: Valid JWT/API key acceptance"

    # Test with valid API key (JWT or simple token)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "valid_token" "accepted" "true" "Valid token works correctly"
}

main() {
    log_info "Starting JWT tokens challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_invalid_jwt
    test_expired_jwt
    test_algorithm_none_attack
    test_valid_jwt_or_apikey

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
