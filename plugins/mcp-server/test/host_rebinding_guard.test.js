#!/usr/bin/env node
'use strict';

/**
 * HXC-172 — DNS-rebinding (Host header) protection + browser side-effect denial
 * for the SSE transport.
 *
 * WHY THIS EXISTS, AND WHY NO DEPENDENCY BUMP CAN REPLACE IT.
 *
 * The surviving advisory on this package is GHSA-w48q-cv73-mx4w,
 * "@modelcontextprotocol/sdk does not enable DNS rebinding protection by
 * default" (<1.24.0). That advisory describes a PROTECTIVE SETTING that is off
 * unless a consumer explicitly turns it on -- so upgrading the package does not
 * by itself close it.
 *
 * For THIS server the finding is sharper than the advisory text. We never
 * construct an SDK transport at all: `runSSE()` in src/index.ts is a hand-rolled
 * `http.createServer`. So the SDK's setting is not merely off -- it is absent,
 * and unreachable by any version bump. The defence the advisory is about is
 * therefore OURS to implement, and before this guard existed we had not:
 *
 *   (1) NO Host-header validation anywhere, and `server.listen(port)` binds every
 *       interface. A page on any site could DNS-rebind its own name to this
 *       server's address and reach the full MCP surface.
 *   (2) A NON-ALLOWLISTED ORIGIN WAS STILL SERVED. The HXC-212 CORS fix
 *       correctly withholds Access-Control-Allow-Origin, which stops a browser
 *       from READING the response -- but the request was still executed first.
 *       CORS governs reads, not side effects, so `tools/call` from a hostile
 *       page still ran. Blocking the read is not blocking the call.
 *
 * ONE SOURCE, TWO ROLES (§11.4.115 polarity switch):
 *
 *   RED_MODE=1 (default) -- reproduce both defects on a PRE-FIX artifact and
 *     assert they are genuinely PRESENT. A RED run that passes on a FIXED
 *     artifact means a hole came back.
 *
 *   RED_MODE=0 -- the standing GREEN regression guard: assert both defects are
 *     ABSENT, and that every legitimate caller still works (loopback Host, an
 *     operator-configured Host, an IP-literal Host, no-Origin CLI clients, the
 *     container health check, and an allowlisted browser origin).
 *
 * REAL END-TO-END, NOT A UNIT STUB. This spawns the actual built artifact
 * (dist/index.js) as a real child process under the container's environment
 * shape (MCP_TRANSPORT=sse) and makes real HTTP requests over a real socket, so
 * it exercises the ARTIFACT layer and not merely the source (§11.4.108): a stale
 * dist/ that still lacks the check is caught here.
 *
 * Every host and origin below is supplied BY THIS TEST through the same
 * environment variables an operator would use. No deployment hostname is baked
 * into the server (§11.4.28 -- this package stays project-unaware).
 */

const assert = require('assert');
const http = require('http');
const path = require('path');
const net = require('net');
const { spawn } = require('child_process');

const RED_MODE = (process.env.RED_MODE ?? '1').trim() !== '0';

// Reserved-for-documentation names that can never be a real deployment of ours.
const HOSTILE_HOST = 'evil.example';
const HOSTILE_ORIGIN = 'https://evil.example';
// Injected by this test via MCP_ALLOWED_HOSTS / MCP_ALLOWED_ORIGINS.
const CONFIGURED_HOST = 'mcp.allowed.test';
const ALLOWED_ORIGIN = 'https://allowed.test';

const SERVER_ENTRY = path.join(__dirname, '..', 'dist', 'index.js');
const BOOT_TIMEOUT_MS = 20000;

function log(...args) {
  console.log(...args);
}

/** Ask the OS for a free port, then release it. */
function freePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
  });
}

/**
 * One real HTTP request. Always CONNECTS to 127.0.0.1; `hostHeader` overrides
 * only the Host header that travels on the wire, which is exactly the shape a
 * DNS-rebinding attack produces (connection lands on us, Host names the
 * attacker).
 */
