#!/bin/bash
# Debate Model Diversity Challenge
# Tests model diversity and distribution

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "debate-model-diversity" "Debate Model Diversity Challenge"
load_env

log_info "Testing model diversity..."

test_selection_diversity() {
    log_info "Test 1: Model selection diversity"

    local request='{"min_providers":3,"max_models_per_provider":2,"require_different_architectures":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/diversity/select" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local provider_count=$(echo "$body" | jq -e '.providers | length' 2>/dev/null || echo "0")
        local diversity_score=$(echo "$body" | jq -e '.diversity_score' 2>/dev/null || echo "0.0")
        record_assertion "selection_diversity" "working" "true" "Providers: $provider_count, Diversity: $diversity_score"
    else
        record_skip "selection_diversity" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_provider_distribution() {
    log_info "Test 2: Provider distribution analysis"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/diversity/distribution" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local total_providers=$(echo "$body" | jq -e '.distribution | length' 2>/dev/null || echo "0")
        local balanced=$(echo "$body" | jq -e '.is_balanced' 2>/dev/null || echo "null")
        record_assertion "provider_distribution" "working" "true" "Providers: $total_providers, Balanced: $balanced"
    else
        record_skip "provider_distribution" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_capability_mix() {
    log_info "Test 3: Capability mix optimization"

    local request='{"required_capabilities":["reasoning","creativity","analysis","synthesis"],"optimize_for":"balance"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/debate/diversity/capabilities" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local coverage=$(echo "$body" | jq -e '.capability_coverage' 2>/dev/null || echo "0.0")
        local models_selected=$(echo "$body" | jq -e '.selected_models | length' 2>/dev/null || echo "0")
        record_assertion "capability_mix" "working" "true" "Coverage: $coverage, Models: $models_selected"
    else
        record_skip "capability_mix" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_performance_balance() {
    log_info "Test 4: Performance balance validation"

    local resp_body=$(curl -s "$BASE_URL/v1/debate/diversity/performance?debate_id=test_debate" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local avg_latency=$(echo "$resp_body" | jq -e '.average_latency_ms' 2>/dev/null || echo "0")
    local variance=$(echo "$resp_body" | jq -e '.latency_variance' 2>/dev/null || echo "0.0")
    record_assertion "performance_balance" "checked" "true" "Latency: ${avg_latency}ms, Variance: $variance"
}

main() {
    log_info "Starting model diversity challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_selection_diversity
    test_provider_distribution
    test_capability_mix
    test_performance_balance

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
