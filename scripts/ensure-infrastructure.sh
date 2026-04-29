#!/bin/bash
# ============================================================================
# HELIXAGENT INFRASTRUCTURE AUTO-BOOT SYSTEM
# ============================================================================
# This script ensures ALL infrastructure is running before any operation.
# Called automatically by: HelixAgent startup, tests, challenges
# ============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
MAX_WAIT_TIME=${MAX_WAIT_TIME:-120}
HEALTH_CHECK_INTERVAL=2

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# ============================================================================
# CONTAINER RUNTIME DETECTION
# ============================================================================

detect_runtime() {
    if command -v podman &> /dev/null && podman info &> /dev/null 2>&1; then
        RUNTIME="podman"
        if command -v podman-compose &> /dev/null; then
            COMPOSE="podman-compose"
        else
            COMPOSE="podman compose"
        fi
    elif command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then
        RUNTIME="docker"
        if docker compose version &> /dev/null 2>&1; then
            COMPOSE="docker compose"
        else
            COMPOSE="docker-compose"
        fi
    else
        log_error "No container runtime found (Docker or Podman required)"
        exit 1
    fi
    log_info "Using runtime: $RUNTIME, compose: $COMPOSE"
}

# ============================================================================
# NETWORK AND VOLUME SETUP
# ============================================================================

ensure_network() {
    log_info "Ensuring network helixagent-network exists..."
    if [ "$RUNTIME" = "podman" ]; then
        podman network exists helixagent-network 2>/dev/null || podman network create helixagent-network
    else
        docker network inspect helixagent-network &>/dev/null || docker network create helixagent-network
    fi
}

ensure_volumes() {
    log_info "Ensuring required volumes exist..."
    local volumes=(
        "mcp_workspace"
        "lsp_cache"
        "go_cache"
        "cargo_cache"
        "pip_cache"
        "npm_cache"
        "maven_cache"
        "postgres_data"
        "redis_data"
        "chromadb_data"
        "cognee_data"
        "cognee_models"
    )

    for vol in "${volumes[@]}"; do
        if [ "$RUNTIME" = "podman" ]; then
            podman volume exists "$vol" 2>/dev/null || podman volume create "$vol" >/dev/null
        else
            docker volume inspect "$vol" &>/dev/null || docker volume create "$vol" >/dev/null
        fi
    done
}

# ============================================================================
# SERVICE HEALTH CHECKS
# ============================================================================

wait_for_http() {
    local name="$1"
    local url="$2"
    local max_wait="${3:-60}"
    local start_time=$(date +%s)

    while true; do
        if curl -sf "$url" >/dev/null 2>&1; then
            return 0
        fi

        local elapsed=$(($(date +%s) - start_time))
        if [ $elapsed -ge $max_wait ]; then
            return 1
        fi
        sleep $HEALTH_CHECK_INTERVAL
    done
}

wait_for_tcp() {
    local name="$1"
    local host="$2"
    local port="$3"
    local max_wait="${4:-60}"
    local start_time=$(date +%s)

    while true; do
        if check_tcp_port "$host" "$port"; then
            return 0
        fi

        local elapsed=$(($(date +%s) - start_time))
        if [ $elapsed -ge $max_wait ]; then
            return 1
        fi
        sleep $HEALTH_CHECK_INTERVAL
    done
}

check_tcp_port() {
    local host="$1"
    local port="$2"
    # Use multiple methods to check port connectivity
    if command -v nc &>/dev/null; then
        nc -z -w2 "$host" "$port" 2>/dev/null && return 0
    fi
    # Fallback to bash /dev/tcp
    (timeout 2 bash -c "exec 3<>/dev/tcp/$host/$port" 2>/dev/null) && return 0
    # Fallback to timeout with cat
    (timeout 2 cat < /dev/tcp/$host/$port >/dev/null 2>&1) && return 0
    return 1
}

check_postgres() {
    # Use TCP check as primary method (works without pg_isready)
    local port="${DB_PORT:-15432}"
    if check_tcp_port "localhost" "$port"; then
        # If pg_isready is available, use it for a more thorough check
        if command -v pg_isready &>/dev/null; then
            PGPASSWORD="${DB_PASSWORD:-helixagent123}" pg_isready -h localhost -p "$port" -U "${DB_USER:-helixagent}" >/dev/null 2>&1
            return $?
        fi
        return 0
    fi
    return 1
}

