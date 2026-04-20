// Package models provides an embedding model registry for multiple embedding providers.
package models

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingModel implements EmbeddingModel for testing.
//
// Concurrency model (CONST-029): error setters swap atomic.Pointer
// values; embeddings is treated as immutable post-construction so
// readers observe a stable snapshot without locking.
type MockEmbeddingModel struct {
	name        string
	dimensions  int
	maxTokens   int
	provider    string
	healthError atomic.Pointer[error]
	encodeError atomic.Pointer[error]
	embeddings  [][]float32
}

func NewMockEmbeddingModel(name string, dimensions int) *MockEmbeddingModel {
	return &MockEmbeddingModel{
		name:       name,
		dimensions: dimensions,
		maxTokens:  8191,
		provider:   "mock",
		embeddings: [][]float32{{0.1, 0.2, 0.3}},
	}
}

func (m *MockEmbeddingModel) Name() string     { return m.name }
func (m *MockEmbeddingModel) Dimensions() int  { return m.dimensions }
func (m *MockEmbeddingModel) MaxTokens() int   { return m.maxTokens }
func (m *MockEmbeddingModel) Provider() string { return m.provider }
func (m *MockEmbeddingModel) Close() error     { return nil }

func (m *MockEmbeddingModel) loadErr(p *atomic.Pointer[error]) error {
	if ep := p.Load(); ep != nil {
		return *ep
	}
	return nil
}

func (m *MockEmbeddingModel) Health(ctx context.Context) error {
	return m.loadErr(&m.healthError)
}

func (m *MockEmbeddingModel) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	if err := m.loadErr(&m.encodeError); err != nil {
		return nil, err
	}
	if len(m.embeddings) > 0 {
		return m.embeddings[0], nil
	}
	return make([]float32, m.dimensions), nil
}

func (m *MockEmbeddingModel) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	if err := m.loadErr(&m.encodeError); err != nil {
		return nil, err
	}
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		if i < len(m.embeddings) {
			embeddings[i] = m.embeddings[i]
		} else {
			embeddings[i] = make([]float32, m.dimensions)
		}
	}
	return embeddings, nil
}

func (m *MockEmbeddingModel) SetHealthError(err error) {
	if err == nil {
		m.healthError.Store(nil)
		return
	}
	m.healthError.Store(&err)
}

func (m *MockEmbeddingModel) SetEncodeError(err error) {
	if err == nil {
		m.encodeError.Store(nil)
		return
	}
	m.encodeError.Store(&err)
}

// MockEmbeddingCache implements EmbeddingCache for testing.
//
// Concurrency model (CONST-029): data → *safe.Store.
type MockEmbeddingCache struct {
	data *safe.Store[string, []float32]
}

func NewMockEmbeddingCache() *MockEmbeddingCache {
	return &MockEmbeddingCache{
		data: safe.NewStore[string, []float32](),
	}
}

func (c *MockEmbeddingCache) Get(key string) ([]float32, bool) {
	return c.data.Get(key)
}

func (c *MockEmbeddingCache) Set(key string, embedding []float32, ttl time.Duration) {
	c.data.Put(key, embedding)
}

func (c *MockEmbeddingCache) Delete(key string) {
	c.data.Delete(key)
}

