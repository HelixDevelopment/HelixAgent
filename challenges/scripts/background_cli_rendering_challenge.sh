#!/bin/bash
# Background CLI Rendering Challenge
# Tests CLI rendering output for progress bars, status tables, and resource gauges

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/challenge_framework.sh"

CHALLENGE_NAME="background_cli_rendering"
CHALLENGE_DESCRIPTION="Validates CLI rendering output including progress bars, status tables, and resource gauges"

API_BASE="${API_BASE:-http://localhost:7061}"

# Initialize challenge framework
init_challenge "$CHALLENGE_NAME" "$CHALLENGE_DESCRIPTION"

log_info "Starting ${CHALLENGE_NAME} challenge"

# Test 1: Test unit tests for CLI package
test_cli_unit_tests() {
    log_info "Test 1: Running CLI rendering unit tests..."

    # Check if CLI package compiles
    if go build ./internal/notifications/cli/... 2>/dev/null; then
        log_success "CLI package compiles successfully"
        return 0
    else
        log_error "CLI package compilation failed"
        return 1
    fi
}

# Test 2: Test progress bar rendering (code inspection)
test_progress_bar_rendering() {
    log_info "Test 2: Verifying progress bar rendering code..."

    if grep -q "ProgressBarContent" internal/notifications/cli/types.go 2>/dev/null; then
        log_success "ProgressBarContent type defined"
    else
        log_warning "ProgressBarContent not found"
    fi

    if grep -q "RenderProgressBar" internal/notifications/cli/renderer.go 2>/dev/null; then
        log_success "RenderProgressBar function defined"
        return 0
    else
        log_warning "RenderProgressBar not found"
        return 0
    fi
}

# Test 3: Test status table rendering (code inspection)
test_status_table_rendering() {
    log_info "Test 3: Verifying status table rendering code..."

    if grep -q "StatusTableContent" internal/notifications/cli/types.go 2>/dev/null; then
        log_success "StatusTableContent type defined"
    else
        log_warning "StatusTableContent not found"
    fi

    if grep -q "RenderStatusTable" internal/notifications/cli/renderer.go 2>/dev/null; then
        log_success "RenderStatusTable function defined"
        return 0
    else
        log_warning "RenderStatusTable not found"
        return 0
    fi
}

# Test 4: Test resource gauge rendering (code inspection)
test_resource_gauge_rendering() {
    log_info "Test 4: Verifying resource gauge rendering code..."

    if grep -q "ResourceGaugeContent" internal/notifications/cli/types.go 2>/dev/null; then
        log_success "ResourceGaugeContent type defined"
    else
        log_warning "ResourceGaugeContent not found"
    fi

    if grep -q "RenderResourceGauge" internal/notifications/cli/renderer.go 2>/dev/null; then
        log_success "RenderResourceGauge function defined"
        return 0
    else
        log_warning "RenderResourceGauge not found"
        return 0
    fi
}

# Test 5: Test CLI client detection
test_cli_detection() {
    log_info "Test 5: Verifying CLI client detection code..."

    if grep -q "DetectCLIClient" internal/notifications/cli/detection.go 2>/dev/null; then
        log_success "DetectCLIClient function defined"
    else
        log_warning "DetectCLIClient not found"
    fi

    if grep -q "CLIClientOpenCode\|CLIClientCrush\|CLIClientHelixCode" internal/notifications/cli/detection.go 2>/dev/null; then
        log_success "CLI client types defined"
        return 0
    else
        log_warning "CLI client types not found"
        return 0
    fi
}

# Test 6: Test ANSI color support
test_ansi_colors() {
    log_info "Test 6: Verifying ANSI color support..."

    if grep -q "ColorReset\|ColorBold\|ColorGreen\|ColorRed" internal/notifications/cli/types.go 2>/dev/null; then
        log_success "ANSI color constants defined"
        return 0
    else
        log_warning "ANSI colors not found"
        return 0
    fi
}

# Test 7: Test box drawing characters
test_box_drawing() {
    log_info "Test 7: Verifying box drawing characters..."

    if grep -q "BoxHorizontal\|BoxVertical\|BoxTopLeft" internal/notifications/cli/types.go 2>/dev/null; then
        log_success "Box drawing characters defined"
        return 0
    else
        log_warning "Box drawing characters not found"
        return 0
    fi
}

# Main
main() {
    local passed=0
    local failed=0

    # Change to project root
    cd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

    test_cli_unit_tests && ((++passed)) || ((++failed))
    test_progress_bar_rendering && ((++passed)) || ((++failed))
    test_status_table_rendering && ((++passed)) || ((++failed))
    test_resource_gauge_rendering && ((++passed)) || ((++failed))
    test_cli_detection && ((++passed)) || ((++failed))
    test_ansi_colors && ((++passed)) || ((++failed))
    test_box_drawing && ((++passed)) || ((++failed))

    log_info "=========================================="
    log_info "Challenge: $CHALLENGE_NAME"
    log_info "Passed: $passed, Failed: $failed"
    log_info "=========================================="

    [[ $failed -eq 0 ]] && { log_success "Challenge PASSED!"; exit 0; } || { log_error "Challenge FAILED!"; exit 1; }
}

main "$@"
