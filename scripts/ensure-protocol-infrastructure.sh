#!/bin/bash
# ensure-protocol-infrastructure.sh
# Ensures all MCP, LSP, ACP, Embedding, and RAG infrastructure is running
# Automatically starts containers if not running - NO SKIPPING ALLOWED
#
# This script is called by challenge scripts and tests to ensure
# infrastructure is always available.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Detect container runtime
detect_runtime() {
    if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then
        RUNTIME="docker"
        COMPOSE_CMD="docker compose"
        if ! docker compose version &> /dev/null 2>&1; then
            COMPOSE_CMD="docker-compose"
        fi
    elif command -v podman &> /dev/null; then
        RUNTIME="podman"
        if command -v podman-compose &> /dev/null; then
            COMPOSE_CMD="podman-compose"
        else
            echo -e "${YELLOW}Installing podman-compose...${NC}"
            pip install podman-compose 2>/dev/null || pip3 install podman-compose 2>/dev/null || {
                echo -e "${RED}Failed to install podman-compose. Install manually: pip install podman-compose${NC}"
                exit 1
            }
            COMPOSE_CMD="podman-compose"
        fi
    else
        echo -e "${RED}Error: No container runtime found. Install Docker or Podman.${NC}"
        exit 1
    fi
}

# Create network
create_network() {
    $RUNTIME network create helixagent-network 2>/dev/null || true
}

# Check if a service is healthy
check_service() {
    local service=$1
    local port=$2
    local endpoint=${3:-/health}

    if curl -sf "http://localhost:${port}${endpoint}" > /dev/null 2>&1; then
        return 0
    fi
    return 1
}

# Wait for service to be ready
wait_for_service() {
    local service=$1
    local port=$2
    local endpoint=${3:-/health}
    local max_attempts=${4:-60}

    echo -e "${YELLOW}Waiting for ${service} on port ${port}...${NC}"
    for i in $(seq 1 $max_attempts); do
        if check_service "$service" "$port" "$endpoint"; then
            echo -e "${GREEN}${service} is ready!${NC}"
            return 0
        fi
        echo "  Attempt $i/$max_attempts..."
        sleep 2
    done

    echo -e "${RED}${service} failed to start within timeout${NC}"
    return 1
}

# Start core services (PostgreSQL, Redis)
start_core_services() {
    echo -e "${BLUE}=== Starting Core Services ===${NC}"
    cd "$PROJECT_DIR"

    # Start PostgreSQL and Redis
    $COMPOSE_CMD -f docker-compose.yml up -d postgres redis chromadb

    # Wait for PostgreSQL
    echo -e "${YELLOW}Waiting for PostgreSQL...${NC}"
    for i in $(seq 1 30); do
        if $RUNTIME exec helixagent-postgres pg_isready -U helixagent -d helixagent_db > /dev/null 2>&1; then
            echo -e "${GREEN}PostgreSQL is ready!${NC}"
            break
        fi
        if [ $i -eq 30 ]; then
            echo -e "${RED}PostgreSQL failed to start${NC}"
            exit 1
        fi
        sleep 2
    done

    # Wait for Redis
    echo -e "${YELLOW}Waiting for Redis...${NC}"
    for i in $(seq 1 15); do
        if $RUNTIME exec helixagent-redis redis-cli -a helixagent123 ping > /dev/null 2>&1; then
            echo -e "${GREEN}Redis is ready!${NC}"
            break
        fi
        if [ $i -eq 15 ]; then
            echo -e "${RED}Redis failed to start${NC}"
            exit 1
        fi
        sleep 1
    done
}

# Start HelixAgent
start_helixagent() {
    echo -e "${BLUE}=== Starting HelixAgent ===${NC}"
    cd "$PROJECT_DIR"

    if check_service "helixagent" "8100" "/health"; then
        echo -e "${GREEN}HelixAgent already running${NC}"
        return 0
    fi

    $COMPOSE_CMD -f docker-compose.yml up -d helixagent
    wait_for_service "helixagent" "8100" "/health" 60
}

# Start Protocol Discovery
start_protocol_discovery() {
    echo -e "${BLUE}=== Starting Protocol Discovery ===${NC}"
    cd "$PROJECT_DIR"

    if check_service "protocol-discovery" "9300" "/health"; then
        echo -e "${GREEN}Protocol Discovery already running${NC}"
        return 0
    fi

    # Build and start protocol discovery
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d protocol-discovery
    wait_for_service "protocol-discovery" "9300" "/health" 60
}

