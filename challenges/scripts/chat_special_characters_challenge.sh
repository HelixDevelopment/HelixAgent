#!/bin/bash
# Chat Special Characters Challenge
# Tests handling of special characters, unicode, emojis, and edge cases

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_special_characters" "Chat Special Characters Challenge"
load_env

log_info "Testing special characters and unicode handling..."

# Test 1: Unicode characters
test_unicode_characters() {
    log_info "Test 1: Unicode characters in messages"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Translate: こんにちは"}],
        "max_tokens": 30
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json; charset=utf-8" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "unicode" "accepted" "true" "Unicode characters accepted"
    else
        record_assertion "unicode" "accepted" "false" "Unicode failed ($http_code)"
    fi
}

# Test 2: Emojis
test_emojis() {
    log_info "Test 2: Emoji handling"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What does 😀 mean?"}],
        "max_tokens": 20
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json; charset=utf-8" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "emojis" "handled" "true" "Emojis handled successfully"
    else
        record_assertion "emojis" "handled" "false" "Emoji handling failed ($http_code)"
    fi
}

# Test 3: Escaped characters
test_escaped_characters() {
    log_info "Test 3: Escaped characters (quotes, backslashes)"

    local request='{"model": "helixagent-debate", "messages": [{"role": "user", "content": "Say: \"Hello\\nWorld\""}], "max_tokens": 20}'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "escaped" "parsed" "true" "Escaped characters parsed correctly"
    else
        record_assertion "escaped" "parsed" "false" "Escape handling failed ($http_code)"
    fi
}

# Test 4: HTML/XML characters
test_html_characters() {
    log_info "Test 4: HTML/XML special characters"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is <html> & </html>?"}],
        "max_tokens": 30
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "html" "handled" "true" "HTML characters handled"
    else
        record_assertion "html" "handled" "false" "HTML handling failed ($http_code)"
    fi
}

# Test 5: Code with special characters
test_code_characters() {
    log_info "Test 5: Code snippets with special characters"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Explain: const x = {a: 1};"}],
        "max_tokens": 50
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "code" "accepted" "true" "Code with braces/semicolons accepted"
    else
        record_assertion "code" "accepted" "false" "Code handling failed ($http_code)"
    fi
}

# Test 6: Long repeated characters
test_repeated_characters() {
    log_info "Test 6: Long repeated character strings"

    local repeated_string=$(printf 'a%.0s' {1..100})
    local request="{
        \"model\": \"helixagent-debate\",
        \"messages\": [{\"role\": \"user\", \"content\": \"Count: $repeated_string\"}],
        \"max_tokens\": 20
    }"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "repeated" "handled" "true" "Long repeated strings handled"
    else
        record_assertion "repeated" "handled" "false" "Failed with $http_code"
    fi
}

# Test 7: Mixed language content
test_mixed_languages() {
    log_info "Test 7: Mixed language content"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Translate: Hello world (English), Hola mundo (Spanish), 你好世界 (Chinese)"}],
        "max_tokens": 50
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json; charset=utf-8" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "mixed_lang" "accepted" "true" "Mixed language content accepted"
    else
        record_assertion "mixed_lang" "accepted" "false" "Failed with $http_code"
    fi
}

# Test 8: Zero-width characters
test_zero_width_characters() {
    log_info "Test 8: Zero-width characters"

    # Zero-width space (U+200B)
    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Hello​world"}],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json; charset=utf-8" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "zero_width" "handled" "true" "Zero-width characters handled"
    else
        record_assertion "zero_width" "handled" "false" "Failed with $http_code"
    fi
}

# Test 9: Mathematical symbols
test_mathematical_symbols() {
    log_info "Test 9: Mathematical symbols"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "What is ∑ and ∫?"}],
        "max_tokens": 30
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json; charset=utf-8" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "math_symbols" "accepted" "true" "Math symbols accepted"
    else
        record_assertion "math_symbols" "accepted" "false" "Failed with $http_code"
    fi
}

# Test 10: Control characters (sanitized)
test_control_characters() {
    log_info "Test 10: Control characters handling"

    # Tab and newline
    local request='{"model": "helixagent-debate", "messages": [{"role": "user", "content": "Line1\tTab\nLine2"}], "max_tokens": 10}'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "control_chars" "handled" "true" "Control characters handled"
    else
        record_assertion "control_chars" "handled" "false" "Failed with $http_code"
    fi
}

# Main execution
main() {
    log_info "Starting Chat Special Characters Challenge..."

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
    test_unicode_characters
    test_emojis
    test_escaped_characters
    test_html_characters
    test_code_characters
    test_repeated_characters
    test_mixed_languages
    test_zero_width_characters
    test_mathematical_symbols
    test_control_characters

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
