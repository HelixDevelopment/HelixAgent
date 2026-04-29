#!/bin/bash
# Chat Model Selection Challenge
# Tests model selection and switching between available models

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_model_selection" "Chat Model Selection Challenge"
load_env

log_info "Testing model selection and availability..."

# Test 1: List available models
test_list_models() {
    log_info "Test 1: List available models via /v1/models"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "list_models" "success" "true" "Models list endpoint succeeded"

        # Check response structure
        local has_data=$(echo "$body" | jq -e '.data' >/dev/null 2>&1 && echo "true" || echo "false")
        if [[ "$has_data" == "true" ]]; then
            record_assertion "list_models" "has_data" "true" "Response has data array"

            local model_count=$(echo "$body" | jq '.data | length' 2>/dev/null || echo "0")
            if [[ $model_count -gt 0 ]]; then
                record_assertion "list_models" "has_models" "true" "Found $model_count models"
            else
                record_assertion "list_models" "has_models" "false" "No models found"
            fi
        else
            record_assertion "list_models" "has_data" "false" "No data array"
        fi
    else
        record_assertion "list_models" "success" "false" "Request failed with $http_code"
    fi
}

# Test 2: Default model (helixagent-debate)
test_default_model() {
    log_info "Test 2: Default model (helixagent-debate)"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hello"}],
        "max_tokens": 20
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "default_model" "works" "true" "helixagent-debate works"

        # Check model field in response
        local response_model=$(echo "$body" | jq -r '.model' 2>/dev/null || echo "")
        if [[ -n "$response_model" ]]; then
            record_assertion "default_model" "model_field" "true" "Response includes model: $response_model"
        else
            record_assertion "default_model" "model_field" "false" "No model field in response"
        fi
    else
        record_assertion "default_model" "works" "false" "Request failed with $http_code"
    fi
}

# Test 3: Ensemble model (helixagent-ensemble)
test_ensemble_model() {
    log_info "Test 3: Ensemble model (helixagent-ensemble)"

    local request='{
        "model": "helixagent-ensemble",
        "messages": [{"role": "user", "content": "What is 2+2?"}],
        "max_tokens": 20
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 45 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "ensemble_model" "works" "true" "helixagent-ensemble works"
    elif [[ "$http_code" == "404" ]] || [[ "$http_code" == "400" ]]; then
        record_assertion "ensemble_model" "works" "false" "Ensemble model not available ($http_code)"
    else
        record_assertion "ensemble_model" "works" "false" "Unexpected status: $http_code"
    fi
}

# Test 4: Provider-specific model (if available)
test_provider_model() {
    log_info "Test 4: Provider-specific model selection"

    # Get list of available models
    local models_response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local first_model=$(echo "$models_response" | jq -r '.data[0].id' 2>/dev/null || echo "")

    if [[ -n "$first_model" ]] && [[ "$first_model" != "null" ]]; then
        local request="{
            \"model\": \"$first_model\",
            \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}],
            \"max_tokens\": 20
        }"

        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "200" ]]; then
            record_assertion "provider_model" "works" "true" "Provider model $first_model works"
        else
            record_assertion "provider_model" "works" "false" "Model $first_model failed ($http_code)"
        fi
    else
        record_assertion "provider_model" "works" "false" "Could not get model list"
    fi
}

# Test 5: Model switching within conversation
test_model_switching() {
    log_info "Test 5: Model switching between requests"

    # First request with debate model
    local request1='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hello"}],
        "max_tokens": 10
    }'

    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 30 2>/dev/null || true)

    local http_code1=$(echo "$response1" | tail -n1)

    # Get another model
    local models_response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)
    local second_model=$(echo "$models_response" | jq -r '.data[1].id // "helixagent-debate"' 2>/dev/null || echo "helixagent-debate")

    # Second request with different model
    local request2="{
        \"model\": \"$second_model\",
        \"messages\": [{\"role\": \"user\", \"content\": \"Hello again\"}],
        \"max_tokens\": 10
    }"

    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" --max-time 30 2>/dev/null || true)

    local http_code2=$(echo "$response2" | tail -n1)

    if [[ "$http_code1" == "200" ]] && [[ "$http_code2" == "200" ]]; then
        record_assertion "model_switch" "both_work" "true" "Both models work in sequence"
    else
        record_assertion "model_switch" "both_work" "false" "One failed ($http_code1, $http_code2)"
    fi
}

# Test 6: Model information in response
test_model_response_info() {
    log_info "Test 6: Model information in chat response"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hi"}],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        # Check for model field
        local model=$(echo "$body" | jq -r '.model' 2>/dev/null || echo "")
        if [[ -n "$model" ]] && [[ "$model" != "null" ]]; then
            record_assertion "model_info" "has_model" "true" "Response includes model: $model"
        else
            record_assertion "model_info" "has_model" "false" "No model field"
        fi

        # Check for id field
        local id=$(echo "$body" | jq -r '.id' 2>/dev/null || echo "")
        if [[ -n "$id" ]] && [[ "$id" != "null" ]]; then
            record_assertion "model_info" "has_id" "true" "Response has unique ID"
        else
            record_assertion "model_info" "has_id" "false" "No ID field"
        fi
    else
        record_assertion "model_info" "http_status" "false" "Request failed with $http_code"
    fi
}

# Test 7: Model capabilities consistency
test_model_capabilities() {
    log_info "Test 7: Model capabilities from /v1/models match usage"

    local models_response=$(curl -s "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local debate_found=$(echo "$models_response" | jq -r '.data[] | select(.id == "helixagent-debate") | .id' 2>/dev/null || echo "")

    if [[ "$debate_found" == "helixagent-debate" ]]; then
        record_assertion "capabilities" "debate_listed" "true" "helixagent-debate in model list"

        # Try to use it
        local request='{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "Test"}],
            "max_tokens": 10
        }'

        # Full debate ensemble can take 25-45s for a real run (5 positions
        # × multi-pass validation). Use 60s timeout to match realistic
        # end-user latency budget — a tighter timeout produces false
        # FAILED (CONST-035 wrapper bluff: test failure when feature
        # actually works, just slowly).
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 60 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "200" ]]; then
            record_assertion "capabilities" "usable" "true" "Listed model is usable"
        else
            record_assertion "capabilities" "usable" "false" "Listed model not usable ($http_code)"
        fi
    else
        record_assertion "capabilities" "debate_listed" "false" "helixagent-debate not in list"
    fi
}

# Main execution
main() {
    log_info "Starting Chat Model Selection Challenge..."

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
    test_list_models
    test_default_model
    test_ensemble_model
    test_provider_model
    test_model_switching
    test_model_response_info
    test_model_capabilities

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
