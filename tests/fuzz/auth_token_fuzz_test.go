//go:build fuzz

// Package fuzz provides Go native fuzzing tests for critical parsing paths
// in the HelixAgent system. These tests ensure that malformed or adversarial
// input never causes panics or undefined behavior.
package fuzz

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/middleware"
)

// FuzzJWTTokenValidation tests that validating arbitrary token strings through
// the AuthMiddleware never panics. This exercises the JWT parsing, signature
// verification, and claims extraction paths in internal/middleware/auth.go.
func FuzzJWTTokenValidation(f *testing.F) {
	// Create a valid middleware instance for generating real tokens
	config := middleware.AuthConfig{
		SecretKey:   "fuzz-test-secret-key-32bytes!!!",
		TokenExpiry: time.Hour,
		Issuer:      "helixagent-fuzz",
	}
	mw, err := middleware.NewAuthMiddleware(config, nil)
	if err != nil {
		f.Fatalf("Failed to create auth middleware: %v", err)
	}

	// Generate a real valid token for the seed corpus
	validToken, err := mw.GenerateToken("user-123", "testuser", "admin")
	if err != nil {
		f.Fatalf("Failed to generate token: %v", err)
	}

	// Seed corpus: valid tokens, malformed tokens, and adversarial inputs
	f.Add(validToken)
	f.Add("invalid.token.string")
	f.Add("")
	f.Add("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c")
	f.Add("not-a-jwt-at-all")
	f.Add(strings.Repeat("a", 10000))
	f.Add("header.payload.signature.extra.parts")
	f.Add("..")                                                               // Three-part but empty
	f.Add("eyJ0eXAiOiJKV1QiLCJhbGciOiJub25lIn0.eyJzdWIiOiIxMjM0NTY3ODkwIn0.") // alg:none attack
	f.Add("\x00\x01\x02\xff\xfe")
	f.Add("eyJhbGciOiJSUzI1NiJ9.eyJ0ZXN0IjoxfQ.signature") // RS256 alg confusion

	f.Fuzz(func(t *testing.T, tokenStr string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzJWTTokenValidation panicked with token %q: %v", tokenStr, r)
			}
		}()

		// Try to refresh the token (calls validateToken internally)
		_, _ = mw.RefreshToken(tokenStr)

		// Try to extract token from a fake authorization header
		_ = mw.ExtractTokenFromHeader("Bearer " + tokenStr)
		_ = mw.ExtractTokenFromHeader(tokenStr)
	})
}

// FuzzJWTTokenExtraction tests the ExtractTokenFromHeader method with arbitrary
// authorization header values, ensuring no panics on malformed headers.
func FuzzJWTTokenExtraction(f *testing.F) {
	f.Add("Bearer eyJhbGciOiJIUzI1NiJ9.eyJ0ZXN0IjoxfQ.sig")
	f.Add("Bearer ")
	f.Add("Bearer")
	f.Add("")
	f.Add("Basic dXNlcjpwYXNz")
	f.Add("InvalidScheme token-value")
	f.Add(strings.Repeat("Bearer ", 100))
	f.Add("bearer lowercase-token")
	f.Add("BEARER UPPERCASE-TOKEN")
	f.Add("\x00Bearer\x00token")
	f.Add("Bearer " + strings.Repeat("x", 65536))
	f.Add("  Bearer  token-with-spaces  ")

	f.Fuzz(func(t *testing.T, authHeader string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzJWTTokenExtraction panicked with header %q: %v", authHeader, r)
			}
		}()

		config := middleware.AuthConfig{
			SecretKey:   "fuzz-test-secret-key-32bytes!!!",
			TokenExpiry: time.Hour,
		}
		mw, err := middleware.NewAuthMiddleware(config, nil)
		if err != nil {
			return
		}

		token := mw.ExtractTokenFromHeader(authHeader)
		_ = len(token)

		// If we extracted something, try to refresh it (validates internally)
		if token != "" {
			_, _ = mw.RefreshToken(token)
		}
	})
}

