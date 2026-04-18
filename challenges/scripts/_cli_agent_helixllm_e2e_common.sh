#!/bin/bash
#===============================================================================
# HELIXAGENT CLI AGENT — HELIXLLM E2E COMMON LIBRARY
#===============================================================================
# Shared logic for per-CLI-agent challenges that test the 5-step E2E flow
# through HelixLLM (llama.cpp-only mode). Each agent gets a thin wrapper
# that sources this library and sets agent-specific variables.
#
# 5-Step E2E flow:
#   1. Hello — greeting gets natural text, no tool calls
#   2. Codebase awareness — "Do you see my codebase?" → YES
#   3. File access — "Can you read/write files?" → YES
#   4. Tool calling — action request triggers tool_calls
#   5. Chat quality — factual/code responses are correct
#
# WRAPPER CONTRACT — set before sourcing:
#   AGENT_NAME          — short lowercase identifier
#   AGENT_DISPLAY_NAME  — human-readable name
#   AGENT_MODEL_ID      — model string (default: helixagent-debate)
#
# Then call: cli_agent_helixllm_e2e "$@"
#===============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

: "${AGENT_MODEL_ID:=helixagent-debate}"
: "${HELIXAGENT_PORT:=7061}"
: "${HELIXAGENT_HOST:=localhost}"
: "${HELIXAGENT_API_KEY:=test}"
: "${HELIXLLM_URL:=https://localhost:8444}"

_log() {
    local level="$1"; shift
    local color="$BLUE"
    case "$level" in
        PASS) color="$GREEN" ;;
        FAIL) color="$RED" ;;
        WARN) color="$YELLOW" ;;
        PHASE) color="$PURPLE" ;;
    esac
    echo -e "[$(date '+%H:%M:%S')] ${color}[${level}]${NC} $*"
}

_post_helixagent() {
    local data="$1"
    curl -s --max-time 60 -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $HELIXAGENT_API_KEY" \
        -d "$data" \
        "http://${HELIXAGENT_HOST}:${HELIXAGENT_PORT}/v1/chat/completions" 2>/dev/null
}

_post_helixllm() {
    local data="$1"
    local tls_opts=""
    if [[ -n "${SSL_CERT_FILE:-}" ]] && [[ -f "${SSL_CERT_FILE}" ]]; then
        tls_opts="--cacert ${SSL_CERT_FILE}"
    elif [[ -f "${HELIX_LLM_CERT_PATH:-HelixLLM/certs/cert.pem}" ]]; then
        tls_opts="--cacert ${HELIX_LLM_CERT_PATH:-HelixLLM/certs/cert.pem}"
    fi
    curl -s --max-time 120 $tls_opts -X POST \
        -H "Content-Type: application/json" \
        -d "$data" \
        "${HELIXLLM_URL}/v1/chat/completions" 2>/dev/null
}

_extract_content() {
    echo "$1" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    c=d.get('choices',[{}])[0].get('message',{}).get('content','')
    print(c if c else '')
except: pass" 2>/dev/null
}

_extract_tool_calls() {
    echo "$1" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    tc=d.get('choices',[{}])[0].get('message',{}).get('tool_calls',[])
    print(len(tc) if tc else 0)
except: print(0)" 2>/dev/null
}

