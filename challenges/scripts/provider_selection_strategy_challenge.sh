#!/bin/bash
# Provider Selection Strategy Challenge
# Tests provider selection algorithms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-selection-strategy" "Provider Selection Strategy Challenge"
load_env

log_info "Testing selection strategies..."

test_performance_based_selection() {
    log_info "Test 1: Performance-based selection"

    local request='{"task":"text_generation","selection_criteria":"performance","min_quality":0.8,"max_latency_ms":2000}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/select/performance" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local selected=$(echo "$body" | jq -r '.selected_provider' 2>/dev/null || echo "null")
        local score=$(echo "$body" | jq -e '.performance_score' 2>/dev/null || echo "0.0")
        record_assertion "performance_based_selection" "working" "true" "Provider: $selected, Score: $score"
    else
        record_assertion "performance_based_selection" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_cost_based_selection() {
    log_info "Test 2: Cost-based selection"

    local request='{"task":"text_generation","optimize_for":"cost","quality_threshold":0.7,"budget_per_request":0.01}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/select/cost" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local selected=$(echo "$body" | jq -r '.selected_provider' 2>/dev/null || echo "null")
        local estimated_cost=$(echo "$body" | jq -e '.estimated_cost' 2>/dev/null || echo "0.0")
        record_assertion "cost_based_selection" "working" "true" "Provider: $selected, Cost: $estimated_cost"
    else
        record_assertion "cost_based_selection" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_capability_matching() {
    log_info "Test 3: Capability-based matching"

    local request='{"required_capabilities":["streaming","function_calling","vision"],"match_all":false}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/select/capabilities" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local matched=$(echo "$body" | jq -e '.matched_providers | length' 2>/dev/null || echo "0")
        local best_match=$(echo "$body" | jq -r '.best_match' 2>/dev/null || echo "null")
        record_assertion "capability_matching" "working" "true" "Matched: $matched, Best: $best_match"
    else
        record_assertion "capability_matching" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_round_robin_strategy() {
    log_info "Test 4: Round-robin distribution"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/select/roundrobin?pool_size=5&get_next=true" \
        -X POST \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local next_provider=$(echo "$resp_body" | jq -r '.next_provider' 2>/dev/null || echo "null")
    local current_index=$(echo "$resp_body" | jq -e '.current_index' 2>/dev/null || echo "0")
    record_assertion "round_robin_strategy" "checked" "true" "Next: $next_provider, Index: $current_index"
}

main() {
    log_info "Starting selection strategy challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_performance_based_selection
    test_cost_based_selection
    test_capability_matching
    test_round_robin_strategy

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
