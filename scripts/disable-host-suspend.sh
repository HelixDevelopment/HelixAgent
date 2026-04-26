#!/bin/bash
# scripts/disable-host-suspend.sh
#
# One-shot fix for the host-suspends-during-work bug (CONST-032 reproduction:
# challenges/scripts/host_no_auto_suspend_challenge.sh). Run with sudo.
#
# Background: 2026-04-26 18:23:43 the host suspended mid-session,
# killing HelixAgent (PID 872657 at the time) + all SSH sessions. The
# trigger was the GDM greeter user's power policy, not the logged-in
# user's GNOME settings. Even with `sleep-inactive-ac-type=nothing`
# at the user level, the greeter at the local console can suspend
# the host after its own idle timeout.
#
# This script applies defense-in-depth fixes so neither the greeter,
# nor any DE, nor any user with logind privileges, can suspend the
# host while it's running mission-critical workloads.
#
# Usage:
#   sudo bash scripts/disable-host-suspend.sh
#
# Verification (re-run the challenge after this script):
#   bash challenges/scripts/host_no_auto_suspend_challenge.sh

set -euo pipefail

if [[ "$EUID" -ne 0 ]]; then
    echo "ERROR: must be run as root (sudo)." >&2
    exit 1
fi

echo "[1/3] Masking sleep / suspend / hibernate / hybrid-sleep targets..."
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target

echo "[2/3] Setting AllowSuspend=no in /etc/systemd/sleep.conf..."
mkdir -p /etc/systemd/sleep.conf.d
cat > /etc/systemd/sleep.conf.d/00-no-suspend.conf <<'EOF'
# CONST-032: host runs mission-critical HelixAgent + remote container
# distribution; auto-suspend is unsafe.
[Sleep]
AllowSuspend=no
AllowHibernation=no
AllowSuspendThenHibernate=no
AllowHybridSleep=no
EOF

echo "[3/3] Setting logind IdleAction=ignore + HandleLidSwitch=ignore..."
mkdir -p /etc/systemd/logind.conf.d
cat > /etc/systemd/logind.conf.d/00-no-idle-suspend.conf <<'EOF'
# CONST-032: do not suspend the host on idle (SSH sessions don't
# count as activity; the greeter's idle policy was triggering this).
[Login]
IdleAction=ignore
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
EOF

echo "Reloading systemd..."
systemctl daemon-reload
systemctl reload-or-restart systemd-logind || true

echo
echo "DONE. Verify with:"
echo "  bash challenges/scripts/host_no_auto_suspend_challenge.sh"
echo
echo "All 4 assertions should now PASS."
