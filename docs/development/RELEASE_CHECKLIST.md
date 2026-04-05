# Pre-Release Checklist

**Date:** 2026-03-30
**Status:** Active

## Overview

This is the human-verified checklist that MUST be completed before every HelixAgent release. Each item requires manual confirmation. Do not rely solely on automated tools -- this checklist exists because automated checks can have false positives.

---

## 1. Build Verification

- [ ] `make build` succeeds with zero warnings
- [ ] `make build-debug` succeeds (debug symbols intact)
- [ ] `make release` produces binaries for all 5 platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
- [ ] All 7 apps build successfully: helixagent, api, grpc-server, cognee-mock, sanity-check, mcp-bridge, generate-constitution
- [ ] `make release-info` shows correct version codes and source hashes
- [ ] Release build was performed inside a container (`make release` uses `helixagent-builder`)
- [ ] `build-info.json` is present and accurate in each release directory

## 2. Code Quality

- [ ] `make fmt` produces no changes (code is already formatted)
- [ ] `make vet` reports zero issues
- [ ] `make lint` reports zero issues (golangci-lint)
- [ ] No `TODO`, `FIXME`, or `HACK` comments in production code (search with `grep -rn`)
- [ ] No unused imports or variables
- [ ] No dead code (validated by `./challenges/scripts/dead_code_elimination_challenge.sh`)

## 3. Test Suite

- [ ] `make test-unit` -- all unit tests pass
- [ ] `make test-integration` -- all integration tests pass (infrastructure running)
- [ ] `make test-e2e` -- all E2E tests pass
- [ ] `make test-security` -- all security tests pass
- [ ] `make test-stress` -- all stress tests pass (63 files, no goroutine leaks)
- [ ] `make test-fuzz` -- fuzz corpus replay passes (52 fuzz functions)
- [ ] `make test-race` -- race detector finds zero data races
- [ ] Pentest suite passes: `go test -tags=pentest ./tests/pentest/`
- [ ] Compliance suite passes: `go test -tags=compliance ./tests/compliance/`
- [ ] Chaos suite passes: `make test-chaos`

## 4. Test Coverage

- [ ] Overall coverage >= 95% (`make test-coverage`)
- [ ] No package has coverage below 80%
- [ ] Coverage report reviewed for critical paths (handlers, ensemble, debate, providers)
- [ ] Coverage gate challenge passes: `./challenges/scripts/coverage_gate_challenge.sh`

## 5. Security Scanning

- [ ] `make security-scan-gosec` -- zero unacknowledged findings
- [ ] `make security-scan-static` -- zero critical findings
- [ ] `make security-scan-trivy` -- zero HIGH/CRITICAL vulnerabilities in container image
- [ ] `make security-scan-semgrep` -- zero critical findings
- [ ] Snyk scan completed (containerized): no critical dependency vulnerabilities
- [ ] SonarQube scan completed (containerized): no blockers or critical issues
- [ ] `.gosec.yml` exclusions reviewed and still valid
- [ ] No API keys, passwords, or secrets in committed code
- [ ] Security scan validation challenge passes: `./challenges/scripts/security_scan_validation_challenge.sh`

## 6. Challenge Validation

- [ ] `./challenges/scripts/run_all_challenges.sh` -- all challenges pass
- [ ] Key individual challenges verified:
  - [ ] `full_system_boot_challenge.sh` (53 tests)
  - [ ] `all_agents_e2e_challenge.sh` (102 tests)
  - [ ] `integration_providers_challenge.sh` (47 tests)
  - [ ] `cli_agent_config_challenge.sh` (60 tests)
  - [ ] `debate_orchestrator_challenge.sh` (61 tests)
  - [ ] `helixmemory_challenge.sh` (80+ tests)
  - [ ] `helixspecifier_challenge.sh` (138 tests)

## 7. Performance

- [ ] `make benchmark-check` -- no regressions > 15% from baseline
- [ ] Startup time measured and acceptable (< 2 minutes with verification)
- [ ] Memory usage under load is bounded (validated by stress tests)
- [ ] No goroutine leaks under sustained operation

## 8. Documentation

- [ ] CLAUDE.md is current and synchronized with Constitution
- [ ] AGENTS.md is current and synchronized with Constitution
- [ ] Constitution (CONSTITUTION.json) is up to date
- [ ] All three documents are synchronized (no contradictions)
- [ ] `docs/MODULES.md` reflects current module inventory
- [ ] Provider count in documentation matches code (currently 43)
- [ ] CLI agent count matches code (currently 48)
- [ ] MCP adapter count matches code (currently 45+)
- [ ] Challenge count matches actual challenge scripts
- [ ] Documentation completeness challenge passes: `./challenges/scripts/documentation_completeness_challenge.sh`

## 9. Infrastructure

- [ ] All containers build successfully (`make docker-build`)
- [ ] Container health checks pass for all required services
- [ ] Remote distribution works if `CONTAINERS_REMOTE_ENABLED=true`
- [ ] Database migrations are current (`sql/schema/`)
- [ ] No orphaned container images from previous builds

## 10. Git Hygiene

- [ ] All changes committed (clean working tree)
- [ ] Commit messages follow Conventional Commits format
- [ ] No merge conflicts with main branch
- [ ] Submodules are at correct commits (`git submodule status`)
- [ ] `go.sum` is committed and up to date
- [ ] `vendor/` is up to date (`go mod vendor`)
- [ ] VERSION file reflects the new version

## 11. Constitution Compliance

- [ ] All 26 mandatory Constitution rules are satisfied
- [ ] No CI/CD pipelines exist (no `.github/workflows/`, no `.gitlab-ci.yml`)
- [ ] No Git hooks installed
- [ ] All builds performed in containers
- [ ] Resource limits enforced in all test execution
- [ ] HTTP/3 (QUIC) is the primary transport
- [ ] SSH-only for all Git operations

---

## Sign-Off

| Reviewer | Date | Status |
|----------|------|--------|
| (name) | (date) | APPROVED / BLOCKED |

**Blocking issues (if any):**
- (describe any blockers)

---

## Cross-References

- Release build guide: `docs/development/RELEASE_BUILD_GUIDE.md`
- Safety fixes: `docs/development/SAFETY_FIXES.md`
- Dead code audit: `docs/development/DEAD_CODE_AUDIT.md`
- Test strategy: `docs/testing/TEST_STRATEGY.md`
- Security scanning: `docs/security/SCANNING_GUIDE.md`
- Feature flags: `docs/configuration/FEATURE_FLAGS.md`
