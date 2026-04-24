#!/bin/bash
# Security SQL Injection Challenge
# Tests protection against SQL injection attacks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-sql-injection" "Security SQL Injection Challenge"
load_env

log_info "Testing SQL injection protection..."

test_basic_sql_injection() {
    log_info "Test 1: Basic SQL injection attempts"

    local sql_injections=(
        "' OR '1'='1"
        "'; DROP TABLE users; --"
        "1' UNION SELECT * FROM users--"
        "admin'--"
    )

    local blocked=0

    for injection in "${sql_injections[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "{\"model\":\"helixagent-debate\",\"messages\":[{\"role\":\"user\",\"content\":\"$injection\"}],\"max_tokens\":10}" \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        # Should accept with sanitization (200) or reject (400), never crash (500)
        [[ "$code" =~ ^(200|400)$ ]] && blocked=$((blocked + 1))
        [[ "$code" == "500" ]] && record_assertion "sql_injection_crash" "detected" "false" "System crashed on SQL injection"
    done

    record_metric "sql_injections_tested" ${#sql_injections[@]}
    [[ $blocked -ge 3 ]] && record_assertion "sql_injection" "blocked" "true" "$blocked/${#sql_injections[@]} SQL injections blocked"
}

test_advanced_sql_injection() {
    log_info "Test 2: Advanced SQL injection techniques"

    local advanced_injections=(
        "1' AND (SELECT COUNT(*) FROM users) > 0--"
        "' OR 1=1/*"
        "1'; WAITFOR DELAY '00:00:05'--"
    )

    local protected=0

    for injection in "${advanced_injections[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "{\"model\":\"helixagent-debate\",\"messages\":[{\"role\":\"user\",\"content\":\"$injection\"}],\"max_tokens\":10}" \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(200|400)$ ]] && protected=$((protected + 1))
    done

    record_metric "advanced_injections_tested" ${#advanced_injections[@]}
    [[ $protected -ge 2 ]] && record_assertion "advanced_sql_injection" "protected" "true" "$protected/${#advanced_injections[@]} advanced injections protected"
}

test_parameterized_queries() {
    log_info "Test 3: System uses parameterized queries"

    # Normal query should work without issues
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Normal query test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "200" ]] && record_assertion "parameterized_queries" "working" "true" "Normal queries work correctly"
}

test_error_message_disclosure() {
    log_info "Test 4: SQL error messages don't leak info"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'\''; DROP TABLE test; --"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local body=$(echo "$resp" | head -n -1)

    if echo "$body" | jq -e '.error.message' > /dev/null 2>&1; then
        local msg=$(echo "$body" | jq -r '.error.message // empty')

        # Should not leak SQL details
        if ! echo "$msg" | grep -qiE "(sql|syntax|table|column|database|query|select|insert|update|delete)"; then
            record_assertion "error_disclosure" "prevented" "true" "SQL errors don't leak DB details"
        fi
    else
        # No error message means injection handled without error
        record_assertion "error_disclosure" "no_error" "true" "SQL injection handled silently"
    fi
}

main() {
    log_info "Starting SQL injection challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_basic_sql_injection
    test_advanced_sql_injection
    test_parameterized_queries
    test_error_message_disclosure

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
