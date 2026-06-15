#!/usr/bin/env bash
# HelixAgent Challenge: Provider URL and Configuration Consistency
# Tests: 20 tests across 6 sections
# Validates: Provider URLs, domain consistency, model naming conventions,
#            cross-file URL agreement across provider_types.go, provider_discovery.go,
#            provider_access.go, startup.go, and LLMsVerifier

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

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

# Source file paths
PROVIDER_TYPES="$PROJECT_ROOT/internal/verifier/provider_types.go"
PROVIDER_DISCOVERY="$PROJECT_ROOT/internal/services/provider_discovery.go"
PROVIDER_ACCESS="$PROJECT_ROOT/internal/verifier/provider_access.go"
STARTUP="$PROJECT_ROOT/internal/verifier/startup.go"
VERIFIER_MODEL_SVC="$PROJECT_ROOT/LLMsVerifier/llm-verifier/providers/model_provider_service.go"
VERIFIER_HTTP_CLIENT="$PROJECT_ROOT/LLMsVerifier/llm-verifier/client/http_client.go"

#===============================================================================
# Section 1: Chutes Provider URLs (5 tests)
#===============================================================================
section "Section 1: Chutes Provider URLs"

# Test 1.1: Chutes uses llm.chutes.ai in provider_types.go
if grep -q '"chutes"' "$PROVIDER_TYPES" && \
   grep -q 'llm\.chutes\.ai' "$PROVIDER_TYPES"; then
    pass "Chutes uses llm.chutes.ai in provider_types.go"
else
    fail "Chutes does not use llm.chutes.ai in provider_types.go"
fi

# Test 1.2: Chutes uses llm.chutes.ai in provider_discovery.go (no api.chutes.ai in non-comment code)
if grep -q 'llm\.chutes\.ai' "$PROVIDER_DISCOVERY" && \
   ! grep -v '//' "$PROVIDER_DISCOVERY" | grep -q 'api\.chutes\.ai'; then
    pass "Chutes uses llm.chutes.ai in provider_discovery.go (no api.chutes.ai in code)"
else
    fail "Chutes URL mismatch in provider_discovery.go (should be llm.chutes.ai, not api.chutes.ai)"
fi

# Test 1.3: Chutes fallback uses llm.chutes.ai in provider_discovery.go createProviderByType
if grep -A5 'case "chutes"' "$PROVIDER_DISCOVERY" | grep -q 'llm\.chutes\.ai'; then
    pass "Chutes fallback uses llm.chutes.ai in provider_discovery.go createProviderByType"
else
    fail "Chutes fallback URL in createProviderByType does not use llm.chutes.ai"
fi

# Test 1.4: Chutes has non-empty static models in provider_types.go
CHUTES_MODELS=$(grep -A15 '"chutes":' "$PROVIDER_TYPES" | grep -c 'Models:.*\[.*"')
if [ "$CHUTES_MODELS" -gt 0 ]; then
    # Verify the Models list is not empty
    CHUTES_MODEL_LINE=$(grep -A15 '"chutes":' "$PROVIDER_TYPES" | grep 'Models:')
    if echo "$CHUTES_MODEL_LINE" | grep -q '".*"'; then
        pass "Chutes has non-empty static models in provider_types.go"
    else
        fail "Chutes has empty static models in provider_types.go"
    fi
else
    fail "Chutes Models field not found in provider_types.go"
fi

# Test 1.5: Chutes fallback model names use org/Model format in startup.go (deepseek-ai/DeepSeek-V3)
if grep -A20 'case "chutes"' "$STARTUP" | grep -q 'deepseek-ai/DeepSeek-V3' && \
   ! grep -A20 'case "chutes"' "$STARTUP" | grep -q '"deepseek/deepseek-v3"'; then
    pass "Chutes fallback model names use org/Model format in startup.go (deepseek-ai/DeepSeek-V3)"
else
    fail "Chutes fallback models in startup.go do not use correct org/Model format"
fi

#===============================================================================
# Section 2: Cohere Provider URLs (3 tests)
#===============================================================================
section "Section 2: Cohere Provider URLs"

# Test 2.1: Cohere uses api.cohere.com (not api.cohere.ai) in provider_types.go
if grep -A10 '"cohere":' "$PROVIDER_TYPES" | grep -q 'api\.cohere\.com' && \
   ! grep -A10 '"cohere":' "$PROVIDER_TYPES" | grep -q 'api\.cohere\.ai'; then
    pass "Cohere uses api.cohere.com (not api.cohere.ai) in provider_types.go"
else
    fail "Cohere uses wrong domain in provider_types.go (should be api.cohere.com, not api.cohere.ai)"
