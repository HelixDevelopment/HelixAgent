#!/bin/bash
# qdrant_vector_challenge.sh - Qdrant Vector Database Challenge
# Tests Qdrant setup and vector operations for HelixAgent RAG

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Qdrant Vector Database Challenge"
PASSED=0
FAILED=0
TOTAL=0

# Configuration
QDRANT_HOST="${QDRANT_HOST:-localhost}"
QDRANT_HTTP_PORT="${QDRANT_HTTP_PORT:-6333}"
QDRANT_GRPC_PORT="${QDRANT_GRPC_PORT:-6334}"
QDRANT_URL="http://${QDRANT_HOST}:${QDRANT_HTTP_PORT}"

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
# Test 1: Server Health (4 tests)
# ===========================================
echo "[1] Server Health"

# Check Qdrant health endpoint
if curl -sf "${QDRANT_URL}/health" > /dev/null 2>&1; then
    log_test "Qdrant health check" "PASS"
else
    log_test "Qdrant health check" "FAIL"
fi

# Check Qdrant readiness
if curl -sf "${QDRANT_URL}/readyz" > /dev/null 2>&1 || curl -sf "${QDRANT_URL}/" > /dev/null 2>&1; then
    log_test "Qdrant ready" "PASS"
else
    log_test "Qdrant ready" "FAIL"
fi

# Check gRPC port (basic TCP check)
if nc -z "${QDRANT_HOST}" "${QDRANT_GRPC_PORT}" 2>/dev/null; then
    log_test "gRPC port accessible" "PASS"
else
    log_test "gRPC port accessible" "FAIL"
fi

# Get cluster info
if curl -sf "${QDRANT_URL}/cluster" 2>/dev/null | grep -q "status" || curl -sf "${QDRANT_URL}/" > /dev/null 2>&1; then
    log_test "Cluster info accessible" "PASS"
else
    log_test "Cluster info accessible" "FAIL"
fi

# ===========================================
# Test 2: Docker Compose Configuration (4 tests)
# ===========================================
echo ""
echo "[2] Docker Compose Configuration"

if grep -q "qdrant:" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Qdrant service defined" "PASS"
else
    log_test "Qdrant service defined" "FAIL"
fi

if grep -q "6333" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "HTTP port configured" "PASS"
else
    log_test "HTTP port configured" "FAIL"
fi

if grep -q "6334" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "gRPC port configured" "PASS"
else
    log_test "gRPC port configured" "FAIL"
fi

if grep -q "qdrant-init:" docker-compose.bigdata.yml 2>/dev/null; then
    log_test "Qdrant init service defined" "PASS"
else
    log_test "Qdrant init service defined" "FAIL"
fi

# ===========================================
# Test 3: Collection Configuration (5 tests)
# ===========================================
echo ""
echo "[3] Collection Configuration"

# Check collections in init script
EXPECTED_COLLECTIONS=("debate_contexts" "agent_memory" "tool_descriptions" "document_chunks" "user_preferences")

for collection in "${EXPECTED_COLLECTIONS[@]}"; do
    if grep -q "$collection" docker-compose.bigdata.yml 2>/dev/null; then
        log_test "Collection '$collection' configured" "PASS"
    else
        log_test "Collection '$collection' configured" "FAIL"
    fi
done

# ===========================================
# Test 4: REST API Operations (4 tests)
# ===========================================
echo ""
echo "[4] REST API Operations"

# Get collections list
COLLECTIONS=$(curl -sf "${QDRANT_URL}/collections" 2>/dev/null)
if echo "$COLLECTIONS" | grep -q "collections" 2>/dev/null; then
    log_test "GET /collections" "PASS"
else
    log_test "GET /collections" "FAIL"
fi

# Get telemetry
if curl -sf "${QDRANT_URL}/telemetry" > /dev/null 2>&1 || curl -sf "${QDRANT_URL}/" > /dev/null 2>&1; then
    log_test "GET /telemetry or root" "PASS"
else
    log_test "GET /telemetry or root" "FAIL"
fi

# Check specific collection existence (debate_contexts)
if curl -sf "${QDRANT_URL}/collections/debate_contexts" 2>/dev/null | grep -q "result" || \
   curl -sf "${QDRANT_URL}/collections" 2>/dev/null | grep -q "debate_contexts"; then
    log_test "Collection 'debate_contexts' exists" "PASS"
else
    log_test "Collection 'debate_contexts' exists" "FAIL"
fi

# Check agent_memory collection
if curl -sf "${QDRANT_URL}/collections/agent_memory" 2>/dev/null | grep -q "result" || \
   curl -sf "${QDRANT_URL}/collections" 2>/dev/null | grep -q "agent_memory"; then
    log_test "Collection 'agent_memory' exists" "PASS"
else
    log_test "Collection 'agent_memory' exists" "FAIL"
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

if grep -q "qdrant:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Qdrant section defined" "PASS"
else
    log_test "Qdrant section defined" "FAIL"
fi

if grep -q "collections:" configs/bigdata.yaml 2>/dev/null; then
    log_test "Collections config defined" "PASS"
else
    log_test "Collections config defined" "FAIL"
fi

if grep -q "rag:" configs/bigdata.yaml 2>/dev/null; then
    log_test "RAG pipeline config defined" "PASS"
else
    log_test "RAG pipeline config defined" "FAIL"
fi

# ===========================================
# Test 6: Vector Settings (4 tests)
# ===========================================
echo ""
echo "[6] Vector Settings"

# Check vector dimensions in config
if grep -q "vector_size: 1536" configs/bigdata.yaml 2>/dev/null; then
    log_test "OpenAI embedding dimension (1536)" "PASS"
else
    log_test "OpenAI embedding dimension" "FAIL"
fi

if grep -q "vector_size: 768" configs/bigdata.yaml 2>/dev/null; then
    log_test "Small model dimension (768)" "PASS"
else
    log_test "Small model dimension" "FAIL"
fi

if grep -q "distance: Cosine" configs/bigdata.yaml 2>/dev/null; then
    log_test "Cosine distance metric" "PASS"
else
    log_test "Cosine distance metric" "FAIL"
fi

if grep -q "indexing_threshold" configs/bigdata.yaml 2>/dev/null; then
    log_test "Indexing threshold configured" "PASS"
else
    log_test "Indexing threshold configured" "FAIL"
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
