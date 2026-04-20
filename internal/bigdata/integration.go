package bigdata

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/analytics"
	"dev.helix.agent/internal/conversation"
	"dev.helix.agent/internal/knowledge"
	"dev.helix.agent/internal/learning"
	"dev.helix.agent/internal/memory"
	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"github.com/sirupsen/logrus"
)

// providerRegistryLLMClient implements conversation.LLMClient using ProviderRegistry
type providerRegistryLLMClient struct {
	registry *services.ProviderRegistry
	logger   *logrus.Logger
}

func (c *providerRegistryLLMClient) Complete(ctx context.Context, prompt string, maxTokens int) (string, int, error) {
	if c.registry == nil {
		c.logger.Warn("Provider registry is nil, cannot perform LLM compression")
		return "", 0, errors.New("provider registry not available")
	}

	// Get the best provider from registry
	providers := c.registry.ListProvidersOrderedByScore()
	if len(providers) == 0 {
		c.logger.Warn("No LLM providers available for compression")
		return "", 0, errors.New("no LLM providers available")
	}

	// Try each provider in order of score
	for _, providerName := range providers {
		provider, err := c.registry.GetProvider(providerName)
		if err != nil {
			c.logger.WithError(err).Warnf("Failed to get provider %s", providerName)
			continue
		}

		// Create LLM request
		req := &models.LLMRequest{
			ID:          fmt.Sprintf("compression-%d", time.Now().UnixNano()),
			Prompt:      prompt,
			RequestType: "compression",
			ModelParams: models.ModelParameters{
				MaxTokens:   maxTokens,
				Temperature: 0.1, // Low temperature for consistent compression
			},
			CreatedAt: time.Now(),
		}

		resp, err := provider.Complete(ctx, req)
		if err != nil {
			c.logger.WithError(err).Warnf("Provider %s failed compression", providerName)
			continue
		}

		return resp.Content, resp.TokensUsed, nil
	}

	return "", 0, errors.New("all LLM providers failed compression")
}

// inMemoryEventLog implements memory.EventLog with in-memory storage.
//
// Concurrency model (CONST-029): events is a *safe.Slice — Append is
// atomic, the filter-by-predicate scans use Range. No mu needed.
type inMemoryEventLog struct {
	events *safe.Slice[*memory.MemoryEvent]
}

// newInMemoryEventLog constructs an inMemoryEventLog ready to use.
func newInMemoryEventLog() *inMemoryEventLog {
	return &inMemoryEventLog{
		events: safe.NewSlice[*memory.MemoryEvent](),
	}
}

func (d *inMemoryEventLog) Append(event *memory.MemoryEvent) error {
	d.events.Append(event)
	return nil
}

func (d *inMemoryEventLog) GetEvents(memoryID string) ([]*memory.MemoryEvent, error) {
	var result []*memory.MemoryEvent
	d.events.Range(func(_ int, event *memory.MemoryEvent) bool {
		if event.MemoryID == memoryID {
			result = append(result, event)
		}
		return true
	})
	return result, nil
}

func (d *inMemoryEventLog) GetEventsSince(timestamp time.Time) ([]*memory.MemoryEvent, error) {
	var result []*memory.MemoryEvent
	d.events.Range(func(_ int, event *memory.MemoryEvent) bool {
		if event.Timestamp.After(timestamp) {
			result = append(result, event)
		}
		return true
	})
	return result, nil
}

func (d *inMemoryEventLog) GetEventsForUser(userID string) ([]*memory.MemoryEvent, error) {
	var result []*memory.MemoryEvent
	d.events.Range(func(_ int, event *memory.MemoryEvent) bool {
		if event.UserID == userID {
			result = append(result, event)
		}
		return true
	})
	return result, nil
}

func (d *inMemoryEventLog) GetEventsFromNode(nodeID string) ([]*memory.MemoryEvent, error) {
	var result []*memory.MemoryEvent
	d.events.Range(func(_ int, event *memory.MemoryEvent) bool {
		if event.NodeID == nodeID {
			result = append(result, event)
		}
		return true
	})
	return result, nil
}

// lazyComponent provides sync.Once-based deferred initialization for a big data component.
// The factory is called at most once, on the first call to get().
type lazyComponent[T any] struct {
	once    sync.Once
	value   T
	initErr error
	factory func(ctx context.Context) (T, error)
}