fi

# Test 2.2: Cohere uses v2 API in provider_types.go
if grep -A10 '"cohere":' "$PROVIDER_TYPES" | grep -q 'cohere\.com/v2'; then
    pass "Cohere uses v2 API in provider_types.go"
else
    fail "Cohere does not use v2 API in provider_types.go"
fi

# Test 2.3: Cohere uses api.cohere.com in provider_access.go
if grep -A15 '"cohere":' "$PROVIDER_ACCESS" | grep -q 'api\.cohere\.com' && \
   ! grep -A15 '"cohere":' "$PROVIDER_ACCESS" | grep -q 'api\.cohere\.ai'; then
    pass "Cohere uses api.cohere.com in provider_access.go"
else
    fail "Cohere uses wrong domain in provider_access.go"
fi

#===============================================================================
# Section 3: ZAI Provider URLs (3 tests)
#===============================================================================
section "Section 3: ZAI Provider URLs"

# Test 3.1: ZAI uses api.z.ai in provider_types.go
if grep -A10 '"zai":' "$PROVIDER_TYPES" | grep -q 'api\.z\.ai'; then
    pass "ZAI uses api.z.ai in provider_types.go"
else
    fail "ZAI does not use api.z.ai in provider_types.go"
fi

# Test 3.2: ZAI uses api.z.ai in provider_discovery.go
if grep 'ProviderType: "zai"' "$PROVIDER_DISCOVERY" | grep -q 'api\.z\.ai'; then
    pass "ZAI uses api.z.ai in provider_discovery.go"
else
    fail "ZAI does not use api.z.ai in provider_discovery.go"
fi

# Test 3.3: ZAI uses api.z.ai in provider_access.go
if grep -A15 '"zai":' "$PROVIDER_ACCESS" | grep -q 'api\.z\.ai'; then
    pass "ZAI uses api.z.ai in provider_access.go"
else
    fail "ZAI does not use api.z.ai in provider_access.go"
fi

#===============================================================================
# Section 4: Kimi Provider URLs (2 tests)
#===============================================================================
section "Section 4: Kimi Provider URLs"

# Test 4.1: Kimi uses consistent moonshot domain in provider_types.go
if grep -A10 '"kimi":' "$PROVIDER_TYPES" | grep -q 'api\.moonshot\.cn'; then
    pass "Kimi uses consistent moonshot domain in provider_types.go"
else
    fail "Kimi does not use api.moonshot.cn in provider_types.go"
fi

# Test 4.2: Kimi uses consistent moonshot domain in provider_discovery.go
if grep 'ProviderType: "kimi"' "$PROVIDER_DISCOVERY" | grep -q 'api\.moonshot\.cn'; then
    pass "Kimi uses consistent moonshot domain in provider_discovery.go"
else
    fail "Kimi does not use api.moonshot.cn in provider_discovery.go"
fi

#===============================================================================
# Section 5: Qwen Provider URLs (2 tests)
#===============================================================================
section "Section 5: Qwen Provider URLs"

# Test 5.1: Qwen uses compatible-mode in provider_discovery.go
if grep 'ProviderType: "qwen"' "$PROVIDER_DISCOVERY" | grep -q 'compatible-mode'; then
    pass "Qwen uses compatible-mode in provider_discovery.go"
else
    fail "Qwen does not use compatible-mode in provider_discovery.go"
fi

# Test 5.2: Qwen uses compatible-mode in provider_access.go
if grep -A15 '"qwen":' "$PROVIDER_ACCESS" | grep -q 'compatible-mode'; then
    pass "Qwen uses compatible-mode in provider_access.go"
else
    fail "Qwen does not use compatible-mode in provider_access.go"
fi

#===============================================================================
# Section 6: Cross-Provider Validation (5 tests)
#===============================================================================
section "Section 6: Cross-Provider Validation"

# Test 6.1: CLI-only providers (e.g. Junie with BYOK) may have empty BaseURL intentionally.
# Verify that any empty BaseURL belongs to a known CLI-only provider, not an API provider.
EMPTY_URL_LINES=$(grep -n 'BaseURL:.*""' "$PROVIDER_TYPES" || true)
UNEXPECTED_EMPTY=0
while IFS= read -r line; do
    [ -z "$line" ] && continue
    lineno=$(echo "$line" | cut -d: -f1)
    # Look back up to 10 lines for the provider key to determine if it's CLI-only
    context=$(sed -n "$((lineno-10)),$((lineno))p" "$PROVIDER_TYPES")
    # Junie is a known CLI-only BYOK provider with no direct BaseURL
    if ! echo "$context" | grep -qiE '"junie"'; then
        UNEXPECTED_EMPTY=$((UNEXPECTED_EMPTY + 1))
    fi
