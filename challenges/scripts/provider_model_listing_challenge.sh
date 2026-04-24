#!/bin/bash
# Provider Model Listing Challenge
# Tests provider model listing and availability

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "provider_model_listing" "Provider Model Listing Challenge"
load_env

log_info "Testing provider model listing and availability..."

# Test 1: /v1/models endpoint
test_models_endpoint() {
    log_info "Test 1: /v1/models endpoint"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "models_endpoint" "works" "true" "Models endpoint returns 200"

        # Check OpenAI format
        local has_data=$(echo "$body" | jq -e '.data' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_object=$(echo "$body" | jq -e '.object' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_data" == "true" ]] && [[ "$has_object" == "true" ]]; then
            record_assertion "models_endpoint" "openai_format" "true" "Response follows OpenAI format"

            local model_count=$(echo "$body" | jq '.data | length' 2>/dev/null || echo "0")
            record_metric "total_models" "$model_count"

            if [[ $model_count -gt 0 ]]; then
                record_assertion "models_endpoint" "has_models" "true" "Found $model_count models"
            else
                record_assertion "models_endpoint" "has_models" "false" "No models listed"
            fi
        else
            record_assertion "models_endpoint" "openai_format" "false" "Not OpenAI format"
        fi
    else
        record_assertion "models_endpoint" "works" "false" "Endpoint returned $http_code"
    fi
}

# Test 2: Model metadata
test_model_metadata() {
    log_info "Test 2: Model metadata fields"

    local response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local first_model=$(echo "$response" | jq -r '.data[0] // empty' 2>/dev/null)

    if [[ -n "$first_model" ]]; then
        # Check required fields
        local has_id=$(echo "$first_model" | jq -e '.id' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_object=$(echo "$first_model" | jq -e '.object' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_created=$(echo "$first_model" | jq -e '.created' >/dev/null 2>&1 && echo "true" || echo "false")
        local has_owned_by=$(echo "$first_model" | jq -e '.owned_by' >/dev/null 2>&1 && echo "true" || echo "false")

        if [[ "$has_id" == "true" ]] && [[ "$has_object" == "true" ]]; then
            record_assertion "metadata" "required_fields" "true" "Model has required fields (id, object)"
        else
            record_assertion "metadata" "required_fields" "false" "Missing required fields"
        fi

        if [[ "$has_created" == "true" ]] && [[ "$has_owned_by" == "true" ]]; then
            record_assertion "metadata" "optional_fields" "true" "Model has optional fields (created, owned_by)"
        else
            record_assertion "metadata" "optional_fields" "false" "Missing optional fields"
        fi
    else
        record_assertion "metadata" "required_fields" "false" "No model data"
    fi
}

# Test 3: Debate team models
test_debate_team_models() {
    log_info "Test 3: Debate team model availability"

    local response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    # Check for helixagent-debate model
    local has_debate=$(echo "$response" | jq '[.data[] | select(.id == "helixagent-debate")] | length' 2>/dev/null || echo "0")

    if [[ $has_debate -gt 0 ]]; then
        record_assertion "debate_team" "debate_model" "true" "helixagent-debate model available"
    else
        record_assertion "debate_team" "debate_model" "false" "helixagent-debate not in list"
    fi

    # Check for ensemble model
    local has_ensemble=$(echo "$response" | jq '[.data[] | select(.id == "helixagent-ensemble")] | length' 2>/dev/null || echo "0")

    if [[ $has_ensemble -gt 0 ]]; then
        record_assertion "debate_team" "ensemble_model" "true" "helixagent-ensemble model available"
    else
        record_assertion "debate_team" "ensemble_model" "false" "helixagent-ensemble not in list"
    fi
}

# Test 4: Provider-specific models
test_provider_models() {
    log_info "Test 4: Provider-specific model listing"

    local response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local model_count=$(echo "$response" | jq '.data | length' 2>/dev/null || echo "0")

    if [[ $model_count -gt 0 ]]; then
        # Get providers from startup verification
        local verification=$(curl -s "$BASE_URL/v1/startup/verification" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

        local provider_count=$(echo "$verification" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

        record_metric "providers_count" "$provider_count"
        record_metric "models_count" "$model_count"

        # Generally models >= providers (some providers have multiple models)
        if [[ $model_count -ge $provider_count ]]; then
            record_assertion "provider_models" "adequate_count" "true" "$model_count models from $provider_count providers"
        else
            record_assertion "provider_models" "adequate_count" "false" "Only $model_count models from $provider_count providers"
        fi
    else
        record_assertion "provider_models" "adequate_count" "false" "No models listed"
    fi
}

# Test 5: Model ID uniqueness
test_model_uniqueness() {
    log_info "Test 5: Model ID uniqueness"

    local response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local model_count=$(echo "$response" | jq '.data | length' 2>/dev/null || echo "0")
    local unique_count=$(echo "$response" | jq '[.data[].id] | unique | length' 2>/dev/null || echo "0")

    if [[ $model_count -gt 0 ]]; then
        if [[ $model_count -eq $unique_count ]]; then
            record_assertion "uniqueness" "all_unique" "true" "All $model_count model IDs are unique"
        else
            local duplicates=$((model_count - unique_count))
            record_assertion "uniqueness" "all_unique" "false" "$duplicates duplicate model IDs"
        fi
    else
        record_assertion "uniqueness" "all_unique" "false" "No models to check"
    fi
}

# Test 6: Model listing consistency
test_listing_consistency() {
    log_info "Test 6: Model listing consistency across calls"

    # Call endpoint twice
    local response1=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    sleep 1

    local response2=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local count1=$(echo "$response1" | jq '.data | length' 2>/dev/null || echo "0")
    local count2=$(echo "$response2" | jq '.data | length' 2>/dev/null || echo "0")

    if [[ $count1 -eq $count2 ]]; then
        record_assertion "consistency" "same_count" "true" "Model count consistent ($count1 both times)"

        # Check if IDs match
        local ids1=$(echo "$response1" | jq -r '.data[].id' 2>/dev/null | sort)
        local ids2=$(echo "$response2" | jq -r '.data[].id' 2>/dev/null | sort)

        if [[ "$ids1" == "$ids2" ]]; then
            record_assertion "consistency" "same_models" "true" "Model IDs identical across calls"
        else
            record_assertion "consistency" "same_models" "false" "Model IDs differ"
        fi
    else
        record_assertion "consistency" "same_count" "false" "Count differs ($count1 vs $count2)"
    fi
}

# Test 7: Model usability
test_model_usability() {
    log_info "Test 7: Listed models are usable"

    local response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local first_model_id=$(echo "$response" | jq -r '.data[0].id // empty' 2>/dev/null)

    if [[ -n "$first_model_id" ]]; then
        # Try to use the first listed model
        local request="{
            \"model\": \"$first_model_id\",
            \"messages\": [{\"role\": \"user\", \"content\": \"Test\"}],
            \"max_tokens\": 10
        }"

        local chat_response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$chat_response" | tail -n1)

        if [[ "$http_code" == "200" ]]; then
            record_assertion "usability" "first_model_works" "true" "First listed model ($first_model_id) is usable"
        else
            record_assertion "usability" "first_model_works" "false" "First model failed: $http_code"
        fi
    else
        record_assertion "usability" "first_model_works" "false" "No model to test"
    fi
}

# Main execution
main() {
    log_info "Starting Provider Model Listing Challenge..."

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
    test_models_endpoint
    test_model_metadata
    test_debate_team_models
    test_provider_models
    test_model_uniqueness
    test_listing_consistency
    test_model_usability

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
