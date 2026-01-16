# Video Course 10: Security Best Practices

## Course Overview

**Duration:** 4.5 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 03 (Deployment)

Master security best practices for HelixAgent, including authentication, authorization, secure configuration, vulnerability management, and compliance.

---

## Module 1: Security Architecture

### Video 1.1: Defense in Depth (25 min)

**Topics:**
- Security layers
- Trust boundaries
- Attack surface reduction
- Security controls

**Security Architecture:**
```
┌────────────────────────────────────────────────────────────────────┐
│                         External Network                            │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                     WAF / DDoS Protection                     │  │
│  └─────────────────────────────┬────────────────────────────────┘  │
│                                │                                    │
│  ┌─────────────────────────────▼────────────────────────────────┐  │
│  │                    Load Balancer (TLS)                        │  │
│  └─────────────────────────────┬────────────────────────────────┘  │
└────────────────────────────────┼────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────┐
│                         Application Zone                            │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    HelixAgent Instances                       │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐              │  │
│  │  │Rate Limiter│  │   Auth     │  │Input Valid.│              │  │
│  │  └────────────┘  └────────────┘  └────────────┘              │  │
│  └─────────────────────────────┬────────────────────────────────┘  │
│                                │                                    │
│  ┌─────────────────────────────▼────────────────────────────────┐  │
│  │                      Internal Network                         │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐              │  │
│  │  │ PostgreSQL │  │   Redis    │  │  Secrets   │              │  │
│  │  │(Encrypted) │  │(Encrypted) │  │  Manager   │              │  │
│  │  └────────────┘  └────────────┘  └────────────┘              │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### Video 1.2: Threat Modeling (30 min)

**Topics:**
- STRIDE methodology
- Asset identification
- Threat identification
- Risk assessment

**STRIDE Analysis:**
| Threat | Example | Mitigation |
|--------|---------|------------|
| **S**poofing | Forged API keys | Strong authentication, key rotation |
| **T**ampering | Modified requests | Input validation, HMAC signatures |
| **R**epudiation | Denied actions | Comprehensive audit logging |
| **I**nformation Disclosure | API key leaks | Encryption, secret management |
| **D**enial of Service | Request flooding | Rate limiting, WAF |
| **E**levation of Privilege | Admin access | RBAC, least privilege |

### Video 1.3: Secure Configuration (20 min)

**Topics:**
- Default deny
- Principle of least privilege
- Configuration hardening
- Secure defaults

**Secure Defaults:**
```yaml
# configs/production.yaml
security:
  # Authentication
  auth:
    enabled: true
    require_api_key: true
    jwt_expiration: 1h
    refresh_token_expiration: 24h

  # Rate limiting
  rate_limit:
    enabled: true
    requests_per_minute: 60
    burst_size: 10

  # TLS
  tls:
    enabled: true
    min_version: "1.2"
    cipher_suites:
      - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
      - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256

  # Headers
  headers:
    strict_transport_security: "max-age=31536000; includeSubDomains"
    content_security_policy: "default-src 'self'"
    x_frame_options: "DENY"
    x_content_type_options: "nosniff"

  # CORS
  cors:
    allowed_origins:
      - "https://app.helixagent.ai"
    allowed_methods:
      - GET
      - POST
    allowed_headers:
      - Authorization
      - Content-Type
    max_age: 3600
```

---

## Module 2: Authentication and Authorization

### Video 2.1: API Key Management (25 min)

**Topics:**
- Key generation
- Key storage
- Key rotation
- Key revocation

**Key Management:**
```go
// internal/auth/apikey.go
type APIKeyManager struct {
    store     KeyStore
    hasher    crypto.Hash
    validator *validator.Validate
}

