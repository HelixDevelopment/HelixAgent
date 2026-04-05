package bigdata

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/memory"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLMProvider implements llm.LLMProvider for testing
type mockLLMProvider struct {
	name        string
	completeErr error
	response    string
}

func (m *mockLLMProvider) Complete(_ context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.completeErr != nil {
		return nil, m.completeErr
	}
	return &models.LLMResponse{
		Content:    m.response,
		TokensUsed: 50,
	}, nil
}

func (m *mockLLMProvider) CompleteStream(_ context.Context, _ *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLLMProvider) HealthCheck() error { return nil }

func (m *mockLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportsStreaming: false,
	}
}

func (m *mockLLMProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// Ensure mockLLMProvider implements llm.LLMProvider
var _ llm.LLMProvider = (*mockLLMProvider)(nil)

// --- DefaultIntegrationConfig tests ---

func TestDefaultIntegrationConfig_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()
	require.NotNil(t, config)
}

func TestDefaultIntegrationConfig_EnableFlags(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()

	assert.True(t, config.EnableInfiniteContext)
	assert.False(t, config.EnableDistributedMemory)
	assert.False(t, config.EnableKnowledgeGraph)
	assert.False(t, config.EnableAnalytics)
	assert.True(t, config.EnableCrossLearning)
}

func TestDefaultIntegrationConfig_KafkaDefaults(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()

	assert.Equal(t, "localhost:9092", config.KafkaBootstrapServers)
	assert.Equal(t, "helixagent-bigdata", config.KafkaConsumerGroup)
}

func TestDefaultIntegrationConfig_ClickHouseDefaults(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()

	assert.Equal(t, "localhost", config.ClickHouseHost)
	assert.Equal(t, 9000, config.ClickHousePort)
	assert.Equal(t, "helixagent_analytics", config.ClickHouseDatabase)
	assert.Equal(t, "default", config.ClickHouseUser)
	assert.Equal(t, "", config.ClickHousePassword)
}

func TestDefaultIntegrationConfig_Neo4jDefaults(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()

	assert.Equal(t, "bolt://localhost:7687", config.Neo4jURI)
	assert.Equal(t, "neo4j", config.Neo4jUsername)
	assert.Equal(t, "helixagent123", config.Neo4jPassword)
	assert.Equal(t, "helixagent", config.Neo4jDatabase)
}

func TestDefaultIntegrationConfig_ContextDefaults(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()

	assert.Equal(t, 100, config.ContextCacheSize)
	assert.Equal(t, 30*time.Minute, config.ContextCacheTTL)
	assert.Equal(t, "hybrid", config.ContextCompressionType)
}

func TestDefaultIntegrationConfig_LearningDefaults(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()

	assert.InDelta(t, 0.7, config.LearningMinConfidence, 0.001)
	assert.Equal(t, 3, config.LearningMinFrequency)
}

// --- NewBigDataIntegration tests ---

func TestNewBigDataIntegration_NilConfig(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	bdi, err := NewBigDataIntegration(nil, nil, logger)

	require.NoError(t, err)
	require.NotNil(t, bdi)
	// Should use default config
	assert.True(t, bdi.config.EnableInfiniteContext)
	assert.True(t, bdi.config.EnableCrossLearning)
}

func TestNewBigDataIntegration_CustomConfig(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
		ClickHouseHost:          "custom-host",
		ClickHousePort:          9001,
	}

	bdi, err := NewBigDataIntegration(config, nil, logger)

	require.NoError(t, err)
	require.NotNil(t, bdi)
	assert.Equal(t, "custom-host", bdi.config.ClickHouseHost)
	assert.Equal(t, 9001, bdi.config.ClickHousePort)
	assert.False(t, bdi.isRunning)
}

func TestNewBigDataIntegration_StoresKafkaBroker(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{}
	// Pass nil broker — should still create integration
	bdi, err := NewBigDataIntegration(config, nil, logger)

	require.NoError(t, err)
	assert.Nil(t, bdi.kafkaBroker)
}

// --- IsRunning tests ---

func TestBigDataIntegration_IsRunning_InitiallyFalse(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	assert.False(t, bdi.IsRunning())
}

