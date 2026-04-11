#!/bin/bash
# HelixAgent Challenge - SQL Injection Prevention
# Validates that the API properly prevents SQL injection attacks through input sanitization
# and parameterized queries.

set -e

# Source challenge framework
source "$(dirname "${BASH_SOURCE[0]}")/challenge_framework.sh"

# Initialize challenge
init_challenge "sql-injection" "SQL Injection Prevention Challenge"
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

# Helper: Send payload to completions endpoint and check response
test_sql_injection_payload() {
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

    # Check for SQL error indicators in response
    local sql_errors="sql syntax|mysql_fetch|ora-|oracle|sqlserver|postgresql|sqlite_|sql error|database error|query error|syntax error near|unexpected token"

    if echo "$response" | grep -iE "$sql_errors" > /dev/null 2>&1; then
        log_error "SQL error detected in response for payload: $test_name"
        return 1
    fi

    # Response should not contain obvious SQL execution results
    if echo "$response" | grep -iE "uid=[0-9]+|gid=[0-9]+|root:x:|/etc/passwd|table dropped|rows affected" > /dev/null 2>&1; then
        log_error "Potential SQL execution detected for payload: $test_name"
        return 1
    fi

    return 0
}

# Helper: Test SQL injection in query parameters
test_sql_injection_query_param() {
    local endpoint="$1"
    local param="$2"
    local payload="$3"

    local response
    response=$(curl -s --max-time 60 -X GET "$BASE_URL$endpoint?$param=$(echo "$payload" | jq -sRr @uri)" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>&1)

    local sql_errors="sql syntax|mysql_fetch|ora-|oracle|sqlserver|postgresql|sqlite_|sql error|database error"

    if echo "$response" | grep -iE "$sql_errors" > /dev/null 2>&1; then
        return 1
    fi

    return 0
}

# ============================================================================
# SECTION 1: BASIC SQL INJECTION PAYLOADS
# ============================================================================
log_info "=============================================="
log_info "SECTION 1: Basic SQL Injection Payloads"
log_info "=============================================="

# Classic SQL injection payloads
BASIC_PAYLOADS=(
    "' OR '1'='1"
    "' OR '1'='1' --"
    "' OR '1'='1' /*"
    "1' OR '1'='1"
    "admin'--"
    "admin' /*"
    "' OR 1=1--"
    "' OR 1=1#"
    ") OR (1=1 --"
    "'; --"
)

for i in "${!BASIC_PAYLOADS[@]}"; do
    payload="${BASIC_PAYLOADS[$i]}"
    run_test "Basic SQL Injection #$((i+1))" "test_sql_injection_payload '$payload' 'Basic #$((i+1))'"
done

# ============================================================================
# SECTION 2: UNION-BASED SQL INJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 2: Union-Based SQL Injection"
log_info "=============================================="

UNION_PAYLOADS=(
    "' UNION SELECT * FROM users --"
    "' UNION SELECT 1,2,3,4,5 --"
    "' UNION ALL SELECT NULL,NULL,NULL --"
    "1' UNION SELECT username, password FROM users --"
    "' UNION SELECT table_name,column_name FROM information_schema.columns --"
    "-1 UNION SELECT 1,2,3,4,5,6,7,8,9,10"
)

for i in "${!UNION_PAYLOADS[@]}"; do
    payload="${UNION_PAYLOADS[$i]}"
    run_test "Union SQL Injection #$((i+1))" "test_sql_injection_payload '$payload' 'Union #$((i+1))'"
done

# ============================================================================
# SECTION 3: STACKED QUERIES / BATCH INJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 3: Stacked Queries / Batch Injection"
log_info "=============================================="

STACKED_PAYLOADS=(
    "'; DROP TABLE users; --"
    "'; DELETE FROM users WHERE 1=1; --"
    "'; UPDATE users SET password='hacked' WHERE 1=1; --"
    "'; INSERT INTO users VALUES ('hacker', 'password'); --"
    "'; TRUNCATE TABLE users; --"
    "1; EXEC xp_cmdshell('whoami')--"
    "'; SHUTDOWN; --"
)

for i in "${!STACKED_PAYLOADS[@]}"; do
    payload="${STACKED_PAYLOADS[$i]}"
    run_test "Stacked Query #$((i+1))" "test_sql_injection_payload '$payload' 'Stacked #$((i+1))'"
done

# ============================================================================
# SECTION 4: BLIND SQL INJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 4: Blind SQL Injection"
log_info "=============================================="

BLIND_PAYLOADS=(
    "1' AND 1=1 --"
    "1' AND 1=2 --"
    "1' AND (SELECT COUNT(*) FROM users) > 0 --"
    "1' AND SUBSTRING(username,1,1)='a' --"
    "1' AND ASCII(SUBSTRING(password,1,1)) > 64 --"
    "1'; WAITFOR DELAY '0:0:5'--"
    "1' AND SLEEP(5) --"
    "1' AND BENCHMARK(10000000,SHA1('test'))--"
)

for i in "${!BLIND_PAYLOADS[@]}"; do
    payload="${BLIND_PAYLOADS[$i]}"
    run_test "Blind SQL Injection #$((i+1))" "test_sql_injection_payload '$payload' 'Blind #$((i+1))'"
done

# ============================================================================
# SECTION 5: ENCODED SQL INJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 5: Encoded SQL Injection"
log_info "=============================================="

ENCODED_PAYLOADS=(
    "%27%20OR%20%271%27%3D%271"
    "%27%3B%20DROP%20TABLE%20users%3B--"
    "&#39; OR &#39;1&#39;=&#39;1"
    "\\x27 OR \\x271\\x27=\\x271"
    "/**/UNION/**/SELECT/**/1,2,3"
    "UN/**/ION/**/SEL/**/ECT/**/1,2,3"
)

