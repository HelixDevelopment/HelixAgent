#!/bin/bash
# Chat System Message Challenge
# Tests system message behavior and influence

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "chat-system-message" "Chat System Message Challenge"
load_env

log_info "Testing system message behavior..."

# Test 1: Basic system message
test_basic_system_message() {
    log_info "Test 1: Basic system message usage"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant that always responds in exactly 3 words."},
            {"role": "user", "content": "What is AI?"}
        ],
        "max_tokens": 20,
        "temperature": 0.3
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "system" "http_status" "true" "System message accepted"

        local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
        local word_count=$(echo "$content" | wc -w)
        record_metric "system_response_words" $word_count

        # Check if response follows instruction (should be ~3 words)
        if [[ $word_count -ge 2 && $word_count -le 5 ]]; then
            record_assertion "system" "instruction_followed" "true" "Response has $word_count words"
        else
            record_assertion "system" "instruction_followed" "false" "Response has $word_count words (expected ~3)"
        fi
    else
        record_assertion "system" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 2: System message persona
test_system_persona() {
    log_info "Test 2: System message defining persona"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are a pirate. Always use pirate language."},
            {"role": "user", "content": "Say hello"}
        ],
        "max_tokens": 50,
        "temperature": 0.7
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "persona" "http_status" "true" "Persona system message accepted"

        local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')

        # Check for pirate-like words (ahoy, mate, arr, etc.)
        if echo "$content" | grep -qiE "(ahoy|mate|arr|ye|aye)"; then
            record_assertion "persona" "character_present" "true" "Response shows persona characteristics"
        else
            record_assertion "persona" "character_present" "false" "No obvious persona markers"
        fi
    else
        record_assertion "persona" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 3: Multiple system messages
test_multiple_system_messages() {
    log_info "Test 3: Multiple system messages handling"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are helpful."},
            {"role": "system", "content": "Be concise."},
            {"role": "user", "content": "Explain quantum computing"}
        ],
        "max_tokens": 100,
        "temperature": 0.5
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "multiple_system" "http_status" "true" "Multiple system messages accepted"

        local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
        local word_count=$(echo "$content" | wc -w)
        record_metric "multiple_system_words" $word_count

        # Response should be relatively concise due to "Be concise" instruction
        if [[ $word_count -lt 150 ]]; then
            record_assertion "multiple_system" "concise" "true" "Response is concise ($word_count words)"
        fi
    else
        record_assertion "multiple_system" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 4: No system message (user-only)
test_no_system_message() {
    log_info "Test 4: Conversation without system message"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hello, how are you?"}
        ],
        "max_tokens": 30,
        "temperature": 0.5
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "no_system" "http_status" "true" "No system message works"

        if echo "$body" | grep -q '"content"'; then
            record_assertion "no_system" "has_response" "true" "Response generated without system message"
        fi
    else
        record_assertion "no_system" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 5: System message with multi-turn
test_system_with_multiturn() {
    log_info "Test 5: System message persistence in multi-turn conversation"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "Always end responses with the word STOP."},
            {"role": "user", "content": "What is 2+2?"},
            {"role": "assistant", "content": "4 STOP"},
            {"role": "user", "content": "What is 5+5?"}
        ],
        "max_tokens": 20,
        "temperature": 0.3
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "system_multiturn" "http_status" "true" "Multi-turn with system works"

        local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')

        # Check if STOP appears in response (system instruction persistence)
        if echo "$content" | grep -qi "STOP"; then
            record_assertion "system_multiturn" "instruction_persists" "true" "System instruction persists"
        else
            record_assertion "system_multiturn" "instruction_persists" "false" "System instruction may not persist"
        fi
    else
        record_assertion "system_multiturn" "http_status" "false" "HTTP $http_code"
    fi
}

main() {
    log_info "Starting system message challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    test_basic_system_message
    test_system_persona
    test_multiple_system_messages
    test_no_system_message
    test_system_with_multiturn

    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All system message tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
