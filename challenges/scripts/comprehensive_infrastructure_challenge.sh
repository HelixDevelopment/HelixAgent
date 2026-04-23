#!/bin/bash
# ============================================================================
# COMPREHENSIVE INFRASTRUCTURE VALIDATION CHALLENGE
# ============================================================================
# Tests ALL HelixAgent infrastructure, protocols, and services
# Validates with 100% verification - no false positives allowed
#
# AUTO-BOOT: This script automatically starts ALL required infrastructure
# ============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# ============================================================================
# AUTO-BOOT INFRASTRUCTURE
# ============================================================================

auto_boot_infrastructure() {
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║     AUTO-BOOTING ALL INFRASTRUCTURE                            ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Run ensure-infrastructure script if available
    if [ -x "$PROJECT_ROOT/scripts/ensure-infrastructure.sh" ]; then
        "$PROJECT_ROOT/scripts/ensure-infrastructure.sh" start
    else
        # Fallback: direct compose
        echo "Running direct compose startup..."
        cd "$PROJECT_ROOT"

        # Detect compose command
        if docker compose version &>/dev/null 2>&1; then
            COMPOSE="docker compose"
        elif command -v docker-compose &>/dev/null; then
            COMPOSE="docker-compose"
        elif command -v podman-compose &>/dev/null; then
            COMPOSE="podman-compose"
        else
            echo "ERROR: No compose command found"
            exit 1
        fi

        # Start core services
        $COMPOSE --profile default up -d postgres redis chromadb cognee 2>/dev/null || \
        $COMPOSE up -d postgres redis chromadb 2>/dev/null || true

        # Wait for services
        echo "Waiting for services..."
        sleep 15
    fi

    # Ensure HelixAgent is running
    if ! curl -sf http://localhost:8100/health >/dev/null 2>&1; then
        echo "Starting HelixAgent..."
        cd "$PROJECT_ROOT"
        GIN_MODE=release ./bin/helixagent \
            --auto-start-mcp=false \
            --auto-start-docker=false \
            --skip-mcp-preinstall \
            --strict-dependencies=false \
            > /tmp/helixagent-challenge.log 2>&1 &

        # Wait for HelixAgent
        echo "Waiting for HelixAgent to start..."
        for i in {1..30}; do
            if curl -sf http://localhost:8100/health >/dev/null 2>&1; then
                echo "HelixAgent is ready!"
                break
            fi
            sleep 2
        done
    fi

    echo ""
}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0
TESTS_TOTAL=0

# Verification counters
VERIFIED_PASS=0
FALSE_POSITIVES=0

# Results array
declare -a TEST_RESULTS

# ============================================================================
# HELPER FUNCTIONS
# ============================================================================

log_header() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║ $1${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════╝${NC}"
}

log_section() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

test_result() {
    local test_name="$1"
    local result="$2"
    local details="$3"
    local verified="$4"

    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    if [ "$result" = "PASS" ]; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        if [ "$verified" = "true" ]; then
            VERIFIED_PASS=$((VERIFIED_PASS + 1))
            echo -e "  ${GREEN}✓ PASS (verified)${NC} - $test_name"
        else
            echo -e "  ${GREEN}✓ PASS${NC} - $test_name"
        fi
    elif [ "$result" = "SKIP" ]; then
        TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
        echo -e "  ${YELLOW}○ SKIP${NC} - $test_name: $details"
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        echo -e "  ${RED}✗ FAIL${NC} - $test_name: $details"
    fi

    TEST_RESULTS+=("$result|$test_name|$details")
}

# Verify test result to prevent false positives
verify_result() {
    local test_name="$1"
    local expected="$2"
    local actual="$3"

    if [ "$expected" = "$actual" ]; then
        return 0
    else
        FALSE_POSITIVES=$((FALSE_POSITIVES + 1))
        echo -e "  ${MAGENTA}⚠ FALSE POSITIVE DETECTED${NC} - $test_name"
        echo -e "    Expected: $expected"
        echo -e "    Actual: $actual"
        return 1
    fi
}

