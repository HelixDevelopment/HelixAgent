#!/bin/bash
#
# SkillRegistry Challenge Script
# Validates SkillRegistry module functionality
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Challenge metadata
CHALLENGE_NAME="SkillRegistry Module"
CHALLENGE_POINTS=100

log_info "Starting $CHALLENGE_NAME Challenge..."
log_info "Target: $CHALLENGE_POINTS points"
echo ""

# Track score
SCORE=0
TOTAL_CHECKS=10

# Check 1: SkillRegistry directory exists
log_info "Check 1: Verifying SkillRegistry module structure..."
if [ -d "$PROJECT_ROOT/SkillRegistry" ]; then
    if [ -f "$PROJECT_ROOT/SkillRegistry/types.go" ]; then
        if [ -f "$PROJECT_ROOT/SkillRegistry/loader.go" ]; then
            if [ -f "$PROJECT_ROOT/SkillRegistry/executor.go" ]; then
                log_info "✓ SkillRegistry module structure complete"
                SCORE=$((SCORE + 10))
            else
                log_error "✗ executor.go not found"
            fi
        else
            log_error "✗ loader.go not found"
        fi
    else
        log_error "✗ types.go not found"
    fi
else
    log_error "✗ SkillRegistry directory not found"
fi

# Check 2: Test files exist
log_info "Check 2: Verifying test coverage..."
TEST_FILES=$(find "$PROJECT_ROOT/SkillRegistry" -name "*_test.go" 2>/dev/null | wc -l)
if [ "$TEST_FILES" -ge 5 ]; then
    log_info "✓ Test files present ($TEST_FILES files)"
    SCORE=$((SCORE + 10))
else
    log_error "✗ Insufficient test files (found $TEST_FILES, need 5+)"
fi

# Check 3: Documentation exists
log_info "Check 3: Verifying documentation..."
if [ -f "$PROJECT_ROOT/SkillRegistry/README.md" ]; then
    log_info "✓ README.md exists"
    SCORE=$((SCORE + 10))
else
    log_error "✗ README.md not found"
fi

# Check 4: Module compiles
log_info "Check 4: Verifying compilation..."
cd "$PROJECT_ROOT"
if go build ./SkillRegistry/... 2>/dev/null; then
    log_info "✓ SkillRegistry module compiles successfully"
    SCORE=$((SCORE + 10))
else
    log_error "✗ Compilation failed"
fi

# Check 5: Unit tests pass
log_info "Check 5: Running unit tests..."
cd "$PROJECT_ROOT"
if go test -short ./SkillRegistry/... 2>/dev/null; then
    log_info "✓ Unit tests pass"
    SCORE=$((SCORE + 10))
else
    log_error "✗ Unit tests failed"
fi

# Check 6: Skill loading functionality
log_info "Check 6: Testing skill loading..."
if [ -f "$PROJECT_ROOT/SkillRegistry/loader.go" ]; then
    if grep -q "LoadSkillFromFile" "$PROJECT_ROOT/SkillRegistry/loader.go"; then
        log_info "✓ Skill loading functions implemented"
        SCORE=$((SCORE + 10))
    else
        log_error "✗ LoadSkillFromFile not found"
    fi
else
    log_error "✗ loader.go not found"
fi

# Check 7: Skill execution functionality
log_info "Check 7: Testing skill execution..."
if [ -f "$PROJECT_ROOT/SkillRegistry/executor.go" ]; then
    if grep -q "Execute" "$PROJECT_ROOT/SkillRegistry/executor.go"; then
        log_info "✓ Skill execution functions implemented"
        SCORE=$((SCORE + 10))
    else
        log_error "✗ Execute function not found"
    fi
else
    log_error "✗ executor.go not found"
fi

# Check 8: Storage implementations
log_info "Check 8: Verifying storage implementations..."
if [ -f "$PROJECT_ROOT/SkillRegistry/storage_memory.go" ] && [ -f "$PROJECT_ROOT/SkillRegistry/storage_postgres.go" ]; then
    log_info "✓ Storage implementations present"
    SCORE=$((SCORE + 10))
else
    log_error "✗ Storage implementations incomplete"
fi

# Check 9: Manager functionality
log_info "Check 9: Testing manager functionality..."
if [ -f "$PROJECT_ROOT/SkillRegistry/manager.go" ]; then
    if grep -q "Register" "$PROJECT_ROOT/SkillRegistry/manager.go" && grep -q "Get" "$PROJECT_ROOT/SkillRegistry/manager.go"; then
        log_info "✓ Manager functions implemented"
        SCORE=$((SCORE + 10))
    else
        log_error "✗ Manager functions incomplete"
    fi
else
    log_error "✗ manager.go not found"
fi

# Check 10: Test coverage
log_info "Check 10: Verifying test coverage..."
cd "$PROJECT_ROOT"
COVERAGE=$(go test -cover ./SkillRegistry/... 2>/dev/null | grep -oE '[0-9]+\.?[0-9]*%' | head -1 | tr -d '%')
if [ -n "$COVERAGE" ] && [ "${COVERAGE%.*}" -ge 90 ]; then
    log_info "✓ Test coverage is ${COVERAGE}% (>= 90%)"
    SCORE=$((SCORE + 10))
else
    log_warn "⚠ Test coverage is ${COVERAGE}% (target: 90%)"
    SCORE=$((SCORE + 5))
fi

echo ""
echo "========================================"
log_info "Challenge Complete!"
echo "Score: $SCORE/$CHALLENGE_POINTS points"
echo "========================================"

# Return appropriate exit code
if [ $SCORE -ge 80 ]; then
    log_info "✓ CHALLENGE PASSED!"
    exit 0
else
    log_error "✗ CHALLENGE FAILED"
    exit 1
fi
