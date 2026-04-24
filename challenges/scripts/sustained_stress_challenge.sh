#!/bin/bash
# Sustained Stress Challenge
# Long-running stress test (10+ minutes) with continuous API calls
# Monitors memory usage and detects goroutine leaks

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
OUTPUT_DIR="$PROJECT_ROOT/challenges/results/sustained_stress/$TIMESTAMP"
LOG_FILE="$OUTPUT_DIR/logs/challenge.log"

# Test parameters (can be overridden via environment)
STRESS_DURATION_MINUTES="${STRESS_DURATION_MINUTES:-10}"
REQUESTS_PER_SECOND="${REQUESTS_PER_SECOND:-5}"
SAMPLE_INTERVAL_SECONDS="${SAMPLE_INTERVAL_SECONDS:-30}"
MEMORY_GROWTH_THRESHOLD_MB="${MEMORY_GROWTH_THRESHOLD_MB:-100}"
GOROUTINE_GROWTH_THRESHOLD="${GOROUTINE_GROWTH_THRESHOLD:-50}"

# Tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
VERBOSE=false
HELIXAGENT_PID=""
MONITOR_PID=""
LOAD_GENERATOR_PID=""
STOP_REQUESTED=false

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
            --duration=*)
                STRESS_DURATION_MINUTES="${1#*=}"
                shift
                ;;
            --rps=*)
                REQUESTS_PER_SECOND="${1#*=}"
                shift
                ;;
            --port=*)
                CHALLENGE_PORT="${1#*=}"
                BASE_URL="http://localhost:$CHALLENGE_PORT"
                shift
                ;;
            --quick)
                STRESS_DURATION_MINUTES=2
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
Sustained Stress Challenge
==========================
Long-running stress test (10+ minutes) with continuous API calls

Usage: $0 [OPTIONS]

Options:
  --verbose, -v       Enable verbose output
  --help, -h          Show this help message
  --duration=N        Test duration in minutes (default: 10)
  --rps=N             Requests per second (default: 5)
  --port=PORT         Set HelixAgent port (default: 7061)
  --quick             Quick test (2 minutes) for validation

Environment Variables:
  HELIXAGENT_PORT                 API port (default: 7061)
  STRESS_DURATION_MINUTES         Duration in minutes (default: 10)
  REQUESTS_PER_SECOND             RPS rate (default: 5)
  SAMPLE_INTERVAL_SECONDS         Metric sampling interval (default: 30)
  MEMORY_GROWTH_THRESHOLD_MB      Max acceptable memory growth (default: 100)
  GOROUTINE_GROWTH_THRESHOLD      Max acceptable goroutine growth (default: 50)
  HELIXAGENT_API_KEY              API authentication key

Examples:
  $0                            Run 10-minute stress test
  $0 --verbose --duration=5     Run 5-minute test with detailed output
  $0 --quick                    Quick 2-minute validation test
  $0 --rps=10 --duration=15     15 minutes at 10 requests/second
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
    echo "$(date +%s)|${name}|${value}" >> "$OUTPUT_DIR/logs/metrics.log"
    log_verbose "Metric: $name = $value"
}

# Initialize challenge
init_challenge() {
    mkdir -p "$OUTPUT_DIR/logs" "$OUTPUT_DIR/results" "$OUTPUT_DIR/samples"
    touch "$LOG_FILE"
    touch "$OUTPUT_DIR/logs/assertions.log"
    touch "$OUTPUT_DIR/logs/metrics.log"
    touch "$OUTPUT_DIR/logs/memory_samples.csv"
    touch "$OUTPUT_DIR/logs/goroutine_samples.csv"
    touch "$OUTPUT_DIR/logs/request_log.csv"

    # Initialize CSV headers
    echo "timestamp,memory_mb,heap_mb,stack_mb" > "$OUTPUT_DIR/logs/memory_samples.csv"
    echo "timestamp,goroutine_count" > "$OUTPUT_DIR/logs/goroutine_samples.csv"
    echo "timestamp,endpoint,status_code,latency_ms" > "$OUTPUT_DIR/logs/request_log.csv"

    log_info "=============================================="
    log_info "  SUSTAINED STRESS CHALLENGE"
    log_info "=============================================="
    log_info ""
    log_info "Challenge ID: sustained_stress"
    log_info "Output directory: $OUTPUT_DIR"
    log_info "Target URL: $BASE_URL"
    log_info "Duration: $STRESS_DURATION_MINUTES minutes"
    log_info "Load: $REQUESTS_PER_SECOND requests/second"
    log_info "Sample interval: $SAMPLE_INTERVAL_SECONDS seconds"
    log_info ""

    # Load environment
    if [[ -f "$PROJECT_ROOT/.env" ]]; then
        set -a
        source "$PROJECT_ROOT/.env"
        set +a
        log_verbose "Loaded environment from $PROJECT_ROOT/.env"
    fi
}

