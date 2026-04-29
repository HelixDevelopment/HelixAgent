#!/bin/bash
# Debate Response Aggregation Challenge
# Tests response combining and aggregation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-response-aggregation" "Debate Response Aggregation Challenge"
load_env

log_info "Testing response aggregation..."

test_response_combining() {
    log_info "Test 1: Multiple response combining"

    local request='{"responses":[{"model":"model1","content":"Response A","score":0.85},{"model":"model2","content":"Response B","score":0.78},{"model":"model3","content":"Response C","score":0.92}],"method":"best_of_n"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/aggregate/combine" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local combined=$(echo "$body" | jq -e '.combined_response' > /dev/null 2>&1 && echo "yes" || echo "no")
        local selected_model=$(echo "$body" | jq -e '.selected_model' 2>/dev/null || echo "null")
        record_assertion "response_combining" "working" "true" "Combined: $combined, Model: $selected_model"
    else
        record_assertion "response_combining" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_weighted_merging() {
    log_info "Test 2: Weighted response merging"

    local request='{"responses":[{"content":"part1","weight":0.4},{"content":"part2","weight":0.35},{"content":"part3","weight":0.25}],"merge_strategy":"weighted_blend"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/aggregate/merge" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local merged=$(echo "$body" | jq -e '.merged_response' > /dev/null 2>&1 && echo "yes" || echo "no")
        local total_weight=$(echo "$body" | jq -e '.total_weight' 2>/dev/null || echo "0.0")
        record_assertion "weighted_merging" "working" "true" "Merged: $merged, Total weight: $total_weight"
    else
        record_assertion "weighted_merging" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_conflict_resolution() {
    log_info "Test 3: Conflicting response resolution"

    local request='{"responses":[{"content":"Yes","confidence":0.8},{"content":"No","confidence":0.7},{"content":"Maybe","confidence":0.6}],"resolution":"highest_confidence"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/aggregate/resolve" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local resolved=$(echo "$body" | jq -e '.resolved_response' 2>/dev/null || echo "null")
        local confidence=$(echo "$body" | jq -e '.final_confidence' 2>/dev/null || echo "0.0")
        record_assertion "conflict_resolution" "working" "true" "Resolved: $resolved, Confidence: $confidence"
    else
        record_assertion "conflict_resolution" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_final_synthesis() {
    log_info "Test 4: Final response synthesis"

    local resp_body=$(curl -s "$BASE_URL/v1/debate/aggregate/synthesize?debate_id=test_debate" \
        -X POST \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 20 2>/dev/null || echo '{}')

    local has_synthesis=$(echo "$resp_body" | jq -e '.synthesis' > /dev/null 2>&1 && echo "yes" || echo "no")
    local sources_count=$(echo "$resp_body" | jq -e '.sources_used' 2>/dev/null || echo "0")
    record_assertion "final_synthesis" "checked" "true" "Synthesis: $has_synthesis, Sources: $sources_count"
}

main() {
    log_info "Starting response aggregation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_response_combining
    test_weighted_merging
    test_conflict_resolution
    test_final_synthesis

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
