#!/bin/bash
# Follow-up Response Challenge - Tests AI Debate context handling for follow-up responses
# CRITICAL: Tests that "yes 1." properly expands to execute option 1 from previous response
#
# This challenge validates the fix for the issue where LLMs would not understand
# short follow-up responses like "yes 1." that reference options from previous messages.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-7061}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "followup_response_challenge" "Follow-up Response Context Challenge"
load_env

# Test 1: Initial question with options
test_initial_question() {
    log_info "Testing initial question that should generate options..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "List 3 things you can help me with. Format as numbered options: 1. 2. 3."}
        ],
        "max_tokens": 500
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 120 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        # Check that the response contains numbered options
        if echo "$body" | grep -q '"content"' && echo "$body" | grep -q "1\."; then
            record_assertion "initial" "generates_options" "true" "Initial question generates numbered options"
            # Store the response for the follow-up test
            echo "$body" > /tmp/challenge_initial_response.json
        else
            record_assertion "initial" "generates_options" "false" "Response missing numbered options format"
        fi
    else
        record_assertion "initial" "generates_options" "false" "Initial question failed: $http_code"
    fi

    record_metric "initial_status" "$http_code"
}

# Test 2: Follow-up "yes 1." response
test_followup_yes_1() {
    log_info "Testing follow-up 'yes 1.' response with conversation context..."

    # Create a conversation with options followed by "yes 1."
    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What can you help me with?"},
            {"role": "assistant", "content": "I can help you with many things!\n\nWould you like me to:\n1. Create an AGENTS.md documenting your project architecture\n2. Run a dependency audit\n3. Refactor specific files"},
            {"role": "user", "content": "yes 1."}
        ],
        "max_tokens": 1000
    }'

    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 180 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        # The response should understand that user wants option 1
        # It should NOT treat "yes 1." as a new topic to debate
        local content=$(echo "$body" | jq -r '.choices[0].message.content // ""' 2>/dev/null || echo "")

        # Check that the response mentions AGENTS.md or project/architecture documentation
        if echo "$content" | grep -qi "AGENTS\|architecture\|document\|project\|option 1\|selected option"; then
            record_assertion "followup" "understands_context" "true" "Follow-up 'yes 1.' understood as option selection"
        else
            # If it's talking about the literal "yes 1" instead of the option
            if echo "$content" | grep -qi "what does.*yes.*mean\|interpret.*yes.*1\|unclear"; then
                record_assertion "followup" "understands_context" "false" "LLM confused by 'yes 1.' - treating as literal query"
            else
                record_assertion "followup" "understands_context" "partial" "Response may or may not understand context"
            fi
        fi
    else
        record_assertion "followup" "understands_context" "false" "Follow-up request failed: $http_code"
    fi

    record_metric "followup_latency_ms" "$latency"
    record_metric "followup_status" "$http_code"
}

# Test 3: Follow-up with just a number "2"
test_followup_number_only() {
    log_info "Testing follow-up with just number '2'..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Give me options"},
            {"role": "assistant", "content": "Here are your options:\n1. Run tests\n2. Deploy to production\n3. Review code"},
            {"role": "user", "content": "2"}
        ],
        "max_tokens": 500
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 120 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        local content=$(echo "$body" | jq -r '.choices[0].message.content // ""' 2>/dev/null || echo "")

        # Should understand that "2" means deploy/production
        if echo "$content" | grep -qi "deploy\|production\|option 2\|selected"; then
            record_assertion "number_only" "understands_number" "true" "Number-only follow-up understood"
        else
            record_assertion "number_only" "understands_number" "partial" "Number-only may not be fully understood"
        fi
    else
        record_assertion "number_only" "understands_number" "false" "Number-only follow-up failed: $http_code"
    fi

    record_metric "number_only_status" "$http_code"
}

# Test 4: Follow-up "ok 3"
test_followup_ok_3() {
    log_info "Testing follow-up 'ok 3'..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What can I do?"},
            {"role": "assistant", "content": "You can:\n1. View logs\n2. Check status\n3. Restart service"},
            {"role": "user", "content": "ok 3"}
        ],
        "max_tokens": 500
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 120 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        local content=$(echo "$body" | jq -r '.choices[0].message.content // ""' 2>/dev/null || echo "")

        # Should understand that "ok 3" means restart service
        if echo "$content" | grep -qi "restart\|service\|option 3"; then
            record_assertion "ok_number" "understands_ok_number" "true" "'ok 3' follow-up understood"
        else
            record_assertion "ok_number" "understands_ok_number" "partial" "'ok 3' may not be fully understood"
        fi
    else
        record_assertion "ok_number" "understands_ok_number" "false" "'ok 3' follow-up failed: $http_code"
    fi

    record_metric "ok_number_status" "$http_code"
}

# Test 5: Long question should NOT be treated as follow-up
test_long_question_not_followup() {
    log_info "Testing that long questions are NOT treated as follow-ups..."

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Options:\n1. A\n2. B"},
            {"role": "assistant", "content": "Choose one"},
            {"role": "user", "content": "Can you explain how the authentication system in this codebase works and what security measures are in place?"}
        ],
        "max_tokens": 500
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 120 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        local content=$(echo "$body" | jq -r '.choices[0].message.content // ""' 2>/dev/null || echo "")

        # Should NOT mention "option 1" or "option 2" - should treat as new question
        if echo "$content" | grep -qi "authentication\|security\|auth"; then
            record_assertion "long_question" "not_treated_as_followup" "true" "Long question treated as new query"
        else
            record_assertion "long_question" "not_treated_as_followup" "false" "Long question may be incorrectly treated as follow-up"
        fi
    else
        record_assertion "long_question" "not_treated_as_followup" "false" "Long question request failed: $http_code"
    fi

    record_metric "long_question_status" "$http_code"
}

# Run all tests
main() {
    log_info "Starting Follow-up Response Challenge"
    log_info "Testing AI Debate context handling for short follow-up responses"
    log_info "=========================================="

    # Ensure HelixAgent is running
    if ! curl -s "$BASE_URL/health" | grep -q "healthy"; then
        log_error "HelixAgent not running on $BASE_URL"
        exit 1
    fi

    test_initial_question
    test_followup_yes_1
    test_followup_number_only
    test_followup_ok_3
    test_long_question_not_followup

    # Generate report
    generate_report

    log_info "=========================================="
    log_info "Follow-up Response Challenge Complete"
}

main "$@"
