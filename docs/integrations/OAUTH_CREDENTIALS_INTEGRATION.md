# OAuth2 Credentials Integration for CLI Agents

This document describes the implementation of OAuth2 credential support for Claude Code and Qwen Code CLI agents in HelixAgent and LLMsVerifier.

## Overview

HelixAgent can now leverage OAuth2 credentials from Claude Code and Qwen Code CLI agents when users are logged in via OAuth2 authentication. This enables seamless integration without requiring separate API keys.

## Environment Variables

Two environment variables control OAuth credential usage:

| Variable | Description |
|----------|-------------|
| `CLAUDE_CODE_USE_OUATH_CREDENTIALS=true` | Enable Claude Code OAuth credentials |
| `QWEN_CODE_USE_OUATH_CREDENTIALS=true` | Enable Qwen Code OAuth credentials |

**Note**: Both `OUATH` (original typo) and `OAUTH` spellings are supported for compatibility.

Add to your `.env` file:

```bash
CLAUDE_CODE_USE_OUATH_CREDENTIALS=true
QWEN_CODE_USE_OUATH_CREDENTIALS=true
```

## Credential Storage Locations

### Claude Code CLI

**Location**: `~/.claude/.credentials.json`

**Structure**:
```json
{
  "claudeAiOauth": {
    "accessToken": "sk-ant-...",
    "refreshToken": "...",
    "expiresAt": 1736380800000,
    "scopes": ["user:inference"],
    "subscriptionType": "max",
    "rateLimitTier": "tier-4"
  }
}
```

**Fields**:
- `accessToken`: Bearer token for API authentication
- `refreshToken`: Token for refreshing expired access tokens
- `expiresAt`: Unix timestamp in milliseconds when token expires
- `scopes`: OAuth scopes granted
- `subscriptionType`: User subscription level (e.g., "max", "pro", "free")
- `rateLimitTier`: Rate limit tier assigned by Claude

### Qwen Code CLI

**Location**: `~/.qwen/oauth_creds.json`

**Structure**:
```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expiry_date": 1736380800000,
  "token_type": "Bearer",
  "resource_url": "https://qwen-api.com"
}
```

**Fields**:
- `access_token`: Bearer token for API authentication
- `refresh_token`: Token for refreshing expired access tokens
- `expiry_date`: Unix timestamp in milliseconds when token expires
- `token_type`: Token type (typically "Bearer")
- `resource_url`: API resource URL

## Architecture

### Credential Reader Package

Located at: `internal/auth/oauth_credentials/oauth_credentials.go`

**Key Components**:

1. **CredentialReader**: Main struct that reads and caches credentials
2. **ClaudeOAuthCredentials**: Struct representing Claude credential file
3. **QwenOAuthCredentials**: Struct representing Qwen credential file
4. **Global Reader**: Singleton instance for application-wide access

**Key Functions**:

| Function | Description |
|----------|-------------|
| `IsClaudeOAuthEnabled()` | Check if Claude OAuth is enabled via env var |
| `IsQwenOAuthEnabled()` | Check if Qwen OAuth is enabled via env var |
| `GetClaudeAccessToken()` | Get valid Claude access token if available |
| `GetQwenAccessToken()` | Get valid Qwen access token if available |
| `HasValidClaudeCredentials()` | Check if valid (non-expired) Claude credentials exist |
| `HasValidQwenCredentials()` | Check if valid (non-expired) Qwen credentials exist |
| `GetClaudeCredentialInfo()` | Get detailed Claude credential information |
| `GetQwenCredentialInfo()` | Get detailed Qwen credential information |

### Provider Updates

Both Claude and Qwen providers support two authentication types:

```go
const (
    AuthTypeAPIKey AuthType = iota  // Traditional API key auth
    AuthTypeOAuth                    // OAuth2 bearer token auth
)
```

**New Provider Constructors**:

| Constructor | Description |
|-------------|-------------|
| `NewClaudeProviderWithOAuth(baseURL, model)` | Create Claude provider using OAuth |
| `NewClaudeProviderAuto(apiKey, baseURL, model)` | Auto-select auth (OAuth preferred) |
| `NewQwenProviderWithOAuth(baseURL, model)` | Create Qwen provider using OAuth |
| `NewQwenProviderAuto(apiKey, baseURL, model)` | Auto-select auth (OAuth preferred) |

### Authentication Flow

