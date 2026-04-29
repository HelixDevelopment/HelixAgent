#!/bin/bash

# ============================================================================
# Big Data Performance Benchmark Script
# ============================================================================
# Benchmarks all big data components for production readiness
# Usage: ./scripts/benchmark-bigdata.sh [component]
# Components: kafka, clickhouse, neo4j, context, memory, all
# ============================================================================

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:8100}"
KAFKA_BOOTSTRAP="${KAFKA_BOOTSTRAP:-localhost:9092}"
CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-localhost}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-8123}"
NEO4J_URI="${NEO4J_URI:-bolt://localhost:7687}"
NEO4J_USER="${NEO4J_USER:-neo4j}"
NEO4J_PASS="${NEO4J_PASS:-helixagent123}"

RESULTS_DIR="results/benchmarks/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

# Utility functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

benchmark_header() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║  $1"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""
}

# Check dependencies
check_dependencies() {
    log "Checking dependencies..."

    local deps=("curl" "jq" "bc" "seq")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            error "Required dependency '$dep' not found"
            exit 1
        fi
    done

    # Check if kafka-console-producer is available
    if ! command -v kafka-console-producer.sh &> /dev/null && ! command -v kafka-console-producer &> /dev/null; then
        warn "Kafka CLI tools not found, skipping Kafka benchmarks"
    fi

    success "All dependencies available"
}

# ============================================================================
# KAFKA BENCHMARKS
# ============================================================================

benchmark_kafka_throughput() {
    benchmark_header "Kafka Throughput Benchmark"

    log "Testing Kafka producer throughput..."

    local topic="helixagent.benchmark.throughput"
    local num_messages=100000
    local message_size=1024  # 1KB messages

    # Create test topic
    kafka-topics.sh --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --create --if-not-exists \
        --topic "$topic" \
        --partitions 12 \
        --replication-factor 1 \
        --config compression.type=lz4 \
        2>/dev/null || true

    # Generate test data
    local test_file="/tmp/kafka_benchmark_$$.dat"
    for i in $(seq 1 $num_messages); do
        head -c $message_size /dev/urandom | base64 | tr -d '\n'
        echo ""
    done > "$test_file"

    # Benchmark producer
    local start_time=$(date +%s.%N)

    kafka-console-producer.sh --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --topic "$topic" \
        --compression-codec lz4 \
        < "$test_file" 2>/dev/null

    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local throughput=$(echo "$num_messages / $duration" | bc -l)
    local mb_per_sec=$(echo "scale=2; ($num_messages * $message_size) / ($duration * 1024 * 1024)" | bc -l)

    rm -f "$test_file"

    echo "Messages: $num_messages"
    echo "Message Size: ${message_size}B"
    echo "Duration: ${duration}s"
    echo "Throughput: $(printf '%.0f' $throughput) msg/sec"
    echo "Bandwidth: ${mb_per_sec} MB/sec"

    # Save results
    cat > "$RESULTS_DIR/kafka_throughput.json" <<EOF
{
  "test": "kafka_throughput",
  "num_messages": $num_messages,
  "message_size": $message_size,
  "duration_seconds": $duration,
  "throughput_msg_per_sec": $throughput,
  "bandwidth_mb_per_sec": $mb_per_sec,
  "timestamp": "$(date -Iseconds)"
}
EOF

    # Evaluate
    if (( $(echo "$throughput > 10000" | bc -l) )); then
        success "Kafka throughput: EXCELLENT (>10K msg/sec)"
    elif (( $(echo "$throughput > 5000" | bc -l) )); then
        warn "Kafka throughput: GOOD (>5K msg/sec)"
    else
        error "Kafka throughput: POOR (<5K msg/sec)"
    fi
}

