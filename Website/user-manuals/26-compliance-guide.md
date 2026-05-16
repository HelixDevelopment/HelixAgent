# User Manual 26: Compliance Guide

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Compliance Architecture](#compliance-architecture)
4. [Supported Standards](#supported-standards)
5. [Data Privacy and GDPR](#data-privacy-and-gdpr)
6. [PII Detection and Redaction](#pii-detection-and-redaction)
7. [Audit Logging](#audit-logging)
8. [Access Control](#access-control)
9. [Encryption](#encryption)
10. [Security Scanning](#security-scanning)
11. [Data Retention Policies](#data-retention-policies)
12. [LLM-Specific Compliance](#llm-specific-compliance)
13. [Compliance Reporting](#compliance-reporting)
14. [Configuration Reference](#configuration-reference)
15. [Troubleshooting](#troubleshooting)
16. [Related Resources](#related-resources)

## Overview

HelixAgent processes user prompts through multiple LLM providers, stores debate sessions in PostgreSQL, caches data in Redis, and manages embeddings in vector databases. This creates compliance obligations around data privacy, data residency, audit trails, access control, and security. This manual covers how HelixAgent addresses these requirements and how to configure the system for compliance with SOC 2, GDPR, HIPAA, and ISO 27001.

The Security module (`digital.vasic.security`) provides guardrails, PII detection/redaction, content filtering, policy enforcement, and vulnerability scanning.

## Prerequisites

- HelixAgent running with all security features enabled
- PostgreSQL configured with TLS and encryption at rest
- Redis configured with authentication and TLS
- Security scanning tools installed (`gosec`, Snyk CLI, SonarQube)
- Audit log storage with appropriate retention

## Compliance Architecture

```
+-------------------+     +-------------------+     +------------------+
|  Incoming Request  |     |  Security Module   |     |  Audit Logger    |
|  (user prompt)    +---->+  - PII Detection   +---->+  - Action log    |
+-------------------+     |  - Content Filter  |     |  - Access log    |
                          |  - Policy Engine   |     |  - Data log      |
                          +--------+-----------+     +--------+---------+
                                   |                          |
                          +--------v-----------+     +--------v---------+
                          |  Guardrails Engine  |     |  PostgreSQL      |
                          |  - Input validation |     |  (encrypted,     |
                          |  - Output filtering |     |   audit tables)  |
                          |  - Rate limiting    |     +------------------+
                          +--------------------+
```

## Supported Standards

| Standard | Coverage | Key Requirements |
|---|---|---|
| SOC 2 Type II | Access controls, monitoring, encryption | Audit trails, access reviews, incident response |
| GDPR | Data privacy, consent, data subject rights | PII handling, data minimization, right to erasure |
| HIPAA | Protected health information | Encryption, access controls, audit logging |
| ISO 27001 | Information security management | Risk assessment, security controls, continuous improvement |

## Data Privacy and GDPR

### Data Minimization

HelixAgent follows data minimization principles:

1. **Prompt data** -- User prompts are processed in memory and not stored unless debate persistence is enabled
2. **Response data** -- LLM responses are cached with configurable TTL and automatically expire
3. **Session data** -- Debate sessions store only metadata by default (topology, timing, consensus status)
4. **Logs** -- Structured logs do not include prompt content by default

### Right to Erasure (Article 17)

```bash
# Delete all data for a specific user
curl -X DELETE "http://localhost:7061/v1/admin/user-data?user_id=user-123" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"

# Verify deletion
curl "http://localhost:7061/v1/admin/user-data?user_id=user-123" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}"
```

### Data Subject Access Request (Article 15)

```bash
# Export all data for a user
curl "http://localhost:7061/v1/admin/user-data/export?user_id=user-123" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -o user-123-data-export.json
```

### Consent Management

Configure consent requirements:

```yaml
compliance:
  gdpr:
    enabled: true
    require_consent: true
    consent_endpoint: /v1/consent
    data_processing_purposes:
      - "llm_query_processing"
      - "debate_session_storage"
      - "performance_analytics"
```

### Data Residency

For GDPR compliance, configure data residency to keep EU user data within the EU region. See [User Manual 25: Multi-Region Deployment](25-multi-region-deployment.md) for multi-region configuration.

## PII Detection and Redaction

The Security module includes built-in PII detection for prompts sent to LLM providers:

### Supported PII Types

| PII Type | Detection Method | Example |
|---|---|---|
| Social Security Numbers | Regex pattern | 123-45-6789 |
| Credit Card Numbers | Luhn algorithm + regex | 4111-1111-1111-1111 |
| Email Addresses | RFC 5322 pattern | user@example.com |
| Phone Numbers | International patterns | +1 (555) 123-4567 |
| IP Addresses | IPv4/IPv6 patterns | 192.168.1.1 |
| Names (contextual) | NER model | John Smith |
| Dates of Birth | Date patterns in context | born on 01/15/1990 |

### Configuration

```yaml
security:
  pii_detection:
    enabled: true
    mode: redact          # "detect", "redact", or "block"
    redaction_string: "[REDACTED]"
    types:
      - ssn
      - credit_card
      - email
      - phone
      - ip_address
    log_detections: true  # Log PII detection events (without PII content)
```

### PII Detection in Code

```go
import "dev.helix.agent/internal/security"

detector := security.NewPIIDetector(security.PIIConfig{
    Mode:       security.ModeRedact,
    Types:      []string{"ssn", "credit_card", "email"},
    Redaction:  "[REDACTED]",
})

// Before sending to LLM provider
sanitized, detections := detector.Process(userPrompt)
if len(detections) > 0 {
    logger.Warn("PII detected in prompt",
        slog.Int("count", len(detections)),
        slog.String("types", joinTypes(detections)),
    )
}
```

## Audit Logging

### Audit Event Structure

All actions are logged with a consistent structure:

```json
{
    "timestamp": "2026-03-08T10:15:30.123Z",
    "event_type": "api_request",
    "user_id": "user-123",
    "action": "chat_completion",
    "resource": "/v1/chat/completions",
    "method": "POST",
    "source_ip": "10.0.1.50",
    "outcome": "success",
    "status_code": 200,
    "duration_ms": 1250,
    "metadata": {
        "provider": "deepseek",
        "model": "deepseek-chat",
        "tokens_used": 150,
        "debate_enabled": true
    }
}
```

### Audit Event Types

| Event Type | Trigger |
|---|---|
| `api_request` | Every API endpoint call |
| `auth_success` | Successful authentication |
| `auth_failure` | Failed authentication attempt |
| `data_access` | Database read/write operations |
| `config_change` | Configuration modifications |
| `provider_switch` | Provider failover event |
| `circuit_breaker` | Circuit breaker state change |
| `pii_detection` | PII found in request/response |
| `admin_action` | Administrative operations |
| `data_export` | User data export request |
| `data_deletion` | User data deletion request |

### Audit Log Storage

Audit logs are stored in PostgreSQL for queryability and retained according to compliance requirements:

```sql
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_type  VARCHAR(50) NOT NULL,
    user_id     VARCHAR(100),
    action      VARCHAR(100) NOT NULL,
    resource    VARCHAR(255),
    source_ip   INET,
    outcome     VARCHAR(20) NOT NULL,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_timestamp ON audit_log (timestamp);
CREATE INDEX idx_audit_log_user_id ON audit_log (user_id);
CREATE INDEX idx_audit_log_event_type ON audit_log (event_type);
```

## Access Control

### Role-Based Access Control (RBAC)

```yaml
access_control:
  roles:
    admin:
      permissions:
        - "admin:*"
        - "api:*"
        - "audit:read"
    operator:
      permissions:
        - "api:*"
        - "monitoring:read"
        - "provider:manage"
    user:
      permissions:
        - "api:chat"
        - "api:models"
    readonly:
      permissions:
        - "api:models"
        - "monitoring:read"
```

### JWT Token Claims

```json
{
    "sub": "user-123",
    "role": "operator",
    "permissions": ["api:*", "monitoring:read"],
    "exp": 1741459200,
    "iss": "helixagent"
}
```

### API Key Scoping

```yaml
api_keys:
  - name: "production-client"
    key_hash: "sha256:..."
    permissions:
      - "api:chat"
      - "api:models"
    rate_limit: 600/min
    ip_whitelist:
      - "10.0.0.0/8"
```

## Encryption

### Encryption at Rest

| Component | Method | Key Management |
|---|---|---|
| PostgreSQL | AES-256 (pgcrypto / TDE) | KMS or local keyfile |
| Redis | AES-256 (Redis encryption) | Configuration password |
| Backups | GPG symmetric encryption | Passphrase in secrets |
| Configuration | Environment variables | Process-level isolation |

### Encryption in Transit

| Channel | Protocol | Configuration |
|---|---|---|
| Client to HelixAgent | TLS 1.3 | NGINX/Ingress termination |
| HelixAgent to PostgreSQL | TLS 1.3 | `sslmode=verify-full` in connection string |
| HelixAgent to Redis | TLS 1.3 | `rediss://` URL scheme |
| HelixAgent to LLM Providers | TLS 1.3 | HTTPS (enforced) |
| Inter-region replication | TLS 1.3 | Mutual TLS certificates |

### HTTP/3 with Brotli

HelixAgent mandates HTTP/3 (QUIC) as primary transport with Brotli compression, providing both performance and security benefits (QUIC encrypts all payload and most header data):

```yaml
networking:
  http3:
    enabled: true
    fallback: http2
  compression:
    primary: brotli
    fallback: gzip
```

## Security Scanning

### Static Analysis

```bash
# Run gosec (Go security checker)
make security-scan

# Run with specific rules
gosec -include=G101,G201,G301 ./...
```

### Dependency Scanning

```bash
# Check for known vulnerabilities in dependencies
go list -json -deps ./... | nancy sleuth

# Snyk scan
snyk test --file=go.mod
```

### Container Scanning

```bash
# Scan container images for vulnerabilities
trivy image helixagent:latest
```

### Continuous Scanning

Integrate scanning into the CI pipeline:

```bash
# Full security validation
make ci-validate-all
```

## Data Retention Policies

```yaml
data_retention:
  # Operational data
  cache_entries:
    ttl: 1h
    description: "LLM response cache"
  session_data:
    ttl: 30d
    description: "Debate session metadata"

  # Audit data
  audit_logs:
    ttl: 90d
    description: "API and access audit logs"
    archive_to: s3

  # Backup data
  database_backups:
    ttl: 365d
    description: "PostgreSQL full backups"
  wal_archives:
    ttl: 7d
    description: "WAL archives for PITR"

  # Analytics
  metrics_data:
    ttl: 90d
    description: "Prometheus metrics"
  trace_data:
    ttl: 30d
    description: "OpenTelemetry traces"
```

### Automated Cleanup

```bash
# Purge expired audit logs
curl -X POST "http://localhost:7061/v1/admin/purge-expired" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"older_than": "90d", "types": ["audit_logs"]}'
```

## LLM-Specific Compliance

### Provider Data Handling

When sending prompts to LLM providers, be aware of each provider's data handling policies:

| Concern | Mitigation |
|---|---|
| Prompt data retention by providers | Use providers with zero-retention policies |
| Cross-border data transfer | Use region-specific provider endpoints |
| PII in prompts | Enable PII detection and redaction before provider calls |
| Model training on user data | Verify provider opt-out policies |
| Response caching | Configure cache TTL and encryption |

### Content Guardrails

The guardrails engine filters both input and output:

```yaml
guardrails:
  input:
    max_prompt_length: 100000
    blocked_patterns:
      - "prompt injection pattern"
    content_filter:
      categories:
        - hate_speech
        - violence
        - illegal_activity
  output:
    filter_pii: true
    max_response_length: 50000
```

## Compliance Reporting

### Generate Monthly Report

```bash
./scripts/generate-compliance-report.sh

# Or via API
curl "http://localhost:7061/v1/admin/compliance-report?period=2026-02" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -o compliance-report-2026-02.json
```

### Report Contents

- Request volume and error rates
- Authentication success/failure statistics
- PII detection events (count by type, no content)
- Access control violations
- Security scan results
- Data retention compliance status
- Provider availability and failover events

## Configuration Reference

| Setting | Default | Description |
|---|---|---|
| `SECURITY_PII_ENABLED` | `true` | Enable PII detection |
| `SECURITY_PII_MODE` | `redact` | PII handling mode (detect/redact/block) |
| `SECURITY_GUARDRAILS_ENABLED` | `true` | Enable content guardrails |
| `AUDIT_LOG_ENABLED` | `true` | Enable audit logging |
| `AUDIT_LOG_RETENTION_DAYS` | `90` | Audit log retention period |
| `ENCRYPTION_AT_REST` | `true` | Enable database encryption |
| `JWT_SECRET` | (required) | JWT signing secret |
| `TLS_CERT_FILE` | `""` | TLS certificate path |
| `TLS_KEY_FILE` | `""` | TLS private key path |

## Troubleshooting

### PII Detection Producing False Positives

**Symptom:** Legitimate content is being redacted as PII.

**Solutions:**
1. Review the PII patterns in the Security module configuration
2. Add whitelisted terms or patterns
3. Switch to `detect` mode to log detections without redacting
4. Fine-tune regex patterns for your specific use case

### Audit Log Table Growing Too Large

**Symptom:** PostgreSQL disk usage increasing rapidly.

**Solutions:**
1. Implement table partitioning by month
2. Archive old audit records to cold storage (S3)
3. Reduce logging verbosity for high-frequency, low-risk events
4. Set up automated purge jobs

### JWT Token Validation Failures

**Symptom:** Authenticated requests return 401 with valid tokens.

**Solutions:**
1. Verify `JWT_SECRET` is consistent across all instances
2. Check token expiration (`exp` claim)
3. Ensure clock synchronization (NTP) across servers
4. Check the `iss` claim matches the expected issuer

## Related Resources

- [User Manual 26: Compliance Guide](#) (this document)
- [User Manual 27: API Rate Limiting](27-api-rate-limiting.md) -- Rate limiting for abuse prevention
- [User Manual 28: Custom Middleware](28-custom-middleware.md) -- Auth and security middleware
- [User Manual 30: Enterprise Architecture](30-enterprise-architecture.md) -- Enterprise security architecture
- Security module: `security/`
- Internal security: `internal/security/`
- Auth module: `Auth/`
- Debate provenance: `internal/debate/audit/`
