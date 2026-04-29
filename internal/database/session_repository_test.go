package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helper Functions for Session Repository
// =============================================================================

func setupSessionTestDB(t *testing.T) (*pgxpool.Pool, *SessionRepository) {
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
	repo := NewSessionRepository(pool, logger)

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
			AND table_name = 'user_sessions'
		)
	`).Scan(&tableExists)
	if err != nil || !tableExists {
		t.Skipf("Skipping test: user_sessions table does not exist (run migrations first) (SKIP-OK: #infra-schema-not-migrated)")
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupSessionTestDB(t *testing.T, pool *pgxpool.Pool, testUserID string) {
	ctx := context.Background()
	// Delete sessions for the test user
	if testUserID != "" {
		_, err := pool.Exec(ctx, "DELETE FROM user_sessions WHERE user_id = $1", testUserID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup user_sessions: %v", err)
		}
		// Delete the test user
		_, err = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", testUserID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup users: %v", err)
		}
	}
}

// createTestUserForSession creates a user record and returns the user ID
func createTestUserForSession(t *testing.T, pool *pgxpool.Pool) string {
	ctx := context.Background()
	timestamp := time.Now().Format("20060102150405.000000")
	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, api_key, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, "test-session-user-"+timestamp, "test-session-"+timestamp+"@example.com",
		"$2a$10$testHashedPassword12345678901234567890", "sk-test-session-"+timestamp, "user").Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return userID
}

func createTestUserSession(userID string) *UserSession {
	return &UserSession{
		UserID:       userID, // Use the provided valid user ID
		SessionToken: "token-" + time.Now().Format("20060102150405.000000"),
		Context:      map[string]interface{}{"theme": "dark", "language": "en"},
		MemoryID:     nil, // Set to nil to avoid memory_id FK issues if any
		Status:       "active",
		RequestCount: 0,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
}

// =============================================================================
// Integration Tests (Require Database)
// =============================================================================

func TestSessionRepository_Create(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	// Create a test user first (required for FK constraint)
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		err := repo.Create(ctx, session)
		assert.NoError(t, err)
		assert.NotEmpty(t, session.ID)
		assert.False(t, session.CreatedAt.IsZero())
		assert.False(t, session.LastActivity.IsZero())
	})

	t.Run("WithNilMemoryID", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.MemoryID = nil
		session.SessionToken = "token-nil-memory-" + time.Now().Format("20060102150405.000000")
		err := repo.Create(ctx, session)
		assert.NoError(t, err)
		assert.NotEmpty(t, session.ID)
	})

	t.Run("WithNilContext", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.Context = nil
		session.SessionToken = "token-nil-context-" + time.Now().Format("20060102150405.000000")
		err := repo.Create(ctx, session)
		assert.NoError(t, err)
		assert.NotEmpty(t, session.ID)
	})
}

func TestSessionRepository_GetByID(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, session.ID)
		assert.NoError(t, err)
		assert.Equal(t, session.ID, fetched.ID)
		assert.Equal(t, session.UserID, fetched.UserID)
		assert.Equal(t, session.SessionToken, fetched.SessionToken)
		assert.Equal(t, session.Status, fetched.Status)
		assert.Equal(t, session.RequestCount, fetched.RequestCount)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSessionRepository_GetByToken(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		fetched, err := repo.GetByToken(ctx, session.SessionToken)
		assert.NoError(t, err)
		assert.Equal(t, session.ID, fetched.ID)
		assert.Equal(t, session.SessionToken, fetched.SessionToken)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByToken(ctx, "non-existent-token-12345")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSessionRepository_GetByUserID(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create multiple sessions for the same test user
		for i := 0; i < 3; i++ {
			session := createTestUserSession(testUserID)
			session.SessionToken = "token-multi-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			err := repo.Create(ctx, session)
			require.NoError(t, err)
		}

		sessions, err := repo.GetByUserID(ctx, testUserID)
		assert.NoError(t, err)
		assert.Len(t, sessions, 3)
	})

	t.Run("EmptyResult", func(t *testing.T) {
		sessions, err := repo.GetByUserID(ctx, "00000000-0000-0000-0000-000000000002")
		assert.NoError(t, err)
		assert.Len(t, sessions, 0)
	})
}

func TestSessionRepository_GetActiveSessions(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create active session
		activeSession := createTestUserSession(testUserID)
		activeSession.SessionToken = "token-active-" + time.Now().Format("20060102150405.000")
		activeSession.Status = "active"
		activeSession.ExpiresAt = time.Now().Add(24 * time.Hour)
		err := repo.Create(ctx, activeSession)
		require.NoError(t, err)

		// Create terminated session
		terminatedSession := createTestUserSession(testUserID)
		terminatedSession.SessionToken = "token-terminated-" + time.Now().Format("20060102150405.001")
		terminatedSession.Status = "terminated"
		terminatedSession.ExpiresAt = time.Now().Add(24 * time.Hour)
		err = repo.Create(ctx, terminatedSession)
		require.NoError(t, err)

		// Create expired session
		expiredSession := createTestUserSession(testUserID)
		expiredSession.SessionToken = "token-expired-" + time.Now().Format("20060102150405.002")
		expiredSession.Status = "active"
		expiredSession.ExpiresAt = time.Now().Add(-24 * time.Hour)
		err = repo.Create(ctx, expiredSession)
		require.NoError(t, err)

		sessions, err := repo.GetActiveSessions(ctx, testUserID)
		assert.NoError(t, err)
		assert.Len(t, sessions, 1)
		assert.Equal(t, "active", sessions[0].Status)
	})
}

func TestSessionRepository_Update(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		session.Context = map[string]interface{}{"theme": "light", "language": "fr"}
		session.Status = "suspended"
		session.RequestCount = 10
		// MemoryID must be a valid UUID since the column is UUID type
		newMemoryID := "11111111-1111-1111-1111-111111111111"
		session.MemoryID = &newMemoryID
		session.ExpiresAt = time.Now().Add(48 * time.Hour)

		err = repo.Update(ctx, session)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, session.ID)
		assert.NoError(t, err)
		assert.Equal(t, "suspended", fetched.Status)
		assert.Equal(t, 10, fetched.RequestCount)
		assert.Equal(t, "11111111-1111-1111-1111-111111111111", *fetched.MemoryID)
	})

	t.Run("NotFound", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.ID = "00000000-0000-0000-0000-000000000000"
		err := repo.Update(ctx, session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSessionRepository_UpdateActivity(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.RequestCount = 5
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		err = repo.UpdateActivity(ctx, session.ID)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, session.ID)
		assert.NoError(t, err)
		assert.Equal(t, 6, fetched.RequestCount)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.UpdateActivity(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSessionRepository_UpdateContext(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		newContext := map[string]interface{}{"theme": "light", "language": "es", "newKey": "newValue"}
		err = repo.UpdateContext(ctx, session.ID, newContext)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, session.ID)
		assert.NoError(t, err)
		assert.Equal(t, "light", fetched.Context["theme"])
		assert.Equal(t, "es", fetched.Context["language"])
		assert.Equal(t, "newValue", fetched.Context["newKey"])
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.UpdateContext(ctx, "00000000-0000-0000-0000-000000000000", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSessionRepository_Terminate(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.Status = "active"
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		err = repo.Terminate(ctx, session.ID)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, session.ID)
		assert.NoError(t, err)
		assert.Equal(t, "terminated", fetched.Status)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.Terminate(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSessionRepository_Delete(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		err = repo.Delete(ctx, session.ID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, session.ID)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create expired session
		expiredSession := createTestUserSession(testUserID)
		expiredSession.ExpiresAt = time.Now().Add(-1 * time.Hour)
		err := repo.Create(ctx, expiredSession)
		require.NoError(t, err)

		// Create non-expired session
		activeSession := createTestUserSession(testUserID)
		activeSession.SessionToken = "token-non-expired-" + time.Now().Format("20060102150405.000000")
		activeSession.ExpiresAt = time.Now().Add(24 * time.Hour)
		err = repo.Create(ctx, activeSession)
		require.NoError(t, err)

		rowsAffected, err := repo.DeleteExpired(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, rowsAffected, int64(1))

		// Active session should still exist
		_, err = repo.GetByID(ctx, activeSession.ID)
		assert.NoError(t, err)
	})
}

func TestSessionRepository_DeleteByUserID(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create multiple sessions for the test user
		for i := 0; i < 3; i++ {
			session := createTestUserSession(testUserID)
			session.SessionToken = "token-delete-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			err := repo.Create(ctx, session)
			require.NoError(t, err)
		}

		rowsAffected, err := repo.DeleteByUserID(ctx, testUserID)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), rowsAffected)

		sessions, err := repo.GetByUserID(ctx, testUserID)
		assert.NoError(t, err)
		assert.Len(t, sessions, 0)
	})

	t.Run("NoMatches", func(t *testing.T) {
		rowsAffected, err := repo.DeleteByUserID(ctx, "00000000-0000-0000-0000-000000000002")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), rowsAffected)
	})
}

func TestSessionRepository_IsValid(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("ValidSession", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.Status = "active"
		session.ExpiresAt = time.Now().Add(24 * time.Hour)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		valid, err := repo.IsValid(ctx, session.SessionToken)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("ExpiredSession", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.SessionToken = "token-expired-valid-" + time.Now().Format("20060102150405.000000")
		session.Status = "active"
		session.ExpiresAt = time.Now().Add(-1 * time.Hour)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		valid, err := repo.IsValid(ctx, session.SessionToken)
		assert.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("TerminatedSession", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.SessionToken = "token-terminated-valid-" + time.Now().Format("20060102150405.000000")
		session.Status = "terminated"
		session.ExpiresAt = time.Now().Add(24 * time.Hour)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		valid, err := repo.IsValid(ctx, session.SessionToken)
		assert.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("NonExistentToken", func(t *testing.T) {
		valid, err := repo.IsValid(ctx, "non-existent-token-12345")
		assert.NoError(t, err)
		assert.False(t, valid)
	})
}

func TestSessionRepository_ExtendExpiration(t *testing.T) {
	pool, repo := setupSessionTestDB(t)
	if pool == nil {
		return
	}
	testUserID := createTestUserForSession(t, pool)
	defer cleanupSessionTestDB(t, pool, testUserID)
	defer pool.Close()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		session := createTestUserSession(testUserID)
		session.ExpiresAt = time.Now().Add(1 * time.Hour)
		err := repo.Create(ctx, session)
		require.NoError(t, err)

		newExpiry := time.Now().Add(48 * time.Hour)
		err = repo.ExtendExpiration(ctx, session.ID, newExpiry)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, session.ID)
		assert.NoError(t, err)
		assert.True(t, fetched.ExpiresAt.After(time.Now().Add(47*time.Hour)))
	})

	t.Run("NotFound", func(t *testing.T) {
		newExpiry := time.Now().Add(48 * time.Hour)
		err := repo.ExtendExpiration(ctx, "00000000-0000-0000-0000-000000000000", newExpiry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// =============================================================================
// Unit Tests (No Database Required)
// =============================================================================

func TestNewSessionRepository(t *testing.T) {
	t.Run("CreatesRepositoryWithNilPool", func(t *testing.T) {
		logger := logrus.New()
		repo := NewSessionRepository(nil, logger)
		assert.NotNil(t, repo)
	})

	t.Run("CreatesRepositoryWithNilLogger", func(t *testing.T) {
		repo := NewSessionRepository(nil, nil)
		assert.NotNil(t, repo)
	})
}

func TestUserSession_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullSession", func(t *testing.T) {
		memoryID := "memory-1"
		session := &UserSession{
			ID:           "session-1",
			UserID:       "user-1",
			SessionToken: "token-1",
			Context:      map[string]interface{}{"theme": "dark"},
			MemoryID:     &memoryID,
			Status:       "active",
			RequestCount: 10,
			LastActivity: time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now(),
		}

		jsonBytes, err := json.Marshal(session)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "session-1")
		assert.Contains(t, string(jsonBytes), "active")

		var decoded UserSession
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, session.ID, decoded.ID)
	})

	t.Run("SerializesMinimalSession", func(t *testing.T) {
		session := &UserSession{
			UserID:       "user-1",
			SessionToken: "token-1",
			Status:       "active",
		}

		jsonBytes, err := json.Marshal(session)
		require.NoError(t, err)

		var decoded UserSession
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "user-1", decoded.UserID)
	})
}

func TestUserSession_Fields(t *testing.T) {
	t.Run("AllFieldsSet", func(t *testing.T) {
		memoryID := "memory-1"
		now := time.Now()
		session := &UserSession{
			ID:           "id-1",
			UserID:       "user-1",
			SessionToken: "token-1",
			Context:      map[string]interface{}{"key": "value"},
			MemoryID:     &memoryID,
			Status:       "active",
			RequestCount: 5,
			LastActivity: now,
			ExpiresAt:    now.Add(24 * time.Hour),
			CreatedAt:    now,
		}

		assert.Equal(t, "id-1", session.ID)
		assert.Equal(t, "user-1", session.UserID)
		assert.Equal(t, "token-1", session.SessionToken)
		assert.Equal(t, "memory-1", *session.MemoryID)
		assert.Equal(t, "active", session.Status)
		assert.Equal(t, 5, session.RequestCount)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		session := &UserSession{}
		assert.Empty(t, session.ID)
		assert.Empty(t, session.UserID)
		assert.Empty(t, session.SessionToken)
		assert.Nil(t, session.Context)
		assert.Nil(t, session.MemoryID)
		assert.Empty(t, session.Status)
		assert.Equal(t, 0, session.RequestCount)
	})

	t.Run("NilPointerFields", func(t *testing.T) {
		session := &UserSession{
			UserID: "test",
		}
		assert.Nil(t, session.MemoryID)
	})
}

func TestUserSession_StatusValues(t *testing.T) {
	statuses := []string{"active", "suspended", "terminated", "expired", ""}

	for _, status := range statuses {
		t.Run("Status_"+status, func(t *testing.T) {
			session := &UserSession{
				Status: status,
			}
			assert.Equal(t, status, session.Status)
		})
	}
}

func TestUserSession_ContextFormats(t *testing.T) {
	t.Run("EmptyContext", func(t *testing.T) {
		session := &UserSession{
			Context: map[string]interface{}{},
		}
		assert.NotNil(t, session.Context)
		assert.Len(t, session.Context, 0)
	})

	t.Run("StandardContext", func(t *testing.T) {
		session := &UserSession{
			Context: map[string]interface{}{
				"theme":    "dark",
				"language": "en",
				"timezone": "UTC",
			},
		}
		assert.Equal(t, "dark", session.Context["theme"])
		assert.Equal(t, "en", session.Context["language"])
	})

	t.Run("ComplexContext", func(t *testing.T) {
		session := &UserSession{
			Context: map[string]interface{}{
				"preferences": map[string]interface{}{
					"theme":    "dark",
					"fontSize": 14,
				},
				"history": []string{"page1", "page2"},
				"flags":   map[string]bool{"feature1": true, "feature2": false},
			},
		}
		assert.NotNil(t, session.Context["preferences"])
		assert.NotNil(t, session.Context["history"])
		assert.NotNil(t, session.Context["flags"])
	})
}

func TestUserSession_JSONRoundTrip(t *testing.T) {
	t.Run("FullRoundTrip", func(t *testing.T) {
		memoryID := "round-trip-memory"
		now := time.Now().Truncate(time.Second)
		original := &UserSession{
			ID:           "round-trip-id",
			UserID:       "round-trip-user",
			SessionToken: "round-trip-token",
			Context:      map[string]interface{}{"key": "value"},
			MemoryID:     &memoryID,
			Status:       "active",
			RequestCount: 10,
			LastActivity: now,
			ExpiresAt:    now.Add(24 * time.Hour),
			CreatedAt:    now,
		}

		jsonBytes, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded UserSession
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.ID, decoded.ID)
		assert.Equal(t, original.UserID, decoded.UserID)
		assert.Equal(t, original.SessionToken, decoded.SessionToken)
		assert.Equal(t, *original.MemoryID, *decoded.MemoryID)
		assert.Equal(t, original.Status, decoded.Status)
		assert.Equal(t, original.RequestCount, decoded.RequestCount)
	})
}

func TestUserSession_EdgeCases(t *testing.T) {
	t.Run("VeryLongToken", func(t *testing.T) {
		longToken := ""
		for i := 0; i < 500; i++ {
			longToken += "a"
		}
		session := &UserSession{
			SessionToken: longToken,
		}
		assert.Len(t, session.SessionToken, 500)
	})

	t.Run("SpecialCharactersInUserID", func(t *testing.T) {
		session := &UserSession{
			UserID: "user@example.com",
		}
		assert.Contains(t, session.UserID, "@")
	})

	t.Run("ZeroRequestCount", func(t *testing.T) {
		session := &UserSession{
			RequestCount: 0,
		}
		assert.Equal(t, 0, session.RequestCount)
	})

	t.Run("LargeRequestCount", func(t *testing.T) {
		session := &UserSession{
			RequestCount: 1000000,
		}
		assert.Equal(t, 1000000, session.RequestCount)
	})

	t.Run("PastExpiresAt", func(t *testing.T) {
		session := &UserSession{
			ExpiresAt: time.Now().Add(-24 * time.Hour),
		}
		assert.True(t, session.ExpiresAt.Before(time.Now()))
	})

	t.Run("FutureExpiresAt", func(t *testing.T) {
		session := &UserSession{
			ExpiresAt: time.Now().Add(365 * 24 * time.Hour),
		}
		assert.True(t, session.ExpiresAt.After(time.Now()))
	})
}

func TestUserSession_JSONKeys(t *testing.T) {
	memoryID := "memory"
	session := &UserSession{
		ID:           "id",
		UserID:       "user",
		SessionToken: "token",
		MemoryID:     &memoryID,
		Status:       "active",
		RequestCount: 5,
	}

	jsonBytes, err := json.Marshal(session)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	expectedKeys := []string{
		"\"id\":", "\"user_id\":", "\"session_token\":", "\"memory_id\":",
		"\"status\":", "\"request_count\":",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, jsonStr, key, "JSON should contain key: "+key)
	}
}

func TestCreateTestUserSession_Helper(t *testing.T) {
	// Use a dummy userID for unit testing (no database required)
	dummyUserID := "test-user-id-for-unit-tests"
	session := createTestUserSession(dummyUserID)

	t.Run("HasRequiredFields", func(t *testing.T) {
		assert.NotEmpty(t, session.UserID)
		assert.Equal(t, dummyUserID, session.UserID)
		assert.NotEmpty(t, session.SessionToken)
		assert.NotEmpty(t, session.Status)
	})

	t.Run("HasOptionalFields", func(t *testing.T) {
		assert.NotNil(t, session.Context)
		// MemoryID is now nil by default to avoid FK constraints
		assert.Nil(t, session.MemoryID)
	})

	t.Run("HasDefaultValues", func(t *testing.T) {
		assert.Equal(t, "active", session.Status)
		assert.Equal(t, 0, session.RequestCount)
		assert.True(t, session.ExpiresAt.After(time.Now()))
	})
}

func TestUserSession_TimeFields(t *testing.T) {
	t.Run("AllTimeFieldsSet", func(t *testing.T) {
		now := time.Now()
		session := &UserSession{
			LastActivity: now,
			ExpiresAt:    now.Add(24 * time.Hour),
			CreatedAt:    now.Add(-1 * time.Hour),
		}

		assert.False(t, session.LastActivity.IsZero())
		assert.False(t, session.ExpiresAt.IsZero())
		assert.False(t, session.CreatedAt.IsZero())
		assert.True(t, session.ExpiresAt.After(session.LastActivity))
		assert.True(t, session.LastActivity.After(session.CreatedAt))
	})

	t.Run("ZeroTimeFields", func(t *testing.T) {
		session := &UserSession{}
		assert.True(t, session.LastActivity.IsZero())
		assert.True(t, session.ExpiresAt.IsZero())
		assert.True(t, session.CreatedAt.IsZero())
	})
}
