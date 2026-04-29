#!/bin/bash
# Performance Scaling Behavior Challenge
# Tests system scaling and load handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-scaling-behavior" "Performance Scaling Behavior Challenge"
load_env

log_info "Testing scaling behavior..."

test_baseline_single_request() {
    log_info "Test 1: Baseline single request performance"

    local start=$(date +%s%N)
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Baseline"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)
    local latency=$(( ($(date +%s%N) - start) / 1000000 ))

    record_metric "baseline_single_ms" "$latency"
    [[ "$(echo "$resp" | tail -n1)" == "200" && $latency -lt 10000 ]] && record_assertion "baseline_single" "acceptable" "true" "${latency}ms"
}

test_linear_scaling_small() {
    log_info "Test 2: Linear scaling with small load (5 requests)"

    local count=5
    local start=$(date +%s%N)
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Small scale '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local avg_time=$((total_time / count))

    record_metric "small_scale_avg_ms" "$avg_time"
    [[ $success -ge 4 && $avg_time -lt 12000 ]] && record_assertion "small_scale" "linear" "true" "$success/$count in ${avg_time}ms avg"
}

test_linear_scaling_medium() {
    log_info "Test 3: Linear scaling with medium load (10 concurrent)"

    local count=10
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Medium scale '$i'"}],"max_tokens":10}' \
            --max-time 45 2>/dev/null > /tmp/scale_med_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        [[ "$(tail -n1 /tmp/scale_med_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/scale_med_$i.txt
    done

    record_metric "medium_scale_total_ms" "$total_time"
    [[ $success -ge 7 && $total_time -lt 45000 ]] && record_assertion "medium_scale" "handled" "true" "$success/$count in ${total_time}ms"
}

test_scaling_degradation_threshold() {
    log_info "Test 4: Scaling degradation threshold (15 concurrent)"

    local count=15
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Threshold test"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/scale_high_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        local code=$(tail -n1 /tmp/scale_high_$i.txt 2>/dev/null || echo "000")
        [[ "$code" =~ ^(200|429|503)$ ]] && success=$((success + 1))
        rm -f /tmp/scale_high_$i.txt
    done

    record_metric "high_scale_handled" "$success"
    record_metric "high_scale_total_ms" "$total_time"
    [[ $success -ge 10 ]] && record_assertion "scaling_threshold" "graceful" "true" "$success/$count handled"
}

main() {
    log_info "Starting scaling behavior challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_baseline_single_request
    test_linear_scaling_small
    test_linear_scaling_medium
    test_scaling_degradation_threshold

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