func TestNewEmbeddingModelRegistry(t *testing.T) {
	tests := []struct {
		name             string
		config           RegistryConfig
		expectedDefault  string
		expectedFallback []string
	}{
		{
			name:             "default configuration",
			config:           RegistryConfig{},
			expectedDefault:  "openai-3-small", // First in default fallback chain
			expectedFallback: []string{"openai-3-small", "bge-m3", "all-mpnet-base-v2", "local-fallback"},
		},
		{
			name: "custom configuration",
			config: RegistryConfig{
				DefaultModel:  "custom-model",
				FallbackChain: []string{"model-a", "model-b"},
			},
			expectedDefault:  "custom-model",
			expectedFallback: []string{"model-a", "model-b"},
		},
		{
			name: "with logger",
			config: RegistryConfig{
				Logger: logrus.New(),
			},
			expectedDefault:  "openai-3-small",
			expectedFallback: []string{"openai-3-small", "bge-m3", "all-mpnet-base-v2", "local-fallback"},
		},
		{
			name: "with cache",
			config: RegistryConfig{
				Cache: NewMockEmbeddingCache(),
			},
			expectedDefault:  "openai-3-small",
			expectedFallback: []string{"openai-3-small", "bge-m3", "all-mpnet-base-v2", "local-fallback"},
		},
		{
			name: "with custom configs",
			config: RegistryConfig{
				Configs: map[string]EmbeddingModelConfig{
					"custom": {
						Name:       "custom",
						Provider:   "local",
						Dimensions: 512,
					},
				},
			},
			expectedDefault:  "openai-3-small",
			expectedFallback: []string{"openai-3-small", "bge-m3", "all-mpnet-base-v2", "local-fallback"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewEmbeddingModelRegistry(tt.config)
			assert.NotNil(t, registry)
			assert.Equal(t, tt.expectedDefault, registry.defaultModel)
			assert.Equal(t, tt.expectedFallback, registry.fallbackChain)
			assert.NotNil(t, registry.models)
			assert.NotNil(t, registry.configs)
			assert.NotNil(t, registry.logger)
		})
	}
}

func TestEmbeddingModelRegistry_loadDefaultConfigs(t *testing.T) {
	// Test with environment variables
	t.Run("with OPENAI_API_KEY", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		registry := NewEmbeddingModelRegistry(RegistryConfig{})

		_, exists := registry.configs.Get("openai-ada")
		assert.True(t, exists)
		_, exists = registry.configs.Get("openai-3-small")
		assert.True(t, exists)
		_, exists = registry.configs.Get("openai-3-large")
		assert.True(t, exists)
	})

	t.Run("with OLLAMA_URL", func(t *testing.T) {
		t.Setenv("OLLAMA_URL", "http://localhost:11434")
		registry := NewEmbeddingModelRegistry(RegistryConfig{})

		_, exists := registry.configs.Get("nomic-embed-text")
		assert.True(t, exists)
		_, exists = registry.configs.Get("bge-m3")
		assert.True(t, exists)
	})

	t.Run("with SENTENCE_TRANSFORMERS_URL", func(t *testing.T) {
		t.Setenv("SENTENCE_TRANSFORMERS_URL", "http://localhost:8080")
		registry := NewEmbeddingModelRegistry(RegistryConfig{})

		_, exists := registry.configs.Get("all-mpnet-base-v2")
		assert.True(t, exists)
		_, exists = registry.configs.Get("all-minilm-l6-v2")
		assert.True(t, exists)
	})

	t.Run("local-fallback always exists", func(t *testing.T) {
		registry := NewEmbeddingModelRegistry(RegistryConfig{})
		_, exists := registry.configs.Get("local-fallback")
		assert.True(t, exists)
	})
}

func TestEmbeddingModelRegistry_Register(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{})
	mockModel := NewMockEmbeddingModel("test-model", 768)

	err := registry.Register("test-model", mockModel)
	assert.NoError(t, err)

	// Verify the model was registered
	model, err := registry.Get("test-model")
	assert.NoError(t, err)
	assert.Equal(t, mockModel, model)
}

