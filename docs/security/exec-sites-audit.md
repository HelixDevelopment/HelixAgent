# Exec-Site Audit — 2026-04-19

This document catalogues every production file that invokes
`exec.Command` / `exec.CommandContext`. It is the authoritative record
backing the baseline in `tests/security/exec-sites-baseline.txt` and the
assertions in `challenges/scripts/exec_hygiene_challenge.sh`.

**Audit posture:** accept existing sites where the shell-out is by design
(CLI-agent Bash tool, MCP/ACP child-process transport, git-helper calls)
and enforce annotation (`//nolint:gosec` or `//#nosec G204`) on every
**new** call site. Any added file that does not appear in the baseline
and whose call line lacks an annotation fails the challenge.

## 1. `bash -c` call sites — accepted by design

The HelixAgent runtime deliberately exposes a "Bash" tool to LLM-backed
coding agents. Four production files invoke `exec.CommandContext(ctx,
"bash", "-c", command)`:

| File:Line | Purpose | Safety controls |
|---|---|---|
| `internal/tools/tool_executors.go:724` | Unified Bash-tool executor used by the HelixAgent tool-handler path | `ctx` timeout, `cmd.Dir = e.workingDir` (project-scoped), explicit env whitelist |
| `internal/clis/agents/claude_code/tool_executor.go:168` | Claude Code CLI-agent Bash tool | `isDangerousCommand(command)` filter before exec, `cmd.Dir = te.workDir`, `ctx` timeout |
| `internal/clis/agents/claude_code/claude_code.go:276` | Legacy Bash entry still used by older code paths | Same workDir + ctx contract |
| `internal/clis/agents/claude_code/claude_code.go:358` | Test-runner invocation | Hard-coded command shape (test binary + flags assembled in-package), no LLM pass-through |

**Rationale for accepting these:** the shell-out is the *product*. Agents are designed to run developer-grade shell commands on the user's own machine; the security boundary is the *LLM's prompt* (prompt injection) and the *workDir* (no traversal outside project). Blacklist-based filters (`isDangerousCommand`) exist for defence-in-depth; the primary control is the architecture.

**Audit cadence:** revisit every 6 months OR when a new CLI agent adds a Bash path OR when a CVE about shell parsing in Go lands in `govulncheck`.

## 2. Non-bash exec sites — accepted

Every other file in `tests/security/exec-sites-baseline.txt` falls into one of three categories:

- **Transport launcher** — spawns a configured MCP/ACP/LSP server per static config (the `command`/`args` come from operator-authored config files, not request bodies). Files: `internal/services/mcp_client.go`, `internal/services/acp_client.go`, `internal/services/lsp_manager.go`, `internal/mcp/bridge/*`, `internal/mcp/connection_pool.go`, `internal/mcp/preinstaller.go`, `internal/mcp/validation/validator.go`.
- **Git / tool helper** — runs a hard-coded binary (`git`, `gh`, `gofmt`, `golangci-lint`, `sed`, `grep`, `find`, `stat`, `wc`) with argv assembled in-package from validated inputs. Files: `internal/tools/handler.go`, `internal/tools/tool_executors.go`, `internal/tools/gittools/autocommit.go`, `internal/tools/cli_agent_extensions.go`, `internal/clis/aider/git_ops.go`, `internal/clis/continueagent/lsp_client.go`, `internal/mcp/servers/git_adapter.go`.
- **CLI-agent transport** — spawns the agent's own binary with transport-negotiation flags; argv is hard-coded by the provider package. Files: `internal/llm/providers/claude/claude_cli.go`, `internal/llm/providers/gemini/gemini_{acp,cli}.go`, `internal/llm/providers/junie/junie_{acp,cli}.go`, `internal/llm/providers/kimicode/kimicode_cli.go`, `internal/llm/providers/qwen/qwen_{acp,cli}.go`, `internal/llm/providers/zen/zen{,_cli,_http}.go`.

Each site carries `//#nosec G204` or `//nolint:gosec` in the code (or is being migrated to carry one). The exec_hygiene_challenge.sh lists every such file in the baseline so a drift check catches additions.

## 3. Explicitly sandboxed exec sites

`internal/tools/sandbox/sandbox.go` is the centralised sandbox implementation. Its own use of `exec.Command(Context)` is by contract — this is the wrapper that protects OTHER exec callers. Its internal sites are:

- `isRuntimeAvailable` — `exec.Command(runtime, "version")` — version probe only.
- `Sandbox.executeDirect` — `exec.CommandContext(ctx, command[0], command[1:]...)` — the direct-mode fallback used only when container runtimes are unavailable; callers acknowledge the direct-mode risk via explicit config opt-in.
- `Sandbox.Execute` / streaming variant — `exec.CommandContext(ctx, string(s.config.Runtime), args...)` — launches a containerised runtime which then enforces isolation.

## 4. Forbidden patterns (never accepted)

- `/bin/sh` — never use (`bash` is the shell; explicit binaries are preferred otherwise). Enforced by `exec_hygiene_challenge.sh` T3.
- `os/exec` via `os.StartProcess` bypassing `exec.Command` — blocked by code review (no current usage).
- Any call whose argv is built by `fmt.Sprintf` with untrusted string input — flagged by code review.

## 5. Follow-up (P1 completion)

- **New:** add a code-review checklist entry requiring every new `exec.Command(Context)?` site to touch this document.
- **New:** evaluate a per-call taint-analysis linter (`gosec -tags G204` with custom rules) as a future quality gate.
- **Deferred to P3 when we add fuzz suites:** fuzz the `isDangerousCommand` filter for bypass classes (quote escape, environment-variable substitution, command substitution).
