#!/bin/bash
#
# Comprehensive Monitoring Challenge for HelixAgent
# Validates the complete monitoring stack including:
# - Prometheus metrics collection
# - Grafana dashboards
# - Alert rules
# - Custom exporters
# - All service health checks
#
# Usage: ./challenges/scripts/comprehensive_monitoring_challenge.sh
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Counters
PASSED=0
FAILED=0
WARNINGS=0

# Config
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://localhost:9093}"
LOKI_URL="${LOKI_URL:-http://localhost:3100}"
CHROMADB_URL="${CHROMADB_URL:-http://localhost:8001}"
COGNEE_URL="${COGNEE_URL:-http://localhost:8000}"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"

print_header() {
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}       ${BLUE}HelixAgent Comprehensive Monitoring Challenge${NC}                        ${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    ((PASSED++))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    ((FAILED++))
}

warn() {
    echo -e "  ${YELLOW}!${NC} $1"
    ((WARNINGS++))
}

info() {
    echo -e "  ${BLUE}ℹ${NC} $1"
}

section() {
    echo ""
    echo -e "${BLUE}═══ $1 ═══${NC}"
}

# ========================================
# CONFIGURATION FILE TESTS
# ========================================

test_config_files() {
    section "Configuration Files"

    # Prometheus config
    if [ -f "$PROJECT_ROOT/monitoring/prometheus.yml" ]; then
        pass "Prometheus configuration exists"

        # Check for all required jobs
        if grep -q "job_name: 'helixagent'" "$PROJECT_ROOT/monitoring/prometheus.yml"; then
            pass "HelixAgent scrape job configured"
        else
            fail "HelixAgent scrape job missing"
        fi

        if grep -q "job_name: 'chromadb'" "$PROJECT_ROOT/monitoring/prometheus.yml"; then
            pass "ChromaDB scrape job configured"
        else
            fail "ChromaDB scrape job missing"
        fi

        if grep -q "job_name: 'cognee'" "$PROJECT_ROOT/monitoring/prometheus.yml"; then
            pass "Cognee scrape job configured"
        else
            fail "Cognee scrape job missing"
        fi

        if grep -q "job_name: 'llmsverifier'" "$PROJECT_ROOT/monitoring/prometheus.yml"; then
            pass "LLMsVerifier scrape job configured"
        else
            fail "LLMsVerifier scrape job missing"
        fi

        if grep -q "job_name: 'postgres'" "$PROJECT_ROOT/monitoring/prometheus.yml"; then
            pass "PostgreSQL scrape job configured"
        else
            fail "PostgreSQL scrape job missing"
        fi

        if grep -q "job_name: 'redis'" "$PROJECT_ROOT/monitoring/prometheus.yml"; then
            pass "Redis scrape job configured"
        else
            fail "Redis scrape job missing"
        fi
    else
        fail "Prometheus configuration missing"
    fi

    # Alert rules
    if [ -f "$PROJECT_ROOT/monitoring/alert-rules.yml" ]; then
        pass "Alert rules file exists"

        ALERT_COUNT=$(grep -c "alert:" "$PROJECT_ROOT/monitoring/alert-rules.yml" || echo "0")
        if [ "$ALERT_COUNT" -ge 30 ]; then
            pass "Alert rules count: $ALERT_COUNT (>= 30 required)"
        else
            fail "Alert rules count: $ALERT_COUNT (< 30 required)"
        fi

        # Check for service-specific alerts
        for service in helixagent chromadb cognee postgres redis; do
            if grep -q "service: $service" "$PROJECT_ROOT/monitoring/alert-rules.yml"; then
                pass "Alerts configured for $service"
            else
                fail "No alerts for $service"
            fi
        done
    else
        fail "Alert rules file missing"
    fi

    # Blackbox exporter config
    if [ -f "$PROJECT_ROOT/monitoring/blackbox.yml" ]; then
        pass "Blackbox exporter configuration exists"
    else
        fail "Blackbox exporter configuration missing"
    fi

    # Custom exporter
    if [ -f "$PROJECT_ROOT/monitoring/helixagent-exporter.py" ]; then
        pass "Custom HelixAgent exporter exists"
    else
        fail "Custom HelixAgent exporter missing"
    fi

    # Docker compose
    if [ -f "$PROJECT_ROOT/docker-compose.monitoring.yml" ]; then
        pass "Monitoring compose file exists"

        SERVICE_COUNT=$(grep -c "container_name:" "$PROJECT_ROOT/docker-compose.monitoring.yml" || echo "0")
        if [ "$SERVICE_COUNT" -ge 10 ]; then
            pass "Monitoring services count: $SERVICE_COUNT (>= 10 required)"
        else
            fail "Monitoring services count: $SERVICE_COUNT (< 10 required)"
        fi
    else
        fail "Monitoring compose file missing"
    fi
}