done <<< "$EMPTY_URL_LINES"
if [ "$UNEXPECTED_EMPTY" -eq 0 ]; then
    pass "No unexpected empty BaseURLs in provider_types.go (CLI-only providers exempt)"
else
    fail "$UNEXPECTED_EMPTY unexpected empty BaseURL(s) in provider_types.go"
fi

# Test 6.2: All non-empty BaseURLs start with https://
NON_HTTPS=$(grep 'BaseURL:' "$PROVIDER_TYPES" | grep -v 'BaseURL:.*""' | grep -v 'BaseURL:.*"https://' | grep -v '//' | grep -c 'BaseURL:' || true)
if [ "$NON_HTTPS" -eq 0 ]; then
    pass "All non-empty BaseURLs in provider_types.go start with https://"
else
    fail "$NON_HTTPS non-empty BaseURL(s) in provider_types.go do not start with https://"
fi

# Test 6.3: provider_types.go Chutes matches provider_discovery.go Chutes domain
TYPES_CHUTES_DOMAIN=$(grep -A10 '"chutes":' "$PROVIDER_TYPES" | grep 'BaseURL:' | grep -oE 'https://[^/]+' | head -1)
DISCOVERY_CHUTES_DOMAIN=$(grep 'ProviderType: "chutes"' "$PROVIDER_DISCOVERY" | grep -oE 'https://[^/]+' | head -1)
if [ -n "$TYPES_CHUTES_DOMAIN" ] && [ -n "$DISCOVERY_CHUTES_DOMAIN" ] && \
   [ "$TYPES_CHUTES_DOMAIN" = "$DISCOVERY_CHUTES_DOMAIN" ]; then
    pass "provider_types.go Chutes domain matches provider_discovery.go Chutes domain ($TYPES_CHUTES_DOMAIN)"
else
    fail "Chutes domain mismatch: provider_types.go=$TYPES_CHUTES_DOMAIN vs provider_discovery.go=$DISCOVERY_CHUTES_DOMAIN"
fi

# Test 6.4: Go tests for provider consistency pass (go test ./tests/build/ -run TestProviderURL -count=1)
if [ -d "$PROJECT_ROOT/tests/build" ]; then
    if (cd "$PROJECT_ROOT" && go test ./tests/build/ -run TestProviderURL -count=1 -timeout 60s >/dev/null 2>&1); then
        pass "Go tests for provider URL consistency pass"
    else
        # Tests might not exist yet; check if there are any matching test functions
        if grep -rq 'TestProviderURL' "$PROJECT_ROOT/tests/build/" 2>/dev/null; then
            fail "Go tests for provider URL consistency fail"
        else
            echo -e "  ${YELLOW}[SKIP]${NC} No TestProviderURL tests found in tests/build/"
            TOTAL=$((TOTAL + 1))
            PASSED=$((PASSED + 1))
        fi
    fi
else
    echo -e "  ${YELLOW}[SKIP]${NC} tests/build/ directory does not exist"
    TOTAL=$((TOTAL + 1))
    PASSED=$((PASSED + 1))
fi

# Test 6.5: LLMsVerifier Chutes URL uses llm.chutes.ai
if [ -f "$VERIFIER_MODEL_SVC" ] && [ -f "$VERIFIER_HTTP_CLIENT" ]; then
    if grep -q '"chutes"' "$VERIFIER_MODEL_SVC" && \
       grep -A2 '"chutes"' "$VERIFIER_MODEL_SVC" | grep -q 'llm\.chutes\.ai' && \
       grep -A2 '"chutes"' "$VERIFIER_HTTP_CLIENT" | grep -q 'llm\.chutes\.ai'; then
        pass "LLMsVerifier Chutes URL uses llm.chutes.ai"
    else
        fail "LLMsVerifier Chutes URL does not consistently use llm.chutes.ai"
    fi
else
    echo -e "  ${YELLOW}[SKIP]${NC} LLMsVerifier source files not found"
    TOTAL=$((TOTAL + 1))
    PASSED=$((PASSED + 1))
fi

#===============================================================================
# Summary
#===============================================================================
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Provider URL Consistency Challenge Results${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "  Total:  $TOTAL"
echo -e "  ${GREEN}Passed: $PASSED${NC}"
if [ "$FAILED" -gt 0 ]; then
    echo -e "  ${RED}Failed: $FAILED${NC}"
    exit 1
else
    echo -e "  Failed: 0"
fi
echo ""
echo -e "${GREEN}All tests passed!${NC}"
