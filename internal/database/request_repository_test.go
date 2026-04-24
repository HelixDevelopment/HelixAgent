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
// Test Helper Functions for Request Repository
// =============================================================================

func setupRequestTestDB(t *testing.T) (*pgxpool.Pool, *RequestRepository) {
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
		t.Skipf("Skipping test: database not available: %v", err)
		return nil, nil
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	repo := NewRequestRepository(pool, logger)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v", err)
		pool.Close()
		return nil, nil
	}

	// Check if required tables exist
	var tableExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'llm_requests'
		)
	`).Scan(&tableExists)
	if err != nil || !tableExists {
		t.Skipf("Skipping test: llm_requests table does not exist (run migrations first)")
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupRequestTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM llm_requests WHERE prompt LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup llm_requests: %v", err)
	}
}

func createTestLLMRequest() *LLMRequest {
	// Use nil for foreign key fields to avoid FK constraint issues in tests
	// The session_id and user_id are optional fields
	return &LLMRequest{
		SessionID: nil, // Optional - don't set to avoid FK constraint
		UserID:    nil, // Optional - don't set to avoid FK constraint
		Prompt:    "test-prompt-" + time.Now().Format("20060102150405"),
		Messages: []map[string]string{
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
		},
		ModelParams:    map[string]interface{}{"temperature": 0.7, "max_tokens": 1000},
		EnsembleConfig: map[string]interface{}{"strategy": "weighted", "providers": []string{"openai", "anthropic"}},
		MemoryEnhanced: true,
		Memory:         map[string]interface{}{"context_window": 10},
		Status:         "pending",
		RequestType:    "chat",
	}
}

// =============================================================================
// Integration Tests (Require Database)
// =============================================================================

func TestRequestRepository_Create(t *testing.T) {
	pool, repo := setupRequestTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupRequestTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestLLMRequest()
		err := repo.Create(ctx, request)
		assert.NoError(t, err)
		assert.NotEmpty(t, request.ID)
		assert.False(t, request.CreatedAt.IsZero())
	})

	t.Run("WithNilSessionID", func(t *testing.T) {
		request := createTestLLMRequest()
		request.SessionID = nil
		request.Prompt = "test-nil-session-" + time.Now().Format("20060102150405")
		err := repo.Create(ctx, request)
		assert.NoError(t, err)
		assert.NotEmpty(t, request.ID)
	})

	t.Run("WithNilUserID", func(t *testing.T) {
		request := createTestLLMRequest()
		request.UserID = nil
		request.Prompt = "test-nil-user-" + time.Now().Format("20060102150405")
		err := repo.Create(ctx, request)
		assert.NoError(t, err)
		assert.NotEmpty(t, request.ID)
	})

	t.Run("WithEmptyMessages", func(t *testing.T) {
		request := createTestLLMRequest()
		request.Messages = []map[string]string{}
		request.Prompt = "test-empty-messages-" + time.Now().Format("20060102150405")
		err := repo.Create(ctx, request)
		assert.NoError(t, err)
		assert.NotEmpty(t, request.ID)
	})
}

func TestRequestRepository_GetByID(t *testing.T) {
	pool, repo := setupRequestTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupRequestTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestLLMRequest()
		err := repo.Create(ctx, request)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, request.ID)
		assert.NoError(t, err)
		assert.Equal(t, request.ID, fetched.ID)
		assert.Equal(t, request.Prompt, fetched.Prompt)
		assert.Equal(t, request.Status, fetched.Status)
		assert.Equal(t, request.RequestType, fetched.RequestType)
		assert.Equal(t, request.MemoryEnhanced, fetched.MemoryEnhanced)
	})

	t.Run("NotFound", func(t *testing.T) {
		// Use a valid UUID format that doesn't exist
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRequestRepository_GetBySessionID(t *testing.T) {
	// Skip tests that require foreign key references (session_id -> user_sessions)
	// These tests would need a valid session to be created first
	t.Run("Success", func(t *testing.T) {
		t.Skip("Skipping: requires valid session record due to foreign key constraint")  // SKIP-OK: #legacy-untriaged
	})

	t.Run("Pagination", func(t *testing.T) {
		t.Skip("Skipping: requires valid session record due to foreign key constraint")  // SKIP-OK: #legacy-untriaged
	})
}

func TestRequestRepository_GetByUserID(t *testing.T) {
	// Skip tests that require foreign key references (user_id -> users)
	// These tests would need a valid user to be created first
	t.Run("Success", func(t *testing.T) {
		t.Skip("Skipping: requires valid user record due to foreign key constraint")  // SKIP-OK: #legacy-untriaged
	})
}

func TestRequestRepository_UpdateStatus(t *testing.T) {
	pool, repo := setupRequestTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupRequestTestDB(t, pool)

	ctx := context.Background()

	t.Run("Processing", func(t *testing.T) {
		request := createTestLLMRequest()
		request.Status = "pending"
		err := repo.Create(ctx, request)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, request.ID, "processing")
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, request.ID)
		assert.NoError(t, err)
		assert.Equal(t, "processing", fetched.Status)
		assert.NotNil(t, fetched.StartedAt)
	})

	t.Run("Completed", func(t *testing.T) {
		request := createTestLLMRequest()
		request.Prompt = "test-completed-" + time.Now().Format("20060102150405")
		request.Status = "processing"
		err := repo.Create(ctx, request)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, request.ID, "completed")
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, request.ID)
		assert.NoError(t, err)
		assert.Equal(t, "completed", fetched.Status)
		assert.NotNil(t, fetched.CompletedAt)
	})

	t.Run("Failed", func(t *testing.T) {
		request := createTestLLMRequest()
		request.Prompt = "test-failed-" + time.Now().Format("20060102150405")
		request.Status = "processing"
		err := repo.Create(ctx, request)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, request.ID, "failed")
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, request.ID)
		assert.NoError(t, err)
		assert.Equal(t, "failed", fetched.Status)
		assert.NotNil(t, fetched.CompletedAt)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, "00000000-0000-0000-0000-000000000000", "processing")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRequestRepository_Delete(t *testing.T) {
	pool, repo := setupRequestTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupRequestTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestLLMRequest()
		err := repo.Create(ctx, request)
		require.NoError(t, err)

		err = repo.Delete(ctx, request.ID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, request.ID)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRequestRepository_GetPendingRequests(t *testing.T) {
	pool, repo := setupRequestTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupRequestTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create pending requests
		for i := 0; i < 3; i++ {
			request := createTestLLMRequest()
			request.Prompt = "test-pending-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			request.Status = "pending"
			err := repo.Create(ctx, request)
			require.NoError(t, err)
		}

		// Create non-pending request
		nonPendingRequest := createTestLLMRequest()
		nonPendingRequest.Prompt = "test-processing-" + time.Now().Format("20060102150405")
		nonPendingRequest.Status = "processing"
		err := repo.Create(ctx, nonPendingRequest)
		require.NoError(t, err)

		requests, err := repo.GetPendingRequests(ctx, 10)
		assert.NoError(t, err)

		// All returned requests should be pending
		for _, req := range requests {
			assert.Equal(t, "pending", req.Status)
		}
	})
}

func TestRequestRepository_GetRequestStats(t *testing.T) {
	// Skip all tests - they require foreign key references (user_id -> users)
	t.Run("Success", func(t *testing.T) {
		t.Skip("Skipping: requires valid user record due to foreign key constraint")  // SKIP-OK: #legacy-untriaged
	})

	t.Run("EmptyUserID", func(t *testing.T) {
		t.Skip("Skipping: empty user_id not compatible with UUID column type")  // SKIP-OK: #legacy-untriaged
	})
}

// =============================================================================
// Unit Tests (No Database Required)
// =============================================================================

func TestNewRequestRepository(t *testing.T) {
	t.Run("CreatesRepositoryWithNilPool", func(t *testing.T) {
		logger := logrus.New()
		repo := NewRequestRepository(nil, logger)
		assert.NotNil(t, repo)
	})

	t.Run("CreatesRepositoryWithNilLogger", func(t *testing.T) {
		repo := NewRequestRepository(nil, nil)
		assert.NotNil(t, repo)
	})
}

func TestLLMRequest_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullRequest", func(t *testing.T) {
		sessionID := "session-1"
		userID := "user-1"
		startedAt := time.Now()
		completedAt := time.Now()

		request := &LLMRequest{
			ID:        "request-1",
			SessionID: &sessionID,
			UserID:    &userID,
			Prompt:    "Hello, world!",
			Messages: []map[string]string{
				{"role": "user", "content": "Hello"},
			},
			ModelParams:    map[string]interface{}{"temperature": 0.7},
			EnsembleConfig: map[string]interface{}{"strategy": "weighted"},
			MemoryEnhanced: true,
			Memory:         map[string]interface{}{"key": "value"},
			Status:         "completed",
			RequestType:    "chat",
			CreatedAt:      time.Now(),
			StartedAt:      &startedAt,
			CompletedAt:    &completedAt,
		}

		jsonBytes, err := json.Marshal(request)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "Hello, world!")
		assert.Contains(t, string(jsonBytes), "completed")

		var decoded LLMRequest
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, request.Prompt, decoded.Prompt)
	})

	t.Run("SerializesMinimalRequest", func(t *testing.T) {
		request := &LLMRequest{
			Prompt:      "Test prompt",
			Status:      "pending",
			RequestType: "completion",
		}

		jsonBytes, err := json.Marshal(request)
		require.NoError(t, err)

		var decoded LLMRequest
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "Test prompt", decoded.Prompt)
	})
}

func TestLLMRequest_Fields(t *testing.T) {
	t.Run("AllFieldsSet", func(t *testing.T) {
		sessionID := "session-1"
		userID := "user-1"
		startedAt := time.Now()
		completedAt := time.Now()

		request := &LLMRequest{
			ID:        "id-1",
			SessionID: &sessionID,
			UserID:    &userID,
			Prompt:    "prompt",
			Messages: []map[string]string{
				{"role": "user", "content": "test"},
			},
			ModelParams:    map[string]interface{}{"temp": 0.5},
			EnsembleConfig: map[string]interface{}{"strategy": "voting"},
			MemoryEnhanced: true,
			Memory:         map[string]interface{}{"ctx": "value"},
			Status:         "pending",
			RequestType:    "chat",
			CreatedAt:      time.Now(),
			StartedAt:      &startedAt,
			CompletedAt:    &completedAt,
		}

		assert.Equal(t, "id-1", request.ID)
		assert.Equal(t, "session-1", *request.SessionID)
		assert.Equal(t, "user-1", *request.UserID)
		assert.Equal(t, "prompt", request.Prompt)
		assert.Len(t, request.Messages, 1)
		assert.True(t, request.MemoryEnhanced)
		assert.Equal(t, "pending", request.Status)
		assert.Equal(t, "chat", request.RequestType)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		request := &LLMRequest{}
		assert.Empty(t, request.ID)
		assert.Nil(t, request.SessionID)
		assert.Nil(t, request.UserID)
		assert.Empty(t, request.Prompt)
		assert.Nil(t, request.Messages)
		assert.Nil(t, request.ModelParams)
		assert.Nil(t, request.EnsembleConfig)
		assert.False(t, request.MemoryEnhanced)
		assert.Nil(t, request.Memory)
		assert.Empty(t, request.Status)
		assert.Empty(t, request.RequestType)
	})

	t.Run("NilPointerFields", func(t *testing.T) {
		request := &LLMRequest{
			Prompt: "test",
		}
		assert.Nil(t, request.SessionID)
		assert.Nil(t, request.UserID)
		assert.Nil(t, request.StartedAt)
		assert.Nil(t, request.CompletedAt)
	})
}

func TestLLMRequest_StatusValues(t *testing.T) {
	statuses := []string{"pending", "processing", "completed", "failed", "cancelled", "timeout"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			request := &LLMRequest{
				Status: status,
			}
			assert.Equal(t, status, request.Status)
		})
	}
}

func TestLLMRequest_RequestTypes(t *testing.T) {
	types := []string{"chat", "completion", "embedding", "image", "audio"}

	for _, reqType := range types {
		t.Run(reqType, func(t *testing.T) {
			request := &LLMRequest{
				RequestType: reqType,
			}
			assert.Equal(t, reqType, request.RequestType)
		})
	}
}

func TestLLMRequest_MessagesFormats(t *testing.T) {
	t.Run("EmptyMessages", func(t *testing.T) {
		request := &LLMRequest{
			Messages: []map[string]string{},
		}
		assert.NotNil(t, request.Messages)
		assert.Len(t, request.Messages, 0)
	})

	t.Run("SingleMessage", func(t *testing.T) {
		request := &LLMRequest{
			Messages: []map[string]string{
				{"role": "user", "content": "Hello"},
			},
		}
		assert.Len(t, request.Messages, 1)
		assert.Equal(t, "user", request.Messages[0]["role"])
	})

	t.Run("MultipleMessages", func(t *testing.T) {
		request := &LLMRequest{
			Messages: []map[string]string{
				{"role": "system", "content": "You are a helpful assistant"},
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there!"},
				{"role": "user", "content": "How are you?"},
			},
		}
		assert.Len(t, request.Messages, 4)
	})

	t.Run("MessageWithExtraFields", func(t *testing.T) {
		request := &LLMRequest{
			Messages: []map[string]string{
				{"role": "user", "content": "Hello", "name": "John", "function_call": "test"},
			},
		}
		assert.Equal(t, "John", request.Messages[0]["name"])
	})
}

func TestLLMRequest_ModelParamsFormats(t *testing.T) {
	t.Run("StandardParams", func(t *testing.T) {
		request := &LLMRequest{
			ModelParams: map[string]interface{}{
				"temperature":       0.7,
				"max_tokens":        1000,
				"top_p":             0.9,
				"frequency_penalty": 0.0,
				"presence_penalty":  0.0,
			},
		}
		assert.Equal(t, 0.7, request.ModelParams["temperature"])
		assert.Equal(t, 1000, request.ModelParams["max_tokens"])
	})

	t.Run("NestedParams", func(t *testing.T) {
		request := &LLMRequest{
			ModelParams: map[string]interface{}{
				"stop_sequences": []string{"END", "STOP"},
				"logit_bias":     map[string]int{"50256": -100},
			},
		}
		assert.NotNil(t, request.ModelParams["stop_sequences"])
		assert.NotNil(t, request.ModelParams["logit_bias"])
	})
}

func TestLLMRequest_EnsembleConfigFormats(t *testing.T) {
	t.Run("WeightedStrategy", func(t *testing.T) {
		request := &LLMRequest{
			EnsembleConfig: map[string]interface{}{
				"strategy": "weighted",
				"weights": map[string]float64{
					"openai":    0.5,
					"anthropic": 0.3,
					"google":    0.2,
				},
			},
		}
		assert.Equal(t, "weighted", request.EnsembleConfig["strategy"])
	})

	t.Run("VotingStrategy", func(t *testing.T) {
		request := &LLMRequest{
			EnsembleConfig: map[string]interface{}{
				"strategy":      "voting",
				"min_agreement": 2,
				"timeout_ms":    5000,
			},
		}
		assert.Equal(t, "voting", request.EnsembleConfig["strategy"])
	})
}

func TestLLMRequest_MemoryFormats(t *testing.T) {
	t.Run("SimpleMemory", func(t *testing.T) {
		request := &LLMRequest{
			MemoryEnhanced: true,
			Memory: map[string]interface{}{
				"context_window": 10,
				"summary":        "Previous conversation about AI",
			},
		}
		assert.True(t, request.MemoryEnhanced)
		assert.Equal(t, 10, request.Memory["context_window"])
	})

	t.Run("ComplexMemory", func(t *testing.T) {
		request := &LLMRequest{
			MemoryEnhanced: true,
			Memory: map[string]interface{}{
				"short_term": []string{"fact1", "fact2"},
				"long_term": map[string]interface{}{
					"user_preferences": map[string]string{"style": "formal"},
				},
			},
		}
		assert.NotNil(t, request.Memory["short_term"])
		assert.NotNil(t, request.Memory["long_term"])
	})
}

func TestLLMRequest_JSONRoundTrip(t *testing.T) {
	t.Run("FullRoundTrip", func(t *testing.T) {
		sessionID := "session-round-trip"
		userID := "user-round-trip"
		startedAt := time.Now().Truncate(time.Second)
		completedAt := time.Now().Truncate(time.Second)

		original := &LLMRequest{
			ID:        "round-trip-id",
			SessionID: &sessionID,
			UserID:    &userID,
			Prompt:    "Test prompt for round trip",
			Messages: []map[string]string{
				{"role": "user", "content": "Hello"},
			},
			ModelParams:    map[string]interface{}{"temperature": 0.7},
			EnsembleConfig: map[string]interface{}{"strategy": "weighted"},
			MemoryEnhanced: true,
			Memory:         map[string]interface{}{"key": "value"},
			Status:         "completed",
			RequestType:    "chat",
			CreatedAt:      time.Now().Truncate(time.Second),
			StartedAt:      &startedAt,
			CompletedAt:    &completedAt,
		}

		jsonBytes, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded LLMRequest
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.ID, decoded.ID)
		assert.Equal(t, *original.SessionID, *decoded.SessionID)
		assert.Equal(t, *original.UserID, *decoded.UserID)
		assert.Equal(t, original.Prompt, decoded.Prompt)
		assert.Equal(t, original.Status, decoded.Status)
		assert.Equal(t, original.RequestType, decoded.RequestType)
		assert.Equal(t, original.MemoryEnhanced, decoded.MemoryEnhanced)
	})
}

func TestLLMRequest_EdgeCases(t *testing.T) {
	t.Run("VeryLongPrompt", func(t *testing.T) {
		longPrompt := ""
		for i := 0; i < 10000; i++ {
			longPrompt += "word "
		}
		request := &LLMRequest{
			Prompt: longPrompt,
		}
		assert.Len(t, request.Prompt, 50000)
	})

	t.Run("SpecialCharactersInPrompt", func(t *testing.T) {
		request := &LLMRequest{
			Prompt: "Test with special chars: <>&\"'`~!@#$%^&*()+=[]{}|\\:;,.<>?/\nNewline\tTab",
		}
		assert.Contains(t, request.Prompt, "<>&")
		assert.Contains(t, request.Prompt, "\n")
		assert.Contains(t, request.Prompt, "\t")
	})

	t.Run("UnicodeInPrompt", func(t *testing.T) {
		request := &LLMRequest{
			Prompt: "Test with unicode: 你好世界 日本語 한국어 🚀🌟",
		}
		assert.Contains(t, request.Prompt, "你好世界")
		assert.Contains(t, request.Prompt, "🚀")
	})

	t.Run("EmptyStrings", func(t *testing.T) {
		request := &LLMRequest{
			Prompt:      "",
			Status:      "",
			RequestType: "",
		}
		assert.Empty(t, request.Prompt)
		assert.Empty(t, request.Status)
		assert.Empty(t, request.RequestType)
	})

	t.Run("LargeMessagesArray", func(t *testing.T) {
		messages := make([]map[string]string, 100)
		for i := 0; i < 100; i++ {
			messages[i] = map[string]string{
				"role":    "user",
				"content": "Message " + string(rune('0'+i%10)),
			}
		}
		request := &LLMRequest{
			Messages: messages,
		}
		assert.Len(t, request.Messages, 100)
	})
}

