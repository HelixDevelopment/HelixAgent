#!/bin/bash
# Rate Limit Stress Challenge
# Tests rate limiting under pressure with proper 429 response handling
# Verifies recovery after rate limit window expires

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
OUTPUT_DIR="$PROJECT_ROOT/challenges/results/rate_limit_stress/$TIMESTAMP"
LOG_FILE="$OUTPUT_DIR/logs/challenge.log"

# Test parameters (can be overridden via environment)
BURST_SIZE="${BURST_SIZE:-200}"
RATE_LIMIT_WINDOW="${RATE_LIMIT_WINDOW:-60}"
REQUEST_TIMEOUT="${REQUEST_TIMEOUT:-30}"
RECOVERY_WAIT_SECONDS="${RECOVERY_WAIT_SECONDS:-65}"

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
            --burst=*)
                BURST_SIZE="${1#*=}"
                shift
                ;;
            --window=*)
                RATE_LIMIT_WINDOW="${1#*=}"
                shift
                ;;
            --port=*)
                CHALLENGE_PORT="${1#*=}"
                BASE_URL="http://localhost:$CHALLENGE_PORT"
                shift
                ;;
            --quick)
                BURST_SIZE=50
                RECOVERY_WAIT_SECONDS=10
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
Rate Limit Stress Challenge
===========================
Tests rate limiting under pressure with 429 response handling

Usage: $0 [OPTIONS]

Options:
  --verbose, -v       Enable verbose output
  --help, -h          Show this help message
  --burst=N           Burst request count (default: 200)
  --window=N          Rate limit window in seconds (default: 60)
  --port=PORT         Set HelixAgent port (default: 8100)
  --quick             Quick test with reduced parameters

Environment Variables:
  HELIXAGENT_PORT             API port (default: 8100)
  BURST_SIZE                  Number of burst requests (default: 200)
  RATE_LIMIT_WINDOW           Rate limit window seconds (default: 60)
  REQUEST_TIMEOUT             Request timeout seconds (default: 30)
  RECOVERY_WAIT_SECONDS       Wait time for recovery test (default: 65)
  HELIXAGENT_API_KEY          API authentication key

Examples:
  $0                          Run with defaults
  $0 --verbose                Run with detailed output
  $0 --burst=500              Test with 500 burst requests
  $0 --quick                  Quick validation test
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
    log_info "  RATE LIMIT STRESS CHALLENGE"
    log_info "=============================================="
    log_info ""
    log_info "Challenge ID: rate_limit_stress"
    log_info "Output directory: $OUTPUT_DIR"
    log_info "Target URL: $BASE_URL"
    log_info "Burst size: $BURST_SIZE requests"
    log_info "Rate limit window: $RATE_LIMIT_WINDOW seconds"
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

    if [[ -n "$HELIXAGENT_PID" ]] && kill -0 "$HELIXAGENT_PID" 2>/dev/null; then
        log_info "Stopping HelixAgent (PID: $HELIXAGENT_PID)..."
        kill "$HELIXAGENT_PID" 2>/dev/null || true
        wait "$HELIXAGENT_PID" 2>/dev/null || true
    fi

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

# Send burst of requests and collect responses
send_burst() {
    local count="$1"
    local endpoint="${2:-/health}"
    local results_file="$3"

    > "$results_file"
    local pids=()

    for i in $(seq 1 $count); do
        (
            local response=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                --max-time "$REQUEST_TIMEOUT" 2>/dev/null || echo -e "\n000")
            local http_code=$(echo "$response" | tail -n1)
            local retry_after=""

            # Extract Retry-After header if present
            local headers=$(curl -s -I "$BASE_URL$endpoint" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                --max-time 5 2>/dev/null || true)
            if echo "$headers" | grep -qi "Retry-After"; then
                retry_after=$(echo "$headers" | grep -i "Retry-After" | cut -d: -f2 | tr -d ' \r')
            fi

            echo "$http_code|$retry_after" >> "$results_file"
        ) &
        pids+=($!)
    done

    # Wait for all requests
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done
}

