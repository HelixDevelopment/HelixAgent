// Package api provides OAuth API implementation for Claude Code integration.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthConfig holds OAuth configuration
 type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthURL      string
	TokenURL     string
	Scopes       []string
}

// Default OAuth configurations
var (
	// ConsoleOAuthConfig for API key creation
	ConsoleOAuthConfig = OAuthConfig{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
		AuthURL: "https://platform.claude.com/oauth/authorize",
		TokenURL: "https://platform.claude.com/v1/oauth/token",
		Scopes:   []string{"org:create_api_key", "user:profile"},
	}
	
	// ClaudeAIOAuthConfig for Claude.ai integration
	ClaudeAIOAuthConfig = OAuthConfig{ // #nosec G101 -- not a credential (map key / config label / env-var reference)
		AuthURL: "https://claude.com/cai/oauth/authorize",
		TokenURL: "https://platform.claude.com/v1/oauth/token",
		Scopes: []string{
			"user:profile",
			"user:inference",
			"user:sessions:claude_code",
			"user:mcp_servers",
			"user:file_upload",
		},
	}
)

// TokenResponse represents an OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// TokenInfo holds token information with expiration tracking
 type TokenInfo struct {
	TokenResponse
	AcquiredAt time.Time
}

// IsExpired checks if the token is expired (with 5 minute buffer)
func (t *TokenInfo) IsExpired() bool {
	if t.ExpiresIn == 0 {
		return false
	}
	expiration := t.AcquiredAt.Add(time.Duration(t.ExpiresIn-300) * time.Second)
	return time.Now().After(expiration)
}

// ExchangeAuthorizationCode exchanges an authorization code for tokens
func (c *Client) ExchangeAuthorizationCode(ctx context.Context, config OAuthConfig, code string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", config.ClientID)
	data.Set("redirect_uri", config.RedirectURI)
	
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}
	
	return c.doTokenRequest(ctx, config.TokenURL, data)
}

// RefreshToken refreshes an access token using a refresh token
func (c *Client) RefreshToken(ctx context.Context, config OAuthConfig, refreshToken string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", config.ClientID)
	
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}
	
	return c.doTokenRequest(ctx, config.TokenURL, data)
}

// doTokenRequest performs a token request
func (c *Client) doTokenRequest(ctx context.Context, tokenURL string, data url.Values) (*TokenInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}
	
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	
	return &TokenInfo{
		TokenResponse: tokenResp,
		AcquiredAt:    time.Now(),
	}, nil
}

// BuildAuthorizationURL builds the OAuth authorization URL
func BuildAuthorizationURL(config OAuthConfig, state string) string {
	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("redirect_uri", config.RedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(config.Scopes, " "))
	params.Set("state", state)
	
	return config.AuthURL + "?" + params.Encode()
}

// CreateAPIKey creates a new API key via OAuth
func (c *Client) CreateAPIKey(ctx context.Context) (string, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/oauth/claude_cli/create_api_key", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "", handleErrorResponse(resp)
	}
	
	var result struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	
	return result.APIKey, nil
}

// GetUserRoles gets the user's roles and permissions
func (c *Client) GetUserRoles(ctx context.Context) (*UserRoles, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/oauth/claude_cli/roles", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}
	
	var result UserRoles
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	
	return &result, nil
}

// UserRoles represents user roles and permissions
type UserRoles struct {
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// OAuthError represents an OAuth error
type OAuthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// DeviceCodeFlow represents device code flow for OAuth
type DeviceCodeFlow struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// StartDeviceCodeFlow starts a device code OAuth flow
func (c *Client) StartDeviceCodeFlow(ctx context.Context, config OAuthConfig) (*DeviceCodeFlow, error) {
	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("scope", strings.Join(config.Scopes, " "))
	
	deviceURL := "https://platform.claude.com/v1/oauth/device/code"
	req, err := http.NewRequestWithContext(ctx, "POST", deviceURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create device code request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute device code request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}
	
	var result DeviceCodeFlow
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}
	
	return &result, nil
}

// PollDeviceCode polls for device code authorization
func (c *Client) PollDeviceCode(ctx context.Context, config OAuthConfig, deviceCode string) (*TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("device_code", deviceCode)
	data.Set("client_id", config.ClientID)
	
	return c.doTokenRequest(ctx, config.TokenURL, data)
}
