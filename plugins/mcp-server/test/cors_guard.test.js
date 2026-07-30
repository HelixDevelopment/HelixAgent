#!/usr/bin/env node
'use strict';

/**
 * HXC-212 — CORS origin-allowlist guard for the SSE transport.
 *
 * WHAT THIS GUARDS. Under the SSE transport this server answered every
 * cross-origin request with a blanket "Access-Control-Allow-Origin: *" and
 * never read the request Origin at all -- on BOTH the OPTIONS preflight and
 * the ordinary POST /mcp response. Any web page on any site, loaded by anyone
 * able to route to the published port, could therefore drive the full MCP
 * surface (initialize, tools/list, tools/call, resources/read) and read back
 * every response.
 *
 * ONE SOURCE, TWO ROLES (§11.4.115 polarity switch):
 *
 *   RED_MODE=1 (default) -- reproduce the defect on a PRE-FIX artifact and
 *     assert it is genuinely PRESENT. Run this against the broken build to
 *     prove the guard is not blind. A RED run that PASSES on a FIXED artifact
 *     would mean the hole is back.
 *
 *   RED_MODE=0 -- the standing GREEN regression guard: assert the defect is
 *     ABSENT, that a permitted origin is still ACCEPTED, and that a client
 *     sending no Origin at all is unaffected.
 *
 * REAL END-TO-END, NOT A UNIT STUB. This spawns the actual built artifact
 * (dist/index.js) as a real child process under the same environment shape the
 * container uses (MCP_TRANSPORT=sse), waits for it to really listen, and makes
 * real HTTP requests over a real socket. It therefore exercises the ARTIFACT
 * layer, not merely the source: a stale dist/ that still carries the wildcard
 * is caught here (§11.4.108).
 *
 * The permitted origin is supplied BY THIS TEST through the same environment
 * variable an operator would use. No origin and no hostname is baked into the
 * server (§11.4.28 -- this submodule stays project-unaware).
 */

const assert = require('assert');
const http = require('http');
const path = require('path');
const net = require('net');
const { spawn } = require('child_process');

const RED_MODE = (process.env.RED_MODE ?? '1').trim() !== '0';

// Test-supplied values. Nothing here is a real deployment host: the allowed
// origin is what THIS test injects via MCP_ALLOWED_ORIGINS, and the hostile
// origin is a reserved-for-documentation name that can never be ours.
const ALLOWED_ORIGIN = 'https://allowed.test';
const SECOND_ALLOWED_ORIGIN = 'https://also-allowed.test';
const HOSTILE_ORIGIN = 'https://evil.example';

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

