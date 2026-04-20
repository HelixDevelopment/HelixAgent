# CLI Agent & AI Product Tool-Schema Cross-Reference

**Source:** distilled from `docs/research/CL4R1T4S/` (pinned community corpus of reverse-engineered system prompts) cross-referenced against HelixAgent's canonical tool vocabulary (`internal/tools/schema.go` and the 48-entry `internal/agents/registry.go`).

**Last refreshed:** 2026-04-20 from CL4R1T4S HEAD.

**Scope:** for each outside agent / product whose tool catalog we can read, map its native tool names onto our canonical shorthand, and note which ones are **integrable** (the agent can be pointed at `http://localhost:7061/v1`) vs **reference-only** (hosted/closed-source; can't plug in).

---

## Canonical HelixAgent Vocabulary (21 tools)

From `internal/tools/schema.go`:

| Canonical name | Intent |
|----------------|--------|
| `Bash`         | Execute a shell command |
| `Read`         | Read a file |
| `Write`        | Create / overwrite a file |
| `Edit`         | In-place file edit (exact-string replace) |
| `Glob`         | List files by pattern / directory |
| `Grep`         | Search file contents by regex |
| `WebFetch`     | Fetch a URL |
| `WebSearch`    | Full-text web search |
| `Git`          | Git operations |
| `Test`         | Run test suite |
| `Lint`         | Run linter |
| `Diff`         | Structured diff |
| `Symbols`      | LSP symbols |
| `References`   | LSP references |
| `Definition`   | LSP jump-to-definition |
| `TreeView`     | Repo tree summary |
| `FileInfo`     | Stat / metadata |
| `Browser`      | Full browser automation |
| `Task`         | Spawn sub-agent |
| `Ask`          | Followup question to user |
| `Complete`     | End the task |

Extensions we've added for LSP/ACP/MCP-heavy agents:

| Extension | Intent |
|-----------|--------|
| `MCP`     | Invoke an MCP tool |
| `Plan`    | Plan-mode response |
| `Workflow`| Long-running orchestration |
| `PR`      | Create/manage pull requests |
| `Issue`   | Create/manage issues |

---

## Cross-Reference Matrix

### Cline (INTEGRABLE)
Source: `docs/research/CL4R1T4S/CLINE/Cline.md` (576 lines).
Already wired in `internal/agents/registry.go:Cline`.

| Cline native               | HelixAgent canonical |
|----------------------------|----------------------|
| `execute_command`          | `Bash`               |
| `read_file`                | `Read`               |
| `write_to_file`            | `Write`              |
| `replace_in_file`          | `Edit`               |
| `search_files`             | `Grep`               |
| `list_files`               | `Glob`               |
| `browser_action`           | `Browser`            |
| `web_fetch`                | `WebFetch`           |
| `use_mcp_tool`             | `MCP`                |
| `access_mcp_resource`      | `MCP`                |
| `load_mcp_documentation`   | `MCP`                |
| `ask_followup_question`    | `Ask`                |
| `attempt_completion`       | `Complete`           |
| `new_task`                 | `Task`               |
| `plan_mode_respond`        | `Plan`               |

**Coverage: 14/14 native Cline tools mapped.** `list_files` was missing from the previous registry entry and has been added.

### Cursor (REFERENCE-ONLY — hosted IDE, cannot point at custom endpoint)
Source: `docs/research/CL4R1T4S/CURSOR/Cursor_Tools.md` (11 tools).

| Cursor native      | HelixAgent canonical         |
|--------------------|------------------------------|
| `codebase_search`  | (semantic; no canonical, closest: Grep+embeddings) |
| `read_file`        | `Read`                       |
| `run_terminal_cmd` | `Bash`                       |
| `list_dir`         | `Glob`                       |
| `grep_search`      | `Grep`                       |
| `edit_file`        | `Edit`                       |
| `file_search`      | (fuzzy; we have no direct analogue — use `Glob`) |
| `delete_file`      | (no canonical — expressible via `Bash rm`) |
| `reapply`          | (Cursor-specific meta-tool; no analogue needed) |
| `web_search`       | `WebSearch`                  |
| `fetch_rules`      | (Cursor-specific rule-loading; no analogue) |

Gap observed: HelixAgent has no first-class **semantic codebase search** tool. RAG/embeddings live in `internal/rag/` and `internal/embeddings/`, but they're not surfaced through `internal/tools/schema.go`. Potential work item (requires user approval): add a `SemanticSearch` tool backed by the existing RAG pipeline.

### Windsurf / Cascade (REFERENCE-ONLY — hosted IDE)
Source: `docs/research/CL4R1T4S/WINDSURF/Windsurf_Tools.md` (19 tools).

| Windsurf native           | HelixAgent canonical     |
|---------------------------|--------------------------|
| `browser_preview`         | (deploy-preview; no analogue) |
| `check_deploy_status`     | (deploy-specific; no analogue) |
| `codebase_search`         | (semantic; see Cursor)   |
| `command_status`          | (async-Bash status; we block on Bash) |
| `create_memory`           | (Cascade memory; closest: our `internal/memory/`) |
| `deploy_web_app`          | (deploy; no analogue) |
| `find_by_name`            | `Glob`                   |
| `grep_search`             | `Grep`                   |
| `list_dir`                | `Glob`                   |
| `read_deployment_config`  | `Read`                   |
| `read_url_content`        | `WebFetch`               |
| `replace_file_content`    | `Edit`                   |
| `run_command`             | `Bash`                   |
| `search_web`              | `WebSearch`              |
| `suggested_responses`     | (UX sugar; no analogue) |
| `view_code_item`          | `Symbols` + `Read`       |
| `view_file`               | `Read`                   |
| `view_web_document_content_chunk` | `WebFetch` (chunked) |
| `write_to_file`           | `Write`                  |

Gap observed: HelixAgent has no async/long-running Bash with `command_status` polling. Our `Bash` tool blocks until completion. Our handlers have background task infrastructure (`internal/background/`), but it's not surfaced through the tool schema. Potential work item: add `BashBackground` + `BashStatus` pair.

### Manus (REFERENCE-ONLY — hosted agent)
Source: `docs/research/CL4R1T4S/MANUS/Manus_Functions.txt` (22 tools).

| Manus native        | HelixAgent canonical |
|---------------------|----------------------|
| `browser_*` (9)     | `Browser`            |
| `shell_*` (5)       | `Bash` + status/async (see Windsurf gap) |
| `file_read`         | `Read`                |
| `file_str_replace`  | `Edit`                |
| `image_view`        | (image input; HelixAgent has `Vision` protocol but not as a tool) |
| `info_search_web`   | `WebSearch`           |
| `message_ask_user`  | `Ask`                 |
| `message_notify_user` | (passive notification; no analogue) |
| `idle`              | (sleep; expressible via `Bash sleep`) |
| `deploy_apply_deployment` / `deploy_expose_port` | (deploy; no analogue) |

Gap observed: Vision is exposed as an `/v1/vision` HTTP endpoint in HelixAgent (`internal/vision/`), but there's no first-class `Vision` tool in the schema. Potential work item: add `VisionAnalyze` tool.

### Devin (REFERENCE-ONLY — hosted agent, SWE-bench-style)
Source: `docs/research/CL4R1T4S/DEVIN/Devin_2.0_Commands.md`.
Devin uses slash-commands, not a function-calling schema. No mappable vocabulary.

### Replit Agent (REFERENCE-ONLY — hosted IDE)
Source: `docs/research/CL4R1T4S/REPLIT/Replit_Functions.md`.
Replit's tool surface is wrapped in natural-language prompt scaffolds rather than structured functions. Not a clean mapping target.

### Bolt / Lovable / V0 / DROID / Same.dev / MultiOn (REFERENCE-ONLY)
All hosted products. Their prompts are in CL4R1T4S but they use bespoke function names tightly coupled to their runtime. No integration value.

---

## Identified Gaps in HelixAgent's Tool Schema

Derived from the mappings above:

| Gap                                     | Observed in                    | Proposed canonical | Priority |
|-----------------------------------------|--------------------------------|--------------------|----------|
| Semantic codebase search                | Cursor, Windsurf               | `SemanticSearch`   | Medium — RAG backend exists, just not surfaced |
| Async Bash with status polling          | Windsurf, Manus                | `BashBackground` + `BashStatus` | Medium — background/ infrastructure exists |
| First-class vision tool                 | Manus                          | `VisionAnalyze`    | Low — HTTP endpoint suffices for most callers |
| Fuzzy filename search                   | Cursor, Windsurf               | (subsume in `Glob`) | Low |
| Agent-memory write from tool layer      | Windsurf (`create_memory`)     | (already via `/v1/memory`; surfacing in schema optional) | Low |

None of these gaps are blocking. They're optional polish for cross-agent-compatibility when wrapping Cursor/Windsurf/Manus output into HelixAgent flows.

---

## How to Use This Document

- When adding a new CLI agent to `internal/agents/registry.go`, consult this matrix to populate `ToolSupport` with canonical names.
- When a user reports "X tool works in Cursor but not here", check the mapping to see whether we have an analogue, and escalate to the gap list if not.
- When evaluating a new hosted agent (Cursor, Windsurf, etc.) for *output-parsing* rather than *endpoint-plug-in*, use the native → canonical map to rewrite tool calls.
