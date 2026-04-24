#!/bin/bash
# Simple Chat Multi-Turn Challenge
# Tests multi-turn conversation with context preservation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "simple_chat_multi_turn" "Simple Chat Multi-Turn Challenge"
load_env

log_info "Testing multi-turn chat conversations with context..."

# Test 1: Basic two-turn conversation
test_two_turn_conversation() {
    log_info "Test 1: Basic two-turn conversation with context"

    # First turn: Introduce a topic
    local request1='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "My name is Alice and I like programming."}
        ],
        "max_tokens": 50,
        "temperature": 0.1
    }'

    local start_time=$(date +%s%N)
    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 60 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency1=$(( (end_time - start_time) / 1000000 ))

    local http_code1=$(echo "$response1" | tail -n1)
    local body1=$(echo "$response1" | head -n -1)

    if [[ "$http_code1" == "200" ]]; then
        record_assertion "two_turn" "first_turn" "true" "First turn succeeded"

        # Extract assistant's response
        local assistant_msg=$(echo "$body1" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")

        # Second turn: Ask about the previous context
        local request2=$(cat <<EOF
{
    "model": "helixagent-debate",
    "messages": [
        {"role": "user", "content": "My name is Alice and I like programming."},
        {"role": "assistant", "content": $(echo "$assistant_msg" | jq -R .)},
        {"role": "user", "content": "What is my name?"}
    ],
    "max_tokens": 20,
    "temperature": 0.1
}
EOF
)

        local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request2" --max-time 60 2>/dev/null || true)

        local http_code2=$(echo "$response2" | tail -n1)
        local body2=$(echo "$response2" | head -n -1)

        if [[ "$http_code2" == "200" ]]; then
            record_assertion "two_turn" "second_turn" "true" "Second turn succeeded"

            # Check if the response mentions Alice
            if echo "$body2" | grep -qi "alice"; then
                record_assertion "two_turn" "context_preserved" "true" "Context preserved (mentioned Alice)"
            else
                record_assertion "two_turn" "context_preserved" "false" "Context may not be preserved"
            fi
        else
            record_assertion "two_turn" "second_turn" "false" "Second turn failed with $http_code2"
        fi
    else
        record_assertion "two_turn" "first_turn" "false" "First turn failed with $http_code1"
    fi

    record_metric "first_turn_latency_ms" "$latency1"
}

# Test 2: Three-turn conversation with cumulative context
test_three_turn_conversation() {
    log_info "Test 2: Three-turn conversation with cumulative context"

    local messages='[{"role": "user", "content": "I have 5 apples."}]'

    # Turn 1
    local request1="{\"model\": \"helixagent-debate\", \"messages\": $messages, \"max_tokens\": 30, \"temperature\": 0.1}"
    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 60 2>/dev/null || true)

    local http_code1=$(echo "$response1" | tail -n1)
    local body1=$(echo "$response1" | head -n -1)

    if [[ "$http_code1" == "200" ]]; then
        local msg1=$(echo "$body1" | jq -r '.choices[0].message.content' 2>/dev/null || echo "acknowledged")
        messages=$(echo "$messages" | jq ". + [{\"role\": \"assistant\", \"content\": \"$msg1\"}]")
        messages=$(echo "$messages" | jq ". + [{\"role\": \"user\", \"content\": \"I bought 3 more apples.\"}]")

        # Turn 2
        local request2="{\"model\": \"helixagent-debate\", \"messages\": $messages, \"max_tokens\": 30, \"temperature\": 0.1}"
        local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request2" --max-time 60 2>/dev/null || true)

        local http_code2=$(echo "$response2" | tail -n1)
        local body2=$(echo "$response2" | head -n -1)

        if [[ "$http_code2" == "200" ]]; then
            local msg2=$(echo "$body2" | jq -r '.choices[0].message.content' 2>/dev/null || echo "acknowledged")
            messages=$(echo "$messages" | jq ". + [{\"role\": \"assistant\", \"content\": \"$msg2\"}]")
            messages=$(echo "$messages" | jq ". + [{\"role\": \"user\", \"content\": \"How many apples do I have now?\"}]")

            # Turn 3
            local request3="{\"model\": \"helixagent-debate\", \"messages\": $messages, \"max_tokens\": 20, \"temperature\": 0.1}"
            local response3=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d "$request3" --max-time 60 2>/dev/null || true)

            local http_code3=$(echo "$response3" | tail -n1)
            local body3=$(echo "$response3" | head -n -1)

            if [[ "$http_code3" == "200" ]]; then
                record_assertion "three_turn" "all_turns" "true" "All three turns succeeded"

                # Check if the answer is 8 (5 + 3)
                if echo "$body3" | grep -qE "8|eight"; then
                    record_assertion "three_turn" "cumulative_context" "true" "Cumulative context preserved (correct answer: 8)"
                else
                    record_assertion "three_turn" "cumulative_context" "false" "Cumulative context may not be preserved"
                fi
            else
                record_assertion "three_turn" "all_turns" "false" "Third turn failed with $http_code3"
            fi
        else
            record_assertion "three_turn" "all_turns" "false" "Second turn failed with $http_code2"
        fi
    else
        record_assertion "three_turn" "all_turns" "false" "First turn failed with $http_code1"
    fi
}

