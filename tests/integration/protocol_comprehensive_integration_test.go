// Package integration contains comprehensive integration tests for all HelixAgent protocols:
// LSP (Language Server Protocol), ACP (Agent Communication Protocol), Embeddings, Vision,
// and their integration with LLMs and the AI Debate system.
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LSP (Language Server Protocol) Tests
// =============================================================================

// LSPServerConfig defines configuration for an LSP server
type LSPServerConfig struct {
	Name     string
	Endpoint string
	Language string
}

// AllLSPServers defines the list of supported LSP servers
var AllLSPServers = []LSPServerConfig{
	{Name: "gopls", Endpoint: "/v1/lsp/gopls", Language: "go"},
	{Name: "pyright", Endpoint: "/v1/lsp/pyright", Language: "python"},
	{Name: "typescript-language-server", Endpoint: "/v1/lsp/typescript", Language: "typescript"},
	{Name: "rust-analyzer", Endpoint: "/v1/lsp/rust", Language: "rust"},
	{Name: "clangd", Endpoint: "/v1/lsp/clangd", Language: "c/c++"},
	{Name: "java-language-server", Endpoint: "/v1/lsp/java", Language: "java"},
	{Name: "kotlin-language-server", Endpoint: "/v1/lsp/kotlin", Language: "kotlin"},
	{Name: "lua-language-server", Endpoint: "/v1/lsp/lua", Language: "lua"},
	{Name: "bash-language-server", Endpoint: "/v1/lsp/bash", Language: "bash"},
	{Name: "yaml-language-server", Endpoint: "/v1/lsp/yaml", Language: "yaml"},
	{Name: "json-language-server", Endpoint: "/v1/lsp/json", Language: "json"},
	{Name: "html-language-server", Endpoint: "/v1/lsp/html", Language: "html"},
	{Name: "css-language-server", Endpoint: "/v1/lsp/css", Language: "css"},
	{Name: "dockerfile-language-server", Endpoint: "/v1/lsp/dockerfile", Language: "dockerfile"},
	{Name: "terraform-ls", Endpoint: "/v1/lsp/terraform", Language: "terraform"},
}

// TestLSPEndpoints tests all LSP endpoints are accessible
func TestLSPEndpoints(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, server := range AllLSPServers {
		t.Run(server.Name, func(t *testing.T) {
			// Test OPTIONS to check endpoint exists
			req, err := http.NewRequest("OPTIONS", baseURL+server.Endpoint, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			if err != nil {
				t.Skipf("HelixAgent not running or LSP endpoint not available: %v (SKIP-OK: #infra-unavailable)", err)
				return
			}
			defer resp.Body.Close()

			t.Logf("LSP server %s (%s) - status: %d", server.Name, server.Language, resp.StatusCode)
		})
	}
}

// TestLSPDiagnostics tests LSP diagnostic capabilities
func TestLSPDiagnostics(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Test Go code diagnostics
	t.Run("GoDiagnostics", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"language": "go",
			"code": `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
    undefinedVar // This should trigger a diagnostic
}`,
		}

		reqData, err := json.Marshal(reqBody)
		require.NoError(t, err)

		resp, err := http.Post(
			baseURL+"/v1/lsp/diagnostics",
			"application/json",
			bytes.NewBuffer(reqData),
		)
		if err != nil {
			t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Go diagnostics response: %.500s", string(body))
	})

	// Test Python code diagnostics
	t.Run("PythonDiagnostics", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"language": "python",
			"code": `def hello():
    print("Hello")
    undefined_var  # This should trigger a diagnostic
`,
		}

		reqData, err := json.Marshal(reqBody)
		require.NoError(t, err)

		resp, err := http.Post(
			baseURL+"/v1/lsp/diagnostics",
			"application/json",
			bytes.NewBuffer(reqData),
		)
		if err != nil {
			t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Python diagnostics response: %.500s", string(body))
	})
}