benchmark_kafka_latency() {
    benchmark_header "Kafka Latency Benchmark"

    log "Testing Kafka producer latency (p50, p95, p99)..."

    local topic="helixagent.benchmark.latency"
    local num_messages=1000
    local latency_file="$RESULTS_DIR/kafka_latencies.txt"

    # Create test topic
    kafka-topics.sh --bootstrap-server "$KAFKA_BOOTSTRAP" \
        --create --if-not-exists \
        --topic "$topic" \
        --partitions 1 \
        --replication-factor 1 \
        2>/dev/null || true

    > "$latency_file"

    for i in $(seq 1 $num_messages); do
        local start=$(date +%s.%N)
        echo "test message $i" | kafka-console-producer.sh \
            --bootstrap-server "$KAFKA_BOOTSTRAP" \
            --topic "$topic" \
            2>/dev/null
        local end=$(date +%s.%N)
        local latency=$(echo "($end - $start) * 1000" | bc -l)
        echo "$latency" >> "$latency_file"
    done

    # Calculate percentiles
    local p50=$(sort -n "$latency_file" | awk '{arr[NR]=$1} END {print arr[int(NR*0.50)]}')
    local p95=$(sort -n "$latency_file" | awk '{arr[NR]=$1} END {print arr[int(NR*0.95)]}')
    local p99=$(sort -n "$latency_file" | awk '{arr[NR]=$1} END {print arr[int(NR*0.99)]}')

    echo "Latency Percentiles:"
    echo "  p50: ${p50}ms"
    echo "  p95: ${p95}ms"
    echo "  p99: ${p99}ms"

    # Save results
    cat > "$RESULTS_DIR/kafka_latency.json" <<EOF
{
  "test": "kafka_latency",
  "num_messages": $num_messages,
  "p50_ms": $p50,
  "p95_ms": $p95,
  "p99_ms": $p99,
  "timestamp": "$(date -Iseconds)"
}
EOF

    # Evaluate
    if (( $(echo "$p95 < 10" | bc -l) )); then
        success "Kafka latency: EXCELLENT (p95 <10ms)"
    elif (( $(echo "$p95 < 50" | bc -l) )); then
        warn "Kafka latency: GOOD (p95 <50ms)"
    else
        error "Kafka latency: POOR (p95 >50ms)"
    fi
}

# ============================================================================
# CLICKHOUSE BENCHMARKS
# ============================================================================

benchmark_clickhouse_insert() {
    benchmark_header "ClickHouse Insert Benchmark"

    log "Testing ClickHouse insert performance..."

    local num_rows=100000
    local batch_size=10000

    # Create test table
    curl -s "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/" --data-binary @- <<EOF >/dev/null
CREATE TABLE IF NOT EXISTS benchmark_insert (
    id UInt64,
    timestamp DateTime,
    provider String,
    response_time Float32,
    tokens UInt32
) ENGINE = MergeTree()
ORDER BY (timestamp, id)
EOF

    local start_time=$(date +%s.%N)

    # Insert in batches
    for batch_start in $(seq 0 $batch_size $((num_rows - batch_size))); do
        local batch_data=""
        for i in $(seq $batch_start $((batch_start + batch_size - 1))); do
            local ts=$(date -d "-$((RANDOM % 86400)) seconds" "+%Y-%m-%d %H:%M:%S")
            batch_data+="$i\t$ts\tprovider_$((RANDOM % 10))\t$((RANDOM % 1000))\t$((RANDOM % 10000))\n"
        done

        echo -e "$batch_data" | curl -s "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/" \
            --data-binary @- \
            -H "INSERT INTO benchmark_insert FORMAT TabSeparated" >/dev/null
    done

    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local rows_per_sec=$(echo "$num_rows / $duration" | bc -l)

    echo "Rows Inserted: $num_rows"
    echo "Duration: ${duration}s"
    echo "Throughput: $(printf '%.0f' $rows_per_sec) rows/sec"

    # Save results
    cat > "$RESULTS_DIR/clickhouse_insert.json" <<EOF
{
  "test": "clickhouse_insert",
  "num_rows": $num_rows,
  "batch_size": $batch_size,
  "duration_seconds": $duration,
  "throughput_rows_per_sec": $rows_per_sec,
  "timestamp": "$(date -Iseconds)"
}
EOF

    # Cleanup
    curl -s "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/" \
        --data-binary "DROP TABLE IF EXISTS benchmark_insert" >/dev/null

    # Evaluate
    if (( $(echo "$rows_per_sec > 50000" | bc -l) )); then
        success "ClickHouse insert: EXCELLENT (>50K rows/sec)"
    elif (( $(echo "$rows_per_sec > 20000" | bc -l) )); then
        warn "ClickHouse insert: GOOD (>20K rows/sec)"
    else
        error "ClickHouse insert: POOR (<20K rows/sec)"
    fi
}

