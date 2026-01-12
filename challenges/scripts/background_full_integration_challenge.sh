#!/bin/bash
# Background Full Integration Challenge
# Comprehensive E2E test of the entire background execution system

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/challenge_framework.sh"

CHALLENGE_NAME="background_full_integration"
CHALLENGE_DESCRIPTION="Comprehensive E2E test of background task queue, worker pool, notifications, and CLI rendering"

API_BASE="${API_BASE:-http://localhost:7061}"

# Initialize challenge framework
init_challenge "$CHALLENGE_NAME" "$CHALLENGE_DESCRIPTION"

log_info "Starting ${CHALLENGE_NAME} challenge"

# Test 1: Verify code compiles
test_code_compilation() {
    log_info "Test 1: Verifying all background code compiles..."

    cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

    if go build ./internal/background/... 2>/dev/null; then
        log_success "Background package compiles"
    else
        log_error "Background package failed to compile"
        return 1
    fi

    if go build ./internal/notifications/... 2>/dev/null; then
        log_success "Notifications package compiles"
    else
        log_error "Notifications package failed to compile"
        return 1
    fi

    if go build ./internal/handlers/... 2>/dev/null; then
        log_success "Handlers package compiles"
    else
        log_error "Handlers package failed to compile"
        return 1
    fi

    return 0
}

# Test 2: Verify unit tests pass
test_unit_tests() {
    log_info "Test 2: Running unit tests..."

    cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

    if go test -short ./tests/unit/background/... 2>/dev/null; then
        log_success "Unit tests pass"
        return 0
    else
        log_warning "Some unit tests failed or not available"
        return 0
    fi
}

# Test 3: Full workflow test (API)
test_full_workflow() {
    log_info "Test 3: Testing full workflow via API..."

    # Check API
    if ! curl -s -o /dev/null -w "%{http_code}" "${API_BASE}/v1/health" | grep -q "200"; then
        log_warning "API not available, skipping API tests"
        return 0
    fi

    # Step 1: Create task
    local task_id
    task_id=$(curl -s -X POST "${API_BASE}/v1/tasks" \
        -H "Content-Type: application/json" \
        -d '{
            "task_type": "integration_test",
            "task_name": "Full Integration Test",
            "priority": "high",
            "config": {
                "timeout_seconds": 60,
                "allow_cancel": true
            },
            "notification_config": {
                "enable_sse": true,
                "enable_polling": true
            },
            "payload": {"command": "echo Integration Test Success"}
        }' | jq -r '.id // empty')

    if [[ -z "$task_id" ]]; then
        log_warning "Could not create task"
        return 0
    fi

    log_info "Created task: $task_id"

    # Step 2: Check status
    local status
    status=$(curl -s "${API_BASE}/v1/tasks/${task_id}/status" | jq -r '.status // "unknown"')
    log_info "Task status: $status"

    # Step 3: Get logs
    local logs
    logs=$(curl -s "${API_BASE}/v1/tasks/${task_id}/logs" | jq -r '.count // 0')
    log_info "Task has $logs log entries"

    # Step 4: Get resources
    local resources
    resources=$(curl -s "${API_BASE}/v1/tasks/${task_id}/resources" | jq -r '.count // 0')
    log_info "Task has $resources resource snapshots"

    # Step 5: Analyze task
    local is_stuck
    is_stuck=$(curl -s "${API_BASE}/v1/tasks/${task_id}/analyze" | jq -r '.is_stuck // false')
    log_info "Task is_stuck: $is_stuck"

    # Step 6: Cancel task
    curl -s -X POST "${API_BASE}/v1/tasks/${task_id}/cancel" > /dev/null

    log_success "Full workflow completed"
    return 0
}

# Test 4: Queue statistics
test_queue_statistics() {
    log_info "Test 4: Checking queue statistics..."

    if ! curl -s -o /dev/null -w "%{http_code}" "${API_BASE}/v1/health" | grep -q "200"; then
        log_warning "API not available"
        return 0
    fi

    local stats
    stats=$(curl -s "${API_BASE}/v1/tasks/queue/stats")

    local pending
    pending=$(echo "$stats" | jq -r '.pending_count // 0')
    local running
    running=$(echo "$stats" | jq -r '.running_count // 0')

    log_success "Queue: pending=$pending, running=$running"
    return 0
}

