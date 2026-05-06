#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

echo "=== HelixAgent Comprehensive Test Runner ==="
echo "Running all tests, challenges, and QA checks..."
echo ""

# Ensure we have necessary tools
command -v go >/dev/null 2>&1 || { echo "Go not found. Aborting."; exit 1; }
command -v docker >/dev/null 2>&1 || command -v podman >/dev/null 2>&1 || { echo "No container runtime found. Aborting."; exit 1; }

# Source environment if .env exists
if [ -f ".env" ]; then
    set -o allexport
    source ".env"
    set +o allexport
fi

# Function to start infrastructure containers
start_infrastructure() {
    echo "Starting test infrastructure containers..."
    make test-infra-start >/dev/null 2>&1 || {
        echo "Failed to start infrastructure with Docker, trying Podman fallback..."
        make test-infra-direct-start >/dev/null 2>&1 || {
            echo "Warning: Could not start infrastructure containers. Tests may fail."
        }
    }
    # Wait for containers to be healthy
    echo "Waiting for infrastructure to be ready..."
    sleep 10
}

# Function to stop infrastructure containers
stop_infrastructure() {
    echo "Stopping test infrastructure containers..."
    make test-infra-stop >/dev/null 2>&1 || true
}

# Trap to ensure infrastructure is stopped on script exit
cleanup() {
    stop_infrastructure
}
trap cleanup EXIT INT TERM

# Step 1: Start infrastructure
start_infrastructure

# Step 2: Run Go tests with resource limits (30-40% host resources)
echo ""
echo "=== Running Go Tests (with resource limits) ==="
# Get all packages excluding vendor.bak
PACKAGES=$(go list ./... | grep -v vendor.bak)
if [ -z "$PACKAGES" ]; then
    echo "No packages found to test"
    TEST_EXIT_CODE=0
else
    GOMAXPROCS=2 nice -n 19 ionice -c 3 go test $PACKAGES -p 1 -v -race -count=1 2>&1 | tee "${PROJECT_ROOT}/reports/go_tests.log"
    TEST_EXIT_CODE=${PIPESTATUS[0]}
    if [ ${TEST_EXIT_CODE} -eq 0 ]; then
        echo "✓ Go tests passed"
    else
        echo "✗ Go tests failed with exit code ${TEST_EXIT_CODE}"
        # Continue to challenges, but record failure
    fi
fi

# Step 3: Run all challenge scripts
echo ""
echo "=== Running Challenge Scripts ==="
if [ -f "./challenges/scripts/run_all_challenges.sh" ]; then
    ./challenges/scripts/run_all_challenges.sh 2>&1 | tee "${PROJECT_ROOT}/reports/challenges.log"
    CHALLENGE_EXIT_CODE=${PIPESTATUS[0]}
    if [ ${CHALLENGE_EXIT_CODE} -eq 0 ]; then
        echo "✓ All challenges passed"
    else
        echo "✗ Challenges failed with exit code ${CHALLENGE_EXIT_CODE}"
    fi
else
    echo "Warning: ./challenges/scripts/run_all_challenges.sh not found"
    CHALLENGE_EXIT_CODE=0
fi

# Step 4: Run HelixQA tests if banks exist
echo ""
echo "=== Running HelixQA Tests ==="
if [ -d "../HelixQA" ] && [ -f "../HelixQA/banks/all-formats.yaml" ]; then
    echo "HelixQA module found. Running QA tests..."
    # Build helixqa binary if needed
    if [ ! -f "../HelixQA/bin/helixqa" ]; then
        echo "Building HelixQA binary..."
        (cd "../HelixQA" && go build -o ./bin/helixqa ./cmd/helixqa)
    fi
    # Create output directory
    mkdir -p "${PROJECT_ROOT}/reports/helixqa"
    # Run HelixQA with default banks
    ../HelixQA/bin/helixqa --banks ../HelixQA/banks/all-formats.yaml --output-dir "${PROJECT_ROOT}/reports/helixqa" --report-format markdown 2>&1 | tee "${PROJECT_ROOT}/reports/helixqa.log"
    HELIXQA_EXIT_CODE=${PIPESTATUS[0]}
    if [ ${HELIXQA_EXIT_CODE} -eq 0 ]; then
        echo "✓ HelixQA tests passed"
    else
        echo "✗ HelixQA tests failed with exit code ${HELIXQA_EXIT_CODE}"
    fi
else
    echo "HelixQA test banks not found, skipping."
    HELIXQA_EXIT_CODE=0
fi

# Step 5: Generate summary report
echo ""
echo "=== Test Summary ==="
echo "Go Tests:        $([ ${TEST_EXIT_CODE} -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "Challenges:      $([ ${CHALLENGE_EXIT_CODE} -eq 0 ] && echo 'PASS' || echo 'FAIL')"
echo "HelixQA:         $([ ${HELIXQA_EXIT_CODE} -eq 0 ] && echo 'PASS' || echo 'FAIL')"

# Determine overall success
OVERALL_EXIT_CODE=0
if [ ${TEST_EXIT_CODE} -ne 0 ]; then
    OVERALL_EXIT_CODE=1
fi
if [ ${CHALLENGE_EXIT_CODE} -ne 0 ]; then
    OVERALL_EXIT_CODE=1
fi
if [ ${HELIXQA_EXIT_CODE} -ne 0 ]; then
    OVERALL_EXIT_CODE=1
fi

if [ ${OVERALL_EXIT_CODE} -eq 0 ]; then
    echo ""
    echo "✅ All tests passed!"
else
    echo ""
    echo "❌ Some tests failed. Check logs in ${PROJECT_ROOT}/reports/"
fi

exit ${OVERALL_EXIT_CODE}