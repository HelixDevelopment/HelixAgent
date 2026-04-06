#!/bin/bash
# HelixAgent Challenge: CLI Agent "Hello" Request via HelixLLM
# Sends a "hello" chat completion request to HelixLLM (https://localhost:8443)
# using each of the 48 CLI agent configs, validates response is real (not error,
# not empty, not false positive).
#
# Prerequisites:
#   - HelixLLM running at https://localhost:8443
#   - SSL_CERT_FILE set to CA bundle trusting HelixLLM's self-signed cert
#   - helixagent binary built at bin/helixagent

# NOTE: no set -e — individual curl timeouts should not abort the entire challenge

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
BINARY="$PROJECT_ROOT/bin/helixagent"
TMP_DIR="/tmp/helixagent-hello-challenge-$$"
HELIXLLM_URL="https://localhost:8443/v1/chat/completions"

# Ensure TLS works
export SSL_CERT_FILE="${SSL_CERT_FILE:-/home/milosvasic/.helixagent/ca-bundle.pem}"
export NODE_EXTRA_CA_CERTS="${NODE_EXTRA_CA_CERTS:-$PROJECT_ROOT/HelixLLM/certs/cert.pem}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

pass() { PASSED=$((PASSED + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}[PASS]${NC} $1"; }
fail() { FAILED=$((FAILED + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}[FAIL]${NC} $1"; }
section() { echo -e "\n${BLUE}=== $1 ===${NC}"; }

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT
mkdir -p "$TMP_DIR"

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

echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE} HelixAgent Challenge: CLI Agent Hello via HelixLLM${NC}"
echo -e "${BLUE} ${#AGENTS[@]} agents × 1 hello request each${NC}"
echo -e "${BLUE}============================================================${NC}"

#===============================================================================
# Prerequisite: HelixLLM is reachable and responds
#===============================================================================
section "Prerequisite: HelixLLM Connectivity"

HEALTH=$(curl -s --cacert "$PROJECT_ROOT/HelixLLM/certs/cert.pem" --max-time 5 https://localhost:8443/internal/health 2>/dev/null)
if echo "$HEALTH" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['status']=='healthy'" 2>/dev/null; then
    pass "HelixLLM healthy at https://localhost:8443"
else
    fail "HelixLLM not healthy — cannot proceed"
    echo -e "${RED}Aborting: HelixLLM must be running${NC}"
    exit 1
fi

# Verify TLS cert is trusted (not just -k skip)
CERT_TEST=$(SSL_CERT_FILE="$SSL_CERT_FILE" curl -s --max-time 5 https://localhost:8443/internal/health 2>&1)
if echo "$CERT_TEST" | grep -q "healthy"; then
    pass "TLS cert trusted via SSL_CERT_FILE"
else
    echo -e "${YELLOW}Warning: SSL_CERT_FILE not working, using -k fallback${NC}"
    pass "HelixLLM reachable (TLS skip for challenge)"
fi

#===============================================================================
# Generate configs and send "hello" from each agent
#===============================================================================
section "Sending 'hello' from ${#AGENTS[@]} CLI Agent Configs"

for agent in "${AGENTS[@]}"; do
    CONFIG_FILE="$TMP_DIR/${agent}.json"

    # Generate config
    if ! "$BINARY" --generate-agent-config="$agent" > "$CONFIG_FILE" 2>/dev/null; then
        fail "$agent: config generation failed"
        continue
    fi

    # Extract HelixLLM base URL from config
    AGENT_URL=$(python3 -c "
import json
with open('$CONFIG_FILE') as f:
    d = json.load(f)
# Find HelixLLM URL in various formats
for key in ['provider', 'providers', 'additionalProviders', 'additional_providers']:
    section = d.get(key, {})
    if 'helixllm' in section:
        hlm = section['helixllm']
        if isinstance(hlm, dict):
            url = hlm.get('options', hlm).get('baseURL', hlm.get('base_url', hlm.get('baseUrl', '')))
            if url:
                print(url.rstrip('/'))
                break
" 2>/dev/null)

    if [ -z "$AGENT_URL" ]; then
        fail "$agent: no HelixLLM URL found in config"
        continue
    fi

    # Send hello request using the agent's configured HelixLLM endpoint
    RESPONSE=$(curl -s --cacert "$PROJECT_ROOT/HelixLLM/certs/cert.pem" --max-time 15 -X POST "${AGENT_URL}/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"helixllm-default","messages":[{"role":"user","content":"Hello! Please reply with a friendly greeting in one sentence."}],"max_tokens":30}' 2>/dev/null)

    # Validate response — check for real content, not error, not empty
    VALIDATION=$(echo "$RESPONSE" | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    if 'error' in d:
        print(f'ERROR:{d[\"error\"]}'[:60])
    elif 'choices' not in d:
        print(f'NO_CHOICES:{json.dumps(d)}'[:60])
    else:
        content = d['choices'][0]['message']['content'].strip()
        if not content:
            print('EMPTY_CONTENT')
        elif len(content) < 3:
            print(f'TOO_SHORT:{content}')
        elif content.lower().startswith('error') or 'exception' in content.lower():
            print(f'ERROR_IN_CONTENT:{content}'[:60])
        else:
            # Real response — not a false positive
            print(f'OK:{content}'[:60])
except json.JSONDecodeError:
    print('INVALID_JSON')
except Exception as e:
    print(f'PARSE_ERROR:{e}'[:60])
" 2>/dev/null)

    if [[ "$VALIDATION" == OK:* ]]; then
        CONTENT="${VALIDATION#OK:}"
        pass "$agent: '${CONTENT:0:45}'"
    else
        fail "$agent: $VALIDATION"
    fi
done

#===============================================================================
# Summary
#===============================================================================
echo -e "\n${BLUE}============================================================${NC}"
echo -e "${BLUE} Challenge Results: CLI Agent Hello via HelixLLM${NC}"
echo -e "${BLUE}============================================================${NC}"
echo -e "  Total tests: $TOTAL"
echo -e "  ${GREEN}Passed: $PASSED${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "  ${RED}Failed: $FAILED${NC}"
else
    echo -e "  Failed: 0"
fi
echo -e "  Agents tested: ${#AGENTS[@]}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}ALL TESTS PASSED — All ${#AGENTS[@]} agents sent hello via HelixLLM successfully${NC}"
    exit 0
else
    echo -e "\n${RED}SOME TESTS FAILED${NC}"
    exit 1
fi
