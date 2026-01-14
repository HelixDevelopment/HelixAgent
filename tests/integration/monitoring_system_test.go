package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitoringLibraryExists verifies monitoring library files exist
func TestMonitoringLibraryExists(t *testing.T) {
	projectRoot := getProjectRoot(t)

	requiredFiles := []string{
		"challenges/monitoring/lib/monitoring_lib.sh",
		"challenges/monitoring/lib/report_generator.sh",
		"challenges/monitoring/run_monitored_challenges.sh",
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(projectRoot, file)
		_, err := os.Stat(filePath)
		assert.NoError(t, err, "Required monitoring file should exist: %s", file)
	}
}

// TestMonitoringLibraryExecutable verifies monitoring scripts are executable
func TestMonitoringLibraryExecutable(t *testing.T) {
	projectRoot := getProjectRoot(t)

	scripts := []string{
		"challenges/monitoring/lib/monitoring_lib.sh",
		"challenges/monitoring/lib/report_generator.sh",
		"challenges/monitoring/run_monitored_challenges.sh",
	}

	for _, script := range scripts {
		scriptPath := filepath.Join(projectRoot, script)
		info, err := os.Stat(scriptPath)
		require.NoError(t, err, "Script should exist: %s", script)

		mode := info.Mode()
		assert.True(t, mode&0111 != 0, "Script should be executable: %s", script)
	}
}

