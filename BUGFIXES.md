# Bug Fixes and Known Issues

## Critical Bug: Silent Crash After Podman-Compose

### Issue
HelixAgent crashed silently after deploying remote services. No error was logged, no panic occurred, no exit code captured. Port 7061 never opened.

**Symptoms:**
- Process exits immediately after "Starting compose services via Containers module" in logs
- No subsequent logs appear
- No error messages visible

### Root Cause
**Multiple copies of `orchestrator.go` existed across the codebase:**
1. `vendor/digital.vasic.containers/pkg/compose/orchestrator.go` - **USED BY BUILDS**
2. `Containers/pkg/compose/orchestrator.go` - Submodule copy
3. `Challenges/Containers/pkg/compose/orchestrator.go` - Challenge copy
4. `HelixLLM/submodules/Containers/pkg/compose/orchestrator.go` - HelixLLM copy

The vendor version was out of sync with the Containers submodule. The vendor version lacked error logging in the `run()` function, which meant compose failures were silently swallowed.

**Original buggy code in `run()`:**
```go
if err := cmd.Run(); err != nil {
    return fmt.Errorf("%s %s failed: %w\nstderr: %s",
        o.composeCmd, strings.Join(allArgs, " "),
        err, stderr.String())
}
```

The error was returned but **never logged**, making debugging impossible.

### Fix Applied
Added proper error logging to all copies of `orchestrator.go`:

```go
o.logger.Debug("executing: %s %s (dir: %s)", o.composeCmd, strings.Join(allArgs, " "), o.workDir)
if err := cmd.Run(); err != nil {
    stderrStr := stderr.String()
    o.logger.Error("%s %s failed: %v\nstderr: %s",
        o.composeCmd, strings.Join(allArgs, " "),
        err, stderrStr)
    return fmt.Errorf("%s %s failed: %w\nstderr: %s",
        o.composeCmd, strings.Join(allArgs, " "),
        err, stderrStr)
}
o.logger.Debug("compose command completed successfully")
```

**Files Modified:**
- `vendor/digital.vasic.containers/pkg/compose/orchestrator.go`
- `Containers/pkg/compose/orchestrator.go`
- `Challenges/Containers/pkg/compose/orchestrator.go`
- `HelixLLM/submodules/Containers/pkg/compose/orchestrator.go`

### Lessons Learned
1. **The `vendor/` directory is the truth for builds** - even with `replace` directives in `go.mod`, Go uses vendor when present
2. **Submodule syncs can create divergence** - always verify vendor directory after submodule updates
3. **Error logging is critical** - errors without logs are debugging nightmares

### Prevention
Before merging submodule updates, run:
```bash
# Check if vendor needs updating
go mod vendor

# Compare vendor with submodule
diff -r vendor/digital.vasic.containers Containers/pkg

# Or use this helper
go list -f '{{.Dir}}' digital.vasic.containers/pkg/compose
```

---

## Issue: Multiple Copies of Containers Module

### Problem
The project has 4+ copies of the `digital.vasic.containers` module:
- `vendor/digital.vasic.containers/` - Used by main project builds
- `Containers/` - Primary submodule
- `Challenges/Containers/` - Challenge-specific copy
- `HelixLLM/submodules/Containers/` - HelixLLM copy

### Recommendation
Standardize on one canonical location and use Go module `replace` directives consistently. Consider:
1. Using only `vendor/` as the source of truth
2. Running `go mod vendor` after any submodule update
3. Adding a pre-commit check to verify vendor sync

---

Last Updated: April 15, 2026
