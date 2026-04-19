// Package context provides context window management for LLM interactions.
package context

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

var (
	// ErrSummarizationFailed indicates summarization failed.
	ErrSummarizationFailed = errors.New("summarization failed")
	// ErrNoContent indicates there is no content to summarize.
	ErrNoContent = errors.New("no content to summarize")
)

// Summarizer defines the interface for context summarization.
type Summarizer interface {
	// Summarize summarizes the given content.
	Summarize(ctx context.Context, content string) (string, error)
	// SummarizeWithConfig summarizes with specific configuration.
	SummarizeWithConfig(ctx context.Context, content string, config *SummaryConfig) (string, error)
}

// SummaryConfig holds configuration for summarization.
type SummaryConfig struct {
	// MaxLength is the maximum summary length in tokens.
	MaxLength int `json:"max_length"`
	// Style is the summarization style.
	Style SummaryStyle `json:"style"`
	// PreserveKeyPoints keeps key points intact.
	PreserveKeyPoints bool `json:"preserve_key_points"`
	// PreserveCode keeps code blocks intact.
	PreserveCode bool `json:"preserve_code"`
	// PreserveNames keeps names/entities intact.
	PreserveNames bool `json:"preserve_names"`
	// Compression is the target compression ratio (0-1).
	Compression float64 `json:"compression"`
}

// SummaryStyle defines summarization styles.
type SummaryStyle string

const (
	// StyleBrief creates a brief summary.
	StyleBrief SummaryStyle = "brief"
	// StyleDetailed creates a detailed summary.
	StyleDetailed SummaryStyle = "detailed"
	// StyleBulletPoints creates bullet points.
	StyleBulletPoints SummaryStyle = "bullet_points"
	// StyleKeyFacts extracts key facts.
	StyleKeyFacts SummaryStyle = "key_facts"
	// StyleConversation summarizes conversation.
	StyleConversation SummaryStyle = "conversation"
)

// DefaultSummaryConfig returns a default configuration.
func DefaultSummaryConfig() *SummaryConfig {
	return &SummaryConfig{
		MaxLength:         256,
		Style:             StyleBrief,
		PreserveKeyPoints: true,
		PreserveCode:      true,
		PreserveNames:     true,
		Compression:       0.25,
	}
}

// LLMBackend defines the interface for LLM-based summarization.
type LLMBackend interface {
	// Complete generates a completion.
	Complete(ctx context.Context, prompt string) (string, error)
}

// LLMSummarizer uses an LLM for summarization.
type LLMSummarizer struct {
	backend LLMBackend
	config  *SummaryConfig
}

// NewLLMSummarizer creates a new LLM-based summarizer.
func NewLLMSummarizer(backend LLMBackend, config *SummaryConfig) *LLMSummarizer {
	if config == nil {
		config = DefaultSummaryConfig()
	}
	return &LLMSummarizer{
		backend: backend,
		config:  config,
	}
}

// Summarize summarizes content using the LLM.
func (s *LLMSummarizer) Summarize(ctx context.Context, content string) (string, error) {
	return s.SummarizeWithConfig(ctx, content, s.config)
}

// SummarizeWithConfig summarizes with specific configuration.
func (s *LLMSummarizer) SummarizeWithConfig(ctx context.Context, content string, config *SummaryConfig) (string, error) {
	if content == "" {
		return "", ErrNoContent
	}

	prompt := s.buildPrompt(content, config)

	summary, err := s.backend.Complete(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSummarizationFailed, err)
	}

	return strings.TrimSpace(summary), nil
}

func (s *LLMSummarizer) buildPrompt(content string, config *SummaryConfig) string {
	var promptBuilder strings.Builder

	promptBuilder.WriteString("Summarize the following content")

	switch config.Style {
	case StyleBrief:
		promptBuilder.WriteString(" in a brief, concise manner")
	case StyleDetailed:
		promptBuilder.WriteString(" with important details preserved")
	case StyleBulletPoints:
		promptBuilder.WriteString(" as bullet points")
	case StyleKeyFacts:
		promptBuilder.WriteString(" extracting only the key facts")
	case StyleConversation:
		promptBuilder.WriteString(" capturing the main points of the conversation")
	}

	if config.MaxLength > 0 {
		promptBuilder.WriteString(fmt.Sprintf(". Keep the summary under %d words", config.MaxLength))
	}

	if config.PreserveCode {
		promptBuilder.WriteString(". Preserve any code snippets or technical details")
	}

	if config.PreserveNames {
		promptBuilder.WriteString(". Keep names and specific references intact")
	}

	promptBuilder.WriteString(":\n\n")
	promptBuilder.WriteString(content)
	promptBuilder.WriteString("\n\nSummary:")

	return promptBuilder.String()
}

