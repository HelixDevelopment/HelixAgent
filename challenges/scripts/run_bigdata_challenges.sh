#!/bin/bash
# run_bigdata_challenges.sh - Run all Big Data challenges
# Tests complete Big Data integration for HelixAgent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=============================================="
echo "  HelixAgent Big Data Challenges"
echo "=============================================="
echo ""

cd "$PROJECT_ROOT"

# Challenge scripts
CHALLENGES=(
    "minio_storage_challenge.sh"
    "flink_integration_challenge.sh"
    "qdrant_vector_challenge.sh"
    "bigdata_pipeline_challenge.sh"
)

# Track results
TOTAL_CHALLENGES=0
PASSED_CHALLENGES=0
CHALLENGE_RESULTS=()

for challenge in "${CHALLENGES[@]}"; do
    TOTAL_CHALLENGES=$((TOTAL_CHALLENGES + 1))

    echo ""
    echo "========================================"
    echo "Running: $challenge"
    echo "========================================"
    echo ""

    if "$SCRIPT_DIR/$challenge"; then
        PASSED_CHALLENGES=$((PASSED_CHALLENGES + 1))
        CHALLENGE_RESULTS+=("✓ $challenge")
    else
        CHALLENGE_RESULTS+=("✗ $challenge")
    fi
done

echo ""
echo "=============================================="
echo "  Big Data Challenge Summary"
echo "=============================================="
echo ""

for result in "${CHALLENGE_RESULTS[@]}"; do
    if [[ "$result" == ✓* ]]; then
        echo -e "\e[32m$result\e[0m"
    else
        echo -e "\e[31m$result\e[0m"
    fi
done

echo ""
echo "----------------------------------------------"
echo "Total: $PASSED_CHALLENGES/$TOTAL_CHALLENGES challenges passed"
echo "----------------------------------------------"

if [ $PASSED_CHALLENGES -eq $TOTAL_CHALLENGES ]; then
    echo -e "\e[32mAll Big Data challenges passed!\e[0m"
    exit 0
else
    FAILED=$((TOTAL_CHALLENGES - PASSED_CHALLENGES))
    echo -e "\e[31m$FAILED challenge(s) failed\e[0m"
    exit 1
fi
