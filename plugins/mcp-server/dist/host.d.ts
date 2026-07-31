/**
 * DNS-rebinding (Host header) protection for the SSE transport.
 *
 * HXC-172. The surviving advisory on this package is GHSA-w48q-cv73-mx4w,
 * "@modelcontextprotocol/sdk does not enable DNS rebinding protection by
 * default" (affected `<1.24.0`). That advisory is about a PROTECTIVE SETTING
 * that stays off unless a consumer explicitly turns it on -- so moving the
 * package version does not by itself close it.
 *
 * For this server the situation is sharper than the advisory text. We never
 * construct an SDK transport at all: the SSE path in ./index.ts is a hand-rolled
 * `http.createServer`, and `@modelcontextprotocol/sdk` is a declared dependency
 * that is never imported. The SDK's setting is therefore not merely off, it is
 * ABSENT and unreachable by any version bump -- which makes the defence the
 * advisory describes ours to implement here.
 *
 * THE ATTACK. `server.listen(port)` binds every interface. A page on any site
 * can point its own DNS name at this server's address (classic DNS rebinding),
 * at which point the browser's same-origin bookkeeping is satisfied while the
 * connection lands on us. The request arrives carrying the ATTACKER's name in
 * the Host header. Refusing unexpected Host values is what breaks that chain,
 * and it is the standard defence for any locally-bound HTTP service.
 *
 * THE POLICY, and why it is ON BY DEFAULT with no configuration required.
 * Rebinding fundamentally needs a DNS *name* the attacker controls; an IP
 * literal cannot be rebound, because there is no name lookup to poison. So:
 *
 *   - No Host header at all      -> ALLOWED. Browsers always send one; its
 *                                   absence indicates a non-browser client and
 *                                   is not a rebinding vector.
 *   - Loopback name or address   -> ALLOWED. `localhost`, `127.0.0.0/8`, `::1`.
 *                                   This is what the container health check and
 *                                   host-side access via the published port use,
 *                                   so the shipped deployment needs zero config.
 *   - Any other IP literal       -> ALLOWED. Reaching us by bare address is not
 *                                   rebinding, and refusing it would remove
 *                                   working LAN / container-network access
 *                                   (§11.4.122).
 *   - Any other DNS name         -> REFUSED unless the operator listed it.
 *
 * The escape hatch is configuration, never code: MCP_ALLOWED_HOSTS (comma-
 * separated) or the `allowedHosts` config field names the DNS names a real
 * deployment is reached by. It defaults to EMPTY, and empty is already safe
 * because of the loopback and IP-literal rules above. No hostname of any
 * consuming project is hardcoded here -- this package stays project-unaware and
 * reusable (§11.4.28).
 */
/**
 * Normalise an allowlist from either a comma-separated string (the environment
 * form) or an array (the programmatic form).
 *
 * A literal "*" is REJECTED for the same reason it is rejected in ./cors.ts:
 * accepting it would let the defect return through configuration rather than
 * through code, which is the same hole with a longer paper trail.
 */
export declare function parseAllowedHosts(raw: string | readonly string[] | undefined): string[];
/**
 * Whether a request's Host header may be served.
 *
 * `allowed` holds the operator-configured DNS names (already normalised by
 * parseAllowedHosts). The loopback and IP-literal rules apply regardless of it,
 * so protection is active out of the box with an empty allowlist.
 */
export declare function isHostAllowed(hostHeader: string | undefined, allowed: ReadonlySet<string>): boolean;
//# sourceMappingURL=host.d.ts.map