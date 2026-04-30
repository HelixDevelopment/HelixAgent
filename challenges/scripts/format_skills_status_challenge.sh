#!/bin/bash
# Format + Skills Status-Code Challenge — CONST-035 anti-bluff regression guard
#
# Validates the bluff fixes from commit c2c623c9. Three contract bluffs
# in /v1/format and /v1/skills:
#
#   1. /v1/format invalid language: 500 → 400 (bad-input, not server error)
#   2. /v1/format failed formatter: 200 success:false → 422 (wrapper bluff)
#   3. /v1/skills/<bogus_category>: 200 empty → 404 (structural bluff)
#
# Plus 2 positive controls (real language → 200, real category → 200) so
# the Challenge isn't itself a contract bluff that just asserts "always 4xx".
#
# Verify-by-mutation (CONST-035 §1):
#   - Revert format_handler.go status mapping → format_invalid_400 fails
#   - Remove the result.Success → 422 mapping → format_unprocessable_422 fails
#   - Remove the GetCategories check in skills_handler.go → skills_bogus_404 fails

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "format_skills_status" "Format + Skills Status-Code Challenge"
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

# Test 2: /v1/format with invalid language returns 400 (was 500)
test_format_invalid_language() {
    log_info "Test 2: /v1/format invalid language → 400"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 10 -X POST "$BASE_URL/v1/format" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"language":"definitely-not-a-real-language","content":"hello"}' 2>/dev/null || echo "000")
    if [[ "$code" == "400" ]]; then
        record_assertion "format" "invalid_lang_400" "true" "Invalid language correctly returns 400"
    elif [[ "$code" == "500" ]]; then
        record_assertion "format" "invalid_lang_400" "false" "Invalid language returned 500 — STATUS BLUFF (caller-side error mapped to server error)"
    else
        record_assertion "format" "invalid_lang_400" "false" "Returned $code (expected 400)"
    fi
}

# Test 3: /v1/format with malformed code returns 422 (was 200 with success:false)
test_format_malformed_code_422() {
    log_info "Test 3: /v1/format malformed code → 422 (not 200 with success:false)"
    local resp=$(curl -s -w "\n%{http_code}" -m 10 -X POST "$BASE_URL/v1/format" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"language":"go","content":"this is definitely not valid go syntax"}' 2>/dev/null || true)
    local code=$(echo "$resp" | tail -n1)
    local payload=$(echo "$resp" | head -n -1)
    local success=$(echo "$payload" | jq -r '.success // empty' 2>/dev/null)

    if [[ "$code" == "422" ]]; then
        record_assertion "format" "malformed_422" "true" "Malformed code → 422 (correct)"
    elif [[ "$code" == "200" && "$success" == "false" ]]; then
        record_assertion "format" "malformed_422" "false" "Returned 200 with success:false — WRAPPER BLUFF (SDK reads 200 = ok)"
    else
        record_assertion "format" "malformed_422" "false" "Returned $code success=$success"
    fi
}

# Test 4: /v1/format with valid go code returns 200 (positive control)
test_format_valid_200() {
    log_info "Test 4: /v1/format with valid go code → 200 (positive control)"
    local body='{"language":"go","content":"package main\n\nfunc main() {\n}"}'
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 10 -X POST "$BASE_URL/v1/format" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$body" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record_assertion "format" "valid_200" "true" "Valid go → 200 (proves fix doesn't break working path)"
    else
        record_assertion "format" "valid_200" "false" "Valid go returned $code (expected 200)"
    fi
}

# Test 5: /v1/skills/<bogus_category> returns 404 (was 200 empty)
test_skills_bogus_404() {
    log_info "Test 5: /v1/skills/<bogus> → 404 (was 200 empty)"
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/skills/definitely-not-a-real-category" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "404" ]]; then
        record_assertion "skills" "bogus_404" "true" "Bogus category → 404 (correct)"
    elif [[ "$code" == "200" ]]; then
        record_assertion "skills" "bogus_404" "false" "Bogus category → 200 — STRUCTURAL BLUFF"
    else
        record_assertion "skills" "bogus_404" "false" "Returned $code (expected 404)"
    fi
}

# Test 6: /v1/skills/<real_category> returns 200 (positive control)
test_skills_real_category_200() {
    log_info "Test 6: /v1/skills/<real_category> → 200 (positive control)"
    # Pick a real category from /v1/skills/categories
    local real=$(curl -fsS -m 5 "$BASE_URL/v1/skills/categories" 2>/dev/null | jq -r '.categories[0] // empty')
    if [[ -z "$real" || "$real" == "null" ]]; then
        record_assertion "skills" "real_category_200" "false" "No real category available for positive control"
        return 1
    fi
    local code=$(curl -s -o /dev/null -w "%{http_code}" -m 5 \
        "$BASE_URL/v1/skills/$real" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record_assertion "skills" "real_category_200" "true" "Real category '$real' → 200 (proves fix doesn't break)"
    else
        record_assertion "skills" "real_category_200" "false" "Real category '$real' returned $code"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_format_invalid_language || true
    test_format_malformed_code_422 || true
    test_format_valid_200 || true
    test_skills_bogus_404 || true
    test_skills_real_category_200 || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
