#!/bin/bash
# ============================================================================
# HELIXAGENT LIVE MONITORING LOOP
# ============================================================================
# This script monitors all HelixAgent services and collects warnings/errors
# Run with: ./scripts/live-monitoring.sh
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

# Log files
LOG_DIR="$PROJECT_ROOT/logs/monitoring"
mkdir -p "$LOG_DIR"
ERROR_LOG="$LOG_DIR/errors_$(date +%Y%m%d_%H%M%S).log"
WARNING_LOG="$LOG_DIR/warnings_$(date +%Y%m%d_%H%M%S).log"
STATUS_LOG="$LOG_DIR/status_$(date +%Y%m%d_%H%M%S).log"

# Initialize logs
echo "=== HelixAgent Monitoring Session Started: $(date) ===" | tee "$ERROR_LOG" "$WARNING_LOG" "$STATUS_LOG"

# Check TCP port
check_tcp() {
    local host="$1"
    local port="$2"
    timeout 2 bash -c "exec 3<>/dev/tcp/$host/$port" 2>/dev/null && return 0 || return 1
}

# Check HTTP endpoint
check_http() {
    local url="$1"
    curl -sf "$url" >/dev/null 2>&1 && return 0 || return 1
}

# Log error
log_error() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1"
    echo -e "${RED}$msg${NC}"
    echo "$msg" >> "$ERROR_LOG"
}

# Log warning
log_warning() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $1"
    echo -e "${YELLOW}$msg${NC}"
    echo "$msg" >> "$WARNING_LOG"
}

# Log success
log_success() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] OK: $1"
    echo -e "${GREEN}$msg${NC}"
}

# Log status
log_status() {
    echo "$1" >> "$STATUS_LOG"
}

# Check all services
check_all_services() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local errors=0
    local warnings=0

    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║     LIVE MONITORING CHECK - $timestamp      ║"
    echo "╚════════════════════════════════════════════════════════════════╝"

    # Core Services
    echo ""
    echo "=== CORE SERVICES ==="

    if check_tcp "localhost" 15432; then
        log_success "PostgreSQL (15432)"
    else
        log_error "PostgreSQL (15432) - DOWN"
        ((errors++))
    fi

    if check_tcp "localhost" 16379; then
        log_success "Redis (16379)"
    else
        log_error "Redis (16379) - DOWN"
        ((errors++))
    fi

    if check_http "http://localhost:8001/api/v2/heartbeat"; then
        log_success "ChromaDB (8001)"
    else
        log_error "ChromaDB (8001) - DOWN"
        ((errors++))
    fi

    if check_http "http://localhost:8000/"; then
        log_success "Cognee (8000)"
    else
        log_warning "Cognee (8000) - Not responding"
        ((warnings++))
    fi

    # HelixAgent API
    echo ""
    echo "=== HELIXAGENT API ==="

    if check_http "http://localhost:8100/health"; then
        log_success "HelixAgent Health (8100)"
    else
        log_error "HelixAgent Health (8100) - DOWN"
        ((errors++))
    fi

    if check_http "http://localhost:8100/v1/acp/health"; then
        log_success "ACP Protocol"
    else
        log_warning "ACP Protocol - Not responding"
        ((warnings++))
    fi

    if check_http "http://localhost:8100/v1/vision/health"; then
        log_success "Vision Protocol"
    else
        log_warning "Vision Protocol - Not responding"
        ((warnings++))
    fi

    # MCP Servers
    echo ""
    echo "=== MCP SERVERS ==="

    for port in 9101 9102 9103 9104 9105 9106 9107; do
        if check_tcp "localhost" "$port"; then
            log_success "MCP Server ($port)"
        else
            log_warning "MCP Server ($port) - Not responding"
            ((warnings++))
        fi
    done

    # Monitoring Stack
    echo ""
    echo "=== MONITORING STACK ==="

    if check_http "http://localhost:9090/-/healthy"; then
        log_success "Prometheus (9090)"
    else
        log_warning "Prometheus (9090) - Not responding"
        ((warnings++))
    fi

    if check_http "http://localhost:3000/api/health"; then
        log_success "Grafana (3000)"
    else
        log_warning "Grafana (3000) - Not responding"
        ((warnings++))
    fi

    if check_http "http://localhost:9093/-/healthy"; then
        log_success "Alertmanager (9093)"
    else
        log_warning "Alertmanager (9093) - Not responding"
        ((warnings++))
    fi

    if check_http "http://localhost:3100/ready"; then
        log_success "Loki (3100)"
    else
        log_warning "Loki (3100) - Not responding"
        ((warnings++))
    fi

    # Summary
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    log_status "[$timestamp] Errors: $errors, Warnings: $warnings"

    if [ $errors -gt 0 ]; then
        echo -e "${RED}ERRORS: $errors${NC}"
    fi
    if [ $warnings -gt 0 ]; then
        echo -e "${YELLOW}WARNINGS: $warnings${NC}"
    fi
    if [ $errors -eq 0 ] && [ $warnings -eq 0 ]; then
        echo -e "${GREEN}ALL SYSTEMS OPERATIONAL${NC}"
    fi
    echo "════════════════════════════════════════════════════════════════"

    return $errors
}

