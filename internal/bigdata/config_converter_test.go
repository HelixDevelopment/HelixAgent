package bigdata

import (
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ConfigToIntegrationConfig tests ---

func TestConfigToIntegrationConfig_NilInput(t *testing.T) {
	t.Parallel()
	result := ConfigToIntegrationConfig(nil)
	require.NotNil(t, result)
	// Should return DefaultIntegrationConfig values
	expected := DefaultIntegrationConfig()
	assert.Equal(t, expected.EnableInfiniteContext, result.EnableInfiniteContext)
	assert.Equal(t, expected.KafkaBootstrapServers, result.KafkaBootstrapServers)
	assert.Equal(t, expected.ClickHouseHost, result.ClickHouseHost)
	assert.Equal(t, expected.Neo4jURI, result.Neo4jURI)
	assert.Equal(t, expected.LearningMinConfidence, result.LearningMinConfidence)
}

func TestConfigToIntegrationConfig_AllFieldsMapped(t *testing.T) {
	t.Parallel()
	cfg := &config.BigDataConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    true,
		EnableAnalytics:         true,
		EnableCrossLearning:     false,
		KafkaBootstrapServers:   "broker1:9092,broker2:9092",
		KafkaConsumerGroup:      "test-group",
		ClickHouseHost:          "clickhouse.local",
		ClickHousePort:          9001,
		ClickHouseDatabase:      "testdb",
		ClickHouseUser:          "admin",
		ClickHousePassword:      "secret",
		Neo4jURI:                "bolt://neo4j.local:7687",
		Neo4jUsername:           "neo4j_user",
		Neo4jPassword:           "neo4j_pass",
		Neo4jDatabase:           "graphdb",
		ContextCacheSize:        200,
		ContextCacheTTL:         1 * time.Hour,
		ContextCompressionType:  "lz4",
		LearningMinConfidence:   0.85,
		LearningMinFrequency:    5,
	}

	result := ConfigToIntegrationConfig(cfg)
	require.NotNil(t, result)

	assert.True(t, result.EnableInfiniteContext)
	assert.True(t, result.EnableDistributedMemory)
	assert.True(t, result.EnableKnowledgeGraph)
	assert.True(t, result.EnableAnalytics)
	assert.False(t, result.EnableCrossLearning)

	assert.Equal(t, "broker1:9092,broker2:9092", result.KafkaBootstrapServers)
	assert.Equal(t, "test-group", result.KafkaConsumerGroup)

	assert.Equal(t, "clickhouse.local", result.ClickHouseHost)
	assert.Equal(t, 9001, result.ClickHousePort)
	assert.Equal(t, "testdb", result.ClickHouseDatabase)
	assert.Equal(t, "admin", result.ClickHouseUser)
	assert.Equal(t, "secret", result.ClickHousePassword)

	assert.Equal(t, "bolt://neo4j.local:7687", result.Neo4jURI)
	assert.Equal(t, "neo4j_user", result.Neo4jUsername)
	assert.Equal(t, "neo4j_pass", result.Neo4jPassword)
	assert.Equal(t, "graphdb", result.Neo4jDatabase)

	assert.Equal(t, 200, result.ContextCacheSize)
	assert.Equal(t, 1*time.Hour, result.ContextCacheTTL)
	assert.Equal(t, "lz4", result.ContextCompressionType)

	assert.Equal(t, 0.85, result.LearningMinConfidence)
	assert.Equal(t, 5, result.LearningMinFrequency)
}

func TestConfigToIntegrationConfig_ZeroValues(t *testing.T) {
	t.Parallel()
	cfg := &config.BigDataConfig{}
	result := ConfigToIntegrationConfig(cfg)
	require.NotNil(t, result)

	assert.False(t, result.EnableInfiniteContext)
	assert.False(t, result.EnableDistributedMemory)
	assert.Empty(t, result.KafkaBootstrapServers)
	assert.Equal(t, 0, result.ClickHousePort)
	assert.Equal(t, 0.0, result.LearningMinConfidence)
}

// --- IntegrationConfigToConfig tests ---

