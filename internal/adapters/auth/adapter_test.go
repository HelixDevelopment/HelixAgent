package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOAuthConfig(t *testing.T) {
	t.Parallel()
	config := adapter.DefaultOAuthConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 10*time.Minute, config.RefreshThreshold)
	assert.Equal(t, 5*time.Minute, config.CacheDuration)
	assert.Equal(t, 30*time.Second, config.RateLimitInterval)
}

func TestNewJWTConfig(t *testing.T) {
	t.Parallel()
	secret := "test-secret-key"
	config := adapter.NewJWTConfig(secret)

	assert.NotNil(t, config)
	assert.Equal(t, []byte(secret), config.Secret)
	assert.Equal(t, time.Hour, config.Expiration)
}

func TestJWTManagerCreateAndValidate(t *testing.T) {
	t.Parallel()
	config := adapter.NewJWTConfig("test-secret-key-12345678")
	manager := adapter.NewJWTManager(config)

	claims := map[string]interface{}{
		"user_id": "user-123",
		"role":    "admin",
	}

	// Create token
	token, err := manager.Create(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate token
	parsed, err := manager.Validate(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", parsed.Claims["user_id"])
	assert.Equal(t, "admin", parsed.Claims["role"])
	assert.False(t, parsed.ExpiresAt.IsZero())
	assert.False(t, parsed.IssuedAt.IsZero())
}

func TestJWTValidatorAdapter(t *testing.T) {
	t.Parallel()
	config := adapter.NewJWTConfig("test-secret-key-12345678")
	manager := adapter.NewJWTManager(config)

	claims := map[string]interface{}{
		"user_id": "user-123",
	}

	token, err := manager.Create(claims)
	require.NoError(t, err)

	// Create validator adapter
	validator := adapter.NewJWTValidatorAdapter(manager)

	// Validate using adapter
	validatedClaims, err := validator.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", validatedClaims["user_id"])
}

func TestJWTValidatorAdapterInvalidToken(t *testing.T) {
	t.Parallel()
	config := adapter.NewJWTConfig("test-secret-key-12345678")
	manager := adapter.NewJWTManager(config)
	validator := adapter.NewJWTValidatorAdapter(manager)

	_, err := validator.ValidateToken("invalid-token")
	assert.Error(t, err)
}

func TestIsExpired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "zero time",
			expiresAt: time.Time{},
			expected:  false,
		},
		{
			name:      "future time",
			expiresAt: time.Now().Add(time.Hour),
			expected:  false,
		},
		{
			name:      "past time",
			expiresAt: time.Now().Add(-time.Hour),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := adapter.IsExpired(tt.expiresAt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNeedsRefresh(t *testing.T) {
	t.Parallel()
	threshold := 10 * time.Minute

	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "zero time",
			expiresAt: time.Time{},
			expected:  false,
		},
		{
			name:      "far future",
			expiresAt: time.Now().Add(time.Hour),
			expected:  false,
		},
		{
			name:      "within threshold",
			expiresAt: time.Now().Add(5 * time.Minute),
			expected:  true,
		},
		{
			name:      "past time",
			expiresAt: time.Now().Add(-time.Hour),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := adapter.NeedsRefresh(tt.expiresAt, threshold)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClaimsFromContext(t *testing.T) {
	t.Parallel()
	// Test with empty context
	claims := adapter.ClaimsFromContext(context.Background())
	assert.Nil(t, claims)
}

func TestScopesFromContext(t *testing.T) {
	t.Parallel()
	// Test with empty context
	scopes := adapter.ScopesFromContext(context.Background())
	assert.Nil(t, scopes)
}

func TestAPIKeyFromContext(t *testing.T) {
	t.Parallel()
	// Test with empty context
	key := adapter.APIKeyFromContext(context.Background())
	assert.Empty(t, key)
}

// TestMiddlewareChain tests the middleware chaining functionality.
func TestMiddlewareChain(t *testing.T) {
	t.Parallel()
	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	chained := adapter.ChainMiddleware(mw1, mw2)
	final := chained(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	final.ServeHTTP(rr, req)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	assert.Equal(t, expected, order)
}
