#!/bin/bash
# Chat Conversation Branching Challenge
# Tests handling of conversation branches and context switching

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat_conversation_branching" "Chat Conversation Branching Challenge"
load_env

log_info "Testing conversation branching and context management..."

# Test 1: Branch from conversation history
test_conversation_branch() {
    log_info "Test 1: Branch from conversation midpoint"

    # Common history
    local base_messages='[
        {"role": "user", "content": "I like cats"},
        {"role": "assistant", "content": "Cats are great pets!"}
    ]'

    # Branch A: Continue with cats
    local branch_a=$(echo "$base_messages" | jq '. + [{"role": "user", "content": "What breeds are there?"}]')
    local request_a="{\"model\": \"helixagent-debate\", \"messages\": $branch_a, \"max_tokens\": 30}"

    local response_a=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_a" --max-time 30 2>/dev/null || true)

    # Branch B: Switch to dogs
    local branch_b=$(echo "$base_messages" | jq '. + [{"role": "user", "content": "What about dogs?"}]')
    local request_b="{\"model\": \"helixagent-debate\", \"messages\": $branch_b, \"max_tokens\": 30}"

    local response_b=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_b" --max-time 30 2>/dev/null || true)

    local http_code_a=$(echo "$response_a" | tail -n1)
    local http_code_b=$(echo "$response_b" | tail -n1)

    if [[ "$http_code_a" == "200" ]] && [[ "$http_code_b" == "200" ]]; then
        record_assertion "branch" "both_branches" "true" "Both conversation branches work"

        # Check content relevance
        local body_a=$(echo "$response_a" | head -n -1)
        local body_b=$(echo "$response_b" | head -n -1)

        local content_a=$(echo "$body_a" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
        local content_b=$(echo "$body_b" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")

        # Branch A should mention cats/breeds
        if echo "$content_a" | grep -qiE "(cat|breed|persian|siamese)"; then
            record_assertion "branch" "branch_a_relevant" "true" "Branch A stays on topic (cats)"
        else
            record_assertion "branch" "branch_a_relevant" "false" "Branch A may be off-topic"
        fi

        # Branch B should mention dogs
        if echo "$content_b" | grep -qiE "(dog|canine)"; then
            record_assertion "branch" "branch_b_relevant" "true" "Branch B follows new topic (dogs)"
        else
            record_assertion "branch" "branch_b_relevant" "false" "Branch B may not follow topic change"
        fi
    else
        record_assertion "branch" "both_branches" "false" "One branch failed ($http_code_a, $http_code_b)"
    fi
}

# Test 2: Multiple branches from same root
test_multiple_branches() {
    log_info "Test 2: Multiple branches from same root"

    local root='[{"role": "user", "content": "Tell me about numbers"}]'

    # Branch 1: Prime numbers
    local branch1=$(echo "$root" | jq '. + [{"role": "assistant", "content": "Numbers are fundamental."}, {"role": "user", "content": "What are primes?"}]')
    local request1="{\"model\": \"helixagent-debate\", \"messages\": $branch1, \"max_tokens\": 30}"

    # Branch 2: Even numbers
    local branch2=$(echo "$root" | jq '. + [{"role": "assistant", "content": "Numbers are fundamental."}, {"role": "user", "content": "What are evens?"}]')
    local request2="{\"model\": \"helixagent-debate\", \"messages\": $branch2, \"max_tokens\": 30}"

    # Branch 3: Negative numbers
    local branch3=$(echo "$root" | jq '. + [{"role": "assistant", "content": "Numbers are fundamental."}, {"role": "user", "content": "What about negatives?"}]')
    local request3="{\"model\": \"helixagent-debate\", \"messages\": $branch3, \"max_tokens\": 30}"

    local success_count=0

    local response1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request1" --max-time 30 2>/dev/null || true)
    [[ $(echo "$response1" | tail -n1) == "200" ]] && success_count=$((success_count + 1))

    local response2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request2" --max-time 30 2>/dev/null || true)
    [[ $(echo "$response2" | tail -n1) == "200" ]] && success_count=$((success_count + 1))

    local response3=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request3" --max-time 30 2>/dev/null || true)
    [[ $(echo "$response3" | tail -n1) == "200" ]] && success_count=$((success_count + 1))

    if [[ $success_count -eq 3 ]]; then
        record_assertion "multi_branch" "all_succeed" "true" "All 3 branches from same root work"
    else
        record_assertion "multi_branch" "all_succeed" "false" "Only $success_count/3 branches succeeded"
    fi

    record_metric "successful_branches" "$success_count"
}

# Test 3: Deep vs shallow branching
test_deep_branching() {
    log_info "Test 3: Deep vs shallow branching"

    # Build a deep conversation
    local deep_messages='[{"role": "user", "content": "Start"}]'
    for i in {1..5}; do
        deep_messages=$(echo "$deep_messages" | jq ". + [{\"role\": \"assistant\", \"content\": \"Response $i\"}, {\"role\": \"user\", \"content\": \"Continue $i\"}]")
    done

    # Branch from deep point
    deep_messages=$(echo "$deep_messages" | jq '. + [{"role": "user", "content": "Final question"}]')

    local request_deep="{\"model\": \"helixagent-debate\", \"messages\": $deep_messages, \"max_tokens\": 20}"

    local response_deep=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_deep" --max-time 30 2>/dev/null || true)

    local http_code_deep=$(echo "$response_deep" | tail -n1)

    if [[ "$http_code_deep" == "200" ]]; then
        record_assertion "deep_branch" "works" "true" "Deep conversation branching works"
    else
        record_assertion "deep_branch" "works" "false" "Deep branch failed ($http_code_deep)"
    fi
}

# Test 4: Branching with system message changes
test_system_message_branching() {
    log_info "Test 4: Branching with different system messages"

    local base='[{"role": "user", "content": "Hello"}]'

    # Branch A: Formal system message
    local branch_formal='[{"role": "system", "content": "You are a formal assistant."}]'
    branch_formal=$(echo "$branch_formal" | jq ". + $(echo "$base" | jq '.')")
    local request_formal="{\"model\": \"helixagent-debate\", \"messages\": $branch_formal, \"max_tokens\": 20}"

    # Branch B: Casual system message
    local branch_casual='[{"role": "system", "content": "You are a casual assistant."}]'
    branch_casual=$(echo "$branch_casual" | jq ". + $(echo "$base" | jq '.')")
    local request_casual="{\"model\": \"helixagent-debate\", \"messages\": $branch_casual, \"max_tokens\": 20}"

    local response_formal=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_formal" --max-time 30 2>/dev/null || true)

    local response_casual=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_casual" --max-time 30 2>/dev/null || true)

    local http_code_formal=$(echo "$response_formal" | tail -n1)
    local http_code_casual=$(echo "$response_casual" | tail -n1)

    if [[ "$http_code_formal" == "200" ]] && [[ "$http_code_casual" == "200" ]]; then
        record_assertion "system_branch" "both_work" "true" "Both system message variants work"
    else
        record_assertion "system_branch" "both_work" "false" "One failed ($http_code_formal, $http_code_casual)"
    fi
}

# Test 5: Rapid branching (stress test)
test_rapid_branching() {
    log_info "Test 5: Rapid branching (10 branches)"

    local root='[{"role": "user", "content": "Root"}]'
    local success_count=0

    for i in {1..10}; do
        local branch=$(echo "$root" | jq ". + [{\"role\": \"assistant\", \"content\": \"OK\"}, {\"role\": \"user\", \"content\": \"Branch $i\"}]")
        local request="{\"model\": \"helixagent-debate\", \"messages\": $branch, \"max_tokens\": 10}"

        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 30 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        [[ "$http_code" == "200" ]] && success_count=$((success_count + 1))

        sleep 0.1
    done

    if [[ $success_count -ge 9 ]]; then
        record_assertion "rapid_branch" "high_success" "true" "$success_count/10 branches succeeded"
    elif [[ $success_count -ge 7 ]]; then
        record_assertion "rapid_branch" "high_success" "false" "Only $success_count/10 branches (acceptable)"
    else
        record_assertion "rapid_branch" "high_success" "false" "Only $success_count/10 branches (poor)"
    fi

    record_metric "rapid_branch_success" "$success_count"
}

# Main execution
main() {
    log_info "Starting Chat Conversation Branching Challenge..."

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
    test_conversation_branch
    test_multiple_branches
    test_deep_branching
    test_system_message_branching
    test_rapid_branching

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
