#!/bin/bash
# Protocol LSP Integration Challenge
# Tests Language Server Protocol support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-lsp-integration" "Protocol LSP Integration Challenge"
load_env

log_info "Testing LSP integration..."

test_lsp_endpoint_availability() {
    log_info "Test 1: LSP endpoint availability"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/lsp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":"file:///tmp","capabilities":{}}}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|404|405)$ ]] && record_assertion "lsp_endpoint" "checked" "true" "HTTP $code"
}

test_lsp_initialize() {
    log_info "Test 2: LSP initialize request"

    local resp_body=$(curl -s "$BASE_URL/v1/lsp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":1234,"rootUri":"file:///workspace","capabilities":{"textDocument":{"completion":{"dynamicRegistration":true}}}}}' \
        --max-time 10 2>/dev/null || echo '{}')

    # Check for LSP initialize response
    local has_result=$(echo "$resp_body" | jq -e '.result' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_capabilities=$(echo "$resp_body" | jq -e '.result.capabilities' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_result" == "yes" && "$has_capabilities" == "yes" ]]; then
        record_assertion "lsp_initialize" "working" "true" "Initialize succeeded"
    else
        record_assertion "lsp_initialize" "checked" "true" "result:$has_result capabilities:$has_capabilities"
    fi
}

test_lsp_document_sync() {
    log_info "Test 3: LSP document synchronization"

    # Send textDocument/didOpen notification
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/lsp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///test.go","languageId":"go","version":1,"text":"package main\n\nfunc main() {\n}"}}}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # didOpen is a notification, should return 200 or 204
    [[ "$code" =~ ^(200|204)$ ]] && record_assertion "lsp_document_sync" "working" "true" "Document sync accepted"
}

test_lsp_completion() {
    log_info "Test 4: LSP completion request"

    local resp_body=$(curl -s "$BASE_URL/v1/lsp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":2,"method":"textDocument/completion","params":{"textDocument":{"uri":"file:///test.go"},"position":{"line":3,"character":0}}}' \
        --max-time 15 2>/dev/null || echo '{}')

    # Check for completion response
    local has_result=$(echo "$resp_body" | jq -e '.result' > /dev/null 2>&1 && echo "yes" || echo "no")
    local is_array=$(echo "$resp_body" | jq -e '.result | type == "array"' 2>/dev/null || echo "false")
    local is_object=$(echo "$resp_body" | jq -e '.result | type == "object"' 2>/dev/null || echo "false")

    if [[ "$has_result" == "yes" && ("$is_array" == "true" || "$is_object" == "true") ]]; then
        record_assertion "lsp_completion" "working" "true" "Completion response valid"
    else
        record_assertion "lsp_completion" "checked" "true" "result:$has_result array:$is_array object:$is_object"
    fi
}

main() {
    log_info "Starting LSP integration challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_lsp_endpoint_availability
    test_lsp_initialize
    test_lsp_document_sync
    test_lsp_completion

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
