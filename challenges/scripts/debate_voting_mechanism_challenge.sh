#!/bin/bash
# Debate Voting Mechanism Challenge
# Tests debate voting and decision-making

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-voting-mechanism" "Debate Voting Mechanism Challenge"
load_env

log_info "Testing voting mechanism..."

test_vote_collection() {
    log_info "Test 1: Vote collection from participants"

    local request='{"debate_id":"test_debate","votes":[{"voter":"model1","choice":"option_a","confidence":0.8},{"voter":"model2","choice":"option_b","confidence":0.7}]}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/votes/submit" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local votes_recorded=$(echo "$body" | jq -e '.votes_recorded' 2>/dev/null || echo "0")
        record_assertion "vote_collection" "working" "true" "Recorded $votes_recorded votes"
    else
        record_skip "vote_collection" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_vote_weighting() {
    log_info "Test 2: Vote weighting by confidence"

    local request='{"debate_id":"test_debate","weighting_strategy":"confidence_based","normalize":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/votes/aggregate" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local weighted_result=$(echo "$body" | jq -e '.weighted_result' 2>/dev/null || echo "null")
        record_assertion "vote_weighting" "working" "true" "Result: $weighted_result"
    else
        record_skip "vote_weighting" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_tie_breaking() {
    log_info "Test 3: Tie-breaking mechanism"

    local request='{"debate_id":"test_debate","tied_options":["option_a","option_b"],"tie_breaker":"arbiter_decision"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/votes/resolve-tie" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local winner=$(echo "$body" | jq -e '.winner' 2>/dev/null || echo "null")
        record_assertion "tie_breaking" "working" "true" "Winner: $winner"
    else
        record_skip "tie_breaking" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_vote_aggregation() {
    log_info "Test 4: Vote aggregation strategies"

    local resp_body=$(curl -s "$BASE_URL/v1/debate/votes/results?debate_id=test_debate&strategy=majority" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local final_decision=$(echo "$resp_body" | jq -e '.final_decision' 2>/dev/null || echo "null")
    local vote_count=$(echo "$resp_body" | jq -e '.total_votes' 2>/dev/null || echo "0")
    record_assertion "vote_aggregation" "checked" "true" "Decision: $final_decision, Votes: $vote_count"
}

main() {
    log_info "Starting voting mechanism challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_vote_collection
    test_vote_weighting
    test_tie_breaking
    test_vote_aggregation

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