// FuzzJWTClaimsParsing tests that parsing JWT claims payloads from arbitrary
// JSON never panics. The claims section is base64-decoded JSON that may contain
// arbitrary fields, types, and values.
func FuzzJWTClaimsParsing(f *testing.F) {
	// Valid claims payloads
	f.Add(`{"user_id":"user-123","username":"testuser","role":"admin","exp":1999999999,"iat":1600000000,"iss":"helixagent","sub":"user-123"}`)
	f.Add(`{"user_id":"","username":"","role":"","exp":0}`)
	f.Add(`{}`)
	f.Add(`null`)
	f.Add(`invalid json`)
	// Adversarial claims
	f.Add(`{"user_id":"'; DROP TABLE users; --","role":"<script>alert(1)</script>"}`)
	f.Add(`{"exp":"not-a-number","iat":true,"iss":42}`)
	f.Add(`{"user_id":"` + strings.Repeat("x", 10000) + `"}`)
	f.Add(`{"role":"superadmin","admin":true,"permissions":["*"]}`)
	f.Add(`{"exp":-1,"iat":-1,"nbf":-1}`)

	f.Fuzz(func(t *testing.T, claimsJSON string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzJWTClaimsParsing panicked with claims %q: %v", claimsJSON, r)
			}
		}()

		// Parse as generic claims map (mirrors JWT library internal parsing)
		var claims map[string]interface{}
		if err := json.Unmarshal([]byte(claimsJSON), &claims); err != nil {
			return
		}

		// Extract standard JWT registered claims
		if exp, ok := claims["exp"]; ok {
			switch v := exp.(type) {
			case float64:
				_ = time.Unix(int64(v), 0)
			}
		}
		if iat, ok := claims["iat"]; ok {
			switch v := iat.(type) {
			case float64:
				_ = time.Unix(int64(v), 0)
			}
		}

		_, _ = claims["iss"].(string)
		_, _ = claims["sub"].(string)
		_, _ = claims["aud"].(string)

		// Extract HelixAgent custom claims
		_, _ = claims["user_id"].(string)
		_, _ = claims["username"].(string)
		_, _ = claims["role"].(string)

		// Try to construct a fake JWT from the claims and parse it
		claimsB64 := base64.RawURLEncoding.EncodeToString([]byte(claimsJSON))
		headerB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
		fakeToken := headerB64 + "." + claimsB64 + ".invalid-signature"

		// This should fail validation but must not panic
		config := middleware.AuthConfig{
			SecretKey:   "fuzz-test-secret-key-32bytes!!!",
			TokenExpiry: time.Hour,
		}
		mw, err := middleware.NewAuthMiddleware(config, nil)
		if err != nil {
			return
		}
		_, _ = mw.RefreshToken(fakeToken)
	})
}

// FuzzAPIKeyValidation tests that API key extraction and validation from
// arbitrary header values never panics. API keys are used as an alternative
// to JWT tokens for authentication.
func FuzzAPIKeyValidation(f *testing.F) {
	f.Add("sk-1234567890abcdef1234567890abcdef")
	f.Add("hx-agent-key-test-12345")
	f.Add("")
	f.Add("   ")
	f.Add(strings.Repeat("a", 256))
	f.Add("\x00\x01\xff\xfe")
	f.Add("key-with-special-chars!@#$%^&*()")
	f.Add("Bearer sk-looks-like-bearer-token")
	f.Add("sk-" + strings.Repeat("x", 10000))
	f.Add("key\nwith\nnewlines")

	f.Fuzz(func(t *testing.T, apiKey string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("FuzzAPIKeyValidation panicked with key %q: %v", apiKey, r)
			}
		}()

		// Simulate API key validation logic from middleware
		trimmed := strings.TrimSpace(apiKey)
		_ = len(trimmed) > 0
		_ = len(trimmed) >= 32 // minimum key length check

		// Check common prefixes
		_ = strings.HasPrefix(trimmed, "sk-")
		_ = strings.HasPrefix(trimmed, "hx-")
		_ = strings.HasPrefix(trimmed, "Bearer ")

		// Strip Bearer prefix if present (common mistake)
		if strings.HasPrefix(trimmed, "Bearer ") {
			trimmed = trimmed[7:]
		}
		_ = len(trimmed)

		// Check for dangerous characters that should be rejected
		for _, c := range trimmed {
			_ = c >= 0x20 && c <= 0x7e // printable ASCII only
		}

		// Hash the key for storage/comparison (common operation)
		_ = hashStringFuzz(trimmed)
	})
}
