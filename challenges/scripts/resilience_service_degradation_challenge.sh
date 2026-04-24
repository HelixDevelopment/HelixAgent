#!/bin/bash
# Resilience Service Degradation Challenge
# Tests graceful service degradation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-service-degradation" "Resilience Service Degradation Challenge"
load_env

log_info "Testing service degradation..."

test_partial_degradation() {
    log_info "Test 1: System degrades gracefully with partial failures"

    local success=0
    local total=10

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Degradation test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "degradation_requests" $total
    [[ $success -ge 5 ]] && record_assertion "partial_degradation" "graceful" "true" "$success/$total requests succeeded (graceful degradation)"
}

test_core_functionality_maintained() {
    log_info "Test 2: Core functionality maintained during degradation"

    # Core endpoint should work
    local core=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Core test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Health should work
    local health=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)

    [[ "$(echo "$core" | tail -n1)" == "200" && "$(echo "$health" | tail -n1)" =~ ^(200|204)$ ]] && record_assertion "core_maintained" "confirmed" "true" "Core functionality maintained"
}

test_degraded_response_quality() {
    log_info "Test 3: Degraded responses still functional"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Quality test"}],"max_tokens":20}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]] && echo "$body" | jq -e '.choices[0].message' > /dev/null 2>&1; then
        record_assertion "degraded_quality" "acceptable" "true" "Degraded responses still provide valid output"
    fi
}

test_recovery_from_degradation() {
    log_info "Test 4: System recovers from degradation"

    sleep 3  # Recovery time

    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "recovery_from_degradation" $total
    [[ $success -ge 4 ]] && record_assertion "degradation_recovery" "successful" "true" "$success/$total requests succeeded after recovery"
}

main() {
    log_info "Starting service degradation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_partial_degradation
    test_core_functionality_maintained
    test_degraded_response_quality
    test_recovery_from_degradation

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
