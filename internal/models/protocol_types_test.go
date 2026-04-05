package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServer_JSON(t *testing.T) {
	t.Parallel()
	cmd := "npx server"
	server := MCPServer{
		ID:        "mcp-1",
		Name:      "test-server",
		Type:      ServerTypeLocal,
		Command:   &cmd,
		Enabled:   true,
		Tools:     json.RawMessage(`[{"name":"tool1"}]`),
		LastSync:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(server)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-server")
	assert.Contains(t, string(data), "npx server")

	var parsed MCPServer
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, "mcp-1", parsed.ID)
	assert.Equal(t, ServerTypeLocal, parsed.Type)
	assert.True(t, parsed.Enabled)
}

func TestLSPServer_Fields(t *testing.T) {
	t.Parallel()
	server := LSPServer{
		ID:           "lsp-1",
		Name:         "gopls",
		Language:     "go",
		Command:      "gopls",
		Enabled:      true,
		Workspace:    "/workspace",
		Capabilities: json.RawMessage(`[]`),
		CreatedAt:    time.Now(),
	}

	assert.Equal(t, "gopls", server.Name)
	assert.Equal(t, "go", server.Language)
	assert.True(t, server.Enabled)
}

func TestACPServer_Fields(t *testing.T) {
	t.Parallel()
	url := "http://localhost:8080"
	server := ACPServer{
		ID:      "acp-1",
		Name:    "test-acp",
		Type:    ServerTypeRemote,
		URL:     &url,
		Enabled: true,
		Tools:   json.RawMessage(`[]`),
	}

	assert.Equal(t, ServerTypeRemote, server.Type)
	require.NotNil(t, server.URL)
	assert.Equal(t, "http://localhost:8080", *server.URL)
}

func TestEmbeddingConfig_APIKeyHidden(t *testing.T) {
	t.Parallel()
	apiKey := "secret-key"
	config := EmbeddingConfig{
		ID:        1,
		Provider:  "openai",
		Model:     "text-embedding-3-small",
		Dimension: 1536,
		APIKey:    &apiKey,
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)
	// APIKey should be hidden from JSON due to json:"-"
	assert.NotContains(t, string(data), "secret-key")
}

func TestVectorDocument_EmbeddingHidden(t *testing.T) {
	t.Parallel()
	doc := VectorDocument{
		ID:                "doc-1",
		Title:             "Test Document",
		Content:           "Test content",
		Metadata:          json.RawMessage(`{}`),
		EmbeddingProvider: "openai",
		Embedding:         []float32{0.1, 0.2, 0.3},
		SearchVector:      []float32{0.4, 0.5, 0.6},
	}

	data, err := json.Marshal(doc)
	require.NoError(t, err)
	// Embedding and SearchVector should be hidden from JSON
	assert.NotContains(t, string(data), "0.1")
	assert.Contains(t, string(data), "Test Document")
}

func TestProtocolCache_Expiration(t *testing.T) {
	t.Parallel()
	cache := ProtocolCache{
		CacheKey:  "test-key",
		CacheData: json.RawMessage(`{"result":"cached"}`),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}

	assert.False(t, time.Now().After(cache.ExpiresAt))
}

func TestProtocolMetrics_Fields(t *testing.T) {
	t.Parallel()
	dur := 150
	metrics := ProtocolMetrics{
		ID:           1,
		ProtocolType: ProtocolTypeMCP,
		Operation:    "list_tools",
		Status:       MetricsStatusSuccess,
		DurationMs:   &dur,
		Metadata:     json.RawMessage(`{}`),
	}

	assert.Equal(t, ProtocolTypeMCP, metrics.ProtocolType)
	assert.Equal(t, MetricsStatusSuccess, metrics.Status)
	require.NotNil(t, metrics.DurationMs)
	assert.Equal(t, 150, *metrics.DurationMs)
}

func TestProtocolTypeConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "mcp", ProtocolTypeMCP)
	assert.Equal(t, "lsp", ProtocolTypeLSP)
	assert.Equal(t, "acp", ProtocolTypeACP)
	assert.Equal(t, "embedding", ProtocolTypeEmbedding)
}

func TestMetricsStatusConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "success", MetricsStatusSuccess)
	assert.Equal(t, "error", MetricsStatusError)
	assert.Equal(t, "timeout", MetricsStatusTimeout)
}

func TestServerTypeConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "local", ServerTypeLocal)
	assert.Equal(t, "remote", ServerTypeRemote)
}

func TestMCPTool_JSON(t *testing.T) {
	t.Parallel()
	tool := MCPTool{
		Name:        "read_file",
		Description: "Reads a file",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}

	data, err := json.Marshal(tool)
	require.NoError(t, err)
	assert.Contains(t, string(data), "read_file")
}

func TestLSPCapability_Fields(t *testing.T) {
	t.Parallel()
	cap := LSPCapability{
		Name:     "completion",
		Enabled:  true,
		Provider: "gopls",
	}

	assert.Equal(t, "completion", cap.Name)
	assert.True(t, cap.Enabled)
}

func TestVectorSearchResult_Fields(t *testing.T) {
	t.Parallel()
	doc := &VectorDocument{ID: "doc-1", Title: "Test"}
	result := VectorSearchResult{
		Document:   doc,
		Similarity: 0.95,
		Distance:   0.05,
	}

	assert.Equal(t, 0.95, result.Similarity)
	assert.Equal(t, 0.05, result.Distance)
	require.NotNil(t, result.Document)
	assert.Equal(t, "doc-1", result.Document.ID)
}

func TestCodeIntelligence_Fields(t *testing.T) {
	t.Parallel()
	ci := CodeIntelligence{
		FilePath:    "/test.go",
		Diagnostics: []*Diagnostic{},
		Completions: []*CompletionItem{},
		Symbols:     []*SymbolInfo{},
	}

	assert.Equal(t, "/test.go", ci.FilePath)
	assert.NotNil(t, ci.Diagnostics)
}

func TestDiagnostic_Fields(t *testing.T) {
	t.Parallel()
	diag := Diagnostic{
		Range:    Range{Start: Position{Line: 10, Character: 5}, End: Position{Line: 10, Character: 15}},
		Severity: 1,
		Code:     "unused_var",
		Source:   "gopls",
		Message:  "x declared and not used",
	}

	assert.Equal(t, 10, diag.Range.Start.Line)
	assert.Equal(t, "unused_var", diag.Code)
}

func TestCompletionItem_Fields(t *testing.T) {
	t.Parallel()
	item := CompletionItem{
		Label:         "Println",
		Kind:          3,
		Detail:        "func(a ...any) (n int, err error)",
		Documentation: "Println formats and prints",
		InsertText:    "Println(${1:})",
	}

	assert.Equal(t, "Println", item.Label)
	assert.Equal(t, 3, item.Kind)
}

func TestWorkspaceEdit_JSON(t *testing.T) {
	t.Parallel()
	edit := WorkspaceEdit{
		Changes: map[string][]*TextEdit{
			"file:///test.go": {
				{Range: Range{Start: Position{0, 0}, End: Position{0, 10}}, NewText: "new text"},
			},
		},
	}

	data, err := json.Marshal(edit)
	require.NoError(t, err)
	assert.Contains(t, string(data), "new text")
}

func TestSymbolInfo_WithChildren(t *testing.T) {
	t.Parallel()
	symbol := SymbolInfo{
		Name:          "MyStruct",
		Kind:          5, // struct
		ContainerName: "main",
		Children: []*SymbolInfo{
			{Name: "Method1", Kind: 6},
			{Name: "Method2", Kind: 6},
		},
	}

	assert.Equal(t, "MyStruct", symbol.Name)
	assert.Len(t, symbol.Children, 2)
}

func TestTool_JSON(t *testing.T) {
	t.Parallel()
	tool := Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "search",
			Description: "Search for documents",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	data, err := json.Marshal(tool)
	require.NoError(t, err)
	assert.Contains(t, string(data), "search")
}

func TestToolCall_JSON(t *testing.T) {
	t.Parallel()
	tc := ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"location":"NYC"}`,
		},
	}

	data, err := json.Marshal(tc)
	require.NoError(t, err)
	assert.Contains(t, string(data), "get_weather")
	assert.Contains(t, string(data), "NYC")
}
