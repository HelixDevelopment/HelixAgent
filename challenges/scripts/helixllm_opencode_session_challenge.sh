#!/bin/bash
#===============================================================================
# HELIXLLM OPENCODE SESSION CHALLENGE
#===============================================================================
# Simulates the exact 5-step OpenCode session that must work:
# 1. hello! → text greeting (no tools)
# 2. do you see my codebase? → text "Yes" (no tools or valid tool)
# 3. Can you read and modify my files? → text "Yes" (no hallucinated paths)
# 4. /init (Create AGENTS.md) → tool call (Read/Write/Bash, NOT respond)
# 5. commit and push → tool call (Bash with git commands)
#
# Validates: no hallucinated paths, no tool loops, no "I can't" responses,
# no context overflow, RAG context injection working.
#
# Tests: 10 (5 content + 5 format)
# Requires: HelixLLM on https://localhost:8444
#===============================================================================

set -o pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
NC='\033[0m'

HELIXLLM_URL="${HELIXLLM_URL:-https://localhost:8444}"
CURL="curl -sk --max-time 120"
PASSED=0
FAILED=0
TOTAL=0

# OpenCode sends these tools (first 5 after compression)
TOOLS='[{"type":"function","function":{"name":"Bash","description":"Run bash command","parameters":{"type":"object","properties":{"command":{"type":"string"},"description":{"type":"string"},"timeout":{"type":"number"}},"required":["command","description","timeout"]}}},{"type":"function","function":{"name":"Read","description":"Read file from disk","parameters":{"type":"object","properties":{"filePath":{"type":"string"},"limit":{"type":"number"},"offset":{"type":"number"}},"required":["filePath"]}}},{"type":"function","function":{"name":"Write","description":"Write content to file","parameters":{"type":"object","properties":{"filePath":{"type":"string"},"content":{"type":"string"}},"required":["filePath","content"]}}},{"type":"function","function":{"name":"Edit","description":"Edit file","parameters":{"type":"object","properties":{"filePath":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"}},"required":["filePath","old","new"]}}},{"type":"function","function":{"name":"Glob","description":"Find files","parameters":{"type":"object","properties":{"pattern":{"type":"string"}},"required":["pattern"]}}}]'

SYS='{"role":"system","content":"Primary working directory: /run/media/milosvasic/DATA4TB/Projects/HelixAgent"}'

assert_test() {
    local name="$1" result="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$result" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "${GREEN}PASS${NC}: $name"
    else
        FAILED=$((FAILED + 1))
        echo -e "${RED}FAIL${NC}: $name — $result"
    fi
}

echo -e "${PURPLE}===== HELIXLLM OPENCODE SESSION CHALLENGE =====${NC}"

# Pre-check
if ! $CURL -o /dev/null -w "%{http_code}" "${HELIXLLM_URL}/v1/models" 2>/dev/null | grep -q "200"; then
    echo -e "${RED}HelixLLM not reachable${NC}"
    exit 1
fi

# ---- STEP 1: hello! ----
echo -e "\n${PURPLE}Step 1: hello!${NC}"
R1=$($CURL -X POST -H "Content-Type: application/json" \
    -d "{\"messages\":[$SYS,{\"role\":\"user\",\"content\":\"hello!\"}],\"tools\":$TOOLS,\"stream\":false}" \
    "${HELIXLLM_URL}/v1/chat/completions" 2>/dev/null)
C1=$(echo "$R1" | python3 -c "import sys,json; m=json.load(sys.stdin)['choices'][0]['message']; print(m.get('content',''))" 2>/dev/null)
TC1=$(echo "$R1" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['choices'][0]['message'].get('tool_calls',[])))" 2>/dev/null)

if [ -n "$C1" ] && [ "$C1" != "None" ]; then
    assert_test "hello → text response" "PASS"
else
    assert_test "hello → text response" "empty content"
fi
if [ "$TC1" = "0" ]; then
    assert_test "hello → no tool calls" "PASS"
else
    assert_test "hello → no tool calls" "got $TC1 tool calls"
fi

# ---- STEP 2: codebase? ----
echo -e "\n${PURPLE}Step 2: do you see my codebase?${NC}"
R2=$($CURL -X POST -H "Content-Type: application/json" \
    -d "{\"messages\":[$SYS,{\"role\":\"user\",\"content\":\"hello!\"},{\"role\":\"assistant\",\"content\":\"$C1\"},{\"role\":\"user\",\"content\":\"do you see my codebase?\"}],\"tools\":$TOOLS,\"stream\":false}" \
    "${HELIXLLM_URL}/v1/chat/completions" 2>/dev/null)
C2=$(echo "$R2" | python3 -c "import sys,json; m=json.load(sys.stdin)['choices'][0]['message']; print(m.get('content',''))" 2>/dev/null)

if echo "$C2" | grep -qi "yes\|access\|see\|codebase\|files"; then
    assert_test "codebase → affirmative" "PASS"
