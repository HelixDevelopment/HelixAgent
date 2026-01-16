package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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

func TestAuthManager_TokenRefresh_WithinBuffer(t *testing.T) {
	callCount := 0
	// Token expires in 3 minutes - within the 5 minute buffer
	refresher := &countingRefresher{
		token:     "about-to-expire-token",
		expiresAt: time.Now().Add(3 * time.Minute),
		callCount: &callCount,
	}

	manager := NewAuthManager("api-key", refresher)
	ctx := context.Background()

	// First call should trigger refresh since token is within 5 minute buffer
	_, err := manager.GetAuthHeader(ctx)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	// Verify refresh was called
	if callCount != 1 {
		t.Errorf("Expected RefreshToken to be called once, got %d", callCount)
	}
}

// ErrorRefresher is a mock that returns errors
type ErrorRefresher struct {
	err error
}

func (e *ErrorRefresher) RefreshToken(ctx context.Context) (*TokenResponse, error) {
	return nil, e.err
}

func TestAuthManager_TokenRefresh_Error(t *testing.T) {
	refresher := &ErrorRefresher{err: http.ErrAbortHandler}

	manager := NewAuthManager("api-key", refresher)
	ctx := context.Background()

	_, err := manager.GetAuthHeader(ctx)
	if err == nil {
		t.Fatal("Expected error from failed token refresh")
	}

	if !strings.Contains(err.Error(), "failed to refresh token") {
		t.Errorf("Expected 'failed to refresh token' in error message, got: %s", err.Error())
	}
}

func TestOAuth2Refresher_InvalidURL(t *testing.T) {
	refresher := NewOAuth2Refresher("client", "secret", "://invalid-url")

	ctx := context.Background()
	_, err := refresher.RefreshToken(ctx)

	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

func TestOAuth2Refresher_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	refresher := NewOAuth2Refresher("client", "secret", server.URL)

	ctx := context.Background()
	_, err := refresher.RefreshToken(ctx)

	if err == nil {
		t.Fatal("Expected error for invalid JSON response")
	}

	if !strings.Contains(err.Error(), "failed to parse token response") {
		t.Errorf("Expected 'failed to parse token response' in error, got: %s", err.Error())
	}
}

func TestOAuth2Refresher_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	refresher := NewOAuth2Refresher("client", "secret", server.URL)

	ctx := context.Background()
	_, err := refresher.RefreshToken(ctx)

	if err == nil {
		t.Fatal("Expected error for non-OK status")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected status code 500 in error, got: %s", err.Error())
	}
}

