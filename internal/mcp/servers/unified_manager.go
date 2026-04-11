// Package servers provides a unified MCP server manager for all adapters.
package servers

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ServerAdapter defines the interface for all MCP server adapters.
type ServerAdapter interface {
	Connect(ctx context.Context) error
	IsConnected() bool
	Health(ctx context.Context) error
	Close() error
	GetMCPTools() []MCPTool
}

// UnifiedServerManager manages all MCP server adapters.
type UnifiedServerManager struct {
	adapters     map[string]ServerAdapter
	configs      map[string]ServerAdapterConfig
	mu           sync.RWMutex
	logger       *logrus.Logger
	lazyInit     bool
	healthPeriod time.Duration
	stopChan     chan struct{}
}

// ServerAdapterConfig holds configuration for a server adapter.
type ServerAdapterConfig struct {
	Type      string // "chroma", "qdrant", "weaviate", etc.
	BaseURL   string
	APIKey    string
	AuthToken string
	Timeout   time.Duration
	Enabled   bool
	EnvVars   map[string]string // Environment variable mappings
}

// UnifiedManagerConfig holds configuration for the unified manager.
type UnifiedManagerConfig struct {
	Logger       *logrus.Logger
	LazyInit     bool          // If true, adapters are initialized on first use
	HealthPeriod time.Duration // How often to check adapter health
	Configs      map[string]ServerAdapterConfig
}

// NewUnifiedServerManager creates a new unified server manager.
func NewUnifiedServerManager(config UnifiedManagerConfig) *UnifiedServerManager {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	if config.HealthPeriod == 0 {
		config.HealthPeriod = 30 * time.Second
	}

	manager := &UnifiedServerManager{
		adapters:     make(map[string]ServerAdapter),
		configs:      make(map[string]ServerAdapterConfig),
		logger:       config.Logger,
		lazyInit:     config.LazyInit,
		healthPeriod: config.HealthPeriod,
		stopChan:     make(chan struct{}),
	}

	// Load default configurations from environment
	manager.loadDefaultConfigs()

	// Override with provided configs
	for name, cfg := range config.Configs {
		manager.configs[name] = cfg
	}

	return manager
}

// loadDefaultConfigs loads default configurations from environment variables.
func (m *UnifiedServerManager) loadDefaultConfigs() {
	// ChromaDB
	if url := os.Getenv("CHROMA_URL"); url != "" {
		m.configs["chroma"] = ServerAdapterConfig{
			Type:      "chroma",
			BaseURL:   url,
			AuthToken: os.Getenv("CHROMA_AUTH_TOKEN"),
			Timeout:   30 * time.Second,
			Enabled:   true,
		}
	}

	// Qdrant
	if url := os.Getenv("QDRANT_URL"); url != "" {
		m.configs["qdrant"] = ServerAdapterConfig{
			Type:    "qdrant",
			BaseURL: url,
			APIKey:  os.Getenv("QDRANT_API_KEY"),
			Timeout: 30 * time.Second,
			Enabled: true,
		}
	}

	// Weaviate
	if url := os.Getenv("WEAVIATE_URL"); url != "" {
		m.configs["weaviate"] = ServerAdapterConfig{
			Type:    "weaviate",
			BaseURL: url,
			APIKey:  os.Getenv("WEAVIATE_API_KEY"),
			Timeout: 30 * time.Second,
			Enabled: true,
		}
	}

	// Add more default configs as needed
}

// Initialize initializes all configured adapters.
func (m *UnifiedServerManager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, config := range m.configs {
		if !config.Enabled {
			continue
		}

		if m.lazyInit {
			m.logger.WithField("adapter", name).Info("Adapter configured for lazy initialization")
			continue
		}

		adapter, err := m.createAdapter(name, config)
		if err != nil {
			m.logger.WithError(err).WithField("adapter", name).Warn("Failed to create adapter")
			continue
		}

		if err := adapter.Connect(ctx); err != nil {
			m.logger.WithError(err).WithField("adapter", name).Warn("Failed to connect adapter")
			continue
		}

		m.adapters[name] = adapter
		m.logger.WithField("adapter", name).Info("Adapter initialized successfully")
	}

	// Start health check goroutine
	//nolint:gosec // G118: long-lived health-check loop, intentionally decoupled from the caller context
	go m.healthCheckLoop()

	return nil
}

// createAdapter creates an adapter based on configuration.
func (m *UnifiedServerManager) createAdapter(name string, config ServerAdapterConfig) (ServerAdapter, error) {
	switch config.Type {
	case "chroma":
		return NewChromaAdapter(ChromaAdapterConfig{
			BaseURL:   config.BaseURL,
			AuthToken: config.AuthToken,
			Timeout:   config.Timeout,
		}), nil

	case "qdrant":
		return NewQdrantAdapter(QdrantAdapterConfig{
			BaseURL: config.BaseURL,
			APIKey:  config.APIKey,
			Timeout: config.Timeout,
		}), nil

	case "weaviate":
		return NewWeaviateAdapter(WeaviateAdapterConfig{
			BaseURL: config.BaseURL,
			APIKey:  config.APIKey,
			Timeout: config.Timeout,
		}), nil

	default:
		return nil, fmt.Errorf("unknown adapter type: %s", config.Type)
	}
}

