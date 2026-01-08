// Package oauth_credentials provides functionality to read OAuth2 credentials
// from Claude Code and Qwen Code CLI agents when users are logged in using OAuth2.
package oauth_credentials

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ClaudeOAuthCredentials represents the OAuth credentials stored by Claude Code CLI
type ClaudeOAuthCredentials struct {
	ClaudeAiOauth *ClaudeAiOauth `json:"claudeAiOauth"`
}

// ClaudeAiOauth contains the Claude AI OAuth token details
type ClaudeAiOauth struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"` // Unix timestamp in milliseconds
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType"`
	RateLimitTier    string   `json:"rateLimitTier"`
}

// QwenOAuthCredentials represents the OAuth credentials stored by Qwen Code CLI
type QwenOAuthCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiryDate   int64  `json:"expiry_date"` // Unix timestamp in milliseconds
	TokenType    string `json:"token_type"`
	ResourceURL  string `json:"resource_url,omitempty"`
}

// OAuthCredentialReader provides methods to read OAuth credentials from CLI agents
type OAuthCredentialReader struct {
	mu sync.RWMutex
	// Cache for credentials with expiration check
	claudeCredentials *ClaudeOAuthCredentials
	qwenCredentials   *QwenOAuthCredentials
	claudeLastRead    time.Time
	qwenLastRead      time.Time
	// Cache duration (credentials are re-read after this duration)
	cacheDuration time.Duration
}

// NewOAuthCredentialReader creates a new OAuth credential reader
func NewOAuthCredentialReader() *OAuthCredentialReader {
	return &OAuthCredentialReader{
		cacheDuration: 5 * time.Minute, // Cache credentials for 5 minutes
	}
}

// GetClaudeCredentialsPath returns the path to Claude Code OAuth credentials
func GetClaudeCredentialsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".claude", ".credentials.json")
}

// GetQwenCredentialsPath returns the path to Qwen Code OAuth credentials
func GetQwenCredentialsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".qwen", "oauth_creds.json")
}

// ReadClaudeCredentials reads and returns Claude OAuth credentials from the CLI config
func (r *OAuthCredentialReader) ReadClaudeCredentials() (*ClaudeOAuthCredentials, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache validity
	if r.claudeCredentials != nil && time.Since(r.claudeLastRead) < r.cacheDuration {
		// Verify token is not expired
		if r.claudeCredentials.ClaudeAiOauth != nil &&
			r.claudeCredentials.ClaudeAiOauth.ExpiresAt > time.Now().UnixMilli() {
			return r.claudeCredentials, nil
		}
	}

	// Read from file
	credPath := GetClaudeCredentialsPath()
	if credPath == "" {
		return nil, fmt.Errorf("unable to determine home directory")
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Claude Code credentials file not found at %s: user may not be logged in via OAuth", credPath)
		}
		return nil, fmt.Errorf("failed to read Claude credentials file: %w", err)
	}

	var creds ClaudeOAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse Claude credentials: %w", err)
	}

	if creds.ClaudeAiOauth == nil {
		return nil, fmt.Errorf("no OAuth credentials found in Claude Code config")
	}

	if creds.ClaudeAiOauth.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in Claude Code credentials")
	}

	// Check expiration
	if creds.ClaudeAiOauth.ExpiresAt > 0 && creds.ClaudeAiOauth.ExpiresAt <= time.Now().UnixMilli() {
		return nil, fmt.Errorf("Claude OAuth token has expired (expired at %s)", time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt).Format(time.RFC3339))
	}

	// Update cache
	r.claudeCredentials = &creds
	r.claudeLastRead = time.Now()

	return &creds, nil
}

// ReadQwenCredentials reads and returns Qwen OAuth credentials from the CLI config
func (r *OAuthCredentialReader) ReadQwenCredentials() (*QwenOAuthCredentials, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache validity
	if r.qwenCredentials != nil && time.Since(r.qwenLastRead) < r.cacheDuration {
		// Verify token is not expired
		if r.qwenCredentials.ExpiryDate > time.Now().UnixMilli() {
			return r.qwenCredentials, nil
		}
	}

	// Read from file
	credPath := GetQwenCredentialsPath()
	if credPath == "" {
		return nil, fmt.Errorf("unable to determine home directory")
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Qwen Code credentials file not found at %s: user may not be logged in via OAuth", credPath)
		}
		return nil, fmt.Errorf("failed to read Qwen credentials file: %w", err)
	}

	var creds QwenOAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse Qwen credentials: %w", err)
	}

	if creds.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in Qwen Code credentials")
	}

	// Check expiration
	if creds.ExpiryDate > 0 && creds.ExpiryDate <= time.Now().UnixMilli() {
		return nil, fmt.Errorf("Qwen OAuth token has expired (expired at %s)", time.UnixMilli(creds.ExpiryDate).Format(time.RFC3339))
	}

	// Update cache
	r.qwenCredentials = &creds
	r.qwenLastRead = time.Now()

	return &creds, nil
}

