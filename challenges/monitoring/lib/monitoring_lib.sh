#!/bin/bash
# HelixAgent Challenges - Core Monitoring Library
# Provides comprehensive system monitoring during challenge execution
#
# Features:
# - Log collection from all components
# - Memory leak detection
# - Resource usage monitoring (CPU, memory, disk I/O, network)
# - Warning/error detection and analysis
# - Report generation

set -e

# Colors for output
export MON_RED='\033[0;31m'
export MON_GREEN='\033[0;32m'
export MON_YELLOW='\033[1;33m'
export MON_BLUE='\033[0;34m'
export MON_PURPLE='\033[0;35m'
export MON_CYAN='\033[0;36m'
export MON_WHITE='\033[1;37m'
export MON_NC='\033[0m'

# Monitoring configuration
export MON_VERSION="1.0.0"
export MON_SAMPLE_INTERVAL="${MON_SAMPLE_INTERVAL:-2}"  # Seconds between resource samples
export MON_MEMORY_THRESHOLD="${MON_MEMORY_THRESHOLD:-100}"  # MB threshold for leak detection
export MON_CPU_THRESHOLD="${MON_CPU_THRESHOLD:-90}"  # CPU % threshold for warnings
export MON_FD_THRESHOLD="${MON_FD_THRESHOLD:-1000}"  # File descriptor threshold

# Global state
declare -g MON_SESSION_ID=""
declare -g MON_START_TIME=""
declare -g MON_BASE_DIR=""
declare -g MON_LOG_DIR=""
declare -g MON_REPORT_DIR=""
declare -g MON_PID_FILE=""
declare -g MON_BACKGROUND_PID=""
declare -g MON_ISSUES_COUNT=0
declare -g MON_WARNINGS_COUNT=0
declare -g MON_ERRORS_COUNT=0
declare -g MON_FIXES_COUNT=0

# Tracked PIDs for resource monitoring
declare -gA MON_TRACKED_PIDS=()

#===============================================================================
# INITIALIZATION
#===============================================================================

mon_init() {
    local session_name="${1:-challenges}"

    MON_SESSION_ID="${session_name}_$(date +%Y%m%d_%H%M%S)"
    MON_START_TIME=$(date +%s)

    # Determine base directory
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    MON_BASE_DIR="$(dirname "$script_dir")"

    # Setup directories
    MON_LOG_DIR="$MON_BASE_DIR/logs/$MON_SESSION_ID"
    MON_REPORT_DIR="$MON_BASE_DIR/reports/$MON_SESSION_ID"

    mkdir -p "$MON_LOG_DIR"/{components,resources,issues}
    mkdir -p "$MON_REPORT_DIR"

    # PID file for background monitor
    MON_PID_FILE="$MON_LOG_DIR/monitor.pid"

    # Initialize log files
    touch "$MON_LOG_DIR/master.log"
    touch "$MON_LOG_DIR/issues/warnings.log"
    touch "$MON_LOG_DIR/issues/errors.log"
    touch "$MON_LOG_DIR/issues/fixes.log"
    touch "$MON_LOG_DIR/resources/samples.jsonl"
    touch "$MON_LOG_DIR/resources/memory_baseline.json"

    # Log initialization
    mon_log "INFO" "Monitoring session initialized: $MON_SESSION_ID"
    mon_log "INFO" "Log directory: $MON_LOG_DIR"
    mon_log "INFO" "Report directory: $MON_REPORT_DIR"

    # Capture initial system state
    mon_capture_baseline

    # Output to stderr to avoid interfering with stdout captures
    echo -e "${MON_CYAN}[MONITOR]${MON_NC} Monitoring initialized: $MON_SESSION_ID" >&2
}