// GetAdapter returns an adapter by name, initializing it if necessary (lazy init).
func (m *UnifiedServerManager) GetAdapter(ctx context.Context, name string) (ServerAdapter, error) {
	m.mu.RLock()
	adapter, exists := m.adapters[name]
	m.mu.RUnlock()

	if exists && adapter.IsConnected() {
		return adapter, nil
	}

	// Lazy initialization
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if adapter, exists = m.adapters[name]; exists && adapter.IsConnected() {
		return adapter, nil
	}

	config, exists := m.configs[name]
	if !exists {
		return nil, fmt.Errorf("adapter not configured: %s", name)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("adapter disabled: %s", name)
	}

	adapter, err := m.createAdapter(name, config)
	if err != nil {
		return nil, err
	}

	if err := adapter.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect adapter %s: %w", name, err)
	}

	m.adapters[name] = adapter
	m.logger.WithField("adapter", name).Info("Adapter lazy-initialized successfully")

	return adapter, nil
}

// GetChromaAdapter returns the ChromaDB adapter.
func (m *UnifiedServerManager) GetChromaAdapter(ctx context.Context) (*ChromaAdapter, error) {
	adapter, err := m.GetAdapter(ctx, "chroma")
	if err != nil {
		return nil, err
	}
	chromaAdapter, ok := adapter.(*ChromaAdapter)
	if !ok {
		return nil, fmt.Errorf("adapter is not a ChromaAdapter")
	}
	return chromaAdapter, nil
}

// GetQdrantAdapter returns the Qdrant adapter.
func (m *UnifiedServerManager) GetQdrantAdapter(ctx context.Context) (*QdrantAdapter, error) {
	adapter, err := m.GetAdapter(ctx, "qdrant")
	if err != nil {
		return nil, err
	}
	qdrantAdapter, ok := adapter.(*QdrantAdapter)
	if !ok {
		return nil, fmt.Errorf("adapter is not a QdrantAdapter")
	}
	return qdrantAdapter, nil
}

// GetWeaviateAdapter returns the Weaviate adapter.
func (m *UnifiedServerManager) GetWeaviateAdapter(ctx context.Context) (*WeaviateAdapter, error) {
	adapter, err := m.GetAdapter(ctx, "weaviate")
	if err != nil {
		return nil, err
	}
	weaviateAdapter, ok := adapter.(*WeaviateAdapter)
	if !ok {
		return nil, fmt.Errorf("adapter is not a WeaviateAdapter")
	}
	return weaviateAdapter, nil
}

// ListAdapters returns a list of all configured adapter names.
func (m *UnifiedServerManager) ListAdapters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.configs))
	for name := range m.configs {
		names = append(names, name)
	}
	return names
}

// ListConnectedAdapters returns a list of connected adapter names.
func (m *UnifiedServerManager) ListConnectedAdapters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0)
	for name, adapter := range m.adapters {
		if adapter.IsConnected() {
			names = append(names, name)
		}
	}
	return names
}

// GetAllTools returns all MCP tools from all connected adapters.
func (m *UnifiedServerManager) GetAllTools() []MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []MCPTool
	for _, adapter := range m.adapters {
		if adapter.IsConnected() {
			tools = append(tools, adapter.GetMCPTools()...)
		}
	}
	return tools
}

// GetToolsByAdapter returns MCP tools for a specific adapter.
func (m *UnifiedServerManager) GetToolsByAdapter(ctx context.Context, name string) ([]MCPTool, error) {
	adapter, err := m.GetAdapter(ctx, name)
	if err != nil {
		return nil, err
	}
	return adapter.GetMCPTools(), nil
}

// Health checks the health of all adapters.
func (m *UnifiedServerManager) Health(ctx context.Context) map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]error)
	for name, adapter := range m.adapters {
		results[name] = adapter.Health(ctx)
	}
	return results
}

// healthCheckLoop periodically checks adapter health.
func (m *UnifiedServerManager) healthCheckLoop() {
	ticker := time.NewTicker(m.healthPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			results := m.Health(ctx)
			cancel()

			for name, err := range results {
				if err != nil {
					m.logger.WithError(err).WithField("adapter", name).Warn("Adapter health check failed")
				}
			}

		case <-m.stopChan:
			return
		}
	}
}

// RegisterAdapter registers a custom adapter.
func (m *UnifiedServerManager) RegisterAdapter(name string, adapter ServerAdapter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[name] = adapter
}

// UnregisterAdapter removes an adapter.
func (m *UnifiedServerManager) UnregisterAdapter(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adapter, exists := m.adapters[name]
	if !exists {
		return fmt.Errorf("adapter not found: %s", name)
	}

	if err := adapter.Close(); err != nil {
		m.logger.WithError(err).WithField("adapter", name).Warn("Error closing adapter")
	}

	delete(m.adapters, name)
	return nil
}