1. Provider Registry checks if OAuth is enabled via environment variable
2. If enabled, checks for valid credentials in CLI credential file
3. If valid credentials found, creates provider with OAuth auth type
4. If no valid OAuth credentials, falls back to API key auth
5. Provider uses appropriate auth header based on auth type:
   - OAuth: `Authorization: Bearer <token>`
   - API Key: `x-api-key: <key>` (Claude) or `Authorization: Bearer <key>` (Qwen)

## Provider Registry Integration

The provider registry in `internal/services/provider_registry.go` automatically:

1. Attempts OAuth authentication first when enabled
2. Falls back to API key authentication if OAuth unavailable
3. Logs which authentication method is being used

Example log output:
```
Using OAuth credentials for Claude provider
Using OAuth credentials for Qwen provider
```

## LLMsVerifier Integration

LLMsVerifier includes the same OAuth credential support in:
- `LLMsVerifier/llm-verifier/auth/oauth_credentials.go`
- `LLMsVerifier/llm-verifier/providers/anthropic.go`
- `LLMsVerifier/llm-verifier/providers/qwen.go`

## Test Results

### Unit Tests (20 tests)

All unit tests pass in the `oauth_credentials` package:

| Test | Status |
|------|--------|
| `TestIsClaudeOAuthEnabled` | PASS |
| `TestIsQwenOAuthEnabled` | PASS |
| `TestIsClaudeOAuthEnabled_WithCorrectSpelling` | PASS |
| `TestIsQwenOAuthEnabled_WithCorrectSpelling` | PASS |
| `TestReadClaudeCredentials_FileNotFound` | PASS |
| `TestReadClaudeCredentials_InvalidJSON` | PASS |
| `TestReadClaudeCredentials_ValidFile` | PASS |
| `TestReadClaudeCredentials_ExpiredToken` | PASS |
| `TestReadQwenCredentials_FileNotFound` | PASS |
| `TestReadQwenCredentials_ValidFile` | PASS |
| `TestReadQwenCredentials_ExpiredToken` | PASS |
| `TestGetClaudeAccessToken` | PASS |
| `TestGetQwenAccessToken` | PASS |
| `TestHasValidClaudeCredentials` | PASS |
| `TestHasValidQwenCredentials` | PASS |
| `TestGetClaudeCredentialInfo` | PASS |
| `TestGetQwenCredentialInfo` | PASS |
| `TestCredentialCaching` | PASS |
| `TestClearCache` | PASS |
| `TestGetGlobalReader` | PASS |

### Integration Tests

| Test | Status |
|------|--------|
| `TestOAuthClaudeProviderIntegration` | PASS |
| `TestOAuthQwenProviderIntegration` | PASS |
| `TestOAuthAutoProviderSelection` | PASS |
| `TestOAuthCredentialInfo` | PASS |
| `TestOAuthLiveAPICallClaude` | PASS |
| `TestOAuthLiveAPICallQwen` | PASS |
| `TestEnvironmentVariableToggle` | PASS |

### OAuth Credentials Challenge

```
======================================
  OAuth Credentials Challenge
======================================

1. Checking OAuth Environment Variables
----------------------------------------
[PASS] Claude OAuth enabled
       CLAUDE_CODE_USE_O[U]ATH_CREDENTIALS=true
[PASS] Qwen OAuth enabled
       QWEN_CODE_USE_O[U]ATH_CREDENTIALS=true

2. Checking OAuth Credential Files
----------------------------------------
[PASS] Claude OAuth credentials
       Valid token (expires in 5h), subscription: max
[PASS] Qwen OAuth credentials
       Valid token (expires in 5h)

3. Running Go Unit Tests
----------------------------------------
[PASS] OAuth credential unit tests

4. Running Integration Tests
----------------------------------------
[PASS] OAuth integration tests

5. Testing Provider Creation with OAuth
----------------------------------------
[PASS] OAuth provider creation

======================================
  Challenge Summary
======================================

Total Tests:   7
Passed:        7
Skipped:       0
Failed:        0

OAuth Credentials Challenge: PASSED
```

## Provider Score Values

When accessed via OAuth2 credentials:

| Provider | Model | Auth Type | Subscription | Rate Limit Tier |
|----------|-------|-----------|--------------|-----------------|
| Claude | claude-3.5-sonnet | OAuth | max | tier-4 |
| Claude | claude-3-haiku | OAuth | max | tier-4 |
| Qwen | qwen-turbo | OAuth | - | - |

### Dynamic Scoring (LLMsVerifier Integration)

