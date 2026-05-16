# Definition of Done

**Scope:** universal. Applies to the root HelixAgent repo and every project-owned submodule. Inherited by reference from each submodule's `CLAUDE.md`.

## Why this document exists

LLM-driven development (Claude Code with Opus and equivalents) reliably produces this failure pattern: near-100% test coverage, green suites, disappointing product. Manual testing reveals that most of the work does not actually work.

The pattern is not caused by insufficient planning, inadequate task decomposition, or missing detail. Projects with exhaustive phase plans and granular tasks exhibit it just as severely. The cause is structural:

> **When the LLM writes both the code and the tests, the tests assert the shape of the LLM's model of the product. Coverage measures the LLM's self-consistency, not the product's correctness. Reality is never consulted.**

The seven failure modes this document addresses:

1. **Tautological tests** — code and test written in the same turn; test restates the code.
2. **Mock bleed** — `CONST-030` forbids mocks outside unit tests; LLMs slip them in under helper names.
3. **Happy-path saturation** — edge cases, concurrency, malformed input, timeouts, partial failures are under-covered because they are hard to assert.
4. **Silent skips** — `t.Skip` / `@Ignore` / `xit` to keep the suite green.
5. **Integration-seam blindness** — each side of a boundary passes; wire format, auth, or CORS between them is broken.
6. **Task ≠ feature** — tasks are ticked; the feature is non-functional.
7. **Self-certification** — "I verified it works" based on re-reading the LLM's own output.

## The rule

A change is NOT done because code compiles and tests pass. **Done** requires pasted terminal output from a real run, produced in the same session as the change, attesting that the real product exhibits the required behavior.

## The six clauses

### 1. No self-certification

The words *verified*, *tested*, *working*, *complete*, *fixed*, *passing* are forbidden in commits, PR bodies, and Claude Code replies unless accompanied by pasted terminal output from a command that ran in that session. Claude must treat its own confidence as insufficient evidence.

### 2. Demo before code

Every task begins by writing the runnable acceptance demo. The demo is the commands one would type to prove the task is done, plus the expected output. The LLM writes the demo first; the implementation is whatever makes the demo pass.

Task template:

```
Title: <what is being built>

Acceptance demo (must be pasted green into the PR):
  $ <setup>
  $ <exercise the change>
  $ <assert outcome, e.g. jq -e '...' or screenshot diff exit 0>
  <expected output>
```

### 3. Real system, every time

Demos must run against real artifacts:

| Surface | Real-run target |
|---|---|
| Go REST / gRPC | `./bin/<binary>` running with real Postgres, real Redis, real dependencies booted per `containers/.env`. No `httptest.NewServer`, no `sqlmock`, no in-memory fakes. |
| Android (phone) | Instrumented test on a real emulator or device driving the real installed APK (Espresso / UiAutomator). Robolectric is unit-only. |
| Android TV | Instrumented test on an **Android TV** emulator or real TV device (behavior and input model differ from phone — do not substitute a phone emulator). |
| Website | Playwright against the built `docker run` image or the production deploy. Not Vitest + JSDOM. Visual regression (`toHaveScreenshot()`) on every PR. |
| Go library / module | One `go test -run <demo>` against a binary that imports the module and calls the public API; no internal test-only hooks. |
| CLI tool | Shell script invoking the built binary with real arguments and asserting on stdout/stderr/exit-code. |

"Runs in the IDE" and "passes `go test ./...`" do not satisfy this clause for anything beyond pure-unit scope.

### 4. Skips are loud

Any skip directive without a ticket reference breaks `make ci-validate-all`:

```bash
# scripts/no-silent-skips.sh
#!/usr/bin/env bash
set -euo pipefail
if grep -rnE 't\.Skip\(|@Ignore|\bxit\(|describe\.skip|it\.skip|@pytest\.mark\.skip' \
    --include='*.go' --include='*.kt' --include='*.java' \
    --include='*.ts' --include='*.tsx' --include='*.js' --include='*.jsx' \
    --include='*.py' \
    . 2>/dev/null | grep -v 'SKIP-OK: #[0-9]'; then
  echo "ERROR: silent skip(s) detected. Annotate with // SKIP-OK: #<ticket> or remove." >&2
  exit 1
fi
```

A skipped test without a tracked ticket is invisible debt. The grep makes it visible.

### 5. Contract tests on every seam

Every integration boundary has a single roundtrip test exercised on any change touching either side. The boundary types are generated from a single source:

- API ↔ Android / Web → generate Kotlin / TypeScript types from an OpenAPI spec, protobuf, or Go struct source. Never hand-write types on both sides.
- API ↔ DB → test against a real Postgres instance with actual migrations applied.
- Shared library ↔ consumer → single test module exercising the public API from a consumer's perspective.

A contract test's job is to fail fast when the two sides disagree on wire format. It is not a unit test of either side.