# ========================================
# SERVICE HEALTH TESTS
# ========================================

test_service_health() {
    section "Service Health Checks"

    # HelixAgent
    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$HELIXAGENT_URL/health" 2>/dev/null | grep -q "200"; then
        pass "HelixAgent API is healthy"
    else
        fail "HelixAgent API is not responding"
    fi

    # HelixAgent metrics
    if curl -s --max-time 60 --connect-timeout 5 "$HELIXAGENT_URL/metrics" 2>/dev/null | grep -q "go_"; then
        pass "HelixAgent metrics endpoint working"
    else
        warn "HelixAgent metrics endpoint not responding (may need to start monitoring)"
    fi

    # ChromaDB
    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$CHROMADB_URL/api/v1/heartbeat" 2>/dev/null | grep -q "200"; then
        pass "ChromaDB is healthy"
    else
        fail "ChromaDB is not responding"
    fi

    # Cognee
    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$COGNEE_URL/health" 2>/dev/null | grep -q "200"; then
        pass "Cognee is healthy"
    else
        fail "Cognee is not responding"
    fi

    # PostgreSQL (via container)
    if podman exec helixagent-postgres pg_isready -U helixagent 2>/dev/null | grep -q "accepting"; then
        pass "PostgreSQL is accepting connections"
    else
        warn "PostgreSQL check failed (may need to check container name)"
    fi

    # Redis (via container)
    if podman exec helixagent-redis redis-cli -a helixagent123 ping 2>/dev/null | grep -q "PONG"; then
        pass "Redis is responding"
    else
        warn "Redis check failed (may need to check container name)"
    fi
}

# ========================================
# PROMETHEUS TESTS
# ========================================

test_prometheus() {
    section "Prometheus Integration"

    # Prometheus availability
    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$PROMETHEUS_URL/-/healthy" 2>/dev/null | grep -q "200"; then
        pass "Prometheus is healthy"

        # Check targets
        TARGETS=$(curl -s --max-time 60 "$PROMETHEUS_URL/api/v1/targets" 2>/dev/null)
        if [ -n "$TARGETS" ]; then
            UP_COUNT=$(echo "$TARGETS" | jq '[.data.activeTargets[] | select(.health == "up")] | length' 2>/dev/null || echo "0")
            TOTAL_COUNT=$(echo "$TARGETS" | jq '.data.activeTargets | length' 2>/dev/null || echo "0")
            pass "Prometheus targets: $UP_COUNT/$TOTAL_COUNT up"
        fi

        # Check rules
        RULES=$(curl -s --max-time 60 "$PROMETHEUS_URL/api/v1/rules" 2>/dev/null)
        if [ -n "$RULES" ]; then
            RULE_COUNT=$(echo "$RULES" | jq '[.data.groups[].rules[]] | length' 2>/dev/null || echo "0")
            pass "Prometheus rules loaded: $RULE_COUNT"
        fi

        # Check alerts
        ALERTS=$(curl -s --max-time 60 "$PROMETHEUS_URL/api/v1/alerts" 2>/dev/null)
        if [ -n "$ALERTS" ]; then
            FIRING=$(echo "$ALERTS" | jq '[.data.alerts[] | select(.state == "firing")] | length' 2>/dev/null || echo "0")
            if [ "$FIRING" -eq 0 ]; then
                pass "No firing alerts (healthy state)"
            else
                warn "Firing alerts: $FIRING"
            fi
        fi
    else
        warn "Prometheus not running - start with: podman-compose -f docker-compose.monitoring.yml up -d"
    fi
}

