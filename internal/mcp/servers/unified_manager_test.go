// Package servers provides MCP server adapters for various services.
package servers

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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockServerAdapter implements ServerAdapter for testing.
//
// Concurrency model (CONST-029): connected → atomic.Bool; healthError
// → atomic.Pointer[error]; tools is immutable post-ctor (Pattern Alpha).
type MockServerAdapter struct {
	connected   atomic.Bool
	healthError atomic.Pointer[error]
	tools       []MCPTool
}

func NewMockAdapter() *MockServerAdapter {
	return &MockServerAdapter{
		tools: []MCPTool{
			{Name: "mock_tool", Description: "A mock tool"},
		},
	}
}

func (m *MockServerAdapter) Connect(ctx context.Context) error {
	m.connected.Store(true)
	return nil
}

func (m *MockServerAdapter) IsConnected() bool {
	return m.connected.Load()
}

func (m *MockServerAdapter) Health(ctx context.Context) error {
	if ep := m.healthError.Load(); ep != nil {
		return *ep
	}
	return nil
}

func (m *MockServerAdapter) Close() error {
	m.connected.Store(false)
	return nil
}

func (m *MockServerAdapter) GetMCPTools() []MCPTool {
	return m.tools
}

func (m *MockServerAdapter) SetHealthError(err error) {
	if err == nil {
		m.healthError.Store(nil)
		return
	}
	m.healthError.Store(&err)
}

func TestNewUnifiedServerManager(t *testing.T) {
	tests := []struct {
		name           string
		config         UnifiedManagerConfig
		expectedLazy   bool
		expectedPeriod time.Duration
	}{
		{
			name: "with custom config",
			config: UnifiedManagerConfig{
				Logger:       logrus.New(),
				LazyInit:     true,
				HealthPeriod: 60 * time.Second,
			},
			expectedLazy:   true,
			expectedPeriod: 60 * time.Second,
		},
		{
			name:           "with default values",
			config:         UnifiedManagerConfig{},
			expectedLazy:   false,
			expectedPeriod: 30 * time.Second,
		},
		{
			name: "with provided configs",
			config: UnifiedManagerConfig{
				Configs: map[string]ServerAdapterConfig{
					"custom": {
						Type:    "chroma",
						BaseURL: "http://custom:8000",
						Enabled: true,
					},
				},
			},
			expectedPeriod: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewUnifiedServerManager(tt.config)
			assert.NotNil(t, manager)
			assert.Equal(t, tt.expectedLazy, manager.lazyInit)
			assert.Equal(t, tt.expectedPeriod, manager.healthPeriod)
			assert.NotNil(t, manager.adapters)
			assert.NotNil(t, manager.configs)
			assert.NotNil(t, manager.logger)
		})
	}
}

func TestUnifiedServerManager_loadDefaultConfigs(t *testing.T) {
	// Set environment variables for testing
	t.Setenv("CHROMA_URL", "http://chroma:8000")
	t.Setenv("CHROMA_AUTH_TOKEN", "chroma-token")
	t.Setenv("QDRANT_URL", "http://qdrant:6333")
	t.Setenv("QDRANT_API_KEY", "qdrant-key")
	t.Setenv("WEAVIATE_URL", "http://weaviate:8080")
	t.Setenv("WEAVIATE_API_KEY", "weaviate-key")

	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	// Check Chroma config
	chromaConfig, exists := manager.configs.Get("chroma")
	assert.True(t, exists)
	assert.Equal(t, "chroma", chromaConfig.Type)
	assert.Equal(t, "http://chroma:8000", chromaConfig.BaseURL)
	assert.Equal(t, "chroma-token", chromaConfig.AuthToken)
	assert.True(t, chromaConfig.Enabled)

	// Check Qdrant config
	qdrantConfig, exists := manager.configs.Get("qdrant")
	assert.True(t, exists)
	assert.Equal(t, "qdrant", qdrantConfig.Type)
	assert.Equal(t, "http://qdrant:6333", qdrantConfig.BaseURL)
	assert.Equal(t, "qdrant-key", qdrantConfig.APIKey)

	// Check Weaviate config
	weaviateConfig, exists := manager.configs.Get("weaviate")
	assert.True(t, exists)
	assert.Equal(t, "weaviate", weaviateConfig.Type)
	assert.Equal(t, "http://weaviate:8080", weaviateConfig.BaseURL)
	assert.Equal(t, "weaviate-key", weaviateConfig.APIKey)
}