// get returns the lazily initialized component. On first call the factory
// is invoked; the result (including any error) is cached for subsequent calls.
func (lc *lazyComponent[T]) get(ctx context.Context) (T, error) {
	lc.once.Do(func() {
		lc.value, lc.initErr = lc.factory(ctx)
	})
	return lc.value, lc.initErr
}

// initialized reports whether the factory has already run.
func (lc *lazyComponent[T]) initialized() bool {
	return lc.initErr != nil || isNonNil(lc.value)
}

// BigDataIntegration manages all big data components
type BigDataIntegration struct {
	// Core components (eagerly initialized — set by Initialize)
	infiniteContext     *conversation.InfiniteContextEngine
	distributedMemory   *memory.DistributedMemoryManager
	graphStreaming      *knowledge.StreamingKnowledgeGraph
	clickhouseAnalytics *analytics.ClickHouseAnalytics
	crossSessionLearner *learning.CrossSessionLearner

	// Lazy components (initialized on first access via getter)
	lazyInfiniteContext *lazyComponent[*conversation.InfiniteContextEngine]
	lazyDistributedMem  *lazyComponent[*memory.DistributedMemoryManager]
	lazyGraphStreaming  *lazyComponent[*knowledge.StreamingKnowledgeGraph]
	lazyAnalytics       *lazyComponent[*analytics.ClickHouseAnalytics]
	lazyCrossLearner    *lazyComponent[*learning.CrossSessionLearner]

	// Messaging
	kafkaBroker messaging.MessageBroker

	// Configuration
	config *IntegrationConfig
	logger *logrus.Logger

	// State
	isRunning bool
}

// IntegrationConfig holds configuration for big data integration
type IntegrationConfig struct {
	// Enable/disable individual components
	EnableInfiniteContext   bool
	EnableDistributedMemory bool
	EnableKnowledgeGraph    bool
	EnableAnalytics         bool
	EnableCrossLearning     bool

	// Kafka configuration
	KafkaBootstrapServers string
	KafkaConsumerGroup    string

	// ClickHouse configuration
	ClickHouseHost     string
	ClickHousePort     int
	ClickHouseDatabase string
	ClickHouseUser     string
	ClickHousePassword string

	// Neo4j configuration
	Neo4jURI      string
	Neo4jUsername string
	Neo4jPassword string
	Neo4jDatabase string

	// Context engine configuration
	ContextCacheSize       int
	ContextCacheTTL        time.Duration
	ContextCompressionType string

	// Learning configuration
	LearningMinConfidence float64
	LearningMinFrequency  int

	// Provider registry for LLM compression
	ProviderRegistry *services.ProviderRegistry
}

// DefaultIntegrationConfig returns default configuration
func DefaultIntegrationConfig() *IntegrationConfig {
	return &IntegrationConfig{
		// Enable all components by default
		EnableInfiniteContext:   true,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,

		// Kafka defaults
		KafkaBootstrapServers: "localhost:9092",
		KafkaConsumerGroup:    "helixagent-bigdata",

		// ClickHouse defaults
		ClickHouseHost:     "localhost",
		ClickHousePort:     9000,
		ClickHouseDatabase: "helixagent_analytics",
		ClickHouseUser:     "default",
		ClickHousePassword: "",

		// Neo4j defaults
		Neo4jURI:      "bolt://localhost:7687",
		Neo4jUsername: "neo4j",
		Neo4jPassword: "helixagent123",
		Neo4jDatabase: "helixagent",

		// Context engine defaults
		ContextCacheSize:       100,
		ContextCacheTTL:        30 * time.Minute,
		ContextCompressionType: "hybrid",

		// Learning defaults
		LearningMinConfidence: 0.7,
		LearningMinFrequency:  3,
	}
}

