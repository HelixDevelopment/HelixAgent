// Package adapters provides MCP server adapter registry tests.
package adapters

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/stretchr/testify/assert"
)

// MockMCPAdapter implements MCPAdapter for testing.
//
// Concurrency model (CONST-029): callResults → *safe.Store, callError
// → atomic.Pointer[error], callCount → atomic.Int64. Remaining fields
// are immutable post-construction.
type MockMCPAdapter struct {
	name         string
	version      string
	description  string
	capabilities []string
	tools        []ToolDefinition
	callResults  *safe.Store[string, *ToolResult]
	callError    atomic.Pointer[error]
	callCount    atomic.Int64
}

func NewMockMCPAdapter(name string) *MockMCPAdapter {
	return &MockMCPAdapter{
		name:         name,
		version:      "1.0.0",
		description:  "Mock adapter for testing",
		capabilities: []string{"test_capability"},
		tools: []ToolDefinition{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"input": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		callResults: safe.NewStore[string, *ToolResult](),
	}
}

func (m *MockMCPAdapter) GetServerInfo() ServerInfo {
	return ServerInfo{
		Name:         m.name,
		Version:      m.version,
		Description:  m.description,
		Capabilities: m.capabilities,
	}
}

func (m *MockMCPAdapter) ListTools() []ToolDefinition {
	return m.tools
}

func (m *MockMCPAdapter) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	m.callCount.Add(1)

	if ep := m.callError.Load(); ep != nil {
		return nil, *ep
	}

	if result, ok := m.callResults.Get(name); ok {
		return result, nil
	}

	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Called %s with %v", name, args)}},
	}, nil
}

func (m *MockMCPAdapter) SetToolResult(toolName string, result *ToolResult) {
	m.callResults.Put(toolName, result)
}

func (m *MockMCPAdapter) SetError(err error) {
	if err == nil {
		m.callError.Store(nil)
		return
	}
	m.callError.Store(&err)
}

func (m *MockMCPAdapter) GetCallCount() int {
	return int(m.callCount.Load())
}

// TestNewAdapterRegistry tests registry creation
func TestNewAdapterRegistry(t *testing.T) {
	registry := NewAdapterRegistry()

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.adapters)
	assert.NotNil(t, registry.metadata)
	assert.Equal(t, 0, registry.adapters.Len())
	assert.Equal(t, 0, registry.metadata.Len())
}

// TestAdapterRegistryRegister tests adapter registration
func TestAdapterRegistryRegister(t *testing.T) {
	registry := NewAdapterRegistry()
	adapter := NewMockMCPAdapter("test-adapter")
	metadata := AdapterMetadata{
		Name:        "test-adapter",
		Category:    CategoryUtility,
		Description: "Test adapter",
		AuthType:    "none",
		Official:    false,
		Supported:   true,
	}

	registry.Register("test-adapter", adapter, metadata)

	// Verify registration
	retrieved, ok := registry.Get("test-adapter")
	assert.True(t, ok)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-adapter", retrieved.GetServerInfo().Name)

	retrievedMeta, ok := registry.GetMetadata("test-adapter")
	assert.True(t, ok)
	assert.Equal(t, "test-adapter", retrievedMeta.Name)
	assert.Equal(t, CategoryUtility, retrievedMeta.Category)
}

// TestAdapterRegistryGet tests adapter retrieval
func TestAdapterRegistryGet(t *testing.T) {
	registry := NewAdapterRegistry()

	// Test non-existent adapter
	adapter, ok := registry.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, adapter)

	// Register and retrieve
	mockAdapter := NewMockMCPAdapter("test")
	registry.Register("test", mockAdapter, AdapterMetadata{Name: "test"})

	adapter, ok = registry.Get("test")
	assert.True(t, ok)
	assert.NotNil(t, adapter)
}

// TestAdapterRegistryGetMetadata tests metadata retrieval
func TestAdapterRegistryGetMetadata(t *testing.T) {
	registry := NewAdapterRegistry()

	// Test non-existent metadata
	meta, ok := registry.GetMetadata("nonexistent")
	assert.False(t, ok)
	assert.Empty(t, meta.Name)

	// Register and retrieve metadata
	registry.Register("test", NewMockMCPAdapter("test"), AdapterMetadata{
		Name:        "test",
		Description: "Test description",
		Category:    CategoryDatabase,
	})

	meta, ok = registry.GetMetadata("test")
	assert.True(t, ok)
	assert.Equal(t, "test", meta.Name)
	assert.Equal(t, "Test description", meta.Description)
	assert.Equal(t, CategoryDatabase, meta.Category)
}

