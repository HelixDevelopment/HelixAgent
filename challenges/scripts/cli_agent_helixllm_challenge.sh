#!/bin/bash
# HelixAgent Challenge: CLI Agent HelixLLM Integration
# Tests all 48 CLI agents generate valid configs with HelixLLM provider,
# verifies HelixLLM HTTPS connectivity, validates MCP protocol handshakes,
# and confirms no false-positive responses.
#
# Prerequisites: HelixLLM running at https://thinker.local:8443

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"
BINARY="$PROJECT_ROOT/bin/helixagent"
TMP_DIR="/tmp/helixagent-cli-agent-challenge-$$"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

pass() {
    PASSED=$((PASSED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${GREEN}[PASS]${NC} $1"
}

fail() {
    FAILED=$((FAILED + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${RED}[FAIL]${NC} $1"
}

section() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$TMP_DIR"

echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE} HelixAgent Challenge: CLI Agent HelixLLM Integration${NC}"
echo -e "${BLUE}============================================================${NC}"
echo -e "Binary: $BINARY"
echo -e "Temp dir: $TMP_DIR"

# All 48 supported CLI agents
AGENTS=(
    opencode crush helixcode kilocode
    kiro aider claude-code cline codename-goose deepseek-cli
    forge gemini-cli gpt-engineer mistral-code ollama-code
    plandex qwen-code amazon-q
    agent-deck bridle cheshire-cat claude-plugins claude-squad
    codai codex codex-skills conduit continue emdash fauxpilot
    get-shit-done github-copilot-cli github-spec-kit git-mcp
    gptme mobile-agent multiagent-coding nanocoder noi octogen
    openhands postgres-mcp shai snow-cli task-weaver
    ui-ux-pro-max vtcode warp
)

#===============================================================================
# Section 1: Binary and Prerequisites (4 tests)
#===============================================================================
section "Section 1: Binary and Prerequisites"

if [ -x "$BINARY" ]; then
    pass "helixagent binary exists and is executable"
else
    fail "helixagent binary not found at $BINARY"
    echo -e "${RED}Cannot continue without binary. Build with: make build${NC}"
    exit 1
fi

# Test HelixLLM HTTPS health
HEALTH=$(curl -s --cacert "$PROJECT_ROOT/HelixLLM/certs/cert.pem" --max-time 5 https://thinker.local:8443/internal/health 2>/dev/null)
if echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['status']=='healthy'" 2>/dev/null; then
    pass "HelixLLM is healthy at https://thinker.local:8443"
else
    fail "HelixLLM not responding at https://thinker.local:8443"
    echo -e "${YELLOW}Warning: HelixLLM-specific tests will fail${NC}"
fi

# Test HelixLLM models endpoint
MODELS=$(curl -s --cacert "$PROJECT_ROOT/HelixLLM/certs/cert.pem" --max-time 5 https://thinker.local:8443/v1/models 2>/dev/null)
if echo "$MODELS" | python3 -c "import sys,json; d=json.load(sys.stdin); assert len(d.get('data',[]))>0" 2>/dev/null; then
    pass "HelixLLM has models available"
else
    fail "HelixLLM models endpoint returned no models"
fi

# Test HelixLLM chat completion (NOT a false positive — validate actual content)
CHAT_RESPONSE=$(curl -s --cacert "$PROJECT_ROOT/HelixLLM/certs/cert.pem" --max-time 15 -X POST https://thinker.local:8443/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"helixllm-default","messages":[{"role":"user","content":"What is 2+2? Reply with just the number."}],"max_tokens":10}' 2>/dev/null)
CHAT_CONTENT=$(echo "$CHAT_RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'])" 2>/dev/null)
if echo "$CHAT_CONTENT" | grep -q "4"; then
    pass "HelixLLM chat completion returns valid response (contains '4')"
else
    fail "HelixLLM chat completion invalid — got: '$CHAT_CONTENT'"
fi

#===============================================================================
# Section 2: Config Generation for ALL 48 Agents (48 tests)
#===============================================================================
section "Section 2: Config Generation for All ${#AGENTS[@]} Agents"

GENERATED=0
GEN_FAILED=0
for agent in "${AGENTS[@]}"; do
    CONFIG_FILE="$TMP_DIR/${agent}.json"
    if "$BINARY" --generate-agent-config="$agent" > "$CONFIG_FILE" 2>/dev/null; then
        if python3 -m json.tool "$CONFIG_FILE" > /dev/null 2>&1; then
            GENERATED=$((GENERATED + 1))
            pass "Config generated: $agent (valid JSON)"
        else
            GEN_FAILED=$((GEN_FAILED + 1))
            fail "Config generated but invalid JSON: $agent"
        fi
    else
        GEN_FAILED=$((GEN_FAILED + 1))
        fail "Config generation failed: $agent"
    fi
done

#===============================================================================
# Section 3: HelixLLM Provider in ALL Configs (48 tests)
#===============================================================================
section "Section 3: HelixLLM Provider Presence in All Configs"

for agent in "${AGENTS[@]}"; do
    CONFIG_FILE="$TMP_DIR/${agent}.json"
    [ ! -f "$CONFIG_FILE" ] && { fail "No config for $agent"; continue; }

    HAS_HELIXLLM=$(python3 -c "
import json
with open('$CONFIG_FILE') as f:
    d = json.load(f)
# Check in provider (OpenCode format)
if 'helixllm' in d.get('provider', {}):
    print('yes')
# Check in providers (generic/crush format)
elif 'helixllm' in d.get('providers', {}):
    print('yes')
# Check in additionalProviders (kilocode/helixcode format)
elif 'helixllm' in d.get('additionalProviders', d.get('additional_providers', {})):
    print('yes')
else:
    print('no')
" 2>/dev/null)

    if [ "$HAS_HELIXLLM" = "yes" ]; then
        pass "HelixLLM provider present: $agent"
    else
        fail "HelixLLM provider MISSING: $agent"
    fi
done

#===============================================================================
# Section 4: HelixLLM HTTPS URL Validation (48 tests)
#===============================================================================
section "Section 4: HelixLLM HTTPS URL in All Configs"

for agent in "${AGENTS[@]}"; do
    CONFIG_FILE="$TMP_DIR/${agent}.json"
    [ ! -f "$CONFIG_FILE" ] && { fail "No config for $agent"; continue; }

    URL_CHECK=$(python3 -c "
import json
with open('$CONFIG_FILE') as f:
    d = json.load(f)
# Find HelixLLM URL in various config formats
url = ''
for key in ['provider', 'providers', 'additionalProviders', 'additional_providers']:
    section = d.get(key, {})
    if 'helixllm' in section:
        hlm = section['helixllm']
        if isinstance(hlm, dict):
            url = hlm.get('options', hlm).get('baseURL', hlm.get('base_url', hlm.get('baseUrl', hlm.get('BaseURL', ''))))
            break
if 'https://' in url:
    print('https')
elif 'http://' in url:
    print('http')
elif url:
    print('other')
else:
    print('missing')
" 2>/dev/null)

    if [ "$URL_CHECK" = "https" ]; then
        pass "HTTPS URL: $agent"
    elif [ "$URL_CHECK" = "http" ]; then
        fail "HTTP (not HTTPS) URL: $agent"
    else
        fail "HelixLLM URL check failed ($URL_CHECK): $agent"
    fi
done

#===============================================================================
# Section 5: MCP Protocol Handshake Validation (20 tests)
#===============================================================================
section "Section 5: MCP Protocol Handshake for OpenCode MCPs"

# Extract MCP list from the OpenCode config
OPENCODE_CONFIG="$TMP_DIR/opencode.json"
if [ -f "$OPENCODE_CONFIG" ]; then
    MCPS=$(python3 -c "
import json
with open('$OPENCODE_CONFIG') as f:
    d = json.load(f)
for name, cfg in sorted(d.get('mcp', {}).items()):
    print(f'{name}|{cfg.get(\"type\",\"?\")}')
" 2>/dev/null)

    while IFS='|' read -r name mtype; do
        if [ "$mtype" = "local" ]; then
            # Extract command from config
            CMD=$(python3 -c "
import json
with open('$OPENCODE_CONFIG') as f:
    d = json.load(f)
mcp = d['mcp']['$name']
print(' '.join(mcp.get('command', [])))" 2>/dev/null)

            if [ -n "$CMD" ]; then
                INIT_RESULT=$(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | timeout 15 bash -c "$CMD" 2>/dev/null | head -1)
                if echo "$INIT_RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'result' in d" 2>/dev/null; then
                    pass "MCP handshake: $name (local)"
                else
                    fail "MCP handshake failed: $name (local)"
                fi
            else
                fail "No command for MCP: $name"
            fi
        elif [ "$mtype" = "remote" ]; then
            URL=$(python3 -c "
import json
with open('$OPENCODE_CONFIG') as f:
    d = json.load(f)
print(d['mcp']['$name'].get('url',''))" 2>/dev/null)
            STATUS=$(timeout 15 curl -s -o /dev/null -w "%{http_code}" "$URL" 2>/dev/null || echo "TIMEOUT")
            if [ "$STATUS" != "TIMEOUT" ] && [ "$STATUS" != "000" ]; then
                pass "MCP reachable: $name (remote, HTTP $STATUS)"
            else
                fail "MCP unreachable: $name (remote, $STATUS)"
            fi
        fi
    done <<< "$MCPS"
fi

#===============================================================================
# Section 6: HelixLLM Chat Completion from CLI Agent Config (5 tests)
#===============================================================================
section "Section 6: HelixLLM End-to-End Chat Validation"

# Test with different prompts to avoid false positives
TESTS=(
    "What is 7 times 8? Reply with just the number.|56"
    "What is the capital of France? Reply with just the city name.|Paris"
    "Is water H2O? Reply yes or no.|yes"
    "How many days in a week? Reply with just the number.|7"
    "What color is the sky on a clear day? One word.|blue"
)

for test_case in "${TESTS[@]}"; do
    PROMPT="${test_case%%|*}"
    EXPECTED="${test_case##*|}"

    RESPONSE=$(curl -s --cacert "$PROJECT_ROOT/HelixLLM/certs/cert.pem" --max-time 15 -X POST https://thinker.local:8443/v1/chat/completions \
        -H "Content-Type: application/json" \
        -d "{\"model\":\"helixllm-default\",\"messages\":[{\"role\":\"user\",\"content\":\"$PROMPT\"}],\"max_tokens\":20}" 2>/dev/null)

    CONTENT=$(echo "$RESPONSE" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'])" 2>/dev/null)

    if echo "$CONTENT" | grep -qi "$EXPECTED"; then
        pass "Chat validation: '$PROMPT' -> contains '$EXPECTED' (got: '$(echo $CONTENT | head -c 30)')"
    else
        fail "Chat validation: '$PROMPT' -> expected '$EXPECTED', got: '$CONTENT'"
    fi
done

#===============================================================================
# Summary
#===============================================================================
echo -e "\n${BLUE}============================================================${NC}"
echo -e "${BLUE} Challenge Results${NC}"
echo -e "${BLUE}============================================================${NC}"
echo -e "  Total tests: $TOTAL"
echo -e "  ${GREEN}Passed: $PASSED${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "  ${RED}Failed: $FAILED${NC}"
else
    echo -e "  Failed: 0"
fi
echo -e "  Agents tested: ${#AGENTS[@]}"
echo -e "  Configs generated: $GENERATED / ${#AGENTS[@]}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}ALL TESTS PASSED${NC}"
    exit 0
else
    echo -e "\n${RED}SOME TESTS FAILED${NC}"
    exit 1
fi
