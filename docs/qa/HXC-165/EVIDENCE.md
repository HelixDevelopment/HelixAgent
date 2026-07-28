# HXC-165 — QA evidence: stress test overwrote its own tracked reference baseline

**Revision:** 1
**Last modified:** 2026-07-29
**Maintainer:** AI-agent (Claude Opus 5)
**Item:** HXC-165 (Bug) — "A stress test overwrites its own committed reference baseline every time it runs"
**Fix commit:** `b43e69d5` (submodule `helix_agent`, branch `main`, local — not pushed)

Captured under §11.4.83 (docs/qa end-user evidence). The §11.4.115 RED reproduction
against the real pre-fix artifact is recorded HERE rather than re-enacted by a
permanent `RED_MODE` branch, because deliberately corrupting a tracked file on
every RED invocation is not crash-safe and races other checkouts in this shared
tree. See `tests/stress/latency_report_test.go` for the standing guard.

---

## 1. The defect (verified, not assumed)

Write site: `tests/stress/extreme_load_test.go`, inside `TestExtremeP99LatencyBaseline`.

```go
reportDir := filepath.Join("..", "..", "reports", "latency")
...
reportPath := filepath.Join(reportDir, "p99-baseline-2026-03-16.txt")
writeErr := os.WriteFile(reportPath, []byte(reportContent), 0o600)
```

The target is git-TRACKED:

```
$ git ls-files reports/latency/
reports/latency/p99-baseline-2026-03-16.txt
```

## 2. Is there a comparison consumer? NO — and this shaped the design

A repo-wide search found only two references to the baseline path: the write site
itself, and `docs/superpowers/plans/2026-03-16-complete-remediation.md:1297` (the
plan that authored it). Nothing reads it for comparison.

Consequence: no comparison was invented as part of this fix. Doing so would have
baked in a bogus yardstick — the committed numbers are themselves one run from a
contended shared host. The project's genuine baseline-comparison mechanism is
elsewhere and untouched: `tests/performance/baseline_regression_test.go` over
`benchmarks/baselines/*.txt`.

## 3. What the file actually is (§11.4.124 git-history investigation)

```
$ git log --follow --oneline -- reports/latency/p99-baseline-2026-03-16.txt
c20215bd test(helix_agent): chaos/compliance/automation hardening + live-endpoint guards
4f6d36f9 Auto-commit
6d39084f chore: update submodules and baseline reports
98cb390e Auto-commit
628e243b fix(tests): resolve 15 root-cause test failures, add short-mode guards
c9cd6728 feat(helixqa): comprehensive 82-test video QA + nil guard fixes for handlers
```

Created 2026-03-23 in `c9cd6728`; every one of the five later commits is a pure
numbers churn (min/mean/p50/p90/p99/max only), two landed via blanket
`Auto-commit` commits, and the file's own `Date: 2026-03-16` header stayed frozen
while its contents drifted months later.

**Determination:** an accidentally-committed run artefact, not a curated
reference. It was NOT deleted — that is a separate decision requiring its own
commit per §11.4.124, and it is now harmless because nothing rewrites it.

## 4. §11.4.115 RED — defect reproduced on the pre-fix artifact

```
--- baseline sha256 BEFORE run ---
73f8ef851156f3a523b4a33e969b1e20bec8ccbb34ccc1a37144228b60bfe116

$ go test -run '^TestExtremeP99LatencyBaseline$' ./tests/stress/ -v -count=1
    extreme_load_test.go:520: Report written to ../../reports/latency/p99-baseline-2026-03-16.txt
--- PASS: TestExtremeP99LatencyBaseline (0.14s)

--- baseline sha256 AFTER run ---
88c8b1a9af6cd563f1122118ac57382c9213fa7c1498ce3bff3a21a823fea7f0

RED CONFIRMED: committed baseline WAS overwritten by the test run
```

Note `73f8ef85…` is the pre-existing DIRTY working copy (earlier run damage from
another session). The committed reference is `a13ff412…`. A §9.2 pre-op backup was
taken before this run and the bytes restored byte-identically afterwards.