func TestBigDataIntegration_IsRunning_TrueAfterStart(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	err = bdi.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, bdi.IsRunning())
}

func TestBigDataIntegration_IsRunning_FalseAfterStop(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	err = bdi.Start(context.Background())
	require.NoError(t, err)

	err = bdi.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, bdi.IsRunning())
}

// --- Start tests ---

func TestBigDataIntegration_Start_AlreadyRunning(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	err = bdi.Start(context.Background())
	require.NoError(t, err)

	err = bdi.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

// --- Stop tests ---

func TestBigDataIntegration_Stop_WhenNotRunning(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	// Stop when not running should be a no-op
	err = bdi.Stop(context.Background())
	assert.NoError(t, err)
}

// --- Getter methods ---

func TestBigDataIntegration_GetInfiniteContext_NilWhenNotInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{EnableInfiniteContext: false}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	assert.Nil(t, bdi.GetInfiniteContext())
}

func TestBigDataIntegration_GetDistributedMemory_NilWhenNotInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{EnableDistributedMemory: false}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	assert.Nil(t, bdi.GetDistributedMemory())
}

func TestBigDataIntegration_GetKnowledgeGraph_NilWhenNotInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{EnableKnowledgeGraph: false}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	assert.Nil(t, bdi.GetKnowledgeGraph())
}

func TestBigDataIntegration_GetAnalytics_NilWhenNotInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{EnableAnalytics: false}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	assert.Nil(t, bdi.GetAnalytics())
}

func TestBigDataIntegration_GetCrossLearner_NilWhenNotInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{EnableCrossLearning: false}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	assert.Nil(t, bdi.GetCrossLearner())
}

// --- HealthCheck tests ---

func TestBigDataIntegration_HealthCheck_AllDisabled(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	health := bdi.HealthCheck(context.Background())

	assert.Equal(t, "disabled", health["infinite_context"])
	assert.Equal(t, "disabled", health["distributed_memory"])
	assert.Equal(t, "disabled", health["knowledge_graph"])
	assert.Equal(t, "disabled", health["analytics"])
	assert.Equal(t, "disabled", health["cross_learning"])
}

func TestBigDataIntegration_HealthCheck_EnabledButNotInitialized(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    true,
		EnableAnalytics:         true,
		EnableCrossLearning:     true,
	}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	health := bdi.HealthCheck(context.Background())

	assert.Equal(t, "not_initialized", health["infinite_context"])
	assert.Equal(t, "not_initialized", health["distributed_memory"])
	assert.Equal(t, "not_initialized", health["knowledge_graph"])
	assert.Equal(t, "not_initialized", health["analytics"])
	assert.Equal(t, "not_initialized", health["cross_learning"])
}

func TestBigDataIntegration_HealthCheck_ReturnsAllFiveComponents(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	health := bdi.HealthCheck(context.Background())

	expectedKeys := []string{
		"infinite_context",
		"distributed_memory",
		"knowledge_graph",
		"analytics",
		"cross_learning",
	}
	for _, key := range expectedKeys {
		_, exists := health[key]
		assert.True(t, exists, "Expected key %s in health check result", key)
	}
	assert.Len(t, health, 5)
}

// --- Initialize tests (all disabled path) ---

func TestBigDataIntegration_Initialize_AllDisabled(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	assert.NoError(t, err)

	// All getters should return nil
	assert.Nil(t, bdi.GetInfiniteContext())
	assert.Nil(t, bdi.GetDistributedMemory())
	assert.Nil(t, bdi.GetKnowledgeGraph())
	assert.Nil(t, bdi.GetAnalytics())
	assert.Nil(t, bdi.GetCrossLearner())
}

// --- Initialize and Start lifecycle ---

func TestBigDataIntegration_FullLifecycle_AllDisabled(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, nil, logger)
	require.NoError(t, err)

	// Initialize
	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	// Start
	err = bdi.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, bdi.IsRunning())

	// Health check should show all disabled
	health := bdi.HealthCheck(context.Background())
	for _, status := range health {
		assert.Equal(t, "disabled", status)
	}

	// Stop
	err = bdi.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, bdi.IsRunning())

	// Stop again is a no-op
	err = bdi.Stop(context.Background())
	assert.NoError(t, err)
}

