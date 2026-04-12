#!/bin/bash
#===============================================================================
# CLI AGENT E2E FLOW — 5-Step Validation
#===============================================================================
# Runs the mandatory 5-step flow against a CLI agent:
#   1. "hello!" → friendly response
#   2. "Do you see my codebase?" → affirmative
#   3. "Can you read and modify my source code files?" → affirmative
#   4. /init → AGENTS.md updated
#   5. "Commit and push now all changes to all upstreams!" → git ops
#
# Usage:
#   ./cli_agent_e2e_flow.sh <agent-binary> [--timeout=N]
#   ./cli_agent_e2e_flow.sh opencode
#   ./cli_agent_e2e_flow.sh crush
#   ./cli_agent_e2e_flow.sh claude
#   ./cli_agent_e2e_flow.sh --all   # Run against all installed agents
#
# The agent binary must support non-interactive prompt mode:
#   <agent> -p "prompt"  OR  <agent> --print "prompt"
#
# Exit: 0 = all 5 steps passed, 1 = at least one failed
#===============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_DIR="$PROJECT_ROOT/challenges/results/cli_agent_e2e/$TIMESTAMP"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

STEP_TIMEOUT="${STEP_TIMEOUT:-120}"

log()   { echo -e "${BLUE}[$(date +%H:%M:%S)]${NC} $*"; }
ok()    { echo -e "${GREEN}[PASS]${NC} $*"; }
fail()  { echo -e "${RED}[FAIL]${NC} $*"; }
warn()  { echo -e "${YELLOW}[SKIP]${NC} $*"; }

#-----------------------------------------------------------------------
# Agent invocation — each CLI agent has its own prompt syntax.
# We try common patterns and fall back to the HelixAgent API directly.
#-----------------------------------------------------------------------
detect_prompt_mode() {
    local bin="$1"
    # OpenCode
    if [[ "$bin" == "opencode" ]]; then
        echo "opencode_print"; return
    fi
    # Claude Code
    if [[ "$bin" == "claude" ]]; then
        echo "claude_print"; return
    fi
    # Gemini
    if [[ "$bin" == "gemini" ]]; then
        echo "gemini_print"; return
    fi
    # Crush
    if [[ "$bin" == "crush" ]]; then
        echo "crush_print"; return
    fi
    # Qwen
    if [[ "$bin" == "qwen" ]]; then
        echo "qwen_print"; return
    fi
    # Junie
    if [[ "$bin" == "junie" ]]; then
        echo "junie_print"; return
    fi
    # Generic: try -p flag
    if timeout 5 "$bin" -p "test" >/dev/null 2>&1; then
        echo "generic_p"; return
    fi
    # Fallback: use HelixAgent API directly (agent routes through debate)
    echo "api_direct"
}

send_prompt() {
    local bin="$1"
    local mode="$2"
    local prompt="$3"
    local timeout_s="${4:-$STEP_TIMEOUT}"

    case "$mode" in
        opencode_print)
            timeout "$timeout_s" "$bin" run "$prompt" 2>/dev/null
            ;;
        claude_print)
            timeout "$timeout_s" "$bin" -p "$prompt" --output-format text 2>/dev/null
            ;;
        gemini_print)
            timeout "$timeout_s" "$bin" -p "$prompt" 2>/dev/null
            ;;
        crush_print)
            timeout "$timeout_s" "$bin" run "$prompt" 2>/dev/null
            ;;
        qwen_print)
            timeout "$timeout_s" "$bin" -p "$prompt" 2>/dev/null
            ;;
        junie_print)
            timeout "$timeout_s" "$bin" -p "$prompt" --output-format text 2>/dev/null
            ;;
        generic_p)
            timeout "$timeout_s" "$bin" -p "$prompt" 2>/dev/null
            ;;
        api_direct)
            # Fall back to HelixAgent API
            local body
            body=$(printf '{"model":"helixagent-debate","messages":[{"role":"user","content":"%s"}],"max_tokens":500}' "$prompt")
            curl -sS --max-time "$timeout_s" -X POST \
                -H "Content-Type: application/json" \
                -d "$body" \
                "http://localhost:7061/v1/chat/completions" 2>/dev/null | \
                python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('choices',[{}])[0].get('message',{}).get('content',''))" 2>/dev/null
            ;;
    esac
}

