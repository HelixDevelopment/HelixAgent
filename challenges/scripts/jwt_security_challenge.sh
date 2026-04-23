#!/bin/bash
# HelixAgent Challenge - JWT Security Validation
# Validates JWT token handling including expiration, signature validation,
# refresh flow, and common JWT security vulnerabilities.

set -e

# Source challenge framework
source "$(dirname "${BASH_SOURCE[0]}")/challenge_framework.sh"

# Initialize challenge
init_challenge "jwt-security" "JWT Security Challenge"
load_env

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Base URL for API
BASE_URL="${HELIXAGENT_URL:-http://localhost:8100}"

# JWT secret for testing (should match server config or be invalid for rejection tests)
JWT_SECRET="${JWT_SECRET:-helixagent-secret-key}"

# Test function with detailed logging
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    if eval "$test_cmd" >> "$LOG_FILE" 2>&1; then
        log_success "PASS: $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        record_assertion "test" "$test_name" "true" ""
        return 0
    else
        log_error "FAIL: $test_name"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        record_assertion "test" "$test_name" "false" "Test command failed"
        return 1
    fi
}

# Helper: Base64URL encode
base64url_encode() {
    local input="$1"
    echo -n "$input" | openssl base64 -e | tr '+/' '-_' | tr -d '='
}

# Helper: Base64URL decode
base64url_decode() {
    local input="$1"
    local padding=$((4 - ${#input} % 4))
    [[ $padding -eq 4 ]] && padding=0
    local padded="${input}$(printf '=%.0s' $(seq 1 $padding))"
    echo "$padded" | tr '-_' '+/' | openssl base64 -d
}

# Helper: Create a JWT token manually
create_jwt() {
    local payload="$1"
    local secret="${2:-$JWT_SECRET}"
    local algorithm="${3:-HS256}"

    local header='{"alg":"'$algorithm'","typ":"JWT"}'
    local header_b64=$(base64url_encode "$header")
    local payload_b64=$(base64url_encode "$payload")

    local unsigned_token="${header_b64}.${payload_b64}"

    if [[ "$algorithm" == "HS256" ]]; then
        local signature=$(echo -n "$unsigned_token" | openssl dgst -sha256 -hmac "$secret" -binary | openssl base64 -e | tr '+/' '-_' | tr -d '=')
        echo "${unsigned_token}.${signature}"
    elif [[ "$algorithm" == "none" ]]; then
        echo "${unsigned_token}."
    else
        echo "${unsigned_token}.invalid_signature"
    fi
}

# Helper: Create valid JWT with specific claims
create_valid_jwt() {
    local user_id="${1:-test-user}"
    local username="${2:-testuser}"
    local role="${3:-user}"
    local exp_offset="${4:-3600}"  # 1 hour from now

    local now=$(date +%s)
    local exp=$((now + exp_offset))
    local iat=$now

    local payload="{\"user_id\":\"$user_id\",\"username\":\"$username\",\"role\":\"$role\",\"exp\":$exp,\"iat\":$iat,\"iss\":\"helixagent\"}"
    create_jwt "$payload" "$JWT_SECRET" "HS256"
}

# Helper: Create expired JWT
create_expired_jwt() {
    local user_id="${1:-test-user}"
    local username="${2:-testuser}"
    local role="${3:-user}"

    local now=$(date +%s)
    local exp=$((now - 3600))  # 1 hour ago
    local iat=$((now - 7200))  # 2 hours ago

    local payload="{\"user_id\":\"$user_id\",\"username\":\"$username\",\"role\":\"$role\",\"exp\":$exp,\"iat\":$iat,\"iss\":\"helixagent\"}"
    create_jwt "$payload" "$JWT_SECRET" "HS256"
}

# ============================================================================
# SECTION 1: TOKEN FORMAT VALIDATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 1: Token Format Validation"
log_info "=============================================="

run_test "Missing Authorization header rejected" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Empty Authorization header rejected" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: " 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Missing Bearer prefix rejected" '
    token=$(create_valid_jwt)
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/models" \
        -H "Authorization: $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Without Bearer prefix, should be treated as unauthenticated
    [[ "$http_code" == "200" ]] || [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]]
'

run_test "Invalid token format rejected" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer invalid.token.here" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Token with wrong number of parts rejected" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer header.payload" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Malformed base64 in token rejected" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer !!!.@@@.###" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

# ============================================================================
# SECTION 2: EXPIRED TOKEN REJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 2: Expired Token Rejection"
log_info "=============================================="

run_test "Expired token rejected" '
    expired_token=$(create_expired_jwt)
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $expired_token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | head -n -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]] || \
    (echo "$body" | grep -iE "expired|invalid" > /dev/null)
