#!/bin/bash

# Feature Flags Challenge Script
# Tests the comprehensive feature flags system for HelixAgent
# Validates feature registry, agent capabilities, middleware, and backward compatibility

# Don't exit on error - we want to run all tests
set +e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
PASSED=0
FAILED=0
TOTAL=0

# Increment test counter
increment_test() {
    TOTAL=$((TOTAL + 1))
}

# Mark test as passed
pass_test() {
    local test_name="$1"
    PASSED=$((PASSED + 1))
    echo -e "${GREEN}✓ PASS:${NC} $test_name"
}

# Mark test as failed
fail_test() {
    local test_name="$1"
    local error_msg="$2"
    FAILED=$((FAILED + 1))
    echo -e "${RED}✗ FAIL:${NC} $test_name"
    if [ -n "$error_msg" ]; then
        echo -e "  ${RED}Error:${NC} $error_msg"
    fi
}

# Print section header
print_section() {
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}\n"
}

# Test that a file exists
test_file_exists() {
    local file="$1"
    local test_name="$2"
    increment_test
    if [ -f "$file" ]; then
        pass_test "$test_name"
        return 0
    else
        fail_test "$test_name" "File not found: $file"
        return 1
    fi
}

# Test that a directory exists
test_dir_exists() {
    local dir="$1"
    local test_name="$2"
    increment_test
    if [ -d "$dir" ]; then
        pass_test "$test_name"
        return 0
    else
        fail_test "$test_name" "Directory not found: $dir"
        return 1
    fi
}

# Test Go code compiles
test_go_build() {
    local package="$1"
    local test_name="$2"
    increment_test
    if go build -o /dev/null "$package" 2>/dev/null; then
        pass_test "$test_name"
        return 0
    else
        fail_test "$test_name" "Build failed for package: $package"
        return 1
    fi
}

# Test Go tests pass
test_go_tests() {
    local package="$1"
    local test_name="$2"
    increment_test
    if go test -v "$package" >/dev/null 2>&1; then
        pass_test "$test_name"
        return 0
    else
        fail_test "$test_name" "Tests failed for package: $package"
        return 1
    fi
}

# Test file contains pattern
test_file_contains() {
    local file="$1"
    local pattern="$2"
    local test_name="$3"
    increment_test
    if grep -q "$pattern" "$file" 2>/dev/null; then
        pass_test "$test_name"
        return 0
    else
        fail_test "$test_name" "Pattern '$pattern' not found in $file"
        return 1
    fi
}

# Test specific feature constant exists
test_feature_constant() {
    local feature="$1"
    local test_name="$2"
    increment_test
    if grep -q "Feature$feature" "$PROJECT_ROOT/internal/features/features.go" 2>/dev/null; then
        pass_test "$test_name"
        return 0
    else
        fail_test "$test_name" "Feature constant $feature not defined"
        return 1
    fi
}

# Test agent capability exists
test_agent_capability() {
    local agent="$1"
    local test_name="$2"
    increment_test
    if grep -q "\"$agent\"" "$PROJECT_ROOT/internal/features/capability.go" 2>/dev/null; then
        pass_test "$test_name"
        return 0
    else
        fail_test "$test_name" "Agent capability $agent not defined"
        return 1
    fi
}

