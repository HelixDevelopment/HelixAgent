#!/bin/bash
# Challenge Framework - Common functions for all challenges
# BINARY ONLY - NO SOURCE CODE EXECUTION

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Globals
CHALLENGE_ID=""
CHALLENGE_NAME=""
OUTPUT_DIR=""
LOG_FILE=""
RESULTS_FILE=""
START_TIME=""
HELIXAGENT_PID=""
PROJECT_ROOT=""

# Initialize challenge
init_challenge() {
    CHALLENGE_ID="$1"
    CHALLENGE_NAME="$2"

    # Get project root
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

    # Create output directory with timestamp.
    #
    # HXC-272 defect 1 (shared results folder). TIMESTAMP used to be the
    # WHOLE of OUTPUT_DIR's leaf component, with nothing appended for
    # uniqueness — two challenges (or two runs of the same challenge) that
    # started within the same wall-clock second computed the IDENTICAL
    # OUTPUT_DIR and therefore shared one assertions.log. finalize_challenge()
    # decides its verdict by grep -c'ing PASSED/FAILED/SKIPPED lines out of
    # that log, so a same-second collision let one run inherit a neighbour's
    # outcomes in BOTH directions: inherited FAILED lines could flip a
    # healthy run red, and — the dangerous direction — inherited PASSED lines
    # could flip a run that verified nothing into a reported success.
    #
    # Fix: TIMESTAMP itself is UNCHANGED (still the leading, human-readable,
    # chronologically-sortable component every existing consumer relies on —
    # the "Output directory:" log line, `ls -t`/`ls -td` recency listings,
    # CATALOG.md's documented "find latest results directory" recipe); the
    # directory `mkdir` actually creates is made collision-proof by
    # CONSTRUCTION rather than by lower-probability entropy. `mkdir` (no -p
    # on the leaf) either wins a path that provably did not exist a moment
    # ago or fails with EEXIST — that is an OS-enforced guarantee, so two
    # processes racing on the identical second can never both "win" the same
    # directory. A PID + nanosecond + $RANDOM suffix makes collisions
    # astronomically unlikely on the first attempt; the retry loop is what
    # makes non-collision a certainty rather than a probability.
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    local _results_root="$PROJECT_ROOT/challenges/results/${CHALLENGE_ID}"
    mkdir -p "$_results_root"
    local _attempt=0
    local _nanos _suffix _candidate
    while :; do
        _nanos=$(date +%N 2>/dev/null || echo 0)
        _suffix="$$_${_nanos}_${RANDOM}${RANDOM}"
        _candidate="${_results_root}/${TIMESTAMP}_${_suffix}"
        if mkdir "$_candidate" 2>/dev/null; then
            OUTPUT_DIR="$_candidate"
            break
        fi
        _attempt=$((_attempt + 1))
        if [[ $_attempt -ge 50 ]]; then
            echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') could not allocate a unique challenge results directory under $_results_root after $_attempt attempts" >&2
            return 1
        fi
    done
    mkdir -p "$OUTPUT_DIR/logs" "$OUTPUT_DIR/results"

    LOG_FILE="$OUTPUT_DIR/logs/${CHALLENGE_ID}.log"
    RESULTS_FILE="$OUTPUT_DIR/results/${CHALLENGE_ID}_results.json"
    START_TIME=$(date +%s)

    # Initialize results JSON
    echo "{\"challenge_id\": \"$CHALLENGE_ID\", \"name\": \"$CHALLENGE_NAME\", \"status\": \"running\", \"start_time\": \"$(date -Iseconds)\", \"assertions\": [], \"metrics\": {}}" > "$RESULTS_FILE"

    log_info "Starting challenge: $CHALLENGE_NAME"
    log_info "Output directory: $OUTPUT_DIR"
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

log_section() {
    local msg="=== $1 ==="
    echo ""
    echo -e "${CYAN}$msg${NC}"
    if [[ -n "$LOG_FILE" ]] && [[ -f "$LOG_FILE" ]]; then
        echo "" >> "$LOG_FILE"
        echo "$msg" >> "$LOG_FILE"
    fi
}

log_pass() {
    local msg="[PASS] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${GREEN}$msg${NC}"
    if [[ -n "$LOG_FILE" ]] && [[ -f "$LOG_FILE" ]]; then
        echo "$msg" >> "$LOG_FILE"
    fi
}

log_fail() {
    local msg="[FAIL] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${RED}$msg${NC}"
    if [[ -n "$LOG_FILE" ]] && [[ -f "$LOG_FILE" ]]; then
        echo "$msg" >> "$LOG_FILE"
    fi
}

log_error() {
    local msg="[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1"
    echo -e "${RED}$msg${NC}"
    echo "$msg" >> "$LOG_FILE"
}

# Load environment variables.
# Uses `set +u` while sourcing because production .env files can contain
# bash-variable substitutions like `FOO=$Bar` where `Bar` is intentionally
# unset on this host (treated as empty by HelixAgent's env loader). Under
# `set -u`, sourcing those lines would fatally fail. We restore the
# caller's `-u` setting after.
load_env() {
    local env_file="$PROJECT_ROOT/.env"
    local prev_u
    case "$-" in *u*) prev_u=1 ;; *) prev_u=0 ;; esac
    set +u
    if [[ -f "$env_file" ]]; then
        set -a
        source "$env_file"
        set +a
        log_info "Loaded environment from $env_file"
    else
        env_file="$PROJECT_ROOT/challenges/.env"
        if [[ -f "$env_file" ]]; then
            set -a
            source "$env_file"
            set +a
            log_info "Loaded environment from $env_file"
        else
            log_warning "No .env file found"
        fi
    fi
    [[ "$prev_u" == "1" ]] && set -u
    return 0
}

