package rag

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingModel implements models.EmbeddingModel for testing
type MockEmbeddingModel struct {
	dim int
}

func (m *MockEmbeddingModel) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		embedding := make([]float32, m.dim)
		for j := range embedding {
			embedding[j] = float32(i+1) * 0.1 / float32(j+1)
		}
		result[i] = embedding
	}
	return result, nil
}

func (m *MockEmbeddingModel) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := m.Encode(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

func (m *MockEmbeddingModel) Dimensions() int {
	return m.dim
}

func (m *MockEmbeddingModel) Name() string {
	return "mock-embedding"
}

func (m *MockEmbeddingModel) MaxTokens() int {
	return 8192
}

func (m *MockEmbeddingModel) Provider() string {
	return "mock"
}

func (m *MockEmbeddingModel) Health(ctx context.Context) error {
	return nil
}

func (m *MockEmbeddingModel) Close() error {
	return nil
}

// MockDocumentGenerator implements DocumentGenerator for testing
type MockDocumentGenerator struct {
	responses []string
	index     int
}

func (m *MockDocumentGenerator) Generate(ctx context.Context, prompt string, maxTokens int, temperature float32) (string, error) {
	if m.index >= len(m.responses) {
		m.index = 0
	}
	response := m.responses[m.index]
	m.index++
	return response, nil
}

// MockVectorDBForHyDE implements VectorDB for testing
type MockVectorDBForHyDE struct {
	results []SearchResult
}

func (m *MockVectorDBForHyDE) SimilaritySearchByVector(ctx context.Context, embedding []float32, topK int) ([]SearchResult, error) {
	if topK > len(m.results) {
		return m.results, nil
	}
	return m.results[:topK], nil
}

func TestDefaultHyDEConfig(t *testing.T) {
	config := DefaultHyDEConfig()

	assert.Equal(t, 3, config.NumHypotheses)
	assert.Equal(t, "default", config.TemplateName)
	assert.Equal(t, 200, config.MaxTokens)
	assert.Equal(t, float32(0.7), config.Temperature)
	assert.Equal(t, HyDEAggregateMean, config.AggregationMethod)
}

func TestDefaultHyDETemplates(t *testing.T) {
	templates := DefaultHyDETemplates()

	assert.Contains(t, templates, "default")
	assert.Contains(t, templates, "technical")
	assert.Contains(t, templates, "scientific")
	assert.Contains(t, templates, "code")
	assert.Contains(t, templates, "qa")

	// Verify templates contain the placeholder
	for name, template := range templates {
		assert.Contains(t, template.Template, "{{QUERY}}", "Template %s should contain {{QUERY}}", name)
	}
}

func TestNewHyDEGenerator(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 384}
	docGen := &MockDocumentGenerator{responses: []string{"test"}}

	generator := NewHyDEGenerator(config, embeddingModel, docGen, logrus.New())

	assert.NotNil(t, generator)
	assert.Equal(t, 3, generator.config.NumHypotheses)
	assert.NotNil(t, generator.templates)
	assert.True(t, generator.templates.Has("default"))
}

func TestNewHyDEGenerator_DefaultDocGen(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 384}

	// Pass nil document generator - should use SimpleDocumentGenerator
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	assert.NotNil(t, generator)
	assert.NotNil(t, generator.documentGen)
}

func TestNewHyDEGenerator_DefaultConfig(t *testing.T) {
	config := HyDEConfig{} // Empty config
	embeddingModel := &MockEmbeddingModel{dim: 384}

	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	assert.Equal(t, 3, generator.config.NumHypotheses)
	assert.Equal(t, 200, generator.config.MaxTokens)
	assert.Equal(t, float32(0.7), generator.config.Temperature)
	assert.Equal(t, HyDEAggregateMean, generator.config.AggregationMethod)
}

func TestHyDEGenerator_AddTemplate(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 384}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	customTemplate := HyDEPromptTemplate{
		Name:        "custom",
		Description: "Custom template",
		Template:    "Custom prompt: {{QUERY}}",
	}

	generator.AddTemplate(customTemplate)

	template, ok := generator.GetTemplate("custom")
	assert.True(t, ok)
	assert.Equal(t, "custom", template.Name)
	assert.Equal(t, "Custom prompt: {{QUERY}}", template.Template)
}

func TestHyDEGenerator_GetTemplate(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 384}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	// Get existing template
	template, ok := generator.GetTemplate("default")
	assert.True(t, ok)
	assert.Equal(t, "default", template.Name)

	// Get non-existent template
	_, ok = generator.GetTemplate("nonexistent")
	assert.False(t, ok)
}

func TestHyDEGenerator_GenerateHypotheticalDocuments(t *testing.T) {
	config := DefaultHyDEConfig()
	config.NumHypotheses = 3
	embeddingModel := &MockEmbeddingModel{dim: 384}
	docGen := &MockDocumentGenerator{
		responses: []string{
			"Hypothetical document 1 about machine learning",
			"Hypothetical document 2 about neural networks",
			"Hypothetical document 3 about deep learning",
		},
	}

	generator := NewHyDEGenerator(config, embeddingModel, docGen, logrus.New())

	docs, err := generator.GenerateHypotheticalDocuments(context.Background(), "What is machine learning?")
	require.NoError(t, err)

	assert.Len(t, docs, 3)
	assert.Equal(t, "Hypothetical document 1 about machine learning", docs[0])
}

