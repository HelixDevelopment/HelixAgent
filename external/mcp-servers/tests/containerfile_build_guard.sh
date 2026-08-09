#!/usr/bin/env bash
#
# RED/GREEN polarity guard for the shipped MCP-servers Containerfile.
#
# THE DEFECT THIS GUARDS (measured 2026-08-09, pre-fix tree)
#
#   $ podman build -f Dockerfile .
#   Error: building at STEP "COPY servers/ /build/servers/": checking on
#   sources under ".../external/mcp-servers": copier: stat: "/servers":
#   no such file or directory
#
# Commit 6e245ff1 deleted external/mcp-servers/servers (promoted to the
# consuming meta-repo's root) but left the Containerfile's unconditional
# `COPY servers/`, so the shipped image could not be built AT ALL.
#
# POLARITY (§11.4.115) — one source, two roles:
#
#   RED_MODE=1 — reproduce the defect condition.
#       Selects the opt-in active-servers stage (`ACTIVE_SERVERS=present`)
#       while `servers/` is absent from the context. That is the EXACT
#       instruction + EXACT context of the original failure, so a PASS here
#       means "the defect condition is faithfully reproducible" — the proof
#       this guard can actually see the bug.
#
#   RED_MODE=0 (default, post-fix) — the standing regression guard.
#       Builds the shipped Containerfile with DEFAULT build args and asserts
#       it SUCCEEDS: the image must build out of the box from its own build
#       context, with no operator-supplied extras.
#
# INSTRUMENT VALIDITY (§11.4.201): before trusting any negative, this script
# runs a CONTROL NEEDLE — the same podman COPY mechanism against a path we
# KNOW is present (servers-archived/). If the control does not succeed, the
# instrument is broken and its "failure" verdicts mean nothing, so the script
# aborts as INSTRUMENT-INVALID rather than reporting a result.
#
# COST: RED_MODE=0 performs a REAL, full container build (npm + pip + an
# `apk add chromium` layer) — it needs network and takes ~10-20 min cold.
# Once podman has cached the shared base layer, subsequent runs are fast.
# There is deliberately no cheap "grep the Containerfile" shortcut: the real
# condition is "does it build", and a grep would be a proxy, not the fact.

set -uo pipefail

RED_MODE="${RED_MODE:-0}"
HERE="$(cd "$(dirname "$0")" && pwd)"
CTX="$(cd "$HERE/.." && pwd)"
CONTAINERFILE="$CTX/Dockerfile"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log() { printf '%s\n' "$*"; }

# ---- preconditions --------------------------------------------------------
if ! command -v podman >/dev/null 2>&1; then
    log "SKIP: podman is not installed on this host — cannot exercise a real"
    log "      container build. (Honest skip, not a pass: the invariant is"
    log "      untested here.)"
    exit 0
fi
if [ ! -f "$CONTAINERFILE" ]; then
    log "FAIL: no Containerfile at $CONTAINERFILE"
    exit 1
fi

# ---- control needle (§11.4.201) -------------------------------------------
# Prove the instrument can confirm a case we KNOW is true before we let it
# pronounce on a case we claim is false.
printf 'FROM scratch\nCOPY servers-archived/ /probe/\n' > "$WORK/Containerfile.control"
if ! podman build --no-cache -f "$WORK/Containerfile.control" \
        -t mcp-servers-guard-control:probe "$CTX" > "$WORK/control.log" 2>&1; then
    log "INSTRUMENT-INVALID: the control needle failed — podman could not COPY"
    log "  servers-archived/ (a path that IS present) from the build context."
    log "  Any 'not found' verdict from this instrument would be meaningless."
    sed 's/^/  | /' "$WORK/control.log"
    exit 2
fi
podman rmi -f mcp-servers-guard-control:probe >/dev/null 2>&1
log "control-needle: OK (podman resolves a known-present context path)"

# ---- polarity -------------------------------------------------------------
if [ "$RED_MODE" = "1" ]; then
    if [ -e "$CTX/servers" ]; then
        log "SKIP: $CTX/servers exists, so the defect's precondition (absent"
        log "      active-server sources) does not hold on this checkout."
        exit 0
    fi
    log "RED_MODE=1: expecting the opt-in active-servers build to FAIL"
    if podman build --build-arg ACTIVE_SERVERS=present \
            -f "$CONTAINERFILE" -t mcp-servers-guard-red:probe "$CTX" \
            > "$WORK/red.log" 2>&1; then
        log "FAIL: build SUCCEEDED but servers/ is absent — the guard is blind."
        exit 1
    fi
    if grep -q 'COPY servers/' "$WORK/red.log" && grep -qi 'no such file' "$WORK/red.log"; then
        log "PASS: defect reproduced — COPY servers/ cannot resolve:"
        grep -i 'Error:' "$WORK/red.log" | sed 's/^/  | /'
        exit 0
    fi
    log "FAIL: build failed, but NOT at the COPY servers/ step — this run does"
    log "      not reproduce the defect under test."
    tail -20 "$WORK/red.log" | sed 's/^/  | /'
    exit 1
fi

log "RED_MODE=0: expecting the shipped Containerfile to BUILD with default args"
if podman build -f "$CONTAINERFILE" -t mcp-servers-guard-green:probe "$CTX" \
        > "$WORK/green.log" 2>&1; then
    log "PASS: shipped Containerfile builds from its own context."
    tail -3 "$WORK/green.log" | sed 's/^/  | /'
    exit 0
fi
log "FAIL: shipped Containerfile does NOT build with default build args."
tail -30 "$WORK/green.log" | sed 's/^/  | /'
exit 1
