// Package oauth_credentials provides tests for OAuth2 credential reading functionality
package oauth_credentials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewOAuthCredentialReader(t *testing.T) {
	reader := NewOAuthCredentialReader()
	if reader == nil {
		t.Fatal("NewOAuthCredentialReader returned nil")
	}
	if reader.cacheDuration != 5*time.Minute {
		t.Errorf("Expected cache duration of 5 minutes, got %v", reader.cacheDuration)
	}
}

func TestGetClaudeCredentialsPath(t *testing.T) {
	path := GetClaudeCredentialsPath()
	if path == "" {
		t.Skip("Unable to determine home directory")
	}

	homeDir, _ := os.UserHomeDir()
	expected := filepath.Join(homeDir, ".claude", ".credentials.json")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}
}

func TestGetQwenCredentialsPath(t *testing.T) {
	path := GetQwenCredentialsPath()
	if path == "" {
		t.Skip("Unable to determine home directory")
	}

	homeDir, _ := os.UserHomeDir()
	expected := filepath.Join(homeDir, ".qwen", "oauth_creds.json")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}
}

func TestIsClaudeOAuthEnabled(t *testing.T) {
	// Save original env value
	original := os.Getenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS")
	originalTypo := os.Getenv("CLAUDE_CODE_USE_OUATH_CREDENTIALS")
	defer func() {
		os.Setenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS", original)
		os.Setenv("CLAUDE_CODE_USE_OUATH_CREDENTIALS", originalTypo)
	}()

	tests := []struct {
		name      string
		envVar    string
		envValue  string
		expected  bool
	}{
		{"true value", "CLAUDE_CODE_USE_OAUTH_CREDENTIALS", "true", true},
		{"1 value", "CLAUDE_CODE_USE_OAUTH_CREDENTIALS", "1", true},
		{"yes value", "CLAUDE_CODE_USE_OAUTH_CREDENTIALS", "yes", true},
		{"false value", "CLAUDE_CODE_USE_OAUTH_CREDENTIALS", "false", false},
		{"empty value", "CLAUDE_CODE_USE_OAUTH_CREDENTIALS", "", false},
		{"typo version true", "CLAUDE_CODE_USE_OUATH_CREDENTIALS", "true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("CLAUDE_CODE_USE_OAUTH_CREDENTIALS")
			os.Unsetenv("CLAUDE_CODE_USE_OUATH_CREDENTIALS")
			if tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
			}
			result := IsClaudeOAuthEnabled()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for %s=%s", tt.expected, result, tt.envVar, tt.envValue)
			}
		})
	}
}

func TestIsQwenOAuthEnabled(t *testing.T) {
	// Save original env value
	original := os.Getenv("QWEN_CODE_USE_OAUTH_CREDENTIALS")
	originalTypo := os.Getenv("QWEN_CODE_USE_OUATH_CREDENTIALS")
	defer func() {
		os.Setenv("QWEN_CODE_USE_OAUTH_CREDENTIALS", original)
		os.Setenv("QWEN_CODE_USE_OUATH_CREDENTIALS", originalTypo)
	}()

	tests := []struct {
		name      string
		envVar    string
		envValue  string
		expected  bool
	}{
		{"true value", "QWEN_CODE_USE_OAUTH_CREDENTIALS", "true", true},
		{"1 value", "QWEN_CODE_USE_OAUTH_CREDENTIALS", "1", true},
		{"yes value", "QWEN_CODE_USE_OAUTH_CREDENTIALS", "yes", true},
		{"false value", "QWEN_CODE_USE_OAUTH_CREDENTIALS", "false", false},
		{"empty value", "QWEN_CODE_USE_OAUTH_CREDENTIALS", "", false},
		{"typo version true", "QWEN_CODE_USE_OUATH_CREDENTIALS", "true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("QWEN_CODE_USE_OAUTH_CREDENTIALS")
			os.Unsetenv("QWEN_CODE_USE_OUATH_CREDENTIALS")
			if tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
			}
			result := IsQwenOAuthEnabled()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for %s=%s", tt.expected, result, tt.envVar, tt.envValue)
			}
		})
	}
}

func TestReadClaudeCredentials_FileNotFound(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir and set HOME to it temporarily
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Clear the cache to force re-read
	reader.ClearCache()

	_, err := reader.ReadClaudeCredentials()
	if err == nil {
		t.Error("Expected error for missing credentials file")
	}
}

