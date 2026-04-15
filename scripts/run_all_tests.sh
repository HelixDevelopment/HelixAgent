#!/bin/bash
#
# run_all_tests.sh - Runs all tests with full infrastructure
#
# This script starts all required services (PostgreSQL, Redis, Mock LLM, etc.)
# and runs all tests WITHOUT SKIPPING any.
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== HelixAgent Full Test Suite ===${NC}"
echo ""

# Function to cleanup on exit
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    cd "$PROJECT_ROOT"
    docker compose -f docker-compose.test.yml down --volumes --remove-orphans 2>/dev/null || true
}

# Register cleanup trap
trap cleanup EXIT

# Start Docker Compose infrastructure
echo -e "${YELLOW}Starting test infrastructure...${NC}"
cd "$PROJECT_ROOT"

# Build mock LLM server first
echo "Building mock LLM server..."
docker compose -f docker-compose.test.yml build mock-llm

# Start all services
echo "Starting services..."
docker compose -f docker-compose.test.yml up -d postgres redis mock-llm

# Wait for services to be healthy
echo -e "${YELLOW}Waiting for services to be ready...${NC}"

wait_for_service() {
    local service=$1
    local url=$2
    local max_attempts=30
    local attempt=1

    echo "Waiting for $service..."
    while [ $attempt -le $max_attempts ]; do
        if curl -sf "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}$service is ready!${NC}"
            return 0
        fi
        echo "  Attempt $attempt/$max_attempts..."
        sleep 2
        attempt=$((attempt + 1))
    done

    echo -e "${RED}$service failed to start${NC}"
    return 1
}

# Wait for PostgreSQL
echo "Waiting for PostgreSQL..."
for i in {1..30}; do
    if docker compose -f docker-compose.test.yml exec -T postgres pg_isready -U helixagent -d helixagent_db > /dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    fi
    echo "  Attempt $i/30..."
    sleep 2
done

# Wait for Redis
echo "Waiting for Redis..."
for i in {1..30}; do
    if docker compose -f docker-compose.test.yml exec -T redis redis-cli ping > /dev/null 2>&1; then
        echo -e "${GREEN}Redis is ready!${NC}"
        break
    fi
    echo "  Attempt $i/30..."
    sleep 2
done

# Wait for Mock LLM
wait_for_service "Mock LLM" "http://localhost:18081/health"

# Export environment variables for tests
export DB_HOST=localhost
export DB_PORT=15432
export DB_USER=helixagent
export DB_PASSWORD=helixagent123
export DB_NAME=helixagent_db
export DATABASE_URL="postgres://helixagent:helixagent123@localhost:15432/helixagent_db?sslmode=disable"

export REDIS_HOST=localhost
export REDIS_PORT=16379
export REDIS_PASSWORD=helixagent123
export REDIS_URL="redis://:helixagent123@localhost:16379"

export MOCK_LLM_URL=http://localhost:18081
export MOCK_LLM_ENABLED=true

# LLM Provider configurations pointing to mock server
export CLAUDE_API_KEY=mock-api-key
export CLAUDE_BASE_URL=http://localhost:18081/v1
export DEEPSEEK_API_KEY=mock-api-key
export DEEPSEEK_BASE_URL=http://localhost:18081/v1
export GEMINI_API_KEY=mock-api-key
export GEMINI_BASE_URL=http://localhost:18081/v1
export QWEN_API_KEY=mock-api-key
export QWEN_BASE_URL=http://localhost:18081/v1
export ZAI_API_KEY=mock-api-key
export ZAI_BASE_URL=http://localhost:18081/v1
export OLLAMA_BASE_URL=http://localhost:18081

# JWT and Server configuration
export JWT_SECRET=test-jwt-secret-key-for-testing
export HELIXAGENT_API_KEY=test-api-key

# Test configuration
export GIN_MODE=test
export CI=true

echo ""
echo -e "${GREEN}=== Running All Tests ===${NC}"
echo ""

TEST_PACKAGES="./cmd/... ./internal/... ./pkg/... ./tests/... ./challenges/..."

# Run all tests with verbose output (increased timeout for large test suites)
cd "$PROJECT_ROOT"
go test $TEST_PACKAGES -v -timeout 900s -p 4 -cover 2>&1 | tee test_results.log

# Count results
PASSED=$(grep -c "^--- PASS" test_results.log || echo "0")
FAILED=$(grep -c "^--- FAIL" test_results.log || echo "0")
SKIPPED=$(grep -c "^--- SKIP" test_results.log || echo "0")

echo ""
echo -e "${GREEN}=== Test Results Summary ===${NC}"
echo -e "  ${GREEN}PASSED:${NC}  $PASSED"
echo -e "  ${RED}FAILED:${NC}  $FAILED"
echo -e "  ${YELLOW}SKIPPED:${NC} $SKIPPED"
echo ""

if [ "$FAILED" -gt 0 ]; then
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi

if [ "$SKIPPED" -gt 0 ]; then
    echo -e "${YELLOW}Warning: Some tests were skipped.${NC}"
    echo "Check test_results.log for details."
fi

echo -e "${GREEN}All tests completed successfully!${NC}"
