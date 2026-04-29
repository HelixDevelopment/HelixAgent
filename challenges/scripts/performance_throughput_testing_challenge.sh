#!/bin/bash
# Performance Throughput Testing Challenge
# Tests maximum throughput and request handling capacity

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-throughput-testing" "Performance Throughput Testing Challenge"
load_env

log_info "Testing throughput capacity..."

test_sequential_throughput() {
    log_info "Test 1: Sequential request throughput"

    local count=10
    local start=$(date +%s%N)
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Sequential '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local throughput=$((success * 1000 / total_time))

    record_metric "sequential_throughput_rps" "$throughput"
    record_metric "sequential_total_ms" "$total_time"
    [[ $success -ge 8 ]] && record_assertion "sequential_throughput" "acceptable" "true" "$success/$count, ${throughput} req/s"
}

test_parallel_throughput() {
    log_info "Test 2: Parallel request throughput"

    local count=12
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Parallel '$i'"}],"max_tokens":10}' \
            --max-time 45 2>/dev/null > /tmp/thr_par_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        [[ "$(tail -n1 /tmp/thr_par_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/thr_par_$i.txt
    done

    local parallel_throughput=$((success * 1000 / total_time))
    record_metric "parallel_throughput_rps" "$parallel_throughput"
    record_metric "parallel_total_ms" "$total_time"
    [[ $success -ge 9 && $total_time -lt 45000 ]] && record_assertion "parallel_throughput" "acceptable" "true" "$success/$count, ${parallel_throughput} req/s"
}

test_sustained_throughput() {
    log_info "Test 3: Sustained throughput measurement"

    local duration=30
    local count=0
    local success=0
    local start=$(date +%s)

    while [[ $(( $(date +%s) - start )) -lt $duration ]]; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Sustained '$count'"}],"max_tokens":10}' \
            --max-time 10 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
        count=$((count + 1))
        sleep 0.8
    done

    local throughput_per_min=$((success * 60 / duration))
    record_metric "sustained_throughput_per_min" "$throughput_per_min"
    record_metric "sustained_total_requests" "$count"
    [[ $success -ge $((count * 70 / 100)) ]] && record_assertion "sustained_throughput" "acceptable" "true" "$success/$count, ${throughput_per_min} req/min"
}

test_throughput_under_load() {
    log_info "Test 4: Throughput under concurrent load"

    local rounds=3
    local requests_per_round=6
    local total_success=0
    local total_time=0

    for round in $(seq 1 $rounds); do
        local round_start=$(date +%s%N)
        local pids=()

        for i in $(seq 1 $requests_per_round); do
            (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Load round '$round'"}],"max_tokens":10}' \
                --max-time 30 2>/dev/null > /tmp/thr_load_${round}_$i.txt) &
            pids+=($!)
        done

        wait

        for i in $(seq 1 $requests_per_round); do
            [[ "$(tail -n1 /tmp/thr_load_${round}_$i.txt 2>/dev/null)" == "200" ]] && total_success=$((total_success + 1))
            rm -f /tmp/thr_load_${round}_$i.txt
        done

        local round_time=$(( ($(date +%s%N) - round_start) / 1000000 ))
        total_time=$((total_time + round_time))
        sleep 1  # Brief pause between rounds
    done

    local total_requests=$((rounds * requests_per_round))
    local avg_throughput=$((total_success * 1000 / total_time))
    record_metric "load_throughput_rps" "$avg_throughput"
    record_metric "load_success_count" "$total_success"
    [[ $total_success -ge $((total_requests * 75 / 100)) ]] && record_assertion "throughput_under_load" "acceptable" "true" "$total_success/$total_requests, ${avg_throughput} req/s"
}

main() {
    log_info "Starting throughput testing challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_sequential_throughput
    test_parallel_throughput
    test_sustained_throughput
    test_throughput_under_load

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
