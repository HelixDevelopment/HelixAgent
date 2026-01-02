package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, *ModelMetadataRepository) {
	ctx := context.Background()
	connString := "postgres://superagent:secret@localhost:5432/superagent_db?sslmode=disable"

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
		return nil, nil
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	repo := NewModelMetadataRepository(pool, logger)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v", err)
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()

	_, err := pool.Exec(ctx, "DELETE FROM models_refresh_history")
	if err != nil {
		t.Logf("Warning: Failed to cleanup models_refresh_history: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM model_benchmarks")
	if err != nil {
		t.Logf("Warning: Failed to cleanup model_benchmarks: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM models_metadata")
	if err != nil {
		t.Logf("Warning: Failed to cleanup models_metadata: %v", err)
	}
}

func createTestModelMetadata() *ModelMetadata {
	contextWindow := 128000
	maxTokens := 4096
	pricingInput := 3.0
	pricingOutput := 15.0
	benchmarkScore := 95.5
	popularityScore := 100
	reliabilityScore := 0.95
	modelType := "chat"
	modelFamily := "claude"
	version := "20240229"
	url := "https://models.dev/models/claude-3-sonnet"
	modelsDevID := "modelsdev-claude-3-sonnet"
	apiVersion := "v1"
	now := time.Now()

	return &ModelMetadata{
		ModelID:                 "claude-3-sonnet-20240229",
		ModelName:               "Claude 3 Sonnet",
		ProviderID:              "anthropic",
		ProviderName:            "Anthropic",
		Description:             "Claude 3 Sonnet is a balanced model",
		ContextWindow:           &contextWindow,
		MaxTokens:               &maxTokens,
		PricingInput:            &pricingInput,
		PricingOutput:           &pricingOutput,
		PricingCurrency:         "USD",
		SupportsVision:          true,
		SupportsFunctionCalling: true,
		SupportsStreaming:       true,
		SupportsJSONMode:        true,
		SupportsImageGeneration: false,
		SupportsAudio:           false,
		SupportsCodeGeneration:  true,
		SupportsReasoning:       true,
		BenchmarkScore:          &benchmarkScore,
		PopularityScore:         &popularityScore,
		ReliabilityScore:        &reliabilityScore,
		ModelType:               &modelType,
		ModelFamily:             &modelFamily,
		Version:                 &version,
		Tags:                    []string{"vision", "function-calling", "json"},
		ModelsDevURL:            &url,
		ModelsDevID:             &modelsDevID,
		ModelsDevAPIVersion:     &apiVersion,
		RawMetadata:             map[string]interface{}{"custom_field": "value"},
		LastRefreshedAt:         now,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
}

func TestModelMetadataRepository_CreateModelMetadata(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()
	metadata := createTestModelMetadata()

	t.Run("Success", func(t *testing.T) {
		err := repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		fetched, err := repo.GetModelMetadata(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Equal(t, metadata.ModelID, fetched.ModelID)
		assert.Equal(t, metadata.ModelName, fetched.ModelName)
		assert.Equal(t, metadata.ProviderID, fetched.ProviderID)
	})

	t.Run("DuplicateUpsert", func(t *testing.T) {
		err := repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		metadata.Description = "Updated description"
		err = repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		fetched, err := repo.GetModelMetadata(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated description", fetched.Description)
	})
}

func TestModelMetadataRepository_GetModelMetadata(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()
	metadata := createTestModelMetadata()

	t.Run("Success", func(t *testing.T) {
		err := repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		fetched, err := repo.GetModelMetadata(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.NotNil(t, fetched)
		assert.Equal(t, metadata.ModelID, fetched.ModelID)
		assert.Equal(t, metadata.ModelName, fetched.ModelName)
		assert.Equal(t, metadata.ProviderID, fetched.ProviderID)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetModelMetadata(ctx, "non-existent-model")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestModelMetadataRepository_ListModels(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	model1 := createTestModelMetadata()
	model1.ModelID = "model-1"
	model1.ProviderID = "anthropic"

	model2 := createTestModelMetadata()
	model2.ModelID = "model-2"
	model2.ProviderID = "openai"

	model3 := createTestModelMetadata()
	model3.ModelID = "model-3"
	model3.ProviderID = "anthropic"
	modelType := "completion"
	model3.ModelType = &modelType

	err := repo.CreateModelMetadata(ctx, model1)
	assert.NoError(t, err)
	err = repo.CreateModelMetadata(ctx, model2)
	assert.NoError(t, err)
	err = repo.CreateModelMetadata(ctx, model3)
	assert.NoError(t, err)

	t.Run("ListAll", func(t *testing.T) {
		models, total, err := repo.ListModels(ctx, "", "", 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, models, 3)
	})

	t.Run("FilterByProvider", func(t *testing.T) {
		models, total, err := repo.ListModels(ctx, "anthropic", "", 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, models, 2)
	})

	t.Run("FilterByModelType", func(t *testing.T) {
		models, total, err := repo.ListModels(ctx, "", "completion", 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, 1, total)
		if len(models) > 0 {
			assert.Equal(t, "model-3", models[0].ModelID)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		_, total, err := repo.ListModels(ctx, "", "", 0, 2)
		assert.NoError(t, err)
		assert.Equal(t, 3, total)
	})
}

func TestModelMetadataRepository_SearchModels(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	model1 := createTestModelMetadata()
	model1.ModelID = "gpt-4-turbo"
	model1.ModelName = "GPT-4 Turbo"
	model1.ProviderID = "openai"

	model2 := createTestModelMetadata()
	model2.ModelID = "gpt-3.5-turbo"
	model2.ModelName = "GPT-3.5 Turbo"
	model2.ProviderID = "openai"

	model3 := createTestModelMetadata()
	model3.ModelID = "claude-3-opus"
	model3.ModelName = "Claude 3 Opus"
	model3.ProviderID = "anthropic"

	err := repo.CreateModelMetadata(ctx, model1)
	assert.NoError(t, err)
	err = repo.CreateModelMetadata(ctx, model2)
	assert.NoError(t, err)
	err = repo.CreateModelMetadata(ctx, model3)
	assert.NoError(t, err)

	t.Run("SearchByName", func(t *testing.T) {
		models, total, err := repo.SearchModels(ctx, "GPT", 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, models, 2)
	})

	t.Run("SearchByPartialName", func(t *testing.T) {
		_, total, err := repo.SearchModels(ctx, "Turbo", 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, 2, total)
	})

	t.Run("SearchNoResults", func(t *testing.T) {
		models, total, err := repo.SearchModels(ctx, "nonexistent", 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Len(t, models, 0)
	})

	t.Run("Pagination", func(t *testing.T) {
		models, total, err := repo.SearchModels(ctx, "GPT", 0, 1)
		assert.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, models, 1)
	})
}

func TestModelMetadataRepository_UpsertModelMetadata(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()
	metadata := createTestModelMetadata()

	t.Run("Success", func(t *testing.T) {
		err := repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		fetched, err := repo.GetModelMetadata(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Equal(t, metadata.ModelID, fetched.ModelID)
	})

	t.Run("Upsert", func(t *testing.T) {
		err := repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		newScore := 98.5
		metadata.Description = "Updated description via upsert"
		metadata.BenchmarkScore = &newScore

		err = repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		fetched, err := repo.GetModelMetadata(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated description via upsert", fetched.Description)
		assert.Equal(t, 98.5, *fetched.BenchmarkScore)
	})
}

func TestModelMetadataRepository_CreateBenchmark(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()
	metadata := createTestModelMetadata()

	err := repo.CreateModelMetadata(ctx, metadata)
	assert.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		benchmarkDate := time.Now()
		score := 95.0
		rank := 1
		normalizedScore := 0.95

		benchmark := &ModelBenchmark{
			ModelID:         metadata.ModelID,
			BenchmarkName:   "MMLU",
			BenchmarkType:   func() *string { s := "reasoning"; return &s }(),
			Score:           &score,
			Rank:            &rank,
			NormalizedScore: &normalizedScore,
			BenchmarkDate:   &benchmarkDate,
			Metadata:        map[string]interface{}{"details": "test"},
		}

		err := repo.CreateBenchmark(ctx, benchmark)
		assert.NoError(t, err)
	})

	t.Run("DuplicateUpsert", func(t *testing.T) {
		benchmarkDate := time.Now()
		score := 95.0
		rank := 1
		normalizedScore := 0.95

		benchmark := &ModelBenchmark{
			ModelID:         metadata.ModelID,
			BenchmarkName:   "HellaSwag",
			BenchmarkType:   func() *string { s := "reasoning"; return &s }(),
			Score:           &score,
			Rank:            &rank,
			NormalizedScore: &normalizedScore,
			BenchmarkDate:   &benchmarkDate,
			Metadata:        map[string]interface{}{"details": "test"},
		}

		err := repo.CreateBenchmark(ctx, benchmark)
		assert.NoError(t, err)

		newScore := 96.0
		benchmark.Score = &newScore
		err = repo.CreateBenchmark(ctx, benchmark)
		assert.NoError(t, err)
	})
}

func TestModelMetadataRepository_GetBenchmarks(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()
	metadata := createTestModelMetadata()

	err := repo.CreateModelMetadata(ctx, metadata)
	assert.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		benchmarkDate := time.Now()
		score1 := 95.0
		score2 := 90.0
		rank1 := 1
		rank2 := 2
		normalizedScore1 := 0.95
		normalizedScore2 := 0.90

		benchmark1 := &ModelBenchmark{
			ModelID:         metadata.ModelID,
			BenchmarkName:   "MMLU",
			BenchmarkType:   func() *string { s := "reasoning"; return &s }(),
			Score:           &score1,
			Rank:            &rank1,
			NormalizedScore: &normalizedScore1,
			BenchmarkDate:   &benchmarkDate,
		}

		benchmark2 := &ModelBenchmark{
			ModelID:         metadata.ModelID,
			BenchmarkName:   "HellaSwag",
			BenchmarkType:   func() *string { s := "reasoning"; return &s }(),
			Score:           &score2,
			Rank:            &rank2,
			NormalizedScore: &normalizedScore2,
			BenchmarkDate:   &benchmarkDate,
		}

		err := repo.CreateBenchmark(ctx, benchmark1)
		assert.NoError(t, err)
		err = repo.CreateBenchmark(ctx, benchmark2)
		assert.NoError(t, err)

		benchmarks, err := repo.GetBenchmarks(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Len(t, benchmarks, 2)
	})

	t.Run("NotFound", func(t *testing.T) {
		benchmarks, err := repo.GetBenchmarks(ctx, "non-existent-model")
		assert.NoError(t, err)
		assert.Len(t, benchmarks, 0)
	})
}

func TestModelMetadataRepository_CreateRefreshHistory(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		startedAt := time.Now()
		duration := 120

		history := &ModelsRefreshHistory{
			RefreshType:     "full",
			Status:          "completed",
			ModelsRefreshed: 100,
			ModelsFailed:    0,
			StartedAt:       startedAt,
			DurationSeconds: &duration,
			Metadata:        map[string]interface{}{"provider": "all"},
		}

		err := repo.CreateRefreshHistory(ctx, history)
		assert.NoError(t, err)
		assert.NotEmpty(t, history.ID)
	})
}

func TestModelMetadataRepository_GetLatestRefreshHistory(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		startedAt := time.Now()
		duration1 := 120
		duration2 := 90

		history1 := &ModelsRefreshHistory{
			RefreshType:     "full",
			Status:          "completed",
			ModelsRefreshed: 100,
			ModelsFailed:    0,
			StartedAt:       startedAt,
			DurationSeconds: &duration1,
		}

		history2 := &ModelsRefreshHistory{
			RefreshType:     "provider",
			Status:          "completed",
			ModelsRefreshed: 50,
			ModelsFailed:    2,
			StartedAt:       startedAt.Add(-time.Hour),
			DurationSeconds: &duration2,
		}

		err := repo.CreateRefreshHistory(ctx, history1)
		assert.NoError(t, err)
		err = repo.CreateRefreshHistory(ctx, history2)
		assert.NoError(t, err)

		histories, err := repo.GetLatestRefreshHistory(ctx, 10)
		assert.NoError(t, err)
		assert.Len(t, histories, 2)
		assert.Equal(t, "full", histories[0].RefreshType)
	})

	t.Run("Limit", func(t *testing.T) {
		startedAt := time.Now()

		for i := 0; i < 5; i++ {
			duration := 60 + i*10
			history := &ModelsRefreshHistory{
				RefreshType:     "test",
				Status:          "completed",
				ModelsRefreshed: 10,
				ModelsFailed:    0,
				StartedAt:       startedAt.Add(time.Duration(i) * time.Minute),
				DurationSeconds: &duration,
			}
			err := repo.CreateRefreshHistory(ctx, history)
			assert.NoError(t, err)
		}

		histories, err := repo.GetLatestRefreshHistory(ctx, 3)
		assert.NoError(t, err)
		assert.Len(t, histories, 3)
	})
}

func TestModelMetadataRepository_UpdateProviderSyncInfo(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		err := repo.UpdateProviderSyncInfo(ctx, "anthropic", 100, 95)
		assert.NoError(t, err)
	})
}

func TestModelMetadataRepository_Integration(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	t.Run("FullWorkflow", func(t *testing.T) {
		metadata := createTestModelMetadata()
		metadata.ModelID = "integration-test-model"

		err := repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		fetched, err := repo.GetModelMetadata(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Equal(t, metadata.ModelID, fetched.ModelID)

		metadata.Description = "Updated via integration test"
		err = repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		benchmarkDate := time.Now()
		score := 97.0
		rank := 1
		normalizedScore := 0.97

		benchmark := &ModelBenchmark{
			ModelID:         metadata.ModelID,
			BenchmarkName:   "IntegrationTest",
			BenchmarkType:   func() *string { s := "custom"; return &s }(),
			Score:           &score,
			Rank:            &rank,
			NormalizedScore: &normalizedScore,
			BenchmarkDate:   &benchmarkDate,
		}

		err = repo.CreateBenchmark(ctx, benchmark)
		assert.NoError(t, err)

		benchmarks, err := repo.GetBenchmarks(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Len(t, benchmarks, 1)
		assert.Equal(t, "IntegrationTest", benchmarks[0].BenchmarkName)

		startedAt := time.Now()
		duration := 60

		history := &ModelsRefreshHistory{
			RefreshType:     "test",
			Status:          "completed",
			ModelsRefreshed: 1,
			ModelsFailed:    0,
			StartedAt:       startedAt,
			DurationSeconds: &duration,
		}

		err = repo.CreateRefreshHistory(ctx, history)
		assert.NoError(t, err)

		histories, err := repo.GetLatestRefreshHistory(ctx, 10)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(histories), 1)
	})
}

func TestModelMetadataRepository_NullableFields(t *testing.T) {
	pool, repo := setupTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	t.Run("AllNullableFields", func(t *testing.T) {
		metadata := &ModelMetadata{
			ModelID:      "minimal-model",
			ModelName:    "Minimal Model",
			ProviderID:   "test-provider",
			ProviderName: "Test Provider",
			Description:  "Minimal model with nullable fields",
			Tags:         []string{},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		err := repo.CreateModelMetadata(ctx, metadata)
		assert.NoError(t, err)

		fetched, err := repo.GetModelMetadata(ctx, metadata.ModelID)
		assert.NoError(t, err)
		assert.Nil(t, fetched.ContextWindow)
		assert.Nil(t, fetched.MaxTokens)
		assert.Nil(t, fetched.BenchmarkScore)
		assert.Nil(t, fetched.PopularityScore)
	})
}

// =============================================================================
// Unit Tests (No Database Required)
// =============================================================================

// TestNewModelMetadataRepository tests repository creation
func TestNewModelMetadataRepository(t *testing.T) {
	t.Run("CreatesRepositoryWithNilPool", func(t *testing.T) {
		logger := logrus.New()
		repo := NewModelMetadataRepository(nil, logger)
		assert.NotNil(t, repo)
	})

	t.Run("CreatesRepositoryWithNilLogger", func(t *testing.T) {
		repo := NewModelMetadataRepository(nil, nil)
		assert.NotNil(t, repo)
	})
}

// TestModelMetadata_JSONSerialization tests JSON serialization
func TestModelMetadata_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullModelMetadata", func(t *testing.T) {
		contextWindow := 128000
		maxTokens := 4096
		pricingInput := 3.0
		pricingOutput := 15.0
		benchmarkScore := 95.5
		popularityScore := 100
		reliabilityScore := 0.95
		modelType := "chat"
		modelFamily := "claude"
		version := "20240229"
		url := "https://models.dev/models/claude-3-sonnet"
		modelsDevID := "modelsdev-claude-3-sonnet"
		apiVersion := "v1"
		now := time.Now()

		metadata := &ModelMetadata{
			ID:                      "test-id",
			ModelID:                 "claude-3-sonnet-20240229",
			ModelName:               "Claude 3 Sonnet",
			ProviderID:              "anthropic",
			ProviderName:            "Anthropic",
			Description:             "Claude 3 Sonnet is a balanced model",
			ContextWindow:           &contextWindow,
			MaxTokens:               &maxTokens,
			PricingInput:            &pricingInput,
			PricingOutput:           &pricingOutput,
			PricingCurrency:         "USD",
			SupportsVision:          true,
			SupportsFunctionCalling: true,
			SupportsStreaming:       true,
			SupportsJSONMode:        true,
			SupportsImageGeneration: false,
			SupportsAudio:           false,
			SupportsCodeGeneration:  true,
			SupportsReasoning:       true,
			BenchmarkScore:          &benchmarkScore,
			PopularityScore:         &popularityScore,
			ReliabilityScore:        &reliabilityScore,
			ModelType:               &modelType,
			ModelFamily:             &modelFamily,
			Version:                 &version,
			Tags:                    []string{"vision", "function-calling", "json"},
			ModelsDevURL:            &url,
			ModelsDevID:             &modelsDevID,
			ModelsDevAPIVersion:     &apiVersion,
			RawMetadata:             map[string]interface{}{"custom_field": "value"},
			LastRefreshedAt:         now,
			CreatedAt:               now,
			UpdatedAt:               now,
		}

		jsonBytes, err := json.Marshal(metadata)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "claude-3-sonnet-20240229")
		assert.Contains(t, string(jsonBytes), "anthropic")

		// Deserialize back
		var decoded ModelMetadata
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, metadata.ModelID, decoded.ModelID)
		assert.Equal(t, metadata.ProviderID, decoded.ProviderID)
		assert.Equal(t, *metadata.ContextWindow, *decoded.ContextWindow)
	})

	t.Run("SerializesMinimalModelMetadata", func(t *testing.T) {
		metadata := &ModelMetadata{
			ModelID:      "minimal-model",
			ModelName:    "Minimal Model",
			ProviderID:   "test-provider",
			ProviderName: "Test Provider",
		}

		jsonBytes, err := json.Marshal(metadata)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "minimal-model")

		var decoded ModelMetadata
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "minimal-model", decoded.ModelID)
		assert.Nil(t, decoded.ContextWindow)
		assert.Nil(t, decoded.MaxTokens)
	})
}

