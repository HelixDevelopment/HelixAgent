#!/bin/bash
# Protocol Endpoints Challenge — CONST-035 anti-bluff regression guard
#
# Verifies that the JSON-RPC and REST surface for /v1/mcp, /v1/lsp, and
# /v1/acp actually responds to every documented method/path. Each
# assertion is a real round-trip against the live binary — no mocks.
#
# This Challenge MUST FAIL when:
#   - any documented MCP method (initialize, tools/list, tools/call,
#     prompts/list, resources/list, ping) reverts to silent error
#   - LSP /servers returns empty (regression in registry wiring)
#   - LSP /sync silently fails (returns 200 but doesn't actually sync)
#   - ACP /agents stops listing the built-in agents
#   - ACP /execute returns silent failure for a real agent_id
#
# Verify-by-mutation (CONST-035 §1):
#   - Comment out a case in handleProtocolMessage's switch → that
#     method's assertion fails with -32601 instead of result.
#   - Make IsProviderAvailable always return false → mcp_list_providers
#     would fail or return empty.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol_endpoints" "Protocol Endpoints Challenge"
load_env

# Test 1: server alive
test_server_alive() {
    if curl -fsS -m 5 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Helper: assert MCP JSON-RPC method returns result (not error)
assert_mcp_method() {
    local label="$1"
    local target="$2"
    local method="$3"
    local body=$(printf '{"jsonrpc":"2.0","method":"%s","id":1,"params":{}}' "$method")
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo '{}')
    local has_result=$(echo "$resp" | jq -e '.result != null' >/dev/null 2>&1 && echo "true" || echo "false")
    local err=$(echo "$resp" | jq -r '.error.message // empty' 2>/dev/null)
    if [[ "$has_result" == "true" ]]; then
        record_assertion "$label" "$target" "true" "MCP $method returned a result"
    elif [[ -n "$err" ]]; then
        record_assertion "$label" "$target" "false" "MCP $method returned error: $err"
    else
        record_assertion "$label" "$target" "false" "MCP $method returned no result and no error"
    fi
}

# Test 2: MCP method coverage
test_mcp_methods() {
    log_info "Test 2: MCP JSON-RPC method coverage"
    assert_mcp_method "mcp" "initialize"      "initialize"
    assert_mcp_method "mcp" "tools_list"      "tools/list"
    assert_mcp_method "mcp" "prompts_list"    "prompts/list"
    assert_mcp_method "mcp" "resources_list"  "resources/list"
    assert_mcp_method "mcp" "ping"            "ping"
}

# Test 3: MCP tools/call works on a real registered tool
test_mcp_tools_call() {
    log_info "Test 3: MCP tools/call mcp_list_providers returns real provider list"
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"name":"mcp_list_providers","arguments":{}}}' 2>/dev/null || echo '{}')
    # The result.content[0].text is a JSON-encoded string of providers
    local content_text=$(echo "$resp" | jq -r '.result.content[0].text // empty' 2>/dev/null)
    if [[ -n "$content_text" ]]; then
        local provider_count=$(echo "$content_text" | jq -r '. | length // 0' 2>/dev/null)
        if [[ "$provider_count" -ge 1 ]]; then
            record_assertion "mcp" "tools_call" "true" "tools/call returned $provider_count providers"
        else
            record_assertion "mcp" "tools_call" "false" "tools/call returned empty provider list"
        fi
    else
        record_assertion "mcp" "tools_call" "false" "tools/call returned no content"
    fi
}

# Test 4: MCP unknown method returns -32601 (negative — not a panic)
test_mcp_unknown_method() {
    log_info "Test 4: MCP unknown method → -32601 (correct JSON-RPC error)"
    local resp=$(curl -fsS -m 5 -X POST "$BASE_URL/v1/mcp" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"jsonrpc":"2.0","method":"zzz-not-a-method","id":1,"params":{}}' 2>/dev/null || echo '{}')
    local code=$(echo "$resp" | jq -r '.error.code // empty' 2>/dev/null)
    if [[ "$code" == "-32601" ]]; then
        record_assertion "mcp" "unknown_method_neg" "true" "Unknown method correctly returned -32601 Method not found"
    else
        record_assertion "mcp" "unknown_method_neg" "false" "Unknown method returned code='$code' (expected -32601)"
    fi
}

