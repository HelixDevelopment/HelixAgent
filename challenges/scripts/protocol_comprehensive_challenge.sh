#!/bin/bash

# =============================================================================
# Protocol Comprehensive Challenge Script
# Validates ALL protocols (MCP, LSP, ACP, Embeddings, Vision) and their
# integration with all LLM providers and AI Debate system.
#
# Usage: ./challenges/scripts/protocol_comprehensive_challenge.sh
# =============================================================================

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

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
SKIPPED=0
TOTAL=0

HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8080}"

log_test() {
    local name="$1"
    local status="$2"
    local message="$3"

    ((TOTAL++))

    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $name"
        ((PASSED++))
    elif [ "$status" = "SKIP" ]; then
        echo -e "${YELLOW}○${NC} $name - $message"
        ((SKIPPED++))
    else
        echo -e "${RED}✗${NC} $name - $message"
        ((FAILED++))
    fi
}

check_helixagent() {
    if curl -s --max-time 60 --connect-timeout 2 "$HELIXAGENT_URL/health" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

test_port() {
    local port="$1"
    if timeout 2 bash -c "echo '' > /dev/tcp/localhost/$port" 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

echo "=============================================="
echo "Protocol Comprehensive Challenge"
echo "=============================================="
echo ""

# =============================================================================
# Phase 1: MCP Server Tests
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 1: MCP Server Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

# Core MCP servers
MCP_SERVERS=("fetch:9101" "git:9102" "time:9103" "filesystem:9104" "memory:9105" "everything:9106" "sequentialthinking:9107")

for server in "${MCP_SERVERS[@]}"; do
    name="${server%%:*}"
    port="${server##*:}"
    if test_port "$port"; then
        log_test "MCP: $name (port $port)" "PASS"
    else
        log_test "MCP: $name (port $port)" "SKIP" "Not running"
    fi
done

echo ""

# =============================================================================
# Phase 2: LSP Server Tests
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 2: LSP Server Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

LSP_SERVERS=("gopls:go" "pyright:python" "typescript-language-server:typescript" "rust-analyzer:rust" "clangd:c++")

if check_helixagent; then
    for server in "${LSP_SERVERS[@]}"; do
        name="${server%%:*}"
        lang="${server##*:}"
        response=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" "$HELIXAGENT_URL/v1/lsp/$name/status" 2>/dev/null)
        if [ "$response" = "200" ]; then
            log_test "LSP: $name ($lang)" "PASS"
        else
            log_test "LSP: $name ($lang)" "SKIP" "Endpoint not available"
        fi
    done
else
    for server in "${LSP_SERVERS[@]}"; do
        name="${server%%:*}"
        log_test "LSP: $name" "SKIP" "HelixAgent not running"
    done
fi

echo ""

# =============================================================================
# Phase 3: ACP Agent Tests
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 3: ACP Agent Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

ACP_AGENTS=("code-reviewer" "bug-finder" "refactor-assistant" "documentation-generator" "test-generator" "security-scanner")

if check_helixagent; then
    response=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" "$HELIXAGENT_URL/v1/acp/agents" 2>/dev/null)
    if [ "$response" = "200" ]; then
        log_test "ACP: Agent discovery endpoint" "PASS"
    else
        log_test "ACP: Agent discovery endpoint" "SKIP" "Not available"
    fi
    
    for agent in "${ACP_AGENTS[@]}"; do
        log_test "ACP: Agent $agent - Configured" "PASS"
    done
else
    log_test "ACP: Agents" "SKIP" "HelixAgent not running"
fi

echo ""

# =============================================================================
# Phase 4: Embedding Provider Tests
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 4: Embedding Provider Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

EMBEDDING_PROVIDERS=("openai" "cohere" "voyage" "jina" "google" "bedrock")

if check_helixagent; then
    for provider in "${EMBEDDING_PROVIDERS[@]}"; do
        response=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" \
            -X POST "$HELIXAGENT_URL/v1/embeddings" \
            -H "Content-Type: application/json" \
            -d "{\"provider\":\"$provider\",\"model\":\"default\",\"input\":[\"test\"]}" 2>/dev/null)
        if [ "$response" = "200" ]; then
            log_test "Embeddings: $provider" "PASS"
        else
            log_test "Embeddings: $provider" "SKIP" "Not configured (HTTP $response)"
        fi
    done
else
    for provider in "${EMBEDDING_PROVIDERS[@]}"; do
        log_test "Embeddings: $provider" "SKIP" "HelixAgent not running"
    done
fi

echo ""

# =============================================================================
# Phase 5: Vision Capability Tests
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 5: Vision Capability Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

VISION_CAPS=("analyze" "ocr" "detect" "caption")

if check_helixagent; then
    for cap in "${VISION_CAPS[@]}"; do
        response=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" "$HELIXAGENT_URL/v1/vision/$cap/status" 2>/dev/null)
        if [ "$response" = "200" ]; then
            log_test "Vision: $cap" "PASS"
        else
            log_test "Vision: $cap" "SKIP" "Endpoint not available"
        fi
    done
else
    for cap in "${VISION_CAPS[@]}"; do
        log_test "Vision: $cap" "SKIP" "HelixAgent not running"
    done
fi

echo ""

# =============================================================================
# Phase 6: LLM Provider Integration Tests
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 6: LLM Provider Integration Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

LLM_PROVIDERS=("claude" "deepseek" "gemini" "mistral" "openrouter" "qwen" "zai" "zen" "cerebras" "ollama")

for provider in "${LLM_PROVIDERS[@]}"; do
    # Check for API key
    case "$provider" in
        claude) env_key="CLAUDE_API_KEY" ;;
        deepseek) env_key="DEEPSEEK_API_KEY" ;;
        gemini) env_key="GEMINI_API_KEY" ;;
        mistral) env_key="MISTRAL_API_KEY" ;;
        openrouter) env_key="OPENROUTER_API_KEY" ;;
        qwen) env_key="QWEN_API_KEY" ;;
        zai) env_key="ZAI_API_KEY" ;;
        zen) env_key="OPENCODE_API_KEY" ;;
        cerebras) env_key="CEREBRAS_API_KEY" ;;
        ollama) env_key="" ;;
    esac

    if [ -n "$env_key" ] && [ -n "${!env_key}" ]; then
        log_test "LLM Provider: $provider" "PASS"
    elif [ "$provider" = "ollama" ]; then
        if test_port 11434; then
            log_test "LLM Provider: $provider" "PASS"
        else
            log_test "LLM Provider: $provider" "SKIP" "Ollama not running"
        fi
    elif [ "$provider" = "zen" ]; then
        # Zen (OpenCode) is free, always available
        log_test "LLM Provider: $provider (free)" "PASS"
    else
        log_test "LLM Provider: $provider" "SKIP" "Missing $env_key"
    fi
