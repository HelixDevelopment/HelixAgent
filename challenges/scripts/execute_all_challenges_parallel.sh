#!/bin/bash
# Execute ALL 1,038 HelixAgent challenges in parallel batches
# Creates comprehensive CSV report with results

set -o pipefail

SCRIPT_DIR="/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/scripts"
RESULTS_DIR="/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/results"
CSV_FILE="$RESULTS_DIR/all_challenges_report.csv"
LOG_DIR="$RESULTS_DIR/logs"
TEMP_DIR="$RESULTS_DIR/temp"

mkdir -p "$RESULTS_DIR"
mkdir -p "$LOG_DIR"
mkdir -p "$TEMP_DIR"

# Initialize CSV header
echo "challenge_name,batch,status,tests_passed,tests_failed,error_message,duration_seconds" > "$CSV_FILE"

# Export for parallel execution
export SCRIPT_DIR LOG_DIR TEMP_DIR CSV_FILE

# Function to run a single challenge
run_single_challenge() {
    local challenge_path="$1"
    local batch="$2"
    local challenge_name=$(basename "$challenge_path" .sh)
    local log_file="$LOG_DIR/${challenge_name}.log"
    local temp_file="$TEMP_DIR/${challenge_name}.csv"
    
    local start_time=$(date +%s)
    
    # Run challenge with nice and capture all output
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
    
    # Extract error message
    if [ "$status" != "PASS" ]; then
        error_message=$(grep -E "ERROR|FAIL|failed" "$log_file" | head -1 | tr ',' ';' | cut -c1-200)
        [ -z "$error_message" ] && error_message="Exit code: $exit_code"
    fi
    
    tests_passed=${tests_passed:-0}
    tests_failed=${tests_failed:-0}
    error_message=$(echo "$error_message" | sed 's/"/""/g')
    
    # Write temp CSV row
    echo "$challenge_name,$batch,$status,$tests_passed,$tests_failed,\"$error_message\",$duration" > "$temp_file"
    
    echo "[$batch] $challenge_name: $status (${duration}s)"
}

export -f run_single_challenge

echo "=========================================="
echo "HelixAgent - Parallel Challenge Execution"
echo "Started: $(date)"
echo "=========================================="
echo ""

# Get all challenges
ALL_CHALLENGES=$(find "$SCRIPT_DIR" -maxdepth 1 -name "*.sh" -type f | sort)

# Create batch lists
echo "Categorizing challenges..."
declare -a BATCH1 BATCH2 BATCH3 BATCH4 BATCH5 BATCH6 BATCH7 BATCH8

while IFS= read -r challenge; do
    name=$(basename "$challenge")
    if [[ "$name" == debate_* ]]; then
        BATCH1+=("$challenge")
    elif [[ "$name" == security_* ]]; then
        BATCH2+=("$challenge")
    elif [[ "$name" == provider_* ]]; then
        BATCH3+=("$challenge")
    elif [[ "$name" == integration_* ]]; then
        BATCH4+=("$challenge")
    elif [[ "$name" == advanced_* ]]; then
        BATCH5+=("$challenge")
    elif [[ "$name" == helixmemory_* ]]; then
        BATCH6+=("$challenge")
    elif [[ "$name" == helixspecifier_* ]]; then
        BATCH7+=("$challenge")
    else
        BATCH8+=("$challenge")
    fi
done <<< "$ALL_CHALLENGES"

echo "Batch 1 (debate): ${#BATCH1[@]} challenges"
echo "Batch 2 (security): ${#BATCH2[@]} challenges"
echo "Batch 3 (provider): ${#BATCH3[@]} challenges"
echo "Batch 4 (integration): ${#BATCH4[@]} challenges"
echo "Batch 5 (advanced): ${#BATCH5[@]} challenges"
echo "Batch 6 (helixmemory): ${#BATCH6[@]} challenges"
echo "Batch 7 (helixspecifier): ${#BATCH7[@]} challenges"
echo "Batch 8 (remaining): ${#BATCH8[@]} challenges"
echo ""

# Execute each batch with parallel processing (limit 4 concurrent)
execute_batch() {
    local -n batch=$1
    local batch_name=$2
    local parallel_jobs=${3:-4}
    
    echo "=========================================="
    echo "Executing $batch_name (${#batch[@]} challenges)"
    echo "=========================================="
    
    # Run challenges in parallel
    for challenge in "${batch[@]}"; do
        run_single_challenge "$challenge" "$batch_name"
    done
    
    echo "$batch_name complete"
    echo ""
}

# Run all batches
execute_batch BATCH1 "1_debate" 8
execute_batch BATCH2 "2_security" 4
execute_batch BATCH3 "3_provider" 4
execute_batch BATCH4 "4_integration" 4
execute_batch BATCH5 "5_advanced" 8
execute_batch BATCH6 "6_helixmemory" 2
execute_batch BATCH7 "7_helixspecifier" 2
execute_batch BATCH8 "8_remaining" 4

# Consolidate all temp files into final CSV
echo "Consolidating results..."
for temp in "$TEMP_DIR"/*.csv; do
    if [ -f "$temp" ]; then
        cat "$temp" >> "$CSV_FILE"
    fi
done

# Generate summary statistics
echo ""
echo "=========================================="
echo "GENERATING SUMMARY"
echo "=========================================="

TOTAL=$(tail -n +2 "$CSV_FILE" | wc -l)
PASSED=$(grep -c ",PASS," "$CSV_FILE" 2>/dev/null | head -1 || echo 0)
FAILED=$(grep -c ",FAIL," "$CSV_FILE" 2>/dev/null | head -1 || echo 0)
ERRORS=$(grep -c ",ERROR," "$CSV_FILE" 2>/dev/null | head -1 || echo 0)

# Calculate pass rate
if [ "$TOTAL" -gt 0 ]; then
    PASS_RATE=$(echo "scale=2; $PASSED * 100 / $TOTAL" | bc 2>/dev/null || echo "N/A")
else
    PASS_RATE="N/A"
fi

echo ""
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

# List failed challenges
if [ "$FAILED" -gt 0 ]; then
    echo "FAILED CHALLENGES:"
    grep ",FAIL," "$CSV_FILE" | cut -d',' -f1 | while read -r c; do
        echo "  - $c"
    done
    echo ""
fi

if [ "$ERRORS" -gt 0 ]; then
    echo "CHALLENGES WITH ERRORS:"
    grep ",ERROR," "$CSV_FILE" | cut -d',' -f1 | while read -r c; do
        echo "  - $c"
    done
fi

# Generate batch statistics
echo ""
echo "BATCH BREAKDOWN:"
for batch in 1_debate 2_security 3_provider 4_integration 5_advanced 6_helixmemory 7_helixspecifier 8_remaining; do
    batch_total=$(grep -c "$batch" "$CSV_FILE" || echo 0)
    batch_passed=$(grep "$batch" "$CSV_FILE" | grep -c ",PASS," || echo 0)
    if [ "$batch_total" -gt 0 ]; then
        batch_rate=$(echo "scale=1; $batch_passed * 100 / $batch_total" | bc 2>/dev/null || echo "0")
        echo "  $batch: $batch_passed/$batch_total ($batch_rate%)"
    fi
done

echo ""
echo "Completed: $(date)"

# Cleanup temp files
rm -rf "$TEMP_DIR"
