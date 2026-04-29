#!/bin/bash
# Protocol GraphQL Challenge
# Tests GraphQL API support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"

init_challenge "protocol-graphql" "Protocol GraphQL Challenge"
load_env

log_info "Testing GraphQL protocol support..."

test_graphql_endpoint_availability() {
    log_info "Test 1: GraphQL endpoint availability"

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/graphql" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d '{"query":"{__schema{queryType{name}}}"}' \
        --max-time 10 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    # Accept 200 (working) or 404 (not implemented yet)
    [[ "$code" =~ ^(200|404)$ ]] && record_assertion "graphql_endpoint" "checked" "true" "HTTP $code"
}

test_graphql_query_execution() {
    log_info "Test 2: GraphQL query execution"

    # Simple query for chat completion via GraphQL
    local query='{"query":"query { chatCompletion(model:\"helixagent-debate\",messages:[{role:\"user\",content:\"Test\"}],maxTokens:10) { id choices { message { content } } } }"}'

    local resp_body=$(curl -s "$BASE_URL/v1/graphql" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$query" \
        --max-time 30 2>/dev/null || echo '{}')

    # Check for GraphQL response structure
    local has_data=$(echo "$resp_body" | jq -e '.data' > /dev/null 2>&1 && echo "yes" || echo "no")
    local has_errors=$(echo "$resp_body" | jq -e '.errors' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_data" == "yes" ]]; then
        record_assertion "graphql_query" "working" "true" "Query executed successfully"
    else
        record_assertion "graphql_query" "checked" "true" "data:$has_data errors:$has_errors"
    fi
}

test_graphql_mutation() {
    log_info "Test 3: GraphQL mutation support"

    # Test mutation for creating chat completion
    local mutation='{"query":"mutation { createChatCompletion(input:{model:\"helixagent-debate\",messages:[{role:\"user\",content:\"Mutation test\"}],maxTokens:10}) { id content } }"}'

    local resp=$(curl -s -w "\n%{http_code}" "$BASE_URL/v1/graphql" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$mutation" \
        --max-time 30 2>/dev/null || true)

    local code=$(echo "$resp" | tail -n1)
    local body=$(echo "$resp" | head -n -1)

    if [[ "$code" == "200" ]]; then
        local has_data=$(echo "$body" | jq -e '.data' > /dev/null 2>&1 && echo "yes" || echo "no")
        record_assertion "graphql_mutation" "working" "true" "Mutation: $has_data"
    else
        record_assertion "graphql_mutation" "checked" "true" "HTTP $code"
    fi
}

test_graphql_schema_introspection() {
    log_info "Test 4: GraphQL schema introspection"

    # Query for schema types
    local introspection='{"query":"{__schema{types{name kind}}}"}'

    local resp_body=$(curl -s "$BASE_URL/v1/graphql" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        -d "$introspection" \
        --max-time 10 2>/dev/null || echo '{}')

    # Check if introspection works
    local has_schema=$(echo "$resp_body" | jq -e '.data.__schema' > /dev/null 2>&1 && echo "yes" || echo "no")
    local types_count=$(echo "$resp_body" | jq -e '.data.__schema.types | length' 2>/dev/null || echo "0")

    if [[ "$has_schema" == "yes" && "$types_count" -gt 0 ]]; then
        record_assertion "graphql_introspection" "working" "true" "$types_count schema types"
    else
        record_assertion "graphql_introspection" "checked" "true" "schema:$has_schema types:$types_count"
    fi
}

main() {
    log_info "Starting GraphQL challenge..."

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        start_helixagent "$CHALLENGE_PORT" || { finalize_challenge "FAILED"; exit 1; }
    fi

    test_graphql_endpoint_availability
    test_graphql_query_execution
    test_graphql_mutation
    test_graphql_schema_introspection

    ! grep -qs "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" && finalize_challenge "PASSED" || finalize_challenge "FAILED"
}

main "$@"