func TestReadClaudeCredentials_ValidFile(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .claude directory
	claudeDir := filepath.Join(tempDir, ".claude")
	err := os.MkdirAll(claudeDir, 0700)
	if err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create mock credentials
	mockCreds := ClaudeOAuthCredentials{
		ClaudeAiOauth: &ClaudeAiOauth{
			AccessToken:      "test-access-token",
			RefreshToken:     "test-refresh-token",
			ExpiresAt:        time.Now().Add(1 * time.Hour).UnixMilli(),
			Scopes:           []string{"user:inference"},
			SubscriptionType: "max",
			RateLimitTier:    "default",
		},
	}

	data, _ := json.Marshal(mockCreds)
	credPath := filepath.Join(claudeDir, ".credentials.json")
	err = os.WriteFile(credPath, data, 0600)
	if err != nil {
		t.Fatalf("Failed to write mock credentials: %v", err)
	}

	// Clear cache
	reader.ClearCache()

	creds, err := reader.ReadClaudeCredentials()
	if err != nil {
		t.Fatalf("Failed to read credentials: %v", err)
	}

	if creds.ClaudeAiOauth.AccessToken != "test-access-token" {
		t.Errorf("Expected access token 'test-access-token', got '%s'", creds.ClaudeAiOauth.AccessToken)
	}
}

func TestReadClaudeCredentials_ExpiredToken(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .claude directory
	claudeDir := filepath.Join(tempDir, ".claude")
	err := os.MkdirAll(claudeDir, 0700)
	if err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create mock credentials with expired token
	mockCreds := ClaudeOAuthCredentials{
		ClaudeAiOauth: &ClaudeAiOauth{
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			ExpiresAt:    time.Now().Add(-1 * time.Hour).UnixMilli(), // Expired
			Scopes:       []string{"user:inference"},
		},
	}

	data, _ := json.Marshal(mockCreds)
	credPath := filepath.Join(claudeDir, ".credentials.json")
	err = os.WriteFile(credPath, data, 0600)
	if err != nil {
		t.Fatalf("Failed to write mock credentials: %v", err)
	}

	// Clear cache
	reader.ClearCache()

	_, err = reader.ReadClaudeCredentials()
	if err == nil {
		t.Error("Expected error for expired token")
	}
}

func TestReadQwenCredentials_FileNotFound(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir and set HOME to it temporarily
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Clear the cache to force re-read
	reader.ClearCache()

	_, err := reader.ReadQwenCredentials()
	if err == nil {
		t.Error("Expected error for missing credentials file")
	}
}

func TestReadQwenCredentials_ValidFile(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .qwen directory
	qwenDir := filepath.Join(tempDir, ".qwen")
	err := os.MkdirAll(qwenDir, 0700)
	if err != nil {
		t.Fatalf("Failed to create .qwen directory: %v", err)
	}

	// Create mock credentials
	mockCreds := QwenOAuthCredentials{
		AccessToken:  "test-qwen-access-token",
		RefreshToken: "test-qwen-refresh-token",
		ExpiryDate:   time.Now().Add(1 * time.Hour).UnixMilli(),
		TokenType:    "Bearer",
	}

	data, _ := json.Marshal(mockCreds)
	credPath := filepath.Join(qwenDir, "oauth_creds.json")
	err = os.WriteFile(credPath, data, 0600)
	if err != nil {
		t.Fatalf("Failed to write mock credentials: %v", err)
	}

	// Clear cache
	reader.ClearCache()

	creds, err := reader.ReadQwenCredentials()
	if err != nil {
		t.Fatalf("Failed to read credentials: %v", err)
	}

	if creds.AccessToken != "test-qwen-access-token" {
		t.Errorf("Expected access token 'test-qwen-access-token', got '%s'", creds.AccessToken)
	}
}

func TestReadQwenCredentials_ExpiredToken(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .qwen directory
	qwenDir := filepath.Join(tempDir, ".qwen")
	err := os.MkdirAll(qwenDir, 0700)
	if err != nil {
		t.Fatalf("Failed to create .qwen directory: %v", err)
	}

	// Create mock credentials with expired token
	mockCreds := QwenOAuthCredentials{
		AccessToken:  "test-qwen-access-token",
		RefreshToken: "test-qwen-refresh-token",
		ExpiryDate:   time.Now().Add(-1 * time.Hour).UnixMilli(), // Expired
		TokenType:    "Bearer",
	}

	data, _ := json.Marshal(mockCreds)
	credPath := filepath.Join(qwenDir, "oauth_creds.json")
	err = os.WriteFile(credPath, data, 0600)
	if err != nil {
		t.Fatalf("Failed to write mock credentials: %v", err)
	}

	// Clear cache
	reader.ClearCache()

	_, err = reader.ReadQwenCredentials()
	if err == nil {
		t.Error("Expected error for expired token")
	}
}

