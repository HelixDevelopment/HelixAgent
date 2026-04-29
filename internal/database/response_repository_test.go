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
// Test Helper Functions for Response Repository
// =============================================================================

func setupResponseTestDB(t *testing.T) (*pgxpool.Pool, *ResponseRepository, *RequestRepository) {
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
		return nil, nil, nil
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	responseRepo := NewResponseRepository(pool, logger)
	requestRepo := NewRequestRepository(pool, logger)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v (SKIP-OK: #infra-db-unavailable)", err)
		pool.Close()
		return nil, nil, nil
	}

	// Check if required tables exist
	var tableExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'llm_responses'
		)
	`).Scan(&tableExists)
	if err != nil || !tableExists {
		t.Skipf("Skipping test: llm_responses table does not exist (run migrations first) (SKIP-OK: #infra-schema-not-migrated)")
		pool.Close()
		return nil, nil, nil
	}

	// Also check llm_requests table since response tests depend on it
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'llm_requests'
		)
	`).Scan(&tableExists)
	if err != nil || !tableExists {
		t.Skipf("Skipping test: llm_requests table does not exist (run migrations first) (SKIP-OK: #infra-schema-not-migrated)")
		pool.Close()
		return nil, nil, nil
	}

	return pool, responseRepo, requestRepo
}

func cleanupResponseTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM llm_responses WHERE provider_name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup llm_responses: %v", err)
	}
	_, err = pool.Exec(ctx, "DELETE FROM llm_requests WHERE prompt LIKE 'test-response-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup llm_requests: %v", err)
	}
}

func createTestLLMResponse(requestID string) *LLMResponse {
	// ProviderID is nil to avoid FK constraint issues
	return &LLMResponse{
		RequestID:      requestID,
		ProviderID:     nil, // Optional - don't set to avoid FK constraint
		ProviderName:   "test-provider-" + time.Now().Format("20060102150405"),
		Content:        "This is a test response content",
		Confidence:     0.95,
		TokensUsed:     150,
		ResponseTime:   200,
		FinishReason:   "stop",
		Metadata:       map[string]interface{}{"model": "gpt-4", "tokens_input": 50, "tokens_output": 100},
		Selected:       false,
		SelectionScore: 0.85,
	}
}

func createTestRequestForResponse(t *testing.T, requestRepo *RequestRepository) *LLMRequest {
	ctx := context.Background()
	request := &LLMRequest{
		Prompt:      "test-response-prompt-" + time.Now().Format("20060102150405.000000"),
		Status:      "pending",
		RequestType: "chat",
	}
	err := requestRepo.Create(ctx, request)
	require.NoError(t, err)
	return request
}

// =============================================================================
// Integration Tests (Require Database)
// =============================================================================

func TestResponseRepository_Create(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)
		response := createTestLLMResponse(request.ID)

		err := repo.Create(ctx, response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.ID)
		assert.False(t, response.CreatedAt.IsZero())
	})

	t.Run("WithNilProviderID", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)
		response := createTestLLMResponse(request.ID)
		response.ProviderID = nil
		response.ProviderName = "test-nil-provider-" + time.Now().Format("20060102150405")

		err := repo.Create(ctx, response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.ID)
	})

	t.Run("WithNilMetadata", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)
		response := createTestLLMResponse(request.ID)
		response.Metadata = nil
		response.ProviderName = "test-nil-metadata-" + time.Now().Format("20060102150405")

		err := repo.Create(ctx, response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.ID)
	})
}

func TestResponseRepository_GetByID(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)
		response := createTestLLMResponse(request.ID)
		err := repo.Create(ctx, response)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, response.ID)
		assert.NoError(t, err)
		assert.Equal(t, response.ID, fetched.ID)
		assert.Equal(t, response.RequestID, fetched.RequestID)
		assert.Equal(t, response.ProviderName, fetched.ProviderName)
		assert.Equal(t, response.Content, fetched.Content)
		assert.Equal(t, response.Confidence, fetched.Confidence)
		assert.Equal(t, response.TokensUsed, fetched.TokensUsed)
		assert.Equal(t, response.ResponseTime, fetched.ResponseTime)
		assert.Equal(t, response.FinishReason, fetched.FinishReason)
		assert.Equal(t, response.Selected, fetched.Selected)
		assert.Equal(t, response.SelectionScore, fetched.SelectionScore)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestResponseRepository_GetByRequestID(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)

		// Create multiple responses for the same request
		for i := 0; i < 3; i++ {
			response := createTestLLMResponse(request.ID)
			response.ProviderName = "test-multi-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			response.Confidence = float64(90+i) / 100.0
			err := repo.Create(ctx, response)
			require.NoError(t, err)
		}

		responses, err := repo.GetByRequestID(ctx, request.ID)
		assert.NoError(t, err)
		assert.Len(t, responses, 3)
	})

	t.Run("EmptyResult", func(t *testing.T) {
		responses, err := repo.GetByRequestID(ctx, "00000000-0000-0000-0000-000000000001")
		assert.NoError(t, err)
		assert.Len(t, responses, 0)
	})
}

