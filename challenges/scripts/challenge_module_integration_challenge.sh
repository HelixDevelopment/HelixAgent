#!/bin/bash
# Challenge Module Integration Challenge
# Validates that the Challenges module is properly integrated with HelixAgent,
# including stuck detection, orchestrator, CLI flags, and the pkg/httptest client.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PASS=0
FAIL=0
TOTAL=0

pass() {
    PASS=$((PASS + 1))
    TOTAL=$((TOTAL + 1))
    echo "  [PASS] $1"
}

fail() {
    FAIL=$((FAIL + 1))
    TOTAL=$((TOTAL + 1))
    echo "  [FAIL] $1"
}

echo "=========================================="
echo "Challenge Module Integration Challenge"
echo "=========================================="
echo ""

# ------------------------------------------------------------------
# Part 1: Challenges Module — pkg/stuckdetect
# ------------------------------------------------------------------
echo "--- Part 1: pkg/stuckdetect package ---"

if [ -f "$PROJECT_ROOT/challenges/pkg/stuckdetect/detector.go" ]; then
    pass "stuckdetect/detector.go exists"
else
    fail "stuckdetect/detector.go missing"
fi

if [ -f "$PROJECT_ROOT/challenges/pkg/stuckdetect/writer.go" ]; then
    pass "stuckdetect/writer.go exists"
else
    fail "stuckdetect/writer.go missing"
fi

if [ -f "$PROJECT_ROOT/challenges/pkg/stuckdetect/options.go" ]; then
    pass "stuckdetect/options.go exists"
else
    fail "stuckdetect/options.go missing"
fi

if [ -f "$PROJECT_ROOT/challenges/pkg/stuckdetect/detector_test.go" ]; then
    pass "stuckdetect/detector_test.go exists"
else
    fail "stuckdetect/detector_test.go missing"
fi

if [ -f "$PROJECT_ROOT/challenges/pkg/stuckdetect/writer_test.go" ]; then
    pass "stuckdetect/writer_test.go exists"
else
    fail "stuckdetect/writer_test.go missing"
fi

# Verify key types exist
if grep -q "type StuckDetector struct" "$PROJECT_ROOT/challenges/pkg/stuckdetect/detector.go"; then
    pass "StuckDetector type defined"
else
    fail "StuckDetector type not found"
fi

if grep -q "type ActivityWriter struct" "$PROJECT_ROOT/challenges/pkg/stuckdetect/writer.go"; then
    pass "ActivityWriter type defined"
else
    fail "ActivityWriter type not found"
fi

if grep -q "type Config struct" "$PROJECT_ROOT/challenges/pkg/stuckdetect/options.go"; then
    pass "Config type defined in options.go"
else
    fail "Config type not found in options.go"
fi

if grep -q "ReasonOutputStalled" "$PROJECT_ROOT/challenges/pkg/stuckdetect/detector.go"; then
    pass "ReasonOutputStalled constant defined"
else
    fail "ReasonOutputStalled not found"
fi

if grep -q "ReasonHeartbeatMissed" "$PROJECT_ROOT/challenges/pkg/stuckdetect/detector.go"; then
    pass "ReasonHeartbeatMissed constant defined"
else
    fail "ReasonHeartbeatMissed not found"
fi

if grep -q "ReasonMaxDuration" "$PROJECT_ROOT/challenges/pkg/stuckdetect/detector.go"; then
    pass "ReasonMaxDuration constant defined"
else
    fail "ReasonMaxDuration not found"
fi

echo ""

# ------------------------------------------------------------------
# Part 2: Challenges Module — Integration into existing packages
# ------------------------------------------------------------------
echo "--- Part 2: Stuck detection integration ---"

if grep -q "StatusStuck" "$PROJECT_ROOT/challenges/pkg/challenge/result.go"; then
    pass "StatusStuck added to result.go"
else
    fail "StatusStuck not in result.go"
fi

if grep -q "StuckDetectable" "$PROJECT_ROOT/challenges/pkg/challenge/challenge.go"; then
    pass "StuckDetectable interface in challenge.go"
else
    fail "StuckDetectable not in challenge.go"
fi

if grep -q "StuckDetectorHandle" "$PROJECT_ROOT/challenges/pkg/challenge/challenge.go"; then
    pass "StuckDetectorHandle interface in challenge.go"
else
    fail "StuckDetectorHandle not in challenge.go"
fi

if grep -q "stuckDetector" "$PROJECT_ROOT/challenges/pkg/challenge/shell.go"; then
    pass "stuckDetector field in shell.go"
else
    fail "stuckDetector not in shell.go"
fi

