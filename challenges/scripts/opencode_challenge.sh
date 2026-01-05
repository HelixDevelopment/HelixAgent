#!/bin/bash
#===============================================================================
# SUPERAGENT OPENCODE CHALLENGE
#===============================================================================
# This challenge validates SuperAgent's integration with OpenCode CLI.
# It runs on top of Main challenge and tests real-world coding assistant usage.
#
# The OpenCode challenge:
# 1. Ensures Main challenge has been run (uses its outputs)
# 2. Generates/validates OpenCode configuration
# 3. Starts SuperAgent server if not running
# 4. Executes OpenCode CLI with a codebase awareness test
# 5. Captures all verbose output and errors
# 6. Analyzes API responses and identifies failures
# 7. Reports on LLM coding capability verification
#
# IMPORTANT: This is a REAL integration test - NO MOCKS!
#
# Usage:
#   ./challenges/scripts/opencode_challenge.sh [options]
#
# Options:
#   --verbose        Enable verbose logging
#   --skip-main      Skip Main challenge dependency check
#   --dry-run        Print commands without executing
#   --help           Show this help message
#
#===============================================================================

set -e

#===============================================================================
# CONFIGURATION
#===============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Timestamps
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
YEAR=$(date +%Y)
MONTH=$(date +%m)
DAY=$(date +%d)

# Directories
RESULTS_BASE="$CHALLENGES_DIR/results/opencode_challenge"
RESULTS_DIR="$RESULTS_BASE/$YEAR/$MONTH/$DAY/$TIMESTAMP"
LOGS_DIR="$RESULTS_DIR/logs"
OUTPUT_DIR="$RESULTS_DIR/results"

# Log files
MAIN_LOG="$LOGS_DIR/opencode_challenge.log"
OPENCODE_LOG="$LOGS_DIR/opencode_verbose.log"
API_LOG="$LOGS_DIR/api_responses.log"
ERROR_LOG="$LOGS_DIR/errors.log"

# Binary paths
SUPERAGENT_BINARY="$PROJECT_ROOT/bin/superagent"
OPENCODE_CONFIG="$HOME/.config/opencode/opencode.json"

# Main challenge latest results
MAIN_CHALLENGE_RESULTS="$CHALLENGES_DIR/results/main_challenge"

# Test configuration
TEST_PROMPT="Do you see my codebase? If yes, tell me what programming language is dominant in this project and list the main directories."
SUPERAGENT_PORT="${SUPERAGENT_PORT:-8080}"
SUPERAGENT_HOST="${SUPERAGENT_HOST:-localhost}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Options
VERBOSE=false
SKIP_MAIN=false
DRY_RUN=false

#===============================================================================
# LOGGING FUNCTIONS
#===============================================================================

log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
    if [ -d "$(dirname "$MAIN_LOG")" ]; then
        echo -e "$msg" | tee -a "$MAIN_LOG"
    else
        echo -e "$msg"
    fi
}

log_info() {
    log "${BLUE}[INFO]${NC} $*"
}

log_success() {
    log "${GREEN}[SUCCESS]${NC} $*"
}

log_warning() {
    log "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    log "${RED}[ERROR]${NC} $*"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$ERROR_LOG" 2>/dev/null || true
}

log_phase() {
    if [ -d "$(dirname "$MAIN_LOG")" ]; then
        echo "" | tee -a "$MAIN_LOG"
    else
        echo ""
    fi
    log "${PURPLE}========================================${NC}"
    log "${PURPLE}  $*${NC}"
    log "${PURPLE}========================================${NC}"
    if [ -d "$(dirname "$MAIN_LOG")" ]; then
        echo "" | tee -a "$MAIN_LOG"
    else
        echo ""
    fi
}

#===============================================================================
# HELPER FUNCTIONS
#===============================================================================

