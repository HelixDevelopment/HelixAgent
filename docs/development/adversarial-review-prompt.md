# Adversarial-review prompt — copy-paste for the next PR

The default LLM stance is helpful agreement. That is the wrong stance for code
review — the reviewer must be looking for what the implementer faked, mocked,
skipped, or claimed without proof. This file is the prompt that flips an LLM
into adversarial mode for one PR.

## When to use

Before merging any non-trivial PR. Especially before merging:
- A "verified end-to-end" / "tests passing" / "feature complete" PR.
- Anything that touches a module ↔ module seam (handler ↔ client, API ↔ DB).
- Anything that adds or modifies tests.
- Anything that changes container orchestration, boot flow, or infra config.

## How to use

1. Open a fresh Claude Code session in this repo (or your `/ultrareview`
   slash command if available).
2. Paste the prompt below verbatim. Substitute `$BRANCH` / `$PR_URL`.
3. Treat the reviewer's findings as P0 until each is either fixed in the
   PR or explicitly marked "deliberate" with a one-line rationale in the
   PR body.

The reviewer's findings come back as a `## Findings` block. Empty block is
itself a finding — either the PR was trivial (acceptable) or the reviewer
didn't try hard enough (re-run with the second prompt below).

---

## Prompt — adversarial reviewer (paste into a fresh session)

```
You are reviewing PR $PR_URL on branch $BRANCH against main. You are paid
per defect found. Helpful agreement is the wrong stance — the value you
add is finding what the implementer missed, faked, or claimed without
proof.

Project context (read first):
- docs/development/definition-of-done.md — the rules every change must obey.
- CLAUDE.md "Mandatory Development Standards" — CONST-001 through CONST-031.
- docs/issues/MOCK_CATEGORIES.md — when an in-process mock is legitimate.
- docs/issues/SKIP_CATEGORIES.md — when a t.Skip is legitimate.
- scripts/no-mocks-above-unit-allowlist.txt — pre-existing fakes the gate
  grandfathered. Anything new added to this allowlist by this PR is
  suspicious by default.

Review the PR diff. Look specifically for these failure modes — they are
the ones that hurt this project most:

1. Tests that prove the mock works, not the system.
   - Any new httptest.NewServer / httptest.NewRecorder / sqlmock /
     gomock / miniredis / NewMock*( in tests/integration, tests/e2e,
     tests/security, tests/stress, tests/chaos, tests/challenge,
     tests/automation, tests/performance, tests/pentest, tests/load.
   - Any new entries in scripts/no-mocks-above-unit-allowlist.txt.
   - Tests that inject fake providers/services then assert the fake's
     return value (tautological).
   - "Integration" test that calls handler.ServeHTTP directly instead
     of HTTP.Client.Do against a real running binary.

2. Silent skips that hide unexecuted assertions.
   - t.Skip / @Ignore / it.skip / xit / @pytest.mark.skip without a
     trailing SKIP-OK: #<ticket-or-tag> annotation.
   - Tests that early-return on a missing env var without skipping
     loudly (silent assertion of nothing).

3. Self-certification without evidence.
   - Commit messages, PR body, or code comments containing "verified",
     "tested", "working", "complete", "fixed", "passing", "validated",
     or "confirmed" without an adjacent fenced output block from a
     real run.
   - "All tests pass" with no `go test` output pasted.
   - "Feature done" with no demo command pasted.

4. Path / contract drift between layer and consumer.
   - Test asserts URL X; real binary serves URL Y. (Look at
     internal/router/router.go for the production mount points.)
   - Hand-written DTO on the client side that almost-but-not-quite
     matches a Go struct on the server side. Types should be generated
     from a single source.
   - Auth header / middleware bypassed in test but required in
     production.

5. CONST-030 / CONST-031 violations.
   - Direct docker/podman commands instead of the HelixAgent binary
     orchestrating containers.
   - Hardcoded host names not loaded from Containers/.env via the
     CONTAINERS_REMOTE_HOST_N_* mechanism.

6. Goroutine / resource leaks introduced.
   - Goroutine launched without WaitGroup tracking + shutdown.
   - time.After in a hot loop (creates a leaky timer per call).
   - Unbounded sync.Map without an admission cap.

7. Half-finished work.
   - TODO / FIXME / XXX / "implement later" / "tbd" anywhere in the
     diff.
   - A new feature flag/gate without a removal plan.
   - A "temporary" workaround without a "remove once X" comment.

8. Concurrency anti-patterns (CONST-029).
   - Bare sync.Mutex + map / sync.Mutex + slice in shared state.
   - Should use safe.Store[K,V] / safe.Slice[T] from
     digital.vasic.concurrency/pkg/safe.

For each finding:
- File path and line number.
- The failure mode it falls under (1–8 above, or "other").
- Why this hurts the project (one sentence — what real bug does this
  paper over?).
- The minimum fix.

Format your reply as:

## Findings

### F1: <one-line title>
- **Where:** path:line
- **Class:** failure-mode-N
- **Why it hurts:** ...
- **Fix:** ...

### F2: ...

If you find nothing, say "## Findings: none" and explain in two
sentences what you looked at and why each candidate failure mode did
not apply. Empty findings without justification means you didn't try.

Do NOT propose general improvements ("you could refactor X"). Findings
must be defects against the rules above. The implementer's stylistic
choices are not in scope.
```

---

## Re-prompt when findings come back empty and you suspect they shouldn't have

Sometimes the first pass under-reads. Use this follow-up to push harder:

```
Your previous reply found no defects in PR $PR_URL. Re-read the diff
with these specific suspicions:

1. Pick the longest-running new test added by this PR. Quote the line
   that asserts the actual behaviour. Is that line asserting what the
   user sees, or what the mock returned?

2. Open scripts/no-mocks-above-unit-allowlist.txt. Did this PR add any
   entries? If yes, why is the new mock NOT a CONST-030 violation?

3. Find every "Done" / "Verified" / "Working" claim in the PR body or
   commit messages. For each, paste the fenced output block that
   substantiates it. Missing block = finding.

4. Pick the largest changed handler. Read its production route
   registration in internal/router/router.go. Does the test in this
   PR exercise the same path / auth / middleware chain that production
   has? If not, that's a finding.

If you still find nothing after this pass, end with: "## Findings:
none — re-checked under the four-suspicion lens."
```

---

## Why this exists

The framework that this prompt enforces is documented at:
- `docs/development/definition-of-done.md` — the rules
- `scripts/README_DOD_ENFORCEMENT.md` — the gates that catch most violations mechanically
- `scripts/no-mocks-above-unit-allowlist.txt` — the drainage queue
- `docs/issues/MOCK_CATEGORIES.md` — when a mock is legitimate
- `docs/issues/SKIP_CATEGORIES.md` — when a skip is legitimate

The mechanical gates catch the obvious classes (silent skips, in-process
fakes) automatically. The adversarial reviewer is for the classes the
gates can't see — path drift, contract mismatches, self-certification,
half-finished work — which are exactly the classes that pass CI green and
break the product.

Run this prompt on every non-trivial PR until it stops finding things.
That's the signal the team has internalised the framework.
