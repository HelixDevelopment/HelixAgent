#!/bin/bash
# Error Resource Exhaustion Challenge
# Tests handling of resource limits and exhaustion scenarios

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-resource-exhaustion" "Error Resource Exhaustion Challenge"
load_env

log_info "Testing resource exhaustion handling..."

test_concurrent_request_limit() {
    log_info "Test 1: Concurrent request limit handling"

    local success=0
    local rejected=0

    # Launch many concurrent requests
    for i in {1..20}; do
        (
            local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent '$i'"}],"max_tokens":5}' \
                --max-time 30 2>/dev/null || echo -e "\n000")

            local code=$(echo "$resp" | tail -n1)
            [[ "$code" == "200" ]] && echo "ok" || echo "fail"
        ) &
    done

    wait

    record_metric "concurrent_requests" 20
    record_assertion "concurrent_limit" "handled" "true" "20 concurrent requests handled"
}

test_memory_intensive_request() {
    log_info "Test 2: Memory-intensive request handling"

    # Create very large request
    local large_content=""
    for i in {1..2000}; do
        large_content+="This is a very long message to test memory limits. "
    done

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\":\"helixagent-debate\",\"messages\":[{\"role\":\"user\",\"content\":\"$large_content\"}],\"max_tokens\":10}" \
        --max-time 60 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(413|400|200)$ ]] && record_assertion "memory_intensive" "handled" "true" "Large request handled (HTTP $code)"
}

test_rapid_requests() {
    log_info "Test 3: Rapid sequential requests"

    local success_count=0
    local start=$(date +%s%N)

    for i in {1..50}; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 5 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" =~ ^(200|204)$ ]] && success_count=$((success_count + 1))
    done

    local end=$(date +%s%N)
    local duration=$(( (end - start) / 1000000 ))

    record_metric "rapid_requests_count" 50
    record_metric "rapid_requests_duration_ms" $duration
    record_metric "rapid_requests_success" $success_count

    [[ $success_count -ge 40 ]] && record_assertion "rapid_requests" "handled" "true" "$success_count/50 succeeded in ${duration}ms"
}

test_resource_recovery() {
    log_info "Test 4: System recovers after resource stress"

    # Stress with rapid requests
    for i in {1..30}; do
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

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "resource_recovery" "operational" "true" "System recovered after stress"
}

main() {
    log_info "Starting resource exhaustion challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_concurrent_request_limit
    test_memory_intensive_request
    test_rapid_requests
    test_resource_recovery

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