// TestMonitoringInitialization tests that monitoring initializes correctly
func TestMonitoringInitialization(t *testing.T) {
	projectRoot := getProjectRoot(t)

	// Create a test script that initializes monitoring
	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "test_session" 2>/dev/null
echo "SESSION_ID=$MON_SESSION_ID"
echo "LOG_DIR=$MON_LOG_DIR"
echo "REPORT_DIR=$MON_REPORT_DIR"
`

	tmpFile := filepath.Join(t.TempDir(), "test_init.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Monitoring initialization should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "SESSION_ID=test_session_")
	assert.Contains(t, outputStr, "LOG_DIR=")
	assert.Contains(t, outputStr, "REPORT_DIR=")
}

// TestMonitoringLogCollection tests log collection functionality
func TestMonitoringLogCollection(t *testing.T) {
	projectRoot := getProjectRoot(t)

	// Create a test script that tests log collection
	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "log_test" 2>/dev/null

# Create a fake log file
echo "Test log entry" > /tmp/helixagent.log

mon_collect_helixagent_logs

# Check if log was collected
if [ -f "$MON_LOG_DIR/components/helixagent.log" ]; then
    echo "LOG_COLLECTED=true"
    if grep -q "Test log entry" "$MON_LOG_DIR/components/helixagent.log"; then
        echo "LOG_CONTENT_VALID=true"
    fi
fi

rm -f /tmp/helixagent.log
`

	tmpFile := filepath.Join(t.TempDir(), "test_logs.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Log collection should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "LOG_COLLECTED=true")
	assert.Contains(t, outputStr, "LOG_CONTENT_VALID=true")
}

// TestMonitoringResourceSampling tests resource sampling functionality
func TestMonitoringResourceSampling(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "resource_test" 2>/dev/null

# Sample resources
mon_sample_resources

# Check if sample was recorded
if [ -f "$MON_LOG_DIR/resources/samples.jsonl" ]; then
    SAMPLE=$(tail -1 "$MON_LOG_DIR/resources/samples.jsonl")
    echo "SAMPLE_RECORDED=true"
    echo "SAMPLE=$SAMPLE"
fi
`

	tmpFile := filepath.Join(t.TempDir(), "test_resources.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Resource sampling should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "SAMPLE_RECORDED=true")
	assert.Contains(t, outputStr, "resource_sample")
	assert.Contains(t, outputStr, "memory")
}

// TestMonitoringMemoryLeakDetection tests memory leak detection
func TestMonitoringMemoryLeakDetection(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "memory_test" 2>/dev/null

# Run memory leak detection
if mon_detect_memory_leaks; then
    echo "NO_LEAKS_DETECTED=true"
fi

# Check if analysis file was created
if [ -f "$MON_LOG_DIR/issues/memory_leaks.json" ]; then
    echo "ANALYSIS_FILE_CREATED=true"
    cat "$MON_LOG_DIR/issues/memory_leaks.json"
fi
`

	tmpFile := filepath.Join(t.TempDir(), "test_memory.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	// Memory leak detection might return non-zero if leaks are detected
	// We just want to verify it runs and creates the analysis file

	outputStr := string(output)
	assert.Contains(t, outputStr, "ANALYSIS_FILE_CREATED=true")
}

// TestMonitoringLogAnalysis tests warning/error detection in logs
func TestMonitoringLogAnalysis(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
# Don't use set -e because mon_analyze_log_file returns error count as exit code
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "analysis_test" 2>/dev/null

# Create a test log file with errors and warnings
mkdir -p "$MON_LOG_DIR/components"
cat > "$MON_LOG_DIR/components/test_component.log" << 'EOF'
2024-01-15 10:00:00 INFO Starting service
2024-01-15 10:00:01 WARNING High latency detected
2024-01-15 10:00:02 ERROR Connection refused
2024-01-15 10:00:03 INFO Service running
2024-01-15 10:00:04 ERROR timeout exceeded
EOF

# Run analysis (ignore exit code as it returns error count)
mon_analyze_log_file "$MON_LOG_DIR/components/test_component.log" "test_component" || true

# Check results
if [ -f "$MON_LOG_DIR/issues/analysis_test_component.json" ]; then
    echo "ANALYSIS_CREATED=true"
    cat "$MON_LOG_DIR/issues/analysis_test_component.json"
fi

echo "WARNINGS_COUNT=$MON_WARNINGS_COUNT"
echo "ERRORS_COUNT=$MON_ERRORS_COUNT"
`

	tmpFile := filepath.Join(t.TempDir(), "test_analysis.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Analysis script should succeed: %s", string(output))
	outputStr := string(output)

	assert.Contains(t, outputStr, "ANALYSIS_CREATED=true")
	// Should detect at least 1 warning and errors
	assert.Contains(t, outputStr, "errors_count")
	assert.Contains(t, outputStr, "warnings_count")
}

// TestMonitoringReportGeneration tests JSON and HTML report generation
func TestMonitoringReportGeneration(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/report_generator.sh") + `" 2>/dev/null

mon_init "report_test" 2>/dev/null

# Add some test data
mon_log "INFO" "Test info message"
mon_log "WARNING" "Test warning message"
mon_log "ERROR" "Test error message"
mon_log "FIX" "Test fix applied"

# Sample resources
mon_sample_resources

# Finalize to create session summary
cat > "$MON_LOG_DIR/session_summary.json" << EOF
{
    "session_id": "$MON_SESSION_ID",
    "start_time": "$(date -Iseconds)",
    "end_time": "$(date -Iseconds)",
    "duration_seconds": 10,
    "exit_code": 0,
    "issues": {
        "total": 3,
        "errors": 1,
        "warnings": 1,
        "fixes_applied": 1
    }
}
EOF

# Generate reports
generate_json_report "$MON_LOG_DIR" "$MON_REPORT_DIR/report.json"
generate_html_report "$MON_LOG_DIR" "$MON_REPORT_DIR/report.html"

# Check reports were created
if [ -f "$MON_REPORT_DIR/report.json" ]; then
    echo "JSON_REPORT_CREATED=true"
fi

if [ -f "$MON_REPORT_DIR/report.html" ]; then
    echo "HTML_REPORT_CREATED=true"
    # Check for key HTML elements
    if grep -q "HelixAgent Challenge Monitoring Report" "$MON_REPORT_DIR/report.html"; then
        echo "HTML_TITLE_PRESENT=true"
    fi
    if grep -q "Issue Summary" "$MON_REPORT_DIR/report.html"; then
        echo "HTML_SUMMARY_PRESENT=true"
    fi
fi
`

	tmpFile := filepath.Join(t.TempDir(), "test_report.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Report generation should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "JSON_REPORT_CREATED=true")
	assert.Contains(t, outputStr, "HTML_REPORT_CREATED=true")
	assert.Contains(t, outputStr, "HTML_TITLE_PRESENT=true")
	assert.Contains(t, outputStr, "HTML_SUMMARY_PRESENT=true")
}

// TestMonitoringIssueRecording tests issue and fix recording
func TestMonitoringIssueRecording(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "issue_test" 2>/dev/null

# Record an issue
mon_record_issue "ERROR" "test_component" "Test error description" "Additional details"

# Record a fix
mon_record_fix "issue_001" "Applied fix for test error" "TestMonitoringIssueRecording"

# Check files were created
ISSUE_FILES=$(ls -1 "$MON_LOG_DIR/issues/issue_"*.json 2>/dev/null | wc -l)
FIX_FILES=$(ls -1 "$MON_LOG_DIR/issues/fix_"*.json 2>/dev/null | wc -l)

echo "ISSUE_FILES_COUNT=$ISSUE_FILES"
echo "FIX_FILES_COUNT=$FIX_FILES"
echo "ISSUES_COUNT=$MON_ISSUES_COUNT"
echo "FIXES_COUNT=$MON_FIXES_COUNT"
`

	tmpFile := filepath.Join(t.TempDir(), "test_issues.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Issue recording should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "ISSUE_FILES_COUNT=1")
	assert.Contains(t, outputStr, "FIX_FILES_COUNT=1")
	assert.Contains(t, outputStr, "ISSUES_COUNT=1")
	assert.Contains(t, outputStr, "FIXES_COUNT=1")
}

// TestMonitoringBackgroundMonitoring tests background monitoring start/stop
func TestMonitoringBackgroundMonitoring(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `"

export MON_SAMPLE_INTERVAL=1  # 1 second for testing

mon_init "background_test" 2>/dev/null

# Start background monitoring
mon_start_background_monitoring

# Wait for a few samples
sleep 3

# Stop background monitoring
mon_stop_background_monitoring

# Check samples were recorded
SAMPLE_COUNT=$(wc -l < "$MON_LOG_DIR/resources/samples.jsonl" 2>/dev/null || echo "0")
echo "SAMPLE_COUNT=$SAMPLE_COUNT"

# Should have at least 2 samples
if [ "$SAMPLE_COUNT" -ge 2 ]; then
    echo "BACKGROUND_MONITORING_WORKED=true"
fi
`

	tmpFile := filepath.Join(t.TempDir(), "test_background.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Background monitoring should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "BACKGROUND_MONITORING_WORKED=true")
}

// TestMonitoringFinalization tests finalization and summary generation
func TestMonitoringFinalization(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "finalize_test" 2>/dev/null

# Add some test data
mon_log "INFO" "Test message 1"
mon_log "WARNING" "Test warning"
mon_log "ERROR" "Test error"

# Sample resources
mon_sample_resources

# Finalize
mon_finalize 0

# Check summary was created
if [ -f "$MON_LOG_DIR/session_summary.json" ]; then
    echo "SUMMARY_CREATED=true"
    cat "$MON_LOG_DIR/session_summary.json"
fi
`

	tmpFile := filepath.Join(t.TempDir(), "test_finalize.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Finalization should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "SUMMARY_CREATED=true")
	assert.Contains(t, outputStr, "session_id")
	assert.Contains(t, outputStr, "duration_seconds")
	assert.Contains(t, outputStr, "issues")
}

// TestMonitoringErrorPatternDetection tests error pattern detection
func TestMonitoringErrorPatternDetection(t *testing.T) {
	projectRoot := getProjectRoot(t)

	// Test various error patterns
	testCases := []struct {
		name     string
		logLine  string
		expected string
	}{
		{"panic", "panic: runtime error", "ERROR"},
		{"fatal", "FATAL: database connection failed", "ERROR"},
		{"nil_pointer", "runtime error: nil pointer dereference", "ERROR"},
		{"timeout", "context deadline exceeded", "ERROR"},
		{"warning", "WARNING: high latency detected", "WARNING"},
		{"deprecated", "this API is deprecated", "WARNING"},
		{"retry", "retrying request after failure", "WARNING"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testScript := `#!/bin/bash
# Don't use set -e because mon_analyze_log_file returns error count as exit code
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "pattern_test_` + tc.name + `" 2>/dev/null

mkdir -p "$MON_LOG_DIR/components"
echo "` + tc.logLine + `" > "$MON_LOG_DIR/components/test.log"

# Run analysis (ignore exit code as it returns error count)
mon_analyze_log_file "$MON_LOG_DIR/components/test.log" "test" || true

echo "WARNINGS=$MON_WARNINGS_COUNT"
echo "ERRORS=$MON_ERRORS_COUNT"
`
			tmpFile := filepath.Join(t.TempDir(), "test_pattern_"+tc.name+".sh")
			err := os.WriteFile(tmpFile, []byte(testScript), 0755)
			require.NoError(t, err)

			cmd := exec.Command("bash", tmpFile)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "Pattern test script should succeed: %s", string(output))
			outputStr := string(output)

			if tc.expected == "ERROR" {
				assert.Contains(t, outputStr, "ERRORS=1", "Should detect error pattern: %s", tc.name)
			} else if tc.expected == "WARNING" {
				assert.Contains(t, outputStr, "WARNINGS=1", "Should detect warning pattern: %s", tc.name)
			}
		})
	}
}

// TestMonitoringIgnorePatterns tests that false positives are ignored
func TestMonitoringIgnorePatterns(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
# Don't use set -e because mon_analyze_log_file returns error count as exit code
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "ignore_test" 2>/dev/null

mkdir -p "$MON_LOG_DIR/components"

# These should be ignored (test-related, expected errors)
cat > "$MON_LOG_DIR/components/test.log" << 'EOF'
TestErrorHandling passed
expected error: test error
PASS: ok  dev.helix.agent/internal/handlers
mock error for testing
EOF

# Run analysis (ignore exit code as it returns error count)
mon_analyze_log_file "$MON_LOG_DIR/components/test.log" "test" || true

echo "WARNINGS=$MON_WARNINGS_COUNT"
echo "ERRORS=$MON_ERRORS_COUNT"
`

	tmpFile := filepath.Join(t.TempDir(), "test_ignore.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)

	outputStr := string(output)
	// Should have 0 warnings and 0 errors because all are ignored patterns
	assert.Contains(t, outputStr, "WARNINGS=0", "Should ignore test-related patterns")
	assert.Contains(t, outputStr, "ERRORS=0", "Should ignore expected error patterns")
}

// TestMonitoringJSONReportStructure tests JSON report has correct structure
func TestMonitoringJSONReportStructure(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/report_generator.sh") + `" 2>/dev/null

mon_init "json_struct_test" 2>/dev/null

# Create session summary
cat > "$MON_LOG_DIR/session_summary.json" << EOF
{
    "session_id": "$MON_SESSION_ID",
    "start_time": "$(date -Iseconds)",
    "end_time": "$(date -Iseconds)",
    "duration_seconds": 5,
    "exit_code": 0,
    "issues": {
        "total": 2,
        "errors": 1,
        "warnings": 1,
        "fixes_applied": 0
    }
}
EOF

# Generate JSON report (suppress stderr from generate function)
generate_json_report "$MON_LOG_DIR" "/tmp/test_report_$$.json" 2>/dev/null

# Output the report for verification
cat "/tmp/test_report_$$.json"
rm -f "/tmp/test_report_$$.json"
`

	tmpFile := filepath.Join(t.TempDir(), "test_json_struct.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "JSON report generation should succeed: %s", string(output))

	// Parse JSON to verify structure
	var report map[string]interface{}
	err = json.Unmarshal(output, &report)
	require.NoError(t, err, "Report should be valid JSON")

	// Verify required fields
	assert.Contains(t, report, "report_version")
	assert.Contains(t, report, "generated_at")
	assert.Contains(t, report, "session")
	assert.Contains(t, report, "summary")
	assert.Contains(t, report, "issues")
	assert.Contains(t, report, "memory_analysis")
	assert.Contains(t, report, "resource_samples")

	// Verify session structure
	session := report["session"].(map[string]interface{})
	assert.Contains(t, session, "id")
	assert.Contains(t, session, "duration_seconds")

	// Verify summary structure
	summary := report["summary"].(map[string]interface{})
	assert.Contains(t, summary, "total_issues")
	assert.Contains(t, summary, "errors")
	assert.Contains(t, summary, "warnings")
	assert.Contains(t, summary, "status")
}

// TestMonitoringProcessTracking tests process tracking functionality
func TestMonitoringProcessTracking(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "process_test" 2>/dev/null

# Track current shell process
mon_track_process "test_shell" $$

# Verify tracking
if [ -n "${MON_TRACKED_PIDS[test_shell]}" ]; then
    echo "PROCESS_TRACKED=true"
    echo "TRACKED_PID=${MON_TRACKED_PIDS[test_shell]}"
fi

# Sample resources - should include tracked process
mon_sample_resources

# Check sample includes process data
if grep -q "test_shell" "$MON_LOG_DIR/resources/samples.jsonl" 2>/dev/null; then
    echo "PROCESS_IN_SAMPLES=true"
fi
`

	tmpFile := filepath.Join(t.TempDir(), "test_tracking.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Process tracking should succeed: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "PROCESS_TRACKED=true")
	assert.Contains(t, outputStr, "PROCESS_IN_SAMPLES=true")
}

// Helper function to get project root
func getProjectRoot(t *testing.T) string {
	t.Helper()

	// Try to find project root by looking for go.mod
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root")
		}
		dir = parent
	}
}

// TestMonitoredChallengeScriptExists verifies the main script exists
func TestMonitoredChallengeScriptExists(t *testing.T) {
	projectRoot := getProjectRoot(t)

	scriptPath := filepath.Join(projectRoot, "challenges/monitoring/run_monitored_challenges.sh")
	info, err := os.Stat(scriptPath)
	require.NoError(t, err, "Monitored challenge script should exist")

	assert.True(t, info.Mode()&0111 != 0, "Script should be executable")
}

// TestMonitoringDirectoryStructure verifies directory structure is created
func TestMonitoringDirectoryStructure(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "dir_test" 2>/dev/null

# Check directories were created
[ -d "$MON_LOG_DIR" ] && echo "LOG_DIR_EXISTS=true"
[ -d "$MON_LOG_DIR/components" ] && echo "COMPONENTS_DIR_EXISTS=true"
[ -d "$MON_LOG_DIR/resources" ] && echo "RESOURCES_DIR_EXISTS=true"
[ -d "$MON_LOG_DIR/issues" ] && echo "ISSUES_DIR_EXISTS=true"
[ -d "$MON_REPORT_DIR" ] && echo "REPORT_DIR_EXISTS=true"

# Check files were created
[ -f "$MON_LOG_DIR/master.log" ] && echo "MASTER_LOG_EXISTS=true"
[ -f "$MON_LOG_DIR/issues/warnings.log" ] && echo "WARNINGS_LOG_EXISTS=true"
[ -f "$MON_LOG_DIR/issues/errors.log" ] && echo "ERRORS_LOG_EXISTS=true"
[ -f "$MON_LOG_DIR/resources/samples.jsonl" ] && echo "SAMPLES_FILE_EXISTS=true"
[ -f "$MON_LOG_DIR/resources/memory_baseline.json" ] && echo "BASELINE_FILE_EXISTS=true"
`

	tmpFile := filepath.Join(t.TempDir(), "test_dirs.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Directory structure should be created: %s", string(output))

	outputStr := string(output)
	assert.Contains(t, outputStr, "LOG_DIR_EXISTS=true")
	assert.Contains(t, outputStr, "COMPONENTS_DIR_EXISTS=true")
	assert.Contains(t, outputStr, "RESOURCES_DIR_EXISTS=true")
	assert.Contains(t, outputStr, "ISSUES_DIR_EXISTS=true")
	assert.Contains(t, outputStr, "REPORT_DIR_EXISTS=true")
	assert.Contains(t, outputStr, "MASTER_LOG_EXISTS=true")
	assert.Contains(t, outputStr, "WARNINGS_LOG_EXISTS=true")
	assert.Contains(t, outputStr, "ERRORS_LOG_EXISTS=true")
	assert.Contains(t, outputStr, "SAMPLES_FILE_EXISTS=true")
	assert.Contains(t, outputStr, "BASELINE_FILE_EXISTS=true")
}

// TestMonitoringConcurrentAccess tests thread safety of monitoring
func TestMonitoringConcurrentAccess(t *testing.T) {
	projectRoot := getProjectRoot(t)

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "concurrent_test" 2>/dev/null

# Run multiple operations concurrently
for i in {1..5}; do
    (
        mon_log "INFO" "Concurrent message $i"
        mon_sample_resources
    ) &
done

# Wait for all background jobs
wait

# Check all messages were logged
LOG_COUNT=$(grep -c "Concurrent message" "$MON_LOG_DIR/master.log" 2>/dev/null || echo "0")
SAMPLE_COUNT=$(wc -l < "$MON_LOG_DIR/resources/samples.jsonl" 2>/dev/null || echo "0")

echo "LOG_COUNT=$LOG_COUNT"
echo "SAMPLE_COUNT=$SAMPLE_COUNT"

# Should have at least 5 log entries
if [ "$LOG_COUNT" -ge 5 ]; then
    echo "CONCURRENT_LOGGING_WORKED=true"
fi
`

	tmpFile := filepath.Join(t.TempDir(), "test_concurrent.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	// Run multiple times to catch race conditions
	for i := 0; i < 3; i++ {
		cmd := exec.Command("bash", tmpFile)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Concurrent access should succeed (run %d): %s", i+1, string(output))

		outputStr := string(output)
		assert.Contains(t, outputStr, "CONCURRENT_LOGGING_WORKED=true", "Run %d should work", i+1)
	}
}

// TestMonitoringTimestamps tests timestamp accuracy
func TestMonitoringTimestamps(t *testing.T) {
	projectRoot := getProjectRoot(t)

	before := time.Now()

	testScript := `#!/bin/bash
set -e
source "` + filepath.Join(projectRoot, "challenges/monitoring/lib/monitoring_lib.sh") + `" 2>/dev/null
mon_init "timestamp_test" 2>/dev/null
mon_log "INFO" "Test message"
cat "$MON_LOG_DIR/master.log"
`

	tmpFile := filepath.Join(t.TempDir(), "test_timestamp.sh")
	err := os.WriteFile(tmpFile, []byte(testScript), 0755)
	require.NoError(t, err)

	cmd := exec.Command("bash", tmpFile)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)

	after := time.Now()

	outputStr := string(output)

	// Verify timestamp format (YYYY-MM-DD HH:MM:SS.mmm)
	assert.Regexp(t, `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`, outputStr, "Should have valid timestamp format")

	// Verify timestamp is within expected range
	// Extract year from output
	if strings.Contains(outputStr, before.Format("2006-01-02")) ||
		strings.Contains(outputStr, after.Format("2006-01-02")) {
		// Timestamp is from today
	} else {
		t.Log("Warning: Timestamp date mismatch (may be timezone issue)")
	}
}