# Detect container runtime
detect_container_runtime() {
    if command -v docker &> /dev/null && docker ps &> /dev/null; then
        echo "docker"
    elif command -v podman &> /dev/null && podman ps &> /dev/null; then
        echo "podman"
    else
        log_error "No container runtime (docker/podman) available"
        return 1
    fi
}

# Check binary exists
check_binary() {
    local binary="$1"
    local name="$2"

    if [[ ! -x "$binary" ]]; then
        log_error "$name binary not found or not executable: $binary"
        return 1
    fi
    log_info "$name binary found: $binary"
    return 0
}

# Get HelixAgent binary path
get_helixagent_binary() {
    local binary="$PROJECT_ROOT/helixagent"
    if [[ ! -x "$binary" ]]; then
        binary="$PROJECT_ROOT/bin/helixagent"
    fi
    if [[ ! -x "$binary" ]]; then
        log_error "HelixAgent binary not found. Run: make build"
        return 1
    fi
    echo "$binary"
}

# Get LLMsVerifier binary path (canonical at meta-repo root dependencies/HelixDevelopment/LLMsVerifier per P1.5-T03.01)
get_verifier_binary() {
    local binary="$PROJECT_ROOT/../dependencies/HelixDevelopment/LLMsVerifier/llm-verifier/llm-verifier"
    if [[ ! -x "$binary" ]]; then
        binary="$PROJECT_ROOT/../dependencies/HelixDevelopment/LLMsVerifier/bin/llm-verifier"
    fi
    if [[ ! -x "$binary" ]]; then
        log_error "LLMsVerifier binary not found. Run: cd ../dependencies/HelixDevelopment/LLMsVerifier && make build"
        return 1
    fi
    echo "$binary"
}

# Start HelixAgent
start_helixagent() {
    # Default port: HELIXAGENT_PORT, then 8100 (canonical per
    # docs/development/port-registry.md). The previous default of 7061
    # was stale and caused start_helixagent to spawn a NEW instance on
    # an unused port even when one was already running on 8100,
    # producing false-positive "HelixAgent stopped" cleanup messages
    # observed in the 2026-04-29 sweep.
    local port="${1:-${HELIXAGENT_PORT:-8100}}"
    local config="${2:-$PROJECT_ROOT/configs/production.yaml}"

    local binary=$(get_helixagent_binary) || return 1

    log_info "Starting HelixAgent on port $port..."

    # Check if already running
    if curl -s --max-time 60 "http://localhost:$port/health" > /dev/null 2>&1; then
        log_info "HelixAgent already running on port $port"
        return 0
    fi

    # Start HelixAgent
    PORT=$port "$binary" > "$OUTPUT_DIR/logs/helixagent.log" 2>&1 &
    HELIXAGENT_PID=$!
    echo "$HELIXAGENT_PID" > "$OUTPUT_DIR/helixagent.pid"

    # Wait for startup
    local max_wait=30
    local waited=0
    while ! curl -s --max-time 60 "http://localhost:$port/health" > /dev/null 2>&1; do
        sleep 1
        waited=$((waited + 1))
        if [[ $waited -ge $max_wait ]]; then
            log_error "HelixAgent failed to start within ${max_wait}s"
            return 1
        fi
    done

    log_success "HelixAgent started (PID: $HELIXAGENT_PID)"
    return 0
}