func TestIntegrationConfigToConfig_NilInput(t *testing.T) {
	t.Parallel()
	result := IntegrationConfigToConfig(nil)
	require.NotNil(t, result)
	// Should return an empty BigDataConfig
	assert.False(t, result.EnableInfiniteContext)
	assert.Empty(t, result.KafkaBootstrapServers)
}

func TestIntegrationConfigToConfig_AllFieldsMapped(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    true,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,
		KafkaBootstrapServers:   "kafka:9092",
		KafkaConsumerGroup:      "my-group",
		ClickHouseHost:          "ch-host",
		ClickHousePort:          8123,
		ClickHouseDatabase:      "mydb",
		ClickHouseUser:          "user1",
		ClickHousePassword:      "pass1",
		Neo4jURI:                "bolt://neo:7687",
		Neo4jUsername:           "u",
		Neo4jPassword:           "p",
		Neo4jDatabase:           "db",
		ContextCacheSize:        50,
		ContextCacheTTL:         15 * time.Minute,
		ContextCompressionType:  "zstd",
		LearningMinConfidence:   0.9,
		LearningMinFrequency:    10,
	}

	result := IntegrationConfigToConfig(icfg)
	require.NotNil(t, result)

	assert.True(t, result.EnableInfiniteContext)
	assert.False(t, result.EnableDistributedMemory)
	assert.True(t, result.EnableKnowledgeGraph)
	assert.False(t, result.EnableAnalytics)
	assert.True(t, result.EnableCrossLearning)

	assert.Equal(t, "kafka:9092", result.KafkaBootstrapServers)
	assert.Equal(t, "my-group", result.KafkaConsumerGroup)
	assert.Equal(t, "ch-host", result.ClickHouseHost)
	assert.Equal(t, 8123, result.ClickHousePort)
	assert.Equal(t, "mydb", result.ClickHouseDatabase)
	assert.Equal(t, "user1", result.ClickHouseUser)
	assert.Equal(t, "pass1", result.ClickHousePassword)
	assert.Equal(t, "bolt://neo:7687", result.Neo4jURI)
	assert.Equal(t, "u", result.Neo4jUsername)
	assert.Equal(t, "p", result.Neo4jPassword)
	assert.Equal(t, "db", result.Neo4jDatabase)
	assert.Equal(t, 50, result.ContextCacheSize)
	assert.Equal(t, 15*time.Minute, result.ContextCacheTTL)
	assert.Equal(t, "zstd", result.ContextCompressionType)
	assert.Equal(t, 0.9, result.LearningMinConfidence)
	assert.Equal(t, 10, result.LearningMinFrequency)
}

// --- Round-trip conversion tests ---

func TestConfigConverter_RoundTrip_BigDataToIntegrationAndBack(t *testing.T) {
	t.Parallel()
	original := &config.BigDataConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         true,
		EnableCrossLearning:     false,
		KafkaBootstrapServers:   "broker:9092",
		KafkaConsumerGroup:      "group",
		ClickHouseHost:          "ch",
		ClickHousePort:          9000,
		ClickHouseDatabase:      "db",
		ClickHouseUser:          "user",
		ClickHousePassword:      "pass",
		Neo4jURI:                "bolt://n:7687",
		Neo4jUsername:           "neo",
		Neo4jPassword:           "neopass",
		Neo4jDatabase:           "neodb",
		ContextCacheSize:        100,
		ContextCacheTTL:         30 * time.Minute,
		ContextCompressionType:  "hybrid",
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}

	intermediate := ConfigToIntegrationConfig(original)
	result := IntegrationConfigToConfig(intermediate)

	assert.Equal(t, original.EnableInfiniteContext, result.EnableInfiniteContext)
	assert.Equal(t, original.EnableDistributedMemory, result.EnableDistributedMemory)
	assert.Equal(t, original.EnableKnowledgeGraph, result.EnableKnowledgeGraph)
	assert.Equal(t, original.EnableAnalytics, result.EnableAnalytics)
	assert.Equal(t, original.EnableCrossLearning, result.EnableCrossLearning)
	assert.Equal(t, original.KafkaBootstrapServers, result.KafkaBootstrapServers)
	assert.Equal(t, original.KafkaConsumerGroup, result.KafkaConsumerGroup)
	assert.Equal(t, original.ClickHouseHost, result.ClickHouseHost)
	assert.Equal(t, original.ClickHousePort, result.ClickHousePort)
	assert.Equal(t, original.ClickHouseDatabase, result.ClickHouseDatabase)
	assert.Equal(t, original.ClickHouseUser, result.ClickHouseUser)
	assert.Equal(t, original.ClickHousePassword, result.ClickHousePassword)
	assert.Equal(t, original.Neo4jURI, result.Neo4jURI)
	assert.Equal(t, original.Neo4jUsername, result.Neo4jUsername)
	assert.Equal(t, original.Neo4jPassword, result.Neo4jPassword)
	assert.Equal(t, original.Neo4jDatabase, result.Neo4jDatabase)
	assert.Equal(t, original.ContextCacheSize, result.ContextCacheSize)
	assert.Equal(t, original.ContextCacheTTL, result.ContextCacheTTL)
	assert.Equal(t, original.ContextCompressionType, result.ContextCompressionType)
	assert.Equal(t, original.LearningMinConfidence, result.LearningMinConfidence)
	assert.Equal(t, original.LearningMinFrequency, result.LearningMinFrequency)
}