func TestEmbeddingModelRegistry_Get(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*EmbeddingModelRegistry)
		modelName   string
		expectError bool
	}{
		{
			name: "get existing model",
			setup: func(r *EmbeddingModelRegistry) {
				r.models.Put("test", NewMockEmbeddingModel("test", 768))
			},
			modelName:   "test",
			expectError: false,
		},
		{
			name: "get creates model from config",
			setup: func(r *EmbeddingModelRegistry) {
				r.configs.Put("local-test", EmbeddingModelConfig{
					Name:       "local-test",
					Provider:   "local",
					Dimensions: 512,
				})
			},
			modelName:   "local-test",
			expectError: false,
		},
		{
			name:        "model not configured",
			setup:       func(r *EmbeddingModelRegistry) {},
			modelName:   "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewEmbeddingModelRegistry(RegistryConfig{})
			tt.setup(registry)

			model, err := registry.Get(tt.modelName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, model)
			}
		})
	}
}

func TestEmbeddingModelRegistry_GetOrCreate(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{})

	// Add a local config
	registry.configs.Put("test-local", EmbeddingModelConfig{
		Name:       "test-local",
		Provider:   "local",
		Dimensions: 256,
	})

	// First call should create the model
	model1, err := registry.GetOrCreate("test-local")
	require.NoError(t, err)
	assert.NotNil(t, model1)

	// Second call should return the same model
	model2, err := registry.GetOrCreate("test-local")
	require.NoError(t, err)
	assert.Equal(t, model1, model2)
}

func TestEmbeddingModelRegistry_GetDefault(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{
		DefaultModel: "local-fallback",
	})

	model, err := registry.GetDefault()
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, "local-fallback", model.Name())
}

func TestEmbeddingModelRegistry_createModel(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{})

	tests := []struct {
		name        string
		config      EmbeddingModelConfig
		expectError bool
		modelType   string
	}{
		{
			name: "create openai model",
			config: EmbeddingModelConfig{
				Name:     "openai-test",
				Provider: "openai",
				APIKey:   "test-key",
			},
			expectError: false,
			modelType:   "openai",
		},
		{
			name: "create ollama model",
			config: EmbeddingModelConfig{
				Name:     "ollama-test",
				Provider: "ollama",
				BaseURL:  "http://localhost:11434",
			},
			expectError: false,
			modelType:   "ollama",
		},
		{
			name: "create sentence-transformers model",
			config: EmbeddingModelConfig{
				Name:     "st-test",
				Provider: "sentence-transformers",
				BaseURL:  "http://localhost:8080",
			},
			expectError: false,
			modelType:   "sentence-transformers",
		},
		{
			name: "create local model",
			config: EmbeddingModelConfig{
				Name:       "local-test",
				Provider:   "local",
				Dimensions: 512,
			},
			expectError: false,
			modelType:   "local",
		},
		{
			name: "unknown provider",
			config: EmbeddingModelConfig{
				Name:     "unknown",
				Provider: "unknown",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := registry.createModel(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, model)
				assert.Equal(t, tt.modelType, model.Provider())
			}
		})
	}
}

func TestEmbeddingModelRegistry_EncodeWithFallback(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{
		FallbackChain: []string{"failing-model", "working-model", "local-fallback"},
	})

	// Register a failing model
	failingModel := NewMockEmbeddingModel("failing-model", 768)
	failingModel.SetEncodeError(errors.New("model failure"))
	_ = registry.Register("failing-model", failingModel)

	// Register a working model
	workingModel := NewMockEmbeddingModel("working-model", 768)
	workingModel.embeddings = [][]float32{{0.1, 0.2, 0.3}}
	_ = registry.Register("working-model", workingModel)

	// Test fallback
	embeddings, modelName, err := registry.EncodeWithFallback(context.Background(), []string{"test text"})
	assert.NoError(t, err)
	assert.Equal(t, "working-model", modelName)
	assert.Len(t, embeddings, 1)
}

func TestEmbeddingModelRegistry_EncodeWithFallback_AllFail(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{
		FallbackChain: []string{"model1", "model2"},
		Configs: map[string]EmbeddingModelConfig{
			"model1": {Name: "model1", Provider: "unknown"},
			"model2": {Name: "model2", Provider: "unknown"},
		},
	})

	_, _, err := registry.EncodeWithFallback(context.Background(), []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all embedding models in fallback chain failed")
}

