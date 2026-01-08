# Module 10: Security Best Practices

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 10: Security Best Practices
- Duration: 60 minutes
- Securing Your AI Infrastructure

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Implement security best practices
- Configure authentication and authorization
- Manage secrets securely
- Harden production deployments

---

## Slide 3: Security Architecture

**Defense in Depth:**

```
+------------------+
|    API Gateway   | <- Rate Limiting, WAF
+--------+---------+
         |
+--------v---------+
| Authentication   | <- JWT, API Keys
+--------+---------+
         |
+--------v---------+
| Authorization    | <- RBAC, Permissions
+--------+---------+
         |
+--------v---------+
| Input Validation | <- Sanitization
+--------+---------+
         |
+--------v---------+
|   Business Logic | <- Core Services
+------------------+
```

---

## Slide 4: Authentication Options

**Supported Authentication Methods:**

| Method | Use Case | Security Level |
|--------|----------|----------------|
| JWT | Web apps, APIs | High |
| API Key | Service-to-service | Medium |
| OAuth2 | Third-party apps | High |
| mTLS | Internal services | Very High |

---

## Slide 5: JWT Configuration

**Setting Up JWT Authentication:**

```yaml
security:
  jwt:
    enabled: true
    secret: ${JWT_SECRET}
    expiration: 24h
    refresh_expiration: 168h
    issuer: "helixagent"
    algorithm: HS256
```

**Never hardcode JWT_SECRET!**

---

## Slide 6: JWT Token Structure

**Token Claims:**

```json
{
  "sub": "user-123",
  "iat": 1704067200,
  "exp": 1704153600,
  "iss": "helixagent",
  "roles": ["user", "admin"],
  "permissions": [
    "read:providers",
    "write:debates",
    "execute:protocols"
  ]
}
```

---

## Slide 7: API Key Authentication

**Configuring API Keys:**

```yaml
security:
  api_keys:
    enabled: true
    header: "X-API-Key"
    rotation_interval: 90d

    keys:
      - id: "key-1"
        hash: "${API_KEY_1_HASH}"
        permissions: ["read"]
      - id: "key-2"
        hash: "${API_KEY_2_HASH}"
        permissions: ["read", "write"]
```

---

## Slide 8: Authorization (RBAC)

**Role-Based Access Control:**

```yaml
security:
  rbac:
    enabled: true
    roles:
      admin:
        - "*"
      operator:
        - "read:*"
        - "execute:debates"
        - "execute:protocols"
      viewer:
        - "read:providers"
        - "read:metrics"
```

---

## Slide 9: Permission Structure

**Permission Format:**

```
<action>:<resource>

Examples:
- read:providers     # Read provider info
- write:debates      # Create/modify debates
- execute:protocols  # Execute protocol tools
- admin:users        # Manage users
- delete:cache       # Clear cache
```

---

## Slide 10: Rate Limiting

**Protecting Against Abuse:**

```yaml
security:
  rate_limiting:
    enabled: true

    global:
      requests_per_second: 100
      burst: 200

    per_user:
      requests_per_minute: 60
      burst: 10

    per_endpoint:
      "/v1/completion":
        requests_per_minute: 30
      "/v1/debate":
        requests_per_minute: 10
```

---

## Slide 11: Input Validation

**Validating All Input:**

```go
func (h *Handler) handleRequest(c *gin.Context) {
    var req CompletionRequest

    // Bind and validate
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    // Additional validation
    if err := h.validator.Validate(req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Sanitize
    req.Prompt = sanitize(req.Prompt)
}
```

---

## Slide 12: Request Size Limits

**Preventing Resource Exhaustion:**

```yaml
server:
  max_request_size: 10MB
  max_header_size: 8KB
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
```

---

## Slide 13: Secrets Management

**Best Practices:**

| DO | DON'T |
|----|-------|
| Use environment variables | Hardcode secrets |
| Use secrets manager | Store in code |
| Rotate regularly | Use same key everywhere |
| Encrypt at rest | Log sensitive data |
| Audit access | Share credentials |

---

## Slide 14: Environment Variables

**Secure Secret Loading:**

```bash
# .env file (never commit!)
JWT_SECRET=your-256-bit-secret
CLAUDE_API_KEY=sk-ant-...
DB_PASSWORD=secure-password

# Loading in application
os.Getenv("JWT_SECRET")

# In Docker
docker run -e JWT_SECRET=... helixagent

# From secrets manager
aws secretsmanager get-secret-value --secret-id helixagent/jwt
```