# Test 3: Multi-turn with system message consistency
test_multi_turn_with_system() {
    log_info "Test 3: Multi-turn conversation with system message"

    # First turn with system message
    local request1='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant. Always respond in uppercase."},
            {"role": "user", "content": "Hello"}
        ],
        "max_tokens": 30,
        "temperature": 0.1
    }'

    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 60 2>/dev/null || true)

    local http_code1=$(echo "$response1" | tail -n1)
    local body1=$(echo "$response1" | head -n -1)

    if [[ "$http_code1" == "200" ]]; then
        record_assertion "system_msg" "first_with_system" "true" "First turn with system message succeeded"

        local msg1=$(echo "$body1" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")

        # Second turn (system message should still apply)
        local request2=$(cat <<EOF
{
    "model": "helixagent-debate",
    "messages": [
        {"role": "system", "content": "You are a helpful assistant. Always respond in uppercase."},
        {"role": "user", "content": "Hello"},
        {"role": "assistant", "content": $(echo "$msg1" | jq -R .)},
        {"role": "user", "content": "What is 2+2?"}
    ],
    "max_tokens": 20,
    "temperature": 0.1
}
EOF
)

        local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request2" --max-time 60 2>/dev/null || true)

        local http_code2=$(echo "$response2" | tail -n1)
        local body2=$(echo "$response2" | head -n -1)

        if [[ "$http_code2" == "200" ]]; then
            record_assertion "system_msg" "second_with_system" "true" "Second turn with system message succeeded"

            # Check if response contains uppercase content (system message followed)
            local content=$(echo "$body2" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
            if echo "$content" | grep -qE "[A-Z]"; then
                record_assertion "system_msg" "system_consistency" "true" "System message appears to be followed"
            else
                record_assertion "system_msg" "system_consistency" "false" "System message may not be consistent"
            fi
        else
            record_assertion "system_msg" "second_with_system" "false" "Second turn failed with $http_code2"
        fi
    else
        record_assertion "system_msg" "first_with_system" "false" "First turn failed with $http_code1"
    fi
}

# Test 4: Long conversation (5+ turns)
test_long_conversation() {
    log_info "Test 4: Long conversation (5+ turns)"

    local messages='[]'
    local turn_count=0
    local all_succeeded=true

    # Simulate 5 turns
    for i in {1..5}; do
        if [[ $i -eq 1 ]]; then
            messages='[{"role": "user", "content": "Count from 1"}]'
        else
            messages=$(echo "$messages" | jq ". + [{\"role\": \"user\", \"content\": \"Continue\"}]")
        fi

        local request="{\"model\": \"helixagent-debate\", \"messages\": $messages, \"max_tokens\": 20, \"temperature\": 0.1}"
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 60 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)

        if [[ "$http_code" == "200" ]]; then
            turn_count=$((turn_count + 1))
            local body=$(echo "$response" | head -n -1)
            local msg=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null || echo "response")
            messages=$(echo "$messages" | jq ". + [{\"role\": \"assistant\", \"content\": \"$msg\"}]")
        else
            all_succeeded=false
            break
        fi
    done

    if [[ "$all_succeeded" == "true" ]] && [[ $turn_count -eq 5 ]]; then
        record_assertion "long_conv" "five_turns" "true" "All 5 turns succeeded"
    else
        record_assertion "long_conv" "five_turns" "false" "Only $turn_count turns succeeded"
    fi

    record_metric "completed_turns" "$turn_count"
}

# Main execution
main() {
    log_info "Starting Simple Chat Multi-Turn Challenge..."

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
    test_two_turn_conversation
    test_three_turn_conversation
    test_multi_turn_with_system
    test_long_conversation

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
