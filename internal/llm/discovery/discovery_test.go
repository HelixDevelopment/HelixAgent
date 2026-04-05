package discovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiscoverer(t *testing.T) {
	t.Parallel()
	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: "https://api.test.com/v1/models",
		ModelsDevID:    "test",
		APIKey:         "test-key",
		FallbackModels: []string{"model-a", "model-b"},
	}

	d := NewDiscoverer(config)
	require.NotNil(t, d)
	assert.Equal(t, "test", d.config.ProviderName)
	assert.Equal(t, "Authorization", d.config.AuthHeader)
	assert.Equal(t, "Bearer ", d.config.AuthPrefix)
	assert.Equal(t, 1*time.Hour, d.config.CacheTTL)
}

func TestNewDiscoverer_CustomAuth(t *testing.T) {
	t.Parallel()
	config := ProviderConfig{
		ProviderName: "anthropic",
		AuthHeader:   "x-api-key",
		AuthPrefix:   "",
		APIKey:       "sk-ant-test",
		CacheTTL:     30 * time.Minute,
	}

	d := NewDiscoverer(config)
	assert.Equal(t, "x-api-key", d.config.AuthHeader)
	assert.Equal(t, "", d.config.AuthPrefix)
	assert.Equal(t, 30*time.Minute, d.config.CacheTTL)
}

func TestDiscoverModels_Tier1_ProviderAPI(t *testing.T) {
	t.Parallel()
	// Mock OpenAI-compatible /v1/models endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		resp := openAIModelsResponse{
			Data: []openAIModel{
				{ID: "gpt-4o", Object: "model"},
				{ID: "gpt-4o-mini", Object: "model"},
				{ID: "text-embedding-ada-002", Object: "model"}, // Should be filtered
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "openai",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		FallbackModels: []string{"gpt-4", "gpt-3.5-turbo"},
	}

	d := NewDiscoverer(config)
	models := d.DiscoverModels()

	assert.Len(t, models, 2)
	assert.Contains(t, models, "gpt-4o")
	assert.Contains(t, models, "gpt-4o-mini")
	assert.NotContains(t, models, "text-embedding-ada-002")
	assert.Equal(t, 1, d.GetDiscoveryTier())
}