// Generate secure API key
func (m *APIKeyManager) GenerateKey(userID string, scopes []string) (*APIKey, error) {
    // Generate 32 random bytes
    keyBytes := make([]byte, 32)
    if _, err := rand.Read(keyBytes); err != nil {
        return nil, fmt.Errorf("failed to generate key: %w", err)
    }

    // Encode as base64 with prefix
    rawKey := "sk-" + base64.RawURLEncoding.EncodeToString(keyBytes)

    // Hash for storage (never store raw key)
    hashedKey := m.hashKey(rawKey)

    key := &APIKey{
        ID:        uuid.New().String(),
        UserID:    userID,
        KeyHash:   hashedKey,
        Prefix:    rawKey[:10], // Store prefix for identification
        Scopes:    scopes,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(90 * 24 * time.Hour), // 90 days
    }

    if err := m.store.Save(key); err != nil {
        return nil, err
    }

    // Return raw key only once (user must save it)
    key.RawKey = rawKey
    return key, nil
}

// Validate API key
func (m *APIKeyManager) ValidateKey(rawKey string) (*APIKey, error) {
    if !strings.HasPrefix(rawKey, "sk-") {
        return nil, ErrInvalidKeyFormat
    }

    hashedKey := m.hashKey(rawKey)
    key, err := m.store.FindByHash(hashedKey)
    if err != nil {
        return nil, ErrKeyNotFound
    }

    if time.Now().After(key.ExpiresAt) {
        return nil, ErrKeyExpired
    }

    if key.Revoked {
        return nil, ErrKeyRevoked
    }

    return key, nil
}
```

### Video 2.2: JWT Authentication (30 min)

**Topics:**
- JWT structure
- Token signing
- Token validation
- Refresh tokens

**JWT Implementation:**
```go
// internal/auth/jwt.go
type JWTManager struct {
    secretKey     []byte
    accessTTL     time.Duration
    refreshTTL    time.Duration
    signingMethod jwt.SigningMethod
}

type Claims struct {
    UserID string   `json:"uid"`
    Email  string   `json:"email"`
    Roles  []string `json:"roles"`
    jwt.RegisteredClaims
}

func (m *JWTManager) GenerateTokenPair(user *User) (*TokenPair, error) {
    now := time.Now()

    // Access token
    accessClaims := &Claims{
        UserID: user.ID,
        Email:  user.Email,
        Roles:  user.Roles,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
            IssuedAt:  jwt.NewNumericDate(now),
            NotBefore: jwt.NewNumericDate(now),
            Issuer:    "helixagent",
            Subject:   user.ID,
            ID:        uuid.New().String(),
        },
    }

    accessToken := jwt.NewWithClaims(m.signingMethod, accessClaims)
    accessString, err := accessToken.SignedString(m.secretKey)
    if err != nil {
        return nil, err
    }

    // Refresh token (longer lived, fewer claims)
    refreshClaims := &jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
        IssuedAt:  jwt.NewNumericDate(now),
        Issuer:    "helixagent",
        Subject:   user.ID,
        ID:        uuid.New().String(),
    }

    refreshToken := jwt.NewWithClaims(m.signingMethod, refreshClaims)
    refreshString, err := refreshToken.SignedString(m.secretKey)
    if err != nil {
        return nil, err
    }

    return &TokenPair{
        AccessToken:  accessString,
        RefreshToken: refreshString,
        ExpiresIn:    int(m.accessTTL.Seconds()),
    }, nil
}

func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if token.Method != m.signingMethod {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return m.secretKey, nil
    })

    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }

    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, ErrInvalidToken
    }

    return claims, nil
}
```

### Video 2.3: Role-Based Access Control (25 min)

**Topics:**
- Role definitions
- Permission mapping
- Policy enforcement
- Audit logging

**RBAC Implementation:**
```go
// internal/auth/rbac.go
type Role string

const (
    RoleAdmin     Role = "admin"
    RoleOperator  Role = "operator"
    RoleDeveloper Role = "developer"
    RoleReadOnly  Role = "readonly"
)

type Permission string

