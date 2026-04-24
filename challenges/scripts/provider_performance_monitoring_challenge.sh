#!/bin/bash
# Provider Performance Monitoring Challenge
# Tests provider performance monitoring and metrics

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "provider_performance_monitoring" "Provider Performance Monitoring Challenge"
load_env

log_info "Testing provider performance monitoring..."

# Test 1: Provider metrics endpoint
test_provider_metrics() {
    log_info "Test 1: Provider metrics endpoint"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/monitoring/provider-metrics" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "metrics" "endpoint_works" "true" "Provider metrics endpoint works"

        # Check for metrics data
        local has_metrics=$(echo "$body" | jq -e '.metrics // .provider_metrics' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_metrics" == "true" ]]; then
            record_assertion "metrics" "has_data" "true" "Metrics data available"
        else
            record_assertion "metrics" "has_data" "false" "No metrics data"
        fi
    else
        record_assertion "metrics" "endpoint_works" "false" "Endpoint returned $http_code"
    fi
}

# Test 2: Latency metrics
test_latency_metrics() {
    log_info "Test 2: Provider latency metrics"

    # Make a request and measure latency
    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Quick test"}],
        "max_tokens": 10
    }'

    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "latency" "measured" "true" "Request completed in ${latency}ms"
        record_metric "request_latency_ms" "$latency"

        # Reasonable latency (<30s)
        if [[ $latency -lt 30000 ]]; then
            record_assertion "latency" "reasonable" "true" "Latency acceptable (<30s)"
        else
            record_assertion "latency" "reasonable" "false" "High latency (${latency}ms)"
        fi
    else
        record_assertion "latency" "measured" "false" "Request failed: $http_code"
    fi
}

# Test 3: Success rate metrics
test_success_rate() {
    log_info "Test 3: Provider success rate"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10
    }'

    local success_count=0
    local total_requests=5

    for i in $(seq 1 $total_requests); do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        [[ "$http_code" == "200" ]] && success_count=$((success_count + 1))

        sleep 0.5
    done

    local success_rate=$((success_count * 100 / total_requests))
    record_metric "provider_success_rate" "$success_rate"

    if [[ $success_rate -ge 80 ]]; then
        record_assertion "success_rate" "high" "true" "$success_rate% success rate (>= 80%)"
    elif [[ $success_rate -ge 50 ]]; then
        record_assertion "success_rate" "high" "false" "$success_rate% success rate (50-80%)"
    else
        record_assertion "success_rate" "high" "false" "Low success rate: $success_rate%"
    fi
}

# Test 4: Performance degradation detection
test_performance_degradation() {
    log_info "Test 4: Performance degradation detection"

    # Make requests and track latency trend
    local latencies=()

    for i in {1..3}; do
        local request='{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "Test '${i}'"}],
            "max_tokens": 10
        }'

        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 > /dev/null 2>&1 || true
        local end=$(date +%s%N)

        local latency=$(( (end - start) / 1000000 ))
        latencies+=($latency)

        sleep 1
    done

    # Check if latencies are stable (no significant increase)
    local first_latency=${latencies[0]}
    local last_latency=${latencies[2]}

    if [[ $last_latency -le $((first_latency * 2)) ]]; then
        record_assertion "degradation" "stable" "true" "Performance stable (${first_latency}ms → ${last_latency}ms)"
    else
        record_assertion "degradation" "stable" "false" "Performance degraded (${first_latency}ms → ${last_latency}ms)"
    fi
}

# Test 5: Provider response time distribution
test_response_time_distribution() {
    log_info "Test 5: Provider response time distribution"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10
    }'

    local response_times=()

    for i in {1..10}; do
        local start=$(date +%s%N)
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)
        local end=$(date +%s%N)

        local http_code=$(echo "$response" | tail -n1)
        if [[ "$http_code" == "200" ]]; then
            local latency=$(( (end - start) / 1000000 ))
            response_times+=($latency)
        fi

        sleep 0.2
    done

    local count=${#response_times[@]}

    if [[ $count -ge 5 ]]; then
        # Calculate average
        local sum=0
        for rt in "${response_times[@]}"; do
            sum=$((sum + rt))
        done
        local avg=$((sum / count))

        record_metric "avg_response_time_ms" "$avg"
        record_assertion "distribution" "measured" "true" "Average response time: ${avg}ms (from $count samples)"
    else
        record_assertion "distribution" "measured" "false" "Only $count successful samples"
    fi
}

# Test 6: Error rate tracking
test_error_rate() {
    log_info "Test 6: Error rate tracking"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10
    }'

    local error_count=0
    local total_requests=10

    for i in $(seq 1 $total_requests); do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        if [[ "$http_code" != "200" ]]; then
            error_count=$((error_count + 1))
        fi

        sleep 0.2
    done

    local error_rate=$((error_count * 100 / total_requests))
    record_metric "error_rate" "$error_rate"

    if [[ $error_rate -le 10 ]]; then
        record_assertion "error_rate" "acceptable" "true" "Error rate: $error_rate% (<= 10%)"
    elif [[ $error_rate -le 30 ]]; then
        record_assertion "error_rate" "acceptable" "false" "Error rate: $error_rate% (10-30%)"
    else
        record_assertion "error_rate" "acceptable" "false" "High error rate: $error_rate% (>30%)"
    fi
}

# Test 7: Monitoring status endpoint
test_monitoring_status() {
    log_info "Test 7: Overall monitoring status"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/monitoring/status" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "monitoring" "status_endpoint" "true" "Monitoring status endpoint works"

        # Check for status fields
        local has_status=$(echo "$body" | jq -e '.status' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_uptime=$(echo "$body" | jq -e '.uptime' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_status" == "true" ]]; then
            local status=$(echo "$body" | jq -r '.status' 2>/dev/null)
            record_assertion "monitoring" "has_status" "true" "Status: $status"
        else
            record_assertion "monitoring" "has_status" "false" "No status field"
        fi

        if [[ "$has_uptime" == "true" ]]; then
            local uptime=$(echo "$body" | jq -r '.uptime' 2>/dev/null)
            record_assertion "monitoring" "has_uptime" "true" "Uptime: $uptime"
        else
            record_assertion "monitoring" "has_uptime" "false" "No uptime field"
        fi
    else
        record_assertion "monitoring" "status_endpoint" "false" "Endpoint returned $http_code"
    fi
}

# Main execution
main() {
    log_info "Starting Provider Performance Monitoring Challenge..."

    # Check if server is running
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_warning "HelixAgent not running, attempting to start..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    # Run tests
    test_provider_metrics
    test_latency_metrics
    test_success_rate
    test_performance_degradation
    test_response_time_distribution
    test_error_rate
    test_monitoring_status

    # Calculate results
    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo "0")
    failed_count=${failed_count:-0}

    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
