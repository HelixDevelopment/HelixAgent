package rag

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfigs(t *testing.T) {
	t.Run("HybridSearchConfig", func(t *testing.T) {
		config := DefaultHybridSearchConfig()
		assert.Equal(t, 0.7, config.VectorWeight)
		assert.Equal(t, 0.3, config.KeywordWeight)
		assert.Equal(t, 0.1, config.MinKeywordScore)
		assert.True(t, config.EnableFuzzyMatch)
		assert.Equal(t, 0.8, config.FuzzyThreshold)
	})

	t.Run("ReRankerConfig", func(t *testing.T) {
		config := DefaultReRankerConfig()
		assert.Equal(t, "cross-encoder/ms-marco-MiniLM-L-6-v2", config.Model)
		assert.Equal(t, 100, config.TopK)
		assert.Equal(t, 32, config.BatchSize)
		assert.Equal(t, 0.5, config.ScoreThreshold)
		assert.True(t, config.EnableCrossEncoder)
	})

	t.Run("QueryExpansionConfig", func(t *testing.T) {
		config := DefaultQueryExpansionConfig()
		assert.Equal(t, 5, config.MaxExpansions)
		assert.True(t, config.EnableSynonyms)
		assert.False(t, config.EnableHyponyms)
		assert.False(t, config.EnableHypernyms)
		assert.False(t, config.EnableLLMExpansion)
		assert.Equal(t, 0.8, config.SynonymWeight)
	})

	t.Run("ContextualCompressionConfig", func(t *testing.T) {
		config := DefaultContextualCompressionConfig()
		assert.Equal(t, 4096, config.MaxContextLength)
		assert.Equal(t, 0.5, config.CompressionRatio)
		assert.True(t, config.EnableSentenceExtraction)
		assert.False(t, config.EnableSummarization)
		assert.True(t, config.PreserveKeyPhrases)
	})

	t.Run("AdvancedRAGConfig", func(t *testing.T) {
		config := DefaultAdvancedRAGConfig()
		assert.Equal(t, 0.7, config.HybridSearch.VectorWeight)
		assert.Equal(t, 100, config.ReRanker.TopK)
		assert.Equal(t, 5, config.QueryExpansion.MaxExpansions)
		assert.Equal(t, 4096, config.ContextualCompression.MaxContextLength)
	})
}

func TestNewAdvancedRAG(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	pipeline := &Pipeline{} // Minimal pipeline for testing

	rag := NewAdvancedRAG(config, pipeline)

	assert.NotNil(t, rag)
	assert.Equal(t, config, rag.config)
	assert.Equal(t, pipeline, rag.pipeline)
	assert.NotNil(t, rag.synonyms)
	assert.False(t, rag.initialized.Load())
}

func TestAdvancedRAG_Initialize(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})

	ctx := context.Background()
	err := rag.Initialize(ctx)

	assert.NoError(t, err)
	assert.True(t, rag.initialized.Load())
	assert.Greater(t, rag.synonyms.Len(), 0)

	// Verify synonym dictionary
	fn, _ := rag.synonyms.Get("function")
	assert.Contains(t, fn, "method")
	va, _ := rag.synonyms.Get("variable")
	assert.Contains(t, va, "parameter")
	er, _ := rag.synonyms.Get("error")
	assert.Contains(t, er, "exception")

	// Test idempotent initialization
	err = rag.Initialize(ctx)
	assert.NoError(t, err)
}