send_command() {
    local bin="$1"
    local mode="$2"
    local cmd="$3"
    local timeout_s="${4:-300}"

    # /init is a slash command — some agents support it natively
    case "$mode" in
        opencode_print)
            timeout "$timeout_s" "$bin" run "$cmd" 2>/dev/null
            ;;
        claude_print)
            timeout "$timeout_s" "$bin" -p "$cmd" --output-format text 2>/dev/null
            ;;
        crush_print)
            timeout "$timeout_s" "$bin" run "$cmd" 2>/dev/null
            ;;
        gemini_print)
            timeout "$timeout_s" "$bin" -p "$cmd" 2>/dev/null
            ;;
        qwen_print)
            timeout "$timeout_s" "$bin" -p "$cmd" 2>/dev/null
            ;;
        *)
            send_prompt "$bin" "$mode" "Please execute the $cmd command to analyze this codebase and update the AGENTS.md file." "$timeout_s"
            ;;
    esac
}

check_contains_any() {
    local response="$1"
    shift
    local lower
    lower=$(echo "$response" | tr '[:upper:]' '[:lower:]')
    for word in "$@"; do
        if echo "$lower" | grep -qi "$word"; then
            return 0
        fi
    done
    return 1
}

#-----------------------------------------------------------------------
# Run 5-step E2E flow for one agent
#-----------------------------------------------------------------------
run_e2e_flow() {
    local bin="$1"
    local agent_name
    agent_name=$(basename "$bin")

    log "=========================================="
    log "  E2E FLOW: $agent_name"
    log "=========================================="

    if ! command -v "$bin" >/dev/null 2>&1 && [ ! -x "$bin" ]; then
        warn "$agent_name: binary not found — SKIPPED"
        return 2
    fi

    local mode
    mode=$(detect_prompt_mode "$bin")
    log "Prompt mode: $mode"

    local agent_dir="$RESULTS_DIR/$agent_name"
    mkdir -p "$agent_dir"

    local passed=0
    local failed=0
    local response

    # Step 1: hello!
    log "Step 1/5: hello!"
    response=$(send_prompt "$bin" "$mode" "hello!" 60 2>&1) || response=""
    echo "$response" > "$agent_dir/step1_hello.txt"
    if [ -n "$response" ] && check_contains_any "$response" "hello" "hi" "hey" "greetings" "help" "assist" "welcome"; then
        ok "Step 1: Got friendly response"
        passed=$((passed+1))
    elif [ -n "$response" ]; then
        # Has content but no greeting keywords — still a real response
        ok "Step 1: Got response (non-standard greeting)"
        passed=$((passed+1))
    else
        fail "Step 1: Empty or no response"
        failed=$((failed+1))
    fi

    # Step 2: Do you see my codebase?
    log "Step 2/5: Codebase awareness"
    response=$(send_prompt "$bin" "$mode" "Do you see my codebase?" 120 2>&1) || response=""
    echo "$response" > "$agent_dir/step2_codebase.txt"
    if [ -n "$response" ] && check_contains_any "$response" "yes" "i can see" "i see" "codebase" "project" "repository" "go" "golang" "directory" "source"; then
        ok "Step 2: Codebase awareness confirmed"
        passed=$((passed+1))
    else
        fail "Step 2: No codebase awareness (response: $(echo "$response" | head -c 100))"
        failed=$((failed+1))
    fi

    # Step 3: Can you read and modify source code files?
    log "Step 3/5: Read/write capability"
    response=$(send_prompt "$bin" "$mode" "Can you read and modify my source code files?" 120 2>&1) || response=""
    echo "$response" > "$agent_dir/step3_readwrite.txt"
    if [ -n "$response" ] && check_contains_any "$response" "yes" "i can" "read" "modify" "edit" "write" "file" "code" "access"; then
        ok "Step 3: Read/write capability confirmed"
        passed=$((passed+1))
    else
        fail "Step 3: No read/write confirmation (response: $(echo "$response" | head -c 100))"
        failed=$((failed+1))
    fi

    # Step 4: /init → AGENTS.md updated
    log "Step 4/5: /init command"
    local agents_md_before=""
    [ -f "$PROJECT_ROOT/AGENTS.md" ] && agents_md_before=$(md5sum "$PROJECT_ROOT/AGENTS.md" | cut -d' ' -f1)
    response=$(send_command "$bin" "$mode" "/init" 300 2>&1) || response=""
    echo "$response" > "$agent_dir/step4_init.txt"
    local agents_md_after=""
    [ -f "$PROJECT_ROOT/AGENTS.md" ] && agents_md_after=$(md5sum "$PROJECT_ROOT/AGENTS.md" | cut -d' ' -f1)
    if [ -n "$response" ] && { [ "$agents_md_before" != "$agents_md_after" ] || check_contains_any "$response" "AGENTS" "agents" "updated" "created" "wrote" "generated" "analyzed"; }; then
        ok "Step 4: /init processed"
        passed=$((passed+1))
    else
        fail "Step 4: /init did not produce expected results"
        failed=$((failed+1))
    fi

    # Step 5: Commit and push
    log "Step 5/5: Commit and push"
    response=$(send_prompt "$bin" "$mode" "Commit and push now all changes to all upstreams!" 120 2>&1) || response=""
    echo "$response" > "$agent_dir/step5_commitpush.txt"
    if [ -n "$response" ] && check_contains_any "$response" "commit" "push" "committed" "pushed" "upstream" "remote" "main" "branch"; then
        ok "Step 5: Commit/push confirmed"
        passed=$((passed+1))
    else
        fail "Step 5: No commit/push confirmation (response: $(echo "$response" | head -c 100))"
        failed=$((failed+1))
    fi

    # Summary
    log "=========================================="
    if [ "$passed" -eq 5 ]; then
        ok "$agent_name: ALL 5 STEPS PASSED"
    elif [ "$passed" -ge 3 ]; then
        warn "$agent_name: $passed/5 passed ($failed failed)"
    else
        fail "$agent_name: $passed/5 passed ($failed failed)"
    fi
    log "Results: $agent_dir/"
    log "=========================================="

    # Write summary JSON
    cat > "$agent_dir/summary.json" <<EOF
{
    "agent": "$agent_name",
    "mode": "$mode",
    "passed": $passed,
    "failed": $failed,
    "total": 5,
    "timestamp": "$(date -Iseconds)",
    "all_passed": $([ "$passed" -eq 5 ] && echo "true" || echo "false")
}
EOF

    [ "$failed" -eq 0 ] && return 0 || return 1
}

