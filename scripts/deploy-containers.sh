#!/bin/bash
# Unified Container Deployment Script
# Respects containers/.env configuration for local vs remote deployment
#
# This script is the SINGLE source of truth for container deployment.
# It MUST be used by all test infrastructure, CI/CD, and manual operations.
#
# Usage:
#   ./scripts/deploy-containers.sh [compose-file] [services...]
#
# Examples:
#   ./scripts/deploy-containers.sh                    # Deploy all default services
#   ./scripts/deploy-containers.sh docker-compose.test.yml postgres redis

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONTAINERS_ENV="$PROJECT_ROOT/containers/.env"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Parse containers/.env file
parse_containers_env() {
    if [[ ! -f "$CONTAINERS_ENV" ]]; then
        log_warn "containers/.env not found, defaulting to local deployment"
        REMOTE_ENABLED=false
        return
    fi

    REMOTE_ENABLED=false
    declare -gA REMOTE_HOSTS=()

    while IFS='=' read -r key value; do
        # Skip comments and empty lines
        [[ "$key" =~ ^#.*$ ]] && continue
        [[ -z "$key" ]] && continue

        # Remove quotes from value
        value="${value%\"}"
        value="${value#\"}"
        value="${value%\'}"
        value="${value#\'}"

        case "$key" in
            CONTAINERS_REMOTE_ENABLED)
                if [[ "${value,,}" == "true" ]]; then
                    REMOTE_ENABLED=true
                fi
                ;;
            CONTAINERS_REMOTE_HOST_*_NAME)
                HOST_NUM=$(echo "$key" | sed 's/CONTAINERS_REMOTE_HOST_\([0-9]*\)_NAME/\1/')
                REMOTE_HOSTS["${HOST_NUM}_name"]="$value"
                ;;
            CONTAINERS_REMOTE_HOST_*_ADDRESS)
                HOST_NUM=$(echo "$key" | sed 's/CONTAINERS_REMOTE_HOST_\([0-9]*\)_ADDRESS/\1/')
                REMOTE_HOSTS["${HOST_NUM}_address"]="$value"
                ;;
            CONTAINERS_REMOTE_HOST_*_PORT)
                HOST_NUM=$(echo "$key" | sed 's/CONTAINERS_REMOTE_HOST_\([0-9]*\)_PORT/\1/')
                REMOTE_HOSTS["${HOST_NUM}_port"]="$value"
                ;;
            CONTAINERS_REMOTE_HOST_*_USER)
                HOST_NUM=$(echo "$key" | sed 's/CONTAINERS_REMOTE_HOST_\([0-9]*\)_USER/\1/')
                REMOTE_HOSTS["${HOST_NUM}_user"]="$value"
                ;;
            CONTAINERS_REMOTE_HOST_*_RUNTIME)
                HOST_NUM=$(echo "$key" | sed 's/CONTAINERS_REMOTE_HOST_\([0-9]*\)_RUNTIME/\1/')
                REMOTE_HOSTS["${HOST_NUM}_runtime"]="$value"
                ;;
        esac
    done < "$CONTAINERS_ENV"
}

# Detect compose command
detect_compose_cmd() {
    if command -v podman-compose &> /dev/null; then
        echo "podman-compose"
    elif command -v docker &> /dev/null && docker compose version &> /dev/null 2>&1; then
        echo "docker compose"
    elif command -v docker-compose &> /dev/null; then
        echo "docker-compose"
    elif command -v podman &> /dev/null && podman compose version &> /dev/null 2>&1; then
        echo "podman compose"
    else
        echo ""
    fi
}

# Deploy containers locally
deploy_local() {
    local compose_file="$1"
    shift
    local services="$@"

    local compose_cmd=$(detect_compose_cmd)
    if [[ -z "$compose_cmd" ]]; then
        log_error "No compose tool available"
        exit 1
    fi

    log_info "Deploying locally using: $compose_cmd"
    log_info "Compose file: $compose_file"
    log_info "Services: ${services:-all}"

    cd "$PROJECT_ROOT"
    if [[ -n "$services" ]]; then
        $compose_cmd -f "$compose_file" up -d $services
    else
        $compose_cmd -f "$compose_file" up -d
    fi

    log_success "Local deployment complete"
}