if grep -q "SetStuckDetector" "$PROJECT_ROOT/challenges/pkg/challenge/shell.go"; then
    pass "SetStuckDetector method in shell.go"
else
    fail "SetStuckDetector not in shell.go"
fi

if grep -q "activityWriter" "$PROJECT_ROOT/challenges/pkg/challenge/shell.go"; then
    pass "activityWriter in shell.go"
else
    fail "activityWriter not in shell.go"
fi

if grep -q "StallThreshold" "$PROJECT_ROOT/challenges/pkg/challenge/config.go"; then
    pass "StallThreshold field in config.go"
else
    fail "StallThreshold not in config.go"
fi

if grep -q "defaultStallThreshold" "$PROJECT_ROOT/challenges/pkg/runner/runner.go"; then
    pass "defaultStallThreshold in runner.go"
else
    fail "defaultStallThreshold not in runner.go"
fi

if grep -q "WithDefaultStallThreshold" "$PROJECT_ROOT/challenges/pkg/runner/options.go"; then
    pass "WithDefaultStallThreshold option in runner"
else
    fail "WithDefaultStallThreshold not in runner"
fi

if grep -q "EventStuck" "$PROJECT_ROOT/challenges/pkg/monitor/events.go"; then
    pass "EventStuck in monitor/events.go"
else
    fail "EventStuck not in monitor/events.go"
fi

if grep -q "Stuck.*int" "$PROJECT_ROOT/challenges/pkg/monitor/collector.go"; then
    pass "Stuck counter in collector.go"
else
    fail "Stuck counter not in collector.go"
fi

echo ""

# ------------------------------------------------------------------
# Part 3: Challenges Module — pkg/httptest
# ------------------------------------------------------------------
echo "--- Part 3: pkg/httptest package ---"

if [ -f "$PROJECT_ROOT/challenges/pkg/httptest/client.go" ]; then
    pass "httptest/client.go exists"
else
    fail "httptest/client.go missing"
fi

if [ -f "$PROJECT_ROOT/challenges/pkg/httptest/options.go" ]; then
    pass "httptest/options.go exists"
else
    fail "httptest/options.go missing"
fi

if [ -f "$PROJECT_ROOT/challenges/pkg/httptest/client_test.go" ]; then
    pass "httptest/client_test.go exists"
else
    fail "httptest/client_test.go missing"
fi

if grep -q "type Client struct" "$PROJECT_ROOT/challenges/pkg/httptest/client.go"; then
    pass "Client type defined"
else
    fail "Client type not found"
fi

if grep -q "type RequestResult struct" "$PROJECT_ROOT/challenges/pkg/httptest/client.go"; then
    pass "RequestResult type defined"
else
    fail "RequestResult type not found"
fi

echo ""

# ------------------------------------------------------------------
# Part 4: HelixAgent Bridge — internal/challenges/
# ------------------------------------------------------------------
echo "--- Part 4: HelixAgent bridge ---"

if [ -f "$PROJECT_ROOT/internal/challenges/orchestrator.go" ]; then
    pass "orchestrator.go exists"
else
    fail "orchestrator.go missing"
fi

if [ -f "$PROJECT_ROOT/internal/challenges/stall_config.go" ]; then
    pass "stall_config.go exists"
else
    fail "stall_config.go missing"
fi

if [ -f "$PROJECT_ROOT/internal/challenges/env_loader.go" ]; then
    pass "env_loader.go exists"
else
    fail "env_loader.go missing"
fi

if [ -f "$PROJECT_ROOT/internal/challenges/reporter.go" ]; then
    pass "reporter.go exists"
else
    fail "reporter.go missing"
fi

# Test files
for f in orchestrator_test.go stall_config_test.go env_loader_test.go reporter_test.go; do
    if [ -f "$PROJECT_ROOT/internal/challenges/$f" ]; then
        pass "$f exists"
    else
        fail "$f missing"
    fi
done

if grep -q "type Orchestrator struct" "$PROJECT_ROOT/internal/challenges/orchestrator.go"; then
    pass "Orchestrator type defined"
else
    fail "Orchestrator type not found"
fi

if grep -q "CategoryStallThresholds" "$PROJECT_ROOT/internal/challenges/stall_config.go"; then
    pass "CategoryStallThresholds defined"
else
    fail "CategoryStallThresholds not found"
fi

if grep -q "RegisterShellChallengesEnhanced" "$PROJECT_ROOT/internal/challenges/orchestrator.go"; then
    pass "RegisterShellChallengesEnhanced defined"
else
    fail "RegisterShellChallengesEnhanced not found"
fi

echo ""

# ------------------------------------------------------------------
# Part 5: CLI integration
# ------------------------------------------------------------------
echo "--- Part 5: CLI integration ---"

