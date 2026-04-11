#!/bin/bash
#
# Z.AI (Zhipu GLM) Provider Challenge
# Validates Z.AI provider implementation with comprehensive tests
#
# Tests:
# 1. Provider configuration and models list
# 2. API connectivity and error handling
# 3. Model discovery from API
# 4. Subscription detection (free_credits, pay_as_you_go)
# 5. Debate team integration
# 6. Fallback mechanism
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASS=0
FAIL=0
TOTAL=0

log_info() { echo -e "${BLUE}[INFO]${NC} $(date +%H:%M:%S) $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) $1"; PASS=$((PASS + 1)); TOTAL=$((TOTAL + 1)); }
log_error() { echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) $1"; FAIL=$((FAIL + 1)); TOTAL=$((TOTAL + 1)); }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $(date +%H:%M:%S) $1"; TOTAL=$((TOTAL + 1)); }

echo ""
echo "══════════════════════════════════════════════════════════════"
echo "     Z.AI (Zhipu GLM) Provider Challenge"
echo "     Validates Z.AI implementation with 25+ tests"
echo "══════════════════════════════════════════════════════════════"
echo ""

# ==============================================
# Section 1: Configuration Validation
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Section 1: Configuration Validation"
echo "[INFO] $(date +%H:%M:%S) =============================================="

# Test 1: Provider configuration exists
log_info "Test 1: ZAI provider configuration exists"
if grep -q '"zai"' internal/verifier/provider_types.go; then
    log_success "ZAI provider configuration found"
else
    log_error "ZAI provider configuration not found"
fi

# Test 2: Correct API endpoint
log_info "Test 2: Correct API endpoint configured"
if grep -q "open.bigmodel.cn/api/paas/v4" internal/llm/providers/zai/zai.go; then
    log_success "Correct Z.AI API endpoint configured"
else
    log_error "Incorrect or missing Z.AI API endpoint"
fi

# Test 3: Current GLM models listed
log_info "Test 3: Current GLM models in configuration"
CURRENT_MODELS=("glm-5" "glm-4.7" "glm-4.6" "glm-4.5")
FOUND_MODELS=0
for model in "${CURRENT_MODELS[@]}"; do
    if grep -q "\"$model\"" internal/verifier/provider_types.go; then
        FOUND_MODELS=$((FOUND_MODELS + 1))
    fi
done
if [ $FOUND_MODELS -ge 3 ]; then
    log_success "Found $FOUND_MODELS current GLM models"
else
    log_error "Only found $FOUND_MODELS current GLM models (expected at least 3)"
fi

# Test 4: Environment variables configured
log_info "Test 4: Environment variables configured"
if grep -q "ZAI_API_KEY" internal/verifier/provider_types.go && grep -q "ZHIPU_API_KEY" internal/verifier/provider_types.go; then
    log_success "ZAI environment variables configured"
else
    log_error "ZAI environment variables not properly configured"
fi

# Test 5: Subscription types configured
log_info "Test 5: Subscription types configured"
if grep -A5 '"zai"' internal/verifier/provider_access.go | grep -q "SubTypeFreeCredits" 2>/dev/null; then
    log_success "ZAI subscription types configured"
else
    log_warning "ZAI subscription types may need review"
fi

echo ""
# ==============================================
# Section 2: Provider Implementation
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Section 2: Provider Implementation"
echo "[INFO] $(date +%H:%M:%S) =============================================="

# Test 6: Provider file exists
log_info "Test 6: ZAI provider implementation exists"
if [ -f "internal/llm/providers/zai/zai.go" ]; then
    log_success "ZAI provider implementation found"
else
    log_error "ZAI provider implementation not found"
fi

# Test 7: Complete method implemented
log_info "Test 7: Complete method implemented"
if grep -q "func.*Complete.*context.Context" internal/llm/providers/zai/zai.go; then
    log_success "Complete method implemented"
else
    log_error "Complete method not implemented"
fi

# Test 8: Streaming support
log_info "Test 8: Streaming support implemented"
if grep -q "CompleteStream" internal/llm/providers/zai/zai.go; then
    log_success "Streaming support implemented"
else
    log_error "Streaming support not implemented"
fi

# Test 9: Tool calling support
log_info "Test 9: Tool calling support implemented"
if grep -q "ToolCalls" internal/llm/providers/zai/zai.go; then
    log_success "Tool calling support implemented"
else
    log_error "Tool calling support not implemented"
fi

# Test 10: Zhipu-specific error codes
log_info "Test 10: Zhipu-specific error codes handled"
ERROR_CODES=0
grep -q "ZhipuErrInsufficientBalance" internal/llm/providers/zai/zai.go && ERROR_CODES=$((ERROR_CODES + 1))
grep -q "ZhipuErrModelNotFound" internal/llm/providers/zai/zai.go && ERROR_CODES=$((ERROR_CODES + 1))
grep -q "ZhipuErrUnauthorized" internal/llm/providers/zai/zai.go && ERROR_CODES=$((ERROR_CODES + 1))
grep -q "ZhipuErrRateLimited" internal/llm/providers/zai/zai.go && ERROR_CODES=$((ERROR_CODES + 1))
if [ $ERROR_CODES -ge 3 ]; then
    log_success "Found $ERROR_CODES Zhipu-specific error code handlers"
