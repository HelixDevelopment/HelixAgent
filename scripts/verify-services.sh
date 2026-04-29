#!/bin/bash
# verify-services.sh - Verify HelixAgent Services Status
# Checks the health and connectivity of all running services

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Detect container runtime
if command -v podman &> /dev/null; then
    RUNTIME="podman"
elif command -v docker &> /dev/null; then
    RUNTIME="docker"
else
    echo -e "${RED}Error: Neither docker nor podman found!${NC}"
    exit 1
fi

echo "=============================================="
echo "HelixAgent - Services Verification"
echo "=============================================="
echo ""

TOTAL=0
PASSED=0
FAILED=0

# Function to test service
test_service() {
    local name="$1"
    local test_cmd="$2"
    local container="$3"

    TOTAL=$((TOTAL + 1))
    echo -n "Testing $name... "

    if eval "$test_cmd" &> /dev/null; then
        echo -e "${GREEN}✓${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗${NC}"
        FAILED=$((FAILED + 1))
        if [ -n "$container" ]; then
            echo "  Container status: $($RUNTIME ps -a --filter name=$container --format '{{.Status}}')"
        fi
    fi
}

# Test container existence
test_container() {
    local name="$1"
    local container="$2"

    TOTAL=$((TOTAL + 1))
    echo -n "Checking $name container... "

    if $RUNTIME ps --filter name="$container" --format '{{.Names}}' | grep -q "$container"; then
        local status=$($RUNTIME ps --filter name="$container" --format '{{.Status}}')
        if echo "$status" | grep -q "Up"; then
            echo -e "${GREEN}✓ Running${NC}"
            PASSED=$((PASSED + 1))
        else
            echo -e "${YELLOW}⊘ Stopped${NC}"
            FAILED=$((FAILED + 1))
        fi
    else
        echo -e "${RED}✗ Not found${NC}"
        FAILED=$((FAILED + 1))
    fi
}

# Core Services
echo "=== Core Infrastructure ==="
test_container "PostgreSQL" "helixagent-postgres"
test_container "Redis" "helixagent-redis"
test_container "Cognee" "helixagent-cognee"
test_container "ChromaDB" "helixagent-chromadb"

# PostgreSQL connectivity
test_service "PostgreSQL connectivity" "$RUNTIME exec helixagent-postgres pg_isready -q" "helixagent-postgres"

# Redis connectivity
test_service "Redis connectivity" "$RUNTIME exec helixagent-redis redis-cli ping | grep -q PONG" "helixagent-redis"

# Cognee health
test_service "Cognee health" "curl -sf http://localhost:8000/health" "helixagent-cognee"

echo ""
echo "=== Messaging Services ==="
test_container "Kafka" "helixagent-kafka"
test_container "RabbitMQ" "helixagent-rabbitmq"
test_container "Zookeeper" "helixagent-zookeeper"

echo ""
echo "=== Monitoring Services ==="
test_container "Prometheus" "helixagent-prometheus"
test_container "Grafana" "helixagent-grafana"

# Prometheus health
test_service "Prometheus health" "curl -sf http://localhost:9090/-/healthy" "helixagent-prometheus"

# Grafana health
test_service "Grafana health" "curl -sf http://localhost:3000/api/health" "helixagent-grafana"

echo ""
echo "=== HelixAgent Server ==="
test_service "HelixAgent API" "curl -sf http://localhost:8100/health"
test_service "HelixAgent verification endpoint" "curl -sf http://localhost:8100/v1/startup/verification"

echo ""
echo "=============================================="
echo "VERIFICATION SUMMARY"
echo "=============================================="
echo "Total checks: $TOTAL"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ All services verified successfully!${NC}"
    exit 0
else
    echo ""
    echo -e "${YELLOW}⚠ Some services are not running or not responding${NC}"
    echo ""
    echo "To start all services:"
    echo "  ./scripts/start-all-services.sh"
    echo ""
    echo "To view service logs:"
    echo "  podman-compose logs -f [service-name]"
    exit 1
fi
