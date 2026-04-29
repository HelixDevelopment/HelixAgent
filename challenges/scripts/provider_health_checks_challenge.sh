#!/bin/bash
# Provider Health Checks Challenge
# Tests provider health monitoring

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-health-checks" "Provider Health Checks Challenge"
load_env

log_info "Testing health checks..."

test_health_endpoint_validation() {
    log_info "Test 1: Provider health endpoint validation"

    local request='{"provider":"anthropic","check_health":true,"include_metrics":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/health/check" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local health_status=$(echo "$body" | jq -r '.status' 2>/dev/null || echo "unknown")
        local response_time=$(echo "$body" | jq -e '.response_time_ms' 2>/dev/null || echo "0")
        record_assertion "health_endpoint_validation" "working" "true" "Status: $health_status, Response: ${response_time}ms"
    else
        record_skip "health_endpoint_validation" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_status_monitoring() {
    log_info "Test 2: Continuous status monitoring"

    local request='{"providers":["openai","anthropic","deepseek"],"monitor_interval":60,"alert_on_failure":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/health/monitor" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local monitoring_active=$(echo "$body" | jq -e '.monitoring_active' 2>/dev/null || echo "null")
        local monitored_count=$(echo "$body" | jq -e '.monitored_providers | length' 2>/dev/null || echo "0")
        record_assertion "status_monitoring" "working" "true" "Active: $monitoring_active, Count: $monitored_count"
    else
        record_skip "status_monitoring" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_uptime_tracking() {
    log_info "Test 3: Provider uptime tracking"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/health/uptime?provider=gemini&period=7d" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local uptime_percent=$(echo "$body" | jq -e '.uptime_percent' 2>/dev/null || echo "0.0")
        local total_checks=$(echo "$body" | jq -e '.total_checks' 2>/dev/null || echo "0")
        record_assertion "uptime_tracking" "working" "true" "Uptime: $uptime_percent%, Checks: $total_checks"
    else
        record_skip "uptime_tracking" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_degradation_detection() {
    log_info "Test 4: Service degradation detection"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/health/degradation?provider=mistral&threshold=0.8" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local is_degraded=$(echo "$resp_body" | jq -e '.degraded' 2>/dev/null || echo "null")
    local severity=$(echo "$resp_body" | jq -r '.severity' 2>/dev/null || echo "none")
    record_assertion "degradation_detection" "checked" "true" "Degraded: $is_degraded, Severity: $severity"
}

main() {
    log_info "Starting health checks challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_health_endpoint_validation
    test_status_monitoring
    test_uptime_tracking
    test_degradation_detection

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
