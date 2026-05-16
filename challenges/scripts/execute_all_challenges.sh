#!/bin/bash
# Execute ALL 1,038 HelixAgent challenges systematically
# Creates comprehensive CSV report with results

set -o pipefail

SCRIPT_DIR="/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/scripts"
RESULTS_DIR="/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/results"
CSV_FILE="$RESULTS_DIR/all_challenges_report.csv"
LOG_DIR="$RESULTS_DIR/logs"

mkdir -p "$RESULTS_DIR"
mkdir -p "$LOG_DIR"

# Initialize CSV header
echo "challenge_name,batch,status,tests_passed,tests_failed,error_message,duration_seconds" > "$CSV_FILE"

# Counters
TOTAL=0
PASSED=0
FAILED=0
ERRORS=0

# Arrays for tracking
FAILED_CHALLENGES=()
ERROR_CHALLENGES=()

# Function to run a single challenge and capture results
run_challenge() {
    local challenge_path="$1"
    local batch="$2"
    local challenge_name=$(basename "$challenge_path" .sh)
    local log_file="$LOG_DIR/${challenge_name}.log"
    
    echo "Running: $challenge_name (Batch: $batch)"
    
    local start_time=$(date +%s)
    
    # Run challenge with nice and capture all output
    nice -n 19 bash "$challenge_path" > "$log_file" 2>&1
    local exit_code=$?
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Parse results from log
    local status="ERROR"
    local tests_passed="0"
    local tests_failed="0"
    local error_message=""
    
    if [ $exit_code -eq 0 ]; then
        # Check if it actually passed or just exited 0
        if grep -q "PASS\|SUCCESS\|passed\|All challenges passed" "$log_file" 2>/dev/null; then
            status="PASS"
            PASSED=$((PASSED + 1))
        elif grep -q "FAIL\|ERROR\|failed" "$log_file" 2>/dev/null; then
            status="FAIL"
            FAILED=$((FAILED + 1))
            FAILED_CHALLENGES+=("$challenge_name")
        else
            status="PASS"
            PASSED=$((PASSED + 1))
        fi
    else
        if grep -q "FAIL\|failed" "$log_file" 2>/dev/null; then
            status="FAIL"
            FAILED=$((FAILED + 1))
            FAILED_CHALLENGES+=("$challenge_name")
        else
            status="ERROR"
            ERRORS=$((ERRORS + 1))
            ERROR_CHALLENGES+=("$challenge_name")
        fi
    fi
    
    # Try to extract test counts
    if grep -qE "[0-9]+ passed|[0-9]+ failed" "$log_file" 2>/dev/null; then
        tests_passed=$(grep -oE "[0-9]+ passed" "$log_file" | grep -oE "[0-9]+" | head -1)
        tests_failed=$(grep -oE "[0-9]+ failed" "$log_file" | grep -oE "[0-9]+" | head -1)
    elif grep -qE "Tests passed: [0-9]+" "$log_file" 2>/dev/null; then
        tests_passed=$(grep -oE "Tests passed: [0-9]+" "$log_file" | grep -oE "[0-9]+")
        tests_failed=$(grep -oE "Tests failed: [0-9]+" "$log_file" | grep -oE "[0-9]+")
    fi
    
    # Extract error message if failed
    if [ "$status" != "PASS" ]; then
        error_message=$(grep -E "ERROR|FAIL|failed" "$log_file" | head -1 | tr ',' ';' | cut -c1-200)
        if [ -z "$error_message" ]; then
            error_message="Exit code: $exit_code"
        fi
    fi
    
    # Default values if empty
    tests_passed=${tests_passed:-0}
    tests_failed=${tests_failed:-0}
    
    # Escape error message for CSV
    error_message=$(echo "$error_message" | sed 's/"/""/g')
    
    # Write to CSV
    echo "$challenge_name,$batch,$status,$tests_passed,$tests_failed,\"$error_message\",$duration" >> "$CSV_FILE"
    
    TOTAL=$((TOTAL + 1))
    
    echo "  Status: $status | Duration: ${duration}s"
}

echo "=========================================="
echo "HelixAgent - ALL 1,038 Challenges Execution"
echo "Started: $(date)"
echo "=========================================="
echo ""

# Get all challenges sorted
ALL_CHALLENGES=$(find "$SCRIPT_DIR" -maxdepth 1 -name "*.sh" -type f | sort)

# BATCH 1: debate_* challenges
echo "=========================================="
echo "BATCH 1: Debate Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep '/debate_'); do
    run_challenge "$challenge" "1_debate"
done
echo ""

# BATCH 2: security_* challenges
echo "=========================================="
echo "BATCH 2: Security Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep '/security_'); do
    run_challenge "$challenge" "2_security"
done
echo ""

# BATCH 3: provider_* challenges
echo "=========================================="
echo "BATCH 3: Provider Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep '/provider_'); do
    run_challenge "$challenge" "3_provider"
done
echo ""

# BATCH 4: integration_* challenges
echo "=========================================="
echo "BATCH 4: Integration Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep '/integration_'); do
    run_challenge "$challenge" "4_integration"
done
echo ""

# BATCH 5: advanced_* challenges
echo "=========================================="
echo "BATCH 5: Advanced Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep '/advanced_'); do
    run_challenge "$challenge" "5_advanced"
done
echo ""

# BATCH 6: helixmemory_* challenges
echo "=========================================="
echo "BATCH 6: HelixMemory Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep '/helixmemory_'); do
    run_challenge "$challenge" "6_helixmemory"
done
echo ""

# BATCH 7: helixspecifier_* challenges
echo "=========================================="
echo "BATCH 7: HelixSpecifier Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep '/helixspecifier_'); do
    run_challenge "$challenge" "7_helixspecifier"
done
echo ""

# BATCH 8: Remaining challenges (all others)
echo "=========================================="
echo "BATCH 8: Remaining Challenges"
echo "=========================================="
for challenge in $(echo "$ALL_CHALLENGES" | grep -v '/debate_' | grep -v '/security_' | grep -v '/provider_' | grep -v '/integration_' | grep -v '/advanced_' | grep -v '/helixmemory_' | grep -v '/helixspecifier_'); do
    run_challenge "$challenge" "8_remaining"
done
echo ""

# Calculate statistics
PASS_RATE=$(echo "scale=2; $PASSED * 100 / $TOTAL" | bc 2>/dev/null || echo "N/A")

echo "=========================================="
echo "FINAL SUMMARY"
echo "=========================================="
echo "Total Challenges Executed: $TOTAL"
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo "Errors: $ERRORS"
echo "Pass Rate: $PASS_RATE%"
echo ""
echo "CSV Report: $CSV_FILE"
echo "Log Directory: $LOG_DIR"
echo ""

if [ ${#FAILED_CHALLENGES[@]} -gt 0 ]; then
    echo "FAILED CHALLENGES:"
    for c in "${FAILED_CHALLENGES[@]}"; do
        echo "  - $c"
    done
    echo ""
fi

if [ ${#ERROR_CHALLENGES[@]} -gt 0 ]; then
    echo "CHALLENGES WITH ERRORS:"
    for c in "${ERROR_CHALLENGES[@]}"; do
        echo "  - $c"
    done
fi

echo ""
echo "Completed: $(date)"