// TestModelBenchmark_JSONSerialization tests benchmark JSON serialization
func TestModelBenchmark_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullBenchmark", func(t *testing.T) {
		score := 95.0
		rank := 1
		normalizedScore := 0.95
		benchmarkDate := time.Now()
		benchmarkType := "reasoning"

		benchmark := &ModelBenchmark{
			ID:              "bench-1",
			ModelID:         "model-1",
			BenchmarkName:   "MMLU",
			BenchmarkType:   &benchmarkType,
			Score:           &score,
			Rank:            &rank,
			NormalizedScore: &normalizedScore,
			BenchmarkDate:   &benchmarkDate,
			Metadata:        map[string]interface{}{"details": "test"},
			CreatedAt:       time.Now(),
		}

		jsonBytes, err := json.Marshal(benchmark)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "MMLU")
		assert.Contains(t, string(jsonBytes), "model-1")

		var decoded ModelBenchmark
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "MMLU", decoded.BenchmarkName)
		assert.Equal(t, 95.0, *decoded.Score)
	})

	t.Run("SerializesMinimalBenchmark", func(t *testing.T) {
		benchmark := &ModelBenchmark{
			ModelID:       "model-1",
			BenchmarkName: "MMLU",
		}

		jsonBytes, err := json.Marshal(benchmark)
		require.NoError(t, err)

		var decoded ModelBenchmark
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "MMLU", decoded.BenchmarkName)
		assert.Nil(t, decoded.Score)
	})
}