function request({ port, method, path: urlPath, headers = {}, body, hostHeader }) {
  return new Promise((resolve, reject) => {
    const finalHeaders = { ...headers };
    if (hostHeader !== undefined) finalHeaders.Host = hostHeader;
    const req = http.request(
      { host: '127.0.0.1', port, method, path: urlPath, headers: finalHeaders },
      (res) => {
        let data = '';
        res.setEncoding('utf8');
        res.on('data', (chunk) => { data += chunk; });
        res.on('end', () => {
          resolve({ status: res.statusCode, headers: res.headers, body: data });
        });
      }
    );
    req.on('error', reject);
    if (body !== undefined) req.write(body);
    req.end();
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Spawn the real server and wait until it genuinely answers on /health. */
async function startServer(port, extraEnv) {
  const child = spawn(process.execPath, [SERVER_ENTRY], {
    env: {
      ...process.env,
      MCP_TRANSPORT: 'sse',
      MCP_PORT: String(port),
      // Deliberately unroutable: the upstream probe fails fast and is swallowed
      // by design, so no backend is needed to exercise the HTTP edge.
      HELIXAGENT_URL: 'http://127.0.0.1:1',
      ...extraEnv,
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  let stderr = '';
  child.stderr.setEncoding('utf8');
  child.stderr.on('data', (c) => { stderr += c; });
  child.stdout.resume();

  let exited = null;
  child.on('exit', (code, signal) => { exited = { code, signal }; });

  const deadline = Date.now() + BOOT_TIMEOUT_MS;
  while (Date.now() < deadline) {
    if (exited) {
      throw new Error(
        `server exited before listening (code=${exited.code} signal=${exited.signal})\n--- stderr ---\n${stderr}`
      );
    }
    try {
      const res = await request({ port, method: 'GET', path: '/health' });
      if (res.status === 200) return child;
    } catch {
      // not up yet
    }
    await sleep(150);
  }

  child.kill('SIGKILL');
  throw new Error(`server never became healthy on port ${port}\n--- stderr ---\n${stderr}`);
}

async function stopServer(child) {
  if (!child || child.exitCode !== null) return;
  child.kill('SIGTERM');
  const deadline = Date.now() + 3000;
  while (child.exitCode === null && Date.now() < deadline) await sleep(50);
  if (child.exitCode === null) child.kill('SIGKILL');
}

// `initialize` is used as the probe because a successful one returns real
// serverInfo -- proof the request was genuinely EXECUTED, not merely accepted
// at the socket. That distinction is the whole point of defect (2).
const MCP_BODY = JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'initialize' });

function mcpPost(port, { hostHeader, origin } = {}) {
  const headers = {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(MCP_BODY),
  };
  if (origin !== undefined) headers.Origin = origin;
  return request({ port, method: 'POST', path: '/mcp', headers, body: MCP_BODY, hostHeader });
}

/** True when the response is a real, executed MCP initialize result. */
function isExecutedMcpResult(res) {
  if (res.status !== 200) return false;
  try {
    const parsed = JSON.parse(res.body);
    return Boolean(parsed && parsed.result && parsed.result.serverInfo);
  } catch {
    return false;
  }
}

async function main() {
  const port = await freePort();
  log(`RED_MODE=${RED_MODE ? 1 : 0}  entry=${SERVER_ENTRY}  port=${port}`);

  const child = await startServer(port, {
    MCP_ALLOWED_HOSTS: ` ${CONFIGURED_HOST} , `,
    MCP_ALLOWED_ORIGINS: ALLOWED_ORIGIN,
  });

  try {
    // --- the two attack shapes ---
    const rebinding = await mcpPost(port, { hostHeader: `${HOSTILE_HOST}:${port}` });
    const hostileOrigin = await mcpPost(port, { origin: HOSTILE_ORIGIN });

    // --- the legitimate callers that must keep working ---
    const loopbackName = await mcpPost(port, { hostHeader: `localhost:${port}` });
    const loopbackIp = await mcpPost(port, { hostHeader: `127.0.0.1:${port}` });
    const configuredHost = await mcpPost(port, { hostHeader: `${CONFIGURED_HOST}:${port}` });
    const noOrigin = await mcpPost(port, {});
    const allowedOrigin = await mcpPost(port, { origin: ALLOWED_ORIGIN });
    const health = await request({ port, method: 'GET', path: '/health' });

    const show = (label, res) =>
      log(`  ${label}: status=${res.status} executed=${isExecutedMcpResult(res)}`);

    show(`POST /mcp  Host: ${HOSTILE_HOST}:${port}   (DNS rebinding)`, rebinding);
    show(`POST /mcp  Origin: ${HOSTILE_ORIGIN}  (hostile browser)`, hostileOrigin);
    show(`POST /mcp  Host: localhost:${port}       (loopback name)`, loopbackName);
    show(`POST /mcp  Host: 127.0.0.1:${port}       (loopback ip)`, loopbackIp);
    show(`POST /mcp  Host: ${CONFIGURED_HOST}:${port} (configured)`, configuredHost);
    show('POST /mcp  (no Origin, CLI agent)', noOrigin);
    show(`POST /mcp  Origin: ${ALLOWED_ORIGIN} (allowed browser)`, allowedOrigin);
    log(`  GET  /health: status=${health.status}`);

    if (RED_MODE) {
      // Defect (1): no Host validation -- a rebound name is served in full.
      assert.ok(
        isExecutedMcpResult(rebinding),
        'RED expected the rebinding Host to be SERVED (no Host validation), but it was refused'
      );
      log(`RED reproduced: POST /mcp with Host: ${HOSTILE_HOST}:${port} -> executed, real serverInfo returned`);

      // Defect (2): a non-allowlisted Origin is still EXECUTED. The HXC-212
      // fix withholds the Allow-Origin header (so the page cannot read the
      // reply) but the call itself already ran -- side effects land.
      assert.ok(
        isExecutedMcpResult(hostileOrigin),
        'RED expected the hostile Origin request to still be EXECUTED, but it was refused'
      );
      assert.strictEqual(
        hostileOrigin.headers['access-control-allow-origin'], undefined,
        'RED sanity: the HXC-212 CORS fix should already be withholding Allow-Origin here'
      );
      log(`RED reproduced: POST /mcp from ${HOSTILE_ORIGIN} -> executed (side effect lands) while the response stays unreadable`);

      log('RED PASS — both defects are present: no Host validation, and a non-allowlisted Origin is still executed.');
      return;
    }

    // ---- GREEN: the standing regression guard ----

    // 1. DNS rebinding is refused, and refused BEFORE execution.
    assert.ok(
      !isExecutedMcpResult(rebinding),
      `a rebinding Host (${HOSTILE_HOST}) was executed -- DNS-rebinding protection is off`
    );
    assert.strictEqual(
      rebinding.status, 403,
      `a rebinding Host should be refused with 403, got ${rebinding.status}`
    );

    // 2. A non-allowlisted Origin is refused OUTRIGHT, not merely made
    //    unreadable. Withholding Allow-Origin alone leaves the side effect.
    assert.ok(
      !isExecutedMcpResult(hostileOrigin),
      `a request from ${HOSTILE_ORIGIN} was still EXECUTED -- blocking the read is not blocking the call`
    );
    assert.strictEqual(
      hostileOrigin.status, 403,
      `a hostile Origin should be refused with 403, got ${hostileOrigin.status}`
    );
    assert.strictEqual(
      hostileOrigin.headers['access-control-allow-origin'], undefined,
      'a refused origin must still never receive an Allow-Origin header'
    );

    // 3. §11.4.122 -- no capability removed. Every legitimate caller still
    //    works. A guard that only tested rejection would not notice a "fix"
    //    that blocks everything, which is why each of these is asserted to
    //    return a REAL executed result and not merely a 200.
    assert.ok(isExecutedMcpResult(loopbackName), 'a loopback-name Host was refused -- the health check and host-side access would break');
    assert.ok(isExecutedMcpResult(loopbackIp), 'a loopback-IP Host was refused -- the container health check would break');
    assert.ok(isExecutedMcpResult(configuredHost), `the operator-configured Host ${CONFIGURED_HOST} was refused -- MCP_ALLOWED_HOSTS is not honoured`);
    assert.ok(isExecutedMcpResult(noOrigin), 'a no-Origin CLI client was refused -- the agents that actually drive this server would break');
    assert.ok(isExecutedMcpResult(allowedOrigin), `the allowlisted browser origin ${ALLOWED_ORIGIN} was refused`);
    assert.strictEqual(
      allowedOrigin.headers['access-control-allow-origin'], ALLOWED_ORIGIN,
      'the allowlisted origin stopped receiving its specific Allow-Origin header'
    );
    assert.strictEqual(health.status, 200, '/health stopped answering');

    log('GREEN PASS — rebinding Host and hostile Origin both refused with 403 before execution; loopback / configured / IP-literal hosts, no-Origin CLI clients, allowlisted browser origin and /health all still work.');
  } finally {
    await stopServer(child);
  }
}

main().then(
  () => process.exit(0),
  (err) => {
    console.error(`${RED_MODE ? 'RED' : 'GREEN'} FAIL: ${err && err.message ? err.message : err}`);
    process.exit(1);
  }
);
