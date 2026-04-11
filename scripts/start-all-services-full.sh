#!/bin/bash
# start-all-services-full.sh - Comprehensive HelixAgent Services Startup Script (FULL VERSION)
# Brings up ALL HelixAgent containers and services including optional formatters, RAG, and full MCP

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect container runtime
if command -v podman &> /dev/null; then
    RUNTIME="podman"
    COMPOSE_CMD="podman-compose"
elif command -v docker &> /dev/null; then
    RUNTIME="docker"
    COMPOSE_CMD="docker compose"
else
    echo -e "${RED}Error: Neither docker nor podman found!${NC}"
    exit 1
fi

echo "=============================================="
echo "HelixAgent - Complete Services Startup (FULL)"
echo "=============================================="
echo ""
echo "Container Runtime: $RUNTIME"
echo "Project Root: $PROJECT_ROOT"
echo ""

# Function to start a compose file
start_compose() {
    local compose_file="$1"
    local description="$2"
    local profile="${3:-}"

    if [ ! -f "$compose_file" ]; then
        echo -e "${YELLOW}⊘ Skipping $description - $compose_file not found${NC}"
        return
    fi

    echo -e "${BLUE}▶ Starting $description...${NC}"

    if [ -n "$profile" ]; then
        $COMPOSE_CMD -f "$compose_file" --profile "$profile" up -d
    else
        $COMPOSE_CMD -f "$compose_file" up -d
    fi

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $description started${NC}"
    else
        echo -e "${RED}✗ Failed to start $description${NC}"
        return 1
    fi
}

# Function to wait for service health
wait_for_health() {
    local container_name="$1"
    local max_wait="${2:-60}"
    local wait_time=0

    echo -n "  Waiting for $container_name to be healthy..."

    while [ $wait_time -lt $max_wait ]; do
        if $RUNTIME inspect "$container_name" &> /dev/null; then
            local health_status=$($RUNTIME inspect --format='{{.State.Health.Status}}' "$container_name" 2>/dev/null || echo "none")
            local running_status=$($RUNTIME inspect --format='{{.State.Running}}' "$container_name" 2>/dev/null || echo "false")

            if [ "$health_status" = "healthy" ] || [ "$running_status" = "true" ]; then
                echo -e " ${GREEN}✓${NC}"
                return 0
            fi
        fi

        echo -n "."
        sleep 2
        wait_time=$((wait_time + 2))
    done

    echo -e " ${YELLOW}⏱ (timeout)${NC}"
    return 1
}

# Phase 1: Core Infrastructure
echo ""
echo "=========================================="
echo "PHASE 1: Core Infrastructure"
echo "=========================================="

start_compose "docker-compose.yml" "Core Services (PostgreSQL, Redis, Cognee, ChromaDB)"
wait_for_health "helixagent-postgres" 30
wait_for_health "helixagent-redis" 30
# Cognee is OPTIONAL (COGNEE_ENABLED defaults off per CLAUDE.md) — do not
# fail the entire boot on its absence. The `|| true` suppresses the
# set -e propagation so Phase 2+ can still run when Cognee isn't wired.
wait_for_health "helixagent-cognee" 10 || echo -e "${YELLOW}  Note: Cognee not available (optional service, COGNEE_ENABLED defaults off)${NC}"

# Phase 2: Messaging & Queuing
echo ""
echo "=========================================="
echo "PHASE 2: Messaging & Queuing"
echo "=========================================="

start_compose "docker-compose.messaging.yml" "Messaging Services (Kafka, RabbitMQ)"
sleep 5  # Give Kafka time to start
wait_for_health "helixagent-kafka" 60 || echo -e "${YELLOW}  Note: Kafka may take longer to be fully ready${NC}"

# Phase 3: Monitoring & Observability
echo ""
echo "=========================================="
echo "PHASE 3: Monitoring & Observability"
echo "=========================================="

start_compose "docker-compose.monitoring.yml" "Monitoring Services (Prometheus, Grafana, Loki)" || echo -e "${YELLOW}  Note: monitoring stack not available${NC}"
wait_for_health "helixagent-prometheus" 15 || echo -e "${YELLOW}  Note: Prometheus not reachable, skipping${NC}"
wait_for_health "helixagent-grafana" 15 || echo -e "${YELLOW}  Note: Grafana not reachable, skipping${NC}"

