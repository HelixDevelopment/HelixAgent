#!/bin/bash
# Continue execution of remaining challenges

set -o pipefail

SCRIPT_DIR="/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/scripts"
RESULTS_DIR="/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/results"
CSV_FILE="$RESULTS_DIR/all_challenges_report.csv"
LOG_DIR="$RESULTS_DIR/logs"

mkdir -p "$LOG_DIR"

run_challenge() {
    local challenge_path="$1"
    local batch="$2"
    local challenge_name=$(basename "$challenge_path" .sh)
    local log_file="$LOG_DIR/${challenge_name}.log"
    
    # Skip if already executed
    if [ -f "$log_file" ]; then
        return 0
    fi
    
    echo "[$batch] Running: $challenge_name"
    
    local start_time=$(date +%s)
    nice -n 19 bash "$challenge_path" > "$log_file" 2>&1
    local exit_code=$?
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Parse results
    local status="ERROR"
    local tests_passed="0"
    local tests_failed="0"
    local error_message=""
    
    if [ $exit_code -eq 0 ]; then
        if grep -q "PASS\|SUCCESS\|passed\|All challenges passed" "$log_file" 2>/dev/null; then
            status="PASS"
        elif grep -q "FAIL\|ERROR\|failed" "$log_file" 2>/dev/null; then
            status="FAIL"
        else
            status="PASS"
        fi
    else
        if grep -q "FAIL\|failed" "$log_file" 2>/dev/null; then
            status="FAIL"
        else
            status="ERROR"
        fi
    fi
    
    # Extract test counts
    if grep -qE "[0-9]+ passed|[0-9]+ failed" "$log_file" 2>/dev/null; then
        tests_passed=$(grep -oE "[0-9]+ passed" "$log_file" | grep -oE "[0-9]+" | head -1)
        tests_failed=$(grep -oE "[0-9]+ failed" "$log_file" | grep -oE "[0-9]+" | head -1)
    fi
    
    tests_passed=${tests_passed:-0}
    tests_failed=${tests_failed:-0}
    
    # Extract error message
    if [ "$status" != "PASS" ]; then
        error_message=$(grep -E "ERROR|FAIL|failed" "$log_file" | head -1 | tr ',' ';' | cut -c1-200)
        [ -z "$error_message" ] && error_message="Exit code: $exit_code"
    fi
    
    error_message=$(echo "$error_message" | sed 's/"/""/g')
    
    # Append to CSV
    echo "$challenge_name,$batch,$status,$tests_passed,$tests_failed,\"$error_message\",$duration" >> "$CSV_FILE"
    
    echo "  Status: $status (${duration}s)"
}

echo "=========================================="
echo "CONTINUING CHALLENGE EXECUTION"
echo "Started: $(date)"
echo "=========================================="
echo ""

# Get all challenges
ALL_CHALLENGES=$(find "$SCRIPT_DIR" -maxdepth 1 -name "*.sh" -type f | sort)

# Count remaining
TOTAL_ALL=$(echo "$ALL_CHALLENGES" | wc -l)
DONE=$(ls -1 "$LOG_DIR"/*.log 2>/dev/null | wc -l)
REMAINING=$((TOTAL_ALL - DONE))

echo "Total challenges: $TOTAL_ALL"
echo "Already completed: $DONE"
echo "Remaining: $REMAINING"
echo ""

# Process remaining challenges
while IFS= read -r challenge; do
    name=$(basename "$challenge")
    log_file="$LOG_DIR/${name%.sh}.log"
    
    # Skip if already done
    [ -f "$log_file" ] && continue
    
    # Determine batch
    if [[ "$name" == debate_* ]]; then
        batch="1_debate"
    elif [[ "$name" == security_* ]]; then
        batch="2_security"
    elif [[ "$name" == provider_* ]]; then
        batch="3_provider"
    elif [[ "$name" == integration_* ]]; then
        batch="4_integration"
    elif [[ "$name" == advanced_* ]]; then
        batch="5_advanced"
    elif [[ "$name" == helixmemory_* ]]; then
        batch="6_helixmemory"
    elif [[ "$name" == helixspecifier_* ]]; then
        batch="7_helixspecifier"
    else
        batch="8_remaining"
    fi
    
    run_challenge "$challenge" "$batch"
done <<< "$ALL_CHALLENGES"

echo ""
echo "=========================================="
echo "COMPLETE"
echo "=========================================="
echo "Finished: $(date)"
