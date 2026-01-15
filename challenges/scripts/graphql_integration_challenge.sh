#!/bin/bash

# GraphQL Integration Challenge Script
# Tests GraphQL schema, resolvers, handler, and TOON integration

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

# Main challenge execution
main() {
    echo -e "${YELLOW}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║       HelixAgent GraphQL Integration Challenge                 ║"
    echo "║                                                                ║"
    echo "║   Testing GraphQL schema, resolvers, handler, TOON            ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    cd "$PROJECT_ROOT"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 1: GraphQL Schema Tests
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 1: GraphQL Schema Structure"

    test_file_exists "internal/graphql/schema.go" "GraphQL schema file exists"
    test_file_exists "internal/graphql/schema_test.go" "GraphQL schema tests exist"
    test_dir_exists "internal/graphql/types" "GraphQL types directory exists"
    test_file_exists "internal/graphql/types/types.go" "GraphQL types definition exists"

    # Check schema defines required types
    test_file_contains "internal/graphql/schema.go" "QueryType" "Schema defines QueryType"
    test_file_contains "internal/graphql/schema.go" "MutationType" "Schema defines MutationType"
    test_file_contains "internal/graphql/schema.go" "providerType" "Schema defines Provider type"
    test_file_contains "internal/graphql/schema.go" "debateType" "Schema defines Debate type"
    test_file_contains "internal/graphql/schema.go" "taskType" "Schema defines Task type"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 2: GraphQL Resolvers Tests
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 2: GraphQL Resolvers"

    test_dir_exists "internal/graphql/resolvers" "Resolvers directory exists"
    test_file_exists "internal/graphql/resolvers/resolvers.go" "Resolvers implementation exists"
    test_file_exists "internal/graphql/resolvers/resolvers_test.go" "Resolvers tests exist"

    # Check resolver functions
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ResolveProviders" "ResolveProviders function defined"
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ResolveDebates" "ResolveDebates function defined"
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ResolveTasks" "ResolveTasks function defined"
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ResolveVerificationResults" "ResolveVerificationResults function defined"
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ResolveCreateDebate" "ResolveCreateDebate mutation defined"
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ResolveCreateTask" "ResolveCreateTask mutation defined"

    # Check resolver context
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ResolverContext" "ResolverContext struct defined"
    test_file_contains "internal/graphql/resolvers/resolvers.go" "ServiceRegistry" "ServiceRegistry interface defined"
    test_file_contains "internal/graphql/resolvers/resolvers.go" "SetGlobalContext" "SetGlobalContext function defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 3: GraphQL Handler Tests
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 3: GraphQL HTTP Handler"

    test_file_exists "internal/handlers/graphql_handler.go" "GraphQL handler exists"
    test_file_exists "internal/handlers/graphql_handler_test.go" "GraphQL handler tests exist"

    # Check handler features
    test_file_contains "internal/handlers/graphql_handler.go" "GraphQLHandler" "GraphQLHandler struct defined"
    test_file_contains "internal/handlers/graphql_handler.go" "Handle(" "Handle method defined"
    test_file_contains "internal/handlers/graphql_handler.go" "HandleIntrospection" "Introspection handler defined"
    test_file_contains "internal/handlers/graphql_handler.go" "HandlePlayground" "Playground handler defined"
    test_file_contains "internal/handlers/graphql_handler.go" "HandleBatch" "Batch handler defined"
    test_file_contains "internal/handlers/graphql_handler.go" "RegisterRoutes" "RegisterRoutes method defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 4: TOON Integration Tests
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 4: TOON Transport Layer"

    test_dir_exists "internal/toon" "TOON directory exists"
    test_file_exists "internal/toon/encoder.go" "TOON encoder exists"
    test_file_exists "internal/toon/transport.go" "TOON transport exists"
    test_file_exists "internal/toon/encoder_test.go" "TOON encoder tests exist"
    test_file_exists "internal/toon/transport_test.go" "TOON transport tests exist"

    # Check TOON features
    test_file_contains "internal/toon/encoder.go" "CompressionLevel" "CompressionLevel type defined"
    test_file_contains "internal/toon/encoder.go" "Encoder" "Encoder struct defined"
    test_file_contains "internal/toon/encoder.go" "Decoder" "Decoder struct defined"
    test_file_contains "internal/toon/encoder.go" "DefaultKeyMapping" "DefaultKeyMapping function defined"
    test_file_contains "internal/toon/transport.go" "Transport" "Transport struct defined"
    test_file_contains "internal/toon/transport.go" "Middleware" "Middleware struct defined"

    # Check GraphQL-TOON integration
    test_file_contains "internal/handlers/graphql_handler.go" "toonEncoder" "GraphQL handler has TOON encoder"
    test_file_contains "internal/handlers/graphql_handler.go" "application/toon+json" "TOON content type supported"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 5: GraphQL Types Tests
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 5: GraphQL Types"

    # Check type definitions
    test_file_contains "internal/graphql/types/types.go" "type Provider struct" "Provider type defined"
    test_file_contains "internal/graphql/types/types.go" "type Model struct" "Model type defined"
    test_file_contains "internal/graphql/types/types.go" "type Debate struct" "Debate type defined"
    test_file_contains "internal/graphql/types/types.go" "type DebateRound struct" "DebateRound type defined"
    test_file_contains "internal/graphql/types/types.go" "type Task struct" "Task type defined"
    test_file_contains "internal/graphql/types/types.go" "type VerificationResults struct" "VerificationResults type defined"
    test_file_contains "internal/graphql/types/types.go" "type ProviderScore struct" "ProviderScore type defined"

    # Check input types
    test_file_contains "internal/graphql/types/types.go" "type ProviderFilter struct" "ProviderFilter input defined"
    test_file_contains "internal/graphql/types/types.go" "type CreateDebateInput struct" "CreateDebateInput defined"
    test_file_contains "internal/graphql/types/types.go" "type CreateTaskInput struct" "CreateTaskInput defined"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 6: Go Test Execution
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 6: Test Execution"

    test_go_tests "./internal/graphql/..." "GraphQL package tests pass"
    test_go_tests "./internal/graphql/resolvers/..." "GraphQL resolvers tests pass"
    test_go_tests "./internal/toon/..." "TOON package tests pass"
    test_go_tests "./internal/handlers/..." "Handlers tests pass"

    # ══════════════════════════════════════════════════════════════════
    # SECTION 7: Build Verification
    # ══════════════════════════════════════════════════════════════════
    print_section "Section 7: Build Verification"

    test_go_build "./internal/graphql/..." "GraphQL packages build successfully"
    test_go_build "./internal/toon/..." "TOON packages build successfully"
    test_go_build "./internal/handlers/..." "Handlers package builds successfully"

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
        echo -e "${GREEN}║        ALL GRAPHQL INTEGRATION TESTS PASSED!                   ║${NC}"
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