// ExtractiveRuleSummarizer uses extractive summarization rules.
type ExtractiveRuleSummarizer struct {
	config *SummaryConfig
}

// NewExtractiveRuleSummarizer creates a new rule-based summarizer.
func NewExtractiveRuleSummarizer(config *SummaryConfig) *ExtractiveRuleSummarizer {
	if config == nil {
		config = DefaultSummaryConfig()
	}
	return &ExtractiveRuleSummarizer{config: config}
}

// Summarize summarizes content using extractive rules.
func (s *ExtractiveRuleSummarizer) Summarize(ctx context.Context, content string) (string, error) {
	return s.SummarizeWithConfig(ctx, content, s.config)
}

// SummarizeWithConfig summarizes with specific configuration.
func (s *ExtractiveRuleSummarizer) SummarizeWithConfig(ctx context.Context, content string, config *SummaryConfig) (string, error) {
	if content == "" {
		return "", ErrNoContent
	}

	// Split into sentences
	sentences := splitIntoSentences(content)
	if len(sentences) == 0 {
		return content, nil
	}

	// Score sentences
	scores := s.scoreSentences(sentences)

	// Select top sentences based on compression
	targetCount := int(float64(len(sentences)) * config.Compression)
	if targetCount < 1 {
		targetCount = 1
	}

	selected := s.selectTopSentences(sentences, scores, targetCount)

	// Join selected sentences
	summary := strings.Join(selected, " ")

	// Enforce max length
	if config.MaxLength > 0 {
		words := strings.Fields(summary)
		if len(words) > config.MaxLength {
			summary = strings.Join(words[:config.MaxLength], " ") + "..."
		}
	}

	return summary, nil
}

func (s *ExtractiveRuleSummarizer) scoreSentences(sentences []string) []float64 {
	scores := make([]float64, len(sentences))

	// Simple scoring based on:
	// 1. Position (first/last sentences more important)
	// 2. Length (moderate length preferred)
	// 3. Keyword presence

	for i, sent := range sentences {
		score := 1.0

		// Position scoring
		if i == 0 {
			score += 2.0 // First sentence bonus
		} else if i == len(sentences)-1 {
			score += 1.0 // Last sentence bonus
		}

		// Length scoring (prefer moderate length)
		wordCount := len(strings.Fields(sent))
		if wordCount >= 10 && wordCount <= 30 {
			score += 1.0
		} else if wordCount < 5 || wordCount > 50 {
			score -= 0.5
		}

		// Keyword presence (simple heuristics)
		lowerSent := strings.ToLower(sent)
		keywords := []string{
			"important", "key", "main", "summary", "conclusion",
			"result", "found", "shows", "demonstrates", "indicates",
		}
		for _, kw := range keywords {
			if strings.Contains(lowerSent, kw) {
				score += 0.5
				break
			}
		}

		scores[i] = score
	}

	return scores
}

func (s *ExtractiveRuleSummarizer) selectTopSentences(sentences []string, scores []float64, count int) []string {
	// Create indexed scores
	type indexedScore struct {
		index int
		score float64
	}

	indexed := make([]indexedScore, len(sentences))
	for i := range sentences {
		indexed[i] = indexedScore{index: i, score: scores[i]}
	}

	// Sort by score (descending)
	for i := 0; i < len(indexed)-1; i++ {
		for j := i + 1; j < len(indexed); j++ {
			if indexed[j].score > indexed[i].score {
				indexed[i], indexed[j] = indexed[j], indexed[i]
			}
		}
	}

	// Select top N and sort back by original position
	selected := indexed[:count]
	for i := 0; i < len(selected)-1; i++ {
		for j := i + 1; j < len(selected); j++ {
			if selected[j].index < selected[i].index {
				selected[i], selected[j] = selected[j], selected[i]
			}
		}
	}

	// Extract sentences
	result := make([]string, len(selected))
	for i, s := range selected {
		result[i] = sentences[s.index]
	}

	return result
}

func splitIntoSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	for i, r := range text {
		current.WriteRune(r)

		if r == '.' || r == '!' || r == '?' {
			// Check if followed by space or end
			if i == len(text)-1 || (i < len(text)-1 && text[i+1] == ' ') {
				sentence := strings.TrimSpace(current.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				current.Reset()
			}
		}
	}

	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		sentences = append(sentences, remaining)
	}

	return sentences
}

// ConversationSummarizer summarizes conversation history.
type ConversationSummarizer struct {
	backend LLMBackend
	config  *ConversationSummaryConfig
}

// ConversationSummaryConfig holds configuration for conversation summarization.
type ConversationSummaryConfig struct {
	// MaxTurns is the maximum turns before summarization.
	MaxTurns int `json:"max_turns"`
	// SummaryMaxTokens is the max tokens for the summary.
	SummaryMaxTokens int `json:"summary_max_tokens"`
	// PreserveLastN keeps the last N turns verbatim.
	PreserveLastN int `json:"preserve_last_n"`
	// IncludeSpeakers includes speaker identification.
	IncludeSpeakers bool `json:"include_speakers"`
}

// DefaultConversationSummaryConfig returns default conversation config.
func DefaultConversationSummaryConfig() *ConversationSummaryConfig {
	return &ConversationSummaryConfig{
		MaxTurns:         20,
		SummaryMaxTokens: 256,
		PreserveLastN:    4,
		IncludeSpeakers:  true,
	}
}

// NewConversationSummarizer creates a new conversation summarizer.
func NewConversationSummarizer(backend LLMBackend, config *ConversationSummaryConfig) *ConversationSummarizer {
	if config == nil {
		config = DefaultConversationSummaryConfig()
	}
	return &ConversationSummarizer{
		backend: backend,
		config:  config,
	}
}

// Turn represents a conversation turn.
type Turn struct {
	// Role is the speaker role (user, assistant, etc.).
	Role string `json:"role"`
	// Content is the turn content.
	Content string `json:"content"`
	// Timestamp is when the turn occurred.
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// ConversationSummaryResult contains the summarization result.
type ConversationSummaryResult struct {
	// Summary is the summarized conversation history.
	Summary string `json:"summary"`
	// PreservedTurns are the verbatim preserved turns.
	PreservedTurns []Turn `json:"preserved_turns"`
	// SummarizedTurnCount is the number of turns summarized.
	SummarizedTurnCount int `json:"summarized_turn_count"`
}

// Summarize summarizes a conversation.
func (s *ConversationSummarizer) Summarize(ctx context.Context, turns []Turn) (*ConversationSummaryResult, error) {
	if len(turns) == 0 {
		return nil, ErrNoContent
	}

	// If not enough turns, return as-is
	if len(turns) <= s.config.PreserveLastN {
		return &ConversationSummaryResult{
			Summary:             "",
			PreservedTurns:      turns,
			SummarizedTurnCount: 0,
		}, nil
	}

	// Split into to-summarize and to-preserve
	toSummarize := turns[:len(turns)-s.config.PreserveLastN]
	toPreserve := turns[len(turns)-s.config.PreserveLastN:]

	// Build conversation text
	var convBuilder strings.Builder
	for _, turn := range toSummarize {
		if s.config.IncludeSpeakers {
			convBuilder.WriteString(fmt.Sprintf("%s: ", turn.Role))
		}
		convBuilder.WriteString(turn.Content)
		convBuilder.WriteString("\n")
	}

	// Summarize
	prompt := fmt.Sprintf(
		"Summarize this conversation history concisely, capturing the main topics discussed and any important decisions or information exchanged. Keep the summary under %d words:\n\n%s\n\nSummary:",
		s.config.SummaryMaxTokens,
		convBuilder.String(),
	)

	summary, err := s.backend.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSummarizationFailed, err)
	}

	return &ConversationSummaryResult{
		Summary:             strings.TrimSpace(summary),
		PreservedTurns:      toPreserve,
		SummarizedTurnCount: len(toSummarize),
	}, nil
}

// IncrementalSummarizer summarizes incrementally as content grows.
type IncrementalSummarizer struct {
	mu             sync.Mutex
	backend        LLMBackend
	config         *IncrementalConfig
	currentSummary string
	newContent     strings.Builder
	contentCount   int
	summarizeAfter int
}

