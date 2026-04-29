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
// Test Helper Functions for Provider Repository
// =============================================================================

func setupProviderTestDB(t *testing.T) (*pgxpool.Pool, *ProviderRepository) {
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
	repo := NewProviderRepository(pool, logger)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v (SKIP-OK: #infra-db-unavailable)", err)
		pool.Close()
		return nil, nil
	}

	// Check if required table exists
	var tableExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'llm_providers'
		)
	`).Scan(&tableExists)
	if err != nil || !tableExists {
		t.Skipf("Skipping test: llm_providers table does not exist (run migrations first) (SKIP-OK: #infra-schema-not-migrated)")
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupProviderTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM llm_providers WHERE name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup llm_providers: %v", err)
	}
}

func createTestLLMProvider() *LLMProvider {
	return &LLMProvider{
		Name:         "test-provider-" + time.Now().Format("20060102150405"),
		Type:         "openai",
		APIKey:       "test-api-key",
		BaseURL:      "https://api.test.com",
		Model:        "gpt-4",
		Weight:       1.0,
		Enabled:      true,
		Config:       map[string]interface{}{"temperature": 0.7, "max_tokens": 1000},
		HealthStatus: "healthy",
		ResponseTime: 150,
	}
}

// =============================================================================
// Integration Tests (Require Database)
// =============================================================================

func TestProviderRepository_Create(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		err := repo.Create(ctx, provider)
		assert.NoError(t, err)
		assert.NotEmpty(t, provider.ID)
		assert.False(t, provider.CreatedAt.IsZero())
		assert.False(t, provider.UpdatedAt.IsZero())
	})

	t.Run("WithNilConfig", func(t *testing.T) {
		provider := createTestLLMProvider()
		provider.Name = "test-nil-config-" + time.Now().Format("20060102150405.000")
		provider.Config = nil
		err := repo.Create(ctx, provider)
		assert.NoError(t, err)
		assert.NotEmpty(t, provider.ID)
	})
}

func TestProviderRepository_GetByID(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, provider.ID)
		assert.NoError(t, err)
		assert.Equal(t, provider.ID, fetched.ID)
		assert.Equal(t, provider.Name, fetched.Name)
		assert.Equal(t, provider.Type, fetched.Type)
		assert.Equal(t, provider.BaseURL, fetched.BaseURL)
		assert.Equal(t, provider.Model, fetched.Model)
		assert.Equal(t, provider.Weight, fetched.Weight)
		assert.Equal(t, provider.Enabled, fetched.Enabled)
		assert.Equal(t, provider.HealthStatus, fetched.HealthStatus)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRepository_GetByName(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		fetched, err := repo.GetByName(ctx, provider.Name)
		assert.NoError(t, err)
		assert.Equal(t, provider.ID, fetched.ID)
		assert.Equal(t, provider.Name, fetched.Name)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := repo.GetByName(ctx, "non-existent-name")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRepository_Update(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		provider.Name = "test-updated-" + time.Now().Format("20060102150405")
		provider.Weight = 2.0
		provider.Enabled = false
		provider.Config = map[string]interface{}{"temperature": 0.5}
		provider.HealthStatus = "unhealthy"

		err = repo.Update(ctx, provider)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, provider.ID)
		assert.NoError(t, err)
		assert.Equal(t, provider.Name, fetched.Name)
		assert.Equal(t, 2.0, fetched.Weight)
		assert.False(t, fetched.Enabled)
		assert.Equal(t, "unhealthy", fetched.HealthStatus)
	})

	t.Run("NotFound", func(t *testing.T) {
		provider := createTestLLMProvider()
		provider.ID = "00000000-0000-0000-0000-000000000000"
		err := repo.Update(ctx, provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRepository_Delete(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		err = repo.Delete(ctx, provider.ID)
		assert.NoError(t, err)

		_, err = repo.GetByID(ctx, provider.ID)
		assert.Error(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRepository_List(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create test providers
		for i := 0; i < 3; i++ {
			provider := createTestLLMProvider()
			provider.Name = "test-list-" + time.Now().Format("20060102150405.000000") + string(rune('a'+i))
			err := repo.Create(ctx, provider)
			require.NoError(t, err)
		}

		providers, total, err := repo.List(ctx, 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, 3)
		assert.GreaterOrEqual(t, len(providers), 3)
	})

	t.Run("Pagination", func(t *testing.T) {
		providers, _, err := repo.List(ctx, 2, 0)
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(providers), 2)
	})
}

func TestProviderRepository_ListEnabled(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create enabled and disabled providers
		enabledProvider := createTestLLMProvider()
		enabledProvider.Name = "test-enabled-" + time.Now().Format("20060102150405.000")
		enabledProvider.Enabled = true
		err := repo.Create(ctx, enabledProvider)
		require.NoError(t, err)

		disabledProvider := createTestLLMProvider()
		disabledProvider.Name = "test-disabled-" + time.Now().Format("20060102150405.001")
		disabledProvider.Enabled = false
		err = repo.Create(ctx, disabledProvider)
		require.NoError(t, err)

		providers, err := repo.ListEnabled(ctx)
		assert.NoError(t, err)

		// All returned providers should be enabled
		for _, p := range providers {
			assert.True(t, p.Enabled)
		}
	})
}

func TestProviderRepository_UpdateHealth(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		err = repo.UpdateHealth(ctx, provider.ID, "unhealthy", 500)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, provider.ID)
		assert.NoError(t, err)
		assert.Equal(t, "unhealthy", fetched.HealthStatus)
		assert.Equal(t, int64(500), fetched.ResponseTime)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.UpdateHealth(ctx, "00000000-0000-0000-0000-000000000000", "healthy", 100)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRepository_SetEnabled(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		provider.Enabled = true
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		err = repo.SetEnabled(ctx, provider.ID, false)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, provider.ID)
		assert.NoError(t, err)
		assert.False(t, fetched.Enabled)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.SetEnabled(ctx, "00000000-0000-0000-0000-000000000000", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRepository_UpdateWeight(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		provider := createTestLLMProvider()
		provider.Weight = 1.0
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		err = repo.UpdateWeight(ctx, provider.ID, 2.5)
		assert.NoError(t, err)

		fetched, err := repo.GetByID(ctx, provider.ID)
		assert.NoError(t, err)
		assert.Equal(t, 2.5, fetched.Weight)
	})

	t.Run("NotFound", func(t *testing.T) {
		err := repo.UpdateWeight(ctx, "00000000-0000-0000-0000-000000000000", 1.0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProviderRepository_ExistsByName(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Exists", func(t *testing.T) {
		provider := createTestLLMProvider()
		err := repo.Create(ctx, provider)
		require.NoError(t, err)

		exists, err := repo.ExistsByName(ctx, provider.Name)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("NotExists", func(t *testing.T) {
		exists, err := repo.ExistsByName(ctx, "non-existent-name")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestProviderRepository_GetHealthyProviders(t *testing.T) {
	pool, repo := setupProviderTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupProviderTestDB(t, pool)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// Create healthy provider
		healthyProvider := createTestLLMProvider()
		healthyProvider.Name = "test-healthy-" + time.Now().Format("20060102150405.000")
		healthyProvider.Enabled = true
		healthyProvider.HealthStatus = "healthy"
		err := repo.Create(ctx, healthyProvider)
		require.NoError(t, err)

		// Create unhealthy provider
		unhealthyProvider := createTestLLMProvider()
		unhealthyProvider.Name = "test-unhealthy-" + time.Now().Format("20060102150405.001")
		unhealthyProvider.Enabled = true
		unhealthyProvider.HealthStatus = "unhealthy"
		err = repo.Create(ctx, unhealthyProvider)
		require.NoError(t, err)

		providers, err := repo.GetHealthyProviders(ctx)
		assert.NoError(t, err)

		// All returned providers should be healthy and enabled
		for _, p := range providers {
			assert.True(t, p.Enabled)
			assert.Equal(t, "healthy", p.HealthStatus)
		}
	})
}

// =============================================================================
// Unit Tests (No Database Required)
// =============================================================================

func TestNewProviderRepository(t *testing.T) {
	t.Run("CreatesRepositoryWithNilPool", func(t *testing.T) {
		logger := logrus.New()
		repo := NewProviderRepository(nil, logger)
		assert.NotNil(t, repo)
	})

	t.Run("CreatesRepositoryWithNilLogger", func(t *testing.T) {
		repo := NewProviderRepository(nil, nil)
		assert.NotNil(t, repo)
	})
}

func TestLLMProvider_JSONSerialization(t *testing.T) {
	t.Run("SerializesFullProvider", func(t *testing.T) {
		provider := &LLMProvider{
			ID:           "provider-1",
			Name:         "test-provider",
			Type:         "openai",
			APIKey:       "secret-key",
			BaseURL:      "https://api.test.com",
			Model:        "gpt-4",
			Weight:       1.5,
			Enabled:      true,
			Config:       map[string]interface{}{"temperature": 0.7},
			HealthStatus: "healthy",
			ResponseTime: 200,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		jsonBytes, err := json.Marshal(provider)
		require.NoError(t, err)
		assert.Contains(t, string(jsonBytes), "test-provider")
		assert.Contains(t, string(jsonBytes), "openai")

		// API key should be omitted in JSON
		var decoded LLMProvider
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, provider.Name, decoded.Name)
	})

	t.Run("SerializesMinimalProvider", func(t *testing.T) {
		provider := &LLMProvider{
			Name: "minimal-provider",
			Type: "custom",
		}

		jsonBytes, err := json.Marshal(provider)
		require.NoError(t, err)

		var decoded LLMProvider
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "minimal-provider", decoded.Name)
	})
}

func TestLLMProvider_Fields(t *testing.T) {
	t.Run("AllFieldsSet", func(t *testing.T) {
		now := time.Now()
		provider := &LLMProvider{
			ID:           "id-1",
			Name:         "provider",
			Type:         "anthropic",
			APIKey:       "key",
			BaseURL:      "https://api.anthropic.com",
			Model:        "claude-3",
			Weight:       2.0,
			Enabled:      true,
			Config:       map[string]interface{}{"max_tokens": 4096},
			HealthStatus: "healthy",
			ResponseTime: 100,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		assert.Equal(t, "id-1", provider.ID)
		assert.Equal(t, "provider", provider.Name)
		assert.Equal(t, "anthropic", provider.Type)
		assert.Equal(t, "key", provider.APIKey)
		assert.Equal(t, "https://api.anthropic.com", provider.BaseURL)
		assert.Equal(t, "claude-3", provider.Model)
		assert.Equal(t, 2.0, provider.Weight)
		assert.True(t, provider.Enabled)
		assert.Equal(t, "healthy", provider.HealthStatus)
		assert.Equal(t, int64(100), provider.ResponseTime)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		provider := &LLMProvider{}
		assert.Empty(t, provider.ID)
		assert.Empty(t, provider.Name)
		assert.Equal(t, 0.0, provider.Weight)
		assert.False(t, provider.Enabled)
		assert.Equal(t, int64(0), provider.ResponseTime)
		assert.Nil(t, provider.Config)
	})

	t.Run("ConfigMapTypes", func(t *testing.T) {
		provider := &LLMProvider{
			Config: map[string]interface{}{
				"string_val": "test",
				"int_val":    100,
				"float_val":  0.7,
				"bool_val":   true,
				"nested_val": map[string]interface{}{"key": "value"},
				"array_val":  []interface{}{"a", "b"},
			},
		}
		assert.Equal(t, "test", provider.Config["string_val"])
		assert.Equal(t, 100, provider.Config["int_val"])
		assert.Equal(t, 0.7, provider.Config["float_val"])
		assert.Equal(t, true, provider.Config["bool_val"])
	})
}

func TestLLMProvider_ProviderTypes(t *testing.T) {
	providerTypes := []string{"openai", "anthropic", "google", "deepseek", "qwen", "ollama", "openrouter"}

	for _, pt := range providerTypes {
		t.Run(pt, func(t *testing.T) {
			provider := &LLMProvider{
				Name: "test-" + pt,
				Type: pt,
			}
			assert.Equal(t, pt, provider.Type)
		})
	}
}

func TestLLMProvider_HealthStatusValues(t *testing.T) {
	statuses := []string{"healthy", "unhealthy", "degraded", "unknown", "timeout"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			provider := &LLMProvider{
				HealthStatus: status,
			}
			assert.Equal(t, status, provider.HealthStatus)
		})
	}
}

func TestLLMProvider_WeightValues(t *testing.T) {
	weights := []float64{0.0, 0.5, 1.0, 1.5, 2.0, 10.0}

	for _, weight := range weights {
		t.Run("Weight_"+string(rune(int(weight*10)+'0')), func(t *testing.T) {
			provider := &LLMProvider{
				Weight: weight,
			}
			assert.Equal(t, weight, provider.Weight)
		})
	}
}

func TestLLMProvider_JSONRoundTrip(t *testing.T) {
	t.Run("FullRoundTrip", func(t *testing.T) {
		original := &LLMProvider{
			ID:           "round-trip-id",
			Name:         "round-trip-provider",
			Type:         "openai",
			APIKey:       "secret",
			BaseURL:      "https://api.openai.com",
			Model:        "gpt-4",
			Weight:       1.5,
			Enabled:      true,
			Config:       map[string]interface{}{"temperature": 0.7, "max_tokens": 1000},
			HealthStatus: "healthy",
			ResponseTime: 150,
			CreatedAt:    time.Now().Truncate(time.Second),
			UpdatedAt:    time.Now().Truncate(time.Second),
		}

		jsonBytes, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded LLMProvider
		err = json.Unmarshal(jsonBytes, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.ID, decoded.ID)
		assert.Equal(t, original.Name, decoded.Name)
		assert.Equal(t, original.Type, decoded.Type)
		assert.Equal(t, original.BaseURL, decoded.BaseURL)
		assert.Equal(t, original.Model, decoded.Model)
		assert.Equal(t, original.Weight, decoded.Weight)
		assert.Equal(t, original.Enabled, decoded.Enabled)
		assert.Equal(t, original.HealthStatus, decoded.HealthStatus)
		assert.Equal(t, original.ResponseTime, decoded.ResponseTime)
	})
}

func TestLLMProvider_EdgeCases(t *testing.T) {
	t.Run("EmptyConfig", func(t *testing.T) {
		provider := &LLMProvider{
			Name:   "empty-config",
			Config: map[string]interface{}{},
		}
		assert.NotNil(t, provider.Config)
		assert.Len(t, provider.Config, 0)
	})

	t.Run("VeryLongName", func(t *testing.T) {
		longName := ""
		for i := 0; i < 500; i++ {
			longName += "a"
		}
		provider := &LLMProvider{
			Name: longName,
		}
		assert.Len(t, provider.Name, 500)
	})

	t.Run("SpecialCharactersInBaseURL", func(t *testing.T) {
		provider := &LLMProvider{
			BaseURL: "https://api.test.com/v1/chat?key=value&param=test",
		}
		assert.Contains(t, provider.BaseURL, "?")
		assert.Contains(t, provider.BaseURL, "&")
	})

	t.Run("NegativeResponseTime", func(t *testing.T) {
		// While unlikely, testing the field accepts any value
		provider := &LLMProvider{
			ResponseTime: -1,
		}
		assert.Equal(t, int64(-1), provider.ResponseTime)
	})

	t.Run("VeryLargeResponseTime", func(t *testing.T) {
		provider := &LLMProvider{
			ResponseTime: 3600000, // 1 hour in ms
		}
		assert.Equal(t, int64(3600000), provider.ResponseTime)
	})
}

func TestLLMProvider_JSONKeys(t *testing.T) {
	provider := &LLMProvider{
		ID:           "id",
		Name:         "name",
		Type:         "type",
		BaseURL:      "url",
		Model:        "model",
		Weight:       1.0,
		Enabled:      true,
		HealthStatus: "healthy",
		ResponseTime: 100,
	}

	jsonBytes, err := json.Marshal(provider)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	expectedKeys := []string{
		"\"id\":", "\"name\":", "\"type\":", "\"base_url\":", "\"model\":",
		"\"weight\":", "\"enabled\":", "\"health_status\":", "\"response_time\":",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, jsonStr, key, "JSON should contain key: "+key)
	}
}

func TestCreateTestLLMProvider_Helper(t *testing.T) {
	provider := createTestLLMProvider()

	t.Run("HasRequiredFields", func(t *testing.T) {
		assert.NotEmpty(t, provider.Name)
		assert.NotEmpty(t, provider.Type)
		assert.NotEmpty(t, provider.BaseURL)
		assert.NotEmpty(t, provider.Model)
	})

	t.Run("HasDefaultValues", func(t *testing.T) {
		assert.Equal(t, 1.0, provider.Weight)
		assert.True(t, provider.Enabled)
		assert.Equal(t, "healthy", provider.HealthStatus)
	})

	t.Run("HasConfig", func(t *testing.T) {
		assert.NotNil(t, provider.Config)
		assert.NotEmpty(t, provider.Config)
	})
}