check_redis() {
    # Use TCP check as primary method (works without redis-cli)
    local port="${REDIS_PORT:-16379}"
    if check_tcp_port "localhost" "$port"; then
        # If redis-cli is available, use it for a more thorough check
        if command -v redis-cli &>/dev/null; then
            redis-cli -h localhost -p "$port" -a "${REDIS_PASSWORD:-helixagent123}" --no-auth-warning ping 2>/dev/null | grep -q "PONG"
            return $?
        fi
        return 0
    fi
    return 1
}

# ============================================================================
# SERVICE STARTUP FUNCTIONS
# ============================================================================

start_core_services() {
    log_info "Starting core services (postgres, redis, chromadb, cognee)..."
    cd "$PROJECT_ROOT"

    # Start with default profile
    $COMPOSE --profile default up -d postgres redis chromadb 2>/dev/null || \
    $COMPOSE up -d postgres redis chromadb 2>/dev/null || true

    # Wait for postgres
    log_info "Waiting for PostgreSQL..."
    local wait_start=$(date +%s)
    while ! check_postgres; do
        if [ $(($(date +%s) - wait_start)) -ge 60 ]; then
            log_warn "PostgreSQL not ready after 60s"
            break
        fi
        sleep 2
    done

    # Wait for redis
    log_info "Waiting for Redis..."
    wait_start=$(date +%s)
    while ! check_redis; do
        if [ $(($(date +%s) - wait_start)) -ge 30 ]; then
            log_warn "Redis not ready after 30s"
            break
        fi
        sleep 2
    done

    # Wait for ChromaDB
    log_info "Waiting for ChromaDB..."
    wait_for_http "ChromaDB" "http://localhost:8001/api/v2/heartbeat" 60 || log_warn "ChromaDB not ready"

    # Start Cognee
    $COMPOSE --profile default up -d cognee 2>/dev/null || \
    $COMPOSE --profile ai up -d cognee 2>/dev/null || true

    log_info "Waiting for Cognee..."
    wait_for_http "Cognee" "http://localhost:8000/" 90 || log_warn "Cognee not ready"

    log_success "Core services started"
}

start_mcp_servers() {
    log_info "Starting MCP servers..."
    cd "$PROJECT_ROOT"

    # Check for MCP compose file
    local mcp_compose="docker/mcp/docker-compose.mcp-servers.yml"
    if [ -f "$mcp_compose" ]; then
        $COMPOSE -f "$mcp_compose" up -d 2>/dev/null || true
    fi

    # Also try the main MCP compose
    local mcp_main="docker/mcp/docker-compose.mcp.yml"
    if [ -f "$mcp_main" ]; then
        $COMPOSE -f "$mcp_main" up -d 2>/dev/null || true
    fi

    # Wait for key MCP servers
    local mcp_ports=(9101 9102 9103 9104 9105 9106 9107)
    for port in "${mcp_ports[@]}"; do
        wait_for_tcp "MCP $port" "localhost" "$port" 10 || true
    done

    log_success "MCP servers started"
}

start_lsp_servers() {
    log_info "Starting LSP servers..."
    cd "$PROJECT_ROOT"

    local lsp_compose="docker/lsp/docker-compose.lsp.yml"
    if [ -f "$lsp_compose" ]; then
        $COMPOSE -f "$lsp_compose" --profile lsp up -d 2>/dev/null || \
        $COMPOSE -f "$lsp_compose" up -d 2>/dev/null || true
    fi

    # Wait for LSP manager
    wait_for_http "LSP Manager" "http://localhost:5100/health" 30 || log_warn "LSP Manager not ready"

    log_success "LSP servers started"
}

start_rag_services() {
    log_info "Starting RAG services..."
    cd "$PROJECT_ROOT"

    local rag_compose="docker/rag/docker-compose.rag.yml"
    if [ -f "$rag_compose" ]; then
        $COMPOSE -f "$rag_compose" --profile rag up -d 2>/dev/null || \
        $COMPOSE -f "$rag_compose" up -d 2>/dev/null || true
    fi

    # Wait for Qdrant
    wait_for_http "Qdrant" "http://localhost:6333/readyz" 60 || log_warn "Qdrant not ready"

    # Wait for RAG Manager
    wait_for_http "RAG Manager" "http://localhost:8030/health" 30 || log_warn "RAG Manager not ready"

    log_success "RAG services started"
}

# ============================================================================
# MAIN FUNCTIONS
# ============================================================================

