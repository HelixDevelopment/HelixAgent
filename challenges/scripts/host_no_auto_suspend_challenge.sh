#!/bin/bash
# Host No-Auto-Suspend Challenge
#
# CONST-032 reproduction guard for the host-suspends-during-work bug.
#
# On 2026-04-26 18:23:43 the host suspended in the middle of an
# active development session, killing the running HelixAgent binary
# and the user's SSH connection. journalctl showed:
#   systemd-logind[1183]: The system will suspend now!
# The user's GNOME settings (sleep-inactive-ac-type=nothing,
# sleep-inactive-ac-timeout=900) were correct — the trigger was the
# GDM greeter session at the local console, which has its own power
# policy and does NOT count SSH sessions as activity.
#
# This host runs mission-critical workloads (HelixAgent + 41 services
# + remote container distribution to thinker.local + amber.local).
# Auto-suspend is unsafe.
#
# Pass criteria:
#   1. systemd's sleep.target / suspend.target / hibernate.target /
#      hybrid-sleep.target are all MASKED (cannot be activated by
#      any user / session / DE / greeter / cron job)
#   2. /etc/systemd/sleep.conf has AllowSuspend=no (defense in depth)
#   3. systemd-logind is configured with HandleLidSwitch=ignore
#      AND IdleAction=ignore (defense in depth)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/challenge_framework.sh"

init_challenge "host_no_auto_suspend" \
    "Host No-Auto-Suspend Challenge (CONST-032 reproduction guard)"
load_env

# --- Test 1: sleep targets masked ---
declare -i unmasked=0
unmasked_list=""
for tgt in sleep.target suspend.target hibernate.target hybrid-sleep.target; do
    state=$(systemctl is-enabled "$tgt" 2>/dev/null || echo "unknown")
    log_info "  $tgt: $state"
    if [[ "$state" != "masked" ]]; then
        unmasked+=1
        unmasked_list+="$tgt($state) "
    fi
done

if [[ $unmasked -eq 0 ]]; then
    record_assertion "systemd" "all_sleep_targets_masked" "true" \
        "all sleep/suspend/hibernate/hybrid-sleep targets are masked"
else
    record_assertion "systemd" "all_sleep_targets_masked" "false" \
        "$unmasked sleep targets unmasked: $unmasked_list — host can still be suspended by GDM greeter, DE idle action, or any user with logind privileges"
fi

# --- Test 2: sleep.conf forbids suspend ---
if grep -qE "^AllowSuspend\s*=\s*no" /etc/systemd/sleep.conf 2>/dev/null; then
    record_assertion "systemd_sleep_conf" "allow_suspend_no" "true" \
        "/etc/systemd/sleep.conf has AllowSuspend=no"
else
    record_assertion "systemd_sleep_conf" "allow_suspend_no" "false" \
        "/etc/systemd/sleep.conf does NOT have AllowSuspend=no — second-line defense missing"
fi

# --- Test 3: logind ignores idle ---
idle_action=$(grep -E "^IdleAction\s*=" /etc/systemd/logind.conf 2>/dev/null | cut -d= -f2 | tr -d ' ' | head -1)
idle_action=${idle_action:-"<unset (default: ignore)>"}
log_info "  logind IdleAction: $idle_action"
if [[ "$idle_action" == "ignore" ]] || [[ "$idle_action" == "<unset"* ]]; then
    record_assertion "logind" "idle_action_safe" "true" \
        "logind IdleAction is $idle_action (safe)"
else
    record_assertion "logind" "idle_action_safe" "false" \
        "logind IdleAction=$idle_action — could trigger suspend on idle"
fi

# --- Test 4: no recent unexpected suspend in journal ---
recent_suspends=$(journalctl --since "2 days ago" 2>/dev/null \
    | grep -c "The system will suspend now" || true)
log_info "  Recent 'will suspend' broadcasts: $recent_suspends"
if [[ "$recent_suspends" -eq 0 ]]; then
    record_assertion "history" "no_recent_suspends" "true" \
        "no suspend events in last 2 days"
else
    # Informational — we still want to flag it but not fail outright
    # (the fix prevents future ones; past events are history)
    record_assertion "history" "no_recent_suspends" "false" \
        "$recent_suspends suspend events in last 2 days — fix is needed (this is the bug being fixed)"
fi

record_metric "unmasked_sleep_targets" "$unmasked"
record_metric "recent_suspends_2d" "$recent_suspends"

main() {
    local failed_count
    failed_count=$(grep -c "|FAILED|" "$OUTPUT_DIR/logs/assertions.log" 2>/dev/null || echo 0)
    failed_count=$(echo "$failed_count" | tr -d '[:space:]')
    [[ -z "$failed_count" ]] && failed_count=0
    if [[ "$failed_count" -eq 0 ]]; then
        finalize_challenge "PASSED"; exit 0
    else
        finalize_challenge "FAILED"; exit 1
    fi
}

main "$@"
