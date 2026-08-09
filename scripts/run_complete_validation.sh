#!/bin/bash
# Complete Validation Suite for CLI Agents Porting
# Runs all tests with detailed provider validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Readiness + test-execution assertions. See the header of that file for why
# `curl -s` is not a readiness check and why the port is discovered rather
# than hardcoded (§11.4.201, §11.4.111, §11.4.6).
# shellcheck source=lib/helixagent_readiness.sh
source "$SCRIPT_DIR/lib/helixagent_readiness.sh"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log_header() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC} ${BOLD}$1${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Check prerequisites
check_prerequisites() {
    log_header "CHECKING PREREQUISITES"
    
    local missing=()
    
    # Check Go
    if ! command -v go &> /dev/null; then
        missing+=("go")
    else
        log_success "Go: $(go version)"
    fi
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        missing+=("docker")
    else
        log_success "Docker: $(docker --version)"
    fi
    
    # Check docker-compose
    if ! command -v docker-compose &> /dev/null; then
        missing+=("docker-compose")
    else
        log_success "Docker Compose: $(docker-compose --version)"
    fi
    
    # Check jq
    if ! command -v jq &> /dev/null; then
        missing+=("jq")
    else
        log_success "jq: $(jq --version)"
    fi
    
    # Check psql
    if ! command -v psql &> /dev/null; then
        missing+=("postgresql-client")
    else
        log_success "PostgreSQL client: $(psql --version | head -1)"
    fi
    
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "Missing prerequisites: ${missing[*]}"
        log_info "Install with:"
        log_info "  Ubuntu/Debian: sudo apt-get install ${missing[*]}"
        log_info "  macOS: brew install ${missing[*]}"
        exit 1
    fi
    
    log_success "All prerequisites met"
}

# Phase 1: Environment Setup
setup_environment() {
    log_header "PHASE 1: ENVIRONMENT SETUP"
    
    log_info "Starting infrastructure containers..."
    docker-compose up -d postgres redis
    
    log_info "Waiting for PostgreSQL to be ready..."
    for i in {1..30}; do
        if pg_isready -h localhost -p 5432 -U helixagent > /dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    
    if ! pg_isready -h localhost -p 5432 -U helixagent > /dev/null 2>&1; then
        log_error "PostgreSQL failed to start"
        exit 1
    fi
    log_success "PostgreSQL is ready"
    
    log_info "Waiting for Redis to be ready..."
    for i in {1..30}; do
        if redis-cli -h localhost -p 6379 ping > /dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    log_success "Redis is ready"
    
    log_info "Running database migrations..."
    if [ -f "bin/helixagent" ]; then
        ./bin/helixagent --migrate 2>&1 | tail -10 || log_warn "Migration may have already run"
    else
        log_warn "helixagent binary not found, skipping migration"
    fi
    
    log_info "Seeding test data..."
    ./scripts/seed_test_data.sh || log_warn "Seeding may have already been done"
}

# Phase 2: Build
build_binaries() {
    log_header "PHASE 2: BUILDING BINARIES"
    
    log_info "Cleaning previous builds..."
    make clean 2>&1 | tail -5 || true
    
    log_info "Building all applications..."
    if ! make build-all 2>&1 | tail -30; then
        log_error "Build failed"
        exit 1
    fi
    
    log_success "Build completed"
    
    # Verify binaries
    log_info "Verifying binaries:"
    for binary in helixagent api grpc-server cognee-mock sanity-check mcp-bridge generate-constitution; do
        if [ -f "bin/$binary" ]; then
            local size=$(du -h "bin/$binary" | cut -f1)
            log_success "  ✓ $binary ($size)"
        else
            log_warn "  ✗ $binary not found"
        fi
    done
}

# Phase 3: Unit Tests
run_unit_tests() {
    log_header "PHASE 3: UNIT TESTS"
    
    export GOMAXPROCS=2
    
    log_info "Running CLIS package tests..."
    if ! nice -n 19 ionice -c 3 go test ./internal/clis/... -v -race -coverprofile=coverage_clis.out 2>&1 | tee logs/test_clis.log | tail -50; then
        log_warn "Some CLIS tests failed (see logs/test_clis.log)"
    else
        log_success "CLIS tests passed"
    fi
    
    log_info "Running Ensemble package tests..."
    if ! nice -n 19 ionice -c 3 go test ./internal/ensemble/... -v -race -coverprofile=coverage_ensemble.out 2>&1 | tee logs/test_ensemble.log | tail -50; then
        log_warn "Some Ensemble tests failed (see logs/test_ensemble.log)"
    else
        log_success "Ensemble tests passed"
    fi
}

# Phase 4: Integration Tests
run_integration_tests() {
    log_header "PHASE 4: INTEGRATION TESTS"
    
    log_info "Starting HelixAgent..."
    ./bin/helixagent > logs/helixagent.log 2>&1 &
    HELIX_PID=$!

    # Readiness = "the server WE started identifies itself at the address it
    # actually bound", not "something answered :8100". The old check used bare
    # `curl -s`, which exits 0 on the 404 a foreign occupant returns, so it
    # passed without ever reaching this server.
    log_info "Waiting for HelixAgent to be ready..."
    local HELIX_URL
    if ! HELIX_URL="$(helix_wait_ready "$HELIX_PID" 60)"; then
        log_error "HelixAgent failed to become ready (check logs/helixagent.log)"
        kill $HELIX_PID 2>/dev/null || true
        return 1
    fi
    # Point the tests at the server we just verified. Without this they keep
    # their :8100 default and assert against whatever holds that port.
    export HELIXAGENT_URL="$HELIX_URL"

    log_success "HelixAgent is running (PID: $HELIX_PID) at $HELIX_URL"

    # Check providers endpoint
    log_info "Checking providers endpoint..."
    curl -sf "$HELIX_URL/v1/providers" | jq '.' > logs/providers_list.json 2>/dev/null || true

    # This script STARTED the server and just proved its identity, so a test
    # that cannot find it is a real failure here, not an honest environment
    # skip. Without this, the guarded tests skip and `go test` still exits 0.
    export HELIXAGENT_REQUIRED=1

    log_info "Running integration tests..."
    local integration_exit=0
    nice -n 19 ionice -c 3 go test ./tests/integration/... -v -timeout 10m 2>&1 \
        | tee logs/test_integration.log | tail -50
    integration_exit=${PIPESTATUS[0]}

    # A suite that ran nothing is not a suite that passed: `go test` exits 0
    # for an all-skipped package, so exit status alone cannot tell "everything
    # passed" from "nothing executed".
    if ! helix_assert_tests_executed logs/test_integration.log "Integration tests"; then
        log_error "Integration tests executed ZERO test cases — nothing was validated"
        kill $HELIX_PID 2>/dev/null || true
        return 1
    fi

    # Genuine test failures keep this script's existing gather-everything
    # posture (warn and continue, as the unit phases do). Only "nothing ran"
    # is fatal: that means the MEASUREMENT was invalid, not that the product
    # is broken, so continuing would report a verdict about nothing.
    if [ "$integration_exit" -ne 0 ]; then
        log_warn "Some integration tests failed (see logs/test_integration.log)"
    else
        log_success "Integration tests passed ($(helix_count_executed_tests logs/test_integration.log) test cases executed)"
    fi

    log_info "Running LLMsVerifier (comprehensive provider validation)..."
    if ! ./scripts/run_llms_verifier.sh 2>&1 | tee logs/test_llms_verifier.log; then
        log_warn "Some providers failed validation (see docs/reports/llms_verifier/$(date +%Y-%m-%d)/)"
    else
        log_success "Provider validation completed"
    fi
    
    log_info "Shutting down HelixAgent..."
    kill $HELIX_PID 2>/dev/null || true
    wait $HELIX_PID 2>/dev/null || true
}

# Phase 5: Challenge Tests
run_challenge_tests() {
    log_header "PHASE 5: CHALLENGE TESTS"
    
    # Start HelixAgent again for challenges
    log_info "Starting HelixAgent for challenges..."
    ./bin/helixagent > logs/helixagent_challenges.log 2>&1 &
    HELIX_PID=$!

    # Same identity-based readiness gate as Phase 4: the challenge scripts read
    # HELIXAGENT_URL, so exporting the address we actually verified keeps them
    # pointed at this server rather than at whatever holds the default port.
    local HELIX_URL
    if ! HELIX_URL="$(helix_wait_ready "$HELIX_PID" 60)"; then
        log_error "HelixAgent failed to become ready for challenges (check logs/helixagent_challenges.log)"
        kill $HELIX_PID 2>/dev/null || true
        return 1
    fi
    export HELIXAGENT_URL="$HELIX_URL"
    log_success "HelixAgent is running for challenges (PID: $HELIX_PID) at $HELIX_URL"

    # Run challenges
    CHALLENGES=(
        "tests/challenges/ensemble_voting_challenge.sh"
        "tests/challenges/multi_strategy_challenge.sh"
        "tests/challenges/performance_challenge.sh"
    )
    
    for challenge in "${CHALLENGES[@]}"; do
        if [ -f "$challenge" ]; then
            log_info "Running challenge: $(basename $challenge)"
            chmod +x "$challenge"
            if bash "$challenge" 2>&1 | tee -a logs/test_challenges.log | tail -30; then
                log_success "Challenge passed: $(basename $challenge)"
            else
                log_warn "Challenge had issues: $(basename $challenge)"
            fi
        else
            log_warn "Challenge not found: $challenge"
        fi
    done
    
    kill $HELIX_PID 2>/dev/null || true
    wait $HELIX_PID 2>/dev/null || true
}

# Phase 6: Coverage Report
generate_coverage() {
    log_header "PHASE 6: COVERAGE REPORT"
    
    log_info "Generating coverage reports..."
    
    # Merge coverage files
    if [ -f coverage_clis.out ] && [ -f coverage_ensemble.out ]; then
        echo "mode: set" > coverage_merged.out
        grep -h -v "^mode:" coverage_clis.out coverage_ensemble.out >> coverage_merged.out
        
        go tool cover -html=coverage_merged.out -o coverage_report.html 2>&1 || true
        log_success "Coverage report: coverage_report.html"
        
        # Show summary
        log_info "Coverage summary:"
        go tool cover -func=coverage_merged.out 2>&1 | tail -20 || true
    else
        log_warn "Coverage files not found"
    fi
}

# Phase 7: Final Summary
print_summary() {
    log_header "VALIDATION COMPLETE"
    
    local llms_report="docs/reports/llms_verifier/$(date +%Y-%m-%d)/report_latest.md"
    
    echo ""
    echo -e "${BOLD}📊 Test Results Summary:${NC}"
    echo "═════════════════════════════════════════════════════════════════"
    
    # NOTE ON `grep -c`: it PRINTS "0" and ALSO exits 1 when there are no
    # matches, so an `|| echo "0"` fallback emits TWO values. They collapse
    # through command substitution into the string $'0\n0', which renders as
    # "0\n0 packages" and makes any `[ "$n" -eq 0 ]` guard abort with
    # "integer expression expected" — a non-zero status that reads as
    # "not zero", i.e. "tests ran". Same footgun documented in
    # lib/helixagent_readiness.sh; `|| true` is the correct suppressor
    # because grep has already printed the count.
    if [ -f logs/test_clis.log ]; then
        local clis_passed
        clis_passed=$(grep -c "^---.*PASS" logs/test_clis.log 2>/dev/null || true)
        echo -e "  CLIS Tests:           ${GREEN}${clis_passed:-0} packages${NC}"
    fi

    if [ -f logs/test_ensemble.log ]; then
        local ensemble_passed
        ensemble_passed=$(grep -c "^---.*PASS" logs/test_ensemble.log 2>/dev/null || true)
        echo -e "  Ensemble Tests:       ${GREEN}${ensemble_passed:-0} packages${NC}"
    fi
    
    if [ -f logs/test_integration.log ]; then
        echo -e "  Integration Tests:    ${GREEN}Completed${NC}"
    fi
    
    if [ -f logs/test_llms_verifier.log ]; then
        local providers_ok
        providers_ok=$(grep -c "\[PASS\].*Provider is healthy" logs/test_llms_verifier.log 2>/dev/null || true)
        echo -e "  Providers Healthy:    ${GREEN}${providers_ok:-0}${NC}"
    fi
    
    if [ -f logs/test_challenges.log ]; then
        echo -e "  Challenge Tests:      ${GREEN}Completed${NC}"
    fi
    
    echo ""
    echo -e "${BOLD}📁 Generated Reports:${NC}"
    echo "═════════════════════════════════════════════════════════════════"
    [ -f coverage_report.html ] && echo "  📊 coverage_report.html"
    [ -f logs/providers_list.json ] && echo "  📋 logs/providers_list.json"
    [ -f "$llms_report" ] && echo "  📋 $llms_report"
    
    # Show LLMsVerifier summary if available
    if [ -f "$llms_report" ]; then
        echo ""
        echo -e "${BOLD}🤖 Provider Validation Summary:${NC}"
        echo "═════════════════════════════════════════════════════════════════"
        grep -A 20 "## Executive Summary" "$llms_report" 2>/dev/null | head -15 || true
    fi
    
    echo ""
    echo -e "${BOLD}📂 Log Files:${NC}"
    echo "═════════════════════════════════════════════════════════════════"
    ls -1 logs/*.log 2>/dev/null | head -10 || echo "  No log files found"
    
    echo ""
    echo -e "${CYAN}Next Steps:${NC}"
    echo "  1. Review detailed provider report:"
    echo "     cat $llms_report"
    echo "  2. Open coverage report:"
    echo "     open coverage_report.html  # or xdg-open on Linux"
    echo "  3. Check detailed logs:"
    echo "     ls -la logs/"
    echo ""
    
    log_success "Complete validation finished! 🎉"
}

# Main execution
main() {
    # Create logs directory
    mkdir -p logs
    
    # Parse arguments
    SKIP_SETUP=false
    SKIP_BUILD=false
    QUICK_MODE=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-setup) SKIP_SETUP=true ;;
            --skip-build) SKIP_BUILD=true ;;
            --quick) QUICK_MODE=true ;;
            *) log_warn "Unknown option: $1" ;;
        esac
        shift
    done
    
    # Run phases
    check_prerequisites
    
    if [ "$SKIP_SETUP" = false ]; then
        setup_environment
    fi
    
    if [ "$SKIP_BUILD" = false ]; then
        build_binaries
    fi
    
    run_unit_tests
    run_integration_tests
    
    if [ "$QUICK_MODE" = false ]; then
        run_challenge_tests
    fi
    
    generate_coverage
    print_summary
}

# Run main
main "$@"
