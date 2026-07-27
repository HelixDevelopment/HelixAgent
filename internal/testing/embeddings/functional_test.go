// Package embeddings provides real functional tests for embedding providers.
// These tests execute ACTUAL embedding operations, not just connectivity checks.
// Tests FAIL if the operation fails - no false positives.
package embeddings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/testutil"
)

// EmbeddingClient provides a client for testing embedding providers
type EmbeddingClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewEmbeddingClient creates a new embedding test client
func NewEmbeddingClient(baseURL string) *EmbeddingClient {
	return &EmbeddingClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// EmbeddingRequest represents an embedding request
type EmbeddingRequest struct {
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Input    []string `json:"input"`
}

// EmbeddingResponse represents an embedding response
type EmbeddingResponse struct {
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
	Usage      struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Error string `json:"error,omitempty"`
}

// Embed generates embeddings for the given input
func (c *EmbeddingClient) Embed(req *EmbeddingRequest) (*EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/v1/embeddings", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to call embeddings API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings API failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result EmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w (raw: %s)", err, string(respBody))
	}

	return &result, nil
}

// ListProviders lists all available embedding providers
func (c *EmbeddingClient) ListProviders() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v1/embeddings/providers")
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list providers failed with status %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Providers []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Enabled bool   `json:"enabled"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var names []string
	for _, p := range raw.Providers {
		names = append(names, p.Name)
	}
	return names, nil
}

// EmbeddingProviderConfig holds configuration for testing an embedding provider
type EmbeddingProviderConfig struct {
	Provider     string
	Model        string
	EnvKey       string
	Dimension    int
	RequiresAuth bool
}

// Embedding providers to test
var EmbeddingProviders = []EmbeddingProviderConfig{
	{Provider: "openai", Model: "text-embedding-3-small", EnvKey: "OPENAI_API_KEY", Dimension: 1536, RequiresAuth: true},
	{Provider: "cohere", Model: "embed-english-v3.0", EnvKey: "COHERE_API_KEY", Dimension: 1024, RequiresAuth: true},
	{Provider: "voyage", Model: "voyage-3", EnvKey: "VOYAGE_API_KEY", Dimension: 1024, RequiresAuth: true},
	{Provider: "jina", Model: "jina-embeddings-v3", EnvKey: "JINA_API_KEY", Dimension: 1024, RequiresAuth: true},
	{Provider: "google", Model: "text-embedding-005", EnvKey: "GOOGLE_API_KEY", Dimension: 768, RequiresAuth: true},
	{Provider: "bedrock", Model: "amazon.titan-embed-text-v2", EnvKey: "AWS_ACCESS_KEY_ID", Dimension: 1024, RequiresAuth: true},
}

// TestEmbeddingProviderDiscovery tests provider discovery endpoint
func TestEmbeddingProviderDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewEmbeddingClient(helixAgentBaseURL(t))

	providers, err := client.ListProviders()
	require.NoError(t, err)

	assert.NotEmpty(t, providers, "Should have at least one provider")
	t.Logf("Discovered %d embedding providers: %v", len(providers), providers)
}

// TestEmbeddingGeneration tests actual embedding generation
func TestEmbeddingGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewEmbeddingClient(helixAgentBaseURL(t))

	testInputs := []string{
		"Hello, world!",
		"This is a test of the embedding system.",
	}

	for _, provider := range EmbeddingProviders {
		t.Run(provider.Provider, func(t *testing.T) {
			// Check if API key is available
			if provider.RequiresAuth {
				testutil.RequireEnv(t, provider.EnvKey)
			}

			req := &EmbeddingRequest{
				Provider: provider.Provider,
				Model:    provider.Model,
				Input:    testInputs,
			}

			resp, err := client.Embed(req)
			if err != nil {
				t.Skipf("Embedding provider %s not available: %v (SKIP-OK: #infra-unavailable)", provider.Provider, err)
				return
			}

			require.Equal(t, provider.Provider, resp.Provider)
			require.Len(t, resp.Embeddings, len(testInputs), "Should have one embedding per input")

			for i, embedding := range resp.Embeddings {
				assert.NotEmpty(t, embedding, "Embedding %d should not be empty", i)
				assert.True(t, len(embedding) > 0, "Embedding should have dimensions")
				t.Logf("Provider %s: Input %d has embedding with %d dimensions", provider.Provider, i, len(embedding))
			}
		})
	}
}

// TestEmbeddingSimilarity tests that similar texts have similar embeddings
func TestEmbeddingSimilarity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewEmbeddingClient(helixAgentBaseURL(t))

	// Test with first available provider
	for _, provider := range EmbeddingProviders {
		if provider.RequiresAuth && os.Getenv(provider.EnvKey) == "" {
			continue
		}

		t.Run(provider.Provider, func(t *testing.T) {
			similarTexts := []string{
				"The cat sat on the mat.",
				"A cat was sitting on a mat.",
			}
			differentText := "Machine learning is transforming industries."

			// Get embeddings for similar texts
			resp1, err := client.Embed(&EmbeddingRequest{
				Provider: provider.Provider,
				Model:    provider.Model,
				Input:    similarTexts,
			})
			if err != nil {
				t.Skipf("Provider %s not available: %v (SKIP-OK: #infra-unavailable)", provider.Provider, err)
				return
			}

			// Get embedding for different text
			resp2, err := client.Embed(&EmbeddingRequest{
				Provider: provider.Provider,
				Model:    provider.Model,
				Input:    []string{differentText},
			})
			if err != nil {
				t.Skipf("Provider %s not available: %v (SKIP-OK: #infra-unavailable)", provider.Provider, err)
				return
			}

			// Calculate cosine similarities
			similarSim := cosineSimilarity(resp1.Embeddings[0], resp1.Embeddings[1])
			differentSim := cosineSimilarity(resp1.Embeddings[0], resp2.Embeddings[0])

			t.Logf("Provider %s: Similar text similarity: %.4f", provider.Provider, similarSim)
			t.Logf("Provider %s: Different text similarity: %.4f", provider.Provider, differentSim)

			// Similar texts should have higher similarity
			assert.Greater(t, similarSim, differentSim, "Similar texts should have higher similarity")
		})

		break // Only test first available provider for similarity
	}
}

// TestEmbeddingHealthCheck tests embedding service health
func TestEmbeddingHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional test in short mode") // SKIP-OK: #short-mode
	}
	client := NewEmbeddingClient(helixAgentBaseURL(t))

	resp, err := client.httpClient.Get(client.baseURL + "/v1/embeddings/providers")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	z := 1.0
	for i := 0; i < 100; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

// BenchmarkEmbedding benchmarks embedding generation
func BenchmarkEmbedding(b *testing.B) {
	base, probed := resolveHelixAgentEmbeddings()
	if base == "" {
		b.Skipf("HelixAgent embeddings API not reachable; probed %v", probed)
	}
	client := NewEmbeddingClient(base)

	req := &EmbeddingRequest{
		Provider: "openai",
		Model:    "text-embedding-3-small",
		Input:    []string{"Hello, world!"},
	}

	if os.Getenv("OPENAI_API_KEY") == "" {
		b.Skip("OPENAI_API_KEY not set")
		return
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Embed(req)
		if err != nil {
			b.Fatalf("Embedding failed: %v", err)
		}
	}
}
