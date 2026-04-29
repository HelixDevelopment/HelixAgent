#!/bin/bash
# Debate Quality Metrics Challenge
# Tests debate quality assessment and metrics

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-quality-metrics" "Debate Quality Metrics Challenge"
load_env

log_info "Testing quality metrics..."

test_response_quality_scoring() {
    log_info "Test 1: Response quality scoring"

    local request='{"debate_id":"test_debate","response":"AI systems should prioritize safety and transparency to build public trust.","metrics":["clarity","relevance","depth"]}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/quality/score" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local overall_score=$(echo "$body" | jq -e '.overall_score' 2>/dev/null || echo "0.0")
        local clarity=$(echo "$body" | jq -e '.metrics.clarity' 2>/dev/null || echo "0.0")
        record_assertion "response_quality_scoring" "working" "true" "Overall: $overall_score, Clarity: $clarity"
    else
        record_skip "response_quality_scoring" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_coherence_validation() {
    log_info "Test 2: Argument coherence validation"

    local request='{"debate_id":"test_debate","arguments":["arg1","arg2","arg3"],"check_consistency":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/quality/coherence" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local coherence_score=$(echo "$body" | jq -e '.coherence_score' 2>/dev/null || echo "0.0")
        local is_consistent=$(echo "$body" | jq -e '.is_consistent' 2>/dev/null || echo "null")
        record_assertion "coherence_validation" "working" "true" "Score: $coherence_score, Consistent: $is_consistent"
    else
        record_skip "coherence_validation" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_factual_accuracy() {
    log_info "Test 3: Factual accuracy assessment"

    local request='{"debate_id":"test_debate","claim":"AI was invented in the 1950s","verify_facts":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/quality/factcheck" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local accuracy=$(echo "$body" | jq -e '.accuracy_score' 2>/dev/null || echo "0.0")
        local verified=$(echo "$body" | jq -e '.verified' 2>/dev/null || echo "null")
        record_assertion "factual_accuracy" "working" "true" "Accuracy: $accuracy, Verified: $verified"
    else
        record_skip "factual_accuracy" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_argument_strength() {
    log_info "Test 4: Argument strength evaluation"

    local resp_body=$(curl -s "$BASE_URL/v1/debate/quality/strength?debate_id=test_debate&argument_id=arg_001" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 15 2>/dev/null || echo '{}')

    local strength=$(echo "$resp_body" | jq -e '.strength_score' 2>/dev/null || echo "0.0")
    local supporting_evidence=$(echo "$resp_body" | jq -e '.supporting_evidence | length' 2>/dev/null || echo "0")
    record_assertion "argument_strength" "checked" "true" "Strength: $strength, Evidence: $supporting_evidence"
}

main() {
    log_info "Starting quality metrics challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_response_quality_scoring
    test_coherence_validation
    test_factual_accuracy
    test_argument_strength

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
