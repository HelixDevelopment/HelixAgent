package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Silence unused import warning for uuid
var _ = uuid.New

// =============================================================================
// Test Helper Functions for User Repository
// =============================================================================

func setupUserTestDB(t *testing.T) (*pgxpool.Pool, *UserRepository) {
	ctx := context.Background()

	// Build connection string from environment variables
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "helixagent"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "secret"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "helixagent_db"
	}
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v (SKIP-OK: #infra-db-unavailable)", err)
		return nil, nil
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	repo := NewUserRepository(pool, logger)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v (SKIP-OK: #infra-db-unavailable)", err)
		pool.Close()
		return nil, nil
	}

	// Check if required tables exist
	var tableExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'users'
		)
	`).Scan(&tableExists)
	if err != nil || !tableExists {
		t.Skipf("Skipping test: users table does not exist (run migrations first) (SKIP-OK: #infra-schema-not-migrated)")
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupUserTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM users WHERE username LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup users: %v", err)
	}
}

func createTestUser() *User {
	timestamp := time.Now().Format("20060102150405.000000")
	return &User{
		Username:     "test-user-" + timestamp,
		Email:        "test-" + timestamp + "@example.com",
		PasswordHash: "$2a$10$testHashedPassword12345678901234567890",
		APIKey:       "sk-test-" + timestamp,
		Role:         "user",
	}
}

// =============================================================================
// Integration Tests (Require Database)
// =============================================================================

func TestUserRepository_Create(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		assert.NoError(t, err)
		assert.NotEmpty(t, user.ID)
		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())
	})

	t.Run("DifferentRoles", func(t *testing.T) {
		roles := []string{"user", "admin", "moderator"}
		for _, role := range roles {
			user := createTestUser()
			user.Username = "test-role-" + role + "-" + time.Now().Format("20060102150405.000000")
			user.Email = "test-role-" + role + "-" + time.Now().Format("20060102150405.000000") + "@example.com"
			user.Role = role
			err := repo.Create(ctx, user)
			assert.NoError(t, err)
			assert.NotEmpty(t, user.ID)
		}
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)
		assert.Equal(t, user.Username, fetched.Username)
		assert.Equal(t, user.Email, fetched.Email)
		assert.Equal(t, user.PasswordHash, fetched.PasswordHash)
		assert.Equal(t, user.APIKey, fetched.APIKey)
		assert.Equal(t, user.Role, fetched.Role)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_GetByEmail(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		fetched, err := repo.GetByEmail(ctx, user.Email)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)
		assert.Equal(t, user.Email, fetched.Email)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_GetByUsername(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		fetched, err := repo.GetByUsername(ctx, user.Username)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)
		assert.Equal(t, user.Username, fetched.Username)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByUsername(ctx, "nonexistent-user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_GetByAPIKey(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		fetched, err := repo.GetByAPIKey(ctx, user.APIKey)
		assert.NoError(t, err)
		assert.Equal(t, user.ID, fetched.ID)
		assert.Equal(t, user.APIKey, fetched.APIKey)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByAPIKey(ctx, "sk-nonexistent-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_Update(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		user.Username = "test-updated-" + time.Now().Format("20060102150405.000000")
		user.Email = "test-updated-" + time.Now().Format("20060102150405.000000") + "@example.com"
		user.Role = "admin"

		err = repo.Update(ctx, user)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, user.Username, fetched.Username)
		assert.Equal(t, user.Email, fetched.Email)
		assert.Equal(t, "admin", fetched.Role)
	})

	t.Run("NotFound", func(t *testing.T) {
		user := createTestUser()
		user.ID = "00000000-0000-0000-0000-000000000000"
		err := repo.Update(ctx, user)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_Delete(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		err = repo.Delete(ctx, user.ID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, user.ID)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_List(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create test users
		for i := 0; i < 3; i++ {
			user := createTestUser()
			user.Username = "test-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			user.Email = "test-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i)) + "@example.com"
			err := repo.Create(ctx, user)
			require.NoError(t, err)
		}

		users, total, err := repo.List(ctx, 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, 3)
		assert.GreaterOrEqual(t, len(users), 3)
	})

	t.Run("Pagination", func(t *testing.T) {
		users, _, err := repo.List(ctx, 2, 0)
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(users), 2)
	})
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		newPasswordHash := "$2a$10$newHashedPassword12345678901234567890"
		err = repo.UpdatePassword(ctx, user.ID, newPasswordHash)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, newPasswordHash, fetched.PasswordHash)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.UpdatePassword(ctx, "00000000-0000-0000-0000-000000000000", "hash")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_RegenerateAPIKey(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		newAPIKey := "sk-new-api-key-" + time.Now().Format("20060102150405.000000")
		err = repo.RegenerateAPIKey(ctx, user.ID, newAPIKey)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, newAPIKey, fetched.APIKey)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.RegenerateAPIKey(ctx, "00000000-0000-0000-0000-000000000000", "new-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUserRepository_ExistsByEmail(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Exists", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		exists, err := repo.ExistsByEmail(ctx, user.Email)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("NotExists", func(t *testing.T) {
		exists, err := repo.ExistsByEmail(ctx, "nonexistent@example.com")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestUserRepository_ExistsByUsername(t *testing.T) {
	pool, repo := setupUserTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupUserTestDB(t, pool)

	ctx := context.Background()

	t.Run("Exists", func(t *testing.T) {
		user := createTestUser()
		err := repo.Create(ctx, user)
		require.NoError(t, err)

		exists, err := repo.ExistsByUsername(ctx, user.Username)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("NotExists", func(t *testing.T) {
		exists, err := repo.ExistsByUsername(ctx, "nonexistent-username")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

// =============================================================================
// Unit Tests (No Database Required)
// =============================================================================

func TestNewUserRepository(t *testing.T) {
	t.Run("CreatesRepositoryWithNilPool", func(t *testing.T) {
		logger := logrus.New()
		repo := NewUserRepository(nil, logger)
		assert.NotNil(t, repo)
	})

	t.Run("CreatesRepositoryWithNilLogger", func(t *testing.T) {
		repo := NewUserRepository(nil, nil)
		assert.NotNil(t, repo)
	})
}

func TestUser_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullUser", func(t *testing.T) {
		user := &User{
			ID:           "user-1",
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "$2a$10$hashedpassword",
			APIKey:       "sk-test-key",
			Role:         "admin",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		jsonBytes, err := json.Marshal(user)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "testuser")
		assert.Contains(t, string(jsonBytes), "test@example.com")
		// Password hash should be omitted due to json:"-"
		assert.NotContains(t, string(jsonBytes), "password_hash")
		assert.NotContains(t, string(jsonBytes), "$2a$10$hashedpassword")

		var decoded User
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, user.Username, decoded.Username)
	})

	t.Run("SerializesMinimalUser", func(t *testing.T) {
		user := &User{
			Username: "minimal",
			Email:    "minimal@example.com",
		}

		jsonBytes, err := json.Marshal(user)
		require.NoError(t, err)

		var decoded User
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "minimal", decoded.Username)
	})
}

func TestUser_Fields(t *testing.T) {
	t.Run("AllFieldsSet", func(t *testing.T) {
		now := time.Now()
		user := &User{
			ID:           "id-1",
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hash",
			APIKey:       "api-key",
			Role:         "admin",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		assert.Equal(t, "id-1", user.ID)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Equal(t, "hash", user.PasswordHash)
		assert.Equal(t, "api-key", user.APIKey)
		assert.Equal(t, "admin", user.Role)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		user := &User{}
		assert.Empty(t, user.ID)
		assert.Empty(t, user.Username)
		assert.Empty(t, user.Email)
		assert.Empty(t, user.PasswordHash)
		assert.Empty(t, user.APIKey)
		assert.Empty(t, user.Role)
	})
}

func TestUser_RoleValues(t *testing.T) {
	roles := []string{"user", "admin", "moderator", "superadmin", "readonly", ""}

	for _, role := range roles {
		t.Run("Role_"+role, func(t *testing.T) {
			user := &User{
				Role: role,
			}
			assert.Equal(t, role, user.Role)
		})
	}
}

func TestUser_EmailFormats(t *testing.T) {
	emails := []string{
		"simple@example.com",
		"user.name@example.com",
		"user+tag@example.com",
		"user@subdomain.example.com",
		"user@example.co.uk",
	}

	for _, email := range emails {
		t.Run("Email", func(t *testing.T) {
			user := &User{
				Email: email,
			}
			assert.Equal(t, email, user.Email)
		})
	}
}

func TestUser_APIKeyFormats(t *testing.T) {
	apiKeys := []string{
		"sk-1234567890abcdef",
		"api-key-with-dashes",
		"API_KEY_WITH_UNDERSCORES",
		"mixedCaseApiKey123",
	}

	for _, apiKey := range apiKeys {
		t.Run("APIKey", func(t *testing.T) {
			user := &User{
				APIKey: apiKey,
			}
			assert.Equal(t, apiKey, user.APIKey)
		})
	}
}

func TestUser_PasswordHashFormats(t *testing.T) {
	t.Run("BcryptFormat", func(t *testing.T) {
		user := &User{
			PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		}
		assert.True(t, len(user.PasswordHash) > 50)
		assert.Contains(t, user.PasswordHash, "$2a$")
	})

	t.Run("Argon2Format", func(t *testing.T) {
		user := &User{
			PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQ$RdescudvJCsgt3ub+b+dWRWJTmaaJObG",
		}
		assert.Contains(t, user.PasswordHash, "$argon2id$")
	})
}

func TestUser_JSONRoundTrip(t *testing.T) {
	t.Run("FullRoundTrip", func(t *testing.T) {
		now := time.Now().Truncate(time.Second)
		original := &User{
			ID:           "round-trip-id",
			Username:     "round-trip-user",
			Email:        "roundtrip@example.com",
			PasswordHash: "hashed-password",
			APIKey:       "sk-round-trip-key",
			Role:         "admin",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		jsonBytes, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded User
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.ID, decoded.ID)
		assert.Equal(t, original.Username, decoded.Username)
		assert.Equal(t, original.Email, decoded.Email)
		assert.Equal(t, original.APIKey, decoded.APIKey)
		assert.Equal(t, original.Role, decoded.Role)
		// PasswordHash is omitted from JSON
		assert.Empty(t, decoded.PasswordHash)
	})
}

func TestUser_EdgeCases(t *testing.T) {
	t.Run("VeryLongUsername", func(t *testing.T) {
		longUsername := ""
		for i := 0; i < 256; i++ {
			longUsername += "a"
		}
		user := &User{
			Username: longUsername,
		}
		assert.Len(t, user.Username, 256)
	})

	t.Run("VeryLongEmail", func(t *testing.T) {
		longLocal := ""
		for i := 0; i < 64; i++ {
			longLocal += "a"
		}
		longEmail := longLocal + "@example.com"
		user := &User{
			Email: longEmail,
		}
		assert.Contains(t, user.Email, "@example.com")
	})

	t.Run("SpecialCharactersInUsername", func(t *testing.T) {
		// Some systems allow special characters in usernames
		user := &User{
			Username: "user_name-123",
		}
		assert.Contains(t, user.Username, "_")
		assert.Contains(t, user.Username, "-")
	})

	t.Run("EmptyStrings", func(t *testing.T) {
		user := &User{
			Username:     "",
			Email:        "",
			PasswordHash: "",
			APIKey:       "",
			Role:         "",
		}
		assert.Empty(t, user.Username)
		assert.Empty(t, user.Email)
		assert.Empty(t, user.PasswordHash)
		assert.Empty(t, user.APIKey)
		assert.Empty(t, user.Role)
	})

	t.Run("CaseSensitiveEmail", func(t *testing.T) {
		user1 := &User{Email: "Test@Example.com"}
		user2 := &User{Email: "test@example.com"}
		// Note: These are different in the struct, but typically should be treated as same in practice
		assert.NotEqual(t, user1.Email, user2.Email)
	})
}

func TestUser_JSONKeys(t *testing.T) {
	user := &User{
		ID:       "id",
		Username: "username",
		Email:    "email@example.com",
		APIKey:   "api-key",
		Role:     "user",
	}

	jsonBytes, err := json.Marshal(user)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	expectedKeys := []string{
		"\"id\":", "\"username\":", "\"email\":", "\"api_key\":", "\"role\":",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, jsonStr, key, "JSON should contain key: "+key)
	}

	// Password hash should NOT be in JSON output
	assert.NotContains(t, jsonStr, "password_hash")
}

func TestCreateTestUser_Helper(t *testing.T) {
	user := createTestUser()

	t.Run("HasRequiredFields", func(t *testing.T) {
		assert.NotEmpty(t, user.Username)
		assert.NotEmpty(t, user.Email)
		assert.NotEmpty(t, user.PasswordHash)
		assert.NotEmpty(t, user.APIKey)
		assert.NotEmpty(t, user.Role)
	})

	t.Run("HasDefaultValues", func(t *testing.T) {
		assert.Equal(t, "user", user.Role)
		assert.Contains(t, user.Username, "test-")
		assert.Contains(t, user.Email, "@example.com")
		assert.Contains(t, user.APIKey, "sk-test-")
	})

	t.Run("HasProperPasswordHashFormat", func(t *testing.T) {
		assert.Contains(t, user.PasswordHash, "$2a$10$")
	})
}

func TestUser_TimeFields(t *testing.T) {
	t.Run("AllTimeFieldsSet", func(t *testing.T) {
		now := time.Now()
		earlier := now.Add(-24 * time.Hour)
		user := &User{
			CreatedAt: earlier,
			UpdatedAt: now,
		}

		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())
		assert.True(t, user.UpdatedAt.After(user.CreatedAt))
	})

	t.Run("ZeroTimeFields", func(t *testing.T) {
		user := &User{}
		assert.True(t, user.CreatedAt.IsZero())
		assert.True(t, user.UpdatedAt.IsZero())
	})
}

func TestUser_UniqueConstraints(t *testing.T) {
	t.Run("UniquenessFields", func(t *testing.T) {
		// Test that unique fields can be set
		user := &User{
			Username: "unique-user",
			Email:    "unique@example.com",
			APIKey:   "sk-unique-key",
		}
		assert.Equal(t, "unique-user", user.Username)
		assert.Equal(t, "unique@example.com", user.Email)
		assert.Equal(t, "sk-unique-key", user.APIKey)
	})
}
