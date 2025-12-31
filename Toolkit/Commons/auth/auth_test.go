package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockTokenRefresher is a mock implementation of TokenRefresher for testing
type MockTokenRefresher struct {
	token     string
	expiresAt time.Time
}

func (m *MockTokenRefresher) RefreshToken(ctx context.Context) (*TokenResponse, error) {
	return &TokenResponse{
		Token:     m.token,
		ExpiresAt: m.expiresAt,
	}, nil
}

func TestNewAuthManager(t *testing.T) {
	apiKey := "test-api-key"
	refresher := &MockTokenRefresher{
		token:     "test-token",
		expiresAt: time.Now().Add(time.Hour),
	}

	manager := NewAuthManager(apiKey, refresher)

	if manager.apiKey != apiKey {
		t.Errorf("Expected apiKey to be %s, got %s", apiKey, manager.apiKey)
	}

	if manager.refresher != refresher {
		t.Error("Expected refresher to be set")
	}
}

func TestAuthManager_GetAuthHeader_APIKey(t *testing.T) {
	apiKey := "test-api-key"
	manager := NewAuthManager(apiKey, nil)

	ctx := context.Background()
	header, err := manager.GetAuthHeader(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Bearer " + apiKey
	if header != expected {
		t.Errorf("Expected header %s, got %s", expected, header)
	}
}

func TestAuthManager_GetAuthHeader_Token(t *testing.T) {
	apiKey := "test-api-key"
	refresher := &MockTokenRefresher{
		token:     "fresh-token",
		expiresAt: time.Now().Add(time.Hour),
	}

	manager := NewAuthManager(apiKey, refresher)

	ctx := context.Background()
	header, err := manager.GetAuthHeader(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Bearer fresh-token"
	if header != expected {
		t.Errorf("Expected header %s, got %s", expected, header)
	}
}

func TestAPIKeyAuth_GetAuthHeader(t *testing.T) {
	apiKey := "test-api-key"
	auth := NewAPIKeyAuth(apiKey)

	ctx := context.Background()
	header, err := auth.GetAuthHeader(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Bearer " + apiKey
	if header != expected {
		t.Errorf("Expected header %s, got %s", expected, header)
	}
}

func TestAuthInterceptor_Intercept(t *testing.T) {
	apiKey := "test-api-key"
	auth := NewAPIKeyAuth(apiKey)
	interceptor := NewAuthInterceptor(auth)

	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	err = interceptor.Intercept(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "Bearer " + apiKey
	if req.Header.Get("Authorization") != expected {
		t.Errorf("Expected Authorization header %s, got %s", expected, req.Header.Get("Authorization"))
	}
}

func TestOAuth2Refresher_RefreshToken_ClientCredentials(t *testing.T) {
	// Create a mock OAuth2 server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}

		// Parse form
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		// Verify client credentials grant
		if r.FormValue("grant_type") != "client_credentials" {
			t.Errorf("Expected grant_type=client_credentials, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "test-client-id" {
			t.Errorf("Expected client_id=test-client-id, got %s", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") != "test-client-secret" {
			t.Errorf("Expected client_secret=test-client-secret, got %s", r.FormValue("client_secret"))
		}

		// Return a valid token response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-access-token-123",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	refresher := NewOAuth2Refresher("test-client-id", "test-client-secret", server.URL)

	ctx := context.Background()
	tokenResp, err := refresher.RefreshToken(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tokenResp.Token != "test-access-token-123" {
		t.Errorf("Expected token test-access-token-123, got %s", tokenResp.Token)
	}

	// Verify expiration is approximately 1 hour from now
	expectedExpiry := time.Now().Add(time.Hour)
	if tokenResp.ExpiresAt.Before(expectedExpiry.Add(-time.Minute)) || tokenResp.ExpiresAt.After(expectedExpiry.Add(time.Minute)) {
		t.Errorf("Expected expiration around %v, got %v", expectedExpiry, tokenResp.ExpiresAt)
	}
}

func TestOAuth2Refresher_RefreshToken_WithRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		// Verify refresh_token grant
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("Expected grant_type=refresh_token, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "old-refresh-token" {
			t.Errorf("Expected refresh_token=old-refresh-token, got %s", r.FormValue("refresh_token"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"expires_in":    7200,
			"refresh_token": "new-refresh-token",
		})
	}))
	defer server.Close()

	refresher := NewOAuth2Refresher(
		"test-client-id",
		"test-client-secret",
		server.URL,
		WithRefreshToken("old-refresh-token"),
	)

	ctx := context.Background()
	tokenResp, err := refresher.RefreshToken(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tokenResp.Token != "new-access-token" {
		t.Errorf("Expected token new-access-token, got %s", tokenResp.Token)
	}

	// Verify the refresh token was updated
	if refresher.refreshToken != "new-refresh-token" {
		t.Errorf("Expected refreshToken to be updated to new-refresh-token, got %s", refresher.refreshToken)
	}
}

func TestOAuth2Refresher_RefreshToken_WithScopes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		expectedScope := "read write admin"
		if r.FormValue("scope") != expectedScope {
			t.Errorf("Expected scope=%s, got %s", expectedScope, r.FormValue("scope"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "scoped-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"scope":        expectedScope,
		})
	}))
	defer server.Close()

	refresher := NewOAuth2Refresher(
		"test-client-id",
		"test-client-secret",
		server.URL,
		WithScopes([]string{"read", "write", "admin"}),
	)

	ctx := context.Background()
	tokenResp, err := refresher.RefreshToken(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tokenResp.Token != "scoped-token" {
		t.Errorf("Expected token scoped-token, got %s", tokenResp.Token)
	}
}

func TestOAuth2Refresher_RefreshToken_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_client",
			"error_description": "Invalid client credentials",
		})
	}))
	defer server.Close()

	refresher := NewOAuth2Refresher("bad-client", "bad-secret", server.URL)

	ctx := context.Background()
	_, err := refresher.RefreshToken(ctx)

	if err == nil {
		t.Fatal("Expected error for invalid client credentials")
	}

	expectedError := "oauth2 error: invalid_client - Invalid client credentials"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}