# Stop HelixAgent
stop_helixagent() {
    if [[ -n "$HELIXAGENT_PID" ]] && kill -0 "$HELIXAGENT_PID" 2>/dev/null; then
        log_info "Stopping HelixAgent (PID: $HELIXAGENT_PID)..."
        kill "$HELIXAGENT_PID" 2>/dev/null || true
        wait "$HELIXAGENT_PID" 2>/dev/null || true
        log_info "HelixAgent stopped"
    fi

    # Also check for pid file
    if [[ -f "$OUTPUT_DIR/helixagent.pid" ]]; then
        local pid=$(cat "$OUTPUT_DIR/helixagent.pid")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
        rm -f "$OUTPUT_DIR/helixagent.pid"
    fi
}

# Start infrastructure (Docker/Podman)
start_infrastructure() {
    local runtime=$(detect_container_runtime) || return 1
    local compose_file="$PROJECT_ROOT/docker-compose.yml"

    if [[ ! -f "$compose_file" ]]; then
        log_warning "No docker-compose.yml found, skipping infrastructure"
        return 0
    fi

    log_info "Starting infrastructure with $runtime..."

    if [[ "$runtime" == "docker" ]]; then
        docker-compose -f "$compose_file" up -d postgres redis 2>&1 | tee -a "$LOG_FILE"
    else
        podman-compose -f "$compose_file" up -d postgres redis 2>&1 | tee -a "$LOG_FILE"
    fi

    # Wait for services
    sleep 5
    log_success "Infrastructure started"
}

# Stop infrastructure
stop_infrastructure() {
    local runtime=$(detect_container_runtime) || return 0
    local compose_file="$PROJECT_ROOT/docker-compose.yml"

    if [[ -f "$compose_file" ]]; then
        log_info "Stopping infrastructure..."
        if [[ "$runtime" == "docker" ]]; then
            docker-compose -f "$compose_file" down 2>&1 | tee -a "$LOG_FILE" || true
        else
            podman-compose -f "$compose_file" down 2>&1 | tee -a "$LOG_FILE" || true
        fi
    fi
}

# Make API request to HelixAgent
api_request() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local port="${HELIXAGENT_PORT:-8100}"

    local url="http://localhost:$port$endpoint"
    local response_file="$OUTPUT_DIR/logs/api_response_$(date +%s%N).json"

    if [[ -n "$data" ]]; then
        curl -s --max-time 60 -X "$method" "$url" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -d "$data" \
            -o "$response_file" \
            -w "%{http_code}"
    else
        curl -s --max-time 60 -X "$method" "$url" \
            -H "Authorization: Bearer ${HELIXAGENT_API_KEY:-test}" \
            -o "$response_file" \
            -w "%{http_code}"
    fi

    local http_code=$?
    cat "$response_file"
    return $http_code
}

# Assertion tracking (no jq dependency)
ASSERTION_LOG=""
METRIC_LOG=""

# Record assertion result
record_assertion() {
    local assertion_type="$1"
    local target="$2"
    local passed="$3"
    local message="$4"

    local status="PASSED"
    [[ "$passed" != "true" ]] && status="FAILED"

    # Append to assertion log (simple text format)
    echo "${assertion_type}|${target}|${status}|${message}" >> "$OUTPUT_DIR/logs/assertions.log"
    ASSERTION_LOG="${ASSERTION_LOG}${assertion_type}|${target}|${status}|${message}\n"

    if [[ "$status" == "PASSED" ]]; then
        log_success "Assertion $assertion_type ($target): PASSED"
    else
        log_error "Assertion $assertion_type ($target): FAILED - $message"
    fi
}