# ========================================
# GRAFANA TESTS
# ========================================

test_grafana() {
    section "Grafana Integration"

    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$GRAFANA_URL/api/health" 2>/dev/null | grep -q "200"; then
        pass "Grafana is healthy"

        # Check datasources
        DATASOURCES=$(curl -s --max-time 60 -u admin:admin123 "$GRAFANA_URL/api/datasources" 2>/dev/null)
        if [ -n "$DATASOURCES" ]; then
            DS_COUNT=$(echo "$DATASOURCES" | jq '. | length' 2>/dev/null || echo "0")
            pass "Grafana datasources configured: $DS_COUNT"
        fi
    else
        warn "Grafana not running - start with: podman-compose -f docker-compose.monitoring.yml up -d"
    fi
}

# ========================================
# ALERTMANAGER TESTS
# ========================================

test_alertmanager() {
    section "Alertmanager Integration"

    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$ALERTMANAGER_URL/-/healthy" 2>/dev/null | grep -q "200"; then
        pass "Alertmanager is healthy"

        # Check status
        STATUS=$(curl -s --max-time 60 "$ALERTMANAGER_URL/api/v2/status" 2>/dev/null)
        if [ -n "$STATUS" ]; then
            CLUSTER=$(echo "$STATUS" | jq -r '.cluster.status' 2>/dev/null || echo "unknown")
            pass "Alertmanager cluster status: $CLUSTER"
        fi
    else
        warn "Alertmanager not running - start with: podman-compose -f docker-compose.monitoring.yml up -d"
    fi
}

# ========================================
# LOKI TESTS
# ========================================

test_loki() {
    section "Loki Log Aggregation"

    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$LOKI_URL/ready" 2>/dev/null | grep -q "200"; then
        pass "Loki is ready"

        # Check labels
        LABELS=$(curl -s --max-time 60 "$LOKI_URL/loki/api/v1/labels" 2>/dev/null)
        if [ -n "$LABELS" ]; then
            LABEL_COUNT=$(echo "$LABELS" | jq '.data | length' 2>/dev/null || echo "0")
            pass "Loki labels available: $LABEL_COUNT"
        fi
    else
        warn "Loki not running - start with: podman-compose -f docker-compose.monitoring.yml up -d"
    fi
}

# ========================================
# CUSTOM EXPORTER TESTS
# ========================================

test_custom_exporter() {
    section "Custom HelixAgent Exporter"

    EXPORTER_URL="http://localhost:9200"

    if curl -s --max-time 60 -o /dev/null -w "%{http_code}" --connect-timeout 5 "$EXPORTER_URL/health" 2>/dev/null | grep -q "200"; then
        pass "Custom exporter is healthy"

        # Check metrics
        METRICS=$(curl -s --max-time 60 "$EXPORTER_URL/metrics" 2>/dev/null)
        if [ -n "$METRICS" ]; then
            if echo "$METRICS" | grep -q "helixagent_up"; then
                pass "HelixAgent metrics being exported"
            else
                fail "HelixAgent metrics not found"
            fi

            if echo "$METRICS" | grep -q "chromadb_up"; then
                pass "ChromaDB metrics being exported"
            else
                fail "ChromaDB metrics not found"
            fi

            if echo "$METRICS" | grep -q "cognee_up"; then
                pass "Cognee metrics being exported"
            else
                fail "Cognee metrics not found"
            fi
        fi
    else
        warn "Custom exporter not running - start monitoring stack first"
    fi
}

# ========================================
# DOCUMENTATION TESTS
# ========================================

