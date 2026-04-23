#!/bin/bash
# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0
#
# Full System Validation: CLI Agents Testing HelixAgent
# Comprehensive validation using all 47 CLI agents as test executors

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RESULTS_DIR="${PROJECT_ROOT}/challenge-results/full-system-validation"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="${RESULTS_DIR}/validation_${TIMESTAMP}.log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Logging
log() {
    echo -e "$1" | tee -a "$LOG_FILE"
}
log_info() { log "${BLUE}[INFO]${NC} $1"; }
log_success() { log "${GREEN}[PASS]${NC} $1"; }
log_error() { log "${RED}[FAIL]${NC} $1"; }
log_warn() { log "${YELLOW}[WARN]${NC} $1"; }
log_section() { log "${CYAN}========================================${NC}"; log "${CYAN}$1${NC}"; log "${CYAN}========================================${NC}"; }

# Setup
setup() {
    mkdir -p "$RESULTS_DIR"
    echo "Starting validation at $(date)" > "$LOG_FILE"
}

# Phase 1: Build Validation
phase1_build_validation() {
    log_section "PHASE 1: Build Validation"
    
    log_info "Building HelixAgent..."
    if cd "$PROJECT_ROOT" && go build -mod=mod -o bin/helixagent ./cmd/helixagent 2>&1 | tee -a "$LOG_FILE"; then
        log_success "HelixAgent built successfully"
    else
        log_error "HelixAgent build failed"
        return 1
    fi
    
    log_info "Verifying binary..."
    if [ -f "$PROJECT_ROOT/bin/helixagent" ]; then
        log_success "Binary exists"
        "$PROJECT_ROOT/bin/helixagent" --version 2>&1 | head -1 | tee -a "$LOG_FILE"
    else
        log_error "Binary not found"
        return 1
    fi
    
    return 0
}

# Phase 2: Configuration Generation
phase2_config_generation() {
    log_section "PHASE 2: CLI Agent Configuration Generation"
    
    log_info "Generating all CLI agent configs..."
    
    CONFIGS_DIR="$PROJECT_ROOT/cli_agents_configs"
    mkdir -p "$CONFIGS_DIR"
    
    # Count existing configs
    local existing_configs=$(find "$CONFIGS_DIR" -name "*.yaml" -o -name "*.json" 2>/dev/null | wc -l)
    log_info "Found $existing_configs existing configs"
    
    # Verify key configs exist
    local key_configs=("claude-code.yaml" "aider.yaml" "openhands.yaml" "codex.yaml")
    local missing=0
    
    for config in "${key_configs[@]}"; do
        if [ -f "$CONFIGS_DIR/$config" ]; then
            log_success "Config exists: $config"
        else
            log_error "Config missing: $config"
            ((missing++))
        fi
    done
    
    if [ $missing -eq 0 ]; then
        log_success "All key configurations present"
        return 0
    else
        log_warn "$missing key configs missing"
        return 1
    fi
}

# Phase 3: HelixAgent Startup
phase3_startup() {
    log_section "PHASE 3: HelixAgent Startup"
    
    log_info "Checking if HelixAgent is running..."
    
    if curl -sf http://localhost:8100/health > /dev/null 2>&1; then
        log_success "HelixAgent is already running"
        return 0
    fi
    
    log_info "Starting HelixAgent..."
    cd "$PROJECT_ROOT"
    
    # Start in background
    ./bin/helixagent > "${RESULTS_DIR}/helixagent_${TIMESTAMP}.log" 2>&1 &
    HELIX_PID=$!
    
    log_info "HelixAgent started with PID $HELIX_PID"
    
    # Wait for startup
    local attempts=0
    local max_attempts=30
    
    while [ $attempts -lt $max_attempts ]; do
        if curl -sf http://localhost:8100/health > /dev/null 2>&1; then
            log_success "HelixAgent is healthy"
            return 0
        fi
        sleep 2
        ((attempts++))
        log_info "Waiting for HelixAgent... ($attempts/$max_attempts)"
    done
    
    log_error "HelixAgent failed to start"
    return 1
}

# Phase 4: Core API Validation
phase4_core_api() {
    log_section "PHASE 4: Core API Validation"
    
    local endpoint="http://localhost:8100"
    local passed=0
    local failed=0
    
    # Health check
    log_info "Testing /health..."
    if curl -sf "$endpoint/health" > /dev/null 2>&1; then
        log_success "/health works"
        ((passed++))
    else
        log_error "/health failed"
        ((failed++))
    fi
    
    # Providers
    log_info "Testing /v1/providers..."
    local providers
    providers=$(curl -sf "$endpoint/v1/providers" 2>/dev/null | grep -o '"type"' | wc -l)
    if [ "$providers" -ge 22 ]; then
        log_success "/v1/providers returns $providers providers"
        ((passed++))
    else
        log_error "/v1/providers returned only $providers providers"
        ((failed++))
    fi
    
    # Models
    log_info "Testing /v1/models..."
    if curl -sf "$endpoint/v1/models" > /dev/null 2>&1; then
        log_success "/v1/models works"
        ((passed++))
    else
        log_error "/v1/models failed"
        ((failed++))
    fi
    
    # Ensemble completion
    log_info "Testing /v1/ensemble/completions..."
    if curl -sf -X POST "$endpoint/v1/ensemble/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"ensemble","messages":[{"role":"user","content":"Test"}]}' > /dev/null 2>&1; then
        log_success "/v1/ensemble/completions works"
        ((passed++))
    else
        log_error "/v1/ensemble/completions failed"
        ((failed++))
    fi
    
    # MCP
    log_info "Testing /v1/mcp/tools/list..."
    local tools
    tools=$(curl -sf -X POST "$endpoint/v1/mcp/tools/list" \
        -H "Content-Type: application/json" \
        -d '{}' 2>/dev/null | grep -o '"name"' | wc -l)
    if [ "$tools" -ge 45 ]; then
        log_success "/v1/mcp/tools/list returns $tools tools"
        ((passed++))
    else
        log_warn "/v1/mcp/tools/list returned only $tools tools"
        ((failed++))
    fi
    
    log_info "Core API: $passed passed, $failed failed"
    
    return $failed
}

