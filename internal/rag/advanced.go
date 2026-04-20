// Package rag provides RAG (Retrieval-Augmented Generation) capabilities
// This file implements advanced RAG techniques including hybrid search,
// re-ranking, query expansion, and contextual compression.
package rag

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync/atomic"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
)

// HybridSearchConfig configures hybrid search behavior
type HybridSearchConfig struct {
	// VectorWeight is the weight for vector similarity (0-1)
	VectorWeight float64 `json:"vector_weight"`
	// KeywordWeight is the weight for keyword matching (0-1)
	KeywordWeight float64 `json:"keyword_weight"`
	// MinKeywordScore is the minimum keyword score to consider
	MinKeywordScore float64 `json:"min_keyword_score"`
	// EnableFuzzyMatch enables fuzzy keyword matching
	EnableFuzzyMatch bool `json:"enable_fuzzy_match"`
	// FuzzyThreshold is the minimum similarity for fuzzy matches (0-1)
	FuzzyThreshold float64 `json:"fuzzy_threshold"`
}

// DefaultHybridSearchConfig returns sensible defaults
func DefaultHybridSearchConfig() HybridSearchConfig {
	return HybridSearchConfig{
		VectorWeight:     0.7,
		KeywordWeight:    0.3,
		MinKeywordScore:  0.1,
		EnableFuzzyMatch: true,
		FuzzyThreshold:   0.8,
	}
}

// ReRankerConfig configures the re-ranking behavior
type ReRankerConfig struct {
	// Model is the re-ranking model to use
	Model string `json:"model"`
	// TopK is the number of results to re-rank
	TopK int `json:"top_k"`
	// BatchSize is the batch size for re-ranking
	BatchSize int `json:"batch_size"`
	// ScoreThreshold is the minimum score after re-ranking
	ScoreThreshold float64 `json:"score_threshold"`
	// EnableCrossEncoder enables cross-encoder re-ranking
	EnableCrossEncoder bool `json:"enable_cross_encoder"`
}

// DefaultReRankerConfig returns sensible defaults
func DefaultReRankerConfig() ReRankerConfig {
	return ReRankerConfig{
		Model:              "cross-encoder/ms-marco-MiniLM-L-6-v2",
		TopK:               100,
		BatchSize:          32,
		ScoreThreshold:     0.5,
		EnableCrossEncoder: true,
	}
}

// QueryExpansionConfig configures query expansion behavior
type QueryExpansionConfig struct {
	// MaxExpansions is the maximum number of expanded queries
	MaxExpansions int `json:"max_expansions"`
	// EnableSynonyms enables synonym expansion
	EnableSynonyms bool `json:"enable_synonyms"`
	// EnableHyponyms enables hyponym expansion (more specific terms)
	EnableHyponyms bool `json:"enable_hyponyms"`
	// EnableHypernyms enables hypernym expansion (more general terms)
	EnableHypernyms bool `json:"enable_hypernyms"`
	// EnableLLMExpansion enables LLM-based query expansion
	EnableLLMExpansion bool `json:"enable_llm_expansion"`
	// SynonymWeight is the weight for synonym-expanded queries
	SynonymWeight float64 `json:"synonym_weight"`
}

// DefaultQueryExpansionConfig returns sensible defaults
func DefaultQueryExpansionConfig() QueryExpansionConfig {
	return QueryExpansionConfig{
		MaxExpansions:      5,
		EnableSynonyms:     true,
		EnableHyponyms:     false,
		EnableHypernyms:    false,
		EnableLLMExpansion: false,
		SynonymWeight:      0.8,
	}
}

// ContextualCompressionConfig configures contextual compression
type ContextualCompressionConfig struct {
	// MaxContextLength is the maximum context length in tokens
	MaxContextLength int `json:"max_context_length"`
	// CompressionRatio is the target compression ratio
	CompressionRatio float64 `json:"compression_ratio"`
	// EnableSentenceExtraction enables extracting relevant sentences
	EnableSentenceExtraction bool `json:"enable_sentence_extraction"`
	// EnableSummarization enables summarization-based compression
	EnableSummarization bool `json:"enable_summarization"`
	// PreserveKeyPhrases preserves key phrases during compression
	PreserveKeyPhrases bool `json:"preserve_key_phrases"`
}