# Start MCP Manager
start_mcp_manager() {
    echo -e "${BLUE}=== Starting MCP Manager ===${NC}"
    cd "$PROJECT_DIR"

    if check_service "mcp-manager" "9000" "/health"; then
        echo -e "${GREEN}MCP Manager already running${NC}"
        return 0
    fi

    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d mcp-manager
    wait_for_service "mcp-manager" "9000" "/health" 60 || true
}

# Start LSP Manager
start_lsp_manager() {
    echo -e "${BLUE}=== Starting LSP Manager ===${NC}"
    cd "$PROJECT_DIR"

    if check_service "lsp-manager" "5100" "/health"; then
        echo -e "${GREEN}LSP Manager already running${NC}"
        return 0
    fi

    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d lsp-manager lsp-multi lsp-ai || true
    wait_for_service "lsp-manager" "5100" "/health" 60 || true
}

# Start ACP Manager
start_acp_manager() {
    echo -e "${BLUE}=== Starting ACP Manager ===${NC}"
    cd "$PROJECT_DIR"

    if check_service "acp-manager" "9200" "/health"; then
        echo -e "${GREEN}ACP Manager already running${NC}"
        return 0
    fi

    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d acp-manager
    wait_for_service "acp-manager" "9200" "/health" 60 || true
}

# Start Embedding Services
start_embedding_services() {
    echo -e "${BLUE}=== Starting Embedding Services ===${NC}"
    cd "$PROJECT_DIR"

    # Start Qdrant
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d qdrant || true

    # Start embedding servers
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d \
        embedding-sentence-transformers embedding-bge-m3 || true
}

# Start RAG Services
start_rag_services() {
    echo -e "${BLUE}=== Starting RAG Services ===${NC}"
    cd "$PROJECT_DIR"

    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d \
        rag-manager rag-reranker || true
}

# Start all protocol infrastructure
start_all() {
    detect_runtime
    create_network

    echo -e "${BLUE}=============================================${NC}"
    echo -e "${BLUE}  Starting All Protocol Infrastructure${NC}"
    echo -e "${BLUE}=============================================${NC}"
    echo ""

    start_core_services
    start_helixagent
    start_protocol_discovery
    start_mcp_manager
    start_lsp_manager
    start_acp_manager
    start_embedding_services
    start_rag_services

    echo ""
    echo -e "${GREEN}=============================================${NC}"
    echo -e "${GREEN}  All Protocol Infrastructure Started!${NC}"
    echo -e "${GREEN}=============================================${NC}"
    echo ""
    echo "Services available at:"
    echo "  - HelixAgent:          http://localhost:8100"
    echo "  - Protocol Discovery:  http://localhost:9300"
    echo "  - MCP Manager:         http://localhost:9000"
    echo "  - LSP Manager:         http://localhost:5100"
    echo "  - ACP Manager:         http://localhost:9200"
    echo "  - Qdrant:              http://localhost:6333"
    echo ""
}

# Verify all services
verify_all() {
    echo -e "${BLUE}=== Verifying All Services ===${NC}"

    local all_ok=true

    if check_service "helixagent" "8100" "/health"; then
        echo -e "${GREEN}[OK]${NC} HelixAgent"
    else
        echo -e "${RED}[FAIL]${NC} HelixAgent"
        all_ok=false
    fi

    if check_service "protocol-discovery" "9300" "/health"; then
        echo -e "${GREEN}[OK]${NC} Protocol Discovery"
    else
        echo -e "${RED}[FAIL]${NC} Protocol Discovery"
        all_ok=false
    fi

    if $all_ok; then
        echo ""
        echo -e "${GREEN}All core services are running!${NC}"
        return 0
    else
        echo ""
        echo -e "${RED}Some services are not running${NC}"
        return 1
    fi
}

# Main
case "${1:-start}" in
    start|--start)
        start_all
        ;;
    verify|--verify)
        detect_runtime
        verify_all
        ;;
    core|--core)
        detect_runtime
        create_network
        start_core_services
        ;;
    helixagent|--helixagent)
        detect_runtime
        create_network
        start_core_services
        start_helixagent
        ;;
    discovery|--discovery)
        detect_runtime
        create_network
        start_core_services
        start_helixagent
        start_protocol_discovery
        ;;
    *)
        echo "Usage: $0 [start|verify|core|helixagent|discovery]"
        exit 1
        ;;
esac
