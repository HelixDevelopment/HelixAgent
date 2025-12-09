package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware handles JWT authentication
type AuthMiddleware struct {
	secretKey   string
	tokenExpiry time.Duration
	issuer      string
}

// Claims represents the JWT claims structure
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	SecretKey   string        `json:"secret_key"`
	TokenExpiry time.Duration `json:"token_expiry"`
	Issuer      string        `json:"issuer"`
	SkipPaths   []string      `json:"skip_paths"`
	Required    bool          `json:"required"`
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(config AuthConfig) *AuthMiddleware {
	if config.SecretKey == "" {
		config.SecretKey = "default-secret-key-change-in-production"
	}
	if config.TokenExpiry == 0 {
		config.TokenExpiry = 24 * time.Hour
	}
	if config.Issuer == "" {
		config.Issuer = "superagent"
	}

	return &AuthMiddleware{
		secretKey:   config.SecretKey,
		tokenExpiry: config.TokenExpiry,
		issuer:      config.Issuer,
	}
}

// Middleware returns a Gin middleware function
func (a *AuthMiddleware) Middleware(skipPaths []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if path should be skipped
		path := c.Request.URL.Path
		for _, skipPath := range skipPaths {
			if strings.HasPrefix(path, skipPath) {
				c.Next()
				return
			}
		}

		// Get token from header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			a.unauthorized(c, "Missing authorization header")
			return
		}

		// Extract Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			a.unauthorized(c, "Invalid authorization header format")
			return
		}

		token := tokenParts[1]

		// Validate token
		claims, err := a.validateToken(token)
		if err != nil {
			a.unauthorized(c, "Invalid token: "+err.Error())
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// Optional returns a middleware that can be optionally applied
func (a *AuthMiddleware) Optional(skipPaths []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if path should be skipped
		path := c.Request.URL.Path
		for _, skipPath := range skipPaths {
			if strings.HasPrefix(path, skipPath) {
				c.Next()
				return
			}
		}

		// Get token from header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No auth header, continue without authentication
			c.Next()
			return
		}

		// Extract Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			// Invalid format, continue without authentication
			c.Next()
			return
		}

		token := tokenParts[1]

		// Validate token
		claims, err := a.validateToken(token)
		if err != nil {
			// Invalid token, continue without authentication
			c.Next()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequireRole returns a middleware that requires a specific role
func (a *AuthMiddleware) RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user role from context
		roleValue, exists := c.Get("role")
		if !exists {
			a.forbidden(c, "User role not found")
			return
		}

		role, ok := roleValue.(string)
		if !ok {
			a.forbidden(c, "Invalid user role format")
			return
		}

		// Check if user has required role
		if role != requiredRole && role != "admin" {
			a.forbidden(c, "Insufficient permissions")
			return
		}

		c.Next()
	}
}

// RequireAdmin returns a middleware that requires admin role
func (a *AuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return a.RequireRole("admin")
}

// GenerateToken creates a new JWT token for a user
func (a *AuthMiddleware) GenerateToken(userID, username, role string) (string, error) {
	// Create claims
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    a.issuer,
			Subject:   userID,
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.secretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns claims
func (a *AuthMiddleware) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(a.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	// Validate token claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrInvalidKey
}

// RefreshToken generates a new token for an existing valid token
func (a *AuthMiddleware) RefreshToken(tokenString string) (string, error) {
	claims, err := a.validateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Generate new token with same claims
	return a.GenerateToken(claims.UserID, claims.Username, claims.Role)
}

// ExtractTokenFromHeader extracts token from Authorization header
func (a *AuthMiddleware) ExtractTokenFromHeader(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		return ""
	}

	return tokenParts[1]
}

// Helper methods for HTTP responses

func (a *AuthMiddleware) unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"error":     "unauthorized",
		"message":   message,
		"timestamp": time.Now().Unix(),
	})
	c.Abort()
}

func (a *AuthMiddleware) forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, gin.H{
		"error":     "forbidden",
		"message":   message,
		"timestamp": time.Now().Unix(),
	})
	c.Abort()
}

// GetCurrentUser returns the current user from context
func GetCurrentUser(c *gin.Context) *Claims {
	if claims, exists := c.Get("claims"); exists {
		if userClaims, ok := claims.(*Claims); ok {
			return userClaims
		}
	}
	return nil
}

// GetUserID returns the current user ID from context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(string); ok {
			return uid
		}
	}
	return ""
}

// GetUserRole returns the current user role from context
func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get("role"); exists {
		if r, ok := role.(string); ok {
			return r
		}
	}
	return ""
}

// IsAuthenticated checks if the current request is authenticated
func IsAuthenticated(c *gin.Context) bool {
	return GetCurrentUser(c) != nil
}

// HasRole checks if the current user has the specified role
func HasRole(c *gin.Context, role string) bool {
	return GetUserRole(c) == role || GetUserRole(c) == "admin"
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *gin.Context) bool {
	return HasRole(c, "admin")
}

// Authentication handlers for login/logout

// LoginRequest represents login request payload
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Token     string   `json:"token"`
	ExpiresIn int      `json:"expires_in"`
	User      UserInfo `json:"user"`
}

// UserInfo represents user information returned on login
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Login handles user authentication
func (a *AuthMiddleware) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
		return
	}

	// TODO: Implement actual user authentication against database
	// For now, accept any username/password and create a token
	// In production, you would validate against a user database

	// Mock user validation - replace with real authentication
	if !a.authenticateUser(req.Username, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_credentials",
			"message": "Invalid username or password",
		})
		return
	}

	// Generate token
	token, err := a.GenerateToken("user123", req.Username, "user")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "token_generation_failed",
			"message": "Failed to generate token",
		})
		return
	}

	// Return token
	response := LoginResponse{
		Token:     token,
		ExpiresIn: int(a.tokenExpiry.Seconds()),
		User: UserInfo{
			ID:       "user123",
			Username: req.Username,
			Role:     "user",
		},
	}

	c.JSON(http.StatusOK, response)
}

// Logout handles user logout
func (a *AuthMiddleware) Logout(c *gin.Context) {
	// For JWT tokens, logout is typically handled client-side
	// by simply discarding the token
	// We can implement token blacklisting if needed

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully logged out",
	})
}

// Refresh handles token refresh
func (a *AuthMiddleware) Refresh(c *gin.Context) {
	// Get current token from header
	authHeader := c.GetHeader("Authorization")
	token := a.ExtractTokenFromHeader(authHeader)

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "missing_token",
			"message": "No token provided for refresh",
		})
		return
	}

	// Validate and refresh token
	newToken, err := a.RefreshToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_token",
			"message": "Invalid or expired token",
		})
		return
	}

	// Return new token
	response := gin.H{
		"token":      newToken,
		"expires_in": int(a.tokenExpiry.Seconds()),
	}

	c.JSON(http.StatusOK, response)
}

// GetAuthInfo returns authentication information for the current user
func (a *AuthMiddleware) GetAuthInfo(c *gin.Context) gin.H {
	claims := GetCurrentUser(c)
	if claims == nil {
		return gin.H{
			"authenticated": false,
		}
	}

	return gin.H{
		"authenticated": true,
		"user_id":       claims.UserID,
		"username":      claims.Username,
		"role":          claims.Role,
		"expires_at":    claims.ExpiresAt.Unix(),
		"issued_at":     claims.IssuedAt.Unix(),
	}
}

// authenticateUser validates user credentials (mock implementation)
func (a *AuthMiddleware) authenticateUser(username, password string) bool {
	// TODO: Implement real authentication against user database
	// For demo purposes, accept any non-empty credentials
	return username != "" && password != ""
}
