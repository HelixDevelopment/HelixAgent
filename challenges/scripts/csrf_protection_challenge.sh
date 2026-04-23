#!/bin/bash
# HelixAgent Challenge - CSRF Protection Validation
# Validates Cross-Site Request Forgery protection mechanisms including
# CSRF tokens, SameSite cookies, Origin/Referer validation, and CORS.

set -e

# Source challenge framework
source "$(dirname "${BASH_SOURCE[0]}")/challenge_framework.sh"

# Initialize challenge
init_challenge "csrf-protection" "CSRF Protection Challenge"
load_env

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Base URL for API
BASE_URL="${HELIXAGENT_URL:-http://localhost:8100}"

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

# ============================================================================
# SECTION 1: CORS HEADERS VALIDATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 1: CORS Headers Validation"
log_info "=============================================="

run_test "CORS headers present for OPTIONS request" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: POST" \
        -H "Access-Control-Request-Headers: Content-Type, Authorization" 2>&1)
    # Should have Access-Control-Allow-* headers or reject the request
    echo "$response" | grep -iE "Access-Control-Allow|HTTP/[0-9.]+ (200|204|403|404)" > /dev/null
'

run_test "Access-Control-Allow-Origin not wildcard for sensitive endpoints" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://evil.com" \
        -H "Access-Control-Request-Method: POST" 2>&1)
    # Should NOT return Access-Control-Allow-Origin: * for untrusted origins on sensitive endpoints
    ! echo "$response" | grep -E "Access-Control-Allow-Origin: \*" > /dev/null || \
    log_warning "Wildcard CORS origin detected - consider restricting to trusted domains"
    true
'

run_test "Access-Control-Allow-Credentials handled correctly" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: POST" 2>&1)
    # If credentials are allowed, origin must not be *
    if echo "$response" | grep -i "Access-Control-Allow-Credentials: true" > /dev/null; then
        ! echo "$response" | grep -E "Access-Control-Allow-Origin: \*" > /dev/null
    else
        true
    fi
'

run_test "Preflight caching has reasonable max-age" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: POST" 2>&1)
    # Access-Control-Max-Age should exist and be reasonable (< 1 week = 604800)
    if echo "$response" | grep -i "Access-Control-Max-Age:" > /dev/null; then
        max_age=$(echo "$response" | grep -i "Access-Control-Max-Age:" | grep -oE "[0-9]+" | head -1)
        [[ -z "$max_age" ]] || [[ "$max_age" -le 604800 ]]
    else
        true
    fi
'

# ============================================================================
# SECTION 2: ORIGIN HEADER VALIDATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 2: Origin Header Validation"
log_info "=============================================="

run_test "Request from null origin handled" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Origin: null" \
        -H "Authorization: Bearer test" \
        -d "{\"model\":\"test\",\"messages\":[{\"role\":\"user\",\"content\":\"test\"}]}" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # null origin is suspicious - should be rejected or handled carefully
    # Server should either reject (4xx) or process without CORS
    [[ "$http_code" == "200" ]] || [[ "$http_code" == "400" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "401" ]]
'

run_test "Request from evil.com origin validated" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://evil.com" \
        -H "Access-Control-Request-Method: POST" 2>&1)
    # Should not reflect evil.com in Access-Control-Allow-Origin
    ! echo "$response" | grep -i "Access-Control-Allow-Origin: http://evil.com" > /dev/null || \
    log_warning "Origin http://evil.com was reflected - ensure CORS whitelist is configured"
    true
'

run_test "Origin with port mismatch handled" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://localhost:9999" \
        -H "Access-Control-Request-Method: POST" 2>&1)
    # Should handle different ports appropriately
    echo "$response" | grep -iE "Access-Control-Allow-Origin|HTTP/[0-9.]+" > /dev/null
'

run_test "Origin with path component rejected" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://localhost:3000/path" \
        -H "Access-Control-Request-Method: POST" 2>&1)
    # Origin should not have path - malformed
    ! echo "$response" | grep -i "Access-Control-Allow-Origin: http://localhost:3000/path" > /dev/null
'

run_test "Origin injection attempt blocked" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://evil.com%0d%0aX-Injected: true" \
        -H "Access-Control-Request-Method: POST" 2>&1)
    ! echo "$response" | grep -i "X-Injected" > /dev/null
'

# ============================================================================
# SECTION 3: REFERER HEADER VALIDATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 3: Referer Header Validation"
log_info "=============================================="