# Deploy containers to remote host
deploy_remote() {
    local compose_file="$1"
    shift
    local services="$@"

    log_info "═══════════════════════════════════════════════════════════════"
    log_info "REMOTE DEPLOYMENT ENABLED"
    log_info "═══════════════════════════════════════════════════════════════"

    # Get first remote host (for now, single host deployment)
    local host_address=""
    local host_user=""
    local host_runtime=""

    for key in "${!REMOTE_HOSTS[@]}"; do
        if [[ "$key" == *_address ]]; then
            host_address="${REMOTE_HOSTS[$key]}"
            host_num="${key%_address}"
            host_user="${REMOTE_HOSTS[${host_num}_user]:-$USER}"
            host_runtime="${REMOTE_HOSTS[${host_num}_runtime]:-podman}"
            break
        fi
    done

    if [[ -z "$host_address" ]]; then
        log_error "No remote host configured in containers/.env"
        exit 1
    fi

    log_info "Target host: $host_address"
    log_info "User: $host_user"
    log_info "Runtime: $host_runtime"

    # Verify SSH connectivity
    log_info "Verifying SSH connectivity to $host_address..."
    if ! ssh -o ConnectTimeout=5 -o BatchMode=yes "$host_user@$host_address" "echo ok" &> /dev/null; then
        log_error "Cannot connect to $host_address via SSH"
        log_error "Ensure SSH keys are configured and host is reachable"
        exit 1
    fi
    log_success "SSH connectivity verified"

    # Stop any local containers first (prevents conflicts)
    log_info "Stopping any local containers..."
    local compose_cmd=$(detect_compose_cmd)
    if [[ -n "$compose_cmd" ]]; then
        $compose_cmd -f "$compose_file" down --remove-orphans 2>/dev/null || true
    fi
    log_success "Local containers stopped"

    # Copy compose file to remote host
    local remote_dir="helixagent/deploy"
    log_info "Ensuring remote directory exists: $remote_dir"
    ssh "$host_user@$host_address" "mkdir -p $remote_dir"

    log_info "Copying compose file to remote host..."
    scp "$PROJECT_ROOT/$compose_file" "$host_user@$host_address:$remote_dir/"

    # Detect remote compose command
    local remote_compose_cmd=""
    if [[ "$host_runtime" == "podman" ]]; then
        if ssh "$host_user@$host_address" "command -v podman-compose" &> /dev/null; then
            remote_compose_cmd="podman-compose"
        else
            remote_compose_cmd="podman compose"
        fi
    else
        if ssh "$host_user@$host_address" "docker compose version" &> /dev/null 2>&1; then
            remote_compose_cmd="docker compose"
        else
            remote_compose_cmd="docker-compose"
        fi
    fi

    log_info "Remote compose command: $remote_compose_cmd"

    # Deploy on remote host
    log_info "Deploying containers on remote host..."
    local compose_basename=$(basename "$compose_file")
    if [[ -n "$services" ]]; then
        ssh "$host_user@$host_address" "cd $remote_dir && $remote_compose_cmd -f $compose_basename up -d $services"
    else
        ssh "$host_user@$host_address" "cd $remote_dir && $remote_compose_cmd -f $compose_basename up -d"
    fi

    log_success "Remote deployment complete"

    # Show running containers
    log_info "Containers running on $host_address:"
    ssh "$host_user@$host_address" "$host_runtime ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'"
}

