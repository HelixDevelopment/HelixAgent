#!/bin/sh
# ==========================================================================
# FROZEN RED FIXTURE — NOT PRODUCTION CODE. NEVER WIRED INTO ANY CONTAINER.
# ==========================================================================
# This is a byte-faithful copy of the FIRST-ATTEMPT HXC-334 healthcheck, kept
# solely so `mcp_healthcheck_test.sh` can reproduce its two defects on the real
# broken artifact (Constitution §11.4.115 RED-baseline-on-the-broken-artifact)
# rather than on a paraphrase of it.
#
# DEFECT UNDER TEST: the `nc` branch below validates with three FLAT greps —
# it accepts `protocolVersion` appearing ANYWHERE in the reply — while the
# `node` and `python3` branches structurally require `result.protocolVersion`.
# A reply carrying `protocolVersion` at top level therefore makes this script
# exit 0 under `nc` and exit 1 under `node`/`python3`: same server, opposite
# verdict, decided by which interpreter the image happens to ship.
#
# This file was never committed as live code (the first attempt was still
# uncommitted when the divergence was found in review), so it is preserved here
# as testdata. Do not "fix" it — its brokenness is the point.
# ==========================================================================
P=${MCP_HC_PORT:-9000}; export P
R='{"jsonrpc":"2.0","id":424242,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"hc","version":"1"}}}'
# Whitespace-tolerant: a conformant server may emit `"id": 424242` with spaces.
V(){ echo "$1" | grep -qE '"id"[[:space:]]*:[[:space:]]*424242' && echo "$1" | grep -qE '"result"[[:space:]]*:' && echo "$1" | grep -qE '"protocolVersion"[[:space:]]*:'; }
if command -v nc >/dev/null 2>&1; then
  V "$(printf '%s\n' "$R" | nc -w 5 127.0.0.1 "$P" 2>/dev/null | head -n 1)"
elif command -v node >/dev/null 2>&1; then
  printf '%s\n' "$R" | node -e '
let d="";process.stdin.on("data",c=>d+=c).on("end",()=>{const s=require("net").connect(+process.env.P||9000,"127.0.0.1");let b="",done=0,f=()=>{if(done++)return;try{const o=JSON.parse(b.split("\n").find(x=>x.trim()));process.exit(o.id===424242&&o.result&&o.result.protocolVersion?0:1)}catch(e){process.exit(1)}};s.setTimeout(5000);s.on("connect",()=>s.write(d));s.on("data",x=>{b+=x;if(b.includes("\n")){s.destroy();f()}});s.on("timeout",()=>{s.destroy();process.exit(1)});s.on("error",()=>process.exit(1));s.on("close",()=>{b.includes("\n")?f():process.exit(1)})})'
elif command -v python3 >/dev/null 2>&1; then
  python3 -c 'import socket,sys,json,os
try:
 s=socket.create_connection(("127.0.0.1",int(os.environ.get("P","9000"))),5);s.settimeout(5)
 s.sendall((sys.argv[1]+"\n").encode());b=b""
 while b"\n" not in b:
  c=s.recv(4096)
  if not c: break
  b+=c
 o=json.loads(b.split(b"\n")[0])
 sys.exit(0 if o.get("id")==424242 and o.get("result",{}).get("protocolVersion") else 1)
except Exception: sys.exit(1)' "$R"
else
  echo "MCP-HC: no probe tool (nc/node/python3) present in image" >&2; exit 1
fi