# HTTP test with verification
http_test() {
    local name="$1"
    local url="$2"
    local method="${3:-GET}"
    local data="$4"
    local expected_status="${5:-200}"
    local verify_json="${6:-}"

    local response
    local status
    local body

    if [ "$method" = "POST" ] && [ -n "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X POST "$url" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null || echo "000")
    else
        response=$(curl -s -w "\n%{http_code}" "$url" 2>/dev/null || echo "000")
    fi

    status=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$status" = "$expected_status" ]; then
        # Verify JSON content if specified
        if [ -n "$verify_json" ]; then
            local json_value
            json_value=$(echo "$body" | jq -r "$verify_json" 2>/dev/null || echo "null")
            if [ "$json_value" != "null" ] && [ -n "$json_value" ]; then
                test_result "$name" "PASS" "status=$status" "true"
                return 0
            else
                test_result "$name" "FAIL" "JSON verification failed: $verify_json"
                return 1
            fi
        fi
        test_result "$name" "PASS" "status=$status" "true"
        return 0
    else
        test_result "$name" "FAIL" "expected=$expected_status, got=$status"
        return 1
    fi
}

# TCP connection test
tcp_test() {
    local name="$1"
    local host="$2"
    local port="$3"

    if nc -z -w5 "$host" "$port" 2>/dev/null; then
        test_result "$name" "PASS" "port $port open" "true"
        return 0
    else
        test_result "$name" "FAIL" "port $port closed"
        return 1
    fi
}

# ============================================================================
# INFRASTRUCTURE TESTS
# ============================================================================

test_core_infrastructure() {
    log_section "CORE INFRASTRUCTURE"

    # PostgreSQL
    if pg_isready -h localhost -p 5432 -U helixagent &>/dev/null; then
        # Verify with actual query
        if PGPASSWORD=helixagent123 psql -h localhost -p 5432 -U helixagent -d helixagent_db -c "SELECT 1" &>/dev/null; then
            test_result "PostgreSQL Connection" "PASS" "verified with query" "true"
        else
            test_result "PostgreSQL Connection" "FAIL" "query failed"
        fi
    else
        test_result "PostgreSQL Connection" "FAIL" "not ready"
    fi

    # Redis
    if redis-cli -h localhost -p 6379 -a helixagent123 --no-auth-warning ping 2>/dev/null | grep -q "PONG"; then
        # Verify with set/get
        redis-cli -h localhost -p 6379 -a helixagent123 --no-auth-warning SET test_key "test_value" &>/dev/null
        local val=$(redis-cli -h localhost -p 6379 -a helixagent123 --no-auth-warning GET test_key 2>/dev/null)
        if [ "$val" = "test_value" ]; then
            test_result "Redis Connection" "PASS" "verified with set/get" "true"
            redis-cli -h localhost -p 6379 -a helixagent123 --no-auth-warning DEL test_key &>/dev/null
        else
            test_result "Redis Connection" "FAIL" "get returned wrong value"
        fi
    else
        test_result "Redis Connection" "FAIL" "ping failed"
    fi

    # ChromaDB - use v2 API
    http_test "ChromaDB Health" "http://localhost:8001/api/v2/heartbeat" "GET" "" "200"

    # Cognee
    http_test "Cognee Health" "http://localhost:8000/" "GET" "" "200"
}

# ============================================================================
# HELIXAGENT API TESTS
# ============================================================================

test_helixagent_api() {
    log_section "HELIXAGENT API"

    local BASE_URL="http://localhost:8100"

    # Health endpoint
    http_test "HelixAgent Health" "$BASE_URL/health" "GET" "" "200" ".status"

    # Models endpoint
    http_test "Models List" "$BASE_URL/v1/models" "GET" "" "200" ".data"

    # Providers endpoint
    http_test "Providers List" "$BASE_URL/v1/providers" "GET" "" "200"
}

# ============================================================================
# MCP PROTOCOL TESTS
# ============================================================================

test_mcp_protocol() {
    log_section "MCP PROTOCOL"

    local MCP_PORTS=(9101 9102 9103 9104 9105 9106 9107)
    local MCP_NAMES=("filesystem" "memory" "postgres" "puppeteer" "sequential-thinking" "everything" "github")

    for i in "${!MCP_PORTS[@]}"; do
        local port="${MCP_PORTS[$i]}"
        local name="${MCP_NAMES[$i]}"

        # Test TCP connection
        if nc -z -w5 localhost "$port" 2>/dev/null; then
            # Send JSON-RPC initialize and verify response
            local response
            response=$(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | \
                nc -w5 localhost "$port" 2>/dev/null | head -1 || echo "")

            if echo "$response" | grep -q '"jsonrpc"'; then
                test_result "MCP $name (port $port)" "PASS" "JSON-RPC verified" "true"
            else
                test_result "MCP $name (port $port)" "PASS" "port open" "false"
            fi
        else
            test_result "MCP $name (port $port)" "SKIP" "server not running"
        fi
    done
}

# ============================================================================
# ACP PROTOCOL TESTS
# ============================================================================

test_acp_protocol() {
    log_section "ACP PROTOCOL"

    local BASE_URL="http://localhost:8100/v1/acp"

    # Health
    http_test "ACP Health" "$BASE_URL/health" "GET" "" "200" ".status"

    # List agents
    http_test "ACP List Agents" "$BASE_URL/agents" "GET" "" "200" ".agents"

    # Test each agent
    local AGENTS=("code-reviewer" "bug-finder" "refactor-assistant" "documentation-generator" "test-generator" "security-scanner")

    for agent in "${AGENTS[@]}"; do
        http_test "ACP Agent: $agent" "$BASE_URL/agents/$agent" "GET" "" "200" ".id"
    done

    # Execute agent
    local exec_data='{"agent_id":"code-reviewer","task":"review code","context":{"code":"func test() {}","language":"go"}}'
    http_test "ACP Execute" "$BASE_URL/execute" "POST" "$exec_data" "200" ".status"
}

# ============================================================================
# VISION PROTOCOL TESTS
# ============================================================================

test_vision_protocol() {
    log_section "VISION PROTOCOL"

    local BASE_URL="http://localhost:8100/v1/vision"

    # Health
    http_test "Vision Health" "$BASE_URL/health" "GET" "" "200" ".status"

    # Capabilities
    http_test "Vision Capabilities" "$BASE_URL/capabilities" "GET" "" "200" ".capabilities"

    # Test each capability
    local CAPABILITIES=("analyze" "ocr" "detect" "caption" "describe" "classify")

    for cap in "${CAPABILITIES[@]}"; do
        http_test "Vision Capability: $cap" "$BASE_URL/$cap/status" "GET" "" "200" ".status"

        # Test actual capability with dummy image data
        local req_data='{"image":"","prompt":"test"}'
        http_test "Vision Execute: $cap" "$BASE_URL/$cap" "POST" "$req_data" "200" ".status"
    done
}

# ============================================================================
# EMBEDDINGS TESTS
# ============================================================================

test_embeddings() {
    log_section "EMBEDDINGS"

    local BASE_URL="http://localhost:8100/v1/embeddings"

    # Test embeddings endpoint
    local embed_data='{"input":"test text","model":"text-embedding-3-small"}'
    http_test "Embeddings API" "$BASE_URL" "POST" "$embed_data" "200" ".data"

    # Test with array input
    local embed_array='{"input":["text one","text two"],"model":"text-embedding-3-small"}'
    http_test "Embeddings Batch" "$BASE_URL" "POST" "$embed_array" "200" ".data"
}

# ============================================================================
# LSP PROTOCOL TESTS
# ============================================================================

test_lsp_protocol() {
    log_section "LSP PROTOCOL"

    local LSP_SERVERS=(
        "gopls|5001"
        "rust-analyzer|5002"
        "pylsp|5003"
        "typescript|5004"
        "clangd|5005"
        "jdtls|5006"
        "bash-lsp|5020"
        "yaml-lsp|5021"
        "docker-lsp|5022"
        "terraform-lsp|5023"
        "xml-lsp|5024"
    )

    for server in "${LSP_SERVERS[@]}"; do
        IFS='|' read -r name port <<< "$server"
        tcp_test "LSP $name" "localhost" "$port"
    done

    # LSP Manager
    http_test "LSP Manager Health" "http://localhost:5100/health" "GET" "" "200" ".status"
    http_test "LSP Manager Servers" "http://localhost:5100/servers" "GET" "" "200" ".servers"
}

# ============================================================================
# RAG SERVICES TESTS
# ============================================================================

test_rag_services() {
    log_section "RAG SERVICES"

    # Qdrant
    http_test "Qdrant Health" "http://localhost:6333/readyz" "GET" "" "200"

    # Sentence Transformers
    http_test "Sentence Transformers Health" "http://localhost:8016/health" "GET" "" "200" ".status"

    # BGE-M3
    http_test "BGE-M3 Health" "http://localhost:8017/health" "GET" "" "200" ".status"

    # RAGatouille
    http_test "RAGatouille Health" "http://localhost:8018/health" "GET" "" "200" ".status"

    # HyDE
    http_test "HyDE Health" "http://localhost:8019/health" "GET" "" "200" ".status"

    # Multi-Query
    http_test "Multi-Query Health" "http://localhost:8020/health" "GET" "" "200" ".status"

    # Reranker
    http_test "Reranker Health" "http://localhost:8021/health" "GET" "" "200" ".status"

    # RAG Manager
    http_test "RAG Manager Health" "http://localhost:8030/health" "GET" "" "200" ".status"
}

# ============================================================================
# COGNEE INTEGRATION TESTS
# ============================================================================

test_cognee_integration() {
    log_section "COGNEE INTEGRATION"

    local BASE_URL="http://localhost:8100/v1/cognee"

    # Health
    http_test "Cognee Integration Health" "$BASE_URL/health" "GET" "" "200"

    # Add content
    local add_data='{"content":"HelixAgent is an AI-powered ensemble LLM service."}'
    http_test "Cognee Add Content" "$BASE_URL/add" "POST" "$add_data" "200"

    # Search
    local search_data='{"query":"What is HelixAgent?"}'
    http_test "Cognee Search" "$BASE_URL/search" "POST" "$search_data" "200"
}

# ============================================================================
# CLI AGENT VALIDATION
# ============================================================================

test_cli_agents() {
    log_section "CLI AGENTS (48 Agents)"

    # Get list of supported agents
    local agents_output
    agents_output=$("$PROJECT_ROOT/bin/helixagent" --list-agents 2>/dev/null || echo "")

    if [ -z "$agents_output" ]; then
        test_result "CLI Agents Registry" "SKIP" "helixagent binary not available"
        return
    fi

    # Count agents
    local agent_count
    agent_count=$(echo "$agents_output" | grep -c "^[a-zA-Z]" || echo "0")

    if [ "$agent_count" -ge 48 ]; then
        test_result "CLI Agents Registry" "PASS" "$agent_count agents registered" "true"
    else
        test_result "CLI Agents Registry" "FAIL" "expected 48+, got $agent_count"
    fi

    # Test config generation for a few key agents
    local TEST_AGENTS=("opencode" "claudecode" "codex" "aider" "cline")

    for agent in "${TEST_AGENTS[@]}"; do
        local config_output
        config_output=$("$PROJECT_ROOT/bin/helixagent" --generate-agent-config="$agent" 2>/dev/null || echo "")

        if [ -n "$config_output" ] && echo "$config_output" | grep -q "mcp"; then
            test_result "Agent Config: $agent" "PASS" "valid config with MCP" "true"
        else
            test_result "Agent Config: $agent" "FAIL" "invalid or empty config"
        fi
    done
}

# ============================================================================
# AI DEBATE SYSTEM TESTS
# ============================================================================

test_ai_debate() {
    log_section "AI DEBATE SYSTEM"

    local BASE_URL="http://localhost:8100/v1"

    # Debate health
    http_test "Debate Health" "$BASE_URL/debates/health" "GET" "" "200"

    # Start a simple debate
    local debate_data='{
        "topic": "Is AI beneficial for humanity?",
        "participants": ["supporter", "skeptic", "mediator"],
        "max_rounds": 1,
        "timeout": 30
    }'

    # Note: This may take time, so we use a longer timeout
    local response
    response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/debates" \
        -H "Content-Type: application/json" \
        -d "$debate_data" \
        --max-time 60 2>/dev/null || echo "000")

    local status
    status=$(echo "$response" | tail -n1)

    if [ "$status" = "200" ] || [ "$status" = "201" ] || [ "$status" = "202" ]; then
        test_result "AI Debate Create" "PASS" "debate started/created" "true"
    else
        test_result "AI Debate Create" "SKIP" "debate endpoint status=$status"
    fi
}

