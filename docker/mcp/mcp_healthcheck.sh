#!/bin/sh
# Genuine MCP liveness check — performs a real JSON-RPC `initialize` round-trip
# and requires a well-formed reply.
#
# WHY THIS EXISTS (HXC-334):
#   The previous check was ["CMD","nc","-z","localhost","9000"]. These images run
#     socat TCP-LISTEN:9000,reuseaddr,fork SYSTEM:"$CMD"
#   so socat accepts a TCP connection whether or not $CMD works. `nc -z` therefore
#   reported `healthy` for servers that crash on every single connection
#   (mcp-git / mcp-time crashed with ModuleNotFoundError) and for servers that
#   print CLI usage text instead of speaking MCP (mcp-github). It proves a socket
#   accepts connections, and nothing about the server behind it.
#
# CONTRACT (the single semantic predicate, identical in every branch):
#   exit 0 <=> the first reply line parses as JSON AND
#              top-level `id` === 424242 (unquoted number, our request id) AND
#              top-level `result` is an OBJECT AND
#              `protocolVersion` is a DIRECT key of that `result` object with a
#              truthy value.
#   exit 1 <=> anything else: connected but silent / malformed / non-MCP / a
#              JSON-RPC `error` reply / `protocolVersion` present but NOT nested
#              inside `result` / nested one level too deep (e.g. inside
#              `result.serverInfo`) / no usable probe tool in the image.
#   The check never silently passes: every failure path exits 1, and the
#   no-tooling path also prints a diagnostic to stderr.
#
# WHY TRANSPORT AND VALIDATION ARE SEPARATE (HXC-334 round 2):
#   An earlier revision implemented three SELF-CONTAINED branches: the `nc`
#   branch did three flat greps ("id", "result", "protocolVersion" each present
#   ANYWHERE in the reply) while the `node` and `python3` branches did a
#   structural parse of `result.protocolVersion`. Those are NOT equivalent. A
#   reply carrying `protocolVersion` at the TOP level instead of inside `result`
#   satisfied all three greps but no structural parse — so the same server was
#   reported healthy or unhealthy purely according to which interpreter its image
#   happened to ship. That is a determinism defect (Constitution §11.4.50) and a
#   latent false-PASS in the branch that served the large majority of images.
#   The fix: transport (fetch the bytes) and validation (judge the bytes) are now
#   orthogonal. Validation has three implementations — node, python3, awk — all
#   of which structurally parse the reply and enforce the SAME predicate above.
#   `docker/mcp/mcp_healthcheck_test.sh` proves that equivalence with a fixture
#   matrix (every fixture, every validator, identical verdict).
#
#   The SAME defect class then turned up one layer down, in TRANSPORT. `printf |
#   nc` lets printf exit at once; a netcat that half-closes on stdin EOF makes
#   socat tear the connection down, so a healthy-but-not-instant server's reply
#   is lost and it is reported unhealthy — while node and python3, which hold the
#   socket open, report it healthy. Found on the real fleet (mcp-redis answers in
#   0.56s: nc said unhealthy 5/5, node/python3 said healthy). Fixed by preferring
#   node/python3 for transport and making the nc branch hold its stdin open.
#   Fixture `good_reply_lost_if_client_half_closes` reproduces it deterministically;
#   an instant-reply fixture CANNOT catch it, which is why it was missed first time.
#
# PORTABILITY: image tooling is heterogeneous. Measured across the 29 running
#   MCP containers: every image has `sh` and `awk`; mcp-notion has no `nc`
#   (node transport carries it); mcp-github and mcp-slack have neither `node`
#   nor `python3` (awk validation carries them). Transport is tried node ->
#   python3 -> nc, validation node -> python3 -> awk. If NO transport or NO
#   validator exists the check fails loudly rather than degrading to a weaker
#   test — there is deliberately no grep-based fallback, because a weaker
#   fallback is exactly the false-PASS this file exists to remove.
#
# SELINUX CAVEAT: this script is bind-mounted into each container read-only. The
#   host this was validated on runs rootless podman WITHOUT SELinux (no
#   /sys/fs/selinux, `selinux=false`), so no relabel option is required. On an
#   SELinux-ENFORCING host the mount in docker-compose.mcp-servers.yml needs the
#   `:z` (shared) or `:Z` (private) relabel suffix
#     - ./mcp_healthcheck.sh:/mcp_healthcheck.sh:ro,z
#   or every container will be denied read access and every healthcheck will
#   report unhealthy for a reason unrelated to the MCP server.
#
# SCOPE NOTE — THIS DOES NOT FIX THE LOG SPAM: the underlying defect is that
#   socat's `fork` accepts every connection and spawns a doomed child for broken
#   servers, which floods the container log: mcp-git measured 272,658 lines via
#   `podman logs --tail 400000 helixagent-mcp-git | wc -l` (the 400k bound was
#   NOT reached, so that is a true count, and it keeps growing). Do NOT measure
#   this with `timeout N podman logs` — that TRUNCATES a still-streaming log and
#   yields a number that rises with N (6s->65,826, 20s->222,310, 60s->267,605),
#   which is an artefact of the instrument, not the log size. Raising the
#   healthcheck interval from 10s to 30s slows that
#   accumulation ~3x but each probe still forks one doomed child. Stopping the
#   spam requires fixing the wrapped servers (or making socat's SYSTEM command
#   fail fast), which is deliberately out of scope for this change.
#
# ENVIRONMENT OVERRIDES
#   MCP_HC_PORT       target port                       (default 9000)
#   MCP_HC_HOST       target host                       (default 127.0.0.1)
#   MCP_HC_TIMEOUT    per-probe timeout in seconds      (default 5)
#   MCP_HC_TRANSPORT  auto | nc | node | python3        (default auto)
#   MCP_HC_VALIDATOR  auto | node | python3 | awk       (default auto)
#   The two selector overrides exist so the test harness can force each
#   implementation and prove they agree; production leaves them unset.

