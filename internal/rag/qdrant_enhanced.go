// Package rag provides enhanced Qdrant retriever with hybrid search and debate evaluation.
package rag

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// QdrantEnhancedRetriever combines dense Qdrant retrieval with BM25 sparse retrieval
// and optional AI debate-based relevance evaluation.
type QdrantEnhancedRetriever struct {
	denseRetriever  Retriever
	sparseIndex     *EnhancedBM25Index
	reranker        Reranker
	debateEvaluator QdrantDebateEvaluator
	config          *QdrantEnhancedConfig
	logger          *logrus.Logger
	mu              sync.RWMutex
}

// QdrantDebateEvaluator uses AI debate to evaluate document relevance
type QdrantDebateEvaluator interface {
	EvaluateRelevance(ctx context.Context, query, document string) (float64, error)
}

// QdrantEnhancedConfig configuration for enhanced retriever
type QdrantEnhancedConfig struct {
	DenseWeight         float64      `json:"dense_weight"`
	SparseWeight        float64      `json:"sparse_weight"`
	UseDebateEvaluation bool         `json:"use_debate_evaluation"`
	DebateTopK          int          `json:"debate_top_k"`
	FusionMethod        FusionMethod `json:"fusion_method"`
	RRFK                float64      `json:"rrf_k"`
}

// DefaultQdrantEnhancedConfig returns default configuration
func DefaultQdrantEnhancedConfig() *QdrantEnhancedConfig {
	return &QdrantEnhancedConfig{
		DenseWeight:         0.6,
		SparseWeight:        0.4,
		UseDebateEvaluation: false,
		DebateTopK:          5,
		FusionMethod:        FusionRRF,
		RRFK:                60.0,
	}
}