// TestLSPCompletion tests LSP code completion
func TestLSPCompletion(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	reqBody := map[string]interface{}{
		"language": "go",
		"code":     "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.",
		"position": map[string]int{"line": 5, "character": 8},
	}

	reqData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(
		baseURL+"/v1/lsp/completion",
		"application/json",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("LSP completion response: %.500s", string(body))
}

// TestLSPWithLLMProviders tests LSP integration with all LLM providers
func TestLSPWithLLMProviders(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	providers := []string{"claude", "deepseek", "gemini", "mistral", "qwen"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"model":    provider,
				"language": "go",
				"code":     "func add(a, b int) int { return a + b }",
				"action":   "explain",
			}

			reqData, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := http.Post(
				baseURL+"/v1/lsp/ai-assist",
				"application/json",
				bytes.NewBuffer(reqData),
			)
			if err != nil {
				t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
				return
			}
			defer resp.Body.Close()

			t.Logf("LSP+%s response status: %d", provider, resp.StatusCode)
		})
	}
}

// =============================================================================
// ACP (Agent Communication Protocol) Tests
// =============================================================================

// ACPAgentConfig defines configuration for an ACP agent
type ACPAgentConfig struct {
	Name        string
	Type        string
	Endpoint    string
	Description string
}

// AllACPAgents defines the list of supported ACP agents
var AllACPAgents = []ACPAgentConfig{
	{Name: "code-reviewer", Type: "analysis", Endpoint: "/v1/acp/agents/code-reviewer", Description: "Code review agent"},
	{Name: "bug-finder", Type: "analysis", Endpoint: "/v1/acp/agents/bug-finder", Description: "Bug detection agent"},
	{Name: "refactor-assistant", Type: "transform", Endpoint: "/v1/acp/agents/refactor", Description: "Code refactoring agent"},
	{Name: "documentation-generator", Type: "generate", Endpoint: "/v1/acp/agents/docs", Description: "Documentation generator"},
	{Name: "test-generator", Type: "generate", Endpoint: "/v1/acp/agents/tests", Description: "Test case generator"},
	{Name: "security-scanner", Type: "analysis", Endpoint: "/v1/acp/agents/security", Description: "Security vulnerability scanner"},
	{Name: "performance-analyzer", Type: "analysis", Endpoint: "/v1/acp/agents/performance", Description: "Performance analysis agent"},
	{Name: "dependency-analyzer", Type: "analysis", Endpoint: "/v1/acp/agents/deps", Description: "Dependency analysis agent"},
}

// TestACPAgentDiscovery tests ACP agent discovery
func TestACPAgentDiscovery(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	resp, err := http.Get(baseURL + "/v1/acp/agents")
	if err != nil {
		t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("ACP agents discovery response: %.500s", string(body))

	if resp.StatusCode == http.StatusOK {
		var agents []ACPAgentConfig
		if err := json.Unmarshal(body, &agents); err == nil {
			t.Logf("Found %d ACP agents", len(agents))
		}
	}
}

// TestACPAgentCommunication tests ACP agent-to-agent communication
func TestACPAgentCommunication(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Test code review agent
	t.Run("CodeReviewAgent", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"agent": "code-reviewer",
			"input": map[string]interface{}{
				"code":     "func add(a, b int) int { return a + b }",
				"language": "go",
			},
		}

		reqData, err := json.Marshal(reqBody)
		require.NoError(t, err)

		resp, err := http.Post(
			baseURL+"/v1/acp/invoke",
			"application/json",
			bytes.NewBuffer(reqData),
		)
		if err != nil {
			t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Code review agent response: %.500s", string(body))
	})
}

// TestACPWithLLMProviders tests ACP integration with LLM providers
func TestACPWithLLMProviders(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	providers := []string{"claude", "deepseek", "gemini"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"agent":    "code-reviewer",
				"provider": provider,
				"input": map[string]interface{}{
					"code":     "def hello(): print('Hello')",
					"language": "python",
				},
			}

			reqData, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := http.Post(
				baseURL+"/v1/acp/invoke",
				"application/json",
				bytes.NewBuffer(reqData),
			)
			if err != nil {
				t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
				return
			}
			defer resp.Body.Close()

			t.Logf("ACP+%s response status: %d", provider, resp.StatusCode)
		})
	}
}

// =============================================================================
// Embeddings Tests
// =============================================================================

// EmbeddingProviderConfig defines configuration for an embedding provider
type EmbeddingProviderConfig struct {
	Name       string
	Models     []string
	Dimensions []int
}