// NewBigDataIntegration creates a new big data integration.
// Component initialization is deferred: each component is created on first
// access through its getter (GetInfiniteContext, GetDistributedMemory, etc.)
// using sync.Once to guarantee thread safety and at-most-once semantics.
// The Initialize method still works for eager startup when preferred.
func NewBigDataIntegration(
	config *IntegrationConfig,
	kafkaBroker messaging.MessageBroker,
	logger *logrus.Logger,
) (*BigDataIntegration, error) {
	if config == nil {
		config = DefaultIntegrationConfig()
	}

	integration := &BigDataIntegration{
		config:      config,
		kafkaBroker: kafkaBroker,
		logger:      logger,
		isRunning:   false,
	}

	// Wire up lazy initializers for each enabled component
	integration.lazyInfiniteContext = &lazyComponent[*conversation.InfiniteContextEngine]{
		factory: func(ctx context.Context) (*conversation.InfiniteContextEngine, error) {
			if !config.EnableInfiniteContext {
				return nil, fmt.Errorf("infinite context is disabled")
			}
			llmClient := &providerRegistryLLMClient{
				registry: config.ProviderRegistry,
				logger:   logger,
			}
			compressor := conversation.NewContextCompressor(llmClient, logger)
			engine := conversation.NewInfiniteContextEngine(kafkaBroker, compressor, logger)
			logger.Info("Infinite context engine initialized (lazy)")
			return engine, nil
		},
	}

	integration.lazyDistributedMem = &lazyComponent[*memory.DistributedMemoryManager]{
		factory: func(ctx context.Context) (*memory.DistributedMemoryManager, error) {
			if !config.EnableDistributedMemory {
				return nil, fmt.Errorf("distributed memory is disabled")
			}
			store := memory.NewInMemoryStore()
			localManager := memory.NewManager(store, nil, nil, nil, nil, logger)
			eventLog := newInMemoryEventLog()
			conflictResolver := memory.NewCRDTResolver("merge_all")
			nodeID := fmt.Sprintf("node-%d", time.Now().Unix())
			manager := memory.NewDistributedMemoryManager(
				localManager, nodeID, eventLog, conflictResolver, kafkaBroker, logger,
			)
			logger.Info("Distributed memory initialized (lazy)")
			return manager, nil
		},
	}

	integration.lazyGraphStreaming = &lazyComponent[*knowledge.StreamingKnowledgeGraph]{
		factory: func(ctx context.Context) (*knowledge.StreamingKnowledgeGraph, error) {
			if !config.EnableKnowledgeGraph {
				return nil, fmt.Errorf("knowledge graph is disabled")
			}
			graphConfig := knowledge.GraphStreamingConfig{
				Neo4jURI:      config.Neo4jURI,
				Neo4jUser:     config.Neo4jUsername,
				Neo4jPassword: config.Neo4jPassword,
				Neo4jDatabase: config.Neo4jDatabase,
				EntityTopic:   "helixagent.entities.updates",
				MemoryTopic:   "helixagent.memory.updates",
				DebateTopic:   "helixagent.debate.updates",
			}
			graph, err := knowledge.NewStreamingKnowledgeGraph(graphConfig, kafkaBroker, logger)
			if err != nil {
				return nil, err
			}
			if err := graph.StartStreaming(ctx); err != nil {
				return nil, err
			}
			logger.Info("Knowledge graph streaming initialized (lazy)")
			return graph, nil
		},
	}

	integration.lazyAnalytics = &lazyComponent[*analytics.ClickHouseAnalytics]{
		factory: func(ctx context.Context) (*analytics.ClickHouseAnalytics, error) {
			if !config.EnableAnalytics {
				return nil, fmt.Errorf("analytics is disabled")
			}
			chConfig := analytics.ClickHouseConfig{
				Host:     config.ClickHouseHost,
				Port:     config.ClickHousePort,
				Database: config.ClickHouseDatabase,
				Username: config.ClickHouseUser,
				Password: config.ClickHousePassword,
			}
			client, err := analytics.NewClickHouseAnalytics(chConfig, logger)
			if err != nil {
				return nil, err
			}
			logger.Info("ClickHouse analytics initialized (lazy)")
			return client, nil
		},
	}

	integration.lazyCrossLearner = &lazyComponent[*learning.CrossSessionLearner]{
		factory: func(ctx context.Context) (*learning.CrossSessionLearner, error) {
			if !config.EnableCrossLearning {
				return nil, fmt.Errorf("cross-session learning is disabled")
			}
			learnConfig := learning.CrossSessionConfig{
				CompletedTopic: "helixagent.conversations.completed",
				InsightsTopic:  "helixagent.learning.insights",
				MinConfidence:  config.LearningMinConfidence,
				MinFrequency:   config.LearningMinFrequency,
			}
			learner := learning.NewCrossSessionLearner(learnConfig, kafkaBroker, logger)
			if err := learner.StartLearning(ctx); err != nil {
				return nil, err
			}
			logger.Info("Cross-session learning initialized (lazy)")
			return learner, nil
		},
	}

	return integration, nil
}

