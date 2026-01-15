#!/bin/bash
# flink_integration_challenge.sh - Apache Flink Integration Challenge
# Tests Flink cluster setup and job management for HelixAgent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Apache Flink Integration Challenge"
PASSED=0
FAILED=0
TOTAL=0

# Configuration
FLINK_HOST="${FLINK_JOBMANAGER_HOST:-localhost}"
FLINK_PORT="${FLINK_UI_PORT:-8082}"
FLINK_URL="http://${FLINK_HOST}:${FLINK_PORT}"

log_test() {
    local test_name="$1"
    local status="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$status" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "  \e[32m✓\e[0m $test_name"
    else
        FAILED=$((FAILED + 1))
        echo -e "  \e[31m✗\e[0m $test_name"
    fi
}

echo "=============================================="
echo "  $CHALLENGE_NAME"
echo "=============================================="
echo ""

cd "$PROJECT_ROOT"

# ===========================================
# Test 1: Cluster Configuration (5 tests)
# ===========================================
echo "[1] Cluster Configuration"

# Check if Flink JobManager is running
if curl -sf "${FLINK_URL}/overview" > /dev/null 2>&1; then
    log_test "JobManager accessible" "PASS"
else
    log_test "JobManager accessible" "FAIL"
fi

# Check TaskManager slots
TASKMANAGERS=$(curl -sf "${FLINK_URL}/taskmanagers" 2>/dev/null | grep -o '"id"' | wc -l || echo 0)
if [ "$TASKMANAGERS" -ge 1 ]; then
    log_test "TaskManager(s) running ($TASKMANAGERS)" "PASS"
else
    log_test "TaskManager(s) running" "FAIL"
fi

# Check Flink UI
if curl -sf "${FLINK_URL}/config" | grep -q "taskmanager" 2>/dev/null; then
    log_test "Flink UI responding" "PASS"
else
    log_test "Flink UI responding" "FAIL"
fi

# Check state backend configuration
if curl -sf "${FLINK_URL}/config" | grep -q "state.backend" 2>/dev/null; then
    log_test "State backend configured" "PASS"
else
    log_test "State backend configured" "FAIL"
fi

# Check checkpoint storage
if curl -sf "${FLINK_URL}/config" | grep -q "checkpoints" 2>/dev/null; then
    log_test "Checkpoint storage configured" "PASS"
else
    log_test "Checkpoint storage configured" "FAIL"
fi

# ===========================================
# Test 2: REST API Operations (5 tests)
# ===========================================
echo ""
echo "[2] REST API Operations"

# Get cluster overview
OVERVIEW=$(curl -sf "${FLINK_URL}/overview" 2>/dev/null)
if [ -n "$OVERVIEW" ]; then
    log_test "GET /overview" "PASS"
else
    log_test "GET /overview" "FAIL"
fi

# Get task managers list
TASKMANAGER_LIST=$(curl -sf "${FLINK_URL}/taskmanagers" 2>/dev/null)
if echo "$TASKMANAGER_LIST" | grep -q "taskmanagers" 2>/dev/null; then
    log_test "GET /taskmanagers" "PASS"
else
    log_test "GET /taskmanagers" "FAIL"
fi

# Get cluster configuration
CONFIG=$(curl -sf "${FLINK_URL}/config" 2>/dev/null)
if echo "$CONFIG" | grep -q "jobmanager" 2>/dev/null; then
    log_test "GET /config" "PASS"
else
    log_test "GET /config" "FAIL"
fi

# Get jobs list
JOBS=$(curl -sf "${FLINK_URL}/jobs" 2>/dev/null)
if echo "$JOBS" | grep -q "jobs" 2>/dev/null; then
    log_test "GET /jobs" "PASS"
else
    log_test "GET /jobs" "FAIL"
fi

# Get job manager metrics
METRICS=$(curl -sf "${FLINK_URL}/jobmanager/metrics" 2>/dev/null)
if [ -n "$METRICS" ]; then
    log_test "GET /jobmanager/metrics" "PASS"
else
    log_test "GET /jobmanager/metrics" "FAIL"
fi

# ===========================================
# Test 3: Docker Compose Configuration (5 tests)
# ===========================================
echo ""
echo "[3] Docker Compose Configuration"

# Check Flink services in docker-compose
if grep -q "flink-jobmanager" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "JobManager service defined" "PASS"
else
    log_test "JobManager service defined" "FAIL"
fi

if grep -q "flink-taskmanager" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "TaskManager service defined" "PASS"
else
    log_test "TaskManager service defined" "FAIL"
fi

if grep -q "FLINK_PROPERTIES" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Flink properties configured" "PASS"
else
    log_test "Flink properties configured" "FAIL"
fi

if grep -q "rocksdb" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "RocksDB state backend" "PASS"
else
    log_test "RocksDB state backend" "FAIL"
fi

if grep -q "healthcheck" docker-compose.bigdata.yml 2>/dev/null && grep -q "flink" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Flink health checks" "PASS"
else
    log_test "Flink health checks" "FAIL"
fi

# ===========================================
# Test 4: Configuration File (5 tests)
# ===========================================
echo ""
echo "[4] Configuration File"

if [ -f "configs/bigdata.yaml" ]; then
    log_test "bigdata.yaml exists" "PASS"
else
    log_test "bigdata.yaml exists" "FAIL"
fi

if grep -q "flink:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Flink section defined" "PASS"
else
    log_test "Flink section defined" "FAIL"
fi

if grep -q "checkpoint:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Checkpoint config defined" "PASS"
else
    log_test "Checkpoint config defined" "FAIL"
fi

if grep -q "jobs:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Jobs section defined" "PASS"
else
    log_test "Jobs section defined" "FAIL"
fi

if grep -q "kafka:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Kafka integration config" "PASS"
else
    log_test "Kafka integration config" "FAIL"
fi

# ===========================================
# Test 5: Flink Client Package (5 tests)
# ===========================================
echo ""
echo "[5] Flink Client Package"

if [ -d "internal/streaming/flink" ] || [ -f "internal/streaming/flink/client.go" ]; then
    log_test "Flink package exists" "PASS"
else
    log_test "Flink package exists" "FAIL"
fi

# Check for expected files (will be created during implementation)
if grep -q "flink" internal/streaming/*.go 2>/dev/null || [ -f "internal/streaming/flink/client.go" ]; then
    log_test "Flink client code exists" "PASS"
else
    log_test "Flink client code exists (pending implementation)" "FAIL"
fi

# Check flink-jobs directory
if [ -d "flink-jobs" ]; then
    log_test "flink-jobs directory exists" "PASS"
else
    log_test "flink-jobs directory exists" "FAIL"
fi

# Check for Flink dependencies in go.mod (if any Go client is used)
if grep -qi "flink" go.mod 2>/dev/null || [ -f "flink-jobs/pom.xml" ] || [ -f "flink-jobs/build.sbt" ]; then
    log_test "Flink dependencies configured" "PASS"
else
    log_test "Flink dependencies configured (pending)" "FAIL"
fi

# Check metrics configuration
if grep -q "prometheus" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Prometheus metrics configured" "PASS"
else
    log_test "Prometheus metrics configured" "FAIL"
fi

echo ""
echo "=============================================="
echo "  Results: $PASSED/$TOTAL tests passed"
echo "=============================================="

if [ $FAILED -gt 0 ]; then
    echo -e "\e[31m$FAILED test(s) failed\e[0m"
    exit 1
else
    echo -e "\e[32mAll tests passed!\e[0m"
    exit 0
fi
