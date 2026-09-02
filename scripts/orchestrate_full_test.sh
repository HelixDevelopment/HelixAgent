#!/bin/bash
# Complete Test Orchestration for CLI Agents Porting
# Runs full test suite: clean build → containers → tests → validation

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

# ============================================================================
# HXC-255: failure accumulator
# ============================================================================
# Mirrors the accumulator idiom already used by the sibling runners
# run_complete_test_suite.sh / run_full_automation.sh: every phase records
# its OWN pass/fail outcome, and a genuine test FAILURE sets
# VALIDATION_EXIT_CODE=1 instead of only being logged as a warning. The
# script now exits non-zero at the end whenever this is set, so a real
# failure can no longer be reported as a clean, green run (§11.4.1).
#
# This is distinct from an honest SKIP (§11.4.3): absent hardware,
# credentials, or topology (an optional tool/binary/submodule genuinely not
# present, e.g. the ../helix_qa checkout) is a legitimate reason a phase
# cannot run at all, and stays a SKIP rather than being counted as a
# failure here. A go test / challenge script that ran and returned
# non-zero, or a challenge file this suite expects but that does not exist
# on disk, is a real failure and always counts.
VALIDATION_EXIT_CODE=0
CHALLENGES_FAILED=false

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

    # HXC-255: these three pipelines used to run with no exit-code capture
    # at all — not even a warning. `cmd | tee logfile | tail -N` reports
    # `tail`'s status (near-always 0), never `go test`'s, and there was no
    # `if`/`PIPESTATUS` check of any kind, so a genuine unit-test failure was
    # completely invisible and the script proceeded as if nothing happened.
    # Capture the real status via PIPESTATUS, mirroring the pattern already
    # used a few phases down for integration/E2E/stress/security tests.
    log_info "Running CLIS package tests..."
    nice -n 19 ionice -c 3 go test ./internal/clis/... -v -race -coverprofile=coverage_clis.out 2>&1 | tee test_output_clis.log | tail -30
    CLIS_EXIT=${PIPESTATUS[0]}
    if [ "$CLIS_EXIT" -eq 0 ]; then
        log_success "CLIS package tests passed"
    else
        log_error "CLIS package tests FAILED (exit: $CLIS_EXIT)"
        VALIDATION_EXIT_CODE=1
    fi

    log_info "Running Ensemble package tests..."
    nice -n 19 ionice -c 3 go test ./internal/ensemble/... -v -race -coverprofile=coverage_ensemble.out 2>&1 | tee test_output_ensemble.log | tail -30
    ENSEMBLE_EXIT=${PIPESTATUS[0]}
    if [ "$ENSEMBLE_EXIT" -eq 0 ]; then
        log_success "Ensemble package tests passed"
    else
        log_error "Ensemble package tests FAILED (exit: $ENSEMBLE_EXIT)"
        VALIDATION_EXIT_CODE=1
    fi

    log_info "Running remaining internal tests..."
    nice -n 19 ionice -c 3 go test ./internal/... -v -race -short -coverprofile=coverage_internal.out 2>&1 | tee test_output_internal.log | tail -50
    INTERNAL_EXIT=${PIPESTATUS[0]}
    if [ "$INTERNAL_EXIT" -eq 0 ]; then
        log_success "Unit tests completed"
    else
        log_error "Remaining internal tests FAILED (exit: $INTERNAL_EXIT)"
        VALIDATION_EXIT_CODE=1
    fi
else
    log_warn "Unit tests skipped"
fi