func TestHyDEGenerator_GenerateHypotheticalDocuments_InvalidTemplate(t *testing.T) {
	config := DefaultHyDEConfig()
	config.TemplateName = "nonexistent"
	embeddingModel := &MockEmbeddingModel{dim: 384}
	generator := NewHyDEGenerator(config, embeddingModel, nil, logrus.New())

	_, err := generator.GenerateHypotheticalDocuments(context.Background(), "test query")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}

func TestHyDEGenerator_ExpandQuery(t *testing.T) {
	config := DefaultHyDEConfig()
	config.NumHypotheses = 2
	embeddingModel := &MockEmbeddingModel{dim: 4}
	docGen := &MockDocumentGenerator{
		responses: []string{
			"Document about Python programming",
			"Document about coding languages",
		},
	}

	generator := NewHyDEGenerator(config, embeddingModel, docGen, logrus.New())

	result, err := generator.ExpandQuery(context.Background(), "What is Python?")
	require.NoError(t, err)

	assert.Equal(t, "What is Python?", result.OriginalQuery)
	assert.Len(t, result.HypotheticalDocuments, 2)
	assert.Len(t, result.OriginalEmbedding, 4)
	assert.Len(t, result.HypotheticalEmbeddings, 2)
	assert.Len(t, result.AggregatedEmbedding, 4)
}

func TestHyDEGenerator_AggregationMethods(t *testing.T) {
	embeddingModel := &MockEmbeddingModel{dim: 4}
	docGen := &MockDocumentGenerator{responses: []string{"doc1", "doc2"}}

	testCases := []struct {
		method HyDEAggregation
		name   string
	}{
		{HyDEAggregateMean, "mean"},
		{HyDEAggregateMax, "max"},
		{HyDEAggregateWeighted, "weighted"},
		{HyDEAggregateConcat, "concat"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultHyDEConfig()
			config.NumHypotheses = 2
			config.AggregationMethod = tc.method

			generator := NewHyDEGenerator(config, embeddingModel, docGen, logrus.New())

			result, err := generator.ExpandQuery(context.Background(), "test query")
			require.NoError(t, err)

			assert.NotNil(t, result.AggregatedEmbedding)
			assert.Len(t, result.AggregatedEmbedding, 4)
		})
	}
}

func TestHyDEGenerator_AggregateMean(t *testing.T) {
	config := DefaultHyDEConfig()
	config.AggregationMethod = HyDEAggregateMean
	embeddingModel := &MockEmbeddingModel{dim: 4}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	original := []float32{1.0, 2.0, 3.0, 4.0}
	hypothetical := [][]float32{
		{2.0, 3.0, 4.0, 5.0},
		{3.0, 4.0, 5.0, 6.0},
	}

	result, err := generator.aggregateMean(original, hypothetical, 4)
	require.NoError(t, err)

	// Mean of [1,2,3,4], [2,3,4,5], [3,4,5,6] = [2, 3, 4, 5]
	assert.InDelta(t, 2.0, result[0], 0.01)
	assert.InDelta(t, 3.0, result[1], 0.01)
	assert.InDelta(t, 4.0, result[2], 0.01)
	assert.InDelta(t, 5.0, result[3], 0.01)
}

func TestHyDEGenerator_AggregateMax(t *testing.T) {
	config := DefaultHyDEConfig()
	config.AggregationMethod = HyDEAggregateMax
	embeddingModel := &MockEmbeddingModel{dim: 4}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	original := []float32{1.0, 5.0, 3.0, 4.0}
	hypothetical := [][]float32{
		{2.0, 3.0, 4.0, 5.0},
		{3.0, 4.0, 6.0, 2.0},
	}

	result, err := generator.aggregateMax(original, hypothetical, 4)
	require.NoError(t, err)

	// Max of [1,5,3,4], [2,3,4,5], [3,4,6,2] = [3, 5, 6, 5]
	assert.Equal(t, float32(3.0), result[0])
	assert.Equal(t, float32(5.0), result[1])
	assert.Equal(t, float32(6.0), result[2])
	assert.Equal(t, float32(5.0), result[3])
}

func TestHyDEGenerator_AggregateWeighted(t *testing.T) {
	config := DefaultHyDEConfig()
	config.AggregationMethod = HyDEAggregateWeighted
	embeddingModel := &MockEmbeddingModel{dim: 4}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	original := []float32{1.0, 2.0, 3.0, 4.0}
	hypothetical := [][]float32{
		{3.0, 4.0, 5.0, 6.0},
		{3.0, 4.0, 5.0, 6.0},
	}

	result, err := generator.aggregateWeighted(original, hypothetical, 4)
	require.NoError(t, err)

	// Original weight 0.5, each hypo weight 0.25
	// [1*0.5 + 3*0.25 + 3*0.25, 2*0.5 + 4*0.25 + 4*0.25, ...] = [2, 3, 4, 5]
	assert.InDelta(t, 2.0, result[0], 0.01)
	assert.InDelta(t, 3.0, result[1], 0.01)
	assert.InDelta(t, 4.0, result[2], 0.01)
	assert.InDelta(t, 5.0, result[3], 0.01)
}