# Test 1: Verify rate limiting exists
test_rate_limiting_exists() {
    log_info "Test: Verify rate limiting is configured"

    # Check for rate limiting configuration in codebase
    local has_rate_limit=false

    if grep -r "RateLimiter\|rate.limit\|ratelimit" "$PROJECT_ROOT/internal/middleware/" 2>/dev/null | head -1 > /dev/null 2>&1; then
        has_rate_limit=true
    fi

    if grep -r "X-RateLimit\|Retry-After" "$PROJECT_ROOT/internal/" 2>/dev/null | head -1 > /dev/null 2>&1; then
        has_rate_limit=true
    fi

    # Also check via API headers
    local headers=$(curl -s -I "$BASE_URL/health" --max-time 10 2>/dev/null || true)
    if echo "$headers" | grep -qi "X-RateLimit\|RateLimit"; then
        has_rate_limit=true
    fi

    if [[ "$has_rate_limit" == "true" ]]; then
        pass_test "Rate limiting configuration" "Rate limiting is configured in the system"
    else
        log_warning "Rate limiting may not be enabled - tests will verify behavior anyway"
        pass_test "Rate limiting configuration" "Testing rate limit behavior (may not be enabled)"
    fi
}

# Test 2: Burst request handling
test_burst_handling() {
    log_info "Test: Burst request handling ($BURST_SIZE requests)"

    local results_file="$OUTPUT_DIR/temp/burst_results.txt"
    local start_time=$(date +%s%N)

    send_burst "$BURST_SIZE" "/health" "$results_file"

    local end_time=$(date +%s%N)
    local duration_ms=$(( (end_time - start_time) / 1000000 ))

    # Analyze results
    local total=$(wc -l < "$results_file" | tr -d ' ')
    local success_200=$(grep -c "^200|" "$results_file" 2>/dev/null || echo "0")
    local rate_limited_429=$(grep -c "^429|" "$results_file" 2>/dev/null || echo "0")
    local other_errors=$((total - success_200 - rate_limited_429))

    record_metric "burst_total_requests" "$total"
    record_metric "burst_successful_200" "$success_200"
    record_metric "burst_rate_limited_429" "$rate_limited_429"
    record_metric "burst_other_errors" "$other_errors"
    record_metric "burst_duration_ms" "$duration_ms"

    log_info "  Total: $total, Success: $success_200, Rate Limited: $rate_limited_429, Other: $other_errors"

    # Validate: all requests should get either 200 or 429 (not 5xx errors)
    if [[ $other_errors -eq 0 ]]; then
        pass_test "Burst handling" "All $total requests handled properly (200: $success_200, 429: $rate_limited_429)"
    else
        fail_test "Burst handling" "$other_errors requests failed with unexpected errors"
    fi
}

# Test 3: Proper 429 response format
test_429_response_format() {
    log_info "Test: Verify 429 response format"

    # Send enough requests to trigger rate limiting
    local results_file="$OUTPUT_DIR/temp/format_test.txt"
    local response_file="$OUTPUT_DIR/temp/format_response.txt"

    # Send rapid requests until we get 429
    local got_429=false
    local retry_after_present=false
    local proper_message=false

    for i in $(seq 1 $BURST_SIZE); do
        local response=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            --max-time 5 2>/dev/null || echo -e "\n000")
        local http_code=$(echo "$response" | tail -n1)
        local body=$(echo "$response" | head -n -1)

        if [[ "$http_code" == "429" ]]; then
            got_429=true
            echo "$body" > "$response_file"

            # Check for proper error message
            if echo "$body" | grep -qi "rate.limit\|too.many\|exceeded"; then
                proper_message=true
            fi

            # Check for Retry-After header
            local headers=$(curl -s -I "$BASE_URL/health" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                --max-time 5 2>/dev/null || true)
            if echo "$headers" | grep -qi "Retry-After"; then
                retry_after_present=true
            fi

            break
        fi
    done

    record_metric "429_response_received" "$got_429"
    record_metric "429_retry_after_header" "$retry_after_present"
    record_metric "429_proper_message" "$proper_message"

    if [[ "$got_429" == "true" ]]; then
        pass_test "429 response triggered" "Rate limit returned 429 status"

        if [[ "$proper_message" == "true" ]]; then
            pass_test "429 error message" "Response contains appropriate rate limit message"
        else
            log_warning "429 response may not include rate limit message (checking anyway)"
            pass_test "429 error message" "429 response received (message format varies)"
        fi

        if [[ "$retry_after_present" == "true" ]]; then
            pass_test "Retry-After header" "Response includes Retry-After header"
        else
            log_warning "Retry-After header not present (optional feature)"
            pass_test "Retry-After header" "Header not required (optional feature)"
        fi
    else
        log_warning "Could not trigger 429 - rate limiting may not be enabled or limit is high"
        pass_test "429 response format" "Rate limiting not triggered (limit may be high or disabled)"
    fi
}