/** One real HTTP request. Resolves with status + headers + body. */
function request({ port, method, path: urlPath, headers = {}, body }) {
  return new Promise((resolve, reject) => {
    const req = http.request(
      { host: '127.0.0.1', port, method, path: urlPath, headers },
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

/**
 * Spawn the real server and wait until it genuinely answers on /health.
 * Returns the child process; the caller must kill it.
 */
async function startServer(port, extraEnv) {
  const child = spawn(process.execPath, [SERVER_ENTRY], {
    env: {
      ...process.env,
      MCP_TRANSPORT: 'sse',
      MCP_PORT: String(port),
      // Deliberately unroutable: the server's upstream probe fails fast and is
      // swallowed by design, so no backend is needed to exercise the HTTP edge.
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

const MCP_BODY = JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'initialize' });

/** Preflight for the MCP endpoint, as a browser would send it. */
function preflight(port, origin) {
  const headers = {
    'Access-Control-Request-Method': 'POST',
    'Access-Control-Request-Headers': 'content-type',
  };
  if (origin !== undefined) headers.Origin = origin;
  return request({ port, method: 'OPTIONS', path: '/mcp', headers });
}

/** Ordinary (simple) cross-origin request to the MCP endpoint. */
function simple(port, origin) {
  const headers = {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(MCP_BODY),
  };
  if (origin !== undefined) headers.Origin = origin;
  return request({ port, method: 'POST', path: '/mcp', headers, body: MCP_BODY });
}

function acao(res) {
  return res.headers['access-control-allow-origin'];
}

function varyHas(res, token) {
  const v = res.headers.vary;
  if (!v) return false;
  return String(v)
    .split(',')
    .map((s) => s.trim().toLowerCase())
    .includes(token.toLowerCase());
}

async function main() {
  const port = await freePort();
  log(`RED_MODE=${RED_MODE ? 1 : 0}  entry=${SERVER_ENTRY}  port=${port}`);

  const child = await startServer(port, {
    // Two entries, whitespace, an empty field and a literal "*" -- the parser
    // must keep the two real origins, drop the noise, and REFUSE the wildcard
    // so the defect cannot be reintroduced through configuration.
    MCP_ALLOWED_ORIGINS: ` ${ALLOWED_ORIGIN} , ,${SECOND_ALLOWED_ORIGIN}, * `,
  });

  try {
    const hostilePre = await preflight(port, HOSTILE_ORIGIN);
    const hostileSimple = await simple(port, HOSTILE_ORIGIN);
    const allowedPre = await preflight(port, ALLOWED_ORIGIN);
    const allowedSimple = await simple(port, ALLOWED_ORIGIN);
    const secondAllowedSimple = await simple(port, SECOND_ALLOWED_ORIGIN);
    const noOriginSimple = await simple(port, undefined);
    const noOriginHealth = await request({ port, method: 'GET', path: '/health' });

    const show = (label, res) =>
      log(`  ${label}: status=${res.status} Access-Control-Allow-Origin=${JSON.stringify(acao(res))} Vary=${JSON.stringify(res.headers.vary)}`);

    show('OPTIONS /mcp   Origin: ' + HOSTILE_ORIGIN, hostilePre);
    show('POST    /mcp   Origin: ' + HOSTILE_ORIGIN, hostileSimple);
    show('OPTIONS /mcp   Origin: ' + ALLOWED_ORIGIN, allowedPre);
    show('POST    /mcp   Origin: ' + ALLOWED_ORIGIN, allowedSimple);
    show('POST    /mcp   Origin: ' + SECOND_ALLOWED_ORIGIN, secondAllowedSimple);
    show('POST    /mcp   (no Origin header)', noOriginSimple);
    show('GET     /health(no Origin header)', noOriginHealth);

    if (RED_MODE) {
      // Reproduce the defect on the pre-fix artifact. BOTH sites are asserted:
      // a check applied to the preflight but not the ordinary response (or the
      // reverse) is exactly the sibling-miss this class keeps producing.
      assert.strictEqual(
        acao(hostilePre), '*',
        'RED expected the wildcard on the OPTIONS preflight, and it was not there'
      );
      log(`RED reproduced: OPTIONS /mcp from ${HOSTILE_ORIGIN} -> Access-Control-Allow-Origin="*"`);

      assert.strictEqual(
        acao(hostileSimple), '*',
        'RED expected the wildcard on the ordinary POST response, and it was not there'
      );
      log(`RED reproduced: POST /mcp from ${HOSTILE_ORIGIN} -> Access-Control-Allow-Origin="*"`);

      log('RED PASS — the permissive-origin defect is present on both the preflight and the ordinary request.');
      return;
    }

    // ---- GREEN: the standing regression guard ----

    // 1. Hostile origin is refused on BOTH paths. No Allow-Origin header at
    //    all, so the browser blocks the caller from reading the response.
    assert.strictEqual(
      acao(hostilePre), undefined,
      `preflight handed an Access-Control-Allow-Origin to ${HOSTILE_ORIGIN}`
    );
    assert.strictEqual(
      acao(hostileSimple), undefined,
      `ordinary response handed an Access-Control-Allow-Origin to ${HOSTILE_ORIGIN}`
    );

    // 2. A literal "*" must never appear on any response, from any origin.
    for (const [label, res] of [
      ['hostile preflight', hostilePre],
      ['hostile simple', hostileSimple],
      ['allowed preflight', allowedPre],
      ['allowed simple', allowedSimple],
      ['no-Origin simple', noOriginSimple],
    ]) {
      assert.notStrictEqual(acao(res), '*', `wildcard origin returned on the ${label} response`);
    }

    // 3. A permitted origin is still ACCEPTED -- echoed back specifically,
    //    never as "*", with Vary: Origin so a shared cache cannot hand one
    //    origin's response to another. A guard that only tested rejection
    //    would not notice a fix that blocks everything.
    assert.strictEqual(acao(allowedPre), ALLOWED_ORIGIN, 'permitted origin was refused on the preflight');
    assert.ok(varyHas(allowedPre, 'Origin'), 'preflight response is missing Vary: Origin');
    assert.strictEqual(acao(allowedSimple), ALLOWED_ORIGIN, 'permitted origin was refused on the ordinary request');
    assert.ok(varyHas(allowedSimple, 'Origin'), 'ordinary response is missing Vary: Origin');

    // 4. Every entry of the allowlist works, not just the first.
    assert.strictEqual(
      acao(secondAllowedSimple), SECOND_ALLOWED_ORIGIN,
      'the second allowlist entry was refused -- only the first entry is honoured'
    );

    // 5. The literal "*" present in MCP_ALLOWED_ORIGINS was REJECTED: had the
    //    parser accepted it, the hostile origin above would have been allowed.
    //    Assertion 1 already proves this; stated here so the intent is explicit.

    // 6. Clients that send no Origin are untouched. Non-browser callers (the
    //    MCP CLI agents that actually drive this server) send no Origin,
    //    ignore CORS entirely, and must keep working under a default-deny
    //    allowlist.
    assert.strictEqual(noOriginSimple.status, 200, 'a client sending no Origin was rejected');
    assert.strictEqual(acao(noOriginSimple), undefined, 'an Allow-Origin header was invented for a request with no Origin');
    const parsed = JSON.parse(noOriginSimple.body);
    assert.strictEqual(parsed.jsonrpc, '2.0', 'no-Origin client did not get a JSON-RPC response');
    assert.ok(parsed.result && parsed.result.serverInfo, 'no-Origin client did not get a real initialize result');
    assert.strictEqual(noOriginHealth.status, 200, '/health stopped answering');

    // 7. §11.4.122 -- no capability was removed. The preflight still
    //    short-circuits with 204 and still advertises the methods and headers
    //    it advertised before; only WHO may read a response changed.
    assert.strictEqual(hostilePre.status, 204, 'the 204 preflight short-circuit was removed');
    assert.strictEqual(allowedPre.status, 204, 'the 204 preflight short-circuit was removed');
    for (const res of [hostilePre, allowedPre]) {
      assert.ok(
        (res.headers['access-control-allow-methods'] || '').includes('POST'),
        'the preflight stopped advertising the methods it advertised before'
      );
      assert.ok(
        (res.headers['access-control-allow-headers'] || '').toLowerCase().includes('content-type'),
        'the preflight stopped advertising the headers it advertised before'
      );
    }

    log('GREEN PASS — hostile origin denied on preflight AND ordinary request; permitted origins accepted with Vary: Origin; no-Origin clients unaffected; preflight capability preserved.');
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