const (
    PermissionDebateCreate  Permission = "debate:create"
    PermissionDebateRead    Permission = "debate:read"
    PermissionDebateDelete  Permission = "debate:delete"
    PermissionProviderAdmin Permission = "provider:admin"
    PermissionConfigRead    Permission = "config:read"
    PermissionConfigWrite   Permission = "config:write"
)

var rolePermissions = map[Role][]Permission{
    RoleAdmin: {
        PermissionDebateCreate, PermissionDebateRead, PermissionDebateDelete,
        PermissionProviderAdmin, PermissionConfigRead, PermissionConfigWrite,
    },
    RoleOperator: {
        PermissionDebateCreate, PermissionDebateRead,
        PermissionProviderAdmin, PermissionConfigRead,
    },
    RoleDeveloper: {
        PermissionDebateCreate, PermissionDebateRead,
        PermissionConfigRead,
    },
    RoleReadOnly: {
        PermissionDebateRead, PermissionConfigRead,
    },
}

type RBACEnforcer struct {
    logger *slog.Logger
}

func (e *RBACEnforcer) Authorize(ctx context.Context, permission Permission) error {
    claims, ok := ctx.Value(ClaimsKey).(*Claims)
    if !ok {
        return ErrNoAuthentication
    }

    for _, role := range claims.Roles {
        if e.hasPermission(Role(role), permission) {
            e.logger.Info("authorization granted",
                "user_id", claims.UserID,
                "role", role,
                "permission", permission)
            return nil
        }
    }

    e.logger.Warn("authorization denied",
        "user_id", claims.UserID,
        "roles", claims.Roles,
        "permission", permission)
    return ErrInsufficientPermissions
}

func (e *RBACEnforcer) hasPermission(role Role, permission Permission) bool {
    perms, ok := rolePermissions[role]
    if !ok {
        return false
    }
    return slices.Contains(perms, permission)
}
```

---

## Module 3: Data Security

### Video 3.1: Encryption at Rest (25 min)

**Topics:**
- Database encryption
- File encryption
- Key management
- Envelope encryption

**Database Encryption:**
```sql
-- Enable TDE (Transparent Data Encryption) in PostgreSQL
-- Use pgcrypto extension

-- Encrypt sensitive columns
ALTER TABLE api_keys
    ALTER COLUMN key_hash TYPE bytea
    USING pgp_sym_encrypt(key_hash::text, current_setting('app.encryption_key'))::bytea;

-- Create encrypted column wrapper
CREATE OR REPLACE FUNCTION encrypt_sensitive(data text)
RETURNS bytea AS $$
BEGIN
    RETURN pgp_sym_encrypt(data, current_setting('app.encryption_key'));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION decrypt_sensitive(data bytea)
RETURNS text AS $$
BEGIN
    RETURN pgp_sym_decrypt(data, current_setting('app.encryption_key'));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
```

**Application-Level Encryption:**
```go
// internal/crypto/encryption.go
type Encryptor struct {
    key []byte
}

func NewEncryptor(keyBase64 string) (*Encryptor, error) {
    key, err := base64.StdEncoding.DecodeString(keyBase64)
    if err != nil {
        return nil, err
    }
    if len(key) != 32 {
        return nil, errors.New("key must be 32 bytes for AES-256")
    }
    return &Encryptor{key: key}, nil
}

func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return nil, err
    }

    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return ciphertext, nil
}