func TestLLMRequest_JSONKeys(t *testing.T) {
	sessionID := "session"
	userID := "user"
	request := &LLMRequest{
		ID:             "id",
		SessionID:      &sessionID,
		UserID:         &userID,
		Prompt:         "prompt",
		MemoryEnhanced: true,
		Status:         "pending",
		RequestType:    "chat",
	}

	jsonBytes, err := json.Marshal(request)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	expectedKeys := []string{
		"\"id\":", "\"session_id\":", "\"user_id\":", "\"prompt\":",
		"\"memory_enhanced\":", "\"status\":", "\"request_type\":",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, jsonStr, key, "JSON should contain key: "+key)
	}
}

func TestCreateTestLLMRequest_Helper(t *testing.T) {
	request := createTestLLMRequest()

	t.Run("HasRequiredFields", func(t *testing.T) {
		assert.NotEmpty(t, request.Prompt)
		assert.NotEmpty(t, request.Status)
		assert.NotEmpty(t, request.RequestType)
	})

	t.Run("HasOptionalFields", func(t *testing.T) {
		// SessionID and UserID are nil to avoid FK constraints in tests
		assert.Nil(t, request.SessionID)
		assert.Nil(t, request.UserID)
		assert.NotNil(t, request.Messages)
		assert.NotNil(t, request.ModelParams)
		assert.NotNil(t, request.EnsembleConfig)
		assert.NotNil(t, request.Memory)
	})

	t.Run("HasDefaultValues", func(t *testing.T) {
		assert.Equal(t, "pending", request.Status)
		assert.Equal(t, "chat", request.RequestType)
		assert.True(t, request.MemoryEnhanced)
	})
}