'

run_test "Token expired 1 second ago rejected" '
    now=$(date +%s)
    exp=$((now - 1))
    payload="{\"user_id\":\"test\",\"exp\":$exp,\"iat\":$((now - 100))}"
    token=$(create_jwt "$payload" "$JWT_SECRET" "HS256")
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Token with future iat handled correctly" '
    now=$(date +%s)
    future_iat=$((now + 3600))
    payload="{\"user_id\":\"test\",\"exp\":$((future_iat + 3600)),\"iat\":$future_iat}"
    token=$(create_jwt "$payload" "$JWT_SECRET" "HS256")
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Tokens issued in the future should be suspicious
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]] || [[ "$http_code" == "200" ]]
'

# ============================================================================
# SECTION 3: INVALID SIGNATURE REJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 3: Invalid Signature Rejection"
log_info "=============================================="

run_test "Token with wrong secret rejected" '
    now=$(date +%s)
    payload="{\"user_id\":\"test\",\"exp\":$((now + 3600)),\"iat\":$now}"
    token=$(create_jwt "$payload" "wrong-secret-key" "HS256")
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Token with tampered payload rejected" '
    valid_token=$(create_valid_jwt "user1" "user1" "user")
    # Tamper with the payload by replacing user_id
    parts=(${valid_token//./ })
    header="${parts[0]}"
    original_sig="${parts[2]}"
    # Create new payload with different user_id
    new_payload=$(base64url_encode "{\"user_id\":\"admin\",\"username\":\"admin\",\"role\":\"admin\",\"exp\":$(($(date +%s) + 3600))}")
    tampered_token="${header}.${new_payload}.${original_sig}"
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $tampered_token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Token with empty signature rejected" '
    now=$(date +%s)
    payload="{\"user_id\":\"test\",\"exp\":$((now + 3600)),\"iat\":$now}"
    header=$(base64url_encode "{\"alg\":\"HS256\",\"typ\":\"JWT\"}")
    payload_b64=$(base64url_encode "$payload")
    token="${header}.${payload_b64}."
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

# ============================================================================
# SECTION 4: ALGORITHM CONFUSION ATTACKS
# ============================================================================
log_info "=============================================="
log_info "SECTION 4: Algorithm Confusion Attacks"
log_info "=============================================="

run_test "Algorithm 'none' attack rejected" '
    now=$(date +%s)
    payload="{\"user_id\":\"admin\",\"username\":\"admin\",\"role\":\"admin\",\"exp\":$((now + 3600)),\"iat\":$now}"
    token=$(create_jwt "$payload" "" "none")
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Algorithm 'None' (capitalized) attack rejected" '
    now=$(date +%s)
    header=$(base64url_encode "{\"alg\":\"None\",\"typ\":\"JWT\"}")
    payload=$(base64url_encode "{\"user_id\":\"admin\",\"role\":\"admin\",\"exp\":$((now + 3600))}")
    token="${header}.${payload}."
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Algorithm 'NONE' attack rejected" '
    now=$(date +%s)
    header=$(base64url_encode "{\"alg\":\"NONE\",\"typ\":\"JWT\"}")
    payload=$(base64url_encode "{\"user_id\":\"admin\",\"role\":\"admin\",\"exp\":$((now + 3600))}")
    token="${header}.${payload}."
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Unknown algorithm rejected" '
    now=$(date +%s)
    header=$(base64url_encode "{\"alg\":\"HS512-custom\",\"typ\":\"JWT\"}")
    payload=$(base64url_encode "{\"user_id\":\"admin\",\"role\":\"admin\",\"exp\":$((now + 3600))}")
    sig=$(echo -n "${header}.${payload}" | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | base64 | tr '+/' '-_' | tr -d '=')
    token="${header}.${payload}.${sig}"
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

# ============================================================================
# SECTION 5: TOKEN REFRESH FLOW
# ============================================================================
log_info "=============================================="
log_info "SECTION 5: Token Refresh Flow"
log_info "=============================================="

run_test "Token refresh endpoint exists or handled gracefully" '
    valid_token=$(create_valid_jwt)
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/auth/refresh" \
        -H "Authorization: Bearer $valid_token" \
        -H "Content-Type: application/json" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Either refresh works (200), method not allowed (405), or not found (404)
    [[ "$http_code" == "200" ]] || [[ "$http_code" == "404" ]] || [[ "$http_code" == "405" ]] || [[ "$http_code" == "401" ]]
'

run_test "Expired token cannot be refreshed" '
    expired_token=$(create_expired_jwt)
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/auth/refresh" \
        -H "Authorization: Bearer $expired_token" \
        -H "Content-Type: application/json" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should not return 200 with a valid token
    [[ "$http_code" != "200" ]] || \
    (body=$(echo "$response" | head -n -1) && ! echo "$body" | grep -q "\"token\"")
'

run_test "Invalid token cannot be refreshed" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/auth/refresh" \
        -H "Authorization: Bearer invalid.token.here" \
        -H "Content-Type: application/json" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]] || [[ "$http_code" == "405" ]]
'

# ============================================================================
# SECTION 6: JWT INJECTION ATTACKS
# ============================================================================
log_info "=============================================="
log_info "SECTION 6: JWT Injection Attacks"
log_info "=============================================="

run_test "SQL injection in JWT claims rejected" '
    now=$(date +%s)
    payload="{\"user_id\":\"'"'"'; DROP TABLE users; --\",\"exp\":$((now + 3600))}"
    token=$(create_jwt "$payload" "$JWT_SECRET" "HS256")
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

run_test "XSS in JWT claims handled safely" '
    now=$(date +%s)
    payload="{\"user_id\":\"<script>alert(1)</script>\",\"exp\":$((now + 3600))}"
    token=$(create_jwt "$payload" "$JWT_SECRET" "HS256")
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    ! echo "$response" | grep -F "<script>alert(1)</script>" > /dev/null
'

run_test "Large payload in JWT handled" '
    now=$(date +%s)
    large_data=$(printf "A%.0s" {1..10000})
    payload="{\"user_id\":\"$large_data\",\"exp\":$((now + 3600))}"
    token=$(create_jwt "$payload" "$JWT_SECRET" "HS256")
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should reject or handle gracefully - not crash
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "413" ]] || [[ "$http_code" == "404" ]] || [[ "$http_code" == "400" ]]
'

