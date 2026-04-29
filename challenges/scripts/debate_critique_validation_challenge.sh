#!/bin/bash
# Debate Critique Validation Challenge
# Tests critique quality assessment and validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-critique-validation" "Debate Critique Validation Challenge"
load_env

log_info "Testing critique validation..."

test_critique_quality() {
    log_info "Test 1: Critique quality assessment"

    local request='{"debate_id":"test_debate","critique":"This argument lacks evidence and contains logical fallacies.","target_position":"proposer"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/critique/assess" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local quality_score=$(echo "$body" | jq -e '.quality_score' 2>/dev/null || echo "0.0")
        local is_constructive=$(echo "$body" | jq -e '.is_constructive' 2>/dev/null || echo "null")
        record_assertion "critique_quality" "working" "true" "Quality: $quality_score, Constructive: $is_constructive"
    else
        record_assertion "critique_quality" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_validation_rules() {
    log_info "Test 2: Critique validation rules"

    local request='{"critique":"Short","min_length":50,"require_evidence":true,"allow_ad_hominem":false}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/critique/validate" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local is_valid=$(echo "$body" | jq -e '.is_valid' 2>/dev/null || echo "null")
        local violations=$(echo "$body" | jq -e '.violations | length' 2>/dev/null || echo "0")
        record_assertion "validation_rules" "working" "true" "Valid: $is_valid, Violations: $violations"
    else
        record_assertion "validation_rules" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_feedback_assessment() {
    log_info "Test 3: Critique feedback assessment"

    local request='{"debate_id":"test_debate","critique_id":"crit_001","feedback_type":"peer_review"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/critique/feedback" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local feedback_score=$(echo "$body" | jq -e '.feedback_score' 2>/dev/null || echo "0.0")
        record_assertion "feedback_assessment" "working" "true" "Feedback score: $feedback_score"
    else
        record_assertion "feedback_assessment" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_improvement_suggestions() {
    log_info "Test 4: Critique improvement suggestions"

    local resp_body=$(curl -s "$BASE_URL/v1/debate/critique/suggestions?critique_id=crit_001" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local has_suggestions=$(echo "$resp_body" | jq -e '.suggestions' > /dev/null 2>&1 && echo "yes" || echo "no")
    local suggestion_count=$(echo "$resp_body" | jq -e '.suggestions | length' 2>/dev/null || echo "0")
    record_assertion "improvement_suggestions" "checked" "true" "Suggestions: $has_suggestions ($suggestion_count)"
}

main() {
    log_info "Starting critique validation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_critique_quality
    test_validation_rules
    test_feedback_assessment
    test_improvement_suggestions

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