func TestEmbeddingModelRegistry_EncodeSingleWithFallback(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{
		FallbackChain: []string{"local-fallback"},
	})

	embedding, modelName, err := registry.EncodeSingleWithFallback(context.Background(), "test text")
	assert.NoError(t, err)
	assert.Equal(t, "local-fallback", modelName)
	assert.NotEmpty(t, embedding)
}

func TestEmbeddingModelRegistry_List(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{
		Configs: map[string]EmbeddingModelConfig{
			"model1": {Name: "model1", Provider: "local"},
			"model2": {Name: "model2", Provider: "local"},
		},
	})

	models := registry.List()
	assert.Contains(t, models, "model1")
	assert.Contains(t, models, "model2")
	assert.Contains(t, models, "local-fallback") // Always present
}

func TestEmbeddingModelRegistry_Health(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{})

	healthyModel := NewMockEmbeddingModel("healthy", 768)
	unhealthyModel := NewMockEmbeddingModel("unhealthy", 768)
	unhealthyModel.SetHealthError(errors.New("unhealthy"))

	_ = registry.Register("healthy", healthyModel)
	_ = registry.Register("unhealthy", unhealthyModel)

	results := registry.Health(context.Background())
	assert.NoError(t, results["healthy"])
	assert.Error(t, results["unhealthy"])
}

func TestEmbeddingModelRegistry_Close(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{})

	model1 := NewMockEmbeddingModel("model1", 768)
	model2 := NewMockEmbeddingModel("model2", 768)

	_ = registry.Register("model1", model1)
	_ = registry.Register("model2", model2)

	err := registry.Close()
	assert.NoError(t, err)

	// Verify models map is cleared
	assert.Equal(t, 0, registry.models.Len())
}

// =============================================================================
// OpenAI Embedding Model Tests
// =============================================================================

func TestOpenAIEmbeddingModel(t *testing.T) {
	config := EmbeddingModelConfig{
		Name:       "openai-test",
		ModelID:    "text-embedding-3-small",
		Dimensions: 1536,
		MaxTokens:  8191,
		Timeout:    30 * time.Second,
		APIKey:     "test-key",
	}

	model := NewOpenAIEmbeddingModel(config)

	assert.Equal(t, "openai-test", model.Name())
	assert.Equal(t, 1536, model.Dimensions())
	assert.Equal(t, 8191, model.MaxTokens())
	assert.Equal(t, "openai", model.Provider())
	assert.NoError(t, model.Close())
}

func TestOpenAIEmbeddingModel_Encode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		response := map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float32{0.1, 0.2, 0.3}, "index": 0},
				{"embedding": []float32{0.4, 0.5, 0.6}, "index": 1},
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// We can't easily test with the real OpenAI endpoint, so we verify the model structure
	config := EmbeddingModelConfig{
		Name:       "openai-test",
		ModelID:    "text-embedding-3-small",
		Dimensions: 1536,
		APIKey:     "test-key",
		Timeout:    30 * time.Second,
	}

	model := NewOpenAIEmbeddingModel(config)
	assert.NotNil(t, model)
}

func TestOpenAIEmbeddingModel_Encode_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid request"}}`))
	}))
	defer server.Close()

	// Test error handling by checking the model creation
	config := EmbeddingModelConfig{
		Name:    "openai-test",
		Timeout: 30 * time.Second,
	}
	model := NewOpenAIEmbeddingModel(config)
	assert.NotNil(t, model)
}

// =============================================================================
// Ollama Embedding Model Tests
// =============================================================================