func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, errors.New("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

### Video 3.2: Encryption in Transit (20 min)

**Topics:**
- TLS configuration
- Certificate management
- mTLS
- Protocol security

**TLS Configuration:**
```go
// internal/server/tls.go
func ConfigureTLS(certFile, keyFile string) (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }

    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS12,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
        },
        PreferServerCipherSuites: true,
        CurvePreferences: []tls.CurveID{
            tls.X25519,
            tls.CurveP256,
        },
    }, nil
}
```

### Video 3.3: Secret Management (30 min)

**Topics:**
- HashiCorp Vault integration
- Secret rotation
- Dynamic secrets
- Access policies

**Vault Integration:**
```go
// internal/secrets/vault.go
type VaultClient struct {
    client *vault.Client
    path   string
}

func NewVaultClient(addr, token string) (*VaultClient, error) {
    config := vault.DefaultConfig()
    config.Address = addr

    client, err := vault.NewClient(config)
    if err != nil {
        return nil, err
    }

    client.SetToken(token)

    return &VaultClient{
        client: client,
        path:   "secret/data/helixagent",
    }, nil
}

func (v *VaultClient) GetSecret(key string) (string, error) {
    secret, err := v.client.Logical().Read(v.path + "/" + key)
    if err != nil {
        return "", err
    }

    if secret == nil || secret.Data == nil {
        return "", fmt.Errorf("secret not found: %s", key)
    }

    data, ok := secret.Data["data"].(map[string]interface{})
    if !ok {
        return "", errors.New("invalid secret format")
    }

    value, ok := data["value"].(string)
    if !ok {
        return "", errors.New("secret value not a string")
    }

    return value, nil
}

func (v *VaultClient) RotateAPIKey(providerID string) error {
    // Generate new key
    newKey, err := generateAPIKey()
    if err != nil {
        return err
    }

    // Store in Vault
    _, err = v.client.Logical().Write(v.path+"/providers/"+providerID, map[string]interface{}{
        "data": map[string]interface{}{
            "api_key":    newKey,
            "rotated_at": time.Now().UTC().Format(time.RFC3339),
        },
    })

    return err
}
```

---

## Module 4: Input Validation and Sanitization

### Video 4.1: Request Validation (25 min)

**Topics:**
- Schema validation
- Type checking
- Size limits
- Content validation

**Validation Implementation:**
```go
// internal/validation/validator.go
type RequestValidator struct {
    validate *validator.Validate
}

func NewRequestValidator() *RequestValidator {
    v := validator.New()

    // Custom validations
    v.RegisterValidation("safe_string", validateSafeString)
    v.RegisterValidation("model_name", validateModelName)

    return &RequestValidator{validate: v}
}

// Validate chat completion request
type ChatCompletionRequest struct {
    Model       string    `json:"model" validate:"required,model_name"`
    Messages    []Message `json:"messages" validate:"required,min=1,max=100,dive"`
    MaxTokens   int       `json:"max_tokens" validate:"omitempty,min=1,max=128000"`
    Temperature float64   `json:"temperature" validate:"omitempty,min=0,max=2"`
    Stream      bool      `json:"stream"`
}

type Message struct {
    Role    string `json:"role" validate:"required,oneof=system user assistant tool"`
    Content string `json:"content" validate:"required,max=1000000,safe_string"`
}

func validateSafeString(fl validator.FieldLevel) bool {
    s := fl.Field().String()

    // Check for null bytes
    if strings.ContainsRune(s, 0) {
        return false
    }

    // Check for control characters (except newline, tab)
    for _, r := range s {
        if r < 32 && r != '\n' && r != '\t' && r != '\r' {
            return false
        }
    }

    return true
}

func validateModelName(fl validator.FieldLevel) bool {
    model := fl.Field().String()
    // Only allow alphanumeric, dash, underscore, colon, dot
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-_:.]+$`, model)
    return matched
}

func (v *RequestValidator) ValidateRequest(req interface{}) error {
    if err := v.validate.Struct(req); err != nil {
        var validationErrors validator.ValidationErrors
        if errors.As(err, &validationErrors) {
            return formatValidationErrors(validationErrors)
        }
        return err
    }
    return nil
}
```

### Video 4.2: SQL Injection Prevention (20 min)

**Topics:**
- Parameterized queries
- Query builders
- ORM safety
- Testing for SQLi

**Safe Database Access:**
```go
// internal/database/repository.go

// NEVER do this - vulnerable to SQL injection
func (r *Repository) UnsafeSearch(query string) ([]Task, error) {
    // BAD: String concatenation
    sql := "SELECT * FROM tasks WHERE title LIKE '%" + query + "%'"
    return r.db.Query(sql)
}