mon_capture_baseline() {
    local baseline_file="$MON_LOG_DIR/resources/memory_baseline.json"

    # Get system memory baseline
    local total_mem=$(free -m | awk '/^Mem:/ {print $2}')
    local used_mem=$(free -m | awk '/^Mem:/ {print $3}')
    local available_mem=$(free -m | awk '/^Mem:/ {print $7}')

    # Get HelixAgent PID if running
    local helixagent_pid=""
    local helixagent_mem=""
    if pgrep -f "helixagent" > /dev/null 2>&1; then
        helixagent_pid=$(pgrep -f "helixagent" | head -1)
        helixagent_mem=$(ps -o rss= -p "$helixagent_pid" 2>/dev/null | awk '{print int($1/1024)}')
        MON_TRACKED_PIDS["helixagent"]="$helixagent_pid"
    fi

    # Get Go processes
    local go_test_pids=$(pgrep -f "go test" 2>/dev/null | tr '\n' ',' | sed 's/,$//')

    cat > "$baseline_file" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "session_id": "$MON_SESSION_ID",
    "system": {
        "total_memory_mb": $total_mem,
        "used_memory_mb": $used_mem,
        "available_memory_mb": $available_mem,
        "cpu_count": $(nproc),
        "load_average": "$(cat /proc/loadavg | cut -d' ' -f1-3)"
    },
    "helixagent": {
        "pid": "${helixagent_pid:-null}",
        "memory_mb": ${helixagent_mem:-null}
    },
    "go_test_pids": "${go_test_pids:-}"
}
EOF

    mon_log "INFO" "Baseline captured: total=${total_mem}MB, available=${available_mem}MB"
}

#===============================================================================
# LOGGING
#===============================================================================

mon_log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S.%3N')

    echo "[$timestamp] [$level] $message" >> "$MON_LOG_DIR/master.log"

    # Route to appropriate log
    case "$level" in
        WARNING)
            echo "[$timestamp] $message" >> "$MON_LOG_DIR/issues/warnings.log"
            MON_WARNINGS_COUNT=$((MON_WARNINGS_COUNT + 1))
            ;;
        ERROR)
            echo "[$timestamp] $message" >> "$MON_LOG_DIR/issues/errors.log"
            MON_ERRORS_COUNT=$((MON_ERRORS_COUNT + 1))
            ;;
        FIX)
            echo "[$timestamp] $message" >> "$MON_LOG_DIR/issues/fixes.log"
            MON_FIXES_COUNT=$((MON_FIXES_COUNT + 1))
            ;;
    esac
}

mon_log_json() {
    local event="$1"
    local data="$2"
    local timestamp=$(date -Iseconds)

    echo "{\"event\":\"$event\",\"timestamp\":\"$timestamp\",$data}" >> "$MON_LOG_DIR/resources/samples.jsonl"
}

#===============================================================================
# COMPONENT LOG COLLECTION
#===============================================================================

mon_collect_helixagent_logs() {
    local dest="$MON_LOG_DIR/components/helixagent.log"

    # Check various log locations
    local log_sources=(
        "/tmp/helixagent.log"
        "$(dirname "$MON_BASE_DIR")/challenges/results/helixagent_challenges.log"
        "/var/log/helixagent.log"
    )

    for src in "${log_sources[@]}"; do
        if [ -f "$src" ]; then
            cat "$src" >> "$dest" 2>/dev/null || true
            mon_log "INFO" "Collected HelixAgent logs from: $src"
        fi
    done

    # Also collect journald logs if available
    if command -v journalctl &> /dev/null; then
        journalctl --user -u helixagent --since "today" --no-pager >> "$dest" 2>/dev/null || true
        journalctl -u helixagent --since "today" --no-pager >> "$dest" 2>/dev/null || true
    fi
}

mon_collect_docker_logs() {
    local dest_dir="$MON_LOG_DIR/components/docker"
    mkdir -p "$dest_dir"

    # Check if docker/podman is available
    local container_cmd=""
    if command -v docker &> /dev/null; then
        container_cmd="docker"
    elif command -v podman &> /dev/null; then
        container_cmd="podman"
    else
        mon_log "INFO" "No container runtime found, skipping docker logs"
        return 0
    fi

    # List of services to collect logs from
    local services=("postgres" "redis" "cognee" "chromadb" "ollama")

    for service in "${services[@]}"; do
        local container_id=$($container_cmd ps -q -f "name=$service" 2>/dev/null | head -1)
        if [ -n "$container_id" ]; then
            $container_cmd logs "$container_id" --since "1h" > "$dest_dir/${service}.log" 2>&1 || true
            mon_log "INFO" "Collected $service container logs"
        fi
    done
}