// Initialize initializes all enabled big data components
func (bdi *BigDataIntegration) Initialize(ctx context.Context) error {
	bdi.logger.Info("Initializing big data integration...")

	// Initialize Infinite Context Engine
	if bdi.config.EnableInfiniteContext {
		if err := bdi.initializeInfiniteContext(ctx); err != nil {
			bdi.logger.WithError(err).Error("Failed to initialize infinite context engine")
			return fmt.Errorf("infinite context initialization failed: %w", err)
		}
		bdi.logger.Info("✓ Infinite context engine initialized")
	}

	// Initialize Distributed Memory
	if bdi.config.EnableDistributedMemory {
		if err := bdi.initializeDistributedMemory(ctx); err != nil {
			bdi.logger.WithError(err).Error("Failed to initialize distributed memory")
			return fmt.Errorf("distributed memory initialization failed: %w", err)
		}
		bdi.logger.Info("✓ Distributed memory initialized")
	}

	// Initialize Knowledge Graph Streaming
	if bdi.config.EnableKnowledgeGraph {
		if err := bdi.initializeKnowledgeGraph(ctx); err != nil {
			bdi.logger.WithError(err).Error("Failed to initialize knowledge graph streaming")
			return fmt.Errorf("knowledge graph initialization failed: %w", err)
		}
		bdi.logger.Info("✓ Knowledge graph streaming initialized")
	}

	// Initialize ClickHouse Analytics
	if bdi.config.EnableAnalytics {
		if err := bdi.initializeAnalytics(ctx); err != nil {
			bdi.logger.WithError(err).Error("Failed to initialize analytics")
			return fmt.Errorf("analytics initialization failed: %w", err)
		}
		bdi.logger.Info("✓ ClickHouse analytics initialized")
	}

	// Initialize Cross-Session Learning
	if bdi.config.EnableCrossLearning {
		if err := bdi.initializeCrossLearning(ctx); err != nil {
			bdi.logger.WithError(err).Error("Failed to initialize cross-session learning")
			return fmt.Errorf("cross-session learning initialization failed: %w", err)
		}
		bdi.logger.Info("✓ Cross-session learning initialized")
	}

	bdi.logger.Info("Big data integration initialized successfully")
	return nil
}

// initializeInfiniteContext initializes the infinite context engine
func (bdi *BigDataIntegration) initializeInfiniteContext(ctx context.Context) error {
	// Create LLM client for compression using provider registry
	llmClient := &providerRegistryLLMClient{
		registry: bdi.config.ProviderRegistry,
		logger:   bdi.logger,
	}
	compressor := conversation.NewContextCompressor(llmClient, bdi.logger)

	engine := conversation.NewInfiniteContextEngine(
		bdi.kafkaBroker,
		compressor,
		bdi.logger,
	)

	bdi.infiniteContext = engine
	return nil
}

// initializeDistributedMemory initializes the distributed memory manager
func (bdi *BigDataIntegration) initializeDistributedMemory(ctx context.Context) error {
	// Create local memory manager with dummy dependencies
	store := memory.NewInMemoryStore()
	localManager := memory.NewManager(store, nil, nil, nil, nil, bdi.logger)

	// Create in-memory event log
	eventLog := newInMemoryEventLog()

	// Create CRDT resolver
	conflictResolver := memory.NewCRDTResolver("merge_all")

	// Generate node ID
	nodeID := fmt.Sprintf("node-%d", time.Now().Unix())

	// Create distributed memory manager
	manager := memory.NewDistributedMemoryManager(
		localManager,
		nodeID,
		eventLog,
		conflictResolver,
		bdi.kafkaBroker,
		bdi.logger,
	)

	bdi.distributedMemory = manager
	return nil
}

// initializeKnowledgeGraph initializes the knowledge graph streaming
func (bdi *BigDataIntegration) initializeKnowledgeGraph(ctx context.Context) error {
	config := knowledge.GraphStreamingConfig{
		Neo4jURI:      bdi.config.Neo4jURI,
		Neo4jUser:     bdi.config.Neo4jUsername,
		Neo4jPassword: bdi.config.Neo4jPassword,
		Neo4jDatabase: bdi.config.Neo4jDatabase,
		EntityTopic:   "helixagent.entities.updates",
		MemoryTopic:   "helixagent.memory.updates",
		DebateTopic:   "helixagent.debate.updates",
	}

	graph, err := knowledge.NewStreamingKnowledgeGraph(
		config,
		bdi.kafkaBroker,
		bdi.logger,
	)
	if err != nil {
		return err
	}

	// Start streaming updates
	if err := graph.StartStreaming(ctx); err != nil {
		return err
	}

	bdi.graphStreaming = graph
	return nil
}

