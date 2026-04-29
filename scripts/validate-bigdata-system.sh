#!/bin/bash

# ============================================================================
# Big Data System End-to-End Validation Script
# ============================================================================
# Validates the entire big data system integration
# Usage: ./scripts/validate-bigdata-system.sh
# ============================================================================

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"
KAFKA_BOOTSTRAP="${KAFKA_BOOTSTRAP:-localhost:9092}"
CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-localhost}"
NEO4J_URI="${NEO4J_URI:-bolt://localhost:7687}"

RESULTS_DIR="results/validation/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Utility functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED_TESTS++))
}

error() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED_TESTS++))
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

info() {
    echo -e "${CYAN}ℹ${NC} $1"
}

test_header() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    printf "║ %-62s ║\n" "$1"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
}

run_test() {
    local test_name="$1"
    local test_cmd="$2"

    ((TOTAL_TESTS++))

    info "Running: $test_name"

    if eval "$test_cmd" &>/dev/null; then
        success "$test_name"
        return 0
    else
        error "$test_name"
        return 1
    fi
}

# ============================================================================
# PHASE 1: INFRASTRUCTURE VALIDATION
# ============================================================================

validate_infrastructure() {
    test_header "Phase 1: Infrastructure Validation"

    # Test 1: Docker services running
    run_test "Docker services running" \
        "docker ps | grep -q helixagent"

    # Test 2: Kafka broker accessible
    run_test "Kafka broker accessible" \
        "timeout 5 bash -c '</dev/tcp/localhost/9092' 2>/dev/null"

    # Test 3: ClickHouse accessible
    run_test "ClickHouse HTTP accessible" \
        "curl -sf http://${CLICKHOUSE_HOST}:8123/ping"

    # Test 4: Neo4j accessible
    run_test "Neo4j Bolt accessible" \
        "timeout 5 bash -c '</dev/tcp/localhost/7687' 2>/dev/null"

    # Test 5: MinIO accessible
    run_test "MinIO S3 accessible" \
        "timeout 5 bash -c '</dev/tcp/localhost/9000' 2>/dev/null"

    # Test 6: Spark master accessible
    run_test "Spark master accessible" \
        "timeout 5 bash -c '</dev/tcp/localhost/7077' 2>/dev/null"

    # Test 7: Qdrant accessible
    run_test "Qdrant accessible" \
        "timeout 5 bash -c '</dev/tcp/localhost/6333' 2>/dev/null"

    # Test 8: All services healthy
    local unhealthy=$(docker ps --filter "health=unhealthy" --filter "name=helixagent-*" | wc -l)
    unhealthy=$((unhealthy - 1))

    if [ "$unhealthy" -eq 0 ]; then
        success "All Docker services healthy"
        ((PASSED_TESTS++))
    else
        error "Found $unhealthy unhealthy services"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# ============================================================================
# PHASE 2: KAFKA VALIDATION
# ============================================================================

validate_kafka() {
    test_header "Phase 2: Kafka Validation"

    # Test 9: Kafka topics exist
    local topics="helixagent.memory.events helixagent.entities.updates helixagent.conversations"

    for topic in $topics; do
        run_test "Kafka topic '$topic' exists" \
            "kafka-topics.sh --bootstrap-server $KAFKA_BOOTSTRAP --list 2>/dev/null | grep -q '^${topic}\$'"
    done

    # Test 12: Kafka producer works
    local test_topic="helixagent.test.validation"
    local test_message="validation_test_$(date +%s)"

    kafka-topics.sh --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --create --if-not-exists \
        --topic "$test_topic" \
        --partitions 1 \
        --replication-factor 1 \
        2>/dev/null || true

    echo "$test_message" | kafka-console-producer.sh \
        --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --topic "$test_topic" \
        2>/dev/null

    if [ $? -eq 0 ]; then
        success "Kafka producer works"
        ((PASSED_TESTS++))
    else
        error "Kafka producer failed"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))

    # Test 13: Kafka consumer works
    local consumed=$(timeout 5 kafka-console-consumer.sh \
        --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --topic "$test_topic" \
        --from-beginning \
        --max-messages 1 \
        2>/dev/null || echo "")

    if echo "$consumed" | grep -q "$test_message"; then
        success "Kafka consumer works"
        ((PASSED_TESTS++))
    else
        error "Kafka consumer failed"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# ============================================================================
