#!/bin/bash
# HelixAgent Debate Group Verification Script
# Purpose: Verify that all LLM providers in the debate group are properly configured
#          and contributing to the ensemble
#
# Usage: ./verify_debate_group.sh [--verbose] [--json]
#
# This script tests:
# 1. Provider registration
# 2. Individual provider health
# 3. API connectivity for each provider
# 4. Ensemble functionality
# 5. Generates detailed reports

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHALLENGES_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$CHALLENGES_DIR")"

# Configuration
HELIXAGENT_URL="${HELIXAGENT_URL:-http://localhost:7061}"
TIMEOUT_SECONDS=30
VERBOSE=false
JSON_OUTPUT=false

# Results
declare -A PROVIDER_STATUS
declare -A PROVIDER_ERROR
PROVIDERS_WORKING=0
PROVIDERS_RATE_LIMITED=0
PROVIDERS_AUTH_FAILED=0
PROVIDERS_FAILED=0
TOTAL_PROVIDERS=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Print functions
print_header() {
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  HelixAgent Debate Group Verification${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
}

print_section() {
    echo -e "${BLUE}[$1]${NC} $2"
}

print_success() {
    echo -e "  ${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "  ${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "  ${RED}✗${NC} $1"
}

print_info() {
    echo -e "  ${CYAN}ℹ${NC} $1"
}

# Parse arguments
parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            -v|--verbose)
                VERBOSE=true
                ;;
            -j|--json)
                JSON_OUTPUT=true
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
        shift
    done
}