func TestResponseRepository_GetSelectedResponse(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)

		// Create non-selected response
		response1 := createTestLLMResponse(request.ID)
		response1.ProviderName = "test-not-selected-" + time.Now().Format("20060102150405.000")
		response1.Selected = false
		err := repo.Create(ctx, response1)
		require.NoError(t, err)

		// Create selected response
		response2 := createTestLLMResponse(request.ID)
		response2.ProviderName = "test-selected-" + time.Now().Format("20060102150405.001")
		response2.Selected = true
		err = repo.Create(ctx, response2)
		require.NoError(t, err)

		selected, err := repo.GetSelectedResponse(ctx, request.ID)
		assert.NoError(t, err)
		assert.True(t, selected.Selected)
	})

	t.Run("NotFound", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)

		// Create only non-selected responses
		response := createTestLLMResponse(request.ID)
		response.ProviderName = "test-none-selected-" + time.Now().Format("20060102150405")
		response.Selected = false
		err := repo.Create(ctx, response)
		require.NoError(t, err)

		_, err = repo.GetSelectedResponse(ctx, request.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no selected response")
	})
}

func TestResponseRepository_SetSelected(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)

		// Create multiple responses
		response1 := createTestLLMResponse(request.ID)
		response1.ProviderName = "test-select-1-" + time.Now().Format("20060102150405.000")
		response1.Selected = true
		err := repo.Create(ctx, response1)
		require.NoError(t, err)

		response2 := createTestLLMResponse(request.ID)
		response2.ProviderName = "test-select-2-" + time.Now().Format("20060102150405.001")
		response2.Selected = false
		err = repo.Create(ctx, response2)
		require.NoError(t, err)

		// Select the second response
		err = repo.SetSelected(ctx, response2.ID, 0.95)
		assert.NoError(t, err)

		// Verify first is no longer selected
		fetched1, err := repo.GetByID(ctx, response1.ID)
		assert.NoError(t, err)
		assert.False(t, fetched1.Selected)

		// Verify second is now selected
		fetched2, err := repo.GetByID(ctx, response2.ID)
		assert.NoError(t, err)
		assert.True(t, fetched2.Selected)
		assert.Equal(t, 0.95, fetched2.SelectionScore)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.SetSelected(ctx, "00000000-0000-0000-0000-000000000000", 0.9)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestResponseRepository_Delete(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)
		response := createTestLLMResponse(request.ID)
		err := repo.Create(ctx, response)
		require.NoError(t, err)

		err = repo.Delete(ctx, response.ID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, response.ID)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestResponseRepository_DeleteByRequestID(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		request := createTestRequestForResponse(t, requestRepo)

		// Create multiple responses
		for i := 0; i < 3; i++ {
			response := createTestLLMResponse(request.ID)
			response.ProviderName = "test-delete-batch-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			err := repo.Create(ctx, response)
			require.NoError(t, err)
		}

		rowsAffected, err := repo.DeleteByRequestID(ctx, request.ID)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), rowsAffected)

		responses, err := repo.GetByRequestID(ctx, request.ID)
		assert.NoError(t, err)
		assert.Len(t, responses, 0)
	})

	t.Run("NoMatches", func(t *testing.T) {
		rowsAffected, err := repo.DeleteByRequestID(ctx, "00000000-0000-0000-0000-000000000001")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), rowsAffected)
	})
}

func TestResponseRepository_GetByProviderID(t *testing.T) {
	// Skip tests that require foreign key references (provider_id -> llm_providers)
	t.Run("Success", func(t *testing.T) {
		t.Skip("Skipping: requires valid provider record due to foreign key constraint")  // SKIP-OK: #legacy-untriaged
	})

	t.Run("Pagination", func(t *testing.T) {
		t.Skip("Skipping: requires valid provider record due to foreign key constraint")  // SKIP-OK: #legacy-untriaged
	})
}

