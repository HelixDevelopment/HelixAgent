# Triage Update: go-elder-plinius-v3 mechanical fixes

**Date:** 2026-04-20
**Supersedes-parts-of:** `docs/research/go-elder-plinius-v3_triage.md` §1.1

The original triage documented two compilation-blocking bug classes across
all 31 scaffold modules. This update records the fixes applied.

## What was fixed (mechanical)

1. **Unterminated string literals (137 sites across 31 `pkg/types/types.go`)**
   The codegen emitted `if strings.TrimSpace(o.X) == "{` where the intent
   was `if strings.TrimSpace(o.X) == "" {` — an empty-string check followed
   by an opening brace. Two-step sed fix: strip the erroneous `"{` → `""`,
   then restore the missing ` {` at line end.

2. **Receiver confusion in clients (121 sites across 30 `pkg/client/client.go`)**
   The codegen emitted `if err := RunOptions.Validate()` (type name) where
   the intent was `if err := opts.Validate()` (the parameter receiver).
   Applied to both `.Validate()` and `.Defaults()` variants via Python
   script that preserved the `if err :=` prefix and indentation.

3. **Missing qualifier / unused import in clients (31 files)**
   Each client imported `.../pkg/types` by default name but used bare type
   names (`RunOptions` not `types.RunOptions`). Converted to dot-import so
   bare names resolve; removed the resulting `"fmt"` dead import.

## Build status after mechanical fix (defensible 9-module subset)

```
PASS: go-plinius-common
PASS: go-gandalf-solutions
FAIL: go-autotemp          (1 err — BenchmarkOptions lacks Validate method)
FAIL: go-hypertune         (4 errs — fields declared [2]int but treated as scalar)
FAIL: go-i-llm             (4 errs — per-module semantic issues)
FAIL: go-v3r1t4s           (2 errs — per-module semantic issues)
FAIL: go-leakhub           (1 err  — per-module semantic issues)
FAIL: go-cl4r1t4s          (8 errs — per-module semantic issues)
FAIL: go-ourobopus         (3 errs — per-module semantic issues)
```

2 of 9 modules now compile. The remaining 7 have per-module semantic
codegen bugs (missing method stubs, wrong field array types, etc.) that
are not fixable by a global sed/regex pass — each needs targeted work.

## What is NOT fixed

- Every compiled method still returns `errors.New(ErrCodeUnimplemented,
  <mod>, "<Method> requires backend service integration")`. **398
  occurrences**, unchanged. Compilation is a prerequisite for re-implementation,
  not a substitute for it.
- Per-module semantic bugs in 7 of the 9 defensible modules (see list above).
- The 22 modules outside the defensible subset have not been individually
  verified; the global fixes were applied to all 31, but fail-counts per
  module were not measured for the 22.

## What is still declined (unchanged from original triage)

- Creating PUBLIC `vasic-digital` repositories for the offensive subset:
  `go-l1b3rt4s`, `go-obliteratus`, `go-g0dm0d3`, `go-dioscuri`,
  `go-p4rs3lt0ngv3` (in filter-bypass use), `go-glossopetrae` (in
  steganographic use), `go-misc-prompthacks`, `go-basilisktoken`, and
  `go-autoredteam` as *attacker* tooling. These remain intake-only under
  `docs/research/`.
- Re-implementing the 398 unimplemented methods without upstream Python
  source reference and without explicit Phase-A approval.
- Publishing non-functional stubs to any public namespace as "integrated"
  or "finished".

## What Phase-A (pending approval) still requires

- Explicit approval for a 9-module re-implementation from Python upstreams.
- ~4 days/module minimum for core-value surface; ~2 weeks/module full-spec.
- Per-module tests, CLAUDE.md, AGENTS.md, README, challenge script.
- Kept INTERNAL to HelixAgent (not 31 public repos).

## References

- Original triage: `docs/research/go-elder-plinius-v3_triage.md`
- Intake of integration plan: `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`
- Scaffolds: `docs/research/go-elder-plinius-v3/go-elder-plinius/`
