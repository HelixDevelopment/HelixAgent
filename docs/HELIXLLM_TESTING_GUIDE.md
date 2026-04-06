# HelixLLM Testing & Verification Guide

## Overview

This guide provides comprehensive documentation for testing, verifying, and scoring HelixLLM integration using the LLMsVerifier framework. All tests are fully automated and can be re-executed at any time.

---

## Quick Start

### Run Complete Test Suite

```bash
# Run all tests with default configuration
./tests/helixllm/llmsverifier_test_suite.sh

# Run with verbose output
VERBOSE=true ./tests/helixllm/llmsverifier_test_suite.sh

# Run with custom endpoint
HELIX_LLM_ENDPOINT=https://localhost:8443 ./tests/helixllm/llmsverifier_test_suite.sh
```

### Run Integration Tests Only

```bash
# Run HelixLLM integration test script
./tests/helixllm/test_helixllm_integration.sh
```

### Run Challenge Tests

```bash
# Run challenge verification
./challenges/scripts/helixllm_integration_challenge.sh
```

---

## Test Suite Architecture

### Test Categories

| Category | Tests | Description |
|----------|-------|-------------|
| **Submodule Verification** | 5 | Verify HelixLLM submodule integrity |
| **Infrastructure** | 7 | Test containerized services |
| **Provider Integration** | 5 | Test provider implementation |
| **Configuration** | 4 | Test environment and config files |
| **API Endpoints** | 4 | Test OpenAI-compatible API |
| **Performance** | 1 | Test response times |
| **Security** | 2 | Test security configurations |

### Scoring System

Each test has a maximum score. Final grade is calculated based on percentage:

| Grade | Range | Status |
|-------|-------|--------|
| A+ | 90-100% | Production Ready |
| A | 80-89% | Very Good |
| B | 70-79% | Good |
| C | 60-69% | Acceptable |
| D | 50-59% | Needs Improvement |
| F | <50% | Failed |

---

## Detailed Test Descriptions

### 1. Submodule Verification Tests

#### Test 1.1: Submodule Directory Exists
- **Purpose**: Verify HelixLLM submodule is cloned
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `test -d 'HelixLLM' && test -f 'HelixLLM/README.md'`

#### Test 1.2: Submodule Has Internal Structure
- **Purpose**: Verify internal directory structure
- **Max Score**: 5
- **Critical**: Yes
- **Command**: `test -d 'HelixLLM/internal' && test -d 'HelixLLM/cmd'`

#### Test 1.3: Submodule Has Deployment Config
- **Purpose**: Verify deployment files exist
- **Max Score**: 5
- **Critical**: No
- **Command**: `test -d 'HelixLLM/deploy'`

#### Test 1.4: Gitmodules Entry Exists
- **Purpose**: Verify .gitmodules configuration
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `grep -q 'HelixLLM' '.gitmodules'`

#### Test 1.5: Documentation Exists
- **Purpose**: Verify integration documentation
- **Max Score**: 5
- **Critical**: No
- **Command**: `test -f 'docs/HELIXLLM_INTEGRATION.md'`

### 2. Infrastructure Tests

#### Test 2.1: PostgreSQL Container Running
- **Purpose**: Verify PostgreSQL container is running
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `podman ps | grep 'helixagent-helixllm-postgres'`

#### Test 2.2: PostgreSQL Healthy
- **Purpose**: Verify PostgreSQL is accepting connections
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `podman exec helixagent-helixllm-postgres pg_isready -U helix -d helixllm`

#### Test 2.3: Redis Container Running
- **Purpose**: Verify Redis container is running
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `podman ps | grep 'helixagent-helixllm-redis'`

#### Test 2.4: Redis Responding
- **Purpose**: Verify Redis responds to ping
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `podman exec helixagent-helixllm-redis redis-cli -a helixllm123 ping`

#### Test 2.5: Qdrant Container Running
- **Purpose**: Verify Qdrant container is running
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `podman ps | grep 'helixagent-helixllm-qdrant'`

#### Test 2.6: Qdrant Health Endpoint
- **Purpose**: Verify Qdrant health check
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `curl -sf http://localhost:6333/healthz`

#### Test 2.7: Kafka Container Running
- **Purpose**: Verify Kafka container is running
- **Max Score**: 5
- **Critical**: No
- **Command**: `podman ps | grep 'helixagent-helixllm-kafka'`

### 3. Provider Integration Tests

#### Test 3.1: Provider Implementation Exists
- **Purpose**: Verify provider.go exists
- **Max Score**: 15
- **Critical**: Yes
- **Command**: `test -f 'internal/llm/providers/helixllm/provider.go'`

#### Test 3.2: Provider Compiles
- **Purpose**: Verify provider compiles without errors
- **Max Score**: 20
- **Critical**: Yes
- **Command**: `go build ./internal/llm/providers/helixllm/...`

#### Test 3.3: Provider Registered in Registry
- **Purpose**: Verify provider is registered
- **Max Score**: 15
- **Critical**: Yes
- **Command**: `grep -q 'helixllm' 'internal/services/provider_registry.go'`

#### Test 3.4: Adapter Implementation Exists
- **Purpose**: Verify adapter.go exists
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `test -f 'internal/adapters/helixllm/adapter.go'`

#### Test 3.5: Adapter Compiles
- **Purpose**: Verify adapter compiles without errors
- **Max Score**: 15
- **Critical**: Yes
- **Command**: `go build ./internal/adapters/helixllm/...`

### 4. Configuration Tests

