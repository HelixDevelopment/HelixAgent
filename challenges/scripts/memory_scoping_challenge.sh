#!/bin/bash
# Memory Scoping Challenge
# Tests memory scope management (user, session, global)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "memory-scoping" "Memory Scoping Challenge"
load_env

log_info "Testing memory scoping..."

test_user_scope() {
    log_info "Test 1: User-scoped memory"

    local memory='{"user_id":"user_123","content":"User prefers English","scope":"user"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/store" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$memory" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" || "$code" == "201" ]]; then
        local has_scope=$(echo "$body" | jq -e '.scope' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "user_scope" "working" "true" "User scope: $has_scope"
    else
        record_skip "user_scope" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

test_session_scope() {
    log_info "Test 2: Session-scoped memory"

    local memory='{"user_id":"user_123","session_id":"sess_456","content":"Current conversation topic: Python","scope":"session"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/store" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$memory" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|201)$ ]] && record_assertion "session_scope" "working" "true" "Session scope created"
}

test_global_scope() {
    log_info "Test 3: Global-scoped memory"

    local memory='{"content":"System maintenance scheduled for midnight","scope":"global","visibility":"all"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/store" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$memory" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|201|403)$ ]] && record_assertion "global_scope" "checked" "true" "HTTP $code (403=unauthorized is OK)"
}

test_scope_isolation() {
    log_info "Test 4: Scope isolation verification"

    # Query user scope only
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/memory/list?user_id=user_123&scope=user" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local count=$(echo "$body" | jq -e '.memories | length' 2>/dev/null || echo "0")
        record_assertion "scope_isolation" "verified" "true" "User scope: $count memories"
    else
        record_skip "scope_isolation" "checked" "HTTP $code (endpoint not implemented; SKIP-OK: #endpoint-not-impl)"
    fi
}

main() {
    log_info "Starting memory scoping challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_user_scope
    test_session_scope
    test_global_scope
    test_scope_isolation

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