// ALWAYS do this - safe parameterized query
func (r *Repository) SafeSearch(ctx context.Context, query string) ([]Task, error) {
    // GOOD: Parameterized query
    sql := `SELECT id, title, status, created_at
            FROM tasks
            WHERE title ILIKE $1
            ORDER BY created_at DESC
            LIMIT 100`

    rows, err := r.db.QueryContext(ctx, sql, "%"+query+"%")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tasks []Task
    for rows.Next() {
        var t Task
        if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.CreatedAt); err != nil {
            return nil, err
        }
        tasks = append(tasks, t)
    }

    return tasks, rows.Err()
}

// Using query builder (sqlx/squirrel)
func (r *Repository) SafeSearchWithBuilder(ctx context.Context, filters TaskFilters) ([]Task, error) {
    query := squirrel.Select("id", "title", "status", "created_at").
        From("tasks").
        OrderBy("created_at DESC").
        Limit(100)

    if filters.Title != "" {
        query = query.Where(squirrel.ILike{"title": "%" + filters.Title + "%"})
    }

    if filters.Status != "" {
        query = query.Where(squirrel.Eq{"status": filters.Status})
    }

    sql, args, err := query.PlaceholderFormat(squirrel.Dollar).ToSql()
    if err != nil {
        return nil, err
    }

    return r.queryTasks(ctx, sql, args...)
}
```

### Video 4.3: XSS and Output Encoding (20 min)

**Topics:**
- Output encoding
- Content-Type headers
- HTML sanitization
- JSON safety

**Output Safety:**
```go
// internal/handlers/response.go

// Safe JSON response
func (h *Handler) JSONResponse(c *gin.Context, status int, data interface{}) {
    // Set security headers
    c.Header("Content-Type", "application/json; charset=utf-8")
    c.Header("X-Content-Type-Options", "nosniff")

    c.JSON(status, data)
}

// Safe HTML response (rare, but if needed)
func (h *Handler) HTMLResponse(c *gin.Context, status int, content string) {
    // Sanitize HTML
    p := bluemonday.UGCPolicy()
    sanitized := p.Sanitize(content)

    c.Header("Content-Type", "text/html; charset=utf-8")
    c.Header("X-Content-Type-Options", "nosniff")
    c.Header("X-XSS-Protection", "1; mode=block")

    c.String(status, sanitized)
}

// Escape user data in logs
func (h *Handler) logUserInput(ctx context.Context, input string) {
    // Escape for logging
    escaped := strings.ReplaceAll(input, "\n", "\\n")
    escaped = strings.ReplaceAll(escaped, "\r", "\\r")

    // Truncate for safety
    if len(escaped) > 1000 {
        escaped = escaped[:1000] + "...[truncated]"
    }

    h.logger.Info("user input received",
        "input", escaped,
        "input_length", len(input))
}
```

---

## Module 5: Vulnerability Management

### Video 5.1: Dependency Scanning (25 min)

**Topics:**
- Go vulnerability scanning
- Container scanning
- SBOM generation
- Remediation workflow

**Scanning Tools:**
```bash
#!/bin/bash
# security-scan.sh

set -e

echo "=== Go Vulnerability Scan ==="
govulncheck ./...

echo "=== Dependency Audit ==="
go list -m -json all | nancy sleuth

echo "=== Container Image Scan ==="
trivy image helixagent:latest --severity HIGH,CRITICAL

echo "=== Secret Detection ==="
gitleaks detect --source . --verbose

echo "=== SBOM Generation ==="
syft packages dir:. -o cyclonedx-json > sbom.json

echo "=== License Check ==="
go-licenses check ./... --disallowed_types=restricted
```

**CI Integration:**
```yaml
# .github/workflows/security.yml
name: Security Scan

on:
  push:
    branches: [main]
  pull_request:
  schedule:
    - cron: '0 0 * * *'  # Daily