#### Test 4.1: Environment Variable Configured
- **Purpose**: Verify USE_HELIX_LLM is set
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `grep -q 'USE_HELIX_LLM=true' '.env'`

#### Test 4.2: Docker Compose File Exists
- **Purpose**: Verify docker-compose.helixllm.yml exists
- **Max Score**: 10
- **Critical**: Yes
- **Command**: `test -f 'docker-compose.helixllm.yml'`

#### Test 4.3: Docker Compose Valid Syntax
- **Purpose**: Verify compose file syntax
- **Max Score**: 5
- **Critical**: No
- **Command**: `podman-compose -f docker-compose.helixllm.yml config`

#### Test 4.4: Feature Flags Configured
- **Purpose**: Verify MCP/LSP/ACP flags are set
- **Max Score**: 5
- **Critical**: No
- **Command**: `grep -q 'HELIX_LLM_USE_HELIXAGENT_MCP=true' '.env'`

### 5. API Endpoint Tests

#### Test 5.1: Health Endpoint Available
- **Purpose**: Verify /internal/health responds
- **Max Score**: 15
- **Critical**: No
- **Command**: `curl -sfk 'https://localhost:8443/internal/health'`

#### Test 5.2: Models List Endpoint
- **Purpose**: Verify /v1/models responds
- **Max Score**: 10
- **Critical**: No
- **Command**: `curl -sfk 'https://localhost:8443/v1/models'`

#### Test 5.3: Chat Completion Endpoint
- **Purpose**: Verify /v1/chat/completions works
- **Max Score**: 20
- **Critical**: No
- **Command**: `curl -sfk -X POST .../v1/chat/completions -d '{...}'`

#### Test 5.4: Embeddings Endpoint
- **Purpose**: Verify /v1/embeddings works
- **Max Score**: 15
- **Critical**: No
- **Command**: `curl -sfk -X POST .../v1/embeddings -d '{...}'`

### 6. Performance Tests

#### Test 6.1: Health Check Response Time
- **Purpose**: Measure API response time
- **Scoring**:
  - < 100ms: 10 points (Excellent)
  - 100-500ms: 7 points (Good)
  - > 500ms: 3 points (Slow)

### 7. Security Tests

#### Test 7.1: TLS Configuration Present
- **Purpose**: Verify TLS certificates exist
- **Max Score**: 5
- **Critical**: No

#### Test 7.2: Environment File Protected
- **Purpose**: Verify .env file permissions
- **Max Score**: 5
- **Critical**: No

---

## Test Results Interpretation

### Understanding Results

After running the test suite, you'll find reports in:
```
reports/helixllm-verification-<timestamp>/
├── report.json     # Machine-readable results
├── REPORT.md       # Human-readable report
└── test.log        # Detailed execution log
```

### JSON Report Structure

```json
{
  "test_run": {
    "timestamp": "2026-04-05T21:04:35+03:00",
    "test_suite": "LLMsVerifier HelixLLM Validation",
    "version": "1.0.0"
  },
  "summary": {
    "total_tests": 25,
    "passed": 23,
    "failed": 0,
    "skipped": 2,
    "total_score": 245,
    "max_score": 260,
    "score_percentage": 94,
    "final_grade": "A+ (Excellent)"
  },
  "configuration": { ... },
  "results": [ ... ]
}
```

---

## Re-execution Guide

### Schedule Automated Testing

Add to crontab for periodic testing:
```bash
# Edit crontab
crontab -e

# Add line for daily testing at 2 AM
0 2 * * * cd /path/to/HelixAgent && ./tests/helixllm/llmsverifier_test_suite.sh >> /var/log/helixllm-tests.log 2>&1
```

### Manual Execution via Makefile

**Note:** This project does not use CI/CD pipelines (no GitHub Actions, GitLab CI, etc.). All builds and tests are run manually via Makefile targets or scripts.

```bash
# Run full HelixLLM test suite
./tests/helixllm/llmsverifier_test_suite.sh

# Run challenge verification
./challenges/scripts/helixllm_integration_challenge.sh

# Run with verbose output
VERBOSE=true ./tests/helixllm/llmsverifier_test_suite.sh
```

---

## Troubleshooting

### Common Issues

#### Issue: Provider Compilation Fails
```bash
# Check Go version
go version

# Verify submodules are initialized
git submodule update --init --recursive

# Try building directly
cd HelixLLM && make build
```

#### Issue: Containers Not Starting
```bash
# Check container runtime
which podman docker

# Check container status
podman ps -a

# View container logs
podman logs helixagent-helixllm-postgres
```

#### Issue: API Endpoints Not Responding
```bash
# Check if HelixLLM binary is running
pgrep -a helixllm

# Check port availability
netstat -tlnp | grep 8443

# Test manually
curl -k https://localhost:8443/internal/health
```

---

## Extending the Test Suite

### Adding Custom Tests

Edit `tests/helixllm/llmsverifier_test_suite.sh`:

```bash
# Add new test function
run_custom_tests() {
    log_section "CUSTOM TESTS"
    
    run_test "My Custom Test" \
        "my_test_command" \
        10 \
        false
}

# Call in main()
main() {
    ...
    run_custom_tests
    ...
}
```

---

## Best Practices

1. **Run tests after every deployment**
2. **Monitor trends in test scores**
3. **Investigate any score degradation**
4. **Keep test suite updated with new features**
5. **Document any test failures**

---

## Support

For issues or questions:
- Check logs in `reports/helixllm-verification-*/test.log`
- Review this guide
- Consult `docs/HELIXLLM_INTEGRATION.md`

---

*Last Updated: April 6, 2026*