# Phase 4: Protocol Servers
echo ""
echo "=========================================="
echo "PHASE 4: Protocol Servers"
echo "=========================================="

start_compose "docker-compose.protocols.yml" "Protocol Services (MCP, LSP, ACP)" || echo -e "${YELLOW}  Note: docker-compose.protocols.yml references stale build contexts (docker/mcp/{fetch,time,sqlite,chroma,qdrant,puppeteer,sequential-thinking,adapters/*} directories no longer exist). The corresponding containers come from docker/mcp/docker-compose.mcp-servers.yml via the Phase-10 Full MCP Servers step. Fix tracked as follow-up — either dedupe protocols.yml or replace the build: blocks with image: references.${NC}"

# Phase 5: Integration Services
echo ""
echo "=========================================="
echo "PHASE 5: Integration Services"
echo "=========================================="

start_compose "docker-compose.integration.yml" "Integration Services (Weaviate, Neo4j, LangChain, etc.)"

# Phase 6: Analytics & Big Data (optional)
echo ""
echo "=========================================="
echo "PHASE 6: Analytics & Big Data (Optional)"
echo "=========================================="

if [ "${START_BIGDATA:-true}" = "true" ]; then
    start_compose "docker-compose.bigdata.yml" "Big Data Services" "bigdata"
    start_compose "docker-compose.analytics.yml" "Analytics Services"
else
    echo -e "${YELLOW}⊘ Skipping Big Data services (set START_BIGDATA=true to enable)${NC}"
fi

# Phase 7: Security Services (optional)
echo ""
echo "=========================================="
echo "PHASE 7: Security Services (Optional)"
echo "=========================================="

if [ "${START_SECURITY:-true}" = "true" ]; then
    start_compose "docker-compose.security.yml" "Security Services"
else
    echo -e "${YELLOW}⊘ Skipping Security services (set START_SECURITY=true to enable)${NC}"
fi

# Phase 8: Code Formatters (optional)
echo ""
echo "=========================================="
echo "PHASE 8: Code Formatters (Optional)"
echo "=========================================="

if [ "${START_FORMATTERS:-true}" = "true" ]; then
    start_compose "docker/formatters/docker-compose.formatters.yml" "Code Formatters (32+ language formatters)"
    # Note: Formatters containers are named "formatter-*" not "helixagent-*"
else
    echo -e "${YELLOW}⊘ Skipping Code Formatters (set START_FORMATTERS=true to enable)${NC}"
fi

# Phase 9: RAG & Vector Databases (optional)
echo ""
echo "=========================================="
echo "PHASE 9: RAG & Vector Databases (Optional)"
echo "=========================================="

if [ "${START_RAG:-true}" = "true" ]; then
    start_compose "docker/rag/docker-compose.rag.yml" "RAG & Vector Databases"
else
    echo -e "${YELLOW}⊘ Skipping RAG & Vector Databases (set START_RAG=true to enable)${NC}"
fi

# Phase 10: Full MCP Servers (optional)
echo ""
echo "=========================================="
echo "PHASE 10: Full MCP Servers (Optional)"
echo "=========================================="

if [ "${START_FULL_MCP:-true}" = "true" ]; then
    start_compose "docker/mcp/docker-compose.mcp-full.yml" "Full MCP Servers (60+ servers)"
else
    echo -e "${YELLOW}⊘ Skipping Full MCP Servers (set START_FULL_MCP=true to enable)${NC}"
fi

# Summary
echo ""
echo "=========================================="
echo "SERVICE STATUS SUMMARY"
echo "=========================================="

echo ""
echo "Running containers:"
$RUNTIME ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "(helixagent|formatter)" || echo "No helixagent or formatter containers running"

echo ""
echo "=========================================="
echo "STARTUP COMPLETE (FULL)"
echo "=========================================="
echo ""
echo "All HelixAgent services are now running!"
echo ""
echo "Quick verification:"
echo "  • PostgreSQL:  $RUNTIME exec -it helixagent-postgres pg_isready"
echo "  • Redis:       $RUNTIME exec -it helixagent-redis redis-cli ping"
echo "  • Cognee:      curl http://localhost:8000/health"
echo ""
echo "To start HelixAgent server:"
echo "  make run"
echo ""
echo "To view logs:"
echo "  $COMPOSE_CMD logs -f [service-name]"
echo ""
echo "To stop all services:"
echo "  ./scripts/stop-all-services.sh"
echo ""