func TestUnifiedServerManager_Initialize(t *testing.T) {
	// Create mock servers for each adapter type
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer chromaServer.Close()

	qdrantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer qdrantServer.Close()

	tests := []struct {
		name        string
		config      UnifiedManagerConfig
		expectError bool
	}{
		{
			name: "successful initialization",
			config: UnifiedManagerConfig{
				Configs: map[string]ServerAdapterConfig{
					"chroma": {
						Type:    "chroma",
						BaseURL: chromaServer.URL,
						Enabled: true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "lazy initialization",
			config: UnifiedManagerConfig{
				LazyInit: true,
				Configs: map[string]ServerAdapterConfig{
					"chroma": {
						Type:    "chroma",
						BaseURL: chromaServer.URL,
						Enabled: true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "disabled adapter skipped",
			config: UnifiedManagerConfig{
				Configs: map[string]ServerAdapterConfig{
					"chroma": {
						Type:    "chroma",
						BaseURL: "http://invalid",
						Enabled: false,
					},
				},
			},
			expectError: false,
		},
		{
			name: "unknown adapter type logged",
			config: UnifiedManagerConfig{
				Configs: map[string]ServerAdapterConfig{
					"unknown": {
						Type:    "unknown",
						BaseURL: "http://unknown",
						Enabled: true,
					},
				},
			},
			expectError: false, // Errors are logged but don't stop initialization
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewUnifiedServerManager(tt.config)
			err := manager.Initialize(context.Background())
			defer func() { _ = manager.Close() }()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnifiedServerManager_createAdapter(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	tests := []struct {
		name        string
		adapterName string
		config      ServerAdapterConfig
		expectError bool
	}{
		{
			name:        "create chroma adapter",
			adapterName: "chroma",
			config: ServerAdapterConfig{
				Type:    "chroma",
				BaseURL: "http://localhost:8000",
			},
			expectError: false,
		},
		{
			name:        "create qdrant adapter",
			adapterName: "qdrant",
			config: ServerAdapterConfig{
				Type:    "qdrant",
				BaseURL: "http://localhost:6333",
			},
			expectError: false,
		},
		{
			name:        "create weaviate adapter",
			adapterName: "weaviate",
			config: ServerAdapterConfig{
				Type:    "weaviate",
				BaseURL: "http://localhost:8080",
			},
			expectError: false,
		},
		{
			name:        "unknown adapter type",
			adapterName: "unknown",
			config: ServerAdapterConfig{
				Type:    "unknown",
				BaseURL: "http://localhost:9999",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := manager.createAdapter(tt.adapterName, tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, adapter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, adapter)
			}
		})
	}
}

func TestUnifiedServerManager_GetAdapter(t *testing.T) {
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer chromaServer.Close()

	tests := []struct {
		name        string
		setup       func(*UnifiedServerManager)
		adapterName string
		expectError bool
	}{
		{
			name: "get existing connected adapter",
			setup: func(m *UnifiedServerManager) {
				adapter := NewMockAdapter()
				_ = adapter.Connect(context.Background())
				m.adapters.Put("test", adapter)
			},
			adapterName: "test",
			expectError: false,
		},
		{
			name: "lazy initialize adapter",
			setup: func(m *UnifiedServerManager) {
				m.configs.Put("chroma", ServerAdapterConfig{
					Type:    "chroma",
					BaseURL: chromaServer.URL,
					Enabled: true,
				})
			},
			adapterName: "chroma",
			expectError: false,
		},
		{
			name: "adapter not configured",
			setup: func(m *UnifiedServerManager) {
				// No setup - adapter not configured
			},
			adapterName: "nonexistent",
			expectError: true,
		},
		{
			name: "adapter disabled",
			setup: func(m *UnifiedServerManager) {
				m.configs.Put("disabled", ServerAdapterConfig{
					Type:    "chroma",
					BaseURL: "http://localhost:8000",
					Enabled: false,
				})
			},
			adapterName: "disabled",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewUnifiedServerManager(UnifiedManagerConfig{})
			tt.setup(manager)

			adapter, err := manager.GetAdapter(context.Background(), tt.adapterName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, adapter)
			}
		})
	}
}

func TestUnifiedServerManager_GetTypedAdapters(t *testing.T) {
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer chromaServer.Close()

	qdrantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer qdrantServer.Close()

	weaviateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer weaviateServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"chroma": {
				Type:    "chroma",
				BaseURL: chromaServer.URL,
				Enabled: true,
			},
			"qdrant": {
				Type:    "qdrant",
				BaseURL: qdrantServer.URL,
				Enabled: true,
			},
			"weaviate": {
				Type:    "weaviate",
				BaseURL: weaviateServer.URL,
				Enabled: true,
			},
		},
	})

	t.Run("GetChromaAdapter", func(t *testing.T) {
		adapter, err := manager.GetChromaAdapter(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.IsType(t, &ChromaAdapter{}, adapter)
	})

	t.Run("GetQdrantAdapter", func(t *testing.T) {
		adapter, err := manager.GetQdrantAdapter(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.IsType(t, &QdrantAdapter{}, adapter)
	})

	t.Run("GetWeaviateAdapter", func(t *testing.T) {
		adapter, err := manager.GetWeaviateAdapter(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.IsType(t, &WeaviateAdapter{}, adapter)
	})
}

func TestUnifiedServerManager_GetTypedAdapters_WrongType(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	// Register a mock adapter with the wrong name
	mockAdapter := NewMockAdapter()
	_ = mockAdapter.Connect(context.Background())

	manager.adapters.Put("chroma", mockAdapter)

	_, err := manager.GetChromaAdapter(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a ChromaAdapter")
}

func TestUnifiedServerManager_ListAdapters(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"chroma":   {Type: "chroma", Enabled: true},
			"qdrant":   {Type: "qdrant", Enabled: true},
			"weaviate": {Type: "weaviate", Enabled: false},
		},
	})

	adapters := manager.ListAdapters()
	assert.Len(t, adapters, 3)
	assert.Contains(t, adapters, "chroma")
	assert.Contains(t, adapters, "qdrant")
	assert.Contains(t, adapters, "weaviate")
}

func TestUnifiedServerManager_ListConnectedAdapters(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	// Register mock adapters
	connectedAdapter := NewMockAdapter()
	_ = connectedAdapter.Connect(context.Background())

	disconnectedAdapter := NewMockAdapter()
	// Don't connect this one

	manager.adapters.Put("connected", connectedAdapter)
	manager.adapters.Put("disconnected", disconnectedAdapter)

	connected := manager.ListConnectedAdapters()
	assert.Len(t, connected, 1)
	assert.Contains(t, connected, "connected")
}

func TestUnifiedServerManager_GetAllTools(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	// Register mock adapters with different tools
	adapter1 := NewMockAdapter()
	adapter1.tools = []MCPTool{{Name: "tool1"}, {Name: "tool2"}}
	_ = adapter1.Connect(context.Background())

	adapter2 := NewMockAdapter()
	adapter2.tools = []MCPTool{{Name: "tool3"}}
	_ = adapter2.Connect(context.Background())

	disconnectedAdapter := NewMockAdapter()
	disconnectedAdapter.tools = []MCPTool{{Name: "tool4"}}
	// Don't connect this one

	manager.adapters.Put("adapter1", adapter1)
	manager.adapters.Put("adapter2", adapter2)
	manager.adapters.Put("disconnected", disconnectedAdapter)

	tools := manager.GetAllTools()
	assert.Len(t, tools, 3) // Only tools from connected adapters
}

func TestUnifiedServerManager_GetToolsByAdapter(t *testing.T) {
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer chromaServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"chroma": {
				Type:    "chroma",
				BaseURL: chromaServer.URL,
				Enabled: true,
			},
		},
	})

	tools, err := manager.GetToolsByAdapter(context.Background(), "chroma")
	assert.NoError(t, err)
	assert.NotEmpty(t, tools)
}

func TestUnifiedServerManager_Health(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	healthyAdapter := NewMockAdapter()
	_ = healthyAdapter.Connect(context.Background())

	unhealthyAdapter := NewMockAdapter()
	_ = unhealthyAdapter.Connect(context.Background())
	unhealthyAdapter.SetHealthError(errors.New("unhealthy"))

	manager.adapters.Put("healthy", healthyAdapter)
	manager.adapters.Put("unhealthy", unhealthyAdapter)

	results := manager.Health(context.Background())
	assert.Len(t, results, 2)
	assert.NoError(t, results["healthy"])
	assert.Error(t, results["unhealthy"])
}

func TestUnifiedServerManager_RegisterAdapter(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	mockAdapter := NewMockAdapter()
	manager.RegisterAdapter("custom", mockAdapter)

	registered, exists := manager.adapters.Get("custom")

	assert.True(t, exists)
	assert.Equal(t, mockAdapter, registered)
}

func TestUnifiedServerManager_UnregisterAdapter(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	mockAdapter := NewMockAdapter()
	_ = mockAdapter.Connect(context.Background())
	manager.adapters.Put("test", mockAdapter)

	err := manager.UnregisterAdapter("test")
	assert.NoError(t, err)

	_, exists := manager.adapters.Get("test")

	assert.False(t, exists)
}

func TestUnifiedServerManager_UnregisterAdapter_NotFound(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	err := manager.UnregisterAdapter("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUnifiedServerManager_Close(t *testing.T) {
	manager := NewUnifiedServerManager(UnifiedManagerConfig{})

	adapter1 := NewMockAdapter()
	_ = adapter1.Connect(context.Background())
	adapter2 := NewMockAdapter()
	_ = adapter2.Connect(context.Background())

	manager.adapters.Put("adapter1", adapter1)
	manager.adapters.Put("adapter2", adapter2)

	err := manager.Close()
	assert.NoError(t, err)

	// Verify all adapters were disconnected
	assert.False(t, adapter1.IsConnected())
	assert.False(t, adapter2.IsConnected())

	// Verify adapters map is cleared
	assert.Equal(t, 0, manager.adapters.Len())
}

func TestUnifiedServerManager_GetAdapterStatuses(t *testing.T) {
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer chromaServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"chroma": {
				Type:    "chroma",
				BaseURL: chromaServer.URL,
				Enabled: true,
			},
			"disabled": {
				Type:    "qdrant",
				BaseURL: "http://localhost:6333",
				Enabled: false,
			},
		},
	})

	// Initialize to create the chroma adapter
	err := manager.Initialize(context.Background())
	require.NoError(t, err)
	defer func() { _ = manager.Close() }()

	statuses := manager.GetAdapterStatuses(context.Background())
	assert.Len(t, statuses, 2)

	// Find statuses by name
	statusMap := make(map[string]AdapterStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s
	}

	chromaStatus := statusMap["chroma"]
	assert.Equal(t, "chroma", chromaStatus.Type)
	assert.True(t, chromaStatus.Enabled)
	assert.True(t, chromaStatus.Connected)

	disabledStatus := statusMap["disabled"]
	assert.Equal(t, "qdrant", disabledStatus.Type)
	assert.False(t, disabledStatus.Enabled)
	assert.False(t, disabledStatus.Connected)
}

