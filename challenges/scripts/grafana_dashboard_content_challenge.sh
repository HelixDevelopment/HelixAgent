#!/bin/bash
# HelixAgent Challenge - Grafana Dashboard Content
# Validates that all required Grafana dashboard JSON files exist, are valid JSON,
# contain a "panels" array, and that supporting infrastructure configs are present.
#
# Tests: 12

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

record_result() {
    local name="$1" status="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$status" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "${GREEN}[PASS]${NC} $name"
    else
        FAILED=$((FAILED + 1))
        echo -e "${RED}[FAIL]${NC} $name"
    fi
}

echo "=========================================="
echo "  Grafana Dashboard Content Challenge"
echo "=========================================="
echo ""

DASHBOARDS_DIR="$PROJECT_ROOT/docker/monitoring/grafana/dashboards"
MONITORING_DIR="$PROJECT_ROOT/docker/monitoring"

# Required dashboard files
REQUIRED_DASHBOARDS=(
    "api-overview.json"
    "cache-performance.json"
    "ensemble-performance.json"
    "mcp-adapters.json"
    "provider-health.json"
    "resource-utilization.json"
    "security-status.json"
)

# --------------------------------------------------------------------------
# Test 1: All 7 dashboard JSON files exist
# --------------------------------------------------------------------------
ALL_EXIST=true
for dashboard in "${REQUIRED_DASHBOARDS[@]}"; do
    if [ ! -f "$DASHBOARDS_DIR/$dashboard" ]; then
        ALL_EXIST=false
        break
    fi
done
if [ "$ALL_EXIST" = true ]; then
    record_result "All 7 required dashboard JSON files exist" "PASS"
else
    record_result "All 7 required dashboard JSON files exist" "FAIL"
fi

# --------------------------------------------------------------------------
# Tests 2-8: Each dashboard is valid JSON
# --------------------------------------------------------------------------
for dashboard in "${REQUIRED_DASHBOARDS[@]}"; do
    FILEPATH="$DASHBOARDS_DIR/$dashboard"
    if [ -f "$FILEPATH" ]; then
        if python3 -c "import json,sys; json.load(open('$FILEPATH'))" 2>/dev/null; then
            record_result "$dashboard is valid JSON" "PASS"
        else
            record_result "$dashboard is valid JSON" "FAIL"
        fi
    else
        record_result "$dashboard is valid JSON (file missing)" "FAIL"
    fi
done

# --------------------------------------------------------------------------
# Test 9: All dashboards contain a "panels" array
# --------------------------------------------------------------------------
PANELS_COUNT=$(grep -l '"panels"' "$DASHBOARDS_DIR"/*.json 2>/dev/null | wc -l)
if [ "$PANELS_COUNT" -ge 7 ]; then
    record_result "All 7 dashboards contain \"panels\" array (found: $PANELS_COUNT)" "PASS"
else
    record_result "All 7 dashboards contain \"panels\" array (found: $PANELS_COUNT)" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 10: Prometheus datasource config (prometheus.yml) exists
# --------------------------------------------------------------------------
if [ -f "$MONITORING_DIR/grafana/datasources/prometheus.yml" ] || \
   [ -f "$MONITORING_DIR/prometheus.yml" ]; then
    record_result "Prometheus datasource config exists" "PASS"
else
    record_result "Prometheus datasource config exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 11: docker/monitoring/docker-compose.yml exists
# --------------------------------------------------------------------------
if [ -f "$MONITORING_DIR/docker-compose.yml" ]; then
    record_result "docker/monitoring/docker-compose.yml exists" "PASS"
else
    record_result "docker/monitoring/docker-compose.yml exists" "FAIL"
fi

# --------------------------------------------------------------------------
# Test 12: At least one dashboard references HelixAgent metric
# --------------------------------------------------------------------------
METRIC_COUNT=$(grep -r "helixagent\|helix_agent\|http_requests\|llm_provider\|ensemble" \
    "$DASHBOARDS_DIR"/*.json 2>/dev/null | wc -l)
if [ "$METRIC_COUNT" -ge 1 ]; then
    record_result "Dashboards reference HelixAgent metrics (found: $METRIC_COUNT matches)" "PASS"
else
    record_result "Dashboards reference HelixAgent metrics" "FAIL"
fi

echo ""
echo "=========================================="
echo "  Results: $PASSED/$TOTAL passed, $FAILED failed"
echo "=========================================="

[ $FAILED -eq 0 ] && exit 0 || exit 1