usage() {
    cat << EOF
${GREEN}SuperAgent OpenCode Challenge${NC}

${BLUE}Usage:${NC}
    $0 [options]

${BLUE}Options:${NC}
    ${YELLOW}--verbose${NC}        Enable verbose logging
    ${YELLOW}--skip-main${NC}      Skip Main challenge dependency check
    ${YELLOW}--dry-run${NC}        Print commands without executing
    ${YELLOW}--help${NC}           Show this help message

${BLUE}What this challenge does:${NC}
    1. Validates Main challenge has been run
    2. Generates/validates OpenCode configuration
    3. Starts SuperAgent if not running
    4. Executes OpenCode CLI with codebase awareness test
    5. Captures all verbose output and errors
    6. Analyzes API responses for failures
    7. Reports LLM coding capability results

${BLUE}Test Prompt:${NC}
    "$TEST_PROMPT"

${BLUE}Requirements:${NC}
    - Main challenge completed
    - OpenCode CLI installed
    - SuperAgent built

${BLUE}Output:${NC}
    Results stored in: ${YELLOW}$RESULTS_BASE/<date>/<timestamp>/${NC}

EOF
}

setup_directories() {
    log_info "Creating directory structure..."
    mkdir -p "$LOGS_DIR"
    mkdir -p "$OUTPUT_DIR"
    touch "$ERROR_LOG"
    log_success "Directories created: $RESULTS_DIR"
}

load_environment() {
    log_info "Loading environment variables..."

    if [ -f "$PROJECT_ROOT/.env" ]; then
        set -a
        source "$PROJECT_ROOT/.env"
        set +a
        log_info "Loaded .env from project root"
    fi
}

check_opencode_installed() {
    log_info "Checking OpenCode CLI installation..."

    if command -v opencode &> /dev/null; then
        local version=$(opencode --version 2>&1 || echo "unknown")
        log_success "OpenCode CLI found: $version"
        return 0
    else
        log_error "OpenCode CLI not found in PATH"
        log_info "Install with: npm install -g @anthropic/opencode"
        return 1
    fi
}

check_main_challenge() {
    log_info "Checking Main challenge results..."

    if [ "$SKIP_MAIN" = true ]; then
        log_warning "Skipping Main challenge check (--skip-main)"
        return 0
    fi

    # Find latest main challenge results
    local latest_main=$(find "$MAIN_CHALLENGE_RESULTS" -name "opencode.json" -type f 2>/dev/null | sort -r | head -1)

    if [ -z "$latest_main" ]; then
        log_error "No Main challenge results found"
        log_info "Please run: ./challenges/scripts/main_challenge.sh"
        return 1
    fi

    log_success "Found Main challenge results: $latest_main"

    # Copy/use the OpenCode config from main challenge
    local main_dir=$(dirname "$latest_main")
    echo "$main_dir" > "$OUTPUT_DIR/main_challenge_source.txt"

    return 0
}

check_superagent_running() {
    log_info "Checking if SuperAgent is running..."

    if curl -s "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/health" > /dev/null 2>&1 || \
       curl -s "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/v1/models" > /dev/null 2>&1; then
        log_success "SuperAgent is running on port $SUPERAGENT_PORT"
        return 0
    else
        return 1
    fi
}

start_superagent() {
    log_info "Starting SuperAgent..."

    if [ ! -x "$SUPERAGENT_BINARY" ]; then
        log_warning "SuperAgent binary not found, building..."
        (cd "$PROJECT_ROOT" && make build)
    fi

    # Start SuperAgent in background
    "$SUPERAGENT_BINARY" > "$LOGS_DIR/superagent.log" 2>&1 &
    local pid=$!
    echo $pid > "$OUTPUT_DIR/superagent.pid"

    # Wait for startup
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if check_superagent_running; then
            log_success "SuperAgent started (PID: $pid)"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done

    log_error "SuperAgent failed to start"
    return 1
}

stop_superagent() {
    if [ -f "$OUTPUT_DIR/superagent.pid" ]; then
        local pid=$(cat "$OUTPUT_DIR/superagent.pid")
        if kill -0 "$pid" 2>/dev/null; then
            log_info "Stopping SuperAgent (PID: $pid)..."
            kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
            log_success "SuperAgent stopped"
        fi
        rm -f "$OUTPUT_DIR/superagent.pid"
    fi
}