OAuth-authenticated providers receive dynamic scores based on:
- Response speed (25%)
- Model efficiency (20%)
- Cost effectiveness (25%)
- Capability (20%)
- Recency (10%)

Typical score ranges:
- **Claude with OAuth (max subscription)**: ~9.5/10
- **Qwen with OAuth**: ~7.5/10

## Usage Examples

### Basic Usage

```go
import "dev.helix.agent/internal/auth/oauth_credentials"

// Check if OAuth is enabled and credentials are available
if oauth_credentials.IsClaudeOAuthEnabled() {
    reader := oauth_credentials.GetGlobalReader()
    if reader.HasValidClaudeCredentials() {
        // Use OAuth credentials
        token, err := reader.GetClaudeAccessToken()
        if err == nil {
            // Use token for API calls
        }
    }
}
```

### Using Auto-Selection

```go
import "dev.helix.agent/internal/llm/providers/claude"

// Auto-select between OAuth and API key
provider, err := claude.NewClaudeProviderAuto(apiKey, baseURL, model)
if err != nil {
    log.Fatal(err)
}

// Check which auth type was selected
authType := provider.GetAuthType()
if authType == claude.AuthTypeOAuth {
    log.Println("Using OAuth authentication")
} else {
    log.Println("Using API key authentication")
}
```

### Getting Credential Info

```go
reader := oauth_credentials.GetGlobalReader()
info := reader.GetClaudeCredentialInfo()

fmt.Printf("Available: %v\n", info["available"])
if available, ok := info["available"].(bool); ok && available {
    fmt.Printf("Subscription: %s\n", info["subscription_type"])
    fmt.Printf("Rate Limit Tier: %s\n", info["rate_limit_tier"])
    fmt.Printf("Expires In: %s\n", info["expires_in"])
}
```

## Files Modified/Created

### HelixAgent

| File | Action | Description |
|------|--------|-------------|
| `internal/auth/oauth_credentials/oauth_credentials.go` | Created | OAuth credential reader |
| `internal/auth/oauth_credentials/oauth_credentials_test.go` | Created | Unit tests |
| `internal/llm/providers/claude/claude.go` | Modified | OAuth support |
| `internal/llm/providers/qwen/qwen.go` | Modified | OAuth support |
| `internal/services/provider_registry.go` | Modified | Auto OAuth selection |
| `tests/integration/oauth_integration_test.go` | Created | Integration tests |
| `challenges/scripts/oauth_credentials_challenge.sh` | Created | Challenge script |

### LLMsVerifier

| File | Action | Description |
|------|--------|-------------|
| `LLMsVerifier/llm-verifier/auth/oauth_credentials.go` | Created | OAuth credential reader |
| `LLMsVerifier/llm-verifier/providers/anthropic.go` | Modified | OAuth support |
| `LLMsVerifier/llm-verifier/providers/qwen.go` | Created | Qwen provider with OAuth |
| `LLMsVerifier/llm-verifier/providers/config.go` | Modified | Qwen config |

## Troubleshooting

### Credentials Not Found

1. Ensure you're logged in via Claude Code or Qwen Code CLI
2. Check credential file exists at expected location
3. Verify environment variable is set to `true`

### Token Expired

OAuth tokens typically expire after several hours. Re-authenticate via:
```bash
# Claude Code
claude --login

# Qwen Code
qwen --login
```

### Environment Variable Not Working

Both spellings are supported:
```bash
# Either works:
CLAUDE_CODE_USE_OUATH_CREDENTIALS=true
CLAUDE_CODE_USE_OAUTH_CREDENTIALS=true
```

### Health Check Failures

Some providers may return 401 on health check endpoints even with valid OAuth credentials. This doesn't affect normal API operations.

## Security Considerations

1. **Credential Files**: OAuth credential files are stored with user-only permissions
2. **Token Caching**: Tokens are cached in memory for 5 minutes to reduce file reads
3. **Expiration Checks**: Tokens are validated for expiration before use
4. **No Token Storage**: HelixAgent never stores or logs access tokens

## Conclusion

OAuth2 credential integration provides seamless authentication for users already logged into Claude Code or Qwen Code CLI agents. The implementation is:

- **Backward Compatible**: Falls back to API keys when OAuth unavailable
- **Secure**: Respects token expiration and doesn't store credentials
- **Transparent**: Logs which authentication method is active
- **Tested**: Comprehensive unit and integration test coverage