// initializeAnalytics initializes the ClickHouse analytics
func (bdi *BigDataIntegration) initializeAnalytics(ctx context.Context) error {
	config := analytics.ClickHouseConfig{
		Host:     bdi.config.ClickHouseHost,
		Port:     bdi.config.ClickHousePort,
		Database: bdi.config.ClickHouseDatabase,
		Username: bdi.config.ClickHouseUser,
		Password: bdi.config.ClickHousePassword,
	}

	client, err := analytics.NewClickHouseAnalytics(
		config,
		bdi.logger,
	)
	if err != nil {
		return err
	}

	// Test connection - health check not available in current version
	// Assume connection is okay

	bdi.clickhouseAnalytics = client
	return nil
}

// initializeCrossLearning initializes the cross-session learner
func (bdi *BigDataIntegration) initializeCrossLearning(ctx context.Context) error {
	config := learning.CrossSessionConfig{
		CompletedTopic: "helixagent.conversations.completed",
		InsightsTopic:  "helixagent.learning.insights",
		MinConfidence:  bdi.config.LearningMinConfidence,
		MinFrequency:   bdi.config.LearningMinFrequency,
	}

	learner := learning.NewCrossSessionLearner(
		config,
		bdi.kafkaBroker,
		bdi.logger,
	)

	// Start learning
	if err := learner.StartLearning(ctx); err != nil {
		return err
	}

	bdi.crossSessionLearner = learner
	return nil
}

// Start starts all big data components
func (bdi *BigDataIntegration) Start(ctx context.Context) error {
	if bdi.isRunning {
		return fmt.Errorf("big data integration is already running")
	}

	bdi.logger.Info("Starting big data integration...")

	// All components are already started during initialization
	// This method is provided for consistency

	bdi.isRunning = true
	bdi.logger.Info("Big data integration started successfully")
	return nil
}

// stopComponentTimeout is the maximum time to wait for each component to stop
const stopComponentTimeout = 10 * time.Second

// Stop stops all big data components with per-component timeouts.
// Handles both eagerly and lazily initialized components.
func (bdi *BigDataIntegration) Stop(ctx context.Context) error {
	if !bdi.isRunning {
		return nil
	}

	bdi.logger.Info("Stopping big data integration...")

	// Stop distributed memory (eager)
	if bdi.distributedMemory != nil {
		bdi.logger.Debug("Distributed memory disabled")
	}

	// Resolve knowledge graph from eager or lazy
	graphToStop := bdi.graphStreaming
	if graphToStop == nil && bdi.lazyGraphStreaming != nil && bdi.lazyGraphStreaming.initialized() {
		graphToStop = bdi.lazyGraphStreaming.value
	}
	if graphToStop != nil {
		stopCtx, cancel := context.WithTimeout(ctx, stopComponentTimeout)
		if err := graphToStop.Stop(stopCtx); err != nil {
			bdi.logger.WithError(err).Warn("Error stopping knowledge graph streaming")
		}
		cancel()
	}

	// Resolve ClickHouse from eager or lazy
	analyticsToClose := bdi.clickhouseAnalytics
	if analyticsToClose == nil && bdi.lazyAnalytics != nil && bdi.lazyAnalytics.initialized() {
		analyticsToClose = bdi.lazyAnalytics.value
	}
	if analyticsToClose != nil {
		done := make(chan error, 1)
		go func() {
			done <- analyticsToClose.Close()
		}()
		select {
		case err := <-done:
			if err != nil {
				bdi.logger.WithError(err).Warn("Error closing ClickHouse connection")
			}
		case <-time.After(stopComponentTimeout):
			bdi.logger.Warn("Timed out closing ClickHouse connection")
		}
	}

	bdi.isRunning = false
	bdi.logger.Info("Big data integration stopped")
	return nil
}

// GetInfiniteContext returns the infinite context engine.
// If not eagerly initialized, triggers lazy initialization on first call.
func (bdi *BigDataIntegration) GetInfiniteContext() *conversation.InfiniteContextEngine {
	if bdi.infiniteContext != nil {
		return bdi.infiniteContext
	}
	if bdi.lazyInfiniteContext != nil {
		engine, err := bdi.lazyInfiniteContext.get(context.Background())
		if err != nil {
			bdi.logger.WithError(err).Debug("Lazy infinite context init failed")
			return nil
		}
		return engine
	}
	return nil
}

