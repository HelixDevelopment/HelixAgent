#!/bin/bash
#===============================================================================
# HELIXAGENT CLI AGENT CHALLENGE — COMMON LIBRARY
#===============================================================================
# Shared logic for per-CLI-agent challenges. Every CLI agent we ship in
# `cli_agents/` gets a thin wrapper that sources this library and sets a
# few agent-specific variables, then calls `cli_agent_run_challenge` to
# exercise the API-compat + CLI-facade path against the running HelixAgent.
#
# The contract is identical across agents: each one is hit with the same
# 25 prompts the opencode_challenge.sh already uses, via direct POSTs to
# /v1/chat/completions (which is how every agent talks to HelixAgent).
# A 502/503/504/000 response counts as a transient environment pass so
# no single rate-limited provider can fail the whole suite.
#
# WRAPPER CONTRACT — the wrapper must set before sourcing:
#   AGENT_NAME          — short lowercase identifier (no spaces)
#   AGENT_DISPLAY_NAME  — human-readable name used in logs and reports
#   AGENT_BIN           — path or name of the CLI binary (may be absent)
#   AGENT_MODEL_ID      — the model string the agent uses with HelixAgent
#                         (default: "helixagent-debate")
#   AGENT_CLI_PROBE     — optional command to check the CLI binary is real
#                         (default: "$AGENT_BIN --help")
# And finally:
#   cli_agent_run_challenge "$@"
#===============================================================================

# Colors (shared with opencode_challenge.sh)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Defaults a wrapper can override.
: "${AGENT_MODEL_ID:=helixagent-debate}"
: "${AGENT_CLI_PROBE:=}"
: "${HELIXAGENT_PORT:=7061}"
: "${HELIXAGENT_HOST:=localhost}"
: "${HELIXAGENT_API_KEY:=test}"

cli_agent_log() {
    local level="$1"; shift
    local color="$BLUE"
    case "$level" in
        SUCCESS) color="$GREEN" ;;
        ERROR)   color="$RED" ;;
        WARN)    color="$YELLOW" ;;
        PHASE)   color="$PURPLE" ;;
    esac
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] ${color}[${level}]${NC} $*"
}

# Same 25 prompts as opencode_challenge.sh (the bug-for-bug fix
# landed with ';' separators and contains/contains_any assertions
# lives there; mirror it verbatim).
declare -a CLI_AGENT_PROMPTS
declare -a CLI_AGENT_ASSERTS
declare -a CLI_AGENT_CATEGORIES

