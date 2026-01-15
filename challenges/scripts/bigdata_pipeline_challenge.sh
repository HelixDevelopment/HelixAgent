#!/bin/bash
# bigdata_pipeline_challenge.sh - Full Big Data Pipeline Challenge
# Tests the complete Big Data integration for HelixAgent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Big Data Pipeline Integration Challenge"
PASSED=0
FAILED=0
TOTAL=0

# Configuration
FLINK_URL="http://${FLINK_JOBMANAGER_HOST:-localhost}:${FLINK_UI_PORT:-8082}"
MINIO_URL="http://${MINIO_ENDPOINT:-localhost:9000}"
QDRANT_URL="http://${QDRANT_HOST:-localhost}:${QDRANT_HTTP_PORT:-6333}"
ICEBERG_URL="http://${ICEBERG_REST_HOST:-localhost}:${ICEBERG_REST_PORT:-8181}"
SPARK_URL="http://${SPARK_MASTER_HOST:-localhost}:${SPARK_MASTER_UI_PORT:-4040}"
SUPERSET_URL="http://${SUPERSET_HOST:-localhost}:${SUPERSET_PORT:-8088}"
KAFKA_BROKER="${KAFKA_BROKER:-localhost:9092}"

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
# Test 1: All Services Running (7 tests)
# ===========================================
echo "[1] Service Availability"

# MinIO
if curl -sf "${MINIO_URL}/minio/health/live" > /dev/null 2>&1; then
    log_test "MinIO running" "PASS"
else
    log_test "MinIO running" "FAIL"
fi

# Flink
if curl -sf "${FLINK_URL}/overview" > /dev/null 2>&1; then
    log_test "Flink JobManager running" "PASS"
else
    log_test "Flink JobManager running" "FAIL"
fi

# Qdrant
if curl -sf "${QDRANT_URL}/health" > /dev/null 2>&1; then
    log_test "Qdrant running" "PASS"
else
    log_test "Qdrant running" "FAIL"
fi

# Iceberg REST Catalog
if curl -sf "${ICEBERG_URL}/v1/config" > /dev/null 2>&1; then
    log_test "Iceberg REST catalog running" "PASS"
else
    log_test "Iceberg REST catalog running" "FAIL"
fi

# Spark Master (check web UI, may not be available during tests)
if curl -sf "http://localhost:4040" > /dev/null 2>&1 || curl -sf "${SPARK_URL}" > /dev/null 2>&1; then
    log_test "Spark Master running" "PASS"
else
    log_test "Spark Master running (or not started)" "FAIL"
fi

# Superset (optional)
if curl -sf "${SUPERSET_URL}/health" > /dev/null 2>&1; then
    log_test "Superset running" "PASS"
else
    log_test "Superset running (optional)" "FAIL"
fi

# Kafka (check via docker or existing challenge)
if docker exec helixagent-kafka kafka-topics --bootstrap-server localhost:9092 --list > /dev/null 2>&1; then
    log_test "Kafka running" "PASS"
else
    log_test "Kafka running" "FAIL"
fi

# ===========================================
# Test 2: Docker Compose Files (5 tests)
# ===========================================
echo ""
echo "[2] Docker Compose Files"

if [ -f "docker-compose.bigdata.yml" ]; then
    log_test "docker-compose.bigdata.yml exists" "PASS"
else
    log_test "docker-compose.bigdata.yml exists" "FAIL"
fi

if [ -f "docker-compose.analytics.yml" ]; then
    log_test "docker-compose.analytics.yml exists" "PASS"
else
    log_test "docker-compose.analytics.yml exists" "FAIL"
fi

if [ -f "docker-compose.messaging.yml" ]; then
    log_test "docker-compose.messaging.yml exists" "PASS"
else
    log_test "docker-compose.messaging.yml exists" "FAIL"
fi

# Check profiles defined
if grep -q "profiles:" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Profiles defined in bigdata compose" "PASS"
else
    log_test "Profiles defined in bigdata compose" "FAIL"
fi

# Check network configuration
if grep -q "helixagent-network" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Network configuration correct" "PASS"
else
    log_test "Network configuration correct" "FAIL"
fi

# ===========================================
# Test 3: Configuration Files (5 tests)
# ===========================================
echo ""
echo "[3] Configuration Files"

if [ -f "configs/bigdata.yaml" ]; then
    log_test "configs/bigdata.yaml exists" "PASS"
else
    log_test "configs/bigdata.yaml exists" "FAIL"
fi

if [ -f "configs/messaging.yaml" ]; then
    log_test "configs/messaging.yaml exists" "PASS"
else
    log_test "configs/messaging.yaml exists" "FAIL"
fi

if [ -f "configs/superset/superset_config.py" ]; then
    log_test "Superset config exists" "PASS"
else
    log_test "Superset config exists" "FAIL"
fi

# Check all major sections in bigdata.yaml
SECTIONS=("minio:" "flink:" "iceberg:" "qdrant:" "spark:")
for section in "${SECTIONS[@]}"; do
    if grep -q "$section" configs/bigdata.yaml 2>/dev/null; then
        log_test "Section $section in bigdata.yaml" "PASS"
    else
        log_test "Section $section in bigdata.yaml" "FAIL"
    fi
done

# ===========================================
# Test 4: Directory Structure (4 tests)
# ===========================================
echo ""
echo "[4] Directory Structure"

if [ -d "flink-jobs" ]; then
    log_test "flink-jobs directory exists" "PASS"
else
    log_test "flink-jobs directory exists" "FAIL"
fi

if [ -d "spark-jobs" ]; then
    log_test "spark-jobs directory exists" "PASS"
else
    log_test "spark-jobs directory exists" "FAIL"