# record_skip — emit a SKIPPED assertion (CONST-035 §4 "Skips are loud").
#
# Use this when a probe cannot exercise the feature (endpoint returns 404,
# upstream service unreachable, env var unset). NEVER use record_assertion
# with status=true to mask a missing feature — that's a wrapper bluff per
# CONST-035 bluff taxonomy. SKIPPED is mechanically distinguishable from
# PASSED in the master aggregator so absence of coverage is loud rather
# than silent.
#
# Args: assertion_type target reason
#   reason MUST include a SKIP-OK ticket reference (e.g. "#endpoint-not-impl")
record_skip() {
    local assertion_type="$1"
    local target="$2"
    local reason="$3"

    if [[ "$reason" != *"SKIP-OK:"* ]]; then
        reason="$reason (SKIP-OK: #unmarked-skip-needs-ticket)"
    fi

    echo "${assertion_type}|${target}|SKIPPED|${reason}" >> "$OUTPUT_DIR/logs/assertions.log"
    ASSERTION_LOG="${ASSERTION_LOG}${assertion_type}|${target}|SKIPPED|${reason}\n"

    log_warning "Assertion $assertion_type ($target): SKIPPED - $reason"
}

# Record metric
record_metric() {
    local name="$1"
    local value="$2"

    # Append to metric log
    echo "${name}=${value}" >> "$OUTPUT_DIR/logs/metrics.log"
    METRIC_LOG="${METRIC_LOG}${name}=${value}\n"

    log_info "Metric $name: $value"
}

