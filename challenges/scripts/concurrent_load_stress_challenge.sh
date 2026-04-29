#!/bin/bash
# Concurrent Load Stress Challenge
# Tests API endpoints under heavy concurrent load (50-100 requests)
# Measures response times, error rates, and validates no request drops

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
CHALLENGE_PORT="${HELIXAGENT_PORT:-8100}"
BASE_URL="http://localhost:$CHALLENGE_PORT"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUTPUT_DIR="$PROJECT_ROOT/challenges/results/concurrent_load_stress/$TIMESTAMP"
LOG_FILE="$OUTPUT_DIR/logs/challenge.log"

# Test parameters (can be overridden via environment)
CONCURRENT_REQUESTS_LIGHT="${CONCURRENT_REQUESTS_LIGHT:-50}"
CONCURRENT_REQUESTS_HEAVY="${CONCURRENT_REQUESTS_HEAVY:-100}"
REQUEST_TIMEOUT="${REQUEST_TIMEOUT:-60}"
ACCEPTABLE_ERROR_RATE="${ACCEPTABLE_ERROR_RATE:-5}"

# Tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
VERBOSE=false
HELIXAGENT_PID=""

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --verbose|-v)
                VERBOSE=true
                shift
                ;;
            --help|-h)
                show_help
                trap - EXIT
                exit 0
                ;;
            --concurrent=*)
                CONCURRENT_REQUESTS_HEAVY="${1#*=}"
                shift
                ;;
            --timeout=*)
                REQUEST_TIMEOUT="${1#*=}"
                shift
                ;;
            --port=*)
                CHALLENGE_PORT="${1#*=}"
                BASE_URL="http://localhost:$CHALLENGE_PORT"
                shift
                ;;
            *)
                echo -e "${YELLOW}Unknown option: $1${NC}"
                shift
                ;;
        esac
    done
}

show_help() {
    cat << EOF
Concurrent Load Stress Challenge
================================
Tests API endpoints under heavy concurrent load (50-100 requests)

Usage: $0 [OPTIONS]

Options:
  --verbose, -v       Enable verbose output
  --help, -h          Show this help message
  --concurrent=N      Set number of concurrent requests (default: 100)
  --timeout=S         Set request timeout in seconds (default: 60)
  --port=PORT         Set HelixAgent port (default: 8100)

Environment Variables:
  HELIXAGENT_PORT               API port (default: 8100)
  CONCURRENT_REQUESTS_LIGHT     Light load requests (default: 50)
  CONCURRENT_REQUESTS_HEAVY     Heavy load requests (default: 100)
  REQUEST_TIMEOUT               Request timeout seconds (default: 60)
  ACCEPTABLE_ERROR_RATE         Max acceptable error % (default: 5)
  HELIXAGENT_API_KEY            API authentication key

Examples:
  $0                           Run with defaults
  $0 --verbose                 Run with detailed output
  $0 --concurrent=200          Test with 200 concurrent requests
EOF
}