func TestConfigConverter_RoundTrip_IntegrationToBigDataAndBack(t *testing.T) {
	t.Parallel()
	original := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    true,
		EnableAnalytics:         false,
		EnableCrossLearning:     true,
		KafkaBootstrapServers:   "k1:9092,k2:9092",
		KafkaConsumerGroup:      "cg",
		ClickHouseHost:          "ch-host",
		ClickHousePort:          8123,
		ClickHouseDatabase:      "analytics",
		ClickHouseUser:          "root",
		ClickHousePassword:      "rootpass",
		Neo4jURI:                "bolt://graph:7687",
		Neo4jUsername:           "admin",
		Neo4jPassword:           "admin123",
		Neo4jDatabase:           "graph",
		ContextCacheSize:        500,
		ContextCacheTTL:         2 * time.Hour,
		ContextCompressionType:  "gzip",
		LearningMinConfidence:   0.5,
		LearningMinFrequency:    1,
	}

	intermediate := IntegrationConfigToConfig(original)
	result := ConfigToIntegrationConfig(intermediate)

	assert.Equal(t, original.EnableInfiniteContext, result.EnableInfiniteContext)
	assert.Equal(t, original.EnableDistributedMemory, result.EnableDistributedMemory)
	assert.Equal(t, original.EnableKnowledgeGraph, result.EnableKnowledgeGraph)
	assert.Equal(t, original.EnableAnalytics, result.EnableAnalytics)
	assert.Equal(t, original.EnableCrossLearning, result.EnableCrossLearning)
	assert.Equal(t, original.KafkaBootstrapServers, result.KafkaBootstrapServers)
	assert.Equal(t, original.KafkaConsumerGroup, result.KafkaConsumerGroup)
	assert.Equal(t, original.ClickHouseHost, result.ClickHouseHost)
	assert.Equal(t, original.ClickHousePort, result.ClickHousePort)
	assert.Equal(t, original.ClickHouseDatabase, result.ClickHouseDatabase)
	assert.Equal(t, original.ClickHouseUser, result.ClickHouseUser)
	assert.Equal(t, original.ClickHousePassword, result.ClickHousePassword)
	assert.Equal(t, original.Neo4jURI, result.Neo4jURI)
	assert.Equal(t, original.Neo4jUsername, result.Neo4jUsername)
	assert.Equal(t, original.Neo4jPassword, result.Neo4jPassword)
	assert.Equal(t, original.Neo4jDatabase, result.Neo4jDatabase)
	assert.Equal(t, original.ContextCacheSize, result.ContextCacheSize)
	assert.Equal(t, original.ContextCacheTTL, result.ContextCacheTTL)
	assert.Equal(t, original.ContextCompressionType, result.ContextCompressionType)
	assert.Equal(t, original.LearningMinConfidence, result.LearningMinConfidence)
	assert.Equal(t, original.LearningMinFrequency, result.LearningMinFrequency)
}