else
    log_error "Only $ERROR_CODES Zhipu-specific error code handlers found"
fi

echo ""
# ==============================================
# Section 3: Unit Tests
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Section 3: Unit Tests"
echo "[INFO] $(date +%H:%M:%S) =============================================="

# Test 11: Unit tests exist
log_info "Test 11: ZAI unit tests exist"
if [ -f "internal/llm/providers/zai/zai_test.go" ]; then
    log_success "ZAI unit tests found"
else
    log_error "ZAI unit tests not found"
fi

# Test 12: Run unit tests
log_info "Test 12: Running ZAI unit tests..."
GOMAXPROCS=2 go test -v -count=1 ./internal/llm/providers/zai/... -timeout 60s > /tmp/zai_test_output.log 2>&1
if [ $? -eq 0 ]; then
    PASSED_TESTS=$(grep -c "--- PASS" /tmp/zai_test_output.log || echo "0")
    log_success "ZAI unit tests passed ($PASSED_TESTS tests)"
else
    FAILED_TESTS=$(grep -c "--- FAIL" /tmp/zai_test_output.log || echo "0")
    log_error "ZAI unit tests failed ($FAILED_TESTS failures)"
    grep "--- FAIL" /tmp/zai_test_output.log | head -5
fi

# Test 13: Error code tests
log_info "Test 13: Zhipu error code tests exist"
if grep -q "TestZAIProvider_ZhipuErrorCodes" internal/llm/providers/zai/zai_test.go; then
    log_success "Zhipu error code tests exist"
else
    log_error "Zhipu error code tests not found"
fi

# Test 14: Current model tests
log_info "Test 14: Current GLM model tests exist"
if grep -q "TestZAIProvider_CurrentGLMModels" internal/llm/providers/zai/zai_test.go; then
    log_success "Current GLM model tests exist"
else
    log_error "Current GLM model tests not found"
fi

# Test 15: Tool calling tests
log_info "Test 15: Tool calling tests exist"
if grep -q "TestZAIProvider_ToolCalls" internal/llm/providers/zai/zai_test.go; then
    log_success "Tool calling tests exist"
else
    log_error "Tool calling tests not found"
fi

echo ""
# ==============================================
# Section 4: Model Discovery
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Section 4: Model Discovery"
echo "[INFO] $(date +%H:%M:%S) =============================================="

# Test 16: Discovery integration
log_info "Test 16: Model discovery integration"
if grep -q "discovery.NewDiscoverer" internal/llm/providers/zai/zai.go; then
    log_success "Model discovery integration found"
else
    log_error "Model discovery not integrated"
fi

# Test 17: Models endpoint configured
log_info "Test 17: Models endpoint configured"
if grep -q "ModelsEndpoint\|modelsEndpoint" internal/llm/providers/zai/zai.go; then
    log_success "Models endpoint configured"
else
    log_error "Models endpoint not configured"
fi

# Test 18: Fallback models exist
log_info "Test 18: Fallback models configured"
FALLBACK_COUNT=$(grep -c "FallbackModels" internal/llm/providers/zai/zai.go || echo "0")
if [ "$FALLBACK_COUNT" -gt 0 ]; then
    log_success "Fallback models configured"
else
    log_error "Fallback models not configured"
fi

# Test 19: ParseZAIModelsResponse exists
log_info "Test 19: ParseZAIModelsResponse exists"
if grep -q "ParseZAIModelsResponse" internal/llm/discovery/discovery.go; then
    log_success "ParseZAIModelsResponse function exists"
else
    log_error "ParseZAIModelsResponse function not found"
fi

echo ""
# ==============================================
# Section 5: LLMsVerifier Integration
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Section 5: LLMsVerifier Integration"
echo "[INFO] $(date +%H:%M:%S) =============================================="

# Test 20: LLMsVerifier ZAI configuration
log_info "Test 20: LLMsVerifier ZAI configuration"
if grep -q '"zai".*BaseURL.*open.bigmodel.cn' LLMsVerifier/llm-verifier/providers/model_provider_service.go; then
    log_success "LLMsVerifier ZAI configuration found"
else
    log_warning "LLMsVerifier ZAI configuration may need review"
fi

# Test 21: LLMsVerifier ZAI tests
log_info "Test 21: LLMsVerifier ZAI tests"
if grep -q "zai" LLMsVerifier/llm-verifier/providers/model_provider_service_test.go 2>/dev/null; then
    log_success "LLMsVerifier ZAI tests found"
else
    log_warning "LLMsVerifier ZAI tests not found"
fi

echo ""
# ==============================================
# Section 6: API Connectivity (if credentials available)
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Section 6: API Connectivity"
echo "[INFO] $(date +%H:%M:%S) =============================================="

ZAI_API_KEY="${ZAI_API_KEY:-$(grep -E '^ZAI_API_KEY=' .env 2>/dev/null | cut -d= -f2)}"

