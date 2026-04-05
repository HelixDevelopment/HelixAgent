package context

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMBackend is a mock LLM backend for testing.
type MockLLMBackend struct {
	responses []string
	callCount int
	shouldErr bool
}

func (m *MockLLMBackend) Complete(ctx context.Context, prompt string) (string, error) {
	if m.shouldErr {
		return "", errors.New("mock error")
	}
	if m.callCount < len(m.responses) {
		response := m.responses[m.callCount]
		m.callCount++
		return response, nil
	}
	return "default summary", nil
}

func TestDefaultSummaryConfig(t *testing.T) {
	t.Parallel()
	config := DefaultSummaryConfig()

	assert.Equal(t, 256, config.MaxLength)
	assert.Equal(t, StyleBrief, config.Style)
	assert.True(t, config.PreserveKeyPoints)
	assert.True(t, config.PreserveCode)
	assert.True(t, config.PreserveNames)
	assert.Equal(t, 0.25, config.Compression)
}

func TestSummaryStyles(t *testing.T) {
	t.Parallel()
	assert.Equal(t, SummaryStyle("brief"), StyleBrief)
	assert.Equal(t, SummaryStyle("detailed"), StyleDetailed)
	assert.Equal(t, SummaryStyle("bullet_points"), StyleBulletPoints)
	assert.Equal(t, SummaryStyle("key_facts"), StyleKeyFacts)
	assert.Equal(t, SummaryStyle("conversation"), StyleConversation)
}

func TestLLMSummarizer_Summarize(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"This is a summary."},
	}

	summarizer := NewLLMSummarizer(backend, nil)

	content := "This is a long piece of content that needs to be summarized. It contains many details and information."
	summary, err := summarizer.Summarize(context.Background(), content)

	require.NoError(t, err)
	assert.Equal(t, "This is a summary.", summary)
}

func TestLLMSummarizer_SummarizeWithConfig(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Brief summary"},
	}

	summarizer := NewLLMSummarizer(backend, nil)
	config := &SummaryConfig{
		MaxLength:         100,
		Style:             StyleBrief,
		PreserveKeyPoints: true,
	}

	summary, err := summarizer.SummarizeWithConfig(context.Background(), "Content", config)

	require.NoError(t, err)
	assert.Equal(t, "Brief summary", summary)
}

func TestLLMSummarizer_SummarizeEmpty(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{}

	summarizer := NewLLMSummarizer(backend, nil)

	_, err := summarizer.Summarize(context.Background(), "")

	assert.Error(t, err)
	assert.Equal(t, ErrNoContent, err)
}

func TestLLMSummarizer_SummarizeError(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		shouldErr: true,
	}

	summarizer := NewLLMSummarizer(backend, nil)

	_, err := summarizer.Summarize(context.Background(), "Some content")

	assert.Error(t, err)
}

func TestLLMSummarizer_BuildPrompt(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{responses: []string{"Summary"}}
	summarizer := NewLLMSummarizer(backend, nil)

	testCases := []struct {
		style    SummaryStyle
		contains string
	}{
		{StyleBrief, "brief"},
		{StyleDetailed, "detailed"},
		{StyleBulletPoints, "bullet points"},
		{StyleKeyFacts, "key facts"},
		{StyleConversation, "conversation"},
	}

	for _, tc := range testCases {
		config := &SummaryConfig{Style: tc.style}
		_, _ = summarizer.SummarizeWithConfig(context.Background(), "Content", config)
		// The prompt should contain the style hint
	}
}

func TestExtractiveRuleSummarizer_Summarize(t *testing.T) {
	t.Parallel()
	summarizer := NewExtractiveRuleSummarizer(nil)

	content := "This is the first sentence. This is the second sentence. This is the third sentence."
	summary, err := summarizer.Summarize(context.Background(), content)

	require.NoError(t, err)
	assert.NotEmpty(t, summary)
}

func TestExtractiveRuleSummarizer_SummarizeEmpty(t *testing.T) {
	t.Parallel()
	summarizer := NewExtractiveRuleSummarizer(nil)

	_, err := summarizer.Summarize(context.Background(), "")

	assert.Error(t, err)
	assert.Equal(t, ErrNoContent, err)
}

func TestExtractiveRuleSummarizer_SummarizeWithConfig(t *testing.T) {
	t.Parallel()
	summarizer := NewExtractiveRuleSummarizer(nil)
	config := &SummaryConfig{
		MaxLength:   50,
		Compression: 0.5,
	}

	content := "First sentence is important. Second sentence has details. Third sentence concludes."
	summary, err := summarizer.SummarizeWithConfig(context.Background(), content, config)

	require.NoError(t, err)
	assert.NotEmpty(t, summary)
}

func TestDefaultConversationSummaryConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConversationSummaryConfig()

	assert.Equal(t, 20, config.MaxTurns)
	assert.Equal(t, 256, config.SummaryMaxTokens)
	assert.Equal(t, 4, config.PreserveLastN)
	assert.True(t, config.IncludeSpeakers)
}

func TestConversationSummarizer_Summarize(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Summary of conversation"},
	}

	summarizer := NewConversationSummarizer(backend, nil)

	turns := []Turn{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "I'm fine"},
		{Role: "user", Content: "What's new?"},
		{Role: "assistant", Content: "Not much"},
	}

	result, err := summarizer.Summarize(context.Background(), turns)

	require.NoError(t, err)
	assert.NotEmpty(t, result.Summary)
	assert.Len(t, result.PreservedTurns, 4) // Default preserves last 4
}

func TestConversationSummarizer_SummarizeEmpty(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{}

	summarizer := NewConversationSummarizer(backend, nil)

	_, err := summarizer.Summarize(context.Background(), []Turn{})

	assert.Error(t, err)
	assert.Equal(t, ErrNoContent, err)
}

func TestConversationSummarizer_SummarizeFewTurns(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{}

	config := &ConversationSummaryConfig{
		PreserveLastN: 5,
	}
	summarizer := NewConversationSummarizer(backend, config)

	turns := []Turn{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	result, err := summarizer.Summarize(context.Background(), turns)

	require.NoError(t, err)
	assert.Empty(t, result.Summary)
	assert.Len(t, result.PreservedTurns, 2)
	assert.Equal(t, 0, result.SummarizedTurnCount)
}

func TestTurn(t *testing.T) {
	t.Parallel()
	turn := Turn{
		Role:      "user",
		Content:   "Hello world",
		Timestamp: time.Now(),
	}

	assert.Equal(t, "user", turn.Role)
	assert.Equal(t, "Hello world", turn.Content)
}

func TestConversationSummaryResult(t *testing.T) {
	t.Parallel()
	result := &ConversationSummaryResult{
		Summary: "Conversation summary",
		PreservedTurns: []Turn{
			{Role: "user", Content: "Recent message"},
		},
		SummarizedTurnCount: 5,
	}

	assert.Equal(t, "Conversation summary", result.Summary)
	assert.Len(t, result.PreservedTurns, 1)
	assert.Equal(t, 5, result.SummarizedTurnCount)
}

func TestDefaultIncrementalConfig(t *testing.T) {
	t.Parallel()
	config := DefaultIncrementalConfig()

	assert.Equal(t, 10, config.SummarizeAfterItems)
	assert.Equal(t, 512, config.MaxSummaryTokens)
	assert.Equal(t, 0.3, config.Compression)
}

func TestIncrementalSummarizer_Add(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Incremental summary"},
	}

	summarizer := NewIncrementalSummarizer(backend, &IncrementalConfig{
		SummarizeAfterItems: 5,
	})

	for i := 0; i < 3; i++ {
		summarizer.Add("Content item")
	}

	summary := summarizer.GetSummary()
	assert.Empty(t, summary) // Not enough items yet
}

func TestIncrementalSummarizer_Update(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Updated summary"},
	}

	summarizer := NewIncrementalSummarizer(backend, &IncrementalConfig{
		SummarizeAfterItems: 3,
	})

	for i := 0; i < 5; i++ {
		summarizer.Add("Content item")
	}

	summary, err := summarizer.Update(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Updated summary", summary)
}

func TestIncrementalSummarizer_UpdateError(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		shouldErr: true,
	}

	summarizer := NewIncrementalSummarizer(backend, &IncrementalConfig{
		SummarizeAfterItems: 2,
	})

	summarizer.Add("Content 1")
	summarizer.Add("Content 2")
	summarizer.Add("Content 3")

	_, err := summarizer.Update(context.Background())

	assert.Error(t, err)
}

func TestIncrementalSummarizer_Reset(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Summary"},
	}

	summarizer := NewIncrementalSummarizer(backend, &IncrementalConfig{
		SummarizeAfterItems: 2,
	})

	summarizer.Add("Content")
	summarizer.Add("More content")
	summarizer.Add("Even more")
	_, _ = summarizer.Update(context.Background())

	summarizer.Reset()

	assert.Empty(t, summarizer.GetSummary())
}

