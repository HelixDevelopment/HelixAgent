package auth

import (
	"context"
	"net/http"
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

func TestOAuth2Refresher_RefreshToken(t *testing.T) {
	refresher := NewOAuth2Refresher("client-id", "client-secret", "http://token.url")

	ctx := context.Background()
	tokenResp, err := refresher.RefreshToken(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tokenResp.Token == "" {
		t.Error("Expected non-empty token")
	}

	if tokenResp.ExpiresAt.IsZero() {
		t.Error("Expected non-zero expiration time")
	}
}

func TestMiddleware_WrapClient(t *testing.T) {
	apiKey := "test-api-key"
	auth := NewAPIKeyAuth(apiKey)
	middleware := NewMiddleware(auth)

	originalClient := &http.Client{}
	wrappedClient := middleware.WrapClient(originalClient)

	// For now, the implementation returns the client as-is
	if wrappedClient != originalClient {
		t.Error("Expected wrapped client to be the same as original (current implementation)")
	}
}