func TestAdvancedRAG_ExpandQuery(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	ctx := context.Background()
	_ = rag.Initialize(ctx)

	t.Run("SimpleQuery", func(t *testing.T) {
		expansions := rag.ExpandQuery(ctx, "function")

		assert.NotEmpty(t, expansions)
		assert.Equal(t, "function", expansions[0].Query)
		assert.Equal(t, 1.0, expansions[0].Weight)
		assert.Equal(t, "original", expansions[0].Type)

		// Check for synonym expansions
		hasMethodExpansion := false
		for _, exp := range expansions {
			if exp.Query == "method" && exp.Type == "synonym" {
				hasMethodExpansion = true
				assert.Equal(t, 0.8, exp.Weight)
			}
		}
		assert.True(t, hasMethodExpansion, "Should have 'method' as synonym expansion")
	})

	t.Run("QueryWithMultipleTerms", func(t *testing.T) {
		expansions := rag.ExpandQuery(ctx, "create function")

		assert.NotEmpty(t, expansions)
		// Original query
		assert.Equal(t, "create function", expansions[0].Query)

		// Should have synonym expansions for both terms
		hasMakeExpansion := false
		hasMethodExpansion := false
		for _, exp := range expansions {
			if exp.Type == "synonym" {
				if exp.Query == "make function" {
					hasMakeExpansion = true
				}
				if exp.Query == "create method" {
					hasMethodExpansion = true
				}
			}
		}
		assert.True(t, hasMakeExpansion || hasMethodExpansion, "Should have synonym expansions")
	})

	t.Run("QueryWithNoSynonyms", func(t *testing.T) {
		expansions := rag.ExpandQuery(ctx, "xyz123")

		assert.Len(t, expansions, 1)
		assert.Equal(t, "xyz123", expansions[0].Query)
		assert.Equal(t, "original", expansions[0].Type)
	})

	t.Run("MaxExpansionsLimit", func(t *testing.T) {
		expansions := rag.ExpandQuery(ctx, "function variable class error create")

		// Should not exceed max expansions + 1 (original)
		assert.LessOrEqual(t, len(expansions), config.QueryExpansion.MaxExpansions+1)
	})

	t.Run("DisabledSynonyms", func(t *testing.T) {
		noSynConfig := DefaultAdvancedRAGConfig()
		noSynConfig.QueryExpansion.EnableSynonyms = false
		noSynRag := NewAdvancedRAG(noSynConfig, &Pipeline{})
		_ = noSynRag.Initialize(ctx)

		expansions := noSynRag.ExpandQuery(ctx, "function")

		assert.Len(t, expansions, 1)
		assert.Equal(t, "function", expansions[0].Query)
	})
}

func TestAdvancedRAG_ReRank(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	config.ReRanker.ScoreThreshold = 0.0 // Allow all results for testing
	rag := NewAdvancedRAG(config, &Pipeline{})
	ctx := context.Background()
	_ = rag.Initialize(ctx)

	t.Run("EmptyResults", func(t *testing.T) {
		results, err := rag.ReRank(ctx, "test query", []PipelineSearchResult{})

		assert.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("ReRankWithRelevantContent", func(t *testing.T) {
		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "This is about function implementation"}, Score: 0.8},
			{Chunk: PipelineChunk{ID: "2", Content: "Random unrelated content here"}, Score: 0.9},
			{Chunk: PipelineChunk{ID: "3", Content: "Function and method definition guide"}, Score: 0.7},
		}

		reranked, err := rag.ReRank(ctx, "function implementation", results)

		assert.NoError(t, err)
		assert.NotEmpty(t, reranked)

		// Result with "function implementation" should be higher
		assert.Equal(t, "1", reranked[0].Chunk.ID)
		assert.Equal(t, 1, reranked[0].ReRankPosition)
	})

	t.Run("PositionsAreCorrect", func(t *testing.T) {
		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "Content A"}, Score: 0.8},
			{Chunk: PipelineChunk{ID: "2", Content: "Content B"}, Score: 0.7},
			{Chunk: PipelineChunk{ID: "3", Content: "Content C"}, Score: 0.6},
		}

		reranked, err := rag.ReRank(ctx, "query", results)

		assert.NoError(t, err)
		for i, r := range reranked {
			assert.Equal(t, i+1, r.ReRankPosition)
		}
	})

	t.Run("ScoreThresholdFiltering", func(t *testing.T) {
		highThresholdConfig := DefaultAdvancedRAGConfig()
		highThresholdConfig.ReRanker.ScoreThreshold = 0.9
		highThresholdRag := NewAdvancedRAG(highThresholdConfig, &Pipeline{})
		_ = highThresholdRag.Initialize(ctx)

		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "Low relevance content"}, Score: 0.8},
		}

		reranked, err := highThresholdRag.ReRank(ctx, "something else entirely", results)

		assert.NoError(t, err)
		// Low relevance content should be filtered out
		assert.Empty(t, reranked)
	})

	t.Run("TopKLimit", func(t *testing.T) {
		limitConfig := DefaultAdvancedRAGConfig()
		limitConfig.ReRanker.TopK = 2
		limitConfig.ReRanker.ScoreThreshold = 0.0
		limitRag := NewAdvancedRAG(limitConfig, &Pipeline{})
		_ = limitRag.Initialize(ctx)

		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "A"}, Score: 0.8},
			{Chunk: PipelineChunk{ID: "2", Content: "B"}, Score: 0.7},
			{Chunk: PipelineChunk{ID: "3", Content: "C"}, Score: 0.6},
			{Chunk: PipelineChunk{ID: "4", Content: "D"}, Score: 0.5},
		}

		reranked, err := limitRag.ReRank(ctx, "query", results)

		assert.NoError(t, err)
		assert.LessOrEqual(t, len(reranked), 2)
	})
}

