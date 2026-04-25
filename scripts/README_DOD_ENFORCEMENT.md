# DoD enforcement — operational README

The three scripts in this directory are the structural gates for the universal Definition of Done (see `docs/development/definition-of-done.md`). Without them, the DoD is documentation with no teeth; with them, `make ci-validate-all` refuses to pass until the rules actually hold.

## The three gates

### `scripts/no-silent-skips.sh`
Greps the tree for test skip directives that are not annotated with `SKIP-OK: #<ticket>`. A skipped test without a ticket is invisible debt — this script makes that debt loud.

**Patterns it catches:** `t.Skip(`, `@Ignore`, `xit(`, `.skip(`, `@pytest.mark.skip`, `@unittest.skip`.

**Excluded trees:** `vendor/`, `node_modules/`, `external/`, `MCP/`, `cli_agents/`, `mcp-servers/`, `releases/`, `reports/`, `test-results/`.

**Annotation format:** end the line (or a trailing comment) with `SKIP-OK: #<N>` where `<N>` is a tracked ticket number.

### `scripts/no-mocks-above-unit.sh`
Greps non-unit test trees for in-process fakes that bypass real infrastructure. This is the gate that catches the **"100% test coverage but the product doesn't work"** pathology at its source: integration tests that prove the mock works, not that the system works (CONST-030).

**Patterns it catches:** `httptest.NewServer(`, `httptest.NewTLSServer(`, `httptest.NewRecorder(`, `sqlmock.`, `gomock.`, `mockgen`, `miniredis.`, `redismock.`, `NewMockXxx(` constructors, imports of `*/mocks` or `*/mock` packages, `testify/mock`.

**Scanned trees:** `tests/integration`, `tests/e2e`, `tests/security`, `tests/stress`, `tests/chaos`, `tests/challenge`, `tests/benchmark(s)`, `tests/load`, `tests/performance`, `tests/pentest`, `tests/automation`, `tests/compliance`, `tests/container`, `tests/helixqa`, `tests/precondition`, `tests/race`, `tests/monitoring`, `tests/helixllm`, `tests/standalone`, `tests/optimization`, `tests/fuzz`. Unit tests (`tests/unit/`, `*_test.go` under `internal/...` run with `-short`) are deliberately out of scope — mocks are legitimate there.

**Excluded subdirs within scan trees:** `testutils`, `fixtures`, `testdata`, `vendor`, `node_modules`.

**Two ways a site can be allowed:**
1. **Per-site annotation** — end the line with `// MOCK-OK: #<ticket-or-tag>`. Use for genuine permanent permission (e.g. a real upstream is offline in CI). Mirror the tag in `docs/issues/MOCK_CATEGORIES.md`.
2. **Bulk allowlist** — `scripts/no-mocks-above-unit-allowlist.txt` lists pre-existing sites pending real-infra migration. The gate is **strict-with-allowlist**: any site present in the allowlist is silently allowed; any NEW site fails the build. Drainage rule: the allowlist file should only ever shrink.

**Operations:**
- `make no-mocks-above-unit` — ratchet mode (default). Exits 0 if hits ⊆ allowlist; exit 1 on new sites.
- `make no-mocks-above-unit-all` — audit mode. Lists every hit, ignoring the allowlist.
- `make no-mocks-above-unit-update-allowlist` — regenerate the allowlist to match current state. Use after intentional drainage; commit the diff.

### `scripts/demo-all.sh`
Walks every project-owned module's CLAUDE.md, extracts the first bash code block from the "### Acceptance demo for this module" section, runs it with a timeout, and reports per-module PASS / FAIL / TODO / NO-DEMO. Logs land in `reports/demos/<module>.log`.

**Status classes:**
- `PASS` — demo ran, exit 0.
- `FAIL` — demo ran, exit non-zero (or timed out).
- `TODO` — bash block is still `# TODO` — module owner hasn't filled in a real demo.
- `NO-DEMO` — no CLAUDE.md, or no bash block in the acceptance-demo section.

**Environment knobs:**
- `DEMO_TIMEOUT=<seconds>` (default 180) — per-demo timeout.
- `DEMO_LOG_DIR=<path>` (default `reports/demos`) — where logs go.
- `DEMO_MODULES="ModA ModB"` — run only a subset.
- `DEMO_ALLOW_TODO=1` — treat `TODO` as warning (not failure).
- `DEMO_ALL_WARN_ONLY=1` — exit 0 regardless of outcome (transitional).

## Graduated enforcement — current state and how to strict-ify

