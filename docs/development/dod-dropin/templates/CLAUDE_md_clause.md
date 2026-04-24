# Definition of Done clause — paste into every CLAUDE.md

Add these two sections near the top of your project's (and each module's) `CLAUDE.md`,
right under the project/module description. The clause is toolchain-agnostic; only the
acceptance demo changes shape per stack.

---

## Definition of Done

A change is NOT done because code compiles and tests pass. "Done" requires pasted
terminal output from a real run of the real system, produced in the same session as
the change. Coverage and passing suites do not count as evidence — they measure the
LLM's model of the product, not the product.

1. **No self-certification.** The words *verified, tested, working, complete, fixed,
   passing* are forbidden in commits, PR bodies, and agent replies unless accompanied
   by pasted output from a command that ran in that session.
2. **Demo before code.** Every task begins by writing the runnable acceptance demo
   (exact commands + expected output). The demo pins "done"; the code is what makes
   the demo pass.
3. **Real system, every time.** Demos run against real artifacts — built binaries,
   live databases, instrumented devices, actual browsers. No `httptest.NewServer`,
   `sqlmock`, JSDOM, or Robolectric as proof-of-done.
4. **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `it.skip` without a trailing
   `SKIP-OK: #<ticket>` annotation fails `make ci-validate-all`.
5. **Contract tests on every seam.** Any change touching a module↔module boundary
   runs one roundtrip test asserting the wire format on both sides. Types are
   generated from a single source; never hand-written on both ends.
6. **Evidence in the PR.** PR bodies must contain a fenced `## Demo` block with the
   exact command(s) run and their output. Reviewers reject PRs missing this block.

### Acceptance demo for this module

<!--
  Replace the block below with the minimal set of commands that prove THIS module
  works end-to-end against real dependencies. Keep it under 15 lines.
  Examples per stack (pick one, delete the rest):
-->

```bash
# TODO
```

<!-- Go service example:
```bash
cd path/to/module
make build
./bin/<binary> &
PID=$!
trap 'kill $PID 2>/dev/null' EXIT
until curl -sf http://localhost:PORT/v1/health >/dev/null; do sleep 1; done
curl -sf -X POST http://localhost:PORT/v1/primary-endpoint \
  -H 'Content-Type: application/json' \
  -d '{"input":"real-request-here"}' \
  | jq -e '.expected_field'
```
Expect: HTTP 200 with the expected field populated.
-->

<!-- Gradle/Android example:
```bash
./gradlew :app:assembleDebug :app:connectedDebugAndroidTest --info
# Capture logcat from the emulator during the run:
adb logcat -d -s YourTestTag > reports/demos/logcat.txt
grep -E 'PASS|TEST SUCCEEDED' reports/demos/logcat.txt
```
Expect: connectedDebugAndroidTest passes, logcat shows PASS lines.
-->

<!-- Node/web example:
```bash
npm ci
npm run build
npx serve dist -p 4173 &
PID=$!
trap 'kill $PID 2>/dev/null' EXIT
until curl -sf http://localhost:4173/ >/dev/null; do sleep 1; done
npx playwright test --reporter=line
```
Expect: build clean, Playwright prints all green.
-->