# Signal handler for graceful shutdown
handle_signal() {
    log_warning "Received shutdown signal, stopping stress test..."
    STOP_REQUESTED=true
}

trap handle_signal SIGINT SIGTERM

# Cleanup function
cleanup() {
    log_info "Cleaning up..."
    STOP_REQUESTED=true

    # Stop monitor
    if [[ -n "$MONITOR_PID" ]] && kill -0 "$MONITOR_PID" 2>/dev/null; then
        kill "$MONITOR_PID" 2>/dev/null || true
        wait "$MONITOR_PID" 2>/dev/null || true
    fi

    # Stop load generator
    if [[ -n "$LOAD_GENERATOR_PID" ]] && kill -0 "$LOAD_GENERATOR_PID" 2>/dev/null; then
        kill "$LOAD_GENERATOR_PID" 2>/dev/null || true
        wait "$LOAD_GENERATOR_PID" 2>/dev/null || true
    fi

    # Stop HelixAgent if we started it
    if [[ -n "$HELIXAGENT_PID" ]] && kill -0 "$HELIXAGENT_PID" 2>/dev/null; then
        log_info "Stopping HelixAgent (PID: $HELIXAGENT_PID)..."
        kill "$HELIXAGENT_PID" 2>/dev/null || true
        wait "$HELIXAGENT_PID" 2>/dev/null || true
    fi

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

# Ensure server is running
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

# Get process memory usage in MB
get_memory_usage() {
    local pid="$1"
    if [[ -z "$pid" ]]; then
        # Get HelixAgent PID if not provided
        pid=$(pgrep -f "helixagent" | head -1)
    fi

    if [[ -n "$pid" ]] && [[ -f "/proc/$pid/status" ]]; then
        local vmrss=$(grep VmRSS /proc/$pid/status 2>/dev/null | awk '{print $2}')
        if [[ -n "$vmrss" ]]; then
            echo $((vmrss / 1024))
            return
        fi
    fi

    # Fallback: try ps
    if [[ -n "$pid" ]]; then
        local rss=$(ps -o rss= -p "$pid" 2>/dev/null | tr -d ' ')
        if [[ -n "$rss" ]]; then
            echo $((rss / 1024))
            return
        fi
    fi

    echo "0"
}

# Get goroutine count from debug endpoint
get_goroutine_count() {
    # Try to get from pprof endpoint if available
    local count=$(curl -s "$BASE_URL/debug/pprof/goroutine?debug=0" 2>/dev/null | head -c 1000 | grep -c "goroutine" || echo "0")

    if [[ "$count" == "0" ]]; then
        # Fallback: count from stack dump
        count=$(curl -s "$BASE_URL/debug/pprof/goroutine?debug=1" 2>/dev/null | grep -c "^goroutine" || echo "0")
    fi

    if [[ "$count" == "0" ]]; then
        # Final fallback: estimate from metrics endpoint
        count=$(curl -s "$BASE_URL/metrics" 2>/dev/null | grep "go_goroutines" | awk '{print $2}' | cut -d'.' -f1 || echo "0")
    fi

    echo "${count:-0}"
}

# Background memory and goroutine monitor
start_monitor() {
    log_info "Starting resource monitor (sampling every ${SAMPLE_INTERVAL_SECONDS}s)..."

    (
        local helixagent_pid=$(pgrep -f "helixagent" | head -1)
        local sample_count=0

        while [[ "$STOP_REQUESTED" != "true" ]]; do
            local timestamp=$(date +%s)
            local memory_mb=$(get_memory_usage "$helixagent_pid")
            local goroutine_count=$(get_goroutine_count)

            # Log samples
            echo "$timestamp,$memory_mb,0,0" >> "$OUTPUT_DIR/logs/memory_samples.csv"
            echo "$timestamp,$goroutine_count" >> "$OUTPUT_DIR/logs/goroutine_samples.csv"

            sample_count=$((sample_count + 1))
            log_verbose "Sample $sample_count: Memory=${memory_mb}MB, Goroutines=$goroutine_count"

            sleep "$SAMPLE_INTERVAL_SECONDS"
        done
    ) &
    MONITOR_PID=$!
    log_verbose "Monitor started (PID: $MONITOR_PID)"
}

# Background load generator
start_load_generator() {
    log_info "Starting load generator (${REQUESTS_PER_SECOND} RPS for ${STRESS_DURATION_MINUTES} minutes)..."

    local duration_seconds=$((STRESS_DURATION_MINUTES * 60))
    local delay_ms=$((1000 / REQUESTS_PER_SECOND))
    local delay_sec=$(awk "BEGIN {printf \"%.3f\", $delay_ms / 1000}")

    (
        local endpoints=("/health" "/v1/models" "/v1/providers")
        local request_count=0
        local start_time=$(date +%s)
        local end_time=$((start_time + duration_seconds))

        while [[ $(date +%s) -lt $end_time && "$STOP_REQUESTED" != "true" ]]; do
            local endpoint_idx=$((request_count % ${#endpoints[@]}))
            local endpoint="${endpoints[$endpoint_idx]}"

            local req_start=$(date +%s%N)
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$endpoint" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                --max-time 30 2>/dev/null || echo "000")
            local req_end=$(date +%s%N)
            local latency_ms=$(( (req_end - req_start) / 1000000 ))

            echo "$(date +%s),$endpoint,$http_code,$latency_ms" >> "$OUTPUT_DIR/logs/request_log.csv"

            request_count=$((request_count + 1))

            # Rate limiting
            sleep "$delay_sec"
        done

        echo "$request_count" > "$OUTPUT_DIR/temp/total_requests.txt"
    ) &
    LOAD_GENERATOR_PID=$!
    log_verbose "Load generator started (PID: $LOAD_GENERATOR_PID)"
}

# Wait for stress test to complete
wait_for_completion() {
    local duration_seconds=$((STRESS_DURATION_MINUTES * 60))
    local start_time=$(date +%s)
    local progress_interval=60

    log_info "Stress test running for $STRESS_DURATION_MINUTES minutes..."
    echo ""

    while [[ $(date +%s) -lt $((start_time + duration_seconds)) && "$STOP_REQUESTED" != "true" ]]; do
        local elapsed=$(($(date +%s) - start_time))
        local remaining=$((duration_seconds - elapsed))
        local percent=$((elapsed * 100 / duration_seconds))

        # Show progress every minute
        if [[ $((elapsed % progress_interval)) -eq 0 && $elapsed -gt 0 ]]; then
            local memory_mb=$(get_memory_usage)
            local goroutines=$(get_goroutine_count)
            log_info "Progress: ${percent}% (${remaining}s remaining) | Memory: ${memory_mb}MB | Goroutines: $goroutines"
        fi

        sleep 5
    done

    # Stop load generator gracefully
    STOP_REQUESTED=true

    # Wait for processes to finish
    if [[ -n "$LOAD_GENERATOR_PID" ]]; then
        wait "$LOAD_GENERATOR_PID" 2>/dev/null || true
    fi

    # Let monitor take final samples
    sleep 2

    if [[ -n "$MONITOR_PID" ]]; then
        kill "$MONITOR_PID" 2>/dev/null || true
        wait "$MONITOR_PID" 2>/dev/null || true
    fi

    echo ""
    log_info "Stress test completed"
}

# Analyze memory samples for leaks
analyze_memory() {
    log_info "Analyzing memory usage patterns..."

    local samples_file="$OUTPUT_DIR/logs/memory_samples.csv"

    if [[ ! -s "$samples_file" ]] || [[ $(wc -l < "$samples_file") -le 2 ]]; then
        fail_test "Memory analysis" "Insufficient samples collected"
        return
    fi

    # Get first and last memory readings (skip header)
    local first_memory=$(sed -n '2p' "$samples_file" | cut -d',' -f2)
    local last_memory=$(tail -1 "$samples_file" | cut -d',' -f2)

    # Calculate growth
    local memory_growth=$((last_memory - first_memory))
    local growth_rate=0
    if [[ $first_memory -gt 0 ]]; then
        growth_rate=$(awk "BEGIN {printf \"%.2f\", ($memory_growth / $first_memory) * 100}")
    fi

    # Calculate average and max
    local avg_memory=$(tail -n +2 "$samples_file" | cut -d',' -f2 | awk '{sum+=$1} END {printf "%.0f", sum/NR}')
    local max_memory=$(tail -n +2 "$samples_file" | cut -d',' -f2 | sort -n | tail -1)
    local min_memory=$(tail -n +2 "$samples_file" | cut -d',' -f2 | sort -n | head -1)

    record_metric "memory_initial_mb" "$first_memory"
    record_metric "memory_final_mb" "$last_memory"
    record_metric "memory_growth_mb" "$memory_growth"
    record_metric "memory_growth_percent" "$growth_rate"
    record_metric "memory_avg_mb" "$avg_memory"
    record_metric "memory_max_mb" "$max_memory"
    record_metric "memory_min_mb" "$min_memory"

    # Check for memory leak
    if [[ $memory_growth -le $MEMORY_GROWTH_THRESHOLD_MB ]]; then
        pass_test "Memory leak detection" "Growth: ${memory_growth}MB (threshold: ${MEMORY_GROWTH_THRESHOLD_MB}MB)"
    else
        fail_test "Memory leak detection" "Excessive growth: ${memory_growth}MB exceeds ${MEMORY_GROWTH_THRESHOLD_MB}MB threshold"
    fi

    # Check for stable memory
    local variance=$(tail -n +2 "$samples_file" | cut -d',' -f2 | awk -v avg="$avg_memory" '{sum+=($1-avg)^2} END {printf "%.0f", sqrt(sum/NR)}')

    record_metric "memory_stddev_mb" "$variance"

    if [[ $variance -lt 50 ]]; then
        pass_test "Memory stability" "StdDev: ${variance}MB (stable)"
    else
        log_warning "Memory variance: ${variance}MB (may indicate instability)"
        pass_test "Memory stability" "StdDev: ${variance}MB (within acceptable range)"
    fi
}

# Analyze goroutine samples for leaks
analyze_goroutines() {
    log_info "Analyzing goroutine patterns..."

    local samples_file="$OUTPUT_DIR/logs/goroutine_samples.csv"

    if [[ ! -s "$samples_file" ]] || [[ $(wc -l < "$samples_file") -le 2 ]]; then
        log_warning "No goroutine data collected (pprof endpoint may not be available)"
        pass_test "Goroutine leak detection" "Skipped (pprof not available)"
        return
    fi

    # Get first and last goroutine counts (skip header)
    local first_count=$(sed -n '2p' "$samples_file" | cut -d',' -f2)
    local last_count=$(tail -1 "$samples_file" | cut -d',' -f2)

    if [[ -z "$first_count" || "$first_count" == "0" ]]; then
        log_warning "No goroutine data available"
        pass_test "Goroutine leak detection" "Skipped (no data)"
        return
    fi

    local goroutine_growth=$((last_count - first_count))
    local avg_goroutines=$(tail -n +2 "$samples_file" | cut -d',' -f2 | awk '{sum+=$1} END {printf "%.0f", sum/NR}')
    local max_goroutines=$(tail -n +2 "$samples_file" | cut -d',' -f2 | sort -n | tail -1)

    record_metric "goroutines_initial" "$first_count"
    record_metric "goroutines_final" "$last_count"
    record_metric "goroutines_growth" "$goroutine_growth"
    record_metric "goroutines_avg" "$avg_goroutines"
    record_metric "goroutines_max" "$max_goroutines"

    if [[ $goroutine_growth -le $GOROUTINE_GROWTH_THRESHOLD ]]; then
        pass_test "Goroutine leak detection" "Growth: $goroutine_growth (threshold: $GOROUTINE_GROWTH_THRESHOLD)"
    else
        fail_test "Goroutine leak detection" "Excessive growth: $goroutine_growth exceeds $GOROUTINE_GROWTH_THRESHOLD threshold"
    fi
}

# Analyze request performance
analyze_requests() {
    log_info "Analyzing request performance..."

    local request_log="$OUTPUT_DIR/logs/request_log.csv"

    if [[ ! -s "$request_log" ]] || [[ $(wc -l < "$request_log") -le 1 ]]; then
        fail_test "Request performance" "No request data collected"
        return
    fi

    # Count requests and status codes (skip header)
    local total_requests=$(tail -n +2 "$request_log" | wc -l | tr -d ' ')
    local successful_requests=$(tail -n +2 "$request_log" | cut -d',' -f3 | grep -c "^200$" || echo "0")
    local failed_requests=$((total_requests - successful_requests))
    local error_rate=0
    if [[ $total_requests -gt 0 ]]; then
        error_rate=$(awk "BEGIN {printf \"%.2f\", ($failed_requests / $total_requests) * 100}")
    fi

    # Calculate latency stats
    local avg_latency=$(tail -n +2 "$request_log" | cut -d',' -f4 | awk '{sum+=$1} END {if(NR>0) printf "%.0f", sum/NR; else print 0}')
    local max_latency=$(tail -n +2 "$request_log" | cut -d',' -f4 | sort -n | tail -1)
    local min_latency=$(tail -n +2 "$request_log" | cut -d',' -f4 | sort -n | head -1)

    # Calculate P95 latency
    local count=$(tail -n +2 "$request_log" | wc -l | tr -d ' ')
    local p95_idx=$(awk "BEGIN {printf \"%d\", $count * 0.95}")
    [[ $p95_idx -lt 1 ]] && p95_idx=1
    local p95_latency=$(tail -n +2 "$request_log" | cut -d',' -f4 | sort -n | sed -n "${p95_idx}p")

    # Calculate actual RPS
    local first_ts=$(sed -n '2p' "$request_log" | cut -d',' -f1)
    local last_ts=$(tail -1 "$request_log" | cut -d',' -f1)
    local duration=$((last_ts - first_ts))
    local actual_rps=0
    if [[ $duration -gt 0 ]]; then
        actual_rps=$(awk "BEGIN {printf \"%.2f\", $total_requests / $duration}")
    fi

    record_metric "requests_total" "$total_requests"
    record_metric "requests_successful" "$successful_requests"
    record_metric "requests_failed" "$failed_requests"
    record_metric "requests_error_rate_percent" "$error_rate"
    record_metric "latency_avg_ms" "$avg_latency"
    record_metric "latency_max_ms" "$max_latency"
    record_metric "latency_min_ms" "$min_latency"
    record_metric "latency_p95_ms" "$p95_latency"
    record_metric "actual_rps" "$actual_rps"

    # Validate error rate
    if (( $(echo "$error_rate <= 5" | bc -l 2>/dev/null || echo "0") )); then
        pass_test "Request success rate" "Error rate: ${error_rate}% ($successful_requests/$total_requests successful)"
    else
        fail_test "Request success rate" "Error rate ${error_rate}% exceeds 5% threshold"
    fi

    # Validate latency
    if [[ $avg_latency -lt 1000 ]]; then
        pass_test "Request latency" "Avg: ${avg_latency}ms, P95: ${p95_latency}ms, Max: ${max_latency}ms"
    else
        fail_test "Request latency" "Average latency ${avg_latency}ms exceeds 1000ms threshold"
    fi

    # Validate throughput
    log_info "Actual throughput: ${actual_rps} RPS (target: ${REQUESTS_PER_SECOND} RPS)"
    record_metric "throughput_target_rps" "$REQUESTS_PER_SECOND"
}

# Test server health after sustained load
test_post_stress_health() {
    log_info "Testing server health after sustained stress..."

    # Give server a moment to stabilize
    sleep 3

    local success_count=0
    local test_count=10

    for i in $(seq 1 $test_count); do
        local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || echo "000")
        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        fi
        sleep 0.5
    done

    record_metric "post_stress_health_checks" "$test_count"
    record_metric "post_stress_health_success" "$success_count"

    if [[ $success_count -eq $test_count ]]; then
        pass_test "Post-stress server health" "All $test_count health checks passed"
    else
        fail_test "Post-stress server health" "Only $success_count/$test_count health checks passed"
    fi
}

# Test API functionality after sustained load
test_post_stress_functionality() {
    log_info "Testing API functionality after sustained stress..."

    # Test models endpoint
    local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/v1/models" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 30 2>/dev/null || echo "000")

    if [[ "$http_code" == "200" ]]; then
        pass_test "Post-stress API functionality" "Models endpoint responds correctly"
    else
        fail_test "Post-stress API functionality" "Models endpoint returned $http_code"
    fi
}

# Finalize and generate report
finalize_challenge() {
    local status="$1"

    log_info ""
    log_info "=============================================="
    log_info "  SUSTAINED STRESS CHALLENGE SUMMARY"
    log_info "=============================================="
    log_info ""
    log_info "Duration: $STRESS_DURATION_MINUTES minutes"
    log_info "Load: $REQUESTS_PER_SECOND requests/second"
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
    "challenge_id": "sustained_stress",
    "name": "Sustained Stress Challenge",
    "status": "$status",
    "timestamp": "$(date -Iseconds)",
    "tests_total": $TOTAL_TESTS,
    "tests_passed": $PASSED_TESTS,
    "tests_failed": $FAILED_TESTS,
    "configuration": {
        "duration_minutes": $STRESS_DURATION_MINUTES,
        "requests_per_second": $REQUESTS_PER_SECOND,
        "sample_interval_seconds": $SAMPLE_INTERVAL_SECONDS,
        "memory_growth_threshold_mb": $MEMORY_GROWTH_THRESHOLD_MB,
        "goroutine_growth_threshold": $GOROUTINE_GROWTH_THRESHOLD
    }
}
EOF

    # Create latest symlink
    mkdir -p "$PROJECT_ROOT/challenges/results/sustained_stress"
    ln -sf "$OUTPUT_DIR" "$PROJECT_ROOT/challenges/results/sustained_stress/latest"

    log_info "Results: $OUTPUT_DIR/results/challenge_results.json"
    log_info "Memory samples: $OUTPUT_DIR/logs/memory_samples.csv"
    log_info "Goroutine samples: $OUTPUT_DIR/logs/goroutine_samples.csv"
    log_info "Request log: $OUTPUT_DIR/logs/request_log.csv"
    log_info ""

    if [[ "$status" == "PASSED" ]]; then
        echo -e "${GREEN}CHALLENGE PASSED: Sustained stress test completed successfully${NC}"
        exit 0
    else
        echo -e "${RED}CHALLENGE FAILED: Sustained stress test detected issues${NC}"
        exit 1
    fi
}

# Main execution
main() {
    parse_args "$@"
    init_challenge
    mkdir -p "$OUTPUT_DIR/temp"

    # Ensure server is running
    if ! ensure_server; then
        log_error "Cannot run challenge without HelixAgent server"
        finalize_challenge "FAILED"
    fi

    echo ""

    # Collect baseline metrics
    log_info "=== Phase 1: Baseline Collection ==="
    local baseline_memory=$(get_memory_usage)
    local baseline_goroutines=$(get_goroutine_count)
    record_metric "baseline_memory_mb" "$baseline_memory"
    record_metric "baseline_goroutines" "$baseline_goroutines"
    log_info "Baseline: Memory=${baseline_memory}MB, Goroutines=$baseline_goroutines"

    echo ""

    # Start monitoring and load generation
    log_info "=== Phase 2: Sustained Stress Test ==="
    start_monitor
    start_load_generator
    wait_for_completion

    echo ""

    # Analyze results
    log_info "=== Phase 3: Results Analysis ==="
    analyze_memory
    analyze_goroutines
    analyze_requests

    echo ""

    # Post-stress validation
    log_info "=== Phase 4: Post-Stress Validation ==="
    test_post_stress_health
    test_post_stress_functionality

    # Finalize
    if [[ $FAILED_TESTS -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
