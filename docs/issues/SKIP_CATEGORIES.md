# Legitimate-skip categories

`scripts/no-silent-skips.sh` accepts either a numeric ticket reference
(`SKIP-OK: #1234`) OR a kebab-case category tag (`SKIP-OK: #short-mode`).
Category tags are for skips that are architecturally correct and have no
associated bug ticket — they document the condition under which the skip
fires so reviewers know the skip is intentional.

When introducing a NEW category, add it here first with a one-line
rationale. The taxonomy below is closed — new patterns need a new
category entry, not reuse of an ill-fitting one.

## Environmental guards (test environment is missing something real)

- `#short-mode` — Test is too slow for `go test -short` and skips there.
  Runs under the full suite.
- `#requires-docker` — Test needs a working Docker/Podman socket and
  skips when neither is present.
- `#requires-podman-rootless` — Test specifically needs rootless podman
  (not docker).
- `#requires-android-sdk` — Test builds Android artifacts and skips
  without ANDROID_SDK_ROOT.
- `#requires-upstream-key` — Test hits a live LLM provider and skips
  without an API key in env.
- `#requires-infra-port` — Test binds a specific port and skips if that
  port isn't available on the host.
- `#requires-network` — Test makes outbound network calls and skips in
  airgapped environments.

## Mode selectors (runtime mode disables the test)

- `#runtime-mock-only` — Test asserts mock behavior and only runs when
  no real container runtime is present.
- `#runtime-real-only` — Test asserts real-runtime behavior and only
  runs when Docker/Podman IS present.
- `#integration-mode-only` — Test only runs when `INTEGRATION=1`.

## Test-case guards (the test itself branches)

- `#test-case-guard` — Inner sub-test skips (`t.Run` branch) because the
  inputs don't apply in this configuration. Almost always inside a
  table-driven test.
- `#invalid-config-branch` — Table-driven test case that asserts
  "invalid configuration is rejected"; the rejection path skips further
  assertions.

## Smart-routing / runtime-decision skips

- `#ensemble-not-engaged` — Test exercised the `helixagent-debate` model
  but the binary's smart-routing decided to short-circuit to a single
  provider (e.g. for trivially simple prompts) instead of engaging the
  full debate ensemble. The test asserts ensemble structure that isn't
  present, so it skips. Drainage report 2026-04-25 Finding #9.
  Long-term: either configure routing to force the ensemble for tests,
  OR write tests that assert "either short-circuit OR ensemble" without
  prescribing which.

## Pre-existing backlog

- `#legacy-untriaged` — Bulk-annotated during the 2026-04-24 skip-backlog
  sweep. Means "this skip existed before the gate went in and hasn't
  been individually reviewed yet; it's believed to be legitimate but
  has not been verified." Target: decremented to zero over the next
  quarter by reviewing and either deleting the skip or retagging with a
  specific category from above.

---

## How to add a category

1. Pick the category name — short, kebab-case, starts with lowercase.
2. Add it here with a one-line rationale.
3. Use it as `// SKIP-OK: #<category>` (Go/Kotlin) or
   `# SKIP-OK: #<category>` (Python/bash) on the same line as the skip
   directive.

Category names are cheap — favor specificity over reuse.