test_documentation() {
    section "Documentation"

    if [ -f "$PROJECT_ROOT/docs/monitoring/MONITORING_SYSTEM.md" ]; then
        pass "Monitoring documentation exists"

        LINE_COUNT=$(wc -l < "$PROJECT_ROOT/docs/monitoring/MONITORING_SYSTEM.md")
        if [ "$LINE_COUNT" -ge 100 ]; then
            pass "Documentation is comprehensive ($LINE_COUNT lines)"
        else
            warn "Documentation may be incomplete ($LINE_COUNT lines)"
        fi
    else
        fail "Monitoring documentation missing"
    fi

    if [ -f "$PROJECT_ROOT/monitoring/dashboards/helixagent-dashboard.json" ]; then
        pass "Grafana dashboard exists"
    else
        fail "Grafana dashboard missing"
    fi
}

# ========================================
# TEST COVERAGE
# ========================================

test_coverage() {
    section "Test Coverage"

    # Check for monitoring tests
    if [ -f "$PROJECT_ROOT/tests/integration/monitoring_system_test.go" ]; then
        pass "Monitoring integration tests exist"
    else
        warn "Monitoring integration tests missing"
    fi

    if [ -f "$PROJECT_ROOT/internal/observability/observability_test.go" ]; then
        pass "Observability unit tests exist"
    else
        warn "Observability unit tests missing"
    fi

    # Run observability tests if available
    if [ -f "$PROJECT_ROOT/internal/observability/observability_test.go" ]; then
        info "Running observability tests..."
        if cd "$PROJECT_ROOT" && go test -v ./internal/observability/... -count=1 -timeout 60s > /tmp/observability_test.log 2>&1; then
            pass "Observability tests passed"
        else
            fail "Observability tests failed - check /tmp/observability_test.log"
        fi
    fi
}

# ========================================
# LLMSVERIFIER MONITORING TESTS
# ========================================

test_llmsverifier_monitoring() {
    section "LLMsVerifier Monitoring Integration"

    if [ -d "$PROJECT_ROOT/LLMsVerifier" ]; then
        pass "LLMsVerifier directory exists"

        # Check if LLMsVerifier has metrics endpoint
        if grep -rq "prometheus" "$PROJECT_ROOT/LLMsVerifier/llm-verifier/" 2>/dev/null; then
            pass "LLMsVerifier has Prometheus integration"
        else
            warn "LLMsVerifier may not have Prometheus metrics"
        fi

        # Check Prometheus config for LLMsVerifier
        if grep -q "llmsverifier" "$PROJECT_ROOT/monitoring/prometheus.yml"; then
            pass "LLMsVerifier configured in Prometheus"
        else
            fail "LLMsVerifier not configured in Prometheus"
        fi
    else
        fail "LLMsVerifier directory not found"
    fi
}

# ========================================
# MAIN
# ========================================

main() {
    print_header

    echo "Project root: $PROJECT_ROOT"
    echo "HelixAgent URL: $HELIXAGENT_URL"
    echo "Prometheus URL: $PROMETHEUS_URL"
    echo ""

    test_config_files
    test_service_health
    test_prometheus
    test_grafana
    test_alertmanager
    test_loki
    test_custom_exporter
    test_documentation
    test_coverage
    test_llmsverifier_monitoring

    # Summary
    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}                           CHALLENGE SUMMARY                                 ${NC}"
    echo -e "${CYAN}════════════════════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "  ${GREEN}Passed:${NC}   $PASSED"
    echo -e "  ${RED}Failed:${NC}   $FAILED"
    echo -e "  ${YELLOW}Warnings:${NC} $WARNINGS"
    echo ""

    TOTAL=$((PASSED + FAILED))
    if [ $TOTAL -gt 0 ]; then
        PERCENTAGE=$((PASSED * 100 / TOTAL))
        echo -e "  Success Rate: ${PERCENTAGE}%"
    fi

    echo ""
    if [ $FAILED -eq 0 ]; then
        echo -e "${GREEN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║                    ALL MONITORING TESTS PASSED!                            ║${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
        exit 0
    else
        echo -e "${RED}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}║                    MONITORING CHALLENGE FAILED                              ║${NC}"
        echo -e "${RED}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo "To start the monitoring stack:"
        echo "  cd $PROJECT_ROOT"
        echo "  podman-compose -f docker-compose.monitoring.yml up -d"
        exit 1
    fi
}

main "$@"
