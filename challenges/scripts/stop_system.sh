#!/bin/bash
#===============================================================================
# SUPERAGENT SYSTEM STOPPER
#===============================================================================
# This script stops all infrastructure and the SuperAgent system.
# Uses ONLY production binaries and Docker/Podman - NO source code execution!
#
# Usage:
#   ./scripts/stop_system.sh [options]
#
# Options:
#   --clean    Also remove volumes and data
#   --help     Show this help
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

CLEAN=false

usage() {
    cat << EOF
${GREEN}SuperAgent System Stopper${NC}

Usage: $0 [options]

Options:
    --clean    Also remove volumes and data
    --help     Show this help

This script gracefully stops all services.
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
        --clean) CLEAN=true ;;
        --help|-h) usage; exit 0 ;;
        *) log_error "Unknown option: $1"; usage; exit 1 ;;
    esac
    shift
done

log_info "=========================================="
log_info "  SuperAgent System Stopper"
log_info "=========================================="
log_info ""

# Stop SuperAgent binary if running
PID_FILE="$CHALLENGES_DIR/results/superagent.pid"
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        log_info "Stopping SuperAgent (PID: $PID)..."
        kill "$PID" 2>/dev/null || true
        sleep 2
        # Force kill if still running
        if kill -0 "$PID" 2>/dev/null; then
            kill -9 "$PID" 2>/dev/null || true
        fi
        log_success "SuperAgent stopped"
    fi
    rm -f "$PID_FILE"
fi

# Detect container runtime
RUNTIME=$(detect_container_runtime)
log_info "Container runtime: $RUNTIME"

if [ "$RUNTIME" = "none" ]; then
    log_warning "No container runtime found"
    log_info "SuperAgent binary stopped (if was running)"
    exit 0
fi

COMPOSE=$(get_compose_command "$RUNTIME")

# Change to project root
cd "$PROJECT_ROOT"

# Stop infrastructure
log_info ""
log_info "Stopping infrastructure services..."

if [ "$CLEAN" = true ]; then
    log_info "Command: $COMPOSE down -v"
    $COMPOSE down -v 2>/dev/null || true
else
    log_info "Command: $COMPOSE down"
    $COMPOSE down 2>/dev/null || true
fi

log_info ""
log_success "=========================================="
log_success "  System Stopped Successfully!"
log_success "=========================================="
log_info ""
log_info "To start the system again:"
log_info "  ./challenges/scripts/start_system.sh"
log_info ""
