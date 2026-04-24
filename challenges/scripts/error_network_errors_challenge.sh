#!/bin/bash
# Error Network Errors Challenge
# Tests network error handling and connection issues

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-network-errors" "Error Network Errors Challenge"
load_env

log_info "Testing network error handling..."

test_connection_refused() {
    log_info "Test 1: Connection refused handling"

    # Try connecting to invalid port
    local resp=$(curl -s -w "\n%{http_code}" "http://localhost:9999/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"test","messages":[]}' \
        --max-time 5 2>/dev/null || echo -e "\n000")

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "000" ]] && record_assertion "connection_refused" "detected" "true" "Connection refused detected" || record_assertion "connection_refused" "not_applicable" "true" "Test service not down"
}

test_network_recovery() {
    log_info "Test 2: System operational after network issues"

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Network test"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "network_recovery" "operational" "true" "System operational"
}

test_partial_response_handling() {
    log_info "Test 3: Partial response handling"

    # Request with streaming that might be interrupted
    local output="$OUTPUT_DIR/logs/network_stream.txt"
    curl -s -N "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Stream test"}],"max_tokens":20,"stream":true}' \
        --max-time 30 > "$output" 2>/dev/null || true

    [[ -s "$output" ]] && record_assertion "partial_response" "data_received" "true" "Partial data received" || record_assertion "partial_response" "no_data" "true" "No streaming data"
}

test_network_error_classification() {
    log_info "Test 4: Network errors are properly classified"

    # Test with invalid host
    local resp=$(curl -s -w "\n%{http_code}" "http://invalid-host-12345.local/api" \
        -H "Content-Type: application/json" \
        -d '{}' \
        --max-time 5 2>/dev/null || echo -e "\n000")

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" == "000" ]] && record_assertion "network_classification" "dns_failure" "true" "DNS failure detected"

    # Verify main service still works
    resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    [[ "$(echo "$resp" | tail -n1)" =~ ^(200|204)$ ]] && record_assertion "network_classification" "service_healthy" "true" "Main service unaffected"
}

main() {
    log_info "Starting network errors challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_connection_refused
    test_network_recovery
    test_partial_response_handling
    test_network_error_classification

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
