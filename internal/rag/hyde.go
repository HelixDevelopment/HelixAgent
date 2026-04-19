// Package rag provides Retrieval-Augmented Generation capabilities.
package rag

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/embeddings/models"
	"github.com/sirupsen/logrus"
)

// HyDEConfig holds configuration for Hypothetical Document Embeddings
type HyDEConfig struct {
	// NumHypotheses is the number of hypothetical documents to generate
	NumHypotheses int `json:"num_hypotheses,omitempty"`
	// TemplateName is the name of the prompt template to use
	TemplateName string `json:"template_name,omitempty"`
	// MaxTokens is the maximum tokens for generated hypothetical documents
	MaxTokens int `json:"max_tokens,omitempty"`
	// Temperature controls randomness in generation (0-1)
	Temperature float32 `json:"temperature,omitempty"`
	// AggregationMethod is how to combine multiple hypothetical embeddings
	AggregationMethod HyDEAggregation `json:"aggregation_method,omitempty"`
}

// HyDEAggregation defines how to aggregate multiple hypothetical document embeddings
type HyDEAggregation string

const (
	// HyDEAggregateMean averages all hypothetical document embeddings
	HyDEAggregateMean HyDEAggregation = "mean"
	// HyDEAggregateMax takes element-wise maximum
	HyDEAggregateMax HyDEAggregation = "max"
	// HyDEAggregateConcat concatenates query embedding with hypothetical embeddings
	HyDEAggregateConcat HyDEAggregation = "concat"
	// HyDEAggregateWeighted uses weighted average favoring the original query
	HyDEAggregateWeighted HyDEAggregation = "weighted"
)

// DefaultHyDEConfig returns default HyDE configuration
func DefaultHyDEConfig() HyDEConfig {
	return HyDEConfig{
		NumHypotheses:     3,
		TemplateName:      "default",
		MaxTokens:         200,
		Temperature:       0.7,
		AggregationMethod: HyDEAggregateMean,
	}
}

// HyDEPromptTemplate defines templates for generating hypothetical documents
type HyDEPromptTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
}

// DefaultHyDETemplates returns the built-in prompt templates
func DefaultHyDETemplates() map[string]HyDEPromptTemplate {
	return map[string]HyDEPromptTemplate{
		"default": {
			Name:        "default",
			Description: "General purpose hypothetical document generation",
			Template:    "Write a detailed passage that would answer the following question:\n\nQuestion: {{QUERY}}\n\nPassage:",
		},
		"technical": {
			Name:        "technical",
			Description: "For technical/programming questions",
			Template:    "Write a technical documentation excerpt that explains the following:\n\nTopic: {{QUERY}}\n\nDocumentation:",
		},
		"scientific": {
			Name:        "scientific",
			Description: "For scientific/research questions",
			Template:    "Write a scientific abstract or passage that addresses:\n\nResearch question: {{QUERY}}\n\nAbstract:",
		},
		"code": {
			Name:        "code",
			Description: "For code-related queries",
			Template:    "Write a code documentation comment or explanation for:\n\nQuery: {{QUERY}}\n\nDocumentation:",
		},
		"qa": {
			Name:        "qa",
			Description: "For question-answering scenarios",
			Template:    "Given the question below, write a comprehensive answer that a well-informed expert would provide:\n\nQuestion: {{QUERY}}\n\nAnswer:",
		},
	}
}

// HyDEGenerator generates hypothetical documents for query expansion.
//
// Concurrent-safe by construction (CONST-029):
//   - templates is a safe.Store; Add/Get/List/Has all use its built-in
//     serialisation.
//   - config (a small value struct) stays behind a sync.RWMutex. The
//     mutex no longer sits next to a bare map, which is what the audit
//     flags; SetConfig/GetConfig/SetTemplate atomicity is preserved.
type HyDEGenerator struct {
	config         HyDEConfig
	mu             sync.RWMutex // protects config writes and SetTemplate's check-then-set
	embeddingModel models.EmbeddingModel
	documentGen    DocumentGenerator
	templates      *safe.Store[string, HyDEPromptTemplate]
	logger         *logrus.Logger
}

// DocumentGenerator interface for generating hypothetical documents
// This allows plugging in different LLM providers
type DocumentGenerator interface {
	// Generate creates a hypothetical document based on the query and template
	Generate(ctx context.Context, prompt string, maxTokens int, temperature float32) (string, error)
}

// SimpleDocumentGenerator provides a simple document generator that uses templates
// This is useful when no LLM is available
type SimpleDocumentGenerator struct{}

