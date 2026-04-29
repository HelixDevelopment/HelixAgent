#!/bin/bash
# Performance Provider Selection Speed Challenge
# Tests speed of provider selection and routing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "performance-provider-selection-speed" "Performance Provider Selection Speed Challenge"
load_env

log_info "Testing provider selection speed..."

test_selection_latency() {
    log_info "Test 1: Provider selection latency"

    local count=5
    local total_time=0

    for i in $(seq 1 $count); do
        local start=$(date +%s%N)
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Selection test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /dev/null
        local latency=$(( ($(date +%s%N) - start) / 1000000 ))
        total_time=$((total_time + latency))
    done

    local avg_time=$((total_time / count))
    record_metric "selection_latency_avg_ms" "$avg_time"
    [[ $avg_time -lt 10000 ]] && record_assertion "selection_latency" "acceptable" "true" "${avg_time}ms average"
}

test_score_based_routing() {
    log_info "Test 2: Score-based routing performance"

    local count=8
    local start=$(date +%s%N)
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Routing test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local avg_time=$((total_time / count))

    record_metric "routing_avg_ms" "$avg_time"
    [[ $success -ge 6 && $avg_time -lt 12000 ]] && record_assertion "score_routing" "efficient" "true" "$success/$count in ${avg_time}ms avg"
}

test_concurrent_selection() {
    log_info "Test 3: Concurrent provider selection"

    local count=6
    local pids=()
    local start=$(date +%s%N)

    for i in $(seq 1 $count); do
        (curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Concurrent selection '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null > /tmp/prov_sel_$i.txt) &
        pids+=($!)
    done

    wait

    local total_time=$(( ($(date +%s%N) - start) / 1000000 ))
    local success=0
    for i in $(seq 1 $count); do
        [[ "$(tail -n1 /tmp/prov_sel_$i.txt 2>/dev/null)" == "200" ]] && success=$((success + 1))
        rm -f /tmp/prov_sel_$i.txt
    done

    record_metric "concurrent_selection_ms" "$total_time"
    [[ $success -ge 5 && $total_time -lt 20000 ]] && record_assertion "concurrent_selection" "efficient" "true" "$success/$count in ${total_time}ms"
}

test_selection_consistency() {
    log_info "Test 4: Selection consistency under load"

    local count=10
    local success=0

    for i in $(seq 1 $count); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Consistency test"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)
        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "selection_consistency_success" "$success"
    [[ $success -ge 8 ]] && record_assertion "selection_consistency" "maintained" "true" "$success/$count succeeded"
}

main() {
    log_info "Starting provider selection speed challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_selection_latency
    test_score_based_routing
    test_concurrent_selection
    test_selection_consistency

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