Also captured during this run: `Max: 28.389065ms` (and `P99: 82.4ms` on a later
run) — direct evidence that shared-host contention makes these numbers
unrepresentative, which is precisely why overwriting the yardstick with them is
destructive.

## 5. §1.1 paired mutations — every assertion proven to bite

Each mutation was applied, asserted to FAIL, then restored and asserted to pass.

| # | Mutation | Result |
|---|---|---|
| 1 | Probe test writes directly to the baseline (simulates re-inlining the defect) | `go test` exit **1**; `HXC-165: the tracked reference baseline ... changed during this package run (sha256 "73f8ef85…" -> "706cae49…")`. Probe removed → exit **0** |
| 2 | Writer emits the frozen `Date: 2026-03-16` header | FAIL — `carries the frozen pre-fix header "Date: 2026-03-16"` |
| 3 | Writer emits an empty report | FAIL — `run report ... is empty` |
| 4 | Writer back-dates `Run-UTC` by 72h | FAIL — `has Run-UTC 2026-07-25 ..., which is not from this run (age 72h0m0.000106401s)` |

Mutation 1 is the load-bearing one: it proves `TestMain` guards the DEFECT SITE,
not merely the new helper — the probe test itself PASSED while the package FAILED.

## 6. §11.4.108 runtime signature — post-fix, against the RESTORED baseline

```
BEFORE  sha256=a13ff412b10ac88d5ff9f22751d0c50eac2f4e61367429a7b12e3b117419aa90
BEFORE  status=''            <- empty = CLEAN, matches HEAD
BEFORE  mtime=1785273093

$ go test -run '^(TestExtremeP99LatencyBaseline|TestP99LatencyBaselineArtifactIsolation)$' \
      ./tests/stress/ -count=1
ok  	dev.helix.agent/tests/stress	0.123s

AFTER   sha256=a13ff412b10ac88d5ff9f22751d0c50eac2f4e61367429a7b12e3b117419aa90
AFTER   status=''
AFTER   mtime=1785273093

>>> PASS: baseline CLEAN and byte-identical after the run (content+status+mtime)
```

mtime is unchanged, so the file is not even opened for writing. Stable across 4
consecutive runs.

## 7. Run output lands at the ignored path (§11.4.77)

```
$ ls -1 reports/latency/runs/
p99-run-20260728T211315.237610959-pid1927601.txt

$ git check-ignore -v reports/latency/runs/p99-run-20260728T211315.237610959-pid1927601.txt
.gitignore:101:reports/latency/runs/	reports/latency/runs/p99-run-...txt

$ git check-ignore -v reports/latency/p99-baseline-2026-03-16.txt
(no match — the baseline correctly stays tracked and read-only)
```

Regeneration mechanism, documented at `.gitignore:95-101`: the directory holds
pure test output; nothing builds, runs or tests against it; recreate with
`go test -run '^TestExtremeP99LatencyBaseline$' ./tests/stress/`.

## 8. Baseline restoration

The working copy had been corrupted by earlier pre-fix runs. Restored:

```
before restore: 73f8ef851156f3a523b4a33e969b1e20bec8ccbb34ccc1a37144228b60bfe116
after  restore: a13ff412b10ac88d5ff9f22751d0c50eac2f4e61367429a7b12e3b117419aa90
git status:     ''   (clean)
```

The corrupted bytes are preserved out-of-tree at
`/tmp/hxc165_backup_1785270412/baseline.dirty.bak` (host-local, ephemeral).
The repo-wide instance of this hazard is tracked as **HXC-182**.

## 9. Honest boundaries (§11.4.6)

- The guard detects that the baseline CHANGED during a package run. It cannot
  attribute the write to a test in this package vs a concurrent external writer,
  and the failure message says exactly that. It fails closed, so the risk is a
  false FAIL under contention, never a false PASS.
- The guard's scope is package `stress`. A write from `tests/stress/verifier`
  (a separate package) would not be caught; no such writer exists today.
- A byte-identical rewrite would be undetected. Not reachable in practice, since
  the report embeds measured nanosecond latencies.
- The committed baseline was NOT deleted. The git-history evidence supports that
  it is an accidentally-committed run artefact, but removal is its own separate
  commit per §11.4.124 and was out of scope here.
