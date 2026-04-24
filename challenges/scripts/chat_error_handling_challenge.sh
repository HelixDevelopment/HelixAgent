#!/bin/bash
# Chat Error Handling Challenge
# Tests error handling for invalid requests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "chat-error-handling" "Chat Error Handling Challenge"
load_env

log_info "Testing error handling for invalid chat requests..."

# Test 1: Invalid model name
test_invalid_model() {
    log_info "Test 1: Invalid model name"

    local request='{
        "model": "nonexistent-model-12345",
        "messages": [
            {"role": "user", "content": "Hello"}
        ],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    # Should return error (400 or 404)
    if [[ "$http_code" =~ ^(400|404)$ ]]; then
        record_assertion "invalid_model" "proper_error" "true" "Returns HTTP $http_code for invalid model"

        # Check for error message in response
        if echo "$body" | jq -e '.error' > /dev/null 2>&1; then
            record_assertion "invalid_model" "error_structure" "true" "Error response has proper structure"
        fi
    else
        record_assertion "invalid_model" "proper_error" "false" "HTTP $http_code (expected 400/404)"
    fi
}

# Test 2: Malformed JSON
test_malformed_json() {
    log_info "Test 2: Malformed JSON request"

    local malformed_request='{"model":"helixagent-debate","messages":[{"role":"user","content":"test"'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$malformed_request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Should return 400 Bad Request
    if [[ "$http_code" == "400" ]]; then
        record_assertion "malformed_json" "proper_error" "true" "Returns 400 for malformed JSON"
    else
        record_assertion "malformed_json" "proper_error" "false" "HTTP $http_code (expected 400)"
    fi
}

# Test 3: Missing required field (messages)
test_missing_messages() {
    log_info "Test 3: Missing required 'messages' field"

    local request='{
        "model": "helixagent-debate",
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    # Should return 400
    if [[ "$http_code" == "400" ]]; then
        record_assertion "missing_messages" "proper_error" "true" "Returns 400 for missing messages"

        if echo "$body" | grep -qi "message"; then
            record_assertion "missing_messages" "descriptive_error" "true" "Error mentions 'message'"
        fi
    else
        record_assertion "missing_messages" "proper_error" "false" "HTTP $http_code (expected 400)"
    fi
}

# Test 4: Empty messages array
test_empty_messages() {
    log_info "Test 4: Empty messages array"

    local request='{
        "model": "helixagent-debate",
        "messages": [],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Should return 400
    if [[ "$http_code" == "400" ]]; then
        record_assertion "empty_messages" "proper_error" "true" "Returns 400 for empty messages"
    else
        record_assertion "empty_messages" "proper_error" "false" "HTTP $http_code (expected 400)"
    fi
}

# Test 5: Invalid message role
test_invalid_role() {
    log_info "Test 5: Invalid message role"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "invalid_role", "content": "Hello"}
        ],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Should return 400
    if [[ "$http_code" == "400" ]]; then
        record_assertion "invalid_role" "proper_error" "true" "Returns 400 for invalid role"
    else
        record_assertion "invalid_role" "proper_error" "false" "HTTP $http_code (expected 400)"
    fi
}

# Test 6: Negative max_tokens
test_negative_max_tokens() {
    log_info "Test 6: Negative max_tokens value"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hello"}
        ],
        "max_tokens": -10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Should return 400
    if [[ "$http_code" == "400" ]]; then
        record_assertion "negative_tokens" "proper_error" "true" "Returns 400 for negative max_tokens"
    else
        record_assertion "negative_tokens" "proper_error" "false" "HTTP $http_code (expected 400)"
    fi
}

# Test 7: Invalid temperature
test_invalid_temperature() {
    log_info "Test 7: Invalid temperature value (> 2.0)"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hello"}
        ],
        "max_tokens": 10,
        "temperature": 5.0
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Should return 400 (or possibly accept and clamp to 2.0)
    if [[ "$http_code" =~ ^(200|400)$ ]]; then
        record_assertion "invalid_temperature" "handled" "true" "HTTP $http_code (handled gracefully)"
    else
        record_assertion "invalid_temperature" "handled" "false" "HTTP $http_code"
    fi
}

# Test 8: Missing content field
test_missing_content() {
    log_info "Test 8: Message missing 'content' field"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user"}
        ],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Should return 400
    if [[ "$http_code" == "400" ]]; then
        record_assertion "missing_content" "proper_error" "true" "Returns 400 for missing content"
    else
        record_assertion "missing_content" "proper_error" "false" "HTTP $http_code (expected 400)"
    fi
}

main() {
    log_info "Starting error handling challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_invalid_model
    test_malformed_json
    test_missing_messages
    test_empty_messages
    test_invalid_role
    test_negative_max_tokens
    test_invalid_temperature
    test_missing_content

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All error handling tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