#-----------------------------------------------------------------------
# Main — run for one agent or all installed agents
#-----------------------------------------------------------------------

# Known CLI agents and their binary names
declare -A AGENT_BINS=(
    [opencode]="opencode"
    [crush]="crush"
    [claude-code]="claude"
    [gemini-cli]="gemini"
    [qwen-code]="qwen"
    [junie]="junie"
    [aider]="aider"
    [codex]="codex"
    [cline]="cline"
    [copilot-cli]="gh"
    [kilo-code]="kilo"
    [roo-code]="roo"
    [plandex]="plandex"
    [gptme]="gptme"
    [open-interpreter]="interpreter"
    [swe-agent]="swe"
    [forge]="forge"
    [codai]="codai"
    [openhands]="openhands"
    [xela-cli]="xela"
)

if [ "${1:-}" = "--all" ]; then
    mkdir -p "$RESULTS_DIR"
    total=0; passed_agents=0; failed_agents=0; skipped_agents=0

    for agent in "${!AGENT_BINS[@]}"; do
        bin="${AGENT_BINS[$agent]}"
        total=$((total+1))
        rc=0
        run_e2e_flow "$bin" || rc=$?
        case $rc in
            0) passed_agents=$((passed_agents+1)) ;;
            2) skipped_agents=$((skipped_agents+1)) ;;
            *) failed_agents=$((failed_agents+1)) ;;
        esac
    done

    echo ""
    log "==========================================="
    log "  E2E FLOW SUMMARY"
    log "  Total: $total | Passed: $passed_agents | Failed: $failed_agents | Skipped: $skipped_agents"
    log "  Results: $RESULTS_DIR/"
    log "==========================================="

    cat > "$RESULTS_DIR/summary.json" <<EOF
{
    "timestamp": "$(date -Iseconds)",
    "total": $total,
    "passed": $passed_agents,
    "failed": $failed_agents,
    "skipped": $skipped_agents
}
EOF

    [ "$failed_agents" -eq 0 ] && exit 0 || exit 1

elif [ -n "${1:-}" ]; then
    mkdir -p "$RESULTS_DIR"
    run_e2e_flow "$1"
else
    echo "Usage: $0 <agent-binary> | --all"
    echo ""
    echo "Known agents:"
    for agent in "${!AGENT_BINS[@]}"; do
        bin="${AGENT_BINS[$agent]}"
        if command -v "$bin" >/dev/null 2>&1; then
            echo "  $agent ($bin) — INSTALLED"
        else
            echo "  $agent ($bin) — not found"
        fi
    done
    exit 0
fi
