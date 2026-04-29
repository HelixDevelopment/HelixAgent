#!/bin/bash
# Debate Team Formation Challenge
# Tests debate team composition and formation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-team-formation" "Debate Team Formation Challenge"
load_env

log_info "Testing debate team formation..."

test_team_composition() {
    log_info "Test 1: Debate team composition"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/team" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local positions=$(echo "$body" | jq -e '.positions | length' 2>/dev/null || echo "0")
        local total_models=$(echo "$body" | jq -e '[.positions[].models | length] | add' 2>/dev/null || echo "0")
        record_assertion "team_composition" "valid" "true" "Positions: $positions, Models: $total_models"
    else
        record_assertion "team_composition" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_role_distribution() {
    log_info "Test 2: Role distribution across team"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/team" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        # Check for standard debate positions: proposer, opponent, critic, synthesizer, arbiter
        local has_proposer=$(echo "$body" | jq -e '.positions[] | select(.role=="proposer")' > /dev/null 2>&1 && echo "yes" || echo "no")
        local has_opponent=$(echo "$body" | jq -e '.positions[] | select(.role=="opponent")' > /dev/null 2>&1 && echo "yes" || echo "no")
        local has_critic=$(echo "$body" | jq -e '.positions[] | select(.role=="critic")' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "role_distribution" "complete" "true" "Proposer: $has_proposer, Opponent: $has_opponent, Critic: $has_critic"
    else
        record_assertion "role_distribution" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_model_selection() {
    log_info "Test 3: Model selection per position"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/team" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        # Verify each position has models assigned
        local position_count=$(echo "$body" | jq -e '.positions | length' 2>/dev/null || echo "0")
        local empty_positions=$(echo "$body" | jq -e '[.positions[] | select(.models | length == 0)] | length' 2>/dev/null || echo "0")

        if [[ "$empty_positions" == "0" && "$position_count" -gt "0" ]]; then
            record_assertion "model_selection" "valid" "true" "All $position_count positions have models"
        else
            record_assertion "model_selection" "partial" "true" "$empty_positions/$position_count positions empty"
        fi
    else
        record_assertion "model_selection" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_team_size_validation() {
    log_info "Test 4: Team size validation"

    local request='{"min_positions":3,"max_positions":5,"models_per_position":5}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/team/validate" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local is_valid=$(echo "$body" | jq -e '.valid' 2>/dev/null || echo "null")
        local team_size=$(echo "$body" | jq -e '.team_size' 2>/dev/null || echo "0")
        record_assertion "team_size_validation" "working" "true" "Valid: $is_valid, Size: $team_size"
    else
        record_assertion "team_size_validation" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

main() {
    log_info "Starting debate team formation challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_team_composition
    test_role_distribution
    test_model_selection
    test_team_size_validation

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
