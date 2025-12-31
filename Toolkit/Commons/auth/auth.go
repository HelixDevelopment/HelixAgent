// Package auth provides authentication management for AI providers.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AuthManager manages authentication for providers.
type AuthManager struct {
	apiKey     string
	token      string
	tokenMutex sync.RWMutex
	expiresAt  time.Time
	refresher  TokenRefresher
}

// TokenRefresher defines the interface for token refresh logic.
type TokenRefresher interface {
	RefreshToken(ctx context.Context) (*TokenResponse, error)
}

// TokenResponse represents a token refresh response.
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewAuthManager creates a new authentication manager.
func NewAuthManager(apiKey string, refresher TokenRefresher) *AuthManager {
	return &AuthManager{
		apiKey:    apiKey,
		refresher: refresher,
	}
}

// GetAuthHeader returns the appropriate authorization header value.
func (am *AuthManager) GetAuthHeader(ctx context.Context) (string, error) {
	if am.refresher != nil {
		return am.getTokenAuth(ctx)
	}
	return am.getAPIKeyAuth(), nil
}

// getAPIKeyAuth returns API key based authentication.
func (am *AuthManager) getAPIKeyAuth() string {
	return "Bearer " + am.apiKey
}

// getTokenAuth returns token-based authentication, refreshing if necessary.
func (am *AuthManager) getTokenAuth(ctx context.Context) (string, error) {
	am.tokenMutex.RLock()
	token := am.token
	expiresAt := am.expiresAt
	am.tokenMutex.RUnlock()

	// Check if token is still valid (with 5 minute buffer)
	if token != "" && time.Now().Add(5*time.Minute).Before(expiresAt) {
		return "Bearer " + token, nil
	}

	// Need to refresh token
	am.tokenMutex.Lock()
	defer am.tokenMutex.Unlock()

	// Double-check after acquiring write lock
	if am.token != "" && time.Now().Add(5*time.Minute).Before(am.expiresAt) {
		return "Bearer " + am.token, nil
	}

	// Refresh the token
	tokenResp, err := am.refresher.RefreshToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	am.token = tokenResp.Token
	am.expiresAt = tokenResp.ExpiresAt

	return "Bearer " + am.token, nil
}

// APIKeyAuth represents simple API key authentication.
type APIKeyAuth struct {
	APIKey string
}

// NewAPIKeyAuth creates a new API key authenticator.
func NewAPIKeyAuth(apiKey string) *APIKeyAuth {
	return &APIKeyAuth{APIKey: apiKey}
}

// GetAuthHeader returns the authorization header for API key auth.
func (a *APIKeyAuth) GetAuthHeader(ctx context.Context) (string, error) {
	return "Bearer " + a.APIKey, nil
}

// AuthInterceptor is an HTTP interceptor that adds authentication.
type AuthInterceptor struct {
	authManager AuthManagerInterface
}

// AuthManagerInterface defines the interface for auth managers.
type AuthManagerInterface interface {
	GetAuthHeader(ctx context.Context) (string, error)
}

// NewAuthInterceptor creates a new authentication interceptor.
func NewAuthInterceptor(authManager AuthManagerInterface) *AuthInterceptor {
	return &AuthInterceptor{authManager: authManager}
}

// Intercept adds authentication to the request.
func (ai *AuthInterceptor) Intercept(req *http.Request) error {
	authHeader, err := ai.authManager.GetAuthHeader(req.Context())
	if err != nil {
		return fmt.Errorf("failed to get auth header: %w", err)
	}

	req.Header.Set("Authorization", authHeader)
	return nil
}

// OAuth2Refresher implements OAuth2 token refresh.
type OAuth2Refresher struct {
	clientID     string
	clientSecret string
	tokenURL     string
	refreshToken string
	scopes       []string
	httpClient   *http.Client
}

// OAuth2RefresherOption is a functional option for OAuth2Refresher.
type OAuth2RefresherOption func(*OAuth2Refresher)

// WithRefreshToken sets the refresh token for token refresh operations.
func WithRefreshToken(token string) OAuth2RefresherOption {
	return func(r *OAuth2Refresher) {
		r.refreshToken = token
	}
}

// WithScopes sets the scopes for token requests.
func WithScopes(scopes []string) OAuth2RefresherOption {
	return func(r *OAuth2Refresher) {
		r.scopes = scopes
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) OAuth2RefresherOption {
	return func(r *OAuth2Refresher) {
		r.httpClient = client
	}
}

// NewOAuth2Refresher creates a new OAuth2 token refresher.
func NewOAuth2Refresher(clientID, clientSecret, tokenURL string, opts ...OAuth2RefresherOption) *OAuth2Refresher {
	r := &OAuth2Refresher{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     tokenURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// oauth2TokenResponse represents the OAuth2 token endpoint response.
type oauth2TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// RefreshToken refreshes an OAuth2 token by making an HTTP request to the token endpoint.
func (r *OAuth2Refresher) RefreshToken(ctx context.Context) (*TokenResponse, error) {
	// Build form data for token request
	data := url.Values{}

	if r.refreshToken != "" {
		// Use refresh_token grant type if we have a refresh token
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", r.refreshToken)
	} else {
		// Use client_credentials grant type for service-to-service auth
		data.Set("grant_type", "client_credentials")
	}

	data.Set("client_id", r.clientID)
	data.Set("client_secret", r.clientSecret)

	if len(r.scopes) > 0 {
		data.Set("scope", strings.Join(r.scopes, " "))
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	// Parse response
	var tokenResp oauth2TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check for OAuth2 error response
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("oauth2 error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// Store new refresh token if provided
	if tokenResp.RefreshToken != "" {
		r.refreshToken = tokenResp.RefreshToken
	}

	return &TokenResponse{
		Token:     tokenResp.AccessToken,
		ExpiresAt: expiresAt,
	}, nil
}

// Middleware provides authentication middleware for HTTP clients.
type Middleware struct {
	authManager AuthManagerInterface
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(authManager AuthManagerInterface) *Middleware {
	return &Middleware{authManager: authManager}
}

// authTransport wraps an http.RoundTripper to add authentication.
type authTransport struct {
	base        http.RoundTripper
	authManager AuthManagerInterface
}

// RoundTrip implements http.RoundTripper interface.
func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	reqClone := req.Clone(req.Context())

	// Add authentication header
	authHeader, err := t.authManager.GetAuthHeader(req.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to get auth header: %w", err)
	}

	reqClone.Header.Set("Authorization", authHeader)

	return t.base.RoundTrip(reqClone)
}

// WrapClient wraps an HTTP client with authentication.
func (m *Middleware) WrapClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}

	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Create a new client with the auth transport
	return &http.Client{
		Transport:     &authTransport{base: transport, authManager: m.authManager},
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       client.Timeout,
	}
}
