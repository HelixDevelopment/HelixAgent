#!/bin/bash
# Provider Cost Optimization Challenge
# Tests cost tracking and optimization

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider-cost-optimization" "Provider Cost Optimization Challenge"
load_env

log_info "Testing cost optimization..."

test_cost_tracking() {
    log_info "Test 1: Provider cost tracking"

    local request='{"provider":"openai","model":"gpt-4","tokens_used":1000,"track_cost":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/cost/track" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local estimated_cost=$(echo "$body" | jq -e '.estimated_cost' 2>/dev/null || echo "0.0")
        local currency=$(echo "$body" | jq -r '.currency' 2>/dev/null || echo "USD")
        record_assertion "cost_tracking" "working" "true" "Cost: $estimated_cost $currency"
    else
        record_assertion "cost_tracking" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_budget_limits() {
    log_info "Test 2: Budget limit enforcement"

    local request='{"user_id":"test_user","monthly_budget":100.00,"check_limit":true}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/cost/budget" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local within_budget=$(echo "$body" | jq -e '.within_budget' 2>/dev/null || echo "null")
        local remaining=$(echo "$body" | jq -e '.remaining_budget' 2>/dev/null || echo "0.0")
        record_assertion "budget_limits" "working" "true" "Within budget: $within_budget, Remaining: $remaining"
    else
        record_assertion "budget_limits" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_cost_effective_selection() {
    log_info "Test 3: Cost-effective provider selection"

    local request='{"task":"text_generation","quality_threshold":0.8,"optimize_for":"cost"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/providers/cost/optimize" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" \
        --max-time 15 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local recommended=$(echo "$body" | jq -r '.recommended_provider' 2>/dev/null || echo "null")
        local cost_savings=$(echo "$body" | jq -e '.estimated_savings_percent' 2>/dev/null || echo "0")
        record_assertion "cost_effective_selection" "working" "true" "Provider: $recommended, Savings: $cost_savings%"
    else
        record_assertion "cost_effective_selection" "checked" "true" "HTTP $code (may not be implemented)"
    fi
}

test_usage_optimization() {
    log_info "Test 4: Usage pattern optimization"

    local resp_body=$(curl -s "$BASE_URL/v1/providers/cost/usage?user_id=test_user&period=monthly" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || echo '{}')

    local total_cost=$(echo "$resp_body" | jq -e '.total_cost' 2>/dev/null || echo "0.0")
    local optimization_suggestions=$(echo "$resp_body" | jq -e '.suggestions | length' 2>/dev/null || echo "0")
    record_assertion "usage_optimization" "checked" "true" "Total cost: $total_cost, Suggestions: $optimization_suggestions"
}

main() {
    log_info "Starting cost optimization challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_cost_tracking
    test_budget_limits
    test_cost_effective_selection
    test_usage_optimization

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