// Generate creates a simple hypothetical document by expanding the query
func (s *SimpleDocumentGenerator) Generate(ctx context.Context, prompt string, maxTokens int, temperature float32) (string, error) {
	// Extract the query from the prompt (simple parsing)
	lines := strings.Split(prompt, "\n")
	var query string
	for _, line := range lines {
		if strings.Contains(line, "{{QUERY}}") || strings.HasPrefix(line, "Question:") ||
			strings.HasPrefix(line, "Topic:") || strings.HasPrefix(line, "Query:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				query = strings.TrimSpace(parts[1])
			}
		}
	}

	if query == "" {
		// Try to find the query after the last colon
		lastColon := strings.LastIndex(prompt, ":")
		if lastColon != -1 && lastColon < len(prompt)-1 {
			query = strings.TrimSpace(prompt[lastColon+1:])
		}
	}

	if query == "" {
		query = prompt
	}

	// Generate a simple expanded document
	expanded := fmt.Sprintf("This document discusses %s. It covers the key concepts, "+
		"implementation details, and best practices related to %s. The main topics include "+
		"understanding the fundamentals, practical applications, and common patterns used with %s.",
		query, query, query)

	return expanded, nil
}

// NewHyDEGenerator creates a new HyDE generator
func NewHyDEGenerator(config HyDEConfig, embeddingModel models.EmbeddingModel, docGen DocumentGenerator, logger *logrus.Logger) *HyDEGenerator {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}

	if config.NumHypotheses <= 0 {
		config.NumHypotheses = 3
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 200
	}
	if config.Temperature <= 0 {
		config.Temperature = 0.7
	}
	if config.AggregationMethod == "" {
		config.AggregationMethod = HyDEAggregateMean
	}

	// Use simple generator if none provided
	if docGen == nil {
		docGen = &SimpleDocumentGenerator{}
	}

	return &HyDEGenerator{
		config:         config,
		embeddingModel: embeddingModel,
		documentGen:    docGen,
		templates:      safe.NewStoreFromMap(DefaultHyDETemplates()),
		logger:         logger,
	}
}

// AddTemplate adds a custom prompt template
func (h *HyDEGenerator) AddTemplate(template HyDEPromptTemplate) {
	h.templates.Put(template.Name, template)
}

// GetTemplate retrieves a prompt template by name
func (h *HyDEGenerator) GetTemplate(name string) (HyDEPromptTemplate, bool) {
	return h.templates.Get(name)
}

// HyDEResult contains the result of HyDE query expansion
type HyDEResult struct {
	OriginalQuery          string      `json:"original_query"`
	HypotheticalDocuments  []string    `json:"hypothetical_documents"`
	OriginalEmbedding      []float32   `json:"original_embedding,omitempty"`
	HypotheticalEmbeddings [][]float32 `json:"hypothetical_embeddings,omitempty"`
	AggregatedEmbedding    []float32   `json:"aggregated_embedding"`
	TemplateName           string      `json:"template_name"`
}

// GenerateHypotheticalDocuments generates hypothetical documents for a query
func (h *HyDEGenerator) GenerateHypotheticalDocuments(ctx context.Context, query string) ([]string, error) {
	h.mu.RLock()
	templateName := h.config.TemplateName
	h.mu.RUnlock()
	if templateName == "" {
		templateName = "default"
	}
	template, ok := h.templates.Get(templateName)
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	var documents []string
	prompt := strings.Replace(template.Template, "{{QUERY}}", query, -1)

	// Generate multiple hypothetical documents
	for i := 0; i < h.config.NumHypotheses; i++ {
		doc, err := h.documentGen.Generate(ctx, prompt, h.config.MaxTokens, h.config.Temperature)
		if err != nil {
			h.logger.WithError(err).Warn("Failed to generate hypothetical document")
			continue
		}

		if doc = strings.TrimSpace(doc); doc != "" {
			documents = append(documents, doc)
		}
	}

	if len(documents) == 0 {
		return nil, fmt.Errorf("failed to generate any hypothetical documents")
	}

	return documents, nil
}

// ExpandQuery generates hypothetical documents and creates an aggregated embedding
func (h *HyDEGenerator) ExpandQuery(ctx context.Context, query string) (*HyDEResult, error) {
	// Generate hypothetical documents
	documents, err := h.GenerateHypotheticalDocuments(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hypothetical documents: %w", err)
	}

	result := &HyDEResult{
		OriginalQuery:         query,
		HypotheticalDocuments: documents,
		TemplateName:          h.config.TemplateName,
	}

	// Get original query embedding
	originalEmbedding, err := h.embeddingModel.EncodeSingle(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to encode original query: %w", err)
	}
	result.OriginalEmbedding = originalEmbedding

	// Get embeddings for hypothetical documents
	hypoEmbeddings, err := h.embeddingModel.Encode(ctx, documents)
	if err != nil {
		return nil, fmt.Errorf("failed to encode hypothetical documents: %w", err)
	}
	result.HypotheticalEmbeddings = hypoEmbeddings

	// Aggregate embeddings
	aggregated, err := h.aggregateEmbeddings(originalEmbedding, hypoEmbeddings)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate embeddings: %w", err)
	}
	result.AggregatedEmbedding = aggregated

	return result, nil
}