// --- DefaultBigDataConfig tests ---

func TestDefaultBigDataConfig_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	result := DefaultBigDataConfig()
	require.NotNil(t, result)
}

func TestDefaultBigDataConfig_MatchesDefaultIntegrationConfig(t *testing.T) {
	t.Parallel()
	defaultICfg := DefaultIntegrationConfig()
	result := DefaultBigDataConfig()

	assert.Equal(t, defaultICfg.EnableInfiniteContext, result.EnableInfiniteContext)
	assert.Equal(t, defaultICfg.EnableDistributedMemory, result.EnableDistributedMemory)
	assert.Equal(t, defaultICfg.EnableKnowledgeGraph, result.EnableKnowledgeGraph)
	assert.Equal(t, defaultICfg.EnableAnalytics, result.EnableAnalytics)
	assert.Equal(t, defaultICfg.EnableCrossLearning, result.EnableCrossLearning)
	assert.Equal(t, defaultICfg.KafkaBootstrapServers, result.KafkaBootstrapServers)
	assert.Equal(t, defaultICfg.KafkaConsumerGroup, result.KafkaConsumerGroup)
	assert.Equal(t, defaultICfg.ClickHouseHost, result.ClickHouseHost)
	assert.Equal(t, defaultICfg.ClickHousePort, result.ClickHousePort)
	assert.Equal(t, defaultICfg.ClickHouseDatabase, result.ClickHouseDatabase)
	assert.Equal(t, defaultICfg.Neo4jURI, result.Neo4jURI)
	assert.Equal(t, defaultICfg.LearningMinConfidence, result.LearningMinConfidence)
	assert.Equal(t, defaultICfg.LearningMinFrequency, result.LearningMinFrequency)
}

// --- ValidateConfig tests ---

func TestValidateConfig_NilConfig(t *testing.T) {
	t.Parallel()
	err := ValidateConfig(nil)
	assert.NoError(t, err)
}

func TestValidateConfig_ValidFullConfig(t *testing.T) {
	t.Parallel()
	icfg := DefaultIntegrationConfig()
	// Enable all and provide required fields
	icfg.EnableCrossLearning = true
	icfg.EnableInfiniteContext = true
	icfg.EnableAnalytics = true
	icfg.EnableKnowledgeGraph = true
	icfg.KafkaBootstrapServers = "kafka:9092"
	icfg.ClickHouseHost = "ch:9000"
	icfg.ClickHousePort = 9000
	icfg.Neo4jURI = "bolt://neo:7687"

	err := ValidateConfig(icfg)
	assert.NoError(t, err)
}

func TestValidateConfig_AllDisabled(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
		ContextCacheSize:        0,
		ContextCacheTTL:         0,
		LearningMinConfidence:   0.5,
		LearningMinFrequency:    0,
	}
	err := ValidateConfig(icfg)
	assert.NoError(t, err)
}

func TestValidateConfig_KafkaRequiredForCrossLearning(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableCrossLearning:   true,
		KafkaBootstrapServers: "",
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Kafka bootstrap servers required")
}

func TestValidateConfig_KafkaRequiredForInfiniteContext(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableInfiniteContext: true,
		KafkaBootstrapServers: "",
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Kafka bootstrap servers required")
}

func TestValidateConfig_KafkaProvidedForCrossLearning(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableCrossLearning:   true,
		KafkaBootstrapServers: "kafka:9092",
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.NoError(t, err)
}

func TestValidateConfig_ClickHouseHostRequiredForAnalytics(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableAnalytics:       true,
		ClickHouseHost:        "",
		ClickHousePort:        9000,
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ClickHouse host required")
}

func TestValidateConfig_ClickHousePortMustBePositive(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableAnalytics:       true,
		ClickHouseHost:        "ch-host",
		ClickHousePort:        0,
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ClickHouse port must be positive")
}

func TestValidateConfig_ClickHousePortNegative(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableAnalytics:       true,
		ClickHouseHost:        "ch-host",
		ClickHousePort:        -1,
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ClickHouse port must be positive")
}