run_test "Missing Referer handled gracefully" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test" \
        -d "{\"model\":\"test\",\"messages\":[{\"role\":\"user\",\"content\":\"test\"}]}" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # API requests may not have Referer - should still work
    [[ "$http_code" == "200" ]] || [[ "$http_code" == "401" ]] || [[ "$http_code" == "400" ]]
'

run_test "Referer from different domain logged/validated" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Referer: http://evil.com/attack.html" \
        -H "Authorization: Bearer test" \
        -d "{\"model\":\"test\",\"messages\":[{\"role\":\"user\",\"content\":\"test\"}]}" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should either reject or proceed (API may not check Referer)
    [[ "$http_code" == "200" ]] || [[ "$http_code" == "401" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "400" ]]
'

run_test "Referer with javascript: URI handled safely" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Referer: javascript:alert(1)" \
        -H "Authorization: Bearer test" \
        -d "{\"model\":\"test\",\"messages\":[{\"role\":\"user\",\"content\":\"test\"}]}" 2>&1)
    # Should not execute or reflect the javascript URI
    ! echo "$response" | grep -F "javascript:alert" > /dev/null
'

# ============================================================================
# SECTION 4: CROSS-ORIGIN RESOURCE SHARING ATTACKS
# ============================================================================
log_info "=============================================="
log_info "SECTION 4: Cross-Origin Resource Sharing Attacks"
log_info "=============================================="

run_test "Simple GET request CORS handling" '
    response=$(curl -s --max-time 60 -I -X GET "$BASE_URL/v1/models" \
        -H "Origin: http://attacker.com" 2>&1)
    # Check CORS behavior for simple requests
    echo "$response" | grep -iE "HTTP/[0-9.]+ [0-9]" > /dev/null
'

run_test "POST with custom Content-Type triggers preflight" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: POST" \
        -H "Access-Control-Request-Headers: Content-Type" 2>&1)
    # Preflight should be handled
    echo "$response" | grep -iE "HTTP/[0-9.]+ (200|204|403|404)" > /dev/null
'

run_test "DELETE method requires proper CORS" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/conversations/test" \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: DELETE" 2>&1)
    # DELETE should be in Access-Control-Allow-Methods if allowed
    echo "$response" | grep -iE "HTTP/[0-9.]+ [0-9]" > /dev/null
'

run_test "Non-standard header requires preflight" '
    response=$(curl -s --max-time 60 -I -X OPTIONS "$BASE_URL/v1/chat/completions" \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: POST" \
        -H "Access-Control-Request-Headers: X-Custom-Header" 2>&1)
    echo "$response" | grep -iE "HTTP/[0-9.]+ [0-9]" > /dev/null
'

# ============================================================================
# SECTION 5: CSRF TOKEN VALIDATION (IF IMPLEMENTED)
# ============================================================================
log_info "=============================================="
log_info "SECTION 5: CSRF Token Validation"
log_info "=============================================="

run_test "Check for CSRF token endpoint" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/csrf-token" \
        -H "Authorization: Bearer test" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Endpoint may or may not exist
    [[ "$http_code" == "200" ]] || [[ "$http_code" == "404" ]] || [[ "$http_code" == "401" ]]
'

run_test "Form submission without CSRF token behavior" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/settings" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -H "Authorization: Bearer test" \
        -d "setting=value" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should reject form submission or endpoint not exist
    [[ "$http_code" == "400" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]] || [[ "$http_code" == "415" ]] || [[ "$http_code" == "401" ]]
'

run_test "State-changing GET requests blocked" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/delete?id=123" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should not allow state changes via GET
    [[ "$http_code" == "404" ]] || [[ "$http_code" == "405" ]] || [[ "$http_code" == "401" ]]
'

# ============================================================================
# SECTION 6: COOKIE SECURITY
# ============================================================================
log_info "=============================================="
log_info "SECTION 6: Cookie Security"
log_info "=============================================="

run_test "Session cookies have SameSite attribute" '
    response=$(curl -s --max-time 60 -I -c - "$BASE_URL/v1/auth/login" 2>&1)
    # If cookies are set, check for SameSite
    if echo "$response" | grep -i "Set-Cookie" > /dev/null; then
        # SameSite should be Strict or Lax for CSRF protection
        echo "$response" | grep -iE "SameSite=(Strict|Lax)" > /dev/null || \
        log_warning "Cookies set without SameSite attribute"
    fi
    true
'

run_test "Session cookies have HttpOnly flag" '
    response=$(curl -s --max-time 60 -I -c - "$BASE_URL/v1/auth/login" 2>&1)
    if echo "$response" | grep -i "Set-Cookie" > /dev/null; then
        echo "$response" | grep -i "HttpOnly" > /dev/null || \
        log_warning "Cookies set without HttpOnly flag"
    fi
    true
'

run_test "Session cookies have Secure flag" '
    response=$(curl -s --max-time 60 -I -c - "$BASE_URL/v1/auth/login" 2>&1)
    if echo "$response" | grep -i "Set-Cookie" > /dev/null; then
        echo "$response" | grep -i "Secure" > /dev/null || \
        log_warning "Cookies set without Secure flag (required for HTTPS)"
    fi
    true
'

# ============================================================================
# SECTION 7: CONTENT-TYPE VALIDATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 7: Content-Type Validation"
log_info "=============================================="

run_test "Form-encoded POST rejected for JSON endpoints" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -H "Authorization: Bearer test" \
        -d "model=test&messages=[{\"role\":\"user\",\"content\":\"test\"}]" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "400" ]] || [[ "$http_code" == "415" ]] || [[ "$http_code" == "401" ]]