// AllEmbeddingProviders defines the list of supported embedding providers
var AllEmbeddingProviders = []EmbeddingProviderConfig{
	{Name: "openai", Models: []string{"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"}, Dimensions: []int{512, 1536, 3072}},
	{Name: "cohere", Models: []string{"embed-english-v3.0", "embed-multilingual-v3.0"}, Dimensions: []int{1024, 4096}},
	{Name: "voyage", Models: []string{"voyage-3", "voyage-code-3"}, Dimensions: []int{512, 1024, 1536}},
	{Name: "jina", Models: []string{"jina-embeddings-v3"}, Dimensions: []int{256, 512, 1024}},
	{Name: "google", Models: []string{"text-embedding-005"}, Dimensions: []int{768}},
	{Name: "bedrock", Models: []string{"amazon.titan-embed-text-v1"}, Dimensions: []int{1024, 1536}},
}

// TestEmbeddingProviders tests all embedding provider endpoints
func TestEmbeddingProviders(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	for _, provider := range AllEmbeddingProviders {
		t.Run(provider.Name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"provider": provider.Name,
				"model":    provider.Models[0],
				"input":    []string{"Hello, world!", "This is a test."},
			}

			reqData, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := http.Post(
				baseURL+"/v1/embeddings",
				"application/json",
				bytes.NewBuffer(reqData),
			)
			if err != nil {
				t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Embedding provider %s response status: %d, body: %.200s", provider.Name, resp.StatusCode, string(body))

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err == nil {
					if data, ok := result["data"].([]interface{}); ok {
						t.Logf("Got %d embeddings", len(data))
					}
				}
			}
		})
	}
}

// TestEmbeddingBatch tests batch embedding capabilities
func TestEmbeddingBatch(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Create a batch of texts
	texts := make([]string, 100)
	for i := range texts {
		texts[i] = "This is test text number " + string(rune('0'+i%10))
	}

	reqBody := map[string]interface{}{
		"provider": "openai",
		"model":    "text-embedding-3-small",
		"input":    texts,
	}

	reqData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	start := time.Now()
	resp, err := http.Post(
		baseURL+"/v1/embeddings",
		"application/json",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		return
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	t.Logf("Batch embedding of %d texts took %v", len(texts), elapsed)
}

// TestEmbeddingWithVectorStores tests embedding integration with vector stores
func TestEmbeddingWithVectorStores(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	vectorStores := []string{"qdrant", "pinecone", "milvus", "pgvector"}

	for _, store := range vectorStores {
		t.Run(store, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"vector_store": store,
				"embedding": map[string]interface{}{
					"provider": "openai",
					"model":    "text-embedding-3-small",
				},
				"texts": []string{"test document 1", "test document 2"},
			}

			reqData, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := http.Post(
				baseURL+"/v1/embeddings/store",
				"application/json",
				bytes.NewBuffer(reqData),
			)
			if err != nil {
				t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
				return
			}
			defer resp.Body.Close()

			t.Logf("Embedding+%s response status: %d", store, resp.StatusCode)
		})
	}
}

// =============================================================================
// Vision Tests
// =============================================================================

// VisionCapability defines a vision capability
type VisionCapability struct {
	Name        string
	Endpoint    string
	Description string
}

// AllVisionCapabilities defines the list of supported vision capabilities
var AllVisionCapabilities = []VisionCapability{
	{Name: "image-analysis", Endpoint: "/v1/vision/analyze", Description: "General image analysis"},
	{Name: "ocr", Endpoint: "/v1/vision/ocr", Description: "Optical character recognition"},
	{Name: "object-detection", Endpoint: "/v1/vision/detect", Description: "Object detection"},
	{Name: "face-detection", Endpoint: "/v1/vision/faces", Description: "Face detection"},
	{Name: "scene-understanding", Endpoint: "/v1/vision/scene", Description: "Scene understanding"},
	{Name: "image-captioning", Endpoint: "/v1/vision/caption", Description: "Image captioning"},
	{Name: "image-comparison", Endpoint: "/v1/vision/compare", Description: "Image comparison"},
}