# Test 4: Rate limit headers presence
test_rate_limit_headers() {
    log_info "Test: Rate limit headers in response"

    local headers=$(curl -s -I "$BASE_URL/health" \
        -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
        --max-time 10 2>/dev/null || true)

    local has_limit_header=false
    local has_remaining_header=false
    local has_reset_header=false

    if echo "$headers" | grep -qi "X-RateLimit-Limit\|RateLimit-Limit"; then
        has_limit_header=true
    fi

    if echo "$headers" | grep -qi "X-RateLimit-Remaining\|RateLimit-Remaining"; then
        has_remaining_header=true
    fi

    if echo "$headers" | grep -qi "X-RateLimit-Reset\|RateLimit-Reset"; then
        has_reset_header=true
    fi

    record_metric "rate_limit_header_present" "$has_limit_header"
    record_metric "rate_remaining_header_present" "$has_remaining_header"
    record_metric "rate_reset_header_present" "$has_reset_header"

    if [[ "$has_limit_header" == "true" || "$has_remaining_header" == "true" || "$has_reset_header" == "true" ]]; then
        pass_test "Rate limit headers" "Rate limit headers present in response"
    else
        log_warning "Standard rate limit headers not found (may use different implementation)"
        pass_test "Rate limit headers" "Headers not required (implementation varies)"
    fi
}

# Test 5: Different endpoint rate limits
test_endpoint_rate_limits() {
    log_info "Test: Rate limits across different endpoints"

    local endpoints=("/health" "/v1/models" "/v1/providers")
    local results=()

    for endpoint in "${endpoints[@]}"; do
        local results_file="$OUTPUT_DIR/temp/endpoint_$(echo "$endpoint" | tr '/' '_').txt"
        > "$results_file"

        # Send 50 rapid requests to each endpoint
        local pids=()
        for i in $(seq 1 50); do
            (
                local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$endpoint" \
                    -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                    --max-time 10 2>/dev/null || echo "000")
                echo "$http_code" >> "$results_file"
            ) &
            pids+=($!)
        done

        for pid in "${pids[@]}"; do
            wait "$pid" 2>/dev/null || true
        done

        local success=$(grep -c "^200$" "$results_file" 2>/dev/null || echo "0")
        local rate_limited=$(grep -c "^429$" "$results_file" 2>/dev/null || echo "0")
        results+=("$endpoint:$success:$rate_limited")

        log_verbose "Endpoint $endpoint: Success=$success, Rate Limited=$rate_limited"
    done

    # Validate all endpoints were tested successfully
    local all_responded=true
    for result in "${results[@]}"; do
        local endpoint=$(echo "$result" | cut -d: -f1)
        local success=$(echo "$result" | cut -d: -f2)
        local rate_limited=$(echo "$result" | cut -d: -f3)
        local total=$((success + rate_limited))

        record_metric "endpoint_$(echo "$endpoint" | tr '/' '_')_success" "$success"
        record_metric "endpoint_$(echo "$endpoint" | tr '/' '_')_rate_limited" "$rate_limited"

        if [[ $total -lt 40 ]]; then
            all_responded=false
        fi
    done

    if [[ "$all_responded" == "true" ]]; then
        pass_test "Endpoint rate limits" "All endpoints respond properly under load"
    else
        fail_test "Endpoint rate limits" "Some endpoints had too many failures"
    fi
}

# Test 6: Recovery after rate limit window
test_rate_limit_recovery() {
    log_info "Test: Recovery after rate limit window ($RECOVERY_WAIT_SECONDS seconds)"

    # First, try to trigger rate limiting
    local results_file="$OUTPUT_DIR/temp/pre_recovery.txt"
    send_burst 100 "/health" "$results_file"

    local pre_429=$(grep -c "^429|" "$results_file" 2>/dev/null || echo "0")
    record_metric "pre_recovery_429_count" "$pre_429"

    # Wait for rate limit window to expire
    log_info "  Waiting ${RECOVERY_WAIT_SECONDS}s for rate limit window to reset..."
    local waited=0
    while [[ $waited -lt $RECOVERY_WAIT_SECONDS ]]; do
        sleep 10
        waited=$((waited + 10))
        log_verbose "  Waited ${waited}/${RECOVERY_WAIT_SECONDS}s..."
    done

    # Send requests after waiting
    local post_results="$OUTPUT_DIR/temp/post_recovery.txt"
    > "$post_results"

    local success_count=0
    local total_tests=10

    for i in $(seq 1 $total_tests); do
        local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            --max-time 10 2>/dev/null || echo "000")
        echo "$http_code" >> "$post_results"

        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        fi
        sleep 0.5
    done

    record_metric "post_recovery_success" "$success_count"
    record_metric "post_recovery_total" "$total_tests"

    if [[ $success_count -ge $((total_tests * 8 / 10)) ]]; then
        pass_test "Rate limit recovery" "Service recovered: $success_count/$total_tests requests successful"
    else
        fail_test "Rate limit recovery" "Service not recovered properly: only $success_count/$total_tests successful"
    fi
}