# Phase 4: Integration Tests
if [ "$RUN_INTEGRATION" = true ]; then
    log_section "PHASE 4: Integration Tests"

    log_info "Starting HelixAgent for integration tests..."
    ./bin/helixagent &
    HELIX_PID=$!

    # Readiness = "the server WE started identifies itself at the address it
    # actually bound", not "something answered :8100". The old check used bare
    # `curl -s`, which exits 0 on the 404 a foreign occupant returns, so it
    # passed without ever reaching this server.
    if ! HELIX_URL="$(helix_wait_ready "$HELIX_PID" 60)"; then
        log_error "HelixAgent failed to become ready"
        kill $HELIX_PID 2>/dev/null || true
        exit 1
    fi
    # Point the tests at the server we just verified. Without this they keep
    # their :8100 default and assert against whatever holds that port.
    export HELIXAGENT_URL="$HELIX_URL"
    log_success "HelixAgent is running (PID: $HELIX_PID) at $HELIX_URL"

    # This script STARTED the server and just proved its identity, so a test
    # that cannot find it is a real failure here, not an honest environment
    # skip. Without this, the guarded tests skip and `go test` still exits 0.
    export HELIXAGENT_REQUIRED=1

    log_info "Running integration tests..."
    nice -n 19 ionice -c 3 go test ./tests/integration/... -v -timeout 10m 2>&1 | tee test_output_integration.log | tail -50
    INTEGRATION_EXIT=${PIPESTATUS[0]}

    log_info "Shutting down HelixAgent..."
    kill $HELIX_PID 2>/dev/null || true
    wait $HELIX_PID 2>/dev/null || true

    # A suite that ran nothing is not a suite that passed: `go test` exits 0
    # for an all-skipped package, so exit status alone cannot tell "everything
    # passed" from "nothing executed".
    if ! helix_assert_tests_executed test_output_integration.log "Integration tests"; then
        log_error "Integration tests executed ZERO test cases — nothing was validated"
        exit 1
    fi

    # HXC-255: a genuine test failure here used to only log_error and
    # continue with no effect on the script's exit code. It now sets the
    # accumulator so the run cannot end green while integration tests are
    # red.
    if [ $INTEGRATION_EXIT -eq 0 ]; then
        log_success "Integration tests passed ($(helix_count_executed_tests test_output_integration.log) test cases executed)"
    else
        log_error "Integration tests failed (exit: $INTEGRATION_EXIT)"
        VALIDATION_EXIT_CODE=1
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

    # HXC-255: real E2E failures used to only log_error with no effect on
    # the script's exit code.
    if [ $E2E_EXIT -eq 0 ]; then
        log_success "E2E tests passed"
    else
        log_error "E2E tests FAILED (exit: $E2E_EXIT)"
        VALIDATION_EXIT_CODE=1
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

    # HXC-255: a non-zero `go test` exit here is a genuine test failure, not
    # an environment limitation — an honest SKIP for absent hardware would
    # show up as a skipped/all-`[no test files]` run with exit 0, which this
    # branch never sees. "May be expected in limited environments" was
    # downgrading real failures to a warning with no effect on the script's
    # exit code; it now counts like every other test phase.
    if [ $STRESS_EXIT -eq 0 ]; then
        log_success "Stress tests passed"
    else
        log_error "Stress tests FAILED (exit: $STRESS_EXIT)"
        VALIDATION_EXIT_CODE=1
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

    # HXC-255: real security-test failures used to only log_warn with no
    # effect on the script's exit code.
    if [ $SECURITY_EXIT -eq 0 ]; then
        log_success "Security tests passed"
    else
        log_error "Security tests FAILED (exit: $SECURITY_EXIT)"
        VALIDATION_EXIT_CODE=1
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

    if [ -d "../helix_qa" ]; then
        log_info "Running HelixQA test bank (canonical at meta-repo root ../helix_qa per P1.5-T03.04)..."

        # Check if HelixQA has its own test runner
        if [ -f "../helix_qa/bin/run_tests" ]; then
            log_info "Using HelixQA runner..."
            cd ../helix_qa
            ./bin/run_tests --all 2>&1 | tee ../helix_agent/test_output_helixqa.log | tail -100
            cd ../helix_agent
        else
            log_info "Running HelixQA tests directly..."
            # Run Go tests for HelixQA if available
            if [ -f "../helix_qa/go.mod" ]; then
                cd ../helix_qa
                nice -n 19 ionice -c 3 go test ./... -v 2>&1 | tee ../helix_agent/test_output_helixqa.log | tail -100
                cd ../helix_agent
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
        # HXC-255: tests/challenges/performance_challenge.sh has never
        # existed in this repository (confirmed missing during the HXC-255
        # blast-radius analysis — this is a tracked gap, not a transient
        # issue). It is kept in this array deliberately so its absence is
        # reported below as a MISSING EXPECTED CHALLENGE and fails the run,
        # instead of being silently warned-past or quietly dropped from the
        # list. Resolving this requires a deliberate, tracked decision:
        # either author tests/challenges/performance_challenge.sh, or remove
        # this entry as an explicit, reviewed change — not as a side effect
        # of this fix.
        "tests/challenges/performance_challenge.sh"
    )

    for script in "${CHALLENGE_SCRIPTS[@]}"; do
        if [ -f "$script" ]; then
            log_info "Running challenge: $script"
            chmod +x "$script"
            bash "$script" 2>&1 | tee -a test_output_challenges.log | tail -50
            CHALLENGE_EXIT=${PIPESTATUS[0]}
            # HXC-255: a failing challenge used to only log_warn with no
            # effect on the script's exit code.
            if [ $CHALLENGE_EXIT -eq 0 ]; then
                log_success "Challenge passed: $(basename $script)"
            else
                log_error "Challenge FAILED: $(basename $script) (exit: $CHALLENGE_EXIT)"
                CHALLENGES_FAILED=true
                VALIDATION_EXIT_CODE=1
            fi
        else
            # HXC-255: a script this suite expects but that does not exist on
            # disk is a maintenance gap, not an honest environment SKIP — it
            # is reported and treated as a failure so the gap stays visible.
            log_error "MISSING EXPECTED CHALLENGE: $script (file does not exist on disk)"
            CHALLENGES_FAILED=true
            VALIDATION_EXIT_CODE=1
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
        # HXC-255: this pipeline used to run with no exit-code capture at
        # all — `tee`'s status masked run_llms_verifier.sh's real one and
        # there was no `if`/`PIPESTATUS` check, so a real provider-validation
        # failure was invisible. Capture it via PIPESTATUS, consistent with
        # run_complete_validation.sh's treatment of the same tool.
        bash scripts/run_llms_verifier.sh 2>&1 | tee test_output_llmsverifier.log | tail -100
        LLMSVERIFIER_EXIT=${PIPESTATUS[0]}
        if [ "$LLMSVERIFIER_EXIT" -eq 0 ]; then
            log_success "LLMsVerifier completed"
        else
            log_error "LLMsVerifier FAILED (exit: $LLMSVERIFIER_EXIT)"
            VALIDATION_EXIT_CODE=1
        fi
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
    # `grep -c` prints "0" AND exits 1 on no-match, so `|| echo "0"` emits two
    # values that collapse to $'0\n0' — rendered as a broken two-line count and
    # fatal to any `-eq` guard. See lib/helixagent_readiness.sh for the same
    # footgun; `|| true` suppresses the status without adding a second value.
    UNIT_STATUS=$(grep -c "^---.*PASS" test_output_clis.log 2>/dev/null || true)
    echo -e "  Unit Tests:          ${GREEN}${UNIT_STATUS:-0} packages${NC}"