mon_collect_system_logs() {
    local dest="$MON_LOG_DIR/components/system.log"

    # Collect dmesg (kernel messages)
    dmesg --since "1 hour ago" >> "$dest" 2>/dev/null || true

    # Collect syslog if available
    if [ -f "/var/log/syslog" ]; then
        tail -n 1000 /var/log/syslog >> "$dest" 2>/dev/null || true
    fi

    mon_log "INFO" "Collected system logs"
}

mon_collect_all_logs() {
    mon_log "INFO" "Collecting logs from all components..."

    mon_collect_helixagent_logs
    mon_collect_docker_logs
    mon_collect_system_logs

    mon_log "INFO" "Log collection complete"
}

#===============================================================================
# RESOURCE MONITORING
#===============================================================================

mon_sample_resources() {
    local timestamp=$(date -Iseconds)

    # System memory
    local mem_info=$(free -m | awk '/^Mem:/ {printf "{\"total\":%d,\"used\":%d,\"free\":%d,\"available\":%d}", $2, $3, $4, $7}')

    # System CPU
    local cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print 100 - $8}' | cut -d'.' -f1)
    local load_avg=$(cat /proc/loadavg | cut -d' ' -f1-3 | tr ' ' ',')

    # Disk I/O
    local disk_io=$(cat /proc/diskstats 2>/dev/null | awk 'NR==1 {printf "{\"reads\":%d,\"writes\":%d}", $4, $8}' || echo '{"reads":0,"writes":0}')

    # Network
    local net_rx=$(cat /proc/net/dev 2>/dev/null | awk '/eth0:|eno|enp/ {print $2}' | head -1)
    local net_tx=$(cat /proc/net/dev 2>/dev/null | awk '/eth0:|eno|enp/ {print $10}' | head -1)

    # Process-specific monitoring
    local process_data="[]"
    if [ ${#MON_TRACKED_PIDS[@]} -gt 0 ]; then
        local proc_items=()
        for name in "${!MON_TRACKED_PIDS[@]}"; do
            local pid="${MON_TRACKED_PIDS[$name]}"
            if kill -0 "$pid" 2>/dev/null; then
                local proc_mem=$(ps -o rss= -p "$pid" 2>/dev/null | awk '{print int($1/1024)}' || echo "0")
                local proc_cpu=$(ps -o %cpu= -p "$pid" 2>/dev/null | tr -d ' ' || echo "0")
                local proc_fd=$(ls -l /proc/$pid/fd 2>/dev/null | wc -l || echo "0")
                proc_items+=("{\"name\":\"$name\",\"pid\":$pid,\"mem_mb\":${proc_mem:-0},\"cpu\":${proc_cpu:-0},\"fd_count\":${proc_fd:-0}}")
            fi
        done
        if [ ${#proc_items[@]} -gt 0 ]; then
            process_data="[$(IFS=,; echo "${proc_items[*]}")]"
        fi
    fi

    # Write sample
    mon_log_json "resource_sample" "\"memory\":$mem_info,\"cpu\":{\"usage\":${cpu_usage:-0},\"load\":[$load_avg]},\"disk\":$disk_io,\"network\":{\"rx\":${net_rx:-0},\"tx\":${net_tx:-0}},\"processes\":$process_data"

    # Check thresholds
    local used_mem=$(echo "$mem_info" | grep -o '"used":[0-9]*' | cut -d: -f2)
    local avail_mem=$(echo "$mem_info" | grep -o '"available":[0-9]*' | cut -d: -f2)

    if [ "${cpu_usage:-0}" -gt "$MON_CPU_THRESHOLD" ]; then
        mon_log "WARNING" "High CPU usage detected: ${cpu_usage}%"
    fi

    if [ "${avail_mem:-1000}" -lt 500 ]; then
        mon_log "WARNING" "Low available memory: ${avail_mem}MB"
    fi
}

mon_start_background_monitoring() {
    mon_log "INFO" "Starting background resource monitoring (interval: ${MON_SAMPLE_INTERVAL}s)"

    # Start background monitoring loop
    (
        while true; do
            mon_sample_resources
            sleep "$MON_SAMPLE_INTERVAL"
        done
    ) &

    MON_BACKGROUND_PID=$!
    echo "$MON_BACKGROUND_PID" > "$MON_PID_FILE"

    mon_log "INFO" "Background monitor started (PID: $MON_BACKGROUND_PID)"
}

mon_stop_background_monitoring() {
    if [ -n "$MON_BACKGROUND_PID" ] && kill -0 "$MON_BACKGROUND_PID" 2>/dev/null; then
        kill "$MON_BACKGROUND_PID" 2>/dev/null || true
        wait "$MON_BACKGROUND_PID" 2>/dev/null || true
        mon_log "INFO" "Background monitor stopped"
    fi

    rm -f "$MON_PID_FILE"
}

#===============================================================================
# MEMORY LEAK DETECTION
#===============================================================================

mon_detect_memory_leaks() {
    mon_log "INFO" "Analyzing memory for potential leaks..."

    local baseline_file="$MON_LOG_DIR/resources/memory_baseline.json"
    local leaks_file="$MON_LOG_DIR/issues/memory_leaks.json"
    local leaks_detected=0
    local leak_details=()

    if [ ! -f "$baseline_file" ]; then
        mon_log "WARNING" "No baseline file found for memory leak detection"
        return 1
    fi

    # Get baseline memory for tracked processes
    local baseline_helixagent_mem=$(jq -r '.helixagent.memory_mb // 0' "$baseline_file" 2>/dev/null || echo "0")

    # Get current memory usage
    for name in "${!MON_TRACKED_PIDS[@]}"; do
        local pid="${MON_TRACKED_PIDS[$name]}"
        if kill -0 "$pid" 2>/dev/null; then
            local current_mem=$(ps -o rss= -p "$pid" 2>/dev/null | awk '{print int($1/1024)}')

            # Get baseline for this process
            local baseline_mem=0
            if [ "$name" = "helixagent" ]; then
                baseline_mem=$baseline_helixagent_mem
            fi

            # Check for significant increase
            local mem_increase=$((current_mem - baseline_mem))
            if [ "$mem_increase" -gt "$MON_MEMORY_THRESHOLD" ]; then
                leaks_detected=1
                mon_log "WARNING" "Potential memory leak in $name: baseline=${baseline_mem}MB, current=${current_mem}MB, increase=${mem_increase}MB"
                leak_details+=("{\"process\":\"$name\",\"pid\":$pid,\"baseline_mb\":$baseline_mem,\"current_mb\":$current_mem,\"increase_mb\":$mem_increase}")
            fi
        fi
    done

    # Check Go test processes (transient)
    local go_test_total=$(pgrep -f "go test" 2>/dev/null | while read pid; do
        ps -o rss= -p "$pid" 2>/dev/null
    done | awk '{sum+=$1} END {print int(sum/1024)}')

    if [ "${go_test_total:-0}" -gt 2000 ]; then
        mon_log "WARNING" "Go test processes using significant memory: ${go_test_total}MB"
        leak_details+=("{\"process\":\"go_test_aggregate\",\"current_mb\":$go_test_total}")
        leaks_detected=1
    fi

    # Write leak analysis results
    cat > "$leaks_file" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "leaks_detected": $leaks_detected,
    "threshold_mb": $MON_MEMORY_THRESHOLD,
    "details": [$(IFS=,; echo "${leak_details[*]}")]
}
EOF

    if [ "$leaks_detected" -eq 1 ]; then
        MON_ISSUES_COUNT=$((MON_ISSUES_COUNT + 1))
        return 1
    fi

    mon_log "INFO" "No memory leaks detected"
    return 0
}

#===============================================================================
# LOG ANALYSIS - WARNING/ERROR DETECTION
#===============================================================================

# Known patterns that should trigger investigation
declare -ga MON_ERROR_PATTERNS=(
    "panic:"
    "fatal error:"
    "FATAL"
    "ERROR"
    "runtime error:"
    "nil pointer"
    "index out of range"
    "deadlock"
    "timeout"
    "connection refused"
    "context deadline exceeded"
    "too many open files"
    "out of memory"
    "OOMKilled"
)

declare -ga MON_WARNING_PATTERNS=(
    "WARN"
    "WARNING"
    "deprecated"
    "retry"
    "reconnect"
    "circuit breaker"
    "rate limit"
    "slow query"
    "high latency"
    "token expired"
    "invalid.*format"
)

# Known false positives that should be ignored
declare -ga MON_IGNORE_PATTERNS=(
    "TestError"
    "error.*test"
    "expected.*error"
    "mock.*error"
    "PASS"
    "ok.*dev.helix"
)

mon_analyze_log_file() {
    local log_file="$1"
    local component="$2"
    local results_file="$MON_LOG_DIR/issues/analysis_${component}.json"

    if [ ! -f "$log_file" ]; then
        return 0
    fi

    # Define patterns inline to ensure they're available in subshells
    local error_patterns="panic:|fatal error:|FATAL|ERROR|runtime error:|nil pointer|index out of range|deadlock|timeout|connection refused|context deadline exceeded|too many open files|out of memory|OOMKilled"
    local warning_patterns="WARN|WARNING|deprecated|retry|reconnect|circuit breaker|rate limit|slow query|high latency|token expired|invalid.*format"
    local ignore_patterns="TestError|error.*test|expected.*error|mock.*error|PASS|ok.*dev.helix"

    local errors_found=()
    local warnings_found=()
    local line_num=0
    local local_errors=0
    local local_warnings=0

    while IFS= read -r line; do
        line_num=$((line_num + 1))

        # Skip ignored patterns
        if echo "$line" | grep -qiE "$ignore_patterns"; then
            continue
        fi

        # Check for errors
        if echo "$line" | grep -qiE "$error_patterns"; then
            local escaped_line=$(echo "$line" | sed 's/"/\\"/g' | head -c 500)
            local matched_pattern=$(echo "$line" | grep -oiE "$error_patterns" | head -1)
            errors_found+=("{\"line\":$line_num,\"pattern\":\"$matched_pattern\",\"content\":\"$escaped_line\"}")
            mon_log "ERROR" "[$component:$line_num] $matched_pattern: $(echo "$line" | head -c 200)"
            local_errors=$((local_errors + 1))
            # Note: MON_ERRORS_COUNT is incremented by mon_log
            continue
        fi

        # Check for warnings
        if echo "$line" | grep -qiE "$warning_patterns"; then
            local escaped_line=$(echo "$line" | sed 's/"/\\"/g' | head -c 500)
            local matched_pattern=$(echo "$line" | grep -oiE "$warning_patterns" | head -1)
            warnings_found+=("{\"line\":$line_num,\"pattern\":\"$matched_pattern\",\"content\":\"$escaped_line\"}")
            mon_log "WARNING" "[$component:$line_num] $matched_pattern: $(echo "$line" | head -c 200)"
            local_warnings=$((local_warnings + 1))
            # Note: MON_WARNINGS_COUNT is incremented by mon_log
        fi
    done < "$log_file"

    # Write analysis results
    cat > "$results_file" << EOF
{
    "component": "$component",
    "log_file": "$log_file",
    "timestamp": "$(date -Iseconds)",
    "total_lines": $line_num,
    "errors_count": $local_errors,
    "warnings_count": $local_warnings,
    "errors": [$(IFS=,; echo "${errors_found[*]}")],
    "warnings": [$(IFS=,; echo "${warnings_found[*]}")]
}
EOF

    return $local_errors
}

mon_analyze_all_logs() {
    mon_log "INFO" "Analyzing all collected logs for issues..."

    local total_errors=0
    local total_warnings=0

    # Analyze each component log
    for log_file in "$MON_LOG_DIR"/components/*.log; do
        if [ -f "$log_file" ]; then
            local component=$(basename "$log_file" .log)
            mon_analyze_log_file "$log_file" "$component"
            local errors=$?
            total_errors=$((total_errors + errors))
        fi
    done

    # Also analyze docker logs
    for log_file in "$MON_LOG_DIR"/components/docker/*.log; do
        if [ -f "$log_file" ]; then
            local component="docker_$(basename "$log_file" .log)"
            mon_analyze_log_file "$log_file" "$component"
            local errors=$?
            total_errors=$((total_errors + errors))
        fi
    done

    mon_log "INFO" "Log analysis complete: $total_errors errors found"

    return $total_errors
}

#===============================================================================
# ISSUE INVESTIGATION & FIX TRACKING
#===============================================================================

mon_record_issue() {
    local severity="$1"
    local component="$2"
    local description="$3"
    local details="$4"

    local issue_file="$MON_LOG_DIR/issues/issue_$(date +%Y%m%d_%H%M%S_%N).json"

    cat > "$issue_file" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "severity": "$severity",
    "component": "$component",
    "description": "$description",
    "details": "$details",
    "status": "open"
}
EOF

    MON_ISSUES_COUNT=$((MON_ISSUES_COUNT + 1))
    # Log without going through mon_log to avoid double counting
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S.%3N')
    echo "[$timestamp] [ISSUE] Issue recorded: [$severity] $component - $description" >> "$MON_LOG_DIR/master.log"
}

mon_record_fix() {
    local issue_id="$1"
    local fix_description="$2"
    local test_name="$3"

    local fix_file="$MON_LOG_DIR/issues/fix_$(date +%Y%m%d_%H%M%S_%N).json"

    cat > "$fix_file" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "issue_id": "$issue_id",
    "fix_description": "$fix_description",
    "test_name": "$test_name",
    "status": "applied"
}
EOF

    MON_FIXES_COUNT=$((MON_FIXES_COUNT + 1))
    # Log to fixes log and master log without using mon_log to avoid double counting
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S.%3N')
    echo "[$timestamp] Fix applied: $fix_description (test: $test_name)" >> "$MON_LOG_DIR/issues/fixes.log"
    echo "[$timestamp] [FIX] Fix applied: $fix_description (test: $test_name)" >> "$MON_LOG_DIR/master.log"
}

#===============================================================================
# FILE DESCRIPTOR MONITORING
#===============================================================================

mon_check_file_descriptors() {
    mon_log "INFO" "Checking file descriptor usage..."

    local fd_issues=0

    for name in "${!MON_TRACKED_PIDS[@]}"; do
        local pid="${MON_TRACKED_PIDS[$name]}"
        if kill -0 "$pid" 2>/dev/null; then
            local fd_count=$(ls -l /proc/$pid/fd 2>/dev/null | wc -l || echo "0")
            local fd_limit=$(cat /proc/$pid/limits 2>/dev/null | grep "Max open files" | awk '{print $4}')

            if [ "$fd_count" -gt "$MON_FD_THRESHOLD" ]; then
                mon_log "WARNING" "High file descriptor usage in $name: $fd_count (limit: $fd_limit)"
                fd_issues=$((fd_issues + 1))
            fi

            # Check for FD leaks (compare with baseline if available)
            mon_log "INFO" "$name FD count: $fd_count / $fd_limit"
        fi
    done

    return $fd_issues
}

#===============================================================================
# GOROUTINE LEAK DETECTION (for Go processes)
#===============================================================================

mon_check_goroutine_leaks() {
    local helixagent_pid="${MON_TRACKED_PIDS[helixagent]}"

    if [ -z "$helixagent_pid" ]; then
        return 0
    fi

    # Try to get goroutine count via pprof endpoint
    local goroutine_count=$(curl -s "http://localhost:7061/debug/pprof/goroutine?debug=0" 2>/dev/null | wc -l || echo "0")

    if [ "$goroutine_count" -gt 1000 ]; then
        mon_log "WARNING" "High goroutine count detected: $goroutine_count"

        # Get goroutine profile for analysis
        curl -s "http://localhost:7061/debug/pprof/goroutine?debug=1" > "$MON_LOG_DIR/issues/goroutine_dump.txt" 2>/dev/null || true

        return 1
    fi

    mon_log "INFO" "Goroutine count: $goroutine_count"
    return 0
}

#===============================================================================
# FINALIZATION & CLEANUP
#===============================================================================

mon_finalize() {
    local exit_code="${1:-0}"
    local end_time=$(date +%s)
    local duration=$((end_time - MON_START_TIME))

    mon_log "INFO" "Finalizing monitoring session..."

    # Stop background monitoring
    mon_stop_background_monitoring

    # Collect final logs
    mon_collect_all_logs

    # Run final analysis
    mon_analyze_all_logs
    mon_detect_memory_leaks
    mon_check_file_descriptors
    mon_check_goroutine_leaks

    # Generate summary
    local summary_file="$MON_LOG_DIR/session_summary.json"
    cat > "$summary_file" << EOF
{
    "session_id": "$MON_SESSION_ID",
    "start_time": "$(date -d "@$MON_START_TIME" -Iseconds)",
    "end_time": "$(date -Iseconds)",
    "duration_seconds": $duration,
    "exit_code": $exit_code,
    "issues": {
        "total": $MON_ISSUES_COUNT,
        "errors": $MON_ERRORS_COUNT,
        "warnings": $MON_WARNINGS_COUNT,
        "fixes_applied": $MON_FIXES_COUNT
    },
    "log_directory": "$MON_LOG_DIR",
    "report_directory": "$MON_REPORT_DIR"
}
EOF

    mon_log "INFO" "Session summary written to: $summary_file"

    # Print summary
    echo ""
    echo -e "${MON_CYAN}╔══════════════════════════════════════════════════════════════════╗${MON_NC}"
    echo -e "${MON_CYAN}║              MONITORING SESSION SUMMARY                          ║${MON_NC}"
    echo -e "${MON_CYAN}╠══════════════════════════════════════════════════════════════════╣${MON_NC}"
    echo -e "${MON_CYAN}║${MON_NC} Session ID:  $MON_SESSION_ID"
    echo -e "${MON_CYAN}║${MON_NC} Duration:    ${duration}s"
    echo -e "${MON_CYAN}║${MON_NC} Exit Code:   $exit_code"
    echo -e "${MON_CYAN}╠══════════════════════════════════════════════════════════════════╣${MON_NC}"

    if [ "$MON_ERRORS_COUNT" -gt 0 ]; then
        echo -e "${MON_CYAN}║${MON_NC} ${MON_RED}Errors:      $MON_ERRORS_COUNT${MON_NC}"
    else
        echo -e "${MON_CYAN}║${MON_NC} ${MON_GREEN}Errors:      0${MON_NC}"
    fi

    if [ "$MON_WARNINGS_COUNT" -gt 0 ]; then
        echo -e "${MON_CYAN}║${MON_NC} ${MON_YELLOW}Warnings:    $MON_WARNINGS_COUNT${MON_NC}"
    else
        echo -e "${MON_CYAN}║${MON_NC} ${MON_GREEN}Warnings:    0${MON_NC}"
    fi

    echo -e "${MON_CYAN}║${MON_NC} ${MON_GREEN}Fixes:       $MON_FIXES_COUNT${MON_NC}"
    echo -e "${MON_CYAN}╠══════════════════════════════════════════════════════════════════╣${MON_NC}"
    echo -e "${MON_CYAN}║${MON_NC} Logs:        $MON_LOG_DIR"
    echo -e "${MON_CYAN}║${MON_NC} Reports:     $MON_REPORT_DIR"
    echo -e "${MON_CYAN}╚══════════════════════════════════════════════════════════════════╝${MON_NC}"

    return $exit_code
}

#===============================================================================
# TRACK NEW PROCESS
#===============================================================================

mon_track_process() {
    local name="$1"
    local pid="$2"

    if kill -0 "$pid" 2>/dev/null; then
        MON_TRACKED_PIDS["$name"]="$pid"
        mon_log "INFO" "Now tracking process: $name (PID: $pid)"
    else
        mon_log "WARNING" "Cannot track process $name: PID $pid not running"
    fi
}

#===============================================================================
# EXPORT FUNCTIONS
#===============================================================================

export -f mon_init
export -f mon_log
export -f mon_log_json
export -f mon_collect_all_logs
export -f mon_sample_resources
export -f mon_start_background_monitoring
export -f mon_stop_background_monitoring
export -f mon_detect_memory_leaks
export -f mon_analyze_all_logs
export -f mon_analyze_log_file
export -f mon_record_issue
export -f mon_record_fix
export -f mon_check_file_descriptors
export -f mon_check_goroutine_leaks
export -f mon_finalize
export -f mon_track_process