When these scripts ship, the codebase already has:
- ~4000 unannotated skips (backlog — inherited, not created by this work). After CONST-030 drainage this dropped to **0** (verify via `make no-silent-skips`).
- ~60 acceptance demos filled in, but most have never been executed against real infrastructure.
- **356 mock-above-unit sites** at the time `no-mocks-above-unit.sh` was added (352 of them `httptest.NewServer/Recorder` — i.e. "integration" tests that don't actually exercise the running binary). All 356 are now frozen in `scripts/no-mocks-above-unit-allowlist.txt`; the gate is strict-with-allowlist and rejects any 357th site.

`ci-validate-all` calls a mix of strict and warn-only variants depending on whether each gate has an allowlist mechanism or a hard backlog:

```make
@$(MAKE) no-silent-skips-warn   # warn-only until backlog = 0
@$(MAKE) no-mocks-above-unit    # strict-with-allowlist (engaged immediately)
@$(MAKE) demo-all-warn          # warn-only until all demos pass
```

Make targets available:

| Target | Behavior | Status |
|---|---|---|
| `make no-silent-skips-warn` | reports violation count, exit 0 | wired into `ci-validate-all` |
| `make no-silent-skips` | reports and exits 1 on any violation | currently OK (backlog = 0) |
| `make no-mocks-above-unit` | strict-with-allowlist; exit 1 on any NEW site | wired into `ci-validate-all` |
| `make no-mocks-above-unit-all` | reports every site, ignoring allowlist | for ad-hoc audits |
| `make no-mocks-above-unit-update-allowlist` | regenerate the allowlist file | after intentional drainage |
| `make demo-all-warn` | runs demos, reports, exit 0 regardless | wired into `ci-validate-all` |
| `make demo-all` | runs demos, exits 1 on FAIL/TODO/NO-DEMO | when all demos pass |
| `make demo-one MOD=EventBus` | run a single module's demo | anytime |

## Graduation procedure

The enforcement arm pays off only once you flip these to strict. The sequence:

### Step 1. Drive the skip backlog to zero
```bash
make no-silent-skips-warn 2>&1 | tee reports/skips.txt
```
Review each violation. Choose one of:
1. Remove the skip (test is obsolete or should run).
2. Fix the skip's root cause (flake, broken infra) so the test can run.
3. Annotate with `SKIP-OK: #<ticket>` and file the ticket.

When `make no-silent-skips` (strict) exits 0, edit the Makefile: change `no-silent-skips-warn` → `no-silent-skips` in the `ci-validate-all` recipe.

### Step 1b. Drain the mock-above-unit allowlist toward zero

The gate is already strict (`ci-validate-all` will fail on any NEW site outside `scripts/no-mocks-above-unit-allowlist.txt`). To shrink the allowlist itself:

```bash
make no-mocks-above-unit-all 2>&1 | tee reports/mocks-above-unit.txt
```
The per-class breakdown at the bottom of the output tells you which fake to attack first. The dominant class today is `httptest.NewServer/Recorder` — these are "integration" tests calling handlers in-process. The fix pattern per site:

1. Boot the real binary: `make build && ./bin/helixagent` (which boots all real containers per the Mandatory Container Orchestration Flow).
2. In the test, replace the `httptest.NewServer(router)` setup with a `http.Client` call to `http://localhost:$HELIXAGENT_PORT_HTTP/...`.
3. Replace `httptest.NewRecorder()` + direct handler calls with the same real HTTP roundtrip.
4. Read the response from the wire, not from the recorder.
5. If the test was implicitly relying on a fake DB / Redis, remove the fake — the real binary already has real Postgres + Redis attached.
6. Run `make no-mocks-above-unit-update-allowlist` to remove the now-fixed sites from the allowlist; commit the smaller allowlist alongside the test changes.

If a specific site genuinely cannot use the real artifact (e.g. a contract test for an upstream that's offline in CI), annotate it: `// MOCK-OK: #<ticket>` and remove it from the allowlist (the annotation is a stronger, per-site permission).

The gate exits 0 when allowlist size = 0 with no annotations needed — that is full strict.

### Step 2. Drive the demo backlog to green
```bash
make demo-all-warn 2>&1 | tee reports/demos.txt
grep -E '^\[(FAIL|TODO|NO-DEMO)\]' reports/demos.txt
```
For each failing module:
- If the demo needs infrastructure, boot it (`./bin/helixagent`) before running.
- If the demo is still `TODO`, fill it in with a real command.
- If the demo is inaccurate, refine it — prefer README Quick Start examples as the starting point.

When `make demo-all` (strict) exits 0 in a clean session, edit the Makefile: change `demo-all-warn` → `demo-all` in the `ci-validate-all` recipe.

### Step 3. Review adversarially
Once strict, run `/ultrareview` (or equivalent) on every PR so the gate is not just `ci-validate-all` green but also a human who looked for pasted demo output in the PR body. This closes the last loophole — an LLM that shuts off `DEMO_ALL_WARN_ONLY` can still pass `ci-validate-all` but can't pass an adversarial reviewer who asks "paste your demo output."

## Why graduated

The usual failure mode of enforcement arms is to ship them hot, break the build, and get disabled within a week. Warn-only shipping lets you:
1. See the real size of the backlog without fighting reds.
2. Show the scripts are running and producing useful output.
3. Pick a realistic graduation date.
4. Flip with a one-line Makefile change.

If you'd rather ship hot: swap the `-warn` suffixes out in the `ci-validate-all` recipe and fix what breaks. Both modes are supported.