func TestHyDEGenerator_AggregateDimensionMismatch(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 4}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	original := []float32{1.0, 2.0, 3.0, 4.0}
	hypothetical := [][]float32{
		{2.0, 3.0}, // Wrong dimension
	}

	_, err := generator.aggregateMean(original, hypothetical, 4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dimension mismatch")
}

func TestHyDEGenerator_HyDESearch(t *testing.T) {
	config := DefaultHyDEConfig()
	config.NumHypotheses = 2
	embeddingModel := &MockEmbeddingModel{dim: 4}
	docGen := &MockDocumentGenerator{
		responses: []string{"doc1", "doc2"},
	}
	vectorDB := &MockVectorDBForHyDE{
		results: []SearchResult{
			{Document: &Document{ID: "1", Content: "Result 1"}, Score: 0.9},
			{Document: &Document{ID: "2", Content: "Result 2"}, Score: 0.8},
		},
	}

	generator := NewHyDEGenerator(config, embeddingModel, docGen, logrus.New())

	result, err := generator.HyDESearch(context.Background(), "test query", vectorDB, 2)
	require.NoError(t, err)

	assert.Equal(t, "test query", result.OriginalQuery)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, 0.9, result.Results[0].Score)
	assert.Equal(t, 2, result.TopK)
}

func TestHyDEGenerator_SetConfig(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 4}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	newConfig := HyDEConfig{
		NumHypotheses:     5,
		TemplateName:      "technical",
		MaxTokens:         300,
		Temperature:       0.5,
		AggregationMethod: HyDEAggregateMax,
	}

	generator.SetConfig(newConfig)

	currentConfig := generator.GetConfig()
	assert.Equal(t, 5, currentConfig.NumHypotheses)
	assert.Equal(t, "technical", currentConfig.TemplateName)
	assert.Equal(t, HyDEAggregateMax, currentConfig.AggregationMethod)
}

func TestHyDEGenerator_SetTemplate(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 4}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	// Set existing template
	err := generator.SetTemplate("technical")
	assert.NoError(t, err)
	assert.Equal(t, "technical", generator.GetConfig().TemplateName)

	// Set non-existent template
	err = generator.SetTemplate("nonexistent")
	assert.Error(t, err)
}

func TestHyDEGenerator_ListTemplates(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 4}
	generator := NewHyDEGenerator(config, embeddingModel, nil, nil)

	templates := generator.ListTemplates()

	assert.GreaterOrEqual(t, len(templates), 5)
	assert.Contains(t, templates, "default")
	assert.Contains(t, templates, "technical")
}

func TestSimpleDocumentGenerator_Generate(t *testing.T) {
	gen := &SimpleDocumentGenerator{}

	// Test with different prompt formats
	testCases := []struct {
		prompt string
		name   string
	}{
		{
			prompt: "Question: What is machine learning?\nAnswer:",
			name:   "question_format",
		},
		{
			prompt: "Topic: Neural networks\nDocumentation:",
			name:   "topic_format",
		},
		{
			prompt: "Query: Deep learning basics\nResponse:",
			name:   "query_format",
		},
		{
			prompt: "Simple query without format",
			name:   "no_format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := gen.Generate(context.Background(), tc.prompt, 200, 0.7)
			require.NoError(t, err)
			assert.NotEmpty(t, result)
			assert.Contains(t, result, "This document discusses")
		})
	}
}

func TestHyDEGenerator_EmptyHypotheticalResults(t *testing.T) {
	config := DefaultHyDEConfig()
	config.NumHypotheses = 3
	embeddingModel := &MockEmbeddingModel{dim: 4}
	docGen := &MockDocumentGenerator{
		responses: []string{"", "", ""}, // All empty responses
	}

	generator := NewHyDEGenerator(config, embeddingModel, docGen, logrus.New())

	_, err := generator.GenerateHypotheticalDocuments(context.Background(), "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate any hypothetical documents")
}

func TestHyDEGenerator_ConcurrencySafety(t *testing.T) {
	config := DefaultHyDEConfig()
	embeddingModel := &MockEmbeddingModel{dim: 4}
	docGen := &MockDocumentGenerator{responses: []string{"doc1", "doc2", "doc3"}}
	generator := NewHyDEGenerator(config, embeddingModel, docGen, logrus.New())

	// Run concurrent operations
	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func() {
			_, _ = generator.GetTemplate("default")
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		go func(i int) {
			generator.AddTemplate(HyDEPromptTemplate{
				Name:     "concurrent_" + string(rune('A'+i)),
				Template: "{{QUERY}}",
			})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panic occurred and templates were added
	templates := generator.ListTemplates()
	assert.GreaterOrEqual(t, len(templates), 5)
}
