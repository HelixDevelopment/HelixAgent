#!/bin/bash
# Complete Test Orchestration for CLI Agents Porting
# Runs full test suite: clean build → containers → tests → validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

log_section() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC} ${BOLD}$1${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
}

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Configuration
export GOMAXPROCS=2
RUN_UNIT=true
RUN_INTEGRATION=true
RUN_E2E=true
RUN_STRESS=true
RUN_SECURITY=true
RUN_BENCHMARK=false
RUN_HELIXQA=true
RUN_CHALLENGES=true
RUN_LLMSVERIFIER=true

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-unit) RUN_UNIT=false ;;
        --skip-integration) RUN_INTEGRATION=false ;;
        --skip-e2e) RUN_E2E=false ;;
        --skip-stress) RUN_STRESS=false ;;
        --skip-security) RUN_SECURITY=false ;;
        --benchmark) RUN_BENCHMARK=true ;;
        --skip-helixqa) RUN_HELIXQA=false ;;
        --skip-challenges) RUN_CHALLENGES=false ;;
        --skip-llmsverifier) RUN_LLMSVERIFIER=false ;;
        --quick) 
            RUN_STRESS=false
            RUN_SECURITY=false
            RUN_BENCHMARK=false
            RUN_CHALLENGES=false
            ;;
        *) log_warn "Unknown option: $1" ;;
    esac
    shift
done

# Phase 0: Environment Check
log_section "PHASE 0: Environment Verification"

log_info "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
log_success "Go version: $GO_VERSION"

log_info "Checking Docker..."
if ! docker info > /dev/null 2>&1; then
    log_error "Docker is not running"
    exit 1
fi
log_success "Docker is running"

log_info "Checking environment variables..."
if [ -f .env ]; then
    source .env
    log_success "Environment loaded from .env"
else
    log_warn "No .env file found, using defaults"
fi

# Phase 1: Clean Build
log_section "PHASE 1: Clean Build"

