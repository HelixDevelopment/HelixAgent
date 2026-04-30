#!/bin/bash
# Vision Stub Honesty Challenge — CONST-035 anti-bluff regression guard
#
# Validates that all 6 /v1/vision/* analysis endpoints (analyze, ocr,
# detect, caption, describe, classify):
#   1. Return 400 when neither 'image' nor 'image_url' is provided
#      (closes the structural bluff: empty body returned 200 with a
#      fabricated "successful analysis" of an empty input).
#   2. Return verified=false AND status="stub_only" until a real
#      vision-capable provider is wired in (CLI agents and SDK
#      consumers can detect that rich response fields are stubs).
#   3. Do NOT fabricate hard-coded colors, captions, or labels in the
#      response body — those fields used to be "#FF0000,#00FF00,#0000FF"
#      / "An image showing visual content" / "object" with confidence
#      0.95 regardless of input.
#
# This Challenge MUST FAIL when:
#   - validateVisionInput is removed → empty body returns 200 again.
#   - Status reverts to "completed" or Verified=true is hardcoded →
#     callers can't distinguish stub from real.
#   - The fabricated color/caption/label arrays are reintroduced →
#     the assertions for absent stub fields fail.
#
# Verify-by-mutation (CONST-035 §1):
#   - Comment out validateVisionInput in any of the 6 handlers →
#     test_<endpoint>_empty_400 fails for that endpoint.
#   - Hardcode Verified: true → test_verified_false_consistent fails.
#   - Re-add the dominant_colors / caption / classifications constants
#     → test_no_fabricated_colors / test_no_fabricated_caption fails.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "vision_stub_honesty" "Vision Stub Honesty Challenge"
load_env

VISION_PATHS=(analyze ocr detect caption describe classify)

# Test 1: server alive
test_server_alive() {
    if curl -fsS -m 10 "$BASE_URL/v1/health" >/dev/null 2>&1; then
        record_assertion "server" "alive" "true" "Server responding"
    else
        record_assertion "server" "alive" "false" "Server not reachable"
        return 1
    fi
}

# Test 2: empty body → 400 on every endpoint
test_empty_400_all() {
    log_info "Test 2: empty body → 400 on all 6 vision endpoints"
    local fails=0
    for ep in "${VISION_PATHS[@]}"; do
        local code=$(curl -s -o /dev/null -w "%{http_code}" -m 10 \
            -X POST "$BASE_URL/v1/vision/$ep" \
            -H "Content-Type: application/json" \
            -d '{}' 2>/dev/null || echo "000")
        if [[ "$code" != "400" ]]; then
            record_assertion "vision_$ep" "empty_400" "false" "$ep empty body → $code (expected 400)"
            ((fails++))
        else
            record_assertion "vision_$ep" "empty_400" "true" "$ep empty body → 400 (correct)"
        fi
    done
    [[ "$fails" == "0" ]]
}

# Test 3: verified=false on every endpoint when given valid input
test_verified_false_consistent() {
    log_info "Test 3: verified=false on all endpoints (no real vision provider)"
    for ep in "${VISION_PATHS[@]}"; do
        local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/vision/$ep" \
            -H "Content-Type: application/json" \
            -d '{"image_url":"https://example.com/test.png"}' 2>/dev/null || echo '{}')
        # Note: do NOT use `// empty` here — jq's // operator treats
        # `false` as falsy and would collapse it to empty string,
        # making the assertion always fail. Use `tostring` to coerce
        # the boolean to its literal string form.
        local verified=$(echo "$resp" | jq -r '.verified | tostring')
        local status=$(echo "$resp" | jq -r '.status // empty')
        if [[ "$verified" == "false" && "$status" == "stub_only" ]]; then
            record_assertion "vision_$ep" "honest_discriminator" "true" "$ep verified=false status=stub_only (honest)"
        else
            record_assertion "vision_$ep" "honest_discriminator" "false" "$ep verified=$verified status=$status — discriminator broken"
        fi
    done
}

# Test 4: response body does NOT contain hardcoded fabricated values
test_no_fabricated_colors() {
    log_info "Test 4: response no longer contains fabricated colors '#FF0000'/'#00FF00'/'#0000FF'"
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/vision/analyze" \
        -H "Content-Type: application/json" \
        -d '{"image_url":"https://example.com/test.png"}' 2>/dev/null || echo '{}')
    if echo "$resp" | grep -q '#FF0000\|#00FF00\|#0000FF'; then
        record_assertion "analyze" "no_fabricated_colors" "false" "Response still contains hardcoded RGB color sentinels"
    else
        record_assertion "analyze" "no_fabricated_colors" "true" "No hardcoded RGB sentinels in response"
    fi
}

# Test 5: caption response no longer contains the stub sentinel string
test_no_fabricated_caption() {
    log_info "Test 5: caption response no longer contains stub 'An image showing visual content'"
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/vision/caption" \
        -H "Content-Type: application/json" \
        -d '{"image_url":"https://example.com/test.png"}' 2>/dev/null || echo '{}')
    if echo "$resp" | grep -q 'An image showing visual content'; then
        record_assertion "caption" "no_fabricated_caption" "false" "Response still contains stub sentinel caption"
    else
        record_assertion "caption" "no_fabricated_caption" "true" "No stub sentinel caption in response"
    fi
}

# Test 6: describe response no longer contains the stub sentinel string
test_no_fabricated_description() {
    log_info "Test 6: describe response no longer contains the long stub sentinel"
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/vision/describe" \
        -H "Content-Type: application/json" \
        -d '{"image_url":"https://example.com/test.png"}' 2>/dev/null || echo '{}')
    if echo "$resp" | grep -q 'graphical elements with various colors and patterns'; then
        record_assertion "describe" "no_fabricated_description" "false" "Response still contains stub sentinel description"
    else
        record_assertion "describe" "no_fabricated_description" "true" "No stub sentinel description in response"
    fi
}

# Test 7: classify response no longer contains the hardcoded category list
test_no_fabricated_categories() {
    log_info "Test 7: classify response no longer contains 'general,digital,graphic' hardcoded triple"
    local resp=$(curl -fsS -m 10 -X POST "$BASE_URL/v1/vision/classify" \
        -H "Content-Type: application/json" \
        -d '{"image_url":"https://example.com/test.png"}' 2>/dev/null || echo '{}')
    # The old code emitted ALL THREE categories — match all three on the same line.
    local found=$(echo "$resp" | jq -r '.result.all_categories // empty | tostring')
    if [[ "$found" == *"general"* && "$found" == *"digital"* && "$found" == *"graphic"* ]]; then
        record_assertion "classify" "no_fabricated_categories" "false" "Response still contains hardcoded category triple: $found"
    else
        record_assertion "classify" "no_fabricated_categories" "true" "No hardcoded category triple"
    fi
}

main() {
    test_server_alive || { finalize_challenge "FAILED"; exit 1; }
    test_empty_400_all || true
    test_verified_false_consistent || true
    test_no_fabricated_colors || true
    test_no_fabricated_caption || true
    test_no_fabricated_description || true
    test_no_fabricated_categories || true

    if ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log"; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