func TestGetClaudeAccessToken(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .claude directory
	claudeDir := filepath.Join(tempDir, ".claude")
	os.MkdirAll(claudeDir, 0700)

	// Create mock credentials
	mockCreds := ClaudeOAuthCredentials{
		ClaudeAiOauth: &ClaudeAiOauth{
			AccessToken: "test-token-12345",
			ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
		},
	}

	data, _ := json.Marshal(mockCreds)
	os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), data, 0600)

	reader.ClearCache()

	token, err := reader.GetClaudeAccessToken()
	if err != nil {
		t.Fatalf("Failed to get access token: %v", err)
	}

	if token != "test-token-12345" {
		t.Errorf("Expected 'test-token-12345', got '%s'", token)
	}
}

func TestGetQwenAccessToken(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .qwen directory
	qwenDir := filepath.Join(tempDir, ".qwen")
	os.MkdirAll(qwenDir, 0700)

	// Create mock credentials
	mockCreds := QwenOAuthCredentials{
		AccessToken: "test-qwen-token-67890",
		ExpiryDate:  time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	data, _ := json.Marshal(mockCreds)
	os.WriteFile(filepath.Join(qwenDir, "oauth_creds.json"), data, 0600)

	reader.ClearCache()

	token, err := reader.GetQwenAccessToken()
	if err != nil {
		t.Fatalf("Failed to get access token: %v", err)
	}

	if token != "test-qwen-token-67890" {
		t.Errorf("Expected 'test-qwen-token-67890', got '%s'", token)
	}
}

func TestHasValidClaudeCredentials(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Test with no credentials file
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	reader.ClearCache()

	if reader.HasValidClaudeCredentials() {
		t.Error("Expected false for missing credentials")
	}

	// Create valid credentials
	claudeDir := filepath.Join(tempDir, ".claude")
	os.MkdirAll(claudeDir, 0700)

	mockCreds := ClaudeOAuthCredentials{
		ClaudeAiOauth: &ClaudeAiOauth{
			AccessToken: "valid-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
		},
	}

	data, _ := json.Marshal(mockCreds)
	os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), data, 0600)

	reader.ClearCache()

	if !reader.HasValidClaudeCredentials() {
		t.Error("Expected true for valid credentials")
	}
}

func TestHasValidQwenCredentials(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Test with no credentials file
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	reader.ClearCache()

	if reader.HasValidQwenCredentials() {
		t.Error("Expected false for missing credentials")
	}

	// Create valid credentials
	qwenDir := filepath.Join(tempDir, ".qwen")
	os.MkdirAll(qwenDir, 0700)

	mockCreds := QwenOAuthCredentials{
		AccessToken: "valid-qwen-token",
		ExpiryDate:  time.Now().Add(1 * time.Hour).UnixMilli(),
	}

	data, _ := json.Marshal(mockCreds)
	os.WriteFile(filepath.Join(qwenDir, "oauth_creds.json"), data, 0600)

	reader.ClearCache()

	if !reader.HasValidQwenCredentials() {
		t.Error("Expected true for valid credentials")
	}
}

func TestGetClaudeCredentialInfo(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .claude directory
	claudeDir := filepath.Join(tempDir, ".claude")
	os.MkdirAll(claudeDir, 0700)

	// Create mock credentials
	mockCreds := ClaudeOAuthCredentials{
		ClaudeAiOauth: &ClaudeAiOauth{
			AccessToken:      "test-token",
			RefreshToken:     "test-refresh",
			ExpiresAt:        time.Now().Add(1 * time.Hour).UnixMilli(),
			Scopes:           []string{"user:inference", "user:profile"},
			SubscriptionType: "max",
			RateLimitTier:    "premium",
		},
	}

	data, _ := json.Marshal(mockCreds)
	os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), data, 0600)

	reader.ClearCache()

	info := reader.GetClaudeCredentialInfo()
	if available, ok := info["available"].(bool); !ok || !available {
		t.Error("Expected available to be true")
	}
	if subType, ok := info["subscription_type"].(string); !ok || subType != "max" {
		t.Error("Expected subscription_type to be 'max'")
	}
	if hasRefresh, ok := info["has_refresh_token"].(bool); !ok || !hasRefresh {
		t.Error("Expected has_refresh_token to be true")
	}
}