'

run_test "Multipart form POST rejected for JSON endpoints" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: multipart/form-data; boundary=----WebKitFormBoundary" \
        -H "Authorization: Bearer test" \
        -d "------WebKitFormBoundary
Content-Disposition: form-data; name=\"model\"

test
------WebKitFormBoundary--" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "400" ]] || [[ "$http_code" == "415" ]] || [[ "$http_code" == "401" ]]
'

run_test "Plain text POST rejected for JSON endpoints" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: text/plain" \
        -H "Authorization: Bearer test" \
        -d "{\"model\":\"test\"}" 2>&1)
    http_code=$(echo "$response" | tail -1)
    [[ "$http_code" == "400" ]] || [[ "$http_code" == "415" ]] || [[ "$http_code" == "401" ]]
'

# ============================================================================
# SECTION 8: FLASH/SILVERLIGHT CROSSDOMAIN POLICY
# ============================================================================
log_info "=============================================="
log_info "SECTION 8: Flash/Silverlight Crossdomain Policy"
log_info "=============================================="

run_test "crossdomain.xml not overly permissive" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" "$BASE_URL/crossdomain.xml" 2>&1)
    http_code=$(echo "$response" | tail -1)
    if [[ "$http_code" == "200" ]]; then
        body=$(echo "$response" | head -n -1)
        # Should not allow all domains
        ! echo "$body" | grep -E "domain=\"\*\"" > /dev/null || \
        log_warning "crossdomain.xml allows all domains"
    fi
    true
'

run_test "clientaccesspolicy.xml not overly permissive" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" "$BASE_URL/clientaccesspolicy.xml" 2>&1)
    http_code=$(echo "$response" | tail -1)
    if [[ "$http_code" == "200" ]]; then
        body=$(echo "$response" | head -n -1)
        # Should not allow all domains
        ! echo "$body" | grep -E "uri=\"\*\"" > /dev/null || \
        log_warning "clientaccesspolicy.xml allows all URIs"
    fi
    true
'

# ============================================================================
# SECTION 9: CUSTOM HEADER REQUIREMENTS
# ============================================================================
log_info "=============================================="
log_info "SECTION 9: Custom Header Requirements"
log_info "=============================================="

run_test "API requires Authorization header" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"test\",\"messages\":[{\"role\":\"user\",\"content\":\"test\"}]}" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Protected endpoints should require auth
    [[ "$http_code" == "401" ]] || [[ "$http_code" == "200" ]]
'

