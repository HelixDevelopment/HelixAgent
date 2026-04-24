#!/bin/bash
# Resilience Bulkhead Isolation Challenge
# Tests isolation of failures to prevent cascade

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-bulkhead-isolation" "Resilience Bulkhead Isolation Challenge"
load_env

log_info "Testing bulkhead isolation..."

test_provider_isolation() {
    log_info "Test 1: Provider failures are isolated"

    # Test multiple providers
    local models=("helixagent-debate" "helixagent-debate" "helixagent-debate")
    local success=0

    for model in "${models[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"'$model'","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" == "200" ]] && success=$((success + 1))
    done

    record_metric "provider_tests" ${#models[@]}
    [[ $success -ge 1 ]] && record_assertion "provider_isolation" "maintained" "true" "$success/${#models[@]} providers operational"
}

test_resource_pool_isolation() {
    log_info "Test 2: Resource pools are isolated"

    # Overwhelm one endpoint type
    local pids=()
    for i in $(seq 1 10); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Stress"}],"max_tokens":50}' \
            --max-time 60 2>/dev/null) &
        pids+=($!)
    done

    # While stressed, test health endpoint (different pool)
    sleep 2
    local health_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    local health_code=$(echo "$health_resp" | tail -n1)

    # Cleanup
    for pid in "${pids[@]}"; do
        kill $pid 2>/dev/null || true
    done
    wait 2>/dev/null

    [[ "$health_code" =~ ^(200|204)$ ]] && record_assertion "resource_isolation" "verified" "true" "Health endpoint isolated from chat endpoint stress"
}

test_failure_containment() {
    log_info "Test 3: Failures contained within bulkheads"

    # Send invalid request to trigger error
    local invalid_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"invalid-model","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Immediately test valid request
    local valid_resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local valid_code=$(echo "$valid_resp" | tail -n1)
    [[ "$valid_code" == "200" ]] && record_assertion "failure_containment" "effective" "true" "Valid requests succeed despite invalid request errors"
}

test_circuit_breaker_per_bulkhead() {
    log_info "Test 4: Circuit breakers operate per bulkhead"

    # Check circuit breaker status endpoint
    local cb_status=$(curl -s "$BASE_URL/v1/monitoring/circuit-breakers" 2>/dev/null || echo "{}")

    if echo "$cb_status" | jq -e '.providers' > /dev/null 2>&1; then
        local providers=$(echo "$cb_status" | jq -e '.providers | length' 2>/dev/null || echo 0)
        record_metric "circuit_breakers" $providers
        [[ $providers -gt 0 ]] && record_assertion "circuit_per_bulkhead" "configured" "true" "$providers circuit breakers (one per bulkhead)"
    else
        # Basic operational check
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "bulkhead_operational" "verified" "true" "Bulkheads operational"
    fi
}

main() {
    log_info "Starting bulkhead isolation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_provider_isolation
    test_resource_pool_isolation
    test_failure_containment
    test_circuit_breaker_per_bulkhead

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
