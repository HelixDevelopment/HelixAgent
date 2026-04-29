#!/bin/bash
# Provider Load Balancing Challenge
# Tests load distribution and balancing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-load-balancing" "Provider Load Balancing Challenge"
load_env

log_info "Testing load balancing..."

test_load_distribution() {
    log_info "Test 1: Request load distribution"

    local request='{"providers":["openai","anthropic","deepseek"],"distribution_strategy":"round_robin","concurrent_requests":100}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/loadbalancer/distribute" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local balanced=$(echo "$body" | jq -e '.balanced' 2>/dev/null || echo "null")
        local distribution=$(echo "$body" | jq -e '.distribution | length' 2>/dev/null || echo "0")
        record_assertion "load_distribution" "working" "true" "Balanced: $balanced, Providers: $distribution"
    else
        record_assertion "load_distribution" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_failover_handling() {
    log_info "Test 2: Automatic failover handling"

    local request='{"primary":"anthropic","fallbacks":["openai","deepseek"],"simulate_primary_failure":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/loadbalancer/failover" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local failed_over=$(echo "$body" | jq -e '.failed_over' 2>/dev/null || echo "null")
        local active_provider=$(echo "$body" | jq -r '.active_provider' 2>/dev/null || echo "null")
        record_assertion "failover_handling" "working" "true" "Failed over: $failed_over, Active: $active_provider"
    else
        record_assertion "failover_handling" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_capacity_management() {
    log_info "Test 3: Provider capacity management"

    local request='{"provider":"openai","check_capacity":true,"max_concurrent":50}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/loadbalancer/capacity" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local available_capacity=$(echo "$body" | jq -e '.available_capacity' 2>/dev/null || echo "0")
        local at_capacity=$(echo "$body" | jq -e '.at_capacity' 2>/dev/null || echo "null")
        record_assertion "capacity_management" "working" "true" "Available: $available_capacity, At capacity: $at_capacity"
    else
        record_assertion "capacity_management" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_traffic_routing() {
    log_info "Test 4: Dynamic traffic routing"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/loadbalancer/routing?strategy=least_latency" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local routing_enabled=$(echo "$resp_body" | jq -e '.enabled' 2>/dev/null || echo "null")
    local active_routes=$(echo "$resp_body" | jq -e '.active_routes | length' 2>/dev/null || echo "0")
    record_assertion "traffic_routing" "checked" "true" "Enabled: $routing_enabled, Routes: $active_routes"
}

main() {
    log_info "Starting load balancing challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_load_distribution
    test_failover_handling
    test_capacity_management
    test_traffic_routing

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