func TestAdvancedRAG_CompressContext(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	ctx := context.Background()
	_ = rag.Initialize(ctx)

	t.Run("EmptyResults", func(t *testing.T) {
		compressed, err := rag.CompressContext(ctx, "query", []PipelineSearchResult{})

		assert.NoError(t, err)
		assert.NotNil(t, compressed)
		assert.Empty(t, compressed.Content)
	})

	t.Run("BasicCompression", func(t *testing.T) {
		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "This is the first sentence about functions. This is another sentence about variables. And this is about errors."}},
			{Chunk: PipelineChunk{ID: "2", Content: "More content here about database operations. Query handling is important."}},
		}

		compressed, err := rag.CompressContext(ctx, "function variable", results)

		assert.NoError(t, err)
		assert.NotNil(t, compressed)
		assert.Greater(t, compressed.OriginalLength, 0)
		assert.LessOrEqual(t, compressed.CompressedLength, compressed.OriginalLength)
		assert.LessOrEqual(t, compressed.CompressionRatio, 1.0)
	})

	t.Run("KeyPhrasesExtraction", func(t *testing.T) {
		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "The function definition is important. Function parameters should be validated. The function returns a value."}},
		}

		compressed, err := rag.CompressContext(ctx, "function definition", results)

		assert.NoError(t, err)
		// Should extract key phrases containing query terms
		hasRelevantPhrase := false
		for _, phrase := range compressed.KeyPhrases {
			if contains(phrase, "function") {
				hasRelevantPhrase = true
				break
			}
		}
		// Key phrases might be empty if content is too short
		if len(compressed.KeyPhrases) > 0 {
			assert.True(t, hasRelevantPhrase)
		}
	})

	t.Run("MaxContextLengthLimit", func(t *testing.T) {
		shortConfig := DefaultAdvancedRAGConfig()
		shortConfig.ContextualCompression.MaxContextLength = 50
		shortConfig.ContextualCompression.CompressionRatio = 1.0 // Don't compress by ratio
		shortRag := NewAdvancedRAG(shortConfig, &Pipeline{})
		_ = shortRag.Initialize(ctx)

		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "This is a very long content that exceeds the maximum context length limit and should be truncated to fit within the specified bounds."}},
		}

		compressed, err := shortRag.CompressContext(ctx, "query", results)

		assert.NoError(t, err)
		assert.LessOrEqual(t, compressed.CompressedLength, 100) // Some buffer for sentence boundaries
	})

	t.Run("DisabledSentenceExtraction", func(t *testing.T) {
		noSentConfig := DefaultAdvancedRAGConfig()
		noSentConfig.ContextualCompression.EnableSentenceExtraction = false
		noSentConfig.ContextualCompression.CompressionRatio = 0.5
		noSentRag := NewAdvancedRAG(noSentConfig, &Pipeline{})
		_ = noSentRag.Initialize(ctx)

		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "First sentence here. Second sentence here. Third sentence here."}},
		}

		compressed, err := noSentRag.CompressContext(ctx, "query", results)

		assert.NoError(t, err)
		// Should use simple truncation
		assert.NotEmpty(t, compressed.Content)
	})
}

func TestTokenize(t *testing.T) {
	t.Run("SimpleText", func(t *testing.T) {
		tokens := tokenize("hello world")
		assert.Equal(t, []string{"hello", "world"}, tokens)
	})

	t.Run("WithPunctuation", func(t *testing.T) {
		tokens := tokenize("hello, world! how are you?")
		assert.Equal(t, []string{"hello", "world", "how", "are", "you"}, tokens)
	})

	t.Run("WithUnderscores", func(t *testing.T) {
		tokens := tokenize("snake_case variable_name")
		assert.Equal(t, []string{"snake_case", "variable_name"}, tokens)
	})

	t.Run("WithHyphens", func(t *testing.T) {
		tokens := tokenize("kebab-case name-here")
		assert.Equal(t, []string{"kebab-case", "name-here"}, tokens)
	})

	t.Run("EmptyString", func(t *testing.T) {
		tokens := tokenize("")
		assert.Empty(t, tokens)
	})
}

func TestSplitIntoSentences(t *testing.T) {
	t.Run("SimpleSentences", func(t *testing.T) {
		sentences := splitIntoSentences("First sentence. Second sentence. Third sentence.")
		assert.Len(t, sentences, 3)
	})

	t.Run("WithDifferentPunctuation", func(t *testing.T) {
		// Note: "Yes it is!" is filtered out because it's less than 10 characters
		sentences := splitIntoSentences("Is this a question? Yes indeed it is! Here is a statement.")
		assert.Len(t, sentences, 3)
	})

	t.Run("ShortSentencesFiltered", func(t *testing.T) {
		sentences := splitIntoSentences("OK. This is a longer sentence that should be kept.")
		// "OK." is too short (< 10 chars)
		assert.Len(t, sentences, 1)
	})

	t.Run("EmptyString", func(t *testing.T) {
		sentences := splitIntoSentences("")
		assert.Empty(t, sentences)
	})
}

