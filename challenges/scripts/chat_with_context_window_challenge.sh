#!/bin/bash
# Chat with Context Window Challenge
# Tests handling of large context windows and context management

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_context_window" "Chat with Context Window Challenge"
load_env

log_info "Testing context window handling and management..."

# Test 1: Large context input
test_large_context() {
    log_info "Test 1: Large context input handling"

    # Generate a large context (simulate a long document)
    local large_text="This is a test document. "
    for i in {1..100}; do
        large_text="${large_text}Paragraph $i contains important information about topic $i. "
    done
    large_text="${large_text}The key information is in paragraph 50."

    local request=$(cat <<EOF
{
    "model": "helixagent-debate",
    "messages": [
        {"role": "user", "content": "Here is a document: $large_text"},
        {"role": "user", "content": "What paragraph contains the key information?"}
    ],
    "max_tokens": 50,
    "temperature": 0.1
}
EOF
)

    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 90 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "large_context" "http_status" "true" "Large context request succeeded"

        # Check if response mentions paragraph 50
        if echo "$body" | grep -qiE "(paragraph 50|50|fifty)"; then
            record_assertion "large_context" "context_retention" "true" "Context retained (found paragraph 50)"
        else
            record_assertion "large_context" "context_retention" "false" "Context retention unclear"
        fi
    else
        record_assertion "large_context" "http_status" "false" "Request failed with $http_code"
    fi

    record_metric "large_context_latency_ms" "$latency"
}

# Test 2: Context window limit handling
test_context_limit() {
    log_info "Test 2: Context window limit handling"

    # Build a very long message list
    local messages='[{"role": "system", "content": "You are a helpful assistant."}]'

    # Add many short messages to approach context limit
    for i in {1..50}; do
        messages=$(echo "$messages" | jq ". + [{\"role\": \"user\", \"content\": \"Message $i\"}, {\"role\": \"assistant\", \"content\": \"Response $i\"}]")
    done

    messages=$(echo "$messages" | jq ". + [{\"role\": \"user\", \"content\": \"Summarize our conversation\"}]")

    local request="{\"model\": \"helixagent-debate\", \"messages\": $messages, \"max_tokens\": 100, \"temperature\": 0.1}"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 120 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    # Either succeeds (context managed) or fails gracefully with error
    if [[ "$http_code" == "200" ]]; then
        record_assertion "context_limit" "handling" "true" "Context limit handled successfully (200)"
    elif [[ "$http_code" == "400" ]]; then
        # 400 with context_length_exceeded is acceptable
        if echo "$body" | grep -qi "context"; then
            record_assertion "context_limit" "handling" "true" "Context limit reported gracefully (400)"
        else
            record_assertion "context_limit" "handling" "false" "Unexpected 400 error"
        fi
    else
        record_assertion "context_limit" "handling" "false" "Unexpected status: $http_code"
    fi
}

# Test 3: Context summarization (if supported)
test_context_summarization() {
    log_info "Test 3: Context summarization ability"

    # Provide context with multiple facts
    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Fact 1: The sky is blue. Fact 2: Water boils at 100C. Fact 3: Earth orbits the Sun. Fact 4: Light travels fast. Fact 5: Trees produce oxygen."},
            {"role": "user", "content": "Summarize the facts in one sentence."}
        ],
        "max_tokens": 100,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "summarization" "http_status" "true" "Summarization request succeeded"

        # Check if summary is concise (less than original)
        local content=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local word_count=$(echo "$content" | wc -w)

        if [[ $word_count -lt 50 ]]; then
            record_assertion "summarization" "concise" "true" "Summary is concise ($word_count words)"
        else
            record_assertion "summarization" "concise" "false" "Summary may be too long ($word_count words)"
        fi
    else
        record_assertion "summarization" "http_status" "false" "Request failed with $http_code"
    fi
}

# Test 4: Context retrieval from history
test_context_retrieval() {
    log_info "Test 4: Context retrieval from conversation history"

    # Build conversation with specific information in early messages
    local messages='[
        {"role": "user", "content": "My favorite color is blue."},
        {"role": "assistant", "content": "Noted that your favorite color is blue."}
    ]'

    # Add several unrelated messages
    for i in {1..10}; do
        messages=$(echo "$messages" | jq ". + [{\"role\": \"user\", \"content\": \"What is $i plus $i?\"}, {\"role\": \"assistant\", \"content\": \"The answer is $((i + i)).\"}]")
    done

    # Ask about the original information
    messages=$(echo "$messages" | jq ". + [{\"role\": \"user\", \"content\": \"What is my favorite color?\"}]")

    local request="{\"model\": \"helixagent-debate\", \"messages\": $messages, \"max_tokens\": 20, \"temperature\": 0.1}"

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "retrieval" "http_status" "true" "Retrieval request succeeded"

        # Check if response mentions blue
        if echo "$body" | grep -qi "blue"; then
            record_assertion "retrieval" "early_context" "true" "Retrieved early context (blue)"
        else
            record_assertion "retrieval" "early_context" "false" "May not have retrieved early context"
        fi
    else
        record_assertion "retrieval" "http_status" "false" "Request failed with $http_code"
    fi
}

# Test 5: Mixed context types (user, assistant, system)
test_mixed_context() {
    log_info "Test 5: Mixed context types handling"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are a math tutor."},
            {"role": "user", "content": "What is 5 times 5?"},
            {"role": "assistant", "content": "5 times 5 equals 25."},
            {"role": "user", "content": "And what is 3 times 3?"},
            {"role": "assistant", "content": "3 times 3 equals 9."},
            {"role": "user", "content": "What was the first question I asked?"}
        ],
        "max_tokens": 50,
        "temperature": 0.1
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "mixed_context" "http_status" "true" "Mixed context request succeeded"

        # Check if response mentions the first question
        if echo "$body" | grep -qiE "(5 times 5|5.?5|25)"; then
            record_assertion "mixed_context" "history_recall" "true" "Recalled conversation history"
        else
            record_assertion "mixed_context" "history_recall" "false" "History recall unclear"
        fi
    else
        record_assertion "mixed_context" "http_status" "false" "Request failed with $http_code"
    fi
}

# Test 6: Empty message handling
test_empty_messages() {
    log_info "Test 6: Empty message handling"

    # Test with empty content (should be rejected)
    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": ""}
        ],
        "max_tokens": 10
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 30 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)

    # Should reject empty messages (400) or handle gracefully (200)
    if [[ "$http_code" == "400" ]]; then
        record_assertion "empty_msg" "validation" "true" "Empty message rejected (400)"
    elif [[ "$http_code" == "200" ]]; then
        record_assertion "empty_msg" "validation" "true" "Empty message handled gracefully (200)"
    else
        record_assertion "empty_msg" "validation" "false" "Unexpected status: $http_code"
    fi
}

# Main execution
main() {
    log_info "Starting Chat with Context Window Challenge..."

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
    test_large_context
    test_context_limit
    test_context_summarization
    test_context_retrieval
    test_mixed_context
    test_empty_messages

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