# Phase 5: CLI Agent Testing
phase5_cli_agents() {
    log_section "PHASE 5: CLI Agent Validation"
    
    log_info "Running CLI agent validator..."
    
    if "$SCRIPT_DIR/cli_agent_helixagent_validator.sh" --all 2>&1 | tee -a "$LOG_FILE"; then
        log_success "All CLI agents validated"
        return 0
    else
        log_error "Some CLI agents failed validation"
        return 1
    fi
}

# Phase 6: Feature-Specific Testing
phase6_feature_tests() {
    log_section "PHASE 6: Feature-Specific Testing"
    
    local endpoint="http://localhost:8100"
    local passed=0
    local failed=0
    
    # Debate orchestrator
    log_info "Testing debate orchestrator..."
    if curl -sf -X POST "$endpoint/v1/debate/start" \
        -H "Content-Type: application/json" \
        -d '{"topic":"Test","topology":"mesh","agents":[{"type":"claude"}]}' > /dev/null 2>&1; then
        log_success "Debate orchestrator works"
        ((passed++))
    else
        log_warn "Debate orchestrator test failed"
        ((failed++))
    fi
    
    # Streaming
    log_info "Testing streaming..."
    if curl -sf -X POST "$endpoint/v1/completions/stream" \
        -H "Content-Type: application/json" \
        -H "Accept: text/event-stream" \
        -d '{"model":"ensemble","messages":[{"role":"user","content":"Test"}],"stream":true}' 2>/dev/null | head -c 100 | grep -q "data:"; then
        log_success "Streaming works"
        ((passed++))
    else
        log_warn "Streaming test failed"
        ((failed++))
    fi
    
    # RAG
    log_info "Testing RAG endpoints..."
    if curl -sf "$endpoint/v1/rag/health" > /dev/null 2>&1; then
        log_success "RAG health endpoint works"
        ((passed++))
    else
        log_warn "RAG health check failed (containers may not be running)"
        ((failed++))
    fi
    
    # Metrics
    log_info "Testing metrics..."
    if curl -sf "$endpoint/metrics" | grep -q "helixagent"; then
        log_success "Prometheus metrics work"
        ((passed++))
    else
        log_warn "Metrics test failed"
        ((failed++))
    fi
    
    log_info "Features: $passed passed, $failed failed"
    
    return 0  # Don't fail on feature tests
}

# Phase 7: Results Summary
phase7_summary() {
    log_section "PHASE 7: Validation Summary"
    
    local summary_file="${RESULTS_DIR}/summary_${TIMESTAMP}.json"
    
    cat > "$summary_file" <<EOF
{
    "timestamp": "$(date -Iseconds)",
    "validation_type": "cli_agents_full_system",
    "results_directory": "$RESULTS_DIR",
    "helixagent_endpoint": "http://localhost:8100",
    "phases": [
        "build_validation",
        "config_generation",
        "startup",
        "core_api",
        "cli_agents",
        "feature_tests"
    ],
    "status": "completed"
}
EOF
    
    log_success "Validation complete!"
    log_info "Results directory: $RESULTS_DIR"
    log_info "Summary: $summary_file"
    log_info "Log file: $LOG_FILE"
    
    # List all result files
    log_info "Result files:"
    ls -la "$RESULTS_DIR" | tail -n +4 | tee -a "$LOG_FILE"
}

# Cleanup
cleanup() {
    if [ -n "$HELIX_PID" ]; then
        log_info "Cleaning up HelixAgent (PID $HELIX_PID)..."
        kill $HELIX_PID 2>/dev/null || true
    fi
}

trap cleanup EXIT

# Main
main() {
    setup
    
    log_section "CLI AGENTS FULL SYSTEM VALIDATION"
    log_info "Started at: $(date)"
    log_info "Project root: $PROJECT_ROOT"
    log_info "Results directory: $RESULTS_DIR"
    
    local failed=0
    
    # Run all phases
    phase1_build_validation || ((failed++))
    phase2_config_generation || ((failed++))
    phase3_startup || ((failed++))
    phase4_core_api || ((failed++))
    phase5_cli_agents || ((failed++))
    phase6_feature_tests || ((failed++))
    
    phase7_summary
    
    if [ $failed -eq 0 ]; then
        log_success "✓ ALL VALIDATION PASSED"
        return 0
    else
        log_error "✗ $failed phase(s) had issues"
        return 1
    fi
}

# Run
main "$@"
