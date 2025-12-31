package services

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/superagent/superagent/internal/database"
	"github.com/superagent/superagent/internal/models"
	"golang.org/x/crypto/argon2"
)

// UserService handles user management and authentication
type UserService struct {
	db        *database.PostgresDB
	jwtSecret string
	jwtExpiry time.Duration
}

// User represents a user in the system
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Token     string    `json:"token"`
	User      User      `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewUserService creates a new user service
func NewUserService(db *database.PostgresDB, jwtSecret string, jwtExpiry time.Duration) *UserService {
	return &UserService{
		db:        db,
		jwtSecret: jwtSecret,
		jwtExpiry: jwtExpiry,
	}
}

// Register creates a new user account
func (u *UserService) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
	// Check if username already exists
	var existingID int
	err := u.db.QueryRow("SELECT id FROM users WHERE username = $1 OR email = $2", req.Username, req.Email).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("username or email already exists")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	// Hash the password
	passwordHash, err := u.hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate API key
	apiKey, err := u.generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Insert user
	var userID int
	err = u.db.QueryRow(`
		INSERT INTO users (username, email, password_hash, api_key, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'user', NOW(), NOW())
		RETURNING id
	`, req.Username, req.Email, passwordHash, apiKey).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Return user info
	user := &User{
		ID:       userID,
		Username: req.Username,
		Email:    req.Email,
		Role:     "user",
		APIKey:   apiKey,
	}

	return user, nil
}

// Authenticate validates user credentials and returns user info
func (u *UserService) Authenticate(ctx context.Context, username, password string) (*User, error) {
	var user User
	var passwordHash string

	err := u.db.QueryRow(`
		SELECT id, username, email, password_hash, api_key, role, created_at, updated_at
		FROM users
		WHERE username = $1
	`, username).Scan(
		&user.ID, &user.Username, &user.Email, &passwordHash,
		&user.APIKey, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, fmt.Errorf("failed to authenticate user: %w", err)
	}

	// Verify password
	if !u.verifyPassword(password, passwordHash) {
		return nil, fmt.Errorf("invalid username or password")
	}

	return &user, nil
}

// AuthenticateByAPIKey validates an API key and returns user info
func (u *UserService) AuthenticateByAPIKey(ctx context.Context, apiKey string) (*User, error) {
	var user User

	err := u.db.QueryRow(`
		SELECT id, username, email, api_key, role, created_at, updated_at
		FROM users
		WHERE api_key = $1
	`, apiKey).Scan(
		&user.ID, &user.Username, &user.Email, &user.APIKey,
		&user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("invalid API key")
		}
		return nil, fmt.Errorf("failed to authenticate API key: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (u *UserService) GetUserByID(ctx context.Context, userID int) (*User, error) {
	var user User

	err := u.db.QueryRow(`
		SELECT id, username, email, api_key, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.APIKey,
		&user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// UpdateUser updates user information
func (u *UserService) UpdateUser(ctx context.Context, userID int, updates map[string]interface{}) (*User, error) {
	// Build dynamic update query
	setParts := []string{}
	args := []interface{}{}
	argCount := 1

	for field, value := range updates {
		switch field {
		case "email", "role":
			setParts = append(setParts, fmt.Sprintf("%s = $%d", field, argCount))
			args = append(args, value)
			argCount++
		default:
			return nil, fmt.Errorf("field %s cannot be updated", field)
		}
	}

	if len(setParts) == 0 {
		return nil, fmt.Errorf("no valid fields to update")
	}

	setParts = append(setParts, "updated_at = NOW()")
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", strings.Join(setParts, ", "), argCount)
	args = append(args, userID)

	err := u.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Return updated user
	return u.GetUserByID(ctx, userID)
}

// ChangePassword changes a user's password
func (u *UserService) ChangePassword(ctx context.Context, userID int, oldPassword, newPassword string) error {
	// Verify old password
	var passwordHash string
	err := u.db.QueryRow("SELECT password_hash FROM users WHERE id = $1", userID).Scan(&passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return fmt.Errorf("failed to verify password: %w", err)
	}

	if !u.verifyPassword(oldPassword, passwordHash) {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	newHash, err := u.hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password
	err = u.db.Exec(`
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2
	`, newHash, userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// RegenerateAPIKey generates a new API key for a user
func (u *UserService) RegenerateAPIKey(ctx context.Context, userID int) (string, error) {
	// Generate new API key
	newAPIKey, err := u.generateAPIKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	// Update in database
	err = u.db.Exec(`
		UPDATE users
		SET api_key = $1, updated_at = NOW()
		WHERE id = $2
	`, newAPIKey, userID)
	if err != nil {
		return "", fmt.Errorf("failed to update API key: %w", err)
	}

	return newAPIKey, nil
}

// DeleteUser deletes a user account
func (u *UserService) DeleteUser(ctx context.Context, userID int) error {
	err := u.db.Exec("DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// hashPassword hashes a password using Argon2
func (u *UserService) hashPassword(password string) (string, error) {
	// Argon2 parameters
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
	saltHex := hex.EncodeToString(salt)
	hashHex := hex.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s", saltHex, hashHex), nil
}

// verifyPassword verifies a password against a hash
func (u *UserService) verifyPassword(password, hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	salt, err := hex.DecodeString(parts[4])
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(parts[5])
	if err != nil {
		return false
	}

	computedHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
}

// generateAPIKey generates a secure API key
func (u *UserService) generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(bytes), nil
}

// CreateSession creates a new user session
func (u *UserService) CreateSession(ctx context.Context, userID int, metadata map[string]interface{}) (*models.UserSession, error) {
	sessionToken, err := u.generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	expiresAt := time.Now().Add(u.jwtExpiry)

	var sessionID string
	err = u.db.QueryRow(`
		INSERT INTO user_sessions (user_id, session_token, expires_at, metadata, created_at, last_activity)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id
	`, userID, sessionToken, expiresAt, metadata).Scan(&sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session := &models.UserSession{
		ID:           sessionID,
		UserID:       fmt.Sprintf("%d", userID),
		SessionToken: sessionToken,
		Context:      metadata,
		ExpiresAt:    expiresAt,
		LastActivity: time.Now(),
		CreatedAt:    time.Now(),
	}

	return session, nil
}

// GetSession retrieves a session by token
func (u *UserService) GetSession(ctx context.Context, token string) (*models.UserSession, error) {
	var session models.UserSession

	err := u.db.QueryRow(`
		SELECT id, user_id, session_token, expires_at, metadata, created_at, last_activity
		FROM user_sessions
		WHERE session_token = $1 AND expires_at > NOW()
	`, token).Scan(
		&session.ID, &session.UserID, &session.SessionToken,
		&session.ExpiresAt, &session.Context, &session.CreatedAt, &session.LastActivity,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found or expired")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Update last activity
	u.db.Exec("UPDATE user_sessions SET last_activity = NOW() WHERE id = $1", session.ID)

	return &session, nil
}

// DeleteSession deletes a session
func (u *UserService) DeleteSession(ctx context.Context, sessionID int) error {
	err := u.db.Exec("DELETE FROM user_sessions WHERE id = $1", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes expired sessions
func (u *UserService) CleanupExpiredSessions(ctx context.Context) error {
	err := u.db.Exec("DELETE FROM user_sessions WHERE expires_at < NOW()")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	return nil
}