// TestModelsRefreshHistory_JSONSerialization tests refresh history JSON serialization
func TestModelsRefreshHistory_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullRefreshHistory", func(t *testing.T) {
		startedAt := time.Now()
		completedAt := time.Now()
		duration := 120
		errorMessage := "test error"

		history := &ModelsRefreshHistory{
			ID:              "history-1",
			RefreshType:     "full",
			Status:          "completed",
			ModelsRefreshed: 100,
			ModelsFailed:    5,
			ErrorMessage:    &errorMessage,
			StartedAt:       startedAt,
			CompletedAt:     &completedAt,
			DurationSeconds: &duration,
			Metadata:        map[string]interface{}{"provider": "all"},
		}

		jsonBytes, err := json.Marshal(history)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "full")
		assert.Contains(t, string(jsonBytes), "completed")

		var decoded ModelsRefreshHistory
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "full", decoded.RefreshType)
		assert.Equal(t, 100, decoded.ModelsRefreshed)
	})

	t.Run("SerializesMinimalRefreshHistory", func(t *testing.T) {
		history := &ModelsRefreshHistory{
			RefreshType:     "provider",
			Status:          "running",
			ModelsRefreshed: 0,
			ModelsFailed:    0,
			StartedAt:       time.Now(),
		}

		jsonBytes, err := json.Marshal(history)
		require.NoError(t, err)

		var decoded ModelsRefreshHistory
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "provider", decoded.RefreshType)
		assert.Nil(t, decoded.ErrorMessage)
	})
}

