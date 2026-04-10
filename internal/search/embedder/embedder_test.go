package embedder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/search/types"
)

// --- LocalEmbedder tests ---

func TestNewLocalEmbedder(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(768)
	require.NotNil(t, emb)
	assert.Equal(t, 768, emb.dimensions)
}

func TestLocalEmbedder_Dimensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dims int
	}{
		{"small", 128},
		{"medium", 768},
		{"large", 1536},
		{"xlarge", 3072},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			emb := NewLocalEmbedder(tc.dims)
			assert.Equal(t, tc.dims, emb.Dimensions())
		})
	}
}

func TestLocalEmbedder_Embed_SingleText(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(128)
	ctx := context.Background()

	results, err := emb.Embed(ctx, []string{"hello world"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Len(t, results[0], 128)
}

func TestLocalEmbedder_Embed_MultipleTexts(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(64)
	ctx := context.Background()

	texts := []string{"hello", "world", "foo", "bar"}
	results, err := emb.Embed(ctx, texts)
	require.NoError(t, err)
	require.Len(t, results, 4)

	for _, vec := range results {
		assert.Len(t, vec, 64)
	}
}

func TestLocalEmbedder_Embed_Deterministic(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(256)
	ctx := context.Background()

	text := "deterministic test"
	results1, err := emb.Embed(ctx, []string{text})
	require.NoError(t, err)

	results2, err := emb.Embed(ctx, []string{text})
	require.NoError(t, err)

	// Same input should produce same output
	assert.Equal(t, results1[0], results2[0])
}

func TestLocalEmbedder_Embed_DifferentTexts(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(128)
	ctx := context.Background()

	results, err := emb.Embed(ctx, []string{"hello", "world"})
	require.NoError(t, err)

	// Different texts should produce different embeddings
	assert.NotEqual(t, results[0], results[1])
}

func TestLocalEmbedder_Embed_EmptySlice(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(128)
	ctx := context.Background()

	results, err := emb.Embed(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestLocalEmbedder_EmbedQuery(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(256)
	ctx := context.Background()

	vec, err := emb.EmbedQuery(ctx, "test query")
	require.NoError(t, err)
	assert.Len(t, vec, 256)
}

func TestLocalEmbedder_EmbedQuery_Deterministic(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(128)
	ctx := context.Background()

	vec1, err := emb.EmbedQuery(ctx, "same query")
	require.NoError(t, err)

	vec2, err := emb.EmbedQuery(ctx, "same query")
	require.NoError(t, err)

	assert.Equal(t, vec1, vec2)
}

func TestLocalEmbedder_EmbedQuery_MatchesEmbed(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(128)
	ctx := context.Background()

	query := "test matching"
	queryVec, err := emb.EmbedQuery(ctx, query)
	require.NoError(t, err)

	embedVecs, err := emb.Embed(ctx, []string{query})
	require.NoError(t, err)

	// EmbedQuery should return the same as Embed for the same text
	assert.Equal(t, embedVecs[0], queryVec)
}

func TestLocalEmbedder_GenerateEmbedding_ValuesInRange(t *testing.T) {
	t.Parallel()

	emb := NewLocalEmbedder(512)
	vec := emb.generateEmbedding("range test")

	for i, v := range vec {
		assert.True(t, v >= -1.0 && v <= 1.0,
			"value at index %d is %f, expected in [-1, 1]", i, v)
	}
}

func TestLocalEmbedder_ImplementsEmbedder(t *testing.T) {
	t.Parallel()

	var _ types.Embedder = (*LocalEmbedder)(nil)
}

// --- OpenAIEmbedder tests ---

func TestNewOpenAIEmbedder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    string
		wantDims int
	}{
		{"default model", "", 1536},
		{"small model", "text-embedding-3-small", 1536},
		{"large model", "text-embedding-3-large", 3072},
		{"custom model", "custom-model", 1536},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			emb := NewOpenAIEmbedder("test-key", tc.model)
			require.NotNil(t, emb)
			assert.Equal(t, tc.wantDims, emb.Dimensions())
			assert.NotNil(t, emb.cache)
		})
	}
}

func TestOpenAIEmbedder_DefaultModel(t *testing.T) {
	t.Parallel()

	emb := NewOpenAIEmbedder("key", "")
	assert.Equal(t, "text-embedding-3-small", emb.model)
}