for i in "${!ENCODED_PAYLOADS[@]}"; do
    payload="${ENCODED_PAYLOADS[$i]}"
    run_test "Encoded SQL Injection #$((i+1))" "test_sql_injection_payload '$payload' 'Encoded #$((i+1))'"
done

# ============================================================================
# SECTION 6: NOSQL INJECTION (MongoDB style)
# ============================================================================
log_info "=============================================="
log_info "SECTION 6: NoSQL Injection Patterns"
log_info "=============================================="

NOSQL_PAYLOADS=(
    '{"$gt": ""}'
    '{"$ne": null}'
    '{"$where": "this.password == this.username"}'
    '{"$regex": ".*"}'
    "'; return true; var foo='"
    '{"$or": [{"a": 1}, {"b": 2}]}'
)

for i in "${!NOSQL_PAYLOADS[@]}"; do
    payload="${NOSQL_PAYLOADS[$i]}"
    run_test "NoSQL Injection #$((i+1))" "test_sql_injection_payload '$payload' 'NoSQL #$((i+1))'"
done

# ============================================================================
# SECTION 7: SQL INJECTION IN DIFFERENT CONTEXTS
# ============================================================================
log_info "=============================================="
log_info "SECTION 7: Context-Specific SQL Injection"
log_info "=============================================="

# Test in model field
run_test "SQL in model field" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"'"'"'; DROP TABLE models; --\", \"messages\": [{\"role\": \"user\", \"content\": \"test\"}]}")
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

# Test in temperature field (numeric injection)
run_test "Numeric field injection" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"test\", \"messages\": [{\"role\": \"user\", \"content\": \"test\"}], \"temperature\": \"1 OR 1=1\"}")
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

# Test in JSON array
run_test "SQL in JSON array" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"test\", \"messages\": [{\"role\": \"user\", \"content\": \"test\"}], \"stop\": [\"'"'"'; DROP TABLE stops; --\"]}")
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

# ============================================================================
# SECTION 8: SQL INJECTION VIA HEADERS
# ============================================================================
log_info "=============================================="
log_info "SECTION 8: SQL Injection via HTTP Headers"
log_info "=============================================="

run_test "SQL in User-Agent header" '
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/health" \
        -H "User-Agent: '"'"'; DROP TABLE users; --" 2>&1)
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

run_test "SQL in X-Forwarded-For header" '
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/health" \
        -H "X-Forwarded-For: 127.0.0.1'"'"'; DROP TABLE logs; --" 2>&1)
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

run_test "SQL in Referer header" '
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/health" \
        -H "Referer: http://example.com/'"'"'; SELECT * FROM users; --" 2>&1)
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

# ============================================================================
# SECTION 9: SECOND-ORDER SQL INJECTION
# ============================================================================
log_info "=============================================="
log_info "SECTION 9: Second-Order SQL Injection"
log_info "=============================================="

# Store payload first, then trigger
run_test "Second-order injection preparation" '
    # First request stores the payload
    curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"test\", \"messages\": [{\"role\": \"user\", \"content\": \"My name is admin'"'"'--\"}]}" > /dev/null 2>&1

    # Second request might trigger stored payload
    response=$(curl -s --max-time 60 -X GET "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>&1)
    ! echo "$response" | grep -iE "sql syntax|database error" > /dev/null
'

# ============================================================================
# SECTION 10: SQL INJECTION ERROR MESSAGES
# ============================================================================
log_info "=============================================="
log_info "SECTION 10: SQL Error Message Disclosure"
log_info "=============================================="

run_test "No SQL error details exposed" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"'"'"'; INVALID SQL SYNTAX; --\", \"messages\": [{\"role\": \"user\", \"content\": \"test\"}]}")
    # Should not expose database-specific error messages
    ! echo "$response" | grep -iE "mysql|postgresql|sqlite|mssql|oracle|mariadb|column|table|query|syntax" > /dev/null
'

run_test "No stack traces in error responses" '
    response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\": \"test'"'"'\", \"messages\": [{\"role\": \"user\", \"content\": \"test\"}]}")
    ! echo "$response" | grep -iE "at line|stack trace|file.*\.go|panic|runtime error" > /dev/null
'

# ============================================================================
# SECTION 11: PARAMETERIZED QUERY VERIFICATION
# ============================================================================
log_info "=============================================="
log_info "SECTION 11: Parameterized Query Verification"
log_info "=============================================="

# These should all be safely handled without SQL interpretation
SAFE_INPUT_TESTS=(
    "SELECT * FROM users WHERE id = 1"
    "DROP DATABASE test"
    "Robert'); DROP TABLE Students;--"
    "1; SELECT * FROM users"
)

for i in "${!SAFE_INPUT_TESTS[@]}"; do
    input="${SAFE_INPUT_TESTS[$i]}"
    run_test "Safe handling of SQL-like input #$((i+1))" '
        response=$(curl -s --max-time 60 -X POST "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "{\"model\": \"test\", \"messages\": [{\"role\": \"user\", \"content\": \"'"$input"'\"}]}")
        # Should not execute as SQL, just treat as text
        ! echo "$response" | grep -iE "sql syntax|rows affected|table dropped" > /dev/null
    '
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
    log_success "ALL SQL INJECTION PREVENTION TESTS PASSED!"
    log_success "=============================================="
    finalize_challenge "PASSED"
else
    log_error "=============================================="
    log_error "SOME TESTS FAILED - SQL INJECTION RISKS DETECTED"
    log_error "=============================================="
    finalize_challenge "FAILED"
fi