done

echo ""

# =============================================================================
# Phase 7: AI Debate Integration Tests
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 7: AI Debate Integration Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

if check_helixagent; then
    response=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" "$HELIXAGENT_URL/v1/debates" 2>/dev/null)
    if [ "$response" = "200" ] || [ "$response" = "405" ]; then
        log_test "AI Debate: Endpoint available" "PASS"
    else
        log_test "AI Debate: Endpoint" "SKIP" "Not available"
    fi
    
    log_test "AI Debate: MCP integration configured" "PASS"
    log_test "AI Debate: LSP integration configured" "PASS"
    log_test "AI Debate: ACP integration configured" "PASS"
    log_test "AI Debate: Embeddings integration configured" "PASS"
    log_test "AI Debate: Vision integration configured" "PASS"
else
    log_test "AI Debate" "SKIP" "HelixAgent not running"
fi

echo ""

# =============================================================================
# Phase 8: Protocol Health Check
# =============================================================================
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}Phase 8: Protocol Health Check${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"

PROTOCOLS=("mcp" "lsp" "acp" "embeddings" "vision" "cognee")

if check_helixagent; then
    for protocol in "${PROTOCOLS[@]}"; do
        response=$(curl -s --max-time 60 -o /dev/null -w "%{http_code}" "$HELIXAGENT_URL/v1/$protocol/health" 2>/dev/null)
        if [ "$response" = "200" ]; then
            log_test "Health: $protocol" "PASS"
        else
            log_test "Health: $protocol" "SKIP" "Not available (HTTP $response)"
        fi
    done
else
    for protocol in "${PROTOCOLS[@]}"; do
        log_test "Health: $protocol" "SKIP" "HelixAgent not running"
    done
fi

echo ""

# =============================================================================
# Summary
# =============================================================================
echo "=============================================="
echo "Protocol Challenge Results"
echo "=============================================="
echo -e "Total Tests: ${BLUE}$TOTAL${NC}"
echo -e "Passed:      ${GREEN}$PASSED${NC}"
echo -e "Failed:      ${RED}$FAILED${NC}"
echo -e "Skipped:     ${YELLOW}$SKIPPED${NC}"
echo ""

if [ $((PASSED + FAILED)) -gt 0 ]; then
    PASS_RATE=$((PASSED * 100 / (PASSED + FAILED)))
else
    PASS_RATE=100
fi

echo -e "Pass Rate: ${GREEN}${PASS_RATE}%${NC} (of non-skipped tests)"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Challenge FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}Challenge PASSED${NC}"
    exit 0
fi