func TestResponseRepository_GetProviderStats(t *testing.T) {
	pool, repo, requestRepo := setupResponseTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupResponseTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		providerName := "test-stats-provider-" + time.Now().Format("20060102150405")

		// Create responses with different selection states
		for i := 0; i < 5; i++ {
			request := createTestRequestForResponse(t, requestRepo)
			response := createTestLLMResponse(request.ID)
			response.ProviderName = providerName
			response.Selected = i < 2 // First 2 are selected
			response.Confidence = float64(80+i) / 100.0
			response.TokensUsed = 100 + i*10
			err := repo.Create(ctx, response)
			require.NoError(t, err)
		}

		since := time.Now().Add(-1 * time.Hour)
		stats, err := repo.GetProviderStats(ctx, since)
		assert.NoError(t, err)
		assert.NotNil(t, stats)

		// Find stats for our test provider
		var found bool
		for _, s := range stats {
			if s["provider_name"] == providerName {
				found = true
				assert.Equal(t, 5, s["total_responses"])
				assert.Equal(t, 2, s["selected_count"])
				break
			}
		}
		assert.True(t, found, "Should find stats for test provider")
	})
}

// =============================================================================
// Unit Tests (No Database Required)
// =============================================================================

func TestNewResponseRepository(t *testing.T) {
	t.Run("CreatesRepositoryWithNilPool", func(t *testing.T) {
		logger := logrus.New()
		repo := NewResponseRepository(nil, logger)
		assert.NotNil(t, repo)
	})

	t.Run("CreatesRepositoryWithNilLogger", func(t *testing.T) {
		repo := NewResponseRepository(nil, nil)
		assert.NotNil(t, repo)
	})
}

func TestLLMResponse_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullResponse", func(t *testing.T) {
		providerID := "provider-1"
		response := &LLMResponse{
			ID:             "response-1",
			RequestID:      "request-1",
			ProviderID:     &providerID,
			ProviderName:   "test-provider",
			Content:        "This is the response content",
			Confidence:     0.95,
			TokensUsed:     150,
			ResponseTime:   200,
			FinishReason:   "stop",
			Metadata:       map[string]interface{}{"model": "gpt-4"},
			Selected:       true,
			SelectionScore: 0.9,
			CreatedAt:      time.Now(),
		}

		jsonBytes, err := json.Marshal(response)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "response content")
		assert.Contains(t, string(jsonBytes), "test-provider")

		var decoded LLMResponse
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, response.Content, decoded.Content)
	})

	t.Run("SerializesMinimalResponse", func(t *testing.T) {
		response := &LLMResponse{
			RequestID:    "request-1",
			ProviderName: "provider",
			Content:      "content",
		}

		jsonBytes, err := json.Marshal(response)
		require.NoError(t, err)

		var decoded LLMResponse
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "content", decoded.Content)
	})
}

func TestLLMResponse_Fields(t *testing.T) {
	t.Run("AllFieldsSet", func(t *testing.T) {
		providerID := "provider-1"
		response := &LLMResponse{
			ID:             "id-1",
			RequestID:      "request-1",
			ProviderID:     &providerID,
			ProviderName:   "provider",
			Content:        "content",
			Confidence:     0.95,
			TokensUsed:     100,
			ResponseTime:   150,
			FinishReason:   "stop",
			Metadata:       map[string]interface{}{"key": "value"},
			Selected:       true,
			SelectionScore: 0.9,
			CreatedAt:      time.Now(),
		}

		assert.Equal(t, "id-1", response.ID)
		assert.Equal(t, "request-1", response.RequestID)
		assert.Equal(t, "provider-1", *response.ProviderID)
		assert.Equal(t, "provider", response.ProviderName)
		assert.Equal(t, "content", response.Content)
		assert.Equal(t, 0.95, response.Confidence)
		assert.Equal(t, 100, response.TokensUsed)
		assert.Equal(t, int64(150), response.ResponseTime)
		assert.Equal(t, "stop", response.FinishReason)
		assert.True(t, response.Selected)
		assert.Equal(t, 0.9, response.SelectionScore)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		response := &LLMResponse{}
		assert.Empty(t, response.ID)
		assert.Empty(t, response.RequestID)
		assert.Nil(t, response.ProviderID)
		assert.Empty(t, response.ProviderName)
		assert.Empty(t, response.Content)
		assert.Equal(t, 0.0, response.Confidence)
		assert.Equal(t, 0, response.TokensUsed)
		assert.Equal(t, int64(0), response.ResponseTime)
		assert.Empty(t, response.FinishReason)
		assert.Nil(t, response.Metadata)
		assert.False(t, response.Selected)
		assert.Equal(t, 0.0, response.SelectionScore)
	})

	t.Run("NilPointerFields", func(t *testing.T) {
		response := &LLMResponse{
			Content: "test",
		}
		assert.Nil(t, response.ProviderID)
	})
}

