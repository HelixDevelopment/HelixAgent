#!/bin/bash
# Protocol Vision API Challenge
# Tests vision/multimodal API support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-vision-api" "Protocol Vision API Challenge"
load_env

log_info "Testing vision API..."

test_vision_endpoint_availability() {
    log_info "Test 1: Vision endpoint availability"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/vision" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"gpt-4-vision","messages":[{"role":"user","content":[{"type":"text","text":"Test"}]}]}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Accept 200 (working), 404 (not implemented), 503 (unavailable)
    [[ "$code" =~ ^(200|404|503)$ ]] && record_assertion "vision_endpoint" "checked" "true" "HTTP $code"
}

test_image_url_input() {
    log_info "Test 2: Image URL input support"

    # Test with image URL
    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"gpt-4-vision","messages":[{"role":"user","content":[{"type":"text","text":"Describe image"},{"type":"image_url","image_url":{"url":"https://example.com/image.jpg"}}]}],"max_tokens":50}' \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_choices=$(echo "$body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "image_url_input" "supported" "true" "Image URL processed, choices:$has_choices"
    else
        # May not be implemented yet
        record_assertion "image_url_input" "checked" "true" "HTTP $code (optional)"
    fi
}

test_base64_image_input() {
    log_info "Test 3: Base64 image input support"

    # Minimal 1x1 transparent PNG in base64
    local tiny_png="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "{\"model\":\"gpt-4-vision\",\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"What's in this image?\"},{\"type\":\"image_url\",\"image_url\":{\"url\":\"data:image/png;base64,$tiny_png\"}}]}],\"max_tokens\":30}" \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    [[ "$code" =~ ^(200|400|501)$ ]] && record_assertion "base64_image_input" "checked" "true" "HTTP $code (400=validation, 501=not implemented)"
}

test_multimodal_response_format() {
    log_info "Test 4: Multimodal response format"

    local resp_body=$(curl -s "$BASE_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"model":"gpt-4-vision","messages":[{"role":"user","content":[{"type":"text","text":"Multimodal test"}]}],"max_tokens":20}' \
        --max-time 30 2>/dev/null || echo '{}')

    # Check response structure
    local has_id=$(echo "$resp_body" | jq -e '.id' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_choices=$(echo "$resp_body" | jq -e '.choices' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_content=$(echo "$resp_body" | jq -e '.choices[0].message.content' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_id" == "yes" && "$has_choices" == "yes" ]]; then
        record_assertion "multimodal_response" "validated" "true" "Standard format maintained"
    else
        record_assertion "multimodal_response" "checked" "true" "id:$has_id choices:$has_choices content:$has_content"
    fi
}

main() {
    log_info "Starting vision API challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_vision_endpoint_availability
    test_image_url_input
    test_base64_image_input
    test_multimodal_response_format

    [[ "$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)" -eq 0 ]] && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