// TestModelMetadata_Fields tests individual fields
func TestModelMetadata_Fields(t *testing.T) {
	t.Run("TagsAreSlice", func(t *testing.T) {
		metadata := &ModelMetadata{
			ModelID: "test",
			Tags:    []string{"tag1", "tag2", "tag3"},
		}
		assert.Len(t, metadata.Tags, 3)
		assert.Contains(t, metadata.Tags, "tag1")
	})

	t.Run("RawMetadataIsMap", func(t *testing.T) {
		metadata := &ModelMetadata{
			ModelID: "test",
			RawMetadata: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
		}
		assert.Equal(t, "value1", metadata.RawMetadata["key1"])
		assert.Equal(t, 42, metadata.RawMetadata["key2"])
		assert.Equal(t, true, metadata.RawMetadata["key3"])
	})

	t.Run("BooleanFieldsDefault", func(t *testing.T) {
		metadata := &ModelMetadata{ModelID: "test"}
		assert.False(t, metadata.SupportsVision)
		assert.False(t, metadata.SupportsFunctionCalling)
		assert.False(t, metadata.SupportsStreaming)
		assert.False(t, metadata.SupportsJSONMode)
		assert.False(t, metadata.SupportsImageGeneration)
		assert.False(t, metadata.SupportsAudio)
		assert.False(t, metadata.SupportsCodeGeneration)
		assert.False(t, metadata.SupportsReasoning)
	})

	t.Run("NilPointerFields", func(t *testing.T) {
		metadata := &ModelMetadata{ModelID: "test"}
		assert.Nil(t, metadata.ContextWindow)
		assert.Nil(t, metadata.MaxTokens)
		assert.Nil(t, metadata.PricingInput)
		assert.Nil(t, metadata.PricingOutput)
		assert.Nil(t, metadata.BenchmarkScore)
		assert.Nil(t, metadata.PopularityScore)
		assert.Nil(t, metadata.ReliabilityScore)
		assert.Nil(t, metadata.ModelType)
		assert.Nil(t, metadata.ModelFamily)
		assert.Nil(t, metadata.Version)
		assert.Nil(t, metadata.ModelsDevURL)
		assert.Nil(t, metadata.ModelsDevID)
		assert.Nil(t, metadata.ModelsDevAPIVersion)
	})
}