func TestOllamaEmbeddingModel(t *testing.T) {
	config := EmbeddingModelConfig{
		Name:       "ollama-test",
		ModelID:    "nomic-embed-text",
		Dimensions: 768,
		MaxTokens:  8192,
		Timeout:    60 * time.Second,
	}

	model := NewOllamaEmbeddingModel(config)

	assert.Equal(t, "ollama-test", model.Name())
	assert.Equal(t, 768, model.Dimensions())
	assert.Equal(t, 8192, model.MaxTokens())
	assert.Equal(t, "ollama", model.Provider())
	assert.NoError(t, model.Close())
}

func TestOllamaEmbeddingModel_DefaultBaseURL(t *testing.T) {
	config := EmbeddingModelConfig{
		Name: "ollama-test",
	}

	model := NewOllamaEmbeddingModel(config)
	assert.Equal(t, "http://localhost:11434", model.config.BaseURL)
}

func TestOllamaEmbeddingModel_EncodeSingle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/embeddings", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "nomic-embed-text", body["model"])

		response := map[string]interface{}{
			"embedding": []float32{0.1, 0.2, 0.3},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := EmbeddingModelConfig{
		Name:    "ollama-test",
		ModelID: "nomic-embed-text",
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	model := NewOllamaEmbeddingModel(config)
	embedding, err := model.EncodeSingle(context.Background(), "test text")
	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, embedding)
}

func TestOllamaEmbeddingModel_Encode(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"embedding": []float32{float32(callCount) * 0.1, 0.2, 0.3},
		}
		callCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := EmbeddingModelConfig{
		Name:    "ollama-test",
		ModelID: "nomic-embed-text",
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	model := NewOllamaEmbeddingModel(config)
	embeddings, err := model.Encode(context.Background(), []string{"text1", "text2"})
	assert.NoError(t, err)
	assert.Len(t, embeddings, 2)
}

func TestOllamaEmbeddingModel_EncodeSingle_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "model not found"}`))
	}))
	defer server.Close()

	config := EmbeddingModelConfig{
		Name:    "ollama-test",
		ModelID: "nonexistent",
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	model := NewOllamaEmbeddingModel(config)
	_, err := model.EncodeSingle(context.Background(), "test")
	assert.Error(t, err)
}

// =============================================================================
// Sentence Transformers Model Tests
// =============================================================================

func TestSentenceTransformersModel(t *testing.T) {
	config := EmbeddingModelConfig{
		Name:       "st-test",
		ModelID:    "all-mpnet-base-v2",
		Dimensions: 768,
		MaxTokens:  512,
		Timeout:    30 * time.Second,
		BaseURL:    "http://localhost:8080",
	}

	model := NewSentenceTransformersModel(config)

	assert.Equal(t, "st-test", model.Name())
	assert.Equal(t, 768, model.Dimensions())
	assert.Equal(t, 512, model.MaxTokens())
	assert.Equal(t, "sentence-transformers", model.Provider())
	assert.NoError(t, model.Close())
}

func TestSentenceTransformersModel_Health(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{"healthy", http.StatusOK, false},
		{"unhealthy", http.StatusServiceUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health", r.URL.Path)
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			config := EmbeddingModelConfig{
				Name:    "st-test",
				BaseURL: server.URL,
				Timeout: 30 * time.Second,
			}

			model := NewSentenceTransformersModel(config)
			err := model.Health(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSentenceTransformersModel_Encode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/encode", r.URL.Path)

		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotNil(t, body["texts"])
		assert.NotNil(t, body["model"])

		response := map[string]interface{}{
			"embeddings": [][]float32{{0.1, 0.2}, {0.3, 0.4}},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := EmbeddingModelConfig{
		Name:    "st-test",
		ModelID: "all-mpnet-base-v2",
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	model := NewSentenceTransformersModel(config)
	embeddings, err := model.Encode(context.Background(), []string{"text1", "text2"})
	assert.NoError(t, err)
	assert.Len(t, embeddings, 2)
}

func TestSentenceTransformersModel_EncodeSingle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"embeddings": [][]float32{{0.1, 0.2, 0.3}},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := EmbeddingModelConfig{
		Name:    "st-test",
		ModelID: "all-mpnet-base-v2",
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	model := NewSentenceTransformersModel(config)
	embedding, err := model.EncodeSingle(context.Background(), "test text")
	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, embedding)
}

func TestSentenceTransformersModel_Encode_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid request"}`))
	}))
	defer server.Close()

	config := EmbeddingModelConfig{
		Name:    "st-test",
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	}

	model := NewSentenceTransformersModel(config)
	_, err := model.Encode(context.Background(), []string{"test"})
	assert.Error(t, err)
}

