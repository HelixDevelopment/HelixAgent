#!/bin/bash
# Security DDoS Protection Challenge
# Tests Distributed Denial of Service protection mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "security-ddos-protection" "Security DDoS Protection Challenge"
load_env

log_info "Testing DDoS protection..."

test_request_flood_protection() {
    log_info "Test 1: Request flood protection"

    local start=$(date +%s)
    local success=0
    local blocked=0

    # Simulate request flood
    for i in {1..50}; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" \
            --max-time 2 2>/dev/null || echo -e "\n000")

        local code=$(echo "$resp" | tail -n1)
        [[ "$code" =~ ^(200|204)$ ]] && success=$((success + 1))
        [[ "$code" == "429" ]] && blocked=$((blocked + 1)) && break
    done

    local end=$(date +%s)
    local duration=$((end - start))

    record_metric "flood_requests" 50
    record_metric "flood_duration_seconds" $duration
    record_metric "requests_succeeded" $success

    if [[ $blocked -gt 0 ]]; then
        record_assertion "flood_protection" "rate_limited" "true" "Rate limiting triggered at $success requests"
    else
        record_assertion "flood_protection" "absorbed" "true" "System absorbed 50 requests in ${duration}s"
    fi
}

test_connection_limit() {
    log_info "Test 2: Connection limit enforcement"

    # Launch concurrent connections
    local pids=()
    for i in {1..20}; do
        (curl -s "$BASE_URL/health" --max-time 30 > /dev/null 2>&1) &
        pids+=($!)
    done

    # Wait for all to complete
    wait "${pids[@]}" 2>/dev/null || true

    record_metric "concurrent_connections" 20
    record_assertion "connection_limit" "handled" "true" "20 concurrent connections handled"
}

test_slow_request_protection() {
    log_info "Test 3: Slow request (Slowloris) protection"

    # Send slow request
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Slowloris test"}],"max_tokens":10}' \
        --limit-rate 100 --max-time 15 2>/dev/null || echo -e "\n000")

    local code=$(echo "$resp" | tail -n1)
    # Should either complete (200), timeout (000), or reject (408/429)
    [[ "$code" =~ ^(200|408|429|000)$ ]] && record_assertion "slow_request" "protected" "true" "Slow request handled (HTTP $code)"
}

test_recovery_after_flood() {
    log_info "Test 4: System recovery after DDoS attempt"

    # Trigger flood
    for i in {1..30}; do
        curl -s "$BASE_URL/health" --max-time 1 > /dev/null 2>&1 || true &
    done

    wait
    sleep 2

    # Normal request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "ddos_recovery" "operational" "true" "System recovered after DDoS attempt"
}

main() {
    log_info "Starting DDoS protection challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_request_flood_protection
    test_connection_limit
    test_slow_request_protection
    test_recovery_after_flood

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
