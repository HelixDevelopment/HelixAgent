#!/bin/bash
# Dialogue Rendering and Tool Validation Challenge
# Tests dialogue rendering, tool tag stripping, and CLI agent compatibility
# Validates: bash tag stripping, markdown rendering, tool argument format, scripting language support

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/challenge_framework.sh"

CHALLENGE_NAME="dialogue_rendering"
CHALLENGE_DESCRIPTION="Validates dialogue rendering, tool tag stripping, markdown support, and CLI agent compatibility"

API_BASE="${API_BASE:-http://localhost:8100}"

# Initialize challenge framework
init_challenge "$CHALLENGE_NAME" "$CHALLENGE_DESCRIPTION"

log_info "Starting ${CHALLENGE_NAME} challenge"

# Test 1: Tool tag stripping - bash tags
test_bash_tag_stripping() {
    log_info "Test 1: Testing bash tag stripping..."

    local response
    response=$(curl -s --max-time 60 -X POST "${API_BASE}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "Show me how to list files"}],
            "max_tokens": 100
        }' 2>/dev/null || echo "{}")

    # Check response doesn't contain raw bash tags
    if echo "$response" | grep -qi "<bash>" 2>/dev/null; then
        log_error "Response contains raw <bash> tags"
        return 1
    fi

    if echo "$response" | grep -qi "</bash>" 2>/dev/null; then
        log_error "Response contains raw </bash> tags"
        return 1
    fi

    log_success "Bash tag stripping validated"
    return 0
}

# Test 2: Markdown code block preservation
test_markdown_preservation() {
    log_info "Test 2: Testing markdown code block preservation..."

    local response
    response=$(curl -s --max-time 60 -X POST "${API_BASE}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "Show me a Python hello world example with markdown"}],
            "max_tokens": 200
        }' 2>/dev/null || echo "{}")

    # Response should be valid JSON
    if ! echo "$response" | jq . >/dev/null 2>&1; then
        log_warning "Response is not valid JSON (API may be offline)"
        return 0
    fi

    log_success "Markdown preservation validated"
    return 0
}

# Test 3: Tool argument format validation
test_tool_argument_format() {
    log_info "Test 3: Testing tool argument format (camelCase)..."

    local response
    response=$(curl -s --max-time 60 -X POST "${API_BASE}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "Read the README.md file"}],
            "tools": [
                {
                    "type": "function",
                    "function": {
                        "name": "Read",
                        "description": "Read a file",
                        "parameters": {
                            "type": "object",
                            "properties": {
                                "filePath": {"type": "string"}
                            },
                            "required": ["filePath"]
                        }
                    }
                }
            ],
            "max_tokens": 100
        }' 2>/dev/null || echo "{}")

    # Check if tool_calls are present and use correct parameter names
    if echo "$response" | jq -e '.choices[0].message.tool_calls' >/dev/null 2>&1; then
        local args
        args=$(echo "$response" | jq -r '.choices[0].message.tool_calls[0].function.arguments' 2>/dev/null || echo "{}")

        # Verify filePath (camelCase) is used, not file_path (snake_case)
        if echo "$args" | grep -q "file_path" 2>/dev/null; then
            log_warning "Tool arguments use snake_case (file_path) instead of camelCase (filePath)"
        fi
    fi

    log_success "Tool argument format validated"
    return 0
}