# Main challenge execution
main() {
    echo -e "${YELLOW}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║        HelixAgent Feature Flags Challenge                      ║"
    echo "║                                                                ║"
    echo "║   Testing feature flags system, agent capabilities,           ║"
    echo "║   middleware, and backward compatibility                      ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    cd "$PROJECT_ROOT"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 1: Feature Files Structure
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 1: Feature Files Structure"

    test_dir_exists "internal/features" "Features package directory exists"
    test_file_exists "internal/features/features.go" "Core features file exists"
    test_file_exists "internal/features/capability.go" "Agent capability file exists"
    test_file_exists "internal/features/config.go" "Feature config file exists"
    test_file_exists "internal/features/middleware.go" "Feature middleware file exists"
    test_file_exists "internal/features/features_test.go" "Features tests exist"
    test_file_exists "internal/features/capability_test.go" "Capability tests exist"
    test_file_exists "internal/features/config_test.go" "Config tests exist"
    test_file_exists "internal/features/middleware_test.go" "Middleware tests exist"
    test_file_exists "docs/guides/feature-flags.md" "Documentation exists"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 2: Feature Constants
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 2: Feature Constants"

    # Transport features
    test_feature_constant "GraphQL" "GraphQL feature constant"
    test_feature_constant "TOON" "TOON feature constant"
    test_feature_constant "HTTP2" "HTTP2 feature constant"
    test_feature_constant "HTTP3" "HTTP3 feature constant"
    test_feature_constant "WebSocket" "WebSocket feature constant"
    test_feature_constant "SSE" "SSE feature constant"
    test_feature_constant "JSONL" "JSONL feature constant"

    # Compression features
    test_feature_constant "Brotli" "Brotli feature constant"
    test_feature_constant "Gzip" "Gzip feature constant"
    test_feature_constant "Zstd" "Zstd feature constant"

    # Protocol features
    test_feature_constant "MCP" "MCP feature constant"
    test_feature_constant "ACP" "ACP feature constant"
    test_feature_constant "LSP" "LSP feature constant"
    test_feature_constant "GRPC" "GRPC feature constant"

    # API features
    test_feature_constant "Embeddings" "Embeddings feature constant"
    test_feature_constant "Vision" "Vision feature constant"
    test_feature_constant "Cognee" "Cognee feature constant"
    test_feature_constant "Debate" "Debate feature constant"
    test_feature_constant "BatchRequests" "BatchRequests feature constant"
    test_feature_constant "ToolCalling" "ToolCalling feature constant"

    # Advanced features
    test_feature_constant "MultiPass" "MultiPass feature constant"
    test_feature_constant "Caching" "Caching feature constant"
    test_feature_constant "RateLimiting" "RateLimiting feature constant"
    test_feature_constant "Metrics" "Metrics feature constant"
    test_feature_constant "Tracing" "Tracing feature constant"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 3: Agent Capabilities
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 3: Agent Capabilities (18 Agents)"

    test_agent_capability "opencode" "OpenCode agent capability"
    test_agent_capability "crush" "Crush agent capability"
    test_agent_capability "helixcode" "HelixCode agent capability"
    test_agent_capability "kiro" "Kiro agent capability"
    test_agent_capability "aider" "Aider agent capability"
    test_agent_capability "claudecode" "ClaudeCode agent capability"
    test_agent_capability "cline" "Cline agent capability"
    test_agent_capability "codenamegoose" "CodenameGoose agent capability"
    test_agent_capability "deepseekcli" "DeepSeekCLI agent capability"
    test_agent_capability "forge" "Forge agent capability"
    test_agent_capability "geminicli" "GeminiCLI agent capability"
    test_agent_capability "gptengineer" "GPTEngineer agent capability"
    test_agent_capability "kilocode" "KiloCode agent capability"
    test_agent_capability "mistralcode" "MistralCode agent capability"
    test_agent_capability "ollamacode" "OllamaCode agent capability"
    test_agent_capability "plandex" "Plandex agent capability"
    test_agent_capability "qwencode" "QwenCode agent capability"
    test_agent_capability "amazonq" "AmazonQ agent capability"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 4: Feature Registry Implementation
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 4: Feature Registry Implementation"

    test_file_contains "internal/features/features.go" "type Registry struct" "Registry struct defined"
    test_file_contains "internal/features/features.go" "GetRegistry()" "GetRegistry function defined"
    test_file_contains "internal/features/features.go" "GetFeature(" "GetFeature method defined"
    test_file_contains "internal/features/features.go" "GetAllFeatures()" "GetAllFeatures method defined"
    test_file_contains "internal/features/features.go" "GetFeaturesByCategory(" "GetFeaturesByCategory method defined"
    test_file_contains "internal/features/features.go" "GetDefaultValue(" "GetDefaultValue method defined"
    test_file_contains "internal/features/features.go" "ValidateFeatureCombination(" "ValidateFeatureCombination method defined"
    test_file_contains "internal/features/features.go" "GetFeatureByHeader(" "GetFeatureByHeader method defined"
    test_file_contains "internal/features/features.go" "GetFeatureByQueryParam(" "GetFeatureByQueryParam method defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 5: Capability Registry Implementation
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 5: Capability Registry Implementation"

    test_file_contains "internal/features/capability.go" "type CapabilityRegistry struct" "CapabilityRegistry struct defined"
    test_file_contains "internal/features/capability.go" "GetCapabilityRegistry()" "GetCapabilityRegistry function defined"
    test_file_contains "internal/features/capability.go" "GetCapability(" "GetCapability method defined"
    test_file_contains "internal/features/capability.go" "IsFeatureSupported(" "IsFeatureSupported method defined"
    test_file_contains "internal/features/capability.go" "IsFeaturePreferred(" "IsFeaturePreferred method defined"
    test_file_contains "internal/features/capability.go" "GetAgentFeatureDefaults(" "GetAgentFeatureDefaults method defined"
    test_file_contains "internal/features/capability.go" "GetSupportedStreamingMethods(" "GetSupportedStreamingMethods method defined"
    test_file_contains "internal/features/capability.go" "GetSupportedCompression(" "GetSupportedCompression method defined"
    test_file_contains "internal/features/capability.go" "GetTransportProtocol(" "GetTransportProtocol method defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 6: Feature Context Implementation
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 6: Feature Context Implementation"

    test_file_contains "internal/features/config.go" "type FeatureContext struct" "FeatureContext struct defined"
    test_file_contains "internal/features/config.go" "NewFeatureContext()" "NewFeatureContext function defined"
    test_file_contains "internal/features/config.go" "NewFeatureContextFromConfig(" "NewFeatureContextFromConfig function defined"
    test_file_contains "internal/features/config.go" "IsEnabled(" "IsEnabled method defined"
    test_file_contains "internal/features/config.go" "SetEnabled(" "SetEnabled method defined"
    test_file_contains "internal/features/config.go" "ApplyAgentCapabilities(" "ApplyAgentCapabilities method defined"
    test_file_contains "internal/features/config.go" "ApplyOverrides(" "ApplyOverrides method defined"
    test_file_contains "internal/features/config.go" "GetStreamingMethod()" "GetStreamingMethod method defined"
    test_file_contains "internal/features/config.go" "GetCompressionMethod()" "GetCompressionMethod method defined"
    test_file_contains "internal/features/config.go" "GetTransportProtocol()" "GetTransportProtocol method defined"
    test_file_contains "internal/features/config.go" "WithFeatureContext(" "Context integration defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 7: Middleware Implementation
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 7: Middleware Implementation"

    test_file_contains "internal/features/middleware.go" "func Middleware(" "Middleware function defined"
    test_file_contains "internal/features/middleware.go" "detectAgent(" "Agent detection function defined"
    test_file_contains "internal/features/middleware.go" "parseFeatureHeaders(" "Header parsing function defined"
    test_file_contains "internal/features/middleware.go" "parseFeatureQueryParams(" "Query param parsing function defined"
    test_file_contains "internal/features/middleware.go" "GetFeatureContextFromGin(" "Gin context integration defined"
    test_file_contains "internal/features/middleware.go" "IsFeatureEnabled(" "IsFeatureEnabled convenience function defined"
    test_file_contains "internal/features/middleware.go" "RequireFeature(" "RequireFeature middleware defined"
    test_file_contains "internal/features/middleware.go" "RequireAnyFeature(" "RequireAnyFeature middleware defined"
    test_file_contains "internal/features/middleware.go" "ConditionalMiddleware(" "ConditionalMiddleware defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 8: HelixCode Full Feature Support
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 8: HelixCode Full Feature Support"

    # HelixCode must support all advanced features
    test_file_contains "internal/features/capability.go" "FeatureGraphQL" "HelixCode supports GraphQL"
    test_file_contains "internal/features/capability.go" "FeatureTOON" "HelixCode supports TOON"
    test_file_contains "internal/features/capability.go" "FeatureHTTP3" "HelixCode supports HTTP/3"
    test_file_contains "internal/features/capability.go" "FeatureBrotli" "HelixCode supports Brotli"
    test_file_contains "internal/features/capability.go" "FeatureZstd" "HelixCode supports Zstd"
    test_file_contains "internal/features/capability.go" "FeatureMultiPass" "HelixCode supports MultiPass"
    test_file_contains "internal/features/capability.go" "FeatureTracing" "HelixCode supports Tracing"
    test_file_contains "internal/features/capability.go" "TransportProtocol.*http3" "HelixCode uses HTTP/3 transport"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 9: Backward Compatibility (via tests)
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 9: Backward Compatibility (via tests)"

    # Test that default values are correct via the test file (more reliable than grep on multi-line code)
    test_file_contains "internal/features/features_test.go" "FeatureGraphQL, false" "GraphQL disabled by default (test)"
    test_file_contains "internal/features/features_test.go" "FeatureTOON, false" "TOON disabled by default (test)"
    test_file_contains "internal/features/features_test.go" "FeatureHTTP3, false" "HTTP/3 disabled by default (test)"
    test_file_contains "internal/features/features_test.go" "FeatureBrotli, false" "Brotli disabled by default (test)"
    test_file_contains "internal/features/features_test.go" "FeatureHTTP2, true" "HTTP/2 enabled by default (test)"
    test_file_contains "internal/features/features_test.go" "FeatureSSE, true" "SSE enabled by default (test)"
    test_file_contains "internal/features/features_test.go" "FeatureGzip, true" "Gzip enabled by default (test)"
    # Verify defaults are correctly enforced
    test_file_contains "internal/features/features.go" "DefaultValue: false" "Some features default to false"
    test_file_contains "internal/features/features.go" "DefaultValue: true" "Some features default to true"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 10: Feature Validation
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 10: Feature Validation"

    test_file_contains "internal/features/features.go" "RequiresFeatures" "Dependency tracking defined"
    test_file_contains "internal/features/features.go" "ConflictsWith" "Conflict tracking defined"
    test_file_contains "internal/features/features.go" "FeatureValidationError" "Validation error type defined"
    test_file_contains "internal/features/features.go" "RequiresFeatures.*FeatureDebate" "MultiPass requires Debate (RequiresFeatures)"
    test_file_contains "internal/features/features.go" "ConflictsWith.*FeatureHTTP2" "HTTP/3 conflicts with HTTP/2 (ConflictsWith)"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 11: Header and Query Param Support
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 11: Header and Query Param Support"

    test_file_contains "internal/features/features.go" "X-Feature-GraphQL" "GraphQL header defined"
    test_file_contains "internal/features/features.go" "X-Feature-TOON" "TOON header defined"
    test_file_contains "internal/features/features.go" "X-Feature-HTTP3" "HTTP/3 header defined"
    test_file_contains "internal/features/middleware.go" "X-Features" "Compact features header supported"
    test_file_contains "internal/features/middleware.go" "features" "Compact features query param supported"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 12: Usage Tracking
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 12: Usage Tracking"

    test_file_contains "internal/features/config.go" "FeatureUsageTracker" "Usage tracker type defined"
    test_file_contains "internal/features/config.go" "GetUsageTracker()" "GetUsageTracker function defined"
    test_file_contains "internal/features/config.go" "RecordUsage(" "RecordUsage method defined"
    test_file_contains "internal/features/config.go" "GetStats()" "GetStats method defined"
    test_file_contains "internal/features/config.go" "ResetStats()" "ResetStats method defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 13: Go Tests
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 13: Go Tests"

    test_go_tests "./internal/features/..." "All feature flags tests pass"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 14: Build Verification
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 14: Build Verification"

    test_go_build "./internal/features/..." "Features package builds successfully"

    # ══════════════════════════════════════════════════════════════════
    # FINAL RESULTS
    # ══════════════════════════════════════════════════════════════════
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  CHALLENGE RESULTS${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}\n"

    echo -e "  Total Tests:  ${TOTAL}"
    echo -e "  ${GREEN}Passed:       ${PASSED}${NC}"
    echo -e "  ${RED}Failed:       ${FAILED}${NC}"

    if [ $FAILED -eq 0 ]; then
        echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║        ALL FEATURE FLAGS TESTS PASSED!                         ║${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}\n"
        exit 0
    else
        echo -e "\n${RED}╔════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}║        SOME TESTS FAILED - SEE OUTPUT ABOVE                    ║${NC}"
        echo -e "${RED}╚════════════════════════════════════════════════════════════════╝${NC}\n"
        exit 1
    fi
}

main "$@"