set -u

MCP_HC_PORT=${MCP_HC_PORT:-9000}
MCP_HC_HOST=${MCP_HC_HOST:-127.0.0.1}
MCP_HC_TIMEOUT=${MCP_HC_TIMEOUT:-5}
MCP_HC_TRANSPORT=${MCP_HC_TRANSPORT:-auto}
MCP_HC_VALIDATOR=${MCP_HC_VALIDATOR:-auto}

MCP_HC_REQ='{"jsonrpc":"2.0","id":424242,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"hc","version":"1"}}}'

export MCP_HC_PORT MCP_HC_HOST MCP_HC_TIMEOUT MCP_HC_REQ

# ---------------------------------------------------------------------------
# TRANSPORT: send the initialize request, echo the FIRST reply line on stdout.
# A transport never judges the reply; an empty stdout means "nothing came back"
# and the validator will reject it.
# ---------------------------------------------------------------------------

# nc is the LAST-RESORT transport (see hc_pick_transport). It must hold its stdin
# open for the whole probe: `printf ... | nc` lets printf exit immediately, and a
# netcat that half-closes its write side on stdin EOF makes socat tear the
# connection down before a server that needs a moment to answer has replied.
# Measured on the real fleet: mcp-redis answers in 0.56s and was reported
# UNHEALTHY by `printf | nc` (0/5 replies) while node and python3 both reported
# healthy; holding stdin open made it 5/5. The `sleep` costs the full
# MCP_HC_TIMEOUT per probe even on success — acceptable because this branch is
# reached only by images that have NEITHER node NOR python3 (measured: mcp-github
# and mcp-slack), and 5s still leaves 2x margin under the compose `timeout: 10s`.
hc_tx_nc() {
    { printf '%s\n' "$MCP_HC_REQ"; sleep "$MCP_HC_TIMEOUT"; } \
        | nc -w "$MCP_HC_TIMEOUT" "$MCP_HC_HOST" "$MCP_HC_PORT" 2>/dev/null \
        | head -n 1
}

hc_tx_node() {
    node -e '
const net = require("net");
let buf = "", done = false;
const sock = net.connect(+process.env.MCP_HC_PORT, process.env.MCP_HC_HOST);
const finish = () => {
  if (done) return;
  done = true;
  const nl = buf.indexOf("\n");
  process.stdout.write(nl >= 0 ? buf.slice(0, nl) : buf);
  try { sock.destroy(); } catch (e) { /* already gone */ }
  process.exit(0);
};
sock.setTimeout(+process.env.MCP_HC_TIMEOUT * 1000);
sock.on("connect", () => sock.write(process.env.MCP_HC_REQ + "\n"));
sock.on("data", (chunk) => { buf += chunk; if (buf.indexOf("\n") >= 0) finish(); });
sock.on("timeout", finish);
sock.on("error", finish);
sock.on("close", finish);
' 2>/dev/null
}

hc_tx_python3() {
    python3 -c '
import os, socket, sys
try:
    tmo = float(os.environ["MCP_HC_TIMEOUT"])
    s = socket.create_connection((os.environ["MCP_HC_HOST"], int(os.environ["MCP_HC_PORT"])), tmo)
    s.settimeout(tmo)
    s.sendall((os.environ["MCP_HC_REQ"] + "\n").encode())
    buf = b""
    while b"\n" not in buf:
        chunk = s.recv(4096)
        if not chunk:
            break
        buf += chunk
    sys.stdout.write(buf.split(b"\n")[0].decode("utf-8", "replace"))
except Exception:
    pass
' 2>/dev/null
}

# ---------------------------------------------------------------------------
# VALIDATION: read the reply line on stdin, exit 0 iff the CONTRACT holds.
# Three implementations, one predicate. Kept structurally parallel on purpose so
# a reviewer can diff them by eye.
# ---------------------------------------------------------------------------

hc_val_node() {
    node -e '
let raw = "";
try { raw = require("fs").readFileSync(0, "utf8"); } catch (e) { process.exit(1); }
let o;
try { o = JSON.parse(raw); } catch (e) { process.exit(1); }
const ok = o !== null && typeof o === "object" &&
           o.id === 424242 &&
           o.result !== null && typeof o.result === "object" && !Array.isArray(o.result) &&
           Boolean(o.result.protocolVersion);
process.exit(ok ? 0 : 1);
' 2>/dev/null
}

