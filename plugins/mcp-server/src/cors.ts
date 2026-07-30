/**
 * CORS origin allowlist for the SSE transport.
 *
 * HXC-212. Under the SSE transport this server answered every cross-origin
 * request with a blanket "Access-Control-Allow-Origin: *" and never read the
 * request Origin at all -- on BOTH the OPTIONS preflight and the ordinary
 * POST /mcp response. Any web page on any site, loaded by anyone able to route
 * to the published port, could therefore drive the entire MCP surface
 * (initialize, tools/list, tools/call, resources/read) and read back every
 * response.
 *
 * WILDCARD, not reflection: the literal "*" was written on every response, so
 * the hole did not even require the attacker page to be reachable by name.
 *
 * Behaviour now, mirroring the already-hardened allowlist used by the sibling
 * services rather than inventing a second dialect:
 *
 *   - Origin present AND on the allowlist -> echo back that SPECIFIC origin,
 *     never "*", plus "Vary: Origin" so a shared cache cannot hand one
 *     origin's response to another.
 *   - Origin present and NOT on the allowlist -> no Allow-Origin header at
 *     all. The browser blocks the caller from reading the response.
 *   - No Origin header -> untouched. Non-browser clients (the MCP CLI agents
 *     that actually drive this server) send no Origin, ignore CORS headers
 *     entirely, and are unaffected by the default-deny allowlist.
 *
 * The advertised methods/headers and the 204 preflight short-circuit are
 * preserved exactly as before: this restricts WHO may read a response, it
 * removes no capability.
 *
 * The allowlist is CONFIGURATION, never code. It arrives via
 * MCP_ALLOWED_ORIGINS (comma-separated) or the `allowedOrigins` config field
 * and defaults to EMPTY, i.e. default-deny. No origin and no hostname of any
 * consuming project is hardcoded here -- this package stays project-unaware
 * and reusable.
 */

/**
 * Normalise an allowlist from either a comma-separated string (the environment
 * form) or an array (the programmatic form).
 *
 * A literal "*" is REJECTED on purpose: accepting it would let the exact
 * defect above return through configuration rather than through code, which is
 * the same hole with a longer paper trail.
 */
export function parseAllowedOrigins(raw: string | readonly string[] | undefined): string[] {
  if (raw === undefined || raw === null) return [];

  const parts = Array.isArray(raw)
    ? (raw as readonly string[])
    : String(raw).split(',');

  const out: string[] = [];
  for (const part of parts) {
    const origin = String(part).trim();
    if (origin === '' || origin === '*') continue;
    if (!out.includes(origin)) out.push(origin);
  }
  return out;
}

/**
 * The CORS headers owed to this request's Origin.
 *
 * Returns an EMPTY object when the request carries no Origin, or carries one
 * that is not on the allowlist -- in both cases no Allow-Origin header is
 * emitted at all.
 */
export function originHeaders(
  origin: string | undefined,
  allowed: ReadonlySet<string>
): Record<string, string> {
  if (!origin) return {};
  if (!allowed.has(origin)) return {};
  return {
    'Access-Control-Allow-Origin': origin,
    Vary: 'Origin',
  };
}