usage() {
    cat << EOF
HelixAgent Debate Group Verification

Usage: $0 [options]

Options:
  -v, --verbose    Enable verbose output
  -j, --json       Output results in JSON format
  -h, --help       Show this help message

Environment Variables:
  HELIXAGENT_URL   HelixAgent server URL (default: http://localhost:7061)

Examples:
  $0                    # Run verification with default settings
  $0 --verbose          # Run with detailed output
  $0 --json             # Output JSON results for automation
EOF
}

# Check if HelixAgent server is running
check_server() {
    print_section "1" "Checking HelixAgent Server"

    local health_response
    health_response=$(curl -s --max-time 5 "${HELIXAGENT_URL}/health" 2>/dev/null)

    if echo "$health_response" | grep -q "healthy"; then
        print_success "Server is healthy at ${HELIXAGENT_URL}"
        return 0
    else
        print_error "Server not responding at ${HELIXAGENT_URL}"
        return 1
    fi
}

# Get registered providers
get_providers() {
    print_section "2" "Getting Registered Providers"

    local providers_response
    providers_response=$(curl -s --max-time 10 "${HELIXAGENT_URL}/v1/providers" 2>/dev/null)

    if [ -z "$providers_response" ]; then
        print_error "Failed to get providers list"
        return 1
    fi

    # Parse provider count
    TOTAL_PROVIDERS=$(echo "$providers_response" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null || echo 0)

    print_info "Found ${TOTAL_PROVIDERS} registered providers"

    # List providers
    if [ "$VERBOSE" = true ]; then
        echo "$providers_response" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for p in d.get('providers', []):
    name = p.get('name', 'unknown')
    models = ', '.join(p.get('supported_models', [])[:2])
    cognee = 'Yes' if p.get('metadata', {}).get('cognee_enhanced') == 'true' else 'No'
    print(f'    - {name}: Cognee={cognee}, Models={models}...')
" 2>/dev/null
    fi

    echo "$providers_response"
}

# Test individual provider
test_provider() {
    local provider=$1
    local test_message="${2:-Say OK}"

    local start_time=$(date +%s%N)
    local response
    response=$(curl -s --max-time ${TIMEOUT_SECONDS} -X POST "${HELIXAGENT_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{\"model\": \"helixagent-debate\", \"messages\": [{\"role\": \"user\", \"content\": \"${test_message}\"}], \"force_provider\": \"${provider}\"}" 2>/dev/null)
    local end_time=$(date +%s%N)
    local duration=$(( (end_time - start_time) / 1000000 ))

    if [ -z "$response" ]; then
        PROVIDER_STATUS[$provider]="timeout"
        PROVIDER_ERROR[$provider]="Request timed out after ${TIMEOUT_SECONDS}s"
        ((PROVIDERS_FAILED++))
        return 1
    fi

    # Check for success
    if echo "$response" | grep -q '"choices"'; then
        PROVIDER_STATUS[$provider]="working"
        PROVIDER_ERROR[$provider]=""
        ((PROVIDERS_WORKING++))
        return 0
    fi

    # Parse error
    local error_message
    error_message=$(echo "$response" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    err = d.get('error', {})
    msg = err.get('message', 'Unknown error')
    code = err.get('code', '')
    print(f'{code}: {msg}')
except:
    print('Parse error')
" 2>/dev/null)

    # Categorize error
    if echo "$error_message" | grep -qi "429\|quota\|rate"; then
        PROVIDER_STATUS[$provider]="rate_limited"
        PROVIDER_ERROR[$provider]="Rate limited (quota exceeded)"
        ((PROVIDERS_RATE_LIMITED++))
    elif echo "$error_message" | grep -qi "401\|unauthorized\|not found\|invalid"; then
        PROVIDER_STATUS[$provider]="auth_failed"
        PROVIDER_ERROR[$provider]="Authentication failed (invalid API key)"
        ((PROVIDERS_AUTH_FAILED++))
    else
        PROVIDER_STATUS[$provider]="failed"
        PROVIDER_ERROR[$provider]="$error_message"
        ((PROVIDERS_FAILED++))
    fi

    return 1
}

# Test all providers
test_all_providers() {
    print_section "3" "Testing Individual Providers"

    local providers=("deepseek" "gemini" "openrouter")

    for provider in "${providers[@]}"; do
        echo -n "  Testing ${provider}... "

        if test_provider "$provider" "Say hello"; then
            echo -e "${GREEN}WORKING${NC}"
        else
            case "${PROVIDER_STATUS[$provider]}" in
                rate_limited)
                    echo -e "${YELLOW}RATE LIMITED${NC}"
                    ;;
                auth_failed)
                    echo -e "${RED}AUTH FAILED${NC}"
                    ;;
                timeout)
                    echo -e "${RED}TIMEOUT${NC}"
                    ;;
                *)
                    echo -e "${RED}FAILED${NC}"
                    ;;
            esac

            if [ "$VERBOSE" = true ] && [ -n "${PROVIDER_ERROR[$provider]}" ]; then
                echo "    Error: ${PROVIDER_ERROR[$provider]}"
            fi
        fi
    done
}

# Test ensemble functionality
test_ensemble() {
    print_section "4" "Testing Ensemble Functionality"

    local response
    response=$(curl -s --max-time ${TIMEOUT_SECONDS} -X POST "${HELIXAGENT_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model": "helixagent-debate", "messages": [{"role": "user", "content": "What is 2+2? Answer briefly."}]}' 2>/dev/null)

    if echo "$response" | grep -q '"choices"'; then
        print_success "Ensemble is operational"

        # Extract details
        local model=$(echo "$response" | python3 -c "import sys,json; print(json.load(sys.stdin).get('model','unknown'))" 2>/dev/null)
        local fingerprint=$(echo "$response" | python3 -c "import sys,json; print(json.load(sys.stdin).get('system_fingerprint','unknown'))" 2>/dev/null)
        local content=$(echo "$response" | python3 -c "import sys,json; print(json.load(sys.stdin)['choices'][0]['message']['content'][:50])" 2>/dev/null)

        print_info "Model: ${model}"
        print_info "Fingerprint: ${fingerprint}"

        if [ "$VERBOSE" = true ]; then
            print_info "Response: ${content}..."
        fi

        return 0
    else
        print_error "Ensemble failed"
        return 1
    fi
}

