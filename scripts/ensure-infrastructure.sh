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

# ============================================================================
# TEST-INFRASTRUCTURE CONTRACT (which compose file, which published ports)
# ============================================================================
# The core services health-checked below (PostgreSQL, Redis, mock LLM) belong
# to the TEST stack, and the ONLY compose file that publishes them on the host
# is docker-compose.test.yml:
#
#     postgres   ${POSTGRES_PORT:-15432} -> 5432
#     redis      ${REDIS_PORT:-16379}    -> 6379
#     mock-llm   ${MOCK_LLM_PORT:-18081} -> 8090
#
# These are exactly the ports tests/precondition/containers_boot_test.go
# requires (PostgreSQL 15432, Redis 16379, Mock LLM 18081 — all `required`).
#
# docker-compose.yml (the default/live stack) CANNOT serve them: its postgres
# runs with `network_mode: host` and publishes nothing on 15432, and its redis
# publishes ${REDIS_PORT:-8102}. Booting the default stack here therefore left
# the health checks below unsatisfiable by construction.
#
# ChromaDB and Cognee do NOT exist in docker-compose.test.yml, so they are
# still started from the default compose file — see start_core_services().
#
# The ports are resolved ONCE, here, and used BOTH to boot the stack and to
# health-check it, so publisher and checker can never disagree. They are passed
# to the test-compose invocation as command-scoped variables so that (a) a
# `.env` carrying the LIVE stack's ports (DB_PORT=8101 / REDIS_PORT=8102, per
# .env.example) cannot leak into the test stack — the shell environment wins
# over `.env` during compose interpolation — and (b) the default compose file's
# own port variables are left untouched for the live platform.
TEST_COMPOSE_FILE="${TEST_COMPOSE_FILE:-$PROJECT_ROOT/docker-compose.test.yml}"
INFRA_PG_PORT="${POSTGRES_PORT:-${DB_PORT:-15432}}"
INFRA_REDIS_PORT="${REDIS_PORT:-16379}"
INFRA_MOCK_LLM_PORT="${MOCK_LLM_PORT:-18081}"

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
    # Port comes from the single source of truth resolved above, so this checks
    # the port docker-compose.test.yml actually publishes.
    local port="$INFRA_PG_PORT"
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
    local port="$INFRA_REDIS_PORT"
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

check_mock_llm() {
    # The mock LLM server is a `required` service for the precondition suite;
    # it exposes /health on its published port.
    curl -sf "http://localhost:${INFRA_MOCK_LLM_PORT}/health" >/dev/null 2>&1
}

# ============================================================================
# SERVICE STARTUP FUNCTIONS
# ============================================================================

# Run a compose command, surfacing its output when it fails instead of
# discarding it. The previous `2>/dev/null || true` boots made a stack that
# never came up look identical to one that did.
compose_up() {
    local description="$1"
    shift
    local output
    if output=$("$@" 2>&1); then
        return 0
    fi
    log_error "$description failed:"
    printf '%s\n' "$output" | sed 's/^/    /'
    return 1
}