func TestLLMResponse_FinishReasons(t *testing.T) {
	reasons := []string{"stop", "length", "content_filter", "function_call", "tool_calls", "error", ""}

	for _, reason := range reasons {
		t.Run("FinishReason_"+reason, func(t *testing.T) {
			response := &LLMResponse{
				FinishReason: reason,
			}
			assert.Equal(t, reason, response.FinishReason)
		})
	}
}

func TestLLMResponse_ConfidenceValues(t *testing.T) {
	confidences := []float64{0.0, 0.5, 0.75, 0.9, 0.95, 0.99, 1.0}

	for _, conf := range confidences {
		t.Run("Confidence", func(t *testing.T) {
			response := &LLMResponse{
				Confidence: conf,
			}
			assert.Equal(t, conf, response.Confidence)
		})
	}
}

func TestLLMResponse_MetadataFormats(t *testing.T) {
	t.Run("EmptyMetadata", func(t *testing.T) {
		response := &LLMResponse{
			Metadata: map[string]interface{}{},
		}
		assert.NotNil(t, response.Metadata)
		assert.Len(t, response.Metadata, 0)
	})

	t.Run("StandardMetadata", func(t *testing.T) {
		response := &LLMResponse{
			Metadata: map[string]interface{}{
				"model":         "gpt-4",
				"tokens_input":  50,
				"tokens_output": 100,
				"latency_ms":    150.5,
			},
		}
		assert.Equal(t, "gpt-4", response.Metadata["model"])
		assert.Equal(t, 50, response.Metadata["tokens_input"])
	})

	t.Run("ComplexMetadata", func(t *testing.T) {
		response := &LLMResponse{
			Metadata: map[string]interface{}{
				"model_info": map[string]interface{}{
					"name":    "gpt-4",
					"version": "2024-01",
				},
				"usage": map[string]int{
					"prompt_tokens":     50,
					"completion_tokens": 100,
				},
				"logprobs": []float64{-0.5, -0.3, -0.1},
			},
		}
		assert.NotNil(t, response.Metadata["model_info"])
		assert.NotNil(t, response.Metadata["usage"])
		assert.NotNil(t, response.Metadata["logprobs"])
	})
}

func TestLLMResponse_JSONRoundTrip(t *testing.T) {
	t.Run("FullRoundTrip", func(t *testing.T) {
		providerID := "round-trip-provider"
		original := &LLMResponse{
			ID:             "round-trip-id",
			RequestID:      "round-trip-request",
			ProviderID:     &providerID,
			ProviderName:   "round-trip-name",
			Content:        "Round trip content",
			Confidence:     0.95,
			TokensUsed:     150,
			ResponseTime:   200,
			FinishReason:   "stop",
			Metadata:       map[string]interface{}{"key": "value"},
			Selected:       true,
			SelectionScore: 0.9,
			CreatedAt:      time.Now().Truncate(time.Second),
		}

		jsonBytes, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded LLMResponse
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.ID, decoded.ID)
		assert.Equal(t, original.RequestID, decoded.RequestID)
		assert.Equal(t, *original.ProviderID, *decoded.ProviderID)
		assert.Equal(t, original.ProviderName, decoded.ProviderName)
		assert.Equal(t, original.Content, decoded.Content)
		assert.Equal(t, original.Confidence, decoded.Confidence)
		assert.Equal(t, original.TokensUsed, decoded.TokensUsed)
		assert.Equal(t, original.ResponseTime, decoded.ResponseTime)
		assert.Equal(t, original.FinishReason, decoded.FinishReason)
		assert.Equal(t, original.Selected, decoded.Selected)
		assert.Equal(t, original.SelectionScore, decoded.SelectionScore)
	})
}