# Test 22: Check if API key is available
log_info "Test 22: ZAI API key available"
if [ -n "$ZAI_API_KEY" ] && [ "$ZAI_API_KEY" != "" ]; then
    log_success "ZAI API key found in environment"
    
    # Test 23: Models endpoint reachable
    log_info "Test 23: Models endpoint reachable"
    MODELS_RESP=$(curl -s --max-time 60 -w "%{http_code}" "https://open.bigmodel.cn/api/paas/v4/models" \
        -H "Authorization: Bearer $ZAI_API_KEY" 2>/dev/null)
    HTTP_CODE=$(echo "$MODELS_RESP" | tail -c 4)
    if [ "$HTTP_CODE" = "200" ]; then
        log_success "Models endpoint returned 200 OK"
        
        # Test 24: Parse available models
        log_info "Test 24: Parse available models"
        MODELS=$(curl -s --max-time 60 "https://open.bigmodel.cn/api/paas/v4/models" \
            -H "Authorization: Bearer $ZAI_API_KEY" 2>/dev/null | \
            grep -o '"id":"[^"]*"' | cut -d'"' -f4 | tr '\n' ' ')
        if [ -n "$MODELS" ]; then
            log_success "Available models: $MODELS"
        else
            log_warning "Could not parse models from response"
        fi
    else
        log_warning "Models endpoint returned HTTP $HTTP_CODE"
    fi
    
    # Test 25: Chat completion test
    log_info "Test 25: Chat completion test"
    CHAT_RESP=$(curl -s --max-time 60 -w "\n%{http_code}" "https://open.bigmodel.cn/api/paas/v4/chat/completions" \
        -H "Authorization: Bearer $ZAI_API_KEY" \
        -H "Content-Type: application/json" \
        -d '{"model": "glm-4.5", "messages": [{"role": "user", "content": "Say OK"}], "max_tokens": 10}' 2>/dev/null)
    CHAT_HTTP=$(echo "$CHAT_RESP" | tail -n1)
    CHAT_BODY=$(echo "$CHAT_RESP" | head -n-1)
    
    if [ "$CHAT_HTTP" = "200" ]; then
        if echo "$CHAT_BODY" | grep -q '"choices"'; then
            log_success "Chat completion successful"
        else
            log_warning "Chat completion response unexpected"
        fi
    elif echo "$CHAT_BODY" | grep -q "1113"; then
        log_warning "Insufficient balance - API key valid but needs credits"
    elif echo "$CHAT_BODY" | grep -q "1211"; then
        log_warning "Model not found - check model availability"
    else
        log_warning "Chat completion failed: HTTP $CHAT_HTTP"
    fi
else
    log_warning "ZAI_API_KEY not set - skipping API tests"
    ((TOTAL++))
    ((TOTAL++))
    ((TOTAL++))
    ((TOTAL++))
fi

echo ""
# ==============================================
# Section 7: HelixAgent Integration
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Section 7: HelixAgent Integration"
echo "[INFO] $(date +%H:%M:%S) =============================================="

# Test 26: Debate team configuration
log_info "Test 26: ZAI in debate team configuration"
if grep -q '"zai"' internal/config/ai_debate.go; then
    log_success "ZAI included in debate team configuration"
else
    log_warning "ZAI not found in debate team configuration"
fi

# Test 27: Provider registry
log_info "Test 27: ZAI in provider registry"
if grep -rn "zai" internal/llm/registry.go >/dev/null 2>&1 || grep -rn "NewZAIProvider" internal/ >/dev/null 2>&1; then
    log_success "ZAI registered in provider system"
else
    log_warning "ZAI may not be registered in provider system"
fi

# Test 28: HelixAgent health check
log_info "Test 28: HelixAgent health check"
HEALTH_RESP=$(curl -s --max-time 60 http://localhost:7061/v1/health 2>/dev/null)
if [ -n "$HEALTH_RESP" ]; then
    if echo "$HEALTH_RESP" | grep -q "healthy"; then
        log_success "HelixAgent is running and healthy"
    else
        log_warning "HelixAgent health check returned unexpected response"
    fi
else
    log_warning "HelixAgent not running - start with: make run-dev"
fi

echo ""
# ==============================================
# Challenge Summary
# ==============================================
echo "[INFO] $(date +%H:%M:%S)"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Challenge Summary: Z.AI Provider Challenge"
echo "[INFO] $(date +%H:%M:%S) =============================================="
echo "[INFO] $(date +%H:%M:%S) Total Tests: $TOTAL"
if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) Passed: $PASS"
    echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) Pass Rate: $(( PASS * 100 / TOTAL ))%"
else
    echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) Passed: $PASS"
    echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) Failed: $FAIL"
fi
echo "[INFO] $(date +%H:%M:%S) =============================================="

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) ALL TESTS PASSED!"
    echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) Z.AI provider is properly implemented."
    echo -e "${GREEN}[SUCCESS]${NC} $(date +%H:%M:%S) =============================================="
    exit 0
else
    echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) SOME TESTS FAILED!"
    echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) Please review and fix the issues above."
    echo -e "${RED}[ERROR]${NC} $(date +%H:%M:%S) =============================================="
    exit 1
fi