# Wait for services to be healthy
wait_for_services() {
    log_info "Waiting for services to be ready..."

    # Check if we should check remote or local
    if [[ "$REMOTE_ENABLED" == "true" ]]; then
        # Get remote host info
        local host_address=""
        local host_user=""
        for key in "${!REMOTE_HOSTS[@]}"; do
            if [[ "$key" == *_address ]]; then
                host_address="${REMOTE_HOSTS[$key]}"
                host_num="${key%_address}"
                host_user="${REMOTE_HOSTS[${host_num}_user]:-$USER}"
                break
            fi
        done

        log_info "Checking services on remote host: $host_address"

        # Check PostgreSQL
        if ssh "$host_user@$host_address" "nc -z localhost 5432" 2>/dev/null; then
            log_success "PostgreSQL is ready (remote)"
        else
            log_warn "PostgreSQL may not be ready on remote host"
        fi

        # Check Redis
        if ssh "$host_user@$host_address" "nc -z localhost 6379" 2>/dev/null; then
            log_success "Redis is ready (remote)"
        else
            log_warn "Redis may not be ready on remote host"
        fi
    else
        # Local checks
        sleep 5

        log_info "Checking PostgreSQL..."
        for i in {1..30}; do
            if nc -z localhost 15432 2>/dev/null || nc -z localhost 5432 2>/dev/null; then
                log_success "PostgreSQL is ready"
                break
            fi
            sleep 1
        done

        log_info "Checking Redis..."
        for i in {1..30}; do
            if nc -z localhost 16379 2>/dev/null || nc -z localhost 6379 2>/dev/null; then
                log_success "Redis is ready"
                break
            fi
            sleep 1
        done
    fi

    log_success "All services ready"
}

# Show deployment status
show_status() {
    echo ""
    log_info "═══════════════════════════════════════════════════════════════"
    log_info "DEPLOYMENT STATUS"
    log_info "═══════════════════════════════════════════════════════════════"

    if [[ "$REMOTE_ENABLED" == "true" ]]; then
        log_info "Mode: REMOTE"
        for key in "${!REMOTE_HOSTS[@]}"; do
            if [[ "$key" == *_name ]]; then
                host_num="${key%_name}"
                log_info "  Host: ${REMOTE_HOSTS[$key]} (${REMOTE_HOSTS[${host_num}_address]})"
            fi
        done
    else
        log_info "Mode: LOCAL"
    fi

    echo ""
    log_info "Services available at:"

    if [[ "$REMOTE_ENABLED" == "true" ]]; then
        local host_address=""
        for key in "${!REMOTE_HOSTS[@]}"; do
            if [[ "$key" == *_address ]]; then
                host_address="${REMOTE_HOSTS[$key]}"
                break
            fi
        done
        log_info "  PostgreSQL: $host_address:5432"
        log_info "  Redis:      $host_address:6379"
        log_info "  Mock LLM:   http://$host_address:8090"
        log_warn ""
        log_warn "NOTE: For local tests, use SSH tunnel or configure services for remote access"
        log_warn "      ssh -L 15432:localhost:5432 -L 16379:localhost:6379 $host_address"
    else
        log_info "  PostgreSQL: localhost:15432 (helixagent/helixagent123)"
        log_info "  Redis:      localhost:16379 (password: helixagent123)"
        log_info "  Mock LLM:   http://localhost:18081"
    fi
}

# Main entry point
main() {
    local compose_file="${1:-docker-compose.test.yml}"
    shift || true
    local services="$@"

    log_info "═══════════════════════════════════════════════════════════════"
    log_info "UNIFIED CONTAINER DEPLOYMENT"
    log_info "═══════════════════════════════════════════════════════════════"

    # Parse configuration
    parse_containers_env

    log_info "Configuration file: $CONTAINERS_ENV"
    log_info "Remote enabled: $REMOTE_ENABLED"
    log_info "Compose file: $compose_file"
    log_info "Services: ${services:-<all>}"

    # Deploy based on configuration
    if [[ "$REMOTE_ENABLED" == "true" ]]; then
        deploy_remote "$compose_file" $services
    else
        deploy_local "$compose_file" $services
    fi

    # Wait for services
    wait_for_services

    # Show status
    show_status
}

main "$@"
