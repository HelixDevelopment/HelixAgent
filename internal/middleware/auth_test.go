package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNewAuthMiddleware(t *testing.T) {
	t.Parallel()
	t.Run("EmptySecretKeyReturnsError", func(t *testing.T) {
		config := AuthConfig{}
		middleware, err := NewAuthMiddleware(config, nil)

		if err == nil {
			t.Error("Expected error for empty secret key, got nil")
		}
		if middleware != nil {
			t.Error("Expected nil middleware for empty secret key")
		}
	})

	t.Run("CustomConfig", func(t *testing.T) {
			t.Parallel()
		config := AuthConfig{
			SecretKey:   "custom-secret-key",
			TokenExpiry: 2 * time.Hour,
			Issuer:      "custom-issuer",
		}
		middleware, err := NewAuthMiddleware(config, nil)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

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

	t.Run("DefaultTokenExpiryAndIssuer", func(t *testing.T) {
			t.Parallel()
		config := AuthConfig{
			SecretKey: "test-secret-key",
		}
		middleware, err := NewAuthMiddleware(config, nil)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if middleware.tokenExpiry != 24*time.Hour {
			t.Errorf("Expected 24 hour default token expiry, got %v", middleware.tokenExpiry)
		}

		if middleware.issuer != "helixagent" {
			t.Errorf("Expected default issuer 'helixagent', got %s", middleware.issuer)
		}
	})
}

func TestAuthMiddleware_GenerateToken(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	t.Run("ValidTokenGeneration", func(t *testing.T) {
			t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	t.Run("ValidToken", func(t *testing.T) {
			t.Parallel()
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
			t.Parallel()
		_, err := middleware.validateToken("invalid.token.string")
		if err == nil {
			t.Error("Expected error for invalid token, got nil")
		}
	})

	t.Run("WrongSecretKey", func(t *testing.T) {
			t.Parallel()
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
		wrongMiddleware, err := NewAuthMiddleware(wrongConfig, nil)
		if err != nil {
			t.Fatalf("Failed to create wrong middleware: %v", err)
		}

		_, err = wrongMiddleware.validateToken(token)
		if err == nil {
			t.Error("Expected error for token with wrong secret key, got nil")
		}
	})
}

func TestAuthMiddleware_ExtractTokenFromHeader(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey: "test-secret-key",
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	t.Run("ValidBearerToken", func(t *testing.T) {
			t.Parallel()
		authHeader := "Bearer test.token.here"
		token := middleware.ExtractTokenFromHeader(authHeader)

		if token != "test.token.here" {
			t.Errorf("Expected token 'test.token.here', got %s", token)
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
			t.Parallel()
		authHeader := "Basic dGVzdDp0ZXN0"
		token := middleware.ExtractTokenFromHeader(authHeader)

		if token != "" {
			t.Errorf("Expected empty token for invalid format, got %s", token)
		}
	})

	t.Run("EmptyHeader", func(t *testing.T) {
			t.Parallel()
		token := middleware.ExtractTokenFromHeader("")

		if token != "" {
			t.Errorf("Expected empty token for empty header, got %s", token)
		}
	})

	t.Run("MalformedHeader", func(t *testing.T) {
			t.Parallel()
		token := middleware.ExtractTokenFromHeader("Bearer")

		if token != "" {
			t.Errorf("Expected empty token for malformed header, got %s", token)
		}
	})
}

func TestAuthMiddleware_RefreshToken(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	t.Run("ValidRefresh", func(t *testing.T) {
			t.Parallel()
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
			t.Parallel()
		_, err := middleware.RefreshToken("invalid.token.string")
		if err == nil {
			t.Error("Expected error when refreshing invalid token, got nil")
		}
	})
}

func TestAuthMiddleware_Middleware(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

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
			t.Parallel()
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("InvalidAuthFormat", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("ValidToken", func(t *testing.T) {
			t.Parallel()
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
			t.Parallel()
		req := httptest.NewRequest("GET", "/public", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for skip path, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_Optional(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Create a test Gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add optional auth route
	router.GET("/optional", middleware.Optional([]string{}), func(c *gin.Context) {
		authenticated := IsAuthenticated(c)
		c.JSON(http.StatusOK, gin.H{"authenticated": authenticated})
	})

	t.Run("NoAuthHeader", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("GET", "/optional", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("WithValidToken", func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Create a test Gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add admin-only route
	router.GET("/admin", middleware.Middleware([]string{}), middleware.RequireRole("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin only"})
	})

	t.Run("UserRoleAccessDenied", func(t *testing.T) {
			t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	// Create a test Gin context
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	t.Run("GetCurrentUser_NoUser", func(t *testing.T) {
			t.Parallel()
		user := GetCurrentUser(c)
		if user != nil {
			t.Errorf("Expected nil user, got %v", user)
		}
	})

	t.Run("GetUserID_NoUser", func(t *testing.T) {
			t.Parallel()
		userID := GetUserID(c)
		if userID != "" {
			t.Errorf("Expected empty user ID, got %s", userID)
		}
	})

	t.Run("GetUserRole_NoUser", func(t *testing.T) {
			t.Parallel()
		role := GetUserRole(c)
		if role != "" {
			t.Errorf("Expected empty role, got %s", role)
		}
	})

	t.Run("IsAuthenticated_NoUser", func(t *testing.T) {
			t.Parallel()
		authenticated := IsAuthenticated(c)
		if authenticated {
			t.Error("Expected false for unauthenticated user")
		}
	})

	t.Run("HasRole_NoUser", func(t *testing.T) {
			t.Parallel()
		hasRole := HasRole(c, "admin")
		if hasRole {
			t.Error("Expected false for user without role")
		}
	})

	t.Run("IsAdmin_NoUser", func(t *testing.T) {
			t.Parallel()
		isAdmin := IsAdmin(c)
		if isAdmin {
			t.Error("Expected false for non-admin user")
		}
	})
}

func TestHelperFunctions_WithUser(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("GetCurrentUser_WithClaims", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		claims := &Claims{
			UserID:   "user123",
			Username: "testuser",
			Role:     "admin",
		}
		c.Set("claims", claims)

		user := GetCurrentUser(c)
		if user == nil {
			t.Fatal("Expected user claims")
		}
		if user.UserID != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", user.UserID)
		}
	})

	t.Run("GetCurrentUser_WrongType", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("claims", "not a claims struct")

		user := GetCurrentUser(c)
		if user != nil {
			t.Error("Expected nil for invalid claims type")
		}
	})

	t.Run("GetUserID_Valid", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("user_id", "user456")

		userID := GetUserID(c)
		if userID != "user456" {
			t.Errorf("Expected 'user456', got %s", userID)
		}
	})

	t.Run("GetUserID_WrongType", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("user_id", 123) // Not a string

		userID := GetUserID(c)
		if userID != "" {
			t.Errorf("Expected empty string for wrong type, got %s", userID)
		}
	})

	t.Run("GetUserRole_Valid", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("role", "moderator")

		role := GetUserRole(c)
		if role != "moderator" {
			t.Errorf("Expected 'moderator', got %s", role)
		}
	})

	t.Run("GetUserRole_WrongType", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("role", 123) // Not a string

		role := GetUserRole(c)
		if role != "" {
			t.Errorf("Expected empty string for wrong type, got %s", role)
		}
	})

	t.Run("IsAuthenticated_WithUser", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		claims := &Claims{UserID: "user123"}
		c.Set("claims", claims)

		if !IsAuthenticated(c) {
			t.Error("Expected true for authenticated user")
		}
	})

	t.Run("HasRole_UserRole", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("role", "editor")

		if HasRole(c, "editor") != true {
			t.Error("Expected true for matching role")
		}
		if HasRole(c, "admin") != false {
			t.Error("Expected false for non-matching role")
		}
	})

	t.Run("HasRole_AdminHasAllRoles", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("role", "admin")

		if !HasRole(c, "editor") {
			t.Error("Admin should have all roles")
		}
		if !HasRole(c, "moderator") {
			t.Error("Admin should have all roles")
		}
	})

	t.Run("IsAdmin_True", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("role", "admin")

		if !IsAdmin(c) {
			t.Error("Expected true for admin user")
		}
	})
}

func TestAuthMiddleware_RequireAdmin(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Create a test Gin engine
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add admin-only route
	router.GET("/admin-only", middleware.Middleware([]string{}), middleware.RequireAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin only"})
	})

	t.Run("AdminAccessGranted", func(t *testing.T) {
			t.Parallel()
		token, _ := middleware.GenerateToken("admin123", "adminuser", "admin")
		req := httptest.NewRequest("GET", "/admin-only", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("NonAdminAccessDenied", func(t *testing.T) {
			t.Parallel()
		token, _ := middleware.GenerateToken("user123", "testuser", "user")
		req := httptest.NewRequest("GET", "/admin-only", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_RequireRole_EdgeCases(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	gin.SetMode(gin.TestMode)

	t.Run("NoRoleInContext", func(t *testing.T) {
			t.Parallel()
		router := gin.New()
		router.GET("/test", middleware.RequireRole("admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})

	t.Run("InvalidRoleType", func(t *testing.T) {
			t.Parallel()
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			c.Set("role", 123) // Invalid type
			c.Next()
		}, middleware.RequireRole("admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_Optional_EdgeCases(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/optional", middleware.Optional([]string{}), func(c *gin.Context) {
		authenticated := IsAuthenticated(c)
		c.JSON(http.StatusOK, gin.H{"authenticated": authenticated})
	})

	t.Run("InvalidBearerFormat", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("GET", "/optional", nil)
		req.Header.Set("Authorization", "Basic invalid")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should proceed without auth (optional)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("GET", "/optional", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should proceed without auth (optional)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("SkipPath", func(t *testing.T) {
			t.Parallel()
		router2 := gin.New()
		router2.GET("/public/test", middleware.Optional([]string{"/public"}), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/public/test", nil)
		w := httptest.NewRecorder()
		router2.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_Middleware_InvalidToken(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", middleware.Middleware([]string{}), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "protected"})
	})

	t.Run("ExpiredToken", func(t *testing.T) {
			t.Parallel()
		// Create middleware with short expiry
		shortConfig := AuthConfig{
			SecretKey:   "test-secret-key",
			TokenExpiry: -time.Hour, // Already expired
		}
		shortMiddleware, _ := NewAuthMiddleware(shortConfig, nil)
		token, _ := shortMiddleware.GenerateToken("user123", "testuser", "user")

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("MalformedToken", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_GetAuthInfo(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	t.Run("Unauthenticated", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		info := middleware.GetAuthInfo(c)

		if info["authenticated"] != false {
			t.Error("Expected authenticated to be false")
		}
	})

	t.Run("Authenticated", func(t *testing.T) {
			t.Parallel()
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		// Generate a valid token and extract claims from it
		token, _ := middleware.GenerateToken("user123", "testuser", "admin")
		claims, _ := middleware.validateToken(token)
		c.Set("claims", claims)

		info := middleware.GetAuthInfo(c)

		if info["authenticated"] != true {
			t.Error("Expected authenticated to be true")
		}
		if info["user_id"] != "user123" {
			t.Errorf("Expected user_id 'user123', got %v", info["user_id"])
		}
		if info["username"] != "testuser" {
			t.Errorf("Expected username 'testuser', got %v", info["username"])
		}
		if info["role"] != "admin" {
			t.Errorf("Expected role 'admin', got %v", info["role"])
		}
	})
}

func TestAuthMiddleware_Logout(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey: "test-secret-key",
	}
	middleware, _ := NewAuthMiddleware(config, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/logout", middleware.Logout)

	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_Refresh(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, _ := NewAuthMiddleware(config, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/refresh", middleware.Refresh)

	t.Run("MissingToken", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("POST", "/refresh", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("POST", "/refresh", nil)
		req.Header.Set("Authorization", "Bearer invalid.token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("ValidToken", func(t *testing.T) {
			t.Parallel()
		token, _ := middleware.GenerateToken("user123", "testuser", "user")
		req := httptest.NewRequest("POST", "/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_Login(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	// Create middleware without user service (nil)
	middleware, _ := NewAuthMiddleware(config, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", middleware.Login)

	t.Run("InvalidRequestFormat", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("POST", "/login", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("InvalidCredentials_NoUserService", func(t *testing.T) {
			t.Parallel()
		body := `{"username": "testuser", "password": "testpass"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 401 since no user service is configured
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("EmptyUsername", func(t *testing.T) {
			t.Parallel()
		body := `{"username": "", "password": "testpass"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("EmptyPassword", func(t *testing.T) {
			t.Parallel()
		body := `{"username": "testuser", "password": ""}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_Register(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	// Create middleware without user service (nil)
	middleware, _ := NewAuthMiddleware(config, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", middleware.Register)

	t.Run("InvalidRequestFormat", func(t *testing.T) {
			t.Parallel()
		req := httptest.NewRequest("POST", "/register", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
			t.Parallel()
		body := `{"username": "testuser"}`
		req := httptest.NewRequest("POST", "/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestAuthMiddleware_AuthenticateUser(t *testing.T) {
	t.Parallel()
	config := AuthConfig{
		SecretKey:   "test-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, _ := NewAuthMiddleware(config, nil)

	t.Run("NoUserServiceConfigured", func(t *testing.T) {
			t.Parallel()
		_, err := middleware.authenticateUser(context.Background(), "testuser", "testpass")
		if err == nil {
			t.Error("Expected error when user service is not configured")
		}
		if err.Error() != "user service not configured" {
			t.Errorf("Expected 'user service not configured' error, got: %v", err)
		}
	})
}

// --- Benchmarks ---

func BenchmarkAuthMiddleware_GenerateToken(b *testing.B) {
	config := AuthConfig{
		SecretKey:   "benchmark-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = middleware.GenerateToken("user123", "testuser", "user")
	}
}

func BenchmarkAuthMiddleware_ValidateToken(b *testing.B) {
	config := AuthConfig{
		SecretKey:   "benchmark-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		b.Fatal(err)
	}

	token, err := middleware.GenerateToken("user123", "testuser", "user")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = middleware.validateToken(token)
	}
}

func BenchmarkAuthMiddleware_ExtractTokenFromHeader(b *testing.B) {
	config := AuthConfig{
		SecretKey: "benchmark-secret-key",
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		b.Fatal(err)
	}

	headers := []string{
		"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.token",
		"Basic dGVzdDp0ZXN0",
		"",
		"Bearer",
		"Bearer valid.token.here",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = middleware.ExtractTokenFromHeader(headers[i%len(headers)])
	}
}

func BenchmarkAuthMiddleware_RefreshToken(b *testing.B) {
	config := AuthConfig{
		SecretKey:   "benchmark-secret-key",
		TokenExpiry: time.Hour,
	}
	middleware, err := NewAuthMiddleware(config, nil)
	if err != nil {
		b.Fatal(err)
	}

	token, err := middleware.GenerateToken("user123", "testuser", "user")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = middleware.RefreshToken(token)
	}
}