func TestDiscoverModels_Tier1_WithCustomFilter(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIModelsResponse{
			Data: []openAIModel{
				{ID: "llama-3.3-70b-versatile"},
				{ID: "llama-3.1-8b-instant"},
				{ID: "whisper-large-v3"},
				{ID: "llama-guard-3-8b"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "groq",
		ModelsEndpoint: server.URL,
		APIKey:         "gsk_test",
		ModelFilter: func(id string) bool {
			return strings.Contains(id, "llama") && !strings.Contains(id, "guard")
		},
		FallbackModels: []string{"llama-3.3-70b-versatile"},
	}

	d := NewDiscoverer(config)
	models := d.DiscoverModels()

	assert.Len(t, models, 2)
	assert.Contains(t, models, "llama-3.3-70b-versatile")
	assert.Contains(t, models, "llama-3.1-8b-instant")
	assert.NotContains(t, models, "whisper-large-v3")
	assert.NotContains(t, models, "llama-guard-3-8b")
}

func TestDiscoverModels_Tier1_APIFails_FallsToTier3(t *testing.T) {
	t.Parallel()
	// API returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		ModelsDevID:    "", // Skip tier 2
		FallbackModels: []string{"fallback-model-1", "fallback-model-2"},
	}

	d := NewDiscoverer(config)
	models := d.DiscoverModels()

	assert.Equal(t, config.FallbackModels, models)
	assert.Equal(t, 3, d.GetDiscoveryTier())
}

func TestDiscoverModels_Tier1_EmptyResponse_FallsToTier3(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openAIModelsResponse{Data: []openAIModel{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		FallbackModels: []string{"fallback-model"},
	}

	d := NewDiscoverer(config)
	models := d.DiscoverModels()

	assert.Equal(t, []string{"fallback-model"}, models)
	assert.Equal(t, 3, d.GetDiscoveryTier())
}

func TestDiscoverModels_NoAPIKey_SkipsTier1(t *testing.T) {
	t.Parallel()
	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: "https://api.test.com/v1/models", // Would fail anyway
		APIKey:         "",                               // No API key
		FallbackModels: []string{"fallback-model"},
	}

	d := NewDiscoverer(config)
	models := d.DiscoverModels()

	assert.Equal(t, []string{"fallback-model"}, models)
	assert.Equal(t, 3, d.GetDiscoveryTier())
}

func TestDiscoverModels_Caching(t *testing.T) {
	t.Parallel()
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		resp := openAIModelsResponse{
			Data: []openAIModel{{ID: "model-1"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		CacheTTL:       1 * time.Hour,
	}

	d := NewDiscoverer(config)

	// First call - should hit API
	models1 := d.DiscoverModels()
	assert.Len(t, models1, 1)
	assert.Equal(t, 1, callCount)

	// Second call - should use cache
	models2 := d.DiscoverModels()
	assert.Len(t, models2, 1)
	assert.Equal(t, 1, callCount) // Still 1 - cache hit
}

func TestDiscoverModels_CacheExpiry(t *testing.T) {
	t.Parallel()
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		resp := openAIModelsResponse{
			Data: []openAIModel{{ID: "model-1"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		CacheTTL:       1 * time.Millisecond, // Very short TTL
	}

	d := NewDiscoverer(config)

	// First call
	d.DiscoverModels()
	assert.Equal(t, 1, callCount)

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	// Second call - cache expired, should hit API again
	d.DiscoverModels()
	assert.Equal(t, 2, callCount)
}

func TestInvalidateCache(t *testing.T) {
	t.Parallel()
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		resp := openAIModelsResponse{
			Data: []openAIModel{{ID: "model-1"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		CacheTTL:       1 * time.Hour,
	}

	d := NewDiscoverer(config)

	d.DiscoverModels()
	assert.Equal(t, 1, callCount)

	d.InvalidateCache()
	assert.Equal(t, 0, d.GetDiscoveryTier())

	d.DiscoverModels()
	assert.Equal(t, 2, callCount) // Cache invalidated, hit API again
}

func TestGetCachedModels_Empty(t *testing.T) {
	t.Parallel()
	config := ProviderConfig{
		ProviderName:   "test",
		FallbackModels: []string{"fallback-1", "fallback-2"},
	}

	d := NewDiscoverer(config)
	models := d.GetCachedModels()

	assert.Equal(t, config.FallbackModels, models)
}

func TestGetCachedModels_AfterDiscovery(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openAIModelsResponse{
			Data: []openAIModel{{ID: "api-model"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		FallbackModels: []string{"fallback"},
	}

	d := NewDiscoverer(config)
	d.DiscoverModels()

	cached := d.GetCachedModels()
	assert.Equal(t, []string{"api-model"}, cached)
}

func TestDiscoverModels_ExtraHeaders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom auth header (x-api-key without Bearer prefix)
		assert.Equal(t, "sk-test", r.Header.Get("x-api-key"))
		// Verify extra headers
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		resp := openAIModelsResponse{
			Data: []openAIModel{{ID: "claude-3-opus-20240229"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "anthropic",
		ModelsEndpoint: server.URL,
		APIKey:         "sk-test",
		AuthHeader:     "x-api-key",
		AuthPrefix:     "", // No Bearer prefix for x-api-key
		ExtraHeaders: map[string]string{
			"anthropic-version": "2023-06-01",
		},
		FallbackModels: []string{"claude-3-opus-20240229"},
	}

	d := NewDiscoverer(config)
	models := d.DiscoverModels()

	assert.Contains(t, models, "claude-3-opus-20240229")
	assert.Equal(t, 1, d.GetDiscoveryTier())
}

func TestDiscoverModels_CustomResponseParser(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom auth header
		assert.Equal(t, "test-key", r.Header.Get("x-goog-api-key"))
		// Return Gemini-style response with supportedGenerationMethods
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[` +
			`{"name":"models/gemini-2.0-flash","supportedGenerationMethods":["generateContent","streamGenerateContent"]},` +
			`{"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent"]},` +
			`{"name":"models/embedding-001","supportedGenerationMethods":["embedContent"]}` +
			`]}`))
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "gemini",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		AuthHeader:     "x-goog-api-key",
		AuthPrefix:     "",
		ResponseParser: ParseGeminiModelsResponse,
		FallbackModels: []string{"gemini-2.0-flash"},
	}

	d := NewDiscoverer(config)
	models := d.DiscoverModels()

	assert.Len(t, models, 2)
	assert.Contains(t, models, "gemini-2.0-flash")
	assert.Contains(t, models, "gemini-2.5-pro")
	assert.NotContains(t, models, "embedding-001")
	assert.Equal(t, 1, d.GetDiscoveryTier())
}

func TestParseGeminiModelsResponse(t *testing.T) {
	t.Parallel()
	body := `{
		"models": [
			{
				"name": "models/gemini-2.0-flash",
				"displayName": "Gemini 2.0 Flash",
				"supportedGenerationMethods": ["generateContent", "streamGenerateContent"]
			},
			{
				"name": "models/embedding-001",
				"displayName": "Embedding 001",
				"supportedGenerationMethods": ["embedContent"]
			},
			{
				"name": "models/gemini-2.5-pro",
				"displayName": "Gemini 2.5 Pro",
				"supportedGenerationMethods": ["generateContent"]
			}
		]
	}`

	resp := &http.Response{
		Body: http.NoBody,
	}
	resp.Body = newReadCloser(body)

	models, err := ParseGeminiModelsResponse(resp)
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Contains(t, models, "gemini-2.0-flash")
	assert.Contains(t, models, "gemini-2.5-pro")
	assert.NotContains(t, models, "embedding-001")
}

func TestParseOllamaModelsResponse(t *testing.T) {
	t.Parallel()
	body := `{
		"models": [
			{"name": "llama2:latest"},
			{"name": "codellama:7b"},
			{"name": "mistral:latest"}
		]
	}`

	resp := &http.Response{
		Body: newReadCloser(body),
	}

	models, err := ParseOllamaModelsResponse(resp)
	require.NoError(t, err)
	assert.Len(t, models, 3)
	assert.Contains(t, models, "llama2:latest")
	assert.Contains(t, models, "codellama:7b")
	assert.Contains(t, models, "mistral:latest")
}

func TestParseCohereModelsResponse(t *testing.T) {
	t.Parallel()
	body := `{
		"models": [
			{"name": "command-r-plus", "endpoints": ["chat", "generate"]},
			{"name": "embed-english-v3.0", "endpoints": ["embed"]},
			{"name": "command-r", "endpoints": ["chat"]}
		]
	}`

	resp := &http.Response{
		Body: newReadCloser(body),
	}

	models, err := ParseCohereModelsResponse(resp)
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Contains(t, models, "command-r-plus")
	assert.Contains(t, models, "command-r")
	assert.NotContains(t, models, "embed-english-v3.0")
}

func TestParseReplicateModelsResponse(t *testing.T) {
	t.Parallel()
	body := `{
		"results": [
			{"owner": "meta", "name": "llama-2-70b-chat"},
			{"owner": "stability-ai", "name": "sdxl"},
			{"owner": "mistralai", "name": "mixtral-8x7b"}
		]
	}`

	resp := &http.Response{
		Body: newReadCloser(body),
	}

	models, err := ParseReplicateModelsResponse(resp)
	require.NoError(t, err)
	assert.Len(t, models, 3)
	assert.Contains(t, models, "meta/llama-2-70b-chat")
	assert.Contains(t, models, "stability-ai/sdxl")
}

func TestParseZAIModelsResponse(t *testing.T) {
	t.Parallel()
	body := `{
		"data": [
			{"id": "glm-4.7"},
			{"id": "glm-4-plus"},
			{"id": "glm-4-flash"}
		]
	}`

	resp := &http.Response{
		Body: newReadCloser(body),
	}

	models, err := ParseZAIModelsResponse(resp)
	require.NoError(t, err)
	assert.Len(t, models, 3)
	assert.Contains(t, models, "glm-4.7")
	assert.Contains(t, models, "glm-4-plus")
}

func TestIsChatModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		modelID string
		want    bool
	}{
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"claude-3-opus-20240229", true},
		{"llama-3.3-70b-versatile", true},
		{"text-embedding-ada-002", false},
		{"text-embedding-3-large", false},
		{"embed-english-v3.0", false},
		{"whisper-large-v3", false},
		{"tts-1", false},
		{"dall-e-3", false},
		{"text-davinci-003", false},
		{"babbage-002", false},
		{"moderation-latest", false},
		{"deepseek-chat", true},
		{"mistral-large-latest", true},
		{"command-r-plus", true},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tt.want, IsChatModel(tt.modelID), "IsChatModel(%q)", tt.modelID)
		})
	}
}

func TestDiscoverModels_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openAIModelsResponse{
			Data: []openAIModel{{ID: "model-1"}, {ID: "model-2"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
	}

	d := NewDiscoverer(config)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			models := d.DiscoverModels()
			assert.True(t, len(models) > 0)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkDiscoverModels_Cached(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := openAIModelsResponse{
			Data: []openAIModel{
				{ID: "model-1"}, {ID: "model-2"}, {ID: "model-3"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := ProviderConfig{
		ProviderName:   "test",
		ModelsEndpoint: server.URL,
		APIKey:         "test-key",
		CacheTTL:       1 * time.Hour,
	}

	d := NewDiscoverer(config)
	d.DiscoverModels() // Prime cache

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.DiscoverModels()
	}
}

func BenchmarkIsChatModel(b *testing.B) {
	models := []string{
		"gpt-4o", "text-embedding-ada-002", "claude-3-opus", "whisper-large-v3",
		"llama-3.3-70b", "tts-1", "deepseek-chat", "dall-e-3",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, m := range models {
			IsChatModel(m)
		}
	}
}

// newReadCloser creates an io.ReadCloser from a string for testing.
func newReadCloser(s string) *readCloser {
	return &readCloser{reader: strings.NewReader(s)}
}

type readCloser struct {
	reader *strings.Reader
}

func (rc *readCloser) Read(p []byte) (n int, err error) {
	return rc.reader.Read(p)
}

func (rc *readCloser) Close() error {
	return nil
}