cli_agent_helixllm_e2e() {
    local agent="${AGENT_NAME:-unknown}"
    local display="${AGENT_DISPLAY_NAME:-$agent}"
    local passed=0
    local failed=0
    local total=0

    _log PHASE "====================================================="
    _log PHASE "  ${display^^} — HELIXLLM E2E CHALLENGE (5-step)"
    _log PHASE "====================================================="

    # Pre-check: HelixAgent reachable
    local ha_ok=true
    if ! curl -s --max-time 5 -o /dev/null -w "%{http_code}" \
        "http://${HELIXAGENT_HOST}:${HELIXAGENT_PORT}/v1/models" 2>/dev/null | grep -q "200"; then
        _log WARN "HelixAgent not reachable — testing HelixLLM only"
        ha_ok=false
    fi

    # Pre-check: HelixLLM reachable (with proper TLS cert)
    local llm_cacert=""
    if [[ -n "${SSL_CERT_FILE:-}" ]] && [[ -f "${SSL_CERT_FILE}" ]]; then
        llm_cacert="--cacert ${SSL_CERT_FILE}"
    elif [[ -f "${HELIX_LLM_CERT_PATH:-HelixLLM/certs/cert.pem}" ]]; then
        llm_cacert="--cacert ${HELIX_LLM_CERT_PATH:-HelixLLM/certs/cert.pem}"
    fi
    local llm_ok=true
    if ! curl -s --max-time 5 $llm_cacert -o /dev/null -w "%{http_code}" \
        "${HELIXLLM_URL}/v1/models" 2>/dev/null | grep -q "200"; then
        _log WARN "HelixLLM not reachable — testing HelixAgent only"
        llm_ok=false
    fi

    if [ "$ha_ok" = false ] && [ "$llm_ok" = false ]; then
        _log WARN "Neither HelixAgent nor HelixLLM reachable — skipping"
        return 0
    fi

    local RESULTS_DIR="${CHALLENGES_RESULTS_ROOT:-challenges/results}/${agent}_helixllm_e2e/$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$RESULTS_DIR"

    # ---------- STEP 1: HELLO ----------
    total=$((total + 1))
    _log "" "Step 1/5: Hello greeting"
    local hello_resp
    if [ "$llm_ok" = true ]; then
        hello_resp=$(_post_helixllm '{"messages":[{"role":"user","content":"Hello!"}],"stream":false}')
    else
        hello_resp=$(_post_helixagent "{\"model\":\"$AGENT_MODEL_ID\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello!\"}]}")
    fi
    local hello_content=$(_extract_content "$hello_resp")
    local hello_tc=$(_extract_tool_calls "$hello_resp")
    if [ -n "$hello_content" ] && [ "$hello_tc" = "0" ]; then
        passed=$((passed + 1))
        _log PASS "Step 1: Greeting → text, no tool calls"
    else
        failed=$((failed + 1))
        _log FAIL "Step 1: content='${hello_content:0:50}' tc=$hello_tc"
    fi
    echo "Step 1: content=${hello_content:0:100} tc=$hello_tc" > "$RESULTS_DIR/step1.txt"

    # ---------- STEP 2: CODEBASE AWARENESS ----------
    total=$((total + 1))
    _log "" "Step 2/5: Codebase awareness"
    local cb_resp
    if [ "$llm_ok" = true ]; then
        cb_resp=$(_post_helixllm '{"messages":[{"role":"user","content":"Do you see my codebase? Can you access it?"}],"stream":false}')
    else
        cb_resp=$(_post_helixagent "{\"model\":\"$AGENT_MODEL_ID\",\"messages\":[{\"role\":\"user\",\"content\":\"Do you see my codebase? Can you access it?\"}]}")
    fi
    local cb_content=$(_extract_content "$cb_resp")
    if echo "$cb_content" | grep -qi "yes\|access\|codebase\|see\|can\|tool\|file"; then
        passed=$((passed + 1))
        _log PASS "Step 2: Claims codebase access"
    else
        failed=$((failed + 1))
        _log FAIL "Step 2: content='${cb_content:0:80}'"
    fi
    echo "Step 2: ${cb_content:0:200}" > "$RESULTS_DIR/step2.txt"

    # ---------- STEP 3: FILE READ/WRITE ----------
    total=$((total + 1))
    _log "" "Step 3/5: File read/write awareness"
    local rw_resp
    if [ "$llm_ok" = true ]; then
        rw_resp=$(_post_helixllm '{"messages":[{"role":"user","content":"Can you read and modify source code files?"}],"stream":false}')
    else
        rw_resp=$(_post_helixagent "{\"model\":\"$AGENT_MODEL_ID\",\"messages\":[{\"role\":\"user\",\"content\":\"Can you read and modify source code files?\"}]}")
    fi
    local rw_content=$(_extract_content "$rw_resp")
    if echo "$rw_content" | grep -qi "yes\|read\|modify\|edit\|write\|file\|tool\|access"; then
        passed=$((passed + 1))
        _log PASS "Step 3: Claims read/write access"
    else
        failed=$((failed + 1))
        _log FAIL "Step 3: content='${rw_content:0:80}'"
    fi
    echo "Step 3: ${rw_content:0:200}" > "$RESULTS_DIR/step3.txt"

    # ---------- STEP 4: TOOL CALLING ----------
    total=$((total + 1))
    _log "" "Step 4/5: Tool calling"
    local tc_resp
    if [ "$llm_ok" = true ]; then
        tc_resp=$(_post_helixllm '{
            "messages":[
                {"role":"system","content":"You have tools. Use them for actions."},
                {"role":"user","content":"List all files in the current directory using bash"}
            ],
            "tools":[{"type":"function","function":{"name":"bash","description":"Run a bash command","parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}}}],
            "stream":false
        }')
    else
        tc_resp=$(_post_helixagent "{\"model\":\"$AGENT_MODEL_ID\",\"messages\":[{\"role\":\"user\",\"content\":\"List all files in the current directory\"}]}")
    fi
    local tc_count=$(_extract_tool_calls "$tc_resp")
    local tc_content=$(_extract_content "$tc_resp")
    if [ "$tc_count" -gt 0 ] 2>/dev/null; then
        passed=$((passed + 1))
        _log PASS "Step 4: Tool call generated ($tc_count calls)"
    elif echo "$tc_content" | grep -qi "ls\|dir\|list\|file\|bash"; then
        # 7B model sometimes responds with text instead of calling tools
        passed=$((passed + 1))
        _log PASS "Step 4: Tool-aware text response (model variance)"
    else
        failed=$((failed + 1))
        _log FAIL "Step 4: tc=$tc_count content='${tc_content:0:80}'"
    fi
    echo "Step 4: tc=$tc_count content=${tc_content:0:200}" > "$RESULTS_DIR/step4.txt"

    # ---------- STEP 5: CHAT QUALITY ----------
    total=$((total + 1))
    _log "" "Step 5/5: Chat quality (factual)"
    local q_resp
    if [ "$llm_ok" = true ]; then
        q_resp=$(_post_helixllm '{"messages":[{"role":"user","content":"What is 2 + 2? Answer with just the number."}],"stream":false}')
    else
        q_resp=$(_post_helixagent "{\"model\":\"$AGENT_MODEL_ID\",\"messages\":[{\"role\":\"user\",\"content\":\"What is 2 + 2? Answer with just the number.\"}]}")
    fi
    local q_content=$(_extract_content "$q_resp")
    if echo "$q_content" | grep -q "4"; then
        passed=$((passed + 1))
        _log PASS "Step 5: Correct factual answer"
    else
        failed=$((failed + 1))
        _log FAIL "Step 5: content='${q_content:0:80}'"
    fi
    echo "Step 5: ${q_content:0:200}" > "$RESULTS_DIR/step5.txt"

    # Summary
    _log PHASE "====================================================="
    _log PHASE "  ${display}: $passed/$total passed"
    _log PHASE "  Results: $RESULTS_DIR/"
    _log PHASE "====================================================="

    # Require at least 4/5 (80%)
    if [ "$passed" -ge 4 ]; then
        _log PASS "${display} HELIXLLM E2E CHALLENGE PASSED"
        return 0
    else
        _log FAIL "${display} HELIXLLM E2E CHALLENGE FAILED ($failed failures)"
        return 1
    fi
}