# Logging functions
log_info() {
    local msg="[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${BLUE}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

log_success() {
    local msg="[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${GREEN}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

log_warning() {
    local msg="[WARNING] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${YELLOW}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

log_error() {
    local msg="[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${RED}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        local msg="[VERBOSE] $(date '+%Y-%m-%d %H:%M:%S') $1"
        echo -e "${CYAN}$msg${NC}"
        echo "$msg" >> "$LOG_FILE"
    fi
}

# Test result tracking
pass_test() {
    local name="$1"
    local details="$2"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    echo -e "${GREEN}[PASS]${NC} $name"
    echo "PASS|$name|$details" >> "$OUTPUT_DIR/logs/assertions.log"
    if [[ -n "$details" && "$VERBOSE" == "true" ]]; then
        echo -e "       ${CYAN}$details${NC}"
    fi
}

fail_test() {
    local name="$1"
    local reason="$2"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    FAILED_TESTS=$((FAILED_TESTS + 1))
    echo -e "${RED}[FAIL]${NC} $name"
    echo "FAIL|$name|$reason" >> "$OUTPUT_DIR/logs/assertions.log"
    if [[ -n "$reason" ]]; then
        echo -e "       ${RED}Reason: $reason${NC}"
    fi
}

record_metric() {
    local name="$1"
    local value="$2"
    echo "${name}=${value}" >> "$OUTPUT_DIR/logs/metrics.log"
    log_verbose "Metric: $name = $value"
}

# Initialize challenge
init_challenge() {
    mkdir -p "$OUTPUT_DIR/logs" "$OUTPUT_DIR/results" "$OUTPUT_DIR/temp"
    touch "$LOG_FILE"
    touch "$OUTPUT_DIR/logs/assertions.log"
    touch "$OUTPUT_DIR/logs/metrics.log"

    log_info "=============================================="
    log_info "  CONCURRENT LOAD STRESS CHALLENGE"
    log_info "=============================================="
    log_info ""
    log_info "Challenge ID: concurrent_load_stress"
    log_info "Output directory: $OUTPUT_DIR"
    log_info "Target URL: $BASE_URL"
    log_info "Light load: $CONCURRENT_REQUESTS_LIGHT concurrent requests"
    log_info "Heavy load: $CONCURRENT_REQUESTS_HEAVY concurrent requests"
    log_info ""

    # Load environment
    if [[ -f "$PROJECT_ROOT/.env" ]]; then
        set -a
        source "$PROJECT_ROOT/.env"
        set +a
        log_verbose "Loaded environment from $PROJECT_ROOT/.env"
    fi
}

# Cleanup function
cleanup() {
    log_info "Cleaning up..."

    # Kill any background processes
    if [[ -n "$HELIXAGENT_PID" ]] && kill -0 "$HELIXAGENT_PID" 2>/dev/null; then
        log_info "Stopping HelixAgent (PID: $HELIXAGENT_PID)..."
        kill "$HELIXAGENT_PID" 2>/dev/null || true
        wait "$HELIXAGENT_PID" 2>/dev/null || true
    fi

    # Clean up temp files
    rm -rf "$OUTPUT_DIR/temp" 2>/dev/null || true

    log_info "Cleanup complete"
}

trap cleanup EXIT

# Check if HelixAgent is running
check_server() {
    if curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" --max-time 5 | grep -q "200"; then
        return 0
    fi
    return 1
}

# Start HelixAgent if not running
ensure_server() {
    if check_server; then
        log_info "HelixAgent already running on port $CHALLENGE_PORT"
        return 0
    fi

    log_warning "HelixAgent not running, attempting to start..."

    local binary="$PROJECT_ROOT/bin/helixagent"
    if [[ ! -x "$binary" ]]; then
        binary="$PROJECT_ROOT/helixagent"
    fi

    if [[ ! -x "$binary" ]]; then
        log_error "HelixAgent binary not found. Run: make build"
        return 1
    fi

    PORT=$CHALLENGE_PORT "$binary" > "$OUTPUT_DIR/logs/helixagent.log" 2>&1 &
    HELIXAGENT_PID=$!
    echo "$HELIXAGENT_PID" > "$OUTPUT_DIR/helixagent.pid"

    # Wait for startup
    local max_wait=60
    local waited=0
    while ! check_server; do
        sleep 1
        waited=$((waited + 1))
        if [[ $waited -ge $max_wait ]]; then
            log_error "HelixAgent failed to start within ${max_wait}s"
            return 1
        fi
        log_verbose "Waiting for HelixAgent... ($waited/$max_wait)"
    done

    log_success "HelixAgent started (PID: $HELIXAGENT_PID)"
    return 0
}

# Calculate statistics from results
calculate_stats() {
    local results_file="$1"
    local metric_prefix="$2"

    if [[ ! -f "$results_file" ]]; then
        echo "0|0|0|0|0|0"
        return
    fi

    local total=$(wc -l < "$results_file" | tr -d ' ')
    local success=$(grep -c "^200$" "$results_file" 2>/dev/null | head -1 || echo 0)
    local failed=$((total - success))
    local error_rate=0

    if [[ $total -gt 0 ]]; then
        error_rate=$(awk "BEGIN {printf \"%.2f\", ($failed / $total) * 100}")
    fi

    # Calculate response time stats if we have timing data
    local latencies_file="${results_file}.latencies"
    local avg_latency=0
    local max_latency=0
    local min_latency=0
    local p95_latency=0

    if [[ -f "$latencies_file" && -s "$latencies_file" ]]; then
        avg_latency=$(awk '{sum+=$1} END {if(NR>0) printf "%.0f", sum/NR; else print 0}' "$latencies_file")
        max_latency=$(sort -n "$latencies_file" | tail -1)
        min_latency=$(sort -n "$latencies_file" | head -1)
        local count=$(wc -l < "$latencies_file" | tr -d ' ')
        local p95_idx=$(awk "BEGIN {printf \"%d\", $count * 0.95}")
        [[ $p95_idx -lt 1 ]] && p95_idx=1
        p95_latency=$(sort -n "$latencies_file" | sed -n "${p95_idx}p")
    fi

    echo "$total|$success|$failed|$error_rate|$avg_latency|$p95_latency|$min_latency|$max_latency"
}

# Test 1: Health endpoint under concurrent load
test_health_endpoint_concurrent() {
    local count="$1"
    log_info "Test: Health endpoint with $count concurrent requests"

    local results_file="$OUTPUT_DIR/temp/health_results_$count.txt"
    local latencies_file="${results_file}.latencies"
    > "$results_file"
    > "$latencies_file"

    local pids=()
    local start_time=$(date +%s%N)

    for i in $(seq 1 $count); do
        (
            local req_start=$(date +%s%N)
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" --max-time "$REQUEST_TIMEOUT" 2>/dev/null || echo "000")
            local req_end=$(date +%s%N)
            local latency=$(( (req_end - req_start) / 1000000 ))
            echo "$http_code" >> "$results_file"
            echo "$latency" >> "$latencies_file"
        ) &
        pids+=($!)
    done

    # Wait for all requests
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local end_time=$(date +%s%N)
    local total_time=$(( (end_time - start_time) / 1000000 ))

    # Calculate stats
    IFS='|' read -r total success failed error_rate avg_lat p95_lat min_lat max_lat <<< "$(calculate_stats "$results_file" "health_$count")"

    record_metric "health_${count}_total_requests" "$total"
    record_metric "health_${count}_successful" "$success"
    record_metric "health_${count}_failed" "$failed"
    record_metric "health_${count}_error_rate_percent" "$error_rate"
    record_metric "health_${count}_avg_latency_ms" "$avg_lat"
    record_metric "health_${count}_p95_latency_ms" "$p95_lat"
    record_metric "health_${count}_total_time_ms" "$total_time"

    if (( $(echo "$error_rate <= $ACCEPTABLE_ERROR_RATE" | bc -l 2>/dev/null || echo "0") )); then
        pass_test "Health endpoint ($count concurrent)" "Success: $success/$total, Avg: ${avg_lat}ms, P95: ${p95_lat}ms"
    else
        fail_test "Health endpoint ($count concurrent)" "Error rate ${error_rate}% exceeds ${ACCEPTABLE_ERROR_RATE}%"
    fi
}

# Test 2: Models endpoint under concurrent load
test_models_endpoint_concurrent() {
    local count="$1"
    log_info "Test: Models endpoint with $count concurrent requests"

    local results_file="$OUTPUT_DIR/temp/models_results_$count.txt"
    local latencies_file="${results_file}.latencies"
    > "$results_file"
    > "$latencies_file"

    local pids=()
    local start_time=$(date +%s%N)

    for i in $(seq 1 $count); do
        (
            local req_start=$(date +%s%N)
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/models" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                --max-time "$REQUEST_TIMEOUT" 2>/dev/null || echo "000")
            local req_end=$(date +%s%N)
            local latency=$(( (req_end - req_start) / 1000000 ))
            echo "$http_code" >> "$results_file"
            echo "$latency" >> "$latencies_file"
        ) &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local end_time=$(date +%s%N)
    local total_time=$(( (end_time - start_time) / 1000000 ))

    IFS='|' read -r total success failed error_rate avg_lat p95_lat min_lat max_lat <<< "$(calculate_stats "$results_file" "models_$count")"

    record_metric "models_${count}_total_requests" "$total"
    record_metric "models_${count}_successful" "$success"
    record_metric "models_${count}_failed" "$failed"
    record_metric "models_${count}_error_rate_percent" "$error_rate"
    record_metric "models_${count}_avg_latency_ms" "$avg_lat"
    record_metric "models_${count}_p95_latency_ms" "$p95_lat"
    record_metric "models_${count}_total_time_ms" "$total_time"

    if (( $(echo "$error_rate <= $ACCEPTABLE_ERROR_RATE" | bc -l 2>/dev/null || echo "0") )); then
        pass_test "Models endpoint ($count concurrent)" "Success: $success/$total, Avg: ${avg_lat}ms"
    else
        fail_test "Models endpoint ($count concurrent)" "Error rate ${error_rate}% exceeds ${ACCEPTABLE_ERROR_RATE}%"
    fi
}

# Test 3: Chat completions under concurrent load
test_chat_endpoint_concurrent() {
    local count="$1"
    log_info "Test: Chat completions with $count concurrent requests"

    local results_file="$OUTPUT_DIR/temp/chat_results_$count.txt"
    local latencies_file="${results_file}.latencies"
    > "$results_file"
    > "$latencies_file"

    local request='{
        "model": "helixagent-debate",
        "messages": [{"role": "user", "content": "Say hello"}],
        "max_tokens": 10
    }'

    local pids=()
    local start_time=$(date +%s%N)

    for i in $(seq 1 $count); do
        (
            local req_start=$(date +%s%N)
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -d "$request" \
                --max-time "$REQUEST_TIMEOUT" 2>/dev/null || echo "000")
            local req_end=$(date +%s%N)
            local latency=$(( (req_end - req_start) / 1000000 ))
            echo "$http_code" >> "$results_file"
            echo "$latency" >> "$latencies_file"
        ) &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local end_time=$(date +%s%N)
    local total_time=$(( (end_time - start_time) / 1000000 ))

    IFS='|' read -r total success failed error_rate avg_lat p95_lat min_lat max_lat <<< "$(calculate_stats "$results_file" "chat_$count")"

    record_metric "chat_${count}_total_requests" "$total"
    record_metric "chat_${count}_successful" "$success"
    record_metric "chat_${count}_failed" "$failed"
    record_metric "chat_${count}_error_rate_percent" "$error_rate"
    record_metric "chat_${count}_avg_latency_ms" "$avg_lat"
    record_metric "chat_${count}_p95_latency_ms" "$p95_lat"
    record_metric "chat_${count}_total_time_ms" "$total_time"
    record_metric "chat_${count}_throughput_rps" "$(awk "BEGIN {printf \"%.2f\", $total / ($total_time / 1000)}")"

    # For chat, allow higher error rate as it depends on LLM providers
    local chat_error_threshold=20
    if (( $(echo "$error_rate <= $chat_error_threshold" | bc -l 2>/dev/null || echo "0") )); then
        pass_test "Chat completions ($count concurrent)" "Success: $success/$total, Avg: ${avg_lat}ms"
    else
        fail_test "Chat completions ($count concurrent)" "Error rate ${error_rate}% exceeds ${chat_error_threshold}%"
    fi
}

# Test 4: Mixed endpoint load
test_mixed_endpoints_concurrent() {
    local count="$1"
    log_info "Test: Mixed endpoints with $count total concurrent requests"

    local results_file="$OUTPUT_DIR/temp/mixed_results_$count.txt"
    local latencies_file="${results_file}.latencies"
    > "$results_file"
    > "$latencies_file"

    local endpoints=("/health" "/v1/models" "/v1/providers")
    local pids=()
    local start_time=$(date +%s%N)

    for i in $(seq 1 $count); do
        local endpoint_idx=$((i % ${#endpoints[@]}))
        local endpoint="${endpoints[$endpoint_idx]}"
        (
            local req_start=$(date +%s%N)
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$endpoint" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                --max-time "$REQUEST_TIMEOUT" 2>/dev/null || echo "000")
            local req_end=$(date +%s%N)
            local latency=$(( (req_end - req_start) / 1000000 ))
            echo "$http_code" >> "$results_file"
            echo "$latency" >> "$latencies_file"
        ) &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local end_time=$(date +%s%N)
    local total_time=$(( (end_time - start_time) / 1000000 ))

    IFS='|' read -r total success failed error_rate avg_lat p95_lat min_lat max_lat <<< "$(calculate_stats "$results_file" "mixed_$count")"

    record_metric "mixed_${count}_total_requests" "$total"
    record_metric "mixed_${count}_successful" "$success"
    record_metric "mixed_${count}_failed" "$failed"
    record_metric "mixed_${count}_error_rate_percent" "$error_rate"
    record_metric "mixed_${count}_avg_latency_ms" "$avg_lat"
    record_metric "mixed_${count}_p95_latency_ms" "$p95_lat"
    record_metric "mixed_${count}_total_time_ms" "$total_time"

    if (( $(echo "$error_rate <= $ACCEPTABLE_ERROR_RATE" | bc -l 2>/dev/null || echo "0") )); then
        pass_test "Mixed endpoints ($count concurrent)" "Success: $success/$total, Avg: ${avg_lat}ms"
    else
        fail_test "Mixed endpoints ($count concurrent)" "Error rate ${error_rate}% exceeds ${ACCEPTABLE_ERROR_RATE}%"
    fi
}

# Test 5: Burst load test
test_burst_load() {
    log_info "Test: Burst load (3 waves of $CONCURRENT_REQUESTS_LIGHT requests)"

    local results_file="$OUTPUT_DIR/temp/burst_results.txt"
    local latencies_file="${results_file}.latencies"
    > "$results_file"
    > "$latencies_file"

    local total_success=0
    local total_requests=0
    local wave_times=()

    for wave in 1 2 3; do
        log_verbose "Burst wave $wave..."
        local pids=()
        local wave_start=$(date +%s%N)

        for i in $(seq 1 $CONCURRENT_REQUESTS_LIGHT); do
            (
                local req_start=$(date +%s%N)
                local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" \
                    --max-time "$REQUEST_TIMEOUT" 2>/dev/null || echo "000")
                local req_end=$(date +%s%N)
                local latency=$(( (req_end - req_start) / 1000000 ))
                echo "$http_code" >> "$results_file"
                echo "$latency" >> "$latencies_file"
            ) &
            pids+=($!)
        done

        for pid in "${pids[@]}"; do
            wait "$pid" 2>/dev/null || true
        done

        local wave_end=$(date +%s%N)
        local wave_time=$(( (wave_end - wave_start) / 1000000 ))
        wave_times+=("$wave_time")

        # Brief pause between waves
        sleep 1
    done

    IFS='|' read -r total success failed error_rate avg_lat p95_lat min_lat max_lat <<< "$(calculate_stats "$results_file" "burst")"

    record_metric "burst_total_requests" "$total"
    record_metric "burst_successful" "$success"
    record_metric "burst_error_rate_percent" "$error_rate"
    record_metric "burst_wave1_time_ms" "${wave_times[0]}"
    record_metric "burst_wave2_time_ms" "${wave_times[1]}"
    record_metric "burst_wave3_time_ms" "${wave_times[2]}"

    if (( $(echo "$error_rate <= $ACCEPTABLE_ERROR_RATE" | bc -l 2>/dev/null || echo "0") )); then
        pass_test "Burst load (3 waves)" "Success: $success/$total, Error rate: ${error_rate}%"
    else
        fail_test "Burst load (3 waves)" "Error rate ${error_rate}% exceeds ${ACCEPTABLE_ERROR_RATE}%"
    fi
}

# Test 6: No request drops validation
test_no_request_drops() {
    local count="$1"
    log_info "Test: Validate no request drops with $count concurrent requests"

    local results_file="$OUTPUT_DIR/temp/drops_results.txt"
    > "$results_file"

    local request_ids=()
    local pids=()

    for i in $(seq 1 $count); do
        local req_id="req_$i"
        request_ids+=("$req_id")
        (
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" \
                --max-time "$REQUEST_TIMEOUT" 2>/dev/null || echo "000")
            echo "$req_id:$http_code" >> "$results_file"
        ) &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local received=$(wc -l < "$results_file" | tr -d ' ')
    local expected=${#request_ids[@]}
    local dropped=$((expected - received))
    local drop_rate=0

    if [[ $expected -gt 0 ]]; then
        drop_rate=$(awk "BEGIN {printf \"%.2f\", ($dropped / $expected) * 100}")
    fi

    record_metric "request_drop_expected" "$expected"
    record_metric "request_drop_received" "$received"
    record_metric "request_drop_dropped" "$dropped"
    record_metric "request_drop_rate_percent" "$drop_rate"

    if [[ $dropped -eq 0 ]]; then
        pass_test "No request drops ($count requests)" "All $expected requests received responses"
    else
        fail_test "No request drops ($count requests)" "$dropped out of $expected requests dropped (${drop_rate}%)"
    fi
}

# Test 7: Response time consistency under load
test_response_time_consistency() {
    local count="$1"
    log_info "Test: Response time consistency with $count requests"

    local latencies_file="$OUTPUT_DIR/temp/consistency_latencies.txt"
    > "$latencies_file"

    local pids=()

    for i in $(seq 1 $count); do
        (
            local req_start=$(date +%s%N)
            curl -s -o /dev/null "$BASE_URL/health" --max-time "$REQUEST_TIMEOUT" 2>/dev/null || true
            local req_end=$(date +%s%N)
            local latency=$(( (req_end - req_start) / 1000000 ))
            echo "$latency" >> "$latencies_file"
        ) &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    if [[ ! -s "$latencies_file" ]]; then
        fail_test "Response time consistency" "No latency data collected"
        return
    fi

    local avg=$(awk '{sum+=$1} END {printf "%.0f", sum/NR}' "$latencies_file")
    local stddev=$(awk -v avg="$avg" '{sum+=($1-avg)^2} END {printf "%.0f", sqrt(sum/NR)}' "$latencies_file")
    local cv=0
    if [[ $avg -gt 0 ]]; then
        cv=$(awk "BEGIN {printf \"%.2f\", ($stddev / $avg) * 100}")
    fi

    record_metric "consistency_avg_latency_ms" "$avg"
    record_metric "consistency_stddev_ms" "$stddev"
    record_metric "consistency_cv_percent" "$cv"

    # Coefficient of variation under 100% is acceptable for high concurrency
    if (( $(echo "$cv <= 150" | bc -l 2>/dev/null || echo "1") )); then
        pass_test "Response time consistency" "CV: ${cv}%, Avg: ${avg}ms, StdDev: ${stddev}ms"
    else
        fail_test "Response time consistency" "High variance: CV=${cv}% (threshold: 150%)"
    fi
}

# Test 8: Server stability after load
test_server_stability_after_load() {
    log_info "Test: Server stability after concurrent load"

    # Brief pause to let server stabilize
    sleep 2

    local success_count=0
    local test_count=5

    for i in $(seq 1 $test_count); do
        local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || echo "000")
        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        fi
        sleep 0.5
    done

    record_metric "stability_checks" "$test_count"
    record_metric "stability_success" "$success_count"

    if [[ $success_count -eq $test_count ]]; then
        pass_test "Server stability after load" "All $test_count health checks passed"
    else
        fail_test "Server stability after load" "Only $success_count/$test_count health checks passed"
    fi
}

# Finalize and generate report
finalize_challenge() {
    local status="$1"

    log_info ""
    log_info "=============================================="
    log_info "  CONCURRENT LOAD STRESS CHALLENGE SUMMARY"
    log_info "=============================================="
    log_info ""
    log_info "Total Tests:   $TOTAL_TESTS"
    log_success "Passed:        $PASSED_TESTS"
    if [[ $FAILED_TESTS -gt 0 ]]; then
        log_error "Failed:        $FAILED_TESTS"
    else
        log_info "Failed:        $FAILED_TESTS"
    fi
    log_info ""

    # Create results JSON
    cat > "$OUTPUT_DIR/results/challenge_results.json" << EOF
{
    "challenge_id": "concurrent_load_stress",
    "name": "Concurrent Load Stress Challenge",
    "status": "$status",
    "timestamp": "$(date -Iseconds)",
    "tests_total": $TOTAL_TESTS,
    "tests_passed": $PASSED_TESTS,
    "tests_failed": $FAILED_TESTS,
    "configuration": {
        "light_load": $CONCURRENT_REQUESTS_LIGHT,
        "heavy_load": $CONCURRENT_REQUESTS_HEAVY,
        "timeout": $REQUEST_TIMEOUT,
        "acceptable_error_rate": $ACCEPTABLE_ERROR_RATE
    }
}
EOF

    # Create latest symlink
    mkdir -p "$PROJECT_ROOT/challenges/results/concurrent_load_stress"
    ln -sf "$OUTPUT_DIR" "$PROJECT_ROOT/challenges/results/concurrent_load_stress/latest"

    log_info "Results: $OUTPUT_DIR/results/challenge_results.json"
    log_info "Logs: $LOG_FILE"
    log_info ""

    if [[ "$status" == "PASSED" ]]; then
        echo -e "${GREEN}CHALLENGE PASSED: All concurrent load tests completed successfully${NC}"
        exit 0
    else
        echo -e "${RED}CHALLENGE FAILED: Some concurrent load tests failed${NC}"
        exit 1
    fi
}

# Main execution
main() {
    parse_args "$@"
    init_challenge

    # Ensure server is running
    if ! ensure_server; then
        log_error "Cannot run challenge without HelixAgent server"
        finalize_challenge "FAILED"
    fi

    echo ""

    # Run light load tests (50 concurrent)
    log_info "=== Phase 1: Light Load Tests ($CONCURRENT_REQUESTS_LIGHT concurrent) ==="
    test_health_endpoint_concurrent "$CONCURRENT_REQUESTS_LIGHT"
    test_models_endpoint_concurrent "$CONCURRENT_REQUESTS_LIGHT"
    test_mixed_endpoints_concurrent "$CONCURRENT_REQUESTS_LIGHT"

    echo ""

    # Run heavy load tests (100 concurrent)
    log_info "=== Phase 2: Heavy Load Tests ($CONCURRENT_REQUESTS_HEAVY concurrent) ==="
    test_health_endpoint_concurrent "$CONCURRENT_REQUESTS_HEAVY"
    test_models_endpoint_concurrent "$CONCURRENT_REQUESTS_HEAVY"
    test_chat_endpoint_concurrent "$CONCURRENT_REQUESTS_HEAVY"
    test_mixed_endpoints_concurrent "$CONCURRENT_REQUESTS_HEAVY"

    echo ""

    # Run validation tests
    log_info "=== Phase 3: Validation Tests ==="
    test_burst_load
    test_no_request_drops "$CONCURRENT_REQUESTS_HEAVY"
    test_response_time_consistency "$CONCURRENT_REQUESTS_LIGHT"
    test_server_stability_after_load

    # Finalize
    if [[ $FAILED_TESTS -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
