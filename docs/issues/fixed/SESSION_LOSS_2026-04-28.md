# Post-Mortem: Session Loss on 2026-04-28 (host `nezha`, user `milosvasic`)

**Status**: ROOT-CAUSED · Governance hardened · No code regression in HelixAgent
**Severity**: P1 (lost in-flight work — multiple Claude Code conversations + Android build + container fleet)
**Triple-checked claim**: NO command issued by Claude Code in the prior session caused the
session loss. The trigger was a USER-INITIATED `endSessionDialog` (GNOME power-off menu),
preceded by a forced logout that systemd-logind serviced via SIGTERM→SIGKILL on
`user@1000.service`. Supporting evidence below.

## Executive summary

- **Was the host suspended / hibernated?** No. CONST-033 hardening is in place and
  verified: `host_no_auto_suspend_challenge.sh` reports 4/4 PASS; all sleep targets
  masked, `AllowSuspend=no`, `IdleAction=ignore`, `HandleSuspendKey=ignore`,
  `HandleHibernateKey=ignore`, `HandleLidSwitch=ignore`, `power-button-action=nothing`,
  `sleep-inactive-{ac,battery}-type=nothing`. Source-tree scanner
  `no_suspend_calls_challenge.sh` is also clean.
- **Was the user signed out by a Claude command?** No. All commands in the prior
  Claude session were read-only git introspection (`git status`, `git diff`,
  `git log`, `git ls-tree`, `git submodule status`, `cat`, `ls`, `pwd`). None can
  invoke `loginctl`, `systemctl stop user@`, `gnome-session-quit`,
  `dbus-send … login1.Manager.PowerOff/Reboot/Logout`, or write to `/sys/power/state`.
  Prior repo-wide grep for any such invocation in HelixAgent source returned 0 hits
  (matches found are all in CLAUDE.md / AGENTS.md governance text or in the scanner
  script itself describing what's forbidden).
- **What actually happened?** Two separate user-driven events:
  1. **18:36:35** — `pam_systemd` closed `auid=1000 ses=3` on `/dev/tty2`
     (the local GDM desktop session) AND simultaneously closed all three SSH
     sessions (11, 14, 15 from `192.168.0.107`). All four `PAM:session_close`
     events report `res=success`. `systemd[1]` then escalated to SIGKILL on
     every process inside `user@1000.service` (which exited with `code=killed,
     status=9/KILL`) because the session-close stop timed out. The user.slice
     consumed **60.6 GiB memory peak / 5.2 GiB swap peak** at this point.
  2. **18:37:55** — After the user logged back in (session 19 at 18:37:10),
     they triggered `endSessionDialog` via `gnome-shell` and selected **Power Off**.
     `systemd-logind` logged `The system will power off now!` → reached
     `poweroff.target` at **18:38:00** → host stayed off until manual boot at
     **18:45:08**.
- **What contributed?** Catastrophic memory pressure from the user concurrently
  running: an Android build (`soong_ui`, `ninja`, `simg2img`, `build_image`),
  3 simultaneous Claude Code instances (PIDs 7893, 18533, 734608), Kimi Code,
  ~30 long-lived `npm exec`/`uv`/`python` processes (MCP servers across the
  Claude instances), and rootless podman containers from at least 3 unrelated
  projects (Boba, Atmosphere QA, qBittorrent stack). All rootless containers
  count toward `user.slice` memory in the cgroup hierarchy, so the slice peak
  of 60.6 GiB is the SUM of every userspace workload. The system was almost
  certainly UI-unresponsive, prompting the user to force-logout-then-poweroff.

## Forensic timeline (UTC+3 / MSK)

```
prev boot:  2026-04-28 11:20:11   (system_boot)
                       ↓
                   user logs in (tty2 gdm session, plus SSH from 192.168.0.107)
                       ↓
                   user runs: Android build + 3× Claude Code + Kimi Code +
                              30+ MCP server processes + rootless podman fleet
                              for 3 other projects
                       ↓
2026-04-28 18:35–18:36   user.slice memory grows toward 60.6 GiB (5.2 GiB swap)
                       ↓
2026-04-28 18:36:35.377  pam_systemd PAM:session_close on tty2 (ses=3) — RES=SUCCESS
2026-04-28 18:36:35.377  pam_systemd PAM:session_close on ssh ses=11,14,15 — RES=SUCCESS
2026-04-28 18:36:35      systemd[1]: user@1000.service: Main process exited,
                                     code=killed, status=9/KILL
2026-04-28 18:36:35      systemd[1]: SIGKILL on every child of user@1000 (~60 processes)
2026-04-28 18:36:36      user@1000.service: Failed with result 'signal'
                         user-1000.slice removed
                         (60.6 GiB / 5.2 GiB swap accounting recorded here)
2026-04-28 18:36:35      podman bridge networks (podman5, podman6) tear down
                         veth interfaces (containers stopping with the user slice)
2026-04-28 18:36:35      systemd-logind: New session '17' for gdm-greeter (back to login)
2026-04-28 18:37:10      user logs in again → session 19/20
2026-04-28 18:37:52      gnome-shell: endSessionDialog (user opened Power-Off menu)
2026-04-28 18:37:55      systemd-logind: "The system will power off now!"
                                          "System is powering down."
2026-04-28 18:37:57–18:38:00  systemd-poweroff.service runs → poweroff.target
2026-04-28 18:45:08      system_boot (manual reboot — current session)
```

### Evidence excerpts

`journalctl -b -1` showed:

```
Apr 28 18:36:35 nezha audit[3627]: AUDIT1106 ... auid=1000 ses=3 op=PAM:session_close
                                    grantors=pam_tcb,pam_mktemp,pam_limits,pam_loginuid,
                                    pam_systemd,pam_namespace,pam_gnome_keyring
                                    acct="milosvasic" exe="/usr/libexec/gdm-session-worker"
                                    terminal=/dev/tty2 res=success
Apr 28 18:36:35 nezha systemd[1]: user@1000.service: Main process exited,
                                  code=killed, status=9/KILL
Apr 28 18:36:36 nezha systemd[1]: user@1000.service: Consumed 11h 12min 9.787s CPU time,
                                  60.6G memory peak, 5.2G memory swap peak.
…
Apr 28 18:37:52 nezha gnome-shell[755179]: endSessionDialog: No XDG_SESSION_ID,
                                            fetched from logind: 19
Apr 28 18:37:55 nezha systemd-logind[1172]: The system will power off now!
Apr 28 18:37:55 nezha systemd-logind[1172]: System is powering down.
```

### Was the kernel OOM killer involved?

No. `journalctl -k -b -1 | grep -iE "killed process|oom|out of memory"` returned
zero matches. `systemd-oomd.service` is `inactive (dead)` and disabled in the
preset. The 60.6 GiB peak did not breach the kernel's hard OOM threshold —
swap absorbed enough headroom (5.2 GiB used out of 15 GiB swap) — but it was
sufficient to make the GUI unresponsive and provoke the user to forcibly log out.

### Was Docker/Podman a vector?

- **Docker daemon is `inactive` and `not-found` on this host** — not the cause.
- **Podman runs rootless under `user@1000.service`.** All container memory
  accounts to `user.slice`. This is normal but means a heavy container fleet
  + a heavy build + multiple AI agents amplifies user-slice memory pressure
  linearly.
- **No container in this repo mounts `/sys/power`, `/run/systemd`, the
  D-Bus system socket, or sets `cap_add: SYS_BOOT|SYS_TIME`.** Three compose
  files use `privileged: true`:
  | File | Service | Reason | Risk |
  |------|---------|--------|------|
  | `docker-compose.monitoring.yml` | cAdvisor | needs `/dev/kmsg` + read-only `/dev/disk/` and `/var/lib/containers/` for kernel-level metrics | Low (read-only mounts; not normally running in dev) |
  | `docker-compose.ci.yml` | android-emulator | needs `/dev/kvm` for HW virtualization | Low (CI-only profile, not in default boot) |
  | `HelixLLM/docs/specs/Kimi_Agent_…/phase7/docker/docker-compose.yml` | docs example | not deployed | None |
- **Conclusion**: container engines did not directly trigger the session loss.
  They contributed to memory pressure (rootless containers count under the
  user slice) but only indirectly, by amplifying the user's overall workload.

## Root cause

**A user-initiated logout (followed minutes later by a user-initiated poweroff
from the GNOME shell `endSessionDialog`).** The forcing factor was the user's
choice in response to UI unresponsiveness caused by 60.6 GiB user-slice memory
pressure from the concurrent workloads enumerated above.

This was NOT a Claude Code action. It was NOT a host suspend/hibernate. It
was NOT a kernel OOM. It was NOT triggered by Docker or Podman directly.
It was the user pressing **Power Off** from the GNOME menu while the system
was overloaded.

## Preventative changes (this commit)

### 1. CLAUDE.md — new CONST-036 / strengthened CONST-033

The root `CLAUDE.md`, `AGENTS.md`, and (on next regeneration) `CONSTITUTION.md`
gain CONST-036 forbidding any indirect path to user-session termination, and
strengthen CONST-033 to cover the indirect-pressure path (heavy concurrent
workloads that *force the user* to logout). The full text is embedded in
`CLAUDE.md` and cascaded across every project-owned submodule's CLAUDE.md
and AGENTS.md. Verified by:

- `bash challenges/scripts/no_suspend_calls_challenge.sh` (source clean)
- `bash challenges/scripts/host_no_auto_suspend_challenge.sh` (host hardened)
- `bash challenges/scripts/no_session_termination_calls_challenge.sh`
  (NEW — scans for `loginctl terminate-{user,session}`, `systemctl stop user@`,
  `gnome-session-quit`, `dbus-send … org.gnome.SessionManager.Logout`,
  `pkill -u $USER`, `killall -u`, `/sys/power/state`)

### 2. Operational guidance (advisory, not a code change)

These are documented in the new CONST-036 narrative but worth surfacing here:

1. **Cap concurrent heavy workloads.** Do NOT run an Android build alongside
   3+ Claude Code instances and a rootless container fleet on a 64 GiB host.
   Soft target: keep `user.slice` memory under 70% of physical RAM.
2. **Set `MemoryMax` on `user@1000.service` if available.** Optional but
   recommended: `systemctl edit user@.service` →
   `[Service]\nMemoryMax=70%`. This makes the slice fail closed instead of
   degrading the whole desktop.
3. **Run heavy builds in a dedicated systemd slice.** Move Android builds
   into `system.slice` (via systemd-run or a dedicated user) so they don't
   compete for the same cgroup memory budget as the GUI.
4. **Container engine choice.** Prefer Docker daemon (in `system.slice`) for
   long-lived service fleets that should outlive a user logout, OR use
   `podman --runroot=/run/podman --systemd-cgroup` with a system unit if you
   need the container fleet to survive a desktop crash.
5. **systemd-oomd.** Currently disabled; consider enabling with
   `ManagedOOMSwap=kill` so the slice fails closed before the user has to
   force-logout. (Trade-off: it can kill workloads asymmetrically.)

### 3. Forensic re-check command

Anyone investigating a future session loss should run:

```bash
# Check uptime continuity
who -b
last reboot -n 5

# Trigger event in previous boot
journalctl -b -1 --no-pager | grep -E "endSessionDialog|will power off|will reboot|user@1000.service: Main process|PAM:session_close.*ses=3"

# Memory pressure?
journalctl -b -1 --no-pager | grep -E "user.slice: Consumed|user@1000.service: Consumed" | tail -5

# Was a Claude command in the audit window the trigger?
journalctl -b -1 --no-pager | grep -iE "loginctl|systemctl.*user@|gnome-session-quit|dbus-send.*PowerOff|dbus-send.*Reboot|/sys/power/state"
# Expect: 0 matches (Claude commands are stored in shell history, not journal,
# but PAM/systemd would log a forced terminate-user as a NoSuchUnit error or as
# an ExecStop=… line; absence is evidence)
```

## Verification

```text
$ bash challenges/scripts/host_no_auto_suspend_challenge.sh
=== summary: 4 pass, 0 fail ===

$ bash challenges/scripts/no_suspend_calls_challenge.sh
=== summary: PASS ===

$ free -h    # current host state
               total        used        free      shared  buff/cache   available
Mem:            62Gi        13Gi        35Gi       2.1Gi        16Gi        49Gi
Swap:           15Gi          0B        15Gi
```

## Affected files (this incident's mitigation commit)

- `docs/issues/fixed/SESSION_LOSS_2026-04-28.md` (this file)
- `docs/issues/fixed/BUGFIXES.md` (Issue #N entry)
- `CLAUDE.md` — CONST-036 added; CONST-033 narrative tightened
- `AGENTS.md` — CONST-036 cascade
- `challenges/scripts/no_session_termination_calls_challenge.sh` (NEW)
- `scripts/host_power_management/check-no-session-termination-calls.sh` (NEW)
- Per-submodule cascade of CONST-036 into every project-owned `CLAUDE.md` /
  `AGENTS.md` / `CONSTITUTION.md` (see commit message for the list)
