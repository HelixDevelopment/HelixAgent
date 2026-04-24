#!/bin/bash
# Provider Discovery Challenge
# Tests automatic provider discovery and verification

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "provider_discovery" "Provider Discovery Challenge"
load_env

log_info "Testing provider discovery and verification..."

# Test 1: Startup verification endpoint
test_startup_verification() {
    log_info "Test 1: Startup verification endpoint"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "startup_verify" "endpoint_works" "true" "Startup verification endpoint works"

        # Check for required fields
        local has_completed=$(echo "$body" | jq -e '.completed_at' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_duration=$(echo "$body" | jq -e '.duration_ms' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_providers=$(echo "$body" | jq -e '.ranked_providers' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_completed" == "true" ]] && [[ "$has_duration" == "true" ]] && [[ "$has_providers" == "true" ]]; then
            record_assertion "startup_verify" "has_fields" "true" "All required fields present"

            # Check provider count
            local provider_count=$(echo "$body" | jq '.ranked_providers | length' 2>/dev/null || echo "0")
            if [[ $provider_count -gt 0 ]]; then
                record_assertion "startup_verify" "has_providers" "true" "Found $provider_count providers"
            else
                record_assertion "startup_verify" "has_providers" "false" "No providers found"
            fi
        else
            record_assertion "startup_verify" "has_fields" "false" "Missing required fields"
        fi
    else
        record_assertion "startup_verify" "endpoint_works" "false" "Request failed with $http_code"
    fi
}

# Test 2: Provider list
test_provider_list() {
    log_info "Test 2: Provider list from startup verification"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local provider_count=$(echo "$response" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

    if [[ $provider_count -gt 0 ]]; then
        record_assertion "provider_list" "count" "true" "Found $provider_count providers"

        # Check for diversity (different provider types)
        local api_key_count=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "api_key")] | length' 2>/dev/null || echo "0")
        local oauth_count=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "oauth")] | length' 2>/dev/null || echo "0")
        local free_count=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "free")] | length' 2>/dev/null || echo "0")

        record_metric "api_key_providers" "$api_key_count"
        record_metric "oauth_providers" "$oauth_count"
        record_metric "free_providers" "$free_count"

        if [[ $api_key_count -gt 0 ]] || [[ $oauth_count -gt 0 ]] || [[ $free_count -gt 0 ]]; then
            record_assertion "provider_list" "diverse_types" "true" "Multiple auth types found"
        else
            record_assertion "provider_list" "diverse_types" "false" "No auth type diversity"
        fi
    else
        record_assertion "provider_list" "count" "false" "No providers found"
    fi
}