// GetClaudeAccessToken returns the Claude access token if available and valid
func (r *OAuthCredentialReader) GetClaudeAccessToken() (string, error) {
	creds, err := r.ReadClaudeCredentials()
	if err != nil {
		return "", err
	}
	return creds.ClaudeAiOauth.AccessToken, nil
}

// GetQwenAccessToken returns the Qwen access token if available and valid
func (r *OAuthCredentialReader) GetQwenAccessToken() (string, error) {
	creds, err := r.ReadQwenCredentials()
	if err != nil {
		return "", err
	}
	return creds.AccessToken, nil
}

// IsClaudeOAuthEnabled checks if Claude OAuth credentials should be used
func IsClaudeOAuthEnabled() bool {
	// Check environment variable (supports both spellings)
	val := os.Getenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS")
	if val == "" {
		val = os.Getenv("CLAUDE_CODE_USE_OUATH_CREDENTIALS") // Support typo in existing configs
	}
	return val == "true" || val == "1" || val == "yes"
}

// IsQwenOAuthEnabled checks if Qwen OAuth credentials should be used
func IsQwenOAuthEnabled() bool {
	// Check environment variable (supports both spellings)
	val := os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS")
	if val == "" {
		val = os.Getenv("QWEN_CODE_USE_OUATH_CREDENTIALS") // Support typo in existing configs
	}
	return val == "true" || val == "1" || val == "yes"
}

// HasValidClaudeCredentials checks if valid Claude OAuth credentials are available
func (r *OAuthCredentialReader) HasValidClaudeCredentials() bool {
	creds, err := r.ReadClaudeCredentials()
	if err != nil {
		return false
	}
	return creds.ClaudeAiOauth != nil &&
		creds.ClaudeAiOauth.AccessToken != "" &&
		(creds.ClaudeAiOauth.ExpiresAt == 0 || creds.ClaudeAiOauth.ExpiresAt > time.Now().UnixMilli())
}

// HasValidQwenCredentials checks if valid Qwen OAuth credentials are available
func (r *OAuthCredentialReader) HasValidQwenCredentials() bool {
	creds, err := r.ReadQwenCredentials()
	if err != nil {
		return false
	}
	return creds.AccessToken != "" &&
		(creds.ExpiryDate == 0 || creds.ExpiryDate > time.Now().UnixMilli())
}

// GetClaudeCredentialInfo returns information about the Claude OAuth credentials
func (r *OAuthCredentialReader) GetClaudeCredentialInfo() map[string]interface{} {
	creds, err := r.ReadClaudeCredentials()
	if err != nil {
		return map[string]interface{}{
			"available": false,
			"error":     err.Error(),
		}
	}

	expiresIn := time.Duration(0)
	if creds.ClaudeAiOauth.ExpiresAt > 0 {
		expiresIn = time.Until(time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt))
	}

	return map[string]interface{}{
		"available":         true,
		"subscription_type": creds.ClaudeAiOauth.SubscriptionType,
		"rate_limit_tier":   creds.ClaudeAiOauth.RateLimitTier,
		"scopes":            creds.ClaudeAiOauth.Scopes,
		"expires_in":        expiresIn.String(),
		"has_refresh_token": creds.ClaudeAiOauth.RefreshToken != "",
	}
}

// GetQwenCredentialInfo returns information about the Qwen OAuth credentials
func (r *OAuthCredentialReader) GetQwenCredentialInfo() map[string]interface{} {
	creds, err := r.ReadQwenCredentials()
	if err != nil {
		return map[string]interface{}{
			"available": false,
			"error":     err.Error(),
		}
	}

	expiresIn := time.Duration(0)
	if creds.ExpiryDate > 0 {
		expiresIn = time.Until(time.UnixMilli(creds.ExpiryDate))
	}

	return map[string]interface{}{
		"available":         true,
		"token_type":        creds.TokenType,
		"resource_url":      creds.ResourceURL,
		"expires_in":        expiresIn.String(),
		"has_refresh_token": creds.RefreshToken != "",
		"has_id_token":      creds.IDToken != "",
	}
}

// ClearCache clears the credential cache
func (r *OAuthCredentialReader) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claudeCredentials = nil
	r.qwenCredentials = nil
	r.claudeLastRead = time.Time{}
	r.qwenLastRead = time.Time{}
}

// Global singleton instance
var (
	globalReader     *OAuthCredentialReader
	globalReaderOnce sync.Once
)

// GetGlobalReader returns the global OAuth credential reader instance
func GetGlobalReader() *OAuthCredentialReader {
	globalReaderOnce.Do(func() {
		globalReader = NewOAuthCredentialReader()
	})
	return globalReader
}
