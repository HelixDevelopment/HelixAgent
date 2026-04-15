# Bug Fixes and Known Issues

## Issue #11: BaseIntegration.Initialize() Fails to Extract WorkDir from Embedded BaseConfig

### Issue
All 20+ CLI agent integrations (Windsurf, Aider, Cline, Codex, etc.) that embed `base.BaseConfig` in their own `Config` struct had their `WorkDir` silently ignored by `BaseIntegration.Initialize()`. This caused agents to fall back to `~/.helixagent/agents/<type>/` instead of the configured directory.

When combined with a corrupted `projects.json` at the default path, this caused Windsurf's `Initialize()` to fail with `invalid character ':' after top-level value`.

### Root Cause
In `internal/clis/agents/base/base.go:46`, the type assertion `config.(*BaseConfig)` only matched direct `*BaseConfig` pointers. All agent configs (e.g., `*windsurf.Config`) embed `BaseConfig` as a field but are NOT `*BaseConfig` themselves, so the assertion always failed.

### Fix Applied
Added `extractWorkDir()` function using `reflect` to find embedded `BaseConfig` fields:

```go
func extractWorkDir(config interface{}) string {
    if cfg, ok := config.(*BaseConfig); ok && cfg != nil && cfg.WorkDir != "" {
        return cfg.WorkDir
    }
    v := reflect.ValueOf(config)
    if v.Kind() == reflect.Ptr && !v.IsNil() {
        v = v.Elem()
        if v.Kind() == reflect.Struct {
            if f := v.FieldByName("BaseConfig"); f.IsValid() {
                if bc, ok := f.Interface().(BaseConfig); ok && bc.WorkDir != "" {
                    return bc.WorkDir
                }
            }
        }
    }
    return ""
}
```

Also cleaned corrupted `~/.helixagent/agents/windsurf/projects.json` (17KB with malformed trailing content) and unskipped 4 Windsurf tests that were incorrectly marked as broken.

**Files Modified:**
- `internal/clis/agents/base/base.go` - Added reflection-based `extractWorkDir()`
- `internal/clis/agents/windsurf/windsurf_test.go` - Removed 4 `t.Skip()` calls

**Tests:** All 16 Windsurf tests pass with race detector clean.

---

## Issue #10: Nil Channel Close Panic in TerminateInstance

### Issue
`internal/clis/instance_manager.go` `TerminateInstance()` called `close()` on nil channels, causing a runtime panic.

### Fix Applied
Added nil checks before closing channels.

**File Modified:** `internal/clis/instance_manager.go`

---

## Issue #9: Nil Logger Panic in Multi-Instance Coordinator

### Issue
`internal/ensemble/multi_instance/coordinator.go` `NewCoordinator()` accepted nil logger and crashed on `logger.Printf()`.

### Fix Applied
Added nil check with fallback to `log.Default()`.

**File Modified:** `internal/ensemble/multi_instance/coordinator.go`

---

## Issue #8: MCP Bridge Tests Running Without Binary

### Issue
`cmd/mcp-bridge/main_test.go` tests ran `go run .` without MCP_COMMAND env set, causing failures.

### Fix Applied
Complete rewrite: build binary first, then run with MCP_COMMAND env set.

**File Modified:** `cmd/mcp-bridge/main_test.go`

---

## Issue #7: Container Tests Skipping When Runtime Available

### Issue
Tests skipped when real container runtime was detected (inverted logic).

### Fix Applied
Inverted skip condition to skip only when NO runtime is available.

**File Modified:** `cmd/helixagent/main_test.go`

---

## Issue #6: Model Name Mismatch in Test Assertions

### Issue
Test asserted model name `helixagent/helixagent-debate` but actual is `helixagent/helix-debate`.

### Fix Applied
Corrected assertion to match actual model name.

**File Modified:** `cmd/helixagent/main_test.go`

---

## Issue #5: Redis Port Mismatch

### Issue
Container running on port 6380 but tests expected 6379.

### Fix Applied
Recreated container on correct port 6379.

---

## Issue #4: Model Verification Timeout at Startup

### Issue
Model verification during startup took too long, blocking server boot.

### Fix Applied
Added `--skip-verification` flag to `cmd/helixagent/main.go`.

**File Modified:** `cmd/helixagent/main.go`

---

## Issue #3: Test Timeout Too Short

### Issue
484 test packages with 300s timeout sequential caused timeouts.

### Fix Applied
Increased to 900s with `-p 4` parallelism.

**File Modified:** `scripts/run_all_tests.sh`

---

## Issue #2: Error Swallowing in copyBuildContexts

### Issue
`internal/adapters/containers/adapter.go` logged warnings but continued on error, hiding real failures.

### Fix Applied
Changed to return aggregated errors.

**File Modified:** `internal/adapters/containers/adapter.go`

---

## Issue #1: SSH Timeout Too Short for Remote Builds

### Issue
60s timeout too short for large build context transfers to remote hosts.

### Fix Applied
Increased CommandTimeout from 60s to 300s.

**File Modified:** `Containers/pkg/remote/options.go`

---

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