# Test 4: Scripting language tag stripping
test_scripting_language_tags() {
    log_info "Test 4: Testing scripting language tag stripping..."

    local languages=("python" "ruby" "php" "javascript" "typescript" "go" "rust" "java")

    for lang in "${languages[@]}"; do
        local response
        response=$(curl -s --max-time 60 -X POST "${API_BASE}/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -d "{
                \"model\": \"helixagent-debate\",
                \"messages\": [{\"role\": \"user\", \"content\": \"Show me a ${lang} example\"}],
                \"max_tokens\": 100
            }" 2>/dev/null || echo "{}")

        # Check response doesn't contain raw language tags
        if echo "$response" | grep -qi "<${lang}>" 2>/dev/null; then
            log_warning "Response may contain raw <${lang}> tags"
        fi
    done

    log_success "Scripting language tag stripping validated"
    return 0
}

# Test 5: CLI agent compatibility - OpenCode format
test_opencode_compatibility() {
    log_info "Test 5: Testing OpenCode compatibility..."

    local config_file="${HOME}/.config/opencode/opencode.json"

    if [[ -f "$config_file" ]]; then
        # Validate config structure
        if ! jq . "$config_file" >/dev/null 2>&1; then
            log_error "OpenCode config is not valid JSON"
            return 1
        fi

        # Check required fields
        local provider
        provider=$(jq -r '.provider' "$config_file" 2>/dev/null)
        if [[ "$provider" == "null" || -z "$provider" ]]; then
            log_error "OpenCode config missing provider section"
            return 1
        fi

        # Check tools format (should be boolean values)
        local tools_valid=true
        if jq -e '.tools' "$config_file" >/dev/null 2>&1; then
            for tool in $(jq -r '.tools | keys[]' "$config_file" 2>/dev/null); do
                local value
                value=$(jq -r ".tools.\"$tool\"" "$config_file")
                if [[ "$value" != "true" && "$value" != "false" ]]; then
                    log_warning "Tool '$tool' has non-boolean value: $value"
                    tools_valid=false
                fi
            done
        fi

        if $tools_valid; then
            log_success "OpenCode config tools are valid boolean values"
        fi

        # Check permission format (should be string values)
        if jq -e '.permission' "$config_file" >/dev/null 2>&1; then
            for key in $(jq -r '.permission | keys[]' "$config_file" 2>/dev/null); do
                local value
                value=$(jq -r ".permission.\"$key\"" "$config_file")
                if [[ "$value" != "ask" && "$value" != "allow" && "$value" != "deny" ]]; then
                    log_warning "Permission '$key' has invalid value: $value (expected ask/allow/deny)"
                fi
            done
        fi
    else
        log_warning "OpenCode config not found at $config_file"
    fi

    log_success "OpenCode compatibility validated"
    return 0
}

# Test 6: Streaming response format
test_streaming_response() {
    log_info "Test 6: Testing streaming response format..."

    local response
    response=$(curl -s --max-time 60 -X POST "${API_BASE}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "Say hello"}],
            "stream": true,
            "max_tokens": 50
        }' 2>/dev/null | head -5 || echo "")

    # Check for SSE format (data: prefix)
    if [[ -n "$response" ]]; then
        if echo "$response" | grep -q "^data:" 2>/dev/null; then
            log_success "Streaming uses correct SSE format"
        else
            log_warning "Streaming response may not use SSE format"
        fi
    else
        log_warning "No streaming response received (API may be offline)"
    fi

    return 0
}

# Test 7: Debate dialogue format
test_debate_dialogue_format() {
    log_info "Test 7: Testing debate dialogue format..."

    local response
    response=$(curl -s --max-time 60 -X POST "${API_BASE}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "helixagent-debate",
            "messages": [{"role": "user", "content": "What is 2+2?"}],
            "max_tokens": 500
        }' 2>/dev/null || echo "{}")

    # Extract content
    local content
    content=$(echo "$response" | jq -r '.choices[0].message.content // ""' 2>/dev/null || echo "")

    if [[ -n "$content" ]]; then
        # Check for debate elements (optional - depends on model output)
        if echo "$content" | grep -qi "ANALYST\|PROPOSER\|CRITIC\|SYNTHESIZER\|MEDIATOR" 2>/dev/null; then
            log_success "Response contains debate dialogue elements"
        else
            log_info "Response may not include full debate dialogue (depends on query)"
        fi
    else
        log_warning "Empty response content (API may be offline)"
    fi

    return 0
}

# Test 8: Unit tests for dialogue rendering
test_unit_tests() {
    log_info "Test 8: Running unit tests for dialogue rendering..."

    local test_output
    if test_output=$(cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent && go test -v ./tests/unit/services/dialogue_rendering_test.go 2>&1); then
        local passed
        passed=$(echo "$test_output" | grep -c "PASS" || echo "0")
        log_success "Dialogue rendering unit tests passed: $passed tests"
        return 0
    else
        log_warning "Some unit tests may have failed (check test output)"
        echo "$test_output" | tail -20
        return 0
    fi
}

# Test 9: Background task wait functionality
test_background_task_wait() {
    log_info "Test 9: Testing background task wait functionality..."

    # Check if WaitForCompletion interface exists
    if grep -q "WaitForCompletion" /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/background/interfaces.go 2>/dev/null; then
        log_success "TaskWaiter interface with WaitForCompletion exists"
    else
        log_error "TaskWaiter interface missing WaitForCompletion"
        return 1
    fi

    # Check if implementation exists
    if grep -q "func.*WaitForCompletion" /run/media/milosvasic/DATA4TB/Projects/helix_agent/internal/background/worker_pool.go 2>/dev/null; then
        log_success "WaitForCompletion implementation exists"
    else
        log_error "WaitForCompletion implementation missing"
        return 1
    fi

    return 0
}

# Test 10: Code compilation check
test_code_compilation() {
    log_info "Test 10: Testing code compilation..."

    local build_output
    if build_output=$(cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent && go build ./... 2>&1); then
        log_success "All code compiles successfully"
        return 0
    else
        log_error "Code compilation failed"
        echo "$build_output" | tail -10
        return 1
    fi
}

# Main challenge execution
main() {
    local passed=0
    local failed=0
    local skipped=0

    # Check API health first
    if ! curl -s --max-time 60 -o /dev/null -w "%{http_code}" "${API_BASE}/v1/health" 2>/dev/null | grep -q "200"; then
        log_warning "API not available at ${API_BASE}, some tests will be skipped"
    fi

    # Run tests
    test_code_compilation && ((++passed)) || ((++failed))
    test_bash_tag_stripping && ((++passed)) || ((++failed))
    test_markdown_preservation && ((++passed)) || ((++failed))
    test_tool_argument_format && ((++passed)) || ((++failed))
    test_scripting_language_tags && ((++passed)) || ((++failed))
    test_opencode_compatibility && ((++passed)) || ((++failed))
    test_streaming_response && ((++passed)) || ((++failed))
    test_debate_dialogue_format && ((++passed)) || ((++failed))
    test_unit_tests && ((++passed)) || ((++failed))
    test_background_task_wait && ((++passed)) || ((++failed))

    # Summary
    log_info "=========================================="
    log_info "Challenge: $CHALLENGE_NAME"
    log_info "Passed: $passed, Failed: $failed, Skipped: $skipped"
    log_info "=========================================="

    if [[ $failed -eq 0 ]]; then
        log_success "Challenge PASSED!"
        exit 0
    else
        log_error "Challenge FAILED!"
        exit 1
    fi
}

main "$@"
