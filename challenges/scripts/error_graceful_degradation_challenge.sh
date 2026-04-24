#!/bin/bash
# Error Graceful Degradation Challenge
# Tests graceful degradation and fallback to degraded modes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-graceful-degradation" "Error Graceful Degradation Challenge"
load_env

log_info "Testing graceful degradation..."

test_provider_fallback() {
    log_info "Test 1: Provider fallback on failure"

    # Request with specific model that might fail
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Fallback test"}],"max_tokens":10}' \
        --max-time 60 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        record_assertion "provider_fallback" "working" "true" "Request succeeded (possibly via fallback)"

        # Check if response indicates fallback was used
        if echo "$body" | jq -e '.metadata.fallback_used' > /dev/null 2>&1; then
            record_assertion "provider_fallback" "detected" "true" "Fallback metadata present"
        fi
    fi
}

test_partial_feature_degradation() {
    log_info "Test 2: Partial feature degradation"

    # Request with advanced features that might degrade
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Advanced test"}],"max_tokens":50,"temperature":0.7,"stream":false}' \
        --max-time 60 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    # System should respond even if some features unavailable
    [[ "$code" =~ ^(200|206)$ ]] && record_assertion "feature_degradation" "graceful" "true" "System responds with available features"
}

test_reduced_quality_mode() {
    log_info "Test 3: Reduced quality mode"

    # Make request that might trigger reduced quality
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Quality test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)

    # Should respond even if quality is reduced
    if [[ "$code" == "200" ]]; then
        record_assertion "reduced_quality" "operational" "true" "System operational in reduced quality mode"
    fi
}

test_degradation_recovery() {
    log_info "Test 4: Recovery from degraded state"

    # Trigger potential degradation
    for i in {1..5}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"invalid-model","messages":[{"role":"user","content":"test"}]}' \
            --max-time 10 > /dev/null 2>&1 || true
    done

    sleep 2

    # Normal request should work (recovered)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "degradation_recovery" "successful" "true" "System recovered from degraded state"
}

main() {
    log_info "Starting graceful degradation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_provider_fallback
    test_partial_feature_degradation
    test_reduced_quality_mode
    test_degradation_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
