package integration

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// =============================================================================
// Shared test constants and helper functions
// =============================================================================

const (
	// HelixAgentBaseURL is the HelixAgent service URL for integration tests
	HelixAgentBaseURL = "http://localhost:7061"
	// TestEmail is the test user email for authentication
	TestEmail = "admin@helixagent.ai"
	// TestPassword is the test user password for authentication
	TestPassword = "HelixAgentPass123"
)

// jwtClaims holds the claims used for generating test JWT tokens
type jwtClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// generateTestJWT creates a valid JWT token for integration tests using
// the JWT_SECRET env var (falls back to the default test secret).
func generateTestJWT() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "helixagent-test-secret-key-for-challenges-1767638342"
	}

	claims := &jwtClaims{
		UserID:   "1",
		Username: "admin",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "helixagent",
			Subject:   "1",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return ""
	}
	return tokenString
}

// getTestAPIKey returns a valid JWT token for integration tests. If
// HELIXAGENT_API_KEY is set and starts with "eyJ" (JWT format) it is used
// directly; otherwise a fresh JWT is generated from JWT_SECRET.
func getTestAPIKey() string {
	apiKey := os.Getenv("HELIXAGENT_API_KEY")
	if len(apiKey) > 3 && apiKey[:3] == "eyJ" {
		// Already a JWT token
		return apiKey
	}
	// Generate a valid JWT using the project JWT secret
	return generateTestJWT()
}

// checkAuthAndHandleFailure checks if the response indicates auth failure.
// Returns true if test should continue normally, false if test should be skipped due to auth issues.
func checkAuthAndHandleFailure(t *testing.T, resp *http.Response, body []byte, endpoint string) bool {
	t.Helper()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Logf("HelixAgent auth not configured for test API key on %s (status %d) - skipping proxy test", endpoint, resp.StatusCode)
		return false
	}
	return true
}