validate_opencode_config() {
    log_info "Validating OpenCode configuration..."

    if [ ! -f "$OPENCODE_CONFIG" ]; then
        log_warning "OpenCode config not found, generating..."

        # Generate using SuperAgent binary
        set -a && source "$PROJECT_ROOT/.env" && set +a
        "$SUPERAGENT_BINARY" -generate-opencode-config -opencode-output "$OPENCODE_CONFIG"

        # Update with correct provider structure
        python3 - "$OPENCODE_CONFIG" << 'UPDATECONFIG'
import json
import sys
import os

config_path = sys.argv[1]

with open(config_path, 'r') as f:
    config = json.load(f)

# Get API key from environment
api_key = os.environ.get('SUPERAGENT_API_KEY', '')

# Update to use openai-compatible provider
config = {
    "$schema": "https://opencode.ai/config.json",
    "provider": {
        "superagent": {
            "npm": "@ai-sdk/openai-compatible",
            "name": "SuperAgent AI Debate",
            "options": {
                "baseURL": "http://localhost:8080/v1",
                "apiKey": api_key
            },
            "models": {
                "superagent-debate": {
                    "name": "SuperAgent Debate Ensemble",
                    "attachments": True,
                    "reasoning": True
                }
            }
        }
    },
    "agent": {
        "model": {
            "provider": "superagent",
            "model": "superagent-debate"
        }
    }
}

with open(config_path, 'w') as f:
    json.dump(config, f, indent=2)

print("OpenCode config updated successfully")
UPDATECONFIG
    fi

    # Validate the config
    if [ -f "$OPENCODE_CONFIG" ]; then
        if python3 -c "import json; json.load(open('$OPENCODE_CONFIG'))" 2>/dev/null; then
            log_success "OpenCode configuration is valid JSON"
            cp "$OPENCODE_CONFIG" "$OUTPUT_DIR/opencode_config_used.json"
            return 0
        else
            log_error "OpenCode configuration is invalid JSON"
            return 1
        fi
    fi

    return 1
}

#===============================================================================
# PHASE 1: PREREQUISITES CHECK
#===============================================================================

phase1_prerequisites() {
    log_phase "PHASE 1: Prerequisites Check"

    local errors=0

    # Check OpenCode CLI
    if ! check_opencode_installed; then
        errors=$((errors + 1))
    fi

    # Check Main challenge
    if ! check_main_challenge; then
        errors=$((errors + 1))
    fi

    # Check/Start SuperAgent
    if ! check_superagent_running; then
        if ! start_superagent; then
            errors=$((errors + 1))
        fi
    fi

    # Validate OpenCode config
    if ! validate_opencode_config; then
        errors=$((errors + 1))
    fi

    if [ $errors -gt 0 ]; then
        log_error "Prerequisites check failed with $errors errors"
        return 1
    fi

    log_success "All prerequisites satisfied"
}

#===============================================================================
# PHASE 2: API CONNECTIVITY TEST
#===============================================================================