fi

if [ -d "configs/superset" ]; then
    log_test "configs/superset directory exists" "PASS"
else
    log_test "configs/superset directory exists" "FAIL"
fi

if [ -d "configs/superset/dashboards" ]; then
    log_test "configs/superset/dashboards exists" "PASS"
else
    log_test "configs/superset/dashboards exists" "FAIL"
fi

# ===========================================
# Test 5: MinIO Buckets (6 tests)
# ===========================================
echo ""
echo "[5] MinIO Buckets Configuration"

EXPECTED_BUCKETS=("helixagent-events" "helixagent-checkpoints" "helixagent-iceberg" "helixagent-models" "helixagent-audit" "helixagent-flink")

for bucket in "${EXPECTED_BUCKETS[@]}"; do
    if grep -q "$bucket" docker-compose.bigdata.yml 2>/dev/null; then
        log_test "Bucket $bucket configured" "PASS"
    else
        log_test "Bucket $bucket configured" "FAIL"
    fi
done

# ===========================================
# Test 6: Qdrant Collections (5 tests)
# ===========================================
echo ""
echo "[6] Qdrant Collections Configuration"

EXPECTED_COLLECTIONS=("debate_contexts" "agent_memory" "tool_descriptions" "document_chunks" "user_preferences")

for collection in "${EXPECTED_COLLECTIONS[@]}"; do
    if grep -q "$collection" docker-compose.bigdata.yml 2>/dev/null || grep -q "$collection" configs/bigdata.yaml 2>/dev/null; then
        log_test "Collection $collection configured" "PASS"
    else
        log_test "Collection $collection configured" "FAIL"
    fi
done

# ===========================================
# Test 7: Flink Jobs Configuration (5 tests)
# ===========================================
echo ""
echo "[7] Flink Jobs Configuration"

EXPECTED_JOBS=("debate_orchestrator" "token_aggregator" "metrics_processor" "anomaly_detector" "session_manager")

for job in "${EXPECTED_JOBS[@]}"; do
    if grep -q "$job" configs/bigdata.yaml 2>/dev/null; then
        log_test "Job $job configured" "PASS"
    else
        log_test "Job $job configured" "FAIL"
    fi
done

# ===========================================
# Test 8: Integration Points (4 tests)
# ===========================================
echo ""
echo "[8] Integration Points"

# Flink -> Kafka integration
if grep -q "kafka:" configs/bigdata.yaml 2>/dev/null && grep -q "flink:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Flink-Kafka integration configured" "PASS"
else
    log_test "Flink-Kafka integration configured" "FAIL"
fi

# Flink -> MinIO (checkpoints)
if grep -q "s3://" configs/bigdata.yaml 2>/dev/null || grep -q "s3://" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Flink-MinIO checkpoint configured" "PASS"
else
    log_test "Flink-MinIO checkpoint configured" "FAIL"
fi

# Iceberg -> MinIO integration
if grep -q "CATALOG_S3_ENDPOINT" docker-compose.bigdata.yml 2>/dev/null || grep -q "s3://" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Iceberg-MinIO integration configured" "PASS"
else
    log_test "Iceberg-MinIO integration configured" "FAIL"
fi

# Event sink configuration
if grep -q "event_sink:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Event sink configured" "PASS"
else
    log_test "Event sink configured" "FAIL"
fi

# ===========================================
# Test 9: Health Check Configuration (4 tests)
# ===========================================
echo ""
echo "[9] Health Check Configuration"

# Check for health check endpoints in config
if grep -q "health:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Health check section defined" "PASS"
else
    log_test "Health check section defined" "FAIL"
fi

# Docker health checks
SERVICES_WITH_HEALTH=("minio" "flink-jobmanager" "qdrant" "iceberg-rest")
health_count=0
for service in "${SERVICES_WITH_HEALTH[@]}"; do
    if grep -A20 "${service}:" docker-compose.bigdata.yml 2>/dev/null | grep -q "healthcheck"; then
        ((health_count++))
    fi
done

if [ $health_count -ge 3 ]; then
    log_test "Docker health checks configured ($health_count services)" "PASS"
else
    log_test "Docker health checks configured ($health_count services)" "FAIL"
fi

# Restart policies
if grep -q "restart: unless-stopped" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Restart policies configured" "PASS"
else
    log_test "Restart policies configured" "FAIL"
fi

# Dependencies
if grep -q "depends_on:" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Service dependencies configured" "PASS"
else
    log_test "Service dependencies configured" "FAIL"
fi

# ===========================================
# Test 10: Plan File Exists (2 tests)
# ===========================================
echo ""
echo "[10] Integration Plan"

if [ -f "/home/milosvasic/.claude/plans/big-data-integration-plan.md" ]; then
    log_test "Big Data integration plan exists" "PASS"
else
    log_test "Big Data integration plan exists" "FAIL"
fi

# Check plan has all major sections
if [ -f "/home/milosvasic/.claude/plans/big-data-integration-plan.md" ]; then
    PLAN_SECTIONS=("Phase 1" "Phase 2" "Phase 3" "Phase 4" "Phase 5" "Testing" "Challenge")
    plan_section_count=0
    for section in "${PLAN_SECTIONS[@]}"; do
        if grep -q "$section" /home/milosvasic/.claude/plans/big-data-integration-plan.md 2>/dev/null; then
            ((plan_section_count++))
        fi
    done
    if [ $plan_section_count -ge 5 ]; then
        log_test "Plan has major sections ($plan_section_count)" "PASS"
    else
        log_test "Plan has major sections ($plan_section_count)" "FAIL"
    fi
else
    log_test "Plan has major sections" "FAIL"
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
