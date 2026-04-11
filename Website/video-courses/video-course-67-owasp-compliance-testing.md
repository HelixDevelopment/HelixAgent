# Video Course 67: OWASP Top 10 Compliance Testing

## Course Overview

**Duration:** 2 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 10 (Security Best Practices), Course 18 (Security Scanning)

Build and run OWASP Top 10 compliance tests for HelixAgent. Learn to validate broken access control, injection prevention, cryptographic safety, security misconfiguration, and SSRF defense through automated Go test suites.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Understand all 10 OWASP Top 10 (2021) vulnerability categories
2. Write Go tests that validate each OWASP category
3. Run the compliance test suite against a live HelixAgent instance
4. Integrate OWASP testing with existing security scanning (Gosec, Snyk, Trivy)
5. Interpret test results and prioritize remediation
6. Extend the test suite for new endpoints and attack vectors

---

## Module 1: OWASP Top 10 Overview (20 min)

### Video 1.1: The OWASP Top 10 Categories (10 min)

**Topics:**
- A01: Broken Access Control
- A02: Cryptographic Failures
- A03: Injection (SQL, XSS, Command)
- A04: Insecure Design
- A05: Security Misconfiguration
- A06: Vulnerable and Outdated Components
- A07: Identification and Authentication Failures
- A08: Software and Data Integrity Failures
- A09: Security Logging and Monitoring Failures
- A10: Server-Side Request Forgery (SSRF)

### Video 1.2: HelixAgent Attack Surface (10 min)

**Topics:**
- HTTP API endpoints (Gin framework, JSON responses)
- Authentication middleware (JWT, API keys)
- External integrations (47+ LLM providers, vector DBs)
- Internal services (PostgreSQL, Redis, MCP servers)
- File system access (tool sandbox, formatters)

---

## Module 2: Access Control and Authentication Tests (25 min)

### Video 2.1: A01 Broken Access Control (15 min)

**Topics:**
- Testing protected endpoints without auth headers
- Testing with invalid/expired JWT tokens
- Testing horizontal privilege escalation
- Test location: `tests/security/owasp_compliance_test.go`

**Code Example:**
```go
func TestOWASP_A01_BrokenAccessControl(t *testing.T) {
    router := setupTestRouter()
    
    // No auth header should return 401
    req := httptest.NewRequest("GET", "/v1/admin/users", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

### Video 2.2: A07 Authentication Failures (10 min)

**Topics:**
- JWT signature validation
- Token expiry enforcement
- Brute-force protection via rate limiting
- API key format validation

---

## Module 3: Injection Prevention Tests (25 min)

### Video 3.1: A03 SQL and Command Injection (15 min)

**Topics:**
- SQL injection payloads in query parameters
- Command injection in tool call parameters
- XSS payloads in request bodies
- Parameterized query verification

### Video 3.2: A10 SSRF Prevention (10 min)

**Topics:**
- Internal IP address blocking (127.0.0.1, 10.x, 172.16.x, 192.168.x)
- DNS rebinding prevention
- URL scheme validation (https only for external)

---

## Module 4: Configuration and Cryptographic Tests (25 min)

### Video 4.1: A02 Cryptographic Failures (10 min)

**Topics:**
- TLS configuration verification (InsecureSkipVerify=false by default)
- No secrets in API responses
- Secure header validation

### Video 4.2: A05 Security Misconfiguration (15 min)

**Topics:**
- CORS header validation
- Debug mode disabled in production
- Stack traces not leaked in error responses
- Default credentials detection

---

## Module 5: Running and Extending the Suite (25 min)

### Video 5.1: Running the Full OWASP Suite (10 min)

**Topics:**
- Command: `GOMAXPROCS=2 go test -v -run TestOWASP ./tests/security/ -count=1 -p 1`
- Integration with `make test-security`
- Resource limits for security testing
- Report generation

### Video 5.2: Extending for New Endpoints (15 min)

**Topics:**
- Adding tests for new API endpoints
- Creating custom attack payload libraries
- Integrating with Semgrep custom rules
- Continuous compliance monitoring

---

## Exercises

1. Run the OWASP compliance suite and review the results
2. Add a new A03 test for a recently added API endpoint
3. Create a custom SQL injection payload list for HelixAgent-specific parameters
4. Write an A10 SSRF test for the RAG handler's URL fetching

---

## Summary

OWASP Top 10 compliance testing provides automated validation of the most critical web application security risks. The HelixAgent OWASP test suite covers all 10 categories with Go-native tests that run against live endpoints, integrated with the existing security scanning infrastructure (Gosec, Snyk, SonarQube, Trivy).