// DefaultContextualCompressionConfig returns sensible defaults
func DefaultContextualCompressionConfig() ContextualCompressionConfig {
	return ContextualCompressionConfig{
		MaxContextLength:         4096,
		CompressionRatio:         0.5,
		EnableSentenceExtraction: true,
		EnableSummarization:      false,
		PreserveKeyPhrases:       true,
	}
}

// AdvancedRAGConfig combines all advanced RAG configuration
type AdvancedRAGConfig struct {
	HybridSearch          HybridSearchConfig          `json:"hybrid_search"`
	ReRanker              ReRankerConfig              `json:"re_ranker"`
	QueryExpansion        QueryExpansionConfig        `json:"query_expansion"`
	ContextualCompression ContextualCompressionConfig `json:"contextual_compression"`
}

// DefaultAdvancedRAGConfig returns sensible defaults for all components
func DefaultAdvancedRAGConfig() AdvancedRAGConfig {
	return AdvancedRAGConfig{
		HybridSearch:          DefaultHybridSearchConfig(),
		ReRanker:              DefaultReRankerConfig(),
		QueryExpansion:        DefaultQueryExpansionConfig(),
		ContextualCompression: DefaultContextualCompressionConfig(),
	}
}

// AdvancedRAG provides advanced RAG techniques.
//
// Concurrency model (CONST-029): synonyms is a *safe.Store populated
// once in Initialize and read lock-free thereafter; initialized is
// atomic.Bool with CompareAndSwap for idempotent Initialize. config
// is frozen at construction so per-method read-locking around it is
// unnecessary.
type AdvancedRAG struct {
	config      AdvancedRAGConfig
	pipeline    *Pipeline
	synonyms    *safe.Store[string, []string]
	initialized atomic.Bool
}

// NewAdvancedRAG creates a new advanced RAG instance
func NewAdvancedRAG(config AdvancedRAGConfig, pipeline *Pipeline) *AdvancedRAG {
	return &AdvancedRAG{
		config:   config,
		pipeline: pipeline,
		synonyms: safe.NewStore[string, []string](),
	}
}

// Initialize initializes the advanced RAG components
func (a *AdvancedRAG) Initialize(ctx context.Context) error {
	if !a.initialized.CompareAndSwap(false, true) {
		return nil
	}

	// Initialize synonym dictionary with common programming terms
	synonyms := map[string][]string{
		"function": {"func", "method", "procedure", "subroutine"},
		"variable": {"var", "parameter", "argument", "field"},
		"class":    {"type", "struct", "object", "interface"},
		"error":    {"exception", "fault", "bug", "issue"},
		"create":   {"make", "new", "build", "construct", "generate"},
		"delete":   {"remove", "destroy", "drop", "erase"},
		"update":   {"modify", "change", "edit", "alter"},
		"read":     {"get", "fetch", "retrieve", "load"},
		"write":    {"put", "store", "save", "persist"},
		"api":      {"endpoint", "interface", "service"},
		"database": {"db", "datastore", "storage"},
		"test":     {"spec", "check", "verify", "validate"},
		"config":   {"configuration", "settings", "options"},
		"async":    {"asynchronous", "concurrent", "parallel"},
		"sync":     {"synchronous", "blocking", "sequential"},
		"cache":    {"memoize", "buffer", "store"},
		"query":    {"search", "find", "lookup", "retrieve"},
		"index":    {"key", "lookup", "pointer"},
		"schema":   {"model", "structure", "definition"},
		"vector":   {"embedding", "representation", "array"},
	}
	for k, v := range synonyms {
		a.synonyms.Put(k, v)
	}

	logrus.Info("Advanced RAG initialized")
	return nil
}

// HybridSearchResult represents a result from hybrid search
type HybridSearchResult struct {
	PipelineSearchResult
	VectorScore   float32 `json:"vector_score"`
	KeywordScore  float32 `json:"keyword_score"`
	CombinedScore float32 `json:"combined_score"`
}