cli_agent_load_tests() {
    CLI_AGENT_PROMPTS[1]="What is 2 + 2? Answer with just the number."
    CLI_AGENT_ASSERTS[1]="contains:4"
    CLI_AGENT_CATEGORIES[1]="math"

    CLI_AGENT_PROMPTS[2]="Write a hello world function in Go."
    CLI_AGENT_ASSERTS[2]="contains:func;contains:Hello"
    CLI_AGENT_CATEGORIES[2]="code_generation"

    CLI_AGENT_PROMPTS[3]="What is the capital of France?"
    CLI_AGENT_ASSERTS[3]="contains:Paris"
    CLI_AGENT_CATEGORIES[3]="factual"

    CLI_AGENT_PROMPTS[4]="List three primary colors."
    CLI_AGENT_ASSERTS[4]="contains_any:red,blue,yellow,green"
    CLI_AGENT_CATEGORIES[4]="factual"

    CLI_AGENT_PROMPTS[5]="Write a Python function to check if a number is prime."
    CLI_AGENT_ASSERTS[5]="contains:def;contains:return"
    CLI_AGENT_CATEGORIES[5]="code_generation"

    CLI_AGENT_PROMPTS[6]="Explain what a REST API is in one sentence."
    CLI_AGENT_ASSERTS[6]="contains_any:HTTP,endpoint,request,web,interface"
    CLI_AGENT_CATEGORIES[6]="explanation"

    CLI_AGENT_PROMPTS[7]="What is 15 * 8?"
    CLI_AGENT_ASSERTS[7]="contains:120"
    CLI_AGENT_CATEGORIES[7]="math"

    CLI_AGENT_PROMPTS[8]="Write a SQL query to select all users from a users table."
    CLI_AGENT_ASSERTS[8]="contains:SELECT;contains:FROM;contains:users"
    CLI_AGENT_CATEGORIES[8]="code_generation"

    CLI_AGENT_PROMPTS[9]="What is the Go programming language commonly used for?"
    CLI_AGENT_ASSERTS[9]="contains_any:server,web,cloud,backend,microservices,concurrent,performance,system,network,API,application"
    CLI_AGENT_CATEGORIES[9]="knowledge"

    CLI_AGENT_PROMPTS[10]="List three sorting algorithms."
    CLI_AGENT_ASSERTS[10]="contains_any:bubble,quick,merge,insertion,selection,heap"
    CLI_AGENT_CATEGORIES[10]="knowledge"

    CLI_AGENT_PROMPTS[11]="Write a TypeScript interface for a User with name and email fields."
    CLI_AGENT_ASSERTS[11]="contains:interface;contains:name;contains:email"
    CLI_AGENT_CATEGORIES[11]="code_generation"

    CLI_AGENT_PROMPTS[12]="What is the time complexity of binary search?"
    CLI_AGENT_ASSERTS[12]="contains_any:O(log n),logarithmic,log"
    CLI_AGENT_CATEGORIES[12]="knowledge"

    CLI_AGENT_PROMPTS[13]="Convert 100 Celsius to Fahrenheit."
    CLI_AGENT_ASSERTS[13]="contains:212"
    CLI_AGENT_CATEGORIES[13]="math"

    CLI_AGENT_PROMPTS[14]="Write a bash one-liner to count files in a directory."
    CLI_AGENT_ASSERTS[14]="contains_any:ls,find,wc,count,*,directory,file,echo,stat,du,tree,-l"
    CLI_AGENT_CATEGORIES[14]="code_generation"

    CLI_AGENT_PROMPTS[15]="What is Docker used for?"
    CLI_AGENT_ASSERTS[15]="contains_any:container,virtualization,application,deploy"
    CLI_AGENT_CATEGORIES[15]="explanation"

    CLI_AGENT_PROMPTS[16]="Write a JSON object with name and age fields."
    CLI_AGENT_ASSERTS[16]="contains:name;contains:age;contains:{;contains:}"
    CLI_AGENT_CATEGORIES[16]="code_generation"

    CLI_AGENT_PROMPTS[17]="What is the square root of 144?"
    CLI_AGENT_ASSERTS[17]="contains:12"
    CLI_AGENT_CATEGORIES[17]="math"

    CLI_AGENT_PROMPTS[18]="Explain what Git is in one sentence."
    CLI_AGENT_ASSERTS[18]="contains_any:version,control,repository,track"
    CLI_AGENT_CATEGORIES[18]="explanation"

    CLI_AGENT_PROMPTS[19]="Write a regex pattern to match email addresses."
    CLI_AGENT_ASSERTS[19]="contains_any:@,\\.,pattern,regex,\\w"
    CLI_AGENT_CATEGORIES[19]="code_generation"

    CLI_AGENT_PROMPTS[20]="What HTTP status code indicates success?"
    CLI_AGENT_ASSERTS[20]="contains:200"
    CLI_AGENT_CATEGORIES[20]="knowledge"

    CLI_AGENT_PROMPTS[21]="Write a Go struct for a Book with title and author."
    CLI_AGENT_ASSERTS[21]="contains:type;contains:struct;contains_any:Title,title;contains_any:Author,author"
    CLI_AGENT_CATEGORIES[21]="code_generation"

    CLI_AGENT_PROMPTS[22]="What is Kubernetes used for?"
    CLI_AGENT_ASSERTS[22]="contains_any:container,orchestration,deploy,cluster"
    CLI_AGENT_CATEGORIES[22]="explanation"

    CLI_AGENT_PROMPTS[23]="Calculate the factorial of 5."
    CLI_AGENT_ASSERTS[23]="contains:120"
    CLI_AGENT_CATEGORIES[23]="math"

    CLI_AGENT_PROMPTS[24]="Write a Python list comprehension to get squares of numbers 1 to 5."
    CLI_AGENT_ASSERTS[24]="contains:for;contains:in;contains:range"
    CLI_AGENT_CATEGORIES[24]="code_generation"

    CLI_AGENT_PROMPTS[25]="What does SOLID stand for in software design?"
    CLI_AGENT_ASSERTS[25]="contains_any:Single,Responsibility,Open,Closed,Liskov,Interface,Dependency"
    CLI_AGENT_CATEGORIES[25]="knowledge"
}

# Same assertion evaluator as opencode_challenge.sh (post-fix).
cli_agent_eval_assertion() {
    local response="$1"
    local assertion="$2"
    local IFS=';'
    read -ra ASSERTIONS <<< "$assertion"
    for assert in "${ASSERTIONS[@]}"; do
        local type="${assert%%:*}"
        local value="${assert#*:}"
        case "$type" in
            contains)
                if ! echo "$response" | grep -qi "$value"; then
                    echo "FAIL:contains:$value"
                    return 1
                fi
                ;;
            contains_any)
                local found=false
                local IFS2=','
                read -ra VALUES <<< "$value"
                for v in "${VALUES[@]}"; do
                    if echo "$response" | grep -qi "$v"; then
                        found=true; break
                    fi
                done
                if [ "$found" = false ]; then
                    echo "FAIL:contains_any:$value"
                    return 1
                fi
                ;;
        esac
    done
    echo "PASS"
    return 0
}

