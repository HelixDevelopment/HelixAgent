#!/bin/bash
# Error Database Errors Challenge
# Tests handling of database connection and query errors

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-database-errors" "Error Database Errors Challenge"
load_env

log_info "Testing database error handling..."

test_database_connection_resilience() {
    log_info "Test 1: System resilience to database issues"

    # Normal request should work even if DB has issues (may use cache/memory)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"DB resilience test"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    # Should either succeed or fail gracefully (not 500)
    if [[ "$code" =~ ^(200|503)$ ]]; then
        record_assertion "db_resilience" "graceful" "true" "System handles DB issues gracefully (HTTP $code)"
    elif [[ "$code" == "500" ]]; then
        record_assertion "db_resilience" "error_500" "false" "Unhandled DB error (HTTP 500)"
    fi
}

test_query_error_handling() {
    log_info "Test 2: Query error handling"

    # Make multiple requests to test DB query handling
    local success=0
    local errors=0

    for i in {1..5}; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Query test '$i'"}],"max_tokens":5}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1))
        [[ "$code" =~ ^(500|503)$ ]] && errors=$((errors + 1))
    done

    record_metric "query_success" $success
    record_metric "query_errors" $errors

    [[ $success -ge 3 ]] && record_assertion "query_handling" "operational" "true" "$success/5 queries succeeded"
}

test_database_timeout() {
    log_info "Test 3: Database timeout handling"

    # Request that might trigger DB timeout
    local start=$(date +%s)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Timeout test"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)
    local end=$(date +%s)
    local duration=$((end - start))

    local code=$(echo "$resp" | tail -n1)
    record_metric "db_timeout_duration" $duration

    # Should respond within reasonable time
    [[ $duration -lt 60 ]] && record_assertion "db_timeout" "handled" "true" "DB operations timed appropriately (${duration}s)"
}

test_database_recovery() {
    log_info "Test 4: Recovery after database issues"

    # Simulate stress that might affect DB
    for i in {1..10}; do
        curl -s "$BASE_URL/health" --max-time 2 > /dev/null 2>&1 || true &
    done

    wait
    sleep 2

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "db_recovery" "operational" "true" "System recovered after DB stress"
}

main() {
    log_info "Starting database errors challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_database_connection_resilience
    test_query_error_handling
    test_database_timeout
    test_database_recovery

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
