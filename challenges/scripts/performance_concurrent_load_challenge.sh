#!/bin/bash
# Performance Concurrent Load Challenge
# Tests performance under concurrent load

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-concurrent-load" "Performance Concurrent Load Challenge"
load_env

log_info "Testing concurrent load performance..."

test_sequential_baseline() {
    log_info "Test 1: Sequential requests baseline"

    local count=5
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Sequential"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    record_metric "sequential_total_ms" "$total_time"
    [[ $total_time -lt 60000 ]] && record_assertion "sequential_baseline" "acceptable" "true" "$count requests in ${total_time}ms"
}

test_concurrent_performance() {
    log_info "Test 2: Concurrent request performance"

    local count=10
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent '$i'"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/conc_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        [[ "$(tail -n1 /tmp/conc_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/conc_$i.txt
    done

    record_metric "concurrent_total_ms" "$total_time"
    record_metric "concurrent_success" "$success"
    [[ $success -ge 8 && $total_time -lt 30000 ]] && record_assertion "concurrent_performance" "acceptable" "true" "$success/$count in ${total_time}ms"
}

test_high_concurrency() {
    log_info "Test 3: High concurrency handling"

    local count=20
    local pids=()

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"High concurrency"}],"max_tokens":10}' \
            --max-time 60 2>/dev/null > /tmp/high_$i.txt) &
        pids+=($!)
    done

    wait

    local success=0
    for i in $(seq 1 $count); do
        local code=$(tail -n1 /tmp/high_$i.txt 2>/dev/null || echo "000")
        [[ "$code" =~ ^(200|429|503)$ ]] && success=$((success + 1))
        rm -f /tmp/high_$i.txt
    done

    record_metric "high_concurrency_handled" "$success"
    [[ $success -ge 15 ]] && record_assertion "high_concurrency" "handled" "true" "$success/$count requests handled"
}

test_concurrent_stability() {
    log_info "Test 4: System remains stable under concurrent load"

    sleep 2  # Recovery

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    [[ "$(echo "$resp" | tail -n1)" =~ ^(200|204)$ ]] && record_assertion "concurrent_stability" "maintained" "true" "System stable after concurrent load"
}

main() {
    log_info "Starting concurrent load challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_sequential_baseline
    test_concurrent_performance
    test_high_concurrency
    test_concurrent_stability

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