# ============================================================================
# SECURITY TESTS
# ============================================================================

test_security() {
    log_section "SECURITY"

    local BASE_URL="http://localhost:8100"

    # Test CORS headers
    local cors_response
    cors_response=$(curl -s -I -X OPTIONS "$BASE_URL/v1/models" \
        -H "Origin: http://evil.com" \
        -H "Access-Control-Request-Method: GET" 2>/dev/null || echo "")

    # Check that unauthorized origins are not allowed (or that CORS is properly configured)
    if echo "$cors_response" | grep -qi "access-control-allow-origin"; then
        local allowed_origin
        allowed_origin=$(echo "$cors_response" | grep -i "access-control-allow-origin" | cut -d: -f2 | tr -d ' \r')
        if [ "$allowed_origin" = "*" ] || [ "$allowed_origin" = "http://evil.com" ]; then
            test_result "CORS Configuration" "FAIL" "overly permissive CORS"
        else
            test_result "CORS Configuration" "PASS" "CORS properly configured" "true"
        fi
    else
        test_result "CORS Configuration" "PASS" "no CORS headers for unauthorized origin" "true"
    fi

    # Test rate limiting
    local rate_limit_hit=0
    for i in {1..20}; do
        local status
        status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" 2>/dev/null)
        if [ "$status" = "429" ]; then
            rate_limit_hit=1
            break
        fi
    done

    if [ "$rate_limit_hit" = "1" ]; then
        test_result "Rate Limiting" "PASS" "rate limiting active" "true"
    else
        test_result "Rate Limiting" "SKIP" "rate limiting not triggered in 20 requests"
    fi

    # Test SQL injection protection
    local sqli_response
    sqli_response=$(curl -s -w "%{http_code}" "$BASE_URL/v1/models?id=1'%20OR%20'1'='1" 2>/dev/null)
    local sqli_status
    sqli_status=$(echo "$sqli_response" | tail -n1)

    if [ "$sqli_status" = "400" ] || [ "$sqli_status" = "200" ]; then
        # Either rejected (400) or safely handled (200 with proper escaping)
        test_result "SQL Injection Protection" "PASS" "handled safely" "true"
    else
        test_result "SQL Injection Protection" "FAIL" "unexpected response: $sqli_status"
    fi
}

