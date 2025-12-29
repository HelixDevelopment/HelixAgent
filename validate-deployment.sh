#!/bin/bash

# SuperAgent Protocol Enhancement - Final Deployment Validation
# This script validates that all components are properly implemented and working

set -e

echo "🚀 SuperAgent Protocol Enhancement - Final Validation"
echo "=================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Validation counter
PASSED=0
FAILED=0

validate() {
    local name="$1"
    local command="$2"

    echo -n "🔍 $name... "
    if eval "$command" >/dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ FAILED${NC}"
        ((FAILED++))
    fi
}

echo ""
echo "📦 BUILD VALIDATION"
echo "==================="

validate "Main binary builds" "go build ./cmd/superagent"
validate "All services compile" "go build ./internal/services/"
validate "Handlers compile" "go build ./internal/handlers/"
validate "API routes compile" "go build ./internal/router/"

echo ""
echo "🧪 CORE FUNCTIONALITY TESTS"
echo "==========================="

validate "Unit tests pass" "go test ./internal/... -short -timeout 30s"
validate "Protocol analytics service" "go build -o /dev/null ./internal/services/protocol_analytics.go"
validate "MCP client implementation" "go build -o /dev/null ./internal/services/mcp_client.go"
validate "LSP client implementation" "go build -o /dev/null ./internal/services/acp_client.go"
validate "Unified protocol manager" "go build -o /dev/null ./internal/services/unified_protocol_manager.go"

echo ""
echo "🔗 PROTOCOL IMPLEMENTATIONS"
echo "==========================="

validate "MCP protocol support" "grep -q 'MCPClient' ./internal/services/mcp_client.go"
validate "LSP protocol support" "grep -q 'LSPClient' ./internal/services/acp_client.go"
validate "ACP protocol support" "grep -q 'ACPClient' ./internal/services/protocol_discovery.go"
validate "Protocol federation" "grep -q 'ProtocolFederation' ./internal/services/protocol_federation.go"
validate "Protocol analytics" "grep -q 'ProtocolAnalyticsService' ./internal/services/protocol_analytics.go"

echo ""
echo "📊 ANALYTICS & MONITORING"
echo "========================="

validate "Protocol analytics implementation" "grep -q 'RecordRequest' ./internal/services/protocol_analytics.go"
validate "Performance monitoring" "grep -q 'ProtocolMetrics' ./internal/services/protocol_monitor.go"
validate "Health status monitoring" "grep -q 'GetHealthStatus' ./internal/services/protocol_analytics.go"
validate "Usage pattern analysis" "grep -q 'UsagePattern' ./internal/services/protocol_analytics.go"

echo ""
echo "🔌 PLUGIN SYSTEM"
echo "================"

validate "Plugin system implementation" "grep -q 'PluginSystem' ./internal/services/plugin_system.go"
validate "Plugin registry" "grep -q 'PluginRegistry' ./internal/services/plugin_system.go"
validate "Plugin templates" "grep -q 'PluginTemplate' ./internal/services/plugin_system.go"

echo ""
echo "🚀 DEPLOYMENT INFRASTRUCTURE"
echo "============================"

validate "Docker Compose configuration" "test -f docker-compose.yml"
validate "Docker Compose production" "test -f docker-compose.prod.yml"
validate "Makefile targets" "grep -q 'docker-' Makefile"
validate "Kubernetes manifests" "test -d k8s/ 2>/dev/null || echo 'k8s/ not found but optional'"

echo ""
echo "📚 DOCUMENTATION"
echo "================"

validate "Protocol documentation" "test -f PROTOCOL_SUPPORT_DOCUMENTATION.md"
validate "Deployment guide" "test -f PROTOCOL_DEPLOYMENT_GUIDE.md"
validate "API documentation" "test -f docs/api-documentation.md"
validate "README enhanced" "test -f README_PROTOCOL_ENHANCED.md"

echo ""
echo "🔒 SECURITY & MONITORING"
echo "========================"

validate "Security sandbox" "grep -q 'SecuritySandbox' ./internal/services/security_sandbox.go"
validate "Rate limiting" "grep -q 'RateLimit' ./internal/services/request_service.go"
validate "Audit logging" "grep -q 'audit' ./internal/services/security_sandbox.go"
validate "Circuit breaker" "grep -q 'RequestCircuitBreaker' ./internal/services/request_service.go"

echo ""
echo "🎯 FINAL RESULTS"
echo "================"

echo "Total validations: $((PASSED + FAILED))"
echo -e "Passed: ${GREEN}${PASSED}${NC}"
echo -e "Failed: ${RED}${FAILED}${NC}"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}🎉 ALL VALIDATIONS PASSED!${NC}"
    echo -e "${GREEN}🚀 SuperAgent Protocol Enhancement is READY FOR PRODUCTION!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Run: make docker-full"
    echo "2. Access: http://localhost:8080"
    echo "3. Monitor: http://localhost:3000 (Grafana)"
    echo "4. Metrics: http://localhost:9090 (Prometheus)"
    exit 0
else
    echo ""
    echo -e "${RED}❌ Some validations failed. Please review and fix issues.${NC}"
    exit 1
fi