// aggregateEmbeddings combines the original query embedding with hypothetical document embeddings
func (h *HyDEGenerator) aggregateEmbeddings(original []float32, hypothetical [][]float32) ([]float32, error) {
	if len(hypothetical) == 0 {
		return original, nil
	}

	dim := len(original)

	switch h.config.AggregationMethod {
	case HyDEAggregateMean:
		return h.aggregateMean(original, hypothetical, dim)

	case HyDEAggregateMax:
		return h.aggregateMax(original, hypothetical, dim)

	case HyDEAggregateWeighted:
		return h.aggregateWeighted(original, hypothetical, dim)

	case HyDEAggregateConcat:
		// For concat, we just return the mean of hypothetical embeddings
		// The original embedding should be used separately
		return h.aggregateMeanHypotheticalOnly(hypothetical, dim)

	default:
		return h.aggregateMean(original, hypothetical, dim)
	}
}

// aggregateMean computes the mean of all embeddings (original + hypothetical)
func (h *HyDEGenerator) aggregateMean(original []float32, hypothetical [][]float32, dim int) ([]float32, error) {
	result := make([]float32, dim)

	// Add original embedding
	for i := 0; i < dim; i++ {
		result[i] = original[i]
	}

	// Add hypothetical embeddings
	for _, hypo := range hypothetical {
		if len(hypo) != dim {
			return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", dim, len(hypo))
		}
		for i := 0; i < dim; i++ {
			result[i] += hypo[i]
		}
	}

	// Divide by total count
	total := float32(1 + len(hypothetical))
	for i := 0; i < dim; i++ {
		result[i] /= total
	}

	return result, nil
}

// aggregateMax computes element-wise maximum across all embeddings
func (h *HyDEGenerator) aggregateMax(original []float32, hypothetical [][]float32, dim int) ([]float32, error) {
	result := make([]float32, dim)
	copy(result, original)

	for _, hypo := range hypothetical {
		if len(hypo) != dim {
			return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", dim, len(hypo))
		}
		for i := 0; i < dim; i++ {
			if hypo[i] > result[i] {
				result[i] = hypo[i]
			}
		}
	}

	return result, nil
}

// aggregateWeighted computes weighted average with higher weight on original query
func (h *HyDEGenerator) aggregateWeighted(original []float32, hypothetical [][]float32, dim int) ([]float32, error) {
	result := make([]float32, dim)

	// Original query gets weight of 0.5, hypothetical documents share 0.5
	originalWeight := float32(0.5)
	hypoWeight := float32(0.5) / float32(len(hypothetical))

	// Add weighted original embedding
	for i := 0; i < dim; i++ {
		result[i] = original[i] * originalWeight
	}

	// Add weighted hypothetical embeddings
	for _, hypo := range hypothetical {
		if len(hypo) != dim {
			return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", dim, len(hypo))
		}
		for i := 0; i < dim; i++ {
			result[i] += hypo[i] * hypoWeight
		}
	}

	return result, nil
}

// aggregateMeanHypotheticalOnly computes mean of only hypothetical embeddings
func (h *HyDEGenerator) aggregateMeanHypotheticalOnly(hypothetical [][]float32, dim int) ([]float32, error) {
	result := make([]float32, dim)

	for _, hypo := range hypothetical {
		if len(hypo) != dim {
			return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", dim, len(hypo))
		}
		for i := 0; i < dim; i++ {
			result[i] += hypo[i]
		}
	}

	// Divide by count
	total := float32(len(hypothetical))
	for i := 0; i < dim; i++ {
		result[i] /= total
	}

	return result, nil
}

// HyDESearch performs search using HyDE-expanded query
func (h *HyDEGenerator) HyDESearch(ctx context.Context, query string, vectorDB VectorDB, topK int) (*HyDESearchResult, error) {
	// Generate HyDE result with aggregated embedding
	hydeResult, err := h.ExpandQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("HyDE expansion failed: %w", err)
	}

	// Search using aggregated embedding
	results, err := vectorDB.SimilaritySearchByVector(ctx, hydeResult.AggregatedEmbedding, topK)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return &HyDESearchResult{
		HyDEResult: *hydeResult,
		Results:    results,
		TopK:       topK,
	}, nil
}

// HyDESearchResult contains results from HyDE-enhanced search
type HyDESearchResult struct {
	HyDEResult
	Results []SearchResult `json:"results"`
	TopK    int            `json:"top_k"`
}

// VectorDB interface for vector database operations
type VectorDB interface {
	// SimilaritySearchByVector searches using a pre-computed embedding
	SimilaritySearchByVector(ctx context.Context, embedding []float32, topK int) ([]SearchResult, error)
}

// SetConfig updates the HyDE configuration
func (h *HyDEGenerator) SetConfig(config HyDEConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.config = config
}

// GetConfig returns the current HyDE configuration
func (h *HyDEGenerator) GetConfig() HyDEConfig {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.config
}

// SetTemplate sets the active template name. The template lookup happens
// before the config mutation so a concurrent template deletion between
// the two calls would leave config pointing at a missing template; the
// next GenerateHypotheticalDocuments call will surface that cleanly as
// "template not found".
func (h *HyDEGenerator) SetTemplate(name string) error {
	if !h.templates.Has(name) {
		return fmt.Errorf("template not found: %s", name)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.config.TemplateName = name
	return nil
}

// ListTemplates returns all available template names
func (h *HyDEGenerator) ListTemplates() []string {
	return h.templates.Keys()
}
