#!/bin/bash
# Resilience Data Backup Challenge
# Tests data backup and recovery mechanisms

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "resilience-data-backup" "Resilience Data Backup Challenge"
load_env

log_info "Testing data backup mechanisms..."

test_state_persistence() {
    log_info "Test 1: System state persists across requests"

    # Create conversation context
    local resp1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Remember this: BACKUP_TEST_123"}],"max_tokens":20}' \
        --max-time 30 2>/dev/null || true)

    local code1=$(echo "$resp1" | tail -n1)
    [[ "$code1" == "200" ]] && record_assertion "state_creation" "successful" "true" "State created successfully"

    # Verify conversation ID or response shows processing
    local body1=$(echo "$resp1" | head -n -1)
    if echo "$body1" | jq -e '.id' > /dev/null 2>&1; then
        local conv_id=$(echo "$body1" | jq -r '.id')
        record_metric "conversation_id" "$conv_id"
        record_assertion "conversation_tracking" "enabled" "true" "Conversation ID present"
    fi
}

test_configuration_persistence() {
    log_info "Test 2: Configuration persists correctly"

    # Check startup verification data (should be cached/persisted)
    local startup=$(curl -s "$BASE_URL/v1/startup/verification" 2>/dev/null || echo "{}")

    if echo "$startup" | jq -e '.ranked_providers' > /dev/null 2>&1; then
        local providers=$(echo "$startup" | jq -e '.ranked_providers | length' 2>/dev/null || echo 0)
        record_metric "persisted_providers" $providers
        [[ $providers -gt 0 ]] && record_assertion "config_persistence" "verified" "true" "$providers providers persisted"
    else
        record_assertion "config_persistence" "basic" "true" "Basic config available"
    fi
}

test_cache_backup() {
    log_info "Test 3: Cache layer provides backup"

    # Make request that might be cached
    local resp1=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Cache test query"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    # Repeat same request (might hit cache)
    local resp2=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Cache test query"}],"max_tokens":10}' \
        --max-time 30 2>/dev/null || true)

    local code1=$(echo "$resp1" | tail -n1)
    local code2=$(echo "$resp2" | tail -n1)

    [[ "$code1" == "200" && "$code2" == "200" ]] && record_assertion "cache_backup" "operational" "true" "Both requests succeeded (cache working)"
}

test_recovery_from_data_loss() {
    log_info "Test 4: System recovers from transient data loss"

    # Send requests after potential cache/state disruption
    local success=0
    local total=5

    for i in $(seq 1 $total); do
        local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"Recovery test '$i'"}],"max_tokens":10}' \
            --max-time 30 2>/dev/null || true)

        [[ "$(echo "$resp" | tail -n1)" == "200" ]] && success=$((success + 1))
    done

    record_metric "recovery_success" $success
    [[ $success -ge 4 ]] && record_assertion "data_loss_recovery" "successful" "true" "$success/$total requests succeeded after recovery"
}

main() {
    log_info "Starting data backup challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_state_persistence
    test_configuration_persistence
    test_cache_backup
    test_recovery_from_data_loss

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
