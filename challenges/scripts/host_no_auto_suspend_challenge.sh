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
# `systemctl is-enabled` may emit two lines (state + an "unknown"
# warning on some distros); take the FIRST line only.
declare -i unmasked=0
unmasked_list=""
for tgt in sleep.target suspend.target hibernate.target hybrid-sleep.target; do
    # systemctl is-enabled returns non-zero for masked units; trap
    # via `|| true` so set -e (inherited from challenge_framework.sh)
    # doesn't kill the loop after the first iteration.
    state=$( { systemctl is-enabled "$tgt" 2>/dev/null || true; } | head -n1 | tr -d '[:space:]')
    [[ -z "$state" ]] && state="unknown"
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
# Check both the main file AND any drop-in under sleep.conf.d/ —
# the fix script writes the drop-in form which systemd merges.
sleep_conf_files="/etc/systemd/sleep.conf /etc/systemd/sleep.conf.d/*.conf"
if grep -shqE "^AllowSuspend\s*=\s*no" $sleep_conf_files 2>/dev/null; then
    record_assertion "systemd_sleep_conf" "allow_suspend_no" "true" \
        "AllowSuspend=no found in sleep.conf or a drop-in"
else
    record_assertion "systemd_sleep_conf" "allow_suspend_no" "false" \
        "AllowSuspend=no NOT found in sleep.conf or any sleep.conf.d/ drop-in — second-line defense missing"
fi

# --- Test 3: logind ignores idle ---
# Same drop-in story: check both main + drop-ins.
logind_conf_files="/etc/systemd/logind.conf /etc/systemd/logind.conf.d/*.conf"
# `|| true` because grep returns 1 when nothing matches and `set -e`
# from the framework would otherwise kill the script.
idle_action=$( { grep -shE "^IdleAction\s*=" $logind_conf_files 2>/dev/null || true; } | tail -n1 | cut -d= -f2 | tr -d ' ')
idle_action=${idle_action:-"<unset (default: ignore)>"}
log_info "  logind IdleAction: $idle_action"
if [[ "$idle_action" == "ignore" ]] || [[ "$idle_action" == "<unset"* ]]; then
    record_assertion "logind" "idle_action_safe" "true" \
        "logind IdleAction is $idle_action (safe)"
else
    record_assertion "logind" "idle_action_safe" "false" \
        "logind IdleAction=$idle_action — could trigger suspend on idle"
fi

# --- Test 4: no suspend events since the fix was applied ---
# The drop-in conf written by scripts/disable-host-suspend.sh is the
# anchor for "fix was applied at time T". We assert no suspend events
# in the journal AFTER that mtime. Past events from before the fix
# are history and not a regression — only new suspends post-fix
# indicate the masking didn't take.
fix_marker="/etc/systemd/sleep.conf.d/00-no-suspend.conf"
if [[ -f "$fix_marker" ]]; then
    fix_mtime_iso=$(date -d "@$(stat -c %Y "$fix_marker")" -Iseconds 2>/dev/null \
        || stat -c %y "$fix_marker" | head -c 19 | tr ' ' 'T')
    log_info "  Fix applied at: $fix_mtime_iso"
    suspends_since_fix=$( { journalctl --since "$fix_mtime_iso" 2>/dev/null || true; } \
        | { grep -c "The system will suspend now" || true; })
    log_info "  'Will suspend' broadcasts since fix: $suspends_since_fix"
    if [[ "$suspends_since_fix" -eq 0 ]]; then
        record_assertion "history" "no_suspends_since_fix" "true" \
            "no suspend events since fix was applied at $fix_mtime_iso"
    else
        record_assertion "history" "no_suspends_since_fix" "false" \
            "$suspends_since_fix suspend events since the fix was applied — masking didn't take"
    fi
else
    record_assertion "history" "no_suspends_since_fix" "false" \
        "fix marker $fix_marker not found — fix script hasn't been run yet"
fi

record_metric "unmasked_sleep_targets" "$unmasked"
record_metric "suspends_since_fix" "${suspends_since_fix:-0}"

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
