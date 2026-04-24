# DoD enforcement — operational README

The two scripts in this directory are the structural gates for the universal Definition of Done (see `docs/development/definition-of-done.md`). Without them, the DoD is documentation with no teeth; with them, `make ci-validate-all` refuses to pass until the rules actually hold.

## The two gates

### `scripts/no-silent-skips.sh`
Greps the tree for test skip directives that are not annotated with `SKIP-OK: #<ticket>`. A skipped test without a ticket is invisible debt — this script makes that debt loud.

**Patterns it catches:** `t.Skip(`, `@Ignore`, `xit(`, `.skip(`, `@pytest.mark.skip`, `@unittest.skip`.

**Excluded trees:** `vendor/`, `node_modules/`, `external/`, `MCP/`, `cli_agents/`, `mcp-servers/`, `releases/`, `reports/`, `test-results/`.

**Annotation format:** end the line (or a trailing comment) with `SKIP-OK: #<N>` where `<N>` is a tracked ticket number.

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
- ~4000 unannotated skips (backlog — inherited, not created by this work).
- ~60 acceptance demos filled in, but most have never been executed against real infrastructure.

Bricking `ci-validate-all` on day one would be unhelpful. `ci-validate-all` currently calls the **warn-only** variants:

```make
@$(MAKE) no-silent-skips-warn
@$(MAKE) demo-all-warn
```

These print the findings but exit 0 so the build stays green. Make targets available:

| Target | Behavior | Ready for CI? |
|---|---|---|
| `make no-silent-skips-warn` | reports violation count, exit 0 | yes |
| `make no-silent-skips` | reports and exits 1 on any violation | when backlog = 0 |
| `make demo-all-warn` | runs demos, reports, exit 0 regardless | yes |
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
