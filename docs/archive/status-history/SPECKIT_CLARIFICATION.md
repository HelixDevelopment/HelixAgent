# SpecKit Implementation Clarification

**Date**: February 10, 2026
**Status**: ✅ CLARIFIED

---

## Critical Decision

**There is ONLY ONE SpecKit implementation**: **GitHub's SpecKit** (Specify CLI)

**Location**: `cli_agents/spec-kit/` (Git submodule)

---

## What This Means

### ✅ Use GitHub's SpecKit
- **Source**: `git@github.com:github/spec-kit.git`
- **Type**: External CLI tool via Git submodule
- **Integration**: Via HelixAgent's agent registry
- **Purpose**: Spec-driven development for the entire project

### ❌ NO Separate HelixAgent SpecKit Module
- The `SpecKit/` directory created earlier was a **misunderstanding**
- HelixAgent does NOT need its own separate SpecKit implementation
- All spec-driven development should use GitHub's SpecKit

---

## Current SpecKit Orchestrator in internal/services/

The existing code in `internal/services/speckit_orchestrator.go` should be:

**Option A (Recommended)**: **Renamed** to avoid confusion
- Rename to `workflow_orchestrator.go` or `development_flow_orchestrator.go`
- This is HelixAgent's internal 7-phase workflow, NOT GitHub's SpecKit
- Keep the functionality but clarify it's a different system

**Option B**: **Integrated with GitHub SpecKit**
- Make it a wrapper/adapter for GitHub's SpecKit
- Call out to `specify` CLI commands
- Add HelixAgent-specific enhancements

---

## Directory Structure Cleanup

### Keep:
```
cli_agents/spec-kit/          ✅ GitHub's SpecKit submodule
internal/services/
  speckit_orchestrator.go     ✅ (rename to workflow_orchestrator.go)
tests/integration/
  github_speckit_integration_test.go  ✅
challenges/scripts/
  speckit_comprehensive_validation_challenge.sh  ✅
docs/guides/
  SPECKIT_USER_GUIDE.md       ✅ (for GitHub SpecKit)
GITHUB_SPECKIT_INTEGRATION_STATUS.md  ✅
```

### Remove:
```
SpecKit/                      ❌ Remove entire directory
  go.mod
  README.md
  CLAUDE.md
  AGENTS.md
  EXTRACTION_PLAN.md
  pkg/
  tests/
```

---

## Action Items

### Immediate

1. **Remove `SpecKit/` directory** created today
   ```bash
   rm -rf SpecKit/
   ```

2. **Rename internal orchestrator** to clarify it's NOT GitHub SpecKit
   ```bash
   git mv internal/services/speckit_orchestrator.go \
         internal/services/workflow_orchestrator.go
   ```

3. **Update all references** from "SpecKit orchestrator" to "Workflow Orchestrator"

4. **Update documentation** to reflect single SpecKit source

### Documentation Updates

- **CLAUDE.md**: Clarify that SpecKit refers to GitHub's implementation
- **AGENTS.md**: Update SpecKit section to reference GitHub's tool only
- **README.md**: Clear messaging about GitHub SpecKit usage
- **User Guide**: Focus on GitHub SpecKit CLI usage with HelixAgent

---

## Terminology Going Forward

| Term | Meaning |
|------|---------|
| **SpecKit** | GitHub's Spec Kit (Specify CLI) - `cli_agents/spec-kit/` |
| **Workflow Orchestrator** | HelixAgent's internal 7-phase development flow |
| **Development Flow** | HelixAgent's orchestration system |

**DO NOT** create confusion by calling internal features "SpecKit"

---

## Integration Strategy

HelixAgent integrates with GitHub's SpecKit:

```go
// In HelixAgent, call GitHub's SpecKit CLI
func (h *HelixAgent) RunSpecKit(ctx context.Context, projectPath string) error {
    // Call specify CLI
    cmd := exec.Command("specify", "init", "--here", "--ai", "claude")
    cmd.Dir = projectPath
    return cmd.Run()
}
```

**NOT** by reimplementing it internally.

---

## Summary

- ✅ **ONE SpecKit**: GitHub's Spec Kit at `cli_agents/spec-kit/`
- ✅ **Git Submodule**: Properly configured and wired
- ❌ **No separate module**: Remove `SpecKit/` directory
- ✅ **Rename internal code**: `workflow_orchestrator.go`
- ✅ **Clear documentation**: No confusion between implementations

---

**Decision**: Use GitHub's SpecKit exclusively via Git submodule integration.

**Last Updated**: February 10, 2026