func TestValidateConfig_Neo4jURIRequiredForKnowledgeGraph(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableKnowledgeGraph:  true,
		Neo4jURI:              "",
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Neo4j URI required")
}

func TestValidateConfig_Neo4jURIProvidedForKnowledgeGraph(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		EnableKnowledgeGraph:  true,
		Neo4jURI:              "bolt://localhost:7687",
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.NoError(t, err)
}

func TestValidateConfig_NegativeContextCacheSize(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		ContextCacheSize:      -1,
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cache size must be non-negative")
}

func TestValidateConfig_NegativeContextCacheTTL(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		ContextCacheTTL:       -1 * time.Second,
		LearningMinConfidence: 0.5,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cache TTL must be non-negative")
}

func TestValidateConfig_LearningConfidenceTooLow(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		LearningMinConfidence: -0.1,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "learning min confidence must be between 0 and 1")
}

func TestValidateConfig_LearningConfidenceTooHigh(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		LearningMinConfidence: 1.1,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "learning min confidence must be between 0 and 1")
}

func TestValidateConfig_LearningConfidenceBoundaryZero(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		LearningMinConfidence: 0.0,
	}
	err := ValidateConfig(icfg)
	assert.NoError(t, err)
}

func TestValidateConfig_LearningConfidenceBoundaryOne(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		LearningMinConfidence: 1.0,
	}
	err := ValidateConfig(icfg)
	assert.NoError(t, err)
}

func TestValidateConfig_NegativeLearningMinFrequency(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		LearningMinConfidence: 0.5,
		LearningMinFrequency:  -1,
	}
	err := ValidateConfig(icfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "learning min frequency must be non-negative")
}

func TestValidateConfig_ZeroLearningMinFrequency(t *testing.T) {
	t.Parallel()
	icfg := &IntegrationConfig{
		LearningMinConfidence: 0.5,
		LearningMinFrequency:  0,
	}
	err := ValidateConfig(icfg)
	assert.NoError(t, err)
}

// --- Benchmarks ---

func BenchmarkConfigToIntegrationConfig(b *testing.B) {
	cfg := &config.BigDataConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    true,
		EnableAnalytics:         true,
		EnableCrossLearning:     true,
		KafkaBootstrapServers:   "localhost:9092",
		KafkaConsumerGroup:      "helixagent-group",
		ClickHouseHost:          "localhost",
		ClickHousePort:          9000,
		ClickHouseDatabase:      "helixagent",
		ClickHouseUser:          "default",
		ClickHousePassword:      "password",
		Neo4jURI:                "bolt://localhost:7687",
		Neo4jUsername:           "neo4j",
		Neo4jPassword:           "password",
		Neo4jDatabase:           "neo4j",
		ContextCacheSize:        1000,
		ContextCacheTTL:         5 * time.Minute,
		ContextCompressionType:  "lz4",
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ConfigToIntegrationConfig(cfg)
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	icfg := &IntegrationConfig{
		LearningMinConfidence: 0.7,
		LearningMinFrequency:  3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateConfig(icfg)
	}
}

func BenchmarkIntegrationConfigToConfig(b *testing.B) {
	icfg := &IntegrationConfig{
		EnableInfiniteContext:   true,
		EnableDistributedMemory: true,
		EnableKnowledgeGraph:    true,
		EnableAnalytics:         true,
		EnableCrossLearning:     true,
		KafkaBootstrapServers:   "localhost:9092",
		KafkaConsumerGroup:      "helixagent-group",
		ClickHouseHost:          "localhost",
		ClickHousePort:          9000,
		ClickHouseDatabase:      "helixagent",
		ClickHouseUser:          "default",
		ClickHousePassword:      "password",
		Neo4jURI:                "bolt://localhost:7687",
		Neo4jUsername:           "neo4j",
		Neo4jPassword:           "password",
		Neo4jDatabase:           "neo4j",
		ContextCacheSize:        1000,
		ContextCacheTTL:         5 * time.Minute,
		ContextCompressionType:  "lz4",
		LearningMinConfidence:   0.7,
		LearningMinFrequency:    3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IntegrationConfigToConfig(icfg)
	}
}