start_all() {
    log_info "Starting ALL HelixAgent infrastructure..."

    detect_runtime
    ensure_network
    ensure_volumes

    # Start services in parallel where possible
    start_core_services

    # Start protocol services in background
    start_mcp_servers &
    MCP_PID=$!

    start_lsp_servers &
    LSP_PID=$!

    start_rag_services &
    RAG_PID=$!

    # Wait for all background jobs
    wait $MCP_PID 2>/dev/null || true
    wait $LSP_PID 2>/dev/null || true
    wait $RAG_PID 2>/dev/null || true

    log_success "All infrastructure started"
}

check_status() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║           INFRASTRUCTURE STATUS                                ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""

    # Core services
    echo "=== CORE SERVICES ==="
    check_postgres && echo -e "  ${GREEN}✓${NC} PostgreSQL" || echo -e "  ${RED}✗${NC} PostgreSQL"
    check_redis && echo -e "  ${GREEN}✓${NC} Redis" || echo -e "  ${RED}✗${NC} Redis"
    curl -sf "http://localhost:8001/api/v2/heartbeat" >/dev/null && echo -e "  ${GREEN}✓${NC} ChromaDB" || echo -e "  ${RED}✗${NC} ChromaDB"
    curl -sf "http://localhost:8000/" >/dev/null && echo -e "  ${GREEN}✓${NC} Cognee" || echo -e "  ${RED}✗${NC} Cognee"

    # MCP servers
    echo ""
    echo "=== MCP SERVERS ==="
    local mcp_names=("filesystem" "memory" "postgres" "puppeteer" "sequential-thinking" "everything" "github")
    local mcp_ports=(9101 9102 9103 9104 9105 9106 9107)
    for i in "${!mcp_ports[@]}"; do
        check_tcp_port "localhost" "${mcp_ports[$i]}" && \
            echo -e "  ${GREEN}✓${NC} MCP ${mcp_names[$i]} (${mcp_ports[$i]})" || \
            echo -e "  ${YELLOW}○${NC} MCP ${mcp_names[$i]} (${mcp_ports[$i]})"
    done

    # LSP servers
    echo ""
    echo "=== LSP SERVERS ==="
    curl -sf "http://localhost:5100/health" >/dev/null && echo -e "  ${GREEN}✓${NC} LSP Manager" || echo -e "  ${YELLOW}○${NC} LSP Manager"

    # RAG services
    echo ""
    echo "=== RAG SERVICES ==="
    curl -sf "http://localhost:6333/readyz" >/dev/null && echo -e "  ${GREEN}✓${NC} Qdrant" || echo -e "  ${YELLOW}○${NC} Qdrant"
    curl -sf "http://localhost:8030/health" >/dev/null && echo -e "  ${GREEN}✓${NC} RAG Manager" || echo -e "  ${YELLOW}○${NC} RAG Manager"

    # HelixAgent
    echo ""
    echo "=== HELIXAGENT ==="
    curl -sf "http://localhost:8100/health" >/dev/null && echo -e "  ${GREEN}✓${NC} HelixAgent API" || echo -e "  ${YELLOW}○${NC} HelixAgent API"
    curl -sf "http://localhost:8100/v1/acp/health" >/dev/null && echo -e "  ${GREEN}✓${NC} ACP Protocol" || echo -e "  ${YELLOW}○${NC} ACP Protocol"
    curl -sf "http://localhost:8100/v1/vision/health" >/dev/null && echo -e "  ${GREEN}✓${NC} Vision Protocol" || echo -e "  ${YELLOW}○${NC} Vision Protocol"

    echo ""
}

stop_all() {
    log_info "Stopping all infrastructure..."
    cd "$PROJECT_ROOT"

    detect_runtime

    # Stop all compose services
    $COMPOSE down 2>/dev/null || true

    for compose_file in docker/*/docker-compose*.yml; do
        [ -f "$compose_file" ] && $COMPOSE -f "$compose_file" down 2>/dev/null || true
    done

    log_success "All infrastructure stopped"
}

# ============================================================================
# ENTRY POINT
# ============================================================================

case "${1:-start}" in
    start|up)
        start_all
        check_status
        ;;
    stop|down)
        stop_all
        ;;
    restart)
        stop_all
        sleep 3
        start_all
        check_status
        ;;
    status|check)
        detect_runtime
        check_status
        ;;
    core)
        detect_runtime
        ensure_network
        ensure_volumes
        start_core_services
        ;;
    mcp)
        detect_runtime
        start_mcp_servers
        ;;
    lsp)
        detect_runtime
        start_lsp_servers
        ;;
    rag)
        detect_runtime
        start_rag_services
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|core|mcp|lsp|rag}"
        exit 1
        ;;
esac