// GetDistributedMemory returns the distributed memory manager.
// If not eagerly initialized, triggers lazy initialization on first call.
func (bdi *BigDataIntegration) GetDistributedMemory() *memory.DistributedMemoryManager {
	if bdi.distributedMemory != nil {
		return bdi.distributedMemory
	}
	if bdi.lazyDistributedMem != nil {
		mgr, err := bdi.lazyDistributedMem.get(context.Background())
		if err != nil {
			bdi.logger.WithError(err).Debug("Lazy distributed memory init failed")
			return nil
		}
		return mgr
	}
	return nil
}

// GetKnowledgeGraph returns the knowledge graph streaming.
// If not eagerly initialized, triggers lazy initialization on first call.
func (bdi *BigDataIntegration) GetKnowledgeGraph() *knowledge.StreamingKnowledgeGraph {
	if bdi.graphStreaming != nil {
		return bdi.graphStreaming
	}
	if bdi.lazyGraphStreaming != nil {
		graph, err := bdi.lazyGraphStreaming.get(context.Background())
		if err != nil {
			bdi.logger.WithError(err).Debug("Lazy knowledge graph init failed")
			return nil
		}
		return graph
	}
	return nil
}

// GetAnalytics returns the ClickHouse analytics client.
// If not eagerly initialized, triggers lazy initialization on first call.
func (bdi *BigDataIntegration) GetAnalytics() *analytics.ClickHouseAnalytics {
	if bdi.clickhouseAnalytics != nil {
		return bdi.clickhouseAnalytics
	}
	if bdi.lazyAnalytics != nil {
		client, err := bdi.lazyAnalytics.get(context.Background())
		if err != nil {
			bdi.logger.WithError(err).Debug("Lazy analytics init failed")
			return nil
		}
		return client
	}
	return nil
}

// GetCrossLearner returns the cross-session learner.
// If not eagerly initialized, triggers lazy initialization on first call.
func (bdi *BigDataIntegration) GetCrossLearner() *learning.CrossSessionLearner {
	if bdi.crossSessionLearner != nil {
		return bdi.crossSessionLearner
	}
	if bdi.lazyCrossLearner != nil {
		learner, err := bdi.lazyCrossLearner.get(context.Background())
		if err != nil {
			bdi.logger.WithError(err).Debug("Lazy cross-session learner init failed")
			return nil
		}
		return learner
	}
	return nil
}

// IsRunning returns whether the integration is running
func (bdi *BigDataIntegration) IsRunning() bool {
	return bdi.isRunning
}

// isNonNil returns true if val is a non-nil value. It correctly handles
// typed nil pointers by using reflect, avoiding the Go nil-interface trap
// where any((*T)(nil)) != nil evaluates to true.
func isNonNil[T any](val T) bool {
	v := reflect.ValueOf(&val).Elem()
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return !v.IsNil()
	default:
		return true
	}
}

// componentHealth returns a health status string for a component.
// It checks the enabled flag first, then eager instance, then lazy instance.
func componentHealth[T any](eager T, lazy *lazyComponent[T], enabled bool) string {
	if !enabled {
		return "disabled"
	}
	if isNonNil(eager) {
		return "healthy"
	}
	if lazy != nil && lazy.initialized() {
		if lazy.initErr != nil {
			return "init_failed"
		}
		return "healthy"
	}
	return "not_initialized"
}

// HealthCheck checks the health of all components
func (bdi *BigDataIntegration) HealthCheck(ctx context.Context) map[string]string {
	health := make(map[string]string)

	health["infinite_context"] = componentHealth(
		bdi.infiniteContext, bdi.lazyInfiniteContext, bdi.config.EnableInfiniteContext)
	health["distributed_memory"] = componentHealth(
		bdi.distributedMemory, bdi.lazyDistributedMem, bdi.config.EnableDistributedMemory)
	health["knowledge_graph"] = componentHealth(
		bdi.graphStreaming, bdi.lazyGraphStreaming, bdi.config.EnableKnowledgeGraph)
	health["analytics"] = componentHealth(
		bdi.clickhouseAnalytics, bdi.lazyAnalytics, bdi.config.EnableAnalytics)
	health["cross_learning"] = componentHealth(
		bdi.crossSessionLearner, bdi.lazyCrossLearner, bdi.config.EnableCrossLearning)

	return health
}
