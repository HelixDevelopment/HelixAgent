package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUserService(t *testing.T) {
	service := NewUserService(nil, "test-secret", 24*time.Hour)

	require.NotNil(t, service)
	assert.Nil(t, service.db)
	assert.Equal(t, "test-secret", service.jwtSecret)
	assert.Equal(t, 24*time.Hour, service.jwtExpiry)
}

func TestUserService_HashPassword(t *testing.T) {
	service := NewUserService(nil, "test-secret", 24*time.Hour)

	t.Run("hash generates valid format", func(t *testing.T) {
		hash, err := service.hashPassword("testpassword123")

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Contains(t, hash, "$argon2id$")
		assert.Contains(t, hash, "$v=19$")
		assert.Contains(t, hash, "$m=65536,t=1,p=4$")
	})

	t.Run("different passwords produce different hashes", func(t *testing.T) {
		hash1, err1 := service.hashPassword("password1")
		hash2, err2 := service.hashPassword("password2")

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("same password produces different hashes (random salt)", func(t *testing.T) {
		hash1, err1 := service.hashPassword("samepassword")
		hash2, err2 := service.hashPassword("samepassword")

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2) // Different salts
	})

	t.Run("empty password can be hashed", func(t *testing.T) {
		hash, err := service.hashPassword("")

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("long password can be hashed", func(t *testing.T) {
		longPassword := "a" + string(make([]byte, 1000))
		hash, err := service.hashPassword(longPassword)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})
}

func TestUserService_VerifyPassword(t *testing.T) {
	service := NewUserService(nil, "test-secret", 24*time.Hour)

	t.Run("correct password verifies", func(t *testing.T) {
		password := "correctpassword123"
		hash, err := service.hashPassword(password)
		require.NoError(t, err)

		result := service.verifyPassword(password, hash)
		assert.True(t, result)
	})

	t.Run("incorrect password fails", func(t *testing.T) {
		password := "correctpassword123"
		hash, err := service.hashPassword(password)
		require.NoError(t, err)

		result := service.verifyPassword("wrongpassword", hash)
		assert.False(t, result)
	})

	t.Run("empty password verification", func(t *testing.T) {
		hash, err := service.hashPassword("")
		require.NoError(t, err)

		result := service.verifyPassword("", hash)
		assert.True(t, result)

		result = service.verifyPassword("notempty", hash)
		assert.False(t, result)
	})

	t.Run("invalid hash format", func(t *testing.T) {
		result := service.verifyPassword("anypassword", "invalid-hash-format")
		assert.False(t, result)
	})

	t.Run("wrong algorithm prefix", func(t *testing.T) {
		result := service.verifyPassword("anypassword", "$bcrypt$invalid$hash$format$here")
		assert.False(t, result)
	})

	t.Run("invalid salt hex", func(t *testing.T) {
		result := service.verifyPassword("anypassword", "$argon2id$v=19$m=65536,t=1,p=4$invalid-salt$abc123")
		assert.False(t, result)
	})

	t.Run("invalid hash hex", func(t *testing.T) {
		// Valid salt but invalid hash
		result := service.verifyPassword("anypassword", "$argon2id$v=19$m=65536,t=1,p=4$0123456789abcdef0123456789abcdef$invalid-hash")
		assert.False(t, result)
	})
}

func TestUserService_GenerateAPIKey(t *testing.T) {
	service := NewUserService(nil, "test-secret", 24*time.Hour)

	t.Run("generates valid API key format", func(t *testing.T) {
		apiKey, err := service.generateAPIKey()

		require.NoError(t, err)
		assert.NotEmpty(t, apiKey)
		assert.True(t, len(apiKey) > 3)
		assert.Equal(t, "sk-", apiKey[:3])
	})

	t.Run("generates unique keys", func(t *testing.T) {
		keys := make(map[string]bool)
		for i := 0; i < 100; i++ {
			key, err := service.generateAPIKey()
			require.NoError(t, err)
			assert.False(t, keys[key], "Duplicate key generated")
			keys[key] = true
		}
	})

	t.Run("key has correct length", func(t *testing.T) {
		apiKey, err := service.generateAPIKey()

		require.NoError(t, err)
		// 3 chars for "sk-" + 64 chars for hex-encoded 32 bytes
		assert.Equal(t, 67, len(apiKey))
	})
}

func TestUser_Struct(t *testing.T) {
	now := time.Now()
	user := User{
		ID:        1,
		Username:  "testuser",
		Email:     "test@example.com",
		Role:      "admin",
		APIKey:    "sk-abc123",
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, 1, user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "admin", user.Role)
	assert.Equal(t, "sk-abc123", user.APIKey)
	assert.Equal(t, now, user.CreatedAt)
	assert.Equal(t, now, user.UpdatedAt)
}

func TestLoginRequest_Struct(t *testing.T) {
	req := LoginRequest{
		Username: "testuser",
		Password: "testpassword",
	}

	assert.Equal(t, "testuser", req.Username)
	assert.Equal(t, "testpassword", req.Password)
}

func TestRegisterRequest_Struct(t *testing.T) {
	req := RegisterRequest{
		Username: "newuser",
		Email:    "new@example.com",
		Password: "securepassword123",
	}

	assert.Equal(t, "newuser", req.Username)
	assert.Equal(t, "new@example.com", req.Email)
	assert.Equal(t, "securepassword123", req.Password)
}

func TestAuthResponse_Struct(t *testing.T) {
	now := time.Now()
	resp := AuthResponse{
		Token: "jwt-token-here",
		User: User{
			ID:       1,
			Username: "testuser",
		},
		ExpiresAt: now.Add(24 * time.Hour),
	}

	assert.Equal(t, "jwt-token-here", resp.Token)
	assert.Equal(t, 1, resp.User.ID)
	assert.True(t, resp.ExpiresAt.After(now))
}

func TestUserService_PasswordHashingRoundTrip(t *testing.T) {
	service := NewUserService(nil, "test-secret", 24*time.Hour)

	testCases := []string{
		"simple",
		"Complex123!@#",
		"with spaces in password",
		"unicode: 日本語テスト",
		"very-long-password-" + string(make([]byte, 100)),
	}

	for _, password := range testCases {
		t.Run(password[:minInt(len(password), 20)], func(t *testing.T) {
			hash, err := service.hashPassword(password)
			require.NoError(t, err)

			assert.True(t, service.verifyPassword(password, hash))
			assert.False(t, service.verifyPassword(password+"x", hash))
		})
	}
}

func TestUserService_ConcurrentAPIKeyGeneration(t *testing.T) {
	service := NewUserService(nil, "test-secret", 24*time.Hour)

	keys := make(chan string, 100)
	done := make(chan bool)

	// Generate 100 keys concurrently
	for i := 0; i < 100; i++ {
		go func() {
			key, err := service.generateAPIKey()
			require.NoError(t, err)
			keys <- key
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
	close(keys)

	// Check all keys are unique
	keyMap := make(map[string]bool)
	for key := range keys {
		assert.False(t, keyMap[key], "Duplicate key found")
		keyMap[key] = true
	}
	assert.Equal(t, 100, len(keyMap))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