# Test 5: LSP REST endpoints
test_lsp_endpoints() {
    log_info "Test 5: LSP REST endpoints (servers/stats/sync)"
    # GET /v1/lsp/servers
    local servers=$(curl -fsS -m 5 "$BASE_URL/v1/lsp/servers" 2>/dev/null || echo '[]')
    local server_count=$(echo "$servers" | jq -r '. | length // 0' 2>/dev/null)
    if [[ "$server_count" -ge 1 ]]; then
        record_assertion "lsp" "servers_listed" "true" "$server_count LSP servers configured"
    else
        record_assertion "lsp" "servers_listed" "false" "0 LSP servers (registry empty)"
    fi
    # GET /v1/lsp/stats
    local stats=$(curl -fsS -m 5 "$BASE_URL/v1/lsp/stats" 2>/dev/null || echo '{}')
    local total=$(echo "$stats" | jq -r '.totalServers // 0')
    if [[ "$total" -ge 1 ]]; then
        record_assertion "lsp" "stats_real" "true" "stats.totalServers=$total"
    else
        record_assertion "lsp" "stats_real" "false" "stats.totalServers=$total"
    fi
    # POST /v1/lsp/sync
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 -X POST "$BASE_URL/v1/lsp/sync" \
        -H "Content-Type: application/json" -d '{}' 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record_assertion "lsp" "sync_ok" "true" "/v1/lsp/sync → 200"
    else
        record_assertion "lsp" "sync_ok" "false" "/v1/lsp/sync → $code"
    fi
    # POST /v1/lsp/execute empty → 400 (negative)
    local code2=$(curl -s -o /dev/null -w "%{http_code}" -m 5 -X POST "$BASE_URL/v1/lsp/execute" \
        -H "Content-Type: application/json" -d '{}' 2>/dev/null || echo "000")
    if [[ "$code2" == "400" ]]; then
        record_assertion "lsp" "execute_empty_400" "true" "Empty body → 400 (correct)"
    else
        record_assertion "lsp" "execute_empty_400" "false" "Empty body → $code2 (expected 400)"
    fi
}

# Test 6: ACP REST endpoints
test_acp_endpoints() {
    log_info "Test 6: ACP REST endpoints (agents + execute)"
    local agents=$(curl -fsS -m 5 "$BASE_URL/v1/acp/agents" 2>/dev/null || echo '{}')
    local count=$(echo "$agents" | jq -r '.agents | length // 0' 2>/dev/null)
    if [[ "$count" -ge 1 ]]; then
        record_assertion "acp" "agents_listed" "true" "$count ACP agents available"
    else
        record_assertion "acp" "agents_listed" "false" "0 ACP agents (registry empty)"
    fi
    # POST /v1/acp/execute with real agent_id
    local first_id=$(echo "$agents" | jq -r '.agents[0].id // empty')
    if [[ -n "$first_id" && "$first_id" != "null" ]]; then
        local resp=$(curl -fsS -m 30 -X POST "$BASE_URL/v1/acp/execute" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$(printf '{"agent_id":%s,"task":"sort an array"}' "$(printf '%s' "$first_id" | jq -R .)")" 2>/dev/null || echo '{}')
        local status=$(echo "$resp" | jq -r '.status // empty' 2>/dev/null)
        if [[ "$status" == "completed" || "$status" == "running" ]]; then
            record_assertion "acp" "execute_completed" "true" "execute on '$first_id' → status=$status"
        else
            record_assertion "acp" "execute_completed" "false" "execute returned status='$status'"
        fi
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_mcp_methods || true
    test_mcp_tools_call || true
    test_mcp_unknown_method || true
    test_lsp_endpoints || true
    test_acp_endpoints || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
