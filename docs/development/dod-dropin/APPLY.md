# DoD drop-in — apply to a new project

The five-minute install. Run these commands from the target project's root.

## 1. Copy the scripts in

```bash
# Assume HelixAgent is cloned as a sibling of your target project.
SRC=/path/to/HelixAgent/docs/development/dod-dropin

mkdir -p scripts reports/demos
cp "$SRC/scripts/no-silent-skips.sh" scripts/
cp "$SRC/scripts/demo-all.sh" scripts/
chmod +x scripts/no-silent-skips.sh scripts/demo-all.sh
```

## 2. Add the CLAUDE.md clause

If you already have a `CLAUDE.md`, paste the contents of
`templates/CLAUDE_md_clause.md` near the top, under the project description.

If you don't, create one with that content as the body.

Repeat per module if the project has submodules — each gets its own acceptance
demo block. The point of per-module demos is to prove each module works
end-to-end *against its real dependencies*, not via its test suite.

## 3. Wire the Makefile

Append the block from `templates/Makefile_additions.md` to your `Makefile`.
If the project uses Gradle instead: wrap the script calls in Gradle tasks
(example in the bottom of this file). If it uses npm: add entries to
`package.json` `scripts`.

## 4. Write the first acceptance demo

Pick the project's MOST IMPORTANT feature — the one a user would break first
if it regressed. Write the 10-line demo that proves that feature works, from
nothing, via a real run. Paste it into the `### Acceptance demo for this module`
block.

Do not proceed to step 5 until this demo exists and passes locally.

## 5. Run the gates in warn-mode

```bash
make ci-validate-all
```

Expect a report like:
```
demo-all totals: PASS=1 FAIL=0 TODO=N NO-DEMO=M
no-silent-skips: ⚠️  K violation(s) detected
```

Record `K` (skip backlog) and `N + M` (demo backlog). These are your starting
numbers. Every PR shrinks them.

## 6. Commit the drop-in

```bash
git add scripts/no-silent-skips.sh scripts/demo-all.sh CLAUDE.md Makefile
git commit -m "chore(dod): install Definition of Done gates (warn-only)"
```

## 7. Graduate (eventually)

When `K=0` and demos for every significant module exist and pass:

```bash
# Edit Makefile: drop the -warn suffix from no-silent-skips-warn and demo-all-warn
# in the ci-validate-all target.
```

Flip in a single PR so the change is audited.

---

## Non-Make build systems

### Gradle

Add to your root `build.gradle` or `build.gradle.kts`:

```kotlin
tasks.register<Exec>("noSilentSkips") {
    commandLine("bash", "scripts/no-silent-skips.sh")
    environment("NO_SILENT_SKIPS_WARN_ONLY", "1")
}
tasks.register<Exec>("demoAll") {
    commandLine("bash", "scripts/demo-all.sh")
    environment("DEMO_ALL_WARN_ONLY", "1")
    environment("DEMO_ALLOW_TODO", "1")
}
tasks.register("ciValidateAll") {
    dependsOn("check", "noSilentSkips", "demoAll")
}
```

### npm/pnpm

Add to `package.json` `scripts`:

```json
{
  "scripts": {
    "gate:skips": "NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh",
    "gate:demos": "DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh",
    "gate:all": "npm run lint && npm test && npm run gate:skips && npm run gate:demos"
  }
}
```

### Cargo / Rust

Add to `Cargo.toml` under `[package.metadata.scripts]` (with `cargo-make`)
or create `Makefile.toml` that invokes `bash scripts/demo-all.sh`.

---

## Why this works

- **Auto-discovery.** `demo-all.sh` finds every `CLAUDE.md` with an acceptance
  demo. Adding a new module = adding its CLAUDE.md. No central list to
  maintain.
- **Warn-mode first.** Nothing breaks on install. You get the baseline
  numbers, then each PR improves them.
- **Portable.** One script. Works on any codebase with bash.
- **Auditable.** `reports/demos/*.log` captures the actual output of each
  demo, so the evidence is always there even when the agent forgets to paste
  it into the commit.