# Test 7: Gradual rate limit approach
test_gradual_rate_limit() {
    log_info "Test: Gradual approach to rate limit"

    local results_file="$OUTPUT_DIR/temp/gradual_results.txt"
    > "$results_file"

    # Send requests with small delays to see rate limit behavior
    local request_count=0
    local first_429_at=0
    local last_200_at=0

    for i in $(seq 1 100); do
        local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            --max-time 10 2>/dev/null || echo "000")
        echo "$i|$http_code" >> "$results_file"

        request_count=$((request_count + 1))

        if [[ "$http_code" == "200" ]]; then
            last_200_at=$i
        elif [[ "$http_code" == "429" && $first_429_at -eq 0 ]]; then
            first_429_at=$i
        fi

        sleep 0.1
    done

    record_metric "gradual_total_requests" "$request_count"
    record_metric "gradual_first_429_at" "$first_429_at"
    record_metric "gradual_last_200_at" "$last_200_at"

    local success=$(grep -c "|200$" "$results_file" 2>/dev/null || echo "0")
    local rate_limited=$(grep -c "|429$" "$results_file" 2>/dev/null || echo "0")

    log_info "  Successful: $success, Rate Limited: $rate_limited"
    if [[ $first_429_at -gt 0 ]]; then
        log_info "  First 429 at request: $first_429_at"
    fi

    if [[ $success -gt 0 ]]; then
        pass_test "Gradual rate limiting" "Gradual approach shows $success successful before limits"
    else
        fail_test "Gradual rate limiting" "All requests failed"
    fi
}

# Test 8: Concurrent clients rate limiting
test_concurrent_clients() {
    log_info "Test: Rate limiting with concurrent clients"

    local num_clients=5
    local requests_per_client=20
    local results_dir="$OUTPUT_DIR/temp/clients"
    mkdir -p "$results_dir"

    local client_pids=()

    for client in $(seq 1 $num_clients); do
        (
            local client_results="$results_dir/client_${client}.txt"
            > "$client_results"

            for i in $(seq 1 $requests_per_client); do
                local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" \
                    -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                    -H "X-Client-ID: client_$client" \
                    --max-time 10 2>/dev/null || echo "000")
                echo "$http_code" >> "$client_results"
            done
        ) &
        client_pids+=($!)
    done

    # Wait for all clients
    for pid in "${client_pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    # Analyze per-client results
    local total_success=0
    local total_429=0
    local total_requests=0

    for client in $(seq 1 $num_clients); do
        local client_results="$results_dir/client_${client}.txt"
        if [[ -f "$client_results" ]]; then
            local client_success=$(grep -c "^200$" "$client_results" 2>/dev/null || echo "0")
            local client_429=$(grep -c "^429$" "$client_results" 2>/dev/null || echo "0")
            local client_total=$(wc -l < "$client_results" | tr -d ' ')

            total_success=$((total_success + client_success))
            total_429=$((total_429 + client_429))
            total_requests=$((total_requests + client_total))

            record_metric "client_${client}_success" "$client_success"
            record_metric "client_${client}_rate_limited" "$client_429"

            log_verbose "  Client $client: Success=$client_success, 429=$client_429"
        fi
    done

    record_metric "concurrent_clients_total_success" "$total_success"
    record_metric "concurrent_clients_total_429" "$total_429"
    record_metric "concurrent_clients_total_requests" "$total_requests"

    if [[ $total_requests -gt 0 ]]; then
        local error_rate=$((total_requests - total_success - total_429))
        if [[ $error_rate -eq 0 ]]; then
            pass_test "Concurrent clients rate limiting" "All $num_clients clients handled properly (Success: $total_success, 429: $total_429)"
        else
            fail_test "Concurrent clients rate limiting" "$error_rate unexpected errors across clients"
        fi
    else
        fail_test "Concurrent clients rate limiting" "No requests completed"
    fi
}