// TestAdapterRegistryList tests listing all adapters
func TestAdapterRegistryList(t *testing.T) {
	registry := NewAdapterRegistry()

	// Empty registry
	list := registry.List()
	assert.Empty(t, list)

	// Add adapters
	registry.Register("adapter1", NewMockMCPAdapter("adapter1"), AdapterMetadata{Name: "adapter1"})
	registry.Register("adapter2", NewMockMCPAdapter("adapter2"), AdapterMetadata{Name: "adapter2"})
	registry.Register("adapter3", NewMockMCPAdapter("adapter3"), AdapterMetadata{Name: "adapter3"})

	list = registry.List()
	assert.Len(t, list, 3)
	assert.Contains(t, list, "adapter1")
	assert.Contains(t, list, "adapter2")
	assert.Contains(t, list, "adapter3")
}

// TestAdapterRegistryListByCategory tests filtering adapters by category
func TestAdapterRegistryListByCategory(t *testing.T) {
	registry := NewAdapterRegistry()

	// Add adapters with different categories
	registry.Register("db1", NewMockMCPAdapter("db1"), AdapterMetadata{Name: "db1", Category: CategoryDatabase})
	registry.Register("db2", NewMockMCPAdapter("db2"), AdapterMetadata{Name: "db2", Category: CategoryDatabase})
	registry.Register("storage1", NewMockMCPAdapter("storage1"), AdapterMetadata{Name: "storage1", Category: CategoryStorage})
	registry.Register("ai1", NewMockMCPAdapter("ai1"), AdapterMetadata{Name: "ai1", Category: CategoryAI})

	// Test database category
	dbAdapters := registry.ListByCategory(CategoryDatabase)
	assert.Len(t, dbAdapters, 2)
	assert.Contains(t, dbAdapters, "db1")
	assert.Contains(t, dbAdapters, "db2")

	// Test storage category
	storageAdapters := registry.ListByCategory(CategoryStorage)
	assert.Len(t, storageAdapters, 1)
	assert.Contains(t, storageAdapters, "storage1")

	// Test AI category
	aiAdapters := registry.ListByCategory(CategoryAI)
	assert.Len(t, aiAdapters, 1)
	assert.Contains(t, aiAdapters, "ai1")

	// Test empty category
	emptyAdapters := registry.ListByCategory(CategoryAutomation)
	assert.Empty(t, emptyAdapters)
}

// TestAdapterRegistryListAll tests listing all metadata
func TestAdapterRegistryListAll(t *testing.T) {
	registry := NewAdapterRegistry()

	// Empty registry
	all := registry.ListAll()
	assert.Empty(t, all)

	// Add adapters
	registry.Register("test1", NewMockMCPAdapter("test1"), AdapterMetadata{Name: "test1", Description: "First"})
	registry.Register("test2", NewMockMCPAdapter("test2"), AdapterMetadata{Name: "test2", Description: "Second"})

	all = registry.ListAll()
	assert.Len(t, all, 2)

	names := make([]string, 0)
	for _, meta := range all {
		names = append(names, meta.Name)
	}
	assert.Contains(t, names, "test1")
	assert.Contains(t, names, "test2")
}

