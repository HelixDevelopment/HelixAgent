# User Manual 45: Security Scanning Guide

**Version:** 1.0
**Last Updated:** April 10, 2026
**Audience:** Security Engineers, DevOps Engineers, Developers

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Scanner Inventory](#scanner-inventory)
4. [Running Individual Scanners](#running-individual-scanners)
5. [Using docker-compose.security.yml Profiles](#using-docker-composesecurityyml-profiles)
6. [Reading and Interpreting Scan Reports](#reading-and-interpreting-scan-reports)
7. [Fixing Common Findings](#fixing-common-findings)
8. [Automated Scanning Workflow](#automated-scanning-workflow)
9. [Configuration Reference](#configuration-reference)
10. [Troubleshooting](#troubleshooting)

---

## Overview

HelixAgent ships with seven security scanners that cover dependency
vulnerabilities, static code analysis, container image scanning,
infrastructure-as-code misconfigurations, and pattern-based detection.
All scanners run inside Docker or Podman containers, requiring no local
tool installations. Reports land in `reports/security/` in JSON format
and are validated by dedicated challenge scripts.

### Key Principles

- **Containerized execution**: Every scanner runs in an isolated container
- **Resource-limited**: Containers respect the 30-40% host resource cap
- **Quality gates**: Critical and high-severity findings block the build
- **Unified reporting**: All tools write JSON to `reports/security/`
- **Challenge-validated**: `security_scan_resolution_challenge.sh` verifies
  scanner correctness

---

## Prerequisites

- Docker or Podman installed and running
- At least 4 GB of free memory (SonarQube requires 2-3 GB alone)
- Network access to container registries (Docker Hub, GitHub Container Registry)
- Snyk API token (optional; free tier available at snyk.io)
- HelixAgent source tree cloned with all submodules initialized
- `make` available (GNU Make 4.x recommended)
- `curl` 7.x+ and `jq` for querying results

Set up environment variables before running any scan:

```bash
# Required for Snyk authenticated scans (optional for OSS scanning)
export SNYK_TOKEN="your_snyk_api_token"

# Optional SonarQube token (default admin/admin on first boot)
export SONAR_TOKEN=""

# Optional Snyk organization
export SNYK_ORG="your_organization_id"
```

---

## Scanner Inventory

HelixAgent integrates seven security scanners, each targeting a
different class of vulnerability:

| # | Scanner | Image | Focus Area | Makefile Target |
|---|---------|-------|------------|-----------------|
| 1 | Gosec | `securego/gosec:latest` | Go source code security | `make security-scan-gosec` |
| 2 | Snyk | `snyk/snyk:golang` | Dependency vulnerabilities, code analysis | `make security-scan-snyk` |
| 3 | SonarQube | `sonarqube:community` + `sonarsource/sonar-scanner-cli` | Code quality, security hotspots, coverage | `make security-scan-sonarqube` |
| 4 | Trivy | `aquasec/trivy:latest` | Filesystem vulnerabilities, secrets, misconfigs | `make security-scan-trivy` |
| 5 | Semgrep | `returntocorp/semgrep:latest` | Pattern-based static analysis | `make security-scan-semgrep` |
| 6 | KICS | `checkmarx/kics:latest` | Infrastructure-as-code scanning (Dockerfiles, Compose, K8s) | `make security-scan-kics` |
| 7 | Grype | `anchore/grype:latest` | Vulnerability scanning (alternative to Trivy) | `make security-scan-grype` |

### Scanner Comparison

| Feature | Gosec | Snyk | SonarQube | Trivy | Semgrep | KICS | Grype |
|---------|-------|------|-----------|-------|---------|------|-------|
| Go source analysis | Yes | Yes | Yes | No | Yes | No | No |
| Dependency CVEs | No | Yes | No | Yes | No | No | Yes |
| Container images | No | Yes | No | Yes | No | No | Yes |
| IaC scanning | No | No | No | Yes | Yes | Yes | No |
| Secret detection | No | No | No | Yes | Yes | No | No |
| Code quality | No | No | Yes | No | No | No | No |
| Requires auth | No | Optional | No | No | No | No | No |

---

## Running Individual Scanners

### Step 1: Run Gosec (Go Security Checker)

Gosec performs static analysis on Go source files, detecting common
security issues such as SQL injection, hardcoded credentials, and
insecure random number generation.

```bash
make security-scan-gosec
```

The report is written to `reports/gosec-report.json`. Verify results:

```bash
cat reports/gosec-report.json | jq '.Issues | length'
```

To run Gosec directly via container:

```bash
docker run --rm -v "$(pwd):/app:rw" -w /app securego/gosec:latest \
  -fmt=json -out=/app/reports/gosec-report.json ./...
```

### Step 2: Run Snyk (Dependency and Code Analysis)

Snyk checks `go.mod` dependencies for known CVEs and performs static
code analysis on Go source files.

```bash
make security-scan-snyk
```

For authenticated scanning with richer results:

```bash
export SNYK_TOKEN="your_snyk_api_token"
make security-scan-snyk
```

To run specific Snyk scan types via Docker Compose:

```bash
# Dependency scan only
docker compose -f docker/security/snyk/docker-compose.yml \
  --profile deps run --rm snyk-deps

# Code analysis only
docker compose -f docker/security/snyk/docker-compose.yml \
  --profile code run --rm snyk-code

# Container image scan only
docker compose -f docker/security/snyk/docker-compose.yml \
  --profile container run --rm snyk-container
```

### Step 3: Run SonarQube (Code Quality and Security)

SonarQube provides deep code quality analysis including bugs,
vulnerabilities, security hotspots, code smells, and coverage
measurement. It requires a running server.

```bash
# Start server and run analysis (takes 2-3 minutes on first boot)
make security-scan-sonarqube
```

Access the dashboard at `http://localhost:9000` (default credentials:
`admin`/`admin`).

Query results via the SonarQube API:

```bash
# Project quality gate status
curl -s -u admin:admin \
  "http://localhost:9000/api/qualitygates/project_status?projectKey=helixagent" \
  | jq .

# Issues by severity
curl -s -u admin:admin \
  "http://localhost:9000/api/issues/search?componentKeys=helixagent&severities=CRITICAL,BLOCKER" \
  | jq '.issues | length'

# Key metrics
curl -s -u admin:admin \
  "http://localhost:9000/api/measures/component?component=helixagent&metricKeys=bugs,vulnerabilities,code_smells,coverage" \
  | jq .
```

### Step 4: Run Trivy (Filesystem and Container Scanner)

Trivy scans the filesystem for vulnerabilities, exposed secrets, and
infrastructure misconfigurations.

```bash
make security-scan-trivy
```

To scan a specific container image:

```bash
make security-scan-container
```

### Step 5: Run Semgrep (Pattern-Based Analysis)

Semgrep uses customizable rules to detect security anti-patterns,
injection flaws, and insecure coding practices.

```bash
make security-scan-semgrep
```

The report is written to `reports/security/semgrep-report.json`.

### Step 6: Run KICS (Infrastructure-as-Code Scanner)

KICS scans Dockerfiles, docker-compose files, Kubernetes manifests,
and Terraform configurations for misconfigurations.

```bash
make security-scan-kics
```

The report is written to `reports/security/`. Alternatively:

```bash
make security-scan-iac
```

### Step 7: Run Grype (Vulnerability Scanner)

Grype is an alternative vulnerability scanner from Anchore that
cross-references dependencies against the NVD and GitHub Advisory
databases.

```bash
make security-scan-grype
```

The report is written to `reports/security/grype-report.json`.

### Step 8: Run All Scanners

To run all scanners except SonarQube in a single command:

```bash
make security-scan
```

To run all scanners including SonarQube:

```bash
make security-scan-all
```

---

## Using docker-compose.security.yml Profiles

All seven scanners are defined in `docker-compose.security.yml` at
the project root. The file uses Docker Compose profiles to allow
selective execution.

### Compose Service Map

| Service | Container Name | Profile | Purpose |
|---------|---------------|---------|---------|
| `sonarqube` | `helixagent-sonarqube` | (default) | SonarQube server |
| `sonarqube-db` | `helixagent-sonarqube-db` | (default) | PostgreSQL for SonarQube |
| `snyk-scanner` | `helixagent-snyk-scanner` | `scan` | Snyk vulnerability scan |
| `trivy-scanner` | `helixagent-trivy-scanner` | `scan` | Trivy filesystem scan |
| `gosec-scanner` | `helixagent-gosec-scanner` | `scan` | Gosec Go analysis |
| `sonar-scanner` | `helixagent-sonar-scanner` | `scan` | SonarQube scanner CLI |
| `semgrep-scanner` | `helixagent-semgrep-scanner` | `scan` | Semgrep analysis |
| `kics-scanner` | `helixagent-kics-scanner` | `scan` | KICS IaC scan |
| `grype-scanner` | `helixagent-grype-scanner` | `scan` | Grype vulnerability scan |

### Running via Compose Directly

Start the SonarQube server (always-on service):

```bash
docker compose -f docker-compose.security.yml up -d sonarqube
```

Run a one-shot scanner using the `scan` profile:

```bash
# Gosec
docker compose -f docker-compose.security.yml \
  --profile scan run --rm gosec-scanner

# Trivy
docker compose -f docker-compose.security.yml \
  --profile scan run --rm trivy-scanner

# Semgrep
docker compose -f docker-compose.security.yml \
  --profile scan run --rm semgrep-scanner

# KICS
docker compose -f docker-compose.security.yml \
  --profile scan run --rm kics-scanner

# Grype
docker compose -f docker-compose.security.yml \
  --profile scan run --rm grype-scanner

# Snyk (requires SNYK_TOKEN)
SNYK_TOKEN=your_token docker compose -f docker-compose.security.yml \
  --profile scan run --rm snyk-scanner

# SonarQube scanner (requires sonarqube service running)
docker compose -f docker-compose.security.yml \
  --profile scan run --rm sonar-scanner
```

Stop all security services:

```bash
docker compose -f docker-compose.security.yml down

# Remove volumes for a clean slate:
docker compose -f docker-compose.security.yml down -v
```

### Podman Equivalent

Replace `docker compose` with `podman-compose`:

```bash
podman-compose -f docker-compose.security.yml up -d sonarqube
podman-compose -f docker-compose.security.yml \
  --profile scan run --rm gosec-scanner
```

---

## Reading and Interpreting Scan Reports

All reports are written to `reports/security/` in JSON format. Each
scanner produces a different schema.

### Report File Locations

| Scanner | Report Path | Format |
|---------|-------------|--------|
| Gosec | `reports/gosec-report.json` | Gosec JSON |
| Snyk | `reports/security/snyk-report.json` | Snyk JSON |
| SonarQube | `http://localhost:9000` (web UI) | API/Dashboard |
| Trivy | stdout (JSON) | Trivy JSON |
| Semgrep | `reports/security/semgrep-report.json` | SARIF-like JSON |
| KICS | `reports/security/results.json` | KICS JSON |
| Grype | `reports/security/grype-report.json` | Grype JSON |

### Severity Classification

All scanners classify findings by severity. Use this table for
prioritization:

| Severity | CVSS Range | Action Required | SLA |
|----------|-----------|-----------------|-----|
| Critical | 9.0-10.0 | Immediate fix | 24 hours |
| High | 7.0-8.9 | Urgent fix | 48 hours |
| Medium | 4.0-6.9 | Planned fix | 1 sprint |
| Low | 0.1-3.9 | Backlog | Best effort |

### Extracting Key Metrics

Count findings by severity from each report:

```bash
# Gosec: count by severity
cat reports/gosec-report.json | jq '[.Issues[].severity] | group_by(.) | map({(.[0]): length}) | add'

# Snyk: count by severity
cat reports/security/snyk-report.json | jq '[.vulnerabilities[].severity] | group_by(.) | map({(.[0]): length}) | add'

# Semgrep: count by severity
cat reports/security/semgrep-report.json | jq '[.results[].extra.severity] | group_by(.) | map({(.[0]): length}) | add'

# Grype: count by severity
cat reports/security/grype-report.json | jq '[.matches[].vulnerability.severity] | group_by(.) | map({(.[0]): length}) | add'

# KICS: count by severity
cat reports/security/results.json | jq '.severity_counters'
```

### Understanding Gosec Findings

Gosec reports include a rule ID, file location, and severity:

```json
{
  "severity": "MEDIUM",
  "confidence": "HIGH",
  "cwe": { "id": "326", "url": "..." },
  "rule_id": "G401",
  "details": "Use of weak cryptographic primitive",
  "file": "internal/security/crypto.go",
  "line": "42",
  "column": "15"
}
```

Common Gosec rules:

| Rule | Description | Fix |
|------|-------------|-----|
| G101 | Hardcoded credentials | Use environment variables |
| G104 | Unhandled error | Check and handle the error |
| G201 | SQL string formatting | Use parameterized queries |
| G301 | Poor file permissions | Use `0600` or `0644` |
| G401 | Weak crypto primitive | Use `crypto/aes` or `crypto/sha256` |
| G501 | Blocklisted import | Replace with safe alternative |

### Understanding Trivy Findings

Trivy results group vulnerabilities by target (dependency, file, image
layer):

```json
{
  "Target": "go.mod",
  "Type": "gomod",
  "Vulnerabilities": [
    {
      "VulnerabilityID": "CVE-2024-12345",
      "PkgName": "github.com/example/pkg",
      "InstalledVersion": "1.2.3",
      "FixedVersion": "1.2.4",
      "Severity": "HIGH"
    }
  ]
}
```

---

## Fixing Common Findings

### Step 1: Fix Dependency Vulnerabilities (Snyk/Trivy/Grype)

When scanners report a vulnerable dependency:

```bash
# Check the current version
go list -m github.com/example/pkg

# Update to the fixed version
go get github.com/example/pkg@v1.2.4

# Tidy and vendor
go mod tidy
go mod vendor

# Re-run the scanner to confirm the fix
make security-scan-snyk
```

### Step 2: Fix Go Source Issues (Gosec)

For hardcoded credentials (G101):

```go
// BEFORE: Hardcoded password
password := "secret123"

// AFTER: Read from environment
password := os.Getenv("DB_PASSWORD")
if password == "" {
    return fmt.Errorf("DB_PASSWORD environment variable not set")
}
```

For unhandled errors (G104):

```go
// BEFORE: Ignored error
resp, _ := http.Get(url)

// AFTER: Handle the error
resp, err := http.Get(url)
if err != nil {
    return fmt.Errorf("fetch %s: %w", url, err)
}
defer resp.Body.Close()
```

### Step 3: Fix IaC Misconfigurations (KICS)

For Dockerfile issues such as running as root:

```dockerfile
# BEFORE: Running as root
FROM golang:1.25-alpine
COPY . /app
CMD ["/app/helixagent"]

# AFTER: Non-root user
FROM golang:1.25-alpine
RUN adduser -D -u 1000 appuser
COPY --chown=appuser:appuser . /app
USER appuser
CMD ["/app/helixagent"]
```

### Step 4: Fix SonarQube Code Smells

Address findings through the SonarQube dashboard:

```bash
# Open the dashboard
# Navigate to: http://localhost:9000
# Select project "helixagent"
# Review Issues tab filtered by severity

# After fixing, re-run analysis
make security-scan-sonarqube
```

### Step 5: Suppress Accepted Risks

For Snyk, create a `.snyk` ignore policy:

```yaml
version: v1.25.0
ignore:
  SNYK-GOLANG-EXAMPLE-12345:
    - '*':
        reason: 'Accepted risk: no user input reaches this code path'
        expires: 2026-12-01T00:00:00.000Z
```

For Gosec, use inline suppression (use sparingly):

```go
// #nosec G104 -- error is logged by the caller
resp, _ := http.Get(url)
```

---

## Automated Scanning Workflow

### Full Scanning Workflow

Follow this sequence for a complete security review:

1. **Run all fast scanners first:**

```bash
make security-scan-gosec
make security-scan-trivy
make security-scan-semgrep
make security-scan-grype
```

2. **Run Snyk for dependency analysis:**

```bash
make security-scan-snyk
```

3. **Start SonarQube for deep analysis:**

```bash
make security-scan-sonarqube
```

4. **Run KICS for infrastructure review:**

```bash
make security-scan-kics
```

5. **Review and aggregate reports:**

```bash
# List all generated reports
ls -la reports/security/

# Generate a unified summary
make security-report
```

6. **Validate with the challenge script:**

```bash
./challenges/scripts/security_scan_resolution_challenge.sh
```

### Unified Security Gate

The security gate aggregates results from all scanners and makes a
pass/fail decision:

```bash
make ci-security-gate
```

The gate passes when:

- Snyk reports 0 critical and 0 unpatched high findings
- SonarQube quality gate passes
- No critical or high findings from Gosec
- All scanner containers ran successfully
- Reports are non-empty and valid JSON

### Stopping All Security Services

```bash
make security-scan-stop
```

---

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SNYK_TOKEN` | (empty) | Snyk API token for authenticated scans |
| `SNYK_ORG` | (empty) | Snyk organization ID |
| `SNYK_SEVERITY_THRESHOLD` | `high` | Minimum severity to report |
| `SONAR_TOKEN` | (empty) | SonarQube authentication token |
| `SONARQUBE_PORT` | `9000` | SonarQube server port |

### SonarQube Project Properties

Located at `docker/security/sonarqube/sonar-project.properties`:

```properties
sonar.projectKey=helixagent
sonar.projectName=HelixAgent
sonar.projectVersion=1.0
sonar.sources=.
sonar.exclusions=vendor/**,cli_agents/**,MCP/**,testdata/**,**/mock_*.go
sonar.tests=.
sonar.test.inclusions=**/*_test.go
sonar.go.coverage.reportPaths=coverage.out
sonar.go.tests.reportPaths=test-report.json
sonar.sourceEncoding=UTF-8
```

### Makefile Targets Reference

| Target | Description |
|--------|-------------|
| `make security-scan` | Run all scanners except SonarQube |
| `make security-scan-all` | Run all scanners including SonarQube |
| `make security-scan-gosec` | Gosec Go security checker |
| `make security-scan-snyk` | Snyk dependency and code analysis |
| `make security-scan-sonarqube` | SonarQube code quality and security |
| `make security-scan-trivy` | Trivy filesystem scanner |
| `make security-scan-semgrep` | Semgrep pattern-based analysis |
| `make security-scan-kics` | KICS infrastructure-as-code scanner |
| `make security-scan-grype` | Grype vulnerability scanner |
| `make security-scan-container` | Trivy container image scan |
| `make security-scan-iac` | Infrastructure-as-code scan (alias for KICS) |
| `make security-scan-go` | Go static analysis (vet, staticcheck) |
| `make security-scan-stop` | Stop all security scanning services |
| `make security-report` | Generate aggregated security report |
| `make ci-security-gate` | Run unified security quality gate |

---

## Troubleshooting

### SonarQube fails to start

**Symptom:** Container exits with `max virtual memory areas` error.

**Fix:** Increase the `vm.max_map_count` kernel parameter:

```bash
sysctl -w vm.max_map_count=524288
```

To make it permanent, add to `/etc/sysctl.conf`:

```
vm.max_map_count=524288
```

### Snyk scan returns empty results

**Symptom:** `snyk test` returns 0 issues on a project with known
vulnerabilities.

**Possible causes:**

1. Unauthenticated scan has limited coverage. Set `SNYK_TOKEN`.
2. The Go module cache inside the container is empty. Ensure the source
   tree has `go.sum` and `vendor/` available.

### Gosec reports false positives

**Symptom:** Gosec flags a line that is actually safe.

**Fix:** Suppress with `// #nosec RULE_ID -- reason`:

```go
// #nosec G104 -- error is intentionally ignored; retry handles it
resp, _ := retryableHTTPGet(url)
```

### Semgrep runs out of memory

**Symptom:** Container is killed by OOM.

**Fix:** Increase the container memory limit in
`docker-compose.security.yml`:

```yaml
semgrep-scanner:
  deploy:
    resources:
      limits:
        memory: 4G
```

### KICS reports issues in vendored code

**Symptom:** KICS flags misconfigurations inside `vendor/` or
`cli_agents/`.

**Fix:** Add exclusion paths to the KICS command:

```bash
docker run --rm -v "$(pwd):/app:ro" checkmarx/kics:latest \
  scan -p /app -o /reports --report-formats json \
  --exclude-paths vendor,cli_agents,MCP \
  --ignore-on-exit all --silent
```

### Reports directory does not exist

**Symptom:** Scanner fails with "no such file or directory" for
`reports/security/`.

**Fix:** Create the directory before running:

```bash
mkdir -p reports/security
```

The Makefile targets create this directory automatically, but direct
Docker Compose invocations may not.

### Container runtime not found

**Symptom:** `make security-scan-*` fails with "docker: command not
found".

**Fix:** HelixAgent supports both Docker and Podman. If using Podman,
the `./scripts/container-runtime.sh` script auto-detects the runtime.
Ensure either `docker` or `podman` is on the PATH.

---

## Related Resources

- [User Manual 10: Security Hardening](10-security-hardening.md)
- [User Manual 17: Security Scanning Guide](17-security-scanning-guide.md)
- [User Manual 32: Automated Security Scanning](32-automated-security-scanning.md)
- [Video Course 18: Security Scanning](../video-courses/course-18-security-scanning.md)
- [Video Course 74: Security Scanning Deep Dive](../video-courses/course-74-security-scanning.md)

---

**Document Version**: 1.0
**Last Updated**: April 10, 2026
