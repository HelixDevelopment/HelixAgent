# Security Scan Findings — 2026-04-11

Baseline snapshot taken during the Phase 4 remediation pass. Records the
current state of the four enabled scanners; triage of individual findings
is tracked separately.

## Scanners enabled this pass

| Scanner | Status | Wired via | Notes |
|---|---|---|---|
| **gosec** | ✓ Baseline captured | `make gosec-baseline` | `.gosec-baseline.json` committed |
| **gitleaks** | ✓ Clean | `make secrets-scan` → `scripts/security/secrets-scan.sh` | Zero secrets detected |
| **govulncheck** | ✓ Wired | `make deps-scan` → `scripts/security/deps-scan.sh` | Runs on demand |
| **semgrep** | ✓ Pre-existing | `.semgrep.yml` (82 rules) | Covered by `make security-scan` |
| **Trivy** | ✓ Pre-existing | `scripts/security-scan.sh` | Container scanning |
| **Snyk** | ✓ Pre-existing | `.snyk` | |
| **SonarQube** | ✓ Pre-existing | `sonar-project.properties` | |

## gosec baseline — 2026-04-11

Full JSON: `.gosec-baseline.json` (417 KB)

| Severity | Count |
|---|---|
| HIGH | 179 |
| MEDIUM | 317 |
| LOW | 163 |
| **Total** | **659** |

- Files scanned: 1063
- Lines scanned: 403,863

Most HIGH-severity gosec findings in Go codebases are `G104 errcheck`
(unchecked errors on `defer Close()` etc.), `G601 implicit-memory-aliasing`
(range-variable capture — fixed as of Go 1.22 loopvar semantics),
`G304 file-inclusion` (path from variable), and `G404 weak-random`. A proper
triage pass needs to separate true positives from `//nolint:gosec` candidates.

## gitleaks — 2026-04-11

- **Status:** clean — 0 findings.
- Report: `reports/security/secrets-2026-04-11T10-40-39Z.md`
- SARIF: `reports/security/secrets-2026-04-11T10-40-39Z.sarif`

## govulncheck

- **Status:** invocation wired. First scheduled run happens via
  `make deps-scan` (on-demand, no CI gating since pipelines are banned).

## Triage priorities

1. **HIGH-severity gosec findings** — walk the 179 entries, classify as:
   (a) true positive requiring fix, (b) false positive requiring `//nolint:gosec`
   comment with justification, (c) intentional pattern to whitelist globally.
2. **Dependency upgrades** — after first `make deps-scan` run, review
   upgrade suggestions and apply low-risk bumps.
3. **SonarQube Quality Gate** — needs to be inspected against the spun-up
   Sonar instance (via the existing compose profile, not automated).

## Non-automation reminder

Per the project Constitution, **no CI/CD pipelines, Git hooks, or
`.github/workflows/` entries may exist in this repository**. All scans run
manually via Makefile targets or by invoking the script directly. Humans
must drive triage cadence.

## Commands recap

```bash
# Regenerate gosec baseline after intentional changes
make gosec-baseline

# Dependency vulnerability check
make deps-scan

# Secrets scan (git working tree + history)
make secrets-scan

# Full scanner fleet (semgrep + trivy + snyk + gosec + go vet)
make security-scan
```

All of the above are non-interactive, respect `GOMAXPROCS=2 nice -n 19`
where applicable, and write timestamped reports to `reports/security/`.