func TestOAuth2Refresher_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	refresher := NewOAuth2Refresher("client", "secret", server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := refresher.RefreshToken(ctx)

	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

func TestAuthInterceptor_ErrorHandler(t *testing.T) {
	errorAuth := &ErrorAuthManager{}
	interceptor := NewAuthInterceptor(errorAuth)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err := interceptor.Intercept(req)

	if err == nil {
		t.Fatal("Expected error from auth interceptor")
	}

	if !strings.Contains(err.Error(), "failed to get auth header") {
		t.Errorf("Expected 'failed to get auth header' in error, got: %s", err.Error())
	}
}

// ErrorAuthManager always returns an error
type ErrorAuthManager struct{}

func (e *ErrorAuthManager) GetAuthHeader(ctx context.Context) (string, error) {
	return "", http.ErrAbortHandler
}

func TestAuthTransport_ErrorHandler(t *testing.T) {
	errorAuth := &ErrorAuthManager{}

	transport := &authTransport{
		base:        http.DefaultTransport,
		authManager: errorAuth,
	}

	client := &http.Client{Transport: transport}

	_, err := client.Get("http://example.com")

	if err == nil {
		t.Fatal("Expected error from auth transport")
	}

	if !strings.Contains(err.Error(), "failed to get auth header") {
		t.Errorf("Expected 'failed to get auth header' in error, got: %s", err.Error())
	}
}

func TestMiddleware_WrapClient_PreservesSettings(t *testing.T) {
	apiKey := "test-api-key"
	auth := NewAPIKeyAuth(apiKey)
	middleware := NewMiddleware(auth)

	jar := &testCookieJar{}
	checkRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	originalClient := &http.Client{
		Timeout:       45 * time.Second,
		Jar:           jar,
		CheckRedirect: checkRedirect,
	}

	wrappedClient := middleware.WrapClient(originalClient)

	// Verify settings are preserved
	if wrappedClient.Timeout != originalClient.Timeout {
		t.Errorf("Expected timeout %v, got %v", originalClient.Timeout, wrappedClient.Timeout)
	}

	if wrappedClient.Jar != originalClient.Jar {
		t.Error("Expected cookie jar to be preserved")
	}
}

// testCookieJar is a minimal cookie jar for testing
type testCookieJar struct{}

func (j *testCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {}
func (j *testCookieJar) Cookies(u *url.URL) []*http.Cookie             { return nil }

func TestAuthManager_ConcurrentAccess(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	refresher := &syncRefresher{
		token:     "concurrent-token",
		expiresAt: time.Now().Add(time.Hour),
		callCount: &callCount,
		mu:        &mu,
	}

	manager := NewAuthManager("api-key", refresher)
	ctx := context.Background()

	// Run multiple goroutines concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.GetAuthHeader(ctx)
			if err != nil {
				t.Errorf("Concurrent call failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// Token should only be refreshed once due to caching
	if callCount != 1 {
		t.Errorf("Expected RefreshToken to be called once, got %d", callCount)
	}
}

type syncRefresher struct {
	token     string
	expiresAt time.Time
	callCount *int
	mu        *sync.Mutex
}

func (s *syncRefresher) RefreshToken(ctx context.Context) (*TokenResponse, error) {
	s.mu.Lock()
	*s.callCount++
	s.mu.Unlock()
	return &TokenResponse{
		Token:     s.token,
		ExpiresAt: s.expiresAt,
	}, nil
}

func TestAPIKeyAuth_NilContext(t *testing.T) {
	auth := NewAPIKeyAuth("test-key")

	// This should work even with a nil context (implementation uses context internally)
	header, err := auth.GetAuthHeader(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if header != "Bearer test-key" {
		t.Errorf("Expected 'Bearer test-key', got '%s'", header)
	}
}

func TestOAuth2Refresher_AllOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 60 * time.Second}

	refresher := NewOAuth2Refresher(
		"client-id",
		"client-secret",
		"http://example.com/token",
		WithRefreshToken("refresh-token"),
		WithScopes([]string{"read", "write"}),
		WithHTTPClient(customClient),
	)

	if refresher.clientID != "client-id" {
		t.Errorf("Expected clientID 'client-id', got %s", refresher.clientID)
	}

	if refresher.clientSecret != "client-secret" {
		t.Errorf("Expected clientSecret 'client-secret', got %s", refresher.clientSecret)
	}

	if refresher.tokenURL != "http://example.com/token" {
		t.Errorf("Expected tokenURL 'http://example.com/token', got %s", refresher.tokenURL)
	}

	if refresher.refreshToken != "refresh-token" {
		t.Errorf("Expected refreshToken 'refresh-token', got %s", refresher.refreshToken)
	}

	if len(refresher.scopes) != 2 || refresher.scopes[0] != "read" || refresher.scopes[1] != "write" {
		t.Errorf("Expected scopes [read, write], got %v", refresher.scopes)
	}

	if refresher.httpClient != customClient {
		t.Error("Expected custom HTTP client to be set")
	}
}

func TestNewMiddleware(t *testing.T) {
	auth := NewAPIKeyAuth("test-key")
	middleware := NewMiddleware(auth)

	if middleware == nil {
		t.Fatal("Expected non-nil middleware")
	}

	if middleware.authManager == nil {
		t.Error("Expected authManager to be set")
	}
}