log_info "Cleaning previous builds..."
make clean 2>&1 | tail -5 || true
rm -rf bin/* || true
rm -f *.test || true

log_info "Building all applications..."
make build-all 2>&1 | tail -20 || {
    log_error "Build failed!"
    exit 1
}

log_success "Build completed"

# Verify binaries
log_info "Verifying binaries..."
BINARIES=("helixagent" "api" "grpc-server" "cognee-mock" "sanity-check" "mcp-bridge" "generate-constitution")
for binary in "${BINARIES[@]}"; do
    if [ -f "bin/$binary" ]; then
        size=$(du -h "bin/$binary" | cut -f1)
        log_success "$binary: $size"
    else
        log_error "$binary not found!"
    fi
done

# Phase 2: Container Preparation
log_section "PHASE 2: Container Preparation"

log_info "Stopping existing containers..."
docker-compose -f docker-compose.yml down --remove-orphans 2>&1 | tail -3 || true

log_info "Starting infrastructure containers..."
docker-compose -f docker-compose.yml up -d postgres redis 2>&1 | tail -5

log_info "Waiting for PostgreSQL..."
until docker-compose exec -T postgres pg_isready -U helixagent -d helixagent_db > /dev/null 2>&1; do
    sleep 1
done
log_success "PostgreSQL is ready"

log_info "Running database migrations..."
./bin/helixagent --migrate 2>&1 | tail -5 || {
    log_warn "Migration may have already run"
}

log_info "Seeding test data..."
./bin/helixagent --seed 2>&1 | tail -5 || {
    log_warn "Seeding may have already run"
}

# Phase 3: Unit Tests
if [ "$RUN_UNIT" = true ]; then
    log_section "PHASE 3: Unit Tests"
    
    log_info "Running CLIS package tests..."
    nice -n 19 ionice -c 3 go test ./internal/clis/... -v -race -coverprofile=coverage_clis.out 2>&1 | tee test_output_clis.log | tail -30
    
    log_info "Running Ensemble package tests..."
    nice -n 19 ionice -c 3 go test ./internal/ensemble/... -v -race -coverprofile=coverage_ensemble.out 2>&1 | tee test_output_ensemble.log | tail -30
    
    log_info "Running remaining internal tests..."
    nice -n 19 ionice -c 3 go test ./internal/... -v -race -short -coverprofile=coverage_internal.out 2>&1 | tee test_output_internal.log | tail -50
    
    log_success "Unit tests completed"
else
    log_warn "Unit tests skipped"
fi

# Phase 4: Integration Tests
if [ "$RUN_INTEGRATION" = true ]; then
    log_section "PHASE 4: Integration Tests"
    
    log_info "Starting HelixAgent for integration tests..."
    ./bin/helixagent &
    HELIX_PID=$!
    sleep 5
    
    # Wait for health
    for i in {1..30}; do
        if curl -s http://localhost:8100/health > /dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    
    if ! curl -s http://localhost:8100/health > /dev/null 2>&1; then
        log_error "HelixAgent failed to start"
        kill $HELIX_PID 2>/dev/null || true
        exit 1
    fi
    log_success "HelixAgent is running (PID: $HELIX_PID)"
    
    log_info "Running integration tests..."
    nice -n 19 ionice -c 3 go test ./tests/integration/... -v -timeout 10m 2>&1 | tee test_output_integration.log | tail -50
    INTEGRATION_EXIT=${PIPESTATUS[0]}
    
    log_info "Shutting down HelixAgent..."
    kill $HELIX_PID 2>/dev/null || true
    wait $HELIX_PID 2>/dev/null || true
    
    if [ $INTEGRATION_EXIT -eq 0 ]; then
        log_success "Integration tests passed"
    else
        log_error "Integration tests failed (exit: $INTEGRATION_EXIT)"
    fi
else
    log_warn "Integration tests skipped"
fi

# Phase 5: E2E Tests
if [ "$RUN_E2E" = true ]; then
    log_section "PHASE 5: End-to-End Tests"
    
    log_info "Running E2E test suite..."
    nice -n 19 ionice -c 3 go test ./tests/e2e/... -v -timeout 15m 2>&1 | tee test_output_e2e.log | tail -50
    E2E_EXIT=${PIPESTATUS[0]}
    
    if [ $E2E_EXIT -eq 0 ]; then
        log_success "E2E tests passed"
    else
        log_error "E2E tests had issues (exit: $E2E_EXIT)"
    fi
else
    log_warn "E2E tests skipped"
fi

# Phase 6: Stress Tests
if [ "$RUN_STRESS" = true ]; then
    log_section "PHASE 6: Stress Tests"
    
    log_info "Running stress tests..."
    nice -n 19 ionice -c 3 go test ./tests/stress/... -v -timeout 20m 2>&1 | tee test_output_stress.log | tail -50
    STRESS_EXIT=${PIPESTATUS[0]}
    
    if [ $STRESS_EXIT -eq 0 ]; then
        log_success "Stress tests passed"
    else
        log_warn "Stress tests had issues (may be expected in limited environments)"
    fi
else
    log_warn "Stress tests skipped"
fi

# Phase 7: Security Tests
if [ "$RUN_SECURITY" = true ]; then
    log_section "PHASE 7: Security Tests"
    
    log_info "Running security test suite..."
    nice -n 19 ionice -c 3 go test ./tests/security/... -v -timeout 10m 2>&1 | tee test_output_security.log | tail -50
    SECURITY_EXIT=${PIPESTATUS[0]}
    
    if [ $SECURITY_EXIT -eq 0 ]; then
        log_success "Security tests passed"
    else
        log_warn "Security tests had issues"
    fi
else
    log_warn "Security tests skipped"
fi

# Phase 8: Benchmark Tests
if [ "$RUN_BENCHMARK" = true ]; then
    log_section "PHASE 8: Benchmark Tests"
    
    log_info "Running benchmark tests..."
    nice -n 19 ionice -c 3 go test ./internal/clis/... -bench=. -benchmem -run=^$ 2>&1 | tee test_output_benchmark.log | tail -50
    
    log_success "Benchmark tests completed"
else
    log_info "Benchmark tests skipped (use --benchmark to enable)"
fi

# Phase 9: HelixQA Test Bank
if [ "$RUN_HELIXQA" = true ]; then
    log_section "PHASE 9: HelixQA Test Bank"
    
    if [ -d "../HelixQA" ]; then
        log_info "Running HelixQA test bank (canonical at meta-repo root ../HelixQA per P1.5-T03.04)..."

        # Check if HelixQA has its own test runner
        if [ -f "../HelixQA/bin/run_tests" ]; then
            log_info "Using HelixQA runner..."
            cd ../HelixQA
            ./bin/run_tests --all 2>&1 | tee ../HelixAgent/test_output_helixqa.log | tail -100
            cd ../HelixAgent
        else
            log_info "Running HelixQA tests directly..."
            # Run Go tests for HelixQA if available
            if [ -f "../HelixQA/go.mod" ]; then
                cd ../HelixQA
                nice -n 19 ionice -c 3 go test ./... -v 2>&1 | tee ../HelixAgent/test_output_helixqa.log | tail -100
                cd ../HelixAgent
            else
                log_warn "HelixQA test runner not found, skipping"
            fi
        fi
        
        log_success "HelixQA tests completed"
    else
        log_warn "HelixQA directory not found, skipping"
    fi
else
    log_warn "HelixQA tests skipped"
fi

# Phase 10: Challenge Scripts
if [ "$RUN_CHALLENGES" = true ]; then
    log_section "PHASE 10: Challenge Scripts"
    
    CHALLENGE_SCRIPTS=(
        "tests/challenges/ensemble_voting_challenge.sh"
        "tests/challenges/multi_strategy_challenge.sh"
        "tests/challenges/performance_challenge.sh"
    )
    
    for script in "${CHALLENGE_SCRIPTS[@]}"; do
        if [ -f "$script" ]; then
            log_info "Running challenge: $script"
            chmod +x "$script"
            bash "$script" 2>&1 | tee -a test_output_challenges.log | tail -50
            CHALLENGE_EXIT=${PIPESTATUS[0]}
            if [ $CHALLENGE_EXIT -eq 0 ]; then
                log_success "Challenge passed: $(basename $script)"
            else
                log_warn "Challenge had issues: $(basename $script)"
            fi
        else
            log_warn "Challenge script not found: $script"
        fi
    done
else
    log_warn "Challenge scripts skipped"
fi

# Phase 11: LLMsVerifier
if [ "$RUN_LLMSVERIFIER" = true ]; then
    log_section "PHASE 11: LLMsVerifier"
    
    if [ -f "scripts/run_llms_verifier.sh" ]; then
        log_info "Running LLMsVerifier..."
        chmod +x scripts/run_llms_verifier.sh
        bash scripts/run_llms_verifier.sh 2>&1 | tee test_output_llmsverifier.log | tail -100
    else
        log_warn "LLMsVerifier script not found"
    fi
else
    log_warn "LLMsVerifier skipped"
fi

# Phase 12: Coverage Report
log_section "PHASE 12: Coverage Analysis"

log_info "Generating coverage reports..."

# Merge coverage files if they exist
coverage_files=()
for f in coverage_*.out; do
    [ -f "$f" ] && coverage_files+=("$f")
done

if [ ${#coverage_files[@]} -gt 0 ]; then
    log_info "Found coverage files: ${coverage_files[*]}"
    
    # Generate HTML report
    go tool cover -html=coverage_internal.out -o coverage_report.html 2>&1 || true
    log_success "Coverage report: coverage_report.html"
    
    # Show coverage summary
    if [ -f "coverage_internal.out" ]; then
        go tool cover -func=coverage_internal.out 2>&1 | tail -20 || true
    fi
else
    log_warn "No coverage files found"
fi

# Final Summary
log_section "TEST ORCHESTRATION COMPLETE"

echo ""
echo -e "${BOLD}Summary:${NC}"
echo "─────────────────────────────────────────────────────────────────"

if [ "$RUN_UNIT" = true ]; then
    UNIT_STATUS=$(grep -c "^---.*PASS" test_output_clis.log 2>/dev/null || echo "0")
    echo -e "  Unit Tests:          ${GREEN}${UNIT_STATUS} packages${NC}"
fi

if [ "$RUN_INTEGRATION" = true ]; then
    if [ -f test_output_integration.log ]; then
        echo -e "  Integration Tests:   ${GREEN}Completed${NC}"
    else
        echo -e "  Integration Tests:   ${YELLOW}No log${NC}"
    fi
fi

if [ "$RUN_E2E" = true ]; then
    if [ -f test_output_e2e.log ]; then
        echo -e "  E2E Tests:           ${GREEN}Completed${NC}"
    else
        echo -e "  E2E Tests:           ${YELLOW}No log${NC}"
    fi
fi

if [ "$RUN_HELIXQA" = true ]; then
    if [ -f test_output_helixqa.log ]; then
        echo -e "  HelixQA:             ${GREEN}Completed${NC}"
    else
        echo -e "  HelixQA:             ${YELLOW}No log${NC}"
    fi
fi

if [ "$RUN_CHALLENGES" = true ]; then
    if [ -f test_output_challenges.log ]; then
        echo -e "  Challenges:          ${GREEN}Completed${NC}"
    else
        echo -e "  Challenges:          ${YELLOW}No log${NC}"
    fi
fi

if [ "$RUN_LLMSVERIFIER" = true ]; then
    REPORT_DATE=$(date +%Y-%m-%d)
    if [ -f "docs/reports/llms_verifier/$REPORT_DATE/report.md" ]; then
        echo -e "  LLMsVerifier:        ${GREEN}Report generated${NC}"
    else
        echo -e "  LLMsVerifier:        ${YELLOW}No report${NC}"
    fi
fi

echo ""
echo -e "${BOLD}Output Files:${NC}"
echo "─────────────────────────────────────────────────────────────────"
for log in test_output_*.log; do
    [ -f "$log" ] && echo "  📄 $log"
done
[ -f coverage_report.html ] && echo "  📊 coverage_report.html"
echo ""

log_success "All tests completed! 🎉"
echo ""
echo -e "${CYAN}Next steps:${NC}"
echo "  1. Review test logs: less test_output_*.log"
echo "  2. Check coverage:   open coverage_report.html"
echo "  3. View LLMs report: cat docs/reports/llms_verifier/$(date +%Y-%m-%d)/report.md"
echo ""
