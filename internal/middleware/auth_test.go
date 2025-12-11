package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNewAuthMiddleware(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := AuthConfig{}
		middleware := NewAuthMiddleware(config, nil)

		if middleware == nil {
			t.Fatal("Expected middleware instance, got nil")
		}

		if middleware.secretKey != "default-secret-key-change-in-production" {
			t.Errorf("Expected default secret key, got %s", middleware.secretKey)
		}

		if middleware.tokenExpiry != 24*time.Hour {
			t.Errorf("Expected 24 hour token expiry, got %v", middleware.tokenExpiry)
		}

		if middleware.issuer != "superagent" {
			t.Errorf("Expected issuer 'superagent', got %s", middleware.issuer)
		}
	})

	t.Run("CustomConfig", func(t *testing.T) {
		config := AuthConfig{
			SecretKey:   "custom-secret-key",
			TokenExpiry: 2 * time.Hour,
			Issuer:      "custom-issuer",
		}
		middleware := NewAuthMiddleware(config, nil)

		if middleware.secretKey != "custom-secret-key" {
			t.Errorf("Expected custom secret key, got %s", middleware.secretKey)
		}

		if middleware.tokenExpiry != 2*time.Hour {
			t.Errorf("Expected 2 hour token expiry, got %v", middleware.tokenExpiry)
		}

		if middleware.issuer != "custom-issuer" {
			t.Errorf("Expected issuer 'custom-issuer', got %s", middleware.issuer)
		}
	})
}

func TestAuthMiddleware_GenerateToken(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	t.Run("ValidTokenGeneration", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		if token == "" {
			t.Fatal("Expected non-empty token, got empty string")
		}

		// Validate the token can be parsed
		claims, err := middleware.validateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate generated token: %v", err)
		}

		if claims.UserID != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", claims.UserID)
		}

		if claims.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", claims.Username)
		}

		if claims.Role != "user" {
			t.Errorf("Expected role 'user', got %s", claims.Role)
		}
	})

	t.Run("DifferentRoles", func(t *testing.T) {
		token, err := middleware.GenerateToken("admin123", "adminuser", "admin")
		if err != nil {
			t.Fatalf("Failed to generate admin token: %v", err)
		}

		claims, err := middleware.validateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate admin token: %v", err)
		}

		if claims.Role != "admin" {
			t.Errorf("Expected role 'admin', got %s", claims.Role)
		}
	})
}

func TestAuthMiddleware_ValidateToken(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	t.Run("ValidToken", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		claims, err := middleware.validateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate valid token: %v", err)
		}

		if claims.UserID != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", claims.UserID)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		_, err := middleware.validateToken("invalid.token.string")
		if err == nil {
			t.Error("Expected error for invalid token, got nil")
		}
	})

	t.Run("WrongSecretKey", func(t *testing.T) {
		// Create token with one middleware
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Try to validate with different secret key
		wrongConfig := AuthConfig{
			SecretKey:   "wrong-secret-key",
			TokenExpiry: time.Hour,
		}
		wrongMiddleware := NewAuthMiddleware(wrongConfig, nil)

		_, err = wrongMiddleware.validateToken(token)
		if err == nil {
			t.Error("Expected error for token with wrong secret key, got nil")
		}
	})
}

func TestAuthMiddleware_ExtractTokenFromHeader(t *testing.T) {
	config := AuthConfig{}
	middleware := NewAuthMiddleware(config, nil)

	t.Run("ValidBearerToken", func(t *testing.T) {
		authHeader := "Bearer test.token.here"
		token := middleware.ExtractTokenFromHeader(authHeader)

		if token != "test.token.here" {
			t.Errorf("Expected token 'test.token.here', got %s", token)
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		authHeader := "Basic dGVzdDp0ZXN0"
		token := middleware.ExtractTokenFromHeader(authHeader)

		if token != "" {
			t.Errorf("Expected empty token for invalid format, got %s", token)
		}
	})

	t.Run("EmptyHeader", func(t *testing.T) {
		token := middleware.ExtractTokenFromHeader("")

		if token != "" {
			t.Errorf("Expected empty token for empty header, got %s", token)
		}
	})

	t.Run("MalformedHeader", func(t *testing.T) {
		token := middleware.ExtractTokenFromHeader("Bearer")

		if token != "" {
			t.Errorf("Expected empty token for malformed header, got %s", token)
		}
	})
}

func TestAuthMiddleware_RefreshToken(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	t.Run("ValidRefresh", func(t *testing.T) {
		originalToken, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate original token: %v", err)
		}

		newToken, err := middleware.RefreshToken(originalToken)
		if err != nil {
			t.Fatalf("Failed to refresh token: %v", err)
		}

		if newToken == "" {
			t.Fatal("Expected non-empty refreshed token, got empty string")
		}

		// Validate new token
		claims, err := middleware.validateToken(newToken)
		if err != nil {
			t.Fatalf("Failed to validate refreshed token: %v", err)
		}

		if claims.UserID != "user123" {
			t.Errorf("Expected user ID 'user123' in refreshed token, got %s", claims.UserID)
		}
	})

	t.Run("InvalidTokenRefresh", func(t *testing.T) {
		_, err := middleware.RefreshToken("invalid.token.string")
		if err == nil {
			t.Error("Expected error when refreshing invalid token, got nil")
		}
	})
}