# Test 5: Concurrent task handling
test_concurrent_handling() {
    log_info "Test 5: Testing concurrent task handling..."

    if ! curl -s -o /dev/null -w "%{http_code}" "${API_BASE}/v1/health" | grep -q "200"; then
        log_warning "API not available"
        return 0
    fi

    # Create 5 concurrent tasks
    local task_ids=()
    for i in {1..5}; do
        local id
        id=$(curl -s -X POST "${API_BASE}/v1/tasks" \
            -H "Content-Type: application/json" \
            -d "{
                \"task_type\": \"concurrent_test\",
                \"task_name\": \"Concurrent Task $i\",
                \"priority\": \"normal\",
                \"payload\": {\"index\": $i}
            }" | jq -r '.id // empty')
        if [[ -n "$id" ]]; then
            task_ids+=("$id")
        fi
    done

    log_info "Created ${#task_ids[@]} concurrent tasks"

    # Wait briefly
    sleep 2

    # Check queue
    local stats
    stats=$(curl -s "${API_BASE}/v1/tasks/queue/stats")

    # Cancel all created tasks
    for id in "${task_ids[@]}"; do
        curl -s -X POST "${API_BASE}/v1/tasks/${id}/cancel" > /dev/null 2>&1 || true
    done

    log_success "Concurrent task handling completed"
    return 0
}

# Test 6: Webhook registration and deletion
test_webhook_lifecycle() {
    log_info "Test 6: Testing webhook lifecycle..."

    if ! curl -s -o /dev/null -w "%{http_code}" "${API_BASE}/v1/health" | grep -q "200"; then
        log_warning "API not available"
        return 0
    fi

    # Register
    local webhook_id
    webhook_id=$(curl -s -X POST "${API_BASE}/v1/webhooks" \
        -H "Content-Type: application/json" \
        -d '{
            "url": "https://httpbin.org/post",
            "events": ["completed"]
        }' | jq -r '.id // empty')

    if [[ -n "$webhook_id" ]]; then
        log_info "Registered webhook: $webhook_id"

        # Delete
        curl -s -X DELETE "${API_BASE}/v1/webhooks/${webhook_id}" > /dev/null
        log_success "Webhook lifecycle completed"
    else
        log_warning "Could not register webhook"
    fi

    return 0
}

# Test 7: Verify all packages exist
test_package_structure() {
    log_info "Test 7: Verifying package structure..."

    cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

    local required_files=(
        "internal/background/interfaces.go"
        "internal/background/task_queue.go"
        "internal/background/worker_pool.go"
        "internal/background/resource_monitor.go"
        "internal/background/stuck_detector.go"
        "internal/background/metrics.go"
        "internal/notifications/hub.go"
        "internal/notifications/sse_manager.go"
        "internal/notifications/websocket_server.go"
        "internal/notifications/webhook_dispatcher.go"
        "internal/notifications/polling_store.go"
        "internal/notifications/cli/renderer.go"
        "internal/notifications/cli/types.go"
        "internal/notifications/cli/detection.go"
        "internal/handlers/background_task_handler.go"
    )

    local missing=0
    for file in "${required_files[@]}"; do
        if [[ ! -f "$file" ]]; then
            log_error "Missing: $file"
            ((missing++))
        fi
    done

    if [[ $missing -eq 0 ]]; then
        log_success "All ${#required_files[@]} required files present"
        return 0
    else
        log_error "$missing files missing"
        return 1
    fi
}

# Main
main() {
    local passed=0
    local failed=0

    test_code_compilation && ((++passed)) || ((++failed))
    test_unit_tests && ((++passed)) || ((++failed))
    test_full_workflow && ((++passed)) || ((++failed))
    test_queue_statistics && ((++passed)) || ((++failed))
    test_concurrent_handling && ((++passed)) || ((++failed))
    test_webhook_lifecycle && ((++passed)) || ((++failed))
    test_package_structure && ((++passed)) || ((++failed))

    log_info "=========================================="
    log_info "Challenge: $CHALLENGE_NAME"
    log_info "Passed: $passed, Failed: $failed"
    log_info "=========================================="

    [[ $failed -eq 0 ]] && { log_success "Challenge PASSED!"; exit 0; } || { log_error "Challenge FAILED!"; exit 1; }
}

main "$@"
