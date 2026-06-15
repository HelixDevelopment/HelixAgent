#!/bin/bash
# Comprehensive Challenge Execution Script
# Runs all 1,038 challenges and logs results

set -e

PROJECT_ROOT="/run/media/milosvasic/DATA4TB/Projects/HelixAgent"
CHALLENGES_DIR="$PROJECT_ROOT/challenges/scripts"
LOG_DIR="$PROJECT_ROOT/execution_logs/$(date +%Y%m%d_%H%M%S)"
RESULTS_FILE="$LOG_DIR/challenge_results.csv"
SUMMARY_FILE="$LOG_DIR/challenge_summary.md"

mkdir -p "$LOG_DIR"

# Initialize results file
echo "challenge_name,status,duration_seconds,passed_tests,failed_tests,exit_code" > "$RESULTS_FILE"

# Count total challenges
TOTAL=$(ls -1 "$CHALLENGES_DIR"/*.sh 2>/dev/null | wc -l)
CURRENT=0
PASSED=0
FAILED=0
SKIPPED=0

echo "========================================"
echo "Starting Challenge Execution"
echo "Total Challenges: $TOTAL"
echo "Log Directory: $LOG_DIR"
echo "Start Time: $(date)"
echo "========================================"

# Run each challenge
for challenge in "$CHALLENGES_DIR"/*.sh; do
    CURRENT=$((CURRENT + 1))
    BASENAME=$(basename "$challenge")
    
    echo "[$CURRENT/$TOTAL] Running: $BASENAME"
    
    START_TIME=$(date +%s)
    
    # Run challenge with resource limits
    if nice -n 19 bash "$challenge" > "$LOG_DIR/${BASENAME%.sh}.log" 2>&1; then
        END_TIME=$(date +%s)
        DURATION=$((END_TIME - START_TIME))
        PASSED=$((PASSED + 1))
        
        # Parse results from log
        PASSED_TESTS=$(sed -nE 's/(^|.*[^0-9])([0-9]+) tests passed.*/\2/p; s/(^|.*[^0-9])([0-9]+)\/[0-9]+ tests.*/\2/p' "$LOG_DIR/${BASENAME%.sh}.log" | tail -1); PASSED_TESTS=${PASSED_TESTS:-0}
        FAILED_TESTS=$(sed -nE 's/.*Failed: ([0-9]+).*/\1/p' "$LOG_DIR/${BASENAME%.sh}.log" | tail -1); FAILED_TESTS=${FAILED_TESTS:-0}

        echo "$BASENAME,PASS,$DURATION,$PASSED_TESTS,$FAILED_TESTS,0" >> "$RESULTS_FILE"
        echo "  ✅ PASSED (${DURATION}s)"
    else
        END_TIME=$(date +%s)
        DURATION=$((END_TIME - START_TIME))
        FAILED=$((FAILED + 1))
        
        PASSED_TESTS=$(sed -nE 's/(^|.*[^0-9])([0-9]+) tests passed.*/\2/p; s/(^|.*[^0-9])([0-9]+)\/[0-9]+ tests.*/\2/p' "$LOG_DIR/${BASENAME%.sh}.log" | tail -1); PASSED_TESTS=${PASSED_TESTS:-0}
        FAILED_TESTS=$(sed -nE 's/.*Failed: ([0-9]+).*/\1/p' "$LOG_DIR/${BASENAME%.sh}.log" | tail -1); FAILED_TESTS=${FAILED_TESTS:-1}

        echo "$BASENAME,FAIL,$DURATION,$PASSED_TESTS,$FAILED_TESTS,1" >> "$RESULTS_FILE"
        echo "  ❌ FAILED (${DURATION}s)"
    fi
done

# Generate summary
cat > "$SUMMARY_FILE" << EOF
# Challenge Execution Summary

**Execution Date:** $(date)
**Total Challenges:** $TOTAL
**Passed:** $PASSED
**Failed:** $FAILED
**Success Rate:** $(( (PASSED * 100) / TOTAL ))%

## Failed Challenges
EOF

grep ",FAIL," "$RESULTS_FILE" >> "$SUMMARY_FILE" || echo "None" >> "$SUMMARY_FILE"

echo ""
echo "========================================"
echo "Challenge Execution Complete"
echo "Passed: $PASSED/$TOTAL"
echo "Failed: $FAILED/$TOTAL"
echo "Results: $RESULTS_FILE"
echo "Summary: $SUMMARY_FILE"
echo "========================================"