func TestAuthMiddleware_Middleware(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	// Create a test Gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add protected route
	router.GET("/protected", middleware.Middleware([]string{}), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "protected"})
	})

	// Add route with skip path
	router.GET("/public", middleware.Middleware([]string{"/public"}), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public"})
	})

	t.Run("MissingAuthHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("InvalidAuthFormat", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("ValidToken", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("SkipPath", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/public", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for skip path, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_Optional(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	// Create a test Gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add optional auth route
	router.GET("/optional", middleware.Optional([]string{}), func(c *gin.Context) {
		authenticated := IsAuthenticated(c)
		c.JSON(http.StatusOK, gin.H{"authenticated": authenticated})
	})

	t.Run("NoAuthHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/optional", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("WithValidToken", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		req := httptest.NewRequest("GET", "/optional", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_RequireRole(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	// Create a test Gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add admin-only route
	router.GET("/admin", middleware.Middleware([]string{}), middleware.RequireRole("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin only"})
	})

	t.Run("UserRoleAccessDenied", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 for non-admin user, got %d", w.Code)
		}
	})

	t.Run("AdminRoleAccessGranted", func(t *testing.T) {
		token, err := middleware.GenerateToken("admin123", "adminuser", "admin")
		if err != nil {
			t.Fatalf("Failed to generate admin token: %v", err)
		}

		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for admin user, got %d", w.Code)
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	// Create a test Gin context
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	t.Run("GetCurrentUser_NoUser", func(t *testing.T) {
		user := GetCurrentUser(c)
		if user != nil {
			t.Errorf("Expected nil user, got %v", user)
		}
	})

	t.Run("GetUserID_NoUser", func(t *testing.T) {
		userID := GetUserID(c)
		if userID != "" {
			t.Errorf("Expected empty user ID, got %s", userID)
		}
	})

	t.Run("GetUserRole_NoUser", func(t *testing.T) {
		role := GetUserRole(c)
		if role != "" {
			t.Errorf("Expected empty role, got %s", role)
		}
	})

	t.Run("IsAuthenticated_NoUser", func(t *testing.T) {
		authenticated := IsAuthenticated(c)
		if authenticated {
			t.Error("Expected false for unauthenticated user")
		}
	})

	t.Run("HasRole_NoUser", func(t *testing.T) {
		hasRole := HasRole(c, "admin")
		if hasRole {
			t.Error("Expected false for user without role")
		}
	})

	t.Run("IsAdmin_NoUser", func(t *testing.T) {
		isAdmin := IsAdmin(c)
		if isAdmin {
			t.Error("Expected false for non-admin user")
		}
	})
}

func TestGenerateToken(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	t.Run("ValidTokenGeneration", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		if token == "" {
			t.Fatal("Expected non-empty token, got empty string")
		}

		// Validate the token
		claims, err := middleware.validateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate generated token: %v", err)
		}

		if claims.UserID != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", claims.UserID)
		}

		if claims.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", claims.Username)
		}

		if claims.Role != "user" {
			t.Errorf("Expected role 'user', got %s", claims.Role)
		}

		if claims.Issuer != "superagent" {
			t.Errorf("Expected issuer 'superagent', got %s", claims.Issuer)
		}
	})

	t.Run("DifferentRoles", func(t *testing.T) {
		testCases := []struct {
			userID   string
			username string
			role     string
		}{
			{"admin1", "adminuser", "admin"},
			{"mod1", "moduser", "moderator"},
			{"user1", "regularuser", "user"},
		}

		for _, tc := range testCases {
			token, err := middleware.GenerateToken(tc.userID, tc.username, tc.role)
			if err != nil {
				t.Fatalf("Failed to generate token for %s: %v", tc.role, err)
			}

			claims, err := middleware.validateToken(token)
			if err != nil {
				t.Fatalf("Failed to validate token for %s: %v", tc.role, err)
			}

			if claims.Role != tc.role {
				t.Errorf("Expected role %s, got %s", tc.role, claims.Role)
			}
		}
	})
}

func TestValidateToken(t *testing.T) {
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware := NewAuthMiddleware(config, nil)

	t.Run("ValidToken", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		claims, err := middleware.validateToken(token)
		if err != nil {
			t.Fatalf("Failed to validate valid token: %v", err)
		}

		if claims.UserID != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", claims.UserID)
		}
	})

	t.Run("InvalidTokenFormat", func(t *testing.T) {
		_, err := middleware.validateToken("invalid.token.format")
		if err == nil {
			t.Error("Expected error for invalid token format, got nil")
		}
	})

	t.Run("EmptyToken", func(t *testing.T) {
		_, err := middleware.validateToken("")
		if err == nil {
			t.Error("Expected error for empty token, got nil")
		}
	})

	t.Run("TamperedToken", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Tamper with the token
		tamperedToken := token[:len(token)-5] + "xxxxx"
		_, err = middleware.validateToken(tamperedToken)
		if err == nil {
			t.Error("Expected error for tampered token, got nil")
		}
	})

	t.Run("WrongSecretKey", func(t *testing.T) {
		token, err := middleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Create middleware with different secret key
		wrongConfig := AuthConfig{
			SecretKey:   "different-secret-key",
			TokenExpiry: time.Hour,
		}
		wrongMiddleware := NewAuthMiddleware(wrongConfig, nil)

		_, err = wrongMiddleware.validateToken(token)
		if err == nil {
			t.Error("Expected error for token with wrong secret key, got nil")
		}
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		// Create middleware with very short expiry
		shortConfig := AuthConfig{
			SecretKey:   "test-secret-key",
			TokenExpiry: time.Millisecond,
		}
		shortMiddleware := NewAuthMiddleware(shortConfig, nil)

		token, err := shortMiddleware.GenerateToken("user123", "testuser", "user")
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Wait for token to expire
		time.Sleep(2 * time.Millisecond)

		_, err = shortMiddleware.validateToken(token)
		if err == nil {
			t.Error("Expected error for expired token, got nil")
		}
	})
}