# Collect container logs for errors
collect_container_errors() {
    echo ""
    echo "=== Collecting Container Errors ==="

    local containers=$(podman ps --format '{{.Names}}' 2>/dev/null)

    for container in $containers; do
        local errors=$(podman logs --tail 50 "$container" 2>&1 | grep -i "error\|fail\|exception" | tail -5)
        if [ -n "$errors" ]; then
            echo "[$container]:" >> "$ERROR_LOG"
            echo "$errors" >> "$ERROR_LOG"
            echo "" >> "$ERROR_LOG"
            log_warning "Errors found in $container logs - see $ERROR_LOG"
        fi
    done
}

# Check HelixAgent application logs
collect_helixagent_errors() {
    echo ""
    echo "=== Collecting HelixAgent Errors ==="

    # Check for recent errors in temp log if exists
    if [ -f "/tmp/helix3.log" ]; then
        local errors=$(tail -100 /tmp/helix3.log 2>/dev/null | grep -i "error\|fail\|panic" | tail -10)
        if [ -n "$errors" ]; then
            echo "[HelixAgent Application]:" >> "$ERROR_LOG"
            echo "$errors" >> "$ERROR_LOG"
            echo "" >> "$ERROR_LOG"
            log_warning "Errors found in HelixAgent logs - see $ERROR_LOG"
        fi
    fi
}

# Prometheus alerts check
check_prometheus_alerts() {
    echo ""
    echo "=== Checking Prometheus Alerts ==="

    local alerts=$(curl -sf "http://localhost:9090/api/v1/alerts" 2>/dev/null | jq -r '.data.alerts[]? | "\(.labels.alertname): \(.labels.severity)"' 2>/dev/null)

    if [ -n "$alerts" ]; then
        echo "[Prometheus Alerts]:" >> "$ERROR_LOG"
        echo "$alerts" >> "$ERROR_LOG"
        log_warning "Active Prometheus alerts found"
    else
        log_success "No active Prometheus alerts"
    fi
}

# Main monitoring loop
main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║           HELIXAGENT LIVE MONITORING STARTED                   ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "Log files:"
    echo "  Errors:   $ERROR_LOG"
    echo "  Warnings: $WARNING_LOG"
    echo "  Status:   $STATUS_LOG"
    echo ""
    echo "Press Ctrl+C to stop monitoring"
    echo ""

    # Initial check
    check_all_services
    collect_container_errors
    collect_helixagent_errors
    check_prometheus_alerts

    # Loop every 30 seconds
    while true; do
        sleep 30
        check_all_services

        # Collect logs every 5 minutes
        if [ $(($(date +%s) % 300)) -lt 30 ]; then
            collect_container_errors
            collect_helixagent_errors
            check_prometheus_alerts
        fi
    done
}

# Run if executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