// HybridSearch performs hybrid search combining vector and keyword search
func (a *AdvancedRAG) HybridSearch(ctx context.Context, query string, topK int) ([]HybridSearchResult, error) {
	config := a.config.HybridSearch

	// Perform vector search
	vectorResults, err := a.pipeline.Search(ctx, query, topK*2)
	if err != nil {
		return nil, err
	}

	// Create a map for combining scores
	resultMap := make(map[string]*HybridSearchResult)

	// Process vector results
	for _, vr := range vectorResults {
		key := vr.Chunk.ID
		resultMap[key] = &HybridSearchResult{
			PipelineSearchResult: vr,
			VectorScore:          vr.Score,
			KeywordScore:         0,
		}
	}

	// Perform keyword search and merge
	keywordScores := a.keywordSearch(query, vectorResults)
	for chunkID, score := range keywordScores {
		if result, ok := resultMap[chunkID]; ok {
			result.KeywordScore = score
		}
	}

	// Calculate combined scores
	results := make([]HybridSearchResult, 0, len(resultMap))
	for _, result := range resultMap {
		result.CombinedScore = float32(config.VectorWeight)*result.VectorScore +
			float32(config.KeywordWeight)*result.KeywordScore
		result.Score = result.CombinedScore
		results = append(results, *result)
	}

	// Sort by combined score
	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// keywordSearch performs keyword-based scoring on results
func (a *AdvancedRAG) keywordSearch(query string, results []PipelineSearchResult) map[string]float32 {
	scores := make(map[string]float32)
	queryTerms := tokenize(query)
	config := a.config.HybridSearch

	for _, result := range results {
		contentTerms := tokenize(result.Chunk.Content)
		score := a.calculateKeywordScore(queryTerms, contentTerms, config)
		if score >= float32(config.MinKeywordScore) {
			scores[result.Chunk.ID] = score
		}
	}

	return scores
}

// calculateKeywordScore calculates keyword match score
func (a *AdvancedRAG) calculateKeywordScore(queryTerms, contentTerms []string, config HybridSearchConfig) float32 {
	if len(queryTerms) == 0 {
		return 0
	}

	contentSet := make(map[string]bool)
	for _, term := range contentTerms {
		contentSet[strings.ToLower(term)] = true
	}

	matches := 0
	for _, qt := range queryTerms {
		qtLower := strings.ToLower(qt)
		if contentSet[qtLower] {
			matches++
			continue
		}

		// Check fuzzy match if enabled
		if config.EnableFuzzyMatch {
			for ct := range contentSet {
				if similarity(qtLower, ct) >= config.FuzzyThreshold {
					matches++
					break
				}
			}
		}
	}

	return float32(matches) / float32(len(queryTerms))
}

// ExpandQuery expands a query using various techniques
func (a *AdvancedRAG) ExpandQuery(ctx context.Context, query string) []ExpandedQuery {
	config := a.config.QueryExpansion
	expansions := []ExpandedQuery{{Query: query, Weight: 1.0, Type: "original"}}

	if !config.EnableSynonyms {
		return expansions
	}

	// Expand using synonyms
	terms := tokenize(query)
	for _, term := range terms {
		termLower := strings.ToLower(term)
		if synonymList, ok := a.synonyms.Get(termLower); ok {
			for _, synonym := range synonymList {
				if len(expansions) >= config.MaxExpansions+1 {
					break
				}
				expandedQuery := strings.Replace(strings.ToLower(query), termLower, synonym, 1)
				expansions = append(expansions, ExpandedQuery{
					Query:  expandedQuery,
					Weight: config.SynonymWeight,
					Type:   "synonym",
				})
			}
		}
	}

	return expansions
}

// ExpandedQuery represents an expanded query with its weight
type ExpandedQuery struct {
	Query  string  `json:"query"`
	Weight float64 `json:"weight"`
	Type   string  `json:"type"`
}

// SearchWithExpansion searches using query expansion
func (a *AdvancedRAG) SearchWithExpansion(ctx context.Context, query string, topK int) ([]PipelineSearchResult, error) {
	expansions := a.ExpandQuery(ctx, query)

	// Collect all results
	allResults := make(map[string]*PipelineSearchResult)
	resultScores := make(map[string]float32)

	for _, expansion := range expansions {
		results, err := a.pipeline.Search(ctx, expansion.Query, topK)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to search for expanded query: %s", expansion.Query)
			continue
		}

		for _, result := range results {
			key := result.Chunk.ID
			weightedScore := result.Score * float32(expansion.Weight)

			if existing, ok := resultScores[key]; ok {
				// Keep the higher weighted score
				if weightedScore > existing {
					resultScores[key] = weightedScore
					resultCopy := result
					resultCopy.Score = weightedScore
					allResults[key] = &resultCopy
				}
			} else {
				resultScores[key] = weightedScore
				resultCopy := result
				resultCopy.Score = weightedScore
				allResults[key] = &resultCopy
			}
		}
	}

	// Convert to slice and sort
	results := make([]PipelineSearchResult, 0, len(allResults))
	for _, result := range allResults {
		results = append(results, *result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// ReRankedResult represents a result after re-ranking
type ReRankedResult struct {
	PipelineSearchResult
	OriginalScore  float32 `json:"original_score"`
	ReRankedScore  float32 `json:"reranked_score"`
	ReRankPosition int     `json:"rerank_position"`
}

// ReRank re-ranks search results using cross-encoder scoring
func (a *AdvancedRAG) ReRank(ctx context.Context, query string, results []PipelineSearchResult) ([]ReRankedResult, error) {
	config := a.config.ReRanker

	if len(results) == 0 {
		return []ReRankedResult{}, nil
	}

	// Limit results to re-rank
	toReRank := results
	if len(toReRank) > config.TopK {
		toReRank = toReRank[:config.TopK]
	}

	reranked := make([]ReRankedResult, len(toReRank))

	// Calculate re-ranking scores based on query-document relevance
	// This is a simplified implementation - in production, use a cross-encoder model
	for i, result := range toReRank {
		relevanceScore := a.calculateRelevanceScore(query, result.Chunk.Content)

		reranked[i] = ReRankedResult{
			PipelineSearchResult: result,
			OriginalScore:        result.Score,
			ReRankedScore:        relevanceScore,
		}
	}

	// Sort by re-ranked score
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].ReRankedScore > reranked[j].ReRankedScore
	})

	// Update positions and filter by threshold
	filtered := make([]ReRankedResult, 0, len(reranked))
	for i := range reranked {
		reranked[i].ReRankPosition = i + 1
		reranked[i].Score = reranked[i].ReRankedScore
		if reranked[i].ReRankedScore >= float32(config.ScoreThreshold) {
			filtered = append(filtered, reranked[i])
		}
	}

	return filtered, nil
}

