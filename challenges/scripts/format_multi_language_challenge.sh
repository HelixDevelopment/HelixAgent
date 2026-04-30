#!/bin/bash
# Multi-Language Format Challenge — CONST-035 anti-bluff regression guard
#
# Verifies that /v1/format actually formats code for each supported
# language with locally-installed binaries. Each assertion follows the
# end-user contract: real input → real formatted output, HTTP 200.
#
# This Challenge directly catches the bug class fixed in commits
# d82d631c (gofmt `-` bug) and da10a8ca (rustfmt `-` bug + python
# black/ruff order). Without these fixes, valid input returned 422 +
# success:false even though the Go/Rust/Python toolchains were
# correctly installed.
#
# Verify-by-mutation (CONST-035 §1):
#   - Revert gofmt.go to NewNativeFormatter → go_formats fails (422)
#   - Revert rustfmt.go to NewNativeFormatter → rust_formats fails (422)
#   - Swap python registration order back to black-first → python_formats fails

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "format_multi_language" "Multi-Language Format Challenge"
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

# Helper: assert /v1/format succeeds for a language with given input.
# Skips (with SKIP-OK) if the underlying CLI binary isn't installed.
assert_format_succeeds() {
    local label="$1"
    local target="$2"
    local language="$3"
    local content="$4"
    local cli_binary="$5"

    if ! command -v "$cli_binary" >/dev/null 2>&1; then
        record_skip "$label" "$target" "$cli_binary not installed (SKIP-OK: #infra-formatter-binary-missing)"
        return 0
    fi

    local body
    body=$(printf '{"language":%s,"content":%s}' \
        "$(printf '%s' "$language" | jq -R -s .)" \
        "$(printf '%s' "$content" | jq -R -s .)")

    local resp=$(curl -s -w "\n%{http_code}" -m 30 -X POST "$BASE_URL/v1/format" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)
    local success=$(echo "$payload" | jq -r '.success // false' 2>/dev/null)

    if [[ "$code" == "200" ]] && [[ "$success" == "true" ]]; then
        local formatter=$(echo "$payload" | jq -r '.formatter_name // ""')
        record_assertion "$label" "$target" "true" "$language → 200 success, formatted by $formatter"
    elif [[ "$code" == "200" ]] && [[ "$success" == "false" ]]; then
        local err=$(echo "$payload" | jq -r '.error // ""' | head -c 80)
        record_assertion "$label" "$target" "false" "$language → 200 success:false (WRAPPER BLUFF): $err"
    elif [[ "$code" == "422" ]]; then
        local err=$(echo "$payload" | jq -r '.error // ""' | head -c 80)
        record_assertion "$label" "$target" "false" "$language → 422 (formatter failed): $err"
    else
        record_assertion "$label" "$target" "false" "$language → $code"
    fi
}

# Test 2: Go formatting
test_go_format() {
    log_info "Test 2: gofmt formats valid Go code (was broken with 'lstat -' bug)"
    assert_format_succeeds "format" "go_formats" "go" \
        "$(printf 'package main\n\nfunc main() {\n}\n')" \
        "gofmt"
}

# Test 3: Rust formatting
test_rust_format() {
    log_info "Test 3: rustfmt formats valid Rust code (was broken with same '-' bug)"
    assert_format_succeeds "format" "rust_formats" "rust" \
        "fn main() { println!(\"hello\"); }" \
        "rustfmt"
}

# Test 4: Python formatting (was broken because black-not-installed picked first)
test_python_format() {
    log_info "Test 4: ruff formats valid Python code (was broken because black registered first)"
    assert_format_succeeds "format" "python_formats" "python" \
        "$(printf 'x = 1\ny=2\n')" \
        "ruff"
}

# Test 5: YAML formatting
test_yaml_format() {
    log_info "Test 5: yamlfmt formats valid YAML"
    assert_format_succeeds "format" "yaml_formats" "yaml" \
        "$(printf 'key: value\nlist:\n  - 1\n  - 2\n')" \
        "yamlfmt"
}

# Test 6: Stylua formatting (Lua)
test_lua_format() {
    log_info "Test 6: stylua formats valid Lua"
    assert_format_succeeds "format" "lua_formats" "lua" \
        "$(printf 'local function hello()\n  print(\"hi\")\nend\n')" \
        "stylua"
}

# Test 7: Negative — invalid language returns 400 (regression for round 16)
test_invalid_language_400() {
    log_info "Test 7: invalid language → 400 (not 500)"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 -X POST "$BASE_URL/v1/format" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"language":"definitely-not-a-real-language","content":"x"}' 2>/dev/null || echo "000")
    if [[ "$code" == "400" ]]; then
        record_assertion "format" "invalid_lang_400" "true" "Invalid language → 400 (correct)"
    else
        record_assertion "format" "invalid_lang_400" "false" "Invalid language → $code (expected 400)"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_go_format || true
    test_rust_format || true
    test_python_format || true
    test_yaml_format || true
    test_lua_format || true
    test_invalid_language_400 || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