# Finalize challenge
finalize_challenge() {
    local status="$1"
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))

    # Count passed/failed assertions from log file.
    # Drainage report 2026-04-25 Finding #16: the original
    #   passed=$(grep -c ... || echo 0)
    # produced multi-line output ("0\n0") in some shell-mode interactions
    # when grep had zero matches (grep -c outputs "0" + exits 1; the OR
    # then ALSO emits "0", and the outer command-substitution preserves
    # the inner newline). Interpolated into the JSON heredoc this produced
    # invalid JSON like:
    #   "assertions_failed": 0
    #   0
    #   }
    # making the file unparseable, breaking the master-summary aggregator
    # and any downstream automation. Confirmed in production output:
    #   xxd → "_failed": 0.0.}.   (two zeros separated by newline)
    # Fix: capture grep output once with set +e so the OR fallback is not
    # needed; default to 0 only if the result is empty.
    local passed=0
    local failed=0
    local skipped=0
    if [[ -f "$OUTPUT_DIR/logs/assertions.log" ]]; then
        set +e
        passed=$(grep -c "|PASSED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null)
        failed=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null)
        skipped=$(grep -c "|SKIPPED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null)
        set -e
        [[ -z "$passed" ]] && passed=0
        [[ -z "$failed" ]] && failed=0
        [[ -z "$skipped" ]] && skipped=0
    fi

    # Create results JSON (simple format)
    cat > "$RESULTS_FILE" << EOF
{
  "challenge_id": "$CHALLENGE_ID",
  "name": "$CHALLENGE_NAME",
  "status": "$status",
  "start_time": "$(date -d "@$START_TIME" -Iseconds 2>/dev/null || date -Iseconds)",
  "end_time": "$(date -Iseconds)",
  "duration_seconds": $duration,
  "assertions_passed": $passed,
  "assertions_failed": $failed,
  "assertions_skipped": $skipped
}
EOF

    # Create summary report
    local report_file="$OUTPUT_DIR/results/${CHALLENGE_ID}_report.md"
    cat > "$report_file" << EOF
# Challenge Report: $CHALLENGE_NAME

**Challenge ID:** $CHALLENGE_ID
**Status:** $status
**Duration:** ${duration}s
**Timestamp:** $(date -Iseconds)

## Assertions

| Type | Target | Status |
|------|--------|--------|
EOF

    # Add assertions from log file
    if [[ -f "$OUTPUT_DIR/logs/assertions.log" ]]; then
        while IFS='|' read -r type target assertion_status message; do
            echo "| $type | $target | $assertion_status |" >> "$report_file"
        done < "$OUTPUT_DIR/logs/assertions.log"
    fi

    cat >> "$report_file" << EOF

**Passed:** $passed
**Failed:** $failed
**Skipped:** $skipped

## Metrics

EOF

    # Add metrics from log file
    if [[ -f "$OUTPUT_DIR/logs/metrics.log" ]]; then
        while IFS='=' read -r name value; do
            echo "- **$name:** $value" >> "$report_file"
        done < "$OUTPUT_DIR/logs/metrics.log"
    fi

    cat >> "$report_file" << EOF

## Output Directory

\`$OUTPUT_DIR\`
EOF

    # Update the "latest" convenience pointer.
    #
    # HXC-272 defect 2 (stale "latest" pointer). `ln -sf SRC DEST` does NOT
    # unconditionally repoint DEST: when DEST already exists as a symlink to
    # a directory, `ln` dereferences it, finds a directory, and creates a NEW
    # link named basename(SRC) INSIDE that directory instead of replacing
    # DEST. From the second run of a day onward this dropped every later
    # run's pointer INSIDE the FIRST run's own results folder — `latest`
    # never moved — and every one of those writes bumped that folder's
    # mtime, so recency-by-mtime listings (e.g. `ls -td .../*/ | head -1`,
    # CATALOG.md's documented "find latest results directory" recipe) also
    # kept returning the first run forever. Both consumers of "the most
    # recent run" were silently wrong.
    #
    # Fix: remove whatever currently occupies "latest" (absent, a stale
    # symlink, or — defensively — a stray regular file) FIRST, then create
    # the symlink fresh. There is then nothing for `ln -s` to dereference, so
    # the replacement is unconditional regardless of what "latest" used to
    # be. And because a finished run's own directory is never written into
    # again by a later run, its mtime stops being disturbed — ordinary
    # recency-by-mtime listings start returning the genuinely newest run too,
    # with no separate fix needed for that consumer.
    local _latest_link="$PROJECT_ROOT/challenges/results/${CHALLENGE_ID}/latest"
    mkdir -p "$PROJECT_ROOT/challenges/results/${CHALLENGE_ID}"
    rm -f "$_latest_link"
    ln -s "$OUTPUT_DIR" "$_latest_link"

    if [[ "$status" == "PASSED" ]]; then
        log_success "Challenge $CHALLENGE_NAME completed: $status (${duration}s)"
        log_success "Assertions: $passed passed, $failed failed, $skipped skipped"
    else
        log_error "Challenge $CHALLENGE_NAME completed: $status (${duration}s)"
        log_error "Assertions: $passed passed, $failed failed, $skipped skipped"
    fi

    log_info "Results: $RESULTS_FILE"
    log_info "Report: $report_file"

    # Drainage report 2026-04-25 Finding #18: propagate status as exit code
    # so callers (and `set -e`) see real PASS/FAIL. Previously this function
    # always returned 0, causing standalone challenge scripts to exit 0
    # even when assertions failed — which then bubbled up through
    # generic_challenge.sh's meta-layer as "Challenge ... PASSED" and
    # produced false-green tallies like "498/498 PASSED" while real
    # assertions had failed.
    #
    # Two calling conventions are accepted (preserve backward compatibility
    # for skills_comprehensive_challenge.sh + cli_agent_integration_challenge.sh
    # which pass numeric "0"/"1" instead of the "PASSED"/"FAILED" strings):
    #   PASSED, "0"  → success (return 0)
    #   anything else → failure (return 1)
    case "$status" in
        PASSED|0) return 0 ;;
        *) return 1 ;;
    esac
}

# Cleanup on exit
cleanup() {
    stop_helixagent
}

trap cleanup EXIT

# Export all functions
export -f init_challenge log_info log_success log_warning log_error
export -f load_env detect_container_runtime check_binary
export -f get_helixagent_binary get_verifier_binary
export -f start_helixagent stop_helixagent
export -f start_infrastructure stop_infrastructure
export -f api_request record_assertion record_skip record_metric finalize_challenge
