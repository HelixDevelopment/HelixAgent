#!/bin/bash
# Provider Capability Detection Challenge
# Tests provider capability detection and feature support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "provider_capability_detection" "Provider Capability Detection Challenge"
load_env

log_info "Testing provider capability detection..."

# Test 1: Basic capability detection
test_basic_capabilities() {
    log_info "Test 1: Basic capability detection"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local first_provider=$(echo "$response" | jq -r '.ranked_providers[0] // empty' 2>/dev/null)

    if [[ -n "$first_provider" ]]; then
        local capabilities=$(echo "$first_provider" | jq -r '.capabilities // empty' 2>/dev/null)

        if [[ -n "$capabilities" ]] && [[ "$capabilities" != "null" ]]; then
            record_assertion "basic" "has_capabilities" "true" "Provider has capabilities field"
        else
            record_assertion "basic" "has_capabilities" "false" "No capabilities field"
        fi
    else
        record_assertion "basic" "has_capabilities" "false" "No provider data"
    fi
}

# Test 2: Streaming support detection
test_streaming_support() {
    log_info "Test 2: Streaming support detection"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10,
        "stream": true
    }'

    local output_file="$OUTPUT_DIR/logs/capability_stream.log"

    curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 > "$output_file" 2>&1 || true

    if grep -q "data:" "$output_file"; then
        record_assertion "streaming" "supported" "true" "Streaming supported"
    else
        record_assertion "streaming" "supported" "false" "Streaming not working"
    fi
}

# Test 3: Model count per provider
test_models_per_provider() {
    log_info "Test 3: Models per provider"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local provider_count=$(echo "$response" | jq '.ranked_providers | length' 2>/dev/null || echo "0")

    if [[ $provider_count -gt 0 ]]; then
        local total_models=0

        for i in $(seq 0 $((provider_count - 1))); do
            local model_count=$(echo "$response" | jq -r ".ranked_providers[$i].models | length // 0" 2>/dev/null)
            total_models=$((total_models + model_count))
        done

        record_metric "total_provider_models" "$total_models"

        if [[ $total_models -gt 0 ]]; then
            record_assertion "models" "providers_have_models" "true" "$total_models total models from $provider_count providers"
        else
            record_assertion "models" "providers_have_models" "false" "No models detected"
        fi
    else
        record_assertion "models" "providers_have_models" "false" "No providers"
    fi
}

# Main execution
main() {
    log_info "Starting Provider Capability Detection Challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_warning "HelixAgent not running, attempting to start..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_basic_capabilities
    test_streaming_support
    test_models_per_provider

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo "0")
    failed_count=${failed_count:-0}

    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
