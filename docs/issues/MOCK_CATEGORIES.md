# Legitimate-mock categories

`scripts/no-mocks-above-unit.sh` accepts either a numeric ticket reference
(`MOCK-OK: #1234`) OR a kebab-case category tag (`MOCK-OK: #provider-fault-injection`).
Category tags are for non-unit-test mock sites that are architecturally correct
and have no associated bug ticket — they document why this specific in-process
fake is *not* a CONST-030 violation so reviewers know the mock is intentional.

When introducing a NEW category, add it here first with a one-line rationale.
The taxonomy below is closed — new patterns need a new category entry, not
reuse of an ill-fitting one.

## What separates a legitimate mock from a CONST-030 violation

The gate forbids in-process fakes that substitute for **HelixAgent itself** —
`httptest.NewServer(myRouter)`, `sqlmock` standing in for the real Postgres
HelixAgent uses, etc. These hide the fact that the test never exercised the
running binary.

It does NOT forbid in-process fakes that simulate an **upstream HelixAgent
talks to** — provider APIs (Mistral, Claude, Gemini), OAuth issuers, MCP
servers we don't own, third-party webhook receivers. These are legitimate
because:
1. The real upstream costs money / is rate-limited / requires a network.
2. The real upstream cannot be deterministically driven into the failure
   modes the test needs to exercise (e.g. "first request returns 401 then
   200" — real Mistral won't do that on cue).
3. The unit under test is HelixAgent's *client-side behavior* against a
   contract, not the upstream itself.

If you are tempted to use a category below for a HelixAgent-substitution
fake, stop — that is the violation the gate was written to catch. Convert
the test to boot `./bin/helixagent` and call it via `http.Client` against
`http://localhost:$HELIXAGENT_PORT_HTTP/...`.

## Upstream simulation (HelixAgent acts as the client)

- `#provider-fault-injection` — fake LLM provider endpoint that returns
  specific HTTP status codes (401, 429, 500, slow responses, malformed
  bodies) on demand to drive client-side retry / backoff / fallback logic.
  Real provider APIs cannot be coerced into these failure modes
  reproducibly.
- `#provider-contract-test` — fake LLM provider endpoint asserting the
  request shape HelixAgent sends (auth headers, body format, model
  identifier, streaming framing). Verifies HelixAgent obeys the upstream's
  documented contract.
- `#oauth-issuer-fake` — fake OAuth/OIDC issuer for token-flow tests
  (authorization code, refresh, introspection). Real issuers tie tests to
  specific tenant accounts and rate-limit aggressively.
- `#mcp-server-stub` — fake MCP server (third-party protocol endpoint
  HelixAgent connects to). Used when validating HelixAgent's MCP client
  behavior, not the protocol implementation of any specific real server.
- `#webhook-receiver-fake` — fake outbound webhook target to capture
  delivery payloads / retry behavior.

## Library-primitive tests (HTTP loop is the test instrument)

- `#library-http-test` — HelixAgent's own HTTP-adjacent library primitives
  (SSE writer, WebSocket framing, streaming chunkers, custom transports)
  that can ONLY be exercised end-to-end over a real HTTP loop. The test
  is verifying the primitive itself, not any HelixAgent product surface;
  there is no `./bin/helixagent` route that exercises the same code path
  in isolation. `httptest.NewServer` here IS the unit-test instrument —
  there is no in-memory equivalent for "did the SSE framing land on the
  wire correctly?" Examples: `tests/integration/streaming_types_integration_test.go`.

## Pre-existing backlog

- `#legacy-untriaged` — Bulk-allowlisted in `scripts/no-mocks-above-unit-allowlist.txt`
  during the 2026-04-25 mock-backlog sweep. Means "this site existed before
  the gate went in and hasn't been individually reviewed yet; it may be a
  legitimate upstream simulation OR a CONST-030 violation that needs
  conversion to a real-binary roundtrip — review pending."
  Target: drained to zero by classifying each site and either annotating
  with a specific category from above OR converting the test to use the
  real artifact.

---

## How to add a category

1. Pick the category name — short, kebab-case, starts with lowercase.
2. Add it here with a one-line rationale that names the upstream class
   and the specific test concern.
3. Use it as `// MOCK-OK: #<category>` on the same line as the mock
   directive (Go).
4. Run `make no-mocks-above-unit-update-allowlist` to remove the now-annotated
   site(s) from `scripts/no-mocks-above-unit-allowlist.txt`. Commit the
   smaller allowlist alongside the annotation in the same PR.

Category names are cheap — favor specificity over reuse.

## How to drain a site from the allowlist

For each grandfathered site in `scripts/no-mocks-above-unit-allowlist.txt`:

1. Open the file at the listed line.
2. Decide which side of the seam the fake is on:
   - **Substituting for HelixAgent** (in-process router, fake DB, fake
     Redis HelixAgent itself uses) → CONST-030 violation. Convert to a
     real-binary roundtrip: boot `./bin/helixagent` (which boots all real
     containers per the Mandatory Container Orchestration Flow), call it
     via `http.Client` against the configured port. Remove the entry from
     the allowlist by running `make no-mocks-above-unit-update-allowlist`.
   - **Simulating an upstream** (provider API, OAuth issuer, MCP server we
     don't own) → legitimate. Annotate the line with the matching category
     tag from above (or add a new category here first if none fits).
     Regenerate the allowlist; the annotated line is filtered out.
3. The allowlist line count must monotonically decrease over time.
