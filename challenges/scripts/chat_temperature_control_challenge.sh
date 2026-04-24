#!/bin/bash
# Chat Temperature Control Challenge
# Tests temperature parameter effects on response variation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

# Initialize challenge
init_challenge "chat-temperature-control" "Chat Temperature Control Challenge"
load_env

log_info "Testing temperature parameter effects on chat responses..."

# Test 1: Low temperature (deterministic)
test_low_temperature() {
    log_info "Test 1: Low temperature (0.1) - should be deterministic"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Say the word hello"}
        ],
        "max_tokens": 20,
        "temperature": 0.1
    }'

    # Make 3 requests with same parameters
    local responses=()
    for i in {1..3}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 60 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        if [[ "$http_code" == "200" ]]; then
            local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
            responses+=("$content")
        fi
    done

    # With low temperature, responses should be similar
    if [[ ${#responses[@]} -eq 3 ]]; then
        record_assertion "low_temp" "requests_completed" "true" "All 3 requests completed"
        record_metric "low_temp_response_count" ${#responses[@]}

        # Check if all responses contain "hello" (case-insensitive)
        local contains_hello=0
        for resp in "${responses[@]}"; do
            if echo "$resp" | grep -qi "hello"; then
                contains_hello=$((contains_hello + 1))
            fi
        done

        if [[ $contains_hello -eq 3 ]]; then
            record_assertion "low_temp" "consistent_content" "true" "All responses contain expected content"
        else
            record_assertion "low_temp" "consistent_content" "false" "Only $contains_hello/3 responses contain expected content"
        fi
    else
        record_assertion "low_temp" "requests_completed" "false" "Only ${#responses[@]}/3 requests completed"
    fi
}

# Test 2: High temperature (creative)
test_high_temperature() {
    log_info "Test 2: High temperature (1.5) - should be more varied"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Write one creative word"}
        ],
        "max_tokens": 20,
        "temperature": 1.5
    }'

    # Make 3 requests
    local response_count=0
    for i in {1..3}; do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 60 2>/dev/null || true)

        local http_code=$(echo "$response" | tail -n1)
        if [[ "$http_code" == "200" ]]; then
            response_count=$((response_count + 1))
        fi
    done

    if [[ $response_count -eq 3 ]]; then
        record_assertion "high_temp" "requests_completed" "true" "All 3 high-temp requests completed"
        record_metric "high_temp_response_count" $response_count
    else
        record_assertion "high_temp" "requests_completed" "false" "Only $response_count/3 requests completed"
    fi
}

# Test 3: Temperature = 0 (maximum determinism)
test_zero_temperature() {
    log_info "Test 3: Temperature 0 - maximum determinism"

    local request='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "What is 5+7? Answer with just the number."}
        ],
        "max_tokens": 10,
        "temperature": 0
    }'

    local start_time=$(date +%s%N)
    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request" --max-time 60 2>/dev/null || true)
    local end_time=$(date +%s%N)
    local latency=$(( (end_time - start_time) / 1000000 ))

    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [[ "$http_code" == "200" ]]; then
        record_assertion "zero_temp" "http_status" "true" "Temperature 0 request succeeded"
        record_metric "zero_temp_latency_ms" $latency

        # Check if response contains the correct answer
        if echo "$body" | grep -q '"content"'; then
            local content=$(echo "$body" | jq -r '.choices[0].message.content // empty')
            if echo "$content" | grep -q "12"; then
                record_assertion "zero_temp" "correct_answer" "true" "Deterministic response is correct"
            else
                record_assertion "zero_temp" "correct_answer" "false" "Response: $content"
            fi
        fi
    else
        record_assertion "zero_temp" "http_status" "false" "HTTP $http_code"
    fi
}

# Test 4: Temperature boundary values
test_temperature_boundaries() {
    log_info "Test 4: Temperature boundary values"

    # Test temperature = 2.0 (maximum)
    local request_max='{
        "model": "helixagent-debate",
        "messages": [
            {"role": "user", "content": "Hi"}
        ],
        "max_tokens": 10,
        "temperature": 2.0
    }'

    local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$request_max" --max-time 60 2>/dev/null || true)

    local http_code=$(echo "$response" | tail -n1)
    if [[ "$http_code" == "200" ]]; then
        record_assertion "temp_boundary" "max_temp_accepted" "true" "Temperature 2.0 accepted"
    else
        # Some providers may not accept 2.0, which is OK
        record_assertion "temp_boundary" "max_temp_accepted" "false" "HTTP $http_code (may be expected)"
    fi
}

# Test 5: Temperature effect on latency
test_temperature_latency() {
    log_info "Test 5: Temperature effect on response latency"

    local temps=(0 0.5 1.0)
    local latencies=()

    for temp in "${temps[@]}"; do
        local request="{
            \"model\": \"helixagent-debate\",
            \"messages\": [{\"role\": \"user\", \"content\": \"Hi\"}],
            \"max_tokens\": 10,
            \"temperature\": $temp
        }"

        local start_time=$(date +%s%N)
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$request" --max-time 60 2>/dev/null || true)
        local end_time=$(date +%s%N)
        local latency=$(( (end_time - start_time) / 1000000 ))

        local http_code=$(echo "$response" | tail -n1)
        if [[ "$http_code" == "200" ]]; then
            latencies+=($latency)
            record_metric "temp_${temp}_latency_ms" $latency
        fi
    done

    if [[ ${#latencies[@]} -eq 3 ]]; then
        record_assertion "temp_latency" "all_temps_tested" "true" "All temperature values tested"

        # Calculate average latency
        local sum=0
        for lat in "${latencies[@]}"; do
            sum=$((sum + lat))
        done
        local avg=$((sum / ${#latencies[@]}))
        record_metric "avg_latency_across_temps_ms" $avg
    else
        record_assertion "temp_latency" "all_temps_tested" "false" "Only ${#latencies[@]}/3 temperatures tested"
    fi
}

# Main execution
main() {
    log_info "Starting temperature control challenge..."

    # Ensure HelixAgent is running
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        log_info "HelixAgent not running, starting..."
        start_helixagent "$CHALLENGE_PORT" || {
            log_error "Failed to start HelixAgent"
            finalize_challenge "FAILED"
            exit 1
        }
    fi

    # Run all tests
    test_low_temperature
    test_high_temperature
    test_zero_temperature
    test_temperature_boundaries
    test_temperature_latency

    # Check results
    local failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)

    if [[ $failed_count -eq 0 ]]; then
        log_info "All temperature control tests passed!"
        finalize_challenge "PASSED"
    else
        log_error "$failed_count test(s) failed"
        finalize_challenge "FAILED"
    fi
}

main "$@"