// calculateRelevanceScore calculates relevance between query and content
func (a *AdvancedRAG) calculateRelevanceScore(query, content string) float32 {
	queryTerms := tokenize(query)
	contentTerms := tokenize(content)

	if len(queryTerms) == 0 || len(contentTerms) == 0 {
		return 0
	}

	// Calculate term frequency in content
	termFreq := make(map[string]int)
	for _, term := range contentTerms {
		termFreq[strings.ToLower(term)]++
	}

	// Calculate score based on term overlap and frequency
	score := 0.0
	maxScore := float64(len(queryTerms))

	for _, qt := range queryTerms {
		qtLower := strings.ToLower(qt)
		if freq, ok := termFreq[qtLower]; ok {
			// Log-scaled frequency bonus
			score += 1.0 + math.Log1p(float64(freq-1))*0.1
		} else {
			// Check for partial match
			for ct, freq := range termFreq {
				if strings.Contains(ct, qtLower) || strings.Contains(qtLower, ct) {
					score += 0.5 + math.Log1p(float64(freq-1))*0.05
					break
				}
			}
		}
	}

	// Normalize to 0-1 range
	normalized := score / (maxScore * 1.5) // Account for frequency bonus
	if normalized > 1.0 {
		normalized = 1.0
	}

	return float32(normalized)
}

// CompressedContext represents compressed context for LLM
type CompressedContext struct {
	OriginalLength   int      `json:"original_length"`
	CompressedLength int      `json:"compressed_length"`
	Content          string   `json:"content"`
	KeyPhrases       []string `json:"key_phrases"`
	CompressionRatio float64  `json:"compression_ratio"`
}

// CompressContext compresses search results into a condensed context
func (a *AdvancedRAG) CompressContext(ctx context.Context, query string, results []PipelineSearchResult) (*CompressedContext, error) {
	config := a.config.ContextualCompression

	if len(results) == 0 {
		return &CompressedContext{}, nil
	}

	// Combine all content
	var originalContent strings.Builder
	for _, result := range results {
		originalContent.WriteString(result.Chunk.Content)
		originalContent.WriteString("\n\n")
	}
	original := originalContent.String()
	originalLen := len(original)

	// Extract relevant sentences if enabled
	var compressed string
	if config.EnableSentenceExtraction {
		compressed = a.extractRelevantSentences(query, original, config)
	} else {
		// Simple truncation
		targetLen := int(float64(originalLen) * config.CompressionRatio)
		if targetLen > config.MaxContextLength {
			targetLen = config.MaxContextLength
		}
		if len(original) > targetLen {
			compressed = original[:targetLen]
		} else {
			compressed = original
		}
	}

	// Extract key phrases if enabled
	var keyPhrases []string
	if config.PreserveKeyPhrases {
		keyPhrases = a.extractKeyPhrases(query, compressed)
	}

	compressedLen := len(compressed)
	ratio := 1.0
	if originalLen > 0 {
		ratio = float64(compressedLen) / float64(originalLen)
	}

	return &CompressedContext{
		OriginalLength:   originalLen,
		CompressedLength: compressedLen,
		Content:          compressed,
		KeyPhrases:       keyPhrases,
		CompressionRatio: ratio,
	}, nil
}