func TestGetQwenCredentialInfo(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .qwen directory
	qwenDir := filepath.Join(tempDir, ".qwen")
	os.MkdirAll(qwenDir, 0700)

	// Create mock credentials
	mockCreds := QwenOAuthCredentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiryDate:   time.Now().Add(1 * time.Hour).UnixMilli(),
		TokenType:    "Bearer",
		ResourceURL:  "https://chat.qwen.ai",
	}

	data, _ := json.Marshal(mockCreds)
	os.WriteFile(filepath.Join(qwenDir, "oauth_creds.json"), data, 0600)

	reader.ClearCache()

	info := reader.GetQwenCredentialInfo()
	if available, ok := info["available"].(bool); !ok || !available {
		t.Error("Expected available to be true")
	}
	if tokenType, ok := info["token_type"].(string); !ok || tokenType != "Bearer" {
		t.Error("Expected token_type to be 'Bearer'")
	}
	if hasRefresh, ok := info["has_refresh_token"].(bool); !ok || !hasRefresh {
		t.Error("Expected has_refresh_token to be true")
	}
}

func TestCredentialCaching(t *testing.T) {
	reader := NewOAuthCredentialReader()
	reader.cacheDuration = 1 * time.Second // Short cache for testing

	// Create a temp dir with mock credentials
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .claude directory
	claudeDir := filepath.Join(tempDir, ".claude")
	os.MkdirAll(claudeDir, 0700)

	// Create mock credentials
	mockCreds := ClaudeOAuthCredentials{
		ClaudeAiOauth: &ClaudeAiOauth{
			AccessToken: "original-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
		},
	}

	data, _ := json.Marshal(mockCreds)
	credPath := filepath.Join(claudeDir, ".credentials.json")
	os.WriteFile(credPath, data, 0600)

	reader.ClearCache()

	// First read
	creds1, err := reader.ReadClaudeCredentials()
	if err != nil {
		t.Fatalf("First read failed: %v", err)
	}

	// Modify the file
	mockCreds.ClaudeAiOauth.AccessToken = "modified-token"
	data, _ = json.Marshal(mockCreds)
	os.WriteFile(credPath, data, 0600)

	// Second read should return cached value
	creds2, _ := reader.ReadClaudeCredentials()
	if creds2.ClaudeAiOauth.AccessToken != creds1.ClaudeAiOauth.AccessToken {
		t.Error("Expected cached value to be returned")
	}

	// Wait for cache to expire
	time.Sleep(1500 * time.Millisecond)

	// Third read should return new value
	creds3, _ := reader.ReadClaudeCredentials()
	if creds3.ClaudeAiOauth.AccessToken != "modified-token" {
		t.Errorf("Expected 'modified-token', got '%s'", creds3.ClaudeAiOauth.AccessToken)
	}
}

func TestClearCache(t *testing.T) {
	reader := NewOAuthCredentialReader()

	// Set up some cached data
	reader.claudeCredentials = &ClaudeOAuthCredentials{}
	reader.qwenCredentials = &QwenOAuthCredentials{}
	reader.claudeLastRead = time.Now()
	reader.qwenLastRead = time.Now()

	reader.ClearCache()

	if reader.claudeCredentials != nil {
		t.Error("Expected claudeCredentials to be nil after ClearCache")
	}
	if reader.qwenCredentials != nil {
		t.Error("Expected qwenCredentials to be nil after ClearCache")
	}
	if !reader.claudeLastRead.IsZero() {
		t.Error("Expected claudeLastRead to be zero after ClearCache")
	}
	if !reader.qwenLastRead.IsZero() {
		t.Error("Expected qwenLastRead to be zero after ClearCache")
	}
}

func TestGetGlobalReader(t *testing.T) {
	reader1 := GetGlobalReader()
	reader2 := GetGlobalReader()

	if reader1 != reader2 {
		t.Error("Expected singleton instance")
	}

	if reader1 == nil {
		t.Error("Expected non-nil global reader")
	}
}