// TestAdapterRegistryCallTool tests calling tools through registry
func TestAdapterRegistryCallTool(t *testing.T) {
	registry := NewAdapterRegistry()

	// Test with non-existent adapter
	result, err := registry.CallTool(context.Background(), "nonexistent", "tool", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "adapter not found")

	// Register adapter and call tool
	mockAdapter := NewMockMCPAdapter("test")
	mockAdapter.SetToolResult("test_tool", &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "Success"}},
	})
	registry.Register("test", mockAdapter, AdapterMetadata{Name: "test"})

	result, err = registry.CallTool(context.Background(), "test", "test_tool", map[string]interface{}{
		"input": "hello",
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Success", result.Content[0].Text)
}

// TestAdapterCategoriesConstants tests category constants
func TestAdapterCategoriesConstants(t *testing.T) {
	categories := []AdapterCategory{
		CategoryDatabase,
		CategoryStorage,
		CategoryVersionControl,
		CategoryProductivity,
		CategoryCommunication,
		CategorySearch,
		CategoryAutomation,
		CategoryInfrastructure,
		CategoryAnalytics,
		CategoryAI,
		CategoryUtility,
		CategoryDesign,
		CategoryCollaboration,
	}

	assert.Len(t, categories, 13)
	assert.Equal(t, AdapterCategory("database"), CategoryDatabase)
	assert.Equal(t, AdapterCategory("storage"), CategoryStorage)
	assert.Equal(t, AdapterCategory("version_control"), CategoryVersionControl)
	assert.Equal(t, AdapterCategory("productivity"), CategoryProductivity)
	assert.Equal(t, AdapterCategory("communication"), CategoryCommunication)
	assert.Equal(t, AdapterCategory("search"), CategorySearch)
	assert.Equal(t, AdapterCategory("automation"), CategoryAutomation)
	assert.Equal(t, AdapterCategory("infrastructure"), CategoryInfrastructure)
	assert.Equal(t, AdapterCategory("analytics"), CategoryAnalytics)
	assert.Equal(t, AdapterCategory("ai"), CategoryAI)
	assert.Equal(t, AdapterCategory("utility"), CategoryUtility)
	assert.Equal(t, AdapterCategory("design"), CategoryDesign)
	assert.Equal(t, AdapterCategory("collaboration"), CategoryCollaboration)
}

// TestServerInfo tests ServerInfo struct
func TestServerInfo(t *testing.T) {
	info := ServerInfo{
		Name:         "test-server",
		Version:      "2.0.0",
		Description:  "Test server description",
		Capabilities: []string{"cap1", "cap2", "cap3"},
	}

	assert.Equal(t, "test-server", info.Name)
	assert.Equal(t, "2.0.0", info.Version)
	assert.Equal(t, "Test server description", info.Description)
	assert.Len(t, info.Capabilities, 3)
}

// TestToolDefinition tests ToolDefinition struct
func TestToolDefinition(t *testing.T) {
	tool := ToolDefinition{
		Name:        "my_tool",
		Description: "Does something useful",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{"type": "string"},
				"param2": map[string]interface{}{"type": "integer"},
			},
			"required": []string{"param1"},
		},
	}

	assert.Equal(t, "my_tool", tool.Name)
	assert.Equal(t, "Does something useful", tool.Description)
	assert.NotNil(t, tool.InputSchema)
	assert.Equal(t, "object", tool.InputSchema["type"])
}

// TestToolResult tests ToolResult struct
func TestToolResult(t *testing.T) {
	// Success result
	result := ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "Hello"},
			{Type: "text", Text: "World"},
		},
		IsError: false,
	}

	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 2)
	assert.Equal(t, "Hello", result.Content[0].Text)

	// Error result
	errorResult := ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "Error: something went wrong"},
		},
		IsError: true,
	}

	assert.True(t, errorResult.IsError)
	assert.Contains(t, errorResult.Content[0].Text, "Error")
}

