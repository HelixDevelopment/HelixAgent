#!/bin/bash
# start-protocol-servers.sh
# Starts all MCP, LSP, ACP, Embedding, and RAG servers using Docker/Podman Compose
#
# Usage:
#   ./scripts/start-protocol-servers.sh [options]
#
# Options:
#   --all       Start all protocol servers (default)
#   --mcp       Start only MCP servers
#   --lsp       Start only LSP servers
#   --acp       Start only ACP servers
#   --rag       Start only RAG/Embedding servers
#   --core      Start core services only (postgres, redis, helixagent)
#   --detach    Run in background (default)
#   --logs      Show logs after starting
#   --status    Show status of all servers
#   --stop      Stop all protocol servers
#   --clean     Stop and remove all volumes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect container runtime
detect_runtime() {
    if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then
        RUNTIME="docker"
        COMPOSE_CMD="docker compose"
        # Check if docker compose v2 is available
        if ! docker compose version &> /dev/null 2>&1; then
            COMPOSE_CMD="docker-compose"
        fi
    elif command -v podman &> /dev/null; then
        RUNTIME="podman"
        if command -v podman-compose &> /dev/null; then
            COMPOSE_CMD="podman-compose"
        else
            echo -e "${RED}Error: podman-compose not found. Install with: pip install podman-compose${NC}"
            exit 1
        fi
    else
        echo -e "${RED}Error: No container runtime found. Install Docker or Podman.${NC}"
        exit 1
    fi
    echo -e "${BLUE}Using container runtime: ${RUNTIME}${NC}"
}

# Create network if not exists
create_network() {
    echo -e "${BLUE}Creating helixagent-network...${NC}"
    $RUNTIME network create helixagent-network 2>/dev/null || true
}

# Start core services
start_core() {
    echo -e "${BLUE}Starting core services (PostgreSQL, Redis, HelixAgent)...${NC}"
    cd "$PROJECT_DIR"
    $COMPOSE_CMD -f docker-compose.yml up -d postgres redis

    echo -e "${YELLOW}Waiting for PostgreSQL to be ready...${NC}"
    for i in {1..30}; do
        if $COMPOSE_CMD -f docker-compose.yml exec -T postgres pg_isready -U helixagent -d helixagent_db > /dev/null 2>&1; then
            echo -e "${GREEN}PostgreSQL is ready!${NC}"
            break
        fi
        echo "  Waiting for PostgreSQL... ($i/30)"
        sleep 2
    done

    echo -e "${YELLOW}Waiting for Redis to be ready...${NC}"
    for i in {1..15}; do
        if $COMPOSE_CMD -f docker-compose.yml exec -T redis redis-cli ping > /dev/null 2>&1; then
            echo -e "${GREEN}Redis is ready!${NC}"
            break
        fi
        echo "  Waiting for Redis... ($i/15)"
        sleep 1
    done

    # Start HelixAgent
    $COMPOSE_CMD -f docker-compose.yml up -d helixagent

    echo -e "${YELLOW}Waiting for HelixAgent to be ready...${NC}"
    for i in {1..30}; do
        if curl -sf http://localhost:8100/health > /dev/null 2>&1; then
            echo -e "${GREEN}HelixAgent is ready!${NC}"
            break
        fi
        echo "  Waiting for HelixAgent... ($i/30)"
        sleep 2
    done
}

# Start all protocol servers
start_all() {
    echo -e "${BLUE}Starting ALL protocol servers...${NC}"
    cd "$PROJECT_DIR"

    # First start core services
    start_core

    # Then start all protocol servers
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d

    echo -e "${GREEN}All protocol servers started!${NC}"
}

# Start MCP servers only
start_mcp() {
    echo -e "${BLUE}Starting MCP servers...${NC}"
    cd "$PROJECT_DIR"

    # Start core first
    start_core

    # Start MCP-related services from protocols compose
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d \
        helixagent-mcp mcp-manager mcp-filesystem mcp-git mcp-memory \
        mcp-fetch mcp-time mcp-sqlite mcp-postgres mcp-puppeteer \
        mcp-sequential-thinking mcp-chroma mcp-qdrant protocol-discovery

    echo -e "${GREEN}MCP servers started!${NC}"
}

