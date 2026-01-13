#!/bin/bash

# sanity_check_challenge.sh - HelixAgent Boot Sanity Check Challenge
# Tests system readiness and validates all critical components

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default configuration
HOST="${HELIXAGENT_HOST:-localhost}"
PORT="${HELIXAGENT_PORT:-7061}"
RESULTS_DIR="${PROJECT_ROOT}/challenges/results/sanity_check/$(date +%Y%m%d_%H%M%S)"

echo ""
echo "======================================================================"
echo "              HELIXAGENT BOOT SANITY CHECK CHALLENGE"
echo "======================================================================"
echo ""
echo "Host: $HOST"
echo "Port: $PORT"
echo "Results: $RESULTS_DIR"
echo ""

# Create results directory
mkdir -p "$RESULTS_DIR/results"

# Build sanity check tool if needed
SANITY_CHECK_BIN="$PROJECT_ROOT/bin/sanity-check"
if [ ! -f "$SANITY_CHECK_BIN" ]; then
    echo -e "${BLUE}[BUILD]${NC} Building sanity-check tool..."
    cd "$PROJECT_ROOT"
    go build -o bin/sanity-check ./cmd/sanity-check
    if [ $? -ne 0 ]; then
        echo -e "${RED}[FAIL]${NC} Failed to build sanity-check tool"
        exit 1
    fi
    echo -e "${GREEN}[PASS]${NC} Built sanity-check tool"
fi

# Challenge tracking
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

run_check() {
    local name="$1"
    local cmd="$2"
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

    echo -n "  Checking $name... "
    if eval "$cmd" > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
        return 1
    fi
}

echo "----------------------------------------------------------------------"
echo "Phase 1: Pre-flight Checks"
echo "----------------------------------------------------------------------"

# Check if sanity-check binary exists and is executable
run_check "sanity-check binary" "[ -x '$SANITY_CHECK_BIN' ]"

# Check if HelixAgent binary exists
run_check "HelixAgent binary" "[ -f '$PROJECT_ROOT/bin/helixagent' ]"

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 2: Running Full Sanity Check"
echo "----------------------------------------------------------------------"

# Run the sanity check - capture exit code
echo -e "${BLUE}[RUN]${NC} Executing sanity check..."
"$SANITY_CHECK_BIN" \
    --host "$HOST" \
    --port "$PORT" > "$RESULTS_DIR/results/sanity_check.txt" 2>&1
SANITY_EXIT=$?

# The sanity check outputs readable format by default
# Copy to summary file
cp "$RESULTS_DIR/results/sanity_check.txt" "$RESULTS_DIR/results/sanity_check_output.txt"

# Check if system is ready based on exit code
if [ $SANITY_EXIT -eq 0 ]; then
    echo ""
    echo "Sanity Check Results:"
    echo -e "  ${GREEN}System is ready to start!${NC}"
    PASSED_CHECKS=$((PASSED_CHECKS + 1))
else
    echo ""
    echo "Sanity Check Results:"
    echo -e "  ${RED}System has critical failures!${NC}"
    FAILED_CHECKS=$((FAILED_CHECKS + 1))
fi
TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

# Parse summary from output
SUMMARY_LINE=$(grep "SUMMARY:" "$RESULTS_DIR/results/sanity_check.txt" 2>/dev/null || echo "")
if [ -n "$SUMMARY_LINE" ]; then
    echo "  $SUMMARY_LINE"
fi

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 3: Endpoint Validation"
echo "----------------------------------------------------------------------"

# Test health endpoint
run_check "Health endpoint" "curl -s 'http://$HOST:$PORT/health' | grep -q 'status'"

# Test models endpoint
run_check "Models endpoint" "curl -s 'http://$HOST:$PORT/v1/models' | grep -q 'data'"

# Test providers endpoint
run_check "Providers endpoint" "curl -s 'http://$HOST:$PORT/v1/providers' | head -1"

echo ""
echo "----------------------------------------------------------------------"
echo "Phase 4: CLI Agent Configuration Validation"
echo "----------------------------------------------------------------------"

# Generate all CLI configs
CLI_CONFIG_DIR="$RESULTS_DIR/cli_configs"
mkdir -p "$CLI_CONFIG_DIR"

UNIFIED_GEN="$PROJECT_ROOT/challenges/codebase/go_files/unified_cli_generator/unified_cli_generator"
if [ -f "$UNIFIED_GEN" ]; then
    echo -e "${BLUE}[RUN]${NC} Generating CLI agent configurations..."
    "$UNIFIED_GEN" --host "$HOST" --port "$PORT" --output-dir "$CLI_CONFIG_DIR" > "$RESULTS_DIR/results/cli_generator.log" 2>&1

    for agent in opencode crush kilocode helixcode; do
        config_file="$CLI_CONFIG_DIR/${agent}-helixagent.json"
        if [ -f "$config_file" ]; then
            echo -e "  ${GREEN}[PASS]${NC} Generated $agent configuration"
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
        else
            echo -e "  ${RED}[FAIL]${NC} Failed to generate $agent configuration"
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
        fi
        TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    done
else
    echo -e "${YELLOW}[WARN]${NC} Unified CLI generator not built. Building..."
    cd "$PROJECT_ROOT/challenges/codebase/go_files/unified_cli_generator"
    go build -o unified_cli_generator . 2>/dev/null || true
    if [ -f "unified_cli_generator" ]; then
        ./unified_cli_generator --host "$HOST" --port "$PORT" --output-dir "$CLI_CONFIG_DIR" > "$RESULTS_DIR/results/cli_generator.log" 2>&1 || true
    fi
fi

echo ""
echo "----------------------------------------------------------------------"
echo "Challenge Summary"
echo "----------------------------------------------------------------------"

PASS_RATE=$((PASSED_CHECKS * 100 / TOTAL_CHECKS))

echo ""
echo "Total Checks: $TOTAL_CHECKS"
echo "Passed: $PASSED_CHECKS"
echo "Failed: $FAILED_CHECKS"
echo "Pass Rate: $PASS_RATE%"
echo ""

# Write summary
cat > "$RESULTS_DIR/results/summary.json" << EOF
{
    "challenge": "sanity_check",
    "timestamp": "$(date -Iseconds)",
    "host": "$HOST",
    "port": $PORT,
    "total_checks": $TOTAL_CHECKS,
    "passed_checks": $PASSED_CHECKS,
    "failed_checks": $FAILED_CHECKS,
    "pass_rate": $PASS_RATE,
    "results_dir": "$RESULTS_DIR"
}
EOF

echo "Results saved to: $RESULTS_DIR"
echo ""

if [ $FAILED_CHECKS -gt 0 ]; then
    echo -e "${RED}======================================================================"
    echo "                    CHALLENGE FAILED"
    echo -e "======================================================================${NC}"
    exit 1
else
    echo -e "${GREEN}======================================================================"
    echo "                    CHALLENGE PASSED"
    echo -e "======================================================================${NC}"
    exit 0
fi