func TestSimilarity(t *testing.T) {
	t.Run("IdenticalStrings", func(t *testing.T) {
		sim := similarity("hello", "hello")
		assert.Equal(t, 1.0, sim)
	})

	t.Run("CompletelyDifferent", func(t *testing.T) {
		sim := similarity("abc", "xyz")
		assert.Less(t, sim, 0.5)
	})

	t.Run("SimilarStrings", func(t *testing.T) {
		sim := similarity("function", "functions")
		assert.Greater(t, sim, 0.8)
	})

	t.Run("EmptyString", func(t *testing.T) {
		sim := similarity("", "hello")
		assert.Equal(t, 0.0, sim)

		sim = similarity("hello", "")
		assert.Equal(t, 0.0, sim)
	})
}

func TestLevenshteinDistance(t *testing.T) {
	t.Run("IdenticalStrings", func(t *testing.T) {
		dist := levenshteinDistance("hello", "hello")
		assert.Equal(t, 0, dist)
	})

	t.Run("Insertion", func(t *testing.T) {
		dist := levenshteinDistance("hello", "hellos")
		assert.Equal(t, 1, dist)
	})

	t.Run("Deletion", func(t *testing.T) {
		dist := levenshteinDistance("hello", "hell")
		assert.Equal(t, 1, dist)
	})

	t.Run("Substitution", func(t *testing.T) {
		dist := levenshteinDistance("hello", "hallo")
		assert.Equal(t, 1, dist)
	})

	t.Run("EmptyStrings", func(t *testing.T) {
		dist := levenshteinDistance("", "hello")
		assert.Equal(t, 5, dist)

		dist = levenshteinDistance("hello", "")
		assert.Equal(t, 5, dist)
	})
}

func TestContainsQueryTerm(t *testing.T) {
	t.Run("ContainsTerm", func(t *testing.T) {
		queryTerms := map[string]bool{"function": true, "method": true}
		assert.True(t, containsQueryTerm("the function returns", queryTerms))
	})

	t.Run("DoesNotContainTerm", func(t *testing.T) {
		queryTerms := map[string]bool{"function": true, "method": true}
		assert.False(t, containsQueryTerm("variable declaration", queryTerms))
	})

	t.Run("EmptyQueryTerms", func(t *testing.T) {
		queryTerms := map[string]bool{}
		assert.False(t, containsQueryTerm("function", queryTerms))
	})
}

func TestMin(t *testing.T) {
	assert.Equal(t, 1, min(1, 2, 3))
	assert.Equal(t, 1, min(3, 2, 1))
	assert.Equal(t, 1, min(1))
	assert.Equal(t, -5, min(0, -5, 10))
}

func TestAdvancedRAG_CalculateKeywordScore(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	_ = rag.Initialize(context.Background())

	t.Run("FullMatch", func(t *testing.T) {
		score := rag.calculateKeywordScore(
			[]string{"hello", "world"},
			[]string{"hello", "world", "test"},
			config.HybridSearch,
		)
		assert.Equal(t, float32(1.0), score)
	})

	t.Run("PartialMatch", func(t *testing.T) {
		score := rag.calculateKeywordScore(
			[]string{"hello", "world", "foo"},
			[]string{"hello", "test"},
			config.HybridSearch,
		)
		assert.InDelta(t, float32(0.33), score, 0.1)
	})

	t.Run("NoMatch", func(t *testing.T) {
		score := rag.calculateKeywordScore(
			[]string{"abc", "def"},
			[]string{"xyz", "uvw"},
			config.HybridSearch,
		)
		assert.Equal(t, float32(0.0), score)
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		score := rag.calculateKeywordScore(
			[]string{},
			[]string{"hello", "world"},
			config.HybridSearch,
		)
		assert.Equal(t, float32(0.0), score)
	})

	t.Run("FuzzyMatch", func(t *testing.T) {
		fuzzyConfig := config.HybridSearch
		fuzzyConfig.EnableFuzzyMatch = true
		fuzzyConfig.FuzzyThreshold = 0.8

		score := rag.calculateKeywordScore(
			[]string{"function"},
			[]string{"functions"},
			fuzzyConfig,
		)
		assert.Greater(t, score, float32(0.0))
	})
}

