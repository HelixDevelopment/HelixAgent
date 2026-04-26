# Known protocol mismatches between CLI agents and HelixAgent

This page documents CLI agents whose default wire protocol does NOT match
HelixAgent's OpenAI-compatible endpoint, plus the operator-side workaround
to get them talking.

If your CLI works against `https://api.openai.com/v1` (Continue, OpenCode,
Crush, HelixCode, …) — it works against HelixAgent. The four agents
below need extra setup or are not wireable today.

## Claude Code (`claude-code`)

**Symptom:** Claude Code POSTs to `/v1/messages` (Anthropic Messages API)
and HelixAgent's router returns 404.

**Why:** Claude Code is hard-wired to Anthropic's protocol — message
shape is `{model, messages: [{role, content}], max_tokens, …}` and
streaming uses `data: {type: "content_block_delta", delta: {…}}`. Our
OpenAI-compatible endpoint uses `chat.completion.chunk`.

**Status:** Tracked as Finding #20 in
`docs/development/MONITORING_REPORT_2026-04-26.md`. The plan is to add
a translating shim at `/v1/messages` that converts Anthropic↔OpenAI on
the wire and reuses HelixAgent's ensemble orchestration. Not yet
implemented.

**Today's workaround:** none — Claude Code cannot be pointed at
HelixAgent. Use OpenCode, Crush, or HelixCode for Anthropic-style
agentic workflows in the meantime.

## Gemini CLI (`gemini`)

**Symptom:** Gemini CLI bypasses HelixAgent entirely and calls
`https://generativelanguage.googleapis.com/v1beta/models/.../generateContent`
directly even when `--openai-base-url` is set.

**Why:** Gemini's CLI doesn't honor `--openai-base-url` for the
default mode; it only does so when explicitly invoked with the OpenAI
shim in the new ACP mode (still gated by `--experimental-acp`).

**Status:** Tracked as Finding #21. Plan: add a `/v1/google/v1beta/...`
translator that accepts Google's `generateContent` shape and forwards
to the ensemble. Not yet implemented.

**Today's workaround:** Use the experimental ACP mode if your Gemini
CLI build supports it: `gemini --experimental-acp --openai-base-url
http://localhost:8100/v1`. Not all builds expose this flag.

## Qwen Code (`qwen`)

**Symptom:** Qwen Code refuses to use `--openai-base-url
http://localhost:8100/v1` because its OAuth check runs before the URL
override is applied. Even with a valid HelixAgent API key, Qwen exits
with an OAuth-related error.

**Why:** The CLI's auth bootstrap is hard-coded to Alibaba's OAuth
endpoint; the `--openai-base-url` override only takes effect after
auth has succeeded.

**Today's workaround:** Set `QWEN_USE_OAUTH=false` in the CLI's
environment if your Qwen build supports that toggle, then point at
HelixAgent. Not all builds expose this. As an alternative, the binary
can use Qwen's CLI internally as a provider when a real Alibaba OAuth
token is present (`internal/llm/providers/qwen/qwen_cli.go`).

**Related:** Finding #44 — once the local Qwen OAuth tier is
discontinued (as Alibaba did 2026-04-15), the in-binary provider
sticky-disables itself after the first failure (commit `fb95624b`)
instead of hammering the dead CLI on every health check.

## GitHub Copilot CLI (`copilot`)

**Symptom:** Copilot CLI ignores `--openai-base-url` and routes through
GitHub's managed auth proxy regardless.

**Why:** The Copilot CLI uses GitHub Premium/Personal-account tokens
issued by `gh auth login`; those tokens are bound to GitHub's
endpoint and the CLI has no override path that bypasses them.

**Status:** Tracked as Finding #25. There is no first-party way to
point Copilot at a third-party endpoint today. A wrapper script
(Finding #27) could intercept Copilot's HTTP calls — out of scope for
the current cycle.

**Today's workaround:** none. Use OpenCode, Crush, or HelixCode for
agentic workflows backed by HelixAgent.

## Crush (`crush`) — TTY requirement

**Not** a protocol mismatch, but a recurring "this works locally but
not in CI/scripts" gotcha worth documenting alongside the protocol set.

**Symptom:** `echo "<prompt>" | crush` (piping in a prompt) crashes
with `bubbletea: error opening TTY`.

**Why:** Crush's default mode is interactive (Bubbletea TUI) and
requires a real TTY. Piping stdin doesn't satisfy that requirement.

**Today's workaround:** Use the non-interactive subcommand:

```bash
crush run "your prompt here"
```

`crush run` skips the TUI and emits plain text suitable for scripts,
CI, and pipes. This is the only invocation that works reliably without
a terminal.

**Verified:** During the 2026-04-26 CLI agent integration cycle,
`crush run "what is 17 * 23?"` produced the correct answer (391) and
generated +17 GIN log entries on HelixAgent port 8100.

## Status summary

| Agent | Status | Workaround |
|---|---|---|
| OpenCode | ✅ Works | None needed |
| Crush | ✅ Works (use `crush run`) | Non-interactive subcommand |
| HelixCode | ✅ Works | Generator now emits `auth.jwt_secret` (Finding #24) |
| Claude Code | ❌ Protocol mismatch (Anthropic) | Pending Finding #20 |
| Gemini CLI | ❌ Protocol mismatch (Google) | Pending Finding #21; partial via `--experimental-acp` |
| Qwen Code | ⚠️ OAuth bootstrap blocks override | `QWEN_USE_OAUTH=false` if available |
| Copilot CLI | ❌ GitHub-managed auth | None today |