hc_val_python3() {
    python3 -c '
import json, sys
try:
    o = json.loads(sys.stdin.read())
    r = o.get("result")
    ok = (isinstance(o, dict)
          and not isinstance(o.get("id"), bool)
          and o.get("id") == 424242
          and isinstance(r, dict)
          and bool(r.get("protocolVersion")))
    sys.exit(0 if ok else 1)
except Exception:
    sys.exit(1)
' 2>/dev/null
}

# awk validator: a minimal string-aware JSON scanner. It tracks nesting depth and
# the key that owns each level, so `protocolVersion` is accepted ONLY as a direct
# key of the top-level `result` object — not at top level, not inside
# `result.serverInfo`, and not inside a string literal that happens to contain
# braces. This is a real parse, not a regex: it is what makes the awk branch
# semantically identical to the node and python3 branches.
hc_val_awk() {
    awk '
function is_space(ch) { return (ch == " " || ch == "\t" || ch == "\r") }
function truthy(v, quoted) {
    if (quoted) return (length(v) > 0)
    if (v == "" || v == "null" || v == "false" || v == "0") return 0
    return 1
}
NR == 1 {
    line = $0
    n = length(line)
    depth = 0; i = 1; ok_id = 0; ok_pv = 0; saw_json = 0
    while (i <= n) {
        c = substr(line, i, 1)
        if (c == "\"") {
            # --- read a string token ---
            s = ""; i++
            while (i <= n) {
                ch = substr(line, i, 1)
                if (ch == "\\") { s = s substr(line, i + 1, 1); i += 2; continue }
                if (ch == "\"") { i++; break }
                s = s ch; i++
            }
            # --- is it a KEY? (next non-space char is a colon) ---
            j = i
            while (j <= n && is_space(substr(line, j, 1))) j++
            if (j <= n && substr(line, j, 1) == ":") {
                i = j + 1
                while (i <= n && is_space(substr(line, i, 1))) i++
                vc = substr(line, i, 1)
                if (vc == "{" || vc == "[") {
                    # structural value: remember which key owns the level we are
                    # about to enter, then let the main loop do the depth++
                    key[depth] = s
                    continue
                }
                if (vc == "\"") {
                    v = ""; i++
                    while (i <= n) {
                        ch = substr(line, i, 1)
                        if (ch == "\\") { v = v substr(line, i + 1, 1); i += 2; continue }
                        if (ch == "\"") { i++; break }
                        v = v ch; i++
                    }
                    quoted = 1
                } else {
                    v = ""
                    while (i <= n) {
                        ch = substr(line, i, 1)
                        if (ch == "," || ch == "}" || ch == "]" || is_space(ch)) break
                        v = v ch; i++
                    }
                    quoted = 0
                }
                if (depth == 1 && s == "id" && quoted == 0 && v ~ /^[0-9]+$/ && v + 0 == 424242) ok_id = 1
                if (depth == 2 && key[1] == "result" && s == "protocolVersion" && truthy(v, quoted)) ok_pv = 1
            }
            continue
        }
        if (c == "{" || c == "[") { saw_json = 1; depth++; i++; continue }
        if (c == "}" || c == "]") { delete key[depth]; depth--; i++; continue }
        i++
    }
    if (saw_json && ok_id && ok_pv) exit 0
    exit 1
}
END { if (NR == 0) exit 1 }
' 2>/dev/null
}

# ---------------------------------------------------------------------------
# SELECTION
# ---------------------------------------------------------------------------

hc_pick_transport() {
    if [ "$MCP_HC_TRANSPORT" != "auto" ]; then
        if command -v "$MCP_HC_TRANSPORT" >/dev/null 2>&1; then
            printf '%s' "$MCP_HC_TRANSPORT"; return 0
        fi
        return 1
    fi
    # node/python3 FIRST: they keep the write side of the socket open for the
    # whole probe, so they cannot lose the half-close race described above.
    # nc is last-resort (and self-defends with the stdin hold).
    for t in node python3 nc; do
        if command -v "$t" >/dev/null 2>&1; then printf '%s' "$t"; return 0; fi
    done
    return 1
}

hc_pick_validator() {
    if [ "$MCP_HC_VALIDATOR" != "auto" ]; then
        if command -v "$MCP_HC_VALIDATOR" >/dev/null 2>&1; then
            printf '%s' "$MCP_HC_VALIDATOR"; return 0
        fi
        return 1
    fi
    for v in node python3 awk; do
        if command -v "$v" >/dev/null 2>&1; then printf '%s' "$v"; return 0; fi
    done
    return 1
}

TX=$(hc_pick_transport) || {
    echo "MCP-HC: no usable transport (nc/node/python3) in image" >&2
    exit 1
}
VAL=$(hc_pick_validator) || {
    echo "MCP-HC: no usable validator (node/python3/awk) in image" >&2
    exit 1
}

REPLY_LINE=$("hc_tx_${TX}")
printf '%s' "$REPLY_LINE" | "hc_val_${VAL}"