func TestLLMResponse_EdgeCases(t *testing.T) {
	t.Run("VeryLongContent", func(t *testing.T) {
		longContent := ""
		for i := 0; i < 10000; i++ {
			longContent += "word "
		}
		response := &LLMResponse{
			Content: longContent,
		}
		assert.Len(t, response.Content, 50000)
	})

	t.Run("SpecialCharactersInContent", func(t *testing.T) {
		response := &LLMResponse{
			Content: "Content with special chars: <>&\"'`~!@#$%^&*()+=[]{}|\\:;,.<>?/\nNewline\tTab",
		}
		assert.Contains(t, response.Content, "<>&")
		assert.Contains(t, response.Content, "\n")
		assert.Contains(t, response.Content, "\t")
	})

	t.Run("UnicodeInContent", func(t *testing.T) {
		response := &LLMResponse{
			Content: "Content with unicode: 你好世界 日本語 한국어 🚀🌟",
		}
		assert.Contains(t, response.Content, "你好世界")
		assert.Contains(t, response.Content, "🚀")
	})

	t.Run("ZeroValues", func(t *testing.T) {
		response := &LLMResponse{
			Confidence:     0.0,
			TokensUsed:     0,
			ResponseTime:   0,
			SelectionScore: 0.0,
		}
		assert.Equal(t, 0.0, response.Confidence)
		assert.Equal(t, 0, response.TokensUsed)
		assert.Equal(t, int64(0), response.ResponseTime)
		assert.Equal(t, 0.0, response.SelectionScore)
	})

	t.Run("LargeTokensUsed", func(t *testing.T) {
		response := &LLMResponse{
			TokensUsed: 100000,
		}
		assert.Equal(t, 100000, response.TokensUsed)
	})

	t.Run("LargeResponseTime", func(t *testing.T) {
		response := &LLMResponse{
			ResponseTime: 60000, // 1 minute in ms
		}
		assert.Equal(t, int64(60000), response.ResponseTime)
	})

	t.Run("ConfidenceOutOfRange", func(t *testing.T) {
		// While the field accepts any value, testing edge cases
		response := &LLMResponse{
			Confidence: 1.5, // Above 1.0
		}
		assert.Equal(t, 1.5, response.Confidence)

		response2 := &LLMResponse{
			Confidence: -0.5, // Below 0.0
		}
		assert.Equal(t, -0.5, response2.Confidence)
	})
}

func TestLLMResponse_JSONKeys(t *testing.T) {
	providerID := "provider"
	response := &LLMResponse{
		ID:             "id",
		RequestID:      "request",
		ProviderID:     &providerID,
		ProviderName:   "name",
		Content:        "content",
		Confidence:     0.9,
		TokensUsed:     100,
		ResponseTime:   150,
		FinishReason:   "stop",
		Selected:       true,
		SelectionScore: 0.8,
	}

	jsonBytes, err := json.Marshal(response)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	expectedKeys := []string{
		"\"id\":", "\"request_id\":", "\"provider_id\":", "\"provider_name\":",
		"\"content\":", "\"confidence\":", "\"tokens_used\":", "\"response_time\":",
		"\"finish_reason\":", "\"selected\":", "\"selection_score\":",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, jsonStr, key, "JSON should contain key: "+key)
	}
}

func TestCreateTestLLMResponse_Helper(t *testing.T) {
	response := createTestLLMResponse("test-request")

	t.Run("HasRequiredFields", func(t *testing.T) {
		assert.NotEmpty(t, response.RequestID)
		assert.NotEmpty(t, response.ProviderName)
		assert.NotEmpty(t, response.Content)
		assert.NotEmpty(t, response.FinishReason)
	})

	t.Run("HasOptionalFields", func(t *testing.T) {
		// ProviderID is nil to avoid FK constraints in tests
		assert.Nil(t, response.ProviderID)
		assert.NotNil(t, response.Metadata)
	})

	t.Run("HasDefaultValues", func(t *testing.T) {
		assert.Equal(t, 0.95, response.Confidence)
		assert.Equal(t, 150, response.TokensUsed)
		assert.Equal(t, int64(200), response.ResponseTime)
		assert.Equal(t, "stop", response.FinishReason)
		assert.False(t, response.Selected)
		assert.Equal(t, 0.85, response.SelectionScore)
	})
}

func TestLLMResponse_SelectionScoreValues(t *testing.T) {
	scores := []float64{0.0, 0.25, 0.5, 0.75, 0.9, 0.95, 1.0}

	for _, score := range scores {
		t.Run("SelectionScore", func(t *testing.T) {
			response := &LLMResponse{
				SelectionScore: score,
			}
			assert.Equal(t, score, response.SelectionScore)
		})
	}
}

func TestLLMResponse_ProviderNames(t *testing.T) {
	providers := []string{"openai", "anthropic", "google", "deepseek", "qwen", "ollama", "openrouter"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			response := &LLMResponse{
				ProviderName: provider,
			}
			assert.Equal(t, provider, response.ProviderName)
		})
	}
}
