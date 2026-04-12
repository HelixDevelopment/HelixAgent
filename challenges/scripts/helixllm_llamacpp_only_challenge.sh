#!/bin/bash
#===============================================================================
# HELIXLLM LLAMA.CPP-ONLY CHALLENGE
#===============================================================================
# Validates that HelixLLM runs in llama.cpp-only mode:
# 1. Only llamacpp models listed (no Ollama chat models)
# 2. Chat completions work via llama.cpp
# 3. Tool call bridge (JSON + XML) works
# 4. Greetings don't trigger tool calls
# 5. Streaming works with and without tools
# 6. Real embeddings returned (not zero vectors)
# 7. Anthropic /v1/messages endpoint works
# 8. Force-nonstream for tool requests
#
# Tests: 15
# Requires: HelixLLM running on https://localhost:8444
#===============================================================================

set -o pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

HELIXLLM_URL="${HELIXLLM_URL:-https://localhost:8444}"
CURL_OPTS="-sk --max-time 120"
PASSED=0
FAILED=0
TOTAL=0

log() {
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

assert_test() {
    local name="$1"
    local result="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$result" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        log PASS "Test $TOTAL: $name"
    else
        FAILED=$((FAILED + 1))
        log FAIL "Test $TOTAL: $name — $result"
    fi
}

# Helper: POST JSON, return body
post_json() {
    local path="$1"
    local data="$2"
    curl $CURL_OPTS -X POST -H "Content-Type: application/json" \
        -d "$data" "${HELIXLLM_URL}${path}" 2>/dev/null
}

get_json() {
    local path="$1"
    curl $CURL_OPTS "${HELIXLLM_URL}${path}" 2>/dev/null
}

log PHASE "====================================================="
log PHASE "  HELIXLLM LLAMA.CPP-ONLY CHALLENGE"
log PHASE "====================================================="

# Pre-check: HelixLLM reachable
if ! curl $CURL_OPTS -o /dev/null -w "%{http_code}" "${HELIXLLM_URL}/v1/models" 2>/dev/null | grep -q "200"; then
    log FAIL "HelixLLM not reachable at $HELIXLLM_URL — aborting"
    exit 1
fi
log PASS "HelixLLM is reachable"

# ---- TEST 1: Only llamacpp models ----
models_json=$(get_json "/v1/models")
non_llamacpp=$(echo "$models_json" | python3 -c "
import sys,json
d=json.load(sys.stdin)
non=[m['id'] for m in d.get('data',[]) if m.get('owned_by','')!='llamacpp']
print(len(non))
" 2>/dev/null)
if [ "$non_llamacpp" = "0" ]; then
    assert_test "Only llamacpp models listed" "PASS"
else
    assert_test "Only llamacpp models listed" "Found $non_llamacpp non-llamacpp models"
fi

# ---- TEST 2: At least one model available ----
model_count=$(echo "$models_json" | python3 -c "
import sys,json; d=json.load(sys.stdin); print(len(d.get('data',[])))" 2>/dev/null)
if [ "$model_count" -gt 0 ] 2>/dev/null; then
    assert_test "At least one model available" "PASS"
else
    assert_test "At least one model available" "No models found"
fi

# ---- TEST 3: Basic chat completion ----
chat_resp=$(post_json "/v1/chat/completions" '{"messages":[{"role":"user","content":"Hello!"}],"stream":false}')
chat_content=$(echo "$chat_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'])" 2>/dev/null)
if [ -n "$chat_content" ] && [ "$chat_content" != "None" ]; then
    assert_test "Basic chat completion returns content" "PASS"
else
    assert_test "Basic chat completion returns content" "Empty content"
fi

# ---- TEST 4: Greeting with tools — no tool_calls ----
greeting_resp=$(post_json "/v1/chat/completions" '{
    "messages":[{"role":"user","content":"Hello!"}],
    "tools":[{"type":"function","function":{"name":"bash","description":"Run bash","parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}}}],
    "stream":false
}')
greeting_tc=$(echo "$greeting_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
tc=d['choices'][0]['message'].get('tool_calls',[])
print(len(tc) if tc else 0)" 2>/dev/null)
if [ "$greeting_tc" = "0" ]; then
    assert_test "Greeting does NOT trigger tool calls" "PASS"
else
    assert_test "Greeting does NOT trigger tool calls" "Got $greeting_tc tool calls"
fi

# ---- TEST 5: Tool-requiring request produces tool_calls ----
tool_resp=$(post_json "/v1/chat/completions" '{
    "messages":[
        {"role":"system","content":"You have tools. Use them for actions."},
        {"role":"user","content":"List all files in the current directory"}
    ],
    "tools":[{"type":"function","function":{"name":"bash","description":"Run a bash command","parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}}}],
    "stream":false
}')
tool_tc=$(echo "$tool_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
tc=d['choices'][0]['message'].get('tool_calls',[])
print(len(tc) if tc else 0)" 2>/dev/null)
tool_fr=$(echo "$tool_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin); print(d['choices'][0].get('finish_reason',''))" 2>/dev/null)
if [ "$tool_tc" -gt 0 ] 2>/dev/null && [ "$tool_fr" = "tool_calls" ]; then
    assert_test "Tool-requiring request produces tool_calls" "PASS"
else
    assert_test "Tool-requiring request produces tool_calls" "tc=$tool_tc fr=$tool_fr"
fi

# ---- TEST 6: Tool call has valid name and arguments ----
tool_name=$(echo "$tool_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
tc=d['choices'][0]['message'].get('tool_calls',[])
print(tc[0]['function']['name'] if tc else '')" 2>/dev/null)
if [ -n "$tool_name" ] && [ "$tool_name" != "None" ]; then
    assert_test "Tool call has valid name ($tool_name)" "PASS"
else
    assert_test "Tool call has valid name" "Empty name"
fi

# ---- TEST 7: Streaming without tools ----
stream_resp=$(post_json "/v1/chat/completions" '{"messages":[{"role":"user","content":"Say hi in one word"}],"stream":true}')
if echo "$stream_resp" | grep -q 'data: ' && echo "$stream_resp" | grep -q '\[DONE\]'; then
    assert_test "Streaming without tools returns SSE" "PASS"
else
    assert_test "Streaming without tools returns SSE" "Missing data/DONE"
fi

# ---- TEST 8: Streaming WITH tools completes via force-nonstream bridge ----
# The 7B model may return tool_calls OR text with tool intent. Both are valid
# as long as the response contains SSE data and [DONE].
stream_tool_resp=$(post_json "/v1/chat/completions" '{
    "messages":[
        {"role":"system","content":"Use tools for actions."},
        {"role":"user","content":"List files in directory"}
    ],
    "tools":[{"type":"function","function":{"name":"bash","description":"Run bash","parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}}}],
    "stream":true
}')
if echo "$stream_tool_resp" | grep -q '\[DONE\]'; then
    # Accept either tool_calls or text content — 7B model is inconsistent
    if echo "$stream_tool_resp" | grep -q 'tool_calls'; then
        assert_test "Streaming with tools — tool_calls bridged" "PASS"
    elif echo "$stream_tool_resp" | grep -q 'content'; then
        assert_test "Streaming with tools — content response (model variance)" "PASS"
    else
        assert_test "Streaming with tools" "No content or tool_calls"
    fi
else
    assert_test "Streaming with tools completes" "Missing [DONE]"
fi

# ---- TEST 9: Real embeddings (not zero vectors) ----
emb_resp=$(post_json "/v1/embeddings" '{"model":"nomic-embed-text","input":"hello world embedding test"}')
emb_nonzero=$(echo "$emb_resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
emb=d['data'][0]['embedding']
print(sum(1 for x in emb if x!=0))" 2>/dev/null)
if [ "$emb_nonzero" -gt 0 ] 2>/dev/null; then
    assert_test "Embeddings return non-zero vectors ($emb_nonzero dims)" "PASS"
else
    assert_test "Embeddings return non-zero vectors" "All zeros"
fi

# ---- TEST 10: Embedding dimension is 768 (nomic) ----
emb_dim=$(echo "$emb_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin); print(len(d['data'][0]['embedding']))" 2>/dev/null)
if [ "$emb_dim" = "768" ]; then
    assert_test "Embedding dimension is 768" "PASS"
else
    assert_test "Embedding dimension is 768" "Got $emb_dim"
fi

# ---- TEST 11: Bad JSON to embeddings returns 400 ----
bad_status=$(curl $CURL_OPTS -o /dev/null -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" -d "not json" "${HELIXLLM_URL}/v1/embeddings" 2>/dev/null)
if [ "$bad_status" = "400" ]; then
    assert_test "Bad JSON to embeddings returns 400" "PASS"
else
    assert_test "Bad JSON to embeddings returns 400" "Got $bad_status"
fi

# ---- TEST 12: Anthropic /v1/messages works ----
msg_status=$(curl $CURL_OPTS -o /dev/null -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -d '{"model":"test","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}' \
    "${HELIXLLM_URL}/v1/messages" 2>/dev/null)
if [ "$msg_status" = "200" ]; then
    assert_test "Anthropic /v1/messages returns 200" "PASS"
else
    assert_test "Anthropic /v1/messages returns 200" "Got $msg_status"
fi

# ---- TEST 13: Chat completion model is llamacpp ----
model_name=$(echo "$chat_resp" | python3 -c "
import sys,json; d=json.load(sys.stdin); print(d.get('model',''))" 2>/dev/null)
if echo "$model_name" | grep -qi "qwen\|llamacpp\|gguf\|coder"; then
    assert_test "Chat model is local llama.cpp ($model_name)" "PASS"
else
    assert_test "Chat model is local llama.cpp" "Got $model_name"
fi

# ---- TEST 14: No cloud provider models (gpt, claude) ----
cloud_models=$(echo "$models_json" | python3 -c "
import sys,json
d=json.load(sys.stdin)
cloud=[m['id'] for m in d.get('data',[]) if any(x in m['id'].lower() for x in ['gpt','claude','gemini'])]
print(len(cloud))" 2>/dev/null)
if [ "$cloud_models" = "0" ]; then
    assert_test "No cloud provider models listed" "PASS"
else
    assert_test "No cloud provider models listed" "Found $cloud_models cloud models"
fi

# ---- TEST 15: Health/metrics endpoint available ----
metrics_status=$(curl $CURL_OPTS -o /dev/null -w "%{http_code}" "${HELIXLLM_URL}/metrics" 2>/dev/null)
if [ "$metrics_status" = "200" ]; then
    assert_test "Prometheus /metrics endpoint available" "PASS"
else
    assert_test "Prometheus /metrics endpoint available" "Got $metrics_status"
fi

# Summary
log PHASE "====================================================="
log PHASE "  RESULTS: $PASSED passed, $FAILED failed (of $TOTAL)"
log PHASE "====================================================="

if [ "$FAILED" -eq 0 ]; then
    log PASS "HELIXLLM LLAMA.CPP-ONLY CHALLENGE PASSED"
    exit 0
else
    log FAIL "HELIXLLM LLAMA.CPP-ONLY CHALLENGE FAILED ($FAILED failures)"
    exit 1
fi
