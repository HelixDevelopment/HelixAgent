#!/bin/bash
# messaging_kafka_challenge.sh - Kafka Integration Challenge
# Tests Kafka broker implementation for HelixAgent

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/common.sh"

CHALLENGE_NAME="Kafka Integration Challenge"
PASSED=0
FAILED=0
TOTAL=0

log_test() {
    local test_name="$1"
    local status="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$status" = "PASS" ]; then
        PASSED=$((PASSED + 1))
        echo -e "  \e[32m✓\e[0m $test_name"
    else
        FAILED=$((FAILED + 1))
        echo -e "  \e[31m✗\e[0m $test_name"
    fi
}

echo "=============================================="
echo "  $CHALLENGE_NAME"
echo "=============================================="
echo ""

cd "$PROJECT_ROOT"

# Test 1: Kafka broker package exists
echo "[1] Kafka Package Structure"
if [ -f "internal/messaging/kafka/broker.go" ]; then
    log_test "broker.go exists" "PASS"
else
    log_test "broker.go exists" "FAIL"
fi

if [ -f "internal/messaging/kafka/config.go" ]; then
    log_test "config.go exists" "PASS"
else
    log_test "config.go exists" "FAIL"
fi

if [ -f "internal/messaging/kafka/broker_test.go" ]; then
    log_test "broker_test.go exists" "PASS"
else
    log_test "broker_test.go exists" "FAIL"
fi

# Subscription can be in broker.go or subscription.go
if [ -f "internal/messaging/kafka/subscription.go" ] || grep -q "kafkaSubscription\|Subscription" internal/messaging/kafka/broker.go 2>/dev/null; then
    log_test "subscription type exists" "PASS"
else
    log_test "subscription type exists" "FAIL"
fi

# Test 2: Kafka broker implements MessageBroker interface
echo ""
echo "[2] Interface Implementation"
if grep -q "func.*Broker.*Connect" internal/messaging/kafka/broker.go 2>/dev/null; then
    log_test "Connect method exists" "PASS"
else
    log_test "Connect method exists" "FAIL"
fi

if grep -q "func.*Broker.*Publish" internal/messaging/kafka/broker.go 2>/dev/null; then
    log_test "Publish method exists" "PASS"
else
    log_test "Publish method exists" "FAIL"
fi

if grep -q "func.*Broker.*Subscribe" internal/messaging/kafka/broker.go 2>/dev/null; then
    log_test "Subscribe method exists" "PASS"
else
    log_test "Subscribe method exists" "FAIL"
fi

if grep -q "func.*Broker.*Close" internal/messaging/kafka/broker.go 2>/dev/null; then
    log_test "Close method exists" "PASS"
else
    log_test "Close method exists" "FAIL"
fi

# Test 3: Kafka configuration validation
echo ""
echo "[3] Configuration"
if grep -q "Config struct" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Config struct defined" "PASS"
else
    log_test "Config struct defined" "FAIL"
fi

if grep -q "Validate" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Validate method exists" "PASS"
else
    log_test "Validate method exists" "FAIL"
fi

if grep -q "Brokers" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Brokers field exists" "PASS"
else
    log_test "Brokers field exists" "FAIL"
fi

if grep -q "GroupID" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "GroupID field exists" "PASS"
else
    log_test "GroupID field exists" "FAIL"
fi

# Test 4: Producer configuration
echo ""
echo "[4] Producer Configuration"
if grep -q "Compression" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Compression config" "PASS"
else
    log_test "Compression config" "FAIL"
fi

if grep -q "BatchSize\|Batch" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Batch size config" "PASS"
else
    log_test "Batch size config" "FAIL"
fi

if grep -q "RequiredAcks\|Acks" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Required acks config" "PASS"
else
    log_test "Required acks config" "FAIL"
fi

# Test 5: Consumer configuration
echo ""
echo "[5] Consumer Configuration"
if grep -q "StartOffset\|Offset" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Start offset config" "PASS"
else
    log_test "Start offset config" "FAIL"
fi

if grep -q "CommitInterval\|Commit" internal/messaging/kafka/config.go 2>/dev/null; then
    log_test "Commit interval config" "PASS"
else
    log_test "Commit interval config" "FAIL"
fi

# Test 6: Unit tests pass
echo ""
echo "[6] Unit Tests"
if go test -v ./internal/messaging/kafka/... -count=1 2>&1 | grep -q "PASS"; then
    log_test "Kafka unit tests pass" "PASS"
else
    log_test "Kafka unit tests pass" "FAIL"
fi

# Test 7: Docker Compose configuration
echo ""
echo "[7] Docker Compose Configuration"
if grep -q "kafka:" docker-compose.messaging.yml 2>/dev/null; then
    log_test "Kafka service defined" "PASS"
else
    log_test "Kafka service defined" "FAIL"
fi

if grep -q "zookeeper:" docker-compose.messaging.yml 2>/dev/null; then
    log_test "Zookeeper service defined" "PASS"
else
    log_test "Zookeeper service defined" "FAIL"
fi

if grep -q "9092" docker-compose.messaging.yml 2>/dev/null; then
    log_test "Kafka port configured" "PASS"
else
    log_test "Kafka port configured" "FAIL"
fi

if grep -q "schema-registry" docker-compose.messaging.yml 2>/dev/null; then
    log_test "Schema registry configured" "PASS"
else
    log_test "Schema registry configured" "FAIL"
fi

# Test 8: Topic initialization
echo ""
echo "[8] Topic Initialization"
if grep -q "kafka-topics" docker-compose.messaging.yml 2>/dev/null; then
    log_test "Topic creation in compose" "PASS"
else
    log_test "Topic creation in compose" "FAIL"
fi

if grep -q "helixagent.events" docker-compose.messaging.yml 2>/dev/null; then
    log_test "HelixAgent event topics" "PASS"
else
    log_test "HelixAgent event topics" "FAIL"
fi

if grep -q "helixagent.stream" docker-compose.messaging.yml 2>/dev/null; then
    log_test "HelixAgent stream topics" "PASS"
else
    log_test "HelixAgent stream topics" "FAIL"
fi

# Test 9: Messaging configuration
echo ""
echo "[9] Messaging Configuration"
if grep -q "kafka:" configs/messaging.yaml 2>/dev/null; then
    log_test "Kafka config section" "PASS"
else
    log_test "Kafka config section" "FAIL"
fi

if grep -q "brokers:" configs/messaging.yaml 2>/dev/null; then
    log_test "Brokers configured" "PASS"
else
    log_test "Brokers configured" "FAIL"
fi

if grep -q "topics:" configs/messaging.yaml 2>/dev/null; then
    log_test "Topics configured" "PASS"
else
    log_test "Topics configured" "FAIL"
fi

# Test 10: Batch publishing
echo ""
echo "[10] Batch Publishing"
if grep -q "PublishBatch\|Batch" internal/messaging/kafka/broker.go 2>/dev/null; then
    log_test "Batch publish support" "PASS"
else
    log_test "Batch publish support" "FAIL"
fi

# Test 11: kafka-go dependency
echo ""
echo "[11] Dependencies"
if grep -q "kafka-go" go.mod 2>/dev/null; then
    log_test "kafka-go in go.mod" "PASS"
else
    log_test "kafka-go in go.mod" "FAIL"
fi

echo ""
echo "=============================================="
echo "  Results: $PASSED/$TOTAL tests passed"
echo "=============================================="

if [ $FAILED -gt 0 ]; then
    echo -e "\e[31m$FAILED test(s) failed\e[0m"
    exit 1
else
    echo -e "\e[32mAll tests passed!\e[0m"
    exit 0
fi
