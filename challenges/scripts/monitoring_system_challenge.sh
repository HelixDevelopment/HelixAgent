#!/bin/bash
# HelixAgent Challenge - Monitoring System Validation
# Validates all aspects of the comprehensive monitoring system

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MONITORING_DIR="$PROJECT_ROOT/challenges/monitoring"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

RESULTS_DIR="$PROJECT_ROOT/challenges/results/monitoring_system/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo ""
echo "======================================================================"
echo "       HELIXAGENT MONITORING SYSTEM VALIDATION CHALLENGE"
echo "======================================================================"
echo ""
echo -e "${CYAN}This challenge validates the comprehensive monitoring system:${NC}"
echo "  1. Monitoring library initialization"
echo "  2. Log collection from all components"
echo "  3. Resource sampling (CPU, memory, disk, network)"
echo "  4. Memory leak detection"
echo "  5. Warning/error pattern detection"
echo "  6. Issue recording and fix tracking"
echo "  7. Background monitoring"
echo "  8. Report generation (JSON and HTML)"
echo "  9. Concurrent access safety"
echo "  10. Directory structure creation"
echo ""

TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

record_result() {
    local test_name="$1"
    local status="$2"
    local details="$3"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    if [ "$status" == "pass" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "  ${GREEN}[PASS]${NC} $test_name"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "  ${RED}[FAIL]${NC} $test_name"
        echo "$details" >> "$RESULTS_DIR/failures.txt"
    fi
}

#==============================================================================
# Phase 1: File Existence Tests
#==============================================================================
echo "----------------------------------------------------------------------"
echo "Phase 1: File Existence Tests"
echo "----------------------------------------------------------------------"

# Test monitoring_lib.sh exists
echo -e "${BLUE}[RUN]${NC} Testing monitoring_lib.sh exists..."
if [ -f "$MONITORING_DIR/lib/monitoring_lib.sh" ]; then
    record_result "monitoring_lib.sh exists" "pass" ""
else
    record_result "monitoring_lib.sh exists" "fail" "File not found: $MONITORING_DIR/lib/monitoring_lib.sh"
fi

# Test report_generator.sh exists
echo -e "${BLUE}[RUN]${NC} Testing report_generator.sh exists..."
if [ -f "$MONITORING_DIR/lib/report_generator.sh" ]; then
    record_result "report_generator.sh exists" "pass" ""
else
    record_result "report_generator.sh exists" "fail" "File not found"
fi

# Test run_monitored_challenges.sh exists
echo -e "${BLUE}[RUN]${NC} Testing run_monitored_challenges.sh exists..."
if [ -f "$MONITORING_DIR/run_monitored_challenges.sh" ]; then
    record_result "run_monitored_challenges.sh exists" "pass" ""
else
    record_result "run_monitored_challenges.sh exists" "fail" "File not found"
fi

# Test scripts are executable
echo -e "${BLUE}[RUN]${NC} Testing scripts are executable..."
if [ -x "$MONITORING_DIR/lib/monitoring_lib.sh" ] && \
   [ -x "$MONITORING_DIR/lib/report_generator.sh" ] && \
   [ -x "$MONITORING_DIR/run_monitored_challenges.sh" ]; then
    record_result "Scripts are executable" "pass" ""
else
    record_result "Scripts are executable" "fail" "Some scripts are not executable"
fi

#==============================================================================
# Phase 2: Monitoring Initialization Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 2: Monitoring Initialization Tests"
echo "----------------------------------------------------------------------"

# Test monitoring initialization
echo -e "${BLUE}[RUN]${NC} Testing monitoring initialization..."
INIT_OUTPUT=$(bash -c 'source "'"$MONITORING_DIR"'/lib/monitoring_lib.sh" 2>/dev/null && mon_init "test_init" 2>/dev/null && echo "SESSION=$MON_SESSION_ID" && echo "LOG_DIR=$MON_LOG_DIR"') || true
if echo "$INIT_OUTPUT" | grep -q "SESSION=test_init_"; then
    record_result "Monitoring initialization works" "pass" ""
else
    record_result "Monitoring initialization works" "fail" "$INIT_OUTPUT"
fi

# Test directory structure creation
echo -e "${BLUE}[RUN]${NC} Testing directory structure creation..."
DIR_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'dir_test'
[ -d \"\$MON_LOG_DIR/components\" ] && echo 'COMPONENTS_OK'
[ -d \"\$MON_LOG_DIR/resources\" ] && echo 'RESOURCES_OK'
[ -d \"\$MON_LOG_DIR/issues\" ] && echo 'ISSUES_OK'
[ -d \"\$MON_REPORT_DIR\" ] && echo 'REPORTS_OK'
" 2>&1) || true
if echo "$DIR_OUTPUT" | grep -q "COMPONENTS_OK" && \
   echo "$DIR_OUTPUT" | grep -q "RESOURCES_OK" && \
   echo "$DIR_OUTPUT" | grep -q "ISSUES_OK" && \
   echo "$DIR_OUTPUT" | grep -q "REPORTS_OK"; then
    record_result "Directory structure created correctly" "pass" ""
else
    record_result "Directory structure created correctly" "fail" "$DIR_OUTPUT"
fi

# Test baseline capture
echo -e "${BLUE}[RUN]${NC} Testing memory baseline capture..."
BASELINE_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'baseline_test'
if [ -f \"\$MON_LOG_DIR/resources/memory_baseline.json\" ]; then
    cat \"\$MON_LOG_DIR/resources/memory_baseline.json\"
fi
" 2>&1) || true
if echo "$BASELINE_OUTPUT" | grep -q "total_memory_mb"; then
    record_result "Memory baseline captured" "pass" ""
else
    record_result "Memory baseline captured" "fail" "$BASELINE_OUTPUT"
fi

#==============================================================================
# Phase 3: Logging Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 3: Logging Tests"
echo "----------------------------------------------------------------------"

# Test mon_log function
echo -e "${BLUE}[RUN]${NC} Testing mon_log function..."
LOG_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'log_test'
mon_log 'INFO' 'Test info message'
mon_log 'WARNING' 'Test warning message'
mon_log 'ERROR' 'Test error message'
cat \"\$MON_LOG_DIR/master.log\"
echo '---WARNINGS---'
cat \"\$MON_LOG_DIR/issues/warnings.log\"
echo '---ERRORS---'
cat \"\$MON_LOG_DIR/issues/errors.log\"
" 2>&1) || true
if echo "$LOG_OUTPUT" | grep -q "Test info message" && \
   echo "$LOG_OUTPUT" | grep -q "Test warning message" && \
   echo "$LOG_OUTPUT" | grep -q "Test error message"; then
    record_result "mon_log function works correctly" "pass" ""
else
    record_result "mon_log function works correctly" "fail" "$LOG_OUTPUT"
fi

# Test log routing (warnings and errors to separate files)
echo -e "${BLUE}[RUN]${NC} Testing log routing to separate files..."
# Check if warnings and errors are in their respective sections
if echo "$LOG_OUTPUT" | grep -A1 -- "---WARNINGS---" | grep -q "Test warning message" && \
   echo "$LOG_OUTPUT" | grep -A1 -- "---ERRORS---" | grep -q "Test error message"; then
    record_result "Log routing to separate files works" "pass" ""
else
    record_result "Log routing to separate files works" "fail" "Logs not routed correctly"
fi

#==============================================================================
# Phase 4: Resource Monitoring Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 4: Resource Monitoring Tests"
echo "----------------------------------------------------------------------"

# Test resource sampling
echo -e "${BLUE}[RUN]${NC} Testing resource sampling..."
RESOURCE_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'resource_test'
mon_sample_resources
cat \"\$MON_LOG_DIR/resources/samples.jsonl\"
" 2>&1) || true
if echo "$RESOURCE_OUTPUT" | grep -q "resource_sample" && \
   echo "$RESOURCE_OUTPUT" | grep -q "memory"; then
    record_result "Resource sampling works" "pass" ""
else
    record_result "Resource sampling works" "fail" "$RESOURCE_OUTPUT"
fi

# Test background monitoring
echo -e "${BLUE}[RUN]${NC} Testing background monitoring..."
BG_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
export MON_SAMPLE_INTERVAL=1
mon_init 'bg_test'
mon_start_background_monitoring
sleep 3
mon_stop_background_monitoring
SAMPLE_COUNT=\$(wc -l < \"\$MON_LOG_DIR/resources/samples.jsonl\" 2>/dev/null || echo '0')
echo \"SAMPLE_COUNT=\$SAMPLE_COUNT\"
[ \"\$SAMPLE_COUNT\" -ge 2 ] && echo 'BG_MONITORING_OK'
" 2>&1) || true
if echo "$BG_OUTPUT" | grep -q "BG_MONITORING_OK"; then
    record_result "Background monitoring works" "pass" ""
else
    record_result "Background monitoring works" "fail" "$BG_OUTPUT"
fi

#==============================================================================
# Phase 5: Memory Leak Detection Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 5: Memory Leak Detection Tests"
echo "----------------------------------------------------------------------"

# Test memory leak detection
echo -e "${BLUE}[RUN]${NC} Testing memory leak detection..."
MEMORY_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'memory_test'
mon_detect_memory_leaks
if [ -f \"\$MON_LOG_DIR/issues/memory_leaks.json\" ]; then
    cat \"\$MON_LOG_DIR/issues/memory_leaks.json\"
fi
" 2>&1) || true
if echo "$MEMORY_OUTPUT" | grep -q "leaks_detected"; then
    record_result "Memory leak detection works" "pass" ""
else
    record_result "Memory leak detection works" "fail" "$MEMORY_OUTPUT"
fi

#==============================================================================
# Phase 6: Log Analysis Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 6: Log Analysis Tests"
echo "----------------------------------------------------------------------"

# Test error pattern detection
echo -e "${BLUE}[RUN]${NC} Testing error pattern detection..."
ANALYSIS_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh' 2>/dev/null
mon_init 'analysis_test' 2>/dev/null
mkdir -p \"\$MON_LOG_DIR/components\"
echo 'panic: runtime error at line 42' > \"\$MON_LOG_DIR/components/test.log\"
echo 'ERROR: connection refused' >> \"\$MON_LOG_DIR/components/test.log\"
mon_analyze_log_file \"\$MON_LOG_DIR/components/test.log\" 'test' || true
echo \"ERRORS=\$MON_ERRORS_COUNT\"
cat \"\$MON_LOG_DIR/issues/analysis_test.json\" 2>/dev/null || echo 'NO_ANALYSIS_FILE'
" 2>&1) || true
if echo "$ANALYSIS_OUTPUT" | grep -q "errors_count" || echo "$ANALYSIS_OUTPUT" | grep -qE "ERRORS=[12]"; then
    record_result "Error pattern detection works" "pass" ""
else
    record_result "Error pattern detection works" "fail" "$ANALYSIS_OUTPUT"
fi

# Test warning pattern detection
echo -e "${BLUE}[RUN]${NC} Testing warning pattern detection..."
WARNING_ANALYSIS=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'warning_test'
mkdir -p \"\$MON_LOG_DIR/components\"
echo 'WARNING: high latency detected' > \"\$MON_LOG_DIR/components/warn.log\"
echo 'deprecated API call' >> \"\$MON_LOG_DIR/components/warn.log\"
mon_analyze_log_file \"\$MON_LOG_DIR/components/warn.log\" 'warn'
echo \"WARNINGS=\$MON_WARNINGS_COUNT\"
" 2>&1) || true
if echo "$WARNING_ANALYSIS" | grep -qE "WARNINGS=[12]"; then
    record_result "Warning pattern detection works" "pass" ""
else
    record_result "Warning pattern detection works" "fail" "$WARNING_ANALYSIS"
fi

# Test ignore patterns (false positive filtering)
echo -e "${BLUE}[RUN]${NC} Testing false positive filtering..."
IGNORE_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'ignore_test'
mkdir -p \"\$MON_LOG_DIR/components\"
echo 'TestErrorHandling passed' > \"\$MON_LOG_DIR/components/ignore.log\"
echo 'PASS: ok  dev.helix.agent' >> \"\$MON_LOG_DIR/components/ignore.log\"
mon_analyze_log_file \"\$MON_LOG_DIR/components/ignore.log\" 'ignore'
echo \"ERRORS=\$MON_ERRORS_COUNT\"
echo \"WARNINGS=\$MON_WARNINGS_COUNT\"
" 2>&1) || true
if echo "$IGNORE_OUTPUT" | grep -q "ERRORS=0" && echo "$IGNORE_OUTPUT" | grep -q "WARNINGS=0"; then
    record_result "False positive filtering works" "pass" ""
else
    record_result "False positive filtering works" "fail" "$IGNORE_OUTPUT"
fi

#==============================================================================
# Phase 7: Issue Tracking Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 7: Issue Tracking Tests"
echo "----------------------------------------------------------------------"

# Test issue recording
echo -e "${BLUE}[RUN]${NC} Testing issue recording..."
ISSUE_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh' 2>/dev/null
mon_init 'issue_test' 2>/dev/null
mon_record_issue 'ERROR' 'test_component' 'Test issue description' 'Additional details'
ISSUE_COUNT=\$(ls -1 \"\$MON_LOG_DIR/issues/issue_\"*.json 2>/dev/null | wc -l)
echo \"ISSUE_COUNT=\$ISSUE_COUNT\"
if [ \"\$ISSUE_COUNT\" -ge 1 ]; then
    cat \"\$MON_LOG_DIR/issues/issue_\"*.json
fi
" 2>&1) || true
if echo "$ISSUE_OUTPUT" | grep -q "ISSUE_COUNT=1" && echo "$ISSUE_OUTPUT" | grep -q "severity"; then
    record_result "Issue recording works" "pass" ""
else
    record_result "Issue recording works" "fail" "$ISSUE_OUTPUT"
fi

# Test fix recording
echo -e "${BLUE}[RUN]${NC} Testing fix recording..."
FIX_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh' 2>/dev/null
mon_init 'fix_test' 2>/dev/null
mon_record_fix 'issue_001' 'Applied fix for the issue' 'TestFixVerification'
FIX_COUNT=\$(ls -1 \"\$MON_LOG_DIR/issues/fix_\"*.json 2>/dev/null | wc -l)
echo \"FIX_COUNT=\$FIX_COUNT\"
if [ \"\$FIX_COUNT\" -ge 1 ]; then
    cat \"\$MON_LOG_DIR/issues/fix_\"*.json
fi
" 2>&1) || true
if echo "$FIX_OUTPUT" | grep -q "FIX_COUNT=1" && echo "$FIX_OUTPUT" | grep -q "fix_description"; then
    record_result "Fix recording works" "pass" ""
else
    record_result "Fix recording works" "fail" "$FIX_OUTPUT"
fi

#==============================================================================
# Phase 8: Report Generation Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 8: Report Generation Tests"
echo "----------------------------------------------------------------------"

# Test JSON report generation
echo -e "${BLUE}[RUN]${NC} Testing JSON report generation..."
JSON_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
source '$MONITORING_DIR/lib/report_generator.sh'
mon_init 'json_report_test'

# Create mock session summary
cat > \"\$MON_LOG_DIR/session_summary.json\" << EOF
{
    \"session_id\": \"\$MON_SESSION_ID\",
    \"start_time\": \"\$(date -Iseconds)\",
    \"end_time\": \"\$(date -Iseconds)\",
    \"duration_seconds\": 10,
    \"exit_code\": 0,
    \"issues\": {\"total\": 2, \"errors\": 1, \"warnings\": 1, \"fixes_applied\": 0}
}
EOF

generate_json_report \"\$MON_LOG_DIR\" \"\$MON_REPORT_DIR/report.json\"
[ -f \"\$MON_REPORT_DIR/report.json\" ] && echo 'JSON_CREATED' && cat \"\$MON_REPORT_DIR/report.json\"
" 2>&1) || true
if echo "$JSON_OUTPUT" | grep -q "JSON_CREATED" && echo "$JSON_OUTPUT" | grep -q "report_version"; then
    record_result "JSON report generation works" "pass" ""
else
    record_result "JSON report generation works" "fail" "$JSON_OUTPUT"
fi

# Test HTML report generation
echo -e "${BLUE}[RUN]${NC} Testing HTML report generation..."
HTML_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
source '$MONITORING_DIR/lib/report_generator.sh'
mon_init 'html_report_test'

# Create mock session summary
cat > \"\$MON_LOG_DIR/session_summary.json\" << EOF
{
    \"session_id\": \"\$MON_SESSION_ID\",
    \"start_time\": \"\$(date -Iseconds)\",
    \"end_time\": \"\$(date -Iseconds)\",
    \"duration_seconds\": 10,
    \"exit_code\": 0,
    \"issues\": {\"total\": 0, \"errors\": 0, \"warnings\": 0, \"fixes_applied\": 0}
}
EOF

generate_html_report \"\$MON_LOG_DIR\" \"\$MON_REPORT_DIR/report.html\"
[ -f \"\$MON_REPORT_DIR/report.html\" ] && echo 'HTML_CREATED'
grep -q 'HelixAgent Challenge Monitoring Report' \"\$MON_REPORT_DIR/report.html\" && echo 'HTML_TITLE_OK'
grep -q 'Issue Summary' \"\$MON_REPORT_DIR/report.html\" && echo 'HTML_SUMMARY_OK'
" 2>&1) || true
if echo "$HTML_OUTPUT" | grep -q "HTML_CREATED" && \
   echo "$HTML_OUTPUT" | grep -q "HTML_TITLE_OK" && \
   echo "$HTML_OUTPUT" | grep -q "HTML_SUMMARY_OK"; then
    record_result "HTML report generation works" "pass" ""
else
    record_result "HTML report generation works" "fail" "$HTML_OUTPUT"
fi

#==============================================================================
# Phase 9: Finalization Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 9: Finalization Tests"
echo "----------------------------------------------------------------------"

# Test session finalization
echo -e "${BLUE}[RUN]${NC} Testing session finalization..."
FINAL_OUTPUT=$(bash -c "
source '$MONITORING_DIR/lib/monitoring_lib.sh'
mon_init 'finalize_test'
mon_log 'INFO' 'Test message'
mon_sample_resources
mon_finalize 0
[ -f \"\$MON_LOG_DIR/session_summary.json\" ] && echo 'SUMMARY_CREATED'
cat \"\$MON_LOG_DIR/session_summary.json\"
" 2>&1) || true
if echo "$FINAL_OUTPUT" | grep -q "SUMMARY_CREATED" && echo "$FINAL_OUTPUT" | grep -q "duration_seconds"; then
    record_result "Session finalization works" "pass" ""
else
    record_result "Session finalization works" "fail" "$FINAL_OUTPUT"
fi

#==============================================================================
# Phase 10: Go Integration Tests
#==============================================================================
echo ""
echo "----------------------------------------------------------------------"
echo "Phase 10: Go Integration Tests"
echo "----------------------------------------------------------------------"

# Run Go tests for monitoring system
echo -e "${BLUE}[RUN]${NC} Running Go integration tests..."
cd "$PROJECT_ROOT"
if go test -v -run "TestMonitoring" ./tests/integration/... > "$RESULTS_DIR/go_tests.txt" 2>&1; then
    record_result "Go integration tests pass" "pass" ""
else
    record_result "Go integration tests pass" "fail" "$(cat $RESULTS_DIR/go_tests.txt)"
fi

#==============================================================================
# RESULTS
#==============================================================================
echo ""
echo "======================================================================"
echo "                      CHALLENGE RESULTS"
echo "======================================================================"
echo ""
echo -e "Total Tests:  ${BLUE}$TOTAL_TESTS${NC}"
echo -e "Passed:       ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:       ${RED}$FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}============================================${NC}"
    echo -e "${GREEN}  ALL TESTS PASSED - CHALLENGE COMPLETE!   ${NC}"
    echo -e "${GREEN}============================================${NC}"
    echo ""
    echo -e "${CYAN}Monitoring system validation complete:${NC}"
    echo "  - All monitoring library functions work correctly"
    echo "  - Log collection and analysis functional"
    echo "  - Resource monitoring operational"
    echo "  - Memory leak detection working"
    echo "  - Issue tracking and fix recording verified"
    echo "  - Report generation (JSON/HTML) successful"
    echo ""

    cat > "$RESULTS_DIR/CHALLENGE_PASSED.txt" << EOF
Monitoring System Challenge PASSED
===================================

Date: $(date)
Total Tests: $TOTAL_TESTS
Passed: $PASSED_TESTS
Failed: $FAILED_TESTS

All monitoring system components validated successfully.
EOF
    exit 0
else
    echo -e "${RED}============================================${NC}"
    echo -e "${RED}  CHALLENGE FAILED - $FAILED_TESTS TESTS FAILED  ${NC}"
    echo -e "${RED}============================================${NC}"
    echo ""
    echo "See $RESULTS_DIR/failures.txt for details"
    exit 1
fi