# Test 3: Provider ranking
test_provider_ranking() {
    log_info "Test 3: Provider ranking and scores"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local provider_count=$(echo "$response" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

    if [[ $provider_count -gt 0 ]]; then
        # Check first provider has score
        local first_score=$(echo "$response" | jq -r '.ranked_providers[0].score // .ranked_providers[0].final_score // 0' 2>/dev/null)

        if [[ $(echo "$first_score > 0" | bc 2>/dev/null || echo "0") -eq 1 ]]; then
            record_assertion "ranking" "has_scores" "true" "Providers have scores"

            # Check if scores are diverse (not all the same)
            local unique_scores=$(echo "$response" | jq '[.ranked_providers[].score // .ranked_providers[].final_score] | unique | length' 2>/dev/null || echo "0")

            if [[ $unique_scores -gt 1 ]]; then
                record_assertion "ranking" "diverse_scores" "true" "$unique_scores different scores (dynamic ranking)"
            else
                record_assertion "ranking" "diverse_scores" "false" "All scores identical (may be hardcoded)"
            fi
        else
            record_assertion "ranking" "has_scores" "false" "No scores found"
        fi
    else
        record_assertion "ranking" "has_scores" "false" "No providers to rank"
    fi
}

# Test 4: Provider verification status
test_provider_status() {
    log_info "Test 4: Provider verification status"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local provider_count=$(echo "$response" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

    if [[ $provider_count -gt 0 ]]; then
        # Count verified vs failed providers
        local verified_count=$(echo "$response" | jq '[.ranked_providers[] | select(.verified == true)] | length' 2>/dev/null || echo "0")
        local failed_count=$(echo "$response" | jq '[.ranked_providers[] | select(.verified == false)] | length' 2>/dev/null || echo "0")

        record_metric "verified_providers" "$verified_count"
        record_metric "failed_providers" "$failed_count"

        if [[ $verified_count -gt 0 ]]; then
            record_assertion "status" "has_verified" "true" "$verified_count providers verified"
        else
            record_assertion "status" "has_verified" "false" "No verified providers"
        fi

        # Check verification completeness
        local total_checked=$((verified_count + failed_count))
        if [[ $total_checked -eq $provider_count ]]; then
            record_assertion "status" "all_checked" "true" "All providers have verification status"
        else
            record_assertion "status" "all_checked" "false" "Some providers missing status"
        fi
    else
        record_assertion "status" "has_verified" "false" "No providers found"
    fi
}

# Test 5: Timestamp freshness
test_timestamp_freshness() {
    log_info "Test 5: Verification timestamp freshness"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local completed_at=$(echo "$response" | jq -r '.completed_at' 2>/dev/null || echo "")

    if [[ -n "$completed_at" ]] && [[ "$completed_at" != "null" ]]; then
        # Parse timestamp and check if recent (within last 10 minutes)
        local completed_ts=$(date -d "$completed_at" +%s 2>/dev/null || echo "0")
        local now_ts=$(date +%s)
        local age_seconds=$((now_ts - completed_ts))

        record_metric "verification_age_seconds" "$age_seconds"

        if [[ $age_seconds -lt 600 ]]; then
            record_assertion "timestamp" "fresh" "true" "Verification is fresh ($age_seconds seconds old)"
        else
            record_assertion "timestamp" "fresh" "false" "Verification is stale ($age_seconds seconds old)"
        fi
    else
        record_assertion "timestamp" "fresh" "false" "No timestamp found"
    fi
}

# Test 6: Provider capabilities
test_provider_capabilities() {
    log_info "Test 6: Provider capabilities information"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local first_provider=$(echo "$response" | jq -r '.ranked_providers[0] // empty' 2>/dev/null)

    if [[ -n "$first_provider" ]]; then
        # Check for capability fields
        local has_name=$(echo "$first_provider" | jq -e '.name' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_models=$(echo "$first_provider" | jq -e '.models' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_auth=$(echo "$first_provider" | jq -e '.auth_type' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_name" == "true" ]] && [[ "$has_auth" == "true" ]]; then
            record_assertion "capabilities" "basic_info" "true" "Provider has name and auth_type"
        else
            record_assertion "capabilities" "basic_info" "false" "Missing basic provider info"
        fi

        if [[ "$has_models" == "true" ]]; then
            local model_count=$(echo "$first_provider" | jq '.models | length' 2>/dev/null || echo "0")
            record_assertion "capabilities" "has_models" "true" "Provider lists $model_count models"
        else
            record_assertion "capabilities" "has_models" "false" "No model list"
        fi
    else
        record_assertion "capabilities" "basic_info" "false" "No provider data"
    fi
}

# Test 7: Discovery performance
test_discovery_performance() {
    log_info "Test 7: Discovery performance metrics"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local duration_ms=$(echo "$response" | jq -r '.duration_ms // 0' 2>/dev/null)

    if [[ $duration_ms -gt 0 ]]; then
        record_metric "discovery_duration_ms" "$duration_ms"

        # Discovery should complete in reasonable time
        if [[ $duration_ms -lt 180000 ]]; then
            record_assertion "performance" "reasonable_time" "true" "Discovery took ${duration_ms}ms (< 3 min)"
        else
            record_assertion "performance" "reasonable_time" "false" "Discovery took ${duration_ms}ms (> 3 min)"
        fi
    else
        record_assertion "performance" "reasonable_time" "false" "No duration metric"
    fi
}

# Main execution
main() {
    log_info "Starting Provider Discovery Challenge..."

    # Check if server is running
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_warning "HelixAgent not running, attempting to start..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    # Run tests
    test_startup_verification
    test_provider_list
    test_provider_ranking
    test_provider_status
    test_timestamp_freshness
    test_provider_capabilities
    test_discovery_performance

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
