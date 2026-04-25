# Makefile additions — wire the gates

Append these targets to your project's root `Makefile`. They call the drop-in
scripts so the gate discipline is one `make ci-validate-all` away.

```make
# Definition of Done gates
.PHONY: no-silent-skips no-silent-skips-warn \
        no-mocks-above-unit no-mocks-above-unit-all no-mocks-above-unit-update-allowlist \
        demo-all demo-all-warn demo-one ci-validate-all

no-silent-skips:
	@bash scripts/no-silent-skips.sh

no-silent-skips-warn:
	@NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh

# Ratchet gate: passes when only allowlisted sites exist; fails on any NEW
# in-process fake in non-unit tests. Allowlist file: scripts/no-mocks-above-unit-allowlist.txt
no-mocks-above-unit:
	@bash scripts/no-mocks-above-unit.sh

# Audit mode: report every site, ignoring the allowlist.
no-mocks-above-unit-all:
	@bash scripts/no-mocks-above-unit.sh --all

# Drainage helper: regenerate the allowlist after intentional cleanup.
# The allowlist should only ever shrink. PRs that grow it require justification.
no-mocks-above-unit-update-allowlist:
	@bash scripts/no-mocks-above-unit.sh --update-allowlist

demo-all:
	@bash scripts/demo-all.sh

demo-all-warn:
	@DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh

# Run a single module's demo: make demo-one MOD=path/to/module
demo-one:
	@DEMO_MODULES="$(MOD)" bash scripts/demo-all.sh

# Single entry point. Add your existing build/test/lint targets before the gates
# so gates run AFTER static checks.
#
# Note: no-mocks-above-unit is strict-with-allowlist by default — no warn variant
# is needed because the ratchet only fails on NEW violations beyond the allowlist.
# Generate the initial allowlist with `make no-mocks-above-unit-update-allowlist`
# right after installing the gate.
ci-validate-all: fmt vet lint test no-silent-skips-warn no-mocks-above-unit demo-all-warn
	@echo "ci-validate-all: all gates executed"
```

## Graduation path

`no-mocks-above-unit` is strict immediately (allowlist locks in current state;
fails on new violations). The other two gates need a graduation step:

1. **Install (day 1):**
   - `make no-mocks-above-unit-update-allowlist` to freeze the current state.
   - Commit `scripts/no-mocks-above-unit-allowlist.txt` to the repo.
   - The mocks gate is now strict — new in-process fakes are rejected.
   - The skip and demo gates are warn-only — they print findings but exit 0.
   - Capture the baseline numbers from the warn output. That is your backlog.
2. **Drain (months 2-3):** each PR shrinks at least one number:
   - skip backlog → fix or annotate `SKIP-OK: #<ticket>`
   - mock allowlist → convert tests to real-artifact roundtrips, regenerate allowlist
   - demo backlog → fill in module CLAUDE.md acceptance demos
3. **Flip the warn-only gates as each hits zero:**
   ```make
   # After skip backlog drained:
   ci-validate-all: fmt vet lint test no-silent-skips no-mocks-above-unit demo-all-warn
   # After demo backlog drained (full strict):
   ci-validate-all: fmt vet lint test no-silent-skips no-mocks-above-unit demo-all
   ```
   Each `-warn` suffix goes away one at a time. Now every gate has teeth.

Before you flip, run `make ci-validate-all` with strict targets locally — if it
passes, flip in the same PR. If it fails, fix the breakage first and retry.

## Why three gates and not one

Each gate catches a distinct flavor of "tests pass but product doesn't":

| Gate | Catches |
|---|---|
| `no-silent-skips` | Tests that ran 0 of N cases because they returned early — invisible coverage gap. |
| `no-mocks-above-unit` | "Integration" tests that prove the mock works, not the system. The test is green but the running binary has never been exercised by it. |
| `demo-all` | Modules where no one has ever assembled a real end-to-end run, so nobody knows whether the documented quick-start actually works today. |

A project with all three at zero — and adversarial PR review — is a project where green CI is genuine evidence the product works.