# Test direct API connectivity for each provider
test_direct_apis() {
    print_section "5" "Testing Direct API Connectivity"

    # DeepSeek
    echo -n "  DeepSeek API: "
    local deepseek_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "https://api.deepseek.com/v1/models" -H "Authorization: Bearer ${DEEPSEEK_API_KEY:-sk-test}" 2>/dev/null)
    if [ "$deepseek_status" = "200" ] || [ "$deepseek_status" = "401" ]; then
        echo -e "${GREEN}Reachable${NC} (HTTP ${deepseek_status})"
    else
        echo -e "${RED}Unreachable${NC} (HTTP ${deepseek_status})"
    fi

    # Gemini
    echo -n "  Gemini API: "
    local gemini_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "https://generativelanguage.googleapis.com/v1beta/models" 2>/dev/null)
    if [ "$gemini_status" = "200" ] || [ "$gemini_status" = "403" ]; then
        echo -e "${GREEN}Reachable${NC} (HTTP ${gemini_status})"
    else
        echo -e "${RED}Unreachable${NC} (HTTP ${gemini_status})"
    fi

    # OpenRouter
    echo -n "  OpenRouter API: "
    local openrouter_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "https://openrouter.ai/api/v1/models" 2>/dev/null)
    if [ "$openrouter_status" = "200" ]; then
        echo -e "${GREEN}Reachable${NC} (HTTP ${openrouter_status})"
    else
        echo -e "${RED}Unreachable${NC} (HTTP ${openrouter_status})"
    fi
}

# Generate summary
generate_summary() {
    echo ""
    print_section "6" "Summary"
    echo ""

    echo "  Provider Status:"
    echo "  ─────────────────────────────────────"
    printf "  %-15s %-15s %s\n" "Provider" "Status" "Details"
    echo "  ─────────────────────────────────────"

    for provider in deepseek gemini openrouter; do
        local status="${PROVIDER_STATUS[$provider]:-not_tested}"
        local error="${PROVIDER_ERROR[$provider]:-N/A}"

        case "$status" in
            working)
                printf "  ${GREEN}%-15s${NC} %-15s %s\n" "$provider" "WORKING" "-"
                ;;
            rate_limited)
                printf "  ${YELLOW}%-15s${NC} %-15s %s\n" "$provider" "RATE LIMITED" "(temporary)"
                ;;
            auth_failed)
                printf "  ${RED}%-15s${NC} %-15s %s\n" "$provider" "AUTH FAILED" "(check API key)"
                ;;
            *)
                printf "  ${RED}%-15s${NC} %-15s %s\n" "$provider" "FAILED" "$error"
                ;;
        esac
    done

    echo "  ─────────────────────────────────────"
    echo ""

    echo "  Statistics:"
    echo "  ─────────────────────────────────────"
    echo "    Total Providers: ${TOTAL_PROVIDERS}"
    echo "    Working:         ${PROVIDERS_WORKING}"
    echo "    Rate Limited:    ${PROVIDERS_RATE_LIMITED}"
    echo "    Auth Failed:     ${PROVIDERS_AUTH_FAILED}"
    echo "    Failed:          ${PROVIDERS_FAILED}"
    echo ""

    # Ensemble status
    if [ $PROVIDERS_WORKING -gt 0 ]; then
        echo -e "  ${GREEN}Ensemble Status: OPERATIONAL${NC}"
        echo "    (Using ${PROVIDERS_WORKING} working provider(s))"
    else
        echo -e "  ${RED}Ensemble Status: DEGRADED${NC}"
        echo "    (No working providers available)"
    fi

    echo ""

    # Recommendations
    if [ $PROVIDERS_RATE_LIMITED -gt 0 ]; then
        echo "  Recommendations:"
        echo "    - Rate limited providers will recover after quota resets"
        echo "    - Consider upgrading API plans for higher quotas"
    fi

    if [ $PROVIDERS_AUTH_FAILED -gt 0 ]; then
        echo "  Recommendations:"
        echo "    - Check API keys for providers with auth failures"
        echo "    - Verify API keys are active and have proper permissions"
    fi
}