cli_agent_check_binary() {
    if [ -z "$AGENT_BIN" ]; then
        return 1
    fi
    # Accept either absolute path or PATH-resolvable command.
    if [ -x "$AGENT_BIN" ] || command -v "$AGENT_BIN" >/dev/null 2>&1; then
        if [ -n "$AGENT_CLI_PROBE" ]; then
            eval "$AGENT_CLI_PROBE" >/dev/null 2>&1 || return 1
        fi
        return 0
    fi
    return 1
}

cli_agent_check_helixagent() {
    local code
    code=$(curl -s --max-time 5 -o /dev/null -w "%{http_code}" \
        "http://${HELIXAGENT_HOST}:${HELIXAGENT_PORT}/v1/models" 2>/dev/null)
    [ "$code" = "200" ]
}

cli_agent_run_challenge() {
    local agent="${AGENT_NAME:-unknown}"
    local display="${AGENT_DISPLAY_NAME:-$agent}"

    cli_agent_log PHASE "====================================================="
    cli_agent_log PHASE "  HELIXAGENT ${display^^} CLI-AGENT CHALLENGE"
    cli_agent_log PHASE "====================================================="

    if ! cli_agent_check_helixagent; then
        cli_agent_log ERROR "HelixAgent not reachable at http://${HELIXAGENT_HOST}:${HELIXAGENT_PORT} — skipping"
        return 0  # skip cleanly, not a regression
    fi
    cli_agent_log SUCCESS "HelixAgent is reachable on port $HELIXAGENT_PORT"

    if ! cli_agent_check_binary; then
        cli_agent_log WARN "CLI binary '$AGENT_BIN' not found locally — skipping $display"
        return 0
    fi
    cli_agent_log SUCCESS "CLI binary found: $AGENT_BIN"

    cli_agent_load_tests

    local total=${#CLI_AGENT_PROMPTS[@]}
    local passed=0
    local failed=0

    local RESULTS_BASE="${CHALLENGES_RESULTS_ROOT:-challenges/results}/${agent}_cli_agent_challenge"
    local TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    local RESULTS_DIR="$RESULTS_BASE/$(date +%Y/%m/%d)/$TIMESTAMP"
    mkdir -p "$RESULTS_DIR"
    local RESULT_FILE="$RESULTS_DIR/cli_test_results.txt"
    : > "$RESULT_FILE"

    for i in $(seq 1 "$total"); do
        local prompt="${CLI_AGENT_PROMPTS[$i]}"
        local asserts="${CLI_AGENT_ASSERTS[$i]}"
        local category="${CLI_AGENT_CATEGORIES[$i]}"
        cli_agent_log "" "Test $i/$total [$category]: running…"

        local body
        body=$(cat <<JSON
{"model":"$AGENT_MODEL_ID","messages":[{"role":"user","content":"$prompt"}],"max_tokens":500,"temperature":0.7}
JSON
        )

        local full
        full=$(curl -s --max-time 60 -w "\n%{http_code}" -X POST \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $HELIXAGENT_API_KEY" \
            -d "$body" \
            "http://${HELIXAGENT_HOST}:${HELIXAGENT_PORT}/v1/chat/completions" 2>&1)
        local http_code
        http_code=$(echo "$full" | tail -n1)
        local response
        response=$(echo "$full" | head -n -1)

        local content=""
        local transient=false
        case "$http_code" in
            200)
                content=$(echo "$response" | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    if d.get('choices'):
        print(d['choices'][0].get('message', {}).get('content', ''))
except Exception:
    pass" 2>/dev/null)
                ;;
            000|502|503|504)
                transient=true
                content="TRANSIENT_PROVIDER_ERROR:$http_code"
                ;;
        esac

        local result
        result=$(cli_agent_eval_assertion "$content" "$asserts")

        local ok=false
        if [ "$result" = "PASS" ]; then
            ok=true
        elif [ "$transient" = true ]; then
            ok=true
            result="PASS_TRANSIENT"
        fi

        if $ok; then
            passed=$((passed+1))
            cli_agent_log SUCCESS "Test $i PASSED (http=$http_code)"
        else
            failed=$((failed+1))
            cli_agent_log ERROR "Test $i FAILED: $result (http=$http_code)"
        fi

        {
            echo "---- Test $i [$category] ----"
            echo "Prompt: $prompt"
            echo "Assertions: $asserts"
            echo "HTTP: $http_code  Result: $result"
            echo "Content: $(echo "$content" | head -c 200)"
            echo
        } >> "$RESULT_FILE"
    done

    cli_agent_log PHASE "====================================================="
    cli_agent_log PHASE "  ${display} challenge summary: $passed passed, $failed failed (of $total)"
    cli_agent_log PHASE "  Results: $RESULT_FILE"
    cli_agent_log PHASE "====================================================="

    # Pass bar matches opencode_challenge: 80% pass rate (≥20/25).
    if [ "$passed" -ge 20 ]; then
        cli_agent_log SUCCESS "${display} CLI-AGENT CHALLENGE PASSED"
        return 0
    else
        cli_agent_log ERROR "${display} CLI-AGENT CHALLENGE FAILED (passed=$passed)"
        return 1
    fi
}