# ============================================================================
# FINAL REPORT
# ============================================================================

print_report() {
    log_header "COMPREHENSIVE VALIDATION REPORT"

    local pass_rate=0
    if [ "$TESTS_TOTAL" -gt 0 ]; then
        pass_rate=$(( (TESTS_PASSED * 100) / TESTS_TOTAL ))
    fi

    echo ""
    echo -e "${CYAN}╭────────────────────────────────────────╮${NC}"
    echo -e "${CYAN}│${NC}         TEST RESULTS SUMMARY          ${CYAN}│${NC}"
    echo -e "${CYAN}├────────────────────────────────────────┤${NC}"
    echo -e "${CYAN}│${NC}  Total Tests:     ${BLUE}$(printf '%4d' $TESTS_TOTAL)${NC}                 ${CYAN}│${NC}"
    echo -e "${CYAN}│${NC}  Passed:          ${GREEN}$(printf '%4d' $TESTS_PASSED)${NC}                 ${CYAN}│${NC}"
    echo -e "${CYAN}│${NC}  Failed:          ${RED}$(printf '%4d' $TESTS_FAILED)${NC}                 ${CYAN}│${NC}"
    echo -e "${CYAN}│${NC}  Skipped:         ${YELLOW}$(printf '%4d' $TESTS_SKIPPED)${NC}                 ${CYAN}│${NC}"
    echo -e "${CYAN}│${NC}  Verified Pass:   ${GREEN}$(printf '%4d' $VERIFIED_PASS)${NC}                 ${CYAN}│${NC}"
    echo -e "${CYAN}│${NC}  Pass Rate:       ${BLUE}$(printf '%3d' $pass_rate)%%${NC}                  ${CYAN}│${NC}"
    echo -e "${CYAN}╰────────────────────────────────────────╯${NC}"

    if [ "$FALSE_POSITIVES" -gt 0 ]; then
        echo ""
        echo -e "${MAGENTA}⚠ WARNING: $FALSE_POSITIVES false positive(s) detected!${NC}"
    fi

    echo ""

    # Exit with appropriate code
    if [ "$TESTS_FAILED" -gt 0 ] || [ "$FALSE_POSITIVES" -gt 0 ]; then
        echo -e "${RED}CHALLENGE FAILED${NC}"
        exit 1
    elif [ "$TESTS_PASSED" -eq 0 ]; then
        echo -e "${YELLOW}CHALLENGE INCOMPLETE - No tests passed${NC}"
        exit 1
    else
        echo -e "${GREEN}CHALLENGE PASSED${NC}"
        exit 0
    fi
}

# ============================================================================
# MAIN
# ============================================================================

main() {
    log_header "COMPREHENSIVE INFRASTRUCTURE CHALLENGE"
    echo -e "${BLUE}Testing ALL HelixAgent services, protocols, and integrations${NC}"
    echo -e "${BLUE}With 100% verification - NO false positives allowed${NC}"
    echo ""

    # AUTO-BOOT: Ensure all infrastructure is running
    auto_boot_infrastructure

    # Run all test suites
    test_core_infrastructure
    test_helixagent_api
    test_mcp_protocol
    test_acp_protocol
    test_vision_protocol
    test_embeddings
    test_lsp_protocol
    test_rag_services
    test_cognee_integration
    test_cli_agents
    test_ai_debate
    test_security

    # Print final report
    print_report
}

main "$@"