// extractRelevantSentences extracts the most relevant sentences
func (a *AdvancedRAG) extractRelevantSentences(query, content string, config ContextualCompressionConfig) string {
	sentences := splitIntoSentences(content)
	queryTerms := tokenize(query)

	// Score each sentence
	type scoredSentence struct {
		text  string
		score float64
		index int
	}

	scored := make([]scoredSentence, len(sentences))
	for i, sentence := range sentences {
		score := a.scoreSentenceRelevance(queryTerms, sentence)
		scored[i] = scoredSentence{text: sentence, score: score, index: i}
	}

	// Sort by score
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Select top sentences within length limit
	var result strings.Builder
	targetLen := int(float64(len(content)) * config.CompressionRatio)
	if targetLen > config.MaxContextLength {
		targetLen = config.MaxContextLength
	}

	// Collect high-scoring sentences
	selectedIndices := make([]int, 0)
	currentLen := 0
	for _, s := range scored {
		if currentLen+len(s.text) > targetLen {
			break
		}
		selectedIndices = append(selectedIndices, s.index)
		currentLen += len(s.text) + 1
	}

	// Sort by original order to maintain coherence
	sort.Ints(selectedIndices)

	// Build result
	for i, idx := range selectedIndices {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(scored[idx].text)
	}

	return result.String()
}

// scoreSentenceRelevance scores a sentence's relevance to query terms
func (a *AdvancedRAG) scoreSentenceRelevance(queryTerms []string, sentence string) float64 {
	sentenceTerms := tokenize(sentence)
	sentenceSet := make(map[string]bool)
	for _, t := range sentenceTerms {
		sentenceSet[strings.ToLower(t)] = true
	}

	matches := 0
	for _, qt := range queryTerms {
		if sentenceSet[strings.ToLower(qt)] {
			matches++
		}
	}

	if len(queryTerms) == 0 {
		return 0
	}
	return float64(matches) / float64(len(queryTerms))
}

// extractKeyPhrases extracts key phrases from content
func (a *AdvancedRAG) extractKeyPhrases(query, content string) []string {
	// Simple n-gram based key phrase extraction
	words := tokenize(content)
	queryTermsSet := make(map[string]bool)
	for _, t := range tokenize(query) {
		queryTermsSet[strings.ToLower(t)] = true
	}

	// Extract 2-3 word phrases that contain query terms
	phrases := make(map[string]int)
	for i := 0; i < len(words)-1; i++ {
		// Bigrams
		bigram := strings.ToLower(words[i] + " " + words[i+1])
		if containsQueryTerm(bigram, queryTermsSet) {
			phrases[bigram]++
		}

		// Trigrams
		if i < len(words)-2 {
			trigram := strings.ToLower(words[i] + " " + words[i+1] + " " + words[i+2])
			if containsQueryTerm(trigram, queryTermsSet) {
				phrases[trigram]++
			}
		}
	}

	// Sort by frequency and take top phrases
	type phraseCount struct {
		phrase string
		count  int
	}
	sorted := make([]phraseCount, 0, len(phrases))
	for p, c := range phrases {
		sorted = append(sorted, phraseCount{p, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	result := make([]string, 0, 10)
	for i, pc := range sorted {
		if i >= 10 {
			break
		}
		result = append(result, pc.phrase)
	}

	return result
}

// Helper functions

// tokenize splits text into tokens
func tokenize(text string) []string {
	// Simple word tokenization
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-')
	})
	return words
}

// splitIntoSentences splits text into sentences
func splitIntoSentences(text string) []string {
	// Simple sentence splitting
	var sentences []string
	var current strings.Builder

	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			sentence := strings.TrimSpace(current.String())
			if len(sentence) > 10 { // Minimum sentence length
				sentences = append(sentences, sentence)
			}
			current.Reset()
		}
	}

	// Add remaining text as last sentence
	if remaining := strings.TrimSpace(current.String()); len(remaining) > 10 {
		sentences = append(sentences, remaining)
	}

	return sentences
}

