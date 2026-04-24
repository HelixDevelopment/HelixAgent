#!/bin/bash
# Provider Authentication Challenge
# Tests provider authentication methods and credential management

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "provider_authentication" "Provider Authentication Challenge"
load_env

log_info "Testing provider authentication methods..."

# Test 1: API key provider detection
test_api_key_providers() {
    log_info "Test 1: API key provider detection"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local api_key_providers=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "api_key")]' 2>/dev/null || echo "[]")
    local api_key_count=$(echo "$api_key_providers" | jq 'length' 2>/dev/null || echo "0")

    if [[ $api_key_count -gt 0 ]]; then
        record_assertion "api_key" "detected" "true" "Found $api_key_count API key providers"

        # List provider names
        local provider_names=$(echo "$api_key_providers" | jq -r '.[].name' 2>/dev/null | tr '\n' ', ' | sed 's/,$//')
        record_metric "api_key_providers" "$provider_names"
    else
        record_assertion "api_key" "detected" "false" "No API key providers found"
    fi
}

# Test 2: OAuth provider detection
test_oauth_providers() {
    log_info "Test 2: OAuth provider detection"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local oauth_providers=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "oauth")]' 2>/dev/null || echo "[]")
    local oauth_count=$(echo "$oauth_providers" | jq 'length' 2>/dev/null || echo "0")

    if [[ $oauth_count -gt 0 ]]; then
        record_assertion "oauth" "detected" "true" "Found $oauth_count OAuth providers"

        # List provider names
        local provider_names=$(echo "$oauth_providers" | jq -r '.[].name' 2>/dev/null | tr '\n' ', ' | sed 's/,$//')
        record_metric "oauth_providers" "$provider_names"

        # Check if OAuth credentials are being used
        local oauth_verified=$(echo "$oauth_providers" | jq '[.[] | select(.verified == true)] | length' 2>/dev/null || echo "0")

        if [[ $oauth_verified -gt 0 ]]; then
            record_assertion "oauth" "credentials_work" "true" "$oauth_verified OAuth providers verified"
        else
            record_assertion "oauth" "credentials_work" "false" "No OAuth providers verified"
        fi
    else
        record_assertion "oauth" "detected" "false" "No OAuth providers found"
    fi
}

# Test 3: Free provider detection
test_free_providers() {
    log_info "Test 3: Free provider detection"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    local free_providers=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "free")]' 2>/dev/null || echo "[]")
    local free_count=$(echo "$free_providers" | jq 'length' 2>/dev/null || echo "0")

    if [[ $free_count -gt 0 ]]; then
        record_assertion "free" "detected" "true" "Found $free_count free providers"

        # List provider names
        local provider_names=$(echo "$free_providers" | jq -r '.[].name' 2>/dev/null | tr '\n' ', ' | sed 's/,$//')
        record_metric "free_providers" "$provider_names"

        # Free providers should work without credentials
        local free_verified=$(echo "$free_providers" | jq '[.[] | select(.verified == true)] | length' 2>/dev/null || echo "0")

        if [[ $free_verified -gt 0 ]]; then
            record_assertion "free" "work_without_creds" "true" "$free_verified free providers work"
        else
            record_assertion "free" "work_without_creds" "false" "Free providers not working"
        fi
    else
        record_assertion "free" "detected" "false" "No free providers found"
    fi
}

# Test 4: Authentication failure handling
test_auth_failure_handling() {
    log_info "Test 4: Authentication failure handling"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    # Check for providers with auth failures
    local failed_auth=$(echo "$response" | jq '[.ranked_providers[] | select(.verified == false and .error_type == "auth_error")] | length' 2>/dev/null || echo "0")

    record_metric "auth_failures" "$failed_auth"

    if [[ $failed_auth -gt 0 ]]; then
        record_assertion "auth_failure" "detected" "true" "$failed_auth providers with auth failures"

        # Check if error messages are helpful
        local has_error_msg=$(echo "$response" | jq '[.ranked_providers[] | select(.verified == false) | select(.error_message != null)] | length' 2>/dev/null || echo "0")

        if [[ $has_error_msg -gt 0 ]]; then
            record_assertion "auth_failure" "helpful_errors" "true" "Auth errors include messages"
        else
            record_assertion "auth_failure" "helpful_errors" "false" "No error messages"
        fi
    else
        record_assertion "auth_failure" "detected" "false" "No auth failures (all configured correctly)"
    fi
}

# Test 5: Mixed authentication support
test_mixed_auth_support() {
    log_info "Test 5: Mixed authentication support"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    # Count different auth types
    local api_key_count=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "api_key")] | length' 2>/dev/null || echo "0")
    local oauth_count=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "oauth")] | length' 2>/dev/null || echo "0")
    local free_count=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "free")] | length' 2>/dev/null || echo "0")

    local auth_types_count=0
    [[ $api_key_count -gt 0 ]] && auth_types_count=$((auth_types_count + 1))
    [[ $oauth_count -gt 0 ]] && auth_types_count=$((auth_types_count + 1))
    [[ $free_count -gt 0 ]] && auth_types_count=$((auth_types_count + 1))

    if [[ $auth_types_count -ge 2 ]]; then
        record_assertion "mixed" "multiple_types" "true" "Supports $auth_types_count auth types"
    elif [[ $auth_types_count -eq 1 ]]; then
        record_assertion "mixed" "multiple_types" "false" "Only 1 auth type configured"
    else
        record_assertion "mixed" "multiple_types" "false" "No auth types detected"
    fi

    record_metric "auth_type_count" "$auth_types_count"
}

# Test 6: Provider credential validation
test_credential_validation() {
    log_info "Test 6: Provider credential validation"

    # Test with valid credentials (default)
    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Test"}],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "credential_val" "request_works" "true" "Request works with credentials"
    else
        record_assertion "credential_val" "request_works" "false" "Request failed: $http_code"
    fi
}

# Test 7: CLI proxy authentication (OAuth providers)
test_cli_proxy_auth() {
    log_info "Test 7: CLI proxy authentication for OAuth providers"

    local response=$(curl -s "$BASE_URL/v1/startup/verification" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || true)

    # Check if OAuth providers are using CLI proxy
    local oauth_providers=$(echo "$response" | jq '[.ranked_providers[] | select(.auth_type == "oauth")]' 2>/dev/null || echo "[]")
    local oauth_count=$(echo "$oauth_providers" | jq 'length' 2>/dev/null || echo "0")

    if [[ $oauth_count -gt 0 ]]; then
        # Check if they indicate CLI proxy usage
        local cli_proxy_count=$(echo "$oauth_providers" | jq '[.[] | select(.access_method == "cli" or .access_method == "cli_proxy")] | length' 2>/dev/null || echo "0")

        if [[ $cli_proxy_count -gt 0 ]]; then
            record_assertion "cli_proxy" "detected" "true" "$cli_proxy_count OAuth providers use CLI proxy"
        else
            record_assertion "cli_proxy" "detected" "false" "OAuth providers not using CLI proxy (may use direct API)"
        fi
    else
        record_assertion "cli_proxy" "detected" "false" "No OAuth providers to check"
    fi
}

# Main execution
main() {
    log_info "Starting Provider Authentication Challenge..."

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
    test_api_key_providers
    test_oauth_providers
    test_free_providers
    test_auth_failure_handling
    test_mixed_auth_support
    test_credential_validation
    test_cli_proxy_auth

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
