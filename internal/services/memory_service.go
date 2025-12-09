package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/superagent/superagent/internal/config"
	llm "github.com/superagent/superagent/internal/llm/cognee"
	"github.com/superagent/superagent/internal/models"
)

// MemoryService provides memory enhancement capabilities using Cognee
type MemoryService struct {
	client      *llm.Client
	enabled     bool
	dataset     string
	cache       map[string][]models.MemorySource
	ttl         time.Duration
	lastCleanup time.Time
}

// NewMemoryService creates a new memory service instance
func NewMemoryService(cfg *config.Config) *MemoryService {
	if cfg == nil || !cfg.Cognee.AutoCognify {
		return &MemoryService{
			enabled: false,
			cache:   make(map[string][]models.MemorySource),
			ttl:     5 * time.Minute,
		}
	}

	return &MemoryService{
		client: llm.NewClient(&config.Config{
			Cognee: config.Cognee,
		}),
		enabled: true,
		dataset: "default",
		cache:   make(map[string][]models.MemorySource),
		ttl:     5 * time.Minute,
	}
}

// AddMemory adds content to memory using Cognee
func (m *MemoryService) AddMemory(ctx context.Context, req *MemoryRequest) error {
	if !m.enabled {
		return fmt.Errorf("memory service is disabled")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", req.ContentType, strings.ToLower(req.Content[:min(50, len(req.Content))]))
	if sources, exists := m.cache[cacheKey]; exists && len(sources) > 0 {
		// Return cached results
		return nil
	}

	// Add to Cognee
	memReq := &llm.MemoryRequest{
		Content:     req.Content,
		DatasetName: req.DatasetName,
		ContentType: req.ContentType,
	}

	resp, err := m.client.AddMemory(ctx, memReq)
	if err != nil {
		return fmt.Errorf("failed to add memory to Cognee: %w", err)
	}

	// Convert to MemorySource format
	sources := m.convertToMemorySources(resp)

	// Cache the results
	m.cache[cacheKey] = sources

	return nil
}

// SearchMemory searches memory for relevant content
func (m *MemoryService) SearchMemory(ctx context.Context, req *SearchRequest) ([]models.MemorySource, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory service is disabled")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("search:%s", strings.ToLower(req.Query))
	if sources, exists := m.cache[cacheKey]; exists {
		// Return cached results
		return sources, nil
	}

	// Search Cognee
	cogneeReq := &llm.SearchRequest{
		Query:       req.Query,
		DatasetName: req.DatasetName,
		Limit:       req.Limit,
	}

	resp, err := m.client.SearchMemory(ctx, cogneeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search memory in Cognee: %w", err)
	}

	// Convert to MemorySource format
	sources := m.convertToMemorySources(resp)

	// Cache the results
	m.cache[cacheKey] = sources

	return sources, nil
}

// EnhanceRequest enhances a request with relevant memory context
func (m *MemoryService) EnhanceRequest(ctx context.Context, req *models.LLMRequest) error {
	if !m.enabled || !req.MemoryEnhanced {
		return nil
	}

	// Search for relevant memory sources
	sources, err := m.SearchMemory(ctx, &SearchRequest{
		Query:       m.extractKeywords(req),
		DatasetName: m.dataset,
		Limit:       5, // Limit for performance
	})
	if err != nil {
		return fmt.Errorf("failed to enhance request with memory: %w", err)
	}

	// Add memory sources to request
	if req.Memory == nil {
		req.Memory = make(map[string]string)
	}

	// Add context from memory sources
	for i, source := range sources {
		if source.Content != "" {
			req.Memory[fmt.Sprintf("memory_%d", i)] = source.Content
		}
	}

	return nil
}

// GetMemorySources returns memory sources for a request
func (m *MemoryService) GetMemorySources(ctx context.Context, req *models.LLMRequest) ([]models.MemorySource, error) {
	if !m.enabled {
		return nil, fmt.Errorf("memory service is disabled")
	}

	// Search for relevant memory sources
	sources, err := m.SearchMemory(ctx, &SearchRequest{
		Query:       m.extractKeywords(req),
		DatasetName: m.dataset,
		Limit:       10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get memory sources: %w", err)
	}

	return sources, nil
}

// CacheCleanup removes expired cache entries
func (m *MemoryService) CacheCleanup() {
	// Simple cleanup - remove all entries older than TTL
	// In a real implementation, this would run periodically
	now := time.Now()

	for key := range m.cache {
		// Mark all entries as expired for simplicity
		delete(m.cache, key)
	}

	// This is a simplified cleanup - real implementation would track timestamps
	m.lastCleanup = now
}

// convertToMemorySources converts Cognee responses to MemorySource format
func (m *MemoryService) convertToMemorySources(resp *llm.MemoryResponse) []models.MemorySource {
	if resp == nil {
		return nil
	}

	sources := make([]models.MemorySource, 0, len(resp.GraphNodes))

	for _, node := range resp.GraphNodes {
		if node.Content != "" {
			sources = append(sources, models.MemorySource{
				DatasetName:    resp.DatasetName,
				Content:        node.Content,
				RelevanceScore: 1.0, // Default relevance
				SourceType:     "cognee",
			})
		}
	}

	return sources
}

// convertToMemorySources converts Cognee search responses to MemorySource format
func (m *MemoryService) convertToMemorySources(resp *llm.MemoryResponse) []models.MemorySource {
	if resp == nil {
		return nil
	}

	sources := make([]models.MemorySource, 0, len(resp.Results))

	for _, result := range resp.Results {
		sources = append(sources, models.MemorySource{
			DatasetName:    result.DatasetName,
			Content:        result.Content,
			RelevanceScore: result.RelevanceScore,
			SourceType:     "cognee",
		})
	}

	return sources
}

// extractKeywords extracts keywords from a request for memory search
func (m *MemoryService) extractKeywords(req *models.LLMRequest) string {
	keywords := []string{}

	// Extract from prompt
	if req.Prompt != "" {
		keywords = append(keywords, strings.Fields(req.Prompt)...)
	}

	// Extract from messages
	for _, msg := range req.Messages {
		if msg.Content != "" {
			keywords = append(keywords, strings.Fields(msg.Content)...)
		}
	}

	// Return first few keywords as search query
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return strings.Join(keywords, " ")
}

// MemoryRequest represents a request to add memory
type MemoryRequest struct {
	Content     string `json:"content"`
	DatasetName string `json:"dataset_name"`
	ContentType string `json:"content_type"`
}

// SearchRequest represents a request to search memory
type SearchRequest struct {
	Query       string `json:"query"`
	DatasetName string `json:"dataset_name"`
	Limit       int    `json:"limit"`
}

// IsEnabled returns whether memory service is enabled
func (m *MemoryService) IsEnabled() bool {
	return m.enabled
}

// ClearCache clears the memory cache
func (m *MemoryService) ClearCache() {
	m.cache = make(map[string][]models.MemorySource)
}

// GetStats returns memory service statistics
func (m *MemoryService) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":     m.enabled,
		"cache_size":  len(m.cache),
		"dataset":     m.dataset,
		"ttl_minutes": m.ttl.Minutes(),
		"cognee_url":  m.client.GetBaseURL(),
	}
}