func TestUnifiedServerManager_ExecuteTool(t *testing.T) {
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/heartbeat" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v1/collections" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]ChromaCollection{
				{ID: "1", Name: "test-collection"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer chromaServer.Close()

	qdrantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/collections" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(QdrantCollectionsResponse{
				Result: struct {
					Collections []QdrantCollection `json:"collections"`
				}{
					Collections: []QdrantCollection{{Name: "test"}},
				},
				Status: "ok",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer qdrantServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"chroma": {
				Type:    "chroma",
				BaseURL: chromaServer.URL,
				Enabled: true,
			},
			"qdrant": {
				Type:    "qdrant",
				BaseURL: qdrantServer.URL,
				Enabled: true,
			},
		},
	})

	tests := []struct {
		name        string
		toolName    string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name:        "execute chroma_list_collections",
			toolName:    "chroma_list_collections",
			args:        map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "execute qdrant_list_collections",
			toolName:    "qdrant_list_collections",
			args:        map[string]interface{}{},
			expectError: false,
		},
		{
			name:        "unknown tool prefix",
			toolName:    "unknown_tool",
			args:        map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.ExecuteTool(context.Background(), tt.toolName, tt.args)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestUnifiedServerManager_ExecuteTool_ChromaTools(t *testing.T) {
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/heartbeat":
			w.WriteHeader(http.StatusOK)
		case "/api/v1/collections":
			if r.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode([]ChromaCollection{{ID: "1", Name: "test"}})
			} else if r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(ChromaCollection{ID: "2", Name: "new"})
			}
		case "/api/v1/collections/test":
			w.WriteHeader(http.StatusNoContent)
		case "/api/v1/collections/test/count":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(42)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer chromaServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"chroma": {
				Type:    "chroma",
				BaseURL: chromaServer.URL,
				Enabled: true,
			},
		},
	})

	t.Run("chroma_list_collections", func(t *testing.T) {
		result, err := manager.ExecuteTool(context.Background(), "chroma_list_collections", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("chroma_create_collection", func(t *testing.T) {
		result, err := manager.ExecuteTool(context.Background(), "chroma_create_collection", map[string]interface{}{
			"name":     "new",
			"metadata": map[string]interface{}{},
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("chroma_delete_collection", func(t *testing.T) {
		result, err := manager.ExecuteTool(context.Background(), "chroma_delete_collection", map[string]interface{}{
			"name": "test",
		})
		assert.NoError(t, err)
		assert.Nil(t, result) // Delete returns nil on success
	})

	t.Run("chroma_count", func(t *testing.T) {
		result, err := manager.ExecuteTool(context.Background(), "chroma_count", map[string]interface{}{
			"collection": "test",
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(42), result)
	})

	t.Run("unknown chroma tool", func(t *testing.T) {
		_, err := manager.ExecuteTool(context.Background(), "chroma_unknown", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown chroma tool")
	})
}

func TestUnifiedServerManager_ExecuteTool_QdrantTools(t *testing.T) {
	qdrantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/readyz":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/collections":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(QdrantCollectionsResponse{
				Result: struct {
					Collections []QdrantCollection `json:"collections"`
				}{Collections: []QdrantCollection{{Name: "test"}}},
				Status: "ok",
			})
		case r.URL.Path == "/collections/new":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/collections/test":
			if r.Method == "DELETE" {
				w.WriteHeader(http.StatusOK)
			}
		case r.URL.Path == "/collections/test/points/count":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"result": map[string]uint64{"count": 100},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer qdrantServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"qdrant": {
				Type:    "qdrant",
				BaseURL: qdrantServer.URL,
				Enabled: true,
			},
		},
	})

	t.Run("qdrant_list_collections", func(t *testing.T) {
		result, err := manager.ExecuteTool(context.Background(), "qdrant_list_collections", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("qdrant_create_collection", func(t *testing.T) {
		_, err := manager.ExecuteTool(context.Background(), "qdrant_create_collection", map[string]interface{}{
			"name":        "new",
			"vector_size": float64(1536),
			"distance":    "Cosine",
		})
		assert.NoError(t, err)
	})

	t.Run("qdrant_delete_collection", func(t *testing.T) {
		_, err := manager.ExecuteTool(context.Background(), "qdrant_delete_collection", map[string]interface{}{
			"name": "test",
		})
		assert.NoError(t, err)
	})

	t.Run("qdrant_count_points", func(t *testing.T) {
		result, err := manager.ExecuteTool(context.Background(), "qdrant_count_points", map[string]interface{}{
			"collection": "test",
		})
		assert.NoError(t, err)
		assert.Equal(t, uint64(100), result)
	})

	t.Run("unknown qdrant tool", func(t *testing.T) {
		_, err := manager.ExecuteTool(context.Background(), "qdrant_unknown", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown qdrant tool")
	})
}

func TestUnifiedServerManager_ExecuteTool_WeaviateTools(t *testing.T) {
	weaviateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/.well-known/ready":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/v1/schema":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(WeaviateSchema{
				Classes: []WeaviateClass{{Class: "Test"}},
			})
		case r.URL.Path == "/v1/schema/Test":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer weaviateServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"weaviate": {
				Type:    "weaviate",
				BaseURL: weaviateServer.URL,
				Enabled: true,
			},
		},
	})

	t.Run("weaviate_list_classes", func(t *testing.T) {
		result, err := manager.ExecuteTool(context.Background(), "weaviate_list_classes", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("weaviate_delete_class", func(t *testing.T) {
		_, err := manager.ExecuteTool(context.Background(), "weaviate_delete_class", map[string]interface{}{
			"class": "Test",
		})
		assert.NoError(t, err)
	})

	t.Run("unknown weaviate tool", func(t *testing.T) {
		_, err := manager.ExecuteTool(context.Background(), "weaviate_unknown", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown weaviate tool")
	})
}

func TestUnifiedServerManager_Concurrency(t *testing.T) {
	chromaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v1/collections" {
			_ = json.NewEncoder(w).Encode([]ChromaCollection{})
		}
	}))
	defer chromaServer.Close()

	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		Configs: map[string]ServerAdapterConfig{
			"chroma": {
				Type:    "chroma",
				BaseURL: chromaServer.URL,
				Enabled: true,
			},
		},
	})

	// Test concurrent access
	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.GetAdapter(context.Background(), "chroma")
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestUnifiedServerManager_HealthCheckLoop(t *testing.T) {
	// This test verifies the health check loop starts and can be stopped
	manager := NewUnifiedServerManager(UnifiedManagerConfig{
		HealthPeriod: 10 * time.Millisecond, // Very short period for testing
	})

	mockAdapter := NewMockAdapter()
	_ = mockAdapter.Connect(context.Background())
	manager.adapters.Put("test", mockAdapter)

	// Start the health check loop
	go manager.healthCheckLoop()

	// Wait a bit for at least one health check to run
	time.Sleep(50 * time.Millisecond)

	// Close should stop the health check loop
	_ = manager.Close()

	// Give it time to stop
	time.Sleep(20 * time.Millisecond)
}