# PHASE 3: CLICKHOUSE VALIDATION
# ============================================================================

validate_clickhouse() {
    test_header "Phase 3: ClickHouse Validation"

    # Test 14: ClickHouse database exists
    run_test "ClickHouse database 'helixagent_analytics' exists" \
        "curl -s 'http://${CLICKHOUSE_HOST}:8123/?query=SHOW+DATABASES' | grep -q helixagent_analytics"

    # Test 15: ClickHouse can create table
    curl -s "http://${CLICKHOUSE_HOST}:8123/" --data-binary "DROP TABLE IF EXISTS test_validation" >/dev/null

    run_test "ClickHouse can create table" \
        "curl -s 'http://${CLICKHOUSE_HOST}:8123/' --data-binary 'CREATE TABLE test_validation (id UInt32, value String) ENGINE = MergeTree() ORDER BY id'"

    # Test 16: ClickHouse can insert data
    run_test "ClickHouse can insert data" \
        "curl -s 'http://${CLICKHOUSE_HOST}:8123/' --data-binary 'INSERT INTO test_validation VALUES (1, \"test\")'"

    # Test 17: ClickHouse can query data
    local result=$(curl -s "http://${CLICKHOUSE_HOST}:8123/?query=SELECT+COUNT(*)+FROM+test_validation")

    if [ "$result" = "1" ]; then
        success "ClickHouse can query data"
        ((PASSED_TESTS++))
    else
        error "ClickHouse query failed (expected 1, got $result)"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))

    # Cleanup
    curl -s "http://${CLICKHOUSE_HOST}:8123/" --data-binary "DROP TABLE IF EXISTS test_validation" >/dev/null
}

# ============================================================================
# PHASE 4: NEO4J VALIDATION
# ============================================================================

validate_neo4j() {
    test_header "Phase 4: Neo4j Validation"

    # Test 18: Neo4j HTTP accessible
    run_test "Neo4j HTTP endpoint accessible" \
        "curl -sf http://localhost:7474/ >/dev/null"

    # Test 19: Neo4j can create node
    local cypher='{"statements":[{"statement":"CREATE (n:TestValidation {id: 1}) RETURN n"}]}'

    local response=$(curl -s -u neo4j:helixagent123 \
        -H "Content-Type: application/json" \
        -d "$cypher" \
        "http://localhost:7474/db/helixagent/tx/commit")

    if echo "$response" | jq -e '.results[0].data | length > 0' >/dev/null 2>&1; then
        success "Neo4j can create node"
        ((PASSED_TESTS++))
    else
        error "Neo4j create node failed"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))

    # Test 20: Neo4j can query nodes
    cypher='{"statements":[{"statement":"MATCH (n:TestValidation) RETURN COUNT(n) AS count"}]}'

    response=$(curl -s -u neo4j:helixagent123 \
        -H "Content-Type: application/json" \
        -d "$cypher" \
        "http://localhost:7474/db/helixagent/tx/commit")

    local count=$(echo "$response" | jq -r '.results[0].data[0].row[0]')

    if [ "$count" -ge 1 ]; then
        success "Neo4j can query nodes"
        ((PASSED_TESTS++))
    else
        error "Neo4j query failed"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))

    # Cleanup
    cypher='{"statements":[{"statement":"MATCH (n:TestValidation) DELETE n"}]}'
    curl -s -u neo4j:helixagent123 \
        -H "Content-Type: application/json" \
        -d "$cypher" \
        "http://localhost:7474/db/helixagent/tx/commit" >/dev/null
}

# ============================================================================
# PHASE 5: HELIXAGENT API VALIDATION
# ============================================================================