// IncrementalConfig holds configuration for incremental summarization.
type IncrementalConfig struct {
	// SummarizeAfterItems triggers summarization after N items.
	SummarizeAfterItems int `json:"summarize_after_items"`
	// MaxSummaryTokens is the max tokens for cumulative summary.
	MaxSummaryTokens int `json:"max_summary_tokens"`
	// Compression is the target compression ratio.
	Compression float64 `json:"compression"`
}

// DefaultIncrementalConfig returns default incremental config.
func DefaultIncrementalConfig() *IncrementalConfig {
	return &IncrementalConfig{
		SummarizeAfterItems: 10,
		MaxSummaryTokens:    512,
		Compression:         0.3,
	}
}

// NewIncrementalSummarizer creates a new incremental summarizer.
func NewIncrementalSummarizer(backend LLMBackend, config *IncrementalConfig) *IncrementalSummarizer {
	if config == nil {
		config = DefaultIncrementalConfig()
	}
	return &IncrementalSummarizer{
		backend:        backend,
		config:         config,
		summarizeAfter: config.SummarizeAfterItems,
	}
}

// Add adds content to be summarized.
func (s *IncrementalSummarizer) Add(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.newContent.Len() > 0 {
		s.newContent.WriteString("\n")
	}
	s.newContent.WriteString(content)
	s.contentCount++
}

// Update updates the summary if needed.
func (s *IncrementalSummarizer) Update(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.contentCount < s.summarizeAfter {
		return s.currentSummary, nil
	}

	// Build prompt combining previous summary with new content
	var promptBuilder strings.Builder

	if s.currentSummary != "" {
		promptBuilder.WriteString("Previous summary:\n")
		promptBuilder.WriteString(s.currentSummary)
		promptBuilder.WriteString("\n\nNew content:\n")
	}

	promptBuilder.WriteString(s.newContent.String())
	promptBuilder.WriteString(fmt.Sprintf("\n\nCreate an updated summary (max %d words) that incorporates both the previous summary and new content:\n\nSummary:", s.config.MaxSummaryTokens))

	summary, err := s.backend.Complete(ctx, promptBuilder.String())
	if err != nil {
		return s.currentSummary, fmt.Errorf("%w: %v", ErrSummarizationFailed, err)
	}

	s.currentSummary = strings.TrimSpace(summary)
	s.newContent.Reset()
	s.contentCount = 0

	return s.currentSummary, nil
}

// GetSummary returns the current summary.
func (s *IncrementalSummarizer) GetSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentSummary
}

// Reset resets the summarizer state.
func (s *IncrementalSummarizer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentSummary = ""
	s.newContent.Reset()
	s.contentCount = 0
}

// SummaryCache caches summaries to avoid re-computation.
// Concurrent-safe by construction (CONST-029): cache is a safe.Store.
type SummaryCache struct {
	cache *safe.Store[string, CachedSummary]
	ttl   time.Duration
}

// CachedSummary represents a cached summary.
type CachedSummary struct {
	// Summary is the cached summary.
	Summary string `json:"summary"`
	// CreatedAt is when the summary was created.
	CreatedAt time.Time `json:"created_at"`
	// ContentHash is the hash of the original content.
	ContentHash string `json:"content_hash"`
}

// NewSummaryCache creates a new summary cache.
func NewSummaryCache(ttl time.Duration) *SummaryCache {
	return &SummaryCache{
		cache: safe.NewStore[string, CachedSummary](),
		ttl:   ttl,
	}
}

// Get retrieves a cached summary.
func (c *SummaryCache) Get(key string) (string, bool) {
	cached, exists := c.cache.Get(key)
	if !exists {
		return "", false
	}
	if time.Since(cached.CreatedAt) > c.ttl {
		return "", false
	}
	return cached.Summary, true
}

// Set stores a summary in the cache.
func (c *SummaryCache) Set(key, summary string) {
	c.cache.Put(key, CachedSummary{
		Summary:     summary,
		CreatedAt:   time.Now(),
		ContentHash: key,
	})
}

// Clear clears the cache.
func (c *SummaryCache) Clear() {
	c.cache.Clear()
}

// Cleanup removes expired entries.
// Iterates a Snapshot before per-key Delete to avoid mutating during Range.
func (c *SummaryCache) Cleanup() {
	now := time.Now()
	for key, cached := range c.cache.Snapshot() {
		if now.Sub(cached.CreatedAt) > c.ttl {
			c.cache.Delete(key)
		}
	}
}