// TestContentBlock tests ContentBlock struct
func TestContentBlock(t *testing.T) {
	tests := []struct {
		name     string
		block    ContentBlock
		expected ContentBlock
	}{
		{
			name: "text block",
			block: ContentBlock{
				Type: "text",
				Text: "Hello World",
			},
			expected: ContentBlock{Type: "text", Text: "Hello World"},
		},
		{
			name: "base64 image block",
			block: ContentBlock{
				Type:     "image",
				MimeType: "image/png",
				Data:     "iVBORw0KGgo=",
			},
			expected: ContentBlock{Type: "image", MimeType: "image/png", Data: "iVBORw0KGgo="},
		},
		{
			name: "json block",
			block: ContentBlock{
				Type:     "text",
				MimeType: "application/json",
				Text:     `{"key": "value"}`,
			},
			expected: ContentBlock{Type: "text", MimeType: "application/json", Text: `{"key": "value"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected.Type, tt.block.Type)
			assert.Equal(t, tt.expected.Text, tt.block.Text)
			assert.Equal(t, tt.expected.MimeType, tt.block.MimeType)
			assert.Equal(t, tt.expected.Data, tt.block.Data)
		})
	}
}

// TestAdapterMetadata tests AdapterMetadata struct
func TestAdapterMetadata(t *testing.T) {
	meta := AdapterMetadata{
		Name:        "postgresql",
		Category:    CategoryDatabase,
		Description: "PostgreSQL database operations",
		AuthType:    "connection_string",
		DocsURL:     "https://example.com/docs",
		Official:    true,
		Supported:   true,
	}

	assert.Equal(t, "postgresql", meta.Name)
	assert.Equal(t, CategoryDatabase, meta.Category)
	assert.Equal(t, "PostgreSQL database operations", meta.Description)
	assert.Equal(t, "connection_string", meta.AuthType)
	assert.Equal(t, "https://example.com/docs", meta.DocsURL)
	assert.True(t, meta.Official)
	assert.True(t, meta.Supported)
}

// TestAvailableAdapters tests the AvailableAdapters list
func TestAvailableAdapters(t *testing.T) {
	// Should have many adapters
	assert.True(t, len(AvailableAdapters) > 30, "Should have more than 30 adapters defined")

	// Check for specific important adapters
	adapterNames := make(map[string]bool)
	for _, adapter := range AvailableAdapters {
		adapterNames[adapter.Name] = true
	}

	// Database adapters
	assert.True(t, adapterNames["postgresql"], "Should have postgresql adapter")
	assert.True(t, adapterNames["sqlite"], "Should have sqlite adapter")
	assert.True(t, adapterNames["redis"], "Should have redis adapter")
	assert.True(t, adapterNames["mongodb"], "Should have mongodb adapter")

	// Storage adapters
	assert.True(t, adapterNames["aws-s3"], "Should have aws-s3 adapter")
	assert.True(t, adapterNames["google-drive"], "Should have google-drive adapter")

	// Version control
	assert.True(t, adapterNames["github"], "Should have github adapter")
	assert.True(t, adapterNames["gitlab"], "Should have gitlab adapter")

	// AI adapters
	assert.True(t, adapterNames["replicate"], "Should have replicate adapter")

	// Utility adapters
	assert.True(t, adapterNames["fetch"], "Should have fetch adapter")
	assert.True(t, adapterNames["filesystem"], "Should have filesystem adapter")
}

// TestGetAdapterCount tests adapter count function
func TestGetAdapterCount(t *testing.T) {
	count := GetAdapterCount()
	assert.True(t, count > 30, "Should have more than 30 adapters")
	assert.Equal(t, len(AvailableAdapters), count)
}

// TestGetSupportedAdapters tests filtering supported adapters
func TestGetSupportedAdapters(t *testing.T) {
	supported := GetSupportedAdapters()

	// All returned should be supported
	for _, adapter := range supported {
		assert.True(t, adapter.Supported, "Adapter %s should be supported", adapter.Name)
	}

	// Should have many supported adapters
	assert.True(t, len(supported) > 20, "Should have many supported adapters")
}

// TestGetOfficialAdapters tests filtering official adapters
func TestGetOfficialAdapters(t *testing.T) {
	official := GetOfficialAdapters()

	// All returned should be official
	for _, adapter := range official {
		assert.True(t, adapter.Official, "Adapter %s should be official", adapter.Name)
	}

	// Should have some official adapters
	assert.True(t, len(official) > 5, "Should have official adapters")

	// Check specific official adapters
	officialNames := make(map[string]bool)
	for _, adapter := range official {
		officialNames[adapter.Name] = true
	}

	assert.True(t, officialNames["postgresql"], "PostgreSQL should be official")
	assert.True(t, officialNames["sqlite"], "SQLite should be official")
	assert.True(t, officialNames["github"], "GitHub should be official")
	assert.True(t, officialNames["slack"], "Slack should be official")
	assert.True(t, officialNames["fetch"], "Fetch should be official")
}

// TestInitializeDefaultRegistry tests default registry initialization
func TestInitializeDefaultRegistry(t *testing.T) {
	// Default registry should be initialized via init()
	assert.NotNil(t, DefaultRegistry)

	// Should have metadata for all available adapters
	for _, meta := range AvailableAdapters {
		retrieved, ok := DefaultRegistry.GetMetadata(meta.Name)
		assert.True(t, ok, "Should have metadata for %s", meta.Name)
		assert.Equal(t, meta.Name, retrieved.Name)
	}
}

// TestConcurrentRegistryAccess tests thread safety
func TestConcurrentRegistryAccess(t *testing.T) {
	registry := NewAdapterRegistry()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("adapter-%d", idx)
			registry.Register(name, NewMockMCPAdapter(name), AdapterMetadata{Name: name})
		}(i)
	}

	wg.Wait()

	// Verify all registered
	list := registry.List()
	assert.Len(t, list, numGoroutines)

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("adapter-%d", idx)
			adapter, ok := registry.Get(name)
			assert.True(t, ok)
			assert.NotNil(t, adapter)
		}(i)
	}

	wg.Wait()
}

// TestAdapterInterfaceCompliance tests that mock adapter complies with interface
func TestAdapterInterfaceCompliance(t *testing.T) {
	var _ MCPAdapter = (*MockMCPAdapter)(nil)
}

// TestToolCallWithContext tests context handling in tool calls
func TestToolCallWithContext(t *testing.T) {
	registry := NewAdapterRegistry()
	mockAdapter := NewMockMCPAdapter("test")
	registry.Register("test", mockAdapter, AdapterMetadata{Name: "test"})

	// Test with normal context
	ctx := context.Background()
	_, err := registry.CallTool(ctx, "test", "test_tool", nil)
	assert.NoError(t, err)

	// Test with cancelled context (adapter doesn't check, but registry should handle)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = registry.CallTool(ctx, "test", "test_tool", nil)
	// Mock doesn't check context, so no error expected
	assert.NoError(t, err)
}

// TestAdapterOverwrite tests overwriting an existing adapter
func TestAdapterOverwrite(t *testing.T) {
	registry := NewAdapterRegistry()

	// Register first adapter
	adapter1 := NewMockMCPAdapter("original")
	adapter1.version = "1.0.0"
	registry.Register("test", adapter1, AdapterMetadata{Name: "test", Description: "Original"})

	// Overwrite with new adapter
	adapter2 := NewMockMCPAdapter("updated")
	adapter2.version = "2.0.0"
	registry.Register("test", adapter2, AdapterMetadata{Name: "test", Description: "Updated"})

	// Should have the updated adapter
	retrieved, ok := registry.Get("test")
	assert.True(t, ok)
	assert.Equal(t, "2.0.0", retrieved.GetServerInfo().Version)

	meta, ok := registry.GetMetadata("test")
	assert.True(t, ok)
	assert.Equal(t, "Updated", meta.Description)
}

// TestEmptyToolResult tests handling empty tool results
func TestEmptyToolResult(t *testing.T) {
	result := ToolResult{
		Content: []ContentBlock{},
		IsError: false,
	}

	assert.Empty(t, result.Content)
	assert.False(t, result.IsError)
}

// TestMultipleContentBlocks tests tool results with multiple content blocks
func TestMultipleContentBlocks(t *testing.T) {
	result := ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "First line"},
			{Type: "text", Text: "Second line"},
			{Type: "image", MimeType: "image/png", Data: "base64data"},
		},
	}

	assert.Len(t, result.Content, 3)
	assert.Equal(t, "text", result.Content[0].Type)
	assert.Equal(t, "text", result.Content[1].Type)
	assert.Equal(t, "image", result.Content[2].Type)
}

// BenchmarkRegistryGet benchmarks adapter retrieval
func BenchmarkRegistryGet(b *testing.B) {
	registry := NewAdapterRegistry()
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("adapter-%d", i)
		registry.Register(name, NewMockMCPAdapter(name), AdapterMetadata{Name: name})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get("adapter-50")
	}
}

// BenchmarkRegistryList benchmarks listing adapters
func BenchmarkRegistryList(b *testing.B) {
	registry := NewAdapterRegistry()
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("adapter-%d", i)
		registry.Register(name, NewMockMCPAdapter(name), AdapterMetadata{Name: name})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.List()
	}
}

// BenchmarkRegistryCallTool benchmarks tool calling
func BenchmarkRegistryCallTool(b *testing.B) {
	registry := NewAdapterRegistry()
	registry.Register("test", NewMockMCPAdapter("test"), AdapterMetadata{Name: "test"})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = registry.CallTool(ctx, "test", "test_tool", nil)
	}
}