func TestOAuth2Refresher_RefreshToken_NetworkError(t *testing.T) {
	// Use an invalid URL to trigger a network error
	refresher := NewOAuth2Refresher("client", "secret", "http://invalid.localhost:99999")

	ctx := context.Background()
	_, err := refresher.RefreshToken(ctx)

	if err == nil {
		t.Fatal("Expected error for network failure")
	}
}

func TestOAuth2Refresher_WithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 10 * time.Second}

	refresher := NewOAuth2Refresher(
		"client",
		"secret",
		"http://example.com",
		WithHTTPClient(customClient),
	)

	if refresher.httpClient != customClient {
		t.Error("Expected custom HTTP client to be set")
	}
}

func TestMiddleware_WrapClient(t *testing.T) {
	apiKey := "test-api-key"
	auth := NewAPIKeyAuth(apiKey)
	middleware := NewMiddleware(auth)

	originalClient := &http.Client{Timeout: 30 * time.Second}
	wrappedClient := middleware.WrapClient(originalClient)

	// The wrapped client should be a different instance
	if wrappedClient == originalClient {
		t.Error("Expected wrapped client to be different from original")
	}

	// Verify the wrapped client preserves timeout
	if wrappedClient.Timeout != originalClient.Timeout {
		t.Errorf("Expected timeout %v, got %v", originalClient.Timeout, wrappedClient.Timeout)
	}
}

func TestMiddleware_WrapClient_NilClient(t *testing.T) {
	apiKey := "test-api-key"
	auth := NewAPIKeyAuth(apiKey)
	middleware := NewMiddleware(auth)

	wrappedClient := middleware.WrapClient(nil)

	if wrappedClient == nil {
		t.Error("Expected non-nil wrapped client")
	}
}

func TestMiddleware_WrapClient_AddsAuth(t *testing.T) {
	// Create a test server to verify auth header is added
	authHeaderReceived := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaderReceived = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiKey := "test-api-key"
	auth := NewAPIKeyAuth(apiKey)
	middleware := NewMiddleware(auth)

	wrappedClient := middleware.WrapClient(&http.Client{})

	// Make a request
	resp, err := wrappedClient.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	expectedAuth := "Bearer " + apiKey
	if authHeaderReceived != expectedAuth {
		t.Errorf("Expected Authorization header %q, got %q", expectedAuth, authHeaderReceived)
	}
}

func TestAuthTransport_RoundTrip(t *testing.T) {
	authHeaderReceived := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaderReceived = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apiKey := "transport-test-key"
	auth := NewAPIKeyAuth(apiKey)

	transport := &authTransport{
		base:        http.DefaultTransport,
		authManager: auth,
	}

	client := &http.Client{Transport: transport}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	expectedAuth := "Bearer " + apiKey
	if authHeaderReceived != expectedAuth {
		t.Errorf("Expected Authorization header %q, got %q", expectedAuth, authHeaderReceived)
	}
}

func TestAuthManager_TokenRefresh_CachesToken(t *testing.T) {
	callCount := 0
	refresher := &countingRefresher{
		token:     "cached-token",
		expiresAt: time.Now().Add(time.Hour),
		callCount: &callCount,
	}

	manager := NewAuthManager("api-key", refresher)
	ctx := context.Background()

	// First call should trigger refresh
	_, err := manager.GetAuthHeader(ctx)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Second call should use cached token
	_, err = manager.GetAuthHeader(ctx)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected RefreshToken to be called once, got %d", callCount)
	}
}

type countingRefresher struct {
	token     string
	expiresAt time.Time
	callCount *int
}

func (c *countingRefresher) RefreshToken(ctx context.Context) (*TokenResponse, error) {
	*c.callCount++
	return &TokenResponse{
		Token:     c.token,
		ExpiresAt: c.expiresAt,
	}, nil
}

func TestAuthManager_TokenRefresh_RefreshesExpiredToken(t *testing.T) {
	callCount := 0
	refresher := &countingRefresher{
		token:     "expired-token",
		expiresAt: time.Now().Add(-time.Hour), // Already expired
		callCount: &callCount,
	}

	manager := NewAuthManager("api-key", refresher)
	ctx := context.Background()

	// First call
	_, err := manager.GetAuthHeader(ctx)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Token is expired, so second call should also refresh
	refresher.expiresAt = time.Now().Add(-time.Hour)
	_, err = manager.GetAuthHeader(ctx)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if callCount < 2 {
		t.Errorf("Expected RefreshToken to be called at least twice for expired tokens, got %d", callCount)
	}
}