---

## Slide 15: API Key Rotation

**Implementing Key Rotation:**

```yaml
security:
  api_keys:
    rotation:
      enabled: true
      interval: 90d
      grace_period: 7d
      notify_before: 14d
```

**Rotation Process:**
1. Generate new key
2. Distribute to clients
3. Grace period with both keys active
4. Revoke old key

---

## Slide 16: TLS Configuration

**HTTPS Setup:**

```yaml
server:
  tls:
    enabled: true
    cert_file: /path/to/cert.pem
    key_file: /path/to/key.pem
    min_version: TLS1.2
    cipher_suites:
      - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
      - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
```

---

## Slide 17: Database Security

**PostgreSQL Security:**

```yaml
database:
  ssl_mode: require
  ssl_cert: /path/to/client-cert.pem
  ssl_key: /path/to/client-key.pem
  ssl_root_cert: /path/to/ca-cert.pem

  connection:
    max_connections: 100
    idle_timeout: 10m
```

---

## Slide 18: Container Security

**Docker Security:**

```dockerfile
# Use non-root user
FROM golang:1.23-alpine
RUN adduser -D -g '' appuser
USER appuser

# Minimal base image
FROM scratch
COPY --from=builder /app/helixagent /helixagent
ENTRYPOINT ["/helixagent"]
```

---

## Slide 19: Network Security

**Network Configuration:**

```yaml
# docker-compose.yml
services:
  helixagent:
    networks:
      - frontend
      - backend
    ports:
      - "8080:7061"  # Only expose API port

  postgres:
    networks:
      - backend  # Not exposed externally

networks:
  frontend:
  backend:
    internal: true
```

---

## Slide 20: Audit Logging

**Security Audit Trail:**

```yaml
logging:
  audit:
    enabled: true
    level: info
    output: /var/log/helixagent/audit.log

    events:
      - authentication
      - authorization
      - api_calls
      - config_changes
      - admin_actions
```

---

## Slide 21: Security Scanning

**Running Security Tests:**

```bash
# Security scan with gosec
make security-scan

# Or directly
gosec ./...

# Security tests
make test-security

# Output:
# [gosec] Found 0 issues
```

---

## Slide 22: Security Headers

**HTTP Security Headers:**

```go
func securityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Content-Security-Policy",
            "default-src 'self'")
        c.Header("Strict-Transport-Security",
            "max-age=31536000; includeSubDomains")
        c.Next()
    }
}
```

---

## Slide 23: Production Hardening

**Hardening Checklist:**

- [ ] TLS enabled for all connections
- [ ] Rate limiting configured
- [ ] Authentication required
- [ ] RBAC implemented
- [ ] Secrets in secrets manager
- [ ] Audit logging enabled
- [ ] Security headers set
- [ ] Container runs as non-root
- [ ] Network segmentation
- [ ] Regular security scans

---

## Slide 24: Incident Response

**Security Incident Handling:**

1. **Detect**: Monitor logs and alerts
2. **Contain**: Isolate affected systems
3. **Investigate**: Analyze root cause
4. **Remediate**: Fix vulnerability
5. **Recover**: Restore normal operations
6. **Review**: Document lessons learned

---

## Slide 25: Security Monitoring

**Key Metrics to Monitor:**

| Metric | Alert Threshold |
|--------|-----------------|
| Failed auth attempts | >10/min |
| Rate limit hits | >50/min |
| 4xx errors | >100/min |
| Unusual API patterns | Anomaly detection |
| Privilege escalation | Any occurrence |

---

## Slide 26: Hands-On Lab

**Lab Exercise 10.1: Security Implementation**

Tasks:
1. Configure JWT authentication
2. Implement rate limiting
3. Set up secure secrets management
4. Run security scan with gosec
5. Review audit logs

Time: 25 minutes

---

## Slide 27: Module Summary

**Key Takeaways:**

- Defense in depth with multiple layers
- JWT for authentication, RBAC for authorization
- Never hardcode secrets
- Rate limiting prevents abuse
- Input validation essential
- Regular security scanning
- Audit logging for compliance

**Next: Module 11 - Testing and CI/CD**

---

## Speaker Notes

### Slide 3 Notes
Draw the architecture showing multiple security layers. Explain that each layer provides defense even if another fails.

### Slide 13 Notes
Emphasize the importance of secrets management. Share stories of production incidents caused by leaked secrets.

### Slide 23 Notes
Go through each checklist item. This is what a security audit would verify in production.