jobs:
  vulnerability-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Go Vulnerability Scan
        uses: golang/govulncheck-action@v1
        with:
          go-version-input: '1.24'

      - name: Nancy Dependency Scan
        uses: sonatype-nexus-community/nancy-github-action@v1

      - name: Trivy Container Scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: 'helixagent:latest'
          severity: 'HIGH,CRITICAL'
          exit-code: '1'

      - name: Gitleaks Secret Scan
        uses: gitleaks/gitleaks-action@v2
```

### Video 5.2: Security Testing (30 min)

**Topics:**
- SAST (Static Analysis)
- DAST (Dynamic Analysis)
- Fuzzing
- Penetration testing

**Security Tests:**
```go
// tests/security/auth_test.go
func TestAuthenticationBypass(t *testing.T) {
    router := setupTestRouter()

    tests := []struct {
        name           string
        authorization  string
        expectedStatus int
    }{
        {
            name:           "missing auth header",
            authorization:  "",
            expectedStatus: http.StatusUnauthorized,
        },
        {
            name:           "invalid token format",
            authorization:  "Bearer invalid",
            expectedStatus: http.StatusUnauthorized,
        },
        {
            name:           "expired token",
            authorization:  "Bearer " + expiredToken,
            expectedStatus: http.StatusUnauthorized,
        },
        {
            name:           "tampered token",
            authorization:  "Bearer " + tamperedToken,
            expectedStatus: http.StatusUnauthorized,
        },
        {
            name:           "null byte injection",
            authorization:  "Bearer valid\x00token",
            expectedStatus: http.StatusUnauthorized,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("GET", "/v1/models", nil)
            if tt.authorization != "" {
                req.Header.Set("Authorization", tt.authorization)
            }

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)
        })
    }
}

func TestSQLInjection(t *testing.T) {
    db := setupTestDB(t)
    repo := NewTaskRepository(db)

    payloads := []string{
        "'; DROP TABLE tasks; --",
        "1' OR '1'='1",
        "1; SELECT * FROM users",
        "' UNION SELECT password FROM users --",
        "1' AND SLEEP(5) --",
    }

    for _, payload := range payloads {
        t.Run(payload, func(t *testing.T) {
            // Should not cause errors or data leakage
            tasks, err := repo.SafeSearch(context.Background(), payload)
            assert.NoError(t, err)
            // Should return empty results, not other data
            assert.Empty(t, tasks)
        })
    }
}
```

### Video 5.3: Incident Response for Security (25 min)

**Topics:**
- Security incident detection
- Response procedures
- Communication protocols
- Post-incident review

**Security Incident Runbook:**
```markdown
# Security Incident Response

## Severity Levels

| Severity | Description | Response Time |
|----------|-------------|---------------|
| Critical | Active data breach, RCE | Immediate |
| High | Auth bypass, SQLi discovered | 1 hour |
| Medium | XSS, information disclosure | 4 hours |
| Low | Minor security misconfiguration | 24 hours |

## Response Steps

### 1. Containment (Immediate)
- Isolate affected systems
- Revoke compromised credentials
- Enable enhanced logging

### 2. Investigation (1-4 hours)
- Determine scope of breach
- Identify attack vector
- Collect forensic evidence

### 3. Eradication (4-24 hours)
- Patch vulnerability
- Remove malicious artifacts
- Reset affected credentials

### 4. Recovery (24-72 hours)
- Restore from clean backups
- Verify system integrity
- Re-enable services gradually

### 5. Post-Incident (1 week)
- Complete incident report
- Implement additional controls
- Update security documentation
```

---

## Module 6: Compliance and Auditing

### Video 6.1: Audit Logging (25 min)

**Topics:**
- What to log
- Log integrity
- Log retention
- Analysis tools

**Audit Logger:**
```go
// internal/audit/logger.go
type AuditLogger struct {
    logger *slog.Logger
    store  AuditStore
}

