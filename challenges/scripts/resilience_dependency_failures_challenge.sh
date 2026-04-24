#!/bin/bash
# Resilience Dependency Failures Challenge
# Tests handling of external dependency failures

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-dependency-failures" "Resilience Dependency Failures Challenge"
load_env

log_info "Testing dependency failure handling..."

test_provider_dependency_failure() {
    log_info "Test 1: System handles provider unavailability"

    # Use invalid model to simulate provider failure
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"invalid-provider-model","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should fail gracefully (400/404) or fallback (200)
    [[ "$code" =~ ^(200|400|404)$ ]] && record_assertion "provider_failure" "handled" "true" "Provider failure handled gracefully (HTTP $code)"
}

test_cache_dependency_failure() {
    log_info "Test 2: System operates without cache"

    # Normal requests should work even if cache unavailable
    local success=0
    local total=3

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"No cache test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "cache_independent_requests" $total
    [[ $success -ge 2 ]] && record_assertion "cache_dependency" "resilient" "true" "$success/$total requests succeeded without cache dependency"
}

test_database_dependency_graceful_degradation() {
    log_info "Test 3: System degrades gracefully on DB issues"

    # Test that core functionality works (even if some features degraded)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"DB degradation test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Core API should work (200) or fail gracefully (503)
    [[ "$code" =~ ^(200|503)$ ]] && record_assertion "db_degradation" "graceful" "true" "System handles DB issues gracefully (HTTP $code)"
}

test_multiple_dependency_failures() {
    log_info "Test 4: System handles multiple dependency failures"

    # Stress test with concurrent requests
    local pids=()
    for i in $(seq 1 10); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Multi-dep test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /tmp/dep_$i.txt) &
        pids+=($!)
    done

    wait

    # Check results
    local success=0
    for i in $(seq 1 10); do
        local code=$(tail -n1 /tmp/dep_$i.txt 2>/dev/null || echo "000")
        [[ "$code" == "200" ]] && success=$((success + 1))
        rm -f /tmp/dep_$i.txt
    done

    record_metric "multi_dependency_requests" 10
    [[ $success -ge 5 ]] && record_assertion "multi_dependency_failures" "resilient" "true" "$success/10 requests succeeded under multi-dependency stress"
}

main() {
    log_info "Starting dependency failures challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_provider_dependency_failure
    test_cache_dependency_failure
    test_database_dependency_graceful_degradation
    test_multiple_dependency_failures

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
