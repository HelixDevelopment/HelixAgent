#!/bin/bash
# Provider Scoring Challenge
# Tests provider scoring and ranking algorithms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-7061}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "provider_scoring" "Provider Scoring Challenge"
load_env

log_info "Testing provider scoring and ranking..."

# Test 1: Score components
test_score_components() {
    log_info "Test 1: Score components presence"

    local response=$(curl -s --max-time 60 "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local first_provider=$(echo "$response" | jq -r '.ranked_providers[0] // empty' 2>/dev/null)

    if [[ -n "$first_provider" ]]; then
        # Check for score components
        local has_score=$(echo "$first_provider" | jq -e '.score // .final_score' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_score" == "true" ]]; then
            record_assertion "components" "has_final_score" "true" "Provider has final score"

            # Check for component scores
            local has_speed=$(echo "$first_provider" | jq -e '.score_components.speed // .speed_score' >/dev/null 2>&1 && echo "true" || echo "false")
            local has_cost=$(echo "$first_provider" | jq -e '.score_components.cost // .cost_score' >/dev/null 2>&1 && echo "true" || echo "false")
            local has_capability=$(echo "$first_provider" | jq -e '.score_components.capability // .capability_score' >/dev/null 2>&1 && echo "true" || echo "false")

            if [[ "$has_speed" == "true" ]] || [[ "$has_cost" == "true" ]] || [[ "$has_capability" == "true" ]]; then
                record_assertion "components" "has_breakdown" "true" "Score components available"
            else
                record_assertion "components" "has_breakdown" "false" "No score breakdown"
            fi
        else
            record_assertion "components" "has_final_score" "false" "No score found"
        fi
    else
        record_assertion "components" "has_final_score" "false" "No provider data"
    fi
}

# Test 2: Score range validation
test_score_range() {
    log_info "Test 2: Score range validation"

    local response=$(curl -s --max-time 60 "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local provider_count=$(echo "$response" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

    if [[ $provider_count -gt 0 ]]; then
        # Check all scores are within valid range (0-10 typically)
        local invalid_count=0

        for i in $(seq 0 $((provider_count - 1))); do
            local score=$(echo "$response" | jq -r ".ranked_providers[$i].score // .ranked_providers[$i].final_score // 0" 2>/dev/null)

            # Check if score is between 0 and 10
            if (( $(echo "$score < 0 || $score > 10" | bc -l 2>/dev/null || echo "1") )); then
                invalid_count=$((invalid_count + 1))
            fi
        done

        if [[ $invalid_count -eq 0 ]]; then
            record_assertion "range" "valid" "true" "All $provider_count scores in valid range (0-10)"
        else
            record_assertion "range" "valid" "false" "$invalid_count scores out of range"
        fi
    else
        record_assertion "range" "valid" "false" "No providers to validate"
    fi
}

# Test 3: Ranking consistency
test_ranking_consistency() {
    log_info "Test 3: Ranking consistency (descending scores)"

    local response=$(curl -s --max-time 60 "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local provider_count=$(echo "$response" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

    if [[ $provider_count -gt 1 ]]; then
        # Check if scores are in descending order
        local prev_score=999
        local out_of_order=0

        for i in $(seq 0 $((provider_count - 1))); do
            local current_score=$(echo "$response" | jq -r ".ranked_providers[$i].score // .ranked_providers[$i].final_score // 0" 2>/dev/null)

            if (( $(echo "$current_score > $prev_score" | bc -l 2>/dev/null || echo "0") )); then
                out_of_order=$((out_of_order + 1))
            fi

            prev_score=$current_score
        done

        if [[ $out_of_order -eq 0 ]]; then
            record_assertion "consistency" "ordered" "true" "Providers ranked in descending score order"
        else
            record_assertion "consistency" "ordered" "false" "$out_of_order providers out of order"
        fi
    else
        record_assertion "consistency" "ordered" "false" "Not enough providers to check ordering"
    fi
}

# Test 4: Free provider scoring
test_free_provider_scoring() {
    log_info "Test 4: Free provider scoring (if present)"

    local response=$(curl -s --max-time 60 "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    # Find free providers
    local free_providers=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "free")]' 2>/dev/null || echo "[]")
    local free_count=$(echo "$free_providers" | jq 'length' 2>/dev/null || echo "0")

    if [[ $free_count -gt 0 ]]; then
        record_assertion "free" "present" "true" "Found $free_count free providers"

        # Check free provider scores (should be in range 6.0-7.0 typically)
        local first_free_score=$(echo "$free_providers" | jq -r '.[0].score // .[0].final_score // 0' 2>/dev/null)

        if (( $(echo "$first_free_score >= 5.0 && $first_free_score <= 8.0" | bc -l 2>/dev/null || echo "0") )); then
            record_assertion "free" "score_range" "true" "Free provider score in expected range ($first_free_score)"
        else
            record_assertion "free" "score_range" "false" "Free provider score unexpected ($first_free_score)"
        fi
    else
        record_assertion "free" "present" "false" "No free providers found"
    fi
}

# Test 5: OAuth provider scoring
test_oauth_provider_scoring() {
    log_info "Test 5: OAuth provider scoring bonus"

    local response=$(curl -s --max-time 60 "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    # Find OAuth providers
    local oauth_providers=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "oauth")]' 2>/dev/null || echo "[]")
    local oauth_count=$(echo "$oauth_providers" | jq 'length' 2>/dev/null || echo "0")

    if [[ $oauth_count -gt 0 ]]; then
        record_assertion "oauth" "present" "true" "Found $oauth_count OAuth providers"

        # OAuth providers should get bonus (score >= 7.0 typically)
        local first_oauth_score=$(echo "$oauth_providers" | jq -r '.[0].score // .[0].final_score // 0' 2>/dev/null)

        if (( $(echo "$first_oauth_score >= 7.0" | bc -l 2>/dev/null || echo "0") )); then
            record_assertion "oauth" "bonus_applied" "true" "OAuth provider has high score ($first_oauth_score >= 7.0)"
        else
            record_assertion "oauth" "bonus_applied" "false" "OAuth score lower than expected ($first_oauth_score)"
        fi
    else
        record_assertion "oauth" "present" "false" "No OAuth providers found"
    fi
}

# Test 6: Score diversity
test_score_diversity() {
    log_info "Test 6: Score diversity (dynamic vs hardcoded)"

    local response=$(curl -s --max-time 60 "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local provider_count=$(echo "$response" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

    if [[ $provider_count -gt 2 ]]; then
        # Get all unique scores
        local unique_scores=$(echo "$response" | jq '[.ranked_providers[].score // .ranked_providers[].final_score] | unique | length' 2>/dev/null || echo "0")

        record_metric "unique_score_count" "$unique_scores"

        # High diversity suggests dynamic scoring
        if [[ $unique_scores -ge 3 ]]; then
            record_assertion "diversity" "dynamic" "true" "$unique_scores unique scores (dynamic scoring)"
        elif [[ $unique_scores -ge 2 ]]; then
            record_assertion "diversity" "dynamic" "false" "Only $unique_scores unique scores (low diversity)"
        else
            record_assertion "diversity" "dynamic" "false" "All scores identical (hardcoded?)"
        fi
    else
        record_assertion "diversity" "dynamic" "false" "Not enough providers to assess diversity"
    fi
}

# Test 7: Top provider selection
test_top_provider() {
    log_info "Test 7: Top provider selection"

    local response=$(curl -s --max-time 60 "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local top_provider=$(echo "$response" | jq -r '.ranked_providers[0] // empty' 2>/dev/null)

    if [[ -n "$top_provider" ]]; then
        local name=$(echo "$top_provider" | jq -r '.name // "unknown"' 2>/dev/null)
        local score=$(echo "$top_provider" | jq -r '.score // .final_score // 0' 2>/dev/null)
        local verified=$(echo "$top_provider" | jq -r '.verified // false' 2>/dev/null)

        record_assertion "top" "exists" "true" "Top provider: $name (score: $score)"

        if [[ "$verified" == "true" ]]; then
            record_assertion "top" "verified" "true" "Top provider is verified"
        else
            record_assertion "top" "verified" "false" "Top provider not verified"
        fi

        # Top provider should have high score
        if (( $(echo "$score >= 7.0" | bc -l 2>/dev/null || echo "0") )); then
            record_assertion "top" "high_score" "true" "Top provider has high score ($score >= 7.0)"
        else
            record_assertion "top" "high_score" "false" "Top provider score lower ($score)"
        fi
    else
        record_assertion "top" "exists" "false" "No top provider found"
    fi
}

# Main execution
main() {
    log_info "Starting Provider Scoring Challenge..."

    # Check if server is running
    if ! curl -s --max-time 60 "$BASE_URL/health" > /dev/null 2>&1; then
        log_warning "HelixAgent not running, attempting to start..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    # Run tests
    test_score_components
    test_score_range
    test_ranking_consistency
    test_free_provider_scoring
    test_oauth_provider_scoring
    test_score_diversity
    test_top_provider

    # Calculate results
    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo "0")
    failed_count=${failed_count:-0}

    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
