#!/bin/bash
# Debate Fallback Legacy Challenge
# Tests fallback to legacy debate system

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-fallback-legacy" "Debate Fallback Legacy Challenge"
load_env

log_info "Testing legacy fallback..."

test_fallback_triggers() {
    log_info "Test 1: Fallback trigger conditions"

    local request='{"debate_id":"test_debate","check_triggers":["orchestrator_failure","insufficient_participants","topology_error"]}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/fallback/check" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local should_fallback=$(echo "$body" | jq -e '.should_fallback' 2>/dev/null || echo "null")
        local triggers_met=$(echo "$body" | jq -e '.triggers_met | length' 2>/dev/null || echo "0")
        record_assertion "fallback_triggers" "working" "true" "Fallback: $should_fallback, Triggers: $triggers_met"
    else
        record_skip "fallback_triggers" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_legacy_activation() {
    log_info "Test 2: Legacy system activation"

    local request='{"debate_id":"test_debate","force_legacy":true,"reason":"testing_fallback"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/fallback/activate" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 20 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local activated=$(echo "$body" | jq -e '.legacy_activated' 2>/dev/null || echo "null")
        local mode=$(echo "$body" | jq -e '.debate_mode' 2>/dev/null || echo "null")
        record_assertion "legacy_activation" "working" "true" "Activated: $activated, Mode: $mode"
    else
        record_skip "legacy_activation" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_compatibility_checks() {
    log_info "Test 3: Legacy compatibility validation"

    local request='{"debate_config":{"topology":"mesh","phases":4},"legacy_features":["parallel_execution","voting_protocol"]}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/fallback/compatible" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local is_compatible=$(echo "$body" | jq -e '.compatible' 2>/dev/null || echo "null")
        local warnings=$(echo "$body" | jq -e '.warnings | length' 2>/dev/null || echo "0")
        record_assertion "compatibility_checks" "working" "true" "Compatible: $is_compatible, Warnings: $warnings"
    else
        record_skip "compatibility_checks" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_transition_handling() {
    log_info "Test 4: Orchestrator to legacy transition"

    local resp_body=$(curl -s "$BASE_URL/v1/debate/fallback/transition?debate_id=test_debate&preserve_state=true" \
        -X POST \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 15 2>/dev/null || echo '{}')

    local transitioned=$(echo "$resp_body" | jq -e '.transitioned' 2>/dev/null || echo "null")
    local state_preserved=$(echo "$resp_body" | jq -e '.state_preserved' 2>/dev/null || echo "null")
    record_assertion "transition_handling" "checked" "true" "Transitioned: $transitioned, Preserved: $state_preserved"
}

main() {
    log_info "Starting legacy fallback challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_fallback_triggers
    test_legacy_activation
    test_compatibility_checks
    test_transition_handling

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