fi

# HXC-255: the four blocks below used to print an unconditional green
# "Completed" the moment the phase's log file existed, regardless of
# whether the tests it recorded actually passed. Integration/E2E/Challenges
# now report the real per-phase exit status captured above (§11.4.1 — never
# report a PASS that was not earned). HelixQA has no reliable per-run
# status to report (its several code paths — external runner, `go test`,
# or skip — don't all set a common exit-status variable), so it gets an
# honest neutral "ran, result not verified" instead of a false green.
if [ "$RUN_INTEGRATION" = true ]; then
    if [ -f test_output_integration.log ]; then
        if [ "${INTEGRATION_EXIT:-1}" -eq 0 ]; then
            echo -e "  Integration Tests:   ${GREEN}PASS${NC}"
        else
            echo -e "  Integration Tests:   ${RED}FAIL${NC}"
        fi
    else
        echo -e "  Integration Tests:   ${YELLOW}No log${NC}"
    fi
fi

if [ "$RUN_E2E" = true ]; then
    if [ -f test_output_e2e.log ]; then
        if [ "${E2E_EXIT:-1}" -eq 0 ]; then
            echo -e "  E2E Tests:           ${GREEN}PASS${NC}"
        else
            echo -e "  E2E Tests:           ${RED}FAIL${NC}"
        fi
    else
        echo -e "  E2E Tests:           ${YELLOW}No log${NC}"
    fi
fi

if [ "$RUN_HELIXQA" = true ]; then
    if [ -f test_output_helixqa.log ]; then
        echo -e "  HelixQA:             ${YELLOW}Ran (result not verified)${NC}"
    else
        echo -e "  HelixQA:             ${YELLOW}No log${NC}"
    fi
fi

if [ "$RUN_CHALLENGES" = true ]; then
    if [ -f test_output_challenges.log ]; then
        if [ "$CHALLENGES_FAILED" = true ]; then
            echo -e "  Challenges:          ${RED}FAIL${NC}"
        else
            echo -e "  Challenges:          ${GREEN}PASS${NC}"
        fi
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

# HXC-255: this used to be an unconditional log_success regardless of
# whether any phase above actually failed.
if [ "$VALIDATION_EXIT_CODE" -eq 0 ]; then
    log_success "All tests completed! 🎉"
else
    log_error "Test orchestration completed WITH FAILURES — see phase results above."
fi
echo ""
echo -e "${CYAN}Next steps:${NC}"
echo "  1. Review test logs: less test_output_*.log"
echo "  2. Check coverage:   open coverage_report.html"
echo "  3. View LLMs report: cat docs/reports/llms_verifier/$(date +%Y-%m-%d)/report.md"
echo ""

# HXC-255: the orchestrator used to fall off the end here and always exit 0,
# regardless of any FAIL recorded above. It now exits non-zero whenever a
# real test failure (not an honest SKIP) occurred.
exit "$VALIDATION_EXIT_CODE"