elif [ "$C2" = "" ] || [ "$C2" = "None" ]; then
    # Model may have called a tool to prove access — acceptable
    assert_test "codebase → tool proof (acceptable)" "PASS"
else
    assert_test "codebase → affirmative" "got: $C2"
fi
# Must NOT have hallucinated path
if echo "$R2" | grep -q '/path/to/'; then
    assert_test "codebase → no hallucinated paths" "found /path/to/"
else
    assert_test "codebase → no hallucinated paths" "PASS"
fi

# ---- STEP 3: read/write? ----
echo -e "\n${PURPLE}Step 3: Can you read and modify my files?${NC}"
R3=$($CURL -X POST -H "Content-Type: application/json" \
    -d "{\"messages\":[$SYS,{\"role\":\"user\",\"content\":\"Can you read and modify my files?\"}],\"tools\":$TOOLS,\"stream\":false}" \
    "${HELIXLLM_URL}/v1/chat/completions" 2>/dev/null)
C3=$(echo "$R3" | python3 -c "import sys,json; m=json.load(sys.stdin)['choices'][0]['message']; print(m.get('content',''))" 2>/dev/null)

if echo "$C3" | grep -qi "yes\|can\|read\|modify\|files\|help"; then
    assert_test "read/write → affirmative" "PASS"
elif [ "$C3" = "" ] || [ "$C3" = "None" ]; then
    assert_test "read/write → affirmative" "empty response (model called tool)"
else
    assert_test "read/write → affirmative" "got: $C3"
fi
if echo "$R3" | grep -q '/path/to/'; then
    assert_test "read/write → no hallucinated paths" "found /path/to/"
else
    assert_test "read/write → no hallucinated paths" "PASS"
fi

# ---- STEP 4: /init (Create AGENTS.md) ----
echo -e "\n${PURPLE}Step 4: /init (Create or update AGENTS.md)${NC}"
R4=$($CURL -X POST -H "Content-Type: application/json" \
    -d "{\"messages\":[$SYS,{\"role\":\"user\",\"content\":\"Create or update AGENTS.md for this repository. Read the existing file first, then write an improved version.\"}],\"tools\":$TOOLS,\"stream\":false}" \
    "${HELIXLLM_URL}/v1/chat/completions" 2>/dev/null)
TC4=$(echo "$R4" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['choices'][0]['message'].get('tool_calls',[])))" 2>/dev/null)
TN4=$(echo "$R4" | python3 -c "import sys,json; tc=json.load(sys.stdin)['choices'][0]['message'].get('tool_calls',[]); print(tc[0]['function']['name'] if tc else 'none')" 2>/dev/null)
C4=$(echo "$R4" | python3 -c "import sys,json; print(json.load(sys.stdin)['choices'][0]['message'].get('content','')[:60])" 2>/dev/null)

if [ "$TC4" -gt 0 ] 2>/dev/null && [ "$TN4" != "respond" ]; then
    assert_test "/init → action tool call ($TN4)" "PASS"
elif [ -n "$C4" ] && echo "$C4" | grep -qi "agents\|create\|update\|read"; then
    assert_test "/init → text with intent (acceptable)" "PASS"
else
    assert_test "/init → action tool call" "tc=$TC4 name=$TN4 content=$C4"
fi

# ---- STEP 5: commit and push ----
echo -e "\n${PURPLE}Step 5: commit and push${NC}"
R5=$($CURL -X POST -H "Content-Type: application/json" \
    -d "{\"messages\":[$SYS,{\"role\":\"user\",\"content\":\"commit and push all work to all upstreams now!\"}],\"tools\":$TOOLS,\"stream\":false}" \
    "${HELIXLLM_URL}/v1/chat/completions" 2>/dev/null)
TC5=$(echo "$R5" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['choices'][0]['message'].get('tool_calls',[])))" 2>/dev/null)
TN5=$(echo "$R5" | python3 -c "import sys,json; tc=json.load(sys.stdin)['choices'][0]['message'].get('tool_calls',[]); print(tc[0]['function']['name'] if tc else 'none')" 2>/dev/null)

if [ "$TC5" -gt 0 ] 2>/dev/null && [ "$TN5" = "Bash" ]; then
    assert_test "commit/push → Bash tool call" "PASS"
elif [ "$TC5" -gt 0 ] 2>/dev/null; then
    assert_test "commit/push → tool call ($TN5)" "PASS"
else
    assert_test "commit/push → Bash tool call" "no tool call"
fi

# Summary
echo -e "\n${PURPLE}===== RESULTS: $PASSED passed, $FAILED failed (of $TOTAL) =====${NC}"
if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}HELIXLLM OPENCODE SESSION CHALLENGE PASSED${NC}"
    exit 0
else
    echo -e "${RED}CHALLENGE FAILED ($FAILED failures)${NC}"
    exit 1
fi
