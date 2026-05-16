# Dependabot / govulncheck Triage — 2026-Q2

**Date:** 2026-04-18 (opened), ongoing through 2026-Q2
**Scope:** Go dependency graph of HelixAgent monorepo (excludes JavaScript/TypeScript `Website/` estate, which goes through its own npm audit track).
**Sources:**
- `govulncheck ./...` (go.dev vuln DB, snapshot 2026-04-18)
- GitHub Dependabot notice received during `git push github main` on 2026-04-18:
  > "GitHub found 149 vulnerabilities on vasic-digital/HelixAgent's default branch (6 critical, 58 high, 70 moderate, 15 low)."
  — API access to `repos/*/dependabot/alerts` denied by current `GITHUB_TOKEN`; UI triage required for the full 149 count.

## 1. govulncheck actionable findings (Go)

### Before 2026-04-18 session

| ID | Module | Severity | Called? | Fix available | Disposition |
|---|---|---|---|---|---|
| GO-2026-4887 | `github.com/docker/docker@v28.5.2+incompatible` | High | **Yes** — `internal/clis/openhands/sandbox.go` | No (N/A) | **Mitigate** (see § 3) |
| GO-2026-4883 | `github.com/docker/docker@v28.5.2+incompatible` | High | **Yes** — same path | No (N/A) | **Mitigate** (see § 3) |
| GO-2026-4772 | `github.com/jackc/pgx/v5@v5.7.6` | High | No (imported, not called) | Yes — `v5.9.0` | **Fixed** 2026-04-18 via `go get github.com/jackc/pgx/v5@v5.9.0` |
| GO-2026-4771 | `github.com/jackc/pgx/v5@v5.7.6` | High | No (imported, not called) | Yes — `v5.9.0` | **Fixed** 2026-04-18 (same upgrade) |

### After pgx upgrade

```
Vulnerability #1: GO-2026-4887  Fixed in: N/A
Vulnerability #2: GO-2026-4883  Fixed in: N/A
Your code is affected by 2 vulnerabilities from 1 module.
This scan found no other vulnerabilities in packages you import or modules you require.
```

Remaining Go-level CVEs: **2** (both Docker, mitigated — see § 3).
Cleared in this round: **2** (pgx/v5 v5.7.6 → v5.9.0).

## 2. GitHub Dependabot 149-CVE inventory

The push-time banner surfaces counts GitHub tracks for the `vasic-digital/HelixAgent` mirror; these include **all** ecosystems — `go.mod`, every `package.json` under `Website/` and `cli_agents/*`, every `requirements.txt` under LLMsVerifier's Python tooling, etc. The Go-only subset is what `govulncheck` sees (4 before, 2 after).

**Action item — UI triage (cannot be done via the API with the current token):**

1. Open <https://github.com/vasic-digital/helix_agent/security/dependabot>.
2. Filter by ecosystem:
   - Go (expected ≤ 2 after this round, matches govulncheck)
   - JavaScript/TypeScript (bulk of the 149 — driven by `Website/package.json` and CLI-agent submodules, many of which are third-party and pinned per CLAUDE.md Rule 10)
   - Python (LLMsVerifier tooling)
3. For each **critical + high** finding:
   - If in an own-module (vasic-digital/* / HelixDevelopment/* / milos85vasic/*) and fix is available → bump.
   - If in a third-party submodule (`cli_agents/**`, `MCP/**`, `external/**`, `helix_qa/tools/opensource/**`) → **do not modify** (Rule 10). Record disposition as "Upstream responsibility; pin advances to fixed upstream release when published".
   - If no fix is available → document mitigation.
4. Record the triage result in this document, one section per month-end snapshot.

## 3. docker/docker mitigation (GO-2026-4887, GO-2026-4883)

Both CVEs affect `github.com/docker/docker` used in a single call site: `internal/clis/openhands/sandbox.go`. No upstream fix is available as of the govulncheck 2026-04-18 database snapshot.

### Threat surface

| Vuln | What is exploitable | Reachability in our architecture |
|---|---|---|
| GO-2026-4887 | Moby AuthZ plugin bypass when oversized request bodies are sent | **Not reachable** — we do not expose the Docker socket to untrusted callers; HelixAgent is the only Docker API consumer; request bodies are built by `sandbox.go` itself with known sizes. |
| GO-2026-4883 | Off-by-one in plugin-privilege validation | **Not reachable** — same reasoning; we do not install arbitrary Docker plugins. |

### Compensating controls

1. **Network:** the Docker socket is a unix domain socket accessible only to the `helixagent` user. No TCP exposure.
2. **Input validation:** `sandbox.go` builds container specs from a fixed schema; no user-supplied YAML/JSON goes through the Docker API verbatim.
3. **Plugin policy:** the compose files under `docker/` never install third-party Docker plugins; `kubernetes-mcp`, `k8s-mcp-server`, etc. run in pods outside the Docker daemon.
4. **Monitoring:** `internal/observability/metrics/phase3_gauges.go` tracks guardrail + pool health; any anomaly in the OpenHands sandbox trips the existing circuit breaker.

### Exit criteria (P1 closure)

- `govulncheck ./...` reports **zero called Go vulnerabilities** OR every remaining Go vulnerability has a documented compensating-control entry in this file.
- Web Dependabot dashboard shows **zero critical + zero high** for the `Go` ecosystem filter.

## 4. History

| Date | Event | By |
|---|---|---|
| 2026-04-18 | Triage opened. pgx/v5 5.7.6 → 5.9.0 applied (fixed GO-2026-4771, GO-2026-4772). Remaining 2 docker/docker vulns documented with mitigation. | Милош Васић + Claude Opus 4.7 |

## 5. Tooling

- `make deps-scan` — runs `scripts/security/deps-scan.sh` (govulncheck + markdown report to `reports/security/deps-<ts>.md`).
- `make secrets-scan` — runs `scripts/security/secrets-scan.sh` (gitleaks).
- `make security-scan` — full scanner matrix (gosec, trivy, snyk, sonarqube via `docker-compose.security.yml`).
- `make security-gates-all` (P0, 2026-04-18) — chains `security-scan-gosec` + `deps-scan` + `secrets-scan` + `security-scan-trivy`.