phase2_api_test() {
    log_phase "PHASE 2: API Connectivity Test"

    log_info "Testing SuperAgent API endpoints..."

    local api_results="$OUTPUT_DIR/api_test_results.json"

    # Test /v1/models endpoint
    log_info "Testing /v1/models..."
    local models_response=$(curl -s -w "\n%{http_code}" "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/v1/models" 2>&1)
    local models_body=$(echo "$models_response" | head -n -1)
    local models_status=$(echo "$models_response" | tail -n 1)

    echo "Models endpoint response:" >> "$API_LOG"
    echo "$models_body" >> "$API_LOG"
    echo "Status: $models_status" >> "$API_LOG"
    echo "" >> "$API_LOG"

    if [ "$models_status" = "200" ]; then
        log_success "/v1/models returned 200 OK"

        # Verify superagent-debate model is present
        if echo "$models_body" | grep -q "superagent-debate"; then
            log_success "superagent-debate model found"
        else
            log_error "superagent-debate model NOT found in response"
            echo "ERROR: superagent-debate model missing from /v1/models" >> "$ERROR_LOG"
        fi
    else
        log_error "/v1/models returned status $models_status"
        echo "ERROR: /v1/models returned $models_status" >> "$ERROR_LOG"
    fi

    # Test /v1/chat/completions endpoint with simple request
    log_info "Testing /v1/chat/completions..."

    local chat_request='{"model":"superagent-debate","messages":[{"role":"user","content":"Say hello"}],"max_tokens":50}'
    local chat_response=$(curl -s -w "\n%{http_code}" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $SUPERAGENT_API_KEY" \
        -d "$chat_request" \
        "http://$SUPERAGENT_HOST:$SUPERAGENT_PORT/v1/chat/completions" 2>&1)

    local chat_body=$(echo "$chat_response" | head -n -1)
    local chat_status=$(echo "$chat_response" | tail -n 1)

    echo "Chat completions response:" >> "$API_LOG"
    echo "$chat_body" >> "$API_LOG"
    echo "Status: $chat_status" >> "$API_LOG"
    echo "" >> "$API_LOG"

    if [ "$chat_status" = "200" ]; then
        log_success "/v1/chat/completions returned 200 OK"
    else
        log_error "/v1/chat/completions returned status $chat_status"
        echo "ERROR: /v1/chat/completions returned $chat_status" >> "$ERROR_LOG"
        echo "Response body: $chat_body" >> "$ERROR_LOG"
    fi

    # Generate API test summary
    cat > "$api_results" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "endpoints_tested": [
        {
            "endpoint": "/v1/models",
            "status": $models_status,
            "success": $([ "$models_status" = "200" ] && echo "true" || echo "false")
        },
        {
            "endpoint": "/v1/chat/completions",
            "status": $chat_status,
            "success": $([ "$chat_status" = "200" ] && echo "true" || echo "false")
        }
    ],
    "superagent_host": "$SUPERAGENT_HOST",
    "superagent_port": "$SUPERAGENT_PORT"
}
EOF

    log_success "API connectivity test completed"
}

#===============================================================================
# PHASE 3: OPENCODE CLI EXECUTION
#===============================================================================

phase3_opencode_execution() {
    log_phase "PHASE 3: OpenCode CLI Execution"

    log_info "Executing OpenCode CLI with codebase awareness test..."
    log_info "Test prompt: $TEST_PROMPT"

    local opencode_result="$OUTPUT_DIR/opencode_result.json"
    local opencode_exit_code=0

    # Change to project directory for codebase context
    cd "$PROJECT_ROOT"

    # Execute OpenCode in verbose mode with timeout
    # Using --print to output without interactive mode
    log_info "Running OpenCode CLI (this may take a moment)..."

    # Create a script to run opencode with proper handling
    cat > "$LOGS_DIR/run_opencode.sh" << 'RUNSCRIPT'
#!/bin/bash
export OPENCODE_LOG_LEVEL=DEBUG

# Run opencode with the prompt using 'run' command, capturing all output
# --print-logs enables verbose logging to stderr
timeout 120 opencode run --print-logs --log-level DEBUG "$1" 2>&1
exit $?
RUNSCRIPT
    chmod +x "$LOGS_DIR/run_opencode.sh"

    # Execute and capture output
    set +e
    "$LOGS_DIR/run_opencode.sh" "$TEST_PROMPT" > "$OPENCODE_LOG" 2>&1
    opencode_exit_code=$?
    set -e

    log_info "OpenCode exit code: $opencode_exit_code"

    # Analyze the output
    local output_lines=$(wc -l < "$OPENCODE_LOG" | tr -d ' ')
    local error_lines=$(grep -ci "error\|fail\|exception" "$OPENCODE_LOG" 2>/dev/null | tr -d ' ' || echo "0")
    local api_errors=$(grep -ci "api.*error\|status.*[45][0-9][0-9]" "$OPENCODE_LOG" 2>/dev/null | tr -d ' ' || echo "0")

    # Ensure numeric values
    output_lines=${output_lines:-0}
    error_lines=${error_lines:-0}
    api_errors=${api_errors:-0}

    log_info "Output lines: $output_lines"
    log_info "Error mentions: $error_lines"
    log_info "API errors: $api_errors"

    # Extract any API error details
    if [ "$api_errors" -gt 0 ] 2>/dev/null; then
        log_warning "API errors detected in OpenCode output"
        grep -i "api.*error\|status.*[45][0-9][0-9]\|failed.*request" "$OPENCODE_LOG" >> "$ERROR_LOG" 2>/dev/null || true
    fi

    # Check if the response mentions the codebase
    local codebase_mentioned=false
    if grep -qi "go\|golang\|codebase\|project\|directory\|internal\|cmd" "$OPENCODE_LOG"; then
        codebase_mentioned=true
        log_success "Response appears to reference the codebase"
    else
        log_warning "Response may not have detected the codebase"
    fi

    # Generate result summary
    cat > "$opencode_result" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "test_prompt": "$TEST_PROMPT",
    "exit_code": $opencode_exit_code,
    "output_lines": $output_lines,
    "error_mentions": $error_lines,
    "api_errors": $api_errors,
    "codebase_detected": $codebase_mentioned,
    "success": $([ $opencode_exit_code -eq 0 ] && [ "$api_errors" -eq 0 ] && echo "true" || echo "false"),
    "log_file": "$OPENCODE_LOG"
}
EOF

    # Show first part of output
    log_info "OpenCode output (first 50 lines):"
    head -50 "$OPENCODE_LOG" | while read line; do
        echo "  $line"
    done

    if [ $opencode_exit_code -ne 0 ] || [ "$api_errors" -gt 0 ]; then
        log_error "OpenCode execution had issues"
        return 1
    fi

    log_success "OpenCode CLI execution completed"
}

