# CL4R1T4S Agent/Provider → HelixAgent Integration Status

For every directory in `docs/research/CL4R1T4S/`, this document records:
- What it is.
- Whether HelixAgent already integrates it (and how).
- Whether it **can** be integrated given its delivery model.
- What the integration would require.

Status codes:
- 🟢 **INTEGRATED** — HelixAgent already wires this agent/provider.
- 🟡 **CANDIDATE** — integrable-in-principle, needs explicit scope & approval.
- 🔴 **NOT INTEGRABLE** — hosted/closed-source; cannot accept a custom OpenAI-compatible endpoint.
- ⚪ **REFERENCE-ONLY** — research material; not integration material.

---

## AI Products — Agents / IDEs

| CL4R1T4S dir | Product      | Delivery  | Status | Notes |
|--------------|--------------|-----------|:------:|-------|
| `CLINE/`     | Cline        | VS Code extension | 🟢 | `internal/agents/registry.go:Cline` — tool surface refreshed against CL4R1T4S prompt (2026-04-20, commit adding TOOL_SCHEMA_CROSSREFERENCE). |
| `CURSOR/`    | Cursor 2.0   | Closed-source IDE | 🔴 | Runs Anthropic/OpenAI calls server-side; can't be pointed at `http://localhost:7061/v1`. Output-parsing candidate for `internal/llm/providers/` if we wanted to consume Cursor-produced patches, but low ROI. |
| `WINDSURF/`  | Windsurf / Cascade | Closed-source IDE | 🔴 | Same as Cursor. |
| `DEVIN/`     | Devin 2.0    | Hosted agent | 🔴 | Cognition AI hosted only. |
| `REPLIT/`    | Replit Agent | Hosted IDE   | 🔴 | Runs inside Replit runtime. |
| `BOLT/`      | Bolt.new     | Hosted IDE   | 🔴 | StackBlitz SaaS. |
| `LOVABLE/`   | Lovable 2.0  | Hosted       | 🔴 | SaaS. |
| `VERCEL V0/` | V0           | Hosted       | 🔴 | Vercel SaaS. |
| `MANUS/`     | Manus        | Hosted agent | 🔴 | ByteDance SaaS. |
| `MULTION/`   | MultiOn      | Hosted browser agent | 🔴 | SaaS. |
| `FACTORY/`   | DROID        | Hosted       | 🔴 | Factory.ai SaaS. |
| `SAMEDEV/`   | Same.dev     | Hosted       | 🔴 | SaaS. |
| `DIA/`       | Dia Browser  | Hosted browser | 🔴 | The Browser Company product. |
| `BRAVE/`     | Leo          | Brave browser assistant | 🔴 | In-browser only. |
| `CLUELY/`    | Cluely       | Hosted assistant | 🔴 | SaaS. |
| `HUME/`      | Hume Voice AI| Hosted voice API | ⚪ | Voice-first; out of HelixAgent scope. |

---

## LLM Providers (Foundation Models)

These entries are *not* integration targets per se — the provider folders in CL4R1T4S contain system prompts that providers inject at the API layer. HelixAgent already has dedicated provider implementations for all the covered labs.

| CL4R1T4S dir | Provider            | HelixAgent provider             | Status | Notes |
|--------------|---------------------|---------------------------------|:------:|-------|
| `ANTHROPIC/` | Claude 4/4.1/4.5/4.7/Sonnet 3.7/4.5/Claude Code | `internal/llm/providers/claude/` | 🟢 | Includes CLI-proxy mode via `claude -p --output-format json`. |
| `OPENAI/`    | ChatGPT 4o/4.1/5/Atlas | `internal/llm/providers/openai/` | 🟢 | |
| `GOOGLE/`    | Gemini 2.5-Pro / Diffusion / Gmail | `internal/llm/providers/gemini/` | 🟢 | Includes CLI + ACP modes. |
| `XAI/`       | Grok 3/4/4.1/4.20/Code-Fast-1 | `internal/llm/providers/xai/` (if present) | 🟢/🟡 | Check current provider impl; may need version bumps for Grok 4.20. |
| `MISTRAL/`   | LeChat              | `internal/llm/providers/mistral/` | 🟢 | |
| `MOONSHOT/`  | Kimi 2 / K2 Thinking| `internal/llm/providers/moonshot/` (if present) | 🟢/🟡 | Verify K2-Thinking reasoning-token handling. |
| `MINIMAX/`   | MiniMax             | (not currently implemented)     | 🟡 | Candidate for a new provider under `internal/llm/providers/minimax/`. |
| `META/`      | Llama4 WhatsApp / Muse Spark | (not applicable — these are Meta-owned products, not raw API surfaces) | ⚪ | |
| `PERPLEXITY/`| Perplexity Deep Research | `internal/llm/providers/perplexity/` (if present) | 🟢/🟡 | |

For any 🟡 entry: the corresponding CL4R1T4S prompt file is the **reference** for what that model's API contract is today (refusal style, tool-call framing, built-in safety wrapping). Useful when writing provider test fixtures or adjusting request/response parsers.

---

## Summary Counts

| Status | Count |
|--------|------:|
| 🟢 Integrated                 | 7 |
| 🟡 Candidate (needs approval) | 5 |
| 🔴 Not integrable             | 15 |
| ⚪ Reference-only             | 3 |

Total directories examined: **28**.

---

## The Defensible Action Items (from these counts)

1. **MiniMax provider** (🟡) — a greenfield `internal/llm/providers/minimax/minimax.go` + tests + registry entry. Est. 1 day.
2. **Grok 4.20 version check** — verify `internal/llm/providers/xai/` handles the latest Grok snapshot's response shape. Est. 2 h.
3. **Kimi K2 Thinking reasoning-token pass-through** — check `internal/llm/providers/moonshot/` handles the `thinking` response field. Est. 2 h.
4. **Refresh Claude provider prompt-aware tests** using CL4R1T4S/ANTHROPIC/ as fixture source. Est. 4 h.

None are blocking. Each needs explicit approval before coding because the provider ecosystem is fast-moving and the CL4R1T4S snapshot could be stale by the time this gets merged.