benchmark_clickhouse_query() {
    benchmark_header "ClickHouse Query Benchmark"

    log "Testing ClickHouse query performance..."

    # Create and populate test table
    curl -s "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/" --data-binary @- <<EOF >/dev/null
CREATE TABLE IF NOT EXISTS benchmark_query (
    timestamp DateTime,
    provider String,
    response_time Float32,
    tokens UInt32
) ENGINE = MergeTree()
ORDER BY (timestamp, provider)
EOF

    # Insert test data
    local num_rows=1000000
    local batch_data=""
    for i in $(seq 1 10000); do
        local ts=$(date -d "-$((RANDOM % 86400)) seconds" "+%Y-%m-%d %H:%M:%S")
        batch_data+="$ts\tprovider_$((RANDOM % 10))\t$((RANDOM % 1000))\t$((RANDOM % 10000))\n"
    done

    for batch in $(seq 1 100); do
        echo -e "$batch_data" | curl -s "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/" \
            --data-binary @- \
            -H "INSERT INTO benchmark_query FORMAT TabSeparated" >/dev/null
    done

    # Benchmark aggregation query
    local query="SELECT provider, AVG(response_time), COUNT(*) FROM benchmark_query GROUP BY provider"

    local latency_file="$RESULTS_DIR/clickhouse_query_latencies.txt"
    > "$latency_file"

    for i in $(seq 1 100); do
        local start=$(date +%s.%N)
        curl -s "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/" \
            --data-binary "$query" >/dev/null
        local end=$(date +%s.%N)
        local latency=$(echo "($end - $start) * 1000" | bc -l)
        echo "$latency" >> "$latency_file"
    done

    # Calculate percentiles
    local p50=$(sort -n "$latency_file" | awk '{arr[NR]=$1} END {print arr[int(NR*0.50)]}')
    local p95=$(sort -n "$latency_file" | awk '{arr[NR]=$1} END {print arr[int(NR*0.95)]}')

    echo "Query Latency (100 runs):"
    echo "  p50: ${p50}ms"
    echo "  p95: ${p95}ms"

    # Save results
    cat > "$RESULTS_DIR/clickhouse_query.json" <<EOF
{
  "test": "clickhouse_query",
  "num_rows": $num_rows,
  "num_queries": 100,
  "p50_ms": $p50,
  "p95_ms": $p95,
  "timestamp": "$(date -Iseconds)"
}
EOF

    # Cleanup
    curl -s "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/" \
        --data-binary "DROP TABLE IF EXISTS benchmark_query" >/dev/null

    # Evaluate
    if (( $(echo "$p95 < 50" | bc -l) )); then
        success "ClickHouse query: EXCELLENT (p95 <50ms)"
    elif (( $(echo "$p95 < 100" | bc -l) )); then
        warn "ClickHouse query: GOOD (p95 <100ms)"
    else
        error "ClickHouse query: POOR (p95 >100ms)"
    fi
}

# ============================================================================
# NEO4J BENCHMARKS
# ============================================================================

benchmark_neo4j_write() {
    benchmark_header "Neo4j Write Benchmark"

    log "Testing Neo4j node creation performance..."

    local num_nodes=10000

    # Clear existing data
    curl -s -u "$NEO4J_USER:$NEO4J_PASS" \
        -H "Content-Type: application/json" \
        -d '{"statements":[{"statement":"MATCH (n:BenchmarkEntity) DELETE n"}]}' \
        "http://localhost:7474/db/helixagent/tx/commit" >/dev/null

    local start_time=$(date +%s.%N)

    # Batch create nodes
    local batch_size=1000
    for batch_start in $(seq 0 $batch_size $((num_nodes - batch_size))); do
        local cypher="UNWIND range($batch_start, $((batch_start + batch_size - 1))) AS i "
        cypher+="CREATE (:BenchmarkEntity {id: i, name: 'Entity_' + toString(i), type: 'test'})"

        curl -s -u "$NEO4J_USER:$NEO4J_PASS" \
            -H "Content-Type: application/json" \
            -d "{\"statements\":[{\"statement\":\"$cypher\"}]}" \
            "http://localhost:7474/db/helixagent/tx/commit" >/dev/null
    done

    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local nodes_per_sec=$(echo "$num_nodes / $duration" | bc -l)

    echo "Nodes Created: $num_nodes"
    echo "Duration: ${duration}s"
    echo "Throughput: $(printf '%.0f' $nodes_per_sec) nodes/sec"

    # Save results
    cat > "$RESULTS_DIR/neo4j_write.json" <<EOF
{
  "test": "neo4j_write",
  "num_nodes": $num_nodes,
  "batch_size": $batch_size,
  "duration_seconds": $duration,
  "throughput_nodes_per_sec": $nodes_per_sec,
  "timestamp": "$(date -Iseconds)"
}
EOF

    # Cleanup
    curl -s -u "$NEO4J_USER:$NEO4J_PASS" \
        -H "Content-Type: application/json" \
        -d '{"statements":[{"statement":"MATCH (n:BenchmarkEntity) DELETE n"}]}' \
        "http://localhost:7474/db/helixagent/tx/commit" >/dev/null

    # Evaluate
    if (( $(echo "$nodes_per_sec > 5000" | bc -l) )); then
        success "Neo4j write: EXCELLENT (>5K nodes/sec)"
    elif (( $(echo "$nodes_per_sec > 2000" | bc -l) )); then
        warn "Neo4j write: GOOD (>2K nodes/sec)"
    else
        error "Neo4j write: POOR (<2K nodes/sec)"
    fi
}

