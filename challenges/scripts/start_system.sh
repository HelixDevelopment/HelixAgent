#!/bin/bash
#===============================================================================
# SUPERAGENT SYSTEM STARTER
#===============================================================================
# This script starts all infrastructure and the SuperAgent system.
# Uses ONLY production binaries and Docker/Podman - NO source code execution!
#
# Usage:
#   ./scripts/start_system.sh [options]
#
# Options:
#   --with-monitoring    Include Prometheus/Grafana
#   --with-ai            Include Ollama for local AI
#   --full               Start all services
#   --help               Show this help
#
#===============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Options
WITH_MONITORING=false
WITH_AI=false
FULL=false

usage() {
    cat << EOF
${GREEN}SuperAgent System Starter${NC}

Usage: $0 [options]

Options:
    --with-monitoring    Include Prometheus/Grafana
    --with-ai            Include Ollama for local AI
    --full               Start all services
    --help               Show this help

This script uses ONLY production binaries and Docker/Podman.
NO source code is executed - only built artifacts!
EOF
}

detect_container_runtime() {
    if command -v docker &> /dev/null && docker ps &> /dev/null 2>&1; then
        echo "docker"
    elif command -v podman &> /dev/null; then
        echo "podman"
    else
        echo "none"
    fi
}

get_compose_command() {
    local runtime="$1"
    if [ "$runtime" = "docker" ]; then
        echo "docker-compose"
    else
        echo "podman-compose"
    fi
}

# Parse arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --with-monitoring) WITH_MONITORING=true ;;
        --with-ai) WITH_AI=true ;;
        --full) FULL=true; WITH_MONITORING=true; WITH_AI=true ;;
        --help|-h) usage; exit 0 ;;
        *) log_error "Unknown option: $1"; usage; exit 1 ;;
    esac
    shift
done

log_info "=========================================="
log_info "  SuperAgent System Starter"
log_info "=========================================="
log_info ""

# Detect container runtime
RUNTIME=$(detect_container_runtime)
log_info "Container runtime: $RUNTIME"

if [ "$RUNTIME" = "none" ]; then
    log_error "No container runtime found!"
    log_error "Please install Docker or Podman"
    exit 1
fi

COMPOSE=$(get_compose_command "$RUNTIME")
log_info "Compose command: $COMPOSE"

# Load environment
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
    log_info "Loaded environment from $PROJECT_ROOT/.env"
fi

# Change to project root
cd "$PROJECT_ROOT"

# Build profile flags
PROFILE_FLAGS=""
if [ "$WITH_AI" = true ]; then
    PROFILE_FLAGS="$PROFILE_FLAGS --profile ai"
fi
if [ "$WITH_MONITORING" = true ]; then
    PROFILE_FLAGS="$PROFILE_FLAGS --profile monitoring"
fi
if [ "$FULL" = true ]; then
    PROFILE_FLAGS="--profile full"
fi

# Start infrastructure
log_info ""
log_info "Starting infrastructure services..."
log_info "Command: $COMPOSE $PROFILE_FLAGS up -d"
log_info ""

$COMPOSE $PROFILE_FLAGS up -d

# Wait for services to be ready
log_info ""
log_info "Waiting for services to be ready..."
sleep 5

# Check service status
log_info ""
log_info "Service status:"
$COMPOSE ps

# Check if SuperAgent binary exists and start it
SUPERAGENT_BIN="$PROJECT_ROOT/bin/superagent"
if [ ! -x "$SUPERAGENT_BIN" ]; then
    SUPERAGENT_BIN="$PROJECT_ROOT/superagent"
fi

if [ -x "$SUPERAGENT_BIN" ]; then
    log_info ""
    log_info "Starting SuperAgent binary..."
    log_info "Binary: $SUPERAGENT_BIN"

    # Start SuperAgent in background
    nohup $SUPERAGENT_BIN server > "$CHALLENGES_DIR/results/superagent.log" 2>&1 &
    SUPERAGENT_PID=$!
    echo $SUPERAGENT_PID > "$CHALLENGES_DIR/results/superagent.pid"
    log_success "SuperAgent started with PID: $SUPERAGENT_PID"
else
    log_warning "SuperAgent binary not found"
    log_warning "Run 'make build' from project root to build it"
fi

# Show access information
log_info ""
log_success "=========================================="
log_success "  System Started Successfully!"
log_success "=========================================="
log_info ""
log_info "Services available at:"
log_info "  SuperAgent API:     http://localhost:8080"
log_info "  PostgreSQL:         localhost:5432"
log_info "  Redis:              localhost:6379"

if [ "$WITH_MONITORING" = true ]; then
    log_info "  Prometheus:         http://localhost:9090"
    log_info "  Grafana:            http://localhost:3000"
fi

if [ "$WITH_AI" = true ]; then
    log_info "  Ollama:             http://localhost:11434"
fi

log_info ""
log_info "To stop the system:"
log_info "  ./challenges/scripts/stop_system.sh"
log_info ""