// similarity calculates string similarity (Levenshtein-based)
func similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	// Simple character-based similarity
	distance := levenshteinDistance(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance calculates edit distance between two strings
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

// containsQueryTerm checks if phrase contains any query term
func containsQueryTerm(phrase string, queryTerms map[string]bool) bool {
	for term := range queryTerms {
		if strings.Contains(phrase, term) {
			return true
		}
	}
	return false
}

// min returns minimum of integers
func min(values ...int) int {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// MultiHopConfig configures multi-hop retrieval behavior
type MultiHopConfig struct {
	// MaxHops is the maximum number of hops to follow
	MaxHops int `json:"max_hops"`
	// MinRelevanceScore is the minimum score for a hop to be followed
	MinRelevanceScore float64 `json:"min_relevance_score"`
	// MaxResultsPerHop limits results collected at each hop
	MaxResultsPerHop int `json:"max_results_per_hop"`
	// DecayFactor reduces scores for results found in later hops
	DecayFactor float64 `json:"decay_factor"`
	// EnableBacklinks follows references pointing back to source documents
	EnableBacklinks bool `json:"enable_backlinks"`
}

// DefaultMultiHopConfig returns sensible defaults for multi-hop retrieval
func DefaultMultiHopConfig() MultiHopConfig {
	return MultiHopConfig{
		MaxHops:           3,
		MinRelevanceScore: 0.3,
		MaxResultsPerHop:  10,
		DecayFactor:       0.8,
		EnableBacklinks:   true,
	}
}

// MultiHopResult represents a result from multi-hop retrieval
type MultiHopResult struct {
	PipelineSearchResult
	HopLevel    int      `json:"hop_level"`
	HopPath     []string `json:"hop_path"`
	SourceDocID string   `json:"source_doc_id,omitempty"`
}

// MultiHopSearch performs multi-hop retrieval following document relationships
func (a *AdvancedRAG) MultiHopSearch(ctx context.Context, query string, topK int, config *MultiHopConfig) ([]MultiHopResult, error) {
	if config == nil {
		defaultConfig := DefaultMultiHopConfig()
		config = &defaultConfig
	}

	// Track visited documents to avoid cycles
	visited := make(map[string]bool)
	allResults := make([]MultiHopResult, 0)

	// Initial search (hop 0)
	initialResults, err := a.pipeline.Search(ctx, query, config.MaxResultsPerHop)
	if err != nil {
		return nil, err
	}

	// Add initial results
	for _, result := range initialResults {
		if result.Score < float32(config.MinRelevanceScore) {
			continue
		}
		visited[result.Chunk.ID] = true
		allResults = append(allResults, MultiHopResult{
			PipelineSearchResult: result,
			HopLevel:             0,
			HopPath:              []string{result.Chunk.ID},
		})
	}

	// Perform subsequent hops
	currentHopResults := initialResults
	for hop := 1; hop <= config.MaxHops; hop++ {
		if len(currentHopResults) == 0 {
			break
		}

		nextHopResults := make([]PipelineSearchResult, 0)
		decayMultiplier := math.Pow(config.DecayFactor, float64(hop))

		for _, sourceResult := range currentHopResults {
			// Extract related terms/concepts from source document for next hop query
			relatedQuery := a.extractRelatedTerms(query, sourceResult.Chunk.Content)
			if relatedQuery == "" {
				continue
			}

			// Search for related documents
			hopResults, err := a.pipeline.Search(ctx, relatedQuery, config.MaxResultsPerHop)
			if err != nil {
				continue
			}

			for _, hopResult := range hopResults {
				// Skip already visited documents
				if visited[hopResult.Chunk.ID] {
					continue
				}

				// Apply minimum score threshold with decay
				adjustedScore := float64(hopResult.Score) * decayMultiplier
				if adjustedScore < config.MinRelevanceScore {
					continue
				}

				visited[hopResult.Chunk.ID] = true
				hopResult.Score = float32(adjustedScore)

				// Build hop path
				var parentPath []string
				for _, existing := range allResults {
					if existing.Chunk.ID == sourceResult.Chunk.ID {
						parentPath = existing.HopPath
						break
					}
				}
				hopPath := append(append([]string{}, parentPath...), hopResult.Chunk.ID)

				allResults = append(allResults, MultiHopResult{
					PipelineSearchResult: hopResult,
					HopLevel:             hop,
					HopPath:              hopPath,
					SourceDocID:          sourceResult.Chunk.ID,
				})
				nextHopResults = append(nextHopResults, hopResult)
			}
		}

		currentHopResults = nextHopResults
	}

	// Sort by score and limit to topK
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	return allResults, nil
}

// extractRelatedTerms extracts key terms from content to use for hop queries
func (a *AdvancedRAG) extractRelatedTerms(originalQuery, content string) string {
	// Tokenize content
	contentTerms := tokenize(content)
	if len(contentTerms) == 0 {
		return ""
	}

	// Get original query terms to avoid duplication
	queryTermsSet := make(map[string]bool)
	for _, t := range tokenize(originalQuery) {
		queryTermsSet[strings.ToLower(t)] = true
	}

	// Count term frequencies in content
	termFreq := make(map[string]int)
	for _, term := range contentTerms {
		termLower := strings.ToLower(term)
		// Skip very short terms and original query terms
		if len(termLower) < 3 || queryTermsSet[termLower] {
			continue
		}
		termFreq[termLower]++
	}

	// Select top frequent terms
	type termCount struct {
		term  string
		count int
	}
	sortedTerms := make([]termCount, 0, len(termFreq))
	for term, count := range termFreq {
		sortedTerms = append(sortedTerms, termCount{term, count})
	}
	sort.Slice(sortedTerms, func(i, j int) bool {
		return sortedTerms[i].count > sortedTerms[j].count
	})

	// Build related query from top terms
	var relatedTerms []string
	for i, tc := range sortedTerms {
		if i >= 5 { // Take top 5 new terms
			break
		}
		relatedTerms = append(relatedTerms, tc.term)
	}

	if len(relatedTerms) == 0 {
		return ""
	}

	// Combine original query with related terms
	return originalQuery + " " + strings.Join(relatedTerms, " ")
}

// IterativeRetrievalConfig configures iterative retrieval behavior
type IterativeRetrievalConfig struct {
	// MaxIterations is the maximum number of retrieval iterations
	MaxIterations int `json:"max_iterations"`
	// ConvergenceThreshold stops iteration when improvement is below this
	ConvergenceThreshold float64 `json:"convergence_threshold"`
	// ResultsPerIteration is the number of results to fetch each iteration
	ResultsPerIteration int `json:"results_per_iteration"`
	// FeedbackWeight controls how much feedback affects subsequent queries
	FeedbackWeight float64 `json:"feedback_weight"`
	// EnableQueryRefinement enables automatic query refinement between iterations
	EnableQueryRefinement bool `json:"enable_query_refinement"`
}

// DefaultIterativeRetrievalConfig returns sensible defaults
func DefaultIterativeRetrievalConfig() IterativeRetrievalConfig {
	return IterativeRetrievalConfig{
		MaxIterations:         5,
		ConvergenceThreshold:  0.05,
		ResultsPerIteration:   20,
		FeedbackWeight:        0.3,
		EnableQueryRefinement: true,
	}
}

// IterativeResult represents a result from iterative retrieval
type IterativeResult struct {
	PipelineSearchResult
	Iteration       int     `json:"iteration"`
	ImprovementGain float64 `json:"improvement_gain"`
}

// IterativeRetrievalMetrics tracks iteration progress
type IterativeRetrievalMetrics struct {
	Iterations    int       `json:"iterations"`
	QueriesUsed   []string  `json:"queries_used"`
	ScoreHistory  []float64 `json:"score_history"`
	Converged     bool      `json:"converged"`
	FinalAvgScore float64   `json:"final_avg_score"`
}

// IterativeSearch performs iterative retrieval with query refinement
func (a *AdvancedRAG) IterativeSearch(ctx context.Context, query string, topK int, config *IterativeRetrievalConfig) ([]IterativeResult, *IterativeRetrievalMetrics, error) {
	if config == nil {
		defaultConfig := DefaultIterativeRetrievalConfig()
		config = &defaultConfig
	}

	metrics := &IterativeRetrievalMetrics{
		QueriesUsed:  []string{query},
		ScoreHistory: make([]float64, 0),
	}

	// Track all results across iterations
	allResultsMap := make(map[string]*IterativeResult)
	currentQuery := query
	previousAvgScore := 0.0

	for iteration := 0; iteration < config.MaxIterations; iteration++ {
		// Perform search with current query
		results, err := a.pipeline.Search(ctx, currentQuery, config.ResultsPerIteration)
		if err != nil {
			return nil, metrics, err
		}

		if len(results) == 0 {
			break
		}

		// Calculate average score for this iteration
		totalScore := 0.0
		for _, result := range results {
			totalScore += float64(result.Score)

			// Update or add result
			if existing, ok := allResultsMap[result.Chunk.ID]; ok {
				// Keep the better score
				if result.Score > existing.Score {
					existing.Score = result.Score
					existing.Iteration = iteration
				}
			} else {
				allResultsMap[result.Chunk.ID] = &IterativeResult{
					PipelineSearchResult: result,
					Iteration:            iteration,
				}
			}
		}

		currentAvgScore := totalScore / float64(len(results))
		metrics.ScoreHistory = append(metrics.ScoreHistory, currentAvgScore)

		// Check for convergence
		improvement := currentAvgScore - previousAvgScore
		if iteration > 0 && improvement < config.ConvergenceThreshold {
			metrics.Converged = true
			break
		}

		// Refine query for next iteration using top results
		if config.EnableQueryRefinement && iteration < config.MaxIterations-1 {
			currentQuery = a.refineQuery(query, results, config.FeedbackWeight)
			metrics.QueriesUsed = append(metrics.QueriesUsed, currentQuery)
		}

		previousAvgScore = currentAvgScore
		metrics.Iterations = iteration + 1
	}

	// Convert map to slice and calculate improvement gains
	allResults := make([]IterativeResult, 0, len(allResultsMap))
	for _, result := range allResultsMap {
		// Calculate improvement gain based on when found
		if len(metrics.ScoreHistory) > result.Iteration {
			baseScore := metrics.ScoreHistory[0]
			if baseScore > 0 {
				result.ImprovementGain = (float64(result.Score) - baseScore) / baseScore
			}
		}
		allResults = append(allResults, *result)
	}

	// Sort by score
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// Limit to topK
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	// Calculate final average score
	if len(allResults) > 0 {
		total := 0.0
		for _, r := range allResults {
			total += float64(r.Score)
		}
		metrics.FinalAvgScore = total / float64(len(allResults))
	}

	return allResults, metrics, nil
}

// refineQuery refines the query based on top results
func (a *AdvancedRAG) refineQuery(originalQuery string, results []PipelineSearchResult, feedbackWeight float64) string {
	if len(results) == 0 || feedbackWeight <= 0 {
		return originalQuery
	}

	// Extract frequent terms from top results
	termFreq := make(map[string]float64)
	originalTerms := make(map[string]bool)
	for _, t := range tokenize(originalQuery) {
		originalTerms[strings.ToLower(t)] = true
	}

	// Weight terms by their document's score
	for _, result := range results {
		terms := tokenize(result.Chunk.Content)
		docWeight := float64(result.Score)

		for _, term := range terms {
			termLower := strings.ToLower(term)
			if len(termLower) >= 3 && !originalTerms[termLower] {
				termFreq[termLower] += docWeight
			}
		}
	}

	// Select top expansion terms
	type weightedTerm struct {
		term   string
		weight float64
	}
	sortedTerms := make([]weightedTerm, 0, len(termFreq))
	for term, weight := range termFreq {
		sortedTerms = append(sortedTerms, weightedTerm{term, weight})
	}
	sort.Slice(sortedTerms, func(i, j int) bool {
		return sortedTerms[i].weight > sortedTerms[j].weight
	})

	// Add top terms to query
	var expansionTerms []string
	for i, wt := range sortedTerms {
		if i >= 3 { // Add up to 3 expansion terms
			break
		}
		expansionTerms = append(expansionTerms, wt.term)
	}

	if len(expansionTerms) == 0 {
		return originalQuery
	}

	return originalQuery + " " + strings.Join(expansionTerms, " ")
}

// RecursiveSearch performs recursive retrieval for deep document exploration
func (a *AdvancedRAG) RecursiveSearch(ctx context.Context, query string, topK int, maxDepth int) ([]MultiHopResult, error) {
	config := &MultiHopConfig{
		MaxHops:           maxDepth,
		MinRelevanceScore: 0.25,
		MaxResultsPerHop:  15,
		DecayFactor:       0.85,
		EnableBacklinks:   true,
	}
	return a.MultiHopSearch(ctx, query, topK, config)
}