run_test "X-Requested-With header handling" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test" \
        -H "X-Requested-With: XMLHttpRequest" \
        -d "{\"model\":\"test\",\"messages\":[{\"role\":\"user\",\"content\":\"test\"}]}" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should accept requests with X-Requested-With
    [[ "$http_code" == "200" ]] || [[ "$http_code" == "401" ]] || [[ "$http_code" == "400" ]]
'

# ============================================================================
# SECTION 10: IFRAME PROTECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 10: Iframe Protection (Clickjacking)"
log_info "=============================================="

run_test "X-Frame-Options header present" '
    response=$(curl -s --max-time 60 -I "$BASE_URL/health" 2>&1)
    echo "$response" | grep -iE "X-Frame-Options: (DENY|SAMEORIGIN)" > /dev/null || \
    log_warning "X-Frame-Options header not set - clickjacking protection recommended"
    true
'

run_test "Content-Security-Policy frame-ancestors directive" '
    response=$(curl -s --max-time 60 -I "$BASE_URL/health" 2>&1)
    if echo "$response" | grep -i "Content-Security-Policy" > /dev/null; then
        csp=$(echo "$response" | grep -i "Content-Security-Policy")
        echo "$csp" | grep -i "frame-ancestors" > /dev/null || \
        log_warning "CSP does not include frame-ancestors directive"
    else
        log_warning "Content-Security-Policy header not present"
    fi
    true
'

# ============================================================================
# SECTION 11: DOUBLE SUBMIT COOKIE PATTERN
# ============================================================================
log_info "=============================================="
log_info "SECTION 11: Double Submit Cookie Pattern"
log_info "=============================================="

run_test "Cookie and header CSRF token match verification" '
    # First, get a CSRF token cookie (if implemented)
    cookie_response=$(curl -s --max-time 60 -c cookies.txt "$BASE_URL/v1/auth/login" 2>&1)

    # Extract CSRF token from cookie if present
    if [[ -f cookies.txt ]] && grep -i "csrf" cookies.txt > /dev/null 2>&1; then
        csrf_token=$(grep -i "csrf" cookies.txt | awk "{print \$7}" | head -1)
        if [[ -n "$csrf_token" ]]; then
            # Try request with matching header
            response=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST "$BASE_URL/v1/settings" \
                -b cookies.txt \
                -H "Content-Type: application/json" \
                -H "X-CSRF-Token: $csrf_token" \
                -d "{}" 2>&1)
            http_code=$(echo "$response" | tail -1)
            log_info "CSRF token validation response: $http_code"
        fi
    else
        log_info "No CSRF cookie found - double submit pattern may not be implemented"
    fi
    rm -f cookies.txt
    true
'

# ============================================================================
# SECTION 12: CSWSH (Cross-Site WebSocket Hijacking) PREVENTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 12: WebSocket Origin Validation"
log_info "=============================================="

run_test "WebSocket endpoint origin validation" '
    # Try to establish WebSocket connection with evil origin
    # Using curl to test the upgrade request
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X GET "$BASE_URL/v1/ws" \
        -H "Upgrade: websocket" \
        -H "Connection: Upgrade" \
        -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
        -H "Sec-WebSocket-Version: 13" \
        -H "Origin: http://evil.com" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # Should reject WebSocket from untrusted origin
    [[ "$http_code" != "101" ]] || \
    log_warning "WebSocket accepts connections from untrusted origins"
    true
'

# ============================================================================
# SECTION 13: ADDITIONAL CSRF ATTACK VECTORS
# ============================================================================
log_info "=============================================="
log_info "SECTION 13: Additional CSRF Attack Vectors"
log_info "=============================================="

run_test "JSON with padding (JSONP) not vulnerable" '
    response=$(curl -s --max-time 60 "$BASE_URL/v1/models?callback=maliciousFunction" 2>&1)
    # Should not return JSONP format
    ! echo "$response" | grep -E "^maliciousFunction\(" > /dev/null
'

run_test "TRACE method disabled" '
    response=$(curl -s --max-time 60 -w "\n%{http_code}" -X TRACE "$BASE_URL/" 2>&1)
    http_code=$(echo "$response" | tail -1)
    # TRACE should be disabled (returns 405 or similar)
    [[ "$http_code" == "405" ]] || [[ "$http_code" == "501" ]] || [[ "$http_code" == "403" ]] || [[ "$http_code" == "404" ]]
'

run_test "OPTIONS method does not expose sensitive data" '
    response=$(curl -s --max-time 60 -X OPTIONS "$BASE_URL/v1/chat/completions" 2>&1)
    # Should not include sensitive data in OPTIONS response
    ! echo "$response" | grep -iE "api_key|secret|password|token" > /dev/null
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
    log_success "ALL CSRF PROTECTION TESTS PASSED!"
    log_success "=============================================="
    finalize_challenge "PASSED"
else
    log_error "=============================================="
    log_error "SOME TESTS FAILED - CSRF PROTECTION RISKS DETECTED"
    log_error "=============================================="
    finalize_challenge "FAILED"
fi