func TestOpenAIEmbedder_Dimensions(t *testing.T) {
	t.Parallel()

	emb := NewOpenAIEmbedder("key", "text-embedding-3-small")
	assert.Equal(t, 1536, emb.Dimensions())
}

func TestOpenAIEmbedder_HashText(t *testing.T) {
	t.Parallel()

	emb := NewOpenAIEmbedder("key", "")

	hash1 := emb.hashText("hello")
	hash2 := emb.hashText("hello")
	hash3 := emb.hashText("world")

	assert.Equal(t, hash1, hash2, "same text should produce same hash")
	assert.NotEqual(t, hash1, hash3, "different text should produce different hash")
	assert.Len(t, hash1, 64, "sha256 hex should be 64 chars")
}

func TestOpenAIEmbedder_Embed_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float64{0.1, 0.2, 0.3}, "index": 0},
				{"embedding": []float64{0.4, 0.5, 0.6}, "index": 1},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	emb := NewOpenAIEmbedder("test-key", "text-embedding-3-small")
	emb.client = &http.Client{
		Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
		Timeout:   5 * time.Second,
	}

	results, err := emb.Embed(context.Background(), []string{"hello", "world"})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.InDelta(t, float32(0.1), results[0][0], 0.01)
	assert.InDelta(t, float32(0.4), results[1][0], 0.01)
}

func TestOpenAIEmbedder_Embed_Caching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float64{0.1, 0.2, 0.3}, "index": 0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	emb := NewOpenAIEmbedder("key", "")
	emb.client = &http.Client{
		Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
		Timeout:   5 * time.Second,
	}

	// First call should hit the server
	_, err := emb.Embed(context.Background(), []string{"cached text"})
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call with same text should use cache
	_, err = emb.Embed(context.Background(), []string{"cached text"})
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "second call should not hit server due to cache")
}

func TestOpenAIEmbedder_Embed_APIError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	emb := NewOpenAIEmbedder("key", "")
	emb.client = &http.Client{
		Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
		Timeout:   5 * time.Second,
	}

	_, err := emb.Embed(context.Background(), []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 429")
}

func TestOpenAIEmbedder_Embed_InvalidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	emb := NewOpenAIEmbedder("key", "")
	emb.client = &http.Client{
		Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
		Timeout:   5 * time.Second,
	}

	_, err := emb.Embed(context.Background(), []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestOpenAIEmbedder_EmbedQuery_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float64{0.7, 0.8, 0.9}, "index": 0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	emb := NewOpenAIEmbedder("key", "")
	emb.client = &http.Client{
		Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
		Timeout:   5 * time.Second,
	}

	vec, err := emb.EmbedQuery(context.Background(), "query")
	require.NoError(t, err)
	assert.Len(t, vec, 3)
	assert.InDelta(t, float32(0.7), vec[0], 0.01)
}

func TestOpenAIEmbedder_ImplementsEmbedder(t *testing.T) {
	t.Parallel()

	var _ types.Embedder = (*OpenAIEmbedder)(nil)
}

func TestOpenAIEmbedder_Embed_CancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	emb := NewOpenAIEmbedder("key", "")
	emb.client = &http.Client{
		Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
		Timeout:   10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := emb.Embed(ctx, []string{"test"})
	assert.Error(t, err)
}

func TestOpenAIEmbedder_Embed_PartialCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should only be called for uncached texts
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		inputs := reqBody["input"].([]interface{})
		data := make([]map[string]interface{}, len(inputs))
		for i := range inputs {
			data[i] = map[string]interface{}{
				"embedding": []float64{float64(i+1) * 0.1, float64(i+1) * 0.2},
				"index":     i,
			}
		}

		resp := map[string]interface{}{"data": data}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	emb := NewOpenAIEmbedder("key", "")
	emb.client = &http.Client{
		Transport: &urlRewriteTransport{base: http.DefaultTransport, baseURL: server.URL},
		Timeout:   5 * time.Second,
	}

	// First call: cache "text1"
	_, err := emb.Embed(context.Background(), []string{"text1"})
	require.NoError(t, err)

	// Second call: "text1" cached, "text2" not
	results, err := emb.Embed(context.Background(), []string{"text1", "text2"})
	require.NoError(t, err)
	require.Len(t, results, 2)
}

// urlRewriteTransport rewrites all request URLs to point at a test server
type urlRewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *urlRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[len("http://"):]
	return t.base.RoundTrip(req)
}