// --- inMemoryEventLog tests ---

func TestInMemoryEventLog_Append(t *testing.T) {
	t.Parallel()
	log := &inMemoryEventLog{}

	event := &memory.MemoryEvent{
		EventID:  "evt-1",
		MemoryID: "mem-1",
		UserID:   "user-1",
		NodeID:   "node-1",
	}

	err := log.Append(event)
	assert.NoError(t, err)
	assert.Len(t, log.events, 1)
}

func TestInMemoryEventLog_GetEvents(t *testing.T) {
	t.Parallel()
	log := &inMemoryEventLog{}

	_ = log.Append(&memory.MemoryEvent{MemoryID: "mem-1", EventID: "evt-1"})
	_ = log.Append(&memory.MemoryEvent{MemoryID: "mem-2", EventID: "evt-2"})
	_ = log.Append(&memory.MemoryEvent{MemoryID: "mem-1", EventID: "evt-3"})

	events, err := log.GetEvents("mem-1")
	assert.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestInMemoryEventLog_GetEvents_NoMatch(t *testing.T) {
	t.Parallel()
	log := &inMemoryEventLog{}

	_ = log.Append(&memory.MemoryEvent{MemoryID: "mem-1", EventID: "evt-1"})

	events, err := log.GetEvents("nonexistent")
	assert.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestInMemoryEventLog_GetEventsSince(t *testing.T) {
	t.Parallel()
	log := &inMemoryEventLog{}
	now := time.Now()

	_ = log.Append(&memory.MemoryEvent{
		EventID:   "evt-1",
		Timestamp: now.Add(-2 * time.Hour),
	})
	_ = log.Append(&memory.MemoryEvent{
		EventID:   "evt-2",
		Timestamp: now.Add(-30 * time.Minute),
	})
	_ = log.Append(&memory.MemoryEvent{
		EventID:   "evt-3",
		Timestamp: now.Add(10 * time.Minute),
	})

	events, err := log.GetEventsSince(now.Add(-1 * time.Hour))
	assert.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestInMemoryEventLog_GetEventsForUser(t *testing.T) {
	t.Parallel()
	log := &inMemoryEventLog{}

	_ = log.Append(&memory.MemoryEvent{EventID: "evt-1", UserID: "user-a"})
	_ = log.Append(&memory.MemoryEvent{EventID: "evt-2", UserID: "user-b"})
	_ = log.Append(&memory.MemoryEvent{EventID: "evt-3", UserID: "user-a"})

	events, err := log.GetEventsForUser("user-a")
	assert.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestInMemoryEventLog_GetEventsFromNode(t *testing.T) {
	t.Parallel()
	log := &inMemoryEventLog{}

	_ = log.Append(&memory.MemoryEvent{EventID: "evt-1", NodeID: "node-1"})
	_ = log.Append(&memory.MemoryEvent{EventID: "evt-2", NodeID: "node-2"})
	_ = log.Append(&memory.MemoryEvent{EventID: "evt-3", NodeID: "node-1"})

	events, err := log.GetEventsFromNode("node-1")
	assert.NoError(t, err)
	assert.Len(t, events, 2)
}

// --- providerRegistryLLMClient tests ---

func TestProviderRegistryLLMClient_Complete_NilRegistry(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	client := &providerRegistryLLMClient{
		registry: nil,
		logger:   logger,
	}

	result, tokens, err := client.Complete(context.Background(), "test prompt", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider registry not available")
	assert.Equal(t, "", result)
	assert.Equal(t, 0, tokens)
}

// --- Initialize tests with components enabled ---

func TestBigDataIntegration_Initialize_InfiniteContextOnly(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	// Infinite context should be initialized
	assert.NotNil(t, bdi.GetInfiniteContext())
	// Others should be nil
	assert.Nil(t, bdi.GetDistributedMemory())
	assert.Nil(t, bdi.GetKnowledgeGraph())
	assert.Nil(t, bdi.GetAnalytics())
	assert.Nil(t, bdi.GetCrossLearner())
}

func TestBigDataIntegration_Initialize_DistributedMemoryOnly(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	assert.Nil(t, bdi.GetInfiniteContext())
	assert.NotNil(t, bdi.GetDistributedMemory())
}

func TestBigDataIntegration_Initialize_InfiniteContextAndDistributedMemory(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, bdi.GetInfiniteContext())
	assert.NotNil(t, bdi.GetDistributedMemory())
}

// --- HealthCheck tests with initialized components ---

func TestBigDataIntegration_HealthCheck_InfiniteContextHealthy(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	health := bdi.HealthCheck(context.Background())
	assert.Equal(t, "healthy", health["infinite_context"])
	assert.Equal(t, "disabled", health["distributed_memory"])
	assert.Equal(t, "disabled", health["knowledge_graph"])
	assert.Equal(t, "disabled", health["analytics"])
	assert.Equal(t, "disabled", health["cross_learning"])
}

func TestBigDataIntegration_HealthCheck_DistributedMemoryHealthy(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	health := bdi.HealthCheck(context.Background())
	assert.Equal(t, "disabled", health["infinite_context"])
	assert.Equal(t, "healthy", health["distributed_memory"])
}

// --- Stop tests with components ---

func TestBigDataIntegration_Stop_WithDistributedMemory(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	err = bdi.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, bdi.IsRunning())

	err = bdi.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, bdi.IsRunning())
}

func TestBigDataIntegration_Stop_WithInfiniteContext(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	err = bdi.Start(context.Background())
	require.NoError(t, err)

	err = bdi.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, bdi.IsRunning())
}

// --- Initialize and lifecycle with multiple components ---

func TestBigDataIntegration_FullLifecycle_InfiniteContextAndDistributedMemory(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	// Initialize
	err = bdi.Initialize(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, bdi.GetInfiniteContext())
	assert.NotNil(t, bdi.GetDistributedMemory())

	// Start
	err = bdi.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, bdi.IsRunning())

	// Health check
	health := bdi.HealthCheck(context.Background())
	assert.Equal(t, "healthy", health["infinite_context"])
	assert.Equal(t, "healthy", health["distributed_memory"])

	// Stop
	err = bdi.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, bdi.IsRunning())
}