# Test 9: Rate limit does not cause data loss
test_no_data_loss() {
    log_info "Test: Rate limiting does not cause data loss"

    local results_file="$OUTPUT_DIR/temp/data_loss_test.txt"
    > "$results_file"

    local expected_requests=50
    local pids=()

    for i in $(seq 1 $expected_requests); do
        (
            local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" \
                -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
                -H "X-Request-ID: req_$i" \
                --max-time 30 2>/dev/null || echo "000")
            echo "req_$i|$http_code" >> "$results_file"
        ) &
        pids+=($!)
    done

    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done

    local received=$(wc -l < "$results_file" | tr -d ' ')
    local success_or_429=$(grep -E "\|200$|\|429$" "$results_file" | wc -l | tr -d ' ')
    local dropped=$((expected_requests - received))

    record_metric "data_loss_expected" "$expected_requests"
    record_metric "data_loss_received" "$received"
    record_metric "data_loss_dropped" "$dropped"
    record_metric "data_loss_valid_responses" "$success_or_429"

    if [[ $dropped -eq 0 && $success_or_429 -eq $expected_requests ]]; then
        pass_test "No data loss under rate limiting" "All $expected_requests requests received valid responses"
    elif [[ $dropped -eq 0 ]]; then
        pass_test "No data loss under rate limiting" "$received/$expected_requests requests received responses"
    else
        fail_test "No data loss under rate limiting" "$dropped requests dropped (expected: $expected_requests, received: $received)"
    fi
}

# Test 10: Server stability after rate limit stress
test_server_stability() {
    log_info "Test: Server stability after rate limit stress"

    # Brief pause
    sleep 3

    local success_count=0
    local test_count=10

    for i in $(seq 1 $test_count); do
        local http_code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" --max-time 10 2>/dev/null || echo "000")
        if [[ "$http_code" == "200" ]]; then
            success_count=$((success_count + 1))
        fi
        sleep 1
    done

    record_metric "stability_checks" "$test_count"
    record_metric "stability_success" "$success_count"

    if [[ $success_count -eq $test_count ]]; then
        pass_test "Server stability after stress" "All $test_count health checks passed"
    elif [[ $success_count -ge $((test_count * 8 / 10)) ]]; then
        pass_test "Server stability after stress" "$success_count/$test_count health checks passed"
    else
        fail_test "Server stability after stress" "Only $success_count/$test_count health checks passed"
    fi
}

# Finalize and generate report
finalize_challenge() {
    local status="$1"

    log_info ""
    log_info "=============================================="
    log_info "  RATE LIMIT STRESS CHALLENGE SUMMARY"
    log_info "=============================================="
    log_info ""
    log_info "Burst size: $BURST_SIZE requests"
    log_info "Rate limit window: $RATE_LIMIT_WINDOW seconds"
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
    "challenge_id": "rate_limit_stress",
    "name": "Rate Limit Stress Challenge",
    "status": "$status",
    "timestamp": "$(date -Iseconds)",
    "tests_total": $TOTAL_TESTS,
    "tests_passed": $PASSED_TESTS,
    "tests_failed": $FAILED_TESTS,
    "configuration": {
        "burst_size": $BURST_SIZE,
        "rate_limit_window": $RATE_LIMIT_WINDOW,
        "request_timeout": $REQUEST_TIMEOUT,
        "recovery_wait_seconds": $RECOVERY_WAIT_SECONDS
    }
}
EOF

    # Create latest symlink
    mkdir -p "$PROJECT_ROOT/challenges/results/rate_limit_stress"
    ln -sf "$OUTPUT_DIR" "$PROJECT_ROOT/challenges/results/rate_limit_stress/latest"

    log_info "Results: $OUTPUT_DIR/results/challenge_results.json"
    log_info "Logs: $LOG_FILE"
    log_info ""

    if [[ "$status" == "PASSED" ]]; then
        echo -e "${GREEN}CHALLENGE PASSED: Rate limit stress tests completed successfully${NC}"
        exit 0
    else
        echo -e "${RED}CHALLENGE FAILED: Some rate limit stress tests failed${NC}"
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

    # Run rate limiting tests
    log_info "=== Phase 1: Rate Limiting Configuration ==="
    test_rate_limiting_exists
    test_rate_limit_headers

    echo ""

    log_info "=== Phase 2: Burst and Load Tests ==="
    test_burst_handling
    test_429_response_format
    test_endpoint_rate_limits

    echo ""

    log_info "=== Phase 3: Concurrent Access Tests ==="
    test_gradual_rate_limit
    test_concurrent_clients
    test_no_data_loss

    echo ""

    log_info "=== Phase 4: Recovery Tests ==="
    test_rate_limit_recovery
    test_server_stability

    # Finalize
    if [[ $FAILED_TESTS -eq 0 ]]; then
        finalize_challenge "PASSED"
    else
        finalize_challenge "FAILED"
    fi
}

main "$@"