type AuditEvent struct {
    ID         string                 `json:"id"`
    Timestamp  time.Time              `json:"timestamp"`
    Action     string                 `json:"action"`
    Actor      string                 `json:"actor"`
    ActorType  string                 `json:"actor_type"`
    Resource   string                 `json:"resource"`
    ResourceID string                 `json:"resource_id"`
    Result     string                 `json:"result"`
    Metadata   map[string]interface{} `json:"metadata"`
    IPAddress  string                 `json:"ip_address"`
    UserAgent  string                 `json:"user_agent"`
}

func (l *AuditLogger) Log(ctx context.Context, event *AuditEvent) error {
    event.ID = uuid.New().String()
    event.Timestamp = time.Now().UTC()

    // Add trace context
    if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
        event.Metadata["trace_id"] = span.SpanContext().TraceID().String()
    }

    // Store for compliance
    if err := l.store.Save(event); err != nil {
        l.logger.Error("failed to store audit event",
            "error", err,
            "event_id", event.ID)
    }

    // Log for real-time monitoring
    l.logger.Info("audit event",
        "event_id", event.ID,
        "action", event.Action,
        "actor", event.Actor,
        "resource", event.Resource,
        "result", event.Result)

    return nil
}

// Middleware for automatic audit logging
func (l *AuditLogger) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()

        // Log after request completes
        l.Log(c.Request.Context(), &AuditEvent{
            Action:     c.Request.Method + " " + c.FullPath(),
            Actor:      c.GetString("user_id"),
            ActorType:  "user",
            Resource:   c.FullPath(),
            ResourceID: c.Param("id"),
            Result:     fmt.Sprintf("%d", c.Writer.Status()),
            Metadata: map[string]interface{}{
                "duration_ms": time.Since(start).Milliseconds(),
                "request_id":  c.GetString("request_id"),
            },
            IPAddress: c.ClientIP(),
            UserAgent: c.Request.UserAgent(),
        })
    }
}
```

### Video 6.2: Compliance Frameworks (25 min)

**Topics:**
- SOC 2 requirements
- GDPR compliance
- Data handling policies
- Control mapping

**Compliance Controls:**
```yaml
# compliance/controls.yaml
controls:
  - id: AC-1
    name: Access Control Policy
    framework: SOC2
    implementation:
      - RBAC enforced on all endpoints
      - JWT with 1-hour expiration
      - API key rotation every 90 days

  - id: AU-2
    name: Audit Events
    framework: SOC2
    implementation:
      - All authentication events logged
      - All data access logged
      - Logs retained for 1 year

  - id: SC-8
    name: Transmission Confidentiality
    framework: SOC2
    implementation:
      - TLS 1.2+ required
      - Strong cipher suites only
      - Certificate pinning for LLM providers

  - id: GDPR-17
    name: Right to Erasure
    framework: GDPR
    implementation:
      - User data deletion API
      - Cascading deletion of related records
      - Audit log of deletion requests
```

---

## Hands-on Labs

### Lab 1: Secure Configuration
Harden HelixAgent security configuration.

### Lab 2: Authentication System
Implement JWT authentication with refresh tokens.

### Lab 3: Security Testing
Write security tests for common vulnerabilities.

### Lab 4: Audit Implementation
Set up comprehensive audit logging.

---

## Resources

- [OWASP Top 10](https://owasp.org/Top10/)
- [Go Security Cheatsheet](https://cheatsheetseries.owasp.org/cheatsheets/Go_Security_Cheatsheet.html)
- [CWE/SANS Top 25](https://cwe.mitre.org/top25/)
- [HelixAgent Security Guide](/docs/security/)

---

## Course Completion

Congratulations! You've completed the Security Best Practices course. You should now be able to:

- Design secure architectures
- Implement authentication and authorization
- Protect data at rest and in transit
- Validate and sanitize all input
- Manage vulnerabilities effectively
- Meet compliance requirements