// TestModelBenchmark_Fields tests benchmark fields
func TestModelBenchmark_Fields(t *testing.T) {
	t.Run("MetadataIsMap", func(t *testing.T) {
		benchmark := &ModelBenchmark{
			ModelID:       "test",
			BenchmarkName: "MMLU",
			Metadata: map[string]interface{}{
				"source": "test",
				"count":  100,
			},
		}
		assert.Equal(t, "test", benchmark.Metadata["source"])
		assert.Equal(t, 100, benchmark.Metadata["count"])
	})

	t.Run("NilPointerFields", func(t *testing.T) {
		benchmark := &ModelBenchmark{ModelID: "test", BenchmarkName: "MMLU"}
		assert.Nil(t, benchmark.BenchmarkType)
		assert.Nil(t, benchmark.Score)
		assert.Nil(t, benchmark.Rank)
		assert.Nil(t, benchmark.NormalizedScore)
		assert.Nil(t, benchmark.BenchmarkDate)
	})
}

// TestModelsRefreshHistory_Fields tests refresh history fields
func TestModelsRefreshHistory_Fields(t *testing.T) {
	t.Run("RequiredFields", func(t *testing.T) {
		history := &ModelsRefreshHistory{
			RefreshType:     "full",
			Status:          "running",
			ModelsRefreshed: 50,
			ModelsFailed:    2,
			StartedAt:       time.Now(),
		}
		assert.Equal(t, "full", history.RefreshType)
		assert.Equal(t, "running", history.Status)
		assert.Equal(t, 50, history.ModelsRefreshed)
		assert.Equal(t, 2, history.ModelsFailed)
		assert.False(t, history.StartedAt.IsZero())
	})

	t.Run("NilPointerFields", func(t *testing.T) {
		history := &ModelsRefreshHistory{
			RefreshType: "test",
			Status:      "pending",
			StartedAt:   time.Now(),
		}
		assert.Nil(t, history.ErrorMessage)
		assert.Nil(t, history.CompletedAt)
		assert.Nil(t, history.DurationSeconds)
	})

	t.Run("MetadataIsMap", func(t *testing.T) {
		history := &ModelsRefreshHistory{
			RefreshType: "test",
			Status:      "completed",
			StartedAt:   time.Now(),
			Metadata: map[string]interface{}{
				"provider": "anthropic",
				"models":   100,
			},
		}
		assert.Equal(t, "anthropic", history.Metadata["provider"])
		assert.Equal(t, 100, history.Metadata["models"])
	})
}

// TestCreateTestModelMetadata_Helper tests the helper function
func TestCreateTestModelMetadata_Helper(t *testing.T) {
	metadata := createTestModelMetadata()

	t.Run("HasRequiredFields", func(t *testing.T) {
		assert.NotEmpty(t, metadata.ModelID)
		assert.NotEmpty(t, metadata.ModelName)
		assert.NotEmpty(t, metadata.ProviderID)
		assert.NotEmpty(t, metadata.ProviderName)
	})

	t.Run("HasOptionalFields", func(t *testing.T) {
		assert.NotNil(t, metadata.ContextWindow)
		assert.NotNil(t, metadata.MaxTokens)
		assert.NotNil(t, metadata.BenchmarkScore)
	})

	t.Run("HasTags", func(t *testing.T) {
		assert.NotEmpty(t, metadata.Tags)
	})

	t.Run("HasBooleanCapabilities", func(t *testing.T) {
		assert.True(t, metadata.SupportsVision)
		assert.True(t, metadata.SupportsFunctionCalling)
		assert.True(t, metadata.SupportsStreaming)
	})
}