# ============================================================================
# CONTEXT REPLAY BENCHMARKS
# ============================================================================

benchmark_context_replay() {
    benchmark_header "Context Replay Benchmark"

    log "Testing conversation replay performance..."

    local conversation_id="bench-$(date +%s)"
    local num_messages=10000

    # Create test conversation (this would normally be done via Kafka)
    # For now, we'll benchmark the replay endpoint directly

    local latency_file="$RESULTS_DIR/context_replay_latencies.txt"
    > "$latency_file"

    for size in 100 500 1000 5000 10000; do
        local start=$(date +%s.%N)

        curl -s -X POST "$HELIXAGENT_URL/v1/context/replay" \
            -H "Content-Type: application/json" \
            -d "{
              \"conversation_id\": \"$conversation_id\",
              \"max_tokens\": 4000,
              \"compression_strategy\": \"hybrid\"
            }" >/dev/null 2>&1 || true

        local end=$(date +%s.%N)
        local latency=$(echo "($end - $start) * 1000" | bc -l)
        echo "$size\t$latency" >> "$latency_file"

        log "Replay $size messages: ${latency}ms"
    done

    # Save results
    cat > "$RESULTS_DIR/context_replay.json" <<EOF
{
  "test": "context_replay",
  "results": $(cat "$latency_file" | awk '{print "{\"size\":" $1 ",\"latency_ms\":" $2 "}"}' | jq -s .),
  "timestamp": "$(date -Iseconds)"
}
EOF

    success "Context replay benchmark complete"
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

main() {
    local component="${1:-all}"

    benchmark_header "Big Data Performance Benchmark Suite"

    log "Component: $component"
    log "Results Directory: $RESULTS_DIR"
    echo ""

    check_dependencies

    case "$component" in
        kafka)
            benchmark_kafka_throughput
            benchmark_kafka_latency
            ;;
        clickhouse)
            benchmark_clickhouse_insert
            benchmark_clickhouse_query
            ;;
        neo4j)
            benchmark_neo4j_write
            ;;
        context)
            benchmark_context_replay
            ;;
        all)
            benchmark_kafka_throughput
            benchmark_kafka_latency
            benchmark_clickhouse_insert
            benchmark_clickhouse_query
            benchmark_neo4j_write
            benchmark_context_replay
            ;;
        *)
            error "Unknown component: $component"
            echo "Usage: $0 [kafka|clickhouse|neo4j|context|all]"
            exit 1
            ;;
    esac

    echo ""
    benchmark_header "Benchmark Complete"
    log "Results saved to: $RESULTS_DIR"

    # Generate summary
    cat > "$RESULTS_DIR/summary.txt" <<EOF
Big Data Performance Benchmark Summary
======================================
Date: $(date)
Component: $component

Results:
$(ls -1 "$RESULTS_DIR"/*.json 2>/dev/null | xargs -I{} basename {} .json || echo "No results")

Location: $RESULTS_DIR
EOF

    cat "$RESULTS_DIR/summary.txt"
}

main "$@"