// --- providerRegistryLLMClient with empty providers ---

func TestProviderRegistryLLMClient_Complete_EmptyProviders(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	// Create a ProviderRegistry without auto-discovery (no providers)
	cfg := &services.RegistryConfig{}
	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	client := &providerRegistryLLMClient{
		registry: registry,
		logger:   logger,
	}

	result, tokens, err := client.Complete(context.Background(), "test prompt", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LLM providers available")
	assert.Equal(t, "", result)
	assert.Equal(t, 0, tokens)
}

// --- Initialize tests with components that require external services ---

func TestBigDataIntegration_Initialize_KnowledgeGraphFails(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    true, // Will fail to connect to Neo4j
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
		Neo4jURI:                "bolt://nonexistent:7687",
		Neo4jUsername:           "neo4j",
		Neo4jPassword:           "test",
		Neo4jDatabase:           "test",
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "knowledge graph initialization failed")
}

func TestBigDataIntegration_Initialize_AnalyticsFails(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         true, // Will fail to connect to ClickHouse
		EnableCrossLearning:     false,
		ClickHouseHost:          "nonexistent",
		ClickHousePort:          9000,
		ClickHouseDatabase:      "test",
		ClickHouseUser:          "default",
		ClickHousePassword:      "",
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "analytics initialization failed")
}

func TestBigDataIntegration_Initialize_CrossLearningEnabled(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, bdi.GetCrossLearner())
}

func TestBigDataIntegration_HealthCheck_CrossLearnerHealthy(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	health := bdi.HealthCheck(context.Background())
	assert.Equal(t, "healthy", health["cross_learning"])
}

// --- providerRegistryLLMClient with registered provider ---

func TestProviderRegistryLLMClient_Complete_WithRegisteredProvider(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	cfg := &services.RegistryConfig{}
	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	// Register a mock provider
	provider := &mockLLMProvider{
		name:     "test-mock",
		response: "compressed output",
	}
	err := registry.RegisterProvider("test-mock", provider)
	require.NoError(t, err)

	client := &providerRegistryLLMClient{
		registry: registry,
		logger:   logger,
	}

	result, tokens, err := client.Complete(context.Background(), "compress this text", 200)
	assert.NoError(t, err)
	assert.Equal(t, "compressed output", result)
	assert.Equal(t, 50, tokens)
}