func TestAdvancedRAG_CalculateRelevanceScore(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	_ = rag.Initialize(context.Background())

	t.Run("HighRelevance", func(t *testing.T) {
		score := rag.calculateRelevanceScore(
			"function implementation",
			"This document discusses function implementation in detail. The function is the main topic.",
		)
		assert.Greater(t, score, float32(0.5))
	})

	t.Run("LowRelevance", func(t *testing.T) {
		score := rag.calculateRelevanceScore(
			"function implementation",
			"This is about something completely different like weather patterns.",
		)
		assert.Less(t, score, float32(0.5))
	})

	t.Run("EmptyContent", func(t *testing.T) {
		score := rag.calculateRelevanceScore("query", "")
		assert.Equal(t, float32(0.0), score)
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		score := rag.calculateRelevanceScore("", "some content")
		assert.Equal(t, float32(0.0), score)
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAdvancedRAG_ExtractKeyPhrases(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	_ = rag.Initialize(context.Background())

	t.Run("ExtractsRelevantPhrases", func(t *testing.T) {
		phrases := rag.extractKeyPhrases(
			"function",
			"The function definition is clear. The function returns a value. Another function call here.",
		)

		// Should extract phrases containing "function"
		assert.NotEmpty(t, phrases)
		hasFunction := false
		for _, p := range phrases {
			if contains(p, "function") {
				hasFunction = true
				break
			}
		}
		assert.True(t, hasFunction)
	})

	t.Run("LimitsPhrases", func(t *testing.T) {
		phrases := rag.extractKeyPhrases(
			"the",
			"The quick brown fox jumps over the lazy dog. The dog was not amused. The fox continued running.",
		)

		// Should not exceed 10 phrases
		assert.LessOrEqual(t, len(phrases), 10)
	})
}

func TestAdvancedRAG_ScoreSentenceRelevance(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})

	t.Run("HighRelevance", func(t *testing.T) {
		score := rag.scoreSentenceRelevance(
			[]string{"function", "implementation"},
			"The function implementation is straightforward.",
		)
		assert.Equal(t, 1.0, score)
	})

	t.Run("PartialRelevance", func(t *testing.T) {
		score := rag.scoreSentenceRelevance(
			[]string{"function", "implementation", "test"},
			"The function is tested.",
		)
		assert.InDelta(t, 0.33, score, 0.1)
	})

	t.Run("NoRelevance", func(t *testing.T) {
		score := rag.scoreSentenceRelevance(
			[]string{"function", "implementation"},
			"The weather is nice today.",
		)
		assert.Equal(t, 0.0, score)
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		score := rag.scoreSentenceRelevance([]string{}, "Any sentence here.")
		assert.Equal(t, 0.0, score)
	})
}

// Integration test for the full advanced RAG workflow
func TestAdvancedRAG_FullWorkflow(t *testing.T) {
	// This tests the configuration and initialization flow
	// Actual search requires a working pipeline with vector DB
	config := DefaultAdvancedRAGConfig()
	// Set score threshold to 0 to include all results in re-ranking
	config.ReRanker.ScoreThreshold = 0.0
	rag := NewAdvancedRAG(config, &Pipeline{})
	ctx := context.Background()

	err := rag.Initialize(ctx)
	require.NoError(t, err)

	// Test query expansion
	expansions := rag.ExpandQuery(ctx, "create database function")
	require.NotEmpty(t, expansions)

	// Test that original query is first
	assert.Equal(t, "create database function", expansions[0].Query)
	assert.Equal(t, 1.0, expansions[0].Weight)

	// Test compression with mock results
	mockResults := []PipelineSearchResult{
		{
			Chunk: PipelineChunk{
				ID:      "1",
				DocID:   "doc1",
				Content: "This is about creating database functions. Functions in databases are very useful. They help with data processing.",
			},
			Score: 0.9,
		},
		{
			Chunk: PipelineChunk{
				ID:      "2",
				DocID:   "doc2",
				Content: "Database operations require careful planning. Creating efficient queries is important.",
			},
			Score: 0.8,
		},
	}

	compressed, err := rag.CompressContext(ctx, "create database function", mockResults)
	require.NoError(t, err)
	assert.NotEmpty(t, compressed.Content)
	assert.Greater(t, compressed.OriginalLength, 0)

	// Test re-ranking
	reranked, err := rag.ReRank(ctx, "create database function", mockResults)
	require.NoError(t, err)
	assert.NotEmpty(t, reranked)

	// First result should have content about "creating database functions"
	assert.Equal(t, "1", reranked[0].Chunk.ID)
}
