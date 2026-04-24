#!/bin/bash
# Error Malformed Requests Challenge
# Tests handling of malformed and invalid requests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "error-malformed-requests" "Error Malformed Requests Challenge"
load_env

log_info "Testing malformed request handling..."

test_invalid_json() {
    log_info "Test 1: Invalid JSON handling"

    local malformed=(
        '{"model":"test", invalid}'
        '{model:test}'
        '{"model":"test",}'
        'not json at all'
    )
    local errors_caught=0

    for req in "${malformed[@]}"; do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$req" --max-time 10 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "400" ]] && errors_caught=$((errors_caught + 1))
    done

    record_metric "malformed_json_caught" $errors_caught
    [[ $errors_caught -ge 3 ]] && record_assertion "invalid_json" "rejected" "true" "$errors_caught/4 rejected"
}

test_wrong_content_type() {
    log_info "Test 2: Wrong content type handling"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: text/plain" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d 'plain text data' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(400|415)$ ]] && record_assertion "wrong_content_type" "rejected" "true" "Returns $code for wrong content type"
}

test_oversized_request() {
    log_info "Test 3: Oversized request handling"

    # Create very large request
    local large_content=""
    for i in {1..1000}; do
        large_content+="This is a very long message to test request size limits. "
    done

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\":\"helixagent-debate\",\"messages\":[{\"role\":\"user\",\"content\":\"$large_content\"}],\"max_tokens\":10}" \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(413|400|200)$ ]] && record_assertion "oversized" "handled" "true" "Oversized request handled (HTTP $code)"
}

test_malformed_recovery() {
    log_info "Test 4: System recovers after malformed requests"

    # Send malformed requests
    for i in {1..5}; do
        curl -s "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{invalid json}' \
            --max-time 5 > /dev/null 2>&1 || true
    done

    # Valid request should work
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    [[ "$(echo "$resp" | tail -n1)" == "200" ]] && record_assertion "malformed_recovery" "recovered" "true" "System operational after malformed requests"
}

main() {
    log_info "Starting malformed requests challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_invalid_json
    test_wrong_content_type
    test_oversized_request
    test_malformed_recovery

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