#===============================================================================
# PHASE 4: ERROR ANALYSIS
#===============================================================================

phase4_error_analysis() {
    log_phase "PHASE 4: Error Analysis"

    log_info "Analyzing errors and API responses..."

    local error_analysis="$OUTPUT_DIR/error_analysis.json"
    local errors_found=0
    local error_categories=()

    # Check error log
    if [ -s "$ERROR_LOG" ]; then
        errors_found=$(wc -l < "$ERROR_LOG")
        log_warning "Found $errors_found error entries"

        # Categorize errors
        if grep -qi "provider\|ensemble" "$ERROR_LOG"; then
            error_categories+=("provider_errors")
        fi
        if grep -qi "api.*key\|auth\|unauthorized" "$ERROR_LOG"; then
            error_categories+=("authentication_errors")
        fi
        if grep -qi "timeout\|connection" "$ERROR_LOG"; then
            error_categories+=("connection_errors")
        fi
        if grep -qi "model.*not.*found\|invalid.*model" "$ERROR_LOG"; then
            error_categories+=("model_errors")
        fi
        if grep -qi "rate.*limit\|too.*many" "$ERROR_LOG"; then
            error_categories+=("rate_limit_errors")
        fi
    else
        log_success "No errors found in error log"
    fi

    # Check API log for issues
    local api_issues=0
    if [ -f "$API_LOG" ]; then
        api_issues=$(grep -ci "error\|fail\|[45][0-9][0-9]" "$API_LOG" 2>/dev/null || echo "0")
        if [ "$api_issues" -gt 0 ]; then
            log_warning "Found $api_issues potential issues in API log"
        fi
    fi

    # Check OpenCode log for specific issues
    local opencode_issues=""
    if [ -f "$OPENCODE_LOG" ]; then
        # Check for common issues
        if grep -qi "no.*provider\|provider.*not.*configured" "$OPENCODE_LOG"; then
            opencode_issues="$opencode_issues,provider_not_configured"
        fi
        if grep -qi "invalid.*api.*key\|api.*key.*missing" "$OPENCODE_LOG"; then
            opencode_issues="$opencode_issues,invalid_api_key"
        fi
        if grep -qi "model.*not.*available\|model.*not.*found" "$OPENCODE_LOG"; then
            opencode_issues="$opencode_issues,model_not_available"
        fi
        if grep -qi "ensemble.*fail\|no.*response.*from.*provider" "$OPENCODE_LOG"; then
            opencode_issues="$opencode_issues,ensemble_failure"
        fi
    fi

    # Generate error analysis report
    cat > "$error_analysis" << EOF
{
    "timestamp": "$(date -Iseconds)",
    "total_errors": $errors_found,
    "api_issues": $api_issues,
    "error_categories": [$(printf '"%s",' "${error_categories[@]}" | sed 's/,$//')],
    "opencode_issues": "$(echo $opencode_issues | sed 's/^,//')",
    "error_log": "$ERROR_LOG",
    "api_log": "$API_LOG",
    "recommendations": []
}
EOF

    # Generate recommendations based on errors
    if [ ${#error_categories[@]} -gt 0 ] || [ -n "$opencode_issues" ]; then
        log_info "Generating recommendations..."

        python3 - "$error_analysis" << 'RECOMMENDATIONS'
import json
import sys

analysis_file = sys.argv[1]

with open(analysis_file, 'r') as f:
    analysis = json.load(f)

recommendations = []

categories = analysis.get('error_categories', [])
issues = analysis.get('opencode_issues', '')

if 'provider_errors' in categories or 'ensemble_failure' in issues:
    recommendations.append({
        "issue": "Provider/Ensemble Errors",
        "recommendation": "Verify LLM providers are properly configured and API keys are valid",
        "action": "Run LLMsVerifier to validate provider connectivity"
    })

if 'authentication_errors' in categories or 'invalid_api_key' in issues:
    recommendations.append({
        "issue": "Authentication Errors",
        "recommendation": "Check SUPERAGENT_API_KEY is set correctly in .env",
        "action": "Regenerate API key using: ./bin/superagent -generate-api-key -api-key-env-file .env"
    })

if 'model_errors' in categories or 'model_not_available' in issues:
    recommendations.append({
        "issue": "Model Not Found",
        "recommendation": "Ensure superagent-debate model is registered",
        "action": "Verify /v1/models returns superagent-debate"
    })

if 'provider_not_configured' in issues:
    recommendations.append({
        "issue": "Provider Not Configured",
        "recommendation": "Update OpenCode config with correct provider settings",
        "action": "Regenerate config: ./bin/superagent -generate-opencode-config"
    })

analysis['recommendations'] = recommendations

with open(analysis_file, 'w') as f:
    json.dump(analysis, f, indent=2)

for rec in recommendations:
    print(f"RECOMMENDATION: {rec['issue']}")
    print(f"  -> {rec['recommendation']}")
    print(f"  Action: {rec['action']}")
    print()
RECOMMENDATIONS
    fi

    log_success "Error analysis completed"
}

#===============================================================================
# PHASE 5: RESULTS SUMMARY
#===============================================================================

phase5_summary() {
    log_phase "PHASE 5: Results Summary"

    local summary_file="$OUTPUT_DIR/challenge_summary.json"
    local report_file="$OUTPUT_DIR/opencode_challenge_report.md"

    # Gather all results
    local api_success=false
    local opencode_success=false
    local overall_success=false

    if [ -f "$OUTPUT_DIR/api_test_results.json" ]; then
        api_success=$(python3 -c "import json; d=json.load(open('$OUTPUT_DIR/api_test_results.json')); print('true' if all(e['success'] for e in d['endpoints_tested']) else 'false')" 2>/dev/null || echo "false")
    fi

    if [ -f "$OUTPUT_DIR/opencode_result.json" ]; then
        opencode_success=$(python3 -c "import json; d=json.load(open('$OUTPUT_DIR/opencode_result.json')); print(str(d.get('success', False)).lower())" 2>/dev/null || echo "false")
    fi

    if [ "$api_success" = "true" ] && [ "$opencode_success" = "true" ]; then
        overall_success=true
    fi

    # Generate summary JSON
    cat > "$summary_file" << EOF
{
    "challenge": "OpenCode Integration",
    "timestamp": "$(date -Iseconds)",
    "results": {
        "api_test": $api_success,
        "opencode_execution": $opencode_success,
        "overall": $overall_success
    },
    "results_directory": "$RESULTS_DIR",
    "logs": {
        "main": "$MAIN_LOG",
        "opencode": "$OPENCODE_LOG",
        "api": "$API_LOG",
        "errors": "$ERROR_LOG"
    }
}
EOF

    # Generate markdown report
    cat > "$report_file" << EOF
# OpenCode Integration Challenge Report

**Generated**: $(date '+%Y-%m-%d %H:%M:%S')
**Status**: $([ "$overall_success" = "true" ] && echo "PASSED" || echo "NEEDS ATTENTION")

## Test Summary

| Test | Result |
|------|--------|
| API Connectivity | $([ "$api_success" = "true" ] && echo "PASSED" || echo "FAILED") |
| OpenCode Execution | $([ "$opencode_success" = "true" ] && echo "PASSED" || echo "FAILED") |
| **Overall** | $([ "$overall_success" = "true" ] && echo "**PASSED**" || echo "**FAILED**") |

## Test Prompt

\`\`\`
$TEST_PROMPT
\`\`\`

## Results Location

\`\`\`
$RESULTS_DIR/
├── logs/
│   ├── opencode_challenge.log
│   ├── opencode_verbose.log
│   ├── api_responses.log
│   └── errors.log
└── results/
    ├── api_test_results.json
    ├── opencode_result.json
    ├── error_analysis.json
    └── challenge_summary.json
\`\`\`

## Error Analysis

$(cat "$OUTPUT_DIR/error_analysis.json" 2>/dev/null | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    print(f\"Total errors: {d.get('total_errors', 0)}\")
    print(f\"API issues: {d.get('api_issues', 0)}\")
    if d.get('recommendations'):
        print('\n### Recommendations\n')
        for rec in d['recommendations']:
            print(f\"- **{rec['issue']}**: {rec['recommendation']}\")
except:
    print('No analysis available')
" 2>/dev/null || echo "See error_analysis.json for details")

## Next Steps

$(if [ "$overall_success" = "true" ]; then
    echo "Challenge completed successfully. SuperAgent is working with OpenCode."
else
    echo "1. Review error logs in the results directory"
    echo "2. Check API connectivity and authentication"
    echo "3. Verify LLM providers are properly configured"
    echo "4. Re-run the challenge after fixes"
fi)

---
*Generated by SuperAgent OpenCode Challenge*
EOF

    # Print summary
    echo ""
    log_info "=========================================="
    log_info "  CHALLENGE SUMMARY"
    log_info "=========================================="
    echo ""
    log_info "API Test:         $([ "$api_success" = "true" ] && echo "${GREEN}PASSED${NC}" || echo "${RED}FAILED${NC}")"
    log_info "OpenCode Test:    $([ "$opencode_success" = "true" ] && echo "${GREEN}PASSED${NC}" || echo "${RED}FAILED${NC}")"
    log_info "Overall:          $([ "$overall_success" = "true" ] && echo "${GREEN}PASSED${NC}" || echo "${RED}FAILED${NC}")"
    echo ""
    log_info "Results: $RESULTS_DIR"
    log_info "Report: $report_file"

    if [ "$overall_success" = "true" ]; then
        log_success "OpenCode Challenge PASSED"
        return 0
    else
        log_error "OpenCode Challenge FAILED - see error analysis"
        return 1
    fi
}

#===============================================================================
# CLEANUP
#===============================================================================

cleanup() {
    log_info "Cleaning up..."
    # Don't stop SuperAgent - user may want it running
    # stop_superagent
}

trap cleanup EXIT

#===============================================================================
# MAIN EXECUTION
#===============================================================================

main() {
    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --verbose)
                VERBOSE=true
                ;;
            --skip-main)
                SKIP_MAIN=true
                ;;
            --dry-run)
                DRY_RUN=true
                ;;
            --help|-h)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
        shift
    done

    START_TIME=$(date '+%Y-%m-%d %H:%M:%S')

    # Setup
    setup_directories
    load_environment

    log_phase "SUPERAGENT OPENCODE CHALLENGE"
    log_info "Start time: $START_TIME"
    log_info "Results directory: $RESULTS_DIR"

    # Execute phases
    phase1_prerequisites
    phase2_api_test
    phase3_opencode_execution
    phase4_error_analysis
    phase5_summary
}

main "$@"