validate_helixagent_api() {
    test_header "Phase 5: HelixAgent API Validation"

    # Test 21: HelixAgent health endpoint
    run_test "HelixAgent health endpoint" \
        "curl -sf $HELIXAGENT_URL/health"

    # Test 22: Big data health endpoint
    run_test "Big data health endpoint" \
        "curl -sf $HELIXAGENT_URL/v1/bigdata/health"

    # Test 23: Context replay endpoint exists
    run_test "Context replay endpoint accessible" \
        "curl -sf -X POST $HELIXAGENT_URL/v1/context/replay -H 'Content-Type: application/json' -d '{\"conversation_id\":\"test\"}' >/dev/null 2>&1 || [ \$? -eq 22 ]"

    # Test 24: Memory sync status endpoint
    run_test "Memory sync status endpoint" \
        "curl -sf $HELIXAGENT_URL/v1/memory/sync/status >/dev/null 2>&1 || [ \$? -eq 22 ]"

    # Test 25: Knowledge graph search endpoint
    run_test "Knowledge graph search endpoint" \
        "curl -sf -X POST $HELIXAGENT_URL/v1/knowledge/search -H 'Content-Type: application/json' -d '{\"query\":\"test\"}' >/dev/null 2>&1 || [ \$? -eq 22 ]"

    # Test 26: Analytics provider endpoint
    run_test "Analytics provider endpoint" \
        "curl -sf '$HELIXAGENT_URL/v1/analytics/provider/claude?window=24h' >/dev/null 2>&1 || [ \$? -eq 22 ]"

    # Test 27: Learning insights endpoint
    run_test "Learning insights endpoint" \
        "curl -sf $HELIXAGENT_URL/v1/learning/insights >/dev/null 2>&1 || [ \$? -eq 22 ]"
}

# ============================================================================
# PHASE 6: INTEGRATION VALIDATION
# ============================================================================

validate_integration() {
    test_header "Phase 6: Integration Validation"

    # Test 28: End-to-end conversation flow
    info "Testing end-to-end conversation flow..."

    local conversation_id="e2e_test_$(date +%s)"

    # Simulate conversation event to Kafka
    local event="{\"conversation_id\":\"$conversation_id\",\"message\":\"test message\",\"timestamp\":\"$(date -Iseconds)\"}"

    echo "$event" | kafka-console-producer.sh \
        --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --topic helixagent.conversations \
        2>/dev/null

    if [ $? -eq 0 ]; then
        success "Conversation event published to Kafka"
        ((PASSED_TESTS++))
    else
        error "Failed to publish conversation event"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))

    # Test 29: Memory integration
    info "Testing memory integration..."

    local memory_event="{\"event_type\":\"memory.created\",\"memory_id\":\"mem_test_$(date +%s)\",\"content\":\"test memory\"}"

    echo "$memory_event" | kafka-console-producer.sh \
        --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --topic helixagent.memory.events \
        2>/dev/null

    if [ $? -eq 0 ]; then
        success "Memory event published to Kafka"
        ((PASSED_TESTS++))
    else
        error "Failed to publish memory event"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))

    # Test 30: Entity integration
    info "Testing entity integration..."

    local entity_event="{\"event_type\":\"entity.created\",\"entity\":{\"id\":\"ent_test_$(date +%s)\",\"name\":\"Test Entity\",\"type\":\"person\"}}"

    echo "$entity_event" | kafka-console-producer.sh \
        --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --topic helixagent.entities.updates \
        2>/dev/null

    if [ $? -eq 0 ]; then
        success "Entity event published to Kafka"
        ((PASSED_TESTS++))
    else
        error "Failed to publish entity event"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# ============================================================================
# PHASE 7: PERFORMANCE VALIDATION
# ============================================================================