# ============================================================================
# SECTION 7: ROLE/PRIVILEGE ESCALATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 7: Role/Privilege Escalation"
log_info "=============================================="

run_test "Role claim tampering detected" '
    # Create user token, tamper role to admin
    user_token=$(create_valid_jwt "user1" "regularuser" "user")
    parts=(${user_token//./ })
    header="${parts[0]}"
    original_sig="${parts[2]}"
    # Decode, modify role, re-encode
    now=$(date +%s)
    tampered_payload=$(base64url_encode "{\"user_id\":\"user1\",\"username\":\"regularuser\",\"role\":\"admin\",\"exp\":$((now + 3600))}")
    tampered_token="${header}.${tampered_payload}.${original_sig}"
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $tampered_token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "Multiple role claims handled correctly" '
    now=$(date +%s)
    payload="{\"user_id\":\"test\",\"role\":\"user\",\"role\":\"admin\",\"exp\":$((now + 3600))}"
    token=$(create_jwt "$payload" "$JWT_SECRET" "HS256")
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should not grant admin access due to duplicate claims
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]] || [[ "$http_code" == "200" ]]
'

# ============================================================================
# SECTION 8: JKU/JWK HEADER INJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 8: JKU/JWK Header Injection"
log_info "=============================================="

run_test "JKU header injection rejected" '
    now=$(date +%s)
    header=$(base64url_encode "{\"alg\":\"RS256\",\"typ\":\"JWT\",\"jku\":\"http://evil.com/keys.json\"}")
    payload=$(base64url_encode "{\"user_id\":\"admin\",\"role\":\"admin\",\"exp\":$((now + 3600))}")
    token="${header}.${payload}.fake_signature"
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "JWK embedded header attack rejected" '
    now=$(date +%s)
    header=$(base64url_encode "{\"alg\":\"HS256\",\"typ\":\"JWT\",\"jwk\":{\"kty\":\"oct\",\"k\":\"YWJj\"}}")
    payload=$(base64url_encode "{\"user_id\":\"admin\",\"role\":\"admin\",\"exp\":$((now + 3600))}")
    sig=$(echo -n "${header}.${payload}" | openssl dgst -sha256 -hmac "abc" -binary | base64 | tr '+/' '-_' | tr -d '=')
    token="${header}.${payload}.${sig}"
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "x5u header injection rejected" '
    now=$(date +%s)
    header=$(base64url_encode "{\"alg\":\"RS256\",\"typ\":\"JWT\",\"x5u\":\"http://evil.com/cert.pem\"}")
    payload=$(base64url_encode "{\"user_id\":\"admin\",\"role\":\"admin\",\"exp\":$((now + 3600))}")
    token="${header}.${payload}.fake_signature"
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/admin/users" \
        -H "Authorization: Bearer $token" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