if [ -f "$PROJECT_ROOT/cmd/helixagent/challenges.go" ]; then
    pass "cmd/helixagent/challenges.go exists"
else
    fail "cmd/helixagent/challenges.go missing"
fi

if grep -q "run-challenges" "$PROJECT_ROOT/cmd/helixagent/main.go"; then
    pass "run-challenges flag defined"
else
    fail "run-challenges flag not found"
fi

if grep -q "list-challenges" "$PROJECT_ROOT/cmd/helixagent/main.go"; then
    pass "list-challenges flag defined"
else
    fail "list-challenges flag not found"
fi

if grep -q "challenge-parallel" "$PROJECT_ROOT/cmd/helixagent/main.go"; then
    pass "challenge-parallel flag defined"
else
    fail "challenge-parallel flag not found"
fi

if grep -q "challenge-stall-threshold" "$PROJECT_ROOT/cmd/helixagent/main.go"; then
    pass "challenge-stall-threshold flag defined"
else
    fail "challenge-stall-threshold flag not found"
fi

if grep -q "handleListChallenges" "$PROJECT_ROOT/cmd/helixagent/challenges.go"; then
    pass "handleListChallenges function exists"
else
    fail "handleListChallenges not found"
fi

if grep -q "handleRunChallenges" "$PROJECT_ROOT/cmd/helixagent/challenges.go"; then
    pass "handleRunChallenges function exists"
else
    fail "handleRunChallenges not found"
fi

echo ""

# ------------------------------------------------------------------
# Part 6: Build verification
# ------------------------------------------------------------------
echo "--- Part 6: Build verification ---"

cd "$PROJECT_ROOT"
if GOMAXPROCS=2 go build ./cmd/helixagent/... 2>/dev/null; then
    pass "helixagent binary builds successfully"
else
    fail "helixagent binary build failed"
fi

if GOMAXPROCS=2 go build ./internal/challenges/... 2>/dev/null; then
    pass "internal/challenges builds successfully"
else
    fail "internal/challenges build failed"
fi

if go vet ./internal/challenges/... 2>/dev/null; then
    pass "go vet passes for internal/challenges"
else
    fail "go vet failed for internal/challenges"
fi

if go vet ./cmd/helixagent/... 2>/dev/null; then
    pass "go vet passes for cmd/helixagent"
else
    fail "go vet failed for cmd/helixagent"
fi

echo ""

# ------------------------------------------------------------------
# Part 7: Challenges module tests
# ------------------------------------------------------------------
echo "--- Part 7: Challenges module tests ---"

cd "$PROJECT_ROOT/Challenges"
if GOMAXPROCS=2 go test ./pkg/stuckdetect/... -count=1 -race -timeout 60s > /dev/null 2>&1; then
    pass "pkg/stuckdetect tests pass"
else
    fail "pkg/stuckdetect tests failed"
fi

if GOMAXPROCS=2 go test ./pkg/httptest/... -count=1 -race -timeout 60s > /dev/null 2>&1; then
    pass "pkg/httptest tests pass"
else
    fail "pkg/httptest tests failed"
fi

if GOMAXPROCS=2 go test ./pkg/challenge/... -count=1 -race -timeout 60s > /dev/null 2>&1; then
    pass "pkg/challenge tests pass"
else
    fail "pkg/challenge tests failed"
fi

if GOMAXPROCS=2 go test ./pkg/runner/... -count=1 -race -timeout 60s > /dev/null 2>&1; then
    pass "pkg/runner tests pass"
else
    fail "pkg/runner tests failed"
fi

if GOMAXPROCS=2 go test ./pkg/monitor/... -count=1 -race -timeout 60s > /dev/null 2>&1; then
    pass "pkg/monitor tests pass"
else
    fail "pkg/monitor tests failed"
fi

echo ""

# ------------------------------------------------------------------
# Part 8: HelixAgent bridge tests
# ------------------------------------------------------------------
echo "--- Part 8: HelixAgent bridge tests ---"

cd "$PROJECT_ROOT"
if GOMAXPROCS=2 go test ./internal/challenges/... -count=1 -race -timeout 120s > /dev/null 2>&1; then
    pass "internal/challenges tests pass"
else
    fail "internal/challenges tests failed"
fi

echo ""

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo "=========================================="
echo "SUMMARY"
echo "=========================================="
echo "Total:  $TOTAL"
echo "Passed: $PASS"
echo "Failed: $FAIL"
echo "=========================================="

if [ "$FAIL" -gt 0 ]; then
    echo "CHALLENGE FAILED"
    exit 1
fi

echo "ALL CHECKS PASSED"
exit 0
