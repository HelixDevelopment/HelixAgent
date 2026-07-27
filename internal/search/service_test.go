package search

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/search/types"
)

func TestDefaultServiceConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultServiceConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "local", cfg.EmbedderType)
	assert.Equal(t, "text-embedding-3-small", cfg.OpenAIModel)
	assert.Equal(t, "chroma", cfg.VectorStoreType)
	assert.Equal(t, "localhost", cfg.ChromaHost)
	assert.Equal(t, 8000, cfg.ChromaPort)
	assert.Equal(t, "localhost", cfg.QdrantHost)
	assert.Equal(t, 6333, cfg.QdrantPort)
	assert.Equal(t, "code_embeddings", cfg.CollectionName)
	assert.Equal(t, "docker-compose.yml", cfg.ComposeFile)
	assert.NotEmpty(t, cfg.IndexerConfig.IncludePatterns)
	assert.NotEmpty(t, cfg.IndexerConfig.ExcludePatterns)
}

func TestNewService_Disabled(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := ServiceConfig{
		Enabled: false,
	}

	svc, err := NewService(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.Nil(t, svc.Indexer)
	assert.Nil(t, svc.Searcher)
}

func TestNewService_UnknownEmbedderType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := ServiceConfig{
		Enabled:         true,
		EmbedderType:    "unknown_embedder",
		VectorStoreType: "chroma",
		ChromaHost:      "localhost",
		ChromaPort:      8000,
		CollectionName:  "test",
	}

	svc, err := NewService(cfg, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown embedder type")
	assert.Nil(t, svc)
}

func TestNewService_OpenAI_NoKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := ServiceConfig{
		Enabled:         true,
		EmbedderType:    "openai",
		OpenAIKey:       "",
		VectorStoreType: "chroma",
		ChromaHost:      "localhost",
		ChromaPort:      8000,
		CollectionName:  "test",
	}

	svc, err := NewService(cfg, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAI API key required")
	assert.Nil(t, svc)
}

func TestNewService_UnknownVectorStoreType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := ServiceConfig{
		Enabled:         true,
		EmbedderType:    "local",
		VectorStoreType: "unknown_store",
		CollectionName:  "test",
	}

	svc, err := NewService(cfg, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown vector store type")
	assert.Nil(t, svc)
}

func TestService_Health_Disabled(t *testing.T) {
	t.Parallel()

	svc := &Service{
		config: ServiceConfig{Enabled: false},
	}

	health := svc.Health()
	assert.Equal(t, "disabled", health["status"])
	assert.Equal(t, false, health["enabled"])
}

func TestService_Health_Enabled(t *testing.T) {
	t.Parallel()

	svc := &Service{
		config: ServiceConfig{
			Enabled:         true,
			EmbedderType:    "local",
			VectorStoreType: "chroma",
			CollectionName:  "test_col",
		},
	}

	health := svc.Health()
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, true, health["enabled"])
	assert.Equal(t, "local", health["embedder"])
	assert.Equal(t, "chroma", health["vector_store"])
	assert.Equal(t, "test_col", health["collection"])
}

func TestService_Search_Disabled(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := &Service{
		config: ServiceConfig{Enabled: false},
		logger: logger,
	}

	results, err := svc.Search(context.Background(), "query", types.SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search service disabled")
	assert.Nil(t, results)
}

func TestService_Index_Disabled(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := &Service{
		config: ServiceConfig{Enabled: false},
		logger: logger,
	}

	result, err := svc.Index(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "search service disabled")
	assert.Nil(t, result)
}

func TestService_Initialize_Disabled(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := &Service{
		config: ServiceConfig{Enabled: false},
		logger: logger,
	}

	err := svc.Initialize(context.Background())
	assert.NoError(t, err)
}

func TestService_Search_Enabled(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	expected := []types.SearchResult{
		{Document: types.Document{ID: "r1"}, Score: 0.9},
	}

	emb := &stubEmbedder{}
	store := &stubVectorStore{searchResults: expected}
	searcher := NewCodeSearcher(emb, store, "col")

	svc := &Service{
		config:   ServiceConfig{Enabled: true},
		logger:   logger,
		Searcher: searcher,
	}

	results, err := svc.Search(context.Background(), "query", types.SearchOptions{})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "r1", results[0].ID)
}

func TestEnsureVectorStoreContainer_NilAdapter(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := ServiceConfig{
		VectorStoreType:  "chroma",
		ContainerAdapter: nil,
	}

	err := ensureVectorStoreContainer(context.Background(), cfg, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container adapter not available")
}

func TestEnsureVectorStoreContainer_UnknownType(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := ServiceConfig{
		VectorStoreType:  "unknown",
		ContainerAdapter: nil,
	}

	err := ensureVectorStoreContainer(context.Background(), cfg, logger)
	assert.Error(t, err)
	// Reconciled: this asserted "container adapter not available".
	//
	// That was never this test's stated subject -- it is named _UnknownType and
	// passes VectorStoreType "unknown" -- but the old ordering checked the
	// adapter before the type, so an unknown type with a nil adapter reported
	// the adapter error and the type switch was unreachable. The old assertion
	// therefore described a code path this test was not written to exercise
	// (its own comment conceded it was testing "the nil path and the unknown
	// type path" at once, which the ordering made impossible).
	//
	// The type is now resolved first, because the reachability short-circuit
	// needs the host/port that the type selects. An unknown vector store type
	// is a configuration error no container adapter can resolve, so reporting
	// it as such is the accurate diagnostic. TestEnsureVectorStoreContainer_
	// NilAdapter still covers the adapter-nil path with a known type.
	assert.Contains(t, err.Error(), "unknown vector store type")
}

// TestEnsureVectorStoreContainer_ReachableStoreSkipsOrchestration covers the
// short-circuit added alongside the reordering: when the vector store already
// answers at its configured endpoint, the function must return without
// touching container orchestration.
//
// ContainerAdapter is deliberately nil. Under the old code that guaranteed an
// error; here it is the assertion mechanism -- reaching the orchestration path
// at all would surface "container adapter not available", so a nil error
// proves the reachability check returned first. A real listener is used rather
// than a stub, because the behaviour under test is an actual TCP dial.
func TestEnsureVectorStoreContainer_ReachableStoreSkipsOrchestration(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("open probe listener: %v", err)
	}
	defer ln.Close()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split listener address %q: %v", ln.Addr(), err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse listener port %q: %v", portStr, err)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	cfg := ServiceConfig{
		VectorStoreType:  "chroma",
		ChromaHost:       host,
		ChromaPort:       port,
		ContainerAdapter: nil,
	}

	if err := ensureVectorStoreContainer(context.Background(), cfg, logger); err != nil {
		t.Fatalf("reachable store must skip orchestration and return nil, got: %v", err)
	}
}

func TestServiceConfig_Defaults_IndexerConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultServiceConfig()

	// Verify indexer config defaults
	assert.Equal(t, ".", cfg.IndexerConfig.RootPath)
	assert.Equal(t, 50, cfg.IndexerConfig.ChunkSize)
	assert.Equal(t, 10, cfg.IndexerConfig.ChunkOverlap)
	assert.Equal(t, int64(1024*1024), cfg.IndexerConfig.MaxFileSize)
	assert.True(t, cfg.IndexerConfig.IndexOnStartup)
	assert.False(t, cfg.IndexerConfig.WatchFiles)
	assert.Equal(t, "code_embeddings", cfg.IndexerConfig.CollectionName)
}