# Generate JSON output
generate_json() {
    cat << EOF
{
  "timestamp": "$(date -Iseconds)",
  "helixagent_url": "${HELIXAGENT_URL}",
  "providers": {
    "total": ${TOTAL_PROVIDERS},
    "working": ${PROVIDERS_WORKING},
    "rate_limited": ${PROVIDERS_RATE_LIMITED},
    "auth_failed": ${PROVIDERS_AUTH_FAILED},
    "failed": ${PROVIDERS_FAILED}
  },
  "status": {
    "deepseek": {
      "status": "${PROVIDER_STATUS[deepseek]:-not_tested}",
      "error": "${PROVIDER_ERROR[deepseek]:-}"
    },
    "gemini": {
      "status": "${PROVIDER_STATUS[gemini]:-not_tested}",
      "error": "${PROVIDER_ERROR[gemini]:-}"
    },
    "openrouter": {
      "status": "${PROVIDER_STATUS[openrouter]:-not_tested}",
      "error": "${PROVIDER_ERROR[openrouter]:-}"
    }
  },
  "ensemble_operational": $([ $PROVIDERS_WORKING -gt 0 ] && echo "true" || echo "false"),
  "all_providers_working": $([ $PROVIDERS_WORKING -eq $TOTAL_PROVIDERS ] && echo "true" || echo "false")
}
EOF
}

# Write results to file
write_results() {
    local results_dir="$CHALLENGES_DIR/results/debate_group_verification"
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local result_file="$results_dir/${timestamp}_verification.txt"
    local json_file="$results_dir/${timestamp}_verification.json"

    mkdir -p "$results_dir"

    # Write text report
    {
        echo "HelixAgent Debate Group Verification Report"
        echo "Generated: $(date)"
        echo "Server: ${HELIXAGENT_URL}"
        echo ""
        echo "Provider Status:"
        for provider in deepseek gemini openrouter; do
            echo "  ${provider}: ${PROVIDER_STATUS[$provider]:-not_tested}"
            [ -n "${PROVIDER_ERROR[$provider]}" ] && echo "    Error: ${PROVIDER_ERROR[$provider]}"
        done
        echo ""
        echo "Statistics:"
        echo "  Working: ${PROVIDERS_WORKING}"
        echo "  Rate Limited: ${PROVIDERS_RATE_LIMITED}"
        echo "  Auth Failed: ${PROVIDERS_AUTH_FAILED}"
        echo "  Failed: ${PROVIDERS_FAILED}"
        echo ""
        echo "Ensemble Operational: $([ $PROVIDERS_WORKING -gt 0 ] && echo "Yes" || echo "No")"
    } > "$result_file"

    # Write JSON
    generate_json > "$json_file"

    print_info "Results written to: $result_file"
}

# Main execution
main() {
    parse_args "$@"

    if [ "$JSON_OUTPUT" = false ]; then
        print_header
    fi

    # Run checks
    if ! check_server; then
        echo "Server check failed. Exiting."
        exit 1
    fi
    echo ""

    get_providers > /dev/null
    echo ""

    test_all_providers
    echo ""

    test_ensemble
    echo ""

    if [ "$VERBOSE" = true ]; then
        test_direct_apis
        echo ""
    fi

    if [ "$JSON_OUTPUT" = true ]; then
        generate_json
    else
        generate_summary
        write_results
    fi

    echo ""
    echo -e "${CYAN}========================================${NC}"

    # Exit code based on results
    if [ $PROVIDERS_WORKING -eq $TOTAL_PROVIDERS ]; then
        exit 0  # All providers working
    elif [ $PROVIDERS_WORKING -gt 0 ]; then
        exit 0  # At least one provider working (ensemble operational)
    else
        exit 1  # No providers working
    fi
}

main "$@"
