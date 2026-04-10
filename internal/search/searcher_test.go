package search

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/search/types"
)

// --- mock helpers for CodeSearcher tests ---

type stubEmbedder struct {
	embedResult [][]float32
	embedErr    error
	queryResult []float32
	queryErr    error
	dims        int
}

func (s *stubEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if s.embedErr != nil {
		return nil, s.embedErr
	}
	if s.embedResult != nil {
		return s.embedResult, nil
	}
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = []float32{0.1, 0.2, 0.3}
	}
	return res, nil
}

func (s *stubEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	if s.queryResult != nil {
		return s.queryResult, nil
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (s *stubEmbedder) Dimensions() int {
	if s.dims > 0 {
		return s.dims
	}
	return 3
}

type stubVectorStore struct {
	searchResults []types.SearchResult
	searchErr     error
	createErr     error
	deleteErr     error
	upsertErr     error
	delDocsErr    error
	textSearchErr error
	statsResult   *types.CollectionStats
	statsErr      error
}

func (s *stubVectorStore) CreateCollection(_ context.Context, _ string, _ int) error {
	return s.createErr
}

func (s *stubVectorStore) DeleteCollection(_ context.Context, _ string) error {
	return s.deleteErr
}

func (s *stubVectorStore) Upsert(_ context.Context, _ string, _ []types.Document) error {
	return s.upsertErr
}

func (s *stubVectorStore) Delete(_ context.Context, _ string, _ []string) error {
	return s.delDocsErr
}

func (s *stubVectorStore) Search(_ context.Context, _ string, _ []float32, _ types.SearchOptions) ([]types.SearchResult, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResults, nil
}

func (s *stubVectorStore) SearchByText(_ context.Context, _ string, _ string, _ types.SearchOptions) ([]types.SearchResult, error) {
	return nil, s.textSearchErr
}

func (s *stubVectorStore) GetCollectionStats(_ context.Context, _ string) (*types.CollectionStats, error) {
	if s.statsErr != nil {
		return nil, s.statsErr
	}
	return s.statsResult, nil
}

// --- Tests ---

func TestNewCodeSearcher(t *testing.T) {
	t.Parallel()

	emb := &stubEmbedder{}
	store := &stubVectorStore{}
	cs := NewCodeSearcher(emb, store, "test_collection")

	require.NotNil(t, cs)
	assert.Equal(t, "test_collection", cs.collection)
}

func TestCodeSearcher_Search_Success(t *testing.T) {
	t.Parallel()

	expected := []types.SearchResult{
		{
			Document: types.Document{ID: "doc-1", Content: "hello world"},
			Score:    0.95,
		},
	}

	emb := &stubEmbedder{queryResult: []float32{0.5, 0.6, 0.7}}
	store := &stubVectorStore{searchResults: expected}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.Search(context.Background(), "hello", types.SearchOptions{TopK: 5})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "doc-1", results[0].ID)
	assert.Equal(t, float32(0.95), results[0].Score)
}

func TestCodeSearcher_Search_EmbedError(t *testing.T) {
	t.Parallel()

	emb := &stubEmbedder{queryErr: errors.New("embed failed")}
	store := &stubVectorStore{}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.Search(context.Background(), "query", types.SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to embed query")
	assert.Nil(t, results)
}

func TestCodeSearcher_Search_StoreError(t *testing.T) {
	t.Parallel()

	emb := &stubEmbedder{}
	store := &stubVectorStore{searchErr: errors.New("store error")}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.Search(context.Background(), "query", types.SearchOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to search")
	assert.Nil(t, results)
}

func TestCodeSearcher_Search_EmptyResults(t *testing.T) {
	t.Parallel()

	emb := &stubEmbedder{}
	store := &stubVectorStore{searchResults: nil}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.Search(context.Background(), "nope", types.SearchOptions{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCodeSearcher_SearchSimilar(t *testing.T) {
	t.Parallel()

	expected := []types.SearchResult{
		{Document: types.Document{ID: "similar-1"}, Score: 0.8},
	}

	emb := &stubEmbedder{}
	store := &stubVectorStore{searchResults: expected}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.SearchSimilar(context.Background(), "main.go", 42, types.SearchOptions{TopK: 3})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "similar-1", results[0].ID)
}

func TestCodeSearcher_SearchSimilar_Error(t *testing.T) {
	t.Parallel()

	emb := &stubEmbedder{queryErr: errors.New("fail")}
	store := &stubVectorStore{}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.SearchSimilar(context.Background(), "file.go", 10, types.SearchOptions{})
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestCodeSearcher_SearchWithContext_NoFiles(t *testing.T) {
	t.Parallel()

	expected := []types.SearchResult{
		{Document: types.Document{ID: "r1"}, Score: 0.9},
	}

	emb := &stubEmbedder{}
	store := &stubVectorStore{searchResults: expected}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.SearchWithContext(context.Background(), "query", nil, types.SearchOptions{})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestCodeSearcher_SearchWithContext_WithFiles(t *testing.T) {
	t.Parallel()

	emb := &stubEmbedder{}
	store := &stubVectorStore{searchResults: []types.SearchResult{
		{Document: types.Document{ID: "ctx-1"}, Score: 0.85},
	}}
	cs := NewCodeSearcher(emb, store, "col")

	results, err := cs.SearchWithContext(context.Background(), "query", []string{"a.go", "b.go"}, types.SearchOptions{})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestCodeSearcher_enhanceQuery(t *testing.T) {
	t.Parallel()

	cs := &CodeSearcher{}

	tests := []struct {
		name         string
		query        string
		contextFiles []string
		expected     string
	}{
		{
			name:         "no context files",
			query:        "find error handling",
			contextFiles: nil,
			expected:     "find error handling",
		},
		{
			name:         "empty context files",
			query:        "find error handling",
			contextFiles: []string{},
			expected:     "find error handling",
		},
		{
			name:         "one context file",
			query:        "find handler",
			contextFiles: []string{"main.go"},
			expected:     "find handler (context: main.go)",
		},
		{
			name:         "multiple context files",
			query:        "search",
			contextFiles: []string{"a.go", "b.go", "c.go"},
			expected:     "search (context: a.go, b.go, c.go)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cs.enhanceQuery(tc.query, tc.contextFiles)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCodeSearcher_RerankResults(t *testing.T) {
	t.Parallel()

	cs := &CodeSearcher{}

	results := []types.SearchResult{
		{Document: types.Document{ID: "1"}, Score: 0.9},
		{Document: types.Document{ID: "2"}, Score: 0.8},
		{Document: types.Document{ID: "3"}, Score: 0.7},
	}

	reranked := cs.RerankResults(results, "query")
	// Currently returns as-is
	assert.Equal(t, results, reranked)
}

func TestCodeSearcher_RerankResults_Empty(t *testing.T) {
	t.Parallel()

	cs := &CodeSearcher{}
	reranked := cs.RerankResults(nil, "query")
	assert.Nil(t, reranked)
}

func TestCodeSearcher_ImplementsSearcher(t *testing.T) {
	t.Parallel()

	// Compile-time check is already in searcher.go, but test it here too
	var _ Searcher = (*CodeSearcher)(nil)
}

func TestCodeSearcher_Search_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	emb := &stubEmbedder{queryErr: context.Canceled}
	store := &stubVectorStore{}
	cs := NewCodeSearcher(emb, store, "col")

	_, err := cs.Search(ctx, "query", types.SearchOptions{})
	assert.Error(t, err)
}