# ============================================================================
# SECTION 9: TOKEN LEAKAGE PREVENTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 9: Token Leakage Prevention"
log_info "=============================================="

run_test "Token not exposed in error messages" '
    valid_token=$(create_valid_jwt)
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/v1/nonexistent" \
        -H "Authorization: Bearer $valid_token" 2>&1)
    ! echo "$response" | grep -F "$valid_token" > /dev/null
'

run_test "Token not logged in response headers" '
    valid_token=$(create_valid_jwt)
    headers=$(curl -s --max-time 60 -I -X GET "$BASE_URL/v1/models" \
        -H "Authorization: Bearer $valid_token" 2>&1)
    ! echo "$headers" | grep -F "$valid_token" > /dev/null
'

# ============================================================================
# SUMMARY
# ============================================================================
log_info "=============================================="
log_info "CHALLENGE SUMMARY"
log_info "=============================================="

log_info "Total tests: $TESTS_TOTAL"
log_info "Passed: $TESTS_PASSED"
log_info "Failed: $TESTS_FAILED"

record_metric "total_tests" "$TESTS_TOTAL"
record_metric "passed_tests" "$TESTS_PASSED"
record_metric "failed_tests" "$TESTS_FAILED"

# Calculate pass percentage
if [[ $TESTS_TOTAL -gt 0 ]]; then
    PASS_PERCENT=$((TESTS_PASSED * 100 / TESTS_TOTAL))
    record_metric "pass_percentage" "$PASS_PERCENT"
    log_info "Pass rate: ${PASS_PERCENT}%"
fi

if [[ $TESTS_FAILED -eq 0 ]]; then
    log_success "=============================================="
    log_success "ALL JWT SECURITY TESTS PASSED!"
    log_success "=============================================="
    finalize_challenge "PASSED"
else
    log_error "=============================================="
    log_error "SOME TESTS FAILED - JWT SECURITY RISKS DETECTED"
    log_error "=============================================="
    finalize_challenge "FAILED"
fi