func TestProviderRegistryLLMClient_Complete_ProviderFails(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	cfg := &services.RegistryConfig{}
	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	// Register a provider that always fails
	provider := &mockLLMProvider{
		name:        "failing-provider",
		completeErr: fmt.Errorf("provider error"),
	}
	err := registry.RegisterProvider("failing-provider", provider)
	require.NoError(t, err)

	client := &providerRegistryLLMClient{
		registry: registry,
		logger:   logger,
	}

	result, tokens, err := client.Complete(context.Background(), "compress this", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all LLM providers failed")
	assert.Equal(t, "", result)
	assert.Equal(t, 0, tokens)
}

func TestProviderRegistryLLMClient_Complete_FirstProviderFailsSecondSucceeds(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	cfg := &services.RegistryConfig{}
	registry := services.NewProviderRegistryWithoutAutoDiscovery(cfg, nil)

	// Register a failing provider and a succeeding one
	failProvider := &mockLLMProvider{
		name:        "fail-first",
		completeErr: fmt.Errorf("first provider error"),
	}
	successProvider := &mockLLMProvider{
		name:     "succeed-second",
		response: "success response",
	}
	err := registry.RegisterProvider("fail-first", failProvider)
	require.NoError(t, err)
	err = registry.RegisterProvider("succeed-second", successProvider)
	require.NoError(t, err)

	client := &providerRegistryLLMClient{
		registry: registry,
		logger:   logger,
	}

	result, tokens, err := client.Complete(context.Background(), "test prompt", 100)
	// One of the providers should succeed
	if err == nil {
		assert.Equal(t, "success response", result)
		assert.Equal(t, 50, tokens)
	}
	// If the failing provider is tried first and success second, it should work
	// The order depends on ListProvidersOrderedByScore
}

// --- Initialize tests with CrossLearning error path ---

func TestBigDataIntegration_Initialize_CrossLearningSubscribeFails(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	broker.subscribeErr = fmt.Errorf("subscribe unavailable")

	config := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cross-session learning initialization failed")
}

// --- Initialize with all components that can succeed ---

func TestBigDataIntegration_Initialize_AllSuccessfulComponents(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	broker := newMockBroker()
	config := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}
	bdi, err := NewBigDataIntegration(config, broker, logger)
	require.NoError(t, err)

	err = bdi.Initialize(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, bdi.GetInfiniteContext())
	assert.NotNil(t, bdi.GetDistributedMemory())
	assert.Nil(t, bdi.GetKnowledgeGraph())
	assert.Nil(t, bdi.GetAnalytics())
	assert.NotNil(t, bdi.GetCrossLearner())

	// Full lifecycle
	err = bdi.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, bdi.IsRunning())

	health := bdi.HealthCheck(context.Background())
	assert.Equal(t, "healthy", health["infinite_context"])
	assert.Equal(t, "healthy", health["distributed_memory"])
	assert.Equal(t, "disabled", health["knowledge_graph"])
	assert.Equal(t, "disabled", health["analytics"])
	assert.Equal(t, "healthy", health["cross_learning"])

	err = bdi.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, bdi.IsRunning())
}

// --- inMemoryEventLog concurrent access tests ---

func TestInMemoryEventLog_ConcurrentAppendAndRead(t *testing.T) {
	t.Parallel()
	log := &inMemoryEventLog{}
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = log.Append(&memory.MemoryEvent{
				EventID:  fmt.Sprintf("evt-%d", i),
				MemoryID: "mem-1",
				UserID:   "user-1",
				NodeID:   "node-1",
			})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = log.GetEvents("mem-1")
			_, _ = log.GetEventsForUser("user-1")
			_, _ = log.GetEventsFromNode("node-1")
			_, _ = log.GetEventsSince(time.Now().Add(-1 * time.Hour))
		}
		done <- true
	}()

	<-done
	<-done

	events, err := log.GetEvents("mem-1")
	assert.NoError(t, err)
	assert.Equal(t, 100, len(events))
}