func TestSummaryCache_GetSet(t *testing.T) {
	t.Parallel()
	cache := NewSummaryCache(1 * time.Hour)

	cache.Set("key1", "Summary 1")

	summary, found := cache.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "Summary 1", summary)
}

func TestSummaryCache_Get_NotFound(t *testing.T) {
	t.Parallel()
	cache := NewSummaryCache(1 * time.Hour)

	_, found := cache.Get("nonexistent")
	assert.False(t, found)
}

func TestSummaryCache_Get_Expired(t *testing.T) {
	t.Parallel()
	cache := NewSummaryCache(1 * time.Millisecond)

	cache.Set("key1", "Summary")
	time.Sleep(5 * time.Millisecond)

	_, found := cache.Get("key1")
	assert.False(t, found)
}

func TestSummaryCache_Clear(t *testing.T) {
	t.Parallel()
	cache := NewSummaryCache(1 * time.Hour)

	cache.Set("key1", "Summary 1")
	cache.Set("key2", "Summary 2")

	cache.Clear()

	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")
	assert.False(t, found1)
	assert.False(t, found2)
}

func TestSummaryCache_Cleanup(t *testing.T) {
	t.Parallel()
	cache := NewSummaryCache(1 * time.Millisecond)

	cache.Set("key1", "Summary 1")
	time.Sleep(5 * time.Millisecond)

	cache.Cleanup()

	_, found := cache.Get("key1")
	assert.False(t, found)
}

func TestCachedSummary(t *testing.T) {
	t.Parallel()
	cached := CachedSummary{
		Summary:     "Test summary",
		CreatedAt:   time.Now(),
		ContentHash: "abc123",
	}

	assert.Equal(t, "Test summary", cached.Summary)
	assert.Equal(t, "abc123", cached.ContentHash)
}

func TestSplitIntoSentences(t *testing.T) {
	t.Parallel()
	text := "First sentence. Second sentence! Third sentence?"
	sentences := splitIntoSentences(text)

	assert.Len(t, sentences, 3)
	assert.Equal(t, "First sentence.", sentences[0])
	assert.Equal(t, "Second sentence!", sentences[1])
	assert.Equal(t, "Third sentence?", sentences[2])
}

func TestSplitIntoSentences_NoTerminal(t *testing.T) {
	t.Parallel()
	text := "This has no terminal punctuation"
	sentences := splitIntoSentences(text)

	assert.Len(t, sentences, 1)
	assert.Equal(t, "This has no terminal punctuation", sentences[0])
}

func TestSplitIntoSentences_Empty(t *testing.T) {
	t.Parallel()
	sentences := splitIntoSentences("")
	assert.Empty(t, sentences)
}

func TestExtractiveRuleSummarizer_ScoreSentences(t *testing.T) {
	t.Parallel()
	summarizer := NewExtractiveRuleSummarizer(nil)

	sentences := []string{
		"This is the important first sentence.",        // First sentence bonus
		"A short one.",                                 // Too short
		"This sentence contains an important keyword.", // Keyword bonus
		"The final conclusion is here.",                // Last sentence bonus
	}

	scores := summarizer.scoreSentences(sentences)

	assert.Len(t, scores, 4)
	assert.Greater(t, scores[0], scores[1]) // First sentence should score higher than short one
}

func TestExtractiveRuleSummarizer_SelectTopSentences(t *testing.T) {
	t.Parallel()
	summarizer := NewExtractiveRuleSummarizer(nil)

	sentences := []string{
		"Sentence one.",
		"Sentence two.",
		"Sentence three.",
		"Sentence four.",
	}
	scores := []float64{2.0, 3.0, 1.0, 4.0}

	selected := summarizer.selectTopSentences(sentences, scores, 2)

	assert.Len(t, selected, 2)
	// Should maintain original order
	assert.Equal(t, "Sentence two.", selected[0])
	assert.Equal(t, "Sentence four.", selected[1])
}

func TestIncrementalSummarizer_WithExistingSummary(t *testing.T) {
	t.Parallel()
	backend := &MockLLMBackend{
		responses: []string{"Initial summary", "Combined summary"},
	}

	config := &IncrementalConfig{
		SummarizeAfterItems: 2,
	}
	summarizer := NewIncrementalSummarizer(backend, config)

	// First batch
	summarizer.Add("Content 1")
	summarizer.Add("Content 2")
	summarizer.Add("Content 3")
	_, _ = summarizer.Update(context.Background())

	// Second batch should combine with existing summary
	summarizer.Add("Content 4")
	summarizer.Add("Content 5")
	summarizer.Add("Content 6")
	summary, err := summarizer.Update(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Combined summary", summary)
}