### 6. Evidence in the PR

PR bodies must contain a fenced `## Demo` block:

````markdown
## Demo

Ran on: <date> against <branch> @ <sha>

```bash
$ ./bin/helixagent &
$ curl -sf -X POST localhost:8100/v1/foo -d '{"bar":"baz"}'
{"status":"ok","id":"abc123"}
$ ./challenges/scripts/foo_challenge.sh
[...87 tests, 87 passing...]
```
````

Reviewers reject PRs lacking this block, even if the code is correct. The block is the artifact that distinguishes "LLM says it works" from "the running system says it works".

## Manual phase-smoke protocol

At the end of every planning phase (not every task — that is too frequent), a human runs the real product and uses it as a user would. This is intentionally unscalable: its job is to measure the residual gap between "suite is green" and "works", and to surface it before it compounds.

Minimum smoke protocol:

1. Build the real artifact from a clean checkout (`make build` or equivalent).
2. Boot it the way a user would (`./bin/helixagent` for the REST API; install the APK on a real TV; `docker run` the website image).
3. Execute the top three user flows end-to-end. Record screen.
4. Every surprise (unexpected error, UI glitch, wrong response, slow path, missing state) becomes a new task.
5. Phase is not complete until the smoke run has zero surprises.

As the discipline takes hold, the smoke reveals fewer surprises and the protocol becomes cheap. Early on it is painful; that is the point.

## Enforcement

Enforcement is layered so the LLM cannot bypass any single gate:

| Gate | Where | What it catches |
|---|---|---|
| Pre-push | Local `make ci-pre-push` | Silent skips, formatting, unit-test regressions |
| CI-validate-all | `make ci-validate-all` | Full validation including concurrency audit, security scan |
| Challenge suite | `./challenges/scripts/run_all_challenges.sh` | Real-system behavior (CONST-030 real infra) |
| PR reviewer | Human + `/ultrareview` | Missing `## Demo` block, missing contract test, self-certification |
| Phase smoke | Human | Residual gap between green suite and working product |

The LLM must pass all five. Claude Code sessions must not treat green tests as evidence of completion; the PR body's `## Demo` block is the evidence.

## Interaction with existing Constitution rules

This Definition of Done strengthens and operationalizes:

- **CONST-001** (100% test coverage) — coverage remains required, but it is no longer evidence of "done".
- **CONST-002** (challenge coverage) — challenges are the primary real-system demo.
- **CONST-013** (comprehensive verification) — the `## Demo` block is the comprehensive verification artifact.
- **CONST-025 / CONST-030** (no mocks outside unit tests) — the "real system" clause enforces this at the demo level, not just the test level.

When this document and a specific Constitution rule appear to conflict, the stricter one wins. The Constitution and this document are co-authoritative on their overlapping concerns.

## Adoption path for projects without this discipline

If you are retrofitting a project that already has the "tests pass, nothing works" pathology:

1. **Week 1** — adopt clauses 1 (no self-certification) and 2 (demo before code). Do not attempt full coverage yet.
2. **Week 2** — add clause 6 (PR evidence block) and require it on every merge. Backfill demos for the last 10 merged changes to practice.
3. **Week 3** — add clause 4 (silent-skip check) to `make ci-validate-all`. Expect many skips to surface — triage them.
4. **Week 4** — add clause 5 (contract tests) on the top three most-broken integration seams (measure: which features get re-broken most by other teams' changes).
5. **Week 5+** — run the first manual phase-smoke. Measure how many surprises surface. That number is your baseline; it should trend to zero over the next two phases.

Expected effect in two weeks (clauses 1+2+6 only): ~70% collapse of the manual-testing breakage rate.

## Frequently asked objections

**"This slows us down."**
Yes, by roughly 20% per task. It prevents the 2–10× slowdown of discovering in manual testing that an entire phase is non-functional and must be redone.

**"The LLM will fake the demo output."**
The demo runs against real infrastructure that the LLM does not control. If the demo passes but the product is broken, the demo is wrong — fix the demo. Reviewers should re-run demos on a sampled basis.

**"Some tasks don't have a clean demo."**
Then the task is the wrong shape. Decompose until each piece has a real-run demo. If a piece cannot be demonstrated against the real system, it should not be a task — it is either refactor (no user-visible change, demo is "tests still pass and previous demo still works") or exploration (not a deliverable).

**"This duplicates our integration tests."**
The demo is not an integration test. It is the minimum proof that *this specific change* produced *this specific user-visible effect* on the *real system*. Integration tests verify invariants over time; the demo verifies causation for this change.

## References

- Root `CLAUDE.md` — the canonical six-clause summary.
- `CONSTITUTION.md` — the authoritative project rules.
- Each submodule's `CLAUDE.md` — inherits this document by reference and adds a module-specific demo slot.