// NewQdrantEnhancedRetriever creates a new enhanced Qdrant retriever
func NewQdrantEnhancedRetriever(
	denseRetriever Retriever,
	reranker Reranker,
	config *QdrantEnhancedConfig,
	logger *logrus.Logger,
) *QdrantEnhancedRetriever {
	if config == nil {
		config = DefaultQdrantEnhancedConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &QdrantEnhancedRetriever{
		denseRetriever: denseRetriever,
		sparseIndex:    NewEnhancedBM25Index(),
		reranker:       reranker,
		config:         config,
		logger:         logger,
	}
}

// SetDebateEvaluator sets the debate evaluator for AI-based relevance
func (r *QdrantEnhancedRetriever) SetDebateEvaluator(evaluator QdrantDebateEvaluator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.debateEvaluator = evaluator
	r.config.UseDebateEvaluation = evaluator != nil
}

// Retrieve implements Retriever interface with hybrid search
func (r *QdrantEnhancedRetriever) Retrieve(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	// Expand retrieval to get more candidates
	expandedOpts := &SearchOptions{
		TopK:            opts.TopK * 3,
		MinScore:        opts.MinScore,
		Filter:          opts.Filter,
		EnableReranking: false, // We'll rerank later
		HybridAlpha:     opts.HybridAlpha,
		IncludeMetadata: opts.IncludeMetadata,
		Namespace:       opts.Namespace,
	}

	// Dense retrieval
	denseResults, err := r.denseRetriever.Retrieve(ctx, query, expandedOpts)
	if err != nil {
		r.logger.WithError(err).Warn("Dense retrieval failed, using sparse only")
		denseResults = []*SearchResult{}
	}

	// Sparse retrieval using BM25
	sparseResults := r.sparseIndex.Search(query, expandedOpts.TopK)

	// Fuse results
	fusedResults := r.fuseResults(denseResults, sparseResults)

	// Limit to requested TopK
	if len(fusedResults) > opts.TopK {
		fusedResults = fusedResults[:opts.TopK]
	}

	// Rerank if enabled and reranker available
	if opts.EnableReranking && r.reranker != nil {
		reranked, err := r.reranker.Rerank(ctx, query, fusedResults, opts.TopK)
		if err != nil {
			r.logger.WithError(err).Warn("Reranking failed, using fused results")
		} else {
			fusedResults = reranked
		}
	}

	// Use debate-based evaluation if enabled
	r.mu.RLock()
	useDebate := r.config.UseDebateEvaluation && r.debateEvaluator != nil
	debateEval := r.debateEvaluator
	debateTopK := r.config.DebateTopK
	r.mu.RUnlock()

	if useDebate && len(fusedResults) > 0 {
		fusedResults = r.evaluateWithDebate(ctx, query, fusedResults, debateEval, debateTopK)
	}

	r.logger.WithFields(logrus.Fields{
		"query":        truncateText(query, 50),
		"dense_count":  len(denseResults),
		"sparse_count": len(sparseResults),
		"fused_count":  len(fusedResults),
	}).Debug("Hybrid retrieval completed")

	return fusedResults, nil
}

// Index implements Retriever interface
func (r *QdrantEnhancedRetriever) Index(ctx context.Context, docs []*Document) error {
	// Index in dense retriever
	if err := r.denseRetriever.Index(ctx, docs); err != nil {
		return err
	}

	// Index in sparse BM25 index
	for _, doc := range docs {
		r.sparseIndex.AddDocument(doc.ID, doc.Content)
	}

	r.logger.WithField("count", len(docs)).Debug("Documents indexed for hybrid search")
	return nil
}

// Delete implements Retriever interface
func (r *QdrantEnhancedRetriever) Delete(ctx context.Context, ids []string) error {
	// Delete from dense retriever
	if err := r.denseRetriever.Delete(ctx, ids); err != nil {
		return err
	}

	// Delete from sparse index
	for _, id := range ids {
		r.sparseIndex.RemoveDocument(id)
	}

	return nil
}

func (r *QdrantEnhancedRetriever) fuseResults(denseResults, sparseResults []*SearchResult) []*SearchResult {
	switch r.config.FusionMethod {
	case FusionRRF:
		return r.rrfFusion(denseResults, sparseResults)
	case FusionWeighted:
		return r.weightedFusion(denseResults, sparseResults)
	default:
		return r.rrfFusion(denseResults, sparseResults)
	}
}

func (r *QdrantEnhancedRetriever) rrfFusion(denseResults, sparseResults []*SearchResult) []*SearchResult {
	k := r.config.RRFK
	scores := make(map[string]float64)
	docs := make(map[string]*SearchResult)

	// Score dense results
	for rank, result := range denseResults {
		if result.Document == nil {
			continue
		}
		id := result.Document.ID
		scores[id] += r.config.DenseWeight / (k + float64(rank+1))
		if _, exists := docs[id]; !exists {
			docs[id] = result
		}
	}

	// Score sparse results
	for rank, result := range sparseResults {
		if result.Document == nil {
			continue
		}
		id := result.Document.ID
		scores[id] += r.config.SparseWeight / (k + float64(rank+1))
		if _, exists := docs[id]; !exists {
			docs[id] = result
		}
	}

	// Create fused results
	var fused []*SearchResult
	for id, score := range scores {
		if doc, ok := docs[id]; ok {
			fused = append(fused, &SearchResult{
				Document:  doc.Document,
				Score:     score,
				MatchType: MatchTypeHybrid,
			})
		}
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	return fused
}

func (r *QdrantEnhancedRetriever) weightedFusion(denseResults, sparseResults []*SearchResult) []*SearchResult {
	scores := make(map[string]float64)
	docs := make(map[string]*SearchResult)

	// Normalize and weight dense scores
	maxDense := 0.0
	for _, result := range denseResults {
		if result.Score > maxDense {
			maxDense = result.Score
		}
	}
	if maxDense > 0 {
		for _, result := range denseResults {
			if result.Document == nil {
				continue
			}
			id := result.Document.ID
			scores[id] += (result.Score / maxDense) * r.config.DenseWeight
			if _, exists := docs[id]; !exists {
				docs[id] = result
			}
		}
	}

	// Normalize and weight sparse scores
	maxSparse := 0.0
	for _, result := range sparseResults {
		if result.Score > maxSparse {
			maxSparse = result.Score
		}
	}
	if maxSparse > 0 {
		for _, result := range sparseResults {
			if result.Document == nil {
				continue
			}
			id := result.Document.ID
			scores[id] += (result.Score / maxSparse) * r.config.SparseWeight
			if _, exists := docs[id]; !exists {
				docs[id] = result
			}
		}
	}

	var fused []*SearchResult
	for id, score := range scores {
		if doc, ok := docs[id]; ok {
			fused = append(fused, &SearchResult{
				Document:  doc.Document,
				Score:     score,
				MatchType: MatchTypeHybrid,
			})
		}
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})

	return fused
}

func (r *QdrantEnhancedRetriever) evaluateWithDebate(
	ctx context.Context,
	query string,
	results []*SearchResult,
	evaluator QdrantDebateEvaluator,
	maxEval int,
) []*SearchResult {
	if maxEval > len(results) {
		maxEval = len(results)
	}

	for i := 0; i < maxEval; i++ {
		result := results[i]
		if result.Document == nil {
			continue
		}

		relevance, err := evaluator.EvaluateRelevance(ctx, query, result.Document.Content)
		if err != nil {
			r.logger.WithError(err).Warn("Debate evaluation failed")
			continue
		}

		// Combine original score with debate relevance
		result.Score = result.Score*0.6 + relevance*0.4
		result.RerankedScore = relevance
	}

	// Re-sort after debate evaluation
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// EnhancedBM25Index provides BM25 sparse retrieval for hybrid search.
//
// Concurrent-safe by construction (CONST-029):
//
// The whole mutable state (four maps + avgDocLen + totalDocs) lives in
// a *bm25State pointed to by an atomic.Pointer. Readers Load the
// pointer once per Search and dereference it — no lock. Writers
// (AddDocument / RemoveDocument) serialise through writeMu, copy the
// current snapshot with the rows they intend to touch, and Store the
// new pointer. k1 and b are immutable after construction.
//
// Per-document inner maps (termFreqs[docID]) are frozen after
// AddDocument returns them in the snapshot, so new snapshots can
// safely share the inner maps of documents they don't mutate. Remove
// only ever *drops* a docID's inner map; it never mutates it.
//
// Exported accessors (TotalDocs, AvgDocLen, K1, B, Documents) let
// tests and external callers inspect state without touching the
// internals or holding any lock.
type EnhancedBM25Index struct {
	state   atomic.Pointer[bm25State]
	writeMu sync.Mutex
	k1      float64 // immutable after construction
	b       float64 // immutable after construction
}

// bm25State is the joint snapshot of all mutable fields. Instances are
// treated as immutable after being Stored into state — writers always
// allocate a fresh bm25State (with shared inner maps where safe) and
// swap the pointer.
type bm25State struct {
	documents  map[string]string
	termFreqs  map[string]map[string]int // per-doc inner maps are frozen post-AddDocument
	docFreqs   map[string]int
	docLengths map[string]int
	avgDocLen  float64
	totalDocs  int
}

// newEmptyBM25State returns an empty state seed.
func newEmptyBM25State() *bm25State {
	return &bm25State{
		documents:  make(map[string]string),
		termFreqs:  make(map[string]map[string]int),
		docFreqs:   make(map[string]int),
		docLengths: make(map[string]int),
	}
}

// loadState returns the current state, substituting an empty seed on
// first read so callers never dereference nil.
func (idx *EnhancedBM25Index) loadState() *bm25State {
	s := idx.state.Load()
	if s == nil {
		return newEmptyBM25State()
	}
	return s
}

// NewEnhancedBM25Index creates a new BM25 index for enhanced retrieval
func NewEnhancedBM25Index() *EnhancedBM25Index {
	idx := &EnhancedBM25Index{
		k1: 1.2,
		b:  0.75,
	}
	idx.state.Store(newEmptyBM25State())
	return idx
}

// TotalDocs returns the number of documents currently indexed.
func (idx *EnhancedBM25Index) TotalDocs() int {
	return idx.loadState().totalDocs
}

// AvgDocLen returns the current average document length.
func (idx *EnhancedBM25Index) AvgDocLen() float64 {
	return idx.loadState().avgDocLen
}

// K1 returns the BM25 k1 tuning constant.
func (idx *EnhancedBM25Index) K1() float64 { return idx.k1 }

// B returns the BM25 b tuning constant.
func (idx *EnhancedBM25Index) B() float64 { return idx.b }

// Documents returns a point-in-time copy of the (id → content) map.
// Returns an empty (non-nil) map when the index is empty, so callers
// using assert.Empty get the intuitive zero result.
func (idx *EnhancedBM25Index) Documents() map[string]string {
	s := idx.loadState()
	out := make(map[string]string, len(s.documents))
	for k, v := range s.documents {
		out[k] = v
	}
	return out
}

// AddDocument adds a document to the BM25 index
func (idx *EnhancedBM25Index) AddDocument(id, content string) {
	idx.writeMu.Lock()
	defer idx.writeMu.Unlock()

	cur := idx.loadState()
	terms := enhancedTokenize(content)

	next := &bm25State{
		documents:  copyStringMap(cur.documents),
		termFreqs:  copyOuterTermFreqs(cur.termFreqs),
		docFreqs:   copyIntMap(cur.docFreqs),
		docLengths: copyIntMap(cur.docLengths),
		totalDocs:  cur.totalDocs,
	}

	next.documents[id] = content
	// Brand-new inner map; once stored it is never mutated in place.
	innerFreqs := make(map[string]int, len(terms))
	next.termFreqs[id] = innerFreqs
	next.docLengths[id] = len(terms)

	termsSeen := make(map[string]bool)
	for _, term := range terms {
		innerFreqs[term]++
		if !termsSeen[term] {
			next.docFreqs[term]++
			termsSeen[term] = true
		}
	}

	next.totalDocs++
	next.avgDocLen = recalcAvgDocLen(next.docLengths, next.totalDocs)

	idx.state.Store(next)
}

// RemoveDocument removes a document from the index
func (idx *EnhancedBM25Index) RemoveDocument(id string) {
	idx.writeMu.Lock()
	defer idx.writeMu.Unlock()

	cur := idx.loadState()
	if _, exists := cur.documents[id]; !exists {
		return
	}

	next := &bm25State{
		documents:  copyStringMap(cur.documents),
		termFreqs:  copyOuterTermFreqs(cur.termFreqs),
		docFreqs:   copyIntMap(cur.docFreqs),
		docLengths: copyIntMap(cur.docLengths),
		totalDocs:  cur.totalDocs,
	}

	// Decrement doc frequencies based on the *old* snapshot's per-doc
	// inner map (frozen, so we can safely read it without copying).
	for term := range cur.termFreqs[id] {
		next.docFreqs[term]--
		if next.docFreqs[term] <= 0 {
			delete(next.docFreqs, term)
		}
	}

	delete(next.documents, id)
	delete(next.termFreqs, id)
	delete(next.docLengths, id)
	next.totalDocs--
	next.avgDocLen = recalcAvgDocLen(next.docLengths, next.totalDocs)

	idx.state.Store(next)
}

// Search performs BM25 search against an atomic snapshot of the index.
// No lock is held for the duration.
func (idx *EnhancedBM25Index) Search(query string, topK int) []*SearchResult {
	s := idx.loadState()
	queryTerms := enhancedTokenize(query)
	scores := make(map[string]float64)

	for _, term := range queryTerms {
		df, exists := s.docFreqs[term]
		if !exists {
			continue
		}

		idf := calculateIDF(df, s.totalDocs)

		for docID, tf := range s.termFreqs {
			termFreq, ok := tf[term]
			if !ok {
				continue
			}

			docLen := float64(s.docLengths[docID])
			tfScore := idx.calculateTF(float64(termFreq), docLen, s.avgDocLen)
			scores[docID] += idf * tfScore
		}
	}

	// Convert to results
	var results []*SearchResult
	for docID, score := range scores {
		results = append(results, &SearchResult{
			Document: &Document{
				ID:      docID,
				Content: s.documents[docID],
			},
			Score:     score,
			MatchType: MatchTypeSparse,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// calculateIDF is a free function so it needs no receiver state.
func calculateIDF(df, totalDocs int) float64 {
	n := float64(totalDocs)
	return math.Log((n-float64(df)+0.5)/(float64(df)+0.5) + 1)
}

// calculateTF takes avgDocLen as a parameter so callers hold exactly
// one consistent snapshot across a Search sweep. The method lives on
// EnhancedBM25Index so k1 and b are still reached via the receiver;
// they are immutable after construction.
func (idx *EnhancedBM25Index) calculateTF(tf, docLen, avgDocLen float64) float64 {
	return (tf * (idx.k1 + 1)) / (tf + idx.k1*(1-idx.b+idx.b*(docLen/avgDocLen)))
}

// recalcAvgDocLen is a free function taking the docLengths map and
// totalDocs so it can operate on a freshly-built snapshot inside
// AddDocument / RemoveDocument (before the snapshot is Stored).
func recalcAvgDocLen(docLengths map[string]int, totalDocs int) float64 {
	if totalDocs == 0 {
		return 0
	}
	total := 0
	for _, length := range docLengths {
		total += length
	}
	return float64(total) / float64(totalDocs)
}

// copyStringMap / copyIntMap / copyOuterTermFreqs are shallow copies.
// For copyOuterTermFreqs the *inner* maps are shared because AddDocument
// never mutates an existing doc's inner map (it only inserts a fresh
// one for the new id) and RemoveDocument only deletes the outer entry.
func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyOuterTermFreqs(in map[string]map[string]int) map[string]map[string]int {
	out := make(map[string]map[string]int, len(in))
	for k, v := range in {
		out[k] = v // inner maps are treated as immutable — safe to share
	}
	return out
}

// enhancedTokenize tokenizes text for BM25 (renamed to avoid conflict with existing tokenize)
func enhancedTokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.Fields(text)

	var tokens []string
	for _, word := range words {
		cleaned := strings.Trim(word, ".,!?;:\"'()[]{}#$%&*+-/<>=@\\^_`|~")
		if len(cleaned) > 0 {
			tokens = append(tokens, cleaned)
		}
	}

	return tokens
}

// truncateText truncates text to a maximum length
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
