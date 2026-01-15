#!/bin/bash
# minio_storage_challenge.sh - MinIO Object Storage Challenge
# Tests MinIO setup and S3 operations for HelixAgent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="MinIO Object Storage Challenge"
PASSED=0
FAILED=0
TOTAL=0

# Configuration
MINIO_HOST="${MINIO_ENDPOINT:-localhost:9000}"
MINIO_URL="http://${MINIO_HOST}"
MINIO_CONSOLE_PORT="${MINIO_CONSOLE_PORT:-9001}"
MINIO_ACCESS_KEY="${MINIO_ROOT_USER:-minioadmin}"
MINIO_SECRET_KEY="${MINIO_ROOT_PASSWORD:-minioadmin123}"

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
# Test 1: MinIO Server Status (4 tests)
# ===========================================
echo "[1] MinIO Server Status"

# Check MinIO health endpoint
if curl -sf "${MINIO_URL}/minio/health/live" > /dev/null 2>&1; then
    log_test "MinIO health check (live)" "PASS"
else
    log_test "MinIO health check (live)" "FAIL"
fi

# Check MinIO ready endpoint
if curl -sf "${MINIO_URL}/minio/health/ready" > /dev/null 2>&1; then
    log_test "MinIO health check (ready)" "PASS"
else
    log_test "MinIO health check (ready)" "FAIL"
fi

# Check MinIO console
if curl -sf "http://localhost:${MINIO_CONSOLE_PORT}" > /dev/null 2>&1; then
    log_test "MinIO console accessible" "PASS"
else
    log_test "MinIO console accessible" "FAIL"
fi

# Check MinIO version
if curl -sf "${MINIO_URL}/minio/health/cluster" 2>/dev/null | grep -q "status" || curl -sf "${MINIO_URL}/minio/health/live" > /dev/null 2>&1; then
    log_test "MinIO cluster status" "PASS"
else
    log_test "MinIO cluster status" "FAIL"
fi

# ===========================================
# Test 2: Docker Compose Configuration (4 tests)
# ===========================================
echo ""
echo "[2] Docker Compose Configuration"

if grep -q "minio:" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "MinIO service defined" "PASS"
else
    log_test "MinIO service defined" "FAIL"
fi

if grep -q "minio-init:" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "MinIO init service defined" "PASS"
else
    log_test "MinIO init service defined" "FAIL"
fi

if grep -q "9000" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "S3 API port configured" "PASS"
else
    log_test "S3 API port configured" "FAIL"
fi

if grep -q "healthcheck" docker-compose.bigdata.yml 2>/dev/null && grep -q "minio" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "MinIO health check configured" "PASS"
else
    log_test "MinIO health check configured" "FAIL"
fi

# ===========================================
# Test 3: Bucket Configuration (6 tests)
# ===========================================
echo ""
echo "[3] Bucket Configuration"

# Check expected buckets are configured
EXPECTED_BUCKETS=("helixagent-events" "helixagent-checkpoints" "helixagent-iceberg" "helixagent-models" "helixagent-audit" "helixagent-flink")

for bucket in "${EXPECTED_BUCKETS[@]}"; do
    if grep -q "$bucket" docker-compose.bigdata.yml 2>/dev/null; then
        log_test "Bucket $bucket configured" "PASS"
    else
        log_test "Bucket $bucket configured" "FAIL"
    fi
done

# ===========================================
# Test 4: Lifecycle Policies (3 tests)
# ===========================================
echo ""
echo "[4] Lifecycle Policies"

if grep -q "ilm rule" docker-compose.bigdata.yml 2>/dev/null || grep -q "expire-days" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Lifecycle rules configured" "PASS"
else
    log_test "Lifecycle rules configured" "FAIL"
fi

if grep -q "expire-days 90" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Events bucket retention (90 days)" "PASS"
else
    log_test "Events bucket retention" "FAIL"
fi

if grep -q "expire-days 365" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Audit bucket retention (365 days)" "PASS"
else
    log_test "Audit bucket retention" "FAIL"
fi

# ===========================================
# Test 5: Configuration File (4 tests)
# ===========================================
echo ""
echo "[5] Configuration File"

if [ -f "configs/bigdata.yaml" ]; then
    log_test "bigdata.yaml exists" "PASS"
else
    log_test "bigdata.yaml exists" "FAIL"
fi

if grep -q "minio:" configs/bigdata.yaml 2>/dev/null; then
    log_test "MinIO section defined" "PASS"
else
    log_test "MinIO section defined" "FAIL"
fi

if grep -q "buckets:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Buckets configuration defined" "PASS"
else
    log_test "Buckets configuration defined" "FAIL"
fi

if grep -q "retention_days:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Retention policies defined" "PASS"
else
    log_test "Retention policies defined" "FAIL"
fi

# ===========================================
# Test 6: Security Configuration (3 tests)
# ===========================================
echo ""
echo "[6] Security Configuration"

# Check for environment variables (credentials)
if grep -q "MINIO_ROOT_USER" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "MinIO root user env var" "PASS"
else
    log_test "MinIO root user env var" "FAIL"
fi

if grep -q "MINIO_ROOT_PASSWORD" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "MinIO root password env var" "PASS"
else
    log_test "MinIO root password env var" "FAIL"
fi

if grep -q "use_ssl:" configs/bigdata.yaml 2>/dev/null; then
    log_test "SSL configuration defined" "PASS"
else
    log_test "SSL configuration defined" "FAIL"
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