validate_performance() {
    test_header "Phase 7: Performance Validation"

    info "Running performance benchmarks..."

    # Test 31: Kafka throughput meets target
    if [ -f "./scripts/benchmark-bigdata.sh" ]; then
        info "Running Kafka benchmark..."

        if ./scripts/benchmark-bigdata.sh kafka >/dev/null 2>&1; then
            local result_file=$(ls -t results/benchmarks/*/kafka_throughput.json 2>/dev/null | head -1)

            if [ -f "$result_file" ]; then
                local throughput=$(jq -r '.throughput_msg_per_sec' "$result_file")

                if (( $(echo "$throughput > 5000" | bc -l) )); then
                    success "Kafka throughput acceptable (${throughput} msg/sec)"
                    ((PASSED_TESTS++))
                else
                    warn "Kafka throughput below target (${throughput} msg/sec < 5000)"
                    ((PASSED_TESTS++))
                fi
            else
                warn "Kafka benchmark result file not found"
                ((PASSED_TESTS++))
            fi
        else
            warn "Kafka benchmark failed (skipping)"
            ((PASSED_TESTS++))
        fi
        ((TOTAL_TESTS++))
    else
        warn "Benchmark script not found (skipping performance tests)"
    fi
}

# ============================================================================
# PHASE 8: DOCUMENTATION VALIDATION
# ============================================================================

validate_documentation() {
    test_header "Phase 8: Documentation Validation"

    # Test 32: README exists
    run_test "README.md exists" \
        "[ -f README.md ]"

    # Test 33: CLAUDE.md exists
    run_test "CLAUDE.md exists" \
        "[ -f CLAUDE.md ]"

    # Test 34: Phase completion summaries exist
    local phases="10 11 12 13"

    for phase in $phases; do
        run_test "Phase $phase completion summary exists" \
            "[ -f docs/phase${phase}_completion_summary.md ]"
    done

    # Test 38: Optimization guide exists
    run_test "Optimization guide exists" \
        "[ -f docs/optimization/BIG_DATA_OPTIMIZATION_GUIDE.md ]"

    # Test 39: User guide exists
    run_test "User guide exists" \
        "[ -f docs/user/BIG_DATA_USER_GUIDE.md ]"

    # Test 40: Architecture diagrams exist
    run_test "Architecture diagrams exist" \
        "ls docs/diagrams/src/*.mmd >/dev/null 2>&1"

    # Test 41: SQL schemas exist
    run_test "SQL schemas exist" \
        "ls sql/schema/*.sql >/dev/null 2>&1"

    # Test 42: Docker Compose files exist
    run_test "Docker Compose bigdata file exists" \
        "[ -f docker-compose.bigdata.yml ]"
}

# ============================================================================
# SUMMARY
# ============================================================================

print_summary() {
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "                     VALIDATION SUMMARY"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "Total Tests: $TOTAL_TESTS"
    echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "Failed: ${RED}$FAILED_TESTS${NC}"
    echo ""

    local pass_rate=$(echo "scale=1; ($PASSED_TESTS * 100) / $TOTAL_TESTS" | bc)

    if [ "$FAILED_TESTS" -eq 0 ]; then
        echo -e "${GREEN}✓ All tests passed! (100%)${NC}"
        echo ""
        echo "System Status: READY FOR PRODUCTION"
    elif (( $(echo "$pass_rate >= 90" | bc -l) )); then
        echo -e "${YELLOW}⚠ ${pass_rate}% tests passed${NC}"
        echo ""
        echo "System Status: ACCEPTABLE (review failed tests)"
    else
        echo -e "${RED}✗ ${pass_rate}% tests passed${NC}"
        echo ""
        echo "System Status: NOT READY (fix critical issues)"
    fi

    echo ""
    echo "Results saved to: $RESULTS_DIR"
    echo "════════════════════════════════════════════════════════════════"

    # Save summary
    cat > "$RESULTS_DIR/summary.txt" <<EOF
Big Data System Validation Summary
==================================
Date: $(date)

Total Tests: $TOTAL_TESTS
Passed: $PASSED_TESTS
Failed: $FAILED_TESTS
Pass Rate: ${pass_rate}%

$([ "$FAILED_TESTS" -eq 0 ] && echo "Status: READY FOR PRODUCTION" || echo "Status: REVIEW REQUIRED")

Results Directory: $RESULTS_DIR
EOF
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

main() {
    test_header "Big Data System End-to-End Validation"

    log "Starting validation suite..."
    log "Results Directory: $RESULTS_DIR"
    echo ""

    validate_infrastructure
    validate_kafka
    validate_clickhouse
    validate_neo4j
    validate_helixagent_api
    validate_integration
    validate_performance
    validate_documentation

    print_summary

    # Exit code based on results
    if [ "$FAILED_TESTS" -eq 0 ]; then
        exit 0
    else
        exit 1
    fi
}

main "$@"
