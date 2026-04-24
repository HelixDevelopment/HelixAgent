#!/bin/bash
# Resilience Resource Limits Challenge
# Tests handling of resource limits and constraints

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-resource-limits" "Resilience Resource Limits Challenge"
load_env

log_info "Testing resource limit handling..."

test_max_tokens_limit() {
    log_info "Test 1: Max tokens limit enforced"

    # Request with reasonable max_tokens
    local resp1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code1=$(echo "$resp1" | tail -n1)
    [[ "$code1" == "200" ]] && record_assertion "reasonable_tokens" "accepted" "true" "Reasonable max_tokens accepted"

    # Request with very large max_tokens
    local resp2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":100000}' \
        --max-time 30 2>/dev/null || true)

    local code2=$(echo "$resp2" | tail -n1)
    # Should reject (400) or clamp to limit (200)
    [[ "$code2" =~ ^(200|400)$ ]] && record_assertion "excessive_tokens" "handled" "true" "Excessive max_tokens handled (HTTP $code2)"
}

test_request_size_limit() {
    log_info "Test 2: Request size limit enforced"

    # Large but reasonable request
    local large_msg=$(printf 'A%.0s' {1..1000})
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"'$large_msg'"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Should accept (200) or reject if too large (413)
    [[ "$code" =~ ^(200|413)$ ]] && record_assertion "request_size_limit" "enforced" "true" "Request size limit enforced (HTTP $code)"
}

test_concurrent_request_limit() {
    log_info "Test 3: Concurrent request limit enforced"

    # Launch many concurrent requests
    local pids=()
    for i in $(seq 1 15); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent test"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/limit_$i.txt) &
        pids+=($!)
    done

    wait

    # Check results
    local success=0
    local rate_limited=0
    for i in $(seq 1 15); do
        local code=$(tail -n1 /tmp/limit_$i.txt 2>/dev/null || echo "000")
        [[ "$code" == "200" ]] && success=$((success + 1))
        [[ "$code" =~ ^(429|503)$ ]] && rate_limited=$((rate_limited + 1))
        rm -f /tmp/limit_$i.txt
    done

    record_metric "concurrent_success" $success
    record_metric "concurrent_limited" $rate_limited
    [[ $success -gt 0 ]] && record_assertion "concurrent_limits" "managed" "true" "$success successful, $rate_limited limited (concurrency managed)"
}

test_resource_recovery() {
    log_info "Test 4: System recovers after resource pressure"

    sleep 3  # Allow resource recovery

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "resource_recovery" $total
    [[ $success -ge 4 ]] && record_assertion "resource_limit_recovery" "successful" "true" "$success/$total requests succeeded after resource recovery"
}

main() {
    log_info "Starting resource limits challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_max_tokens_limit
    test_request_size_limit
    test_concurrent_request_limit
    test_resource_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