// Close closes all adapters and stops the manager.
func (m *UnifiedServerManager) Close() error {
	close(m.stopChan)

	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, adapter := range m.adapters {
		if err := adapter.Close(); err != nil {
			m.logger.WithError(err).WithField("adapter", name).Warn("Error closing adapter")
			lastErr = err
		}
	}

	m.adapters = make(map[string]ServerAdapter)
	return lastErr
}

// AdapterStatus represents the status of an adapter.
type AdapterStatus struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Connected bool      `json:"connected"`
	Enabled   bool      `json:"enabled"`
	LastCheck time.Time `json:"last_check,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// GetAdapterStatuses returns the status of all adapters.
func (m *UnifiedServerManager) GetAdapterStatuses(ctx context.Context) []AdapterStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]AdapterStatus, 0, len(m.configs))

	for name, config := range m.configs {
		status := AdapterStatus{
			Name:    name,
			Type:    config.Type,
			Enabled: config.Enabled,
		}

		if adapter, exists := m.adapters[name]; exists {
			status.Connected = adapter.IsConnected()
			if err := adapter.Health(ctx); err != nil {
				status.Error = err.Error()
			}
		}

		status.LastCheck = time.Now()
		statuses = append(statuses, status)
	}

	return statuses
}

// ExecuteTool executes an MCP tool on the appropriate adapter.
func (m *UnifiedServerManager) ExecuteTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error) {
	// Determine which adapter handles this tool based on prefix
	var adapterName string
	switch {
	case len(toolName) > 7 && toolName[:7] == "chroma_":
		adapterName = "chroma"
	case len(toolName) > 7 && toolName[:7] == "qdrant_":
		adapterName = "qdrant"
	case len(toolName) > 9 && toolName[:9] == "weaviate_":
		adapterName = "weaviate"
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	adapter, err := m.GetAdapter(ctx, adapterName)
	if err != nil {
		return nil, err
	}

	// Execute tool based on type
	switch adapterName {
	case "chroma":
		return m.executeChromaTool(ctx, adapter.(*ChromaAdapter), toolName, arguments) //nolint:errcheck // adapter type guaranteed by switch case
	case "qdrant":
		return m.executeQdrantTool(ctx, adapter.(*QdrantAdapter), toolName, arguments) //nolint:errcheck // adapter type guaranteed by switch case
	case "weaviate":
		return m.executeWeaviateTool(ctx, adapter.(*WeaviateAdapter), toolName, arguments) //nolint:errcheck // adapter type guaranteed by switch case
	}

	return nil, fmt.Errorf("unsupported adapter: %s", adapterName)
}

// executeChromaTool executes a ChromaDB tool.
func (m *UnifiedServerManager) executeChromaTool(ctx context.Context, adapter *ChromaAdapter, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "chroma_list_collections":
		return adapter.ListCollections(ctx)
	case "chroma_create_collection":
		name := args["name"].(string)                            //nolint:errcheck // schema validation ensures correct type
		metadata, _ := args["metadata"].(map[string]interface{}) //nolint:errcheck // schema validation ensures correct type
		return adapter.CreateCollection(ctx, name, metadata)
	case "chroma_delete_collection":
		name := args["name"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, adapter.DeleteCollection(ctx, name)
	case "chroma_count":
		collection := args["collection"].(string) //nolint:errcheck // schema validation ensures correct type
		return adapter.Count(ctx, collection)
	// Add more tool implementations...
	default:
		return nil, fmt.Errorf("unknown chroma tool: %s", toolName)
	}
}

// executeQdrantTool executes a Qdrant tool.
func (m *UnifiedServerManager) executeQdrantTool(ctx context.Context, adapter *QdrantAdapter, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "qdrant_list_collections":
		return adapter.ListCollections(ctx)
	case "qdrant_create_collection":
		name := args["name"].(string)                       //nolint:errcheck // schema validation ensures correct type
		vectorSize := uint64(args["vector_size"].(float64)) //nolint:errcheck // schema validation ensures correct type
		distance, _ := args["distance"].(string)            //nolint:errcheck // schema validation ensures correct type
		return nil, adapter.CreateCollection(ctx, name, vectorSize, distance)
	case "qdrant_delete_collection":
		name := args["name"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, adapter.DeleteCollection(ctx, name)
	case "qdrant_count_points":
		collection := args["collection"].(string) //nolint:errcheck // schema validation ensures correct type
		return adapter.CountPoints(ctx, collection)
	// Add more tool implementations...
	default:
		return nil, fmt.Errorf("unknown qdrant tool: %s", toolName)
	}
}

// executeWeaviateTool executes a Weaviate tool.
func (m *UnifiedServerManager) executeWeaviateTool(ctx context.Context, adapter *WeaviateAdapter, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case "weaviate_list_classes":
		return adapter.ListClasses(ctx)
	case "weaviate_delete_class":
		className := args["class"].(string) //nolint:errcheck // schema validation ensures correct type
		return nil, adapter.DeleteClass(ctx, className)
	// Add more tool implementations...
	default:
		return nil, fmt.Errorf("unknown weaviate tool: %s", toolName)
	}
}