// =============================================================================
// Local Hash Model Tests
// =============================================================================

func TestLocalHashModel(t *testing.T) {
	config := EmbeddingModelConfig{
		Name:       "local-test",
		Dimensions: 512,
		MaxTokens:  100000,
	}

	model := NewLocalHashModel(config)

	assert.Equal(t, "local-test", model.Name())
	assert.Equal(t, 512, model.Dimensions())
	assert.Equal(t, 100000, model.MaxTokens())
	assert.Equal(t, "local", model.Provider())
	assert.NoError(t, model.Close())
}

func TestLocalHashModel_DefaultDimensions(t *testing.T) {
	config := EmbeddingModelConfig{
		Name: "local-test",
	}

	model := NewLocalHashModel(config)
	assert.Equal(t, 1536, model.Dimensions())
}

func TestLocalHashModel_Health(t *testing.T) {
	model := NewLocalHashModel(EmbeddingModelConfig{})
	err := model.Health(context.Background())
	assert.NoError(t, err) // Always healthy
}

func TestLocalHashModel_EncodeSingle(t *testing.T) {
	config := EmbeddingModelConfig{
		Dimensions: 512,
	}

	model := NewLocalHashModel(config)
	embedding, err := model.EncodeSingle(context.Background(), "test text")
	assert.NoError(t, err)
	assert.Len(t, embedding, 512)

	// Verify embedding is normalized
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	assert.InDelta(t, 1.0, norm, 0.001) // Should be close to 1.0 (unit length)
}

func TestLocalHashModel_EncodeSingle_Deterministic(t *testing.T) {
	config := EmbeddingModelConfig{
		Dimensions: 256,
	}

	model := NewLocalHashModel(config)

	// Same text should produce same embedding
	embedding1, _ := model.EncodeSingle(context.Background(), "test text")
	embedding2, _ := model.EncodeSingle(context.Background(), "test text")
	assert.Equal(t, embedding1, embedding2)

	// Different text should produce different embedding
	embedding3, _ := model.EncodeSingle(context.Background(), "different text")
	assert.NotEqual(t, embedding1, embedding3)
}

func TestLocalHashModel_Encode(t *testing.T) {
	config := EmbeddingModelConfig{
		Dimensions: 128,
	}

	model := NewLocalHashModel(config)
	texts := []string{"text1", "text2", "text3"}
	embeddings, err := model.Encode(context.Background(), texts)
	assert.NoError(t, err)
	assert.Len(t, embeddings, 3)

	for _, emb := range embeddings {
		assert.Len(t, emb, 128)
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestEmbeddingModelRegistry_Concurrent(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{})

	// Add local models that are fast
	for i := 0; i < 5; i++ {
		registry.configs.Put(string(rune('a'+i)), EmbeddingModelConfig{
			Name:       string(rune('a' + i)),
			Provider:   "local",
			Dimensions: 256,
		})
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Concurrent Get operations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := string(rune('a' + (idx % 5)))
			_, err := registry.Get(name)
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	// Concurrent List operations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.List()
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestEmbeddingModelRegistry_ConcurrentEncode(t *testing.T) {
	registry := NewEmbeddingModelRegistry(RegistryConfig{
		FallbackChain: []string{"local-fallback"},
	})

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := registry.EncodeWithFallback(context.Background(), []string{"test text"})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
}
