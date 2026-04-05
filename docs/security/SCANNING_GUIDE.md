# Security Scanning Workflow

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent uses six security scanning tools to identify vulnerabilities, code quality issues, and supply chain risks. Scanning is manual (per Constitution -- no automated CI/CD pipelines). This document describes each tool, how to run it, and how to triage findings.

---

## Tool Inventory

### 1. gosec -- Go Security Checker

**Purpose:** Static analysis for Go-specific security issues (SQL injection, hardcoded credentials, weak crypto, insecure TLS).
**Configuration:** `.gosec.yml` at project root
**Run:**
```bash
make security-scan-gosec
```
Or directly:
```bash
gosec -fmt json -out reports/security/gosec-report.json ./...
```
**Key exclusions in `.gosec.yml`:**
- `G404` (weak random) -- suppressed for jitter calculations in LLM provider retry logic across all 43 providers. `math/rand` is appropriate for non-security jitter; `crypto/rand` would be wasteful.
- `G115` -- suppressed for formatter executor integer conversions.
- Excluded directories: `vendor`, `testdata`, `reports`, `bin`, `docs`

### 2. staticcheck -- Go Static Analysis

**Purpose:** Broad static analysis covering correctness, performance, style, and deprecated API usage.
**Run:**
```bash
make security-scan-static
```
**Notes:** Part of the `golangci-lint` suite when run via `make lint`.

### 3. Snyk -- Dependency Vulnerability Scanning

**Purpose:** Detect known vulnerabilities in Go module dependencies (CVEs in third-party libraries).
**Infrastructure:** Containerized via `docker/security/snyk/`
**Run:**
```bash
# Start Snyk container
cd docker/security/snyk && docker compose up -d

# Or via Makefile
make security-scan
```
**Compose file:** `docker/security/snyk/docker-compose.yml`
**Dockerfile:** `docker/security/snyk/Dockerfile`
**Challenge:** `./challenges/scripts/snyk_automated_scanning_challenge.sh` (38 tests)

### 4. SonarQube -- Continuous Code Quality

**Purpose:** Code smells, bugs, vulnerabilities, security hotspots, and coverage analysis.
**Infrastructure:** Containerized via `docker/security/sonarqube/`
**Run:**
```bash
# Start SonarQube server
cd docker/security/sonarqube && docker compose up -d

# Run scanner against local SonarQube
sonar-scanner
```
**Configuration:** `docker/security/sonarqube/sonar-project.properties`
**Compose file:** `docker/security/sonarqube/docker-compose.yml`
**Challenge:** `./challenges/scripts/sonarqube_automated_scanning_challenge.sh` (45 tests)

### 5. Trivy -- Container Image Scanning

**Purpose:** Scan Docker/Podman images for OS package vulnerabilities and misconfigurations.
**Run:**
```bash
make security-scan-trivy
```
Or directly:
```bash
trivy image --severity HIGH,CRITICAL helixagent:latest
```
**Notes:** Focuses on the release container image. Scans both OS packages and application dependencies.

### 6. Semgrep -- Pattern-Based Security Analysis

**Purpose:** Custom and community rule-based scanning for security anti-patterns, OWASP issues, and language-specific vulnerabilities.
**Run:**
```bash
make security-scan-semgrep
```
This runs Semgrep in a container:
```bash
docker run --rm -v "$(PWD):/app:ro" \
  -v "$(PWD)/reports/security:/reports" \
  returntocorp/semgrep:latest \
  --config auto --json \
  --output /reports/semgrep-report.json \
  --metrics off /app
```

---

## Triage Process

### Severity Classification

| Severity | Action | Timeline |
|----------|--------|----------|
| **Critical** | Fix immediately | Same session |
| **High** | Fix before next release | Within 1-2 days |
| **Medium** | Evaluate and fix or suppress with rationale | Within 1 week |
| **Low** | Document; fix if low-effort | Best effort |
| **Info** | Review; usually no action needed | N/A |

### Triage Workflow

1. **Run all scanners** and collect reports in `reports/security/`
2. **De-duplicate** -- the same issue may appear in multiple tools
3. **Classify by severity** using the table above
4. **For Critical/High:** Create a fix, add a test that validates the fix, verify the scanner no longer flags it
5. **For Medium:** Evaluate whether the finding is a true positive. If it is, fix it. If it is a false positive, add a suppression with a comment explaining why.
6. **For suppressions:** Add to `.gosec.yml` (for gosec) or inline `//nolint` comments (for staticcheck/golangci-lint) with a rationale comment.

### Consolidated Report

```bash
make security-report
```

Generates `reports/security/consolidated-report.md` aggregating findings from all tools.

---

## `.gosec.yml` Exclusion Management

The `.gosec.yml` file suppresses known false positives. Rules for managing it:

1. **Every exclusion MUST have a comment** explaining why the finding is not a real vulnerability
2. **G404 exclusions** are pre-approved for jitter/backoff calculations in LLM providers (43 providers use `math/rand` for retry jitter -- this is intentional and security-neutral)
3. **New exclusions** require peer review and documentation in this file
4. **Periodic review:** Re-evaluate all exclusions quarterly to ensure they are still valid
5. **Never suppress G401 (weak crypto)** or **G501 (blacklisted imports)** without extraordinary justification

---

## Report Locations

| Tool | Report File |
|------|------------|
| gosec | `reports/security/gosec-report.json` |
| Trivy | `reports/security/trivy-report.json` |
| Semgrep | `reports/security/semgrep-report.json` |
| SonarQube | SonarQube web UI (containerized) |
| Snyk | `reports/security/snyk-report.json` |
| Consolidated | `reports/security/consolidated-report.md` |

---

## Penetration Tests (Code-Level)

In addition to the above scanning tools, HelixAgent has code-level penetration tests in `tests/pentest/` (build tag `pentest`):

| Test File | Attack Scenario |
|-----------|----------------|
| `rate_limit_bypass_test.go` | Attempts to bypass rate limiting |
| `auth_bypass_test.go` | Attempts to bypass authentication |
| `api_key_leakage_test.go` | Checks for API key exposure in responses/logs |
| `injection_attacks_test.go` | SQL injection, command injection, XSS |
| `ssrf_prevention_test.go` | Server-Side Request Forgery attempts |
| `ddos_resistance_test.go` | Resource exhaustion attacks |
| `provider_security_test.go` | Provider credential isolation |

Run with:
```bash
GOMAXPROCS=2 nice -n 19 go test -tags=pentest -v ./tests/pentest/ -p 1
```

---

## Cross-References

- Security architecture: `docs/security/best-practices.md`
- Threat model: `docs/security/threat-model.md`
- Vulnerability disclosure: `docs/security/vulnerability-disclosure.md`
- Suppression details: `docs/security/SUPPRESSIONS.md`
- Test strategy: `docs/testing/TEST_STRATEGY.md`
- Challenge validation: `./challenges/scripts/security_scan_validation_challenge.sh`