// TestVisionEndpoints tests all vision endpoints
func TestVisionEndpoints(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	for _, cap := range AllVisionCapabilities {
		t.Run(cap.Name, func(t *testing.T) {
			// Test with a small base64 test image (1x1 white pixel PNG)
			testImage := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

			reqBody := map[string]interface{}{
				"image":  testImage,
				"format": "base64",
			}

			reqData, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := http.Post(
				baseURL+cap.Endpoint,
				"application/json",
				bytes.NewBuffer(reqData),
			)
			if err != nil {
				t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Vision %s response status: %d, body: %.200s", cap.Name, resp.StatusCode, string(body))
		})
	}
}

// TestVisionWithLLMProviders tests vision integration with LLM providers
func TestVisionWithLLMProviders(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Vision-capable LLM providers
	providers := []string{"claude", "gemini", "openai"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			// Test with a small base64 test image
			testImage := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

			reqBody := map[string]interface{}{
				"provider": provider,
				"image":    testImage,
				"format":   "base64",
				"prompt":   "What do you see in this image?",
			}

			reqData, err := json.Marshal(reqBody)
			require.NoError(t, err)

			resp, err := http.Post(
				baseURL+"/v1/vision/analyze",
				"application/json",
				bytes.NewBuffer(reqData),
			)
			if err != nil {
				t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
				return
			}
			defer resp.Body.Close()

			t.Logf("Vision+%s response status: %d", provider, resp.StatusCode)
		})
	}
}

// TestVisionOCR tests OCR functionality
func TestVisionOCR(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Test image with text (would need a real image in production)
	reqBody := map[string]interface{}{
		"image_url": "https://example.com/test-image-with-text.png",
		"languages": []string{"en", "de", "fr"},
	}

	reqData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(
		baseURL+"/v1/vision/ocr",
		"application/json",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("OCR response: %.500s", string(body))
}

// =============================================================================
// Cross-Protocol Integration Tests
// =============================================================================

// TestProtocolsWithAIDebate tests all protocols within AI Debate system
func TestProtocolsWithAIDebate(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Test debate with multiple protocols enabled
	reqBody := map[string]interface{}{
		"topic": "How should we structure the codebase for optimal maintainability?",
		"participants": []map[string]interface{}{
			{"name": "Architect", "role": "expert", "provider": "claude"},
			{"name": "Developer", "role": "analyst", "provider": "gemini"},
			{"name": "QA", "role": "critic", "provider": "deepseek"},
		},
		"enabled_protocols": []string{"mcp", "lsp", "acp", "embeddings"},
		"mcp_servers":       []string{"filesystem", "memory", "git"},
		"lsp_servers":       []string{"gopls", "pyright"},
		"acp_agents":        []string{"code-reviewer", "bug-finder"},
	}

	reqData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(
		baseURL+"/v1/debates",
		"application/json",
		bytes.NewBuffer(reqData),
	)
	if err != nil {
		t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Multi-protocol debate response status: %d", resp.StatusCode)
	t.Logf("Response (first 1000 chars): %.1000s", string(body))
}

// TestProtocolHealthCheck tests health endpoints for all protocols
func TestProtocolHealthCheck(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	endpoints := []struct {
		name     string
		endpoint string
	}{
		{"MCP", "/v1/mcp/health"},
		{"LSP", "/v1/lsp/health"},
		{"ACP", "/v1/acp/health"},
		{"Embeddings", "/v1/embeddings/health"},
		{"Vision", "/v1/vision/health"},
		{"Cognee", "/v1/cognee/health"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			resp, err := http.Get(baseURL + ep.endpoint)
			if err != nil {
				t.Skipf("HelixAgent not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
				return
			}
			defer resp.Body.Close()

			_, _ = io.ReadAll(resp.Body)
			status := "healthy"
			if resp.StatusCode != http.StatusOK {
				status = "unhealthy"
			}
			t.Logf("Protocol %s: %s (status: %d)", ep.name, status, resp.StatusCode)
			assert.Contains(t, []int{200, 503}, resp.StatusCode, "Status should be 200 OK or 503 Service Unavailable")
		})
	}
}

// BenchmarkEmbeddings benchmarks embedding generation
func BenchmarkEmbeddings(b *testing.B) {
	baseURL := os.Getenv("HELIXAGENT_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	reqBody := map[string]interface{}{
		"provider": "openai",
		"model":    "text-embedding-3-small",
		"input":    []string{"Hello, world!"},
	}
	reqData, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Post(
			baseURL+"/v1/embeddings",
			"application/json",
			bytes.NewBuffer(reqData),
		)
		if err != nil {
			b.Skipf("HelixAgent not running: %v", err)
			return
		}
		resp.Body.Close()
	}
}