start_core_services() {
    log_info "Starting core services (postgres, redis, mock-llm, chromadb, cognee)..."
    cd "$PROJECT_ROOT"

    local core_failed=0

    log_info "Test-stack ports: postgres=$INFRA_PG_PORT redis=$INFRA_REDIS_PORT mock-llm=$INFRA_MOCK_LLM_PORT"

    # --- postgres + redis + mock-llm: docker-compose.test.yml ----------------
    # This is the only compose file that publishes the ports checked below.
    if [ ! -f "$TEST_COMPOSE_FILE" ]; then
        log_error "Test compose file not found: $TEST_COMPOSE_FILE"
        log_error "PostgreSQL/Redis/mock-llm cannot be published on the expected ports without it."
        core_failed=1
    elif ! compose_up "core test stack (postgres, redis, mock-llm)" \
        env POSTGRES_PORT="$INFRA_PG_PORT" \
            REDIS_PORT="$INFRA_REDIS_PORT" \
            MOCK_LLM_PORT="$INFRA_MOCK_LLM_PORT" \
        $COMPOSE -f "$TEST_COMPOSE_FILE" up -d postgres redis mock-llm; then
        # Most common cause: a container named helixagent-postgres/-redis is
        # already running from the DEFAULT compose file (both files pin the
        # same container_name), so the test stack cannot claim the name.
        log_error "Hint: run '$0 stop' first if the default stack is already up."
        core_failed=1
    fi

    # --- chromadb: default compose only -------------------------------------
    # docker-compose.test.yml has NO chromadb service, so naming it in a
    # `-f docker-compose.test.yml up` would fail the whole invocation. ChromaDB
    # lives in the default compose under the `default` profile with
    # `network_mode: host` and `--port 8001` — exactly the endpoint waited on
    # below. It is `required: false` for the precondition suite, so a failure
    # here warns rather than failing the boot.
    if ! compose_up "ChromaDB (default compose)" $COMPOSE --profile default up -d chromadb; then
        log_warn "ChromaDB did not start (optional service)"
    fi

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

    # Wait for the mock LLM server (required by the precondition suite)
    log_info "Waiting for Mock LLM server..."
    wait_for_http "Mock LLM" "http://localhost:${INFRA_MOCK_LLM_PORT}/health" 60 || \
        log_warn "Mock LLM not ready"

    # Wait for ChromaDB (port 8001 is hardcoded in the default compose file's
    # chromadb command: ["--host","0.0.0.0","--port","8001"])
    log_info "Waiting for ChromaDB..."
    wait_for_http "ChromaDB" "http://localhost:8001/api/v2/heartbeat" 60 || log_warn "ChromaDB not ready"

    # Start Cognee (default compose only — absent from docker-compose.test.yml)
    if ! { $COMPOSE --profile default up -d cognee || \
           $COMPOSE --profile ai up -d cognee; } >/dev/null 2>&1; then
        log_warn "Cognee did not start (optional service)"
    fi

    log_info "Waiting for Cognee..."
    wait_for_http "Cognee" "http://localhost:8000/" 90 || log_warn "Cognee not ready"

    # Honest verdict: report success only when every REQUIRED core service is
    # actually reachable on the port this script booted it on.
    if ! check_postgres; then
        log_error "PostgreSQL is NOT reachable on port $INFRA_PG_PORT"
        core_failed=1
    fi
    if ! check_redis; then
        log_error "Redis is NOT reachable on port $INFRA_REDIS_PORT"
        core_failed=1
    fi
    if ! check_mock_llm; then
        log_error "Mock LLM is NOT reachable on port $INFRA_MOCK_LLM_PORT"
        core_failed=1
    fi

    if [ "$core_failed" -ne 0 ]; then
        log_error "Core services did NOT all start"
        return 1
    fi

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

    # Start services in parallel where possible.
    # Capture (do not abort on) a core failure so the protocol services still
    # start and check_status can print the full picture; the failure is
    # re-surfaced as this function's exit code below.
    local core_failed=0
    start_core_services || core_failed=1

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

    if [ "$core_failed" -ne 0 ]; then
        log_error "Infrastructure start INCOMPLETE — core services failed (see above)"
        return 1
    fi

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
    check_postgres && echo -e "  ${GREEN}✓${NC} PostgreSQL (${INFRA_PG_PORT})" || echo -e "  ${RED}✗${NC} PostgreSQL (${INFRA_PG_PORT})"
    check_redis && echo -e "  ${GREEN}✓${NC} Redis (${INFRA_REDIS_PORT})" || echo -e "  ${RED}✗${NC} Redis (${INFRA_REDIS_PORT})"
    check_mock_llm && echo -e "  ${GREEN}✓${NC} Mock LLM (${INFRA_MOCK_LLM_PORT})" || echo -e "  ${RED}✗${NC} Mock LLM (${INFRA_MOCK_LLM_PORT})"
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

    # The core test stack (postgres/redis/mock-llm) is booted from
    # docker-compose.test.yml by start_core_services, so it must be torn down
    # from the same file — a plain `$COMPOSE down` does not reach it.
    if [ -f "$TEST_COMPOSE_FILE" ]; then
        $COMPOSE -f "$TEST_COMPOSE_FILE" down 2>/dev/null || true
    fi

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
        # Print status even when the boot failed, then exit with the real
        # verdict so callers (Makefile targets, challenge scripts) cannot
        # mistake a failed boot for a successful one.
        start_rc=0
        start_all || start_rc=$?
        check_status
        exit "$start_rc"
        ;;
    stop|down)
        stop_all
        ;;
    restart)
        stop_all
        sleep 3
        start_rc=0
        start_all || start_rc=$?
        check_status
        exit "$start_rc"
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