# Start LSP servers only
start_lsp() {
    echo -e "${BLUE}Starting LSP servers...${NC}"
    cd "$PROJECT_DIR"

    # Start core first
    start_core

    # Start LSP-related services
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d \
        lsp-ai lsp-multi lsp-manager

    echo -e "${GREEN}LSP servers started!${NC}"
}

# Start ACP servers only
start_acp() {
    echo -e "${BLUE}Starting ACP servers...${NC}"
    cd "$PROJECT_DIR"

    # Start core first
    start_core

    # Start ACP-related services
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d \
        acp-manager

    echo -e "${GREEN}ACP servers started!${NC}"
}

# Start RAG/Embedding servers only
start_rag() {
    echo -e "${BLUE}Starting RAG and Embedding servers...${NC}"
    cd "$PROJECT_DIR"

    # Start core first
    start_core

    # Start RAG-related services
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml up -d \
        qdrant chromadb embedding-sentence-transformers embedding-bge-m3 \
        rag-manager rag-reranker

    echo -e "${GREEN}RAG and Embedding servers started!${NC}"
}

# Show status
show_status() {
    echo -e "${BLUE}Protocol Server Status:${NC}"
    cd "$PROJECT_DIR"

    echo ""
    echo -e "${YELLOW}=== Core Services ===${NC}"
    $COMPOSE_CMD -f docker-compose.yml ps 2>/dev/null || true

    echo ""
    echo -e "${YELLOW}=== Protocol Servers ===${NC}"
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml ps 2>/dev/null || true

    echo ""
    echo -e "${YELLOW}=== Protocol Discovery ===${NC}"
    if curl -sf http://localhost:9300/v1/discovery > /dev/null 2>&1; then
        echo -e "${GREEN}Protocol Discovery Service: RUNNING${NC}"
        echo ""
        echo "Server counts:"
        curl -s http://localhost:9300/v1/discovery | jq -r '.total_servers as $t | "  Total: \($t)\n  MCP: \(.mcp_servers | length)\n  LSP: \(.lsp_servers | length)\n  ACP: \(.acp_servers | length)\n  Embedding: \(.embedding_servers | length)\n  RAG: \(.rag_servers | length)"' 2>/dev/null || echo "  (install jq for detailed output)"
    else
        echo -e "${RED}Protocol Discovery Service: NOT RUNNING${NC}"
    fi
}

# Stop all servers
stop_all() {
    echo -e "${BLUE}Stopping all protocol servers...${NC}"
    cd "$PROJECT_DIR"

    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml down

    echo -e "${GREEN}All protocol servers stopped!${NC}"
}

# Clean all
clean_all() {
    echo -e "${YELLOW}Stopping and removing all volumes...${NC}"
    cd "$PROJECT_DIR"

    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml down -v --remove-orphans

    echo -e "${GREEN}Cleanup complete!${NC}"
}

# Show logs
show_logs() {
    cd "$PROJECT_DIR"
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.protocols.yml logs -f
}

# Main
main() {
    detect_runtime
    create_network

    case "${1:-all}" in
        --all|-a|all)
            start_all
            ;;
        --mcp|mcp)
            start_mcp
            ;;
        --lsp|lsp)
            start_lsp
            ;;
        --acp|acp)
            start_acp
            ;;
        --rag|rag)
            start_rag
            ;;
        --core|core)
            start_core
            ;;
        --status|status)
            show_status
            ;;
        --stop|stop)
            stop_all
            ;;
        --clean|clean)
            clean_all
            ;;
        --logs|logs)
            show_logs
            ;;
        --help|-h|help)
            echo "Usage: $0 [option]"
            echo ""
            echo "Options:"
            echo "  all, --all       Start all protocol servers (default)"
            echo "  mcp, --mcp       Start only MCP servers"
            echo "  lsp, --lsp       Start only LSP servers"
            echo "  acp, --acp       Start only ACP servers"
            echo "  rag, --rag       Start only RAG/Embedding servers"
            echo "  core, --core     Start core services only"
            echo "  status, --status Show status of all servers"
            echo "  stop, --stop     Stop all protocol servers"
            echo "  clean, --clean   Stop and remove all volumes"
            echo "  logs, --logs     Show logs"
            echo "  help, --help     Show this help"
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
}

main "$@"
