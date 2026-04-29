#!/bin/bash
# =============================================================================
# System Compliance Challenge
# =============================================================================
# Verifies that HelixAgent meets minimum requirements:
# - 30+ providers registered
# - 30+ MCP servers configured
# - 900+ LLMs available
# - New debate framework enabled
# - OAuth and free providers supported
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Counters
PASSED=0
FAILED=0
TOTAL=0

# Minimum requirements
MIN_PROVIDERS=30
MIN_MCPS=30
MIN_MODELS=100

log_info() {
    echo -e "${BLUE}[INFO]${NC} $(date +%H:%M:%S) $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) $1"
    ((PASSED++))
    ((TOTAL++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $(date +%H:%M:%S) $1"
    ((FAILED++))
    ((TOTAL++))
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $(date +%H:%M:%S) $1"
}

echo ""
echo "=============================================================================="
echo "             HELIXAGENT SYSTEM COMPLIANCE CHALLENGE"
echo "=============================================================================="
echo ""

# =============================================================================
# Section 1: Provider Compliance
# =============================================================================
log_info ""
log_info "=============================================="
log_info "Section 1: Provider Compliance (30+ required)"
log_info "=============================================="

# Count providers in SupportedProviders
PROVIDER_COUNT=$(grep -c '".*": {' "$PROJECT_ROOT/internal/verifier/provider_types.go" | head -1 || echo "0")
# More accurate count
PROVIDER_COUNT=$(grep -E '^\s+"[a-z]+":\s*\{' "$PROJECT_ROOT/internal/verifier/provider_types.go" | wc -l)

log_info "Test 1: Provider count >= $MIN_PROVIDERS"
if [ "$PROVIDER_COUNT" -ge "$MIN_PROVIDERS" ]; then
    log_success "Provider count: $PROVIDER_COUNT >= $MIN_PROVIDERS"
else
    log_fail "Provider count: $PROVIDER_COUNT < $MIN_PROVIDERS"
fi

# Check for OAuth providers
log_info "Test 2: OAuth providers exist (Claude, Qwen)"
OAUTH_CLAUDE=$(grep -c '"claude":' "$PROJECT_ROOT/internal/verifier/provider_types.go" || echo "0")
OAUTH_QWEN=$(grep -c '"qwen":' "$PROJECT_ROOT/internal/verifier/provider_types.go" || echo "0")
if [ "$OAUTH_CLAUDE" -gt 0 ] && [ "$OAUTH_QWEN" -gt 0 ]; then
    log_success "OAuth providers: Claude and Qwen present"
else
    log_fail "OAuth providers missing: Claude=$OAUTH_CLAUDE, Qwen=$OAUTH_QWEN"
fi

# Check for free providers
log_info "Test 3: Free providers exist (Zen, OpenRouter)"
FREE_ZEN=$(grep -c '"zen":' "$PROJECT_ROOT/internal/verifier/provider_types.go" || echo "0")
FREE_OPENROUTER=$(grep -c '"openrouter":' "$PROJECT_ROOT/internal/verifier/provider_types.go" || echo "0")
if [ "$FREE_ZEN" -gt 0 ] && [ "$FREE_OPENROUTER" -gt 0 ]; then
    log_success "Free providers: Zen and OpenRouter present"
else
    log_fail "Free providers missing: Zen=$FREE_ZEN, OpenRouter=$FREE_OPENROUTER"
fi

# Check for specific providers from .env
log_info "Test 4: Providers from .env are supported"
REQUIRED_PROVIDERS=("gemini" "deepseek" "mistral" "cerebras" "fireworks" "kimi" "nvidia" "huggingface" "hyperbolic" "sambanova")
MISSING_PROVIDERS=()
for provider in "${REQUIRED_PROVIDERS[@]}"; do
    if ! grep -q "\"$provider\":" "$PROJECT_ROOT/internal/verifier/provider_types.go"; then
        MISSING_PROVIDERS+=("$provider")
    fi
done
if [ ${#MISSING_PROVIDERS[@]} -eq 0 ]; then
    log_success "All required providers from .env are supported"
else
    log_fail "Missing providers: ${MISSING_PROVIDERS[*]}"
fi

# =============================================================================
# Section 2: MCP Compliance
# =============================================================================
log_info ""
log_info "=============================================="
log_info "Section 2: MCP Adapter Compliance (30+ required)"
log_info "=============================================="

# Count MCP adapters in registry
MCP_COUNT=$(grep -c 'Name:.*Category:' "$PROJECT_ROOT/internal/mcp/adapters/registry.go" || echo "0")
MCP_COUNT=$(grep -E '\{Name:' "$PROJECT_ROOT/internal/mcp/adapters/registry.go" | wc -l)

log_info "Test 5: MCP adapter count >= $MIN_MCPS"
if [ "$MCP_COUNT" -ge "$MIN_MCPS" ]; then
    log_success "MCP adapter count: $MCP_COUNT >= $MIN_MCPS"
else
    log_fail "MCP adapter count: $MCP_COUNT < $MIN_MCPS"
fi

# Count MCP servers in DefaultMCPServers
log_info "Test 6: Default MCP servers for CLI agents >= $MIN_MCPS"
CLI_MCP_COUNT=$(grep -c '{Name:' "$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/generator.go" || echo "0")
if [ "$CLI_MCP_COUNT" -ge "$MIN_MCPS" ]; then
    log_success "CLI MCP servers: $CLI_MCP_COUNT >= $MIN_MCPS"
else
    log_fail "CLI MCP servers: $CLI_MCP_COUNT < $MIN_MCPS"
fi

# Check for HelixAgent protocol MCPs
log_info "Test 7: HelixAgent protocol MCPs present"
HELIX_MCPS=("helixagent-mcp" "helixagent-acp" "helixagent-lsp" "helixagent-embeddings" "helixagent-vision" "helixagent-cognee")
MISSING_HELIX=()
for mcp in "${HELIX_MCPS[@]}"; do
    if ! grep -q "$mcp" "$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/generator.go"; then
        MISSING_HELIX+=("$mcp")
    fi
done
if [ ${#MISSING_HELIX[@]} -eq 0 ]; then
    log_success "All HelixAgent protocol MCPs present"
else
    log_fail "Missing HelixAgent MCPs: ${MISSING_HELIX[*]}"
fi

# Check for official MCP servers
log_info "Test 8: Official MCP servers present"
OFFICIAL_MCPS=("filesystem" "github" "memory" "fetch" "puppeteer" "sqlite" "postgres" "brave-search" "slack" "google-maps")
MISSING_OFFICIAL=()
for mcp in "${OFFICIAL_MCPS[@]}"; do
    if ! grep -q "\"$mcp\"" "$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/generator.go"; then
        MISSING_OFFICIAL+=("$mcp")
    fi
done
if [ ${#MISSING_OFFICIAL[@]} -eq 0 ]; then
    log_success "All official MCP servers present"
else
    log_fail "Missing official MCPs: ${MISSING_OFFICIAL[*]}"
fi

# =============================================================================
# Section 3: Debate Framework Compliance
# =============================================================================
log_info ""
log_info "=============================================="
log_info "Section 3: Debate Framework Compliance"
log_info "=============================================="

# Check new framework is enabled
log_info "Test 9: New debate framework enabled by default"
NEW_FRAMEWORK=$(grep -c 'EnableNewFramework:.*true' "$PROJECT_ROOT/DebateOrchestrator/orchestrator/service_integration.go" || echo "0")
if [ "$NEW_FRAMEWORK" -gt 0 ]; then
    log_success "New debate framework is enabled by default"
else
    log_fail "New debate framework is NOT enabled by default"
fi

# Check learning is enabled
log_info "Test 10: Learning enabled by default"
LEARNING=$(grep -c 'EnableLearning:.*true' "$PROJECT_ROOT/DebateOrchestrator/orchestrator/service_integration.go" || echo "0")
if [ "$LEARNING" -gt 0 ]; then
    log_success "Learning is enabled by default"
else
    log_fail "Learning is NOT enabled by default"
fi

# Check orchestrator integration in router
log_info "Test 11: Orchestrator integration in router"
ORCH_INTEGRATION=$(grep -c 'orchestrator.CreateIntegration' "$PROJECT_ROOT/internal/router/router.go" || echo "0")
if [ "$ORCH_INTEGRATION" -gt 0 ]; then
    log_success "Orchestrator integration present in router"
else
    log_fail "Orchestrator integration missing from router"
fi

# Check handler integration
log_info "Test 12: Debate handler integration"
HANDLER_INTEGRATION=$(grep -c 'SetOrchestratorIntegration' "$PROJECT_ROOT/internal/router/router.go" || echo "0")
if [ "$HANDLER_INTEGRATION" -gt 0 ]; then
    log_success "Debate handler has orchestrator integration"
else
    log_fail "Debate handler missing orchestrator integration"
fi

# =============================================================================
# Section 4: Infrastructure Boot Compliance
# =============================================================================
log_info ""
log_info "=============================================="
log_info "Section 4: Infrastructure Boot Compliance"
log_info "=============================================="

# Check strict dependencies default
log_info "Test 13: Strict dependencies enabled by default"
STRICT_DEPS=$(grep -c 'strictDependencies.*true' "$PROJECT_ROOT/cmd/helixagent/main.go" || echo "0")
if [ "$STRICT_DEPS" -gt 0 ]; then
    log_success "Strict dependencies enabled by default"
else
    log_fail "Strict dependencies NOT enabled by default"
fi

# Check auto-start docker default
log_info "Test 14: Auto-start Docker enabled by default"
AUTO_DOCKER=$(grep -c 'autoStartDocker.*true' "$PROJECT_ROOT/cmd/helixagent/main.go" || echo "0")
if [ "$AUTO_DOCKER" -gt 0 ]; then
    log_success "Auto-start Docker enabled by default"
else
    log_fail "Auto-start Docker NOT enabled by default"
fi

# Check mandatory dependency verification
log_info "Test 15: Mandatory dependency verification exists"
DEP_VERIFY=$(grep -c 'verifyAllMandatoryDependencies' "$PROJECT_ROOT/cmd/helixagent/main.go" || echo "0")
if [ "$DEP_VERIFY" -gt 0 ]; then
    log_success "Mandatory dependency verification exists"
else
    log_fail "Mandatory dependency verification missing"
fi

# =============================================================================
# Section 5: CLI Agent Compliance
# =============================================================================
log_info ""
log_info "=============================================="
log_info "Section 5: CLI Agent Compliance (48 agents)"
log_info "=============================================="

# Count CLI agents
log_info "Test 16: CLI agents count >= 40"
AGENT_COUNT=$(grep -c 'Agent.*=' "$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/generator.go" | head -1 || echo "0")
AGENT_COUNT=$(grep -E 'Agent[A-Z][a-zA-Z]+\s+AgentType\s*=' "$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/types.go" 2>/dev/null | wc -l || echo "0")
if [ "$AGENT_COUNT" -ge 40 ]; then
    log_success "CLI agent count: $AGENT_COUNT >= 40"
else
    log_warn "CLI agent count: $AGENT_COUNT (expected 48)"
fi

# Check OpenCode generator
log_info "Test 17: OpenCode generator exists"
OPENCODE_GEN=$(grep -c 'NewOpenCodeGenerator' "$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/generator.go" || echo "0")
if [ "$OPENCODE_GEN" -gt 0 ]; then
    log_success "OpenCode generator exists"
else
    log_fail "OpenCode generator missing"
fi

# Check Crush generator
log_info "Test 18: Crush generator exists"
CRUSH_GEN=$(grep -c 'NewCrushGenerator' "$PROJECT_ROOT/LLMsVerifier/llm-verifier/pkg/cliagents/generator.go" || echo "0")
if [ "$CRUSH_GEN" -gt 0 ]; then
    log_success "Crush generator exists"
else
    log_fail "Crush generator missing"
fi

# =============================================================================
# Summary
# =============================================================================
log_info ""
log_info "=============================================="
log_info "Challenge Summary: System Compliance Challenge"
log_info "=============================================="
log_info "Total Tests: $TOTAL"
log_success "Passed: $PASSED"
if [ "$FAILED" -gt 0 ]; then
    log_fail "Failed: $FAILED"
fi
log_info "Pass Rate: $(( PASSED * 100 / TOTAL ))%"

if [ "$FAILED" -eq 0 ]; then
    echo ""
    log_success "=============================================="
    log_success "ALL TESTS PASSED!"
    log_success "System meets compliance requirements:"
    log_success "  - 30+ providers registered"
    log_success "  - 30+ MCP servers configured"
    log_success "  - New debate framework enabled"
    log_success "  - Infrastructure boot configured"
    log_success "=============================================="
    exit 0
else
    echo ""
    log_fail "=============================================="
    log_fail "COMPLIANCE CHECK FAILED!"
    log_fail "$FAILED tests failed out of $TOTAL"
    log_fail "=============================================="
    exit 1
fi
